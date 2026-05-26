package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DataDir      string `yaml:"data_dir"`
	HTTPAddr     string `yaml:"http_addr"`
	PodmanSocket string `yaml:"podman_socket"`
	// CaddyAdmin is retained only to load legacy config files. Caddy admin
	// now lives on a Unix socket inside the container; this field is unused.
	CaddyAdmin string `yaml:"caddy_admin,omitempty"`
	JWTSecret  string `yaml:"jwt_secret"`
	LogLevel   string `yaml:"log_level"`
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		DataDir:      filepath.Join(home, "BasePodData"),
		HTTPAddr:     ":8080",
		PodmanSocket: "",
		LogLevel:     "info",
	}
}

func (c Config) StatePath() string {
	return filepath.Join(c.DataDir, "_basepod", "state.db")
}

func (c Config) ConfigPath() string {
	return filepath.Join(c.DataDir, "_basepod", "config.yaml")
}

func (c Config) WorkDir() string {
	return filepath.Join(c.DataDir, "_basepod", "work")
}

func Load() (Config, error) {
	cfg := Default()
	if env := os.Getenv("BASEPOD_DATA_DIR"); env != "" {
		cfg.DataDir = env
	}
	if env := os.Getenv("BASEPOD_HTTP_ADDR"); env != "" {
		cfg.HTTPAddr = env
	}
	path := cfg.ConfigPath()
	if _, err := os.Stat(path); err == nil {
		b, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config: %w", err)
		}
	}
	if err := cfg.ensureDirs(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) ensureDirs() error {
	for _, d := range []string{
		c.DataDir,
		filepath.Join(c.DataDir, "_basepod"),
		c.WorkDir(),
	} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func (c Config) Save() error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.ConfigPath(), b, 0o600)
}
