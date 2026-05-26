package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/flakerimi/basepod/internal/caddy"
	"github.com/flakerimi/basepod/internal/config"
	"github.com/flakerimi/basepod/internal/podman"
)

const (
	NetworkName     = "basepod"
	CaddyContainer  = "basepod-caddy"
	CaddyImage      = "docker.io/library/caddy:2-alpine"
	CaddyDataVolume = "basepod-caddy-data"
	VMDataDir       = "/BasePodData"
)

var defaultPodmanMachineMounts = []string{"/Users", "/private", "/var/folders"}

type Result struct {
	PodmanSocket   string
	CaddyConfigDir string // host directory bind-mounted into the Caddy container at /etc/caddy
}

func Run(ctx context.Context, cfg config.Config, log *slog.Logger) (Result, error) {
	if log == nil {
		log = slog.Default()
	}
	log.Info("bootstrap: checking podman")
	if _, err := exec.LookPath("podman"); err != nil {
		return Result{}, fmt.Errorf("podman not found in PATH; install via 'brew install podman'")
	}
	if err := ensureMachine(ctx, cfg, log); err != nil {
		return Result{}, err
	}
	pc, err := podman.New(cfg.PodmanSocket)
	if err != nil {
		return Result{}, fmt.Errorf("podman client: %w", err)
	}
	if err := waitPodman(ctx, pc, log); err != nil {
		return Result{}, err
	}
	if err := ensureNetwork(ctx, pc, log); err != nil {
		return Result{}, err
	}
	configDir, err := ensureCaddy(ctx, pc, cfg, log)
	if err != nil {
		return Result{}, err
	}
	cc := caddy.New(pc, CaddyContainer, configDir)
	if err := waitCaddy(ctx, cc, log); err != nil {
		return Result{}, err
	}
	return Result{PodmanSocket: pc.SocketURI(), CaddyConfigDir: configDir}, nil
}

func ensureMachine(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	out, err := exec.CommandContext(ctx, "podman", "machine", "list", "--format", "{{.Name}} {{.Running}}").Output()
	if err != nil {
		return fmt.Errorf("podman machine list: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		log.Info("bootstrap: initializing podman machine")
		dataDir, err := filepath.Abs(cfg.DataDir)
		if err != nil {
			return fmt.Errorf("resolve data dir: %w", err)
		}
		volumeSpec := dataDir + ":" + VMDataDir
		if err := exec.CommandContext(ctx, "podman", "machine", "init", "--volume", volumeSpec).Run(); err != nil {
			return fmt.Errorf("podman machine init: %w", err)
		}
	}
	running := false
	for _, l := range lines {
		if strings.HasSuffix(l, "true") {
			running = true
			break
		}
	}
	if !running {
		log.Info("bootstrap: starting podman machine")
		if err := exec.CommandContext(ctx, "podman", "machine", "start").Run(); err != nil && !strings.Contains(err.Error(), "already") {
			return fmt.Errorf("podman machine start: %w", err)
		}
	}
	if !pathWithinAny(cfg.DataDir, defaultPodmanMachineMounts) {
		if err := exec.CommandContext(ctx, "podman", "machine", "ssh", "mountpoint", "-q", VMDataDir).Run(); err != nil {
			return fmt.Errorf("basepod data dir %s is not mounted in the podman machine; initialize the machine with 'podman machine init --volume %s:%s'", cfg.DataDir, cfg.DataDir, VMDataDir)
		}
	}
	return nil
}

func waitPodman(ctx context.Context, pc *podman.Client, log *slog.Logger) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if err := pc.Ping(ctx); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("podman REST socket not reachable after 30s")
}

func ensureNetwork(ctx context.Context, pc *podman.Client, log *slog.Logger) error {
	exists, err := pc.NetworkExists(ctx, NetworkName)
	if err != nil {
		return fmt.Errorf("network exists check: %w", err)
	}
	if exists {
		return nil
	}
	log.Info("bootstrap: creating podman network", "name", NetworkName)
	_, err = pc.NetworkCreate(ctx, podman.NetworkCreateRequest{
		Name:       NetworkName,
		Driver:     "bridge",
		DNSEnabled: true,
		Labels:     map[string]string{"basepod": "true"},
	})
	return err
}

