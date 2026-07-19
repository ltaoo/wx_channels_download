package registry

import (
	"encoding/json"
	"fmt"
	"sync"

	"wx_channel/internal/database/model"
)

// DownloadConfig 下载配置，各平台通用
type DownloadConfig struct {
	SavePath      string `json:"save_path"`
	Filename      string `json:"filename"`
	Spec          string `json:"spec"`
	Overwrite     bool   `json:"overwrite"`
	SkipDuplicate bool   `json:"skip_duplicate"`
}

// DownloadInfo 下载任务构建结果
type DownloadInfo struct {
	Task     model.DownloadTaskV1
	Resource model.DownloadResource
	Endpoint model.DownloadEndpoint
}

// PlatformHandler 平台处理器接口。
// 每个平台模块实现此接口，负责解析自身的 scraper 类型并生成下载任务。
type PlatformHandler interface {
	// PlatformID 返回平台唯一标识，如 "wx_channels"、"wx_mp"
	PlatformID() string

	// BuildDownloadTask 根据平台原始内容 JSON 和下载配置，生成 V1 下载模型。
	// contentJSON: 平台 scraper 对象的 JSON 原始数据
	// config: 下载配置（目录、文件名、清晰度、覆盖策略等）
	// 返回 DownloadInfo（task/resource/endpoint）、Content、Account
	BuildDownloadTask(contentJSON json.RawMessage, config DownloadConfig) (*DownloadInfo, *model.Content, *model.Account, error)
}

var (
	handlersMu sync.RWMutex
	handlers   = map[string]PlatformHandler{}
)

// Register 注册一个平台处理器。应在 init() 中调用。
func Register(h PlatformHandler) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	id := h.PlatformID()
	if _, dup := handlers[id]; dup {
		panic(fmt.Sprintf("registry: duplicate platform handler %q", id))
	}
	handlers[id] = h
}

// Get 根据平台 ID 获取处理器，不存在返回 nil。
func Get(platformID string) PlatformHandler {
	handlersMu.RLock()
	defer handlersMu.RUnlock()
	return handlers[platformID]
}
