package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

func (c *APIClient) handle_influencer_list(ctx *gin.Context) {
	page_str := ctx.Query("page")
	size_str := ctx.Query("page_size")
	page := 1
	size := 20
	if page_str != "" {
		if value, err := strconv.Atoi(page_str); err == nil && value > 0 {
			page = value
		}
	}
	if size_str != "" {
		if value, err := strconv.Atoi(size_str); err == nil && value > 0 {
			size = value
		}
	}

	// Use service
	page_result, err := c.account_service.ListInfluencers(page, size)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, page_result)
}

func (c *APIClient) handle_influencer_get(ctx *gin.Context) {
	id_str := ctx.Param("id")
	id, err := strconv.Atoi(id_str)
	if err != nil || id <= 0 {
		result.Err(ctx, 400, "invalid id")
		return
	}

	// Use service
	influencer, err := c.account_service.GetInfluencer(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, err.Error())
			return
		}
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, influencer)
}

type influencerCreateBody struct {
	Name               string  `json:"name"`
	Alias              string  `json:"alias"`
	AvatarURL          string  `json:"avatar_url"`
	Sex                int     `json:"sex"`
	Description        string  `json:"description"`
	Biography          string  `json:"biography"`
	ProfilePath        string  `json:"profile_path"`
	Birthday           string  `json:"birthday"`
	PlaceOfBirth       string  `json:"place_of_birth"`
	KnownForDepartment string  `json:"known_for_department"`
	Profile            string  `json:"profile"`
	TMDBId             *string `json:"tmdb_id"`
	DoubanId           *string `json:"douban_id"`
	IMDBId             *string `json:"imdb_id"`
	MetadataJSON       string  `json:"metadata_json"`
}

func (c *APIClient) handle_influencer_create(ctx *gin.Context) {
	var body influencerCreateBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.Name == "" {
		result.Err(ctx, 400, "name is required")
		return
	}

	// Use service
	influencer, err := c.account_service.CreateInfluencer(&services.CreateInfluencerInput{
		Name:               body.Name,
		Alias:              body.Alias,
		AvatarURL:          body.AvatarURL,
		Sex:                body.Sex,
		Description:        body.Description,
		Biography:          body.Biography,
		ProfilePath:        body.ProfilePath,
		Birthday:           body.Birthday,
		PlaceOfBirth:       body.PlaceOfBirth,
		KnownForDepartment: body.KnownForDepartment,
		Profile:            body.Profile,
		TMDBId:             body.TMDBId,
		DoubanId:           body.DoubanId,
		IMDBId:             body.IMDBId,
		MetadataJSON:       body.MetadataJSON,
	})
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, influencer)
}

type influencerUpdateBody struct {
	Name               string  `json:"name"`
	Alias              string  `json:"alias"`
	AvatarURL          string  `json:"avatar_url"`
	Sex                *int    `json:"sex"`
	Description        string  `json:"description"`
	Biography          string  `json:"biography"`
	ProfilePath        string  `json:"profile_path"`
	Birthday           string  `json:"birthday"`
	PlaceOfBirth       string  `json:"place_of_birth"`
	KnownForDepartment string  `json:"known_for_department"`
	Profile            string  `json:"profile"`
	TMDBId             *string `json:"tmdb_id"`
	DoubanId           *string `json:"douban_id"`
	IMDBId             *string `json:"imdb_id"`
	MetadataJSON       string  `json:"metadata_json"`
}

func (c *APIClient) handle_influencer_update(ctx *gin.Context) {
	id_str := ctx.Param("id")
	id, err := strconv.Atoi(id_str)
	if err != nil || id <= 0 {
		result.Err(ctx, 400, "invalid id")
		return
	}
	var body influencerUpdateBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	// Use service
	influencer, err := c.account_service.UpdateInfluencer(id, &services.UpdateInfluencerInput{
		Name:               body.Name,
		Alias:              body.Alias,
		AvatarURL:          body.AvatarURL,
		Sex:                body.Sex,
		Description:        body.Description,
		Biography:          body.Biography,
		ProfilePath:        body.ProfilePath,
		Birthday:           body.Birthday,
		PlaceOfBirth:       body.PlaceOfBirth,
		KnownForDepartment: body.KnownForDepartment,
		Profile:            body.Profile,
		TMDBId:             body.TMDBId,
		DoubanId:           body.DoubanId,
		IMDBId:             body.IMDBId,
		MetadataJSON:       body.MetadataJSON,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, err.Error())
			return
		}
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, influencer)
}

func (c *APIClient) handle_account_list(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}

	page := 1
	page_size := 24
	if value, err := strconv.Atoi(ctx.Query("page")); err == nil && value > 0 {
		page = value
	}
	if value, err := strconv.Atoi(ctx.Query("page_size")); err == nil && value > 0 {
		page_size = value
	}
	if page_size > 200 {
		page_size = 200
	}
	page_result, err := c.account_service.ListAccounts(ctx.Request.Context(), services.AccountListInput{
		Page:      page,
		PageSize:  page_size,
		Keyword:   ctx.Query("keyword"),
		AccountID: ctx.Query("account_id"),
	})
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	list := make([]gin.H, 0, len(page_result.List))
	for _, item := range page_result.List {
		account := item.Account
		list = append(list, gin.H{
			"id":               account.Id,
			"platform_id":      account.PlatformId,
			"nickname":         account.Nickname,
			"avatar_url":       account.AvatarURL,
			"external_id":      account.ExternalId,
			"created_at":       account.CreatedAt,
			"updated_at":       account.UpdatedAt,
			"content_count":    item.ContentCount,
			"has_content":      item.ContentCount > 0,
			"content_accounts": item.ContentCount,
		})
	}
	result.Ok(ctx, gin.H{
		"list":      list,
		"total":     page_result.Total,
		"page":      page_result.Page,
		"page_size": page_result.PageSize,
	})
}

func (c *APIClient) handle_account_synchronize(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		AccountID string `json:"account_id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	account_id := strings.TrimSpace(body.AccountID)
	if account_id == "" {
		result.Err(ctx, 400, "account_id 不能为空")
		return
	}

	var account model.Account
	if err := c.db.WithContext(ctx.Request.Context()).First(&account, "id = ?", account_id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, "账号不存在")
			return
		}
		result.Err(ctx, 500, err.Error())
		return
	}
	handler := adapter.Get(account.PlatformId)
	if handler == nil {
		result.Err(ctx, 400, "不支持的平台: "+account.PlatformId)
		return
	}
	home_builder, ok := handler.(adapter.HomeContentsBuilder)
	if !ok {
		result.Err(ctx, 400, "平台 "+account.PlatformId+" 不支持主页同步")
		return
	}
	contents, err := home_builder.BuildHomeContents(&account)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"list": contents, "total": len(contents)})
}
