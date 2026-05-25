package auth

import (
	"context"
	"net/http"
	"strings"
)

const (
	SessionCookie = "basepod_session"
)

type ctxKey int

const userCtxKey ctxKey = 1

// UserFromContext returns the authenticated user attached to r.Context().
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey).(User)
	return u, ok
}

// Middleware enforces auth on protected routes.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.resolve(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) resolve(r *http.Request) (User, bool) {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		tok := strings.TrimPrefix(h, "Bearer ")
		u, err := s.ResolveToken(r.Context(), tok)
		if err == nil {
			return u, true
		}
	}
	if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
		u, err := s.ResolveSession(r.Context(), c.Value)
		if err == nil {
			return u, true
		}
	}
	return User{}, false
}
