package api

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/util"
)

const (
	scraper_platform_wxchannels = "wxchannels"
	scraper_platform_wxmp       = "wxmp"
	scraper_platform_douyin     = "douyin"
	scraper_platform_bilibili   = "bilibili"
	scraper_platform_zhihu      = "zhihu"
	scraper_platform_69shuba    = "69shuba"
)

func (c *APIClient) handle_scraper_fetch(ctx *gin.Context) {
	raw_url := strings.TrimSpace(ctx.Query("url"))
	if raw_url == "" {
		result.Err(ctx, result.CodeMissingUrl, "缺少参数：url")
		return
	}

	platform_id, err := detect_scraper_platform(raw_url)
	if err != nil {
		result.Err(ctx, result.CodeInvalidParams, err.Error())
		return
	}

	handler := adapter.Get(platform_id)
	if handler == nil {
		result.Err(ctx, result.CodeInvalidParams, fmt.Sprintf("未注册的平台 adapter: %s", platform_id))
		return
	}

	data, err := handler.Fetch(raw_url)
	if err != nil {
		result.Err(ctx, result.CodeFetchMsgFailed, err.Error())
		return
	}
	content, err := handler.ToContent(data)
	if err != nil {
		result.Err(ctx, result.CodeFetchMsgFailed, fmt.Sprintf("转换 content 失败: %v", err))
		return
	}
	if content != nil && strings.TrimSpace(content.SourceURL) == "" {
		content.SourceURL = raw_url
	}
	account, err := handler.ToAccount(data)
	if err != nil {
		result.Err(ctx, result.CodeFetchMsgFailed, fmt.Sprintf("转换 account 失败: %v", err))
		return
	}

	result.Ok(ctx, gin.H{
		"platform": platform_id,
		"url":      raw_url,
		"result":   data,
		"content":  content,
		"account":  account,
	})
}

func detect_scraper_platform(raw_url string) (string, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return "", fmt.Errorf("url 不能为空")
	}

	if strings.HasPrefix(strings.ToLower(raw_url), "zhihu://") {
		return scraper_platform_zhihu, nil
	}

	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Hostname() == "" {
		return "", fmt.Errorf("无法解析 URL: %s", raw_url)
	}

	host := strings.ToLower(parsed_url.Hostname())
	switch {
	case host == "weixin.qq.com" && strings.HasPrefix(parsed_url.EscapedPath(), "/sph/"):
		return scraper_platform_wxchannels, nil
	case host == "channels.weixin.qq.com":
		return scraper_platform_wxchannels, nil
	case host == "mp.weixin.qq.com":
		return scraper_platform_wxmp, nil
	case scraper_host_matches(host, "douyin.com") || scraper_host_matches(host, "iesdouyin.com"):
		return scraper_platform_douyin, nil
	case scraper_host_matches(host, "bilibili.com") || host == "b23.tv" || host == "bili2233.cn":
		return scraper_platform_bilibili, nil
	case host == "www.zhihu.com" || host == "zhuanlan.zhihu.com":
		return scraper_platform_zhihu, nil
	case strings.Contains(host, "69shuba"):
		return scraper_platform_69shuba, nil
	default:
		return "", fmt.Errorf("暂不支持该 URL: %s", raw_url)
	}
}

func scraper_host_matches(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}
