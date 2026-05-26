package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flakerimi/basepod/internal/podman"
)

// AdminSocketPathInContainer is the Caddy admin Unix socket path inside the
// basepod-caddy container. It lives on a tmpfs/volume that is NOT shared with
// any other container. The socket is the ONLY way to talk to Caddy admin.
const AdminSocketPathInContainer = "unix//var/run/caddy/admin.sock"

// AdminSocketArg is the form expected by `caddy reload --address …`.
const AdminSocketArg = "unix//var/run/caddy/admin.sock"

// ConfigFileInContainer is where the rendered JSON config lives inside the
// Caddy container — bind-mounted from the host so basepod-server can write it
// without going through the admin API.
const ConfigFileInContainer = "/etc/caddy/current.json"

// Client controls Caddy via the local `podman` CLI's exec interface, against
// the basepod-caddy container. Construction needs:
//   - pc:           podman REST client (for diagnostics)
//   - containerName: usually "basepod-caddy"
//   - hostConfigDir: host directory bind-mounted into the container at /etc/caddy
type Client struct {
	pc            *podman.Client
	containerName string
	hostConfigDir string
}

func New(pc *podman.Client, containerName, hostConfigDir string) *Client {
	return &Client{
		pc:            pc,
		containerName: containerName,
		hostConfigDir: hostConfigDir,
	}
}

// configPath is the host-side path to current.json.
func (c *Client) configPath() string {
	return filepath.Join(c.hostConfigDir, "current.json")
}

// Load writes the JSON config to the host-mounted file, then asks Caddy to
// reload via `caddy reload --config … --address unix//…`. Atomic on disk —
// we write to a temp file and rename.
func (c *Client) Load(ctx context.Context, cfg []byte) error {
	if c == nil {
		return errors.New("caddy: client is nil")
	}
	// Validate JSON before writing.
	var probe any
	if err := json.Unmarshal(cfg, &probe); err != nil {
		return fmt.Errorf("caddy: invalid config JSON: %w", err)
	}
	if err := os.MkdirAll(c.hostConfigDir, 0o700); err != nil {
		return fmt.Errorf("caddy: mkdir config dir: %w", err)
	}
	tmp := c.configPath() + ".tmp"
	if err := os.WriteFile(tmp, cfg, 0o600); err != nil {
		return fmt.Errorf("caddy: write tmp: %w", err)
	}
	if err := os.Rename(tmp, c.configPath()); err != nil {
		return fmt.Errorf("caddy: rename: %w", err)
	}
	out, err := podman.Exec(ctx, c.containerName, []string{
		"caddy", "reload",
		"--config", ConfigFileInContainer,
		"--address", AdminSocketArg,
	})
	if err != nil {
		return fmt.Errorf("caddy reload (exec): %w: %s", err, out)
	}
	return nil
}

// Get returns the most recent rendered config written by Load.
func (c *Client) Get(ctx context.Context) ([]byte, error) {
	if c == nil {
		return nil, errors.New("caddy: client is nil")
	}
	b, err := os.ReadFile(c.configPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// ApplyAtomic loads cfg; on failure, restores the prior config from disk.
func (c *Client) ApplyAtomic(ctx context.Context, cfg []byte) error {
	prior, _ := c.Get(ctx)
	if err := c.Load(ctx, cfg); err != nil {
		if prior != nil && !bytes.Equal(prior, []byte("null")) {
			_ = c.Load(ctx, prior)
		}
		return err
	}
	return nil
}

// Ping checks the container is up by running `caddy version` inside it.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil {
		return errors.New("caddy: client is nil")
	}
	if _, err := podman.Exec(ctx, c.containerName, []string{"caddy", "version"}); err != nil {
		return err
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
