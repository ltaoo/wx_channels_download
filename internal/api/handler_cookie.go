package api

import (
	"path/filepath"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/util"
	"wx_channel/pkg/cookies"
)

// handle_cookie_extract imports Chrome cookies into workdir/cookies.json and
// returns them to the caller. Browser and operating-system details belong to
// the cookies package.
func (c *APIClient) handle_cookie_extract(ctx *gin.Context) {
	cookiePath := filepath.Join(c.cfg.WorkDir, "cookies.json")
	imported, err := cookies.ImportChrome(cookies.ChromeImportOptions{
		Domain:     ctx.Query("domain"),
		OutputPath: cookiePath,
	})
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to import Chrome cookies")
		result.Err(ctx, 500, "提取或保存 Chrome Cookie 失败: "+err.Error())
		return
	}

	c.logger.Info().
		Int("loaded", imported.Loaded).
		Int("skipped", imported.Skipped).
		Str("path", cookiePath).
		Msg("Chrome cookies imported and saved")

	result.Ok(ctx, gin.H{
		"count":   len(imported.Cookies),
		"skipped": imported.Skipped,
		"path":    cookiePath,
		"cookies": imported.Cookies,
	})
}
