package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	httpc   *http.Client
}

func New(adminURL string) *Client {
	return &Client{
		baseURL: adminURL,
		httpc:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Load posts a full Caddy JSON config to /load.
func (c *Client) Load(ctx context.Context, cfg []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/load", bytes.NewReader(cfg))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("caddy load: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy load %d: %s", resp.StatusCode, body)
	}
	return nil
}

// Get returns the current Caddy config as raw JSON.
func (c *Client) Get(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/config/", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("caddy get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("caddy get %d: %s", resp.StatusCode, body)
	}
	return io.ReadAll(resp.Body)
}

// ApplyAtomic loads cfg and, on failure, restores the prior config.
func (c *Client) ApplyAtomic(ctx context.Context, cfg []byte) error {
	prior, err := c.Get(ctx)
	if err != nil {
		return fmt.Errorf("snapshot prior caddy config: %w", err)
	}
	if err := c.Load(ctx, cfg); err != nil {
		if prior != nil && !bytes.Equal(prior, []byte("null")) {
			_ = c.Load(ctx, prior)
		}
		return err
	}
	return nil
}

// Ping returns nil if Caddy admin endpoint is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/config/", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("caddy admin status %d", resp.StatusCode)
	}
	return nil
}

// Encode helps callers prepare config from a Go map.
func Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
