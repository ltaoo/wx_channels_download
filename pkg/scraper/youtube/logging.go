package youtube

import (
	"net/url"
	"path"
	"strings"
	"time"
)

func (c *Client) log_extraction_result(info *VideoInfo, elapsed time.Duration) {
	if c == nil || c.Logger == nil || info == nil {
		return
	}
	client_counts := make(map[string]int)
	challenge_count := 0
	solved_challenge_count := 0
	requires_pot_count := 0
	pot_count := 0
	for _, format := range info.Formats {
		client_name := strings.TrimSpace(format.SourceClient)
		if client_name == "" {
			client_name = "unknown"
		}
		client_counts[client_name]++
		if format.HadNChallenge {
			challenge_count++
		}
		if format.SolvedNChallenge {
			solved_challenge_count++
		}
		if format.RequiresPOT {
			requires_pot_count++
		}
		if query_value(format.URL, "pot") != "" {
			pot_count++
		}
	}
	c.Logger.Info().
		Str("video_id", info.ID).
		Int("format_count", len(info.Formats)).
		Interface("source_client_counts", client_counts).
		Int("n_challenge_count", challenge_count).
		Int("solved_n_challenge_count", solved_challenge_count).
		Int("requires_pot_count", requires_pot_count).
		Int("pot_attached_count", pot_count).
		Int("warning_count", len(info.Warnings)).
		Dur("elapsed", elapsed).
		Msg("youtube extraction completed")
	if len(info.Warnings) > 0 {
		c.Logger.Warn().
			Str("video_id", info.ID).
			Strs("warnings", info.Warnings).
			Msg("youtube extraction completed with warnings")
	}
}

func (r *player_resolver) log_challenge_solver_result(challenge_type, solver string, challenge_count int, started time.Time, err error) {
	if r == nil || r.client == nil || r.client.Logger == nil {
		return
	}
	event := r.client.Logger.Info()
	if err != nil {
		event = r.client.Logger.Warn().Err(err)
	}
	event.
		Str("challenge_type", challenge_type).
		Str("solver", solver).
		Int("challenge_count", challenge_count).
		Dur("elapsed", time.Since(started)).
		Str("player_id", youtube_player_id(r.player_url)).
		Msg("youtube player challenge solver completed")
}

func (r *player_resolver) log_player_solver_preparation(cache_layer, cache_key string, entry *compiled_player_solver, started time.Time, err error) {
	if r == nil || r.client == nil || r.client.Logger == nil {
		return
	}
	if len(cache_key) > 12 {
		cache_key = cache_key[:12]
	}
	event := r.client.Logger.Info()
	if err != nil {
		event = r.client.Logger.Warn().Err(err)
	}
	preprocessed_bytes := 0
	if entry != nil {
		preprocessed_bytes = entry.preprocessed_bytes
	}
	event.
		Str("cache_layer", cache_layer).
		Str("cache_key", cache_key).
		Int("preprocessed_player_bytes", preprocessed_bytes).
		Dur("elapsed", time.Since(started)).
		Str("player_id", youtube_player_id(r.player_url)).
		Msg("youtube player solver prepared")
}

func (r *player_resolver) log_player_solver_cache_warning(operation, cache_key string, err error) {
	if r == nil || r.client == nil || r.client.Logger == nil || err == nil {
		return
	}
	if len(cache_key) > 12 {
		cache_key = cache_key[:12]
	}
	r.client.Logger.Warn().
		Err(err).
		Str("operation", operation).
		Str("cache_key", cache_key).
		Str("player_id", youtube_player_id(r.player_url)).
		Msg("youtube player solver cache operation failed")
}

func youtube_player_id(player_url string) string {
	parsed, err := url.Parse(strings.TrimSpace(player_url))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(path.Clean(parsed.Path), "/"), "/")
	for index, part := range parts {
		if part == "player" && index+1 < len(parts) {
			return parts[index+1]
		}
	}
	return ""
}
