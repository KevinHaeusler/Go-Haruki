package jellyseerr

import (
	"context"
	"fmt"
)

type userListResp struct {
	Results []struct {
		ID int `json:"id"`
	} `json:"results"`
}

type userDetailResp struct {
	ID       int `json:"id"`
	Settings struct {
		DiscordID string `json:"discordId"`
	} `json:"settings"`
}

func (c *Client) DiscordUserToOverseerrUserID(ctx context.Context, discordUserID string) (int, error) {
	const take = 100

	for skip := 0; skip < 2000; skip += take { // safety cap
		listURL := fmt.Sprintf("%s/api/v1/user?take=%d&skip=%d", c.BaseURL, take, skip)

		var list userListResp
		if err := c.HTTP.DoJSON(ctx, "GET", listURL, c.headers(), nil, &list); err != nil {
			return 0, err
		}
		if len(list.Results) == 0 {
			return 0, nil
		}

		for _, u := range list.Results {
			detailURL := fmt.Sprintf("%s/api/v1/user/%d", c.BaseURL, u.ID)

			var detail userDetailResp
			if err := c.HTTP.DoJSON(ctx, "GET", detailURL, c.headers(), nil, &detail); err != nil {
				continue
			}

			if detail.Settings.DiscordID == discordUserID {
				return detail.ID, nil
			}
		}

		if len(list.Results) < take {
			return 0, nil
		}
	}

	return 0, nil
}
