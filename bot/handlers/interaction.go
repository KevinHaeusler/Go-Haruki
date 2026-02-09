package handlers

import (
	"log"

	"github.com/bwmarrin/discordgo"

	"github.com/KevinHaeusler/go-haruki/bot/appctx"
	"github.com/KevinHaeusler/go-haruki/bot/commands"
)

func NewInteractionHandler(ctx *appctx.Context) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {

		case discordgo.InteractionApplicationCommand:
			name := i.ApplicationCommandData().Name
			h, ok := commands.Handlers[name]
			if !ok {
				log.Println("No slash handler for:", name)
				return
			}
			if err := h(ctx, s, i); err != nil {
				log.Println("Slash handler error:", name, err)
			}

		case discordgo.InteractionMessageComponent:
			cd := i.MessageComponentData()
			customID := cd.CustomID
			h, ok := commands.ComponentHandlers[customID]
			if !ok {
				log.Println("No component handler for:", customID)
				return
			}
			if err := h(ctx, s, i); err != nil {
				log.Println("Component handler error:", customID, err)
			}

		default:
			// ignore
		}
	}
}
