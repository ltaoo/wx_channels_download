package tmdb

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/internal/jsonartifact"
	tmdbpkg "wx_channel/pkg/scraper/tmdb"
)

const PlatformID = "tmdb"

var leadingDigitsRE = regexp.MustCompile(`^([0-9]+)`)

type ProfileFetcher interface {
	FetchTVProfile(ctx context.Context, id int) (*tmdbpkg.TVProfile, error)
	FetchMovieProfile(ctx context.Context, id int) (*tmdbpkg.MovieProfile, error)
	FetchSeasonProfile(ctx context.Context, tvID int, seasonNumber int) (*tmdbpkg.SeasonProfile, error)
	FetchEpisodeProfile(ctx context.Context, tvID int, seasonNumber int, episodeNumber int) (*tmdbpkg.EpisodeProfile, error)
}

type Handler struct {
	Client ProfileFetcher
}

type tmdbURLParts struct {
	Kind          string
	TVID          int
	MovieID       int
	SeasonNumber  int
	EpisodeNumber int
	CanonicalURL  string
	ContentID     string
}

func New(client ProfileFetcher) *Handler {
	if client == nil {
		client = tmdbpkg.NewClient()
	}
	return &Handler{Client: client}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	_, ok := parseTMDBURL(rawURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	parts, ok := parseTMDBURL(input.URL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	data, summary, metadata, output, err := h.fetch(ctx, parts, input.URL)
	if err != nil {
		return nil, err
	}
	content := contentdownload.NewContent(summary, data, metadata, output)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: parts.CanonicalURL,
		ContentID:    parts.ContentID,
		Content:      content,
		Variants:     []contentdownload.Variant{jsonartifact.Variant(summary.Type)},
		Defaults:     jsonartifact.Defaults(),
		Internal:     map[string]any{"profile": data},
	}, nil
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	probe := input.Probe
	if probe == nil {
		var err error
		probe, err = h.Probe(ctx, contentdownload.ProbeInput{URL: input.URL, Extra: input.Extra})
		if err != nil {
			return nil, err
		}
	}
	variant, err := contentdownload.SelectVariant(probe, input.Options)
	if err != nil {
		return nil, err
	}
	return jsonartifact.Resolve(ctx, PlatformID, input, probe, variant)
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return jsonartifact.Plan(PlatformID), nil
}

func (h *Handler) fetch(ctx context.Context, parts tmdbURLParts, sourceURL string) (any, contentdownload.ContentSummary, map[string]any, map[string]any, error) {
	switch parts.Kind {
	case "tv":
		profile, err := h.Client.FetchTVProfile(ctx, parts.TVID)
		if err != nil {
			return nil, contentdownload.ContentSummary{}, nil, nil, fmt.Errorf("fetch tmdb tv profile: %w", err)
		}
		if profile == nil {
			return nil, contentdownload.ContentSummary{}, nil, nil, fmt.Errorf("fetch tmdb tv profile: empty profile")
		}
		id := strconv.Itoa(firstNonZero(profile.ID, parts.TVID))
		title := jsonartifact.FirstNonEmpty(profile.Name, profile.OriginalName, id)
		return profile, contentdownload.ContentSummary{
				Platform:    PlatformID,
				Type:        "tv",
				ID:          parts.ContentID,
				Title:       title,
				Description: profile.Overview,
				URL:         parts.CanonicalURL,
				SourceURL:   sourceURL,
				CoverURL:    profile.PosterPath,
			}, map[string]any{
				"tmdb_id":            id,
				"media_type":         "tv",
				"original_name":      profile.OriginalName,
				"first_air_date":     profile.FirstAirDate,
				"vote_average":       profile.VoteAverage,
				"number_of_episodes": profile.NumberOfEpisodes,
				"number_of_seasons":  profile.NumberOfSeasons,
			}, map[string]any{
				"format":             "json",
				"content_type":       "tv",
				"id":                 parts.ContentID,
				"title":              title,
				"source_url":         sourceURL,
				"canonical_url":      parts.CanonicalURL,
				"vote_average":       profile.VoteAverage,
				"number_of_episodes": profile.NumberOfEpisodes,
				"number_of_seasons":  profile.NumberOfSeasons,
			}, nil
	case "movie":
		profile, err := h.Client.FetchMovieProfile(ctx, parts.MovieID)
		if err != nil {
			return nil, contentdownload.ContentSummary{}, nil, nil, fmt.Errorf("fetch tmdb movie profile: %w", err)
		}
		if profile == nil {
			return nil, contentdownload.ContentSummary{}, nil, nil, fmt.Errorf("fetch tmdb movie profile: empty profile")
		}
		id := strconv.Itoa(firstNonZero(profile.ID, parts.MovieID))
		title := jsonartifact.FirstNonEmpty(profile.Name, profile.OriginalName, id)
		return profile, contentdownload.ContentSummary{
				Platform:    PlatformID,
				Type:        "movie",
				ID:          parts.ContentID,
				Title:       title,
				Description: profile.Overview,
				URL:         parts.CanonicalURL,
				SourceURL:   sourceURL,
				CoverURL:    profile.PosterPath,
				Duration:    int64(profile.Runtime) * 60,
			}, map[string]any{
				"tmdb_id":       id,
				"media_type":    "movie",
				"original_name": profile.OriginalName,
				"air_date":      profile.AirDate,
				"vote_average":  profile.VoteAverage,
				"runtime":       profile.Runtime,
			}, map[string]any{
				"format":        "json",
				"content_type":  "movie",
				"id":            parts.ContentID,
				"title":         title,
				"source_url":    sourceURL,
				"canonical_url": parts.CanonicalURL,
				"vote_average":  profile.VoteAverage,
				"runtime":       profile.Runtime,
			}, nil
	case "season":
		profile, err := h.Client.FetchSeasonProfile(ctx, parts.TVID, parts.SeasonNumber)
		if err != nil {
			return nil, contentdownload.ContentSummary{}, nil, nil, fmt.Errorf("fetch tmdb season profile: %w", err)
		}
		if profile == nil {
			return nil, contentdownload.ContentSummary{}, nil, nil, fmt.Errorf("fetch tmdb season profile: empty profile")
		}
		title := jsonartifact.FirstNonEmpty(profile.Name, parts.ContentID)
		return profile, contentdownload.ContentSummary{
				Platform:    PlatformID,
				Type:        "season",
				ID:          parts.ContentID,
				Title:       title,
				Description: profile.Overview,
				URL:         parts.CanonicalURL,
				SourceURL:   sourceURL,
				CoverURL:    profile.PosterPath,
			}, map[string]any{
				"tmdb_id":       profile.ID,
				"tv_id":         parts.TVID,
				"media_type":    "season",
				"season_number": parts.SeasonNumber,
				"air_date":      profile.AirDate,
				"episode_count": len(profile.Episodes),
			}, map[string]any{
				"format":        "json",
				"content_type":  "season",
				"id":            parts.ContentID,
				"title":         title,
				"source_url":    sourceURL,
				"canonical_url": parts.CanonicalURL,
				"episode_count": len(profile.Episodes),
			}, nil
	case "episode":
		profile, err := h.Client.FetchEpisodeProfile(ctx, parts.TVID, parts.SeasonNumber, parts.EpisodeNumber)
		if err != nil {
			return nil, contentdownload.ContentSummary{}, nil, nil, fmt.Errorf("fetch tmdb episode profile: %w", err)
		}
		if profile == nil {
			return nil, contentdownload.ContentSummary{}, nil, nil, fmt.Errorf("fetch tmdb episode profile: empty profile")
		}
		title := jsonartifact.FirstNonEmpty(profile.Name, parts.ContentID)
		return profile, contentdownload.ContentSummary{
				Platform:    PlatformID,
				Type:        "episode",
				ID:          parts.ContentID,
				Title:       title,
				Description: profile.Overview,
				URL:         parts.CanonicalURL,
				SourceURL:   sourceURL,
				CoverURL:    profile.StillPath,
				Duration:    int64(profile.Runtime) * 60,
			}, map[string]any{
				"tmdb_id":        profile.ID,
				"tv_id":          parts.TVID,
				"media_type":     "episode",
				"season_number":  parts.SeasonNumber,
				"episode_number": parts.EpisodeNumber,
				"air_date":       profile.AirDate,
				"runtime":        profile.Runtime,
			}, map[string]any{
				"format":         "json",
				"content_type":   "episode",
				"id":             parts.ContentID,
				"title":          title,
				"source_url":     sourceURL,
				"canonical_url":  parts.CanonicalURL,
				"season_number":  parts.SeasonNumber,
				"episode_number": parts.EpisodeNumber,
			}, nil
	default:
		return nil, contentdownload.ContentSummary{}, nil, nil, contentdownload.ErrUnsupportedURL
	}
}

func parseTMDBURL(rawURL string) (tmdbURLParts, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return tmdbURLParts{}, false
	}
	host := strings.ToLower(u.Hostname())
	if host != "themoviedb.org" && !strings.HasSuffix(host, ".themoviedb.org") {
		return tmdbURLParts{}, false
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) < 2 {
		return tmdbURLParts{}, false
	}
	switch segments[0] {
	case "movie":
		id, ok := leadingInt(segments[1])
		if !ok {
			return tmdbURLParts{}, false
		}
		return tmdbURLParts{
			Kind:         "movie",
			MovieID:      id,
			CanonicalURL: fmt.Sprintf("https://www.themoviedb.org/movie/%d", id),
			ContentID:    fmt.Sprintf("movie_%d", id),
		}, true
	case "tv":
		tvID, ok := leadingInt(segments[1])
		if !ok {
			return tmdbURLParts{}, false
		}
		parts := tmdbURLParts{
			Kind:         "tv",
			TVID:         tvID,
			CanonicalURL: fmt.Sprintf("https://www.themoviedb.org/tv/%d", tvID),
			ContentID:    fmt.Sprintf("tv_%d", tvID),
		}
		if len(segments) >= 4 && segments[2] == "season" {
			season, ok := leadingInt(segments[3])
			if !ok {
				return tmdbURLParts{}, false
			}
			parts.Kind = "season"
			parts.SeasonNumber = season
			parts.CanonicalURL = fmt.Sprintf("https://www.themoviedb.org/tv/%d/season/%d", tvID, season)
			parts.ContentID = fmt.Sprintf("tv_%d_season_%d", tvID, season)
		}
		if len(segments) >= 6 && segments[2] == "season" && segments[4] == "episode" {
			episode, ok := leadingInt(segments[5])
			if !ok {
				return tmdbURLParts{}, false
			}
			parts.Kind = "episode"
			parts.EpisodeNumber = episode
			parts.CanonicalURL = fmt.Sprintf("https://www.themoviedb.org/tv/%d/season/%d/episode/%d", tvID, parts.SeasonNumber, episode)
			parts.ContentID = fmt.Sprintf("tv_%d_season_%d_episode_%d", tvID, parts.SeasonNumber, episode)
		}
		return parts, true
	default:
		return tmdbURLParts{}, false
	}
}

func leadingInt(value string) (int, bool) {
	match := leadingDigitsRE.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0, false
	}
	id, err := strconv.Atoi(match[1])
	return id, err == nil && id > 0
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
