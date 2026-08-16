package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
)

type scraper_fetch_create_body struct {
	ID           string `json:"id"`
	RequestID    string `json:"request_id"`
	URL          string `json:"url"`
	ForceRefresh bool   `json:"force_refresh"`
}

type scraper_fetch_interrupt_body struct {
	ID        string `json:"id"`
	RequestID string `json:"request_id"`
}

func (c *APIClient) handle_scraper_fetch(ctx *gin.Context) {
	request, err := scraper_fetch_request_from_http(ctx)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	if request.URL == "" {
		result.Err(ctx, api_code_missing_url, "缺少参数：url")
		return
	}

	job, err := c.scraper_job_service.Create(services.ScraperFetchRequest{
		ID:           request.ID,
		URL:          request.URL,
		ForceRefresh: request.ForceRefresh,
	})
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, job)
}

func scraper_fetch_request_from_http(ctx *gin.Context) (scraper_fetch_create_body, error) {
	var request scraper_fetch_create_body
	if ctx.Request.Method == "POST" {
		if err := ctx.ShouldBindJSON(&request); err != nil {
			return request, fmt.Errorf("请求参数无效")
		}
		request.ID = strings.TrimSpace(request.ID)
		if request.ID == "" {
			request.ID = strings.TrimSpace(request.RequestID)
		}
		request.URL = strings.TrimSpace(request.URL)
		return request, nil
	}

	request.ID = strings.TrimSpace(ctx.Query("id"))
	if request.ID == "" {
		request.ID = strings.TrimSpace(ctx.Query("request_id"))
	}
	request.URL = strings.TrimSpace(ctx.Query("url"))
	force_refresh_value := strings.TrimSpace(ctx.Query("force_refresh"))
	if force_refresh_value == "" {
		return request, nil
	}
	force_refresh, err := strconv.ParseBool(force_refresh_value)
	if err != nil {
		return request, fmt.Errorf("参数 force_refresh 必须是布尔值")
	}
	request.ForceRefresh = force_refresh
	return request, nil
}

func (c *APIClient) handle_scraper_job(ctx *gin.Context) {
	job_id := strings.TrimSpace(ctx.Query("id"))
	if job_id == "" {
		result.Err(ctx, api_code_invalid_params, "缺少参数：id")
		return
	}
	job := c.scraper_job_service.Get(job_id, true)
	if job == nil {
		result.Err(ctx, api_code_invalid_params, "fetch job 不存在")
		return
	}
	result.Ok(ctx, job)
}

func (c *APIClient) handle_scraper_fetch_interrupt(ctx *gin.Context) {
	var body scraper_fetch_interrupt_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	job_id := strings.TrimSpace(body.ID)
	if job_id == "" {
		job_id = strings.TrimSpace(body.RequestID)
	}
	if job_id == "" {
		result.Err(ctx, api_code_invalid_params, "缺少参数：id")
		return
	}
	interrupted := c.scraper_job_service.Interrupt(job_id)
	result.Ok(ctx, gin.H{
		"id":          job_id,
		"interrupted": interrupted,
	})
}
