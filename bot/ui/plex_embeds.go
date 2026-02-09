package ui

import (
	"strings"

	"github.com/KevinHaeusler/go-haruki/bot/clients/tautulli"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	plexColorMusic    = 0x3498db
	plexColorTV       = 0x2ecc71
	plexColorMovie    = 0xe74c3c
	plexColorOther    = 0x95a5a6
	TransparentBanner = "https://placehold.co/1000x1/transparent/F00/png"
)

func PlexActivityMediaEmbeds(tc *tautulli.Client, sessions []tautulli.Session) []*discordgo.MessageEmbed {
	out := make([]*discordgo.MessageEmbed, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, PlexSessionMediaEmbed(tc, sess))
	}
	return out
}

func PlexSessionMediaEmbed(tc *tautulli.Client, s tautulli.Session) *discordgo.MessageEmbed {
	title, subtitle := s.DisplayTitleSubtitle()
	if strings.TrimSpace(subtitle) == "" {
		subtitle = "\u200b"
	}

	thumbURL := ""
	if tc != nil {
		if p := s.BestThumbPath(); strings.TrimSpace(p) != "" {
			thumbURL = tc.ImageProxyURL(p, 300)
		}
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "User", Value: nonEmptyAny(s.User, s.FriendlyName), Inline: true},
		{Name: "Quality", Value: s.QualityLabel(), Inline: true},
	}

	switch {
	case s.IsMusic():
		fields = append(fields,
			&discordgo.MessageEmbedField{Name: "Artist", Value: nonEmptyAny(s.GrandparentTitle), Inline: false},
			&discordgo.MessageEmbedField{Name: "Album", Value: nonEmptyAny(s.ParentTitle), Inline: false},
			&discordgo.MessageEmbedField{Name: "Song", Value: nonEmptyAny(s.Title), Inline: false},
		)
	case s.IsTV():
		fields = append(fields,
			&discordgo.MessageEmbedField{Name: "Show", Value: nonEmptyAny(s.GrandparentTitle), Inline: false},
			&discordgo.MessageEmbedField{Name: "Season", Value: nonEmptyAny(s.ParentTitle), Inline: false},
			&discordgo.MessageEmbedField{Name: "Episode", Value: nonEmptyAny(s.Title), Inline: false},
		)
	default:
		label := "Title"
		if s.IsMovie() {
			label = "Movie"
		}
		fields = append(fields,
			&discordgo.MessageEmbedField{Name: label, Value: nonEmptyAny(s.Title, s.FullTitle), Inline: false},
		)
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: subtitle,
		Color:       plexColor(s),
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: plexFooter(s),
		},
	}

	if thumbURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: thumbURL}
	}

	bannerURL := TransparentBanner
	if tc != nil && strings.TrimSpace(s.Art) != "" {
		bannerURL = tc.ImageProxyURL(strings.TrimSpace(s.Art), 900)
	}
	embed.Image = &discordgo.MessageEmbedImage{URL: bannerURL}

	return embed
}

func plexColor(s tautulli.Session) int {
	switch {
	case s.IsMusic():
		return plexColorMusic
	case s.IsTV():
		return plexColorTV
	case s.IsMovie():
		return plexColorMovie
	default:
		return plexColorOther
	}
}

func plexFooter(s tautulli.Session) string {
	var parts []string

	if strings.TrimSpace(s.Player) != "" {
		parts = append(parts, s.Player)
	}

	if strings.TrimSpace(s.Product) != "" {
		parts = append(parts, s.Product)
	} else if strings.TrimSpace(s.Platform) != "" {
		parts = append(parts, s.Platform)
	}

	if strings.TrimSpace(s.State) != "" {
		parts = append(parts, cases.Title(language.English).String(s.State))
	}

	return strings.Join(parts, " • ")
}

func nonEmptyAny(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "—"
}
