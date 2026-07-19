package api

import (
	"github.com/gin-gonic/gin"

	// "wx_channel/pkg/scraper"
)

func platformView(platformID string) gin.H {
	return gin.H{
		// "id":          scraper.DefaultAuthor(platformID),
		// "code":        scraper.DefaultAuthor(platformID),
		// "name":        scraper.DisplayName(platformID),
		// "homepage":    scraper.HomepageURL(platformID),
		// "favicon_url": scraper.FaviconDataURL(platformID),
	}
}
