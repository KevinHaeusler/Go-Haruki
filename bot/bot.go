package bot

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"

	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/KevinHaeusler/go-haruki/bot/clients/jellyseerr"

	"github.com/KevinHaeusler/go-haruki/bot/commands"
	"github.com/KevinHaeusler/go-haruki/bot/config"
	"github.com/KevinHaeusler/go-haruki/bot/handlers"
	"github.com/KevinHaeusler/go-haruki/bot/httpx"
)

var Session *discordgo.Session

func Start(token, guildID string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}

	httpClient := httpx.New()

	ctx := &appctx.Context{
		Config: cfg,
		HTTP:   httpClient,
	}

	if cfg.JellyseerrURL != "" && cfg.JellyseerrAPIKey != "" {
		ctx.Jelly = jellyseerr.New(cfg.JellyseerrURL, cfg.JellyseerrAPIKey, httpClient)
	}

	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return fmt.Errorf("discord session: %w", err)
	}
	s.Identify.Intents = discordgo.IntentsGuilds

	s.AddHandler(handlers.NewInteractionHandler(ctx))

	if err := s.Open(); err != nil {
		return fmt.Errorf("discord open: %w", err)
	}

	Session = s

	if err := commands.RegisterAll(Session, guildID); err != nil {
		_ = Session.Close()
		Session = nil
		return fmt.Errorf("register commands: %w", err)
	}

	log.Println("Bot running as:", s.State.User.Username)
	return nil
}

func Stop() {
	if Session != nil {
		_ = Session.Close()
		Session = nil
	}
}
