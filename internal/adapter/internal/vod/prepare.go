package vod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
	"wx_channel/internal/adapter"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/util"
)

var _ adapter.DownloadPreparer = (*Handler)(nil)

func (h *Handler) decode_input(resource hermes.ResourceJob) (Input, bool, error) {
	raw := resource.Extra["vod_input"]
	if raw == "" {
		for _, endpoint := range resource.Endpoints {
			if endpoint.Protocol == SourceProtocol || endpoint.Protocol == "drama" {
				raw = endpoint.URL
				break
			}
		}
	}
	if raw == "" {
		return Input{}, false, nil
	}
	var input Input
	if len(raw) > 64<<10 || json.Unmarshal([]byte(raw), &input) != nil || input.Platform != h.platform {
		return input, true, fmt.Errorf("媒体任务来源无效")
	}
	_, _, id, err := h.spec.ParseURL(input.SourceURL)
	if err != nil || id == "" || input.Episode.ID != id || resource.UniqueID != id {
		return input, true, fmt.Errorf("视频来源与分集编号不匹配")
	}
	input.Episode.URL = ""
	return input, true, nil
}

func (h *Handler) source_client(task *hermes.TaskJob) (*http.Client, func(), error) {
	if h.spec.HTTPClient != nil {
		copy := *h.spec.HTTPClient
		return &copy, func() {}, nil
	}
	proxy, err := task.ProxyServer.URL()
	if err != nil {
		return nil, nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		link, err := url.Parse(proxy)
		if err != nil {
			return nil, nil, err
		}
		transport.Proxy = http.ProxyURL(link)
	}
	return &http.Client{Transport: transport}, transport.CloseIdleConnections, nil
}

func media_client(client *http.Client, check_address func(string) error, transport http.RoundTripper) *http.Client {
	copy := *client
	copy.Jar = nil
	if transport != nil {
		copy.Transport = transport
	}
	previous := copy.CheckRedirect
	copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("媒体跳转次数过多")
		}
		if previous != nil {
			if err := previous(req, via); err != nil {
				return err
			}
		}
		req.Header.Del("Cookie")
		return check_address(req.URL.String())
	}
	return &copy
}

