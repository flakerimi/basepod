package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/flakerimi/basepod/internal/caddy"
	"github.com/flakerimi/basepod/internal/config"
	"github.com/flakerimi/basepod/internal/podman"
)

const (
	NetworkName       = "basepod"
	CaddyContainer    = "basepod-caddy"
	CaddyImage        = "docker.io/library/caddy:2-alpine"
	CaddyDataVolume   = "basepod-caddy-data"
	CaddyConfigVolume = "basepod-caddy-config"
)

type Result struct {
	PodmanSocket string
	CaddyAdmin   string
}

func Run(ctx context.Context, cfg config.Config, log *slog.Logger) (Result, error) {
	if log == nil {
		log = slog.Default()
	}
	log.Info("bootstrap: checking podman")
	if _, err := exec.LookPath("podman"); err != nil {
		return Result{}, fmt.Errorf("podman not found in PATH; install via 'brew install podman'")
	}
	if err := ensureMachine(ctx, log); err != nil {
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
	if err := ensureCaddy(ctx, pc, log); err != nil {
		return Result{}, err
	}
	cc := caddy.New(cfg.CaddyAdmin)
	if err := waitCaddy(ctx, cc, log); err != nil {
		return Result{}, err
	}
	return Result{PodmanSocket: pc.SocketURI(), CaddyAdmin: cfg.CaddyAdmin}, nil
}

func ensureMachine(ctx context.Context, log *slog.Logger) error {
	out, err := exec.CommandContext(ctx, "podman", "machine", "list", "--format", "{{.Name}} {{.Running}}").Output()
	if err != nil {
		return fmt.Errorf("podman machine list: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		log.Info("bootstrap: initializing podman machine")
		if err := exec.CommandContext(ctx, "podman", "machine", "init").Run(); err != nil {
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

func ensureCaddy(ctx context.Context, pc *podman.Client, log *slog.Logger) error {
	if err := pc.VolumeCreate(ctx, podman.VolumeCreateRequest{Name: CaddyDataVolume, Labels: map[string]string{"basepod": "true"}}); err != nil {
		// ignore "already exists" via API error sniff
		if !strings.Contains(err.Error(), "exists") {
			return fmt.Errorf("volume create: %w", err)
		}
	}
	if err := pc.VolumeCreate(ctx, podman.VolumeCreateRequest{Name: CaddyConfigVolume, Labels: map[string]string{"basepod": "true"}}); err != nil {
		if !strings.Contains(err.Error(), "exists") {
			return fmt.Errorf("volume create: %w", err)
		}
	}

	exists, err := pc.ContainerExists(ctx, CaddyContainer)
	if err != nil {
		return err
	}
	if exists {
		ci, err := pc.ContainerInspect(ctx, CaddyContainer)
		if err == nil && !ci.State.Running {
			log.Info("bootstrap: starting existing caddy container")
			return pc.ContainerStart(ctx, CaddyContainer)
		}
		return nil
	}

	log.Info("bootstrap: pulling caddy image", "ref", CaddyImage)
	if err := pc.ImagePull(ctx, CaddyImage); err != nil {
		return fmt.Errorf("caddy pull: %w", err)
	}

	log.Info("bootstrap: creating caddy container")
	req := podman.ContainerCreateRequest{
		Name:        CaddyContainer,
		Image:       CaddyImage,
		Hostname:    "caddy",
		CNINetworks: []string{NetworkName},
		Labels:      map[string]string{"basepod": "true", "basepod.role": "caddy"},
		Restart:     "always",
		Command:     []string{"caddy", "run", "--config", "/etc/caddy/caddy.json", "--resume"},
		PortMaps: []podman.PortMapping{
			{ContainerPort: 80, HostPort: 80, Protocol: "tcp"},
			{ContainerPort: 443, HostPort: 443, Protocol: "tcp"},
			{ContainerPort: 443, HostPort: 443, Protocol: "udp"},
			{ContainerPort: 2019, HostPort: 2019, HostIP: "127.0.0.1", Protocol: "tcp"},
		},
		Mounts: []podman.Mount{
			{Type: "volume", Source: CaddyDataVolume, Destination: "/data"},
			{Type: "volume", Source: CaddyConfigVolume, Destination: "/config"},
		},
	}
	if _, err := pc.ContainerCreate(ctx, req); err != nil {
		return fmt.Errorf("caddy create: %w", err)
	}
	if err := pc.ContainerStart(ctx, CaddyContainer); err != nil {
		return fmt.Errorf("caddy start: %w", err)
	}
	return nil
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
