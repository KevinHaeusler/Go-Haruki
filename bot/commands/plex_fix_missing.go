package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/KevinHaeusler/go-haruki/bot/session"
	"github.com/KevinHaeusler/go-haruki/bot/ui"
	"github.com/KevinHaeusler/go-haruki/bot/util"
)

const (
	PlexFixMissingSelectMedia   = "plex_fix_missing_select_media"
	PlexFixMissingSelectSeason  = "plex_fix_missing_select_season"
	PlexFixMissingSelectEpisode = "plex_fix_missing_select_episode"
	PlexFixMissingSelectRelease = "plex_fix_missing_select_release"
	PlexFixMissingChangeRelease = "plex_fix_missing_change_release"
	PlexFixMissingApprove       = "plex_fix_missing_approve"
	PlexFixMissingAbort         = "plex_fix_missing_abort"
	PlexFixMissingPageNext      = "plex_fix_missing_page_next"
	PlexFixMissingPagePrev      = "plex_fix_missing_page_prev"
	PlexFixMissingEpPageNext    = "plex_fix_missing_ep_page_next"
	PlexFixMissingEpPagePrev    = "plex_fix_missing_ep_page_prev"
)

var PlexFixMissingCommand = &discordgo.ApplicationCommand{
	Name:        "plex-fix-missing",
	Description: "Fix missing Plex media (missing only or all files).",
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
			Description: "Enter the media to search for",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "listing-mode",
			Description: "Choose listing mode: missing only or all files",
			Required:    false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "missing only", Value: "missing only"},
				{Name: "all files", Value: "all files"},
			},
		},
	},
}

const (
	pfmPageSize   = 25
	pfmSessionTTL = 180 * time.Second
)

type pfmSession struct {
	UserID      string
	IsMovie     bool
	ListingMode string
	Query       string

	SearchResults []map[string]any
	Page          int

	SelectedMedia map[string]any

	// TV specific
	MissingEpisodes  []map[string]any
	CurrentSeasonEps []map[string]any
	EpisodePage      int

	// Releases
	Releases        []map[string]any
	SelectedRelease map[string]any

	ChannelID   string
	MessageID   string
	Interaction *discordgo.Interaction
}

var (
	pfmStore = session.NewStore[pfmSession](pfmSessionTTL)
)

// ---- helpers ----

func pfmSelectRow(customID string, opts []discordgo.SelectMenuOption, placeholder string) discordgo.ActionsRow {
	return ui.SelectMenu(customID, placeholder, opts)
}

func pfmButtonsRow(btns ...discordgo.MessageComponent) discordgo.ActionsRow {
	return ui.ButtonsRow(btns...).(discordgo.ActionsRow)
}

func pfmAbortBtn() discordgo.Button {
	return ui.AbortButton(PlexFixMissingAbort)
}

// ---- slash handler ----

func PlexFixMissingHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	// ensure required clients
	mt := strings.ToLower(strings.TrimSpace(util.GetOptString(i, "media-type")))
	q := strings.TrimSpace(util.GetOptString(i, "media"))
	mode := util.GetOptString(i, "listing-mode")
	if mode == "" {
		mode = "missing only"
	}

	if !util.UserHasRole(i, PlexRoleID) {
		return util.RespondEphemeral(s, i, "You need the Plex role to use `/plex-fix-missing`.")
	}
	if mt != "tv" && mt != "movie" {
		return util.RespondEphemeral(s, i, "media-type must be `tv` or `movie`.")
	}
	if q == "" {
		return util.RespondEphemeral(s, i, "media cannot be empty.")
	}
	if mt == "tv" && ctx.Sonarr == nil {
		return util.RespondEphemeral(s, i, "Sonarr is not configured.")
	}
	if mt == "movie" && ctx.Radarr == nil {
		return util.RespondEphemeral(s, i, "Radarr is not configured.")
	}

	// defer response
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredChannelMessageWithSource}); err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var results []map[string]any
	if mt == "movie" {
		if mode == "all files" {
			var movies []map[string]any
			if err := ctx.Radarr.Get(callCtx, "movie", &movies); err != nil {
				_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: util.PtrString("Fetch movies failed: " + err.Error())})
				return nil
			}
			results = movies
		} else {
			var resp struct {
				Records []map[string]any `json:"records"`
			}
			if err := ctx.Radarr.Get(callCtx, "wanted/missing?page=1&pageSize=1000&monitored=true", &resp); err != nil {
				_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: util.PtrString("Fetch missing failed: " + err.Error())})
				return nil
			}
			results = resp.Records
		}
		// filter by title
		qL := strings.ToLower(q)
		filtered := make([]map[string]any, 0)
		for _, m := range results {
			t, _ := m["title"].(string)
			if strings.Contains(strings.ToLower(t), qL) {
				filtered = append(filtered, m)
			}
		}
		results = filtered
	} else {
		var series []map[string]any
		if err := ctx.Sonarr.Get(callCtx, "series", &series); err != nil {
			_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: util.PtrString("Fetch series failed: " + err.Error())})
			return nil
		}
		qL := strings.ToLower(q)
		for _, srs := range series {
			t, _ := srs["title"].(string)
			if strings.Contains(strings.ToLower(t), qL) {
				results = append(results, srs)
			}
		}
	}

	if len(results) == 0 {
		msg := fmt.Sprintf("No %sresults found for `%s`.", map[bool]string{true: "", false: "missing "}[mode == "all files"], q)
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: util.PtrString(msg)})
		return nil
	}

	opts := make([]discordgo.SelectMenuOption, 0, min(pfmPageSize, len(results)))
	for _, it := range results[:min(pfmPageSize, len(results))] {
		title, _ := it["title"].(string)
		label := title
		if len(label) > 100 {
			label = label[:97] + "..."
		}
		id := fmt.Sprintf("%v", it["id"])
		opts = append(opts, discordgo.SelectMenuOption{Label: label, Value: id})
	}
	components := []discordgo.MessageComponent{
		pfmSelectRow(PlexFixMissingSelectMedia, opts, "Select Media"),
		pfmButtonsRow(pfmAbortBtn()),
	}

	msg, _ := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds:     &[]*discordgo.MessageEmbed{ui.PlexFixMissingMediaEmbed(1, (len(results)-1)/pfmPageSize+1)},
		Components: &components,
	})

	userID := i.Member.User.ID
	pfmStore.Set(userID, pfmSession{
		UserID:        userID,
		IsMovie:       mt == "movie",
		ListingMode:   mode,
		Query:         q,
		SearchResults: results,
		Page:          0,
		Interaction:   i.Interaction,
		ChannelID:     msg.ChannelID,
		MessageID:     msg.ID,
	})
	go pfmExpire(s, userID, msg.ChannelID, msg.ID)
	return nil
}

func pfmExpire(s *discordgo.Session, userID, channelID, messageID string) {
	for {
		time.Sleep(10 * time.Second)
		_, expired := pfmStore.GetWithExpiration(userID)
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
		// Check if it was cleared manually (sess will be nil but expired will be false)
		if pfmStore.Get(userID) == nil {
			return
		}
	}
}

// ---- paging media ----

func PlexFixMissingMediaPagingHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	userID := i.Member.User.ID
	sess := pfmStore.Get(userID)
	if sess == nil {
		return nil
	}
	pfmStore.Touch(userID)
	if i.MessageComponentData().CustomID == PlexFixMissingPageNext {
		sess.Page++
	} else if sess.Page > 0 {
		sess.Page--
	}
	pfmStore.Set(userID, *sess)
	return pfmSendMediaPage(s, sess)
}

