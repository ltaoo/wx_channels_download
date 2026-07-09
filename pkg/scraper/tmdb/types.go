package tmdb

type Language string

const (
	LanguageZhCN Language = "zh-CN"
	LanguageEnUS Language = "en-US"
)

type SearchTVResult struct {
	Page  int              `json:"page"`
	Total int              `json:"total"`
	List  []SearchedTVItem `json:"list"`
}

type SearchMovieResult struct {
	Page  int                 `json:"page"`
	Total int                 `json:"total"`
	List  []SearchedMovieItem `json:"list"`
}

type SearchedTVItem struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	OriginalName     string   `json:"original_name"`
	Overview         string   `json:"overview"`
	PosterPath       string   `json:"poster_path"`
	BackdropPath     string   `json:"backdrop_path"`
	FirstAirDate     string   `json:"first_air_date"`
	VoteAverage      float64  `json:"vote_average"`
	Popularity       float64  `json:"popularity"`
	NumberOfEpisodes int      `json:"number_of_episodes,omitempty"`
	NumberOfSeasons  int      `json:"number_of_seasons,omitempty"`
	InProduction     bool     `json:"in_production,omitempty"`
	NextEpisodeToAir string   `json:"next_episode_to_air,omitempty"`
	Genres           []Genre  `json:"genres,omitempty"`
	OriginCountry    []string `json:"origin_country"`
	Seasons          []Season `json:"seasons,omitempty"`
	Type             string   `json:"type"`
	Source           string   `json:"source"`
}

type SearchedMovieItem struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	OriginalName  string   `json:"original_name"`
	FirstAirDate  string   `json:"first_air_date"`
	AirDate       string   `json:"air_date"`
	Overview      string   `json:"overview"`
	PosterPath    string   `json:"poster_path"`
	BackdropPath  string   `json:"backdrop_path"`
	OriginCountry []string `json:"origin_country"`
	Type          string   `json:"type"`
	Source        string   `json:"source"`
}

type TVProfile = SearchedTVItem

type MovieProfile struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	OriginalName  string   `json:"original_name"`
	AirDate       string   `json:"air_date"`
	Overview      string   `json:"overview"`
	Status        string   `json:"status"`
	VoteAverage   float64  `json:"vote_average"`
	Popularity    float64  `json:"popularity"`
	Genres        []Genre  `json:"genres"`
	Runtime       int      `json:"runtime"`
	OriginCountry []string `json:"origin_country"`
	PosterPath    string   `json:"poster_path"`
	BackdropPath  string   `json:"backdrop_path"`
	Type          string   `json:"type"`
	Source        string   `json:"source"`
}

type SeasonProfile struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Overview     string    `json:"overview"`
	PosterPath   string    `json:"poster_path"`
	Number       int       `json:"number"`
	AirDate      string    `json:"air_date"`
	SeasonNumber int       `json:"season_number"`
	Episodes     []Episode `json:"episodes"`
}

type EpisodeProfile struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	StillPath     string `json:"still_path"`
	AirDate       string `json:"air_date"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	Runtime       int    `json:"runtime"`
}

type Season struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	AirDate      string  `json:"air_date"`
	EpisodeCount int     `json:"episode_count"`
	SeasonNumber int     `json:"season_number"`
	VoteAverage  float64 `json:"vote_average"`
}

type Episode struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	StillPath     string `json:"still_path"`
	AirDate       string `json:"air_date"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	Runtime       int    `json:"runtime"`
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Person struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Gender             int    `json:"gender,omitempty"`
	ProfilePath        string `json:"profile_path,omitempty"`
	KnownForDepartment string `json:"known_for_department,omitempty"`
	Order              int    `json:"order,omitempty"`
	Biography          string `json:"biography,omitempty"`
	PlaceOfBirth       string `json:"place_of_birth,omitempty"`
	Birthday           string `json:"birthday,omitempty"`
}

type tmdbImagePaths struct {
	BackdropPath string `json:"backdrop_path"`
	PosterPath   string `json:"poster_path"`
	ProfilePath  string `json:"profile_path"`
	StillPath    string `json:"still_path"`
}

