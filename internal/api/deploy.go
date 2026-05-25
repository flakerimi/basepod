package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/flakerimi/basepod/internal/apps"
	"github.com/flakerimi/basepod/internal/config"
	"github.com/flakerimi/basepod/internal/deploy"
	"github.com/flakerimi/basepod/internal/store/db"
)


// deployHandler handles POST /api/v1/apps/:name/deploy.
//
// Accepts multipart/form-data with:
//   - "tar":     gzip-tar build context (Dockerfile build)
//   - "appfile": basepod.yaml content (optional override)
//
// Or application/json with: {"image": "ghcr.io/.../foo:tag"} for image deploy.
//
// Streams progress events to the events hub under topic "app:<name>:deploy".
func deployHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			if errors.Is(err, apps.ErrAppNotFound) {
				writeErr(w, 404, "not_found", "app not found")
				return
			}
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		if d.Orchestrator == nil || d.Builder == nil {
			writeErr(w, 503, "podman_unavailable", "deploy pipeline not connected")
			return
		}

		ct := r.Header.Get("Content-Type")
		switch {
		case strings.HasPrefix(ct, "application/json"):
			handleImageDeploy(w, r, d, a)
		default:
			handleTarballDeploy(w, r, d, a)
		}
	}
}

func handleImageDeploy(w http.ResponseWriter, r *http.Request, d Deps, a *apps.App) {
	var body struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Image == "" {
		writeErr(w, 400, "bad_request", "image required")
		return
	}
	topic := "app:" + a.Name + ":deploy"
	d.Events.Publish(topic, "started", map[string]any{"image": body.Image})
	if err := d.Builder.PullImage(r.Context(), body.Image); err != nil {
		d.Events.Publish(topic, "failed", map[string]any{"error": err.Error()})
		writeErr(w, 502, "pull_failed", err.Error())
		return
	}
	version := nowVersion()
	verID, _ := d.Apps.RecordVersion(r.Context(), a.ID, version, body.Image, "deploying")
	go func() {
		ctx := context.Background()
		err := d.Orchestrator.Deploy(ctx, deploy.Request{
			App:      a,
			ImageTag: body.Image,
			Version:  version,
			Strategy: a.DeployStrategy,
		})
		updateVersionStatus(ctx, d, verID, err)
		if err != nil {
			d.Events.Publish(topic, "failed", map[string]any{"error": err.Error()})
		} else {
			d.Events.Publish(topic, "deployed", map[string]any{"version": version})
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"version": version})
}

func handleTarballDeploy(w http.ResponseWriter, r *http.Request, d Deps, a *apps.App) {
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeErr(w, 400, "bad_request", "expected multipart upload")
		return
	}
	f, _, err := r.FormFile("tar")
	if err != nil {
		writeErr(w, 400, "bad_request", "tar file required")
		return
	}
	defer f.Close()

	workdir, _ := d.Orchestrator.EnsureWorkdir(a.Name)
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	tarPath := filepath.Join(workdir, "context.tar")
	out, err := os.Create(tarPath)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	if _, err := io.Copy(out, f); err != nil {
		out.Close()
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	out.Close()

	dockerfile := "Dockerfile"
	if appfile, _, err := r.FormFile("appfile"); err == nil {
		defer appfile.Close()
		if spec, err := config.ParseAppSpec(appfile); err == nil && spec.Build.Dockerfile != "" {
			dockerfile = filepath.Base(spec.Build.Dockerfile)
		}
	}

	topic := "app:" + a.Name + ":deploy"
	d.Events.Publish(topic, "started", map[string]any{"kind": "tarball"})

	verID, _ := d.Apps.RecordVersion(r.Context(), a.ID, "pending", "", "building")
	go func() {
		ctx := context.Background()
		f, err := os.Open(tarPath)
		if err != nil {
			d.Events.Publish(topic, "failed", map[string]any{"error": err.Error()})
			return
		}
		defer f.Close()
		res, err := d.Builder.Build(ctx, a.Name, f, dockerfile, func(line string) {
			d.Events.Publish(topic, "log", map[string]any{"line": line})
		})
		if err != nil {
			updateVersionStatus(ctx, d, verID, err)
			d.Events.Publish(topic, "failed", map[string]any{"error": err.Error()})
			return
		}
		d.Events.Publish(topic, "built", map[string]any{"tag": res.Tag, "version": res.Version})

		// Retain source for backup/rollback. Keep last 5 per app.
		retainSource(d, a.Name, res.Version, tarPath)
		err = d.Orchestrator.Deploy(ctx, deploy.Request{
			App:      a,
			ImageTag: res.Tag,
			Version:  res.Version,
			Strategy: a.DeployStrategy,
		})
		updateVersionStatus(ctx, d, verID, err)
		if err != nil {
			d.Events.Publish(topic, "failed", map[string]any{"error": err.Error()})
			return
		}
		d.Events.Publish(topic, "deployed", map[string]any{"version": res.Version})
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued"})
}

func updateVersionStatus(ctx context.Context, d Deps, versionID string, err error) {
	status := "succeeded"
	excerpt := ""
	if err != nil {
		status = "failed"
		excerpt = err.Error()
	}
	_ = d.Queries.UpdateAppVersionStatus(ctx, db.UpdateAppVersionStatusParams{
		ID:         versionID,
		Status:     status,
		LogExcerpt: excerpt,
	})
}

func nowVersion() string {
	return fmt.Sprintf("v%d", time.Now().UnixNano())
}

// retainSource gzips the upload tar into _basepod/builds/<app>/<version>/source.tar.gz
// and prunes older builds so we keep at most 5 per app.
func retainSource(d Deps, appName, version, srcPath string) {
	dst := filepath.Join(d.Cfg.DataDir, "_basepod", "builds", appName, version)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		d.Log.Warn("retainSource mkdir", "err", err)
		return
	}
	in, err := os.Open(srcPath)
	if err != nil {
		d.Log.Warn("retainSource open", "err", err)
		return
	}
	defer in.Close()
	out, err := os.Create(filepath.Join(dst, "source.tar.gz"))
	if err != nil {
		d.Log.Warn("retainSource create", "err", err)
		return
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	if _, err := io.Copy(gz, in); err != nil {
		d.Log.Warn("retainSource copy", "err", err)
		return
	}
	pruneBuilds(d.Cfg.DataDir, appName, 5)
}

func pruneBuilds(dataDir, appName string, keep int) {
	root := filepath.Join(dataDir, "_basepod", "builds", appName)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	type item struct {
		name string
		mod  time.Time
	}
	items := make([]item, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, item{e.Name(), info.ModTime()})
	}
	if len(items) <= keep {
		return
	}
	// oldest first
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].mod.Before(items[i].mod) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	for _, it := range items[:len(items)-keep] {
		_ = os.RemoveAll(filepath.Join(root, it.name))
	}
}
