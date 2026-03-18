package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/internal/assets"
	"wx_channel/internal/channels"
	downloaderclient "wx_channel/internal/downloader"
	"wx_channel/internal/officialaccount"
	"wx_channel/pkg/decrypt"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

type APIClient struct {
	downloader    *downloadpkg.Downloader
	official      *officialaccount.OfficialAccountClient
	channels      *channels.ChannelsClient
	downloader_ws *downloaderclient.DownloaderClient
	filehelper    *FileHelperHandler
	formatter     *util.FilenameProcessor
	cfg           *APIConfig
	engine        *gin.Engine
	logger        *zerolog.Logger
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
	channels_client = channels.NewChannelsClient(cfg.ChannelsRefreshInterval)
	downloader_ws := downloaderclient.NewDownloaderClient()

	get_sorted_tasks := func() []*downloadpkg.Task {
		tasks := downloader.GetTasks()
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
		})
		return tasks
	}

	downloader_ws.OnConnected = func(client *downloaderclient.WSClient) {
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

	downloader_ws.OnMessage = func(client *downloaderclient.WSClient, message []byte) {
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
		downloader:    downloader,
		official:      officialaccount_client,
		channels:      channels_client,
		downloader_ws: downloader_ws,
		filehelper:    NewFileHelperHandler(),
		formatter:     util.NewFilenameProcessor(cfg.DownloadDir, make(map[string]int)),
		cfg:           cfg,
		engine:        gin.Default(),
		logger:        &logger,
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
		c.downloader_ws.Broadcast(APIClientWSMessage{
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
	if c.downloader_ws != nil {
		c.downloader_ws.Stop()
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

func (c *APIClient) createFeedTaskBody(oid, nid, reqUrl, eid string, isMp3, isCover bool) (*FeedDownloadTaskBody, error) {
	// 获取视频详情
	r, err := c.channels.FetchChannelsFeedProfile(oid, nid, reqUrl, eid)
	if err != nil {
		return nil, fmt.Errorf("获取详情失败: %w", err)
	}
	if r.ErrCode != 0 {
		return nil, fmt.Errorf("获取详情失败: %s", r.ErrMsg)
	}
	if len(r.Data.Object.ObjectDesc.Media) == 0 {
		return nil, fmt.Errorf("缺少可下载的视频内容")
	}

	media := r.Data.Object.ObjectDesc.Media[0]
	key := 0
	if media.DecodeKey != "" {
		k, err := strconv.Atoi(media.DecodeKey)
		if err != nil {
			return nil, fmt.Errorf("解析 DecodeKey 失败: %w", err)
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

	payload := &FeedDownloadTaskBody{
		Id:       feed.ID,
		Title:    feed.ObjectDesc.Description,
		Key:      key,
		Spec:     spec,
		Suffix:   ".mp4",
		URL:      media.URL + media.URLToken,
		Filename: filename,
	}

	if isMp3 {
		payload.Suffix = ".mp3"
	}
	if isCover {
		payload.Suffix += ".jpg"
		payload.URL = media.CoverUrl
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

	return payload, nil
}

// autoCreateChannelsTask 根据视频号消息自动创建下载任务
func (c *APIClient) autoCreateChannelsTask(objectID, objectNonceID string) error {
	c.logger.Info().
		Str("objectID", objectID).
		Str("objectNonceID", objectNonceID).
		Msg("收到视频号消息，开始自动创建下载任务")

	payload, err := c.createFeedTaskBody(objectID, objectNonceID, "", "", false, false)
	if err != nil {
		c.logger.Error().Err(err).Msg("构建下载任务失败")
		return err
	}

	// 记录 payload 内容
	if payloadJSON, err := json.Marshal(payload); err == nil {
		c.logger.Info().Str("payload", string(payloadJSON)).Msg("创建 payload 成功")
	} else {
		c.logger.Warn().Err(err).Msg("序列化 payload 用于日志记录失败")
	}

	if payload.Id == "" {
		return fmt.Errorf("缺少 feed id")
	}

	// 发送创建任务请求
	var targetURL string
	if c.cfg.RemoteServerEnabled {
		protocol := c.cfg.RemoteServerProtocol
		if protocol == "" {
			protocol = "http"
		}
		targetURL = fmt.Sprintf("%s://%s:%d/api/task/create", protocol, c.cfg.RemoteServerHostname, c.cfg.RemoteServerPort)
	} else {
		protocol := c.cfg.Protocol
		if protocol == "" {
			protocol = "http"
		}
		hostname := c.cfg.Hostname
		if hostname == "0.0.0.0" {
			hostname = "127.0.0.1"
		}
		targetURL = fmt.Sprintf("%s://%s:%d/api/task/create", protocol, hostname, c.cfg.Port)
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		c.logger.Error().Err(err).Msg("序列化请求参数失败")
		return fmt.Errorf("序列化请求参数失败: %w", err)
	}

	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error().Err(err).Str("url", targetURL).Msg("请求创建任务接口失败")
		return fmt.Errorf("请求创建任务接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.logger.Error().Int("status", resp.StatusCode).Str("body", string(bodyBytes)).Msg("创建任务失败")
		return fmt.Errorf("创建任务失败, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	c.logger.Info().
		Str("url", targetURL).
		Msg("自动创建下载任务请求发送成功")

	return nil
}
