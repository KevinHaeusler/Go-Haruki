package ui

import (
	"fmt"
	"strings"

	"github.com/KevinHaeusler/go-haruki/bot/clients/jellyseerr"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	TMDBImageURL     = "https://image.tmdb.org/t/p/w500"
	MissingPosterURL = "https://via.placeholder.com/500x750?text=No+Poster"
)

func JellyRequestListEmbed(user *discordgo.User, requests []jellyseerr.UserRequest, page, totalPages, totalResults int) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("Overseerr Requests for %s", user.Username),
		Description: "**########### Legend ###########**\n" +
			"✅ = Available, ❔ = Unknown, ⌛ = Pending, 🔄 = Processing, ⚠️ = Partial, ❌ = Deleted\n\n" +
			"Media that is 🔄 = Processing usually needs to be downloaded manually, try `/plex-fix-missing`\n" +
			"Media that is ⚠️ = Partial usually means episodes are missing (can be running shows with future episodes)\n\n" +
			"**Status — Title — Requested Date**\n",
		Color: 0x00ADFF,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: user.AvatarURL(""),
		},
	}

	if len(requests) == 0 {
		embed.Description += "\nNo requests found."
		return embed
	}

	availMap := map[int]string{
		1: "❔",  // UNKNOWN? (Python says 1: ❔)
		2: "⌛",  // PENDING
		3: "🔄",  // PROCESSING
		4: "⚠️", // PARTIAL
		5: "✅",  // AVAILABLE
		6: "❌",  // DELETED
	}

	var sb strings.Builder
	sb.WriteString(embed.Description)

	for _, r := range requests {
		// statusEmoji is based on media status, not request status
		statusEmoji := availMap[r.Media.Status]
		if statusEmoji == "" {
			statusEmoji = fmt.Sprintf("%d", r.Media.Status)
		}

		title := r.Title
		if title == "" {
			title = r.Media.Title
		}
		if title == "" {
			title = r.Media.Name
		}
		if title == "" && len(r.Media.Requests) > 0 {
			title = r.Media.Requests[0]
		}
		if title == "" {
			title = "Unknown Title"
		}

		// Ensure title doesn't contain the year already if we're about to add it
		title = strings.TrimSpace(title)

		year := ""
		if len(r.Media.ReleaseDate) >= 4 {
			year = r.Media.ReleaseDate[:4]
		}

		displayTitle := title
		if year != "" && !strings.Contains(title, "("+year+")") {
			displayTitle = fmt.Sprintf("%s (%s)", title, year)
		}

		created := r.CreatedAt.Format("02.01.06")

		mediaType := r.Type
		if mediaType == "" {
			mediaType = r.Media.MediaType
		}
		typeLabel := ""
		if strings.EqualFold(mediaType, "movie") {
			typeLabel = "[Movie] "
		} else if strings.EqualFold(mediaType, "tv") {
			typeLabel = "[TV] "
		}

		line := fmt.Sprintf("%s — %s**%s** — %s\n", statusEmoji, typeLabel, displayTitle, created)
		if len(sb.String())+len(line) > 4000 {
			break
		}
		sb.WriteString(line)
	}

	embed.Description = sb.String()
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Page %d of %d (Total: %d)", page, totalPages, totalResults),
	}

	return embed
}

func JellyResultListEmbed(query string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "🔎 Jellyseerr Search",
		Description: fmt.Sprintf("Results for: **%s**\nSelect an item below.", query),
	}
}

func JellyDetailEmbed(d jellyseerr.MediaDetail, mediaType string) *discordgo.MessageEmbed {
	title := fmt.Sprintf("%s (%s)", d.DisplayTitle(mediaType), d.DisplayYear(mediaType))

	poster := MissingPosterURL
	if d.PosterPath != "" {
		poster = TMDBImageURL + d.PosterPath
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: Truncate(d.Overview, 4000),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: poster},
	}
}

func JellyAlreadyRequestedEmbed(d jellyseerr.MediaDetail, mediaType string) *discordgo.MessageEmbed {
	title := fmt.Sprintf("%s", d.DisplayTitle(mediaType))

	poster := MissingPosterURL
	if d.PosterPath != "" {
		poster = TMDBImageURL + d.PosterPath
	}

	requester, watchers := d.RequesterSummary()

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "Requested by",
			Value:  requester,
			Inline: true,
		},
	}

	if len(watchers) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Will be notified",
			Value:  strings.Join(watchers, "\n"),
			Inline: true,
		})
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: d.Overview,
		Color:       0x66ccff,
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: poster},
		Author:      &discordgo.MessageEmbedAuthor{Name: "🔄 Already Requested"},
		Fields:      fields,
	}
}

func JellyAvailabilityEmbed(d jellyseerr.MediaDetail, mediaType string, status int) *discordgo.MessageEmbed {
	_ = mediaType
	poster := MissingPosterURL
	if d.PosterPath != "" {
		poster = TMDBImageURL + d.PosterPath
	}

	label := "✅ Media Already Available"
	color := 0x00cc66
	if status == 4 {
		label = "⚠️ Partial Availability"
		color = 0xff9966
	}

	return &discordgo.MessageEmbed{
		Title:       label,
		Description: Truncate(d.Overview, 4000),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: poster},
		Color:       color,
	}
}

func JellyPartialAvailabilityEmbed(d jellyseerr.MediaDetail, mediaType string) *discordgo.MessageEmbed {
	poster := MissingPosterURL
	if d.PosterPath != "" {
		poster = TMDBImageURL + d.PosterPath
	}

	requester, watchers := d.RequesterSummary()

	fields := []*discordgo.MessageEmbedField{}
	if requester != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Requested by",
			Value:  requester,
			Inline: true,
		})
	}
	if len(watchers) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Will be notified",
			Value:  strings.Join(watchers, "\n"),
			Inline: true,
		})
	}

	title := fmt.Sprintf("⚠️ Partial Availability: %s", d.DisplayTitle(mediaType))
	year := d.DisplayYear(mediaType)
	if year != "" {
		title = fmt.Sprintf("⚠️ Partial Availability: %s (%s)", d.DisplayTitle(mediaType), year)
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: Truncate(d.Overview, 4000),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: poster},
		Color:       0xff9966,
		Fields:      fields,
	}
}

func JellyRequestSentEmbed(d jellyseerr.MediaDetail, mediaType, requester string, totalRequests int) *discordgo.MessageEmbed {
	poster := MissingPosterURL
	if d.PosterPath != "" {
		poster = TMDBImageURL + d.PosterPath
	}

	title := fmt.Sprintf("%s (%s)", d.DisplayTitle(mediaType), d.DisplayYear(mediaType))

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: Truncate(d.Overview, 4000),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: poster},
		Color:       0x9c5db3,
		Author:      &discordgo.MessageEmbedAuthor{Name: fmt.Sprintf("%s Request Sent", cases.Title(language.English).String(mediaType))},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Requested By", Value: requester, Inline: true},
			{Name: "Request Status", Value: "Processing", Inline: true},
			{Name: "Total Requests", Value: fmt.Sprintf("%d", totalRequests), Inline: true},
		},
	}
}
