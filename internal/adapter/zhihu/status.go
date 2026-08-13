package zhihuadapter

import (
	"context"
	"errors"
	"strings"
	"time"

	"wx_channel/internal/adapter"
	"wx_channel/internal/events"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/zhihu"
)

const (
	zhihu_status_check_url     = "https://www.zhihu.com/question/19550256"
	zhihu_status_check_timeout = 15 * time.Second
	zhihu_cookie_status_domain = "www.zhihu.com"
)

var _ adapter.PlatformStatusRefresher = (*handler)(nil)

func (h *handler) RefreshPlatformStatus() {
	if h == nil {
		return
	}
	h.status_mu.Lock()
	defer h.status_mu.Unlock()
	if h.cancel_status_check != nil {
		h.cancel_status_check()
		h.cancel_status_check = nil
	}
	if h.status_bus == nil {
		return
	}
	h.cancel_status_check = h.start_status_check(h.status_bus)
}

func (h *handler) stop_status_check() {
	if h == nil {
		return
	}
	h.status_mu.Lock()
	defer h.status_mu.Unlock()
	if h.cancel_status_check != nil {
		h.cancel_status_check()
		h.cancel_status_check = nil
	}
}

func (h *handler) start_status_check(bus *events.Bus) context.CancelFunc {
	if bus == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	bus.Publish(events.PlatformStatusChanged{
		Platform:  PlatformID,
		Status:    "checking",
		Available: false,
		Reason:    "正在检测知乎 Cookie 可用性",
	})
	go func() {
		status := h.check_status()
		select {
		case <-ctx.Done():
			return
		default:
			bus.Publish(status)
		}
	}()
	return cancel
}

func (h *handler) check_status() events.PlatformStatusChanged {
	h.runtime_mu.RLock()
	cookie_reader := h.cookie_reader
	logger := h.logger
	h.runtime_mu.RUnlock()

	if cookie_reader == nil {
		return zhihu_unavailable("缺少知乎 Cookie 读取器")
	}
	if _, err := cookie_reader.HeaderForDomain(zhihu_cookie_status_domain); err != nil {
		if errors.Is(err, cookies.ErrCookieNotFound) {
			return zhihu_unavailable("缺少知乎 Cookie，请先导入 www.zhihu.com Cookie")
		}
		return zhihu_unavailable(compact_zhihu_status_reason("读取知乎 Cookie 失败：", err))
	}
	client := zhihu.NewClient(cookie_reader, logger)
	client.SetHTTPTimeout(zhihu_status_check_timeout)
	if _, err := client.Fetch(zhihu_status_check_url); err != nil {
		return zhihu_unavailable(compact_zhihu_status_reason("知乎 Cookie 检测失败：", err))
	}
	return events.PlatformStatusChanged{
		Platform:  PlatformID,
		Status:    "available",
		Available: true,
		Reason:    "知乎 Cookie 已通过检测",
	}
}

func zhihu_unavailable(reason string) events.PlatformStatusChanged {
	return events.PlatformStatusChanged{
		Platform:  PlatformID,
		Status:    "unavailable",
		Available: false,
		Reason:    strings.TrimSpace(reason),
	}
}

func compact_zhihu_status_reason(prefix string, err error) string {
	text := ""
	if err != nil {
		text = strings.TrimSpace(err.Error())
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return strings.TrimSpace(prefix)
	}
	reason := strings.TrimSpace(prefix) + text
	runes := []rune(reason)
	if len(runes) > 180 {
		return string(runes[:180]) + "..."
	}
	return reason
}
