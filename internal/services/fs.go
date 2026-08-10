package services

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ltaoo/velo/fileserver"
)

// FSService provides read-only filesystem discovery for the application.
// Keeping fileserver behind this service prevents handlers from depending on
// the implementation package directly.
type FSService struct{}

func NewFSService() *FSService {
	return &FSService{}
}

type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
	Ext     string `json:"ext"`
}

type FileSortBy string

const (
	FileSortByName    FileSortBy = "name"
	FileSortBySize    FileSortBy = "size"
	FileSortByModTime FileSortBy = "modTime"
	FileSortByExt     FileSortBy = "ext"
)

type FileSortOption struct {
	By        FileSortBy `json:"by"`
	Ascending bool       `json:"ascending"`
}

// ListFilesOptions controls a directory listing. An empty Dir defaults to the
// current user's Documents directory.
type ListFilesOptions struct {
	Dir       string          `json:"dir"`
	Ignore    []string        `json:"ignore"`
	Accept    []string        `json:"accept"`
	Recursive bool            `json:"recursive"`
	Depth     int             `json:"depth"`
	Sort      *FileSortOption `json:"sort"`
	Offset    int             `json:"offset"`
	Limit     int             `json:"limit"`
}

type FileResult struct {
	Files []FileInfo `json:"files"`
	Total int        `json:"total"`
}

func (s *FSService) ListFiles(options ListFilesOptions) (*FileResult, error) {
	return fetch_files(fileserver.FetchFilesOption{
		Dir:       s.resolve_dir(options.Dir),
		Ignore:    with_default_file_ignores(options.Ignore),
		Accept:    options.Accept,
		Recursive: options.Recursive,
		Depth:     options.Depth,
		Sort:      to_file_server_sort(options.Sort),
		Offset:    normalize_file_offset(options.Offset),
		Limit:     normalize_file_limit(options.Limit),
	})
}

func (s *FSService) CommonDirs() []string {
	dirs := fileserver.FetchCommonDirs()
	return append([]string(nil), dirs...)
}

// DocumentsDir returns the current user's Documents directory.
func (s *FSService) DocumentsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Documents")
}

func (s *FSService) resolve_dir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir != "" {
		return dir
	}
	return s.DocumentsDir()
}

func fetch_files(options fileserver.FetchFilesOption) (*FileResult, error) {
	files, total, err := fileserver.FetchFiles(options)
	if err != nil {
		return nil, err
	}
	result := make([]FileInfo, 0, len(files))
	for _, file := range files {
		result = append(result, FileInfo{
			Name:    file.Name,
			Path:    file.Path,
			Size:    file.Size,
			ModTime: file.ModTime,
			IsDir:   file.IsDir,
			Ext:     file.Ext,
		})
	}
	return &FileResult{Files: result, Total: total}, nil
}

func to_file_server_sort(option *FileSortOption) *fileserver.SortOption {
	if option == nil {
		return nil
	}
	return &fileserver.SortOption{
		By:        fileserver.SortBy(option.By),
		Ascending: option.Ascending,
	}
}

func normalize_file_offset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func normalize_file_limit(limit int) int {
	if limit < 0 {
		return 0
	}
	return limit
}

func with_default_file_ignores(ignore []string) []string {
	result := append([]string(nil), ignore...)
	for _, pattern := range result {
		if pattern == ".DS_Store" {
			return result
		}
	}
	return append(result, ".DS_Store")
}
