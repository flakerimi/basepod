package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

const apiVersion = "v5.0.0"

type Client struct {
	httpc   *http.Client
	baseURL string
}

// New returns a client. If socketURI is empty, it is auto-detected from
// `podman system connection list`.
func New(socketURI string) (*Client, error) {
	if socketURI == "" {
		u, err := DetectSocket()
		if err != nil {
			return nil, err
		}
		socketURI = u
	}
	u, err := url.Parse(socketURI)
	if err != nil {
		return nil, fmt.Errorf("parse socket uri: %w", err)
	}
	var transport *http.Transport
	if u.Scheme == "unix" {
		path := u.Path
		transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, "unix", path)
			},
		}
	} else {
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	return &Client{
		httpc:   &http.Client{Transport: transport, Timeout: 0},
		baseURL: "http://d/libpod",
	}, nil
}

// DetectSocket asks the local `podman` CLI for the default connection URI.
func DetectSocket() (string, error) {
	cmd := exec.Command("podman", "system", "connection", "list", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("podman system connection list: %w", err)
	}
	var conns []struct {
		Name    string `json:"Name"`
		URI     string `json:"URI"`
		Default bool   `json:"Default"`
	}
	if err := json.Unmarshal(out, &conns); err != nil {
		return "", fmt.Errorf("parse connections: %w", err)
	}
	for _, c := range conns {
		if c.Default {
			return convertURI(c.URI), nil
		}
	}
	if len(conns) > 0 {
		return convertURI(conns[0].URI), nil
	}
	return "", errors.New("no podman connections found; is `podman machine` running?")
}

// convertURI converts ssh:// URIs (used by `podman machine` on Mac) to the
// underlying unix socket. For ssh URIs we instead use `podman system service`
// hint of the local forwarded socket path.
func convertURI(uri string) string {
	if strings.HasPrefix(uri, "unix://") {
		return uri
	}
	// For ssh:// connections from podman machine, the local unix socket is
	// also exposed. Try to discover it via `podman info`.
	out, err := exec.Command("podman", "info", "--format", "{{.Host.RemoteSocket.Path}}").Output()
	if err == nil {
		path := strings.TrimSpace(string(out))
		if path != "" {
			return "unix://" + path
		}
	}
	return uri
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.httpc.Do(req)
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return decodeAPIError(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	resp, err := c.do(ctx, http.MethodPost, path, body, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return decodeAPIError(resp)
	}
	if out == nil {
		return nil
	}
	if resp.ContentLength == 0 {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) delete(ctx context.Context, path string) error {
	resp, err := c.do(ctx, http.MethodDelete, path, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		return decodeAPIError(resp)
	}
	return nil
}

type APIError struct {
	Status  int
	Message string `json:"message"`
	Cause   string `json:"cause,omitempty"`
}

func (e *APIError) Error() string {
	if e.Cause != "" {
		return fmt.Sprintf("podman %d: %s (%s)", e.Status, e.Message, e.Cause)
	}
	return fmt.Sprintf("podman %d: %s", e.Status, e.Message)
}

func decodeAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	e := &APIError{Status: resp.StatusCode}
	if json.Unmarshal(body, e) != nil {
		e.Message = strings.TrimSpace(string(body))
	}
	if e.Message == "" {
		e.Message = http.StatusText(resp.StatusCode)
	}
	return e
}

// Ping returns nil if the podman API responds.
func (c *Client) Ping(ctx context.Context) error {
	return c.getJSON(ctx, "/_ping", nil)
}

// Version returns the libpod version string.
func (c *Client) Version(ctx context.Context) (string, error) {
	var v struct {
		Components []struct {
			Name    string `json:"Name"`
			Version string `json:"Version"`
		} `json:"Components"`
		Version string `json:"Version"`
	}
	if err := c.getJSON(ctx, "/version", &v); err != nil {
		return "", err
	}
	return v.Version, nil
}

// SocketURI is exposed for diagnostics.
func (c *Client) SocketURI() string { return c.baseURL }

var _ = apiVersion
