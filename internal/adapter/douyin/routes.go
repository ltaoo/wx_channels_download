package douyinadapter

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
)

const douyin_home_path = "/api/douyin/contact/home"

type routes struct {
	handler *handler
}

func new_routes(handler *handler) *routes {
	return &routes{handler: handler}
}

func (r *routes) register_routes(registrar adapter.RouteRegistrar) {
	if r == nil || r.handler == nil || registrar == nil {
		return
	}
	registrar.RegisterGET(douyin_home_path, r.handle_fetch_home)
	registrar.RegisterGET("/api/douyin/contact/feed/list", r.handle_fetch_list)
}

func (r *routes) handle_fetch_list(ctx *gin.Context) {
	sec_uid := strings.TrimSpace(ctx.Query("sec_uid"))
	if sec_uid == "" || strings.ContainsAny(sec_uid, "/?#") {
		result.Err(ctx, 400, "sec_uid 必须是抖音主页 /user/ 后的账号标识")
		return
	}
	page_url := "https://www.douyin.com/user/" + url.PathEscape(sec_uid)
	list, err := r.handler.douyin_home_client().FetchHomeContentsPage(page_url, ctx.Query("next_marker"))
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, list)
}

func (r *routes) handle_fetch_home(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Query("id"))
	if id == "" {
		result.Err(ctx, 400, "id parameter is required")
		return
	}
	client := r.handler.douyin_home_client()
	home, err := client.FetchHome(id)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, home)
}
