package api

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
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
	full_file_path := resolve_show_file_path(c.cfg.DownloadDir, body)
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

func resolve_show_file_path(download_dir string, body ShowFileBody) string {
	path := strings.TrimSpace(body.Path)
	name := strings.TrimSpace(body.Name)
	if path == "" {
		if name == "" {
			return ""
		}
		return filepath.Join(download_dir, name)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(download_dir, path)
	}
	path = filepath.Clean(path)
	if name == "" {
		return path
	}

	// Download task responses store the directory and file name separately.
	// Keep accepting a complete file path for compatibility with older tasks.
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return filepath.Join(path, name)
		}
		return path
	}
	if filepath.Base(path) == name {
		return path
	}
	return filepath.Join(path, name)
}

func (c *APIClient) handle_fetch_file(ctx *gin.Context) {
	path := ctx.Query("path")
	if path == "" {
		result.Err(ctx, 400, "missing path")
		return
	}
	path, fi, err := c.resolve_fetch_file_path(path)
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

func api_file_url(path string) string {
	values := url.Values{}
	values.Set("path", path)
	return "/api/file?" + values.Encode()
}

func (c *APIClient) resolve_fetch_file_path(path string) (string, os.FileInfo, error) {
	if !filepath.IsAbs(path) && c.cfg != nil {
		path = filepath.Join(c.cfg.DownloadDir, path)
	}
	path = filepath.Clean(path)
	fi, err := os.Stat(path)
	if err == nil {
		return path, fi, nil
	}
	if !os.IsNotExist(err) {
		return path, nil, err
	}
	if matched_path, ok := c.resolve_unique_download_file_prefix(path); ok {
		fi, stat_err := os.Stat(matched_path)
		if stat_err == nil {
			return matched_path, fi, nil
		}
	}
	return path, nil, err
}

func (c *APIClient) resolve_unique_download_file_prefix(path string) (string, bool) {
	if c == nil || c.cfg == nil || strings.TrimSpace(c.cfg.DownloadDir) == "" {
		return "", false
	}
	root, err := filepath.Abs(c.cfg.DownloadDir)
	if err != nil {
		return "", false
	}
	root = filepath.Clean(root)
	absolute_path, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	absolute_path = filepath.Clean(absolute_path)
	if !pathWithinDownloadRoot(root, absolute_path) {
		return "", false
	}
	dir := filepath.Dir(absolute_path)
	base := filepath.Base(absolute_path)
	if base == "." || base == string(filepath.Separator) || strings.TrimSpace(base) == "" {
		return "", false
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	var matched_path string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), base) {
			continue
		}
		candidate := filepath.Join(dir, entry.Name())
		candidate, err = filepath.Abs(candidate)
		if err != nil {
			continue
		}
		candidate = filepath.Clean(candidate)
		if !pathWithinDownloadRoot(root, candidate) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.IsDir() {
			continue
		}
		if matched_path != "" {
			return "", false
		}
		matched_path = candidate
	}
	return matched_path, matched_path != ""
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
