package jellyseerr

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
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
	return map[string]string{"X-Api-Key": c.APIKey}
}

/* ---------- Search summary ---------- */

type MediaSummary struct {
	ID        int
	Title     string
	Year      string
	MediaType string
}

type searchResp struct {
	Results []struct {
		ID          int    `json:"id"`
		MediaType   string `json:"mediaType"`
		Title       string `json:"title"`
		Name        string `json:"name"`
		ReleaseDate string `json:"releaseDate"`
		FirstAir    string `json:"firstAirDate"`
	} `json:"results"`
}

func (c *Client) SearchSummary(ctx context.Context, query, mediaType string) ([]MediaSummary, error) {
	escaped := url.QueryEscape(query)
	// Force spaces to %20 instead of +
	escaped = strings.ReplaceAll(escaped, "+", "%20")

	u := fmt.Sprintf("%s/api/v1/search?query=%s", c.BaseURL, escaped)

	var out searchResp
	if err := c.HTTP.DoJSON(ctx, "GET", u, c.headers(), nil, &out); err != nil {
		return nil, err
	}

	res := make([]MediaSummary, 0, len(out.Results))
	for _, r := range out.Results {
		mt := r.MediaType
		if mt == "" {
			mt = mediaType
		}
		if mediaType != "" && mt != mediaType {
			continue
		}

		title := r.Title
		if mt == "tv" && title == "" {
			title = r.Name
		}
		year := ""
		d := r.ReleaseDate
		if mt == "tv" && d == "" {
			d = r.FirstAir
		}
		if len(d) >= 4 {
			year = d[:4]
		}

		res = append(res, MediaSummary{
			ID:        r.ID,
			Title:     title,
			Year:      year,
			MediaType: mt,
		})
	}

	return res, nil
}

/* ---------- Detail (includes mediaInfo + requests) ---------- */

type MediaDetail struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Name        string    `json:"name"`
	Overview    string    `json:"overview"`
	ReleaseDate string    `json:"releaseDate"`
	FirstAir    string    `json:"firstAirDate"`
	PosterPath  string    `json:"posterPath"`
	MediaInfo   MediaInfo `json:"mediaInfo"`
}

type MediaInfo struct {
	Status   int `json:"status"`
	Requests []struct {
		RequestedBy struct {
			ID          int    `json:"id"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"requestedBy"`
	} `json:"requests"`
}

func (d MediaDetail) DisplayTitle(mediaType string) string {
	if mediaType == "tv" && d.Name != "" {
		return d.Name
	}
	if d.Title != "" {
		return d.Title
	}
	return d.Name
}

func (d MediaDetail) DisplayYear(mediaType string) string {
	date := d.ReleaseDate
	if mediaType == "tv" && date == "" {
		date = d.FirstAir
	}
	if len(date) >= 4 {
		return date[:4]
	}
	return ""
}

func (d MediaDetail) HasRequester(userID int) bool {
	for _, r := range d.MediaInfo.Requests {
		if r.RequestedBy.ID == userID {
			return true
		}
	}
	return false
}

func (c *Client) GetDetail(ctx context.Context, mediaType string, id int) (MediaDetail, error) {
	// Python did: GET f"{media_type}/{id}" with language param
	u := fmt.Sprintf("%s/api/v1/%s/%d?language=en", c.BaseURL, mediaType, id)

	var out MediaDetail
	if err := c.HTTP.DoJSON(ctx, "GET", u, c.headers(), nil, &out); err != nil {
		return out, err
	}
	return out, nil
}
func (d MediaDetail) RequesterAndWatchers() (requester string, watchers []string) {
	if len(d.MediaInfo.Requests) == 0 {
		return "", nil
	}

	// First requester
	requester = d.MediaInfo.Requests[0].RequestedBy.DisplayName
	if requester == "" {
		requester = fmt.Sprintf("User %d", d.MediaInfo.Requests[0].RequestedBy.ID)
	}

	// Remaining are watchers
	if len(d.MediaInfo.Requests) > 1 {
		watchers = make([]string, 0, len(d.MediaInfo.Requests)-1)
		for _, r := range d.MediaInfo.Requests[1:] {
			n := r.RequestedBy.DisplayName
			if n == "" {
				n = fmt.Sprintf("User %d", r.RequestedBy.ID)
			}
			watchers = append(watchers, n)
		}
	}

	return requester, watchers
}

func (d MediaDetail) RequesterSummary() (string, []string) {
	if len(d.MediaInfo.Requests) == 0 {
		return "", nil
	}

	requester := d.MediaInfo.Requests[0].RequestedBy.DisplayName
	if requester == "" {
		requester = fmt.Sprintf("User %d", d.MediaInfo.Requests[0].RequestedBy.ID)
	}

	watchers := []string{}
	for _, r := range d.MediaInfo.Requests[1:] {
		n := r.RequestedBy.DisplayName
		if n == "" {
			n = fmt.Sprintf("User %d", r.RequestedBy.ID)
		}
		watchers = append(watchers, n)
	}

	return requester, watchers
}

/* ---------- small helper ---------- */

func atoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
