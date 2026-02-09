package commands

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/KevinHaeusler/go-haruki/bot/ui"
	"github.com/KevinHaeusler/go-haruki/bot/util"
)

var PlexActivityCommand = &discordgo.ApplicationCommand{
	Name:        "plex-activity",
	Description: "Show active Plex sessions (via Tautulli)",
}

func PlexActivityHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	if ctx.Tautulli == nil {
		return util.RespondEphemeral(s, i, "Tautulli is not configured.")
	}

	// Defer immediately (avoid Discord 3s timeout)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := ctx.Tautulli.GetActivity(callCtx)
	if err != nil {
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: util.PtrString("❌ Failed to fetch Plex activity: " + err.Error()),
		})
		return nil
	}

	sessions := resp.Response.Data.Sessions
	if len(sessions) == 0 {
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: util.PtrString("No active Plex sessions right now."),
		})
		return nil
	}

	// Optional: only show "playing"
	// filtered := make([]tautulli.Session, 0, len(sessions))
	// for _, sess := range sessions {
	// 	if sess.State == "playing" {
	// 		filtered = append(filtered, sess)
	// 	}
	// }
	// sessions = filtered

	embeds := ui.PlexActivityMediaEmbeds(ctx.Tautulli, sessions)
	_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &embeds,
	})
	return nil
}
