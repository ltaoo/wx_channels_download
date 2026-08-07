package api

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"wx_channel/pkg/clawreq"
)

func (c *APIClient) handleImgProxy(ctx *gin.Context) {
	rawURL := ctx.Query("url")
	if rawURL == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "url is required"})
		return
	}

	resp, err := c.getClawreqClient().Get(ctx.Request.Context(), rawURL,
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

	contentType := resp.ContentType()
	if contentType == "" {
		contentType = "image/jpeg"
	}

	ctx.Header("Content-Type", contentType)
	ctx.Header("Cache-Control", "public, max-age=31536000, immutable")
	ctx.Data(resp.StatusCode, contentType, resp.Body)
}

var clawreqInitOnce sync.Once

func (c *APIClient) getClawreqClient() *clawreq.Client {
	clawreqInitOnce.Do(func() {
		client, err := clawreq.New(clawreq.Config{
			Profile:         clawreq.ProfileChrome,
			FollowRedirects: true,
		})
		if err != nil {
			c.logger.Error().Err(err).Msg("imgproxy: failed to init clawreq client")
			return
		}
		c.clawclient = client
	})
	return c.clawclient
}
