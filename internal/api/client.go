package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/internal/assets"
	"wx_channel/internal/channels"
	"wx_channel/internal/officialaccount"
	"wx_channel/pkg/decrypt"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

type APIClient struct {
	downloader  *downloadpkg.Downloader
	official    *officialaccount.OfficialAccountClient
	channels    *channels.ChannelsClient
	filehelper  *FileHelperHandler
	formatter   *util.FilenameProcessor
	cfg         *APIConfig
	engine      *gin.Engine
	logger      *zerolog.Logger
}

func NewAPIClient(cfg *APIConfig, parent_logger *zerolog.Logger) *APIClient {
	data_dir := cfg.RootDir
	downloader := downloadpkg.NewDownloader(&downloadpkg.DownloaderConfig{
		RefreshInterval: 360,
		Storage:         downloadpkg.NewBoltStorage(data_dir),
		StorageDir:      data_dir,
	})
	var channels_client *channels.ChannelsClient
	official_cfg := officialaccount.NewOfficialAccountConfig(cfg.Original, cfg.RemoteServerMode)
	officialaccount_client := officialaccount.NewOfficialAccountClient(official_cfg, parent_logger)
	channels_client = channels.NewChannelsClient()

	get_sorted_tasks := func() []*downloadpkg.Task {
		tasks := downloader.GetTasks()
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
		})
		return tasks
	}

	channels_client.OnConnected = func(client *channels.Client) {
		// Initial tasks
		all_tasks := get_sorted_tasks()
		limit := 50
		if limit > len(all_tasks) {
			limit = len(all_tasks)
		}
		tasks := all_tasks[:limit]
		if data, err := json.Marshal(APIClientWSMessage{Type: "tasks", Data: map[string]interface{}{
			"list":  tasks,
			"total": len(all_tasks),
		}}); err == nil {
			client.Send <- data
		}
	}

	channels_client.OnMessage = func(client *channels.Client, message []byte) {
		var req struct {
			Type  string `json:"type"`
			Page  int    `json:"page"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(message, &req); err == nil && req.Type == "fetch_tasks" {
			allTasks := get_sorted_tasks()
			start := (req.Page - 1) * req.Limit
			if start < 0 {
				start = 0
			}
			if req.Limit <= 0 {
				req.Limit = 50
			}

			if start >= len(allTasks) {
				if data, err := json.Marshal(APIClientWSMessage{Type: "tasks", Data: map[string]interface{}{
					"list":  []*downloadpkg.Task{},
					"total": len(allTasks),
				}}); err == nil {
					client.Send <- data
				}
				return
			}
			end := start + req.Limit
			if end > len(allTasks) {
				end = len(allTasks)
			}
			tasks := allTasks[start:end]
			if data, err := json.Marshal(APIClientWSMessage{Type: "tasks", Data: map[string]interface{}{
				"list":  tasks,
				"total": len(allTasks),
			}}); err == nil {
				client.Send <- data
			}
		}
	}
	logger := parent_logger.With().Str("Client", "api_client").Logger()
	client := &APIClient{
		downloader: downloader,
		official:   officialaccount_client,
		channels:   channels_client,
		filehelper: NewFileHelperHandler(),
		formatter:  util.NewFilenameProcessor(cfg.DownloadDir, make(map[string]int)),
		cfg:        cfg,
		engine:     gin.Default(),
		logger:     &logger,
	}

	// 设置文件传输助手视频号自动下载回调
	client.filehelper.SetFinderAutoDownloadCallback(client.autoCreateChannelsTask)

	client.SetupRoutes()
	return client
}

type APIClientWSMessage struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

type ClientWSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}
type ClientWebsocketRequestBody struct {
	ID   string      `json:"id"`
	Key  string      `json:"key"`
	Body interface{} `json:"data"`
}
type ClientWebsocketResponse struct {
	Id string `json:"id"`
	// 调用 wx api 原始响应
	Data json.RawMessage `json:"data"`
}

func (c *APIClient) Start() error {
	if err := c.downloader.Setup(); err != nil {
		return err
	}
	_ = c.downloader.PutConfig(&base.DownloaderStoreConfig{
		DownloadDir: c.cfg.DownloadDir,
		MaxRunning:  c.cfg.MaxRunning,
		ProtocolConfig: map[string]any{
			"http": map[string]any{
				"connections": 4,
			},
		},
		Extra: map[string]any{},
		Proxy: &base.DownloaderProxyConfig{},
	})
	c.downloader.Listener(func(evt *downloadpkg.Event) {
		if evt == nil || evt.Task == nil || evt.Task.ID == "" {
			return
		}
		c.channels.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: evt,
		})
		if evt.Key == downloadpkg.EventKeyDone {
			if c.cfg.PlayDoneAudio {
				go assets.PlayDoneAudio()
			}
			task := c.downloader.GetTask(evt.Task.ID)
			file_path := task.Meta.SingleFilepath()
			go func() {
				k := task.Meta.Req.Labels["key"]
				if k != "" {
					key, err := strconv.Atoi(k)
					if err == nil {
						if key != 0 {
							data, err := os.ReadFile(file_path)
							if err == nil {
								length := uint32(131072)
								_key := uint64(key)
								decrypt.DecryptData(data, length, _key)
								_ = os.WriteFile(file_path, data, 0644)
							}
						}
					}
				}
				suffix := task.Meta.Req.Labels["suffix"]
				if suffix == ".mp3" {
					temp_path := file_path + ".temp"
					if err := os.Rename(file_path, temp_path); err == nil {
						if err := system.RunCommand("ffmpeg", "-i", temp_path, "-vn", "-acodec", "libmp3lame", "-ab", "192k", "-f", "mp3", file_path); err == nil {
							_ = os.Remove(temp_path)
						} else {
							_ = os.Rename(temp_path, file_path)
						}
					}
				}
			}()
		}
	})
	return nil
}

func (c *APIClient) Stop() error {
	if c.downloader != nil {
		c.downloader.Pause(nil)
	}
	if c.channels != nil {
		c.channels.Stop()
	}
	return nil
}

func (c *APIClient) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.engine.ServeHTTP(w, r)
}

func (c *APIClient) resolve_connections(url string) int {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Head(url)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()

	if resp.ContentLength > 0 && resp.ContentLength < 1024*1024 {
		return 1
	}
	return 4
}

func (c *APIClient) check_existing_feed(tasks []*downloadpkg.Task, body *FeedDownloadTaskBody) bool {
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		same_id := t.Meta.Req.Labels["id"] == body.Id
		same_spec := t.Meta.Req.Labels["spec"] == body.Spec
		same_suffix := t.Meta.Req.Labels["suffix"] == body.Suffix
		if same_id && same_spec && same_suffix {
			return true
		}
	}
	return false
}

// autoCreateChannelsTask 根据视频号消息自动创建下载任务
func (c *APIClient) autoCreateChannelsTask(objectID, objectNonceID string) error {
	c.logger.Info().
		Str("objectID", objectID).
		Str("objectNonceID", objectNonceID).
		Msg("收到视频号消息，开始自动创建下载任务")

	// 获取视频详情
	r, err := c.channels.FetchChannelsFeedProfile(objectID, objectNonceID, "", "")
	if err != nil {
		c.logger.Error().Err(err).Msg("获取视频号详情失败")
		return fmt.Errorf("获取详情失败: %w", err)
	}
	if r.ErrCode != 0 {
		c.logger.Error().Str("errMsg", r.ErrMsg).Msg("获取视频号详情失败")
		return fmt.Errorf("获取详情失败: %s", r.ErrMsg)
	}
	if len(r.Data.Object.ObjectDesc.Media) == 0 {
		c.logger.Warn().Msg("缺少可下载的视频内容")
		return fmt.Errorf("缺少可下载的视频内容")
	}

	media := r.Data.Object.ObjectDesc.Media[0]
	key := 0
	if media.DecodeKey != "" {
		k, err := strconv.Atoi(media.DecodeKey)
		if err != nil {
			c.logger.Error().Err(err).Msg("解析 DecodeKey 失败")
			return fmt.Errorf("解析 DecodeKey 失败: %w", err)
		}
		key = k
	}

	spec := "original"
	if !c.cfg.Original.GetBool("download.defaultHighest") {
		if len(media.Spec) > 0 {
			spec = media.Spec[0].FileFormat
		}
	}

	// 构建文件名
	feed := r.Data.Object
	defaultName := feed.ObjectDesc.Description
	if defaultName == "" {
		if feed.ID != "" {
			defaultName = feed.ID
		} else {
			defaultName = util.NowSecondsStr()
		}
	}
	template := c.cfg.Original.GetString("download.filenameTemplate")
	filename := defaultName
	if template != "" {
		params := map[string]string{
			"filename":    defaultName,
			"id":          feed.ID,
			"title":       feed.ObjectDesc.Description,
			"spec":        spec,
			"created_at":  strconv.Itoa(feed.CreateTime),
			"download_at": util.NowSecondsStr(),
			"author":      feed.Contact.Nickname,
		}
		filename = template
		for k, v := range params {
			filename = strings.ReplaceAll(filename, "{{"+k+"}}", v)
		}
	}

	payload := FeedDownloadTaskBody{
		Id:       feed.ID,
		Title:    feed.ObjectDesc.Description,
		Key:      key,
		Spec:     spec,
		Suffix:   ".mp4",
		URL:      media.URL + media.URLToken,
		Filename: filename,
	}

	// 处理图集类型
	if feed.ObjectDesc.MediaType == 2 {
		payload.Suffix = ".zip"
		var files []map[string]string
		for i, m := range feed.ObjectDesc.Media {
			files = append(files, map[string]string{
				"url":      m.URL + m.URLToken,
				"filename": fmt.Sprintf("%d.jpg", i+1),
			})
		}
		data, _ := json.Marshal(files)
		payload.URL = fmt.Sprintf("zip://weixin.qq.com?files=%s", url.QueryEscape(string(data)))
	}

	if payload.Id == "" {
		return fmt.Errorf("缺少 feed id")
	}

	// 检查是否已存在相同任务
	tasks := c.downloader.GetTasks()
	if c.check_existing_feed(tasks, &payload) {
		c.logger.Info().Str("id", payload.Id).Msg("任务已存在，跳过创建")
		return nil
	}

	// 处理文件名
	connections := c.resolve_connections(payload.URL)
	processedFilename, dir, err := c.formatter.ProcessFilename(payload.Filename + payload.Suffix)
	if err != nil {
		c.logger.Error().Err(err).Msg("处理文件名失败")
		return fmt.Errorf("处理文件名失败: %w", err)
	}

	// 创建下载任务
	req := &base.Request{
		URL: payload.URL,
		Labels: map[string]string{
			"id":     payload.Id,
			"title":  payload.Title,
			"key":    strconv.Itoa(payload.Key),
			"spec":   payload.Spec,
			"suffix": payload.Suffix,
		},
	}
	opt := &base.Options{
		Name: processedFilename,
		Path: filepath.Join(c.cfg.DownloadDir, dir),
		Extra: &gopeedhttp.OptsExtra{
			Connections: connections,
		},
	}

	id, err := c.downloader.CreateDirect(req, opt)
	if err != nil {
		c.logger.Error().Err(err).Msg("创建下载任务失败")
		return fmt.Errorf("创建下载任务失败: %w", err)
	}

	// 广播新任务事件
	task := c.downloader.GetTask(id)
	if task != nil {
		c.channels.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}

	c.logger.Info().
		Str("id", payload.Id).
		Str("taskId", id).
		Str("filename", processedFilename).
		Msg("自动创建下载任务成功")

	return nil
}
