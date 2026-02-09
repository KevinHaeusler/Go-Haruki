package ui

import (
	"fmt"
	"strconv"

	"github.com/KevinHaeusler/go-haruki/bot/clients/jellyseerr"
	"github.com/bwmarrin/discordgo"
)

func ResultsSelect(customID string, results []jellyseerr.MediaSummary, selectedID int) discordgo.MessageComponent {
	opts := make([]discordgo.SelectMenuOption, 0, min(25, len(results)))

	for _, m := range results {
		label := m.Title
		if m.Year != "" {
			label = fmt.Sprintf("%s (%s)", m.Title, m.Year)
		}

		opts = append(opts, discordgo.SelectMenuOption{
			Label:   Truncate(label, 100),
			Value:   strconv.Itoa(m.ID),
			Default: m.ID == selectedID,
		})
		if len(opts) == 25 {
			break
		}
	}

	menu := discordgo.SelectMenu{
		CustomID:    customID,
		Placeholder: "Choose a result…",
		Options:     opts,
	}

	return discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}}
}

func ButtonsRow(buttons ...discordgo.MessageComponent) discordgo.MessageComponent {
	return discordgo.ActionsRow{Components: buttons}
}

func AbortButton(customID string) discordgo.Button {
	return discordgo.Button{Label: "Abort", Style: discordgo.DangerButton, CustomID: customID}
}

func ConfirmButton(customID string) discordgo.Button {
	return discordgo.Button{Label: "Request", Style: discordgo.SuccessButton, CustomID: customID}
}

func NotifyButton(customID string) discordgo.Button {
	return discordgo.Button{Label: "Notify Me", Style: discordgo.PrimaryButton, CustomID: customID}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
