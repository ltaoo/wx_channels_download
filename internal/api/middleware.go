package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"strings"
)

func with_cors(h http.Handler, cfg *APIConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		if !trusted_api_request(r, cfg) {
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

func trusted_api_request(r *http.Request, cfg *APIConfig) bool {
	request_url := &url.URL{Scheme: "http", Host: r.Host}
	if r.TLS != nil || (cfg != nil && cfg.Protocol == "https") {
		request_url.Scheme = "https"
	}
	// Forwarded protocol only affects origins on an API that requires authentication.
	if cfg != nil && cfg.Username != "" && cfg.Password != "" && r.Header.Get("X-Forwarded-Proto") == "https" {
		request_url.Scheme = "https"
	}
	hostname := request_url.Hostname()
	if hostname == "" {
		return false
	}
	// A loopback listener must not accept a hostile DNS name that resolves to it.
	if cfg != nil && (cfg.Username == "" || cfg.Password == "") && (cfg.Hostname == "localhost" || net.ParseIP(cfg.Hostname).IsLoopback()) {
		if hostname != "localhost" && !net.ParseIP(hostname).IsLoopback() {
			return false
		}
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return r.Header.Get("Sec-Fetch-Site") != "cross-site"
	}
	origin_url, err := url.Parse(origin)
	if err != nil || origin_url.User != nil || origin_url.RawQuery != "" || origin_url.Fragment != "" || origin_url.Path != "" {
		return false
	}
	if origin_url.Scheme == request_url.Scheme && strings.EqualFold(origin_url.Host, request_url.Host) {
		return true
	}
	// These pages host the application's injected WeChat clients.
	switch origin {
	case "https://channels.weixin.qq.com", "https://mp.weixin.qq.com", "https://weixin110.qq.com":
		return true
	}
	return false
}
