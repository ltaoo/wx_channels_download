package services

import (
	"testing"

	"wx_channel/internal/database/model"
)

func TestComputeEffectiveTaskStatus(t *testing.T) {
	tests := []struct {
		name     string
		dbStatus int
		files    []DownloadTaskFileRecord
		want     int
	}{
		{
			name:     "all finished",
			dbStatus: model.TaskStatusDownloading,
			files: []DownloadTaskFileRecord{
				{Status: "finished"},
				{Status: "finished"},
				{Status: "finished"},
			},
			want: model.TaskStatusFinished,
		},
		{
			name:     "mixed status with downloading",
			dbStatus: model.TaskStatusDownloading,
			files: []DownloadTaskFileRecord{
				{Status: "finished"},
				{Status: "downloading"},
				{Status: "waiting"},
			},
			want: model.TaskStatusDownloading,
		},
		{
			name:     "mixed status with waiting only",
			dbStatus: model.TaskStatusDownloading,
			files: []DownloadTaskFileRecord{
				{Status: "finished"},
				{Status: "finished"},
				{Status: "waiting"},
			},
			want: model.TaskStatusWaiting,
		},
		{
			name:     "preserves paused",
			dbStatus: model.TaskStatusPaused,
			files: []DownloadTaskFileRecord{
				{Status: "finished"},
				{Status: "downloading"},
			},
			want: model.TaskStatusPaused,
		},
		{
			name:     "preserves failed",
			dbStatus: model.TaskStatusFailed,
			files: []DownloadTaskFileRecord{
				{Status: "finished"},
				{Status: "waiting"},
			},
			want: model.TaskStatusFailed,
		},
		{
			name:     "preserves cancelled",
			dbStatus: model.TaskStatusCancelled,
			files: []DownloadTaskFileRecord{
				{Status: "finished"},
			},
			want: model.TaskStatusCancelled,
		},
		{
			name:     "preserves merging",
			dbStatus: model.TaskStatusMerging,
			files: []DownloadTaskFileRecord{
				{Status: "finished"},
				{Status: "finished"},
			},
			want: model.TaskStatusMerging,
		},
		{
			name:     "db finished with waiting resources",
			dbStatus: model.TaskStatusFinished,
			files: []DownloadTaskFileRecord{
				{Status: "finished"},
				{Status: "finished"},
				{Status: "finished"},
				{Status: "finished"},
				{Status: "waiting"},
			},
			want: model.TaskStatusWaiting,
		},
		{
			name:     "db finished with all finished resources",
			dbStatus: model.TaskStatusFinished,
			files: []DownloadTaskFileRecord{
				{Status: "finished"},
				{Status: "finished"},
			},
			want: model.TaskStatusFinished,
		},
		{
			name:     "empty files list",
			dbStatus: model.TaskStatusDownloading,
			files:    []DownloadTaskFileRecord{},
			want:     model.TaskStatusDownloading,
		},
		{
			name:     "all files waiting",
			dbStatus: model.TaskStatusDownloading,
			files: []DownloadTaskFileRecord{
				{Status: "waiting"},
				{Status: "waiting"},
			},
			want: model.TaskStatusWaiting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeEffectiveTaskStatus(tt.dbStatus, tt.files)
			if got != tt.want {
				t.Errorf("computeEffectiveTaskStatus(%d, files) = %d, want %d", tt.dbStatus, got, tt.want)
			}
		})
	}
}
