package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/api/services"
	"wx_channel/internal/assets"
	"wx_channel/internal/channels"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	downloaderclient "wx_channel/internal/downloader"
	"wx_channel/internal/officialaccount"
	"wx_channel/internal/storage"
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
	db            *database.ClientDatabase
	logger        *zerolog.Logger

	// Services
	downloadService *services.DownloadService
	channelsService *services.ChannelsService
	accountService  *services.AccountService
	contentService  *services.ContentService
}

func NewAPIClient(cfg *APIConfig, parent_logger *zerolog.Logger, db *database.ClientDatabase) *APIClient {
	data_dir := cfg.RootDir
	logger := parent_logger.With().Str("Client", "api_client").Logger()
	var st downloadpkg.Storage
	if db != nil && db.DB() != nil {
		st = storage.NewSqliteStorage(db.DB(), &logger, cfg.DownloadDir)
	} else {
		st = downloadpkg.NewBoltStorage(data_dir)
	}
	downloader := downloadpkg.NewDownloader(&downloadpkg.DownloaderConfig{
		RefreshInterval: 360,
		Storage:         st,
		StorageDir:      data_dir,
	})
	var channels_client *channels.ChannelsClient
	official_cfg := officialaccount.NewOfficialAccountConfig(cfg.Original, cfg.RemoteServerMode)
	officialaccount_client := officialaccount.NewOfficialAccountClient(official_cfg, parent_logger)
	channels_client = channels.NewChannelsClient(cfg.ChannelsRefreshInterval)
	if db != nil && db.DB() != nil {
		channels_client.SetDB(db.DB())
	}
	downloader_ws := downloaderclient.NewDownloaderClient()

	// get_sorted_tasks := func() []*downloadpkg.Task {
	// 	tasks := downloader.GetTasks()
	// 	sort.Slice(tasks, func(i, j int) bool {
	// 		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	// 	})
	// 	return tasks
	// }

	downloader_ws.OnConnected = func(client *downloaderclient.WSClient) {
		// all_tasks := get_sorted_tasks()
		// limit := 50
		// if limit > len(all_tasks) {
		// 	limit = len(all_tasks)
		// }
		// tasks := all_tasks[:limit]
		// if data, err := json.Marshal(APIClientWSMessage{Type: "tasks", Data: map[string]interface{}{
		// 	"list":  tasks,
		// 	"total": len(all_tasks),
		// }}); err == nil {
		// 	client.Send <- data
		// }
	}

	downloader_ws.OnMessage = func(client *downloaderclient.WSClient, message []byte) {
	}

	// Initialize services
	downloadService := services.NewDownloadService(downloader, db, cfg.DownloadDir, downloader_ws)
	channelsService := services.NewChannelsService(channels_client)
	accountService := services.NewAccountService(db)
	contentService := services.NewContentService(db)

	client := &APIClient{
		downloader:      downloader,
		official:        officialaccount_client,
		channels:        channels_client,
		downloader_ws:   downloader_ws,
		filehelper:      NewFileHelperHandler(),
		formatter:       util.NewFilenameProcessor(cfg.DownloadDir, make(map[string]int)),
		cfg:             cfg,
		engine:          gin.Default(),
		db:              db,
		logger:          &logger,
		downloadService: downloadService,
		channelsService: channelsService,
		accountService:  accountService,
		contentService:  contentService,
	}

	// 设置文件传输助手视频号自动下载回调
	client.filehelper.SetFinderAutoDownloadCallback(client.autoCreateChannelsTask)
	// 设置文件传输助手 SPH 自动下载回调
	client.filehelper.SetSphAutoDownloadCallback(client.autoDownloadSphVideo)

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
	if c.db != nil {
		_ = c.db.Close()
		c.db = nil
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

func (c *APIClient) check_existing_feed(tasks []*downloadpkg.Task, body *services.FeedDownloadTaskBody) bool {
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

func (c *APIClient) createFeedTaskBody(oid, nid, reqUrl, eid string, isMp3, isCover bool, customSpec ...string) (*FeedDownloadTaskBody, error) {
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

	spec := ""
	if len(customSpec) > 0 && customSpec[0] != "" {
		spec = customSpec[0]
	} else if !c.cfg.Original.GetBool("download.defaultHighest") {
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

	payload := &services.FeedDownloadTaskBody{
		Id:       feed.ID,
		NonceId:  nid,
		Title:    feed.ObjectDesc.Description,
		Key:      key,
		Spec:     spec,
		Suffix:   ".mp4",
		URL:      media.URL + media.URLToken,
		Filename: filename,
	}

	// 处理 URL：非空 spec 添加 X-snsvideoflag 参数，空 spec 则清理 URL 只保留 encfilekey 和 token
	if !isCover {
		if spec != "" {
			payload.URL += "&X-snsvideoflag=" + spec
		} else {
			if u, err := url.Parse(payload.URL); err == nil {
				filekey := u.Query().Get("encfilekey")
				token := u.Query().Get("token")
				if filekey != "" && token != "" {
					newURL := u.Scheme + "://" + u.Host + u.Path
					newURL += "?encfilekey=" + filekey + "&token=" + token
					payload.URL = newURL
				}
			}
		}
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
		if bgmURL := feed.ObjectDesc.FollowPostInfo.MusicInfo.MediaStreamingUrl; bgmURL != "" {
			files = append(files, map[string]string{
				"url":      bgmURL,
				"filename": "bgm.mp3",
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
		errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", err.Error())
		c.logger.Error().Err(err).Msg("构建下载任务失败")
		c.sendMessageToFilehelper(errMsg)
		return err
	}

	// 记录 payload 内容
	if payloadJSON, err := json.Marshal(payload); err == nil {
		c.logger.Info().Str("payload", string(payloadJSON)).Msg("创建 payload 成功")
	} else {
		c.logger.Warn().Err(err).Msg("序列化 payload 用于日志记录失败")
	}

	if payload.Id == "" {
		errMsg := "✗ 视频号下载失败: 缺少 feed id"
		c.logger.Error().Msg("缺少 feed id")
		c.sendMessageToFilehelper(errMsg)
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
		errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", err.Error())
		c.logger.Error().Err(err).Msg("序列化请求参数失败")
		c.sendMessageToFilehelper(errMsg)
		return fmt.Errorf("序列化请求参数失败: %w", err)
	}

	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", err.Error())
		c.logger.Error().Err(err).Str("url", targetURL).Msg("请求创建任务接口失败")
		c.sendMessageToFilehelper(errMsg)
		return fmt.Errorf("请求创建任务接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("✗ 视频号下载失败: %s", string(bodyBytes))
		c.logger.Error().Int("status", resp.StatusCode).Str("body", string(bodyBytes)).Msg("创建任务失败")
		c.sendMessageToFilehelper(errMsg)
		return fmt.Errorf("创建任务失败, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	c.logger.Info().
		Str("url", targetURL).
		Msg("自动创建下载任务请求发送成功")

	// 发送成功消息
	successMsg := fmt.Sprintf("✓ 视频号已开始下载: %s", payload.Filename)
	c.sendMessageToFilehelper(successMsg)

	return nil
}

// sendMessageToFilehelper 发送消息到 filehelper
func (c *APIClient) sendMessageToFilehelper(msg string) {
	go func() {
		fhClient := c.filehelper.GetClient()
		if err := fhClient.SendText(msg); err != nil {
			c.logger.Warn().Err(err).Str("msg", msg).Msg("发送消息失败")
		} else {
			c.logger.Info().Str("msg", msg).Msg("消息发送成功")
		}
	}()
}

// autoDownloadSphVideo 从 SPH URL 自动下载视频
func (c *APIClient) autoDownloadSphVideo(sphUrl string) error {
	c.logger.Info().
		Str("url", sphUrl).
		Msg("收到 SPH URL，开始获取视频信息")

	// 从配置获取 cookie
	cookie := c.cfg.CloudflareSphCookie
	if cookie == "" {
		c.logger.Error().Msg("cloudflare.sphCookie not configured")
		c.sendMessageToFilehelper("✗ SPH 下载失败: 未配置 cookie")
		return fmt.Errorf("cloudflare.sphCookie not configured")
	}

	// 获取视频信息
	feedResp, err := fetchVideoProfileWithShareUrl(sphUrl, cookie)
	if err != nil {
		c.logger.Error().Err(err).Msg("获取视频信息失败")
		c.sendMessageToFilehelper(fmt.Sprintf("✗ SPH 下载失败: %s", err.Error()))
		return fmt.Errorf("获取视频信息失败: %w", err)
	}

	// 处理 video URL：仅保留 encfilekey 和 token 参数，存储为 originVideoUrl
	if feedResp != nil && feedResp.Data.Feedinfo.Videourl != "" {
		feedResp.Data.Feedinfo.OriginVideoUrl = cleanVideoURL(feedResp.Data.Feedinfo.Videourl)
	}

	if feedResp == nil || feedResp.Data.Feedinfo.OriginVideoUrl == "" {
		c.logger.Error().Msg("获取 originVideoUrl 失败")
		c.sendMessageToFilehelper("✗ SPH 下载失败: 无法获取视频链接")
		return fmt.Errorf("获取 originVideoUrl 失败")
	}

	// 构建下载任务
	downloadUrl := feedResp.Data.Feedinfo.OriginVideoUrl
	filename := feedResp.Data.Feedinfo.Description
	if filename == "" {
		filename = feedResp.Data.Authorinfo.Nickname + "_" + util.NowSecondsStr()
	}
	// 添加 .mp4 后缀
	if !strings.HasSuffix(filename, ".mp4") {
		filename += ".mp4"
	}

	// 创建下载任务
	taskBody := &DownloadTaskPayload{
		URL:      downloadUrl,
		Filename: filename,
		Dir:      "",
		Extra:    make(map[string]string),
	}

	// 发送创建任务请求
	var targetURL string
	if c.cfg.RemoteServerEnabled {
		protocol := c.cfg.RemoteServerProtocol
		if protocol == "" {
			protocol = "http"
		}
		targetURL = fmt.Sprintf("%s://%s:%d/api/task/create2", protocol, c.cfg.RemoteServerHostname, c.cfg.RemoteServerPort)
	} else {
		protocol := c.cfg.Protocol
		if protocol == "" {
			protocol = "http"
		}
		hostname := c.cfg.Hostname
		if hostname == "0.0.0.0" {
			hostname = "127.0.0.1"
		}
		targetURL = fmt.Sprintf("%s://%s:%d/api/task/create2", protocol, hostname, c.cfg.Port)
	}

	jsonData, err := json.Marshal(taskBody)
	if err != nil {
		c.logger.Error().Err(err).Msg("序列化请求参数失败")
		c.sendMessageToFilehelper(fmt.Sprintf("✗ SPH 下载失败: %s", err.Error()))
		return fmt.Errorf("序列化请求参数失败: %w", err)
	}

	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error().Err(err).Str("url", targetURL).Msg("请求创建任务接口失败")
		c.sendMessageToFilehelper(fmt.Sprintf("✗ SPH 下载失败: %s", err.Error()))
		return fmt.Errorf("请求创建任务接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.logger.Error().Int("status", resp.StatusCode).Str("body", string(bodyBytes)).Msg("创建任务失败")
		c.sendMessageToFilehelper(fmt.Sprintf("✗ SPH 下载失败: 创建任务失败"))
		return fmt.Errorf("创建任务失败, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	c.logger.Info().
		Str("url", targetURL).
		Str("filename", filename).
		Msg("自动创建 SPH 下载任务请求发送成功")

	// 发送下载开始的消息到 filehelper
	successMsg := fmt.Sprintf("✓ SPH 已开始下载: %s", filename)
	c.sendMessageToFilehelper(successMsg)

	return nil
}

func (c *APIClient) CreateContentDownloadTask(content *model.Content, t *downloadpkg.Task, reason string) (*model.DownloadTask, error) {
	if content == nil {
		return nil, errors.New("content is nil")
	}
	if t == nil {
		return nil, errors.New("download task is nil")
	}
	if c.db == nil || c.db.DB() == nil {
		return nil, errors.New("db is nil")
	}

	db := c.db.DB()

	title := ""
	if t.Meta != nil && t.Meta.Opts != nil {
		title = strings.TrimSpace(t.Meta.Opts.Name)
	}
	if title == "" {
		title = strings.TrimSpace(content.Title)
	}

	taskURL := strings.TrimSpace(content.ContentURL)
	if taskURL == "" {
		taskURL = strings.TrimSpace(content.URL)
	}
	if taskURL == "" && t.Meta != nil && t.Meta.Req != nil {
		taskURL = strings.TrimSpace(t.Meta.Req.URL)
	}

	size := content.Size
	if size <= 0 {
		size = content.FileSize
	}

	meta2Bytes, _ := json.Marshal(map[string]any{
		"platform":    content.PlatformId,
		"external_id": content.ExternalId,
		"nonce_id":    content.ExternalId2,
		"eid":         "",
	})

	statusToInt := func(s base.Status) int {
		switch s {
		case base.DownloadStatusReady:
			return 0
		case base.DownloadStatusRunning:
			return 1
		case base.DownloadStatusPause:
			return 2
		case base.DownloadStatusWait:
			return 3
		case base.DownloadStatusDone:
			return 4
		case base.DownloadStatusError:
			return 5
		default:
			return 0
		}
	}

	var rec model.DownloadTask
	err := db.Where("task_id = ?", t.ID).First(&rec).Error
	updates := map[string]any{
		"url":         taskURL,
		"external_id": content.ExternalId,
		"title":       title,
		"cover_url":   content.CoverURL,
		"metadata2":   string(meta2Bytes),
		"reason":      reason,
		"updated_at":  util.TimeToMillisInt64(t.UpdatedAt),
	}
	if size > 0 {
		updates["size"] = size
	}

	if err == nil {
		if err := db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(updates).Error; err != nil {
			return nil, err
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		rec = model.DownloadTask{
			TaskId:     t.ID,
			Status:     statusToInt(t.Status),
			Protocol:   t.Protocol,
			URL:        taskURL,
			ExternalId: content.ExternalId,
			Title:      title,
			CoverURL:   content.CoverURL,
			Size:       size,
			Reason:     reason,
			Metadata2:  string(meta2Bytes),
			Timestamps: model.Timestamps{
				CreatedAt: util.TimeToMillisInt64(t.CreatedAt),
				UpdatedAt: util.TimeToMillisInt64(t.UpdatedAt),
			},
		}
		if err := db.Create(&rec).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	now := util.NowMillis()
	if err := db.Model(&model.Content{}).Where("id = ?", content.Id).Updates(map[string]any{
		"download_task_id": rec.Id,
		"download_status":  rec.Status,
		"download_path":    rec.Filepath,
		"updated_at":       now,
	}).Error; err != nil {
		return &rec, err
	}

	return &rec, nil
}
