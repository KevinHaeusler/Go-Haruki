package util

import "github.com/bwmarrin/discordgo"

// GetOptString returns the string value of a slash command option by name.
func GetOptString(i *discordgo.InteractionCreate, name string) string {
	for _, o := range i.ApplicationCommandData().Options {
		if o.Name == name {
			return o.StringValue()
		}
	}
	return ""
}

// GetOptUser returns the user object of a slash command option by name.
func GetOptUser(s *discordgo.Session, i *discordgo.InteractionCreate, name string) *discordgo.User {
	for _, o := range i.ApplicationCommandData().Options {
		if o.Name == name {
			return o.UserValue(s)
		}
	}
	return nil
}

// UserHasRole checks if the invoking member has a role ID.
func UserHasRole(i *discordgo.InteractionCreate, roleID string) bool {
	if i.Member == nil {
		return false
	}
	for _, r := range i.Member.Roles {
		if r == roleID {
			return true
		}
	}
	return false
}

// UserIsAdmin checks if the invoking member has Administrator permission.
func UserIsAdmin(i *discordgo.InteractionCreate) bool {
	if i.Member == nil {
		return false
	}
	return (i.Member.Permissions & discordgo.PermissionAdministrator) != 0
}

// RespondEphemeral sends an ephemeral interaction response.
func RespondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// PtrString returns a pointer to the given string (handy for WebhookEdit/MessageEdit fields).
func PtrString(v string) *string { return &v }
