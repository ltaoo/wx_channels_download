package util

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

func BuildFilename(feed struct {
	Title     string
	ObjectId  string
	CreatedAt string
	Contact   struct {
		Nickname string
		Username string
	}
}, spec *struct{ FileFormat string }, cfg struct{ FilenameTemplate string }) string {
	default_name := func() string {
		if feed.Title != "" {
			return feed.Title
		}
		if feed.ObjectId != "" {
			return feed.ObjectId
		}
		return NowMillisStr()
	}()

	params := map[string]string{
		"filename":    default_name,
		"id":          feed.ObjectId,
		"title":       feed.Title,
		"spec":        "original",
		"created_at":  string(feed.CreatedAt),
		"download_at": NowMillisStr(),
	}
	if feed.Contact.Nickname != "" {
		params["author"] = feed.Contact.Nickname
	}
	if spec != nil && spec.FileFormat != "" {
		params["spec"] = spec.FileFormat
	}

	template := cfg.FilenameTemplate
	if strings.TrimSpace(template) == "" {
		return default_name
	}
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	filename := re.ReplaceAllStringFunc(template, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if len(sub) > 1 {
			if v, ok := params[sub[1]]; ok {
				return v
			}
		}
		return ""
	})
	return filename
}

func ValidateAndSplitFilename(input string) (string, string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", "", fmt.Errorf("filename is empty")
	}
	s = strings.ReplaceAll(s, "\\", "/")
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		default:
			return r
		}
	}, s)
	if strings.HasSuffix(s, "/") {
		s = strings.TrimSuffix(s, "/")
	}
	parts := make([]string, 0)
	for _, p := range strings.Split(s, "/") {
		if p == "" {
			continue
		}
		if p == "." || p == ".." {
			return "", "", fmt.Errorf("invalid path segment")
		}
		if len(p) > 255 {
			return "", "", fmt.Errorf("segment too long")
		}
		invalid := false
		for _, r := range p {
			if unicode.IsControl(r) {
				invalid = true
				break
			}
		}
		if invalid {
			return "", "", fmt.Errorf("invalid characters")
		}
		if regexp.MustCompile(`[<>:"\\|?*]`).MatchString(p) {
			return "", "", fmt.Errorf("invalid characters")
		}
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		return "", "", fmt.Errorf("filename is empty")
	}
	name := parts[len(parts)-1]
	dir := strings.Join(parts[:len(parts)-1], "/")
	return dir, name, nil
}

func EnsureFilename(name string, dir string, download_dir string) (string, error) {
	if !strings.HasSuffix(strings.ToLower(name), ".mp4") {
		name = name + ".mp4"
	}
	latest_download_dir := download_dir
	if dir != "" {
		latest_download_dir = filepath.Join(latest_download_dir, dir)
	}
	if err := os.MkdirAll(latest_download_dir, 0o755); err != nil {
		return "", err
	}
	// 检查是否有重名文件，如果有则重命名
	base_name := name
	ext := filepath.Ext(name)
	name_without_ext := name[:len(name)-len(ext)]
	counter := 1
	for {
		if _, err := os.Stat(filepath.Join(latest_download_dir, base_name)); err == nil {
			base_name = fmt.Sprintf("%s_%d%s", name_without_ext, counter, ext)
			counter++
		} else {
			break
		}
	}
	name = base_name
	return name, nil
}

// 文件名处理器
type FilenameProcessor struct {
	// 存储已使用的文件名，key: 路径/文件名, value: 使用次数
	usedFilenames map[string]int
	// 平台特定的非法字符
	forbiddenChars *regexp.Regexp
	// 最大文件名长度
	maxFilenameLength int
	// 当前工作目录
	baseDir string
}

// 初始化处理器
func NewFilenameProcessor(root_dir string, existing_files map[string]int) *FilenameProcessor {
	// 跨平台的非法字符（Windows最严格）
	// Windows: <>:"/\|?* 以及控制字符
	// Unix-like: / 和 null
	forbiddenPattern := `[<>:"/\\|?*\x00-\x1f]`

	return &FilenameProcessor{
		usedFilenames:     existing_files,
		forbiddenChars:    regexp.MustCompile(forbiddenPattern),
		maxFilenameLength: 235, // 大多数文件系统的限制
		baseDir:           root_dir,
	}
}

// 辅助函数：按字节截取字符串，保持UTF-8完整性
func (fp *FilenameProcessor) truncateString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	// 确保不截断UTF-8字符
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			s = s[:len(s)-1]
		} else {
			break
		}
	}
	return s
}

