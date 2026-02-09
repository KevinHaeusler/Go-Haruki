package jellyseerr

import (
	"context"
	"fmt"
)

type UserSummary struct {
	ID          int    `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	UserType    int    `json:"userType"`
	Permissions int    `json:"permissions"`
}

type listUsersResp struct {
	Results []UserSummary `json:"results"`
	Total   int           `json:"totalResults"` // may be 0 if not provided; ok
	Page    int           `json:"page"`         // optional
	Pages   int           `json:"pages"`        // optional
}

func (c *Client) ListUsers(ctx context.Context, take, skip int) ([]UserSummary, int, error) {
	u := fmt.Sprintf("%s/api/v1/user?take=%d&skip=%d", c.BaseURL, take, skip)

	var out listUsersResp
	if err := c.HTTP.DoJSON(ctx, "GET", u, c.headers(), nil, &out); err != nil {
		return nil, 0, err
	}
	return out.Results, out.Total, nil
}

type UserDetail struct {
	ID          int    `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`

	Settings struct {
		DiscordID string `json:"discordId"`
	} `json:"settings"`
}

func (c *Client) GetUserDetail(ctx context.Context, id int) (UserDetail, error) {
	u := fmt.Sprintf("%s/api/v1/user/%d", c.BaseURL, id)

	var out UserDetail
	if err := c.HTTP.DoJSON(ctx, "GET", u, c.headers(), nil, &out); err != nil {
		return out, err
	}
	return out, nil
}

// UpdateUserDiscordID sets settings.discordId for a Jellyseerr user.
//
// This implementation uses: PUT /api/v1/user/{id}/settings with { "discordId": "..." }.
// If your instance expects PATCH or a different path, change it here.
func (c *Client) UpdateUserDiscordID(ctx context.Context, jellyUserID int, discordID string) error {
	u := fmt.Sprintf("%s/api/v1/user/%d/settings", c.BaseURL, jellyUserID)

	body := map[string]any{
		"discordId": discordID,
	}

	// Some servers return the updated settings; we don't need it.
	var ignore any
	return c.HTTP.DoJSON(ctx, "PUT", u, c.headers(), body, &ignore)
}
