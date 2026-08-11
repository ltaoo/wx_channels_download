package fanqienoveladapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/scraper/fanqienovel"
)

type chapter_text_extraction struct {
	text              string
	title             string
	paragraph_count   int
	encoding_rotation string
	replacements      int
}

// Postprocess extracts chapter text from downloaded Fanqie HTML and restores
// iconfont-substituted characters in the extracted body.
func (a *FanqieNovelAdapter) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	if info == nil {
		return fmt.Errorf("fanqienovel postprocess: task is nil")
	}
	converted_count := 0
	skipped_count := 0
	for resource_index := range info.Resources {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		resource := &info.Resources[resource_index]
		if !is_fanqie_html_resource(resource) {
			continue
		}
		file_path := strings.TrimSpace(resource.FilePath)
		if file_path == "" {
			return fmt.Errorf("fanqienovel postprocess: resource %d has no downloaded file", resource.ID)
		}
		html_data, err := os.ReadFile(file_path)
		if err != nil {
			return fmt.Errorf("fanqienovel postprocess: read resource %q: %w", resource.Name, err)
		}
		extraction, err := extract_chapter_text(html_data)
		if err != nil {
			return fmt.Errorf("fanqienovel postprocess: extract resource %q: %w", resource.Name, err)
		}
		if extraction.paragraph_count == 0 {
			skipped_count++
			deps.Logger.Warn().
				Int("task_id", info.ID).
				Int("resource_id", resource.ID).
				Str("resource_name", resource.Name).
				Msg("Postprocessor.fanqienovel: no chapter body found, preserving HTML")
			continue
		}
		processed_data := []byte(extraction.text)
		if err := replace_file_contents(file_path, processed_data); err != nil {
			return fmt.Errorf("fanqienovel postprocess: write resource %q: %w", resource.Name, err)
		}

		resource.Name = postprocessed_chapter_name(resource, extraction.title)
		resource.Kind = "text/plain"
		resource.Size = int64(len(processed_data))
		resource.Downloaded = resource.Size
		if resource.Extra == nil {
			resource.Extra = make(map[string]string)
		}
		resource.Extra["postprocessed"] = "true"
		resource.Extra["encoding_rotation"] = extraction.encoding_rotation
		resource.Extra["encoding_replacements"] = strconv.Itoa(extraction.replacements)
		converted_count++
		deps.Logger.Info().
			Int("task_id", info.ID).
			Int("resource_id", resource.ID).
			Str("resource_name", resource.Name).
			Str("encoding_rotation", extraction.encoding_rotation).
			Int("encoding_replacements", extraction.replacements).
			Int("paragraph_count", extraction.paragraph_count).
			Msg("Postprocessor.fanqienovel: chapter body converted to text")
	}

	deps.Logger.Info().
		Int("task_id", info.ID).
		Int("converted_count", converted_count).
		Int("preserved_html_count", skipped_count).
		Msg("Postprocessor.fanqienovel: completed")
	return nil
}

func is_fanqie_html_resource(resource *hermes.ResourceJob) bool {
	if resource == nil || strings.TrimSpace(resource.FilePath) == "" {
		return false
	}
	kind := strings.ToLower(strings.TrimSpace(resource.Kind))
	return kind == "html" || kind == "text/html" || strings.HasPrefix(kind, "text/html;")
}

func extract_chapter_text(html_data []byte) (chapter_text_extraction, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(html_data)))
	if err != nil {
		return chapter_text_extraction{}, err
	}
	title := strings.TrimSpace(document.Find(".muye-reader-title").First().Text())
	paragraphs := make([]string, 0)
	paragraph_selection := document.Find(".muye-reader-content > div > p")
	if paragraph_selection.Length() == 0 {
		paragraph_selection = document.Find(".muye-reader-content p")
	}
	paragraph_selection.Each(func(_ int, selection *goquery.Selection) {
		paragraph := strings.TrimSpace(selection.Text())
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	})
	if len(paragraphs) == 0 {
		return chapter_text_extraction{title: title}, nil
	}

	decode_result := fanqienovel.DecodeText(strings.Join(paragraphs, "\n\n"))
	text := strings.TrimSpace(decode_result.Text)
	if title != "" {
		text = title + "\n\n" + text
	}
	return chapter_text_extraction{
		text:              text + "\n",
		title:             title,
		paragraph_count:   len(paragraphs),
		encoding_rotation: decode_result.Rotation,
		replacements:      decode_result.Replacements,
	}, nil
}

func postprocessed_chapter_name(resource *hermes.ResourceJob, extracted_title string) string {
	title := strings.TrimSpace(extracted_title)
	if resource != nil && resource.Extra != nil {
		if configured_title := strings.TrimSpace(resource.Extra["chapter_title"]); configured_title != "" {
			title = configured_title
		}
		if title != "" {
			if chapter_index, err := strconv.Atoi(strings.TrimSpace(resource.Extra["chapter_index"])); err == nil && chapter_index > 0 {
				return fmt.Sprintf("%04d_%s", chapter_index, sanitize_filename(title))
			}
			return sanitize_filename(title)
		}
	}
	if title != "" {
		return sanitize_filename(title)
	}
	if resource == nil {
		return "chapter"
	}
	name := strings.TrimSpace(resource.Name)
	for {
		extension := strings.ToLower(filepath.Ext(name))
		if extension != ".html" && extension != ".htm" && extension != ".tmp" {
			break
		}
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	if name == "" {
		return "chapter"
	}
	return name
}

func replace_file_contents(file_path string, data []byte) error {
	file_info, err := os.Stat(file_path)
	if err != nil {
		return err
	}
	temporary_file, err := os.CreateTemp(filepath.Dir(file_path), ".fanqie-postprocess-*.tmp")
	if err != nil {
		return err
	}
	temporary_path := temporary_file.Name()
	defer os.Remove(temporary_path)
	if err := temporary_file.Chmod(file_info.Mode().Perm()); err != nil {
		_ = temporary_file.Close()
		return err
	}
	if _, err := temporary_file.Write(data); err != nil {
		_ = temporary_file.Close()
		return err
	}
	if err := temporary_file.Sync(); err != nil {
		_ = temporary_file.Close()
		return err
	}
	if err := temporary_file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary_path, file_path); err != nil {
		return err
	}
	return nil
}
