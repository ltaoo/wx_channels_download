package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/database/model"
	"wx_channel/internal/adapter"
	result "wx_channel/internal/util"
	"wx_channel/pkg/util"
)

func (c *APIClient) CreateBrowseHistory(browse *model.BrowseHistory) error {
	if c.browse_service == nil {
		if c.logger != nil {
			c.logger.Warn().Str("api", "CreateBrowseHistory").Msg("database not initialized")
		}
		return ErrDBNotInitialized
	}
	if browse == nil {
		if c.logger != nil {
			c.logger.Warn().Str("api", "CreateBrowseHistory").Msg("browse history payload is nil")
		}
		return ErrInvalidInput
	}
	return c.RecordBrowseHistory(browse.ExternalId, adapter.BrowseHistoryInfo{
		PlatformId:        browse.PlatformId,
		AccountExternalId: browse.AccountExternalId,
		AccountNickname:   browse.AccountNickname,
		AccountAvatarURL:  browse.AccountAvatarURL,
		ContentType:       browse.Type,
		ContentTitle:      browse.Title,
		ContentURL:        browse.URL,
		ContentSourceURL:  browse.SourceURL,
		ContentCoverURL:   browse.CoverURL,
		ExtraDataJSON:     browse.ExtraData,
	})
}

func (c *APIClient) RecordBrowseHistory(uniqueMark string, info adapter.BrowseHistoryInfo) error {
	if c.browse_service == nil {
		if c.logger != nil {
			c.logger.Warn().Str("api", "RecordBrowseHistory").Msg("database not initialized")
		}
		return ErrDBNotInitialized
	}
	if err := c.browse_service.Record(uniqueMark, info); err != nil {
		if c.logger != nil {
			c.logger.Error().Str("api", "RecordBrowseHistory").Err(err).
				Str("platform_id", info.PlatformId).
				Str("content_external_id", uniqueMark).
				Msg("failed to record browse history")
		}
		return err
	}
	return nil
}

type CreateBrowseHistoryBody struct {
	Platform string          `json:"platform"`
	Content  json.RawMessage `json:"content"`
}

type browseHistoryCreateRequest struct {
	Objects []CreateBrowseHistoryBody `json:"objects"`
}

var errBrowseHistoryMissingID = errors.New("content id is missing")

const defaultBrowseHistoryPlatform = "wxchannels"

func normalizeBrowseHistoryPlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func parseBrowseHistoryCreateItem(item CreateBrowseHistoryBody) (string, json.RawMessage, error) {
	platform := normalizeBrowseHistoryPlatform(item.Platform)
	if platform == "" {
		platform = defaultBrowseHistoryPlatform
	}
	if len(item.Content) == 0 {
		return "", nil, errors.New("browse history item missing content")
	}
	return platform, item.Content, nil
}

