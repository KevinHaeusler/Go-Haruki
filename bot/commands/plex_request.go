package commands

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/KevinHaeusler/go-haruki/bot/clients/jellyseerr"
	"github.com/KevinHaeusler/go-haruki/bot/session"
	"github.com/KevinHaeusler/go-haruki/bot/ui"
	"github.com/KevinHaeusler/go-haruki/bot/util"
)

const (
	// PlexRoleID Required role to use /plex-request
	PlexRoleID = "1228676841057816707"

	PlexRequestSelectID  = "plex_request_select"
	PlexRequestConfirmID = "plex_request_confirm"
	PlexRequestAbortID   = "plex_request_abort"
	PlexRequestNotifyID  = "plex_request_notify"

	// PlexSessionTTL Session TTL
	PlexSessionTTL = 180 * time.Second
)

var PlexRequestCommand = &discordgo.ApplicationCommand{
	Name:        "plex-request",
	Description: "Request media via Jellyseerr/Overseerr",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "media-type",
			Description: "tv or movie",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "tv", Value: "tv"},
				{Name: "movie", Value: "movie"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "media",
			Description: "Media name to search",
			Required:    true,
		},
	},
}

// ---- session state ----

type requestSession struct {
	UserID     string
	MediaType  string
	Query      string
	Results    []jellyseerr.MediaSummary
	SelectedID int

	ChannelID string
	MessageID string
}

var (
	requestStore = session.NewStore[requestSession](PlexSessionTTL)
)

// ---- slash handler ----

func PlexRequestHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	if ctx.Jelly == nil {
		return util.RespondEphemeral(s, i, "Jellyseerr is not configured.")
	}

	if !util.UserHasRole(i, PlexRoleID) {
		return util.RespondEphemeral(s, i, "You need the Plex role to use `/plex-request`.")
	}

	mt := strings.ToLower(strings.TrimSpace(util.GetOptString(i, "media-type")))
	q := strings.TrimSpace(util.GetOptString(i, "media"))
	log.Printf("[CMD] /plex-request invoked by %s (%s) guild=%s channel=%s media-type=%s query=%q", i.Member.User.Username, i.Member.User.ID, i.GuildID, i.ChannelID, mt, q)

	if mt != "tv" && mt != "movie" {
		return util.RespondEphemeral(s, i, "media-type must be `tv` or `movie`.")
	}
	if q == "" {
		return util.RespondEphemeral(s, i, "media cannot be empty.")
	}

	// Defer immediately (avoid Discord 3s timeout)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	results, err := ctx.Jelly.SearchSummary(callCtx, q, mt)
	if err != nil {
		log.Printf("[CMD] /plex-request search error for %s (%s): %v", i.Member.User.Username, i.Member.User.ID, err)
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: util.PtrString("Search failed: " + err.Error()),
		})
		return nil
	}
	log.Printf("[CMD] /plex-request results=%d for query=%q", len(results), q)
	if len(results) == 0 {
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: util.PtrString(fmt.Sprintf("No results for `%s`.", q)),
		})
		return nil
	}

	embed := ui.JellyResultListEmbed(q)

	components := []discordgo.MessageComponent{
		ui.ResultsSelect(PlexRequestSelectID, results, 0),
		ui.ButtonsRow(ui.AbortButton(PlexRequestAbortID)),
	}

	msg, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	})
	if err != nil {
		return nil
	}

	userID := i.Member.User.ID
	requestStore.Set(userID, requestSession{
		UserID:     userID,
		MediaType:  mt,
		Query:      q,
		Results:    results,
		SelectedID: 0,
		ChannelID:  msg.ChannelID,
		MessageID:  msg.ID,
	})

	go expireSession(s, userID, msg.ChannelID, msg.ID)

	return nil
}

// ---- component handlers ----

func PlexRequestSelectHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	// Acknowledge component interaction immediately
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	log.Printf("[CMD] /plex-request select by %s (%s) guild=%s channel=%s", i.Member.User.Username, i.Member.User.ID, i.GuildID, i.ChannelID)

	userID := i.Member.User.ID
	sess := requestStore.Get(userID)
	if sess == nil {
		return nil
	}
	requestStore.Touch(userID)

	vals := i.MessageComponentData().Values
	if len(vals) == 0 {
		return nil
	}

	selectedID, err := strconv.Atoi(vals[0])
	if err != nil {
		return nil
	}
	log.Printf("[CMD] /plex-request selected mediaID=%d by %s (%s)", selectedID, i.Member.User.Username, i.Member.User.ID)

	sess.SelectedID = selectedID
	requestStore.Set(userID, *sess)

	callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	detail, err := ctx.Jelly.GetDetail(callCtx, sess.MediaType, sess.SelectedID)
	if err != nil {
		return editSessionMessage(s, sess, "Failed to load details: "+err.Error(), nil, nil)
	}

	embed := ui.JellyDetailEmbed(detail, sess.MediaType)

	components := []discordgo.MessageComponent{
		ui.ResultsSelect(PlexRequestSelectID, sess.Results, sess.SelectedID),
		ui.ButtonsRow(
			ui.ConfirmButton(PlexRequestConfirmID),
			ui.AbortButton(PlexRequestAbortID),
		),
	}

	return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, components)
}

func PlexRequestConfirmHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	log.Printf("[CMD] /plex-request confirm by %s (%s) guild=%s channel=%s", i.Member.User.Username, i.Member.User.ID, i.GuildID, i.ChannelID)

	userID := i.Member.User.ID
	sess := requestStore.Get(userID)
	if sess == nil {
		return nil
	}
	requestStore.Touch(userID)

	callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	detail, err := ctx.Jelly.GetDetail(callCtx, sess.MediaType, sess.SelectedID)
	if err != nil {
		log.Printf("[CMD] /plex-request confirm load details error: %v", err)
		return editSessionMessage(s, sess, "Failed to load details: "+err.Error(), nil, nil)
	}

	status := detail.MediaInfo.Status
	log.Printf("[CMD] /plex-request media status=%d mediaType=%s mediaID=%d", status, sess.MediaType, sess.SelectedID)

	// 2 or 3 => already requested
	if status == 2 || status == 3 {
		log.Printf("[CMD] /plex-request already requested by someone else. user=%s (%s) mediaID=%d", i.Member.User.Username, i.Member.User.ID, sess.SelectedID)
		embed := ui.JellyAlreadyRequestedEmbed(detail, sess.MediaType)

		comps := []discordgo.MessageComponent{
			ui.ButtonsRow(
				ui.NotifyButton(PlexRequestNotifyID),
				ui.AbortButton(PlexRequestAbortID),
			),
		}

		return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, comps)
	}

	// 4 = partial availability -> show requester + notify button (NOT terminal)
	if status == 4 {
		log.Printf("[CMD] /plex-request partial availability for mediaID=%d", sess.SelectedID)
		embed := ui.JellyPartialAvailabilityEmbed(detail, sess.MediaType)

		comps := []discordgo.MessageComponent{
			ui.ButtonsRow(
				ui.NotifyButton(PlexRequestNotifyID),
				ui.AbortButton(PlexRequestAbortID),
			),
		}

		return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, comps)
	}

	// 5 = already available -> terminal
	if status == 5 {
		log.Printf("[CMD] /plex-request already available mediaID=%d", sess.SelectedID)
		requestStore.Clear(userID)
		embed := ui.JellyAvailabilityEmbed(detail, sess.MediaType, status)
		return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, []discordgo.MessageComponent{})
	}

	overseerrUserID, err := ctx.Jelly.DiscordUserToJellyseerrUserID(callCtx, userID)
	if err != nil {
		log.Printf("[CMD] /plex-request error mapping Discord->Jellyseerr for %s (%s): %v", i.Member.User.Username, i.Member.User.ID, err)
		return editSessionMessage(s, sess, "Failed to link your Discord ID in Overseerr.", nil, nil)
	}
	if overseerrUserID == 0 {
		log.Printf("[CMD] /plex-request no Jellyseerr link for %s (%s)", i.Member.User.Username, i.Member.User.ID)
		return editSessionMessage(s, sess, "Your Discord ID is not linked in Overseerr.", nil, nil)
	}

	if detail.HasRequester(overseerrUserID) {
		log.Printf("[CMD] /plex-request user already requester jellyUserID=%d discordUser=%s (%s) mediaID=%d", overseerrUserID, i.Member.User.Username, i.Member.User.ID, sess.SelectedID)
		requestStore.Clear(userID)
		embed := &discordgo.MessageEmbed{
			Title:       "ℹ️ Already Requested",
			Description: "You’ve already requested this media.",
		}
		return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, []discordgo.MessageComponent{})
	}

	resp, err := ctx.Jelly.RequestMedia(callCtx, sess.MediaType, sess.SelectedID, overseerrUserID)
	if err != nil {
		log.Printf("[CMD] /plex-request request failed jellyUserID=%d mediaType=%s mediaID=%d err=%v", overseerrUserID, sess.MediaType, sess.SelectedID, err)
		return editSessionMessage(s, sess, "Request failed: "+err.Error(), nil, nil)
	}

	log.Printf("[CMD] /plex-request sent by %s (%s) jellyUserID=%d mediaType=%s mediaID=%d", i.Member.User.Username, i.Member.User.ID, overseerrUserID, sess.MediaType, sess.SelectedID)
	requestStore.Clear(userID)

	total := resp.RequestedBy.RequestCount + 1
	embed := ui.JellyRequestSentEmbed(detail, sess.MediaType, i.Member.User.Username, total)

	return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, []discordgo.MessageComponent{})
}

func PlexRequestAbortHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = ctx

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	log.Printf("[CMD] /plex-request abort by %s (%s) guild=%s channel=%s", i.Member.User.Username, i.Member.User.ID, i.GuildID, i.ChannelID)

	userID := i.Member.User.ID
	sess := requestStore.Get(userID)
	if sess == nil {
		return nil
	}

	if userID != sess.UserID && !util.UserIsAdmin(i) {
		return nil
	}

	requestStore.Clear(userID)

	embed := &discordgo.MessageEmbed{
		Title:       "Aborted",
		Description: "Request session aborted.",
	}

	return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, []discordgo.MessageComponent{})
}

func expireSession(s *discordgo.Session, userID, channelID, messageID string) {
	for {
		time.Sleep(10 * time.Second)
		_, expired := requestStore.GetWithExpiration(userID)
		if expired {
			embed := &discordgo.MessageEmbed{
				Title:       "Aborted",
				Description: "Session timed out after 3 minutes of inactivity.",
				Color:       0xff0000,
			}
			embeds := []*discordgo.MessageEmbed{embed}
			_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:         messageID,
				Channel:    channelID,
				Embeds:     &embeds,
				Components: &[]discordgo.MessageComponent{},
			})
			return
		}
		if requestStore.Get(userID) == nil {
			return
		}
	}
}

func editSessionMessage(s *discordgo.Session, sess *requestSession, content string, embeds []*discordgo.MessageEmbed, comps []discordgo.MessageComponent) error {
	var embPtr *[]*discordgo.MessageEmbed
	if embeds != nil {
		embPtr = &embeds
	}

	var compPtr *[]discordgo.MessageComponent
	if comps != nil {
		compPtr = &comps
	}

	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         sess.MessageID,
		Channel:    sess.ChannelID,
		Content:    &content,
		Embeds:     embPtr,
		Components: compPtr,
	})
	return err
}

func PlexRequestNotifyHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	userID := i.Member.User.ID
	sess := requestStore.Get(userID)

	if sess == nil || sess.SelectedID == 0 {
		return nil
	}
	requestStore.Touch(userID)

	callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	overID, err := ctx.Jelly.DiscordUserToJellyseerrUserID(callCtx, userID)
	if err != nil || overID == 0 {
		embed := &discordgo.MessageEmbed{
			Title:       "Not linked",
			Description: "Your Discord ID is not linked in Jellyseerr.",
		}
		requestStore.Clear(userID)
		return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, []discordgo.MessageComponent{})
	}

	detail, err := ctx.Jelly.GetDetail(callCtx, sess.MediaType, sess.SelectedID)
	if err != nil {
		return editSessionMessage(s, sess, "Failed to load details: "+err.Error(), nil, nil)
	}

	if detail.HasRequester(overID) {
		embed := &discordgo.MessageEmbed{
			Title:       "ℹ️ Already Requested",
			Description: "You’ll be notified (already on the watcher list).",
		}
		requestStore.Clear(userID)
		return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, []discordgo.MessageComponent{})
	}

	_, err = ctx.Jelly.RequestMedia(callCtx, sess.MediaType, sess.SelectedID, overID)
	if err != nil {
		return editSessionMessage(s, sess, "Notify request failed: "+err.Error(), nil, nil)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🔔 Notification Requested",
		Description: "You'll be notified when this item becomes available.",
	}

	requestStore.Clear(userID)
	return editSessionMessage(s, sess, "", []*discordgo.MessageEmbed{embed}, []discordgo.MessageComponent{})
}
