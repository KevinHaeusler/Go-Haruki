package jellyseerr

import (
	"context"
	"fmt"
)

// requestsListResp models the minimal fields we need from GET /api/v1/request
// It returns a paged list with totalResults; we only care about totalResults
// to display a user's total number of requests.
type requestsListResp struct {
	Results      []any `json:"results"`
	TotalResults int   `json:"totalResults"`
}

// GetUserRequestTotal returns the total number of requests created by the given
// Jellyseerr user ID. It queries the requests endpoint with take=1 to avoid
// transferring the full list and reads totalResults.
//
// API shape (Overseerr/Jellyseerr):
//
//	GET /api/v1/request?take=1&skip=0&requestedBy={userID}
func (c *Client) GetUserRequestTotal(ctx context.Context, userID int) (int, error) {
	u := fmt.Sprintf("%s/api/v1/request?take=1&skip=0&requestedBy=%d", c.BaseURL, userID)
	var out requestsListResp
	if err := c.HTTP.DoJSON(ctx, "GET", u, c.headers(), nil, &out); err != nil {
		return 0, err
	}
	return out.TotalResults, nil
}
