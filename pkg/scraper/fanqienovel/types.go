package fanqienovel

import (
	"encoding/json"
	"time"
)

const BaseURL = "https://fanqienovel.com"

type Author struct {
	Name      string `json:"name"`
	Desc      string `json:"desc,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	URL       string `json:"url,omitempty"`
}

type BookVolume struct {
	Idx      int       `json:"idx"`
	Title    string    `json:"title"`
	Chapters []Chapter `json:"chapters"`
}

type Chapter struct {
	Idx   int    `json:"idx"`
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type BookProfile struct {
	URL              string          `json:"url"`
	Title            string          `json:"title"`
	Description      string          `json:"description"`
	Slogan           string          `json:"slogan"`
	CoverURL         string          `json:"cover_url"`
	LatestUpdateAt   *time.Time      `json:"latest_update_at,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
	LatestChapter    Chapter         `json:"latest_chapter"`
	ChapterCount     int             `json:"chapter_count"`
	Author           Author          `json:"author"`
	Volumes          []BookVolume    `json:"volumes,omitempty"`
	InitialStateJSON json.RawMessage `json:"-"`
}

type ChapterContent struct {
	Title            string          `json:"title"`
	PublishAt        *time.Time      `json:"publish_at,omitempty"`
	Content          string          `json:"content"`
	WorkCount        string          `json:"work_count,omitempty"`
	InitialStateJSON json.RawMessage `json:"-"`
}

type InitialState struct {
	Raw    json.RawMessage `json:"-"`
	Page   PageState       `json:"page"`
	Reader ReaderState     `json:"reader"`
}

type PageState struct {
	Author                string         `json:"author"`
	AuthorID              string         `json:"authorId"`
	BookID                string         `json:"bookId"`
	MediaID               string         `json:"mediaId"`
	BookName              string         `json:"bookName"`
	Status                int            `json:"status"`
	Category              string         `json:"category"`
	CategoryV2            string         `json:"categoryV2"`
	Abstract              string         `json:"abstract"`
	ThumbURI              string         `json:"thumbUri"`
	CreationStatus        int            `json:"creationStatus"`
	WordNumber            int64          `json:"wordNumber"`
	ReadCount             int64          `json:"readCount"`
	Description           string         `json:"description"`
	AvatarURI             string         `json:"avatarUri"`
	CreatorID             string         `json:"creatorId"`
	LastPublishTime       string         `json:"lastPublishTime"`
	LastChapterItemID     string         `json:"lastChapterItemId"`
	LastChapterTitle      string         `json:"lastChapterTitle"`
	VolumeNameList        []string       `json:"volumeNameList"`
	ChapterListWithVolume []VolumeState  `json:"chapterListWithVolume"`
	ChapterTotal          int            `json:"chapterTotal"`
	ItemIDs               []string       `json:"itemIds"`
	ChapterList           []ChapterState `json:"chapterList"`
	AuthorName            string         `json:"authorName"`
	ThumbURL              string         `json:"thumbUrl"`
	SourceURI             string         `json:"sourceUri"`
	OriginalAuthors       string         `json:"originalAuthors"`
}

type VolumeState struct {
	VolumeName  string         `json:"volumeName"`
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	ChapterList []ChapterState `json:"chapterList"`
	Chapters    []ChapterState `json:"chapters"`
	ItemList    []ChapterState `json:"itemList"`
}

type ChapterState struct {
	VolumeName string `json:"volumeName"`
	ItemID     string `json:"itemId"`
	Title      string `json:"title"`
	NeedPay    int    `json:"needPay"`
	Order      int    `json:"order"`
}

type ReaderState struct {
	ChapterData     ReaderChapterData `json:"chapterData"`
	PreChapterData  ReaderChapterData `json:"preChapterData"`
	NextChapterData ReaderChapterData `json:"nextChapterData"`
	CatalogData     any               `json:"catalogData"`
}

type ReaderChapterData struct {
	ItemID            string `json:"itemId"`
	ChapterID         string `json:"chapterId"`
	Title             string `json:"title"`
	ChapterName       string `json:"chapterName"`
	Content           string `json:"content"`
	ChapterWordNumber any    `json:"chapterWordNumber"`
	WordNumber        any    `json:"wordNumber"`
	WordCount         any    `json:"wordCount"`
	PublishTime       any    `json:"publishTime"`
	CreateTime        any    `json:"createTime"`
}
