package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"go.etcd.io/bbolt"
	"gorm.io/gorm"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/download"

	"wx_channel/internal/database/model"
	pkgdatabase "wx_channel/pkg/database"
	"wx_channel/pkg/util"
)

var (
	migrateSource   string
	migrateTarget   string
	migratePlatform string
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
