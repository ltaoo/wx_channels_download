package zhihu

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

type ZhihuClient struct {
	db     *gorm.DB
	logger *zerolog.Logger
}

func NewZhihuClient(db *gorm.DB, parentLogger *zerolog.Logger) *ZhihuClient {
	var logger *zerolog.Logger
	if parentLogger != nil {
		l := parentLogger.With().Str("service", "ZhihuClient").Logger()
		logger = &l
	}
	return &ZhihuClient{
		db:     db,
		logger: logger,
	}
}

func (c *ZhihuClient) FetchPage(pageURL string) (string, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if u.Hostname() == "" {
		return "", errors.New("invalid url")
	}
	if !strings.HasSuffix(u.Hostname(), "zhihu.com") {
		return "", errors.New("not a zhihu url")
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	if cookieHeader, credID := c.getDefaultCookieHeader(); cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
		c.touchCredential(credID)
	}

	client := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("fetch failed: %d", resp.StatusCode)
	}
	return string(b), nil
}

func (c *ZhihuClient) getDefaultCookieHeader() (string, int) {
	if c.db == nil {
		return "", 0
	}
	var cred model.AuthCredential
	err := c.db.
		Where("platform_id = ? AND kind = ? AND status = ? AND deleted_at IS NULL", PlatformID, "cookie", 1).
		Order("is_default DESC, updated_at DESC, id DESC").
		First(&cred).
		Error
	if err != nil {
		return "", 0
	}
	return strings.TrimSpace(cred.Secret), cred.Id
}

func (c *ZhihuClient) touchCredential(id int) {
	if c.db == nil || id <= 0 {
		return
	}
	now := util.NowMillis()
	_ = c.db.Model(&model.AuthCredential{}).Where("id = ?", id).Updates(map[string]any{
		"last_used_at": now,
		"updated_at":   now,
	}).Error
}
