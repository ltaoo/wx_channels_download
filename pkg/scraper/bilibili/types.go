package bilibili

// VideoInfo holds Bilibili video information.
type VideoInfo struct {
	URL      string // video playback URL
	AudioURL string // audio URL (separate in DASH format)
	Title    string // video title
	VideoID  string // video ID (BV number or episode number)
	CoverURL string // cover image URL
	Page     int    // part/page number
	Source   string // source identifier
}

// ViewResponse is the video info API response (x/web-interface/view).
type ViewResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    VideoData `json:"data"`
}

// VideoData holds video metadata.
type VideoData struct {
	Bvid        string `json:"bvid"`
	Aid         int64  `json:"aid"`
	Cid         int64  `json:"cid"`
	Title       string `json:"title"`
	Pic         string `json:"pic"`
	Desc        string `json:"desc"`
	Owner       Owner  `json:"owner"`
	Pages       []Page `json:"pages"`
	RedirectURL string `json:"redirect_url"`
}

// Owner holds uploader information.
type Owner struct {
	Mid  int64  `json:"mid"`
	Name string `json:"name"`
	Face string `json:"face"`
}

// Page holds part/page information.
type Page struct {
	Cid      int64  `json:"cid"`
	Page     int    `json:"page"`
	Part     string `json:"part"`
	Duration int    `json:"duration"`
}

// PlayURLResponse is the playback URL API response (x/player/playurl).
type PlayURLResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    PlayURLData `json:"data"`
}

// PlayURLData holds playback URL data.
type PlayURLData struct {
	From    string     `json:"from"`
	Quality int        `json:"quality"`
	Format  string     `json:"format"`
	Durl    []DurlItem `json:"durl"`
	Dash    DashInfo   `json:"dash"`
}

