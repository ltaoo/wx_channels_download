package nfo

import (
	"encoding/xml"
)

type NfoClient struct {
}

func NewNFOClient() *NfoClient {
	return &NfoClient{}
}

// Create generates NFO XML string from the provided struct (Movie, TVShow, or Episode)
func (c *NfoClient) Create(v interface{}) (string, error) {
	output, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return xml.Header + string(output), nil
}

// ---------------------------------------------------------
// Shared Structures
// ---------------------------------------------------------

type UniqueID struct {
	Type    string `xml:"type,attr,omitempty"`
	Default bool   `xml:"default,attr,omitempty"`
	ID      string `xml:",chardata"`
}

type Rating struct {
	Name    string  `xml:"name,attr,omitempty"`
	Max     float32 `xml:"max,attr,omitempty"`
	Default bool    `xml:"default,attr,omitempty"`
	Value   float32 `xml:"value,omitempty"`
	Votes   int     `xml:"votes,omitempty"`
}

type Thumb struct {
	Aspect  string `xml:"aspect,attr,omitempty"`
	Preview string `xml:"preview,attr,omitempty"`
	URL     string `xml:",chardata"`
}

type Fanart struct {
	Thumbs []Thumb `xml:"thumb,omitempty"`
}

type Actor struct {
	Name  string `xml:"name,omitempty"`
	Role  string `xml:"role,omitempty"`
	Order int    `xml:"order,omitempty"`
	Thumb string `xml:"thumb,omitempty"`
}

type FileInfo struct {
	StreamDetails StreamDetails `xml:"streamdetails,omitempty"`
}

type StreamDetails struct {
	Video    []VideoDetail    `xml:"video,omitempty"`
	Audio    []AudioDetail    `xml:"audio,omitempty"`
	Subtitle []SubtitleDetail `xml:"subtitle,omitempty"`
}

type VideoDetail struct {
	Codec         string  `xml:"codec,omitempty"`
	Aspect        float32 `xml:"aspect,omitempty"`
	Width         int     `xml:"width,omitempty"`
	Height        int     `xml:"height,omitempty"`
	DurationInSec int     `xml:"durationinseconds,omitempty"`
	StereoMode    string  `xml:"stereomode,omitempty"`
}

type AudioDetail struct {
	Codec    string `xml:"codec,omitempty"`
	Language string `xml:"language,omitempty"`
	Channels int    `xml:"channels,omitempty"`
}

type SubtitleDetail struct {
	Language string `xml:"language,omitempty"`
}

// ---------------------------------------------------------
// Movie NFO
// ---------------------------------------------------------

// Movie represents a movie nfo
type Movie struct {
	XMLName       xml.Name   `xml:"movie"`
	Title         string     `xml:"title,omitempty"`         // 标题
	OriginalTitle string     `xml:"originaltitle,omitempty"` // 原名
	SortTitle     string     `xml:"sorttitle,omitempty"`     // 排序标题
	EpBookmark    string     `xml:"epbookmark,omitempty"`    // 书签
	Year          int        `xml:"year,omitempty"`          // 年份
	Ratings       []Rating   `xml:"rating,omitempty"`        // 评分列表
	UserRating    float32    `xml:"userrating,omitempty"`    // 用户评分
	Top250        int        `xml:"top250,omitempty"`        // Top 250 排名
	Set           *Set       `xml:"set,omitempty"`           // 电影集
	Plot          string     `xml:"plot,omitempty"`          // 剧情简介
	Outline       string     `xml:"outline,omitempty"`       // 内容大纲
	Tagline       string     `xml:"tagline,omitempty"`       // 标语/副标题
	Runtime       int        `xml:"runtime,omitempty"`       // 运行时间（分钟）
	Thumbs        []Thumb    `xml:"thumb,omitempty"`         // 缩略图/海报
	Fanart        *Fanart    `xml:"fanart,omitempty"`        // 剧照/同人画
	Mpaa          string     `xml:"mpaa,omitempty"`          // 分级
	Certification string     `xml:"certification,omitempty"` // 认证
	UniqueIDs     []UniqueID `xml:"uniqueid,omitempty"`      // 唯一标识符（IMDb, TMDB 等）
	Genres        []string   `xml:"genre,omitempty"`         // 类型
	Country       string     `xml:"country,omitempty"`       // 国家
	Premiered     string     `xml:"premiered,omitempty"`     // 首映日期
	Aired         string     `xml:"aired,omitempty"`         // 播出日期
	Watched       bool       `xml:"watched,omitempty"`       // 是否已看
	PlayCount     int        `xml:"playcount,omitempty"`     // 播放次数
	Studios       []string   `xml:"studio,omitempty"`        // 制片公司
	Credits       []string   `xml:"credits,omitempty"`       // 编剧
	Directors     []string   `xml:"director,omitempty"`      // 导演
	Tags          []string   `xml:"tag,omitempty"`           // 标签
	Actors        []Actor    `xml:"actor,omitempty"`         // 演员列表
	Trailer       string     `xml:"trailer,omitempty"`       // 预告片链接
	DateAdded     string     `xml:"dateadded,omitempty"`     // 添加日期
	FileInfo      *FileInfo  `xml:"fileinfo,omitempty"`      // 文件信息
}

