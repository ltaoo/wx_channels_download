package feishu

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

var (
	attribute_operation_pattern = regexp.MustCompile(`((?:\*[0-9a-z]+)*)(?:\|[0-9a-z]+)?([+=-])([0-9a-z]+)`)
	attribute_code_pattern      = regexp.MustCompile(`\*([0-9a-z]+)`)
	safe_extension_pattern      = regexp.MustCompile(`^\.[A-Za-z0-9]{1,10}$`)
	semantic_name_pattern       = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
	hex_color_pattern           = regexp.MustCompile(`^#[0-9A-Fa-f]{3,8}$`)
	function_color_pattern      = regexp.MustCompile(`^rgba?\([0-9., ]+\)$`)
)

const document_css = `:root{color-scheme:light;--text:#1f2329;--muted:#646a73;--border:#dee0e3;--blue:#3370ff}*{box-sizing:border-box}body{margin:0;background:#fff;color:var(--text);font:16px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif}.document-shell{width:min(820px,calc(100% - 48px));margin:54px auto 120px}.document-title{margin:0 0 36px;font-size:36px;line-height:1.3}h1,h2,h3,h4,h5,h6{margin:1.7em 0 .65em;line-height:1.35}p{margin:8px 0;white-space:pre-wrap;overflow-wrap:anywhere}a{color:var(--blue);text-decoration:none}.rich-highlight{padding:0 2px;border-radius:2px;background:var(--highlight-color,#fff1b8)}.rich-emphasis{color:#d83931}.rich-comment{text-decoration:underline 2px #3370ff}.inline-code{padding:2px 5px;border-radius:4px;background:#f2f3f5;font:.9em ui-monospace,monospace}blockquote{margin:16px 0;padding-left:18px;border-left:4px solid #bbbfc4}.callout{display:flex;gap:12px;margin:16px 0;padding:14px 16px;border:1px solid #fed4a4;border-radius:8px;background:#fff5eb}ul,ol{margin:8px 0;padding-left:28px}.todo-list{list-style:none}.folded-content{margin-left:18px}.code-block{overflow:auto;padding:16px;border-radius:8px;background:#f5f6f7}.grid-block{display:grid;grid-template-columns:var(--grid-template,repeat(var(--grid-columns),minmax(0,1fr)));gap:18px;margin:14px 0}.table-scroll{max-width:100%;margin:16px 0;overflow-x:auto}.table-block{width:100%;border-collapse:collapse;table-layout:fixed}.table-block th,.table-block td{padding:9px 12px;border:1px solid var(--border);vertical-align:top}.bookmark,.file-card{display:flex;max-width:560px;gap:12px;margin:12px 0;padding:12px 14px;border:1px solid var(--border);border-radius:8px;color:inherit}.bookmark{display:block}.bookmark-summary,.file-meta{display:block;color:var(--muted);font-size:13px}.file-icon{font-size:24px}.view-block{margin:16px 0;border:1px solid var(--border);border-radius:8px;background:#f5f6f7}.view-header{display:flex;justify-content:space-between;padding:9px 12px;background:#fff}.view-placeholder{padding:28px 16px;color:var(--muted);text-align:center}.image-block{max-width:100%;margin:18px 0}.image-content,.inline-image{max-width:100%;height:auto}.unsupported{display:inline-block;padding:2px 7px;border-radius:4px;background:#f2f3f5;color:var(--muted)}.align-center{text-align:center}.align-right{text-align:right}.align-justify{text-align:justify}@media(max-width:700px){.document-shell{width:calc(100% - 28px);margin-top:28px}.grid-block{grid-template-columns:1fr}}`

type rich_text_data struct {
	APool struct {
		NumToAttrib map[string][]json.RawMessage `json:"numToAttrib"`
	} `json:"apool"`
	InitialAttributedTexts struct {
		Text    map[string]string `json:"text"`
		Attribs map[string]string `json:"attribs"`
	} `json:"initialAttributedTexts"`
}

type html_renderer struct {
	blocks map[string]block_data
	assets map[string]Asset
	active map[string]bool
}

