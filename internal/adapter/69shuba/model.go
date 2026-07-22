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

// MockNovel returns a hardcoded NovelDetail for frontend testing.
// All endpoints point to the fsmock server at http://127.0.0.1:7001
func MockNovel() *NovelDetail {
	base := "http://127.0.0.1:7001/download"
	return &NovelDetail{
		Name:       "斗破苍穹",
		Author:     "天蚕土豆",
		CoverURL:   base + "?filename=%E6%96%97%E7%A0%B4%E8%8B%8D%E7%A9%B9_cover.jpg&size=50K",
		ProfileURL: base + "?filename=%E6%96%97%E7%A0%B4%E8%8B%8D%E7%A9%B9_profile.html&size=10K",
		Chapters: []NovelChapter{
			{Title: "第一章 陨落的天才", URL: base + "?filename=%E7%AC%AC%E4%B8%80%E7%AB%A0_%E9%99%A8%E8%90%BD%E7%9A%84%E5%A4%A9%E6%89%8D.html&size=15K", Index: 1},
			{Title: "第二章 斗气大陆", URL: base + "?filename=%E7%AC%AC%E4%BA%8C%E7%AB%A0_%E6%96%97%E6%B0%94%E5%A4%A7%E9%99%86.html&size=15K", Index: 2},
			{Title: "第三章 客人", URL: base + "?filename=%E7%AC%AC%E4%B8%89%E7%AB%A0_%E5%AE%A2%E4%BA%BA.html&size=15K", Index: 3},
			{Title: "第四章 云岚宗", URL: base + "?filename=%E7%AC%AC%E5%9B%9B%E7%AB%A0_%E4%BA%91%E5%B2%9A%E5%AE%97.html&size=15K", Index: 4},
			{Title: "第五章 聚气散", URL: base + "?filename=%E7%AC%AC%E4%BA%94%E7%AB%A0_%E8%81%9A%E6%B0%94%E6%95%A3.html&size=15K", Index: 5},
		},
	}
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
			Protocol: "http",
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
				Protocol: "http",
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
				Protocol: "http",
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
