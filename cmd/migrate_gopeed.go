package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go.etcd.io/bbolt"
	"gorm.io/gorm"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/download"

	"wx_channel/internal/database/model"
	pkgdatabase "wx_channel/pkg/database"
	"wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/util"
)

var (
	migrateSource   string
	migrateTarget   string
	migratePlatform string
	enrichCacheDir  string
)

var migrateGopeedCmd = &cobra.Command{
	Use:   "migrate-gopeed",
	Short: "将 gopeed.db 中的下载任务迁移到新的 SQLite 数据库",
	Run: func(cmd *cobra.Command, args []string) {
		runGopeedMigration()
	},
}

func init() {
	migrateGopeedCmd.Flags().StringVar(&migrateSource, "source", "gopeed.db", "源 BoltDB 文件路径")
	migrateGopeedCmd.Flags().StringVar(&migrateTarget, "target", "", "目标 SQLite 数据库文件路径（必需）")
	migrateGopeedCmd.Flags().StringVar(&migratePlatform, "platform", "wx_channels", "迁移时使用的默认平台标识")
	migrateGopeedCmd.Flags().StringVar(&enrichCacheDir, "cache-file", "cache/channels_feed_profile.json", "API 响应缓存文件路径（单文件 Map 结构，可读可同步）")
	migrateGopeedCmd.MarkFlagRequired("target")
	root_cmd.AddCommand(migrateGopeedCmd)
}

