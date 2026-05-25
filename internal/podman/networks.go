package podman

import (
	"context"
	"net/url"
)

type NetworkCreateRequest struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver,omitempty"`
	DNSEnabled bool              `json:"dns_enabled,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Internal   bool              `json:"internal,omitempty"`
}

type NetworkInfo struct {
	Name       string            `json:"name"`
	ID         string            `json:"id"`
	Driver     string            `json:"driver"`
	DNSEnabled bool              `json:"dns_enabled"`
	Labels     map[string]string `json:"labels,omitempty"`
}

func (c *Client) NetworkCreate(ctx context.Context, req NetworkCreateRequest) (string, error) {
	var out NetworkInfo
	if err := c.postJSON(ctx, "/networks/create", req, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *Client) NetworkExists(ctx context.Context, name string) (bool, error) {
	resp, err := c.do(ctx, "GET", "/networks/"+url.PathEscape(name)+"/exists", nil, "")
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

type VolumeCreateRequest struct {
	Name    string            `json:"Name,omitempty"`
	Driver  string            `json:"Driver,omitempty"`
	Labels  map[string]string `json:"Labels,omitempty"`
	Options map[string]string `json:"Options,omitempty"`
}

func (c *Client) VolumeCreate(ctx context.Context, req VolumeCreateRequest) error {
	return c.postJSON(ctx, "/volumes/create", req, nil)
}

func (c *Client) VolumeExists(ctx context.Context, name string) (bool, error) {
	resp, err := c.do(ctx, "GET", "/volumes/"+url.PathEscape(name)+"/exists", nil, "")
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