func buildBrowseHistoryFromPayload(platform string, content json.RawMessage) (*model.BrowseHistory, error) {
	plat := normalizeBrowseHistoryPlatform(platform)
	if plat == "" {
		plat = defaultBrowseHistoryPlatform
	}
	handler := adapter.Get(plat)
	if handler == nil {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	info, err := handler.BuildDownloadTask(content, json.RawMessage("{}"))
	if err != nil {
		return nil, fmt.Errorf("failed to build browse history from payload: %w", err)
	}
	if info == nil || info.Content == nil {
		return nil, errors.New("failed to build browse history record")
	}

	record := browseHistoryFromDownloadTaskResult(info)
	if record == nil {
		return nil, errors.New("failed to build browse history record")
	}

	if strings.TrimSpace(record.ExternalId) == "" {
		return nil, errBrowseHistoryMissingID
	}
	return record, nil
}

func browseHistoryFromDownloadTaskResult(result *adapter.DownloadTaskResult) *model.BrowseHistory {
	if result == nil || result.Content == nil {
		return nil
	}

	content := result.Content
	now := util.NowMillis()

	platformID := strings.TrimSpace(content.PlatformId)
	if platformID == "" && result.Task != nil {
		platformID = strings.TrimSpace(result.Task.PlatformId)
	}
	if platformID == "" {
		return nil
	}

	extra := ""
	if result.Task != nil {
		extra = strings.TrimSpace(result.Task.MetadataJSON)
	}
	if extra == "" {
		extra = strings.TrimSpace(content.Metadata)
	}

	accountExternalID := ""
	accountNickname := ""
	accountAvatarURL := ""
	if result.Account != nil {
		accountExternalID = strings.TrimSpace(result.Account.ExternalId)
		accountNickname = result.Account.Nickname
		accountAvatarURL = result.Account.AvatarURL
	}

	return &model.BrowseHistory{
		PlatformId:        platformID,
		VisitedTimes:      1,
		AccountExternalId: accountExternalID,
		AccountNickname:   accountNickname,
		AccountAvatarURL:  accountAvatarURL,
		Type:              content.Type,
		ExternalId:        strings.TrimSpace(content.ExternalId),
		Title:             content.Title,
		URL:               content.URL,
		SourceURL:         content.SourceURL,
		CoverURL:          content.CoverURL,
		CoverWidth:        content.CoverWidth,
		CoverHeight:       content.CoverHeight,
		PublishTime:       content.PublishTime,
		ExtraData:         extra,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

func (c *APIClient) handleCreateBrowseHistory(ctx *gin.Context) {
	var body browseHistoryCreateRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		if c.logger != nil {
			c.logger.Warn().
				Str("api", "POST /api/browse_history/create").
				Err(err).
				Msg("failed to parse single browse history request body")
		}
		result.Err(ctx, 400, "invalid request payload")
		return
	}
	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/create").
			Int("request_object_count", len(body.Objects)).
			Msg("received browse history create request")
	}

	if len(body.Objects) == 0 {
		if c.logger != nil {
			c.logger.Warn().
				Str("api", "POST /api/browse_history/create").
				Msg("single browse history create rejected: objects empty")
		}
		result.Err(ctx, 400, "browse history objects cannot be empty")
		return
	}

	results := make([]gin.H, 0, len(body.Objects))
	createdCount := 0
	failedCount := 0
	skippedCount := 0
	for idx, raw := range body.Objects {
		platform, content, err := parseBrowseHistoryCreateItem(raw)
		if err != nil {
			if c.logger != nil {
				c.logger.Warn().
					Str("api", "POST /api/browse_history/create").
					Int("index", idx).
					Err(err).
					Msg("failed to parse browse history item")
			}
			results = append(results, gin.H{
				"index":   idx,
				"success": false,
				"error":   err.Error(),
			})
			failedCount++
			continue
		}

		record, err := buildBrowseHistoryFromPayload(platform, content)
		if err != nil {
			if errors.Is(err, errBrowseHistoryMissingID) {
				skippedCount++
				if c.logger != nil {
					c.logger.Warn().
						Str("api", "POST /api/browse_history/create").
						Int("index", idx).
						Str("platform_id", platform).
						Msg("skip browse history item: content id missing")
				}
				results = append(results, gin.H{
					"index":    idx,
					"success":  false,
					"skipped":  true,
					"error":    errBrowseHistoryMissingID.Error(),
					"platform": platform,
				})
				continue
			}
			if c.logger != nil {
				c.logger.Error().
					Str("api", "POST /api/browse_history/create").
					Str("platform_id", platform).
					Err(err).
					Msg("failed to build browse history record from payload")
			}
			results = append(results, gin.H{
				"index":   idx,
				"success": false,
				"error":   err.Error(),
			})
			failedCount++
			continue
		}

		if c.logger != nil {
			c.logger.Info().
				Str("api", "POST /api/browse_history/create").
				Int("index", idx).
				Str("platform_id", record.PlatformId).
				Str("content_id", record.ExternalId).
				Str("account_external_id", record.AccountExternalId).
				Str("content_title", record.Title).
				Str("content_url", record.URL).
				Str("content_source_url", record.SourceURL).
				Str("content_cover_url", record.CoverURL).
				Msg("persisting browse history item")
		}

		if err := c.CreateBrowseHistory(record); err != nil {
			if c.logger != nil {
				c.logger.Error().
					Str("api", "POST /api/browse_history/create").
					Int("index", idx).
					Str("platform_id", record.PlatformId).
					Str("content_id", record.ExternalId).
					Err(err).
					Msg("failed to create browse history item")
			}
			results = append(results, gin.H{
				"index":      idx,
				"success":    false,
				"error":      err.Error(),
				"platform":   record.PlatformId,
				"content_id": record.ExternalId,
			})
			failedCount++
			continue
		}
		createdCount++
		results = append(results, gin.H{
			"index":      idx,
			"success":    true,
			"platform":   record.PlatformId,
			"content_id": record.ExternalId,
			"account":    record.AccountExternalId,
		})
	}
	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/create").
			Int("created_count", createdCount).
			Int("failed_count", failedCount).
			Int("skipped_count", skippedCount).
			Int("object_count", len(body.Objects)).
			Msg("browse history create completed")
	}
	result.Ok(ctx, gin.H{
		"results":       results,
		"created_count": createdCount,
		"failed_count":  failedCount,
		"skipped_count": skippedCount,
	})
}

