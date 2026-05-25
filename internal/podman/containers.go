package podman

import (
	"context"
	"fmt"
	"io"
	"net/url"
)

type PortMapping struct {
	ContainerPort uint16 `json:"container_port"`
	HostPort      uint16 `json:"host_port,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	HostIP        string `json:"host_ip,omitempty"`
}

type Mount struct {
	Type        string   `json:"Type"`        // bind | volume
	Source      string   `json:"Source"`
	Destination string   `json:"Destination"`
	Options     []string `json:"Options,omitempty"`
}

type HealthConfig struct {
	Test     []string `json:"Test,omitempty"`
	Interval int64    `json:"Interval,omitempty"`
	Timeout  int64    `json:"Timeout,omitempty"`
	Retries  uint     `json:"Retries,omitempty"`
}

type ResourceLimits struct {
	Memory int64 `json:"memory,omitempty"` // bytes
	CPU    int64 `json:"nano_cpus,omitempty"`
}

type ContainerCreateRequest struct {
	Name        string            `json:"name,omitempty"`
	Image       string            `json:"image"`
	Command     []string          `json:"command,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Networks    map[string]struct{} `json:"-"`
	NetNS       Namespace         `json:"netns,omitempty"`
	CNINetworks []string          `json:"cni_networks,omitempty"`
	PortMaps    []PortMapping     `json:"portmappings,omitempty"`
	Mounts      []Mount           `json:"mounts,omitempty"`
	Healthcheck *HealthConfig     `json:"healthconfig,omitempty"`
	Resources   *ResourceLimits   `json:"resource_limits,omitempty"`
	Remove      bool              `json:"remove,omitempty"`
	Hostname    string            `json:"hostname,omitempty"`
	Restart     string            `json:"restart_policy,omitempty"`
}

type Namespace struct {
	NSMode string `json:"nsmode,omitempty"`
	Value  string `json:"value,omitempty"`
}

type ContainerCreateResponse struct {
	ID       string   `json:"Id"`
	Warnings []string `json:"Warnings,omitempty"`
}

func (c *Client) ContainerCreate(ctx context.Context, req ContainerCreateRequest) (string, error) {
	var resp ContainerCreateResponse
	if err := c.postJSON(ctx, "/containers/create", req, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *Client) ContainerStart(ctx context.Context, name string) error {
	return c.postJSON(ctx, "/containers/"+url.PathEscape(name)+"/start", nil, nil)
}

func (c *Client) ContainerStop(ctx context.Context, name string, timeoutSec int) error {
	path := fmt.Sprintf("/containers/%s/stop?timeout=%d", url.PathEscape(name), timeoutSec)
	return c.postJSON(ctx, path, nil, nil)
}

func (c *Client) ContainerRemove(ctx context.Context, name string, force bool) error {
	q := url.Values{}
	if force {
		q.Set("force", "true")
	}
	return c.delete(ctx, "/containers/"+url.PathEscape(name)+"?"+q.Encode())
}

func (c *Client) ContainerRename(ctx context.Context, oldName, newName string) error {
	q := url.Values{}
	q.Set("name", newName)
	return c.postJSON(ctx, "/containers/"+url.PathEscape(oldName)+"/rename?"+q.Encode(), nil, nil)
}

type ContainerInspect struct {
	ID    string `json:"Id"`
	Name  string `json:"Name"`
	State struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		Restarting bool   `json:"Restarting"`
		ExitCode   int    `json:"ExitCode"`
		Health     struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	Config struct {
		Image string            `json:"Image"`
		Env   []string          `json:"Env"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	NetworkSettings struct {
		IPAddress string `json:"IPAddress"`
	} `json:"NetworkSettings"`
}

func (c *Client) ContainerInspect(ctx context.Context, name string) (*ContainerInspect, error) {
	var ci ContainerInspect
	if err := c.getJSON(ctx, "/containers/"+url.PathEscape(name)+"/json", &ci); err != nil {
		return nil, err
	}
	return &ci, nil
}

type ContainerSummary struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Labels map[string]string `json:"Labels"`
}

func (c *Client) ContainerList(ctx context.Context, all bool) ([]ContainerSummary, error) {
	q := url.Values{}
	if all {
		q.Set("all", "true")
	}
	var out []ContainerSummary
	if err := c.getJSON(ctx, "/containers/json?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ContainerExists(ctx context.Context, name string) (bool, error) {
	resp, err := c.do(ctx, "GET", "/containers/"+url.PathEscape(name)+"/exists", nil, "")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 204:
		return true, nil
	case 404:
		return false, nil
	}
	return false, decodeAPIError(resp)
}

// ContainerLogsStream returns the (stdout+stderr) stream for the container.
// follow=true keeps the connection open and streams new output.
func (c *Client) ContainerLogsStream(ctx context.Context, name string, follow bool) (io.ReadCloser, error) {
	q := url.Values{}
	q.Set("stdout", "true")
	q.Set("stderr", "true")
	q.Set("timestamps", "false")
	if follow {
		q.Set("follow", "true")
	}
	resp, err := c.do(ctx, "GET", "/containers/"+url.PathEscape(name)+"/logs?"+q.Encode(), nil, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, decodeAPIError(resp)
	}
	return resp.Body, nil
}
