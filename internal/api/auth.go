package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/flakerimi/basepod/internal/auth"
)

func authStatusHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, _ := d.Queries.CountUsers(r.Context())
		writeJSON(w, 200, map[string]any{"setup_complete": n > 0})
	}
}

func setupHandler(d Deps) http.HandlerFunc {
	type req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := d.Queries.CountUsers(r.Context())
		if err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		if n > 0 {
			writeErr(w, 409, "setup_complete", "setup already completed")
			return
		}
		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || len(body.Password) < 8 {
			writeErr(w, 400, "bad_request", "username and password (min 8 chars) required")
			return
		}
		if _, err := d.Auth.EnsureAdmin(r.Context(), body.Username, body.Password); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 201, map[string]any{"ok": true})
	}
}

func loginHandler(d Deps) http.HandlerFunc {
	type req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, "bad_request", "invalid JSON")
			return
		}
		sid, u, err := d.Auth.Login(r.Context(), body.Username, body.Password)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				writeErr(w, 401, "invalid_credentials", "invalid credentials")
				return
			}
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookie,
			Value:    sid,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
			SameSite: http.SameSiteStrictMode,
			Expires:  time.Now().Add(auth.SessionTTL),
		})
		writeJSON(w, 200, map[string]any{"user": u})
	}
}

func logoutHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(auth.SessionCookie); err == nil {
			_ = d.Auth.Logout(r.Context(), c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: auth.SessionCookie, Path: "/", MaxAge: -1})
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func meHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFromContext(r.Context())
		writeJSON(w, 200, map[string]any{"user": u})
	}
}

func listTokensHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFromContext(r.Context())
		rows, err := d.Auth.ListTokens(r.Context(), u.ID)
		if err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"tokens": rows})
	}
}

func createTokenHandler(d Deps) http.HandlerFunc {
	type req struct {
		Name string `json:"name"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b req
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Name == "" {
			writeErr(w, 400, "bad_request", "name required")
			return
		}
		u, _ := auth.UserFromContext(r.Context())
		tok, err := d.Auth.IssueToken(r.Context(), u.ID, b.Name)
		if err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 201, map[string]any{"token": tok, "name": b.Name})
	}
}

func revokeTokenHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		u, _ := auth.UserFromContext(r.Context())
		if err := d.Auth.RevokeToken(r.Context(), u.ID, id); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}
