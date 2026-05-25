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
