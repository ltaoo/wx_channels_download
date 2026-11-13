package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/PuerkitoBio/goquery"
	"wx_channel/internal/adapter"
	wxchannelsadapter "wx_channel/internal/adapter/wxchannels"
	wxmpadapter "wx_channel/internal/adapter/wxmp"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxmp"
)

func gopeed_article_url(task *downloadpkg.Task, article_id string) string {
	raw_url := strings.TrimSpace(task_url(task))
	if strings.HasPrefix(strings.ToLower(raw_url), "officialaccount://") {
		raw_url = raw_url[len("officialaccount://"):]
		if !strings.HasPrefix(raw_url, "http") {
			raw_url = "https://" + raw_url
		}
	}
	parsed, err := url.Parse(raw_url)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && strings.EqualFold(parsed.Hostname(), "mp.weixin.qq.com") {
		return raw_url
	}
	return build_wxmp_article_url(article_id)
}

func build_gopeed_history_content(task *downloadpkg.Task, labels map[string]string, platform_id, external_id, source_url, save_path string) (*adapter.DownloadTaskResult, error) {
	if external_id == "" {
		return nil, fmt.Errorf("历史任务缺少内容 ID，无法创建 content")
	}
	info := &adapter.DownloadTaskResult{Task: &model.DownloadTask{MetadataJSON: `{"detail_missing":true}`}}
	if platform_id == platform_wxmp {
		html_file, err := os.Open(save_path)
		if err != nil {
			return nil, fmt.Errorf("读取历史公众号 HTML: %w", err)
		}
		defer html_file.Close()
		document, err := goquery.NewDocumentFromReader(html_file)
		if err != nil {
			return nil, err
		}
		parsed, err := url.Parse(source_url)
		if err != nil {
			return nil, err
		}
		query := parsed.Query()
		mid, _ := strconv.Atoi(query.Get("mid"))
		idx, _ := strconv.Atoi(query.Get("idx"))
		body, err := document.Find("#js_content, .rich_media_content").First().Html()
		if err != nil || strings.TrimSpace(body) == "" {
			body, err = document.Find(".split-container, body").First().Html()
		}
		if err != nil || strings.TrimSpace(body) == "" {
			return nil, fmt.Errorf("历史公众号 HTML 缺少正文")
		}
		article := &wxmp.ArticleCgiDataNew{
			Title:           first_non_empty(document.Find("h1.rich_media_title").First().Text(), document.Find("title").Text()),
			NickName:        strings.TrimSpace(document.Find(".rich_media_meta_nickname").First().Text()),
			ContentNoencode: body, Bizuin: query.Get("__biz"), Mid: mid, Idx: idx,
			Link: source_url, SourceURL: source_url,
		}
		content, detail, err := wxmpadapter.ToContent(article)
		if err != nil {
			return nil, err
		}
		content.Metadata = merge_migration_metadata_json(content.Metadata, map[string]any{"detail_missing": true, "detail_source": "gopeed_html", "archive_author": article.NickName})
		info.Content = content
		info.ContentDetails = []adapter.ContentDetail{{Key: content.Id, Type: content.Type, Data: detail}}
	} else {
		title := first_non_empty(labels["title"], strings.TrimSuffix(filepath.Base(save_path), filepath.Ext(save_path)), external_id)
		content := &model.Content{Id: wxchannelsadapter.BuildContentID(external_id), PlatformId: platform_id, ExternalId: external_id,
			ExternalId2: first_label(labels, "uid", "nid", "nonce_id"), Type: "video", Title: title, Description: title,
			SourceURL: build_channels_fallback_source_url(external_id, first_label(labels, "uid", "nid", "nonce_id"), ""),
			Metadata:  must_json(map[string]any{"detail_missing": true, "detail_source": "gopeed_labels", "key": first_label(labels, "key", "decode_key")})}
		if strings.HasPrefix(resource_kind_for_path(save_path), "image/") {
			content.CoverURL = task_url(task)
		} else {
			content.URL = task_url(task)
		}
		info.Content = content
	}
	info.Task.ContentId = &info.Content.Id
	return info, nil
}