func (c *APIClient) handleCreateBrowseHistories(ctx *gin.Context) {
	var body browseHistoryCreateRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		if c.logger != nil {
			c.logger.Warn().
				Str("api", "POST /api/browse_history/create").
				Err(err).
				Msg("failed to parse browse history batch create request body")
		}
		result.Err(ctx, 400, "invalid request payload")
		return
	}
	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/create").
			Int("request_object_count", len(body.Objects)).
			Msg("received browse history batch create request")
	}
	if len(body.Objects) == 0 {
		if c.logger != nil {
			c.logger.Warn().
				Str("api", "POST /api/browse_history/create").
				Msg("browse history batch create rejected: objects empty")
		}
		result.Err(ctx, 400, "browse history objects cannot be empty")
		return
	}

	createdCount := 0
	failedCount := 0
	skippedCount := 0
	results := make([]gin.H, 0, len(body.Objects))
	for idx, raw := range body.Objects {
		platform, content, err := parseBrowseHistoryCreateItem(raw)
		if err != nil {
			if c.logger != nil {
				c.logger.Warn().
					Str("api", "POST /api/browse_history/create").
					Int("index", idx).
					Err(err).
					Msg("failed to parse browse history item")
			}
			results = append(results, gin.H{
				"index":   idx,
				"success": false,
				"error":   err.Error(),
			})
			failedCount++
			continue
		}

		record, err := buildBrowseHistoryFromPayload(platform, content)
		if err != nil {
			if errors.Is(err, errBrowseHistoryMissingID) {
				skippedCount++
				c.logger.Warn().
					Str("api", "POST /api/browse_history/create").
					Int("index", idx).
					Str("platform_id", platform).
					Msg("skip browse history item: content id missing")
				results = append(results, gin.H{
					"index":    idx,
					"success":  false,
					"skipped":  true,
					"error":    errBrowseHistoryMissingID.Error(),
					"platform": platform,
				})
				continue
			}
			c.logger.Error().
				Str("api", "POST /api/browse_history/create").
				Int("index", idx).
				Str("platform_id", platform).
				Err(err).
				Msg("failed to build browse history record from payload")
			results = append(results, gin.H{
				"index":   idx,
				"success": false,
				"error":   err.Error(),
			})
			failedCount++
			continue
		}

		if c.logger != nil {
			c.logger.Info().
				Str("api", "POST /api/browse_history/create").
				Int("index", idx).
				Str("platform_id", record.PlatformId).
				Str("content_id", record.ExternalId).
				Str("account_external_id", record.AccountExternalId).
				Str("content_title", record.Title).
				Str("content_url", record.URL).
				Str("content_source_url", record.SourceURL).
				Msg("persisting browse history item")
		}

		if err := c.CreateBrowseHistory(record); err != nil {
			if c.logger != nil {
				c.logger.Error().
					Str("api", "POST /api/browse_history/create").
					Int("index", idx).
					Str("platform_id", platform).
					Str("content_id", record.ExternalId).
					Err(err).
					Msg("failed to create browse history item in batch")
			}
			results = append(results, gin.H{
				"index":      idx,
				"success":    false,
				"error":      err.Error(),
				"platform":   record.PlatformId,
				"content_id": record.ExternalId,
			})
			failedCount++
			continue
		}
		createdCount++
		results = append(results, gin.H{
			"index":      idx,
			"success":    true,
			"platform":   record.PlatformId,
			"content_id": record.ExternalId,
			"account":    record.AccountExternalId,
		})
	}
	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/create").
			Int("created_count", createdCount).
			Int("failed_count", failedCount).
			Int("skipped_count", skippedCount).
			Int("object_count", len(body.Objects)).
			Msg("browse history batch create completed")
	}
	result.Ok(ctx, gin.H{
		"results":       results,
		"created_count": createdCount,
		"failed_count":  failedCount,
		"skipped_count": skippedCount,
	})
}

