package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHostname = "https://proxy.funzm.com/api/tmdb/3"
	DefaultToken    = "c2e5d34999e27f8e0ef18421aa5dec38"
)

type Client struct {
	HTTPClient *http.Client
	Hostname   string
	Token      string
	Language   Language
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.HTTPClient = client
	}
}

func WithHostname(hostname string) Option {
	return func(c *Client) {
		c.Hostname = hostname
	}
}

func WithToken(token string) Option {
	return func(c *Client) {
		c.Token = token
	}
}

func WithLanguage(language Language) Option {
	return func(c *Client) {
		c.Language = language
	}
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Hostname:   DefaultHostname,
		Token:      DefaultToken,
		Language:   LanguageZhCN,
	}
	for _, opt := range opts {
		opt(c)
	}
	if strings.TrimSpace(c.Hostname) == "" {
		c.Hostname = DefaultHostname
	}
	if c.Language == "" {
		c.Language = LanguageZhCN
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return c
}

func (c *Client) SearchTV(ctx context.Context, keyword string, page int) (*SearchTVResult, error) {
	var resp searchTVResponse
	query := c.commonQuery(map[string]string{
		"query":         keyword,
		"page":          strconv.Itoa(page),
		"include_adult": "false",
	})
	if err := c.getJSON(ctx, "/search/tv", query, &resp); err != nil {
		return nil, err
	}
	list := make([]SearchedTVItem, 0, len(resp.Results))
	for _, result := range resp.Results {
		image := fixTMDBImagePath(result.tmdbImagePaths)
		list = append(list, SearchedTVItem{
			ID:            result.ID,
			Name:          firstNonEmpty(result.Name, result.OriginalName),
			OriginalName:  result.OriginalName,
			Overview:      result.Overview,
			PosterPath:    image.PosterPath,
			BackdropPath:  image.BackdropPath,
			FirstAirDate:  result.FirstAirDate,
			OriginCountry: result.OriginCountry,
			Popularity:    result.Popularity,
			VoteAverage:   result.VoteAverage,
			Type:          "tv",
			Source:        "tmdb",
		})
	}
	return &SearchTVResult{
		Page:  resp.Page,
		Total: resp.TotalResults,
		List:  list,
	}, nil
}

func (c *Client) SearchMovie(ctx context.Context, keyword string, page int) (*SearchMovieResult, error) {
	var resp searchMovieResponse
	query := c.commonQuery(map[string]string{
		"query":         keyword,
		"page":          strconv.Itoa(page),
		"include_adult": "false",
	})
	if err := c.getJSON(ctx, "/search/movie", query, &resp); err != nil {
		return nil, err
	}
	list := make([]SearchedMovieItem, 0, len(resp.Results))
	for _, result := range resp.Results {
		image := fixTMDBImagePath(result.tmdbImagePaths)
		list = append(list, SearchedMovieItem{
			ID:            result.ID,
			Name:          result.Title,
			OriginalName:  result.OriginalTitle,
			Overview:      result.Overview,
			PosterPath:    image.PosterPath,
			BackdropPath:  image.BackdropPath,
			FirstAirDate:  result.ReleaseDate,
			AirDate:       result.ReleaseDate,
			OriginCountry: []string{},
			Type:          "movie",
			Source:        "tmdb",
		})
	}
	return &SearchMovieResult{
		Page:  resp.Page,
		Total: resp.TotalResults,
		List:  list,
	}, nil
}

