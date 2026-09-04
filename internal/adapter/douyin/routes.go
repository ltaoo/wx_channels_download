package douyinadapter

import (
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
	"wx_channel/pkg/scraper/douyin"
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
}

func (r *routes) handle_fetch_home(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Query("id"))
	if id == "" {
		result.Err(ctx, 400, "id parameter is required")
		return
	}
	client := douyin.NewClientWithLoggerAndCookieReader(
		r.handler.config_string("douyin.cookie"),
		r.handler.cookie_reader(),
		r.handler.get_logger(),
	)
	home, err := client.FetchHome(id)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, home)
}