func (c *APIClient) handleFetchBrowseHistoryList(ctx *gin.Context) {
	var body struct {
		Username       *string  `json:"username"`
		PlatformId     string   `json:"platform_id"`
		PlatformIds    []string `json:"platform_ids"`
		Page           *int     `json:"page"`
		PageSize       *int     `json:"page_size"`
		PageSizeLegacy *int     `json:"pageSize"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		if c.logger != nil {
			c.logger.Warn().
				Str("api", "POST /api/browse_history/list").
				Err(err).
				Msg("Failed to parse browse history list request body")
		}
		result.Err(ctx, 400, err.Error())
		return
	}

	platformIds := body.PlatformIds
	if body.PlatformId != "" {
		platformIds = []string{body.PlatformId}
	}
	if len(platformIds) == 0 {
		platformIds = []string{"wxchannels", "wxmp", "zhihu", "xiaohongshu", "bilibili", "youtube", "weibo"}
	}
	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/list").
			Strs("platform_ids", platformIds).
			Str("username", func() string {
				if body.Username == nil {
					return ""
				}
				return *body.Username
			}()).
			Int("request_page", func() int {
				if body.Page != nil {
					return *body.Page
				}
				return 0
			}()).
			Int("request_page_size", func() int {
				if body.PageSize != nil {
					return *body.PageSize
				}
				if body.PageSizeLegacy != nil {
					return *body.PageSizeLegacy
				}
				return 0
			}()).
			Msg("Received browse history list request")
	}

	page := 1
	pageSize := 20
	if body.Page != nil && *body.Page > 0 {
		page = *body.Page
	}
	if body.PageSize != nil && *body.PageSize > 0 {
		pageSize = *body.PageSize
	} else if body.PageSizeLegacy != nil && *body.PageSizeLegacy > 0 {
		pageSize = *body.PageSizeLegacy
	}

	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/list").
			Strs("platform_ids", platformIds).
			Str("username", func() string {
				if body.Username == nil {
					return ""
				}
				return *body.Username
			}()).
			Int("page", page).
			Int("page_size", pageSize).
			Msg("Querying browse history list")
	}
	if c.browse_service == nil {
		if c.logger != nil {
			c.logger.Warn().
				Str("api", "POST /api/browse_history/list").
				Msg("browse service not initialized")
		}
		result.Err(ctx, 500, "browse service not initialized")
		return
	}
	browseHistories, err := c.browse_service.ListPlatforms(platformIds, body.Username, page, pageSize)
	if err != nil {
		if c.logger != nil {
			c.logger.Error().
				Str("api", "POST /api/browse_history/list").
				Strs("platform_ids", platformIds).
				Str("username", func() string {
					if body.Username == nil {
						return ""
					}
					return *body.Username
				}()).
				Int("page", page).
				Int("page_size", pageSize).
				Err(err).
				Msg("Failed to query browse history list")
		}
		result.Err(ctx, 500, err.Error())
		return
	}
	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/list").
			Strs("platform_ids", platformIds).
			Str("username", func() string {
				if body.Username == nil {
					return ""
				}
				return *body.Username
			}()).
			Int("page", browseHistories.Page).
			Int("page_size", browseHistories.PageSize).
			Int64("total", browseHistories.Total).
			Int("returned", len(browseHistories.List)).
			Msg("Browse history list returned")
	}
	result.Ok(ctx, browseHistories)
}