func render_document_html(document *Document) string {
	if document == nil {
		return ""
	}
	assets := make(map[string]Asset, len(document.Assets))
	for _, asset := range document.Assets {
		assets[asset.Token] = asset
	}
	renderer := &html_renderer{blocks: document.blocks, assets: assets, active: make(map[string]bool)}
	content := renderer.render_block(document.root_id, false)
	if content == "" {
		content = `<article data-n="document-page" class="document-page"><header data-n="document-header"><h1 data-n="document-title" class="document-title">` + html.EscapeString(document.Title) + `</h1></header><p data-n="document-fallback-text">` + html.EscapeString(document.Text) + `</p></article>`
	}
	return `<!doctype html><html data-n="document-html" lang="zh-CN"><head data-n="document-head"><meta data-n="document-charset" charset="utf-8"><meta data-n="document-viewport" name="viewport" content="width=device-width,initial-scale=1"><title data-n="document-browser-title">` + html.EscapeString(document.Title) + `</title><style data-n="document-styles">` + document_css + `</style></head><body data-n="document-body"><main data-n="document-main" class="document-shell">` + content + `</main></body></html>` + "\n"
}

func (r *html_renderer) render_block(block_id string, inline bool) string {
	block, exists := r.blocks[block_id]
	if !exists {
		return unsupported_html("block")
	}
	if block.Hidden {
		return ""
	}
	if r.active[block_id] {
		return unsupported_html(block.Type)
	}
	if block.Type == "bullet" || block.Type == "ordered" || block.Type == "todo" {
		tag := "ul"
		if block.Type == "ordered" {
			tag = "ol"
		}
		return `<` + tag + ` data-n="` + semantic_name(block.Type) + `-list">` + r.render_list_item(block_id) + `</` + tag + `>`
	}

	r.active[block_id] = true
	defer delete(r.active, block_id)
	block_attr := html.EscapeString(block_id)
	text := r.rich_text(block.Text)
	align_class := ""
	if block.Align == "center" || block.Align == "right" || block.Align == "justify" {
		align_class = " align-" + block.Align
	}
	switch block.Type {
	case "page":
		return `<article data-n="document-page" class="document-page" data-block-id="` + block_attr + `"><header data-n="document-header"><h1 data-n="document-title" class="document-title">` + text + `</h1></header>` + r.render_children(block.Children) + `</article>`
	case "text":
		paragraph := ""
		if text != "" {
			paragraph = `<p data-n="text-block" class="text-block` + align_class + `" data-block-id="` + block_attr + `">` + text + `</p>`
		}
		return paragraph + r.render_children(block.Children)
	case "heading1", "heading2", "heading3", "heading4", "heading5", "heading6":
		level := strings.TrimPrefix(block.Type, "heading")
		return `<h` + level + ` data-n="` + block.Type + `-block" class="heading-block` + align_class + `" data-block-id="` + block_attr + `">` + text + `</h` + level + `>` + r.render_children(block.Children)
	case "code":
		return `<pre data-n="code-block" class="code-block" data-block-id="` + block_attr + `"><code data-n="code-content" class="code-content language-` + semantic_name(block.Language) + `">` + html.EscapeString(block_text(block)) + `</code></pre>` + r.render_children(block.Children)
	case "quote_container":
		return `<blockquote data-n="quote-block" data-block-id="` + block_attr + `">` + r.render_children(block.Children) + `</blockquote>`
	case "callout":
		icon := "💡"
		if block.EmojiID == "+1" {
			icon = "👍"
		}
		return `<aside data-n="callout-block" class="callout" data-block-id="` + block_attr + `"><span data-n="callout-icon">` + icon + `</span><div data-n="callout-body">` + r.render_children(block.Children) + `</div></aside>`
	case "grid":
		ratios := make([]string, 0, len(block.Children))
		for _, child_id := range block.Children {
			if ratio := r.blocks[child_id].WidthRatio; ratio > 0 && ratio <= 1 {
				ratios = append(ratios, strconv.FormatFloat(ratio, 'g', 12, 64)+"fr")
			}
		}
		style := `--grid-columns:` + strconv.Itoa(maximum(1, len(block.Children)))
		if len(ratios) == len(block.Children) && len(ratios) > 0 {
			style += `;--grid-template:` + strings.Join(ratios, " ")
		}
		return `<section data-n="grid-block" class="grid-block" style="` + style + `" data-block-id="` + block_attr + `">` + r.render_children(block.Children) + `</section>`
	case "grid_column":
		return `<div data-n="grid-column" class="grid-column" data-block-id="` + block_attr + `">` + r.render_children(block.Children) + `</div>`
	case "table":
		return r.render_table(block_id, block)
	case "table_cell":
		return `<div data-n="table-cell-content" data-block-id="` + block_attr + `">` + r.render_children(block.Children) + `</div>`
	case "image":
		return r.render_image(block_id, block, inline)
	case "file":
		return r.render_file(block_id, block, inline)
	case "bookmark":
		if safe_url := safe_http_url(block.Bookmark.URL); safe_url != "#" {
			title := first_non_empty(block.Bookmark.Title, block.Bookmark.URL)
			summary := ""
			if block.Bookmark.Summary != "" {
				summary = `<span data-n="bookmark-summary" class="bookmark-summary">` + html.EscapeString(block.Bookmark.Summary) + `</span>`
			}
			return `<a data-n="bookmark-card" class="bookmark" href="` + safe_url + `" data-block-id="` + block_attr + `"><strong data-n="bookmark-title">` + html.EscapeString(title) + `</strong>` + summary + `</a>`
		}
		return unsupported_html("bookmark")
	case "view":
		if len(block.Children) > 0 {
			child := r.blocks[block.Children[0]]
			if child.Type == "file" {
				return r.render_file_view(block_id, child)
			}
		}
		return `<section data-n="view-fallback" class="view-block" data-block-id="` + block_attr + `">` + r.render_children(block.Children) + `</section>`
	case "divider":
		return `<hr data-n="divider-block" data-block-id="` + block_attr + `>`
	case "synced_source":
		return `<section data-n="synced-source" data-block-id="` + block_attr + `">` + r.render_children(block.Children) + `</section>`
	default:
		return unsupported_html(block.Type)
	}
}

