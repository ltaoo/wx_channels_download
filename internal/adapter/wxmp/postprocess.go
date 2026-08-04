package wxmp

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	"wx_channel/internal/pipeline"
	"wx_channel/pkg/hermes"
)

// Postprocess assembles downloaded wxmp resources into the final HTML file.
func (h *handler) Postprocess(ctx context.Context, info *hermes.TaskJob, deps registry.PostprocessDeps) error {
	log := func(msg string, args ...interface{}) {
		deps.Logger.Info().Msg(fmt.Sprintf(msg, args...))
	}
	taskID := info.ID
	var meta struct {
		ExternalID string `json:"external_id"`
		BizType    int    `json:"biz_type"`
	}
	if info.Metadata != nil {
		meta.ExternalID, _ = info.Metadata["external_id"].(string)
		if bt, ok := info.Metadata["biz_type"].(float64); ok {
			meta.BizType = int(bt)
		}
	}
	if meta.ExternalID == "" && info.Metadata != nil {
		m, _ := json.Marshal(info.Metadata)
		_ = json.Unmarshal(m, &meta)
	}

	contentID := BuildContentID(meta.ExternalID)
	var article model.ContentArticle
	if deps.DB == nil {
		return fmt.Errorf("wxmp postprocess: database is nil")
	}
	if err := deps.DB.Where("id = ?", contentID).First(&article).Error; err != nil {
		log("Postprocessor.wxmp: task_id=%d ContentArticle not found: %v, skipping assembly", taskID, err)
		return nil
	}
	if article.HTML == "" {
		return nil
	}

	pc := pipeline.NewContext()
	pc.Values["content_html"] = article.HTML
	pc.Values["save_path"] = deps.BasePath
	pc.Values["task_id"] = taskID
	pc.Values["task_name"] = info.Name
	pc.Values["biz_type"] = meta.BizType
	pc.Values["db"] = deps.DB
	pc.Values["logger"] = deps.Logger
	for _, r := range info.Resources {
		if r.Kind == "html" && r.FilePath != "" {
			pc.Values["output_file"] = r.FilePath
			break
		}
	}

	p := pipeline.NewBuilder("wxmp_postprocess").Add("assemble_html", AssembleHTMLNode).Build()
	if _, err := p.Run(ctx, pc); err != nil {
		return err
	}

	embeddedNames, _ := pc.Values["embedded_resource_names"].(map[string]bool)
	keptResources := make([]hermes.ResourceJob, 0, len(info.Resources))
	for _, r := range info.Resources {
		if r.Kind == "image" && embeddedNames[r.Name] {
			if err := os.Remove(r.FilePath); err != nil && !os.IsNotExist(err) {
				log("Postprocessor.wxmp: task_id=%d remove image %s: %v", taskID, r.FilePath, err)
			}
			if err := deps.DB.Where("id = ? AND task_id = ?", r.ID, taskID).Delete(&model.DownloadResource{}).Error; err != nil {
				log("Postprocessor.wxmp: task_id=%d remove image record id=%d: %v", taskID, r.ID, err)
			}
			continue
		}
		keptResources = append(keptResources, r)
	}
	info.Resources = keptResources
	return nil
}

