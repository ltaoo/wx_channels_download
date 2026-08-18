package ttkadapter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/ttk"
	"wx_channel/pkg/util"
)

const (
	ttk_official_account_name = "ttk"
	ttk_official_profile_url  = "https://ttks.tw/"
)

// NovelModelSet contains all database models derived from a TTK novel.
type NovelModelSet struct {
	Content  *model.Content
	Novel    *model.ContentNovel
	Chapters []model.ContentNovelChapter
	Account  *model.Account
}

// BuildContentID builds a content identifier from a TTK book ID.
func BuildContentID(book_id string) string {
	return PlatformID + ":" + strings.TrimSpace(book_id)
}

// BuildAccountID builds an account identifier from an author name.
func BuildAccountID(author_name string) string {
	return PlatformID + ":" + strings.TrimSpace(author_name)
}

// ToContent converts a TTK fetch result into the common content model.
func ToContent(result *ttk.TtkFetchResult) (*model.Content, error) {
	result, err := validate_ttk_result(result)
	if err != nil {
		return nil, err
	}
	book_id, err := book_id_from_url(result.Profile.URL)
	if err != nil {
		return nil, err
	}
	metadata_data, _ := json.Marshal(map[string]any{
		"book_id":       book_id,
		"chapter_count": len(result.Profile.Chapters),
	})
	now := util.NowMillis()
	return &model.Content{
		Id:         BuildContentID(book_id),
		PlatformId: PlatformID,
		ExternalId: book_id,
		Type:       model.ContentTypeNovel,
		Title:      strings.TrimSpace(result.Profile.Title),
		URL:        strings.TrimSpace(result.Profile.URL),
		SourceURL:  strings.TrimSpace(result.Profile.URL),
		CoverURL:   strings.TrimSpace(result.Profile.CoverURL),
		Metadata:   string(metadata_data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToAccount converts the TTK author into the common account model.
func ToAccount(result *ttk.TtkFetchResult) (*model.Account, error) {
	result, err := validate_ttk_result(result)
	if err != nil {
		return nil, err
	}
	author_name := strings.TrimSpace(result.Profile.Author)
	if author_name == "" {
		author_name = ttk_official_account_name
	}
	now := util.NowMillis()
	account := &model.Account{
		Id:         BuildAccountID(author_name),
		PlatformId: PlatformID,
		ExternalId: author_name,
		Nickname:   author_name,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if author_name == ttk_official_account_name {
		account.ProfileURL = ttk_official_profile_url
	}
	return account, nil
}

// ToContentNovel converts fetched TTK chapters into a novel extension.
func ToContentNovel(result *ttk.TtkFetchResult, content_id string) (*model.ContentNovel, error) {
	result, err := validate_ttk_result(result)
	if err != nil {
		return nil, err
	}
	chapter_count := len(result.Profile.Chapters)
	if len(result.Chapters) > 0 {
		chapter_count = len(result.Chapters)
	}
	word_count := 0
	for _, chapter := range result.Chapters {
		word_count += count_text_characters(chapter.Content)
	}
	return &model.ContentNovel{
		Id:           strings.TrimSpace(content_id),
		AuthorName:   strings.TrimSpace(result.Profile.Author),
		WordCount:    word_count,
		ChapterCount: chapter_count,
		SeriesName:   strings.TrimSpace(result.Profile.Title),
		Text:         novel_text(result.Chapters),
	}, nil
}

// ToContentNovelChapters converts fetched chapters or the profile directory
// into database models.
func ToContentNovelChapters(result *ttk.TtkFetchResult, content_id string) ([]model.ContentNovelChapter, error) {
	result, err := validate_ttk_result(result)
	if err != nil {
		return nil, err
	}
	if len(result.Chapters) > 0 {
		chapters := make([]model.ContentNovelChapter, 0, len(result.Chapters))
		for chapter_index, chapter := range result.Chapters {
			idx := chapter.Index
			if idx <= 0 {
				idx = chapter_index + 1
			}
			chapters = append(chapters, model.ContentNovelChapter{
				NovelId:    strings.TrimSpace(content_id),
				ChapterKey: model.BuildContentNovelChapterKey(chapter_id_from_url(chapter.URL), chapter.URL, idx),
				Idx:        idx,
				Title:      strings.TrimSpace(chapter.Title),
				URL:        strings.TrimSpace(chapter.URL),
				WordCount:  count_text_characters(chapter.Content),
			})
		}
		return chapters, nil
	}
	chapters := make([]model.ContentNovelChapter, 0, len(result.Profile.Chapters))
	for chapter_index, chapter := range result.Profile.Chapters {
		idx := chapter.Index
		if idx <= 0 {
			idx = chapter_index + 1
		}
		chapters = append(chapters, model.ContentNovelChapter{
			NovelId:    strings.TrimSpace(content_id),
			ChapterKey: model.BuildContentNovelChapterKey(chapter_id_from_url(chapter.URL), chapter.URL, idx),
			Idx:        idx,
			Title:      strings.TrimSpace(chapter.Title),
			URL:        strings.TrimSpace(chapter.URL),
		})
	}
	return chapters, nil
}

// ToContentDetails converts a complete fetch result into ordered novel
// details.
func ToContentDetails(result *ttk.TtkFetchResult) ([]adapter.ContentDetail, error) {
	content, err := ToContent(result)
	if err != nil {
		return nil, err
	}
	novel, err := ToContentNovel(result, content.Id)
	if err != nil {
		return nil, err
	}
	chapters, err := ToContentNovelChapters(result, content.Id)
	if err != nil {
		return nil, err
	}
	return ttk_content_details(content, novel, chapters), nil
}

func ttk_content_details(
	content *model.Content,
	novel *model.ContentNovel,
	chapters []model.ContentNovelChapter,
) []adapter.ContentDetail {
	novel_detail := *novel
	novel_detail.Chapters = nil
	details := make([]adapter.ContentDetail, 0, len(chapters)+1)
	details = append(details, adapter.ContentDetail{Type: "novel", Key: content.Id, Data: &novel_detail})
	for chapter_index := range chapters {
		chapter := chapters[chapter_index]
		details = append(details, adapter.ContentDetail{
			Type: "novel_chapter",
			Key:  fmt.Sprintf("%s:chapter:%d", content.Id, chapter.Idx),
			Data: &chapter,
		})
	}
	return details
}

func fetched_chapter_content_detail(chapter ttk.TtkFetchedChapter, content_id string) adapter.ContentDetail {
	idx := chapter.Index
	return adapter.ContentDetail{
		Type: "novel_chapter",
		Key:  fmt.Sprintf("%s:chapter:%d", content_id, idx),
		Data: &model.ContentNovelChapter{
			NovelId:    strings.TrimSpace(content_id),
			ChapterKey: model.BuildContentNovelChapterKey(chapter_id_from_url(chapter.URL), chapter.URL, idx),
			Idx:        idx,
			Title:      strings.TrimSpace(chapter.Title),
			URL:        strings.TrimSpace(chapter.URL),
			WordCount:  count_text_characters(chapter.Content),
		},
	}
}

// ToNovelModelSet converts one fetch result into all supported models.
func ToNovelModelSet(result *ttk.TtkFetchResult) (*NovelModelSet, error) {
	content, err := ToContent(result)
	if err != nil {
		return nil, err
	}
	novel, err := ToContentNovel(result, content.Id)
	if err != nil {
		return nil, err
	}
	chapters, err := ToContentNovelChapters(result, content.Id)
	if err != nil {
		return nil, err
	}
	account, err := ToAccount(result)
	if err != nil {
		return nil, err
	}
	novel.Chapters = chapters
	return &NovelModelSet{
		Content:  content,
		Novel:    novel,
		Chapters: chapters,
		Account:  account,
	}, nil
}

// BuildDownloadTask builds a collection task containing every fetched TTK
// chapter as plain text.
func BuildDownloadTask(result *ttk.TtkFetchResult, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	return build_download_task(result, config_json)
}

func build_download_task(result *ttk.TtkFetchResult, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	model_set, err := ToNovelModelSet(result)
	if err != nil {
		return nil, err
	}
	if len(result.Chapters) == 0 {
		return nil, fmt.Errorf("TT看书未返回可下载章节正文")
	}
	config := make(map[string]any)
	config_text := strings.TrimSpace(string(config_json))
	if config_text != "" && config_text != "null" {
		if err := json.Unmarshal(config_json, &config); err != nil {
			return nil, fmt.Errorf("解析下载配置失败: %w", err)
		}
	}
	if config == nil {
		config = make(map[string]any)
	}
	task_name := strings.TrimSpace(config_string(config, "filename"))
	if task_name == "" {
		task_name = model_set.Content.Title
	}
	config_data, _ := json.Marshal(config)
	metadata_data, _ := json.Marshal(map[string]any{
		"platform":      PlatformID,
		"external_id":   model_set.Content.ExternalId,
		"title":         model_set.Content.Title,
		"author":        model_set.Novel.AuthorName,
		"cover_url":     model_set.Content.CoverURL,
		"chapter_count": model_set.Novel.ChapterCount,
		"word_count":    model_set.Novel.WordCount,
	})

	resources := make([]*adapter.ResourceInfo, 0, len(result.Chapters))
	for chapter_index, chapter := range result.Chapters {
		idx := chapter.Index
		if idx <= 0 {
			idx = chapter_index + 1
		}
		chapter_title := strings.TrimSpace(chapter.Title)
		if chapter_title == "" {
			chapter_title = fmt.Sprintf("chapter_%04d", idx)
		}
		chapter_content := chapter_title + "\n\n" + strings.TrimSpace(chapter.Content) + "\n"
		extra_data, _ := json.Marshal(map[string]any{
			"chapter_index": idx,
			"chapter_title": chapter_title,
			"source_url":    strings.TrimSpace(chapter.URL),
		})
		chapter_key := model.BuildContentNovelChapterKey(chapter_id_from_url(chapter.URL), chapter.URL, idx)
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId: &model_set.Content.Id,
				Name:      fmt.Sprintf("chapters/%04d_%s", idx, sanitize_filename(chapter_title)),
				Kind:      "text/plain",
				UniqueID:  model_set.Content.ExternalId + "_chapter_" + strconv.Itoa(idx),
				Size:      int64(len(chapter_content)),
				Extra:     string(extra_data),
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "inline",
				URL:      chapter_content,
				Enabled:  1,
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:            model.ContentAssetKindText,
				Role:            model.ContentAssetRoleNovelChapter,
				AssetKey:        model.BuildContentNovelChapterAssetKey(chapter_key, "txt"),
				Relation:        model.DownloadResourceAssetRelationSource,
				SubjectType:     model.ContentAssetSubjectNovelChapter,
				SubjectKey:      chapter_key,
				SubjectRelation: model.ContentAssetSubjectRelationRepresentation,
			}},
		})
	}
	if config_bool(config, "download_cover") && model_set.Content.CoverURL != "" {
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId:  &model_set.Content.Id,
				Name:       "cover",
				Kind:       "image",
				UniqueID:   model_set.Content.ExternalId + "_cover",
				MergeOrder: 999,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: endpoint_protocol(model_set.Content.CoverURL),
				URL:      model_set.Content.CoverURL,
				Enabled:  1,
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindImage,
				Role:     model.ContentAssetRoleCover,
				AssetKey: "cover",
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &model_set.Content.Id,
			Name:         task_name,
			UniqueID:     model_set.Content.ExternalId + "_text",
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    model_set.Content.SourceURL,
			CoverURL:     model_set.Content.CoverURL,
			ConfigJSON:   string(config_data),
			MetadataJSON: string(metadata_data),
		},
		Resources:      resources,
		Account:        model_set.Account,
		Content:        model_set.Content,
		ContentDetail:  model_set.Novel,
		ContentDetails: ttk_content_details(model_set.Content, model_set.Novel, model_set.Chapters),
	}, nil
}

