package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/KevinHaeusler/go-haruki/bot/clients/jellyseerr"
	"github.com/KevinHaeusler/go-haruki/bot/session"
	"github.com/KevinHaeusler/go-haruki/bot/ui"
	"github.com/KevinHaeusler/go-haruki/bot/util"
	"github.com/bwmarrin/discordgo"
)

var (
	GetRequestsCommand = &discordgo.ApplicationCommand{
		Name:        "get-requests",
		Description: "List Jellyseerr requests for a user",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "The Discord user to check requests for (default: you)",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "include-finished",
				Description: "Include finished/completed requests",
				Required:    false,
			},
		},
	}

	GetRequestsPrevID  = "get_requests_prev"
	GetRequestsNextID  = "get_requests_next"
	GetRequestsAbortID = "get_requests_abort"

	getRequestsSessions = session.NewStore[getRequestsSession](180 * time.Second)
)

type getRequestsSession struct {
	DiscordUser     *discordgo.User
	JellyUserID     int
	Page            int
	Take            int
	IncludeFinished bool
	AllResults      []jellyseerr.UserRequest
	MessageID       string
	ChannelID       string
}

func GetRequestsHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	if ctx.Jelly == nil {
		return util.RespondEphemeral(s, i, "Jellyseerr client not configured.")
	}

	// Determine who ran the command
	invoker := i.Member.User
	if invoker == nil {
		invoker = i.User
	}

	targetUser := util.GetOptUser(s, i, "user")
	if targetUser == nil {
		targetUser = invoker
	}

	includeFinished := false
	for _, o := range i.ApplicationCommandData().Options {
		if o.Name == "include-finished" {
			includeFinished = o.BoolValue()
			break
		}
	}

	// Resolve Discord User to Jellyseerr User ID
	jellyID, err := ctx.Jelly.DiscordUserToJellyseerrUserID(context.Background(), targetUser.ID)
	if err != nil {
		return util.RespondEphemeral(s, i, fmt.Sprintf("Error resolving user: %v", err))
	}
	if jellyID == 0 {
		return util.RespondEphemeral(s, i, fmt.Sprintf("Discord user %s is not linked to Jellyseerr.", targetUser.Username))
	}

	take := 20
	results, err := ctx.Jelly.GetUserRequests(context.Background(), jellyID, includeFinished)
	if err != nil {
		return util.RespondEphemeral(s, i, fmt.Sprintf("Error fetching requests: %v", err))
	}

	sess := getRequestsSession{
		DiscordUser:     targetUser,
		JellyUserID:     jellyID,
		Page:            1,
		Take:            take,
		IncludeFinished: includeFinished,
		AllResults:      results,
	}

	embed, comps := buildGetRequestsPage(&sess)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: comps,
		},
	})
	if err != nil {
		return err
	}

	msg, err := s.InteractionResponse(i.Interaction)
	if err == nil {
		sess.MessageID = msg.ID
		sess.ChannelID = msg.ChannelID
		getRequestsSessions.Set(invoker.ID, sess)
		go getRequestsExpireLoop(s, invoker.ID, sess.ChannelID, sess.MessageID)
	}

	return nil
}

func GetRequestsPrevHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	invoker := i.Member.User
	if invoker == nil {
		invoker = i.User
	}
	sess := getRequestsSessions.Get(invoker.ID)
	if sess == nil || sess.Page <= 1 {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
	}

	sess.Page--
	return updateGetRequestsPage(ctx, s, i, sess)
}

func GetRequestsNextHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	invoker := i.Member.User
	if invoker == nil {
		invoker = i.User
	}
	sess := getRequestsSessions.Get(invoker.ID)
	totalPages := (len(sess.AllResults) + sess.Take - 1) / sess.Take
	if sess == nil || sess.Page >= totalPages {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
	}

	sess.Page++
	return updateGetRequestsPage(ctx, s, i, sess)
}

func GetRequestsAbortHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	invoker := i.Member.User
	if invoker == nil {
		invoker = i.User
	}
	getRequestsSessions.Clear(invoker.ID)
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "Request list aborted.",
			Embeds:     nil,
			Components: nil,
		},
	})
}

func updateGetRequestsPage(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate, sess *getRequestsSession) error {
	invoker := i.Member.User
	if invoker == nil {
		invoker = i.User
	}
	getRequestsSessions.Touch(invoker.ID)

	embed, comps := buildGetRequestsPage(sess)
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: comps,
		},
	})
}

func buildGetRequestsPage(sess *getRequestsSession) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	start := (sess.Page - 1) * sess.Take
	end := start + sess.Take
	if end > len(sess.AllResults) {
		end = len(sess.AllResults)
	}

	var pageResults []jellyseerr.UserRequest
	if start < len(sess.AllResults) {
		pageResults = sess.AllResults[start:end]
	}

	totalPages := (len(sess.AllResults) + sess.Take - 1) / sess.Take
	if totalPages == 0 {
		totalPages = 1
	}

	embed := ui.JellyRequestListEmbed(sess.DiscordUser, pageResults, sess.Page, totalPages, len(sess.AllResults))

	prevBtn := discordgo.Button{
		Label:    "Previous",
		Style:    discordgo.SecondaryButton,
		CustomID: GetRequestsPrevID,
		Disabled: sess.Page <= 1,
	}
	nextBtn := discordgo.Button{
		Label:    "Next",
		Style:    discordgo.SecondaryButton,
		CustomID: GetRequestsNextID,
		Disabled: sess.Page >= totalPages,
	}
	abortBtn := ui.AbortButton(GetRequestsAbortID)

	comps := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{prevBtn, nextBtn, abortBtn},
		},
	}

	return embed, comps
}

func getRequestsExpireLoop(s *discordgo.Session, userID string, channelID string, messageID string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sess, expired := getRequestsSessions.GetWithExpiration(userID)
		if sess == nil {
			if expired {
				_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
					Channel:    channelID,
					ID:         messageID,
					Components: &[]discordgo.MessageComponent{},
				})
			}
			return
		}
	}
}
