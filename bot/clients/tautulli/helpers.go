package tautulli

import (
	"fmt"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"strconv"
	"strings"
)

func (s Session) IsMusic() bool { return strings.EqualFold(s.MediaType, "track") }
func (s Session) IsTV() bool    { return strings.EqualFold(s.MediaType, "episode") }
func (s Session) IsMovie() bool { return strings.EqualFold(s.MediaType, "movie") }

type FieldKV struct {
	K string
	V string
}

// CardFields returns basic label/value pairs (useful if you ever want a generic renderer).
// - Music: Artist / Album / Song
// - TV: Show / Season / Episode
// - Movie: Movie
func (s Session) CardFields() []FieldKV {
	switch {
	case s.IsMusic():
		return []FieldKV{
			{K: "User", V: s.User},
			{K: "Quality", V: s.QualityLabel()},
			{K: "Artist", V: nonEmpty(s.GrandparentTitle, "—")},
			{K: "Album", V: nonEmpty(s.ParentTitle, "—")},
			{K: "Song", V: nonEmpty(s.Title, "—")},
		}
	case s.IsTV():
		return []FieldKV{
			{K: "User", V: s.User},
			{K: "Quality", V: s.QualityLabel()},
			{K: "Show", V: nonEmpty(s.GrandparentTitle, "—")},
			{K: "Season", V: nonEmpty(s.ParentTitle, "—")},
			{K: "Episode", V: nonEmpty(s.Title, "—")},
		}
	default:
		label := "Title"
		if s.IsMovie() {
			label = "Movie"
		}
		return []FieldKV{
			{K: "User", V: s.User},
			{K: "Quality", V: s.QualityLabel()},
			{K: label, V: nonEmpty(firstNonEmpty(s.Title, s.FullTitle), "—")},
		}
	}
}

// QualityLabel returns "Original" or "X Mbps 1080p" style when data is present.
func (s Session) QualityLabel() string {
	if strings.TrimSpace(s.QualityProfile) != "" && strings.EqualFold(s.QualityProfile, "Original") {
		return "Original"
	}

	mbps := bitrateToMbps(s.StreamBitrate) // stream_bitrate usually kbps as string
	res := firstNonEmpty(s.StreamVideoFullRes, s.VideoFullRes)

	parts := make([]string, 0, 2)
	if mbps != "" {
		parts = append(parts, mbps)
	}
	if res != "" {
		parts = append(parts, res)
	}

	if len(parts) == 0 && strings.TrimSpace(s.QualityProfile) != "" {
		return s.QualityProfile
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " ")
}

// DisplayTitleSubtitle builds a nice “media embed” title/subtitle pair.
// (UI calls this instead of duplicating logic)
func (s Session) DisplayTitleSubtitle() (title, subtitle string) {
	switch {
	case s.IsMusic():
		// Title: Song
		// Subtitle: Artist • Album
		title = nonEmpty(s.Title, "Now Playing")
		subtitle = joinNonEmpty(" • ", s.GrandparentTitle, s.ParentTitle)

	case s.IsTV():
		// Title: Show
		// Subtitle: Season • Episode
		title = nonEmpty(s.GrandparentTitle, "Now Watching")
		subtitle = joinNonEmpty(" • ", s.ParentTitle, s.Title)

	default:
		// Movie/other
		title = nonEmpty(firstNonEmpty(s.Title, s.FullTitle), "Now Watching")
		if strings.TrimSpace(s.LibraryName) != "" {
			subtitle = s.LibraryName
		} else {
			subtitle = cases.Title(language.English).String(strings.TrimSpace(s.MediaType))
		}
	}

	return title, subtitle
}

// BestThumbPath selects a good poster/cover image path for the session.
func (s Session) BestThumbPath() string {
	switch {
	case s.IsMusic():
		// Artist > album > track
		return firstNonEmpty(s.GrandparentThumb, s.ParentThumb, s.Thumb)
	case s.IsTV():
		// Show poster > episode thumb
		return firstNonEmpty(s.GrandparentThumb, s.Thumb)
	default:
		// Movie poster usually in thumb
		return firstNonEmpty(s.Thumb, s.GrandparentThumb, s.ParentThumb)
	}
}

/* ----------------- internal helpers ----------------- */

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func joinNonEmpty(sep string, vals ...string) string {
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, sep)
}

func bitrateToMbps(kbpsStr string) string {
	kbpsStr = strings.TrimSpace(kbpsStr)
	if kbpsStr == "" {
		return ""
	}
	kbps, err := strconv.Atoi(kbpsStr)
	if err != nil || kbps <= 0 {
		return ""
	}

	mbps := float64(kbps) / 1000.0
	if mbps == float64(int(mbps)) {
		return fmt.Sprintf("%d Mbps", int(mbps))
	}
	return fmt.Sprintf("%.1f Mbps", mbps)
}