func (h *Handler) PrepareDownload(ctx context.Context, task *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	if task == nil || deps.DB == nil || h.spec.Resolve == nil {
		return fmt.Errorf("媒体资源准备依赖缺失")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	client, close_idle, err := h.source_client(task)
	if err != nil {
		return err
	}
	defer close_idle()
	existing := map[string]hermes.ResourceJob{}
	for _, resource := range task.Resources {
		existing[resource.UniqueID] = resource
	}
	var prepared []hermes.ResourceJob
	var obsolete_files []string
	for _, root := range task.Resources {
		if root.Extra["vod_owner"] != "" {
			continue
		}
		input, matched, err := h.decode_input(root)
		if err != nil {
			return err
		}
		if !matched {
			prepared = append(prepared, root)
			continue
		}
		if root.Extra == nil {
			root.Extra = map[string]string{}
		}
		root.Extra["vod_input"] = string_marshal(input)
		if root.Extra["postprocessed"] == "true" {
			filename := root.FilePath
			if filename == "" {
				directory := root.DownloadDir
				if directory == "" {
					directory = task.DownloadDir
				}
				filename = hermes.ResolveOutputPath(deps.BasePath, directory, root.UniqueID)
				if _, err := os.Stat(filename); err != nil {
					filename = hermes.ResolveOutputPath(deps.BasePath, directory, root.Name)
				}
			}
			plan, err := parse_plan(root.Extra["vod_plan"])
			if err != nil {
				return err
			}
			if err := validate_media(ctx, filename, plan.Info); err != nil {
				return err
			}
			root.Endpoints = []hermes.Endpoint{{Protocol: "file", URL: filename}}
			for _, child := range task.Resources {
				if child.Extra["vod_owner"] == root.UniqueID {
					obsolete_files = append(obsolete_files, input_filename(task, child, deps.BasePath))
				}
			}
			prepared = append(prepared, root)
			continue
		}
		cookie := h.cookie()
		if cookie == "" {
			for _, endpoint := range root.Endpoints {
				if endpoint.Cookies != "" {
					cookie = endpoint.Cookies
				}
				for key, value := range endpoint.Headers {
					if strings.EqualFold(key, "Cookie") {
						cookie = value
					}
				}
			}
		}
		source, err := h.spec.Resolve(ctx, input.SourceURL, input.Episode, cookie, client)
		if err != nil {
			return err
		}
		plan, err := plan_source(root.UniqueID, source)
		if err != nil {
			return err
		}
		if previous := root.Extra["vod_plan"]; previous != "" {
			old, err := parse_plan(previous)
			if err != nil {
				return err
			}
			if old.Fingerprint != plan.Fingerprint {
				return fmt.Errorf("片源结构或版本已变化，请重新创建任务以避免混合分片")
			}
		} else {
			var count int64
			if err := deps.DB.WithContext(ctx).Model(&model.DownloadSegment{}).Where("resource_id = ? AND downloaded > 0", root.ID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("旧版中转任务已有下载进度，请重新创建任务")
			}
		}
		root.Extra["vod_plan"] = string_marshal(plan)
		agent, referer := source.UserAgent, source.Referer
		if referer == "" {
			referer = input.SourceURL
		}
		for index, item := range plan.Items {
			resource := root
			if index > 0 {
				resource = existing[item.UniqueID]
				if resource.UniqueID == "" {
					resource = hermes.ResourceJob{UniqueID: item.UniqueID, DownloadDir: root.DownloadDir, Name: fmt.Sprintf("%s_%05d", root.Name, index), Kind: "application/octet-stream", Type: "FILE"}
				}
				resource.Extra = map[string]string{"vod_owner": root.UniqueID}
			}
			link, _ := url.Parse(item.Address)
			transport := source.Transport
			if item.Key {
				transport = nil
			}
			headers := map[string]string{"Referer": referer}
			if agent != "" {
				headers["User-Agent"] = agent
			}
			resource.Endpoints = []hermes.Endpoint{{Protocol: link.Scheme, URL: item.Address, Headers: headers, HTTPClient: media_client(client, source.CheckMediaAddress, transport)}}
			prepared = append(prepared, resource)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, resource := range prepared {
		if seen[resource.UniqueID] {
			return fmt.Errorf("下载资源编号冲突")
		}
		seen[resource.UniqueID] = true
	}
	if err := persist_prepared(ctx, deps.DB, task.ID, prepared); err != nil {
		return err
	}
	for _, filename := range obsolete_files {
		if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	task.Resources = prepared
	return nil
}

func persist_prepared(ctx context.Context, db *gorm.DB, task_id int, resources []hermes.ResourceJob) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old []model.DownloadResource
		if err := tx.Where("task_id = ?", task_id).Find(&old).Error; err != nil {
			return err
		}
		kept := map[int]bool{}
		for index := range resources {
			resource := &resources[index]
			if resource.Extra["vod_input"] == "" && resource.Extra["vod_owner"] == "" {
				kept[resource.ID] = true
				continue
			}
			if resource.ID == 0 {
				row := model.DownloadResource{TaskId: &task_id, UniqueID: resource.UniqueID, Name: resource.Name, DownloadDir: resource.DownloadDir, Kind: resource.Kind, Type: resource.Type, Extra: string_marshal(resource.Extra), Timestamps: model.Timestamps{CreatedAt: util.NowMillis(), UpdatedAt: util.NowMillis()}}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
				resource.ID = row.Id
			} else {
				if err := tx.Model(&model.DownloadResource{}).Where("id = ? AND task_id = ?", resource.ID, task_id).Updates(map[string]any{"extra": string_marshal(resource.Extra)}).Error; err != nil {
					return err
				}
			}
			kept[resource.ID] = true
			if resource.Extra["postprocessed"] == "true" {
				continue
			}
			endpoint := &resource.Endpoints[0]
			var row model.DownloadEndpoint
			if err := tx.Where("resource_id = ?", resource.ID).Order("id").Limit(1).Find(&row).Error; err != nil {
				return err
			}
			row.ResourceId, row.Protocol, row.URL, row.Headers, row.Cookies, row.Enabled = resource.ID, endpoint.Protocol, endpoint.URL, string_marshal(endpoint.Headers), "", 1
			if row.CreatedAt == 0 {
				row.CreatedAt = util.NowMillis()
			}
			row.UpdatedAt = util.NowMillis()
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
			endpoint.ID = row.Id
			if err := tx.Model(&model.DownloadEndpoint{}).Where("resource_id = ? AND id != ?", resource.ID, row.Id).Update("enabled", 0).Error; err != nil {
				return err
			}
		}
		var stale []int
		for _, row := range old {
			if !kept[row.Id] {
				stale = append(stale, row.Id)
			}
		}
		return database.NewDBTaskStore(tx, nil).DeleteStaleResources(task_id, stale)
	})
}

func input_filename(task *hermes.TaskJob, resource hermes.ResourceJob, base_path string) string {
	if resource.FilePath != "" {
		return resource.FilePath
	}
	directory := resource.DownloadDir
	if directory == "" {
		directory = task.DownloadDir
	}
	return hermes.ResolveOutputPath(base_path, directory, filepath.Clean(resource.UniqueID))
}
