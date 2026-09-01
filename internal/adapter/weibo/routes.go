package weiboadapter

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/scraper/weibo"
)

const image_proxy_path = "/api/weibo/imgproxy"

type routes struct {
	client *weibo.Client
}

func new_routes(client *weibo.Client) *routes {
	return &routes{client: client}
}

func (r *routes) register_routes(registrar adapter.RouteRegistrar) {
	if r == nil || registrar == nil {
		return
	}
	registrar.RegisterGET(image_proxy_path, r.proxy_img)
}

func (r *routes) proxy_img(ctx *gin.Context) {
	raw_url := strings.TrimSpace(ctx.Query("url"))
	if raw_url == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "url is required"})
		return
	}
	response, err := r.client.ProxyImgContext(ctx.Request.Context(), raw_url)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"code": -1, "msg": err.Error()})
		return
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		ctx.JSON(http.StatusBadGateway, gin.H{"code": -1, "msg": fmt.Sprintf("upstream returned %d", response.StatusCode)})
		return
	}
	content_type := response.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(content_type), "image/") {
		ctx.JSON(http.StatusBadGateway, gin.H{"code": -1, "msg": "upstream did not return an image"})
		return
	}
	for _, header_name := range []string{"Cache-Control", "ETag", "Last-Modified"} {
		if header_value := response.Header.Get(header_name); header_value != "" {
			ctx.Header(header_name, header_value)
		}
	}
	if response.Header.Get("Cache-Control") == "" {
		ctx.Header("Cache-Control", "public, max-age=86400")
	}
	ctx.Header("Content-Type", content_type)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Status(response.StatusCode)
	_, _ = io.Copy(ctx.Writer, response.Body)
}
