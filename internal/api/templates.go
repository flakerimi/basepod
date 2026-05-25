package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/flakerimi/basepod/internal/apps"
	"github.com/flakerimi/basepod/internal/config"
	"github.com/flakerimi/basepod/internal/deploy"
	"github.com/flakerimi/basepod/internal/templates"
)

// Registry singleton wired in main; for now we make it lazy so the handler can
// build without a global.
var globalRegistry *templates.Registry

// SetTemplateRegistry is called from main to inject the registry.
func SetTemplateRegistry(r *templates.Registry) { globalRegistry = r }

func listTemplatesHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if globalRegistry == nil {
			writeJSON(w, 200, map[string]any{"templates": []any{}})
			return
		}
		writeJSON(w, 200, map[string]any{"templates": globalRegistry.List()})
	}
}

func installTemplateHandler(d Deps) http.HandlerFunc {
	type req struct {
		TemplateID string            `json:"template_id"`
		AppName    string            `json:"app_name"`
		Fields     map[string]string `json:"fields,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if globalRegistry == nil {
			writeErr(w, 503, "no_registry", "template registry not loaded")
			return
		}
		var b req
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			writeErr(w, 400, "bad_request", "invalid JSON")
			return
		}
		if !config.ValidName(b.AppName) {
			writeErr(w, 400, "bad_name", "invalid app name")
			return
		}
		t, ok := globalRegistry.Get(b.TemplateID)
		if !ok {
			writeErr(w, 404, "template_not_found", "no such template")
			return
		}
		rendered, err := t.Render(b.AppName, b.Fields)
		if err != nil {
			writeErr(w, 400, "render_error", err.Error())
			return
		}

		vols := make([]apps.Volume, 0, len(rendered.Volumes))
		for _, v := range rendered.Volumes {
			vols = append(vols, apps.Volume{
				Container:   v.Container,
				Host:        v.Host,
				NamedVolume: v.Named,
			})
		}
		healthPath := ""
		var healthCmd string
		if rendered.Healthcheck != nil {
			healthPath = rendered.Healthcheck.Path
			if len(rendered.Healthcheck.Cmd) > 0 {
				js, _ := json.Marshal(rendered.Healthcheck.Cmd)
				healthCmd = string(js)
			}
		}
		a, err := d.Apps.Create(r.Context(), apps.CreateInput{
			Name:            b.AppName,
			ImageRepo:       rendered.Image,
			Instances:       1,
			DeployStrategy:  config.StrategyStopStart,
			HealthcheckPath: healthPath,
			HealthcheckCmd:  healthCmd,
			InternalOnly:    rendered.InternalOnly,
			Ports:           rendered.Ports,
			Volumes:         vols,
		})
		if err != nil {
			if errors.Is(err, apps.ErrNameInUse) {
				writeErr(w, 409, "name_in_use", err.Error())
				return
			}
			writeErr(w, 400, "create_failed", err.Error())
			return
		}
		// Apply env (encrypted at rest).
		if err := d.Apps.ReplaceEnv(r.Context(), a.ID, rendered.Env); err != nil {
			writeErr(w, 500, "env_failed", err.Error())
			return
		}
		// Pull + deploy in background.
		go func() {
			ctx := r.Context()
			_ = d.Builder.PullImage(ctx, rendered.Image)
			version := nowVersion()
			_, _ = d.Apps.RecordVersion(ctx, a.ID, version, rendered.Image, "deploying")
			_ = d.Orchestrator.Deploy(ctx, deploy.Request{
				App:      a,
				ImageTag: rendered.Image,
				Version:  version,
				Strategy: a.DeployStrategy,
			})
		}()
		writeJSON(w, 201, map[string]any{"app": a, "template": b.TemplateID})
	}
}
