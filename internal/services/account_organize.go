package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/dm"
	"wx_channel/pkg/hermes"
)

var kodi_organize_mu sync.Mutex

type kodi_unique_id struct {
	Type    string `xml:"type,attr"`
	Default bool   `xml:"default,attr"`
	Value   string `xml:",chardata"`
}

type kodi_thumb struct {
	Aspect string `xml:"aspect,attr"`
	URL    string `xml:",chardata"`
}

type kodi_movie_nfo struct {
	XMLName   xml.Name       `xml:"movie"`
	Title     string         `xml:"title"`
	Plot      string         `xml:"plot,omitempty"`
	Premiered string         `xml:"premiered,omitempty"`
	Year      int            `xml:"year,omitempty"`
	Studio    string         `xml:"studio,omitempty"`
	Thumb     *kodi_thumb    `xml:"thumb,omitempty"`
	UniqueID  kodi_unique_id `xml:"uniqueid"`
}

// OrganizeAccountFiles moves completed videos and persists their new locations.
func (s *DownloadTaskService) OrganizeAccountFiles(ctx context.Context, request dm.OrganizeAccountFilesRequest) (*dm.OrganizeAccountFilesResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(request.OutputDir) {
		return nil, errors.New("output_dir 必须是下载器所在主机上的绝对路径")
	}
	if s == nil || s.db == nil {
		return nil, errors.New("数据库不可用")
	}
	kodi_organize_mu.Lock()
	defer kodi_organize_mu.Unlock()
	request.AccountID = strings.TrimSpace(request.AccountID)
	db := s.db.WithContext(ctx)
	var account model.Account
	if err := db.Where("id = ? AND deleted_at IS NULL", request.AccountID).First(&account).Error; err != nil {
		return nil, err
	}
	var rows []struct {
		model.DownloadResource
		ContentID   string `gorm:"column:organize_content_id"`
		Title       string
		Description string
		CoverURL    string
		PublishTime *int64
	}
	err := db.Table("download_resource r").
		Select("r.*, c.id AS organize_content_id, c.title, c.description, c.cover_url, c.publish_time").
		Joins("LEFT JOIN download_task t ON t.id = r.task_id").
		Joins("JOIN content c ON c.id = COALESCE(NULLIF(r.content_id, ''), t.content_id)").
		Where("r.deleted_at IS NULL AND r.status = ? AND LOWER(r.type) = 'file'", model.ResourceStatusFinished).
		Where("r.task_id IS NULL OR (t.deleted_at IS NULL AND t.status = ?)", model.TaskStatusFinished).
		Where("c.deleted_at IS NULL AND EXISTS (SELECT 1 FROM content_account ca WHERE ca.content_id = c.id AND ca.account_id = ?)", account.Id).
		Order("r.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	var all_resources []model.DownloadResource
	if err := db.Select("id, download_dir, name").Where("deleted_at IS NULL").Find(&all_resources).Error; err != nil {
		return nil, err
	}
	source_counts := make(map[string]int)
	for _, resource := range all_resources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path, err := filepath.EvalSymlinks(hermes.ResolveOutputPath(s.download_dir, resource.DownloadDir, resource.Name))
		if err == nil {
			path, err = filepath.Abs(path)
		}
		if err == nil {
			source_counts[path]++
		}
	}
	account_name := account.Nickname
	if strings.TrimSpace(account_name) == "" {
		account_name = account.Id
	}
	account_hash := sha256.Sum256([]byte(account.Id))
	output_root, err := kodi_output_root(filepath.Clean(request.OutputDir))
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(output_root, fmt.Sprintf("%s [%x]", kodi_filename(account_name), account_hash[:6]))
	result := &dm.OrganizeAccountFilesResult{
		AccountID: account.Id, Directory: directory,
		DryRun: request.DryRun == nil || *request.DryRun, Files: []dm.OrganizeAccountFile{},
	}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		extension := strings.ToLower(filepath.Ext(row.Name))
		switch extension {
		case ".mp4", ".mkv", ".avi", ".mov", ".webm", ".m4v", ".ts", ".m2ts", ".mpeg", ".mpg", ".flv", ".wmv", ".vob", ".ogv":
		default:
			continue
		}
		title := row.Title
		if strings.TrimSpace(title) == "" {
			title = strings.TrimSuffix(filepath.Base(row.Name), filepath.Ext(row.Name))
			title = strings.TrimSuffix(title, fmt.Sprintf(" [%d]", row.Id))
		}
		// Resource IDs distinguish videos with identical titles and keep retries stable.
		basename := fmt.Sprintf("%s [%d]", kodi_filename(title), row.Id)
		movie_directory := filepath.Join(directory, basename)
		item := dm.OrganizeAccountFile{ResourceID: row.Id, Source: hermes.ResolveOutputPath(s.download_dir, row.DownloadDir, row.Name),
			Video: filepath.Join(movie_directory, basename+extension), NFO: filepath.Join(movie_directory, basename+".nfo"), Status: "planned"}
		func() {
			info, err := os.Lstat(item.Source)
			if err == nil && (!info.Mode().IsRegular() || info.Size() == 0) {
				err = errors.New("源文件不是非空普通文件")
			}
			if err == nil {
				var resolved_source string
				resolved_source, err = filepath.EvalSymlinks(item.Source)
				if err == nil {
					resolved_source, err = filepath.Abs(resolved_source)
				}
				// shortcut: shared source paths require a coordinated move of all owners.
				if err == nil && source_counts[resolved_source] > 1 {
					err = errors.New("多个资源记录共用此文件，未移动，请先处理共享路径")
				}
			}
			if err != nil {
				item.Status, item.Error = "error", err.Error()
				return
			}
			if result.DryRun {
				return
			}
			premiered, year := "", 0
			if row.PublishTime != nil && *row.PublishTime > 0 {
				published := time.Unix(*row.PublishTime, 0).UTC()
				premiered, year = published.Format("2006-01-02"), published.Year()
			}
			nfo := kodi_movie_nfo{Title: title, Plot: row.Description,
				Premiered: premiered, Year: year, Studio: account_name, Thumb: &kodi_thumb{Aspect: "poster", URL: row.CoverURL},
				UniqueID: kodi_unique_id{Type: "dm", Default: true, Value: fmt.Sprintf("%s/resource/%d", row.ContentID, row.Id)}}
			err = s.kodi_move_resource(ctx, row.DownloadResource, item, nfo)
			if err != nil {
				item.Status, item.Error = "error", err.Error()
				return
			}
			item.Status = "ready"
		}()
		result.Files = append(result.Files, item)
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func (s *DownloadTaskService) kodi_move_resource(ctx context.Context, resource model.DownloadResource, item dm.OrganizeAccountFile, nfo kodi_movie_nfo) (err error) {
	source_path, err := filepath.EvalSymlinks(item.Source)
	if err != nil {
		return err
	}
	source_path, err = filepath.Abs(source_path)
	if err != nil {
		return err
	}
	source_info, err := os.Lstat(source_path)
	if err != nil {
		return err
	}
	video_created, nfo_created, committed := false, false, false
	defer func() {
		if err == nil || committed {
			return
		}
		for path, created := range map[string]bool{item.Video: video_created, item.NFO: nfo_created} {
			if created {
				if remove_err := os.Remove(path); remove_err != nil {
					err = fmt.Errorf("%w; 清理新文件 %s 失败: %v", err, path, remove_err)
				}
			}
		}
	}()
	// Finish metadata validation before preparing or removing the video.
	nfo_created, err = kodi_write_nfo(ctx, item.NFO, nfo)
	if err != nil {
		return err
	}
	if source_path != item.Video {
		if err = os.Link(source_path, item.Video); err == nil {
			video_created = true
		} else {
			// A copy is required across filesystems or when hard links are unsupported.
			source, open_err := os.Open(source_path)
			if open_err != nil {
				return open_err
			}
			video_created, err = kodi_write_file(ctx, item.Video, source)
			close_err := source.Close()
			if err != nil {
				return err
			}
			if close_err != nil {
				return close_err
			}
		}
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&model.DownloadResource{}).
			Where("id = ? AND download_dir = ? AND name = ? AND status = ? AND deleted_at IS NULL", resource.Id, resource.DownloadDir, resource.Name, model.ResourceStatusFinished).
			Where("task_id IS NULL OR EXISTS (SELECT 1 FROM download_task t WHERE t.id = download_resource.task_id AND t.status = ? AND t.deleted_at IS NULL)", model.TaskStatusFinished).
			Updates(map[string]any{"download_dir": filepath.Dir(item.Video), "name": filepath.Base(item.Video), "updated_at": time.Now().UnixMilli()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errors.New("资源已变更或已不处于完成状态，未移动")
		}
		return nil
	})
	if err != nil {
		return err
	}
	committed = true
	if source_path != item.Video {
		current_info, stat_err := os.Lstat(source_path)
		if stat_err != nil {
			return fmt.Errorf("新路径已保存，但检查旧文件失败: %w", stat_err)
		}
		if !os.SameFile(source_info, current_info) || source_info.Size() != current_info.Size() || !source_info.ModTime().Equal(current_info.ModTime()) {
			return fmt.Errorf("新路径已保存，但旧文件在整理期间被修改或替换，未移除: %s", source_path)
		}
		if err := os.Remove(source_path); err != nil {
			return fmt.Errorf("新路径已保存，但移除旧文件 %s 失败: %w", source_path, err)
		}
	}
	return nil
}

