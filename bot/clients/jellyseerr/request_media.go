package jellyseerr

import (
	"context"
	"fmt"
)

// RequestMediaResponse is the response shape we need (requestCount).
type RequestMediaResponse struct {
	RequestedBy struct {
		RequestCount int `json:"requestCount"`
	} `json:"requestedBy"`
}

type requestMediaPayload struct {
	MediaType string `json:"mediaType"`
	MediaID   int    `json:"mediaId"`
	UserID    int    `json:"userId"`

	// For TV requests, Jellyseerr expects seasons. If omitted, some versions throw:
	// "Cannot read properties of undefined (reading 'filter')"
	// Common accepted values: "all" or []int (season numbers).
	Seasons any `json:"seasons,omitempty"`
}

// RequestMedia sends a request to Jellyseerr/Overseerr.
// For TV, it defaults to requesting all seasons.
func (c *Client) RequestMedia(ctx context.Context, mediaType string, mediaID int, userID int) (RequestMediaResponse, error) {
	u := fmt.Sprintf("%s/api/v1/request", c.BaseURL)

	body := requestMediaPayload{
		MediaType: mediaType,
		MediaID:   mediaID,
		UserID:    userID,
	}

	// ✅ Fix for Jellyseerr TV requests
	if mediaType == "tv" {
		body.Seasons = "all"
	}

	var out RequestMediaResponse
	if err := c.HTTP.DoJSON(ctx, "POST", u, c.headers(), body, &out); err != nil {
		return out, err
	}
	return out, nil
}
