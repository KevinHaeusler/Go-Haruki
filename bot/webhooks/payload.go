package webhooks

type NotificationPayload struct {
	NotificationType string       `json:"notification_type"`
	Event            string       `json:"event"`
	Subject          string       `json:"subject"`
	Message          string       `json:"message"`
	Image            string       `json:"image"`
	Media            *MediaInfo   `json:"media,omitempty"`
	Request          *RequestInfo `json:"request,omitempty"`
	Issue            *IssueInfo   `json:"issue,omitempty"`
	Comment          *CommentInfo `json:"comment,omitempty"`
	Extra            []ExtraInfo  `json:"extra"`
	DiscordChannelID string       `json:"discord_channel_id,omitempty"`
}

type MediaInfo struct {
	MediaType string `json:"media_type"`
	TMDBID    string `json:"tmdbId"`
	TVDBID    string `json:"tvdbId"`
	Status    string `json:"status"`
	Status4k  string `json:"status4k"`
}

type RequestInfo struct {
	RequestID                     string `json:"request_id"`
	RequestedByEmail              string `json:"requestedBy_email"`
	RequestedByUsername           string `json:"requestedBy_username"`
	RequestedByAvatar             string `json:"requestedBy_avatar"`
	RequestedByID                 string `json:"requestedBy_id"`
	RequestedByRequestCount       string `json:"requestedBy_requestCount"`
	RequestedBySettingsDiscordID  string `json:"requestedBy_settings_discordId"`
	RequestedBySettingsTelegramID string `json:"requestedBy_settings_telegramChatId"`
}

type IssueInfo struct {
	IssueID                      string `json:"issue_id"`
	IssueType                    string `json:"issue_type"`
	IssueStatus                  string `json:"issue_status"`
	ReportedByEmail              string `json:"reportedBy_email"`
	ReportedByUsername           string `json:"reportedBy_username"`
	ReportedByAvatar             string `json:"reportedBy_avatar"`
	ReportedByID                 string `json:"reportedBy_id"`
	ReportedByRequestCount       string `json:"reportedBy_requestCount"`
	ReportedBySettingsDiscordID  string `json:"reportedBy_settings_discordId"`
	ReportedBySettingsTelegramID string `json:"reportedBy_settings_telegramChatId"`
}

type CommentInfo struct {
	CommentMessage                string `json:"comment_message"`
	CommentedByEmail              string `json:"commentedBy_email"`
	CommentedByUsername           string `json:"commentedBy_username"`
	CommentedByAvatar             string `json:"commentedBy_avatar"`
	CommentedByID                 string `json:"commentedBy_id"`
	CommentedByRequestCount       string `json:"commentedBy_requestCount"`
	CommentedBySettingsDiscordID  string `json:"commentedBy_settings_discordId"`
	CommentedBySettingsTelegramID string `json:"commentedBy_settings_telegramChatId"`
}

type ExtraInfo struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
