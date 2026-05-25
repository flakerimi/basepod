package deploy

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/flakerimi/basepod/internal/apps"
	"github.com/flakerimi/basepod/internal/podman"
)

// HealthPoller periodically inspects every app's container and restarts ones
// that report unhealthy or have crashed. Best-effort, fire-and-forget.
type HealthPoller struct {
	apps     *apps.Service
	pc       *podman.Client
	log      *slog.Logger
	interval time.Duration

	mu       sync.Mutex
	failures map[string]int // name → consecutive failures
}

const (
	restartAfterFails = 3
	defaultInterval   = 30 * time.Second
)

func NewHealthPoller(s *apps.Service, pc *podman.Client, log *slog.Logger, interval time.Duration) *HealthPoller {
	if interval == 0 {
		interval = defaultInterval
	}
	return &HealthPoller{
		apps:     s,
		pc:       pc,
		log:      log,
		interval: interval,
		failures: map[string]int{},
	}
}

func (h *HealthPoller) Run(ctx context.Context) {
	if h.pc == nil {
		return
	}
	tick := time.NewTicker(h.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			h.tick(ctx)
		}
	}
}

func (h *HealthPoller) tick(ctx context.Context) {
	list, err := h.apps.List(ctx)
	if err != nil {
		return
	}
	for _, a := range list {
		if a.CurrentVersion == "" {
			continue
		}
		ci, err := h.pc.ContainerInspect(ctx, a.Name)
		if err != nil {
			continue
		}
		bad := !ci.State.Running ||
			(ci.State.Health.Status == "unhealthy")
		if !bad {
			h.reset(a.Name)
			continue
		}
		n := h.bump(a.Name)
		h.log.Warn("healthcheck failed", "app", a.Name, "fails", n, "status", ci.State.Status, "health", ci.State.Health.Status)
		if n >= restartAfterFails {
			h.log.Warn("restarting unhealthy app", "app", a.Name)
			_ = h.pc.ContainerStart(ctx, a.Name)
			h.reset(a.Name)
		}
	}
}

func (h *HealthPoller) bump(name string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failures[name]++
	return h.failures[name]
}

func (h *HealthPoller) reset(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.failures, name)
}
