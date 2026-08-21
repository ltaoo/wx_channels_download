package services

import (
	"path/filepath"
	"strings"

	"wx_channel/internal/database/model"
)

// DownloadTaskItem is the compact task shape returned after creating a task.
type DownloadTaskItem struct {
	ID           int                    `json:"id"`
	ContentID    *string                `json:"content_id,omitempty"`
	ParentTaskID *int                   `json:"parent_task_id,omitempty"`
	RootTaskID   int                    `json:"root_task_id"`
	RelationType string                 `json:"relation_type,omitempty"`
	Name         string                 `json:"name"`
	UniqueID     string                 `json:"unique_id"`
	PlatformID   string                 `json:"platform_id"`
	Status       int                    `json:"status"`
	SourceURL    string                 `json:"source_url,omitempty"`
	CoverURL     string                 `json:"cover_url,omitempty"`
	CoverWidth   string                 `json:"cover_width,omitempty"`
	CoverHeight  string                 `json:"cover_height,omitempty"`
	ConfigJSON   string                 `json:"config_json,omitempty"`
	MetadataJSON string                 `json:"metadata_json,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Resources    []DownloadResourceItem `json:"resources"`
	CreatedAt    int64                  `json:"created_at"`
	UpdatedAt    int64                  `json:"updated_at"`
}

// DownloadResourceItem is the compact resource shape embedded in DownloadTaskItem.
type DownloadResourceItem struct {
	ID          int     `json:"id"`
	TaskID      *int    `json:"task_id,omitempty"`
	ContentID   *string `json:"content_id,omitempty"`
	DownloadDir string  `json:"download_dir"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Type        string  `json:"type"`
	UniqueID    string  `json:"unique_id"`
	Size        int64   `json:"size"`
	Downloaded  int64   `json:"downloaded"`
	Speed       int64   `json:"speed"`
	Status      int     `json:"status"`
	OutputPath  string  `json:"output_path,omitempty"`
	FilePath    string  `json:"file_path,omitempty"`
}

// BuildDownloadTaskItem converts a task creation result into its public compact view.
func BuildDownloadTaskItem(create_result *CreateTaskResult) DownloadTaskItem {
	if create_result == nil {
		return DownloadTaskItem{}
	}
	task := create_result.Task
	resources := make([]DownloadResourceItem, 0, len(create_result.Resources))
	for _, resource := range create_result.Resources {
		resources = append(resources, build_download_resource_item(resource))
	}
	return DownloadTaskItem{
		ID:           task.Id,
		ContentID:    task.ContentId,
		ParentTaskID: task.ParentTaskID,
		RootTaskID:   task.RootTaskID,
		RelationType: task.RelationType,
		Name:         task.Name,
		UniqueID:     task.UniqueID,
		PlatformID:   task.PlatformId,
		Status:       task.Status,
		SourceURL:    task.SourceURL,
		CoverURL:     task.CoverURL,
		CoverWidth:   task.CoverWidth,
		CoverHeight:  task.CoverHeight,
		ConfigJSON:   task.ConfigJSON,
		MetadataJSON: task.MetadataJSON,
		ErrorMessage: task.ErrorMessage,
		Resources:    resources,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

func build_download_resource_item(resource model.DownloadResource) DownloadResourceItem {
	file_path := ""
	if strings.TrimSpace(resource.DownloadDir) != "" {
		file_path = filepath.Join(resource.DownloadDir, resource.Name)
	}
	return DownloadResourceItem{
		ID:          resource.Id,
		TaskID:      resource.TaskId,
		ContentID:   resource.ContentId,
		DownloadDir: resource.DownloadDir,
		Name:        resource.Name,
		Kind:        resource.Kind,
		Type:        resource.Type,
		UniqueID:    resource.UniqueID,
		Size:        resource.Size,
		Downloaded:  resource.Downloaded,
		Speed:       resource.Speed,
		Status:      resource.Status,
		OutputPath:  resource.Name,
		FilePath:    file_path,
	}
}