func pfmSendMediaPage(s *discordgo.Session, sess *pfmSession) error {
	start := sess.Page * pfmPageSize
	end := start + pfmPageSize
	if start > len(sess.SearchResults) {
		sess.Page = 0
		start = 0
		end = min(pfmPageSize, len(sess.SearchResults))
	}
	if end > len(sess.SearchResults) {
		end = len(sess.SearchResults)
	}
	slice := sess.SearchResults[start:end]
	opts := make([]discordgo.SelectMenuOption, 0, len(slice))
	for _, it := range slice {
		title, _ := it["title"].(string)
		label := title
		if len(label) > 100 {
			label = label[:97] + "..."
		}
		id := fmt.Sprintf("%v", it["id"])
		opts = append(opts, discordgo.SelectMenuOption{Label: label, Value: id})
	}
	rows := []discordgo.MessageComponent{
		pfmSelectRow(PlexFixMissingSelectMedia, opts, "Select Media"),
	}
	// nav
	btns := []discordgo.MessageComponent{}
	if sess.Page > 0 {
		btns = append(btns, discordgo.Button{Label: "Prev", Style: discordgo.SecondaryButton, CustomID: PlexFixMissingPagePrev})
	}
	if end < len(sess.SearchResults) {
		btns = append(btns, discordgo.Button{Label: "Next", Style: discordgo.SecondaryButton, CustomID: PlexFixMissingPageNext})
	}
	btns = append(btns, pfmAbortBtn())
	rows = append(rows, pfmButtonsRow(btns...))

	embeds := []*discordgo.MessageEmbed{ui.PlexFixMissingMediaEmbed(sess.Page+1, (len(sess.SearchResults)-1)/pfmPageSize+1)}
	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Embeds: &embeds, Components: &rows})
	return err
}

// ---- media selection ----

func PlexFixMissingMediaSelectHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	sess := pfmStore.Get(i.Member.User.ID)
	if sess == nil {
		return nil
	}
	pfmStore.Touch(i.Member.User.ID)
	choice := i.MessageComponentData().Values[0]
	var item map[string]any
	for _, it := range sess.SearchResults {
		if fmt.Sprintf("%v", it["id"]) == choice {
			item = it
			break
		}
	}
	if item == nil {
		_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Content: util.PtrString("Media not found."), Components: &[]discordgo.MessageComponent{}})
		pfmStore.Clear(sess.UserID)
		return nil
	}
	sess.SelectedMedia = item
	pfmStore.Set(sess.UserID, *sess)
	if sess.IsMovie {
		return pfmShowMovieReleases(ctx, s, sess)
	}
	// TV: fetch episodes
	callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	seriesID := fmt.Sprintf("%v", item["id"])
	var eps []map[string]any
	if err := ctx.Sonarr.Get(callCtx, "episode?seriesId="+seriesID, &eps); err != nil {
		_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Content: util.PtrString("Fetch episodes failed: " + err.Error())})
		return nil
	}
	if sess.ListingMode == "missing only" {
		filtered := make([]map[string]any, 0)
		for _, ep := range eps {
			if has, _ := ep["hasFile"].(bool); !has {
				filtered = append(filtered, ep)
			}
		}
		eps = filtered
	}
	sess.MissingEpisodes = eps
	pfmStore.Set(sess.UserID, *sess)
	// seasons
	seen := map[int]bool{}
	opts := []discordgo.SelectMenuOption{}
	for _, ep := range eps {
		se, _ := ep["seasonNumber"].(float64)
		season := int(se)
		if !seen[season] {
			seen[season] = true
			opts = append(opts, discordgo.SelectMenuOption{Label: fmt.Sprintf("Season %d", season), Value: fmt.Sprintf("%d", season)})
		}
	}
	if len(opts) == 0 {
		opts = []discordgo.SelectMenuOption{{Label: "No episodes found", Value: "0"}}
	}
	rows := []discordgo.MessageComponent{
		pfmSelectRow(PlexFixMissingSelectSeason, opts, "Select Season"),
		pfmButtonsRow(pfmAbortBtn()),
	}
	embeds := []*discordgo.MessageEmbed{ui.PlexFixMissingSeasonEmbed()}
	_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Embeds: &embeds, Components: &rows})
	return nil
}

func PlexFixMissingSeasonSelectHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	sess := pfmStore.Get(i.Member.User.ID)
	if sess == nil {
		return nil
	}
	pfmStore.Touch(i.Member.User.ID)
	val := i.MessageComponentData().Values[0]
	season := 0
	fmt.Sscanf(val, "%d", &season)
	// filter eps by season
	eps := make([]map[string]any, 0)
	for _, ep := range sess.MissingEpisodes {
		seF, _ := ep["seasonNumber"].(float64)
		if int(seF) == season {
			eps = append(eps, ep)
		}
	}
	sess.CurrentSeasonEps = eps
	sess.EpisodePage = 0
	pfmStore.Set(sess.UserID, *sess)
	return pfmSendEpisodePage(s, sess)
}