func (c *Client) FetchTVProfile(ctx context.Context, id int) (*TVProfile, error) {
	var resp tvProfileResponse
	if err := c.getJSON(ctx, fmt.Sprintf("/tv/%d", id), c.commonQuery(nil), &resp); err != nil {
		return nil, err
	}
	image := fixTMDBImagePath(resp.tmdbImagePaths)
	seasons := make([]Season, 0, len(resp.Seasons))
	for _, season := range resp.Seasons {
		seasonImage := fixTMDBImagePath(season.tmdbImagePaths)
		seasons = append(seasons, Season{
			ID:           season.ID,
			Name:         season.Name,
			Overview:     season.Overview,
			PosterPath:   seasonImage.PosterPath,
			AirDate:      season.AirDate,
			EpisodeCount: season.EpisodeCount,
			SeasonNumber: season.SeasonNumber,
			VoteAverage:  season.VoteAverage,
		})
	}
	return &TVProfile{
		ID:               resp.ID,
		Name:             firstNonEmpty(resp.Name, resp.OriginalName),
		OriginalName:     resp.OriginalName,
		Overview:         resp.Overview,
		PosterPath:       image.PosterPath,
		BackdropPath:     image.BackdropPath,
		FirstAirDate:     resp.FirstAirDate,
		VoteAverage:      resp.VoteAverage,
		Popularity:       resp.Popularity,
		NumberOfEpisodes: resp.NumberOfEpisodes,
		NumberOfSeasons:  resp.NumberOfSeasons,
		InProduction:     resp.InProduction,
		NextEpisodeToAir: resp.NextEpisodeToAir,
		Genres:           resp.Genres,
		OriginCountry:    resp.OriginCountry,
		Seasons:          seasons,
		Type:             "tv",
		Source:           "tmdb",
	}, nil
}

func (c *Client) FetchSeasonProfile(ctx context.Context, tvID int, seasonNumber int) (*SeasonProfile, error) {
	var resp seasonProfileResponse
	endpoint := fmt.Sprintf("/tv/%d/season/%d", tvID, seasonNumber)
	if err := c.getJSON(ctx, endpoint, c.commonQuery(nil), &resp); err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			return nil, nil
		}
		return nil, err
	}
	image := fixTMDBImagePath(resp.tmdbImagePaths)
	episodes := make([]Episode, 0, len(resp.Episodes))
	for _, episode := range resp.Episodes {
		episodeImage := fixTMDBImagePath(episode.tmdbImagePaths)
		episodes = append(episodes, Episode{
			ID:            episode.ID,
			Name:          episode.Name,
			Overview:      episode.Overview,
			StillPath:     episodeImage.StillPath,
			AirDate:       episode.AirDate,
			EpisodeNumber: episode.EpisodeNumber,
			SeasonNumber:  episode.SeasonNumber,
			Runtime:       episode.Runtime,
		})
	}
	return &SeasonProfile{
		ID:           resp.ID,
		Name:         resp.Name,
		Overview:     resp.Overview,
		PosterPath:   image.PosterPath,
		Number:       resp.SeasonNumber,
		AirDate:      resp.AirDate,
		SeasonNumber: resp.SeasonNumber,
		Episodes:     episodes,
	}, nil
}

func (c *Client) FetchEpisodeProfile(ctx context.Context, tvID int, seasonNumber int, episodeNumber int) (*EpisodeProfile, error) {
	var resp episodeProfileResponse
	endpoint := fmt.Sprintf("/tv/%d/season/%d/episode/%d", tvID, seasonNumber, episodeNumber)
	if err := c.getJSON(ctx, endpoint, c.commonQuery(nil), &resp); err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			return nil, nil
		}
		return nil, err
	}
	image := fixTMDBImagePath(resp.tmdbImagePaths)
	return &EpisodeProfile{
		ID:            resp.ID,
		Name:          resp.Name,
		Overview:      resp.Overview,
		StillPath:     image.StillPath,
		AirDate:       resp.AirDate,
		EpisodeNumber: resp.EpisodeNumber,
		SeasonNumber:  resp.SeasonNumber,
		Runtime:       resp.Runtime,
	}, nil
}

