package sonarr

import (
	"context"
	"fmt"
	"strings"

	"github.com/KevinHaeusler/go-haruki/bot/httpx"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *httpx.Client
}

func New(baseURL, apiKey string, http *httpx.Client) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    http,
	}
}

func (c *Client) headers() map[string]string {
	return map[string]string{
		"X-Api-Key": c.APIKey,
	}
}

type SystemStatus struct {
	Version string `json:"version"`
}

func (c *Client) GetSystemStatus(ctx context.Context) (*SystemStatus, error) {
	var out SystemStatus
	url := fmt.Sprintf("%s/api/v3/system/status", c.BaseURL)
	if err := c.HTTP.DoJSON(ctx, "GET", url, c.headers(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) apiURL(path string) string {
	p := strings.TrimLeft(path, "/")
	return fmt.Sprintf("%s/api/v3/%s", c.BaseURL, p)
}

func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.HTTP.DoJSON(ctx, "GET", c.apiURL(path), c.headers(), nil, out)
}

func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	return c.HTTP.DoJSON(ctx, "POST", c.apiURL(path), c.headers(), body, out)
}