func ensureCaddy(ctx context.Context, pc *podman.Client, cfg config.Config, log *slog.Logger) (string, error) {
	// Host-side config dir, bind-mounted into the container at /etc/caddy.
	// basepod-server writes JSON here; Caddy reads it (and is reloaded via
	// `podman exec caddy reload …`).
	hostConfigDir := filepath.Join(cfg.DataDir, "_basepod", "caddy")
	if err := os.MkdirAll(hostConfigDir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir caddy config dir: %w", err)
	}
	// Initial config: admin on Unix socket, empty server.
	initial := map[string]any{
		"admin": map[string]any{"listen": caddy.AdminSocketPathInContainer},
		"apps": map[string]any{
			"http": map[string]any{
				"servers": map[string]any{
					"basepod": map[string]any{
						"listen": []string{":80", ":443"},
						"routes": []any{},
					},
				},
			},
		},
	}
	initialJSON, _ := json.MarshalIndent(initial, "", "  ")
	cfgPath := filepath.Join(hostConfigDir, "current.json")
	if _, err := os.Stat(cfgPath); err != nil {
		if err := os.WriteFile(cfgPath, initialJSON, 0o600); err != nil {
			return "", fmt.Errorf("write initial caddy config: %w", err)
		}
	}

	if err := pc.VolumeCreate(ctx, podman.VolumeCreateRequest{Name: CaddyDataVolume, Labels: map[string]string{"basepod": "true"}}); err != nil {
		if !strings.Contains(err.Error(), "exists") {
			return "", fmt.Errorf("volume create: %w", err)
		}
	}

	vmConfigDir, err := vmPathForDataDir(cfg.DataDir, hostConfigDir)
	if err != nil {
		return "", err
	}

	exists, err := pc.ContainerExists(ctx, CaddyContainer)
	if err != nil {
		return "", err
	}
	if exists {
		ci, err := pc.ContainerInspect(ctx, CaddyContainer)
		if err != nil {
			return "", fmt.Errorf("inspect caddy: %w", err)
		}
		if caddyContainerNeedsRecreate(ci, vmConfigDir) {
			log.Warn("bootstrap: recreating legacy caddy container without Unix-socket admin")
			if ci.State.Running {
				_ = pc.ContainerStop(ctx, CaddyContainer, 10)
			}
			if err := pc.ContainerRemove(ctx, CaddyContainer, true); err != nil {
				return "", fmt.Errorf("remove legacy caddy container: %w", err)
			}
		} else {
			if !ci.State.Running {
				log.Info("bootstrap: starting existing caddy container")
				if err := pc.ContainerStart(ctx, CaddyContainer); err != nil {
					return "", err
				}
			}
			return hostConfigDir, nil
		}
	}

	log.Info("bootstrap: pulling caddy image", "ref", CaddyImage)
	if err := pc.ImagePull(ctx, CaddyImage); err != nil {
		return "", fmt.Errorf("caddy pull: %w", err)
	}

	log.Info("bootstrap: creating caddy container", "config_dir", vmConfigDir)
	req := podman.ContainerCreateRequest{
		Name:        CaddyContainer,
		Image:       CaddyImage,
		Hostname:    "caddy",
		CNINetworks: []string{NetworkName},
		Labels:      map[string]string{"basepod": "true", "basepod.role": "caddy"},
		Restart:     "always",
		Command:     []string{"caddy", "run", "--config", caddy.ConfigFileInContainer},
		PortMaps: []podman.PortMapping{
			{ContainerPort: 80, HostPort: 80, Protocol: "tcp"},
			{ContainerPort: 443, HostPort: 443, Protocol: "tcp"},
			{ContainerPort: 443, HostPort: 443, Protocol: "udp"},
			// NO admin port published — admin lives on a Unix socket reachable
			// only via `podman exec basepod-caddy`.
		},
		Mounts: []podman.Mount{
			{Type: "volume", Source: CaddyDataVolume, Destination: "/data"},
			{Type: "bind", Source: vmConfigDir, Destination: "/etc/caddy", Options: []string{"rw"}},
		},
	}
	if _, err := pc.ContainerCreate(ctx, req); err != nil {
		return "", fmt.Errorf("caddy create: %w", err)
	}
	if err := pc.ContainerStart(ctx, CaddyContainer); err != nil {
		return "", fmt.Errorf("caddy start: %w", err)
	}
	return hostConfigDir, nil
}

func caddyContainerNeedsRecreate(ci *podman.ContainerInspect, expectedConfigSource string) bool {
	if ci == nil {
		return true
	}
	if hasAdminPortBinding(ci) {
		return true
	}
	if !stringSlicesEqual(ci.Config.Cmd, []string{"caddy", "run", "--config", caddy.ConfigFileInContainer}) {
		return true
	}
	return !hasMount(ci.Mounts, expectedConfigSource, "/etc/caddy")
}

func hasAdminPortBinding(ci *podman.ContainerInspect) bool {
	for port := range ci.HostConfig.PortBindings {
		if strings.HasPrefix(port, "2019/") {
			return true
		}
	}
	return false
}

func hasMount(mounts []podman.ContainerMount, source, destination string) bool {
	for _, m := range mounts {
		if m.Destination == destination && cleanSlashPath(m.Source) == cleanSlashPath(source) {
			return true
		}
	}
	return false
}

func cleanSlashPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func vmPathForDataDir(dataDir, hostPath string) (string, error) {
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", fmt.Errorf("resolve data dir: %w", err)
	}
	absHostPath, err := filepath.Abs(hostPath)
	if err != nil {
		return "", fmt.Errorf("resolve host path: %w", err)
	}
	rel, err := filepath.Rel(absDataDir, absHostPath)
	if err != nil {
		return "", fmt.Errorf("map caddy config dir into podman VM: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("host path %s is outside data dir %s", absHostPath, absDataDir)
	}
	if rel == "." {
		if pathWithinAny(absHostPath, defaultPodmanMachineMounts) {
			return filepath.ToSlash(absHostPath), nil
		}
		return VMDataDir, nil
	}
	if pathWithinAny(absHostPath, defaultPodmanMachineMounts) {
		return filepath.ToSlash(absHostPath), nil
	}
	return filepath.ToSlash(filepath.Join(VMDataDir, rel)), nil
}

func pathWithinAny(path string, roots []string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, root := range roots {
		if pathWithin(absPath, root) {
			return true
		}
	}
	return false
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func waitCaddy(ctx context.Context, cc *caddy.Client, log *slog.Logger) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if err := cc.Ping(ctx); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("caddy admin endpoint not reachable after 30s")
}
