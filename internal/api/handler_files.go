package api

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/channels"
	result "wx_channel/internal/util"
	"wx_channel/pkg/system"
)

func (c *APIClient) handleIndex(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, string(files.HTMLHome))
}

func (c *APIClient) handlePlay(ctx *gin.Context) {
	targetURL := ctx.Query("url")
	if targetURL == "" {
		result.Err(ctx, 400, "missing targetURL")
		return
	}
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://" + targetURL
	}
	if _, err := url.Parse(targetURL); err != nil {
		result.Err(ctx, 400, "Invalid URL")
		return
	}
	decryptKeyStr := ctx.Query("key")
	decryptor := channels.NewChannelsVideoDecryptor()
	if decryptKeyStr != "" {
		decryptKey, err := strconv.ParseUint(decryptKeyStr, 0, 64)
		if err != nil {
			result.Err(ctx, 400, "invalid decryptKey")
			return
		}
		decryptor.DecryptOnlyInline(ctx.Writer, ctx.Request, targetURL, decryptKey, 131072)
		return
	}
	decryptor.SimpleProxy(targetURL, ctx.Writer, ctx.Request)
}

func (c *APIClient) handleOpenDownloadDir(ctx *gin.Context) {
	dir := c.cfg.DownloadDir
	if err := system.Open(dir); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

type OpenFolderAndHighlightFileBody struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func (c *APIClient) handleHighlightFileInFolder(ctx *gin.Context) {
	var body OpenFolderAndHighlightFileBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.Path == "" || body.Name == "" {
		result.Err(ctx, 400, "Missing the `path` or `name`")
		return
	}
	fullFilepath := filepath.Join(body.Path, body.Name)
	if _, err := os.Stat(fullFilepath); err != nil {
		result.Err(ctx, 500, "找不到文件")
		return
	}
	if err := system.ShowInExplorer(fullFilepath); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

func (c *APIClient) handleStreamVideo(ctx *gin.Context) {
	path := ctx.Query("path")
	if path == "" {
		taskID := ctx.Query("id")
		if taskID != "" {
			task := c.downloader.GetTask(taskID)
			if task != nil && task.Meta != nil && task.Meta.Opts != nil {
				path = filepath.Join(task.Meta.Opts.Path, task.Meta.Opts.Name)
			}
		}
	}

	if path == "" {
		result.Err(ctx, 400, "missing path or id")
		return
	}

	if _, err := os.Stat(path); err != nil {
		result.Err(ctx, 404, "file not found")
		return
	}
	ctx.File(path)
}

func (c *APIClient) handleStreamImage(ctx *gin.Context) {
	c.handleStreamVideo(ctx)
}

func (c *APIClient) handlePreviewFile(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, string(files.HTMLPreview))
}

func (c *APIClient) handleFetchFile(ctx *gin.Context) {
	path := ctx.Query("path")
	if path == "" {
		result.Err(ctx, 400, "missing path")
		return
	}
	fi, err := os.Stat(path)
	if err != nil {
		result.Err(ctx, 404, "file not found")
		return
	}
	if fi.IsDir() {
		result.Err(ctx, 400, "path is a directory")
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	if c.isImage(ext) {
		result.Ok(ctx, gin.H{
			"type": "image",
			"url":  "/file?path=" + url.QueryEscape(path),
		})
		return
	}

	if ext == ".mp3" || (c.isVideoOrImage(ext) && !c.isImage(ext)) {
		result.Ok(ctx, gin.H{
			"type": "video",
			"url":  "/file?path=" + url.QueryEscape(path),
		})
		return
	}

	if ext == ".html" || ext == ".htm" {
		result.Ok(ctx, gin.H{
			"type": "html",
			"url":  "/file?path=" + url.QueryEscape(path),
		})
		return
	}

	if ext == ".zip" {
		r, err := zip.OpenReader(path)
		if err != nil {
			result.Err(ctx, 500, fmt.Sprintf("failed to open zip: %v", err))
			return
		}
		defer r.Close()

		var images []map[string]string
		for _, f := range r.File {
			fExt := strings.ToLower(filepath.Ext(f.Name))
			if c.isImage(fExt) {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				if f.FileInfo().Size() > 10*1024*1024 {
					_ = rc.Close()
					continue
				}
				data, err := io.ReadAll(rc)
				_ = rc.Close()
				if err != nil {
					continue
				}

				base64Str := base64.StdEncoding.EncodeToString(data)
				mimeType := c.getMimeType(fExt)
				imgSrc := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str)
				images = append(images, map[string]string{
					"name": f.Name,
					"url":  imgSrc,
				})
			}
		}
		result.Ok(ctx, gin.H{
			"type":   "zip",
			"images": images,
		})
		return
	}

	result.Err(ctx, 400, "unsupported file type")
}

func (c *APIClient) isVideoOrImage(ext string) bool {
	if c.isImage(ext) {
		return true
	}
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".webm":
		return true
	}
	return false
}

func (c *APIClient) isImage(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

func (c *APIClient) getMimeType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	}
	return "image/jpeg"
}

func (c *APIClient) handleGetFileURL(ctx *gin.Context) {
	id := ctx.Query("id")
	if id == "" {
		result.Err(ctx, 400, "missing id")
		return
	}
	u := c.cfg.Protocol + "://" + c.cfg.Hostname
	if c.cfg.Port != 80 {
		u += ":" + strconv.Itoa(c.cfg.Port)
	}
	u += "/video?id=" + id
	result.Ok(ctx, gin.H{
		"url": u,
	})
}

func (c *APIClient) handleTest(ctx *gin.Context) {
	dir := c.cfg.DownloadDir
	if err := system.Open(dir); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

