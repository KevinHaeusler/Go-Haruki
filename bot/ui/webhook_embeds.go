package ui

import (
	"fmt"
	"log"
	"strings"

	"github.com/KevinHaeusler/go-haruki/bot/webhooks"
	"github.com/bwmarrin/discordgo"
)

func WebhookNotificationEmbed(p webhooks.NotificationPayload) *discordgo.MessageEmbed {
	log.Printf("[EMBED] Generating embed. Event: %q, Subject: %q", p.Event, p.Subject)
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
			tmdbURL := ""
			if strings.ToLower(p.Media.MediaType) == "movie" {
				tmdbURL = fmt.Sprintf("https://www.themoviedb.org/movie/%s", p.Media.TMDBID)
			} else if strings.ToLower(p.Media.MediaType) == "tv" {
				tmdbURL = fmt.Sprintf("https://www.themoviedb.org/tv/%s", p.Media.TMDBID)
			}

			if tmdbURL != "" {
				mediaDetails += fmt.Sprintf("**TMDB ID:** [%s](%s)\n", p.Media.TMDBID, tmdbURL)
			} else {
				mediaDetails += fmt.Sprintf("**TMDB ID:** %s\n", p.Media.TMDBID)
			}
		}
		if p.Media.TVDBID != "" {
			tvdbURL := fmt.Sprintf("https://www.thetvdb.com/dereferrer/series/%s", p.Media.TVDBID)
			mediaDetails += fmt.Sprintf("**TVDB ID:** [%s](%s)\n", p.Media.TVDBID, tvdbURL)
		}
		if p.Media.Status != "" {
			mediaDetails += fmt.Sprintf("**Status:** %s\n", p.Media.Status)
		}

		if mediaDetails != "" {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:  fmt.Sprintf("Media Info: %s", p.Media.MediaType),
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

		// Total Requests (if provided via payload or extras like total_requests/request_count)
		v, ok := findExtraValue(p.Extra, "total_requests", "request_count", "totalRequests")
		if !ok || strings.TrimSpace(v) == "" || v == "0" {
			if p.Request != nil && strings.TrimSpace(p.Request.RequestedByRequestCount) != "" && p.Request.RequestedByRequestCount != "0" {
				v = p.Request.RequestedByRequestCount
			} else {
				v = "—"
			}
		}
		if v != "—" {
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
	log.Printf("[EMBED] Event for color: %q", p.Event)
	eventUpper := strings.ToUpper(p.Event)
	switch {
	case eventUpper == "MEDIA_AVAILABLE" || strings.Contains(eventUpper, "NOW AVAILABLE"):
		log.Println("[EMBED] Case MEDIA_AVAILABLE")
		embed.Color = 0x2ecc71 // Green
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "✅ Media Available"}
	case eventUpper == "MEDIA_REQUESTED" || strings.Contains(eventUpper, "NEW REQUEST") || strings.Contains(eventUpper, "MEDIA REQUESTED"):
		log.Println("[EMBED] Case MEDIA_REQUESTED")
		embed.Color = 0x9c5db3 // Purple
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "📥 New Request"}
	case eventUpper == "MEDIA_PENDING" || strings.Contains(eventUpper, "PENDING APPROVAL"):
		log.Println("[EMBED] Case MEDIA_PENDING")
		embed.Color = 0xe67e22 // Orange
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "⏳ Pending Approval"}
	case eventUpper == "MEDIA_APPROVED" || strings.Contains(eventUpper, "REQUEST APPROVED"):
		log.Println("[EMBED] Case MEDIA_APPROVED")
		embed.Color = 0x2ecc71 // Green
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "✅ Request Approved"}
	case eventUpper == "MEDIA_DECLINED" || strings.Contains(eventUpper, "REQUEST DECLINED"):
		log.Println("[EMBED] Case MEDIA_DECLINED")
		embed.Color = 0xe74c3c // Red
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "❌ Request Declined"}
	case eventUpper == "MEDIA_FAILED" || strings.Contains(eventUpper, "REQUEST FAILED"):
		log.Println("[EMBED] Case MEDIA_FAILED")
		embed.Color = 0xe74c3c // Red
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "❌ Request Failed"}
	case eventUpper == "MEDIA_AUTO_APPROVED" || strings.Contains(eventUpper, "REQUEST AUTO-APPROVED"):
		log.Println("[EMBED] Case MEDIA_AUTO_APPROVED")
		embed.Color = 0x2ecc71 // Green
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "✅ Request Auto-Approved"}
	case eventUpper == "ISSUE_REPORTED" || strings.Contains(eventUpper, "ISSUE REPORTED"):
		log.Println("[EMBED] Case ISSUE_REPORTED")
		embed.Color = 0xe67e22 // Orange
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "⚠️ Issue Reported"}
	case eventUpper == "ISSUE_COMMENT" || strings.Contains(eventUpper, "NEW COMMENT"):
		log.Println("[EMBED] Case ISSUE_COMMENT")
		embed.Color = 0x3498db // Blue
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "💬 New Comment"}
	case eventUpper == "ISSUE_RESOLVED" || strings.Contains(eventUpper, "ISSUE RESOLVED"):
		log.Println("[EMBED] Case ISSUE_RESOLVED")
		embed.Color = 0x2ecc71 // Green
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "✅ Issue Resolved"}
	case eventUpper == "ISSUE_REOPENED" || strings.Contains(eventUpper, "ISSUE REOPENED"):
		log.Println("[EMBED] Case ISSUE_REOPENED")
		embed.Color = 0xe67e22 // Orange
		embed.Author = &discordgo.MessageEmbedAuthor{Name: "⚠️ Issue Reopened"}
	default:
		log.Printf("[EMBED] Unhandled event: %q, keeping default color", p.Event)
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
	upper := strings.ToUpper(strings.TrimSpace(e))
	switch {
	case upper == "MEDIA_AVAILABLE" || strings.Contains(upper, "NOW AVAILABLE"):
		return "Available"
	case upper == "MEDIA_REQUESTED" || strings.Contains(upper, "NEW REQUEST") || strings.Contains(upper, "MEDIA REQUESTED"):
		return "Requested"
	case upper == "MEDIA_PENDING" || strings.Contains(upper, "PENDING APPROVAL"):
		return "Pending"
	case upper == "MEDIA_APPROVED" || strings.Contains(upper, "REQUEST APPROVED"):
		return "Approved"
	case upper == "MEDIA_DECLINED" || strings.Contains(upper, "REQUEST DECLINED"):
		return "Declined"
	case upper == "MEDIA_FAILED" || strings.Contains(upper, "REQUEST FAILED"):
		return "Failed"
	case upper == "MEDIA_AUTO_APPROVED" || strings.Contains(upper, "REQUEST AUTO-APPROVED"):
		return "Auto-Approved"
	case upper == "ISSUE_REPORTED" || strings.Contains(upper, "ISSUE REPORTED"):
		return "Reported"
	case upper == "ISSUE_COMMENT" || strings.Contains(upper, "NEW COMMENT"):
		return "Comment"
	case upper == "ISSUE_RESOLVED" || strings.Contains(upper, "ISSUE RESOLVED"):
		return "Resolved"
	case upper == "ISSUE_REOPENED" || strings.Contains(upper, "ISSUE REOPENED"):
		return "Reopened"
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
