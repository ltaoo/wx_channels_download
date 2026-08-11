package cookies

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const persistent_cookie_file_name = "cookies.json"

// ErrCookieNotFound indicates that persistent storage has no usable cookie for
// the requested domain.
var ErrCookieNotFound = errors.New("cookies: matching persistent cookie not found")

// Reader exposes persisted cookies without leaking their storage layout to
// consumers. Clients should retain a pointer to avoid copying reader state.
type Reader struct {
	work_dir string
}

// NewPersistentReader creates a reader backed by the application's runtime
// cookie storage. Callers only provide the work directory and do not depend on
// the concrete filename or JSON representation.
func NewPersistentReader(work_dir string) *Reader {
	return &Reader{work_dir: strings.TrimSpace(work_dir)}
}

func (r *Reader) HeaderForDomain(domain string) (string, error) {
	normalized_domain := normalize_persistent_domain(domain)
	if normalized_domain == "" || r == nil || r.work_dir == "" {
		return "", ErrCookieNotFound
	}

	cookie_list, err := r.load()
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	matched := make([]Cookie, 0)
	for _, cookie := range cookie_list {
		if cookie.Name == "" || !persistent_cookie_domain_matches(cookie.Domain, normalized_domain) {
			continue
		}
		if cookie.Expires > 0 && cookie.Expires <= now {
			continue
		}
		matched = append(matched, cookie)
	}
	if len(matched) == 0 {
		return "", ErrCookieNotFound
	}
	return FormatCookieHeader(matched), nil
}

func (r *Reader) load() ([]Cookie, error) {
	path := filepath.Join(r.work_dir, persistent_cookie_file_name)
	json_file_mu.Lock()
	data, err := os.ReadFile(path)
	json_file_mu.Unlock()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCookieNotFound
		}
		return nil, fmt.Errorf("cookies: read persistent storage: %w", err)
	}

	var cookie_list []Cookie
	if err := json.Unmarshal(data, &cookie_list); err != nil {
		return nil, fmt.Errorf("cookies: parse persistent storage: %w", err)
	}
	return cookie_list, nil
}

func normalize_persistent_domain(domain string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
}

func persistent_cookie_domain_matches(cookie_domain string, request_domain string) bool {
	request_domain = normalize_persistent_domain(request_domain)
	cookie_domain = strings.ToLower(strings.TrimSpace(cookie_domain))
	if request_domain == "" || cookie_domain == "" {
		return false
	}
	if strings.HasPrefix(cookie_domain, ".") {
		domain_scope := strings.TrimPrefix(cookie_domain, ".")
		return request_domain == domain_scope || strings.HasSuffix(request_domain, "."+domain_scope)
	}
	return request_domain == cookie_domain
}
