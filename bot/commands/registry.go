package commands

import (
	"fmt"

	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/bwmarrin/discordgo"
)

type Handler func(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error
type ComponentHandler func(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error

var Definitions = []*discordgo.ApplicationCommand{
	HelpCommand,
	PingCommand,
	PlexRequestCommand,
	JellyLinkCommand,
	PlexActivityCommand,
	PlexFixMissingCommand,
}

var Handlers = map[string]Handler{
	HelpCommand.Name:           HelpHandler,
	PingCommand.Name:           PingHandler,
	PlexRequestCommand.Name:    PlexRequestHandler,
	JellyLinkCommand.Name:      JellyLinkHandler,
	PlexActivityCommand.Name:   PlexActivityHandler,
	PlexFixMissingCommand.Name: PlexFixMissingHandler,
}

// ComponentHandlers CustomID -> handler
var ComponentHandlers = map[string]ComponentHandler{
	PlexRequestSelectID:         PlexRequestSelectHandler,
	PlexRequestConfirmID:        PlexRequestConfirmHandler,
	PlexRequestAbortID:          PlexRequestAbortHandler,
	PlexRequestNotifyID:         PlexRequestNotifyHandler,
	JellyLinkSelectID:           JellyLinkSelectHandler,
	JellyLinkAbortID:            JellyLinkAbortHandler,
	JellyLinkPrevID:             JellyLinkPrevHandler,
	JellyLinkNextID:             JellyLinkNextHandler,
	PlexFixMissingPageNext:      PlexFixMissingMediaPagingHandler,
	PlexFixMissingPagePrev:      PlexFixMissingMediaPagingHandler,
	PlexFixMissingSelectMedia:   PlexFixMissingMediaSelectHandler,
	PlexFixMissingSelectSeason:  PlexFixMissingSeasonSelectHandler,
	PlexFixMissingEpPageNext:    PlexFixMissingEpisodePagingHandler,
	PlexFixMissingEpPagePrev:    PlexFixMissingEpisodePagingHandler,
	PlexFixMissingSelectEpisode: PlexFixMissingEpisodeSelectHandler,
	PlexFixMissingSelectRelease: PlexFixMissingReleaseSelectHandler,
	PlexFixMissingChangeRelease: PlexFixMissingChangeReleaseHandler,
	PlexFixMissingApprove:       PlexFixMissingApproveHandler,
	PlexFixMissingAbort:         PlexFixMissingAbortHandler,
}

func RegisterAll(s *discordgo.Session, guildID string) error {
	appID := s.State.User.ID
	_, err := s.ApplicationCommandBulkOverwrite(appID, guildID, Definitions)
	if err != nil {
		scope := "global"
		if guildID != "" {
			scope = "guild " + guildID
		}
		return fmt.Errorf("bulk overwrite commands (%s): %w", scope, err)
	}
	return nil
}
