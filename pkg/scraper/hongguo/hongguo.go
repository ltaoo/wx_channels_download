// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
package hongguo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const site = "https://hongguoduanju.com"
const user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

var id_pattern = regexp.MustCompile(`^\d{1,30}$`)
var tag_pattern = regexp.MustCompile(`<[^>]*>`)
var router_pattern = regexp.MustCompile(`(?:window\.)?_ROUTER_DATA\s*=\s*(\{)`)

type api_client struct {
	HTTPClient *http.Client
}

type series_detail struct {
	SeriesID   sourceutil.TextValue   `json:"series_id"`
	SeriesName string                 `json:"series_name"`
	VidList    []sourceutil.TextValue `json:"vid_list"`
}

type page_data struct {
	IsSuccess    *bool                `json:"isSuccess"`
	SeriesID     sourceutil.TextValue `json:"series_id"`
	Vid          sourceutil.TextValue `json:"vid"`
	SeriesDetail *series_detail       `json:"seriesDetail"`
}

func clean_text(value string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tag_pattern.ReplaceAllString(value, ""))), " ")
}

func first_text(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func parse_series_id(value string) (string, error) {
	value = strings.TrimSpace(value)
	if id_pattern.MatchString(value) {
		return value, nil
	}
	link, err := url.Parse(value)
	if err == nil && link.Scheme == "https" && link.Host == "hongguoduanju.com" && link.User == nil {
		var series_id string
		parts := strings.Split(strings.Trim(link.Path, "/"), "/")
		if link.Path == "/detail" {
			series_id = link.Query().Get("series_id")
		} else if (len(parts) == 2 || len(parts) == 3) && parts[0] == "player" {
			series_id = parts[1]
		}
		if id_pattern.MatchString(series_id) {
			return series_id, nil
		}
	}
	return "", errors.New("请输入数字剧集编号，或 https://hongguoduanju.com 的 detail/player 链接")
}

func (client *api_client) Request(ctx context.Context, address string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", user_agent)
	req.Header.Set("Referer", site+"/")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	response, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("%s 返回 HTTP %d", req.URL.Path, response.StatusCode)
	}
	return response, nil
}

func (client *api_client) request_text(ctx context.Context, address string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	response, err := client.Request(ctx, address)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	const max_size = 8 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, max_size+1))
	if err != nil {
		return "", err
	}
	if len(body) > max_size {
		return "", errors.New("官网页面/接口响应超过 8 MiB")
	}
	return string(body), nil
}

func router_pages(body string) ([]page_data, error) {
	match := router_pattern.FindStringSubmatchIndex(body)
	if match == nil {
		return nil, errors.New("官网页面缺少 _ROUTER_DATA")
	}
	var payload struct {
		LoaderData map[string]json.RawMessage `json:"loaderData"`
	}
	if err := json.NewDecoder(strings.NewReader(body[match[2]:])).Decode(&payload); err != nil {
		return nil, fmt.Errorf("官网页面数据无效：%w", err)
	}
	pages := make([]page_data, 0, len(payload.LoaderData))
	for _, raw := range payload.LoaderData {
		var page page_data
		if json.Unmarshal(raw, &page) == nil && (page.IsSuccess == nil || *page.IsSuccess) {
			pages = append(pages, page)
		}
	}
	return pages, nil
}

func (client *api_client) series(ctx context.Context, series_id string) (*series_detail, error) {
	var failure error
	for _, address := range []string{site + "/player/" + series_id, site + "/detail?series_id=" + series_id} {
		body, err := client.request_text(ctx, address)
		if err == nil {
			var pages []page_data
			pages, err = router_pages(body)
			for _, page := range pages {
				detail := page.SeriesDetail
				if detail != nil && string(detail.SeriesID) == series_id && detail.VidList != nil {
					if len(detail.VidList) == 0 {
						return nil, errors.New("该短剧暂无分集")
					}
					for _, vid := range detail.VidList {
						if !id_pattern.MatchString(string(vid)) {
							return nil, errors.New("官网分集列表包含无效编号")
						}
					}
					return detail, nil
				}
			}
			if err == nil {
				err = errors.New("官网未返回对应剧集的分集列表，可能已下架或页面结构已变化")
			}
		}
		failure = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, failure
}
