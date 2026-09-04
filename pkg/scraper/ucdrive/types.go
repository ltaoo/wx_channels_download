// Package ucdrive fetches public UC Drive share pages.
package ucdrive

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

const PlatformID = "ucdrive"

// File is one entry in a UC Drive share tree.
type File struct {
	Fid           string `json:"fid"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	ParentFid     string `json:"parent_fid,omitempty"`
	IsDir         bool   `json:"is_dir"`
	Size          int64  `json:"size,omitempty"`
	FormatType    string `json:"format_type,omitempty"`
	FileType      int    `json:"file_type,omitempty"`
	ShareFidToken string `json:"share_fid_token,omitempty"`
	CreatedAt     int64  `json:"created_at,omitempty"`
	UpdatedAt     int64  `json:"updated_at,omitempty"`
	DownloadURL   string `json:"download_url,omitempty"`
	Children      []File `json:"children,omitempty"`
}

// Share is the normalized result returned by Client.Fetch.
type Share struct {
	URL             string `json:"url"`
	PwdID           string `json:"pwd_id"`
	Stoken          string `json:"stoken,omitempty"`
	DownloadCookies string `json:"download_cookies,omitempty"`
	Title           string `json:"title"`
	Author          string `json:"author,omitempty"`
	AuthorAvatarURL string `json:"author_avatar_url,omitempty"`
	ExpiresAt       int64  `json:"expires_at,omitempty"`
	FileCount       int    `json:"file_count"`
	TotalSize       int64  `json:"total_size"`
	Files           []File `json:"files"`
}

// FlattenFiles returns all entries in display order.
func (s *Share) FlattenFiles() []File {
	if s == nil {
		return nil
	}
	var files []File
	var walk func([]File)
	walk = func(entries []File) {
		for _, entry := range entries {
			files = append(files, entry)
			walk(entry.Children)
		}
	}
	walk(s.Files)
	return files
}

// ParseShareURL validates a UC Drive share URL and returns its password ID.
func ParseShareURL(raw_url string) (string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return "", fmt.Errorf("解析 UC 网盘 URL 失败: %w", err)
	}
	if parsed_url.Scheme != "http" && parsed_url.Scheme != "https" {
		return "", fmt.Errorf("UC 网盘仅支持 HTTP/HTTPS URL")
	}
	if strings.ToLower(parsed_url.Hostname()) != "drive.uc.cn" {
		return "", fmt.Errorf("不是 UC 网盘分享 URL")
	}
	parts := strings.Split(strings.Trim(path.Clean(parsed_url.Path), "/"), "/")
	if len(parts) != 2 || (parts[0] != "s" && parts[0] != "share") || strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("UC 网盘分享 URL 格式错误")
	}
	pwd_id, err := url.PathUnescape(parts[1])
	if err != nil || strings.TrimSpace(pwd_id) == "" {
		return "", fmt.Errorf("UC 网盘分享 ID 格式错误")
	}
	return pwd_id, nil
}

func file_path(parent_path string, name string, fid string) string {
	name = strings.TrimSpace(strings.NewReplacer("/", "_", "\\", "_").Replace(name))
	if name == "" || name == "." || name == ".." {
		name = strings.TrimSpace(fid)
	}
	if parent_path == "" {
		return name
	}
	return path.Join(parent_path, name)
}
