package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

// DownloadTaskImportRequest restores one already-downloaded task from an
// exported archive (record + content details). Files stay at their archived
// paths; nothing is downloaded or copied.
type DownloadTaskImportRequest struct {
	DownloadDir string                        `json:"download_dir"`
	Archive     *services.DownloadTaskArchive `json:"archive"`
}

// DownloadTaskImportResult reports the recreated task. Updated is true when a
// task with the same identity already existed and was updated in place.
type DownloadTaskImportResult struct {
	TaskID  int  `json:"task_id"`
	Updated bool `json:"updated"`
}

// handle_import_download_task recreates a download record through the same
// persistence pipeline as task creation and sync import, without touching the
// downloader or the source files.
// POST /api/v1/download_task/import
func (c *APIClient) handle_import_download_task(ctx *gin.Context) {
	var req DownloadTaskImportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if req.Archive == nil {
		result.Err(ctx, 400, "archive 不能为空")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}
	download_dir, err := c.resolve_download_dir(req.DownloadDir)
	if err != nil {
		result.Err(ctx, 400, "准备下载目录失败: "+err.Error())
		return
	}

	var import_result DownloadTaskImportResult
	transaction_err := c.db.Transaction(func(tx *gorm.DB) error {
		import_result.Updated = download_task_archive_exists(tx, req.Archive.Task)
		created, err := services.RecreateArchivedTask(tx, *req.Archive, download_dir, nil)
		if err != nil {
			return err
		}
		import_result.TaskID = created.Task.Id
		return nil
	})
	if transaction_err != nil {
		result.Err(ctx, 400, transaction_err.Error())
		return
	}
	if c.download_task_broadcaster != nil && import_result.TaskID > 0 {
		c.download_task_broadcaster.broadcast_download_task_upsert([]int{import_result.TaskID})
		c.download_task_broadcaster.broadcast_download_task_stats()
	}
	result.Ok(ctx, import_result)
}

// download_task_archive_exists mirrors the identity lookup RecreateArchivedTask
// uses so the caller can tell an update from a fresh import.
func download_task_archive_exists(tx *gorm.DB, task model.DownloadTask) bool {
	query := tx.Where("platform_id = ? AND unique_id = ? AND created_at = ?", task.PlatformId, task.UniqueID, task.CreatedAt)
	if task.UniqueID == "" {
		query = tx.Where("platform_id = ? AND source_url = ? AND name = ? AND created_at = ?", task.PlatformId, task.SourceURL, task.Name, task.CreatedAt)
	}
	var existing model.DownloadTask
	return query.First(&existing).Error == nil
}
