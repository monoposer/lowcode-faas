package metaclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RuntimeAction is meta's runtime payload for the worker (metadata + compiled JS).
type RuntimeAction struct {
	Name    string `json:"name"`
	Group   string `json:"group"`
	Etag    string `json:"etag"`
	Timeout int    `json:"timeout"`
	JsURL   string `json:"js_url"`
	JS      string `json:"js"`
}

// Client talks to the meta service.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetRuntime loads action metadata and compiled JS from meta.
// If ifNoneMatch is non-empty and matches the current etag, meta returns 304 and (nil, true, nil).
// GET {meta}/api/actions/{name}/runtime?group=
func (c *Client) GetRuntime(ctx context.Context, name, group, ifNoneMatch string) (rt *RuntimeAction, notModified bool, err error) {
	if c == nil || c.BaseURL == "" {
		return nil, false, fmt.Errorf("meta url not configured")
	}
	u := c.BaseURL + "/api/actions/" + url.PathEscape(name) + "/runtime"
	if group != "" {
		u += "?group=" + url.QueryEscape(group)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotModified {
		return nil, true, nil
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, false, err
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, false, fmt.Errorf("action not found")
	}
	if res.StatusCode >= 300 {
		return nil, false, fmt.Errorf("meta status %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out RuntimeAction
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false, fmt.Errorf("meta response: %w", err)
	}
	if out.JS == "" {
		return nil, false, fmt.Errorf("meta returned empty js")
	}
	return &out, false, nil
}
