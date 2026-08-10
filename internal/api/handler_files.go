package api

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/frontend"
	"wx_channel/internal/services"
	result "wx_channel/internal/util"
	"wx_channel/pkg/system"
)

type ShowFileBody struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func (c *APIClient) handle_show_file(ctx *gin.Context) {
	var body ShowFileBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	full_file_path := ""
	if strings.TrimSpace(body.Path) != "" {
		relative_path := strings.TrimSpace(body.Path)
		if filepath.IsAbs(relative_path) {
			full_file_path = relative_path
		} else {
			full_file_path = filepath.Join(c.cfg.DownloadDir, relative_path)
		}
	}
	if full_file_path == "" && strings.TrimSpace(body.Name) != "" {
		full_file_path = filepath.Join(c.cfg.DownloadDir, body.Name)
	}
	if full_file_path == "" {
		result.Err(ctx, 400, "Missing the resource path")
		return
	}
	if _, err := os.Stat(full_file_path); err != nil {
		result.Err(ctx, 404, "文件不存在")
		return
	}
	if err := system.ShowInExplorer(full_file_path); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

func (c *APIClient) handle_preview_page(ctx *gin.Context) {
	data, err := frontend.Assets().ReadRoot("preview.html")
	if err != nil {
		ctx.String(http.StatusInternalServerError, "preview page not found")
		return
	}
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, string(data))
}

func (c *APIClient) handle_fetch_file(ctx *gin.Context) {
	path := ctx.Query("path")
	if path == "" {
		result.Err(ctx, 400, "missing path")
		return
	}
	if !filepath.IsAbs(path) && c.cfg != nil {
		path = filepath.Join(c.cfg.DownloadDir, path)
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
	if ctx.Query("preview") != "1" {
		ctx.File(path)
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".zip" {
		r, err := zip.OpenReader(path)
		if err != nil {
			result.Err(ctx, 500, fmt.Sprintf("failed to open zip: %v", err))
			return
		}
		defer r.Close()

		var images []map[string]string
		for _, f := range r.File {
			file_ext := strings.ToLower(filepath.Ext(f.Name))
			if c.is_image(file_ext) {
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

				base64_str := base64.StdEncoding.EncodeToString(data)
				mime_type := c.get_mime_type(file_ext)
				image_src := fmt.Sprintf("data:%s;base64,%s", mime_type, base64_str)
				images = append(images, map[string]string{
					"name": f.Name,
					"url":  image_src,
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

func (c *APIClient) is_image(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

func (c *APIClient) get_mime_type(ext string) string {
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

func (c *APIClient) handle_list_files(ctx *gin.Context) {
	if c.fs_service == nil {
		result.Err(ctx, 500, "文件服务未初始化")
		return
	}
	var options services.ListFilesOptions
	if err := ctx.ShouldBindJSON(&options); err != nil {
		result.Err(ctx, 400, "请求参数无效: "+err.Error())
		return
	}
	files, err := c.fs_service.ListFiles(options)
	if err != nil {
		result.Err(ctx, 500, "读取文件列表失败: "+err.Error())
		return
	}
	result.Ok(ctx, files)
}
