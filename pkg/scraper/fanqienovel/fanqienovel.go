package fanqienovel

import (
	"encoding/json"
	"fmt"
	stdhtml "html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func ParseBookProfileHTML(reqURL string, htmlText string) (*BookProfile, error) {
	state, err := ParseInitialState([]byte(htmlText))
	if err != nil {
		return nil, err
	}
	return BookProfileFromInitialState(reqURL, state)
}

func BookProfileFromInitialState(reqURL string, state *InitialState) (*BookProfile, error) {
	if state == nil {
		return nil, fmt.Errorf("fanqienovel initial state is nil")
	}
	page := state.Page
	bookID := strings.TrimSpace(page.BookID)
	title := strings.TrimSpace(page.BookName)
	if bookID == "" && title == "" {
		return nil, fmt.Errorf("missing fanqienovel book profile in initial state")
	}
	profile := &BookProfile{
		URL:              firstNonEmpty(reqURL, BaseURL+"/page/"+bookID),
		Title:            title,
		Description:      strings.TrimSpace(page.Abstract),
		CoverURL:         NormalizeURL(firstNonEmpty(page.ThumbURL, page.ThumbURI), BaseURL+"/"),
		Tags:             categoryTags(page),
		ChapterCount:     page.ChapterTotal,
		InitialStateJSON: state.Raw,
		Author: Author{
			Name:      firstNonEmpty(page.AuthorName, page.Author),
			Desc:      strings.TrimSpace(page.Description),
			AvatarURL: NormalizeURL(page.AvatarURI, BaseURL+"/"),
		},
	}
	if bookID != "" {
		profile.URL = BaseURL + "/page/" + bookID
	}
	if page.AuthorID != "" {
		profile.Author.URL = BaseURL + "/author-page/" + strings.TrimSpace(page.AuthorID)
	}
	if page.CreatorID != "" && profile.Author.URL == "" {
		profile.Author.URL = BaseURL + "/author-page/" + strings.TrimSpace(page.CreatorID)
	}
	if t := parseUnixTime(page.LastPublishTime); t != nil {
		profile.LatestUpdateAt = t
	}
	profile.LatestChapter = Chapter{
		ID:    strings.TrimSpace(page.LastChapterItemID),
		Idx:   page.ChapterTotal,
		Title: strings.TrimSpace(page.LastChapterTitle),
		URL:   chapterURL(page.LastChapterItemID),
	}
	profile.Volumes = volumesFromPage(page)
	if profile.ChapterCount == 0 {
		profile.ChapterCount = countChapters(profile.Volumes)
	}
	if profile.LatestChapter.Title == "" {
		if chapter := firstChapter(profile.Volumes); chapter != nil {
			profile.LatestChapter = *chapter
		}
	}
	return profile, nil
}

func ParseChapterContentHTML(htmlText string) (*ChapterContent, error) {
	state, err := ParseInitialState([]byte(htmlText))
	if err != nil {
		return nil, err
	}
	return ChapterContentFromInitialState(state)
}

func ChapterContentFromInitialState(state *InitialState) (*ChapterContent, error) {
	if state == nil {
		return nil, fmt.Errorf("fanqienovel initial state is nil")
	}
	chapter := state.Reader.ChapterData
	title := firstNonEmpty(chapter.Title, chapter.ChapterName)
	content := normalizeChapterContent(chapter.Content)
	if title == "" && content == "" {
		return nil, fmt.Errorf("missing fanqienovel reader chapter in initial state")
	}
	out := &ChapterContent{
		Title:            title,
		Content:          content,
		WorkCount:        firstNonEmpty(valueString(chapter.ChapterWordNumber), valueString(chapter.WordNumber), valueString(chapter.WordCount)),
		InitialStateJSON: state.Raw,
	}
	if t := parseFlexibleTime(firstNonEmpty(valueString(chapter.PublishTime), valueString(chapter.CreateTime))); t != nil {
		out.PublishAt = t
	}
	return out, nil
}

