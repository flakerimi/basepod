package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/flakerimi/basepod/internal/auth"
	"github.com/flakerimi/basepod/internal/store/db"
)

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// audit writes an audit_log row. Failures are swallowed (it's diagnostic, not
// authoritative).
func audit(ctx context.Context, d Deps, action, target string, payload map[string]any) {
	var userID string
	if u, ok := auth.UserFromContext(ctx); ok {
		userID = u.ID
	}
	js := ""
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			js = string(b)
		}
	}
	_ = d.Queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ID:          uuid.NewString(),
		UserID:      nullStr(userID),
		Action:      action,
		Target:      target,
		PayloadJson: js,
		CreatedAt:   time.Now().Unix(),
	})
}

func auditLogHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
				limit = v
			}
		}
		rows, err := d.Queries.ListAuditLog(r.Context(), db.ListAuditLogParams{
			Limit:  int64(limit),
			Offset: 0,
		})
		if err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"entries": rows})
	}
}