func (r *html_renderer) render_children(children []string) string {
	var result strings.Builder
	for child_index := 0; child_index < len(children); {
		block_type := r.blocks[children[child_index]].Type
		if block_type == "bullet" || block_type == "ordered" || block_type == "todo" {
			tag := "ul"
			if block_type == "ordered" {
				tag = "ol"
			}
			result.WriteString(`<` + tag + ` data-n="` + semantic_name(block_type) + `-list">`)
			for child_index < len(children) && r.blocks[children[child_index]].Type == block_type {
				result.WriteString(r.render_list_item(children[child_index]))
				child_index++
			}
			result.WriteString(`</` + tag + `>`)
			continue
		}
		result.WriteString(r.render_block(children[child_index], false))
		child_index++
	}
	return result.String()
}

func (r *html_renderer) render_list_item(block_id string) string {
	block, exists := r.blocks[block_id]
	if !exists || r.active[block_id] {
		return `<li data-n="cyclic-list-item">` + unsupported_html("list-item") + `</li>`
	}
	r.active[block_id] = true
	defer delete(r.active, block_id)
	content := `<div data-n="list-item-content">` + r.rich_text(block.Text) + `</div>`
	if block.Type == "todo" {
		checked := ""
		if block.Done {
			checked = " checked"
		}
		content = `<div data-n="todo-item-content"><input data-n="todo-checkbox" type="checkbox" disabled` + checked + `><span data-n="todo-label">` + r.rich_text(block.Text) + `</span></div>`
	}
	body := r.render_children(block.Children)
	if block.Folded && body != "" {
		return `<li data-n="` + semantic_name(block.Type) + `-item" class="folded-item" data-block-id="` + html.EscapeString(block_id) + `"><details data-n="folded-details"><summary data-n="folded-summary">` + content + `</summary><div data-n="folded-content" class="folded-content">` + body + `</div></details></li>`
	}
	return `<li data-n="` + semantic_name(block.Type) + `-item" data-block-id="` + html.EscapeString(block_id) + `">` + content + body + `</li>`
}

