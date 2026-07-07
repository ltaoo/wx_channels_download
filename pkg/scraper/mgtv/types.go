package mgtv

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
	Platform        string   `json:"platform,omitempty"`
	Type            string   `json:"type,omitempty"`
	ID              string   `json:"id"`
	ClipID          string   `json:"clip_id,omitempty"`
	VideoID         string   `json:"video_id,omitempty"`
	Name            string   `json:"name"`
	Overview        string   `json:"overview"`
	PosterPath      string   `json:"poster_path"`
	BackdropPath    string   `json:"backdrop_path"`
	OriginalName    string   `json:"original_name"`
	Kind            string   `json:"kind,omitempty"`
	SourceURL       string   `json:"source_url,omitempty"`
	APIURL          string   `json:"api_url,omitempty"`
	CurrentEpisode  *Episode `json:"current_episode,omitempty"`
	Seasons         []Season `json:"seasons"`
	FirstAirDate    string   `json:"first_air_date"`
	VoteAverage     float64  `json:"vote_average"`
	Popularity      float64  `json:"popularity"`
	NumberOfSeasons int      `json:"number_of_seasons"`
	Status          string   `json:"status"`
}

type Season struct {
	ID           string    `json:"id,omitempty"`
	Name         string    `json:"name,omitempty"`
	PosterPath   string    `json:"poster_path,omitempty"`
	SeasonNumber int       `json:"season_number,omitempty"`
	VoteAverage  float64   `json:"vote_average"`
	Episodes     []Episode `json:"episodes,omitempty"`
}

type Episode struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	URL           string `json:"url,omitempty"`
	Thumbnail     string `json:"thumbnail"`
	Duration      string `json:"duration,omitempty"`
	EpisodeNumber int    `json:"episode_number"`
	AirDate       string `json:"air_date"`
}
