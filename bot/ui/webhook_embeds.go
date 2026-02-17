package ui

import (
	"fmt"
	"strings"

	"github.com/KevinHaeusler/go-haruki/bot/webhooks"
	"github.com/bwmarrin/discordgo"
)

func WebhookNotificationEmbed(p webhooks.NotificationPayload) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       p.Subject,
		Description: p.Message,
		Color:       0x00ADFF, // Light blue default
	}

	if p.Image != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: p.Image,
		}
	}

	// Add media info if available
	if p.Media != nil {
		var mediaDetails string
		if p.Media.TMDBID != "" {
			mediaDetails += fmt.Sprintf("**TMDB ID:** %s\n", p.Media.TMDBID)
		}
		if p.Media.TVDBID != "" {
			mediaDetails += fmt.Sprintf("**TVDB ID:** %s\n", p.Media.TVDBID)
		}
		if p.Media.Status != "" {
			mediaDetails += fmt.Sprintf("**Status:** %s\n", p.Media.Status)
		}

		if mediaDetails != "" {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:  fmt.Sprintf("Media Info (%s)", p.Media.MediaType),
				Value: mediaDetails,
			})
		}
	}

	// Add request/issue info and requested-by/request-status/total-requests like the request embed
	if p.Request != nil {
		// Requested By
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Requested By",
			Value:  nonEmpty(p.Request.RequestedByUsername, p.Request.RequestedByEmail, "—"),
			Inline: true,
		})

		// Request Status (prefer media status if present, else use event label)
		status := "—"
		if p.Media != nil && strings.TrimSpace(p.Media.Status) != "" {
			status = p.Media.Status
		} else if strings.TrimSpace(p.Event) != "" {
			status = prettifyEvent(p.Event)
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Request Status",
			Value:  status,
			Inline: true,
		})

		// Total Requests (if provided via extras like total_requests/request_count)
		if v, ok := findExtraValue(p.Extra, "total_requests", "request_count", "totalRequests"); ok && strings.TrimSpace(v) != "" {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "Total Requests",
				Value:  v,
				Inline: true,
			})
		}
	} else if p.Issue != nil {
		// Issue flow keeps the reporter info
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Reported By",
			Value:  nonEmpty(p.Issue.ReportedByUsername, p.Issue.ReportedByEmail, "—"),
			Inline: true,
		})
	}

	// Set color based on event type
	switch p.Event {
	case "MEDIA_AVAILABLE":
		embed.Color = 0x2ecc71 // Green
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "✅ Media Available"}
	case "MEDIA_REQUESTED":
		embed.Color = 0x9c5db3 // Purple
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "📥 New Request"}
	case "ISSUE_REPORTED":
		embed.Color = 0xff9966 // Orange
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "⚠️ Issue Reported"}
	case "ISSUE_COMMENT":
		embed.Color = 0x66ccff // Light Blue
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "💬 New Comment"}
	}

	// Plex link support in footer (if provided via extras as plex_url/plex_link)
	if link, ok := findExtraValue(p.Extra, "plex_url", "plex_link", "plex"); ok && strings.TrimSpace(link) != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Plex: %s", link)}
		// Also make the embed clickable
		embed.URL = link
	}

	return embed
}

// helpers
func nonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "—"
}

func prettifyEvent(e string) string {
	switch strings.ToUpper(strings.TrimSpace(e)) {
	case "MEDIA_AVAILABLE":
		return "Available"
	case "MEDIA_REQUESTED":
		return "Requested"
	case "MEDIA_APPROVED":
		return "Approved"
	case "MEDIA_DECLINED":
		return "Declined"
	default:
		return e
	}
}

func findExtraValue(extras []webhooks.ExtraInfo, keys ...string) (string, bool) {
	if len(extras) == 0 {
		return "", false
	}
	for _, ex := range extras {
		name := strings.ToLower(strings.TrimSpace(ex.Name))
		for _, k := range keys {
			if name == strings.ToLower(strings.TrimSpace(k)) {
				return ex.Value, true
			}
		}
	}
	return "", false
}
