package iqiyi

type SearchResult struct {
	List []SearchItem `json:"list"`
}

type SearchItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	OriginalName  string   `json:"original_name"`
	Overview      string   `json:"overview"`
	PosterPath    string   `json:"poster_path"`
	BackdropPath  string   `json:"backdrop_path"`
	FirstAirDate  string   `json:"first_air_date"`
	OriginCountry []string `json:"origin_country"`
	Type          string   `json:"type"`
	Source        string   `json:"source"`
}

type ProfileWithSeasons struct {
	Platform     string   `json:"platform"`
	Type         string   `json:"type"`
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Overview     string   `json:"overview"`
	PosterPath   string   `json:"poster_path"`
	BackdropPath string   `json:"backdrop_path"`
	OriginalName string   `json:"original_name"`
	Seasons      []Season `json:"seasons"`
}

type SeasonProfile struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	OriginalName  string    `json:"original_name"`
	Overview      string    `json:"overview"`
	PosterPath    string    `json:"poster_path"`
	BackdropPath  string    `json:"backdrop_path"`
	Episodes      []Episode `json:"episodes"`
	AirDate       string    `json:"air_date"`
	Genres        []string  `json:"genres"`
	OriginCountry []string  `json:"origin_country"`
	Persons       []Person  `json:"persons"`
}

type Season struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Overview      string    `json:"overview,omitempty"`
	PosterPath    string    `json:"poster_path,omitempty"`
	AirDate       string    `json:"air_date"`
	Episodes      []Episode `json:"episodes,omitempty"`
	Genres        []string  `json:"genres,omitempty"`
	OriginCountry []string  `json:"origin_country,omitempty"`
	Persons       []Person  `json:"persons,omitempty"`
}

type Episode struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name"`
	AirDate       string `json:"air_date"`
	EpisodeNumber int    `json:"episode_number"`
	Thumbnail     string `json:"thumbnail"`
}

type Person struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Avatar     string   `json:"avatar"`
	Character  []string `json:"character"`
	Department string   `json:"department"`
	Order      int      `json:"order"`
}

type ProfilePageInfo struct {
	TVID          int64                  `json:"tvId"`
	AlbumID       int64                  `json:"albumId"`
	ChannelID     int                    `json:"channelId"`
	Description   string                 `json:"description"`
	Subtitle      string                 `json:"subtitle"`
	ImageURL      string                 `json:"imageUrl"`
	AlbumImageURL string                 `json:"albumImageUrl"`
	VideoCount    string                 `json:"videoCount"`
	Duration      string                 `json:"duration"`
	DurationSec   int                    `json:"durationSec"`
	Period        string                 `json:"period"`
	Categories    []Category             `json:"categories"`
	AlbumName     string                 `json:"albumName"`
	Score         float64                `json:"score"`
	People        map[string][]RawPerson `json:"people"`
}

type Category struct {
	ID      int64  `json:"id"`
	QipuID  int64  `json:"qipuId"`
	Name    string `json:"name"`
	SubType int    `json:"subType"`
	SubName string `json:"subName"`
	URL     string `json:"url"`
}

type RawPerson struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Character []string `json:"character"`
	ImageURL  string   `json:"image_url"`
}

type BaseInfo struct {
	ID           int64    `json:"_id"`
	ShareURL     string   `json:"share_url"`
	Title        string   `json:"title"`
	Desc         string   `json:"desc"`
	ImageURL     string   `json:"image_url"`
	PublishDate  string   `json:"publish_date"`
	TotalEpisode int      `json:"total_episode"`
	Seasons      []Season `json:"seasons"`
}

type baseInfoResponse struct {
	StatusCode int    `json:"status_code"`
	Msg        string `json:"msg"`
	Data       struct {
		BaseData struct {
			ID           int64  `json:"_id"`
			ShareURL     string `json:"share_url"`
			Title        string `json:"title"`
			Desc         string `json:"desc"`
			ImageURL     string `json:"image_url"`
			PublishDate  string `json:"publish_date"`
			TotalEpisode int    `json:"total_episode"`
		} `json:"base_data"`
		Template baseInfoTemplate `json:"template"`
	} `json:"data"`
}

type baseInfoTemplate struct {
	PureData struct {
		SelectorBK       []seasonSource `json:"selector_bk"`
		SourceSelectorBK []seasonSource `json:"source_selector_bk"`
	} `json:"pure_data"`
	Tabs []baseInfoTab `json:"tabs"`
}

type baseInfoTab struct {
	Blocks []baseInfoBlock `json:"blocks"`
}

type baseInfoBlock struct {
	BKID string `json:"bk_id"`
	Data struct {
		Data any `json:"data"`
	} `json:"data"`
}

type seasonSource struct {
	Videos   any    `json:"videos"`
	TabName  string `json:"tab_name"`
	Order    int    `json:"order"`
	EntityID int64  `json:"entity_id"`
}

type selectorResponse struct {
	Data struct {
		Videos   any   `json:"videos"`
		EntityID int64 `json:"entity_id"`
	} `json:"data"`
}

type videoItem struct {
	PageURL          string `json:"page_url"`
	ImageURL         string `json:"image_url"`
	ShortDisplayName string `json:"short_display_name"`
	Title            string `json:"title"`
	PublishDate      string `json:"publish_date"`
	ContentType      int    `json:"content_type"`
	AlbumOrder       int    `json:"album_order"`
}
