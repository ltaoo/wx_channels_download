package novelsource

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const (
	ContentTypeNovel   = "novel"
	ContentTypeChapter = "chapter"
)

type URLPattern struct {
	Expr       string
	Canonical  string
	BookID     string
	ChapterID  string
	BookIDJoin string
}

type Source struct {
	ID                      string
	Name                    string
	BaseURL                 string
	Hosts                   []string
	BookPatterns            []URLPattern
	ChapterPatterns         []URLPattern
	CatalogTemplates        []string
	CatalogSelectors        []string
	ChapterTitleSelectors   []string
	ChapterContentSelectors []string
	SampleNovelURL          string
	SampleChapterURL        string
}

type PageURL struct {
	Kind      string `json:"kind"`
	BookID    string `json:"book_id,omitempty"`
	ChapterID string `json:"chapter_id,omitempty"`
	Canonical string `json:"canonical"`
}

type Novel struct {
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	Author         string    `json:"author,omitempty"`
	Category       string    `json:"category,omitempty"`
	Status         string    `json:"status,omitempty"`
	BookID         string    `json:"book_id,omitempty"`
	Description    string    `json:"description,omitempty"`
	CoverURL       string    `json:"cover_url,omitempty"`
	WordCount      string    `json:"word_count,omitempty"`
	UpdateTime     string    `json:"update_time,omitempty"`
	LatestChapter  string    `json:"latest_chapter,omitempty"`
	ChapterCount   int       `json:"chapter_count,omitempty"`
	FullCatalogURL string    `json:"full_catalog_url,omitempty"`
	Chapters       []Chapter `json:"chapters,omitempty"`
}

type Chapter struct {
	Index     int    `json:"index"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ChapterContent struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type NovelFetchResult struct {
	Novel                 *Novel `json:"novel,omitempty"`
	SourceURL             string `json:"source_url,omitempty"`
	SourceHTML            string `json:"-"`
	SourceNovel           *Novel `json:"source_novel,omitempty"`
	SourceParsedHTML      string `json:"source_parsed_html,omitempty"`
	FullCatalogURL        string `json:"full_catalog_url,omitempty"`
	FullCatalogHTML       string `json:"-"`
	FullCatalogNovel      *Novel `json:"full_catalog_novel,omitempty"`
	FullCatalogParsedHTML string `json:"full_catalog_parsed_html,omitempty"`
}

type ChapterFetchResult struct {
	Chapter    Chapter         `json:"chapter"`
	URL        string          `json:"url"`
	HTML       string          `json:"-"`
	Content    *ChapterContent `json:"content,omitempty"`
	ParsedHTML string          `json:"parsed_html,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func AllSources() []Source {
	out := append([]Source(nil), registeredSources...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func SourceByID(id string) (Source, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, source := range registeredSources {
		if strings.ToLower(source.ID) == id {
			return source.normalized(), true
		}
	}
	return Source{}, false
}

func MustSource(id string) Source {
	source, ok := SourceByID(id)
	if !ok {
		panic(fmt.Sprintf("unknown novel source %q", id))
	}
	return source
}

func (s Source) CanParse(rawURL string) bool {
	_, ok := s.ParseURL(rawURL)
	return ok
}

func (s Source) ParseURL(rawURL string) (PageURL, bool) {
	s = s.normalized()
	parsed, ok := parseHTTPURL(rawURL)
	if !ok || !s.matchHost(parsed.Hostname()) {
		return PageURL{}, false
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	for _, pattern := range s.ChapterPatterns {
		if out, ok := s.matchPattern(ContentTypeChapter, pattern, path, parsed); ok {
			return out, true
		}
	}
	for _, pattern := range s.BookPatterns {
		if out, ok := s.matchPattern(ContentTypeNovel, pattern, path, parsed); ok {
			return out, true
		}
	}
	return PageURL{}, false
}

func (s Source) CatalogURL(bookID string) string {
	s = s.normalized()
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return ""
	}
	for _, tmpl := range s.CatalogTemplates {
		tmpl = strings.TrimSpace(tmpl)
		if tmpl == "" {
			continue
		}
		value := strings.ReplaceAll(tmpl, "%s", bookID)
		return joinBaseURL(s.BaseURL, value)
	}
	return ""
}

func (s Source) normalized() Source {
	s.BaseURL = strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
	if len(s.Hosts) == 0 {
		if parsed, err := url.Parse(s.BaseURL); err == nil {
			host := strings.ToLower(parsed.Hostname())
			if host != "" {
				s.Hosts = []string{host}
			}
		}
	}
	return s
}

func (s Source) matchHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, candidate := range s.Hosts {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate == "" {
			continue
		}
		if host == candidate {
			return true
		}
		if strings.HasPrefix(candidate, "www.") && host == strings.TrimPrefix(candidate, "www.") {
			return true
		}
		if !strings.HasPrefix(candidate, "www.") && host == "www."+candidate {
			return true
		}
	}
	return false
}

func (s Source) matchPattern(kind string, pattern URLPattern, path string, parsed *url.URL) (PageURL, bool) {
	re, err := regexp.Compile(pattern.Expr)
	if err != nil {
		return PageURL{}, false
	}
	matches := re.FindStringSubmatchIndex(path)
	if matches == nil {
		return PageURL{}, false
	}
	bookID := expandPatternValue(pattern.BookID, "$1", re, path, matches)
	chapterID := expandPatternValue(pattern.ChapterID, "", re, path, matches)
	if kind == ContentTypeChapter && chapterID == "" {
		if len(matches) >= 6 {
			chapterID = expandPatternValue("", "$2", re, path, matches)
		} else {
			chapterID = bookID
		}
	}
	if pattern.BookIDJoin != "" {
		bookID = expandPatternValue(pattern.BookIDJoin, "", re, path, matches)
	}
	canonical := strings.TrimSpace(parsed.String())
	if pattern.Canonical != "" {
		expanded := expandPatternValue(pattern.Canonical, "", re, path, matches)
		canonical = joinBaseURL(s.BaseURL, expanded)
	}
	return PageURL{
		Kind:      kind,
		BookID:    strings.TrimSpace(bookID),
		ChapterID: strings.TrimSpace(chapterID),
		Canonical: canonical,
	}, true
}

func expandPatternValue(tmpl string, fallback string, re *regexp.Regexp, value string, matches []int) string {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		tmpl = fallback
	}
	if tmpl == "" {
		return ""
	}
	return string(re.ExpandString(nil, tmpl, value, matches))
}

func parseHTTPURL(rawURL string) (*url.URL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return nil, false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, false
	}
	if parsed.Hostname() == "" {
		return nil, false
	}
	return parsed, true
}
