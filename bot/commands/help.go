package commands

import (
	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/bwmarrin/discordgo"
)

var HelpCommand = &discordgo.ApplicationCommand{
	Name:        "help",
	Description: "Show help information",
}

func HelpHandler(ctx *appctx.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_ = ctx // not used yet

	embed := &discordgo.MessageEmbed{
		Title:       "📘 Help",
		Description: "Available commands:",
		Fields: []*discordgo.MessageEmbedField{
			{Name: "/help", Value: "Show this help message"},
			{Name: "/ping", Value: "Check if the bot is online"},
			{Name: "/plex-request <mediaType> <media>", Value: "Search Jellyseerr for a movie or TV show"},
			{Name: "/jelly-link", Value: "Link your Discord account to a Jellyseerr user"},
		},
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}
