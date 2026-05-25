package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.HTTPAddr != ":8080" {
		t.Fatalf("want :8080, got %q", c.HTTPAddr)
	}
	if c.DataDir == "" {
		t.Fatal("DataDir empty")
	}
}

func TestLoadCreatesDirsAndReadsYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("BASEPOD_DATA_DIR", tmp)
	t.Setenv("BASEPOD_HTTP_ADDR", ":9090")

	cfgDir := filepath.Join(tmp, "_basepod")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("log_level: debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.HTTPAddr != ":9090" {
		t.Fatalf("env override failed: %q", c.HTTPAddr)
	}
	if c.LogLevel != "debug" {
		t.Fatalf("yaml load failed: %q", c.LogLevel)
	}
	if _, err := os.Stat(c.WorkDir()); err != nil {
		t.Fatalf("workdir not created: %v", err)
	}
}
