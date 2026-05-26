package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server string `yaml:"server"`
	Token  string `yaml:"token,omitempty"`
}

type Client struct {
	cfg  Config
	http *http.Client
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".basepod", "config.yaml")
}

func loadConfig() (Config, error) {
	var c Config
	b, err := os.ReadFile(configPath())
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}

func saveConfig(c Config) error {
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), b, 0o600)
}

func newClient(serverFlag string) (*Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if serverFlag != "" {
		cfg.Server = serverFlag
	}
	if cfg.Server == "" {
		cfg.Server = "http://localhost:8080"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 60 * time.Second}}, nil
}

func (c *Client) url(p string) string { return c.cfg.Server + p }

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), body)
	if err != nil {
		return nil, err
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.http.Do(req)
}

func (c *Client) JSON(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	resp, err := c.do(ctx, method, path, body, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
	}
	if out == nil {
		return nil
	}
	if resp.ContentLength == 0 {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Stream connects to an SSE endpoint and calls onLine for each "data:" payload.
func (c *Client) Stream(ctx context.Context, path string, onLine func(string)) error {
	resp, err := c.do(ctx, "GET", path, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
	}
	sc := newScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if len(line) > 6 && line[:6] == "data: " {
			onLine(line[6:])
		}
	}
	return sc.Err()
}
