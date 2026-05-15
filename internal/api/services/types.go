package services

import (
	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"gorm.io/gorm"
)

type TaskFilter struct {
	Statuses []base.Status
	IDs      []string
}

type TaskInfo struct {
	ID        string
	Status    string
	Name      string
	Path      string
	Progress  *TaskProgress
	Meta      *TaskMeta
	CreatedAt int64
	UpdatedAt int64
}

type TaskProgress struct {
	Downloaded int64
	Speed      int64
}

type TaskMeta struct {
	Req  *TaskReq
	Opts *TaskOpts
	Res  *TaskRes
}

type TaskRes struct {
	Size  int64
	Name  string
	Files []TaskFile
}

type TaskFile struct {
	Name string
	Size int64
}

type TaskReq struct {
	URL    string
	Labels map[string]string
}

type TaskOpts struct {
	Name string
	Path string
}

func ConvertTask(t *downloadpkg.Task) *TaskInfo {
	if t == nil {
		return nil
	}
	info := &TaskInfo{
		ID:        t.ID,
		Status:    string(t.Status),
		CreatedAt: t.CreatedAt.UnixMilli(),
		UpdatedAt: t.UpdatedAt.UnixMilli(),
	}
	if t.Meta != nil {
		info.Meta = &TaskMeta{}
		if t.Meta.Req != nil {
			info.Meta.Req = &TaskReq{
				URL:    t.Meta.Req.URL,
				Labels: t.Meta.Req.Labels,
			}
		}
		if t.Meta.Opts != nil {
			info.Meta.Opts = &TaskOpts{
				Name: t.Meta.Opts.Name,
				Path: t.Meta.Opts.Path,
			}
		}
		if t.Meta.Res != nil {
			info.Meta.Res = &TaskRes{
				Size: t.Meta.Res.Size,
				Name: t.Meta.Res.Name,
			}
		}
	}
	if t.Progress != nil {
		info.Progress = &TaskProgress{
			Downloaded: t.Progress.Downloaded,
			Speed:      t.Progress.Speed,
		}
	}
	return info
}

type PageResult struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

type FeedDownloadTaskBody struct {
	Id       string `json:"id"`
	NonceId  string `json:"nonce_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Key      int    `json:"key"`
	Spec     string `json:"spec"`
	Suffix   string `json:"suffix"`
}

type ChannelsFeedProfile struct {
	ObjectId    string
	NonceId     string
	SourceURL   string
	URL         string
	Title       string
	DecryptKey  string
	CoverURL    string
	CoverWidth  int
	CoverHeight int
	Duration    int
	FileSize    int
	CreatedAt   int
	Spec        []interface{}
	Contact     FeedContact
}

type FeedContact struct {
	Username  string
	Nickname  string
	AvatarURL string
}

type DBClient interface {
	DB() *gorm.DB
}
