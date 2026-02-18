package jellyseerr

import (
	"context"
	"fmt"
	"time"
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

type NotificationSettings struct {
	NotificationTypes struct {
		Discord    int `json:"discord"`
		Email      int `json:"email"`
		Pushbullet int `json:"pushbullet"`
		Pushover   int `json:"pushover"`
		Slack      int `json:"slack"`
		Telegram   int `json:"telegram"`
		Webhook    int `json:"webhook"`
		Webpush    int `json:"webpush"`
	} `json:"notificationTypes"`
	EmailEnabled             bool   `json:"emailEnabled"`
	PGPKey                   string `json:"pgpKey,omitempty"`
	DiscordEnabled           bool   `json:"discordEnabled"`
	DiscordEnabledTypes      int    `json:"discordEnabledTypes"`
	DiscordID                string `json:"discordId,omitempty"`
	PushbulletAccessToken    string `json:"pushbulletAccessToken,omitempty"`
	PushoverApplicationToken string `json:"pushoverApplicationToken,omitempty"`
	PushoverUserKey          string `json:"pushoverUserKey,omitempty"`
	PushoverSound            string `json:"pushoverSound,omitempty"`
	TelegramEnabled          bool   `json:"telegramEnabled"`
	TelegramBotUsername      string `json:"telegramBotUsername,omitempty"`
	TelegramChatId           string `json:"telegramChatId,omitempty"`
	TelegramSendSilently     bool   `json:"telegramSendSilently"`
}

func (c *Client) GetUserDetail(ctx context.Context, id int) (UserDetail, error) {
	u := fmt.Sprintf("%s/api/v1/user/%d", c.BaseURL, id)

	var out UserDetail
	if err := c.HTTP.DoJSON(ctx, "GET", u, c.headers(), nil, &out); err != nil {
		return out, err
	}
	return out, nil
}

// UserRequest models the request data returned by /api/v1/user/{id}/requests
type UserRequest struct {
	ID        int       `json:"id"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Is4k      bool      `json:"is4k"`
	Title     string    `json:"title"`
	Type      string    `json:"type"` // "movie" or "tv"
	Media     struct {
		ID          int      `json:"id"`
		TMDBID      int      `json:"tmdbId"`
		TVDBID      int      `json:"tvdbId"`
		Status      int      `json:"status"`
		Requests    []string `json:"requests"`
		Title       string   `json:"title"`
		Name        string   `json:"name"`
		MediaType   string   `json:"mediaType"`
		ReleaseDate string   `json:"releaseDate"`
		CreatedAt   string   `json:"createdAt"`
		UpdatedAt   string   `json:"updatedAt"`
	} `json:"media"`
}

type UserRequestsResponse struct {
	PageInfo struct {
		Page    int `json:"page"`
		Pages   int `json:"pages"`
		Results int `json:"results"`
	} `json:"pageInfo"`
	Results []UserRequest `json:"results"`
}

func (c *Client) GetUserRequests(ctx context.Context, userID int, includeFinished bool) ([]UserRequest, error) {
	var allResults []UserRequest
	take := 100
	skip := 0

	for {
		u := fmt.Sprintf("%s/api/v1/user/%d/requests?take=%d&skip=%d", c.BaseURL, userID, take, skip)

		var out UserRequestsResponse
		if err := c.HTTP.DoJSON(ctx, "GET", u, c.headers(), nil, &out); err != nil {
			return nil, err
		}

		if len(out.Results) == 0 {
			break
		}

		for i, r := range out.Results {
			if !includeFinished && r.Media.Status == 5 {
				continue
			}

			// If title is missing, try to resolve it.
			// Sometimes the list response is missing the title.
			if r.Title == "" && r.Media.Title == "" && r.Media.Name == "" {
				mediaType := r.Type
				if mediaType == "" {
					mediaType = r.Media.MediaType
				}
				mediaID := r.Media.TMDBID
				if mediaType == "tv" && r.Media.TVDBID != 0 {
					// Jellyseerr often uses TMDB for TV too, but let's be safe.
					// Actually GetDetail uses TMDB ID for movie and TV (usually).
				}

				if mediaType != "" && mediaID != 0 {
					detail, err := c.GetDetail(ctx, mediaType, mediaID)
					if err == nil {
						// Apply detail to the actual slice element
						out.Results[i].Media.Title = detail.Title
						out.Results[i].Media.Name = detail.Name
						if out.Results[i].Media.ReleaseDate == "" {
							out.Results[i].Media.ReleaseDate = detail.DisplayYear(mediaType)
						}
						// Use the newly fetched data for the current 'r' as well
						r.Media.Title = detail.Title
						r.Media.Name = detail.Name
						r.Media.ReleaseDate = out.Results[i].Media.ReleaseDate
					}
				}
			}

			allResults = append(allResults, out.Results[i])
		}

		if len(out.Results) < take {
			break
		}
		skip += take

		// Safety cap to prevent infinite loops or excessive memory usage
		if skip >= 2000 {
			break
		}
	}

	return allResults, nil
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

func (c *Client) UpdateUserNotificationSettings(ctx context.Context, jellyUserID int, settings NotificationSettings) error {
	u := fmt.Sprintf("%s/api/v1/user/%d/settings/notifications", c.BaseURL, jellyUserID)

	var ignore any
	return c.HTTP.DoJSON(ctx, "POST", u, c.headers(), settings, &ignore)
}

func GetUserName(c *Client, id int) (string, error) {
	detail, err := c.GetUserDetail(context.Background(), id)
	if err != nil {
		return "", err
	}
	return detail.DisplayName, nil
}
