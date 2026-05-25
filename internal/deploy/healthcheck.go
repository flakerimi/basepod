package deploy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/flakerimi/basepod/internal/podman"
)

// WaitHealthy polls the container until it is healthy or the deadline expires.
//
//   - If httpPath is set, an HTTP request is made against the container at port[0]
//     via the basepod network IP.
//   - If the container has a HEALTHCHECK declared, container state.Health is used.
//   - Otherwise we just wait until the container is in "running" state.
func WaitHealthy(ctx context.Context, pc *podman.Client, name string, httpPath string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		if time.Now().After(deadline) {
			return errors.New("healthcheck timeout")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		ci, err := pc.ContainerInspect(ctx, name)
		if err != nil {
			continue
		}
		if !ci.State.Running {
			continue
		}
		// Prefer Health status if container has HEALTHCHECK.
		switch ci.State.Health.Status {
		case "healthy":
			return nil
		case "unhealthy":
			return errors.New("container reported unhealthy")
		}
		if httpPath != "" && port > 0 {
			if ok := tryHTTP(ctx, ci.NetworkSettings.IPAddress, port, httpPath); ok {
				return nil
			}
			continue
		}
		// No HEALTHCHECK and no httpPath: a 2s grace after running is enough.
		if ci.State.Status == "running" {
			return nil
		}
	}
}

func tryHTTP(ctx context.Context, ip string, port int, path string) bool {
	if ip == "" {
		return false
	}
	url := fmt.Sprintf("http://%s:%d%s", ip, port, ensureLeadingSlash(path))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

func ensureLeadingSlash(p string) string {
	if p == "" || p[0] == '/' {
		return p
	}
	return "/" + p
}
