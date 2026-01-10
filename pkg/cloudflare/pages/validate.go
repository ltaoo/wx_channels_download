package pages

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"wx_channel/pkg/hash"
)

// getMimeType returns the appropriate MIME type based on file extension
func getMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	// Text files
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"

	// Image files
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"

	// Font files
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".eot":
		return "application/vnd.ms-fontobject"

	// Audio files
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"

	// Video files
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".wmv":
		return "video/x-ms-wmv"

	// Archive files
	case ".zip":
		return "application/zip"
	case ".rar":
		return "application/vnd.rar"
	case ".7z":
		return "application/x-7z-compressed"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"

	// Document files
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"

	// Code files
	case ".py":
		return "text/x-python"
	case ".java":
		return "text/x-java-source"
	case ".c":
		return "text/x-c"
	case ".cpp", ".cc":
		return "text/x-c++src"
	case ".h":
		return "text/x-c"
	case ".php":
		return "application/x-httpd-php"
	case ".rb":
		return "text/x-ruby"
	case ".go":
		return "text/x-go"
	case ".rs":
		return "text/x-rust"
	case ".swift":
		return "text/x-swift"
	case ".kt":
		return "text/x-kotlin"
	case ".scala":
		return "text/x-scala"
	case ".sh":
		return "application/x-sh"
	case ".bat":
		return "application/x-msdos-program"
	case ".ps1":
		return "application/x-powershell"

	// Data files
	case ".csv":
		return "text/csv"
	case ".tsv":
		return "text/tab-separated-values"
	case ".yaml", ".yml":
		return "application/x-yaml"
	case ".toml":
		return "application/toml"
	case ".ini":
		return "text/plain"
	case ".conf":
		return "text/plain"
	case ".config":
		return "text/plain"

	default:
		return "application/octet-stream"
	}
}

func walk(dir string, files_map map[string]FileContainer, dir_start string, ignore_patterns []string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Error reading directory %s: %v", dir, err)
		return
	}

	for _, file := range files {
		file_path := filepath.Join(dir, file.Name())
		relative_filepath, _ := filepath.Rel(dir_start, file_path)

		// 检查是否应该忽略此文件/目录
		if should_ignore(file_path, ignore_patterns) {
			fmt.Printf("Ignoring: %s\n", relative_filepath)
			continue
		}

		if file.IsDir() {
			// 递归遍历子目录
			walk(file_path, files_map, dir_start, ignore_patterns)
		} else {
			// 处理文件
			info, err := file.Info()
			if err != nil {
				log.Printf("Error getting file info for %s: %v", file_path, err)
				continue
			}

			// 计算文件哈希值
			file_hash, err := hash.FileHashWithExtension(file_path)
			if err != nil {
				log.Printf("Error calculating hash for %s: %v", file_path, err)
				file_hash = "" // 如果计算失败，设置为空字符串
			}

			// 创建 FileContainer
			_the_file := FileContainer{
				Filename:    file.Name(),
				Path:        file_path,
				SizeInBytes: int(info.Size()),
				ContentType: getMimeType(file.Name()), // 根据文件类型获取正确的 MIME type
				Hash:        file_hash,
			}

			files_map[relative_filepath] = _the_file
			// fmt.Printf("Found: %s (%d bytes, hash: %s)\n", relative_filepath, info.Size(), file_hash)
		}
	}
}

func Validate(directory string) (map[string]FileContainer, error) {
	IGNORE_FILES := []string{
		"_worker.js",
		"_redirects",
		"_headers",
		"_routes.json",
		"functions",
		"**/.DS_Store",
		"**/node_modules",
		"**/.git",
	}
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		// log.Fatalf("Directory does not exist: %s", directory)
		return make(map[string]FileContainer), err
	}
	// 获取绝对路径
	abs_filepath, err := filepath.Abs(directory)
	if err != nil {
		// log.Fatalf("Error getting absolute path: %v", err)
		return make(map[string]FileContainer), err
	}

	file_map := make(map[string]FileContainer)

	// fmt.Printf("Starting to walk directory: %s\n", abs_filepath)
	// fmt.Printf("Ignore patterns: %v\n", IGNORE_FILES)

	walk(abs_filepath, file_map, abs_filepath, IGNORE_FILES)

	// fmt.Printf("\nTotal files found: %d\n", len(file_map))
	return file_map, nil
}
