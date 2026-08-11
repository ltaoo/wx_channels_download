package fanqienoveladapter

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
	"wx_channel/pkg/scraper/fanqienovel"
	"wx_channel/pkg/util"
)

// NovelModelSet contains all database models derived from a Fanqie book.
type NovelModelSet struct {
	Content  *model.Content
	Novel    *model.ContentNovel
	Volumes  []model.ContentNovelVolume
	Chapters []model.ContentNovelChapter
	Account  *model.Account
}

// BuildContentID builds a content identifier from a Fanqie book ID.
func BuildContentID(book_id string) string {
	return PlatformID + ":" + book_id
}

// BuildAccountID builds an account identifier from a Fanqie author ID.
func BuildAccountID(author_id string) string {
	return PlatformID + ":" + author_id
}

// ToContent converts a Fanqie fetch result into the common content model.
func ToContent(result *fanqienovel.FanqieFetchResult) (*model.Content, error) {
	result, err := validate_fanqie_result(result)
	if err != nil {
		return nil, err
	}
	book_id, err := book_id_from_url(result.Profile.URL)
	if err != nil {
		return nil, err
	}

	tags_json, _ := json.Marshal(result.Profile.Tags)
	now := util.NowMillis()
	content := &model.Content{
		Id:          BuildContentID(book_id),
		PlatformId:  PlatformID,
		ExternalId:  book_id,
		Type:        "novel",
		Title:       strings.TrimSpace(result.Profile.Title),
		Description: strings.TrimSpace(result.Profile.Description),
		URL:         strings.TrimSpace(result.Profile.URL),
		SourceURL:   strings.TrimSpace(result.Profile.URL),
		CoverURL:    strings.TrimSpace(result.Profile.CoverURL),
		Tags:        string(tags_json),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if result.Profile.LatestUpdateAt != nil {
		updated_at := result.Profile.LatestUpdateAt.UnixMilli()
		content.UpdateTime = &updated_at
	}
	return content, nil
}

// ToAccount converts the Fanqie author into the common account model.
func ToAccount(result *fanqienovel.FanqieFetchResult) (*model.Account, error) {
	result, err := validate_fanqie_result(result)
	if err != nil {
		return nil, err
	}
	author := result.Profile.Author
	author_name := strings.TrimSpace(author.Name)
	if author_name == "" {
		return nil, nil
	}
	author_id := author_id_from_profile(author)
	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(author_id),
		PlatformId: PlatformID,
		ExternalId: author_id,
		Nickname:   author_name,
		Signature:  strings.TrimSpace(author.Desc),
		AvatarURL:  strings.TrimSpace(author.AvatarURL),
		ProfileURL: strings.TrimSpace(author.URL),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToContentNovel converts the fetched chapter bodies into a novel extension.
func ToContentNovel(result *fanqienovel.FanqieFetchResult, content_id string) (*model.ContentNovel, error) {
	result, err := validate_fanqie_result(result)
	if err != nil {
		return nil, err
	}
	chapter_count := result.Profile.ChapterCount
	if len(result.Chapters) > 0 {
		chapter_count = len(result.Chapters)
	}
	word_count := 0
	for _, chapter := range result.Chapters {
		word_count += count_text_characters(chapter.Content)
	}
	return &model.ContentNovel{
		Id:           content_id,
		AuthorName:   strings.TrimSpace(result.Profile.Author.Name),
		WordCount:    word_count,
		ChapterCount: chapter_count,
		VolumeCount:  len(result.Profile.Volumes),
		SeriesName:   strings.TrimSpace(result.Profile.Title),
		IsFinished:   finished_status(result.Profile.Tags),
		Text:         novel_text(result.Chapters),
	}, nil
}

// ToContentNovelVolumes converts the directory volumes into database models.
func ToContentNovelVolumes(result *fanqienovel.FanqieFetchResult, content_id string) ([]model.ContentNovelVolume, error) {
	result, err := validate_fanqie_result(result)
	if err != nil {
		return nil, err
	}
	volumes := make([]model.ContentNovelVolume, 0, len(result.Profile.Volumes))
	for volume_index, volume := range result.Profile.Volumes {
		idx := volume.Idx
		if idx <= 0 {
			idx = volume_index + 1
		}
		volumes = append(volumes, model.ContentNovelVolume{
			NovelId: content_id,
			Idx:     idx,
			Title:   strings.TrimSpace(volume.Title),
		})
	}
	return volumes, nil
}

// ToContentNovelChapters converts the fetched chapters into database models.
func ToContentNovelChapters(result *fanqienovel.FanqieFetchResult, content_id string) ([]model.ContentNovelChapter, error) {
	result, err := validate_fanqie_result(result)
	if err != nil {
		return nil, err
	}
	if len(result.Chapters) > 0 {
		chapters := make([]model.ContentNovelChapter, 0, len(result.Chapters))
		for chapter_index, chapter := range result.Chapters {
			idx := chapter.Idx
			if idx <= 0 {
				idx = chapter_index + 1
			}
			chapters = append(chapters, model.ContentNovelChapter{
				NovelId:   content_id,
				Idx:       idx,
				Title:     strings.TrimSpace(chapter.Title),
				URL:       strings.TrimSpace(chapter.URL),
				WordCount: count_text_characters(chapter.Content),
			})
		}
		return chapters, nil
	}

	chapter_count := result.Profile.ChapterCount
	chapters := make([]model.ContentNovelChapter, 0, chapter_count)
	chapter_index := 0
	for _, volume := range result.Profile.Volumes {
		for _, chapter := range volume.Chapters {
			chapter_index++
			chapters = append(chapters, model.ContentNovelChapter{
				NovelId: content_id,
				Idx:     chapter_index,
				Title:   strings.TrimSpace(chapter.Title),
				URL:     strings.TrimSpace(chapter.URL),
			})
		}
	}
	return chapters, nil
}

// ToContentDetails converts a complete fetch result into ordered novel details.
func ToContentDetails(result *fanqienovel.FanqieFetchResult) ([]adapter.ContentDetail, error) {
	content, err := ToContent(result)
	if err != nil {
		return nil, err
	}
	novel, err := ToContentNovel(result, content.Id)
	if err != nil {
		return nil, err
	}
	volumes, err := ToContentNovelVolumes(result, content.Id)
	if err != nil {
		return nil, err
	}
	chapters, err := ToContentNovelChapters(result, content.Id)
	if err != nil {
		return nil, err
	}
	details := []adapter.ContentDetail{{Type: "novel", Key: content.Id, Data: novel}}
	for volume_index := range volumes {
		volume := volumes[volume_index]
		details = append(details, adapter.ContentDetail{
			Type: "novel_volume",
			Key:  fmt.Sprintf("%s:volume:%d", content.Id, volume.Idx),
			Data: &volume,
		})
	}
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

func fetched_chapter_content_detail(chapter fanqienovel.FanqieFetchedChapter, content_id string) adapter.ContentDetail {
	idx := chapter.Idx
	return adapter.ContentDetail{
		Type: "novel_chapter",
		Key:  fmt.Sprintf("%s:chapter:%d", content_id, idx),
		Data: &model.ContentNovelChapter{
			NovelId:   content_id,
			Idx:       idx,
			Title:     strings.TrimSpace(chapter.Title),
			URL:       strings.TrimSpace(chapter.URL),
			WordCount: count_text_characters(chapter.Content),
		},
	}
}

// ToNovelModelSet converts one fetch result into all currently supported models.
func ToNovelModelSet(result *fanqienovel.FanqieFetchResult) (*NovelModelSet, error) {
	content, err := ToContent(result)
	if err != nil {
		return nil, err
	}
	novel, err := ToContentNovel(result, content.Id)
	if err != nil {
		return nil, err
	}
	volumes, err := ToContentNovelVolumes(result, content.Id)
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
	return &NovelModelSet{
		Content:  content,
		Novel:    novel,
		Volumes:  volumes,
		Chapters: chapters,
		Account:  account,
	}, nil
}

// BuildDownloadTask builds a collection task containing every fetched chapter.
func BuildDownloadTask(result *fanqienovel.FanqieFetchResult, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	return build_download_task(result, config_json, "")
}

func build_download_task(result *fanqienovel.FanqieFetchResult, config_json json.RawMessage, work_dir string) (*adapter.DownloadTaskResult, error) {
	model_set, err := ToNovelModelSet(result)
	if err != nil {
		return nil, err
	}
	config := make(map[string]any)
	if len(strings.TrimSpace(string(config_json))) > 0 {
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
		"chapter_count": model_set.Novel.ChapterCount,
		"volume_count":  model_set.Novel.VolumeCount,
	})

	resources := make([]*adapter.ResourceInfo, 0, len(result.Chapters)+1)
	for chapter_index, chapter := range result.Chapters {
		idx := chapter.Idx
		if idx <= 0 {
			idx = chapter_index + 1
		}
		chapter_title := strings.TrimSpace(chapter.Title)
		if chapter_title == "" {
			chapter_title = fmt.Sprintf("chapter_%04d", idx)
		}
		chapter_name := fmt.Sprintf("chapters/%04d_%s.txt", idx, sanitize_filename(chapter_title))
		chapter_content := chapter_title + "\n\n" + strings.TrimSpace(chapter.Content) + "\n"
		resource_kind := "text/plain"
		resource_size := int64(len(chapter_content))
		endpoint_protocol_name := "inline"
		endpoint_url := chapter_content
		cache_reused := false
		if strings.TrimSpace(work_dir) != "" {
			cached_html, cache_err := fanqienovel.LookupChapterHTMLCache(work_dir, result.Profile.URL, chapter.URL)
			if cache_err != nil {
				return nil, fmt.Errorf("查找章节 %q 缓存失败: %w", chapter_title, cache_err)
			}
			if cached_html != nil {
				chapter_name = fmt.Sprintf("chapters/%04d_%s.html", idx, sanitize_filename(chapter_title))
				resource_kind = "text/html"
				resource_size = cached_html.Size
				endpoint_protocol_name = "file"
				endpoint_url = cached_html.Path
				cache_reused = true
			}
		}
		extra_data, _ := json.Marshal(map[string]any{
			"chapter_index": idx,
			"chapter_title": chapter_title,
			"volume_index":  chapter.VolumeIdx,
			"volume_title":  chapter.VolumeTitle,
			"source_url":    chapter.URL,
			"cache_reused":  cache_reused,
		})
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId: &model_set.Content.Id,
				Name:      chapter_name,
				Kind:      resource_kind,
				UniqueID:  model_set.Content.ExternalId + "_chapter_" + strconv.Itoa(idx),
				Size:      resource_size,
				Extra:     string(extra_data),
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: endpoint_protocol_name,
				URL:      endpoint_url,
				Enabled:  1,
			}},
		})
	}

	if len(result.Chapters) == 0 {
		for _, volume := range result.Profile.Volumes {
			for _, chapter := range volume.Chapters {
				chapter_index := len(resources) + 1
				chapter_name := fmt.Sprintf("chapters/%04d_%s.html", chapter_index, sanitize_filename(chapter.Title))
				chapter_url := strings.TrimSpace(chapter.URL)
				if chapter_url == "" {
					continue
				}
				resource_size := int64(0)
				endpoint_protocol_name := endpoint_protocol(chapter_url)
				endpoint_url := chapter_url
				cache_reused := false
				if strings.TrimSpace(work_dir) != "" {
					cached_html, cache_err := fanqienovel.LookupChapterHTMLCache(work_dir, result.Profile.URL, chapter_url)
					if cache_err != nil {
						return nil, fmt.Errorf("查找章节 %q 缓存失败: %w", chapter.Title, cache_err)
					}
					if cached_html != nil {
						resource_size = cached_html.Size
						endpoint_protocol_name = "file"
						endpoint_url = cached_html.Path
						cache_reused = true
					}
				}
				extra_data, _ := json.Marshal(map[string]any{
					"chapter_index": chapter_index,
					"chapter_title": strings.TrimSpace(chapter.Title),
					"volume_index":  volume.Idx,
					"volume_title":  volume.Title,
					"source_url":    chapter_url,
					"cache_reused":  cache_reused,
				})
				resources = append(resources, &adapter.ResourceInfo{
					Resource: model.DownloadResource{
						ContentId: &model_set.Content.Id,
						Name:      chapter_name,
						Kind:      "text/html",
						UniqueID:  model_set.Content.ExternalId + "_chapter_" + strconv.Itoa(chapter_index),
						Size:      resource_size,
						Extra:     string(extra_data),
					},
					Endpoints: []model.DownloadEndpoint{{
						Protocol: endpoint_protocol_name,
						URL:      endpoint_url,
						Enabled:  1,
					}},
				})
			}
		}
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
		})
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("番茄小说未返回可下载章节")
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
		Resources:     resources,
		Account:       model_set.Account,
		Content:       model_set.Content,
		ContentDetail: model_set.Novel,
	}, nil
}

