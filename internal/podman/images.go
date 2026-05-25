package podman

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) ImagePull(ctx context.Context, reference string) error {
	q := url.Values{}
	q.Set("reference", reference)
	resp, err := c.do(ctx, "POST", "/images/pull?"+q.Encode(), nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return decodeAPIError(resp)
	}
	// Drain progress stream.
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

type ImageSummary struct {
	ID          string   `json:"Id"`
	RepoTags    []string `json:"RepoTags"`
	Created     int64    `json:"Created"`
	Size        int64    `json:"Size"`
}

func (c *Client) ImageList(ctx context.Context) ([]ImageSummary, error) {
	var out []ImageSummary
	if err := c.getJSON(ctx, "/images/json", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ImageRemove(ctx context.Context, name string, force bool) error {
	q := url.Values{}
	if force {
		q.Set("force", "true")
	}
	return c.delete(ctx, "/images/"+url.PathEscape(name)+"?"+q.Encode())
}

// ImageBuild streams a build context tar and returns the raw build output
// stream. Caller drains it; tag is applied via `t=` query.
func (c *Client) ImageBuild(ctx context.Context, tarStream io.Reader, tag, dockerfile string) (io.ReadCloser, error) {
	q := url.Values{}
	q.Set("t", tag)
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	q.Set("dockerfile", dockerfile)
	q.Set("rm", "true")
	q.Set("forcerm", "true")
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/build?"+q.Encode(), tarStream)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, decodeAPIError(resp)
	}
	return resp.Body, nil
}

// ImageTag adds a new tag to an existing image.
func (c *Client) ImageTag(ctx context.Context, name, repo, tag string) error {
	q := url.Values{}
	q.Set("repo", repo)
	q.Set("tag", tag)
	resp, err := c.do(ctx, "POST", fmt.Sprintf("/images/%s/tag?%s", url.PathEscape(name), q.Encode()), nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return decodeAPIError(resp)
	}
	return nil
}
