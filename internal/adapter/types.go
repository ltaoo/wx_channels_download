package adapter

import (
	"errors"

	"wx_channel/internal/database/model"
)

var ErrBrowseHistoryNotSupported = errors.New("browse history is not supported")

// DownloadTaskResult is the result of building a download task by a platform handler.
type DownloadTaskResult struct {
	Task      *model.DownloadTask
	Resources []*ResourceInfo
	// Account for the content
	Account *model.Account
	// Content is the base content model (common fields).
	Content *model.Content
	// Extension carries content-type-specific data set by platform adapters.
	ContentDetail any
}

// BrowseHistoryResult is returned by a platform BrowseHistoryBuilder.
type BrowseHistoryResult struct {
	BrowseHistory *model.BrowseHistory
	Account       *model.Account
}

// ResourceInfo describes a resource and its mirror endpoints.
type ResourceInfo struct {
	Resource      model.DownloadResource
	Endpoints     []model.DownloadEndpoint
	ContentAssets []ContentAssetReference
}

// ContentAssetReference connects an adapter resource to a stable archive asset.
// SubjectType and SubjectKey optionally bind the asset to a nested entity such
// as a novel chapter.
type ContentAssetReference struct {
	Kind            string `json:"kind"`
	Role            string `json:"role"`
	AssetKey        string `json:"asset_key"`
	Relation        string `json:"relation"`
	SubjectType     string `json:"subject_type,omitempty"`
	SubjectKey      string `json:"subject_key,omitempty"`
	SubjectRelation string `json:"subject_relation,omitempty"`
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
		Resource:  resource,
		Endpoints: endpoints,
	})
	return r
}
