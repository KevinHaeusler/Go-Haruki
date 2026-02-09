package commands

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/KevinHaeusler/go-haruki/bot/clients/jellyseerr"
	"github.com/KevinHaeusler/go-haruki/bot/util"
)

const (
	JellyLinkSelectID = "jelly_link_select"
	JellyLinkPrevID   = "jelly_link_prev"
	JellyLinkNextID   = "jelly_link_next"
	JellyLinkAbortID  = "jelly_link_abort"

	jellyLinkPageSize = 25
	jellyLinkTake     = 100
	jellyLinkTTL      = 5 * time.Minute
)

var JellyLinkCommand = &discordgo.ApplicationCommand{
	Name:        "jelly-link",
	Description: "Assign a Discord user to a Jellyseerr user",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        "user",
			Description: "Discord user to link (optional; defaults to you)",
			Required:    false,
		},
	},
}

type jellyLinkCandidate struct {
	ID          int
	DisplayName string
	Email       string
	DiscordID   string // existing discordId, may be empty
}

type jellyLinkSession struct {
	OwnerDiscordID  string // who ran the command
	TargetDiscordID string // discord id that will be assigned
	IsAdmin         bool

	Candidates []jellyLinkCandidate // filtered list shown to user
	Page       int

	ExpiresAt time.Time
	ChannelID string
	MessageID string
}

var (
	jellyLinkMu       sync.Mutex
	jellyLinkSessions = map[string]*jellyLinkSession{} // ownerDiscordID -> session
)

func JellyLinkHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	if ctx.Jelly == nil {
		return util.RespondEphemeral(s, i, "Jellyseerr is not configured.")
	}

	ownerID := i.Member.User.ID
	targetID := ownerID
	if u := getUserOption(s, i, "user"); u != nil {
		targetID = u.ID
	}

	isAdmin := util.UserIsAdmin(i)

	// Defer (ephemeral would be ideal, but component updates for ephemerals can be awkward depending on your flow.
	// We'll keep it normal response here; change to ephemeral if you prefer.)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		return err
	}

	// Build candidate list (paged fetch + detail check for settings.discordId)
	callCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	candidates, err := fetchJellyLinkCandidates(callCtx, ctx.Jelly, isAdmin)
	if err != nil {
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: util.PtrString("Failed to load Jellyseerr users: " + err.Error()),
		})
		return nil
	}

	if len(candidates) == 0 {
		msg := "No Jellyseerr users found."
		if !isAdmin {
			msg = "No Jellyseerr users without a Discord ID were found."
		}
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: util.PtrString(msg),
		})
		return nil
	}

	sess := &jellyLinkSession{
		OwnerDiscordID:  ownerID,
		TargetDiscordID: targetID,
		IsAdmin:         isAdmin,
		Candidates:      candidates,
		Page:            0,
		ExpiresAt:       time.Now().Add(jellyLinkTTL),
	}

	embed, comps := buildJellyLinkPage(sess)
	msg, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &comps,
	})
	if err != nil {
		return nil
	}

	sess.ChannelID = msg.ChannelID
	sess.MessageID = msg.ID

	setJellyLinkSession(ownerID, sess)
	go jellyLinkExpireLoop(s, ownerID)

	return nil
}

func fetchJellyLinkCandidates(ctx context.Context, c *jellyseerr.Client, isAdmin bool) ([]jellyLinkCandidate, error) {
	all := make([]jellyLinkCandidate, 0, 64)

	for skip := 0; skip < 5000; skip += jellyLinkTake { // safety cap
		users, _, err := c.ListUsers(ctx, jellyLinkTake, skip)
		if err != nil {
			return nil, err
		}
		if len(users) == 0 {
			break
		}

		for _, u := range users {
			detail, err := c.GetUserDetail(ctx, u.ID)
			if err != nil {
				// If one user detail fails, skip it (don’t break the whole command)
				continue
			}

			discordID := detail.Settings.DiscordID

			// Non-admins see only unassigned
			if !isAdmin && discordID != "" {
				continue
			}

			all = append(all, jellyLinkCandidate{
				ID:          detail.ID,
				DisplayName: detail.DisplayName,
				Email:       detail.Email,
				DiscordID:   discordID,
			})
		}

		if len(users) < jellyLinkTake {
			break
		}
	}

	return all, nil
}

func JellyLinkSelectHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	ownerID := i.Member.User.ID
	sess := getJellyLinkSession(ownerID)
	if sess == nil || time.Now().After(sess.ExpiresAt) {
		return nil
	}

	vals := i.MessageComponentData().Values
	if len(vals) == 0 {
		return nil
	}
	jellyUserID, err := strconv.Atoi(vals[0])
	if err != nil {
		return nil
	}

	callCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Update Jellyseerr user settings.discordId
	if err := ctx.Jelly.UpdateUserDiscordID(callCtx, jellyUserID, sess.TargetDiscordID); err != nil {
		embed := &discordgo.MessageEmbed{
			Title:       "Link failed",
			Description: "Jellyseerr returned an error: " + err.Error(),
			Color:       0xff0000,
		}
		clearJellyLinkSession(ownerID)
		return editSessionMessageSimple(s, sess.ChannelID, sess.MessageID, embed, []discordgo.MessageComponent{})
	}

	embed := &discordgo.MessageEmbed{
		Title: "Linked ✅",
		Description: fmt.Sprintf(
			"Assigned Discord ID `%s` to Jellyseerr user ID `%d`.\n\nIf this is wrong, run `/jelly-link` again to reassign.",
			sess.TargetDiscordID, jellyUserID,
		),
		Color: 0x00cc66,
	}

	clearJellyLinkSession(ownerID)
	return editSessionMessageSimple(s, sess.ChannelID, sess.MessageID, embed, []discordgo.MessageComponent{})
}

func JellyLinkPrevHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = ctx
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})

	ownerID := i.Member.User.ID
	sess := getJellyLinkSession(ownerID)
	if sess == nil || time.Now().After(sess.ExpiresAt) {
		return nil
	}

	if sess.Page > 0 {
		sess.Page--
	}
	sess.ExpiresAt = time.Now().Add(jellyLinkTTL)
	setJellyLinkSession(ownerID, sess)

	embed, comps := buildJellyLinkPage(sess)
	return editSessionMessageSimple(s, sess.ChannelID, sess.MessageID, embed, comps)
}

func JellyLinkNextHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = ctx
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})

	ownerID := i.Member.User.ID
	sess := getJellyLinkSession(ownerID)
	if sess == nil || time.Now().After(sess.ExpiresAt) {
		return nil
	}

	maxPage := (len(sess.Candidates) - 1) / jellyLinkPageSize
	if sess.Page < maxPage {
		sess.Page++
	}
	sess.ExpiresAt = time.Now().Add(jellyLinkTTL)
	setJellyLinkSession(ownerID, sess)

	embed, comps := buildJellyLinkPage(sess)
	return editSessionMessageSimple(s, sess.ChannelID, sess.MessageID, embed, comps)
}

func JellyLinkAbortHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = ctx
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})

	ownerID := i.Member.User.ID
	sess := getJellyLinkSession(ownerID)
	if sess == nil {
		return nil
	}

	// Only owner or admin can abort
	if ownerID != sess.OwnerDiscordID && !util.UserIsAdmin(i) {
		return nil
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Aborted",
		Description: "Link session aborted.",
		Color:       0x999999,
	}
	clearJellyLinkSession(ownerID)
	return editSessionMessageSimple(s, sess.ChannelID, sess.MessageID, embed, []discordgo.MessageComponent{})
}

