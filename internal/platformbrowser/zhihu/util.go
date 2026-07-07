package zhihu

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"wx_channel/internal/api/services"
	"wx_channel/internal/database/model"
	"wx_channel/internal/interceptor"
	platformbrowser "wx_channel/internal/platformbrowser"
	utilpkg "wx_channel/pkg/util"
)

const (
	PlatformId   = "zhihu"
	PlatformName = "知乎"
	Match        = "zhihu.com"
	ContentType  = "article"
)

var (
	zhihuPeopleAPIBase    = "https://api.zhihu.com/people/"
	zhihuPeopleHTTPClient = &http.Client{Timeout: 5 * time.Second}
)

const zhihuProfileBase = "https://www.zhihu.com/people/"

func Config() interceptor.PlatformBrowserConfig {
	return interceptor.PlatformBrowserConfig{
		PlatformId:   PlatformId,
		PlatformName: PlatformName,
		Match:        Match,
		ContentType:  ContentType,
	}
}

func FormatProfile(profile *Profile) *Profile {
	if profile == nil {
		return nil
	}
	var raw RawPayload
	if len(profile.Raw) > 0 {
		_ = json.Unmarshal(profile.Raw, &raw)
	}
	profile.PlatformId = firstValue(profile.PlatformId, PlatformId)
	profile.PlatformName = firstValue(profile.PlatformName, PlatformName)
	profile.ContentType = normalizeZhihuContentType(firstValue(profile.ContentType, raw.ZhihuContentKind))
	token := firstValue(raw.ZhihuContentToken, strings.TrimPrefix(profile.ContentExternalId, "zhihu:"+profile.ContentType+":"))
	if token != "" && !strings.HasPrefix(profile.ContentExternalId, "zhihu:") {
		profile.ContentExternalId = zhihuUnique(profile.ContentType, token)
	}
	if profile.ContentExternalId == "" && token != "" {
		profile.ContentExternalId = zhihuUnique(profile.ContentType, token)
	}
	if profile.ContentExternalId == "" {
		profile.ContentExternalId = firstValue(profile.ContentURL, profile.ContentSourceURL, profile.ContentTitle)
	}
	profile.AccountExternalId = firstValue(raw.ZhihuAuthorMemberHashID, profile.AccountExternalId, profile.AccountUsername, profile.AccountNickname)
	if profile.AccountUsername == "" {
		profile.AccountUsername = profile.AccountExternalId
	}
	return profile
}

type FormattedRecords struct {
	Account        model.Account
	AccountID      string
	ContentID      string
	Browse         services.BrowseHistoryInfo
	ZhihuRawFields RawPayload
}