type Set struct {
	Name     string `xml:"name,omitempty"`
	Overview string `xml:"overview,omitempty"`
}

// ---------------------------------------------------------
// TV Show NFO
// ---------------------------------------------------------

// TVShow represents a tvshow nfo
type TVShow struct {
	XMLName        xml.Name      `xml:"tvshow"`
	Title          string        `xml:"title,omitempty"`
	OriginalTitle  string        `xml:"originaltitle,omitempty"`
	ShowTitle      string        `xml:"showtitle,omitempty"`
	SortTitle      string        `xml:"sorttitle,omitempty"`
	Ratings        []Rating      `xml:"rating,omitempty"`
	UserRating     float32       `xml:"userrating,omitempty"`
	Year           int           `xml:"year,omitempty"`
	Top250         int           `xml:"top250,omitempty"`
	Season         int           `xml:"season,omitempty"`
	Episode        int           `xml:"episode,omitempty"`
	DisplaySeason  int           `xml:"displayseason,omitempty"`
	DisplayEpisode int           `xml:"displayepisode,omitempty"`
	Plot           string        `xml:"plot,omitempty"`
	Outline        string        `xml:"outline,omitempty"`
	Tagline        string        `xml:"tagline,omitempty"`
	Runtime        int           `xml:"runtime,omitempty"`
	Thumbs         []Thumb       `xml:"thumb,omitempty"`
	Fanart         *Fanart       `xml:"fanart,omitempty"`
	Mpaa           string        `xml:"mpaa,omitempty"`
	Certification  string        `xml:"certification,omitempty"`
	EpBookmark     string        `xml:"epbookmark,omitempty"`
	UniqueIDs      []UniqueID    `xml:"uniqueid,omitempty"`
	Genres         []string      `xml:"genre,omitempty"`
	Premiered      string        `xml:"premiered,omitempty"`
	Status         string        `xml:"status,omitempty"`
	Studios        []string      `xml:"studio,omitempty"`
	Tags           []string      `xml:"tag,omitempty"`
	Actors         []Actor       `xml:"actor,omitempty"`
	NamedSeasons   []NamedSeason `xml:"namedseason,omitempty"`
	DateAdded      string        `xml:"dateadded,omitempty"`
}

type NamedSeason struct {
	Number int    `xml:"number,attr"`
	Name   string `xml:",chardata"`
}

// ---------------------------------------------------------
// Episode NFO
// ---------------------------------------------------------

// Episode represents an episodedetails nfo
type Episode struct {
	XMLName        xml.Name   `xml:"episodedetails"`
	Title          string     `xml:"title,omitempty"`
	OriginalTitle  string     `xml:"originaltitle,omitempty"`
	ShowTitle      string     `xml:"showtitle,omitempty"`
	Season         int        `xml:"season,omitempty"`
	Episode        int        `xml:"episode,omitempty"`
	DisplaySeason  int        `xml:"displayseason,omitempty"`
	DisplayEpisode int        `xml:"displayepisode,omitempty"`
	Ratings        []Rating   `xml:"rating,omitempty"`
	UserRating     float32    `xml:"userrating,omitempty"`
	Plot           string     `xml:"plot,omitempty"`
	Outline        string     `xml:"outline,omitempty"`
	Runtime        int        `xml:"runtime,omitempty"`
	Thumbs         []Thumb    `xml:"thumb,omitempty"`
	Mpaa           string     `xml:"mpaa,omitempty"`
	UniqueIDs      []UniqueID `xml:"uniqueid,omitempty"`
	Aired          string     `xml:"aired,omitempty"`
	Watched        bool       `xml:"watched,omitempty"`
	PlayCount      int        `xml:"playcount,omitempty"`
	Studios        []string   `xml:"studio,omitempty"`
	Credits        []string   `xml:"credits,omitempty"`
	Directors      []string   `xml:"director,omitempty"`
	Actors         []Actor    `xml:"actor,omitempty"`
	DateAdded      string     `xml:"dateadded,omitempty"`
	FileInfo       *FileInfo  `xml:"fileinfo,omitempty"`
}