func book_id_from_url(raw_url string) (string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return "", fmt.Errorf("parse fanqienovel profile url: %w", err)
	}
	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	if len(path_parts) != 2 || path_parts[0] != "page" {
		return "", fmt.Errorf("invalid fanqienovel profile url %q", raw_url)
	}
	book_id := path_parts[1]
	if _, err := strconv.ParseUint(book_id, 10, 64); err != nil {
		return "", fmt.Errorf("invalid fanqienovel book id %q", book_id)
	}
	return book_id, nil
}

func author_id_from_profile(author fanqienovel.FanqieAuthor) string {
	parsed_url, err := url.Parse(strings.TrimSpace(author.URL))
	if err == nil {
		path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
		if len(path_parts) > 0 {
			author_id := strings.TrimSpace(path_parts[len(path_parts)-1])
			if author_id != "" {
				return author_id
			}
		}
	}
	return strings.TrimSpace(author.Name)
}

func novel_text(chapters []fanqienovel.FanqieFetchedChapter) string {
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

func finished_status(tags []string) int {
	for _, tag := range tags {
		if strings.Contains(strings.TrimSpace(tag), "完结") {
			return 1
		}
	}
	return 0
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

func endpoint_protocol(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
}

func config_string(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}

func config_bool(config map[string]any, key string) bool {
	value, _ := config[key].(bool)
	return value
}
