package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/flakerimi/basepod/internal/apps"
	"github.com/flakerimi/basepod/internal/bootstrap"
	"github.com/flakerimi/basepod/internal/caddy"
	"github.com/flakerimi/basepod/internal/config"
	"github.com/flakerimi/basepod/internal/podman"
)

type Orchestrator struct {
	apps     *apps.Service
	pc       *podman.Client
	cc       *caddy.Client
	caddyRender CaddyRenderer
	log      *slog.Logger
	cfg      config.Config
}

// CaddyRenderer returns a Caddy JSON config given the current app set.
type CaddyRenderer func(ctx context.Context) ([]byte, error)

func NewOrchestrator(svc *apps.Service, pc *podman.Client, cc *caddy.Client, render CaddyRenderer, cfg config.Config, log *slog.Logger) *Orchestrator {
	return &Orchestrator{apps: svc, pc: pc, cc: cc, caddyRender: render, cfg: cfg, log: log}
}

type Request struct {
	App       *apps.App
	ImageTag  string
	Version   string
	Strategy  string
	Spec      config.AppSpec // optional: used to set healthcheck, ports, env overrides on first deploy
}

func (o *Orchestrator) Deploy(ctx context.Context, req Request) error {
	if req.Strategy == "" {
		req.Strategy = req.App.DeployStrategy
	}
	if req.Strategy == "" {
		req.Strategy = config.StrategyBlueGreen
	}
	o.log.Info("deploy starting", "app", req.App.Name, "version", req.Version, "strategy", req.Strategy)

	switch req.Strategy {
	case config.StrategyBlueGreen:
		return o.deployBlueGreen(ctx, req)
	case config.StrategyStopStart:
		return o.deployStopStart(ctx, req)
	default:
		return fmt.Errorf("unknown strategy %q", req.Strategy)
	}
}

func (o *Orchestrator) deployBlueGreen(ctx context.Context, req Request) error {
	newName := req.App.Name + "-new"
	if exists, _ := o.pc.ContainerExists(ctx, newName); exists {
		_ = o.pc.ContainerStop(ctx, newName, 5)
		_ = o.pc.ContainerRemove(ctx, newName, true)
	}
	env, err := o.apps.GetEnv(ctx, req.App.ID)
	if err != nil {
		return fmt.Errorf("get env: %w", err)
	}
	createReq := o.containerCreateRequest(req, newName, env)
	if _, err := o.pc.ContainerCreate(ctx, createReq); err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	if err := o.pc.ContainerStart(ctx, newName); err != nil {
		_ = o.pc.ContainerRemove(ctx, newName, true)
		return fmt.Errorf("start container: %w", err)
	}
	primaryPort := 0
	if len(req.App.Ports) > 0 {
		primaryPort = req.App.Ports[0]
	}
	if err := WaitHealthy(ctx, o.pc, newName, req.App.HealthcheckPath, primaryPort, 60*time.Second); err != nil {
		_ = o.pc.ContainerStop(ctx, newName, 5)
		_ = o.pc.ContainerRemove(ctx, newName, true)
		return fmt.Errorf("healthcheck: %w", err)
	}

	// rename old → -old, new → primary
	oldExists, _ := o.pc.ContainerExists(ctx, req.App.Name)
	if oldExists {
		if err := o.pc.ContainerRename(ctx, req.App.Name, req.App.Name+"-old"); err != nil {
			o.log.Warn("rename old failed", "err", err)
		}
	}
	if err := o.pc.ContainerRename(ctx, newName, req.App.Name); err != nil {
		// fatal — try to recover
		if oldExists {
			_ = o.pc.ContainerRename(ctx, req.App.Name+"-old", req.App.Name)
		}
		return fmt.Errorf("rename new: %w", err)
	}

	if err := o.reloadCaddy(ctx); err != nil {
		o.log.Error("caddy reload failed; restoring old container", "err", err)
		_ = o.pc.ContainerStop(ctx, req.App.Name, 5)
		_ = o.pc.ContainerRename(ctx, req.App.Name, newName)
		if oldExists {
			_ = o.pc.ContainerRename(ctx, req.App.Name+"-old", req.App.Name)
		}
		_ = o.pc.ContainerRemove(ctx, newName, true)
		return fmt.Errorf("caddy reload: %w", err)
	}

	if oldExists {
		_ = o.pc.ContainerStop(ctx, req.App.Name+"-old", 10)
		_ = o.pc.ContainerRemove(ctx, req.App.Name+"-old", true)
	}

	return o.finalize(ctx, req)
}

func (o *Orchestrator) deployStopStart(ctx context.Context, req Request) error {
	env, err := o.apps.GetEnv(ctx, req.App.ID)
	if err != nil {
		return err
	}
	if exists, _ := o.pc.ContainerExists(ctx, req.App.Name); exists {
		_ = o.pc.ContainerStop(ctx, req.App.Name, 10)
		_ = o.pc.ContainerRemove(ctx, req.App.Name, true)
	}
	createReq := o.containerCreateRequest(req, req.App.Name, env)
	if _, err := o.pc.ContainerCreate(ctx, createReq); err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	if err := o.pc.ContainerStart(ctx, req.App.Name); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	primaryPort := 0
	if len(req.App.Ports) > 0 {
		primaryPort = req.App.Ports[0]
	}
	if err := WaitHealthy(ctx, o.pc, req.App.Name, req.App.HealthcheckPath, primaryPort, 60*time.Second); err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	if err := o.reloadCaddy(ctx); err != nil {
		return fmt.Errorf("caddy reload: %w", err)
	}
	return o.finalize(ctx, req)
}

func (o *Orchestrator) finalize(ctx context.Context, req Request) error {
	repo := strings.SplitN(req.ImageTag, ":", 2)[0]
	if err := o.apps.SetVersion(ctx, req.App.ID, req.Version, repo); err != nil {
		return err
	}
	_ = o.apps.PruneOldVersions(ctx, req.App.ID, 5)
	o.log.Info("deploy succeeded", "app", req.App.Name, "version", req.Version)
	return nil
}

func (o *Orchestrator) containerCreateRequest(req Request, name string, env map[string]string) podman.ContainerCreateRequest {
	mounts := []podman.Mount{}
	for _, v := range req.App.Volumes {
		if v.Host != "" {
			mounts = append(mounts, podman.Mount{
				Type:        "bind",
				Source:      v.Host,
				Destination: v.Container,
				Options:     []string{"rw"},
			})
		} else if v.NamedVolume != "" {
			mounts = append(mounts, podman.Mount{
				Type:        "volume",
				Source:      v.NamedVolume,
				Destination: v.Container,
			})
		}
	}
	cr := podman.ContainerCreateRequest{
		Name:        name,
		Image:       req.ImageTag,
		Hostname:    req.App.Name,
		Env:         env,
		Labels:      map[string]string{"basepod": "true", "basepod.app": req.App.Name, "basepod.version": req.Version},
		CNINetworks: []string{bootstrap.NetworkName},
		Mounts:      mounts,
		Restart:     "always",
	}
	return cr
}

func (o *Orchestrator) reloadCaddy(ctx context.Context) error {
	if o.cc == nil || o.caddyRender == nil {
		return nil
	}
	cfg, err := o.caddyRender(ctx)
	if err != nil {
		return fmt.Errorf("render caddy: %w", err)
	}
	return o.cc.ApplyAtomic(ctx, cfg)
}

// EnsureWorkdir ensures a per-app workdir exists for deploy uploads.
func (o *Orchestrator) EnsureWorkdir(app string) (string, error) {
	dir := filepath.Join(o.cfg.WorkDir(), app)
	return dir, nil
}