func volumesFromPage(page PageState) []BookVolume {
	if len(page.ChapterListWithVolume) > 0 {
		volumes := make([]BookVolume, 0, len(page.ChapterListWithVolume))
		chapterIdx := 0
		for i, source := range page.ChapterListWithVolume {
			title := firstNonEmpty(source.VolumeName, source.Title, source.Name)
			volume := BookVolume{Idx: i + 1, Title: title}
			for _, chapter := range firstChapterList(source) {
				chapterIdx++
				if out, ok := chapterFromState(chapter, chapterIdx); ok {
					volume.Chapters = append(volume.Chapters, out)
				}
			}
			if len(volume.Chapters) > 0 || volume.Title != "" {
				volumes = append(volumes, volume)
			}
		}
		if len(volumes) > 0 {
			return volumes
		}
	}
	if len(page.ChapterList) == 0 {
		return nil
	}
	var volumes []BookVolume
	volumeIndex := map[string]int{}
	for i, source := range page.ChapterList {
		volumeName := firstNonEmpty(source.VolumeName, "默认")
		idx, ok := volumeIndex[volumeName]
		if !ok {
			idx = len(volumes)
			volumeIndex[volumeName] = idx
			volumes = append(volumes, BookVolume{Idx: idx + 1, Title: volumeName})
		}
		if chapter, ok := chapterFromState(source, i+1); ok {
			volumes[idx].Chapters = append(volumes[idx].Chapters, chapter)
		}
	}
	return volumes
}

func firstChapterList(volume VolumeState) []ChapterState {
	switch {
	case len(volume.ChapterList) > 0:
		return volume.ChapterList
	case len(volume.Chapters) > 0:
		return volume.Chapters
	default:
		return volume.ItemList
	}
}

func chapterFromState(source ChapterState, fallbackIdx int) (Chapter, bool) {
	id := strings.TrimSpace(source.ItemID)
	title := strings.TrimSpace(source.Title)
	if id == "" && title == "" {
		return Chapter{}, false
	}
	idx := source.Order
	if idx == 0 {
		idx = fallbackIdx
	}
	return Chapter{
		Idx:   idx,
		ID:    id,
		Title: title,
		URL:   chapterURL(id),
	}, true
}

func chapterURL(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return BaseURL + "/reader/" + id
}

func categoryTags(page PageState) []string {
	var tags []string
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		tags = append(tags, value)
	}
	add(page.Category)
	if strings.TrimSpace(page.CategoryV2) != "" {
		var categories []struct {
			Name string `json:"Name"`
		}
		if err := json.Unmarshal([]byte(page.CategoryV2), &categories); err == nil {
			for _, category := range categories {
				add(category.Name)
			}
		}
	}
	return tags
}

func countChapters(volumes []BookVolume) int {
	total := 0
	for _, volume := range volumes {
		total += len(volume.Chapters)
	}
	return total
}

func firstChapter(volumes []BookVolume) *Chapter {
	for _, volume := range volumes {
		if len(volume.Chapters) > 0 {
			return &volume.Chapters[0]
		}
	}
	return nil
}

func parseUnixTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 {
		return nil
	}
	t := time.Unix(seconds, 0)
	return &t
}

func parseFlexibleTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if t := parseUnixTime(value); t != nil {
		return t
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, value); err == nil {
			return &t
		}
	}
	return nil
}

func normalizeChapterContent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "<") && strings.Contains(value, ">") {
		value = strings.ReplaceAll(value, "<br>", "\n")
		value = strings.ReplaceAll(value, "<br/>", "\n")
		value = strings.ReplaceAll(value, "<br />", "\n")
		value = strings.ReplaceAll(value, "</p>", "\n")
		re := regexp.MustCompile(`<[^>]+>`)
		value = re.ReplaceAllString(value, "")
	}
	value = stdhtml.UnescapeString(value)
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func NormalizeURL(value string, base string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	base = firstNonEmpty(base, BaseURL+"/")
	baseURL, err := url.Parse(base)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		baseURL, _ = url.Parse(BaseURL + "/")
	}
	ref, err := url.Parse(value)
	if err != nil {
		return value
	}
	return baseURL.ResolveReference(ref).String()
}

func valueString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case json.Number:
		return v.String()
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
