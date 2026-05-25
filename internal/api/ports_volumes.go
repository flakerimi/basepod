package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/flakerimi/basepod/internal/store/db"
)

func addPortHandler(d Deps) http.HandlerFunc {
	type req struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		var b req
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Port < 1 || b.Port > 65535 {
			writeErr(w, 400, "bad_request", "port must be 1..65535")
			return
		}
		proto := strings.ToLower(b.Protocol)
		if proto == "" {
			proto = "tcp"
		}
		if err := d.Queries.AddAppPort(r.Context(), db.AddAppPortParams{
			ID:            uuid.NewString(),
			AppID:         a.ID,
			ContainerPort: int64(b.Port),
			Protocol:      proto,
		}); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		audit(r.Context(), d, "app.port.add", a.Name, map[string]any{"port": b.Port, "proto": proto})
		writeJSON(w, 201, map[string]any{"ok": true, "redeploy_required": true})
	}
}

func deletePortHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		port, err := strconv.Atoi(chi.URLParam(r, "port"))
		if err != nil {
			writeErr(w, 400, "bad_request", "port must be integer")
			return
		}
		if err := d.Queries.DeleteAppPort(r.Context(), db.DeleteAppPortParams{
			AppID:         a.ID,
			ContainerPort: int64(port),
		}); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		audit(r.Context(), d, "app.port.delete", a.Name, map[string]any{"port": port})
		writeJSON(w, 200, map[string]any{"ok": true, "redeploy_required": true})
	}
}

func addVolumeHandler(d Deps) http.HandlerFunc {
	type req struct {
		Container   string `json:"container"`
		Host        string `json:"host"`
		NamedVolume string `json:"named_volume"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		var b req
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Container == "" {
			writeErr(w, 400, "bad_request", "container path required")
			return
		}
		if b.Host == "" && b.NamedVolume == "" {
			writeErr(w, 400, "bad_request", "host or named_volume required")
			return
		}
		host := expandTilde(b.Host)
		if err := d.Queries.AddAppVolume(r.Context(), db.AddAppVolumeParams{
			ID:            uuid.NewString(),
			AppID:         a.ID,
			ContainerPath: b.Container,
			HostPath:      host,
			NamedVolume:   b.NamedVolume,
		}); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		audit(r.Context(), d, "app.volume.add", a.Name, map[string]any{
			"container": b.Container, "host": host, "named": b.NamedVolume,
		})
		writeJSON(w, 201, map[string]any{"ok": true, "redeploy_required": true})
	}
}

func deleteVolumeHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		cpath := chi.URLParam(r, "path")
		if err := d.Queries.DeleteAppVolumeByPath(r.Context(), db.DeleteAppVolumeByPathParams{
			AppID:         a.ID,
			ContainerPath: cpath,
		}); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		audit(r.Context(), d, "app.volume.delete", a.Name, map[string]any{"container": cpath})
		writeJSON(w, 200, map[string]any{"ok": true, "redeploy_required": true})
	}
}

func expandTilde(p string) string {
	if !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[2:])
}
