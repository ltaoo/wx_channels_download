package api

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

func (c *APIClient) CreateBrowseHistory(browse_history *model.BrowseHistory, account *model.Account) error {
	if browse_history == nil {
		return ErrInvalidInput
	}

	account_external_id := ""
	if account != nil {
		account_external_id = account.ExternalId
	}
	return c.RecordBrowseHistory(browse_history.ExternalId, adapter.BrowseHistoryInfo{
		PlatformId:        browse_history.PlatformId,
		AccountExternalId: account_external_id,
		Account:           account,
		ContentType:       browse_history.Type,
		ContentTitle:      browse_history.Title,
		ContentURL:        browse_history.URL,
		ContentSourceURL:  browse_history.SourceURL,
		ContentCoverURL:   browse_history.CoverURL,
		ExtraDataJSON:     browse_history.ExtraData,
	})
}

func (c *APIClient) RecordBrowseHistory(unique_mark string, info adapter.BrowseHistoryInfo) error {
	if c.browse_history_service == nil {
		return ErrDBNotInitialized
	}
	return c.browse_history_service.Record(unique_mark, info)
}

type browseHistoryCreateRequest struct {
	Objects []services.CreateBrowseHistoryBody `json:"objects"`
}

// createBrowseHistorySingle creates one browse history item through the
// service layer. Platform payload parsing and persistence stay out of the API.
func (c *APIClient) createBrowseHistorySingle(body services.CreateBrowseHistoryBody) (gin.H, error) {
	if c.browse_history_service == nil {
		return nil, fmt.Errorf("browse history service not initialized")
	}

	created, err := c.browse_history_service.CreateBrowseHistory(body)
	if err != nil {
		return nil, err
	}

	return gin.H{
		"browse_history": created.BrowseHistory,
		"account":        created.Account,
	}, nil
}

// handle_create_browse_history batch-creates browse history records.
func (c *APIClient) handle_create_browse_history(ctx *gin.Context) {
	var req browseHistoryCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if c.logger != nil {
			c.logger.Warn().
				Str("api", "POST /api/browse_history/create").
				Err(err).
				Msg("failed to parse browse history create request body")
		}
		result.Err(ctx, 400, "invalid request payload")
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "browse history objects cannot be empty")
		return
	}

	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/create").
			Int("object_count", len(req.Objects)).
			Msg("received browse history create request")
	}

	results := make([]gin.H, 0, len(req.Objects))
	created_count := 0
	failed_count := 0
	skipped_count := 0
	for index, body := range req.Objects {
		data, err := c.createBrowseHistorySingle(body)
		if err != nil {
			item := gin.H{
				"index":   index,
				"success": false,
				"error":   err.Error(),
			}
			if errors.Is(err, services.ErrBrowseHistoryMissingID) {
				item["skipped"] = true
				skipped_count++
			} else {
				failed_count++
			}
			results = append(results, item)
			if c.logger != nil {
				c.logger.Warn().
					Str("api", "POST /api/browse_history/create").
					Int("index", index).
					Str("platform_id", body.Platform).
					Err(err).
					Msg("failed to create browse history item")
			}
			continue
		}

		created_count++
		results = append(results, gin.H{
			"index":   index,
			"success": true,
			"data":    data,
		})
	}

	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/create").
			Int("created_count", created_count).
			Int("failed_count", failed_count).
			Int("skipped_count", skipped_count).
			Int("object_count", len(req.Objects)).
			Msg("browse history create completed")
	}
	result.Ok(ctx, gin.H{
		"results":       results,
		"created_count": created_count,
		"failed_count":  failed_count,
		"skipped_count": skipped_count,
	})
}

func (c *APIClient) handle_fetch_browse_history_list(ctx *gin.Context) {
	var body struct {
		Username       *string  `json:"username"`
		Keyword        string   `json:"keyword"`
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

	platform_ids := body.PlatformIds
	if body.PlatformId != "" {
		platform_ids = []string{body.PlatformId}
	}
	if len(platform_ids) == 0 {
		platform_ids = []string{"wxchannels", "wxmp", "zhihu", "xiaohongshu", "bilibili", "youtube", "weibo"}
	}
	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/list").
			Strs("platform_ids", platform_ids).
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
	page_size := 20
	if body.Page != nil && *body.Page > 0 {
		page = *body.Page
	}
	if body.PageSize != nil && *body.PageSize > 0 {
		page_size = *body.PageSize
	} else if body.PageSizeLegacy != nil && *body.PageSizeLegacy > 0 {
		page_size = *body.PageSizeLegacy
	}

	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/list").
			Strs("platform_ids", platform_ids).
			Str("username", func() string {
				if body.Username == nil {
					return ""
				}
				return *body.Username
			}()).
			Int("page", page).
			Int("page_size", page_size).
			Msg("Querying browse history list")
	}
	if c.browse_history_service == nil {
		if c.logger != nil {
			c.logger.Warn().
				Str("api", "POST /api/browse_history/list").
				Msg("browse history service not initialized")
		}
		result.Err(ctx, 500, "browse history service not initialized")
		return
	}
	browse_histories, err := c.browse_history_service.ListPlatforms(
		platform_ids,
		body.Username,
		page,
		page_size,
		body.Keyword,
	)
	if err != nil {
		if c.logger != nil {
			c.logger.Error().
				Str("api", "POST /api/browse_history/list").
				Strs("platform_ids", platform_ids).
				Str("username", func() string {
					if body.Username == nil {
						return ""
					}
					return *body.Username
				}()).
				Int("page", page).
				Int("page_size", page_size).
				Err(err).
				Msg("Failed to query browse history list")
		}
		result.Err(ctx, 500, err.Error())
		return
	}
	if c.logger != nil {
		c.logger.Info().
			Str("api", "POST /api/browse_history/list").
			Strs("platform_ids", platform_ids).
			Str("username", func() string {
				if body.Username == nil {
					return ""
				}
				return *body.Username
			}()).
			Int("page", browse_histories.Page).
			Int("page_size", browse_histories.PageSize).
			Int64("total", browse_histories.Total).
			Int("returned", len(browse_histories.List)).
			Msg("Browse history list returned")
	}
	result.Ok(ctx, browse_histories)
}
