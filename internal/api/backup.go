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
//   basepod-backup-YYYYMMDD-HHMMSS/
//   ├── backup.json             - manifest
//   ├── database/state.db
//   ├── config/
//   │   ├── caddy.json          - rendered Caddy admin snapshot
//   │   └── basepod.yaml        - server config (sans secrets)
//   ├── volumes/<name>.tar      - one tar per Podman named volume in use
//   └── data/<app>/...          - bind-mount host data tree
//
// The manifest enumerates what's inside so a restore tool can be selective.

type backupManifest struct {
	ID          string   `json:"id"`
	CreatedAt   string   `json:"created_at"`
	Version     string   `json:"basepod_version"`
	Database    bool     `json:"database"`
	Config      bool     `json:"config"`
	Volumes     []string `json:"volumes"`
	Apps        []string `json:"apps"`
	Compression string   `json:"compression"`
}

func backupHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := time.Now().UTC().Format("20060102-150405")
		root := "basepod-backup-" + id
		name := root + ".tar.gz"

		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", "attachment; filename="+name)

		// Collect named volumes referenced by any app.
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

		manifest := backupManifest{
			ID:          id,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			Version:     d.Version,
			Database:    true,
			Config:      true,
			Volumes:     volList,
			Apps:        appNames,
			Compression: "gzip",
		}

		gz := gzip.NewWriter(w)
		defer gz.Close()
		tw := tar.NewWriter(gz)
		defer tw.Close()

		// manifest
		mb, _ := json.MarshalIndent(manifest, "", "  ")
		_ = addBytes(tw, root+"/backup.json", mb)

		// database
		_, _ = d.DB.ExecContext(r.Context(), "PRAGMA wal_checkpoint(FULL);")
		if err := addFile(tw, d.Cfg.StatePath(), root+"/database/state.db"); err != nil {
			d.Log.Error("backup state.db", "err", err)
			return
		}

		// config — caddy snapshot + sanitized server config
		if d.Caddy != nil {
			if cfg, err := d.Caddy.Get(r.Context()); err == nil {
				_ = addBytes(tw, root+"/config/caddy.json", cfg)
			}
		}
		if cfgYAML, err := os.ReadFile(d.Cfg.ConfigPath()); err == nil {
			_ = addBytes(tw, root+"/config/basepod.yaml", cfgYAML)
		}

		// volumes — podman volume export, one tar per named volume
		if d.Podman != nil {
			for _, v := range volList {
				if err := writeVolumeTar(r.Context(), tw, d.Podman, v, root+"/volumes/"+v+".tar"); err != nil {
					d.Log.Warn("backup volume failed", "vol", v, "err", err)
				}
			}
		}

		// data — bind-mount host paths under DataDir (excluding _basepod)
		_ = addDir(tw, d.Cfg.DataDir, root+"/data", filepath.Join(d.Cfg.DataDir, "_basepod"))
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
