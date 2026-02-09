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
}

var Handlers = map[string]Handler{
	HelpCommand.Name:         HelpHandler,
	PingCommand.Name:         PingHandler,
	PlexRequestCommand.Name:  PlexRequestHandler,
	JellyLinkCommand.Name:    JellyLinkHandler,
	PlexActivityCommand.Name: PlexActivityHandler,
}

// CustomID -> handler
var ComponentHandlers = map[string]ComponentHandler{
	PlexRequestSelectID:  PlexRequestSelectHandler,
	PlexRequestConfirmID: PlexRequestConfirmHandler,
	PlexRequestAbortID:   PlexRequestAbortHandler,
	PlexRequestNotifyID:  PlexRequestNotifyHandler,
	JellyLinkSelectID:    JellyLinkSelectHandler,
	JellyLinkAbortID:     JellyLinkAbortHandler,
	JellyLinkPrevID:      JellyLinkPrevHandler,
	JellyLinkNextID:      JellyLinkNextHandler,
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
