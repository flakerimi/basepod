package auth

import (
	"context"
	"net/http"
	"strings"
)

const (
	SessionCookie  = "basepod_session"
	CSRFHeader     = "X-Requested-With"
	CSRFHeaderValue = "BasePod"
)

type ctxKey int

const userCtxKey ctxKey = 1

// UserFromContext returns the authenticated user attached to r.Context().
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey).(User)
	return u, ok
}

// Middleware enforces auth on protected routes.
//
// Cookie auth additionally requires X-Requested-With: BasePod on unsafe
// methods (POST/PUT/PATCH/DELETE) — a same-site app on a sibling subdomain
// cannot send that header via a plain <form>, blocking CSRF. Bearer tokens
// bypass this check (legitimate CLI/CI usage).
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, viaBearer, ok := s.resolve(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !viaBearer && unsafeMethod(r.Method) {
			if r.Header.Get(CSRFHeader) != CSRFHeaderValue {
				http.Error(w, "csrf check failed: missing X-Requested-With header", http.StatusForbidden)
				return
			}
		}
		ctx := context.WithValue(r.Context(), userCtxKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func unsafeMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func (s *Service) resolve(r *http.Request) (User, bool, bool) {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		tok := strings.TrimPrefix(h, "Bearer ")
		u, err := s.ResolveToken(r.Context(), tok)
		if err == nil {
			return u, true, true
		}
	}
	if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
		u, err := s.ResolveSession(r.Context(), c.Value)
		if err == nil {
			return u, false, true
		}
	}
	return User{}, false, false
}