func pfmSendEpisodePage(s *discordgo.Session, sess *pfmSession) error {
	start := sess.EpisodePage * pfmPageSize
	end := start + pfmPageSize
	if end > len(sess.CurrentSeasonEps) {
		end = len(sess.CurrentSeasonEps)
	}
	slice := sess.CurrentSeasonEps[start:end]
	opts := []discordgo.SelectMenuOption{}
	for _, ep := range slice {
		has, _ := ep["hasFile"].(bool)
		status := "❓"
		if has {
			status = "✅"
		}
		seF, _ := ep["seasonNumber"].(float64)
		epF, _ := ep["episodeNumber"].(float64)
		title, _ := ep["title"].(string)
		label := fmt.Sprintf("%s S%02dE%02d - %s", status, int(seF), int(epF), title)
		if len(label) > 100 {
			label = label[:97] + "..."
		}
		id := fmt.Sprintf("%v", ep["id"])
		opts = append(opts, discordgo.SelectMenuOption{Label: label, Value: id})
	}
	rows := []discordgo.MessageComponent{pfmSelectRow(PlexFixMissingSelectEpisode, opts, "Select Episode")}
	btns := []discordgo.MessageComponent{}
	if sess.EpisodePage > 0 {
		btns = append(btns, discordgo.Button{Label: "Prev", Style: discordgo.SecondaryButton, CustomID: PlexFixMissingEpPagePrev})
	}
	if end < len(sess.CurrentSeasonEps) {
		btns = append(btns, discordgo.Button{Label: "Next", Style: discordgo.SecondaryButton, CustomID: PlexFixMissingEpPageNext})
	}
	btns = append(btns, pfmAbortBtn())
	rows = append(rows, pfmButtonsRow(btns...))
	embeds := []*discordgo.MessageEmbed{ui.PlexFixMissingEpisodeEmbed(sess.EpisodePage+1, (len(sess.CurrentSeasonEps)-1)/pfmPageSize+1)}
	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Embeds: &embeds, Components: &rows})
	return err
}

func PlexFixMissingEpisodePagingHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	sess := pfmStore.Get(i.Member.User.ID)
	if sess == nil {
		return nil
	}
	pfmStore.Touch(i.Member.User.ID)
	if i.MessageComponentData().CustomID == PlexFixMissingEpPageNext {
		sess.EpisodePage++
	} else if sess.EpisodePage > 0 {
		sess.EpisodePage--
	}
	pfmStore.Set(i.Member.User.ID, *sess)
	return pfmSendEpisodePage(s, sess)
}

func PlexFixMissingEpisodeSelectHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	sess := pfmStore.Get(i.Member.User.ID)
	if sess == nil {
		return nil
	}
	pfmStore.Touch(i.Member.User.ID)

	// Show "Now Searching"
	searchingEmbeds := []*discordgo.MessageEmbed{ui.PlexFixMissingSearchingEmbed()}
	_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         sess.MessageID,
		Channel:    sess.ChannelID,
		Embeds:     &searchingEmbeds,
		Components: &[]discordgo.MessageComponent{}, // clear components
	})

	epID := i.MessageComponentData().Values[0]
	callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var releases []map[string]any
	if err := ctx.Sonarr.Get(callCtx, "release?episodeId="+epID, &releases); err != nil {
		// If the fetch timed out (after 60s), abort the session with a friendly message
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			seriesTitle, _ := sess.SelectedMedia["title"].(string)
			epDisplay := "Episode"
			for _, ep := range sess.CurrentSeasonEps {
				if fmt.Sprintf("%v", ep["id"]) == epID {
					seF, _ := ep["seasonNumber"].(float64)
					epF, _ := ep["episodeNumber"].(float64)
					title, _ := ep["title"].(string)
					epDisplay = fmt.Sprintf("S%02dE%02d - %s", int(seF), int(epF), title)
					break
				}
			}
			msg := fmt.Sprintf("No Downloads found for Media - %s - %s", seriesTitle, epDisplay)
			abortEmbed := &discordgo.MessageEmbed{
				Title:       "No results",
				Description: msg,
				Color:       0xff0000,
			}
			abortEmbeds := []*discordgo.MessageEmbed{abortEmbed}
			_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:         sess.MessageID,
				Channel:    sess.ChannelID,
				Embeds:     &abortEmbeds,
				Components: &[]discordgo.MessageComponent{},
			})
			pfmStore.Clear(sess.UserID)
			return nil
		}
		_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Content: util.PtrString("Fetch releases failed: " + err.Error())})
		return nil
	}
	sess.Releases = releases
	pfmStore.Set(sess.UserID, *sess)
	return pfmDisplayReleaseOptions(s, sess, false)
}