func book_id_from_url(raw_url string) (string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Scheme == "" || parsed_url.Hostname() == "" {
		return "", fmt.Errorf("invalid ttk profile url %q", raw_url)
	}
	host := strings.ToLower(strings.TrimSpace(parsed_url.Hostname()))
	if host != "ttks.tw" && host != "www.ttks.tw" {
		return "", fmt.Errorf("invalid ttk profile url %q", raw_url)
	}
	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	for path_index := len(path_parts) - 1; path_index >= 0; path_index-- {
		if book_id := safe_identifier(path_parts[path_index]); book_id != "" {
			return book_id, nil
		}
	}
	return "", fmt.Errorf("invalid ttk book id in %q", raw_url)
}

func chapter_id_from_url(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ""
	}
	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	for path_index := len(path_parts) - 1; path_index >= 0; path_index-- {
		if chapter_id := safe_identifier(path_parts[path_index]); chapter_id != "" {
			return chapter_id
		}
	}
	return ""
}

func safe_identifier(value string) string {
	var builder strings.Builder
	last_dash := false
	for _, character := range strings.TrimSpace(value) {
		switch {
		case character >= 'a' && character <= 'z':
			builder.WriteRune(character)
			last_dash = false
		case character >= 'A' && character <= 'Z':
			builder.WriteRune(character + ('a' - 'A'))
			last_dash = false
		case character >= '0' && character <= '9':
			builder.WriteRune(character)
			last_dash = false
		case character == '_' || character == '-':
			builder.WriteRune(character)
			last_dash = character == '-'
		default:
			if !last_dash {
				builder.WriteRune('-')
				last_dash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-_")
}

func novel_text(chapters []ttk.TtkFetchedChapter) string {
	var text_builder strings.Builder
	for chapter_index, chapter := range chapters {
		if chapter_index > 0 {
			text_builder.WriteString("\n\n")
		}
		text_builder.WriteString(strings.TrimSpace(chapter.Title))
		text_builder.WriteString("\n\n")
		text_builder.WriteString(strings.TrimSpace(chapter.Content))
	}
	return text_builder.String()
}

func count_text_characters(value string) int {
	count := 0
	for _, character := range value {
		if !unicode.IsSpace(character) {
			count++
		}
	}
	return count
}

func sanitize_filename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(character rune) rune {
		switch character {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		}
		if unicode.IsControl(character) {
			return '_'
		}
		return character
	}, value)
	value = strings.Trim(value, ". ")
	if value == "" {
		return "chapter"
	}
	if utf8.RuneCountInString(value) > 100 {
		value = string([]rune(value)[:100])
	}
	return value
}

func config_string(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}

func config_bool(config map[string]any, key string) bool {
	value, _ := config[key].(bool)
	return value
}

func endpoint_protocol(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
}