func buildJellyLinkPage(sess *jellyLinkSession) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	total := len(sess.Candidates)
	maxPage := (total - 1) / jellyLinkPageSize
	if maxPage < 0 {
		maxPage = 0
	}

	start := sess.Page * jellyLinkPageSize
	end := start + jellyLinkPageSize
	if end > total {
		end = total
	}

	embed := &discordgo.MessageEmbed{
		Title: "Jellyseerr user link",
		Description: fmt.Sprintf(
			"Assign Discord ID `%s` to a Jellyseerr user.\n\nShowing %d–%d of %d (page %d/%d).",
			sess.TargetDiscordID,
			start+1, end, total,
			sess.Page+1, maxPage+1,
		),
	}

	// Select options (max 25)
	opts := make([]discordgo.SelectMenuOption, 0, end-start)
	for _, u := range sess.Candidates[start:end] {
		label := u.DisplayName
		if label == "" {
			label = u.Email
		}
		if label == "" {
			label = fmt.Sprintf("User %d", u.ID)
		}

		desc := ""
		if sess.IsAdmin && u.DiscordID != "" {
			desc = "Already linked"
		} else if u.Email != "" {
			desc = u.Email
		}

		opts = append(opts, discordgo.SelectMenuOption{
			Label:       truncate(label, 100),
			Value:       strconv.Itoa(u.ID),
			Description: truncate(desc, 100),
		})
	}

	selectRow := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    JellyLinkSelectID,
				Placeholder: "Choose a Jellyseerr user…",
				Options:     opts,
			},
		},
	}

	prevDisabled := sess.Page == 0
	nextDisabled := sess.Page >= maxPage

	navRow := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{Label: "Prev", Style: discordgo.SecondaryButton, CustomID: JellyLinkPrevID, Disabled: prevDisabled},
			discordgo.Button{Label: "Next", Style: discordgo.SecondaryButton, CustomID: JellyLinkNextID, Disabled: nextDisabled},
			discordgo.Button{Label: "Abort", Style: discordgo.DangerButton, CustomID: JellyLinkAbortID},
		},
	}

	return embed, []discordgo.MessageComponent{selectRow, navRow}
}

// ---- session storage / expiry ----

func jellyLinkExpireLoop(s *discordgo.Session, ownerID string) {
	for {
		sess := getJellyLinkSession(ownerID)
		if sess == nil {
			return
		}

		wait := time.Until(sess.ExpiresAt)
		if wait <= 0 {
			clearJellyLinkSession(ownerID)

			embed := &discordgo.MessageEmbed{
				Title:       "Session expired",
				Description: "Run `/jelly-link` again.",
				Color:       0x999999,
			}
			_ = editSessionMessageSimple(s, sess.ChannelID, sess.MessageID, embed, []discordgo.MessageComponent{})
			return
		}

		time.Sleep(wait)
	}
}

func getJellyLinkSession(ownerID string) *jellyLinkSession {
	jellyLinkMu.Lock()
	defer jellyLinkMu.Unlock()
	return jellyLinkSessions[ownerID]
}

func setJellyLinkSession(ownerID string, sess *jellyLinkSession) {
	jellyLinkMu.Lock()
	defer jellyLinkMu.Unlock()
	jellyLinkSessions[ownerID] = sess
}

func clearJellyLinkSession(ownerID string) {
	jellyLinkMu.Lock()
	defer jellyLinkMu.Unlock()
	delete(jellyLinkSessions, ownerID)
}

// ---- tiny helpers local to this file ----

func getUserOption(s *discordgo.Session, i *discordgo.InteractionCreate, name string) *discordgo.User {
	for _, o := range i.ApplicationCommandData().Options {
		if o.Name == name {
			return o.UserValue(s)
		}
	}
	return nil
}

func editSessionMessageSimple(s *discordgo.Session, channelID, messageID string, embed *discordgo.MessageEmbed, comps []discordgo.MessageComponent) error {
	embeds := []*discordgo.MessageEmbed{embed}
	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Embeds:     &embeds,
		Components: &comps, // empty slice clears components
	})
	return err
}

func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(rs[:n-1]) + "…"
}
