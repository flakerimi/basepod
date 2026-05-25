package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flakerimi/basepod/internal/podman"
)

// Backup layout (matches the established BasePod backup format):
//
//   backup.json                   - manifest at tar root, no outer dir
//   database/state.db
//   config/caddy.json             - rendered Caddy admin snapshot
//   config/basepod.yaml           - server config (sans secrets)
//   volumes/<name>.tar            - one tar per Podman named volume in use
//   apps/<app>/...                - bind-mount host data tree per app
//   builds/<app>/<version>/source.tar.gz  - retained deploy source archives
//
// The manifest enumerates what's inside so a restore tool can be selective.

type backupContents struct {
	Database bool     `json:"database"`
	Config   bool     `json:"config"`
	Volumes  []string `json:"volumes"`
	Apps     []string `json:"apps"`
	Builds   []string `json:"builds"`
}

type backupManifest struct {
	ID          string          `json:"id"`
	CreatedAt   string          `json:"created_at"`
	Version     string          `json:"basepod_version"`
	Compression string          `json:"compression"`
	Contents    backupContents  `json:"contents"`
}

func backupHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := time.Now().UTC().Format("20060102-150405")
		name := "basepod-backup-" + id + ".tar.gz"

		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", "attachment; filename="+name)

		apps, _ := d.Apps.List(r.Context())
		appNames := make([]string, 0, len(apps))
		volSet := map[string]struct{}{}
		for _, a := range apps {
			appNames = append(appNames, a.Name)
			for _, v := range a.Volumes {
				if v.NamedVolume != "" {
					volSet[v.NamedVolume] = struct{}{}
				}
			}
		}
		volList := make([]string, 0, len(volSet))
		for v := range volSet {
			volList = append(volList, v)
		}

		buildsRoot := filepath.Join(d.Cfg.DataDir, "_basepod", "builds")
		buildEntries := []string{}
		if entries, err := os.ReadDir(buildsRoot); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					buildEntries = append(buildEntries, e.Name())
				}
			}
		}

		manifest := backupManifest{
			ID:          id,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			Version:     d.Version,
			Compression: "gzip",
			Contents: backupContents{
				Database: true,
				Config:   true,
				Volumes:  volList,
				Apps:     appNames,
				Builds:   buildEntries,
			},
		}

		gz := gzip.NewWriter(w)
		defer gz.Close()
		tw := tar.NewWriter(gz)
		defer tw.Close()

		// manifest at tar root (no outer dir)
		mb, _ := json.MarshalIndent(manifest, "", "  ")
		_ = addBytes(tw, "backup.json", mb)

		// database/
		_, _ = d.DB.ExecContext(r.Context(), "PRAGMA wal_checkpoint(FULL);")
		if err := addFile(tw, d.Cfg.StatePath(), "database/state.db"); err != nil {
			d.Log.Error("backup state.db", "err", err)
			return
		}

		// config/
		if d.Caddy != nil {
			if cfg, err := d.Caddy.Get(r.Context()); err == nil {
				_ = addBytes(tw, "config/caddy.json", cfg)
			}
		}
		if cfgYAML, err := os.ReadFile(d.Cfg.ConfigPath()); err == nil {
			_ = addBytes(tw, "config/basepod.yaml", cfgYAML)
		}

		// volumes/
		if d.Podman != nil {
			for _, v := range volList {
				if err := writeVolumeTar(r.Context(), tw, d.Podman, v, "volumes/"+v+".tar"); err != nil {
					d.Log.Warn("backup volume failed", "vol", v, "err", err)
				}
			}
		}

		// apps/<name>/...  — bind-mount data per app
		for _, a := range apps {
			for _, vol := range a.Volumes {
				if vol.Host == "" {
					continue
				}
				prefix := filepath.Join("apps", a.Name, filepath.Base(vol.Container))
				_ = addDir(tw, vol.Host, prefix, "")
			}
		}

		// builds/
		if len(buildEntries) > 0 {
			_ = addDir(tw, buildsRoot, "builds", "")
		}
	}
}

func restoreHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotImplemented, "manual_restore",
			"automated restore is not implemented in v1. Manual steps: 1) stop the server, "+
				"2) replace ~/BasePodData/_basepod/state.db with database/state.db from the backup, "+
				"3) restore ~/BasePodData/<app>/ from data/<app>/, "+
				"4) for each volumes/<name>.tar run `podman volume import <name> volumes/<name>.tar`, "+
				"5) restart the server.")
	}
}

func writeVolumeTar(ctx context.Context, tw *tar.Writer, pc *podman.Client, name, archivePath string) error {
	// Run `podman volume export` via the local podman binary — the libpod
	// REST endpoint for volume export is gated behind compat mode.
	tmp, err := os.CreateTemp("", "vol-*.tar")
	if err != nil {
		return err
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()
	if err := pc.VolumeExportToFile(ctx, name, tmp.Name()); err != nil {
		return err
	}
	info, err := os.Stat(tmp.Name())
	if err != nil {
		return err
	}
	hdr := &tar.Header{
		Name:    archivePath,
		Mode:    0o644,
		Size:    info.Size(),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(tw, tmp)
	return err
}

func addBytes(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), ModTime: time.Now()}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func addFile(tw *tar.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

func addDir(tw *tar.Writer, root, prefix, skip string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if skip != "" && (path == skip || strings.HasPrefix(path, skip+string(filepath.Separator))) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.Join(prefix, rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}