// AssembleHTMLNode parses content HTML, replaces image URLs with local downloaded filenames,
// and wraps it as a complete HTML document. For album type (biz_type=2), generates gallery
// HTML directly from downloaded image resources.
var AssembleHTMLNode = pipeline.NewFuncNode("assemble_html", "assemble_html", func(ctx context.Context, pc *pipeline.Context) error {
	savePath, _ := pc.Values["save_path"].(string)
	if savePath == "" {
		return fmt.Errorf("缺少 save_path")
	}
	taskName, _ := pc.Values["task_name"].(string)
	if taskName == "" {
		taskName = "article"
	}
	taskID, _ := pc.Values["task_id"].(int)
	bizType, _ := pc.Values["biz_type"].(int)

	log, _ := pc.Values["logger"].(zerolog.Logger)
	infof := func(msg string, args ...interface{}) {
		log.Info().Int("task_id", taskID).Msg(fmt.Sprintf(msg, args...))
	}
	warnf := func(msg string, args ...interface{}) {
		log.Warn().Int("task_id", taskID).Msg(fmt.Sprintf(msg, args...))
	}

	infof("assemble_html: savePath=%q taskName=%q bizType=%d", savePath, taskName, bizType)

	var bodyHTML string

	if bizType == 2 {
		// Album: generate gallery HTML from downloaded image resources
		db, _ := pc.Values["db"].(*gorm.DB)
		if db == nil {
			return fmt.Errorf("缺少 db")
		}

		var imageResources []model.DownloadResource
		if err := db.Where("task_id = ? AND kind = ?", taskID, "image").
			Order("merge_order ASC").Find(&imageResources).Error; err != nil {
			return fmt.Errorf("加载图片资源失败: %w", err)
		}
		infof("assemble_html: album mode, loaded %d image resources", len(imageResources))

		var galleryBuilder strings.Builder
		contentHTML, _ := pc.Values["content_html"].(string)
		if contentHTML != "" {
			galleryBuilder.WriteString(`<div class="album_desc">`)
			galleryBuilder.WriteString(contentHTML)
			galleryBuilder.WriteString(`</div>`)
		}
		galleryBuilder.WriteString(`<div class="album_gallery">`)
		for _, res := range imageResources {
			galleryBuilder.WriteString(fmt.Sprintf(`<img src="%s" alt="">`, res.Name))
		}
		galleryBuilder.WriteString(`</div>`)
		bodyHTML = galleryBuilder.String()
	} else {
		// Article: parse content HTML, replace image URLs with local filenames
		contentHTML, _ := pc.Values["content_html"].(string)
		if contentHTML == "" {
			return fmt.Errorf("缺少 content_html")
		}
		db, _ := pc.Values["db"].(*gorm.DB)
		if db == nil {
			return fmt.Errorf("缺少 db")
		}

		// Load image resources from DB to get hermes-assigned extensions
		var imageResources []model.DownloadResource
		if err := db.Where("task_id = ? AND kind = ?", taskID, "image").
			Order("merge_order ASC").Find(&imageResources).Error; err != nil {
			return fmt.Errorf("加载图片资源失败: %w", err)
		}
		infof("assemble_html: article mode, loaded %d image resources from DB (kind=\"image\")", len(imageResources))
		for i, res := range imageResources {
			infof("assemble_html: DB image[%d] id=%d name=%q merge_order=%d", i, res.Id, res.Name, res.MergeOrder)
		}

		// Build map: MD5 hash → resource.Name (strip directory prefix and template suffix)
		// Original resource name is just the MD5 hash (e.g. "644ac3fe..."). After filenameTemplate
		// (e.g. "{{author}}/{{filename}}_{{spec}}"), the name becomes "author/hash_.jpg".
		// We need to extract just the hash part to match against image URLs.
		md5ToName := make(map[string]string, len(imageResources))
		for _, res := range imageResources {
			filename := filepath.Base(res.Name)                     // "hash_.jpg"
			ext := filepath.Ext(filename)                           // ".jpg"
			filenameWithoutExt := strings.TrimSuffix(filename, ext) // "hash_"
			// Strip trailing "_" from filenameTemplate pattern ({{filename}}_{{spec}})
			filenameWithoutExt = strings.TrimSuffix(filenameWithoutExt, "_") // "hash"
			md5ToName[filenameWithoutExt] = res.Name
			infof("assemble_html: md5ToName[%q] = %q (from DB name=%q)", filenameWithoutExt, res.Name, res.Name)
		}

		infof("assemble_html: md5ToName map built with %d entries", len(md5ToName))

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(contentHTML))
		if err != nil {
			return fmt.Errorf("解析 content HTML 失败: %w", err)
		}

		embeddedNames := make(map[string]bool) // resource names embedded as base64
		totalImgs := 0
		matchedImgs := 0
		dataURIImgs := 0
		filenameFallbackImgs := 0
		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			totalImgs++
			imgURL := s.AttrOr("data-src", "")
			srcAttr := "data-src"
			if imgURL == "" {
				imgURL = s.AttrOr("src", "")
				srcAttr = "src"
			}
			imgURL = normalizeImageURL(imgURL)
			if imgURL == "" {
				warnf("assemble_html: img[%d] no valid URL (both data-src and src are empty)", i)
				return
			}

			hash := md5.Sum([]byte(imgURL))
			hashStr := hex.EncodeToString(hash[:])
			infof("assemble_html: img[%d] URL=%q attr=%s → hash=%s", i, imgURL, srcAttr, hashStr)

			if name, ok := md5ToName[hashStr]; ok {
				matchedImgs++
				filePath := filepath.Join(savePath, name)
				dataURI := imageToDataURI(filePath)

				if dataURI != "" {
					s.SetAttr("src", dataURI)
					dataURIImgs++
					embeddedNames[name] = true
					infof("assemble_html: img[%d] ✓ base64 inline success (file=%s size=%d)",
						i, name, len(dataURI))
				} else {
					s.SetAttr("src", name)
					filenameFallbackImgs++
					warnf("assemble_html: img[%d] ✗ base64 read failed, falling back to filename (file=%s path=%s)",
						i, name, filePath)
				}
				s.RemoveAttr("data-src")
			} else {
				warnf("assemble_html: img[%d] ✗ no local resource matched (URL=%q hash=%q, md5ToName keys=%v)",
					i, imgURL, hashStr, mapKeys(md5ToName))
			}
		})

		infof("assemble_html: stats total_imgs=%d matched=%d data_uri=%d filename_fallback=%d unmatched=%d embedded_names=%v",
			totalImgs, matchedImgs, dataURIImgs, filenameFallbackImgs, totalImgs-matchedImgs, mapKeysBool(embeddedNames))

		pc.Values["embedded_resource_names"] = embeddedNames

		bodyHTML, err = doc.Find("body").Html()
		if err != nil {
			bodyHTML = contentHTML
		}
	}

	// Wrap in full HTML document
	var fullHTML string
	if bizType == 2 {
		fullHTML = assembleAlbumFullHTML(taskName, bodyHTML)
	} else {
		fullHTML = AssembleFullHTML(taskName, bodyHTML)
	}

	// Ensure output file has .html extension.
	// Sanitize task name to replace "/" which can appear in titles (e.g. "zh-CN/zh-TW")
	// and would break filepath.Join by creating unintended subdirectories.
	var outputPath string
	if outFile, _ := pc.Values["output_file"].(string); outFile != "" {
		outputPath = outFile
	} else {
		safeName := strings.ReplaceAll(taskName, "/", "-")
		if !strings.HasSuffix(safeName, ".html") {
			safeName += ".html"
		}
		outputPath = filepath.Join(savePath, safeName)
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(fullHTML), 0644); err != nil {
		return fmt.Errorf("写入 HTML 文件失败: %w", err)
	}

	pc.Values["final_html"] = outputPath
	return nil
})

