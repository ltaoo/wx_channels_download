package services

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"wx_channel/pkg/system"
)

type FileService struct {
	downloadDir string
}

func NewFileService(downloadDir string) *FileService {
	return &FileService{
		downloadDir: downloadDir,
	}
}

func (s *FileService) GetDownloadDir() string {
	return s.downloadDir
}

func (s *FileService) OpenDownloadDir() error {
	return system.Open(s.downloadDir)
}

func (s *FileService) ShowInExplorer(path string) error {
	return system.ShowInExplorer(path)
}

func (s *FileService) IsVideoOrImage(ext string) bool {
	ext = strings.ToLower(ext)
	if s.IsImage(ext) {
		return true
	}
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".webm":
		return true
	}
	return false
}

func (s *FileService) IsImage(ext string) bool {
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

func (s *FileService) GetMimeType(ext string) string {
	switch strings.ToLower(ext) {
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

func (s *FileService) GetFileInfo(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (s *FileService) IsDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

type FileInfo struct {
	Type   string
	URL    string
	Images []map[string]string
}

func (s *FileService) GetFileInfoAndType(path string) (*FileInfo, error) {
	ext := strings.ToLower(filepath.Ext(path))

	if s.IsImage(ext) {
		return &FileInfo{
			Type: "image",
			URL:  path,
		}, nil
	}

	if ext == ".mp3" || (s.IsVideoOrImage(ext) && !s.IsImage(ext)) {
		return &FileInfo{
			Type: "video",
			URL:  path,
		}, nil
	}

	if ext == ".html" || ext == ".htm" {
		return &FileInfo{
			Type: "html",
			URL:  path,
		}, nil
	}

	if ext == ".zip" {
		r, err := zip.OpenReader(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open zip: %v", err)
		}
		defer r.Close()

		var images []map[string]string
		for _, f := range r.File {
			fExt := strings.ToLower(filepath.Ext(f.Name))
			if s.IsImage(fExt) {
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
				mimeType := s.GetMimeType(fExt)
				imgSrc := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str)
				images = append(images, map[string]string{
					"name": f.Name,
					"url":  imgSrc,
				})
			}
		}
		return &FileInfo{
			Type:   "zip",
			Images: images,
		}, nil
	}

	return nil, fmt.Errorf("unsupported file type")
}
