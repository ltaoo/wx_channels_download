package dm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// AccountListQuery describes a read-only account query.
type AccountListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	AccountID string
}

// BrowseHistoryListQuery describes a read-only browse history query.
type BrowseHistoryListQuery struct {
	Page        int
	PageSize    int
	Keyword     string
	Username    string
	PlatformIDs []string
}

// LogListQuery describes a read-only application log query.
type LogListQuery struct {
	Page     int
	PageSize int
	MaxBytes int
	Keyword  string
	Source   string
	Levels   []string
}

// Accounts returns the raw paged account list response.
func (c *Client) Accounts(ctx context.Context, query AccountListQuery) (json.RawMessage, error) {
	return c.get(ctx, "/api/account/list", url.Values{
		"page":       []string{strconv.Itoa(query.Page)},
		"page_size":  []string{strconv.Itoa(query.PageSize)},
		"keyword":    []string{query.Keyword},
		"account_id": []string{query.AccountID},
	})
}

// BrowseHistory returns the raw paged browse history response.
func (c *Client) BrowseHistory(ctx context.Context, query BrowseHistoryListQuery) (json.RawMessage, error) {
	body := map[string]any{
		"page":         query.Page,
		"page_size":    query.PageSize,
		"keyword":      query.Keyword,
		"platform_ids": query.PlatformIDs,
	}
	if query.Username != "" {
		body["username"] = query.Username
	}
	return c.do(ctx, http.MethodPost, "/api/browse_history/list", nil, body)
}

// Logs returns the raw paged log response.
func (c *Client) Logs(ctx context.Context, query LogListQuery) (json.RawMessage, error) {
	return c.get(ctx, "/api/logs", url.Values{
		"page":      []string{strconv.Itoa(query.Page)},
		"page_size": []string{strconv.Itoa(query.PageSize)},
		"max_bytes": []string{strconv.Itoa(query.MaxBytes)},
		"keyword":   []string{query.Keyword},
		"source":    []string{query.Source},
		"levels":    []string{strings.Join(query.Levels, ",")},
	})
}

// CertificateStatus returns the raw proxy certificate status response.
func (c *Client) CertificateStatus(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "/api/proxy/certificate/status", nil)
}
