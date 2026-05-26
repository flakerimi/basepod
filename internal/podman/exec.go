package podman

import (
	"context"
	"fmt"
	"os/exec"
)

// runPodman invokes the local `podman` CLI. Used for the few operations the
// libpod REST surface doesn't cover (machine mgmt, volume export/import).
func runPodman(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "podman", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman %v: %w: %s", args, err, out)
	}
	return nil
}

// Exec runs a command inside a running container and returns combined output.
// This is the control channel for Caddy admin (which lives on a Unix socket
// only reachable from inside the Caddy container's mount namespace).
func Exec(ctx context.Context, container string, argv []string) ([]byte, error) {
	full := append([]string{"exec", container}, argv...)
	cmd := exec.CommandContext(ctx, "podman", full...)
	return cmd.CombinedOutput()
}