func fixTMDBImagePath(paths tmdbImagePaths) tmdbImagePaths {
	out := tmdbImagePaths{}
	if paths.BackdropPath != "" {
		out.BackdropPath = "https://www.themoviedb.org/t/p/w1920_and_h800_multi_faces" + paths.BackdropPath
	}
	if paths.PosterPath != "" {
		out.PosterPath = "https://www.themoviedb.org/t/p/w600_and_h900_bestv2" + paths.PosterPath
	}
	if paths.ProfilePath != "" {
		out.ProfilePath = "https://www.themoviedb.org/t/p/w600_and_h900_bestv2" + paths.ProfilePath
	}
	if paths.StillPath != "" {
		out.StillPath = "https://www.themoviedb.org/t/p/w227_and_h127_bestv2" + paths.StillPath
	}
	return out
}

type searchTVResponse struct {
	Page         int `json:"page"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
	Results      []struct {
		ID            int      `json:"id"`
		Name          string   `json:"name"`
		OriginalName  string   `json:"original_name"`
		Overview      string   `json:"overview"`
		FirstAirDate  string   `json:"first_air_date"`
		OriginCountry []string `json:"origin_country"`
		Popularity    float64  `json:"popularity"`
		VoteAverage   float64  `json:"vote_average"`
		tmdbImagePaths
	} `json:"results"`
}

type searchMovieResponse struct {
	Page         int `json:"page"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
	Results      []struct {
		ID            int    `json:"id"`
		Title         string `json:"title"`
		OriginalTitle string `json:"original_title"`
		Overview      string `json:"overview"`
		ReleaseDate   string `json:"release_date"`
		tmdbImagePaths
	} `json:"results"`
}

type tvProfileResponse struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	OriginalName     string   `json:"original_name"`
	Overview         string   `json:"overview"`
	FirstAirDate     string   `json:"first_air_date"`
	Genres           []Genre  `json:"genres"`
	NumberOfEpisodes int      `json:"number_of_episodes"`
	NumberOfSeasons  int      `json:"number_of_seasons"`
	InProduction     bool     `json:"in_production"`
	NextEpisodeToAir string   `json:"next_episode_to_air"`
	OriginCountry    []string `json:"origin_country"`
	Popularity       float64  `json:"popularity"`
	Status           string   `json:"status"`
	VoteAverage      float64  `json:"vote_average"`
	Seasons          []struct {
		ID           int     `json:"id"`
		Name         string  `json:"name"`
		Overview     string  `json:"overview"`
		AirDate      string  `json:"air_date"`
		EpisodeCount int     `json:"episode_count"`
		SeasonNumber int     `json:"season_number"`
		VoteAverage  float64 `json:"vote_average"`
		tmdbImagePaths
	} `json:"seasons"`
	tmdbImagePaths
}

type seasonProfileResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	AirDate      string `json:"air_date"`
	SeasonNumber int    `json:"season_number"`
	Episodes     []struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		Overview      string `json:"overview"`
		AirDate       string `json:"air_date"`
		EpisodeNumber int    `json:"episode_number"`
		SeasonNumber  int    `json:"season_number"`
		Runtime       int    `json:"runtime"`
		tmdbImagePaths
	} `json:"episodes"`
	tmdbImagePaths
}

type episodeProfileResponse struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	AirDate       string `json:"air_date"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	Runtime       int    `json:"runtime"`
	tmdbImagePaths
}

type movieProfileResponse struct {
	ID                  int     `json:"id"`
	Title               string  `json:"title"`
	OriginalTitle       string  `json:"original_title"`
	Overview            string  `json:"overview"`
	ReleaseDate         string  `json:"release_date"`
	Genres              []Genre `json:"genres"`
	Popularity          float64 `json:"popularity"`
	ProductionCountries []struct {
		ISO31661 string `json:"iso_3166_1"`
		Name     string `json:"name"`
	} `json:"production_countries"`
	Runtime     int     `json:"runtime"`
	Status      string  `json:"status"`
	VoteAverage float64 `json:"vote_average"`
	tmdbImagePaths
}

type creditsResponse struct {
	Cast []creditPerson `json:"cast"`
	Crew []creditPerson `json:"crew"`
}

type creditPerson struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Gender             int    `json:"gender"`
	KnownForDepartment string `json:"known_for_department"`
	ProfilePath        string `json:"profile_path"`
	Order              int    `json:"order"`
}

type personProfileResponse struct {
	ID                 int      `json:"id"`
	Name               string   `json:"name"`
	AlsoKnownAs        []string `json:"also_known_as"`
	Biography          string   `json:"biography"`
	Birthday           string   `json:"birthday"`
	Gender             int      `json:"gender"`
	KnownForDepartment string   `json:"known_for_department"`
	PlaceOfBirth       string   `json:"place_of_birth"`
	ProfilePath        string   `json:"profile_path"`
}
