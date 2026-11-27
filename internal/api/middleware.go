package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"

	"github.com/rs/zerolog"
)

func with_cors(h http.Handler, cfg *APIConfig, loggers ...*zerolog.Logger) http.Handler {
	var logger *zerolog.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		if !trusted_api_request(r, cfg) {
			if logger != nil {
				logger.Warn().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("host", r.Host).
					Str("origin", r.Header.Get("Origin")).
					Str("sec_fetch_site", r.Header.Get("Sec-Fetch-Site")).
					Str("sec_fetch_mode", r.Header.Get("Sec-Fetch-Mode")).
					Str("referer", r.Header.Get("Referer")).
					Str("remote_addr", r.RemoteAddr).
					Str("user_agent", r.Header.Get("User-Agent")).
					Msg("rejected untrusted API request")
			}
			http.Error(w, "untrusted request origin or host", http.StatusForbidden)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
		allow_headers := r.Header.Get("Access-Control-Request-Headers")
		if allow_headers == "" {
			allow_headers = "Content-Type, Range, Accept, Origin, X-Requested-With, Authorization"
		}
		w.Header().Set("Access-Control-Allow-Headers", allow_headers)
		w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Accept-Ranges, Content-Type, Content-Length, Content-Disposition")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if cfg != nil && cfg.Username != "" && cfg.Password != "" {
			username, password, ok := r.BasicAuth()
			username_hash, password_hash := sha256.Sum256([]byte(username)), sha256.Sum256([]byte(password))
			expected_username, expected_password := sha256.Sum256([]byte(cfg.Username)), sha256.Sum256([]byte(cfg.Password))
			if !ok || subtle.ConstantTimeCompare(username_hash[:], expected_username[:])&subtle.ConstantTimeCompare(password_hash[:], expected_password[:]) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="wx_channels_download", charset="UTF-8"`)
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// trusted_api_request only guards against DNS rebinding to a loopback listener.
// Request origins are intentionally not restricted, so injected clients and
// local tooling can connect from any page.
func trusted_api_request(r *http.Request, cfg *APIConfig) bool {
	hostname := (&url.URL{Host: r.Host}).Hostname()
	if hostname == "" {
		return false
	}
	// A loopback listener must not accept a hostile DNS name that resolves to it.
	if cfg != nil && (cfg.Username == "" || cfg.Password == "") && (cfg.Hostname == "localhost" || net.ParseIP(cfg.Hostname).IsLoopback()) {
		if hostname != "localhost" && !net.ParseIP(hostname).IsLoopback() {
			return false
		}
	}
	return true
}