func kodi_filename(value string) string {
	value = strings.Map(func(char rune) rune {
		if unicode.IsControl(char) || strings.ContainsRune(`<>:"/\|?*`, char) {
			return '_'
		}
		return char
	}, value)
	characters := []rune(strings.Trim(value, " ."))
	if len(characters) > 48 {
		characters = characters[:48]
	}
	if len(characters) == 0 {
		return "untitled"
	}
	return string(characters)
}

func kodi_write_nfo(ctx context.Context, path string, nfo kodi_movie_nfo) (bool, error) {
	if nfo.Thumb != nil && nfo.Thumb.URL == "" {
		nfo.Thumb = nil
	}
	data, err := xml.MarshalIndent(nfo, "", "  ")
	if err != nil {
		return false, err
	}
	return kodi_write_file(ctx, path, bytes.NewReader(append([]byte(xml.Header), append(data, '\n')...)))
}

// Resolve only the user-selected root; generated child directories must not be symlinks.
func kodi_output_root(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if !errors.Is(err, os.ErrNotExist) {
		return resolved, err
	}
	parent, err := kodi_output_root(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

// kodi_make_directory rejects symlink parents so an export cannot overwrite another tree.
func kodi_make_directory(path string) error {
	if parent := filepath.Dir(path); parent != path {
		if err := kodi_make_directory(parent); err != nil {
			return err
		}
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, 0755); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("目标目录不是普通目录: %s", path)
	}
	return nil
}

// kodi_write_file never replaces existing data; identical files allow safe retries.
func kodi_write_file(ctx context.Context, path string, source io.Reader) (bool, error) {
	if err := kodi_make_directory(filepath.Dir(path)); err != nil {
		return false, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, os.ErrExist) {
		info, stat_err := os.Lstat(path)
		if stat_err != nil {
			return false, stat_err
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("目标不是普通文件: %s", path)
		}
		existing, open_err := os.Open(path)
		if open_err != nil {
			return false, open_err
		}
		defer existing.Close()
		source_hash, target_hash := sha256.New(), sha256.New()
		if err := kodi_copy(ctx, source_hash, source); err != nil {
			return false, err
		}
		if err := kodi_copy(ctx, target_hash, existing); err != nil {
			return false, err
		}
		if !bytes.Equal(source_hash.Sum(nil), target_hash.Sum(nil)) {
			return false, fmt.Errorf("目标已存在且内容不同，未覆盖: %s", path)
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = kodi_copy(ctx, file, source)
	if err == nil {
		err = file.Sync()
	}
	close_err := file.Close()
	if err == nil {
		err = close_err
	}
	if err != nil {
		if remove_err := os.Remove(path); remove_err != nil {
			return true, fmt.Errorf("%w; 清理未完成文件失败: %v", err, remove_err)
		}
	}
	return err == nil, err
}

func kodi_copy(ctx context.Context, target io.Writer, source io.Reader) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := io.CopyN(target, source, 1024*1024)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
