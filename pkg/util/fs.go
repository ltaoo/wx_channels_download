package util

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
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
		"spec":        "",
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
	if filepath.Ext(name) == "" {
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
			base_name = fmt.Sprintf("%s(%d)%s", name_without_ext, counter, ext)
			counter++
		} else {
			break
		}
	}
	name = base_name
	return name, nil
}
