package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

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
			log.Printf("[WEBHOOK] Received payload. Event: %q, Subject: %q, ChannelID: %q", p.Event, p.Subject, p.DiscordChannelID)
			if Session == nil {
				log.Printf("[WEBHOOK] Session is nil, skipping")
				return
			}
			channelID := cfg.DiscordChannelID
			if p.DiscordChannelID != "" {
				channelID = p.DiscordChannelID
			}
			log.Printf("[WEBHOOK] Target ChannelID: %q", channelID)
			if channelID != "" {
				// Create the embed
				embed := ui.WebhookNotificationEmbed(p)
				log.Printf("[WEBHOOK] Embed created. Title: %q, Color: %x", embed.Title, embed.Color)

				// Try to find Discord IDs to ping
				pingIDs := make(map[string]struct{})

				// 1. Initial IDs from payload
				if p.Request != nil && p.Request.RequestedBySettingsDiscordID != "" {
					pingIDs[p.Request.RequestedBySettingsDiscordID] = struct{}{}
				}
				if p.Issue != nil && p.Issue.ReportedBySettingsDiscordID != "" {
					pingIDs[p.Issue.ReportedBySettingsDiscordID] = struct{}{}
				}
				if p.Comment != nil && p.Comment.CommentedBySettingsDiscordID != "" {
					pingIDs[p.Comment.CommentedBySettingsDiscordID] = struct{}{}
				}

				// 2. If it's a media event and we have Jellyseerr client, fetch all requesters
				if ctx.Jelly != nil && p.Media != nil && (p.Event == "MEDIA_AVAILABLE" || p.Event == "MEDIA_APPROVED" || p.Event == "MEDIA_AUTO_APPROVED" || p.Event == "MEDIA_REQUESTED" || strings.Contains(strings.ToUpper(p.Event), "REQUEST")) {
					mediaIDStr := p.Media.TMDBID
					if mediaIDStr == "" {
						mediaIDStr = p.Media.TVDBID
					}
					mediaID, _ := strconv.Atoi(mediaIDStr)
					if mediaID > 0 {
						detail, err := ctx.Jelly.GetDetail(context.Background(), p.Media.MediaType, mediaID)
						if err == nil {
							for _, req := range detail.MediaInfo.Requests {
								user, err := ctx.Jelly.GetUserDetail(context.Background(), req.RequestedBy.ID)
								if err == nil && user.Settings.DiscordID != "" {
									pingIDs[user.Settings.DiscordID] = struct{}{}
								}
							}

							// Also update embed fields with all requesters if it's available or multiple exist
							if len(detail.MediaInfo.Requests) > 0 {
								requester, watchers := detail.RequesterSummary()
								allNames := append([]string{requester}, watchers...)

								foundRequestedBy := false
								foundTotalRequests := false

								for i, field := range embed.Fields {
									if field.Name == "Requested By" {
										embed.Fields[i].Value = strings.Join(allNames, ", ")
										foundRequestedBy = true
									}
									if field.Name == "Total Requests" {
										embed.Fields[i].Value = fmt.Sprintf("%d", len(detail.MediaInfo.Requests))
										foundTotalRequests = true
									}
								}

								// If fields were not found (e.g. not added by WebhookNotificationEmbed), add them
								if !foundRequestedBy && p.Request != nil {
									embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
										Name:   "Requested By",
										Value:  strings.Join(allNames, ", "),
										Inline: true,
									})
								}
								if !foundTotalRequests && p.Request != nil {
									embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
										Name:   "Total Requests",
										Value:  fmt.Sprintf("%d", len(detail.MediaInfo.Requests)),
										Inline: true,
									})
								}
							}
						}
					}
				}

				var content string
				if len(pingIDs) > 0 {
					var pings []string
					for id := range pingIDs {
						pings = append(pings, fmt.Sprintf("<@%s>", id))
					}
					content = strings.Join(pings, " ")
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