// AssembleFullHTML wraps body HTML into a complete standalone HTML document.
func AssembleFullHTML(title, bodyHTML string) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>`)
	b.WriteString(escapeHTMLStr(title))
	b.WriteString(`</title>
    <style>
        html { height: 100%; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            max-width: 677px;
            margin: 0 auto;
            padding: 20px;
            color: #333;
        }
        h1 { font-size: 1.8em; margin-bottom: 0.5em; }
        img { max-width: 100%; height: auto; }
    </style>
</head>
<body>`)
	b.WriteString(bodyHTML)
	b.WriteString(`</body>
</html>`)
	return b.String()
}

// imageToDataURI reads an image file and returns a base64 data URI.
// Returns empty string on any error (caller falls back to filename).
func imageToDataURI(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	mime := mimeByExtension(filepath.Ext(path))
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// mimeByExtension maps file extensions to MIME types.
func mimeByExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/jpeg"
	}
}

// mapKeys returns the keys of a map[string]string as a slice for debug logging.
func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// mapKeysBool returns the keys of a map[string]bool as a slice for debug logging.
func mapKeysBool(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func escapeHTMLStr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// assembleAlbumFullHTML wraps album body HTML into a complete HTML document
// using a centered gallery layout.
func assembleAlbumFullHTML(title, bodyHTML string) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>`)
	b.WriteString(escapeHTMLStr(title))
	b.WriteString(`</title>
    <style>
        html { height: 100%; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            max-width: 1024px;
            margin: 0 auto;
            padding: 20px;
            color: #333;
        }
        h1 { font-size: 1.8em; margin-bottom: 0.5em; }
        img { max-width: 100%; height: auto; display: block; margin-bottom: 20px; border-radius: 6px; }
        .album_desc { font-size: 1.1em; margin-bottom: 30px; color: #666; text-align: center; }
        .album_gallery { max-width: 677px; margin: 0 auto; }
    </style>
</head>
<body>`)
	b.WriteString(bodyHTML)
	b.WriteString(`</body>
</html>`)
	return b.String()
}
