package bootstrap

import (
	"testing"

	"github.com/flakerimi/basepod/internal/caddy"
	"github.com/flakerimi/basepod/internal/podman"
)

func TestCaddyContainerNeedsRecreateForLegacyTCPAdmin(t *testing.T) {
	var ci podman.ContainerInspect
	ci.Config.Cmd = []string{"caddy", "run", "--config", "/etc/caddy/caddy.json", "--resume"}
	ci.HostConfig.PortBindings = map[string][]podman.PortBinding{
		"2019/tcp": {{HostIP: "127.0.0.1", HostPort: "2019"}},
	}
	ci.Mounts = []podman.ContainerMount{
		{Type: "volume", Destination: "/config"},
	}

	if !caddyContainerNeedsRecreate(&ci, "/Users/alice/BasePodData/_basepod/caddy") {
		t.Fatal("legacy caddy container should be recreated")
	}
}

func TestCaddyContainerNeedsRecreateKeepsCurrentContainer(t *testing.T) {
	var ci podman.ContainerInspect
	ci.Config.Cmd = []string{"caddy", "run", "--config", caddy.ConfigFileInContainer}
	ci.Mounts = []podman.ContainerMount{
		{Type: "bind", Source: "/Users/alice/BasePodData/_basepod/caddy", Destination: "/etc/caddy"},
	}

	if caddyContainerNeedsRecreate(&ci, "/Users/alice/BasePodData/_basepod/caddy") {
		t.Fatal("current caddy container should not be recreated")
	}
}

func TestCaddyContainerNeedsRecreateForWrongConfigMountSource(t *testing.T) {
	var ci podman.ContainerInspect
	ci.Config.Cmd = []string{"caddy", "run", "--config", caddy.ConfigFileInContainer}
	ci.Mounts = []podman.ContainerMount{
		{Type: "bind", Source: "/BasePodData/_basepod/caddy", Destination: "/etc/caddy"},
	}

	if !caddyContainerNeedsRecreate(&ci, "/Users/alice/BasePodData/_basepod/caddy") {
		t.Fatal("caddy container with stale config bind source should be recreated")
	}
}

func TestVMPathForDataDirUsesDefaultPodmanMachineMounts(t *testing.T) {
	dataDir := "/Users/alice/BasePodData"
	hostConfigDir := "/Users/alice/BasePodData/_basepod/caddy"

	got, err := vmPathForDataDir(dataDir, hostConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/Users/alice/BasePodData/_basepod/caddy" {
		t.Fatalf("vm path = %q", got)
	}
}

func TestVMPathForDataDirMapsCustomDirToBasePodMount(t *testing.T) {
	dataDir := "/Volumes/fast/BasePodData"
	hostConfigDir := "/Volumes/fast/BasePodData/_basepod/caddy"

	got, err := vmPathForDataDir(dataDir, hostConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/BasePodData/_basepod/caddy" {
		t.Fatalf("vm path = %q", got)
	}
}
