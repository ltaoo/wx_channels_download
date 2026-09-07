package instagramadapter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/instagram"
	"wx_channel/pkg/util"
)

func result_from_fetch(data any) (*instagram.FetchResult, error) {
	var result *instagram.FetchResult
	switch value := data.(type) {
	case *instagram.FetchResult:
		result = value
	case instagram.FetchResult:
		result = &value
	case []byte:
		return result_from_fetch(json.RawMessage(value))
	case json.RawMessage:
		if err := json.Unmarshal(value, &result); err != nil {
			return nil, fmt.Errorf("instagram: decode fetch result: %w", err)
		}
	default:
		return nil, fmt.Errorf("instagram: unsupported fetch data %T", data)
	}
	if result == nil || strings.TrimSpace(result.ExternalID) == "" || result.Account == nil || strings.TrimSpace(result.Account.ExternalID) == "" || len(result.Media) == 0 {
		return nil, fmt.Errorf("instagram: incomplete post, account, or media data")
	}
	if _, err := instagram.ExtractShortcode(result.SourceURL); err != nil {
		return nil, err
	}
	if _, err := instagram.ExtractUsername(result.Account.Username); err != nil {
		return nil, err
	}
	seen_ids := make(map[string]bool)
	for _, media := range result.Media {
		if strings.TrimSpace(media.ID) == "" || seen_ids[media.ID] || (media.Type != "image" && media.Type != "video") || instagram.NormalizeMediaURL(media.URL) == "" {
			return nil, fmt.Errorf("instagram: invalid or duplicate media %q", media.ID)
		}
		seen_ids[media.ID] = true
	}
	return result, nil
}

func content_title(body_text string, username string, external_id string) string {
	title := []rune(strings.TrimSpace(body_text))
	if len(title) > 80 {
		title = append(title[:80], '…')
	}
	if len(title) == 0 {
		title = []rune(username + "_" + external_id)
	}
	return string(title)
}

func to_content(result *instagram.FetchResult) *model.Content {
	media := result.Media[0]
	content_type, subtype := model.ContentTypeAlbum, model.ContentSubtypePhotoAlbum
	content_url := result.SourceURL
	if len(result.Media) == 1 && media.Type == "video" {
		content_type, subtype, content_url = model.ContentTypeVideo, model.ContentSubtypeShortVideo, media.URL
	}
	metadata, _ := json.Marshal(map[string]any{"shortcode": result.Shortcode, "media": result.Media})
	now := util.NowMillis()
	content := &model.Content{Id: PlatformID + ":" + result.ExternalID, PlatformId: PlatformID, Type: content_type, Subtype: subtype,
		ExternalId: result.ExternalID, ExternalId2: result.Shortcode, Title: content_title(result.BodyText, result.Account.Username, result.ExternalID), Description: result.BodyText,
		URL: content_url, SourceURL: result.SourceURL, CoverURL: instagram.NormalizeMediaURL(media.CoverURL),
		CoverWidth: strconv.FormatInt(media.Width, 10), CoverHeight: strconv.FormatInt(media.Height, 10),
		LikeCount: result.LikeCount, CommentCount: result.CommentCount, Metadata: string(metadata), Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now}}
	if content.CoverURL == "" && media.Type == "image" {
		content.CoverURL = media.URL
	}
	if result.PublishTime > 0 {
		publish_time := result.PublishTime
		content.PublishTime = &publish_time
	}
	if result.Account.IsPrivate {
		content.IsPrivate = 1
	}
	return content
}

func to_account(account *instagram.Account) *model.Account {
	now := util.NowMillis()
	return &model.Account{Id: PlatformID + ":" + account.ExternalID, PlatformId: PlatformID, ExternalId: account.ExternalID,
		Alias: account.Username, Nickname: account.Nickname, Signature: account.Biography,
		AvatarURL: instagram.NormalizeMediaURL(account.AvatarURL), ProfileURL: "https://www.instagram.com/" + account.Username + "/",
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now}}
}

func to_details(result *instagram.FetchResult, content *model.Content, account *model.Account) []adapter.ContentDetail {
	accounts := []adapter.ContentAccountReference{{Account: account, Role: "owner"}}
	if content.Type == model.ContentTypeVideo {
		return []adapter.ContentDetail{{Type: content.Type, Key: content.Id, Content: content, Accounts: accounts, Data: to_video(content.Id, result.Media[0])}}
	}
	album := &model.ContentAlbum{Id: content.Id, Description: result.BodyText}
	details := []adapter.ContentDetail{{Type: content.Type, Key: content.Id, Content: content, Accounts: accounts, Data: album}}
	for media_index, media := range result.Media {
		if media.Type == "image" {
			album.Images = append(album.Images, model.ContentImage{AlbumId: content.Id,
				ImageKey: model.BuildContentAlbumImageKey(media.ID, media.URL, media_index), SortOrder: media_index,
				URL: media.URL, Width: int(media.Width), Height: int(media.Height), ImageType: model.ContentImageTypeStill})
			continue
		}
		video_content := &model.Content{Id: video_content_id(result, media), PlatformId: PlatformID,
			Type: model.ContentTypeVideo, Subtype: model.ContentSubtypeShortVideo, ExternalId: media.ID,
			Title: fmt.Sprintf("%s_%02d", content.Title, media_index+1), Description: media.AltText,
			URL: media.URL, SourceURL: result.SourceURL, CoverURL: instagram.NormalizeMediaURL(media.CoverURL),
			CoverWidth: strconv.FormatInt(media.Width, 10), CoverHeight: strconv.FormatInt(media.Height, 10),
			PublishTime: content.PublishTime, IsPrivate: content.IsPrivate, Timestamps: content.Timestamps}
		details = append(details, adapter.ContentDetail{Type: model.ContentTypeVideo, Key: video_content.Id, Content: video_content,
			Data: to_video(video_content.Id, media), Accounts: accounts,
			Relation: &model.ContentRelation{SourceContentId: content.Id, TargetContentId: video_content.Id,
				Type: model.ContentRelationContains, SortOrder: media_index, CreatedAt: content.CreatedAt}})
	}
	album.ImageCount = len(album.Images)
	album.CoverWidth, album.CoverHeight = int(result.Media[0].Width), int(result.Media[0].Height)
	return details
}

func video_content_id(result *instagram.FetchResult, media instagram.Media) string {
	return PlatformID + ":" + result.ExternalID + ":video:" + media.ID
}

func to_video(content_id string, media instagram.Media) *model.ContentVideo {
	width, height := int(media.Width), int(media.Height)
	return &model.ContentVideo{Id: content_id, URL: media.URL, Width: width, Height: height, Format: "mp4",
		Variants: []model.ContentVideoVariant{{VideoId: content_id, VariantKey: "default", URL: media.URL,
			Width: &width, Height: &height, Format: "mp4", StreamType: model.ContentVideoVariantStreamTypeProgressive, HasVideo: 1, HasAudio: 1, IsDefault: 1}}}
}
