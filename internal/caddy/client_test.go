package caddy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadWritesFile verifies that Load writes the config JSON to disk. The
// `podman exec caddy reload` will fail in a unit test (no podman/caddy), so
// we only check the file-write part.
func TestLoadWritesFile(t *testing.T) {
	dir := t.TempDir()
	c := New(nil, "basepod-caddy-test", dir)
	cfg := []byte(`{"apps":{"http":{}}}`)
	_ = c.Load(context.Background(), cfg) // exec will fail; ignore
	b, err := os.ReadFile(filepath.Join(dir, "current.json"))
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if string(b) != string(cfg) {
		t.Fatalf("file contents: %s", b)
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	c := New(nil, "x", t.TempDir())
	if err := c.Load(context.Background(), []byte("{not json")); err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestGetReturnsLastWritten(t *testing.T) {
	dir := t.TempDir()
	c := New(nil, "x", dir)
	_ = os.WriteFile(filepath.Join(dir, "current.json"), []byte(`{"v":1}`), 0o600)
	b, err := c.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"v":1}` {
		t.Fatalf("got %s", b)
	}
}

func TestGetMissingReturnsNil(t *testing.T) {
	c := New(nil, "x", t.TempDir())
	b, err := c.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if b != nil {
		t.Fatalf("expected nil, got %s", b)
	}
}