func (r *html_renderer) render_image(block_id string, block block_data, inline bool) string {
	asset, exists := r.assets[block.Image.Token]
	if !exists || asset.RelativePath == "" {
		return unsupported_html("image")
	}
	name := html.EscapeString(first_non_empty(asset.Name, "Feishu image"))
	dimensions := ""
	if asset.Width > 0 && asset.Height > 0 {
		dimensions = ` width="` + strconv.Itoa(asset.Width) + `" height="` + strconv.Itoa(asset.Height) + `"`
	}
	class_name := "image-content"
	semantic := "block-image"
	if inline {
		class_name = "inline-image"
		semantic = "inline-image"
	}
	image_tag := `<img data-n="` + semantic + `" class="` + class_name + `" src="` + html.EscapeString(asset.RelativePath) + `" alt="` + name + `" loading="lazy" decoding="async"` + dimensions + `>`
	if inline {
		return image_tag
	}
	return `<figure data-n="image-block" class="image-block" data-block-id="` + html.EscapeString(block_id) + `">` + image_tag + `</figure>`
}

func (r *html_renderer) render_file(block_id string, block block_data, inline bool) string {
	asset, exists := r.assets[block.File.Token]
	if !exists || asset.RelativePath == "" {
		return unsupported_html("file")
	}
	path := html.EscapeString(asset.RelativePath)
	name := html.EscapeString(first_non_empty(asset.Name, "Feishu file"))
	if inline {
		return `<a data-n="inline-file-link" class="file-link" href="` + path + `" download title="` + name + ` · 独立下载资源"><span data-n="inline-file-icon">📎</span><span data-n="inline-file-name">` + name + `</span></a>`
	}
	return `<a data-n="file-card" class="file-card" href="` + path + `" download data-block-id="` + html.EscapeString(block_id) + `"><span data-n="file-icon" class="file-icon">📎</span><span data-n="file-details"><strong data-n="file-name">` + name + `</strong><span data-n="file-meta" class="file-meta">` + readable_size(asset.Size) + ` · 独立下载资源</span></span></a>`
}

func (r *html_renderer) render_file_view(block_id string, file_block block_data) string {
	asset, exists := r.assets[file_block.File.Token]
	if !exists || asset.RelativePath == "" {
		return unsupported_html("view")
	}
	name := html.EscapeString(first_non_empty(asset.Name, "Feishu file"))
	path := html.EscapeString(asset.RelativePath)
	label := "文件作为独立资源下载"
	if asset.MIMEType == "application/pdf" {
		label = "PDF 作为独立资源下载"
	} else if strings.HasPrefix(asset.MIMEType, "video/") {
		label = "视频作为独立资源下载"
	}
	return `<section data-n="view-block" class="view-block" data-block-id="` + html.EscapeString(block_id) + `"><header data-n="view-header" class="view-header"><strong data-n="view-title">` + name + `</strong><a data-n="view-address-link" href="` + path + `" download>下载文件</a></header><div data-n="file-address-placeholder" class="view-placeholder">` + label + `</div></section>`
}

func (r *html_renderer) render_table(block_id string, block block_data) string {
	if len(block.RowsID) == 0 || len(block.ColumnsID) == 0 {
		return unsupported_html("table")
	}
	var columns strings.Builder
	for _, column_id := range block.ColumnsID {
		style := ""
		if width := block.ColumnSet[column_id].ColumnWidth; width >= 20 && width <= 3000 {
			style = ` style="width:` + strconv.Itoa(int(width)) + `px"`
		}
		columns.WriteString(`<col data-n="table-column"` + style + `>`)
	}
	covered := make(map[[2]int]bool)
	var rows strings.Builder
	for row_index, row_id := range block.RowsID {
		rows.WriteString(`<tr data-n="table-row">`)
		for column_index, column_id := range block.ColumnsID {
			if covered[[2]int{row_index, column_index}] {
				continue
			}
			cell := block.CellSet[row_id+column_id]
			row_span := maximum(1, cell.MergeInfo.RowSpan)
			column_span := maximum(1, cell.MergeInfo.ColSpan)
			row_span = minimum(row_span, len(block.RowsID)-row_index)
			column_span = minimum(column_span, len(block.ColumnsID)-column_index)
			for covered_row := row_index; covered_row < row_index+row_span; covered_row++ {
				for covered_column := column_index; covered_column < column_index+column_span; covered_column++ {
					if covered_row != row_index || covered_column != column_index {
						covered[[2]int{covered_row, covered_column}] = true
					}
				}
			}
			tag := "td"
			if row_index == 0 && block.HeaderRow || column_index == 0 && block.HeaderColumn {
				tag = "th"
			}
			spans := ""
			if row_span > 1 {
				spans += ` rowspan="` + strconv.Itoa(row_span) + `"`
			}
			if column_span > 1 {
				spans += ` colspan="` + strconv.Itoa(column_span) + `"`
			}
			content := unsupported_html("table-cell")
			if cell.BlockID != "" {
				content = r.render_block(cell.BlockID, false)
			}
			rows.WriteString(`<` + tag + ` data-n="table-cell"` + spans + `>` + content + `</` + tag + `>`)
		}
		rows.WriteString(`</tr>`)
	}
	return `<div data-n="table-scroll-container" class="table-scroll" data-block-id="` + html.EscapeString(block_id) + `"><table data-n="table-block" class="table-block"><colgroup data-n="table-columns">` + columns.String() + `</colgroup><tbody data-n="table-body">` + rows.String() + `</tbody></table></div>`
}