// 验证和清理文件名
func (fp *FilenameProcessor) SanitizeFilename(filename string) (string, error) {
	fmt.Println("SanitizeFilename", filename)
	if filename == "" {
		return "", fmt.Errorf("filename cannot be empty")
	}

	// 检查长度
	filename = fp.truncateString(filename, fp.maxFilenameLength)

	// 移除非法字符
	filename = fp.forbiddenChars.ReplaceAllString(filename, "")

	// 移除首尾空格和点
	filename = strings.TrimSpace(filename)
	filename = strings.Trim(filename, ".")

	if filename == "" {
		return "", fmt.Errorf("filename contains only invalid characters")
	}

	// 检查保留文件名（Windows）
	reservedNames := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true,
		"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
		"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}

	baseName := strings.SplitN(filename, ".", 2)[0]
	baseName = strings.ToUpper(baseName)
	if reservedNames[baseName] {
		return filename + "_", nil
	}

	return filename, nil
}

// 处理单个文件名，考虑文件夹
func (fp *FilenameProcessor) ProcessFilename(input_name string) (string, string, error) {
	input_name = strings.ReplaceAll(input_name, "/", "_")
	// 分离目录和文件名
	dir, filename := filepath.Split(input_name)
	// 清理文件名部分
	clean_name, err := fp.SanitizeFilename(filename)
	if err != nil {
		return "", "", fmt.Errorf("invalid filename '%s': %v", filename, err)
	}
	// 处理目录部分
	if dir != "" {
		dir = strings.TrimSuffix(dir, string(filepath.Separator))
		dir_components := strings.Split(dir, string(filepath.Separator))
		valid_dirs := []string{}
		for _, comp := range dir_components {
			valid_dir, err := fp.SanitizeFilename(comp)
			if err != nil {
				continue
				// return "", "", fmt.Errorf("invalid directory name '%s' in path: %v", comp, err)
			}
			valid_dirs = append(valid_dirs, valid_dir)
		}
		dir = filepath.Join(valid_dirs...)
	}
	// 组合完整路径
	full_path := filepath.Join(dir, clean_name)
	path_key := filepath.Clean(full_path)
	// 处理重复文件名
	count, exists := fp.usedFilenames[path_key]
	if exists {
		// 添加后缀
		ext := filepath.Ext(clean_name)
		name_without_ext := clean_name[:len(clean_name)-len(ext)]
		for {
			count++
			new_name := fmt.Sprintf("%s(%d)%s", name_without_ext, count, ext)
			new_path := filepath.Join(dir, new_name)
			new_path_key := filepath.Clean(new_path)

			if _, ok := fp.usedFilenames[new_path_key]; !ok {
				clean_name = new_name
				path_key = new_path_key
				full_path = new_path
				break
			}
		}
	}
	// 记录已使用的文件名
	fp.usedFilenames[path_key] = count
	if exists {
		fp.usedFilenames[path_key] = 0
	}
	return clean_name, dir, nil
}

// 主处理函数
func ProcessFilename(existing_task_map map[string]int, items []map[string]string, base_dir string) ([]map[string]string, error) {
	processor := NewFilenameProcessor(base_dir, existing_task_map)
	results := make([]map[string]string, 0, len(items))

	// 第一遍：收集所有原始文件名
	original_names := make(map[string][]int) // name -> [ids]
	for _, item := range items {
		if name, ok := item["name"]; ok {
			original_names[name] = append(original_names[name], get_id(item))
		}
	}

	// 第二遍：处理重复
	for _, item := range items {
		result := make(map[string]string)

		// 复制原始项
		for k, v := range item {
			result[k] = v
		}

		name, ok := item["name"]
		if !ok || name == "" {
			return nil, fmt.Errorf("item %v has no name field", item)
		}

		// 处理文件名
		final_name, dir, err := processor.ProcessFilename(name)
		if err != nil {
			return nil, fmt.Errorf("failed to process filename for item %v: %v", item, err)
		}

		// 更新结果
		result["name"] = final_name
		result["original_name"] = name
		result["full_path"] = filepath.Join(dir, final_name)

		results = append(results, result)
	}

	return results, nil
}

// 辅助函数：从map中获取ID
func get_id(item map[string]string) int {
	// 根据你的实际情况调整
	// 这里假设id是数字字符串
	idStr, ok := item["id"]
	if !ok {
		return 0
	}

	var id int
	fmt.Sscanf(idStr, "%d", &id)
	return id
}
