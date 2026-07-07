package qq

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

type TVProfile struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Overview        string   `json:"overview"`
	PosterPath      string   `json:"poster_path"`
	BackdropPath    string   `json:"backdrop_path"`
	Seasons         []Season `json:"seasons"`
	NumberOfSeasons int      `json:"number_of_seasons"`
}

type SeasonProfile struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Overview        string    `json:"overview"`
	PosterPath      string    `json:"poster_path"`
	BackdropPath    string    `json:"backdrop_path"`
	AirDate         string    `json:"air_date"`
	SeasonNumber    int       `json:"season_number"`
	Genres          []string  `json:"genres"`
	OriginCountry   []string  `json:"origin_country"`
	NumberOfEpisode int       `json:"number_of_episode"`
	Episodes        []Episode `json:"episodes"`
	Persons         []Person  `json:"persons"`
}

type Season struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	PosterPath   string `json:"poster_path"`
	SeasonNumber int    `json:"season_number"`
}

type Episode struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	Thumbnail     string `json:"thumbnail"`
	AirDate       string `json:"air_date"`
	EpisodeNumber int    `json:"episode_number"`
	Duration      int    `json:"duration"`
}

type Person struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

type VideoPageURL struct {
	Raw       string `json:"raw"`
	Canonical string `json:"canonical"`
	CID       string `json:"cid"`
	VID       string `json:"vid"`
}

type VideoDetailPage struct {
	URL              VideoPageURL   `json:"url"`
	APIURL           string         `json:"api_url"`
	CID              string         `json:"cid"`
	VID              string         `json:"vid"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	Year             string         `json:"year"`
	AreaName         string         `json:"area_name"`
	TypeName         string         `json:"type_name"`
	Genres           []string       `json:"genres"`
	EpisodeAll       int            `json:"episode_all"`
	DetailInfo       string         `json:"detail_info"`
	UpdateNotifyDesc string         `json:"update_notify_desc"`
	Score            string         `json:"score"`
	Hot              string         `json:"hot"`
	CoverURL         string         `json:"cover_url"`
	VerticalCoverURL string         `json:"vertical_cover_url"`
	TitleImageURL    string         `json:"title_image_url"`
	CurrentEpisode   *VideoEpisode  `json:"current_episode,omitempty"`
	Episodes         []VideoEpisode `json:"episodes"`
}

type VideoEpisode struct {
	CID           string `json:"cid"`
	VID           string `json:"vid"`
	Title         string `json:"title"`
	PlayTitle     string `json:"play_title"`
	UnionTitle    string `json:"union_title"`
	Subtitle      string `json:"subtitle"`
	ImageURL      string `json:"image_url"`
	PublishDate   string `json:"publish_date"`
	EpisodeNumber int    `json:"episode_number"`
	Duration      int    `json:"duration"`
	IsTrailer     bool   `json:"is_trailer"`
	URL           string `json:"url"`
}

type piniaState struct {
	Global struct {
		CurrentVid string `json:"currentVid"`
		CurrentCid string `json:"currentCid"`
		CoverInfo  struct {
			CoverID      string   `json:"cover_id"`
			TypeName     string   `json:"type_name"`
			PublishDate  string   `json:"publish_date"`
			EpisodeAll   string   `json:"episode_all"`
			Description  string   `json:"description"`
			Title        string   `json:"title"`
			AreaName     string   `json:"area_name"`
			NewPicHz     string   `json:"new_pic_hz"`
			Alias        []string `json:"alias"`
			LeadingActor []string `json:"leading_actor"`
		} `json:"coverInfo"`
	} `json:"global"`
	EpisodeMain episodeMainState `json:"episodeMain"`
}

type episodeMainState struct {
	EpTabs []struct {
		IsSelected  bool   `json:"isSelected"`
		Text        string `json:"text"`
		PageContext string `json:"pageContext"`
	} `json:"epTabs"`
	ListData []struct {
		List []episodeList `json:"list"`
	} `json:"listData"`
}

type episodeList []struct {
	CID         string `json:"cid"`
	Index       int    `json:"index"`
	Pic         string `json:"pic"`
	PicVertial  string `json:"picVertial"`
	Title       string `json:"title"`
	VID         string `json:"vid"`
	Duration    int    `json:"duration"`
	PublishDate string `json:"publishDate"`
}

type pageServiceResponse struct {
	Data pageServiceData `json:"data"`
	Ret  int             `json:"ret"`
	Msg  string          `json:"msg"`
}

type pageServiceData struct {
	CardList      []pageCard        `json:"CardList"`
	OtherPageInfo map[string]string `json:"other_page_info"`
}

type pageCard struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Params       map[string]string        `json:"params"`
	ChildrenList map[string]pageCardGroup `json:"children_list"`
}

type pageCardGroup struct {
	Cards []pageCard `json:"cards"`
}
