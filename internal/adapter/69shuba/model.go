package shuba69

import (
	"fmt"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
)

// NovelChapter represents a single chapter in a novel.
type NovelChapter struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Index int    `json:"index"`
}

// NovelDetail contains all information about a novel.
type NovelDetail struct {
	Name       string         `json:"name"`
	Author     string         `json:"author"`
	CoverURL   string         `json:"cover_url"`
	ProfileURL string         `json:"profile_url"`
	Chapters   []NovelChapter `json:"chapters"`
}

// BuildDownloadTask builds a COLLECTION download task from a NovelDetail.
// It creates resources for profile.html, cover.jpg, and each chapter
// (saved under chapters/0001.html etc.).
func BuildDownloadTask(novel *NovelDetail, config registry.DownloadConfig) (*registry.DownloadInfo, error) {
	title := config.Filename
	if title == "" {
		title = novel.Name
	}
	savePath := config.SavePath
	if savePath == "" {
		savePath = "/downloads/69shuba"
	}

	var resources []registry.DownloadResourceInfo

	// profile page resource
	resources = append(resources, registry.DownloadResourceInfo{
		Resource: model.DownloadResource{
			Name: "profile.html",
			Kind: "profile",
		},
		Endpoints: []model.DownloadEndpoint{{
			Protocol: "https",
			URL:      novel.ProfileURL,
			Enabled:  1,
		}},
	})

	// cover image resource (optional)
	if novel.CoverURL != "" {
		resources = append(resources, registry.DownloadResourceInfo{
			Resource: model.DownloadResource{
				Name: "cover.jpg",
				Kind: "cover",
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      novel.CoverURL,
				Enabled:  1,
			}},
		})
	}

	// chapter resources (under chapters/ subdirectory)
	for _, ch := range novel.Chapters {
		resources = append(resources, registry.DownloadResourceInfo{
			Resource: model.DownloadResource{
				Name: fmt.Sprintf("chapters/%04d.html", ch.Index),
				Kind: "chapter",
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      ch.URL,
				Enabled:  1,
			}},
		})
	}

	// Use profile resource as the primary resource/endpoint
	primary := resources[0]

	return &registry.DownloadInfo{
		Task: model.DownloadTaskV1{
			Name:         title,
			ResourceType: model.ResourceTypeCollection,
			Status:       model.TaskStatusWaiting,
			SavePath:     savePath,
		},
		Resource:  primary.Resource,
		Endpoint:  primary.Endpoints[0],
		Resources: resources,
	}, nil
}
