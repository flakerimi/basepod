package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error":   message,
		"code":    code,
		"status":  status,
	})
}

func healthHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		out := map[string]any{
			"status":  "ok",
			"version": d.Version,
			"time":    time.Now().Unix(),
		}
		if d.Podman != nil {
			out["podman"] = d.Podman.Ping(ctx) == nil
		}
		if d.Caddy != nil {
			out["caddy"] = d.Caddy.Ping(ctx) == nil
		}
		writeJSON(w, http.StatusOK, out)
	}
}
