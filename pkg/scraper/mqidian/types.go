package mqidian

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const (
	PlatformID = "mqidian"
	BaseURL    = "https://m.qidian.com"
)

type URLParts struct {
	BookID    string
	Canonical string
}

type BookVolume struct {
	Idx      int       `json:"idx"`
	Title    string    `json:"title"`
	Chapters []Chapter `json:"chapters"`
}

type Chapter struct {
	Idx         int       `json:"idx"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Locked      bool      `json:"locked,omitempty"`
	WordCount   int64     `json:"word_count,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
}

type Author struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Avatar string `json:"avatar,omitempty"`
	Desc   string `json:"desc,omitempty"`
}

type BookProfile struct {
	URL              string           `json:"url"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	Slogan           string           `json:"slogan"`
	CoverURL         string           `json:"cover_url"`
	LatestUpdateAt   time.Time        `json:"latest_update_at,omitempty"`
	Tags             []string         `json:"tags,omitempty"`
	LatestChapter    Chapter          `json:"latest_chapter"`
	ChapterCount     int              `json:"chapter_count"`
	WordCount        int64            `json:"word_count,omitempty"`
	DisplayWordCount string           `json:"display_word_count,omitempty"`
	Category         string           `json:"category,omitempty"`
	SubCategory      string           `json:"sub_category,omitempty"`
	Status           string           `json:"status,omitempty"`
	Author           Author           `json:"author"`
	Volumes          []BookVolume     `json:"volumes,omitempty"`
	PageContext      *PageContextRoot `json:"-"`
	PageContextJSON  json.RawMessage  `json:"-"`
	PageHTML         string           `json:"-"`
}

type PageContextRoot struct {
	PageContext PageContext     `json:"pageContext"`
	Raw         json.RawMessage `json:"-"`
}

type PageContext struct {
	PageID       string          `json:"_pageId"`
	PageProps    PageProps       `json:"pageProps"`
	URLPathname  string          `json:"urlPathname"`
	URLOriginal  string          `json:"urlOriginal"`
	Hostname     string          `json:"hostname"`
	RouteParams  json.RawMessage `json:"routeParams,omitempty"`
	InitialState json.RawMessage `json:"INITIAL_STATE,omitempty"`
}

type PageProps struct {
	PageData   PageData        `json:"pageData"`
	ConfigData json.RawMessage `json:"configData,omitempty"`
}

type PageData struct {
	BookInfo        PageBookInfo    `json:"bookInfo"`
	BookID          int64           `json:"bookId"`
	BookName        string          `json:"bookName"`
	BookStatus      string          `json:"bookStatus"`
	ActionStatus    string          `json:"actionStatus"`
	UpdateTime      string          `json:"updateTime"`
	ChapterCount    int             `json:"cTCnt"`
	ChapterTotalCnt int             `json:"chapterTotalCnt"`
	RecentChapters  []RecentChapter `json:"recentChapters"`
	CatalogVolumes  []CatalogVolume `json:"vs"`
	AuthorInfo      PageAuthorInfo  `json:"authorInfo"`
	BookExtra       PageBookExtra   `json:"bookExtra"`
	BookAlbum       json.RawMessage `json:"bookAlbum,omitempty"`
	Roles           []PageRole      `json:"roles,omitempty"`
}

type PageBookInfo struct {
	BookID         int64  `json:"bookId"`
	BookName       string `json:"bookName"`
	Desc           string `json:"desc"`
	AuthorID       int64  `json:"authorId"`
	CAuthorID      string `json:"cAuthorId"`
	AuthorName     string `json:"authorName"`
	UpdChapterID   int64  `json:"updChapterId"`
	UpdChapterName string `json:"updChapterName"`
	UpdChapterURL  string `json:"updChapterUrl"`
	UpdTimes       int64  `json:"updTimes"`
	UpdTime        string `json:"updTime"`
	ChanName       string `json:"chanName"`
	SubCateName    string `json:"subCateName"`
	ChanURL        string `json:"chanUrl"`
	BookStatus     string `json:"bookStatus"`
	ActionStatus   string `json:"actionStatus"`
	SignStatus     string `json:"signStatus"`
	WordsCnt       int64  `json:"wordsCnt"`
	ShowWordsCnt   string `json:"showWordsCnt"`
}

type RecentChapter struct {
	ID          int64  `json:"id"`
	UUID        int64  `json:"uuid"`
	Name        string `json:"cN"`
	URL         string `json:"cU"`
	UpdateTime  string `json:"uT"`
	UpdateTime2 string `json:"uTm"`
	WordCount   int64  `json:"cnt"`
}

type CatalogVolume struct {
	ID           int64            `json:"vId"`
	Name         string           `json:"vN"`
	ChapterCount int              `json:"cCnt"`
	Chapters     []CatalogChapter `json:"cs"`
}

type CatalogChapter struct {
	ID         int64  `json:"id"`
	UUID       int64  `json:"uuid"`
	Name       string `json:"cN"`
	URL        string `json:"cU"`
	UpdateTime string `json:"uT"`
	WordCount  int64  `json:"cnt"`
	FreeStatus int    `json:"sS"`
}

type PageAuthorInfo struct {
	AuthorID       FlexibleInt64 `json:"authorId"`
	AuthorName     string        `json:"authorName"`
	Name           string        `json:"name"`
	AuthorNickName string        `json:"authorNickName"`
	Avatar         string        `json:"avatar"`
	Desc           string        `json:"desc"`
	Rank           string        `json:"rank"`
	AuthorLevel    string        `json:"authorLevel"`
}

type PageBookExtra struct {
	TagInfo      PageTagInfo     `json:"tagInfo"`
	UGCTagInfos  []PageUGCTag    `json:"ugcTagInfos"`
	FinishStatus json.RawMessage `json:"finishStatus,omitempty"`
}

type PageTagInfo struct {
	RankName string `json:"rankName"`
	RankNum  string `json:"rankNum"`
}

type PageUGCTag struct {
	TagName  string `json:"tagName"`
	TagName2 string `json:"TagName"`
}

type PageRole struct {
	RoleID   string `json:"roleId"`
	RoleName string `json:"roleName"`
}

type FlexibleInt64 int64

func (v *FlexibleInt64) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*v = 0
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		unquoted, err := strconv.Unquote(text)
		if err != nil {
			return err
		}
		text = strings.TrimSpace(unquoted)
	}
	if text == "" {
		*v = 0
		return nil
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*v = FlexibleInt64(n)
	return nil
}

func (v FlexibleInt64) Int64() int64 {
	return int64(v)
}
