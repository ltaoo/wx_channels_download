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
	page_result, err := c.service_account.ListInfluencers(page, size)
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
	influencer, err := c.service_account.GetInfluencer(id)
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

type influencer_create_body struct {
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
	var body influencer_create_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.Name == "" {
		result.Err(ctx, 400, "name is required")
		return
	}

	// Use service
	influencer, err := c.service_account.CreateInfluencer(&services.CreateInfluencerInput{
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

type influencer_update_body struct {
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
	var body influencer_update_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	// Use service
	influencer, err := c.service_account.UpdateInfluencer(id, &services.UpdateInfluencerInput{
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
	page_result, err := c.service_account.ListAccounts(ctx.Request.Context(), services.AccountListInput{
		Page:       page,
		PageSize:   page_size,
		Keyword:    ctx.Query("keyword"),
		AccountID:  ctx.Query("account_id"),
		PlatformID: ctx.Query("platform_id"),
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

const account_content_page_size = 50

// handle_account_details_content_list lists content records associated with an
// account. It reads the persisted content_account associations instead of
// calling the platform adapter (and therefore the platform client), so the
// drawer keeps working without a live platform session.
func (c *APIClient) handle_account_details_content_list(ctx *gin.Context) {
	if c.db == nil || c.service_content == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	account_id := strings.TrimSpace(ctx.Query("id"))
	if account_id == "" {
		result.Err(ctx, 400, "id 不能为空")
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

	scope := strings.TrimSpace(ctx.Param("scope"))
	if scope == "" {
		scope = "feed"
	}
	page := 1
	if value, err := strconv.Atoi(strings.TrimSpace(ctx.Query("page"))); err == nil && value > 0 {
		page = value
	}
	offset := (page - 1) * account_content_page_size

	page_result, err := c.service_content.ListContents(services.ContentListOptions{
		AccountID:          account.Id,
		Scope:              services.ContentListScopeAll,
		Page:               page,
		PageSize:           account_content_page_size,
		Offset:             &offset,
		OrderByPublishTime: true,
	})
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	next_marker := ""
	if int64(page*account_content_page_size) < page_result.Total {
		next_marker = strconv.Itoa(page + 1)
	}

	result.Ok(ctx, adapter.HomeDetails{
		Scopes:     []adapter.HomeDetailsScope{{Label: scope, Value: scope}},
		Scope:      scope,
		Contents:   page_result.List,
		NextMarker: next_marker,
	})
}

func (c *APIClient) handle_account_synchronize(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		AccountID string `json:"account_id"`
		Scope     string `json:"scope"`
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
	tabs := home_builder.HomeContentTabs(&account)
	if len(tabs) == 0 {
		result.Err(ctx, 400, "平台 "+account.PlatformId+" 没有可用的主页内容 tab")
		return
	}
	scope := strings.TrimSpace(body.Scope)
	if scope == "" {
		result.Ok(ctx, gin.H{
			"tabs":  tabs,
			"list":  []model.Content{},
			"total": 0,
		})
		return
	}
	valid_scope := false
	for _, tab := range tabs {
		if tab.Scope == scope {
			valid_scope = true
			break
		}
	}
	if !valid_scope {
		result.Err(ctx, 400, "平台 "+account.PlatformId+" 不支持主页 scope: "+scope)
		return
	}
	home_contents, err := home_builder.BuildHomeContents(&account, scope)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if home_contents == nil {
		result.Err(ctx, 500, "平台 "+account.PlatformId+" 返回了空的主页内容结果")
		return
	}
	result.Ok(ctx, gin.H{
		"tabs":  tabs,
		"scope": home_contents.Scope,
		"list":  home_contents.Contents,
		"total": len(home_contents.Contents),
	})
}
