package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/flakerimi/basepod/internal/apps"
)

func listAppsHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := d.Apps.List(r.Context())
		if err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"apps": out})
	}
}

func createAppHandler(d Deps) http.HandlerFunc {
	type req struct {
		Name           string         `json:"name"`
		ImageRepo      string         `json:"image_repo,omitempty"`
		Instances      int            `json:"instances,omitempty"`
		DeployStrategy string         `json:"deploy_strategy,omitempty"`
		HealthcheckPath string        `json:"healthcheck_path,omitempty"`
		HealthcheckCmd  string        `json:"healthcheck_cmd,omitempty"`
		InternalOnly   bool           `json:"internal_only,omitempty"`
		MemoryMB       int            `json:"memory_mb,omitempty"`
		CPUPct         int            `json:"cpu_pct,omitempty"`
		Ports          []int          `json:"ports,omitempty"`
		Volumes        []apps.Volume  `json:"volumes,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b req
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			writeErr(w, 400, "bad_request", "invalid JSON")
			return
		}
		a, err := d.Apps.Create(r.Context(), apps.CreateInput{
			Name:            b.Name,
			ImageRepo:       b.ImageRepo,
			Instances:       b.Instances,
			DeployStrategy:  b.DeployStrategy,
			HealthcheckPath: b.HealthcheckPath,
			HealthcheckCmd:  b.HealthcheckCmd,
			InternalOnly:    b.InternalOnly,
			MemoryMB:        b.MemoryMB,
			CPUPct:          b.CPUPct,
			Ports:           b.Ports,
			Volumes:         b.Volumes,
		})
		if err != nil {
			switch {
			case errors.Is(err, apps.ErrNameInUse):
				writeErr(w, 409, "name_in_use", err.Error())
			default:
				writeErr(w, 400, "bad_request", err.Error())
			}
			return
		}
		writeJSON(w, 201, map[string]any{"app": a})
	}
}

func getAppHandler(d Deps) http.HandlerFunc {
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
		writeJSON(w, 200, map[string]any{"app": a})
	}
}

func updateAppHandler(d Deps) http.HandlerFunc {
	// minimal v1: not implemented
	return notImplemented("updateApp")
}

func deleteAppHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if err := d.Apps.Delete(r.Context(), name); err != nil {
			if errors.Is(err, apps.ErrAppNotFound) {
				writeErr(w, 404, "not_found", "app not found")
				return
			}
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func listVersionsHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		rows, err := d.Apps.ListVersions(r.Context(), a.ID)
		if err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"versions": rows})
	}
}

func getEnvHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		env, err := d.Apps.GetEnv(r.Context(), a.ID)
		if err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"env": env})
	}
}

func putEnvHandler(d Deps) http.HandlerFunc {
	type req struct {
		Env map[string]string `json:"env"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b req
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			writeErr(w, 400, "bad_request", "invalid JSON")
			return
		}
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		if err := d.Apps.ReplaceEnv(r.Context(), a.ID, b.Env); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func addDomainHandler(d Deps) http.HandlerFunc {
	type req struct {
		Domain string `json:"domain"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b req
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Domain == "" {
			writeErr(w, 400, "bad_request", "domain required")
			return
		}
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		if err := d.Apps.AddDomain(r.Context(), a.ID, b.Domain); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func removeDomainHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		dom := chi.URLParam(r, "domain")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		if err := d.Apps.RemoveDomain(r.Context(), a.ID, dom); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func restartAppHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if _, err := d.Apps.GetByName(r.Context(), name); err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		if d.Podman == nil {
			writeErr(w, 503, "podman_unavailable", "podman not connected")
			return
		}
		if err := d.Podman.ContainerStop(r.Context(), name, 5); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		if err := d.Podman.ContainerStart(r.Context(), name); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func rollbackHandler(d Deps) http.HandlerFunc {
	return notImplemented("rollback")
}
