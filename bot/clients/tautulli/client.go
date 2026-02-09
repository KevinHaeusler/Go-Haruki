package tautulli

import (
	"context"
	"fmt"
	"net/url"
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
	return map[string]string{}
}

type GetActivityResponse struct {
	Response struct {
		Result  string       `json:"result"`
		Message interface{}  `json:"message"`
		Data    ActivityData `json:"data"`
	} `json:"response"`
}

type ActivityData struct {
	StreamCount string    `json:"stream_count"`
	Sessions    []Session `json:"sessions"`

	StreamCountDirectPlay   int `json:"stream_count_direct_play"`
	StreamCountDirectStream int `json:"stream_count_direct_stream"`
	StreamCountTranscode    int `json:"stream_count_transcode"`
	TotalBandwidth          int `json:"total_bandwidth"`
	LanBandwidth            int `json:"lan_bandwidth"`
	WanBandwidth            int `json:"wan_bandwidth"`
}

type Session struct {
	MediaType    string `json:"media_type"`
	State        string `json:"state"`
	User         string `json:"user"`
	FriendlyName string `json:"friendly_name"`
	Player       string `json:"player"`
	Product      string `json:"product"`
	Platform     string `json:"platform"`
	LibraryName  string `json:"library_name"`

	Title            string `json:"title"`
	ParentTitle      string `json:"parent_title"`
	GrandparentTitle string `json:"grandparent_title"`
	FullTitle        string `json:"full_title"`

	GrandparentRatingKey string `json:"grandparent_rating_key"`
	ParentRatingKey      string `json:"parent_rating_key"`
	RatingKey            string `json:"rating_key"`

	ProgressPercent string `json:"progress_percent"`
	ViewOffset      string `json:"view_offset"`
	Duration        string `json:"duration"`

	QualityProfile    string `json:"quality_profile"`
	TranscodeDecision string `json:"transcode_decision"`

	StreamBitrate      string `json:"stream_bitrate"`
	StreamVideoFullRes string `json:"stream_video_full_resolution"`
	VideoFullRes       string `json:"video_full_resolution"`

	Thumb            string `json:"thumb"`
	ParentThumb      string `json:"parent_thumb"`
	GrandparentThumb string `json:"grandparent_thumb"`
	Art              string `json:"art"`
}

func (c *Client) GetActivity(ctx context.Context) (*GetActivityResponse, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(u.Path, "/api/") {
		u.Path = strings.TrimRight(u.Path, "/") + "/api/v2"
	}
	q := u.Query()
	q.Set("apikey", c.APIKey)
	q.Set("cmd", "get_activity")
	u.RawQuery = q.Encode()

	var out GetActivityResponse
	if err := c.HTTP.DoJSON(ctx, "GET", u.String(), c.headers(), nil, &out); err != nil {
		return nil, err
	}

	if out.Response.Result != "success" {
		return nil, fmt.Errorf("tautulli get_activity failed: result=%s msg=%v", out.Response.Result, out.Response.Message)
	}

	return &out, nil
}

func (c *Client) ImageProxyURL(imgPath string, width int) string {
	imgPath = strings.TrimSpace(imgPath)
	if imgPath == "" {
		return ""
	}

	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return ""
	}

	if !strings.Contains(u.Path, "/api/") {
		u.Path = strings.TrimRight(u.Path, "/") + "/api/v2"
	}

	q := u.Query()
	q.Set("apikey", c.APIKey)
	q.Set("cmd", "pms_image_proxy")
	q.Set("img", imgPath)
	if width > 0 {
		q.Set("width", fmt.Sprintf("%d", width))
	}
	u.RawQuery = q.Encode()

	return u.String()
}
