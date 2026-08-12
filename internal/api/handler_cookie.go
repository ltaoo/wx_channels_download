package api

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
	"wx_channel/pkg/cookies"
)

const cookie_update_max_body_size = int64(50 * 1024 * 1024)

// handle_cookie_extract imports Chrome cookies into workdir/cookies.json and
// returns them to the caller. Browser and operating-system details belong to
// the cookies package.
func (c *APIClient) handle_cookie_extract(ctx *gin.Context) {
	cookie_path := filepath.Join(c.cfg.WorkDir, "cookies.json")
	imported, err := cookies.ImportChrome(cookies.ChromeImportOptions{
		Domain:     ctx.Query("domain"),
		OutputPath: cookie_path,
	})
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to import Chrome cookies")
		result.Err(ctx, 500, "提取或保存 Chrome Cookie 失败: "+err.Error())
		return
	}

	c.logger.Info().
		Int("loaded", imported.Loaded).
		Int("skipped", imported.Skipped).
		Str("path", cookie_path).
		Msg("Chrome cookies imported and saved")

	result.Ok(ctx, gin.H{
		"count":   len(imported.Cookies),
		"skipped": imported.Skipped,
		"path":    cookie_path,
		"cookies": imported.Cookies,
	})
}

func (c *APIClient) handle_cookie_update(ctx *gin.Context) {
	body, err := read_cookie_update_body(ctx)
	if err != nil {
		c.logger.Warn().Err(err).Msg("invalid CookieCloud update body")
		write_cookie_update_error(ctx, http.StatusBadRequest, "Cookie 更新请求格式错误")
		return
	}

	update_result, err := services.ProcessCookieCloudUpdate(body, services.CookieCloudConfig{
		UUID:     c.cfg.CookieUUID,
		Password: c.cfg.CookiePassword,
		Key:      c.cfg.CookieKey,
	})
	if err != nil {
		handle_cookie_cloud_service_error(c, ctx, err)
		return
	}

	cookie_path := filepath.Join(c.cfg.WorkDir, "cookies.json")
	if err := cookies.SaveJSON(update_result.Cookies, cookie_path); err != nil {
		c.logger.Error().Err(err).Str("path", cookie_path).Msg("failed to save CookieCloud cookies")
		write_cookie_update_error(ctx, http.StatusInternalServerError, "保存 Cookie 失败")
		return
	}

	c.logger.Info().
		Int("loaded", len(update_result.Cookies)).
		Int("skipped", update_result.Skipped).
		Int("local_storage_domains", update_result.LocalStorageDomains).
		Str("crypto_type", update_result.CryptoType).
		Str("path", cookie_path).
		Msg("CookieCloud cookies updated and saved")

	ctx.JSON(http.StatusOK, gin.H{
		"action": "done",
		"code":   0,
		"msg":    "成功",
		"data": gin.H{
			"count":                 len(update_result.Cookies),
			"skipped":               update_result.Skipped,
			"local_storage_domains": update_result.LocalStorageDomains,
			"path":                  cookie_path,
		},
	})
}

func handle_cookie_cloud_service_error(c *APIClient, ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrCookieCloudInvalidRequest):
		c.logger.Warn().Err(err).Msg("failed to parse CookieCloud update request")
		write_cookie_update_error(ctx, http.StatusBadRequest, "Cookie 更新请求不是有效的 JSON")
	case errors.Is(err, services.ErrCookieCloudIncompleteRequest):
		write_cookie_update_error(ctx, http.StatusBadRequest, "Cookie 更新请求缺少 uuid 或 encrypted")
	case errors.Is(err, services.ErrCookieCloudUUIDMismatch):
		c.logger.Warn().Msg("rejected CookieCloud update with an unexpected UUID")
		write_cookie_update_error(ctx, http.StatusForbidden, "CookieCloud UUID 与配置不匹配")
	case errors.Is(err, services.ErrCookieCloudKeyNotConfigured):
		c.logger.Error().Msg("CookieCloud decryption key is not configured")
		write_cookie_update_error(ctx, http.StatusInternalServerError, "请先在 config.yaml 配置 cookie.password 或 cookie.key")
	case errors.Is(err, services.ErrCookieCloudInvalidConfig):
		c.logger.Error().Err(err).Msg("invalid CookieCloud decryption configuration")
		write_cookie_update_error(ctx, http.StatusInternalServerError, err.Error())
	case errors.Is(err, services.ErrCookieCloudDecryption):
		c.logger.Warn().Err(err).Msg("failed to decrypt CookieCloud update")
		write_cookie_update_error(ctx, http.StatusBadRequest, "CookieCloud 数据解密失败，请检查 UUID、密钥和 crypto_type")
	case errors.Is(err, services.ErrCookieCloudInvalidData):
		c.logger.Warn().Err(err).Msg("invalid CookieCloud data")
		write_cookie_update_error(ctx, http.StatusBadRequest, "CookieCloud 数据格式错误")
	default:
		c.logger.Error().Err(err).Msg("unexpected CookieCloud service error")
		write_cookie_update_error(ctx, http.StatusInternalServerError, "处理 CookieCloud 数据失败")
	}
}

func read_cookie_update_body(ctx *gin.Context) ([]byte, error) {
	limited_body := http.MaxBytesReader(ctx.Writer, ctx.Request.Body, cookie_update_max_body_size)
	var body_reader io.Reader = limited_body
	content_encoding := strings.ToLower(strings.TrimSpace(ctx.GetHeader("Content-Encoding")))
	if content_encoding != "" && content_encoding != "identity" {
		if content_encoding != "gzip" {
			return nil, fmt.Errorf("unsupported content encoding %q", content_encoding)
		}
		gzip_reader, err := gzip.NewReader(limited_body)
		if err != nil {
			return nil, fmt.Errorf("open gzip request body: %w", err)
		}
		defer gzip_reader.Close()
		body_reader = gzip_reader
	}

	body, err := io.ReadAll(io.LimitReader(body_reader, cookie_update_max_body_size+1))
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	if int64(len(body)) > cookie_update_max_body_size {
		return nil, fmt.Errorf("request body exceeds %d bytes", cookie_update_max_body_size)
	}
	return body, nil
}

func write_cookie_update_error(ctx *gin.Context, status int, message string) {
	ctx.JSON(status, gin.H{
		"action": "error",
		"code":   status,
		"msg":    message,
	})
}
