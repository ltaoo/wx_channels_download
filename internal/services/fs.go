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

// SearchFilesOptions controls fuzzy filename search. Recursive defaults to
// true when nil so that search traverses the directory tree by default.
type SearchFilesOptions struct {
	Dir       string          `json:"dir"`
	Query     string          `json:"query"`
	Ignore    []string        `json:"ignore"`
	Accept    []string        `json:"accept"`
	Recursive *bool           `json:"recursive"`
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
	return fetchFiles(fileserver.FetchFilesOption{
		Dir:       s.resolveDir(options.Dir),
		Ignore:    withDefaultFileIgnores(options.Ignore),
		Accept:    options.Accept,
		Recursive: options.Recursive,
		Depth:     options.Depth,
		Sort:      toFileServerSort(options.Sort),
		Offset:    normalizeFileOffset(options.Offset),
		Limit:     normalizeFileLimit(options.Limit),
	})
}

func (s *FSService) SearchFiles(options SearchFilesOptions) (*FileResult, error) {
	recursive := true
	if options.Recursive != nil {
		recursive = *options.Recursive
	}
	return fetchFiles(fileserver.FetchFilesOption{
		Dir:       s.resolveDir(options.Dir),
		Search:    strings.TrimSpace(options.Query),
		Ignore:    withDefaultFileIgnores(options.Ignore),
		Accept:    options.Accept,
		Recursive: recursive,
		Depth:     options.Depth,
		Sort:      toFileServerSort(options.Sort),
		Offset:    normalizeFileOffset(options.Offset),
		Limit:     normalizeFileLimit(options.Limit),
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

func (s *FSService) resolveDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir != "" {
		return dir
	}
	return s.DocumentsDir()
}

func fetchFiles(options fileserver.FetchFilesOption) (*FileResult, error) {
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

func toFileServerSort(option *FileSortOption) *fileserver.SortOption {
	if option == nil {
		return nil
	}
	return &fileserver.SortOption{
		By:        fileserver.SortBy(option.By),
		Ascending: option.Ascending,
	}
}

func normalizeFileOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func normalizeFileLimit(limit int) int {
	if limit < 0 {
		return 0
	}
	return limit
}

func withDefaultFileIgnores(ignore []string) []string {
	result := append([]string(nil), ignore...)
	for _, pattern := range result {
		if pattern == ".DS_Store" {
			return result
		}
	}
	return append(result, ".DS_Store")
}
