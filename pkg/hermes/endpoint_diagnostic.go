package hermes

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// add_endpoint_diagnostic records enough request identity to correlate a CDN
// failure without writing signed query values, cookies, or access tokens.
func add_endpoint_diagnostic(event *zerolog.Event, endpoint Endpoint) *zerolog.Event {
	if event == nil {
		return event
	}
	digest := sha256.Sum256([]byte(endpoint.URL))
	event = event.Str("endpoint_url_fingerprint", hex.EncodeToString(digest[:8]))

	parsed, parse_err := url.Parse(strings.TrimSpace(endpoint.URL))
	if parse_err == nil {
		query := parsed.Query()
		event = event.
			Str("endpoint_scheme", strings.ToLower(parsed.Scheme)).
			Str("endpoint_host", parsed.Hostname()).
			Str("endpoint_path", parsed.EscapedPath()).
			Str("endpoint_itag", query.Get("itag")).
			Str("endpoint_client", query.Get("c")).
			Bool("endpoint_n_present", query.Get("n") != "").
			Int("endpoint_n_length", len(query.Get("n"))).
			Bool("endpoint_pot_present", query.Get("pot") != "").
			Bool("endpoint_signature_present", query.Get("sig") != "" || query.Get("signature") != "").
			Bool("endpoint_lsig_present", query.Get("lsig") != "").
			Bool("endpoint_spc_present", query.Get("spc") != "").
			Bool("endpoint_ip_bound", query.Get("ip") != "")
		if expires_at, err := strconv.ParseInt(query.Get("expire"), 10, 64); err == nil && expires_at > 0 {
			event = event.
				Int64("endpoint_expires_at", expires_at).
				Int64("endpoint_ttl_seconds", expires_at-time.Now().Unix())
		}
	}

	header_names := make([]string, 0, len(endpoint.Headers))
	for name := range endpoint.Headers {
		header_names = append(header_names, name)
	}
	sort.Strings(header_names)
	event = event.
		Strs("request_header_names", header_names).
		Bool("request_cookie_present", strings.TrimSpace(endpoint.Cookies) != "" || strings.TrimSpace(endpoint_header(endpoint.Headers, "Cookie")) != "").
		Str("request_user_agent", endpoint_header(endpoint.Headers, "User-Agent")).
		Str("request_youtube_client_name", endpoint_header(endpoint.Headers, "X-YouTube-Client-Name")).
		Str("request_youtube_client_version", endpoint_header(endpoint.Headers, "X-YouTube-Client-Version"))
	if referer, err := url.Parse(endpoint_header(endpoint.Headers, "Referer")); err == nil {
		event = event.Str("request_referer_host", referer.Hostname())
	}
	return event
}

func endpoint_header(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(strings.TrimSpace(key), name) {
			return value
		}
	}
	return ""
}
