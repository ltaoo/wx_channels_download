package types

import (
	"wx_channel/internal/database/model"
)

// DownloadTaskResult is the result of building a download task by a platform handler.
type DownloadTaskResult struct {
	Task      *model.DownloadTask
	Resources []*ResourceInfo
	// Extension carries content-type-specific data set by platform adapters.
	ContentDetail any
	// Sub-entity lists saved alongside ContentDetail in PrepareTask.
	AlbumImages   []*model.ContentImage
	NovelVolumes  []*model.ContentNovelVolume
	NovelChapters []*model.ContentNovelChapter
	// Account for the content
	Account *model.Account
	// Content is the base content model (common fields).
	Content *model.Content
}

// ResourceInfo describes a resource and its mirror endpoints. DownloadResource is embedded for direct access to Name/Kind/Size fields.
type ResourceInfo struct {
	model.DownloadResource
	Endpoints []model.DownloadEndpoint
}

// NewDownloadTaskResult creates a DownloadTaskResult with the given task-level fields.
func NewDownloadTaskResult(name, uniqueID, platformID, configJSON, metadataJSON string) *DownloadTaskResult {
	return &DownloadTaskResult{
		Task: &model.DownloadTask{
			Name:         name,
			UniqueID:     uniqueID,
			PlatformId:   platformID,
			ConfigJSON:   configJSON,
			MetadataJSON: metadataJSON,
		},
	}
}

// AddResource appends a resource and its endpoints, returning self for chaining.
func (r *DownloadTaskResult) AddResource(resource model.DownloadResource, endpoints ...model.DownloadEndpoint) *DownloadTaskResult {
	r.Resources = append(r.Resources, &ResourceInfo{
		DownloadResource: resource,
		Endpoints:        endpoints,
	})
	return r
}