func runGopeedMigration() {
	fmt.Println("=== gopeed.db 数据库迁移 ===")
	fmt.Printf("源文件: %s\n", migrateSource)
	fmt.Printf("目标数据库: %s\n", migrateTarget)

	// Verify source file exists
	if _, err := os.Stat(migrateSource); err != nil {
		fmt.Printf("[错误] 源文件不存在: %v\n", err)
		os.Exit(1)
	}

	// Open BoltDB
	boltDB, err := bbolt.Open(migrateSource, 0600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		fmt.Printf("[错误] 无法打开 BoltDB: %v\n", err)
		os.Exit(1)
	}
	defer boltDB.Close()

	// Read tasks from BoltDB
	var tasks []*download.Task
	err = boltDB.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("task"))
		if b == nil {
			return fmt.Errorf("未找到 task bucket，数据库可能为空")
		}
		return b.ForEach(func(k, v []byte) error {
			var task download.Task
			if err := json.Unmarshal(v, &task); err != nil {
				fmt.Printf("[警告] 解析任务 %s 失败: %v\n", string(k), err)
				return nil // skip broken tasks
			}
			tasks = append(tasks, &task)
			return nil
		})
	})
	if err != nil {
		fmt.Printf("[错误] 读取任务失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("从 BoltDB 读取到 %d 个任务\n", len(tasks))
	if len(tasks) == 0 {
		fmt.Println("没有需要迁移的数据。")
		return
	}

	// Connect to target SQLite database
	db, err := pkgdatabase.NewDatabase(&pkgdatabase.DatabaseConfig{
		DBType: "sqlite",
		DBPath: migrateTarget,
	}, nil)
	if err != nil {
		fmt.Printf("[错误] 无法连接目标数据库: %v\n", err)
		os.Exit(1)
	}

	// Verify target database has required tables
	if !db.Migrator().HasTable(&model.DownloadTask{}) {
		fmt.Println("[错误] 目标数据库缺少 download_task 表，请先运行主程序初始化数据库，或使用一个已有的数据库文件")
		os.Exit(1)
	}

	// Migrate tasks
	var (
		taskCount   int
		contentCount int
		skipCount   int
		errorCount  int
	)

	for _, task := range tasks {
		// Skip tasks with nil Meta
		if task.Meta == nil || task.Meta.Req == nil {
			fmt.Printf("[跳过] 任务 %s: Meta/Req 为空\n", task.ID)
			skipCount++
			continue
		}

		// Check if this task already exists
		var existing model.DownloadTask
		if err := db.Where("task_id = ?", task.ID).First(&existing).Error; err == nil {
			fmt.Printf("[跳过] 任务 %s: 已存在于目标数据库\n", task.ID)
			skipCount++
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Printf("[错误] 任务 %s: 查询重复失败 - %v\n", task.ID, err)
			errorCount++
			continue
		}

		labels := task.Meta.Req.Labels
		externalID := getLabel(labels, "id")
		nonceID := getLabel(labels, "nonce_id")
		title := getLabel(labels, "title")
		if title == "" && task.Meta.Opts != nil {
			title = task.Meta.Opts.Name
		}
		if title == "" {
			title = task.Name()
		}

		createdAt := timeToMillis(task.CreatedAt)
		updatedAt := timeToMillis(task.UpdatedAt)
		status := mapBoltStatusToInt(task.Status)

		// Build metadata2 JSON
		metadata2, _ := json.Marshal(map[string]string{
			"platform_id":  migratePlatform,
			"external_id":  externalID,
			"nonce_id":     nonceID,
			"protocol":     task.Protocol,
			"bolt_task_id": task.ID,
		})

		// Determine filepath
		filepath := ""
		if task.Meta.Res != nil && len(task.Meta.Res.Files) > 0 {
			fullPath := task.Meta.SingleFilepath()
			if task.Meta.Opts != nil && task.Meta.Opts.Path != "" {
				relPath, err := filepathRel(task.Meta.Opts.Path, fullPath)
				if err == nil && relPath != "" {
					filepath = relPath
				} else {
					filepath = fullPath
				}
			} else {
				filepath = fullPath
			}
		}

		// Determine size
		size := int64(0)
		if task.Meta.Res != nil {
			size = task.Meta.Res.Size
		}
		if size == 0 && task.Progress != nil {
			size = task.Progress.Downloaded
		}

		downloadTask := model.DownloadTask{
			TaskId:     task.ID,
			Status:     status,
			Protocol:   task.Protocol,
			URL:        task.Meta.Req.URL,
			ExternalId: externalID,
			Title:      title,
			Size:       size,
			Downloaded: getDownloaded(task.Progress),
			Filepath:   filepath,
			Error:      task.Error,
			Metadata2:  string(metadata2),
			Timestamps: model.Timestamps{
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
		}

		if err := db.Create(&downloadTask).Error; err != nil {
			fmt.Printf("[错误] 任务 %s: 创建 download_task 失败 - %v\n", task.ID, err)
			errorCount++
			continue
		}

		// Create download task events
		createTaskEvents(db, downloadTask.Id, task)

		// Create content record if external_id exists
		if externalID != "" {
			if err := createContent(db, task, externalID, nonceID, title, labels, createdAt, updatedAt, downloadTask.Id); err != nil {
				fmt.Printf("[警告] 任务 %s: 创建 content 失败 - %v\n", task.ID, err)
			} else {
				contentCount++
			}

			// 调用 API 补充 content 和 account 信息（nonce_id 可为空）
		if err := enrichContentFromAPI(db, enrichCacheDir, externalID, nonceID); err != nil {
			fmt.Printf("[警告] 任务 %s: 补充 content 信息失败 - %v\n", task.ID, err)
		}
		}

		taskCount++
		if taskCount%50 == 0 {
			fmt.Printf("进度: 已迁移 %d 个任务...\n", taskCount)
		}
	}

	fmt.Println()
	fmt.Println("=== 迁移完成 ===")
	fmt.Printf("成功迁移任务: %d\n", taskCount)
	fmt.Printf("成功迁移内容: %d\n", contentCount)
	fmt.Printf("跳过（已存在）: %d\n", skipCount)
	fmt.Printf("失败: %d\n", errorCount)

	// Verify
	var verifyCount int64
	db.Model(&model.DownloadTask{}).Count(&verifyCount)
	fmt.Printf("目标数据库 download_task 总数: %d\n", verifyCount)
	db.Model(&model.Content{}).Count(&verifyCount)
	fmt.Printf("目标数据库 content 总数: %d\n", verifyCount)
}

func getLabel(labels map[string]string, key string) string {
	if labels == nil {
		return ""
	}
	return labels[key]
}

func timeToMillis(t time.Time) int64 {
	if t.IsZero() {
		return util.NowMillis()
	}
	return t.UnixMilli()
}

func getDownloaded(progress *download.Progress) int64 {
	if progress == nil {
		return 0
	}
	return progress.Downloaded
}

func filepathRel(basePath, targetPath string) (string, error) {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func mapBoltStatusToInt(status base.Status) int {
	switch status {
	case base.DownloadStatusReady, base.DownloadStatusWait:
		return 0
	case base.DownloadStatusRunning:
		return 1
	case base.DownloadStatusPause:
		return 2
	case base.DownloadStatusDone:
		return 3
	case base.DownloadStatusError:
		return 4
	default:
		return 0
	}
}

func mapToContentDownloadStatus(status base.Status) int {
	switch status {
	case base.DownloadStatusDone:
		return 2
	case base.DownloadStatusRunning:
		return 1
	case base.DownloadStatusError:
		return 3
	default:
		return 0
	}
}

func createTaskEvents(db *gorm.DB, taskID int, task *download.Task) {
	createEvent := model.DownloadTaskEvent{
		TaskId:    taskID,
		Type:      "create",
		Message:   fmt.Sprintf("从 gopeed.db 迁移，原始任务ID: %s", task.ID),
		CreatedAt: timeToMillis(task.CreatedAt),
	}
	if err := db.Create(&createEvent).Error; err != nil {
		fmt.Printf("[警告] 任务 %s: 创建 create 事件失败 - %v\n", task.ID, err)
	}

	// Status event based on final status
	var eventType string
	var message string
	switch task.Status {
	case base.DownloadStatusDone:
		eventType = "done"
		message = "下载完成"
	case base.DownloadStatusError:
		eventType = "error"
		message = task.Error
	case base.DownloadStatusPause:
		eventType = "pause"
		message = "下载暂停"
	case base.DownloadStatusReady, base.DownloadStatusWait:
		eventType = "pause"
		message = "下载未开始"
	default:
		return
	}

	statusEvent := model.DownloadTaskEvent{
		TaskId:    taskID,
		Type:      eventType,
		Message:   message,
		CreatedAt: timeToMillis(task.UpdatedAt),
	}
	if err := db.Create(&statusEvent).Error; err != nil {
		fmt.Printf("[警告] 任务 %s: 创建 %s 事件失败 - %v\n", task.ID, eventType, err)
	}
}

func createContent(db *gorm.DB, task *download.Task, externalID, nonceID, title string, labels map[string]string, createdAt, updatedAt int64, downloadTaskID int) error {
	// Check for existing content
	var existing model.Content
	if err := db.Where("platform_id = ? AND external_id = ?", migratePlatform, externalID).First(&existing).Error; err == nil {
		// Content already exists, update it
		updates := map[string]any{
			"title":           title,
			"description":     title,
			"external_id2":    nonceID,
			"download_status": mapToContentDownloadStatus(task.Status),
			"updated_at":      updatedAt,
		}
		if task.Meta.Req != nil {
			updates["content_url"] = task.Meta.Req.URL
		}
		if task.Meta.Res != nil {
			updates["file_size"] = task.Meta.Res.Size
		}
		if downloadTaskID > 0 {
			updates["download_task_id"] = downloadTaskID
		}
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			return err
		}
		// Also update/create ContentVideo
		ensureContentVideo(db, existing.Id, nonceID, labels)
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// Create new content
	content := model.Content{
		PlatformId:     migratePlatform,
		ContentType:    "video",
		ExternalId:     externalID,
		ExternalId2:    nonceID,
		Title:          title,
		Description:    title,
		DownloadTaskId: &downloadTaskID,
		DownloadStatus: mapToContentDownloadStatus(task.Status),
		Timestamps: model.Timestamps{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	if task.Meta.Req != nil {
		content.ContentURL = task.Meta.Req.URL
	}
	if task.Meta.Res != nil {
		content.FileSize = task.Meta.Res.Size
	}

	if err := db.Create(&content).Error; err != nil {
		return err
	}

	// Create ContentVideo
	ensureContentVideo(db, content.Id, nonceID, labels)

	return nil
}

func ensureContentVideo(db *gorm.DB, contentID int, nonceID string, labels map[string]string) {
	var existing model.ContentVideo
	if err := db.Where("content_id = ?", contentID).First(&existing).Error; err == nil {
		// Update existing
		updates := map[string]any{}
		if nonceID != "" {
			updates["nonce_id"] = nonceID
		}
		if key := getLabel(labels, "key"); key != "" {
			updates["decode_key"] = key
		}
		if len(updates) > 0 {
			db.Model(&existing).Updates(updates)
		}
		return
	}

	video := model.ContentVideo{
		ContentId: contentID,
		NonceId:   nonceID,
		DecodeKey: getLabel(labels, "key"),
	}
	db.Create(&video)
}

// enrichCache 单文件缓存结构，key 为 "oid:nid"
type enrichCache struct {
	mu      sync.RWMutex
	Entries map[string]enrichCacheEntry `json:"entries"`
}

type enrichCacheEntry struct {
	CachedAt int64           `json:"cached_at"`
	Response json.RawMessage `json:"response"`
}

// enrichContentFromAPI 调用本地 API 获取 feed profile，补充 content 和 account 信息
// cacheFile 为单个缓存文件路径，为空则不使用缓存
func enrichContentFromAPI(db *gorm.DB, cacheFile, externalID, nonceID string) error {
	// nonce_id 可能含下划线后缀（如 "5073863920001900660_0_146_0_0"），只取第一部分
	nidClean := nonceID
	if idx := strings.IndexByte(nonceID, '_'); idx > 0 {
		nidClean = nonceID[:idx]
	}

	cacheKey := externalID + ":" + nidClean
	var body []byte

	if cacheFile != "" {
		cc := loadEnrichCache(cacheFile)
		cc.mu.RLock()
		if entry, ok := cc.Entries[cacheKey]; ok {
			body = entry.Response
		}
		cc.mu.RUnlock()
	}

	// 缓存未命中，调用 API
	if body == nil {
		url := fmt.Sprintf("http://127.0.0.1:3022/api/channels/feed/profile?oid=%s&nid=%s", externalID, nidClean)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("API 请求失败: %w", err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("读取响应失败: %w", err)
		}
	}

	// 解析 API 响应
	var apiResp struct {
		Code int                                `json:"code"`
		Msg  string                             `json:"msg"`
		Data wxchannels.ChannelsFeedProfileResp `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if apiResp.Code != 0 {
		return fmt.Errorf("API 错误 (code=%d): %s", apiResp.Code, apiResp.Msg)
	}

	if apiResp.Data.ErrCode != 0 {
		return fmt.Errorf("API 业务错误 (errCode=%d): %s", apiResp.Data.ErrCode, apiResp.Data.ErrMsg)
	}

	// 成功后写入缓存
	if cacheFile != "" && len(body) > 0 {
		cc := loadEnrichCache(cacheFile)
		cc.mu.Lock()
		cc.Entries[cacheKey] = enrichCacheEntry{
			CachedAt: util.NowMillis(),
			Response: body,
		}
		data, _ := json.MarshalIndent(cc, "", "  ")
		cc.mu.Unlock()
		os.MkdirAll(filepath.Dir(cacheFile), 0755)
		os.WriteFile(cacheFile, data, 0644)
	}

	obj := apiResp.Data.Data.Object
	if obj.ID == "" {
		return fmt.Errorf("API 返回的 Object ID 为空")
	}

	// 转换为 ChannelsFeedProfile
	profile, err := wxchannels.ChannelsObjectToChannelsFeedProfile(&obj)
	if err != nil {
		return fmt.Errorf("转换 profile 失败: %w", err)
	}

	// 确保 SourceURL 已设置
	if profile.SourceURL == "" {
		profile.SourceURL = wxchannels.BuildJumpURL(profile)
	}

	// 若 URL 为空（非视频类型），使用 SourceURL 作为回退
	if profile.URL == "" {
		profile.URL = profile.SourceURL
	}

	// 使用 ChannelsClient 写入数据库（content + account + content_account）
	client := newChannelsClientWithDB(db)
	if _, err := client.UpsertChannelsFeed(profile); err != nil {
		return fmt.Errorf("写入数据库失败: %w", err)
	}

	return nil
}

// loadEnrichCache 加载缓存文件（不存在则返回空缓存）
func loadEnrichCache(path string) *enrichCache {
	cc := &enrichCache{Entries: make(map[string]enrichCacheEntry)}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, cc)
	}
	if cc.Entries == nil {
		cc.Entries = make(map[string]enrichCacheEntry)
	}
	return cc
}

// newChannelsClientWithDB 创建一个仅用于数据库操作的 ChannelsClient
func newChannelsClientWithDB(db *gorm.DB) *wxchannels.ChannelsClient {
	client := wxchannels.NewChannelsClient(0)
	client.SetDB(db)
	return client
}

// --- enrich-download-contents 命令 ---

var (
	enrichTarget string
	enrichLimit  int
	enrichDryRun bool
	enrichPort   int
)

var enrichDownloadContentsCmd = &cobra.Command{
	Use:   "enrich-download-contents",
	Short: "遍历下载记录并通过 API 补充 content / author 信息",
	Run: func(cmd *cobra.Command, args []string) {
		runEnrichDownloadContents()
	},
}

func init() {
	enrichDownloadContentsCmd.Flags().StringVar(&enrichTarget, "target", "", "目标 SQLite 数据库文件路径（必需）")
	enrichDownloadContentsCmd.Flags().IntVar(&enrichLimit, "limit", 0, "最大处理数量（0表示不限制）")
	enrichDownloadContentsCmd.Flags().BoolVar(&enrichDryRun, "dry-run", false, "只读取和分析，不写库")
	enrichDownloadContentsCmd.Flags().IntVar(&enrichPort, "port", 8025, "WebSocket 服务端口")
	enrichDownloadContentsCmd.MarkFlagRequired("target")
	root_cmd.AddCommand(enrichDownloadContentsCmd)
}

func runEnrichDownloadContents() {
	fmt.Println("=== 补充下载内容信息 ===")
	fmt.Printf("目标数据库: %s\n", enrichTarget)
	if enrichDryRun {
		fmt.Println("模式: dry-run（只读不写）")
	}
	if enrichLimit > 0 {
		fmt.Printf("限制数量: %d\n", enrichLimit)
	}

	if _, err := os.Stat(enrichTarget); err != nil {
		fmt.Printf("[错误] 目标文件不存在: %v\n", err)
		os.Exit(1)
	}

	// 连接目标数据库
	db, err := pkgdatabase.NewDatabase(&pkgdatabase.DatabaseConfig{
		DBType: "sqlite",
		DBPath: enrichTarget,
	}, nil)
	if err != nil {
		fmt.Printf("[错误] 无法连接目标数据库: %v\n", err)
		os.Exit(1)
	}

	// 创建 ChannelsClient
	channelsClient := wxchannels.NewChannelsClient(0)
	channelsClient.SetDB(db)

	// 设置 HTTP 服务用于 WebSocket
	engine := gin.New()
	engine.GET("/ws/channels", channelsClient.HandleChannelsWebsocket)

	addr := fmt.Sprintf(":%d", enrichPort)
	srv := &http.Server{Addr: addr, Handler: engine}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[错误] HTTP 服务启动失败: %v\n", err)
			os.Exit(1)
		}
	}()

	fmt.Printf("WebSocket 服务地址: ws://localhost:%d/ws/channels\n", enrichPort)
	fmt.Println("请在浏览器中打开前端页面并确保已登录视频号，浏览器会自动连接此 WebSocket...")
	fmt.Println("等待 WebSocket 客户端连接...")

	// 等待 WebSocket 客户端连接（最多 60 秒）
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer waitCancel()

	connected := false
	for !connected {
		select {
		case <-waitCtx.Done():
			fmt.Println("[错误] 等待 WebSocket 客户端连接超时")
			srv.Close()
			os.Exit(1)
		default:
			if channelsClient.Available() {
				connected = true
			} else {
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
	fmt.Println("WebSocket 客户端已连接，开始处理...")
	fmt.Println()

	// 查询 download_task 记录
	var tasks []model.DownloadTask
	query := db.Where("external_id != ''")
	if enrichLimit > 0 {
		query = query.Limit(enrichLimit)
	}
	if err := query.Find(&tasks).Error; err != nil {
		fmt.Printf("[错误] 查询 download_task 失败: %v\n", err)
		srv.Close()
		os.Exit(1)
	}

	fmt.Printf("找到 %d 条记录\n\n", len(tasks))

	var successCount, skipCount, errorCount int

	for i, task := range tasks {
		externalID := task.ExternalId

		// 解析 metadata2 获取 nonce_id
		var metadata2 map[string]string
		nonceID := ""
		if task.Metadata2 != "" {
			if err := json.Unmarshal([]byte(task.Metadata2), &metadata2); err == nil {
				nonceID = metadata2["nonce_id"]
			} else {
				fmt.Printf("[%d/%d] 警告 %s: metadata2 解析失败 - %v\n", i+1, len(tasks), task.TaskId, err)
			}
		}

		if externalID == "" || nonceID == "" {
			fmt.Printf("[%d/%d] 跳过 %s: external_id 或 nonce_id 为空\n", i+1, len(tasks), task.TaskId)
			skipCount++
			continue
		}

		// 检查 content 是否已有丰富数据
		var content model.Content
		if err := db.Where("platform_id = ? AND external_id = ?", "wx_channels", externalID).First(&content).Error; err == nil {
			if content.Description != "" && content.Description != content.Title {
				fmt.Printf("[%d/%d] 跳过 %s: content 已有丰富数据\n", i+1, len(tasks), task.TaskId)
				skipCount++
				continue
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Printf("[%d/%d] 错误 %s: 查询 content 失败 - %v\n", i+1, len(tasks), task.TaskId, err)
			errorCount++
			continue
		}

		if enrichDryRun {
			fmt.Printf("[%d/%d] [DRY-RUN] 将处理 %s (external_id=%s, nonce_id=%s)\n", i+1, len(tasks), task.TaskId, externalID, nonceID)
			continue
		}

		// 检查 WebSocket 连接是否仍然可用
		if !channelsClient.Available() {
			fmt.Printf("[%d/%d] 错误: WebSocket 连接已断开\n", i+1, len(tasks))
			errorCount++
			srv.Close()
			os.Exit(1)
		}

		// 调用 API 获取 feed profile
		fmt.Printf("[%d/%d] 处理 %s (external_id=%s)...\n", i+1, len(tasks), task.TaskId, externalID)
		resp, err := channelsClient.FetchChannelsFeedProfile(externalID, nonceID, "", "")
		if err != nil {
			fmt.Printf("[%d/%d] 错误 %s: API 调用失败 - %v\n", i+1, len(tasks), task.TaskId, err)
			errorCount++
			continue
		}

		if resp.ErrCode != 0 {
			fmt.Printf("[%d/%d] 错误 %s: API 返回错误 (code=%d) - %s\n", i+1, len(tasks), task.TaskId, resp.ErrCode, resp.ErrMsg)
			errorCount++
			continue
		}

		if resp.Data.Object.ID == "" {
			fmt.Printf("[%d/%d] 错误 %s: API 返回的 Object ID 为空\n", i+1, len(tasks), task.TaskId)
			errorCount++
			continue
		}

		// 转换为 ChannelsFeedProfile
		profile, err := wxchannels.ChannelsObjectToChannelsFeedProfile(&resp.Data.Object)
		if err != nil {
			fmt.Printf("[%d/%d] 错误 %s: 转换 profile 失败 - %v\n", i+1, len(tasks), task.TaskId, err)
			errorCount++
			continue
		}

		// 写入数据库（content + account + content_account）
		if _, err := channelsClient.UpsertChannelsFeed(profile); err != nil {
			fmt.Printf("[%d/%d] 错误 %s: 写入数据库失败 - %v\n", i+1, len(tasks), task.TaskId, err)
			errorCount++
			continue
		}

		successCount++
	}

	fmt.Println()
	fmt.Println("=== 处理完成 ===")
	fmt.Printf("成功: %d\n", successCount)
	fmt.Printf("跳过: %d\n", skipCount)
	fmt.Printf("失败: %d\n", errorCount)

	// 验证
	var contentCount int64
	db.Model(&model.Content{}).Where("description IS NOT NULL AND description != '' AND description != title").Count(&contentCount)
	fmt.Printf("数据库中已补充丰富信息的 content 总数: %d\n", contentCount)
	var accountCount int64
	db.Model(&model.Account{}).Where("platform_id = ?", "wx_channels").Count(&accountCount)
	fmt.Printf("数据库中 wx_channels account 总数: %d\n", accountCount)
	var contentAccountCount int64
	db.Model(&model.ContentAccount{}).Where("role = ?", "owner").Count(&contentAccountCount)
	fmt.Printf("数据库中 owner 关联总数: %d\n", contentAccountCount)

	// 关闭 HTTP 服务
	srv.Close()
}