// DurlItem is a regular video stream segment.
type DurlItem struct {
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// DashInfo holds DASH format information.
type DashInfo struct {
	Video []DashItem `json:"video"`
	Audio []DashItem `json:"audio"`
}

// DashItem is a DASH streaming media item.
type DashItem struct {
	ID      int    `json:"id"`
	BaseURL string `json:"base_url"`
	Codecs  string `json:"codecs"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Size    int64  `json:"size"`
}

// PGCSeasonResponse is the bangumi season API response (pgc/view/web/season).
type PGCSeasonResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Result  PGCResult `json:"result"`
}

// PGCResult holds bangumi result data.
type PGCResult struct {
	Actors      string        `json:"actors"`
	Alias       string        `json:"alias"`
	Areas       []PGCArea     `json:"areas"`
	Background  string        `json:"bkg_cover"`
	Cover       string        `json:"cover"`
	Episodes    []PGCEpisode  `json:"episodes"`
	Evaluate    string        `json:"evaluate"`
	JPTitle     string        `json:"jp_title"`
	Link        string        `json:"link"`
	MediaID     int64         `json:"media_id"`
	Publish     PGCPublish    `json:"publish"`
	Rating      *PGCRating    `json:"rating"`
	SeasonID    int64         `json:"season_id"`
	SeasonTitle string        `json:"season_title"`
	Section     []PGCSection  `json:"section"`
	ShareURL    string        `json:"share_url"`
	SquareCover string        `json:"square_cover"`
	Staff       string        `json:"staff"`
	Stat        PGCSeasonStat `json:"stat"`
	Status      int           `json:"status"`
	Styles      []string      `json:"styles"`
	Subtitle    string        `json:"subtitle"`
	Title       string        `json:"title"`
	Total       int           `json:"total"`
	Type        int           `json:"type"`
}

// PGCArea is one production/origin area associated with a season.
type PGCArea struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// PGCPublish describes a season's release and production status.
type PGCPublish struct {
	IsFinish      int    `json:"is_finish"`
	IsStarted     int    `json:"is_started"`
	PublishTime   string `json:"pub_time"`
	PublishTimeUI string `json:"pub_time_show"`
	UnknownDate   int    `json:"unknow_pub_date"`
	Weekday       int    `json:"weekday"`
}

// PGCRating contains the public season score.
type PGCRating struct {
	Count int     `json:"count"`
	Score float64 `json:"score"`
}

// PGCSeasonStat contains season-wide engagement counters.
type PGCSeasonStat struct {
	Coins     int64 `json:"coins"`
	Danmakus  int64 `json:"danmakus"`
	Favorite  int64 `json:"favorite"`
	Favorites int64 `json:"favorites"`
	Likes     int64 `json:"likes"`
	Reply     int64 `json:"reply"`
	Share     int64 `json:"share"`
	Views     int64 `json:"views"`
	VT        int64 `json:"vt"`
}

// PGCEpisode holds a bangumi episode.
type PGCEpisode struct {
	AID         int64  `json:"aid"`
	BVID        string `json:"bvid"`
	CID         int64  `json:"cid"`
	Cover       string `json:"cover"`
	Duration    int64  `json:"duration"`
	EpID        int64  `json:"ep_id"`
	ID          int64  `json:"id"`
	Link        string `json:"link"`
	LongTitle   string `json:"long_title"`
	PublishTime int64  `json:"pub_time"`
	ReleaseDate string `json:"release_date"`
	ShareCopy   string `json:"share_copy"`
	ShareURL    string `json:"share_url"`
	ShowTitle   string `json:"show_title"`
	Title       string `json:"title"`
}

// TargetEpisode returns one episode by its public ep_id.
func (response *PGCSeasonResponse) TargetEpisode(ep_id int64) *PGCEpisode {
	if response == nil || ep_id <= 0 {
		return nil
	}
	for episode_index := range response.Result.Episodes {
		episode := &response.Result.Episodes[episode_index]
		if episode.EpID == ep_id {
			return episode
		}
	}
	for section_index := range response.Result.Section {
		section := &response.Result.Section[section_index]
		for episode_index := range section.Episodes {
			episode := &section.Episodes[episode_index]
			if episode.EpID == ep_id {
				return episode
			}
		}
	}
	return nil
}

// PGCSection holds a bangumi section.
type PGCSection struct {
	Episodes []PGCEpisode `json:"episodes"`
}

// PGCSeasonSectionResponse is the bangumi season section response (pgc/web/season/section).
type PGCSeasonSectionResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Result  PGCSeasonSection `json:"result"`
}

// PGCSeasonSection holds a bangumi season section.
type PGCSeasonSection struct {
	MainSection PGCMainSection `json:"main_section"`
	Section     []PGCSection   `json:"section"`
}

// PGCMainSection holds the bangumi main section.
type PGCMainSection struct {
	Episodes []PGCEpisode `json:"episodes"`
}

// PGCPlayURLResponse is the bangumi playback URL response (pgc/player/web/v2/playurl).
type PGCPlayURLResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Result  PGCPlayData `json:"result"`
}

// PGCPlayData holds bangumi playback data.
type PGCPlayData struct {
	VideoInfo PGCVideoInfo `json:"video_info"`
}

// PGCVideoInfo holds bangumi video information.
type PGCVideoInfo struct {
	Dash DashInfo `json:"dash"`
}

// PUGVSeasonResponse is the course API response (pugv/view/web/season).
type PUGVSeasonResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    PUGVSeasonData `json:"data"`
}

// PUGVSeasonData holds course data.
type PUGVSeasonData struct {
	Episodes []PUGVEpisode `json:"episodes"`
}

// PUGVEpisode holds a course episode.
type PUGVEpisode struct {
	ID    int64  `json:"id"`
	Aid   int64  `json:"aid"`
	Cid   int64  `json:"cid"`
	Title string `json:"title"`
	Cover string `json:"cover"`
}

// PUGVPlayURLResponse is the course playback URL response (pugv/player/web/playurl).
type PUGVPlayURLResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    PUGVPlayData `json:"data"`
}

// PUGVPlayData holds course playback data.
type PUGVPlayData struct {
	Dash DashInfo `json:"dash"`
}