func (r *html_renderer) rich_text(text_data rich_text_data) string {
	texts := text_data.InitialAttributedTexts.Text
	if len(texts) == 0 {
		return ""
	}
	keys := ordered_text_keys(texts)
	var result strings.Builder
	for _, key := range keys {
		raw_text := texts[key]
		changes := text_data.InitialAttributedTexts.Attribs[key]
		if changes == "" {
			result.WriteString(html.EscapeString(raw_text))
			continue
		}
		encoded := utf16.Encode([]rune(raw_text))
		offset := 0
		for _, match := range attribute_operation_pattern.FindAllStringSubmatch(changes, -1) {
			length, err := strconv.ParseInt(match[3], 36, 32)
			if err != nil || length < 0 {
				continue
			}
			end := minimum(len(encoded), offset+int(length))
			segment := string(utf16.Decode(encoded[offset:end]))
			offset = end
			if match[2] == "-" {
				continue
			}
			attributes := attribute_map(text_data.APool.NumToAttrib, match[1])
			result.WriteString(r.render_rich_segment(segment, attributes))
		}
		if offset < len(encoded) {
			result.WriteString(html.EscapeString(string(utf16.Decode(encoded[offset:]))))
		}
	}
	return result.String()
}

func (r *html_renderer) render_rich_segment(raw_text string, attributes map[string]string) string {
	segment := html.EscapeString(raw_text)
	if block_id := attributes["inlineblock"]; block_id != "" {
		segment = r.render_block(block_id, true)
	} else if component_text := render_inline_component(attributes["inline-component"]); component_text != "" {
		segment = component_text
	} else if link := attributes["link"]; link != "" && raw_text != "" {
		segment = `<a data-n="rich-text-link" href="` + safe_http_url(link) + `">` + segment + `</a>`
	}
	if _, exists := attributes["inlineCode"]; exists {
		segment = `<code data-n="inline-code" class="inline-code">` + segment + `</code>`
	}
	if _, exists := attributes["bold"]; exists {
		segment = `<strong data-n="bold-text">` + segment + `</strong>`
	}
	if _, exists := attributes["underline"]; exists {
		segment = `<u data-n="underlined-text">` + segment + `</u>`
	}
	if color := safe_css_color(attributes["textHighlightBackground"]); color != "" {
		segment = `<span data-n="highlighted-text" class="rich-highlight" style="--highlight-color:` + color + `">` + segment + `</span>`
	}
	if _, exists := attributes["textHighlight"]; exists {
		segment = `<span data-n="emphasized-text" class="rich-emphasis">` + segment + `</span>`
	}
	comment_ids := make([]string, 0)
	for name, value := range attributes {
		if strings.HasPrefix(name, "comment-id-") && strings.EqualFold(value, "true") {
			comment_ids = append(comment_ids, strings.TrimPrefix(name, "comment-id-"))
		}
	}
	if len(comment_ids) > 0 {
		sort.Strings(comment_ids)
		segment = `<span data-n="commented-text" class="rich-comment" data-comment-ids="` + html.EscapeString(strings.Join(comment_ids, ",")) + `">` + segment + `</span>`
	}
	return segment
}