func (c *Client) FetchMovieProfile(ctx context.Context, id int) (*MovieProfile, error) {
	var resp movieProfileResponse
	if err := c.getJSON(ctx, fmt.Sprintf("/movie/%d", id), c.commonQuery(nil), &resp); err != nil {
		return nil, err
	}
	image := fixTMDBImagePath(resp.tmdbImagePaths)
	return &MovieProfile{
		ID:            resp.ID,
		Name:          resp.Title,
		OriginalName:  resp.OriginalTitle,
		AirDate:       resp.ReleaseDate,
		Overview:      resp.Overview,
		Status:        resp.Status,
		VoteAverage:   resp.VoteAverage,
		Popularity:    resp.Popularity,
		Genres:        resp.Genres,
		Runtime:       resp.Runtime,
		OriginCountry: productionCountryCodes(resp.ProductionCountries),
		PosterPath:    image.PosterPath,
		BackdropPath:  image.BackdropPath,
		Type:          "movie",
		Source:        "tmdb",
	}, nil
}

func (c *Client) FetchPersonsOfSeason(ctx context.Context, tvID int, seasonNumber int) ([]Person, error) {
	var resp creditsResponse
	endpoint := fmt.Sprintf("/tv/%d/season/%d/credits", tvID, seasonNumber)
	if err := c.getJSON(ctx, endpoint, c.commonQuery(nil), &resp); err != nil {
		return nil, err
	}
	return personsFromCredits(resp), nil
}

func (c *Client) FetchPersonsOfMovie(ctx context.Context, movieID int) ([]Person, error) {
	var resp creditsResponse
	endpoint := fmt.Sprintf("/movie/%d/credits", movieID)
	if err := c.getJSON(ctx, endpoint, c.commonQuery(nil), &resp); err != nil {
		return nil, err
	}
	return personsFromCredits(resp), nil
}

func (c *Client) FetchPersonProfile(ctx context.Context, personID int) (*Person, error) {
	var resp personProfileResponse
	if err := c.getJSON(ctx, fmt.Sprintf("/person/%d", personID), c.commonQuery(nil), &resp); err != nil {
		return nil, err
	}
	image := fixTMDBImagePath(tmdbImagePaths{ProfilePath: resp.ProfilePath})
	name := resp.Name
	if isChinesePlace(resp.PlaceOfBirth) {
		for _, candidate := range resp.AlsoKnownAs {
			if isChineseText(candidate) {
				name = strings.TrimSpace(candidate)
				break
			}
		}
	}
	return &Person{
		ID:                 resp.ID,
		Name:               name,
		Biography:          resp.Biography,
		ProfilePath:        image.ProfilePath,
		PlaceOfBirth:       resp.PlaceOfBirth,
		Birthday:           resp.Birthday,
		KnownForDepartment: resp.KnownForDepartment,
		Gender:             resp.Gender,
	}, nil
}

func (c *Client) commonQuery(extra map[string]string) map[string]string {
	query := map[string]string{
		"api_key":  c.Token,
		"language": string(c.Language),
	}
	for key, value := range extra {
		if value != "" {
			query[key] = value
		}
	}
	return query
}

func (c *Client) getJSON(ctx context.Context, endpoint string, query map[string]string, out any) error {
	rawURL := strings.TrimRight(c.Hostname, "/") + endpoint
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	q := u.Query()
	for key, value := range query {
		if value != "" {
			q.Set(key, value)
		}
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, out)
}

func personsFromCredits(resp creditsResponse) []Person {
	all := append(append([]creditPerson{}, resp.Cast...), resp.Crew...)
	persons := make([]Person, 0, len(all))
	for _, item := range all {
		image := fixTMDBImagePath(tmdbImagePaths{ProfilePath: item.ProfilePath})
		persons = append(persons, Person{
			ID:                 item.ID,
			Name:               item.Name,
			Gender:             item.Gender,
			ProfilePath:        image.ProfilePath,
			KnownForDepartment: item.KnownForDepartment,
			Order:              item.Order,
		})
	}
	return persons
}

func productionCountryCodes(values []struct {
	ISO31661 string `json:"iso_3166_1"`
	Name     string `json:"name"`
}) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value.ISO31661 != "" {
			out = append(out, value.ISO31661)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isChinesePlace(value string) bool {
	return regexp.MustCompile(`China|中国|Hong Kong|Taiwan`).MatchString(value)
}

func isChineseText(value string) bool {
	return regexp.MustCompile(`^[\p{Han}]+$`).MatchString(value)
}
