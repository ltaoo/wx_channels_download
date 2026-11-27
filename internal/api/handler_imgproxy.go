package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"wx_channel/pkg/clawreq"
	"wx_channel/pkg/util"
)

func (c *APIClient) handle_img_proxy(ctx *gin.Context) {
	raw_url := ctx.Query("url")
	if raw_url == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "url is required"})
		return
	}

	client := c.get_clawreq_client()
	if client == nil {
		ctx.Status(http.StatusBadGateway)
		return
	}
	resp, err := client.Get(ctx.Request.Context(), raw_url,
		clawreq.WithHeader("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8"),
	)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"code": -1, "msg": err.Error()})
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ctx.JSON(http.StatusBadGateway, gin.H{"code": -1, "msg": fmt.Sprintf("upstream returned %d", resp.StatusCode)})
		return
	}

	content_type := util.SafeImageContentType(resp.Body)
	if content_type == "" {
		ctx.JSON(http.StatusBadGateway, gin.H{"code": -1, "msg": "upstream did not return a safe image"})
		return
	}

	ctx.Header("Content-Type", content_type)
	ctx.Header("Cache-Control", "public, max-age=31536000, immutable")
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Content-Security-Policy", "sandbox; default-src 'none'")
	ctx.Data(resp.StatusCode, content_type, resp.Body)
}

func (c *APIClient) get_clawreq_client() *clawreq.Client {
	c.claw_init_once.Do(func() {
		client, err := clawreq.New(clawreq.Config{
			Profile:         clawreq.ProfileChrome,
			FollowRedirects: true,
		})
		if err != nil {
			c.logger.Error().Err(err).Msg("imgproxy: failed to init clawreq client")
			return
		}
		c.claw_client = client
	})
	return c.claw_client
}
