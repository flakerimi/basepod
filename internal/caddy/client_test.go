package caddy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadAndGet(t *testing.T) {
	var lastLoad []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/load", func(w http.ResponseWriter, r *http.Request) {
		lastLoad, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	})
	mux.HandleFunc("/config/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(lastLoad)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL)
	cfg := []byte(`{"apps":{"http":{}}}`)
	if err := c.Load(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(cfg) {
		t.Fatalf("get returned %s", got)
	}
}

func TestApplyAtomicRevertsOnFailure(t *testing.T) {
	prior := []byte(`{"v":"prior"}`)
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/config/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(prior)
	})
	mux.HandleFunc("/load", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "boom", 500)
			return
		}
		// second call = revert; record success
		prior, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL)
	err := c.ApplyAtomic(context.Background(), []byte(`{"new":true}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if calls < 2 {
		t.Fatalf("expected revert call, got %d total", calls)
	}
}