func attribute_map(pool map[string][]json.RawMessage, operations string) map[string]string {
	attributes := make(map[string]string)
	for _, code_match := range attribute_code_pattern.FindAllStringSubmatch(operations, -1) {
		pair := pool[code_match[1]]
		if len(pair) != 2 {
			continue
		}
		var name string
		if json.Unmarshal(pair[0], &name) != nil || name == "" {
			continue
		}
		var value any
		if json.Unmarshal(pair[1], &value) != nil {
			continue
		}
		switch typed_value := value.(type) {
		case string:
			attributes[name] = typed_value
		case bool:
			attributes[name] = strconv.FormatBool(typed_value)
		case float64:
			attributes[name] = strconv.FormatFloat(typed_value, 'g', -1, 64)
		default:
			encoded, _ := json.Marshal(typed_value)
			attributes[name] = string(encoded)
		}
	}
	return attributes
}

func render_inline_component(raw_component string) string {
	if raw_component == "" {
		return ""
	}
	var component struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	if json.Unmarshal([]byte(raw_component), &component) != nil {
		return unsupported_html("inline-component")
	}
	if component.Type == "mention_doc" {
		raw_url, _ := component.Data["raw_url"].(string)
		title, _ := component.Data["title"].(string)
		if raw_url != "" && title != "" {
			return `<a data-n="mentioned-document-link" href="` + safe_http_url(raw_url) + `">` + html.EscapeString(title) + `</a>`
		}
	}
	if component.Type == "reminder" {
		expire_time, ok := component.Data["expire_time"].(float64)
		if ok {
			reminder_time := time.UnixMilli(int64(expire_time)).In(time.FixedZone("CST", 8*60*60))
			value := fmt.Sprintf("%d年%d月%d日", reminder_time.Year(), reminder_time.Month(), reminder_time.Day())
			if whole_day, _ := component.Data["is_whole_day"].(bool); !whole_day {
				value += reminder_time.Format(" 15:04")
			}
			return html.EscapeString(value)
		}
	}
	return unsupported_html(first_non_empty(component.Type, "inline-component"))
}

func ordered_text_keys(texts map[string]string) []string {
	keys := make([]string, 0, len(texts))
	for key := range texts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left_index int, right_index int) bool {
		left, left_err := strconv.ParseInt(keys[left_index], 36, 64)
		right, right_err := strconv.ParseInt(keys[right_index], 36, 64)
		if left_err == nil && right_err == nil {
			return left < right
		}
		return keys[left_index] < keys[right_index]
	})
	return keys
}

func asset_relative_path(kind string, token string, name string, mime_type string) string {
	directory := "files"
	if kind == "image" {
		directory = "images"
	}
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
	if kind == "image" || !safe_extension_pattern.MatchString(extension) {
		extension = extension_for_mime(mime_type)
	}
	if extension == "" {
		extension = ".bin"
	}
	return directory + "/" + token + extension
}

func extension_for_mime(mime_type string) string {
	switch strings.ToLower(strings.TrimSpace(mime_type)) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4":
		return ".m4a"
	case "text/csv":
		return ".csv"
	default:
		return ""
	}
}

func safe_http_url(raw_url string) string {
	decoded_url, err := url.PathUnescape(strings.TrimSpace(raw_url))
	if err != nil {
		decoded_url = strings.TrimSpace(raw_url)
	}
	parsed_url, err := url.Parse(decoded_url)
	if err != nil || parsed_url.Scheme != "http" && parsed_url.Scheme != "https" && parsed_url.Scheme != "mailto" {
		return "#"
	}
	return html.EscapeString(parsed_url.String())
}

func safe_css_color(value string) string {
	value = strings.TrimSpace(value)
	if hex_color_pattern.MatchString(value) || function_color_pattern.MatchString(value) {
		return value
	}
	return ""
}

func semantic_name(value string) string {
	value = semantic_name_pattern.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "content"
	}
	return value
}

func unsupported_html(block_type string) string {
	label := semantic_name(block_type)
	return `<span data-n="unsupported-` + label + `" class="unsupported">` + html.EscapeString("<"+label+" not supported>") + `</span>`
}

func readable_size(size int64) string {
	value := float64(size)
	for _, unit := range []string{"B", "KB", "MB", "GB"} {
		if value < 1024 || unit == "GB" {
			if unit == "B" {
				return strconv.FormatInt(size, 10) + " B"
			}
			return strconv.FormatFloat(value, 'f', 1, 64) + " " + unit
		}
		value /= 1024
	}
	return "0 B"
}

func minimum(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maximum(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
