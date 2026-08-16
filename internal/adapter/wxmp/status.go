package wxmpadapter

import (
	"context"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/events"
	"wx_channel/pkg/scraper/wxmp"
)

const wxmp_status_check_url = "https://mp.weixin.qq.com/s/z17N2Twe7pnt7UW5hJGHiQ"

var _ adapter.PlatformStatusRefresher = (*OfficialAccountAdapter)(nil)

func (a *OfficialAccountAdapter) RefreshPlatformStatus() {
	if a == nil {
		return
	}
	a.status_mu.Lock()
	defer a.status_mu.Unlock()
	if a.cancel_status_check != nil {
		a.cancel_status_check()
		a.cancel_status_check = nil
	}
	if a.status_bus == nil {
		return
	}
	a.cancel_status_check = a.start_status_check(a.status_bus)
}

func (a *OfficialAccountAdapter) stop_status_check() {
	if a == nil {
		return
	}
	a.status_mu.Lock()
	defer a.status_mu.Unlock()
	if a.cancel_status_check != nil {
		a.cancel_status_check()
		a.cancel_status_check = nil
	}
}

func (a *OfficialAccountAdapter) start_status_check(bus *events.Bus) context.CancelFunc {
	if bus == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	bus.Publish(events.PlatformStatusChanged{
		Platform:  PlatformID,
		Status:    "checking",
		Available: false,
		Reason:    "正在检测公众号文章抓取",
	})
	go func() {
		status := a.check_status()
		select {
		case <-ctx.Done():
			return
		default:
			bus.Publish(status)
		}
	}()
	return cancel
}

func (a *OfficialAccountAdapter) check_status() events.PlatformStatusChanged {
	client := a.status_client()
	if client == nil {
		return wxmp_unavailable("缺少公众号抓取客户端")
	}
	article, err := client.FetchArticle(wxmp_status_check_url)
	if err != nil {
		return wxmp_unavailable(compact_wxmp_status_reason("公众号文章检测失败：", err))
	}
	if article == nil || (strings.TrimSpace(article.Title) == "" && strings.TrimSpace(article.ContentNoencode) == "") {
		return wxmp_unavailable("公众号文章检测失败：响应缺少文章内容")
	}
	return events.PlatformStatusChanged{
		Platform:  PlatformID,
		Status:    "available",
		Available: true,
		Reason:    "公众号文章抓取已通过检测",
	}
}

func (a *OfficialAccountAdapter) status_client() *wxmp.Client {
	if a == nil {
		return nil
	}
	a.runtime_mu.Lock()
	defer a.runtime_mu.Unlock()
	return a.client
}

func wxmp_unavailable(reason string) events.PlatformStatusChanged {
	return events.PlatformStatusChanged{
		Platform:  PlatformID,
		Status:    "unavailable",
		Available: false,
		Reason:    strings.TrimSpace(reason),
	}
}

func compact_wxmp_status_reason(prefix string, err error) string {
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
