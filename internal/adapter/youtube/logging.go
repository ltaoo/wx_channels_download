package youtubeadapter

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"wx_channel/pkg/scraper/youtube"
)

func log_selected_download_format(logger *zerolog.Logger, video_id string, resource_index int, format youtube.VideoFormat, headers map[string]string) {
	if logger == nil {
		return
	}
	parsed, _ := url.Parse(strings.TrimSpace(format.URL))
	query := parsed.Query()
	header_names := make([]string, 0, len(headers))
	for name := range headers {
		header_names = append(header_names, name)
	}
	sort.Strings(header_names)

	expires_at := int64(0)
	if value := strings.TrimSpace(query.Get("expire")); value != "" {
		expires_at, _ = strconv.ParseInt(value, 10, 64)
	}
	event := logger.Info().
		Str("component", "youtube_adapter").
		Str("video_id", video_id).
		Int("resource_index", resource_index).
		Int("itag", format.Itag).
		Str("source_client", format.SourceClient).
		Str("source_client_id", format.SourceClientID).
		Str("source_client_version", format.SourceVersion).
		Bool("adaptive", format.Adaptive).
		Bool("has_video", format.HasVideo).
		Bool("has_audio", format.HasAudio).
		Int64("content_length", format.ContentLength).
		Bool("requires_pot", format.RequiresPOT).
		Bool("pot_present", query.Get("pot") != "").
		Bool("n_challenge_present", format.HadNChallenge).
		Bool("n_challenge_solved", format.SolvedNChallenge).
		Str("endpoint_host", parsed.Hostname()).
		Str("endpoint_path", parsed.EscapedPath()).
		Strs("request_header_names", header_names).
		Bool("request_cookie_present", strings.TrimSpace(headers["Cookie"]) != "").
		Str("request_user_agent", headers["User-Agent"]).
		Str("request_youtube_client_name", headers["X-YouTube-Client-Name"]).
		Str("request_youtube_client_version", headers["X-YouTube-Client-Version"])
	if expires_at > 0 {
		event = event.
			Int64("endpoint_expires_at", expires_at).
			Int64("endpoint_ttl_seconds", expires_at-time.Now().Unix())
	}
	event.Msg("youtube download format selected")
}
