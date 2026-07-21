package interceptor

// PlatformBrowserProfile is a common data structure extracted by the interceptor
// when a user browses a platform page. Platform adapters translate their
// scraper-specific payloads into this shared shape for persistence.
type PlatformBrowserProfile struct {
	PlatformId        string      `json:"platform_id"`
	PlatformName      string      `json:"platform_name"`
	ContentExternalId string      `json:"content_external_id"`
	ContentType       string      `json:"content_type"`
	ContentTitle      string      `json:"content_title"`
	ContentURL        string      `json:"content_url"`
	ContentSourceURL  string      `json:"content_source_url"`
	ContentCoverURL   string      `json:"content_cover_url"`
	AccountExternalId string      `json:"account_external_id"`
	AccountUsername   string      `json:"account_username"`
	AccountNickname   string      `json:"account_nickname"`
	AccountAvatarURL  string      `json:"account_avatar_url"`
	Raw               interface{} `json:"raw"`
}