func pfmShowMovieReleases(ctx *appctx.Context, s *discordgo.Session, sess *pfmSession) error {
	// Show "Now Searching"
	searchingEmbeds := []*discordgo.MessageEmbed{ui.PlexFixMissingSearchingEmbed()}
	_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         sess.MessageID,
		Channel:    sess.ChannelID,
		Embeds:     &searchingEmbeds,
		Components: &[]discordgo.MessageComponent{}, // clear components
	})

	callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	movieID := fmt.Sprintf("%v", sess.SelectedMedia["id"])
	var releases []map[string]any
	if err := ctx.Radarr.Get(callCtx, "release?movieId="+movieID, &releases); err != nil {
		// If the fetch timed out (after 60s), abort the session with a friendly message
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			title, _ := sess.SelectedMedia["title"].(string)
			msg := fmt.Sprintf("No files found for Media - %s", title)
			abortEmbed := &discordgo.MessageEmbed{
				Title:       "No results",
				Description: msg,
				Color:       0xff0000,
			}
			abortEmbeds := []*discordgo.MessageEmbed{abortEmbed}
			_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:         sess.MessageID,
				Channel:    sess.ChannelID,
				Embeds:     &abortEmbeds,
				Components: &[]discordgo.MessageComponent{},
			})
			pfmStore.Clear(sess.UserID)
			return nil
		}
		_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Content: util.PtrString("Fetch releases failed: " + err.Error())})
		return nil
	}
	// filter valid
	valid := make([]map[string]any, 0)
	for _, r := range releases {
		rej, _ := r["rejected"].(bool)
		score, _ := r["customFormatScore"].(float64)
		if !rej || score > 0 {
			valid = append(valid, r)
		}
	}
	if len(valid) == 0 {
		valid = releases
	}
	sess.Releases = valid
	pfmStore.Set(sess.UserID, *sess)
	return pfmDisplayReleaseOptions(s, sess, true)
}

func pfmDisplayReleaseOptions(s *discordgo.Session, sess *pfmSession, isMovie bool) error {
	opts := pfmBuildReleaseOptions(sess, isMovie, "")
	rows := []discordgo.MessageComponent{
		pfmSelectRow(PlexFixMissingSelectRelease, opts, "Select Release"),
		pfmButtonsRow(discordgo.Button{Label: "Change", Style: discordgo.PrimaryButton, CustomID: PlexFixMissingChangeRelease}, pfmAbortBtn()),
	}
	embeds := []*discordgo.MessageEmbed{ui.PlexFixMissingReleaseEmbed(isMovie)}
	_, err := s.InteractionResponseEdit(sess.Interaction, &discordgo.WebhookEdit{Embeds: &embeds, Components: &rows})
	return err
}

func pfmBuildReleaseOptions(sess *pfmSession, isMovie bool, selectedGUID string) []discordgo.SelectMenuOption {
	// sort key
	key := func(m map[string]any) float64 {
		if v, ok := m["customFormatScore"].(float64); ok {
			return v
		}
		if v, ok := m["qualityWeight"].(float64); ok {
			return v
		}
		return 0
	}
	sort.SliceStable(sess.Releases, func(i, j int) bool { return key(sess.Releases[i]) > key(sess.Releases[j]) })
	opts := []discordgo.SelectMenuOption{}
	for _, r := range sess.Releases {
		approved, _ := r["approved"].(bool)
		emoji := "❌"
		if approved {
			emoji = "✅"
		}
		title, _ := r["title"].(string)
		if title == "" {
			if mts, ok := r["movieTitles"].([]any); ok && len(mts) > 0 {
				title, _ = mts[0].(string)
			}
		}
		label := fmt.Sprintf("%s %s", emoji, title)
		if len(label) > 100 {
			label = label[:97] + "..."
		}
		guid := fmt.Sprintf("%v", r["guid"])
		opts = append(opts, discordgo.SelectMenuOption{Label: label, Value: guid, Default: guid == selectedGUID})
		if len(opts) == 25 {
			break
		}
	}
	return opts
}

func PlexFixMissingReleaseSelectHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	sess := pfmStore.Get(i.Member.User.ID)
	if sess == nil {
		return nil
	}
	pfmStore.Touch(i.Member.User.ID)
	guid := i.MessageComponentData().Values[0]
	var rel map[string]any
	for _, r := range sess.Releases {
		if fmt.Sprintf("%v", r["guid"]) == guid {
			rel = r
			break
		}
	}
	if rel == nil {
		_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Content: util.PtrString("Release not found."), Components: &[]discordgo.MessageComponent{}})
		pfmStore.Clear(sess.UserID)
		return nil
	}
	sess.SelectedRelease = rel
	pfmStore.Set(sess.UserID, *sess)
	// Build info embed
	embed := ui.PlexFixMissingReleaseInfoEmbed(rel)

	rows := []discordgo.MessageComponent{
		pfmSelectRow(PlexFixMissingSelectRelease, pfmBuildReleaseOptions(sess, sess.IsMovie, guid), "Select Release"),
		pfmButtonsRow(discordgo.Button{Label: "Approve", Style: discordgo.SuccessButton, CustomID: PlexFixMissingApprove}, pfmAbortBtn()),
	}
	_, _ = s.InteractionResponseEdit(sess.Interaction, &discordgo.WebhookEdit{Embeds: &[]*discordgo.MessageEmbed{embed}, Components: &rows})
	return nil
}

func PlexFixMissingChangeReleaseHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	sess := pfmStore.Get(i.Member.User.ID)
	if sess == nil {
		return nil
	}
	pfmStore.Touch(i.Member.User.ID)
	return pfmDisplayReleaseOptions(s, sess, sess.IsMovie)
}

func PlexFixMissingApproveHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	sess := pfmStore.Get(i.Member.User.ID)
	if sess == nil {
		return nil
	}
	pfmStore.Touch(i.Member.User.ID)
	rel := sess.SelectedRelease
	if rel == nil {
		_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Content: util.PtrString("No release selected."), Components: &[]discordgo.MessageComponent{}})
		pfmStore.Clear(sess.UserID)
		return nil
	}
	payload := map[string]any{
		"guid":      rel["guid"],
		"indexerId": rel["indexerId"],
		"title": func() string {
			if t, ok := rel["title"].(string); ok && t != "" {
				return t
			}
			if mts, ok := rel["movieTitles"].([]any); ok && len(mts) > 0 {
				if t, ok := mts[0].(string); ok {
					return t
				}
			}
			return ""
		}(),
		"protocol": rel["protocol"],
	}
	callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var out any
	var err error
	if sess.IsMovie {
		err = ctx.Radarr.Post(callCtx, "release", payload, &out)
	} else {
		err = ctx.Sonarr.Post(callCtx, "release", payload, &out)
	}
	if err != nil {
		_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Content: util.PtrString("Approve failed: " + err.Error())})
		return nil
	}
	embeds := []*discordgo.MessageEmbed{ui.PlexFixMissingDownloadStartedEmbed(payload["title"].(string))}
	_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Embeds: &embeds, Components: &[]discordgo.MessageComponent{}})
	pfmStore.Clear(sess.UserID)
	return nil
}

func PlexFixMissingAbortHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	// allow only author or admin to abort
	userID := i.Member.User.ID
	sess := pfmStore.Get(userID)
	if sess == nil {
		return nil
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: sess.MessageID, Channel: sess.ChannelID, Content: util.PtrString("Aborted."), Components: &[]discordgo.MessageComponent{}})
	pfmStore.Clear(userID)
	return nil
}
