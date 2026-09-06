package xiaohongshuadapter

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/xiaohongshu"
	"wx_channel/pkg/util"
)

var xiaohongshu_home_tabs = []adapter.HomeContentTab{
	{Scope: "notes", Name: "笔记", ContentTypes: []string{model.ContentTypePost, model.ContentTypeVideo, model.ContentTypeAlbum}},
	{Scope: "collections", Name: "收藏", ContentTypes: []string{model.ContentTypePost, model.ContentTypeVideo, model.ContentTypeAlbum}},
}

func (a *XiaohongshuAdapter) HomeContentTabs(_ *model.Account) []adapter.HomeContentTab {
	return append([]adapter.HomeContentTab(nil), xiaohongshu_home_tabs...)
}

func (a *XiaohongshuAdapter) BuildHomeContents(account *model.Account, scope string) (*adapter.HomeContents, error) {
	return a.build_home_contents_page(account, scope, "")
}

func (a *XiaohongshuAdapter) build_home_contents_page(account *model.Account, scope string, page_marker string) (*adapter.HomeContents, error) {
	profile_url, err := xiaohongshu_profile_url(account)
	if err != nil {
		return nil, err
	}
	scope = strings.TrimSpace(scope)
	if scope != "notes" && scope != "collections" {
		return nil, fmt.Errorf("小红书不支持主页 scope: %s", scope)
	}
	client := a.new_scraper_client()
	defer client.Close()
	home_result, err := client.FetchHomeContentsPage(profile_url, scope, page_marker)
	if err != nil {
		return nil, fmt.Errorf("获取小红书主页%s失败: %w", xiaohongshu_home_scope_name(scope), err)
	}
	return &adapter.HomeContents{
		Scope:      scope,
		Contents:   xiaohongshu_home_contents(home_result.Items, home_result.Source),
		NextMarker: home_result.NextMarker,
	}, nil
}

func xiaohongshu_home_contents(note_items []xiaohongshu.HomeNoteItem, profile_url string) []model.Content {
	contents := make([]model.Content, 0, len(note_items))
	seen := make(map[string]struct{}, len(note_items))
	now := util.NowMillis()
	for _, note_item := range note_items {
		content, ok := xiaohongshu_home_note_to_content(note_item, profile_url, now)
		if !ok {
			continue
		}
		if _, exists := seen[content.Id]; exists {
			continue
		}
		seen[content.Id] = struct{}{}
		contents = append(contents, content)
	}
	return contents
}

