package api

import (
	"encoding/json"
	"net/http"

	"github.com/flakerimi/basepod/internal/store/db"
)

var allowedSettings = map[string]bool{
	"root_domain":  true,
	"acme_email":   true,
	"dns_provider": true,
	"dns_token":    true,
}

func getSettingsHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := d.Queries.ListSettings(r.Context())
		if err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		out := map[string]string{}
		for _, row := range rows {
			if row.Key == "dns_token" {
				if row.Value != "" {
					out[row.Key] = "***"
				}
				continue
			}
			out[row.Key] = row.Value
		}
		writeJSON(w, 200, map[string]any{"settings": out})
	}
}

func putSettingsHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, "bad_request", "invalid JSON")
			return
		}
		for k, v := range body {
			if !allowedSettings[k] {
				writeErr(w, 400, "bad_setting", "unknown setting: "+k)
				return
			}
			if err := d.Queries.UpsertSetting(r.Context(), db.UpsertSettingParams{
				Key:   k,
				Value: v,
			}); err != nil {
				writeErr(w, 500, "server_error", err.Error())
				return
			}
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}
