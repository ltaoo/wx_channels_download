package services

import (
	"fmt"
	"net/url"
	"strings"

	douyin_scraper "wx_channel/pkg/scraper/douyin"
	kuaishou_scraper "wx_channel/pkg/scraper/kuaishou"
	weibo_scraper "wx_channel/pkg/scraper/weibo"
	x_scraper "wx_channel/pkg/scraper/x"
	xiaohongshu_scraper "wx_channel/pkg/scraper/xiaohongshu"
	youtube_scraper "wx_channel/pkg/scraper/youtube"
)

const (
	scraper_platform_wxchannels  = "wxchannels"
	scraper_platform_wxmp        = "wxmp"
	scraper_platform_douyin      = "douyin"
	scraper_platform_kuaishou    = "kuaishou"
	scraper_platform_bilibili    = "bilibili"
	scraper_platform_youtube     = "youtube"
	scraper_platform_xiaohongshu = "xiaohongshu"
	scraper_platform_zhihu       = "zhihu"
	scraper_platform_cctv        = "cctv"
	scraper_platform_ttk         = "ttk"
	scraper_platform_weibo       = "weibo"
	scraper_platform_x           = "x"
	scraper_platform_ucdrive     = "ucdrive"
	scraper_platform_webpage     = "webpage"
)

type ScraperPlatformResolution struct {
	Platform  string
	StatusKey string
}

type scraper_raw_url_rule struct {
	platform_id string
	match       func(raw_url string) bool
}

type scraper_http_url_rule struct {
	platform_id   string
	status_key    string
	exact_hosts   []string
	domain_hosts  []string
	path_prefixes []string
}

var scraper_raw_url_rules = []scraper_raw_url_rule{
	{
		platform_id: scraper_platform_douyin,
		match: func(raw_url string) bool {
			_, err := douyin_scraper.ExtractURL(raw_url)
			return err == nil
		},
	},
	{
		platform_id: scraper_platform_kuaishou,
		match: func(raw_url string) bool {
			_, err := kuaishou_scraper.ExtractURL(raw_url)
			return err == nil
		},
	},
	{
		platform_id: scraper_platform_zhihu,
		match: func(raw_url string) bool {
			return strings.HasPrefix(strings.ToLower(raw_url), "zhihu://")
		},
	},
	{
		platform_id: scraper_platform_weibo,
		match:       weibo_scraper.IsDetailURL,
	},
	{
		platform_id: scraper_platform_x,
		match: func(raw_url string) bool {
			_, err := x_scraper.ExtractStatusID(raw_url)
			return err == nil
		},
	},
	{
		platform_id: scraper_platform_xiaohongshu,
		match: func(raw_url string) bool {
			_, err := xiaohongshu_scraper.ExtractURL(raw_url)
			return err == nil
		},
	},
	{
		platform_id: scraper_platform_youtube,
		match: func(raw_url string) bool {
			_, ok := youtube_scraper.ExtractVideoID(raw_url)
			return ok
		},
	},
}

// Keep specific path rules before broad host rules. The first match wins.
var scraper_http_url_rules = []scraper_http_url_rule{
	{
		platform_id:   scraper_platform_wxchannels,
		status_key:    "wxchannels:sph",
		exact_hosts:   []string{"weixin.qq.com"},
		path_prefixes: []string{"/sph/"},
	},
	{
		platform_id: scraper_platform_wxchannels,
		status_key:  "wxchannels:page",
		exact_hosts: []string{"channels.weixin.qq.com"},
	},
	{
		platform_id: scraper_platform_wxmp,
		exact_hosts: []string{"mp.weixin.qq.com"},
	},
	{
		platform_id:  scraper_platform_douyin,
		domain_hosts: []string{"douyin.com", "iesdouyin.com"},
	},
	{
		platform_id:  scraper_platform_kuaishou,
		domain_hosts: []string{"kuaishou.com", "kuaishou.cn"},
	},
	{
		platform_id:  scraper_platform_bilibili,
		exact_hosts:  []string{"b23.tv", "bili2233.cn"},
		domain_hosts: []string{"bilibili.com"},
	},
	{
		platform_id: scraper_platform_cctv,
		exact_hosts: []string{"v.cctv.com"},
	},
	{
		platform_id: scraper_platform_zhihu,
		exact_hosts: []string{"www.zhihu.com", "zhuanlan.zhihu.com"},
	},
	{
		platform_id: scraper_platform_ttk,
		exact_hosts: []string{"ttks.tw", "www.ttks.tw"},
	},
	{
		platform_id:   scraper_platform_ucdrive,
		exact_hosts:   []string{"drive.uc.cn"},
		path_prefixes: []string{"/s/", "/share/"},
	},
	{
		platform_id:  scraper_platform_xiaohongshu,
		domain_hosts: []string{"xiaohongshu.com", "xhslink.cn", "xhslink.com"},
	},
}

// DetectScraperPlatform maps a supported scraper URL to its adapter ID.
func DetectScraperPlatform(raw_url string) (string, error) {
	resolution, err := ResolveScraperPlatform(raw_url)
	if err != nil {
		return "", err
	}
	return resolution.Platform, nil
}

// ResolveScraperPlatform maps a supported scraper URL to its adapter ID and
// the status entry that should gate that URL.
func ResolveScraperPlatform(raw_url string) (ScraperPlatformResolution, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return ScraperPlatformResolution{}, fmt.Errorf("url 不能为空")
	}

	for _, rule := range scraper_raw_url_rules {
		if rule.match(raw_url) {
			return scraper_platform_resolution(rule.platform_id, ""), nil
		}
	}

	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Hostname() == "" {
		return ScraperPlatformResolution{}, fmt.Errorf("无法解析 URL: %s", raw_url)
	}
	parsed_url.Scheme = strings.ToLower(strings.TrimSpace(parsed_url.Scheme))
	if parsed_url.Scheme != "http" && parsed_url.Scheme != "https" {
		return ScraperPlatformResolution{}, fmt.Errorf("仅支持 HTTP/HTTPS URL: %s", raw_url)
	}
	parsed_url.Host = strings.ToLower(parsed_url.Host)

	for _, rule := range scraper_http_url_rules {
		if rule.matches(parsed_url) {
			return scraper_platform_resolution(rule.platform_id, rule.status_key), nil
		}
	}
	return scraper_platform_resolution(scraper_platform_webpage, ""), nil
}

func scraper_platform_resolution(platform_id string, status_key string) ScraperPlatformResolution {
	if strings.TrimSpace(status_key) == "" {
		status_key = platform_id
	}
	return ScraperPlatformResolution{Platform: platform_id, StatusKey: status_key}
}

func (rule scraper_http_url_rule) matches(parsed_url *url.URL) bool {
	host := parsed_url.Hostname()
	for _, exact_host := range rule.exact_hosts {
		if host == exact_host {
			return rule.matches_path(parsed_url.EscapedPath())
		}
	}
	for _, domain_host := range rule.domain_hosts {
		if host == domain_host || strings.HasSuffix(host, "."+domain_host) {
			return rule.matches_path(parsed_url.EscapedPath())
		}
	}
	return false
}

func (rule scraper_http_url_rule) matches_path(path string) bool {
	if len(rule.path_prefixes) == 0 {
		return true
	}
	for _, path_prefix := range rule.path_prefixes {
		if strings.HasPrefix(path, path_prefix) {
			return true
		}
	}
	return false
}
