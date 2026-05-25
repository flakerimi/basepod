package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flakerimi/basepod/internal/store"
	"github.com/flakerimi/basepod/internal/store/db"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	d, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return NewService(db.New(d))
}

func TestLoginAndResolveSession(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	if _, err := s.EnsureAdmin(ctx, "admin", "secret123"); err != nil {
		t.Fatal(err)
	}
	sid, u, err := s.Login(ctx, "admin", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "admin" {
		t.Fatalf("username: %q", u.Username)
	}
	got, err := s.ResolveSession(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID {
		t.Fatal("ID mismatch")
	}
	if _, _, err := s.Login(ctx, "admin", "wrong"); err == nil {
		t.Fatal("expected ErrInvalidCredentials")
	}
}

func TestTokenIssueAndResolve(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	u, _ := s.EnsureAdmin(ctx, "admin", "pw")
	tok, err := s.IssueToken(ctx, u.ID, "ci")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ResolveToken(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID {
		t.Fatal("token user mismatch")
	}
	if _, err := s.ResolveToken(ctx, "bp_garbage"); err == nil {
		t.Fatal("expected ErrInvalidToken")
	}
}

func TestMiddleware(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	u, _ := s.EnsureAdmin(ctx, "admin", "pw")
	tok, _ := s.IssueToken(ctx, u.ID, "ci")
	sid, _, _ := s.Login(ctx, "admin", "pw")

	handler := s.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromContext(r.Context()); !ok {
			t.Fatal("no user in context")
		}
		w.WriteHeader(200)
	}))

	// Bearer
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("bearer auth: %d", w.Code)
	}
	// Cookie
	req = httptest.NewRequest("GET", "/x", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sid})
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("cookie auth: %d", w.Code)
	}
	// No auth
	req = httptest.NewRequest("GET", "/x", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: %d", w.Code)
	}
}
