package api

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func backupHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := fmt.Sprintf("basepod-backup-%s.tar.gz", time.Now().UTC().Format("20060102-150405"))
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", "attachment; filename="+name)

		gz := gzip.NewWriter(w)
		defer gz.Close()
		tw := tar.NewWriter(gz)
		defer tw.Close()

		// SQLite state.db (after a checkpoint to flush WAL).
		_, _ = d.DB.ExecContext(r.Context(), "PRAGMA wal_checkpoint(FULL);")
		statePath := d.Cfg.StatePath()
		if err := addFile(tw, statePath, "state.db"); err != nil {
			d.Log.Error("backup add state.db", "err", err)
			return
		}
		// Caddy JSON snapshot (best-effort).
		if d.Caddy != nil {
			if cfg, err := d.Caddy.Get(r.Context()); err == nil {
				_ = addBytes(tw, "caddy.json", cfg)
			}
		}
		// Data tree (~/BasePodData/<app>/...) — excluding the _basepod dir itself.
		_ = addDir(tw, d.Cfg.DataDir, "data", filepath.Join(d.Cfg.DataDir, "_basepod"))
	}
}

func restoreHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotImplemented, "manual_restore",
			"restore is manual in v1: stop the server, replace ~/BasePodData/_basepod/state.db with the one from the backup tar, then start the server")
	}
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
