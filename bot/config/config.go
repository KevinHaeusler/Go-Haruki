package config

import (
	"fmt"
	"os"
)

type Config struct {
	JellyseerrURL    string
	JellyseerrAPIKey string

	RadarrURL    string
	RadarrAPIKey string

	SonarrURL    string
	SonarrAPIKey string

	TautulliURL    string
	TautulliAPIKey string

	// Optional webhook server to receive external notifications
	WebhookAddr string // e.g. ":8080"
	WebhookPath string // e.g. "/webhook"

	// Optional: where to post incoming notifications
	DiscordChannelID string

	// Optional: authorization token for incoming webhooks
	WebhookAuthToken string
}

func Load() (Config, error) {
	c := Config{
		JellyseerrURL:    os.Getenv("JELLYSEERR_URL"),
		JellyseerrAPIKey: os.Getenv("JELLYSEERR_API_KEY"),
		RadarrURL:        os.Getenv("RADARR_URL"),
		RadarrAPIKey:     os.Getenv("RADARR_API_KEY"),
		SonarrURL:        os.Getenv("SONARR_URL"),
		SonarrAPIKey:     os.Getenv("SONARR_API_KEY"),
		TautulliURL:      os.Getenv("TAUTULLI_URL"),
		TautulliAPIKey:   os.Getenv("TAUTULLI_API_KEY"),
		WebhookAddr:      os.Getenv("WEBHOOK_ADDR"),
		WebhookPath:      os.Getenv("WEBHOOK_PATH"),
		DiscordChannelID: os.Getenv("DISCORD_CHANNEL_ID"),
		WebhookAuthToken: os.Getenv("WEBHOOK_AUTH_TOKEN"),
	}

	if c.JellyseerrURL == "" {
		return c, fmt.Errorf("missing JELLYSEERR_URL")
	}
	return c, nil
}