func FormatRecords(profile *Profile, now int64) *FormattedRecords {
	profile = FormatProfile(profile)
	if profile == nil || profile.PlatformId == "" || profile.ContentExternalId == "" {
		return nil
	}
	var raw RawPayload
	if len(profile.Raw) > 0 {
		_ = json.Unmarshal(profile.Raw, &raw)
	}
	accountID := firstValue(profile.AccountExternalId, profile.AccountUsername)
	account := model.Account{
		PlatformId: profile.PlatformId,
		ExternalId: accountID,
		Username:   profile.AccountUsername,
		Nickname:   profile.AccountNickname,
		AvatarURL:  profile.AccountAvatarURL,
		ProfileURL: profile.AccountUsername,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	return &FormattedRecords{
		Account:        account,
		AccountID:      accountID,
		ContentID:      profile.ContentExternalId,
		ZhihuRawFields: raw,
		Browse: services.BrowseHistoryInfo{
			PlatformId:        profile.PlatformId,
			AccountExternalId: accountID,
			AccountUsername:   profile.AccountUsername,
			AccountNickname:   profile.AccountNickname,
			AccountAvatarURL:  profile.AccountAvatarURL,
			ContentType:       profile.ContentType,
			ContentTitle:      profile.ContentTitle,
			ContentURL:        profile.ContentURL,
			ContentSourceURL:  profile.ContentSourceURL,
			ContentCoverURL:   profile.ContentCoverURL,
			ExtraData: map[string]any{
				"platform_name": profile.PlatformName,
				"zhihu":         raw,
				"raw":           profile.Raw,
			},
		},
	}
}

func HandleLoaded(db *gorm.DB, recorder platformbrowser.BrowseRecorder, logger zerolog.Logger, profile *Profile) {
	profile = FormatProfile(profile)
	result := platformbrowser.RecordLoadedProfile(db, recorder, logger, profile)
	// fmt.Println("after RecordLoadedProfile", result.AccountReady)
	if result.AccountReady {
		EnrichAccountAsync(db, logger, result.AccountExternalID)
	}
}

type PeopleProfile struct {
	ID            string `json:"id"`
	URLToken      string `json:"url_token"`
	Name          string `json:"name"`
	AvatarURL     string `json:"avatar_url"`
	FollowerCount int64  `json:"follower_count"`
}

func EnrichAccountAsync(db *gorm.DB, logger zerolog.Logger, accountID string) {
	accountID = strings.TrimSpace(accountID)
	if db == nil || accountID == "" {
		fmt.Println("the db not existing or accountId not existing")
		return
	}
	go func() {
		if err := EnrichAccount(db, zhihuPeopleHTTPClient, accountID); err != nil {
			logger.Warn().Err(err).Str("account_external_id", accountID).Msg("enrich zhihu account failed")
		}
	}()
}

func EnrichAccount(db *gorm.DB, client *http.Client, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if accountID == "" {
		return fmt.Errorf("missing account id")
	}
	profile, err := FetchPeopleProfile(client, accountID)
	if err != nil {
		return err
	}
	updates := PeopleProfileUpdates(profile)
	if len(updates) == 0 {
		return nil
	}
	now := utilpkg.NowMillis()
	updates["updated_at"] = now
	if err := db.Model(&model.Account{}).
		Where("platform_id = ? AND external_id = ?", PlatformId, accountID).
		Updates(updates).Error; err != nil {
		return err
	}

	browseUpdates := BrowseHistoryPeopleProfileUpdates(profile)
	if len(browseUpdates) == 0 {
		return nil
	}
	browseUpdates["updated_at"] = now
	return db.Model(&model.BrowseHistory{}).
		Where("platform_id = ? AND account_external_id = ?", PlatformId, accountID).
		Updates(browseUpdates).Error
}

func FetchPeopleProfile(client *http.Client, accountID string) (*PeopleProfile, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("missing account id")
	}
	if client == nil {
		client = zhihuPeopleHTTPClient
	}
	req, err := http.NewRequest(http.MethodGet, zhihuPeopleAPIBase+url.PathEscape(accountID), nil)
	if err != nil {
		return nil, err
	}
	setZhihuPeopleHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("zhihu people api status %d url=%s body=%s", resp.StatusCode, req.URL.String(), strings.TrimSpace(string(body)))
	}
	var profile PeopleProfile
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func setZhihuPeopleHeaders(req *http.Request) {
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("priority", "u=0, i")
	req.Header.Set("sec-ch-ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-fetch-dest", "document")
	req.Header.Set("sec-fetch-mode", "navigate")
	req.Header.Set("sec-fetch-site", "none")
	req.Header.Set("sec-fetch-user", "?1")
	req.Header.Set("upgrade-insecure-requests", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	if cookie := strings.TrimSpace(viper.GetString("zhihu.cookie")); cookie != "" {
		req.Header.Set("cookie", cookie)
	}
}

func PeopleProfileUpdates(profile *PeopleProfile) map[string]any {
	if profile == nil {
		return nil
	}
	updates := make(map[string]any)
	if username := strings.TrimSpace(profile.URLToken); username != "" {
		updates["username"] = username
		updates["profile_url"] = zhihuProfileBase + url.PathEscape(username)
	}
	if nickname := strings.TrimSpace(profile.Name); nickname != "" {
		updates["nickname"] = nickname
	}
	if avatarURL := strings.TrimSpace(profile.AvatarURL); avatarURL != "" {
		updates["avatar_url"] = avatarURL
	}
	if profile.FollowerCount > 0 {
		updates["follower_count"] = profile.FollowerCount
	}
	return updates
}

func BrowseHistoryPeopleProfileUpdates(profile *PeopleProfile) map[string]any {
	if profile == nil {
		return nil
	}
	updates := make(map[string]any)
	if username := strings.TrimSpace(profile.URLToken); username != "" {
		updates["account_username"] = username
	}
	if nickname := strings.TrimSpace(profile.Name); nickname != "" {
		updates["account_nickname"] = nickname
	}
	if avatarURL := strings.TrimSpace(profile.AvatarURL); avatarURL != "" {
		updates["account_avatar_url"] = avatarURL
	}
	return updates
}

func normalizeZhihuContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "answer", "answers":
		return "answer"
	case "article", "articles", "post":
		return "article"
	case "video", "zvideo":
		return "video"
	case "":
		return "other"
	default:
		return strings.ToLower(strings.TrimSpace(contentType))
	}
}

func zhihuUnique(contentType string, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	return "zhihu:" + normalizeZhihuContentType(contentType) + ":" + token
}

func firstValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
