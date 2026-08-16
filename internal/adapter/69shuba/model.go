package shuba69adapter

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
	shuba69 "wx_channel/pkg/scraper/69shuba"
	"wx_channel/pkg/util"
)

// NovelModelSet contains all database models derived from a 69shuba novel.
type NovelModelSet struct {
	Content  *model.Content
	Novel    *model.ContentNovel
	Chapters []model.ContentNovelChapter
	Account  *model.Account
}

// BuildContentID builds a content identifier from a 69shuba book ID.
func BuildContentID(book_id string) string {
	return PlatformID + ":" + strings.TrimSpace(book_id)
}

// BuildAccountID builds an account identifier from an author name.
func BuildAccountID(author_name string) string {
	return PlatformID + ":" + strings.TrimSpace(author_name)
}

// ToContent converts a 69shuba novel into the common content model.
func ToContent(novel *shuba69.Novel) (*model.Content, error) {
	novel, err := validate_novel(novel)
	if err != nil {
		return nil, err
	}
	external_id, err := novel_external_id(novel)
	if err != nil {
		return nil, err
	}
	metadata_data, _ := json.Marshal(map[string]any{
		"book_id": novel.BookID,
		"status":  strings.TrimSpace(novel.Status),
	})
	now := util.NowMillis()
	return &model.Content{
		Id:         BuildContentID(external_id),
		PlatformId: PlatformID,
		ExternalId: external_id,
		Type:       "novel",
		Title:      strings.TrimSpace(novel.Title),
		URL:        strings.TrimSpace(novel.URL),
		SourceURL:  strings.TrimSpace(novel.URL),
		CoverURL:   strings.TrimSpace(novel.CoverURL),
		Category:   strings.TrimSpace(novel.Category),
		Metadata:   string(metadata_data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToAccount converts a 69shuba author into the common account model.
func ToAccount(novel *shuba69.Novel) (*model.Account, error) {
	novel, err := validate_novel(novel)
	if err != nil {
		return nil, err
	}
	author_name := strings.TrimSpace(novel.Author)
	if author_name == "" {
		return nil, nil
	}
	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(author_name),
		PlatformId: PlatformID,
		ExternalId: author_name,
		Nickname:   author_name,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToContentNovel converts a 69shuba novel profile into its novel extension.
func ToContentNovel(novel *shuba69.Novel, content_id string) (*model.ContentNovel, error) {
	novel, err := validate_novel(novel)
	if err != nil {
		return nil, err
	}
	return &model.ContentNovel{
		Id:           strings.TrimSpace(content_id),
		AuthorName:   strings.TrimSpace(novel.Author),
		ChapterCount: len(novel.Chapters),
		SeriesName:   strings.TrimSpace(novel.Title),
		IsFinished:   finished_status(novel.Status),
	}, nil
}

// ToContentNovelChapters converts the fetched directory into chapter models.
func ToContentNovelChapters(novel *shuba69.Novel, content_id string) ([]model.ContentNovelChapter, error) {
	novel, err := validate_novel(novel)
	if err != nil {
		return nil, err
	}
	chapters := make([]model.ContentNovelChapter, 0, len(novel.Chapters))
	for chapter_index, chapter := range novel.Chapters {
		idx := chapter.Index
		if idx <= 0 {
			idx = chapter_index + 1
		}
		chapters = append(chapters, model.ContentNovelChapter{
			NovelId:    strings.TrimSpace(content_id),
			ChapterKey: model.BuildContentNovelChapterKey("", chapter.URL, idx),
			Idx:        idx,
			Title:      strings.TrimSpace(chapter.Title),
			URL:        strings.TrimSpace(chapter.URL),
		})
	}
	return chapters, nil
}

// ToContentDetails converts a 69shuba novel into ordered novel details.
func ToContentDetails(novel *shuba69.Novel) ([]adapter.ContentDetail, error) {
	content, err := ToContent(novel)
	if err != nil {
		return nil, err
	}
	novel_detail, err := ToContentNovel(novel, content.Id)
	if err != nil {
		return nil, err
	}
	chapters, err := ToContentNovelChapters(novel, content.Id)
	if err != nil {
		return nil, err
	}
	details := make([]adapter.ContentDetail, 0, len(chapters)+1)
	details = append(details, adapter.ContentDetail{
		Type: "novel",
		Key:  content.Id,
		Data: novel_detail,
	})
	for chapter_index := range chapters {
		chapter := chapters[chapter_index]
		details = append(details, adapter.ContentDetail{
			Type: "novel_chapter",
			Key:  fmt.Sprintf("%s:chapter:%d", content.Id, chapter.Idx),
			Data: &chapter,
		})
	}
	return details, nil
}

// ToNovelModelSet converts a 69shuba novel into all supported database models.
func ToNovelModelSet(novel *shuba69.Novel) (*NovelModelSet, error) {
	content, err := ToContent(novel)
	if err != nil {
		return nil, err
	}
	novel_detail, err := ToContentNovel(novel, content.Id)
	if err != nil {
		return nil, err
	}
	chapters, err := ToContentNovelChapters(novel, content.Id)
	if err != nil {
		return nil, err
	}
	account, err := ToAccount(novel)
	if err != nil {
		return nil, err
	}
	novel_detail.Chapters = chapters
	return &NovelModelSet{
		Content:  content,
		Novel:    novel_detail,
		Chapters: chapters,
		Account:  account,
	}, nil
}

// BuildDownloadTask builds a collection task containing the novel profile and
// every chapter page in directory order.
func BuildDownloadTask(novel *shuba69.Novel, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	return build_download_task(novel, config_json, "")
}

func build_download_task(novel *shuba69.Novel, config_json json.RawMessage, cookie string) (*adapter.DownloadTaskResult, error) {
	model_set, err := ToNovelModelSet(novel)
	if err != nil {
		return nil, err
	}
	if len(novel.Chapters) == 0 {
		return nil, fmt.Errorf("69书吧未返回可下载章节")
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
		"category":      model_set.Content.Category,
		"status":        strings.TrimSpace(novel.Status),
		"cover_url":     model_set.Content.CoverURL,
		"chapter_count": model_set.Novel.ChapterCount,
	})

	resources := make([]*adapter.ResourceInfo, 0, len(novel.Chapters)+2)
	resources = append(resources, &adapter.ResourceInfo{
		Resource: model.DownloadResource{
			ContentId: &model_set.Content.Id,
			Name:      "profile",
			Kind:      "text/html",
			UniqueID:  model_set.Content.ExternalId + "_profile",
		},
		Endpoints: []model.DownloadEndpoint{{
			Protocol: endpoint_protocol(model_set.Content.URL),
			URL:      model_set.Content.URL,
			Enabled:  1,
			Headers:  endpoint_headers(""),
			Cookies:  strings.TrimSpace(cookie),
		}},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindText,
			Role:     model.ContentAssetRoleSourceSnapshot,
			AssetKey: "profile:html",
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	})

	for chapter_index, chapter := range novel.Chapters {
		idx := chapter.Index
		if idx <= 0 {
			idx = chapter_index + 1
		}
		chapter_url := strings.TrimSpace(chapter.URL)
		if chapter_url == "" {
			continue
		}
		chapter_title := strings.TrimSpace(chapter.Title)
		if chapter_title == "" {
			chapter_title = fmt.Sprintf("chapter_%04d", idx)
		}
		extra_data, _ := json.Marshal(map[string]any{
			"chapter_index": idx,
			"chapter_title": chapter_title,
			"source_url":    chapter_url,
		})
		chapter_key := model.BuildContentNovelChapterKey("", chapter_url, idx)
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId: &model_set.Content.Id,
				Name:      fmt.Sprintf("chapters/%04d_%s", idx, sanitize_filename(chapter_title)),
				Kind:      "text/html",
				UniqueID:  model_set.Content.ExternalId + "_chapter_" + strconv.Itoa(idx),
				Extra:     string(extra_data),
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: endpoint_protocol(chapter_url),
				URL:      chapter_url,
				Enabled:  1,
				Headers:  endpoint_headers(model_set.Content.URL),
				Cookies:  strings.TrimSpace(cookie),
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:            model.ContentAssetKindText,
				Role:            model.ContentAssetRoleNovelChapter,
				AssetKey:        model.BuildContentNovelChapterAssetKey(chapter_key, "html"),
				Relation:        model.DownloadResourceAssetRelationSource,
				SubjectType:     model.ContentAssetSubjectNovelChapter,
				SubjectKey:      chapter_key,
				SubjectRelation: model.ContentAssetSubjectRelationRepresentation,
			}},
		})
	}
	if len(resources) == 1 {
		return nil, fmt.Errorf("69书吧未返回有效章节下载地址")
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
				Headers:  endpoint_headers(model_set.Content.URL),
				Cookies:  strings.TrimSpace(cookie),
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
			UniqueID:     model_set.Content.ExternalId + "_html",
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    model_set.Content.SourceURL,
			CoverURL:     model_set.Content.CoverURL,
			ConfigJSON:   string(config_data),
			MetadataJSON: string(metadata_data),
		},
		Resources:     resources,
		Account:       model_set.Account,
		Content:       model_set.Content,
		ContentDetail: model_set.Novel,
	}, nil
}

func novel_external_id(novel *shuba69.Novel) (string, error) {
	book_id := strings.TrimSpace(novel.BookID)
	if book_id == "" {
		parsed_url, err := url.Parse(strings.TrimSpace(novel.URL))
		if err == nil {
			path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
			if len(path_parts) == 2 && path_parts[0] == "book" {
				book_id = path_parts[1]
			}
		}
	}
	if _, err := strconv.ParseUint(book_id, 10, 64); err != nil {
		return "", fmt.Errorf("invalid 69shuba book id %q", book_id)
	}
	return book_id, nil
}

func finished_status(status string) int {
	status = strings.TrimSpace(status)
	if strings.Contains(status, "完结") || strings.Contains(status, "完本") {
		return 1
	}
	return 0
}

func endpoint_headers(referer string) string {
	headers := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Cache-Control":   "max-age=0",
		"Sec-Fetch-Dest":  "document",
		"Sec-Fetch-Mode":  "navigate",
		"Sec-Fetch-Site":  "same-origin",
	}
	if strings.TrimSpace(referer) != "" {
		headers["Referer"] = strings.TrimSpace(referer)
	}
	encoded, _ := json.Marshal(headers)
	return string(encoded)
}

func endpoint_protocol(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
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
