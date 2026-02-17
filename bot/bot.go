package bot

import (
	"context"
	"fmt"
	"log"

	"github.com/KevinHaeusler/go-haruki/bot/clients/radarr"
	"github.com/KevinHaeusler/go-haruki/bot/clients/sonarr"
	"github.com/KevinHaeusler/go-haruki/bot/clients/tautulli"
	"github.com/bwmarrin/discordgo"

	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/KevinHaeusler/go-haruki/bot/clients/jellyseerr"

	"github.com/KevinHaeusler/go-haruki/bot/commands"
	"github.com/KevinHaeusler/go-haruki/bot/config"
	"github.com/KevinHaeusler/go-haruki/bot/handlers"
	"github.com/KevinHaeusler/go-haruki/bot/httpx"
	"github.com/KevinHaeusler/go-haruki/bot/ui"
	"github.com/KevinHaeusler/go-haruki/bot/webhooks"
)

var Session *discordgo.Session
var webhookServerStop func()

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
	if cfg.TautulliURL != "" && cfg.TautulliAPIKey != "" {
		ctx.Tautulli = tautulli.New(cfg.TautulliURL, cfg.TautulliAPIKey, httpClient)
	}
	if cfg.SonarrURL != "" && cfg.SonarrAPIKey != "" {
		ctx.Sonarr = sonarr.New(cfg.SonarrURL, cfg.SonarrAPIKey, httpClient)
	}
	if cfg.RadarrURL != "" && cfg.RadarrAPIKey != "" {
		ctx.Radarr = radarr.New(cfg.RadarrURL, cfg.RadarrAPIKey, httpClient)
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

	// Optionally start webhook server
	if cfg.WebhookAddr != "" && cfg.WebhookPath != "" {
		server, err := webhooks.Start(cfg.WebhookAddr, cfg.WebhookPath, cfg.WebhookAuthToken, func(p webhooks.NotificationPayload) {
			if Session == nil {
				return
			}
			channelID := cfg.DiscordChannelID
			if p.DiscordChannelID != "" {
				channelID = p.DiscordChannelID
			}
			if channelID != "" {
				// Create the embed
				embed := ui.WebhookNotificationEmbed(p)

				// Try to find a Discord ID to ping
				var pingID string
				if p.Request != nil && p.Request.RequestedBySettingsDiscordID != "" {
					pingID = p.Request.RequestedBySettingsDiscordID
				} else if p.Issue != nil && p.Issue.ReportedBySettingsDiscordID != "" {
					pingID = p.Issue.ReportedBySettingsDiscordID
				} else if p.Comment != nil && p.Comment.CommentedBySettingsDiscordID != "" {
					pingID = p.Comment.CommentedBySettingsDiscordID
				}

				var content string
				if pingID != "" {
					content = fmt.Sprintf("<@%s>", pingID)
				}

				_, _ = Session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
					Content: content,
					Embed:   embed,
				})
			}
		})
		if err != nil {
			_ = Session.Close()
			Session = nil
			return fmt.Errorf("start webhook server: %w", err)
		}
		webhookServerStop = func() {
			_ = server.Shutdown(context.Background())
		}
		log.Printf("Webhook listening on %s%s", cfg.WebhookAddr, cfg.WebhookPath)
	}

	log.Println("Bot running as:", s.State.User.Username)
	return nil
}

func Stop() {
	if webhookServerStop != nil {
		webhookServerStop()
		webhookServerStop = nil
	}
	if Session != nil {
		_ = Session.Close()
		Session = nil
	}
}
