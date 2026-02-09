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
	}

	if c.JellyseerrURL == "" {
		return c, fmt.Errorf("missing JELLYSEERR_URL")
	}
	return c, nil
}