func xiaohongshu_home_note_to_content(note_item xiaohongshu.HomeNoteItem, profile_url string, now int64) (model.Content, bool) {
	note_id := first_non_empty(note_item.NoteCard.NoteID, note_item.ID)
	note_type := strings.ToLower(strings.TrimSpace(note_item.NoteCard.Type))
	content_type := model.ContentTypeAlbum
	content_subtype := model.ContentSubtypePhotoAlbum
	if note_type == "video" {
		content_type = model.ContentTypeVideo
		content_subtype = model.ContentSubtypeShortVideo
	}
	title := strings.TrimSpace(note_item.NoteCard.DisplayTitle)
	if title == "" {
		title = "小红书笔记"
	}
	redacted := note_id == ""
	if redacted {
		note_id = xiaohongshu_redacted_note_id(note_item)
		if note_id == "" {
			return model.Content{}, false
		}
	} else if title == "小红书笔记" {
		title += "_" + note_id
	}
	xsec_token := first_non_empty(note_item.NoteCard.XSecToken, note_item.XSecToken)
	source_url := first_non_empty(note_item.URL, profile_url)
	if !redacted {
		source_url = first_non_empty(note_item.URL, xiaohongshu_home_note_url(note_id, xsec_token))
	}
	cover_url := xiaohongshu.ImageURL(note_item.NoteCard.Cover)
	metadata_json, _ := json.Marshal(note_item.NoteCard)

	var publish_time *int64
	if note_item.NoteCard.Time > 0 {
		value := note_item.NoteCard.Time
		publish_time = &value
	}
	return model.Content{
		Id:          BuildContentID(note_id),
		PlatformId:  PlatformID,
		Type:        content_type,
		Subtype:     content_subtype,
		ExternalId:  note_id,
		Title:       title,
		Description: title,
		URL:         source_url,
		SourceURL:   source_url,
		CoverURL:    cover_url,
		CoverWidth:  positive_int_string(note_item.NoteCard.Cover.Width),
		CoverHeight: positive_int_string(note_item.NoteCard.Cover.Height),
		PublishTime: publish_time,
		LikeCount:   int64(note_item.NoteCard.InteractInfo.LikedCount),
		Metadata:    string(metadata_json),
		Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, true
}

func xiaohongshu_redacted_note_id(note_item xiaohongshu.HomeNoteItem) string {
	card := note_item.NoteCard
	cover_identity := xiaohongshu_cover_identity(card.Cover)
	if strings.TrimSpace(card.DisplayTitle) == "" && card.Time <= 0 && cover_identity == "" {
		return ""
	}
	identity := strings.Join([]string{
		strings.TrimSpace(card.User.UserID),
		strconv.FormatInt(card.Time, 10),
		strings.TrimSpace(card.Type),
		strings.TrimSpace(card.DisplayTitle),
		cover_identity,
		strconv.Itoa(note_item.Index),
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return "redacted-" + fmt.Sprintf("%x", digest[:])
}

func xiaohongshu_cover_identity(cover xiaohongshu.Image) string {
	cover_url := xiaohongshu.ImageURL(cover)
	parsed_url, err := url.Parse(cover_url)
	if err != nil || parsed_url == nil {
		return cover_url
	}
	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	if len(path_parts) == 0 {
		return cover_url
	}
	file_name := path_parts[len(path_parts)-1]
	if separator_index := strings.Index(file_name, "!"); separator_index >= 0 {
		file_name = file_name[:separator_index]
	}
	return strings.TrimSpace(file_name)
}

func xiaohongshu_home_note_url(note_id string, xsec_token string) string {
	note_url := &url.URL{
		Scheme: "https",
		Host:   "www.xiaohongshu.com",
		Path:   "/explore/" + strings.TrimSpace(note_id),
	}
	if xsec_token = strings.TrimSpace(xsec_token); xsec_token != "" {
		query := note_url.Query()
		query.Set("xsec_source", "pc_user")
		query.Set("xsec_token", xsec_token)
		note_url.RawQuery = query.Encode()
	}
	return note_url.String()
}

type xiaohongshu_home_contents_builder interface {
	build_home_contents_page(account *model.Account, scope string, page_marker string) (*adapter.HomeContents, error)
}

// FetchHomeDetails fetches the selected Xiaohongshu profile tab without
// persisting its contents. Notes are the default profile feed.
func (a *XiaohongshuAdapter) FetchHomeDetails(account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	return fetch_xiaohongshu_home_details(a, account, scope, page_marker)
}

func fetch_xiaohongshu_home_details(builder xiaohongshu_home_contents_builder, account *model.Account, scope string, page_marker string) (*adapter.HomeDetails, error) {
	if builder == nil {
		return nil, fmt.Errorf("小红书主页内容读取器不能为空")
	}
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "feed" {
		scope = "notes"
	}
	if scope != "notes" && scope != "collections" {
		return nil, fmt.Errorf("小红书不支持主页详情 scope: %s", scope)
	}

	home_contents, err := builder.build_home_contents_page(account, scope, page_marker)
	if err != nil {
		return nil, err
	}
	if home_contents == nil {
		return nil, fmt.Errorf("小红书主页%s列表为空", xiaohongshu_home_scope_name(scope))
	}
	scopes := make([]adapter.HomeDetailsScope, 0, len(xiaohongshu_home_tabs))
	for _, tab := range xiaohongshu_home_tabs {
		scopes = append(scopes, adapter.HomeDetailsScope{Label: tab.Name, Value: tab.Scope})
	}
	contents := home_contents.Contents
	if contents == nil {
		contents = make([]model.Content, 0)
	}
	return &adapter.HomeDetails{Scopes: scopes, Scope: scope, Contents: contents, NextMarker: home_contents.NextMarker}, nil
}

func xiaohongshu_home_scope_name(scope string) string {
	for _, tab := range xiaohongshu_home_tabs {
		if tab.Scope == scope {
			return tab.Name
		}
	}
	return scope
}

func xiaohongshu_profile_url(account *model.Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("小红书账号不能为空")
	}
	if raw_url := strings.TrimSpace(account.ProfileURL); raw_url != "" {
		parsed_url, err := url.Parse(raw_url)
		if err == nil && strings.HasSuffix(strings.ToLower(parsed_url.Hostname()), "xiaohongshu.com") {
			return parsed_url.String(), nil
		}
	}
	external_id := strings.TrimSpace(account.ExternalId)
	if external_id == "" {
		return "", fmt.Errorf("小红书账号 external_id 不能为空")
	}
	return "https://www.xiaohongshu.com/user/profile/" + url.PathEscape(external_id), nil
}
