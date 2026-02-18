package jellyseerr

import (
	"context"
)

func (c *Client) DiscordUserToJellyseerrUserID(ctx context.Context, discordUserID string) (int, error) {
	const take = 100

	for skip := 0; skip < 2000; skip += take { // safety cap
		results, total, err := c.ListUsers(ctx, take, skip)
		if err != nil {
			return 0, err
		}
		if len(results) == 0 {
			return 0, nil
		}

		for _, u := range results {
			detail, err := c.GetUserDetail(ctx, u.ID)
			if err != nil {
				continue
			}

			if detail.Settings.DiscordID == discordUserID {
				return detail.ID, nil
			}
		}

		if len(results) < take || skip+take >= total {
			return 0, nil
		}
	}

	return 0, nil
}
