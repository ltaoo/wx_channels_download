package novelpdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signintech/gopdf"
)

const (
	marginLeft   = 60.0
	marginRight  = 60.0
	marginTop    = 70.0
	marginBottom = 70.0
	fontSize     = 12.0
	titleSize    = 18.0
	metaSize     = 11.0
	headerSize   = 9.0
	lineSpacing  = 8.0
)

var (
	pageW = gopdf.PageSizeA4.W
	pageH = gopdf.PageSizeA4.H
	textW = pageW - marginLeft - marginRight
)

type Chapter struct {
	Title   string
	Content string
}

type Novel struct {
	Title       string
	Author      string
	Category    string
	Status      string
	Description string
	CoverImage  string
	Chapters    []Chapter
}

type Options struct {
	FontPath string
}

type writer struct {
	pdf     *gopdf.GoPdf
	title   string
	chTitle string
	pageNum int
	lineH   float64
}

func Export(path string, novel *Novel, opts Options) error {
	if novel == nil {
		return fmt.Errorf("novel is nil")
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("pdf output path is empty")
	}
	if len(novel.Chapters) == 0 {
		return fmt.Errorf("novel has no chapters")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})

	fontPath := opts.FontPath
	if fontPath == "" {
		fontPath = defaultFontPath()
	}
	if fontPath == "" {
		return fmt.Errorf("no usable CJK font found")
	}
	fontData, err := os.ReadFile(fontPath)
	if err != nil {
		return fmt.Errorf("read font %s: %w", fontPath, err)
	}
	if err := pdf.AddTTFFontData("novel", fontData); err != nil {
		return fmt.Errorf("add font %s: %w", fontPath, err)
	}

	if novel.CoverImage != "" {
		if info, err := os.Stat(novel.CoverImage); err == nil && !info.IsDir() {
			pdf.AddPage()
			_ = pdf.Image(novel.CoverImage, 0, 0, &gopdf.Rect{W: pageW, H: pageH})
		}
	}

	w := &writer{pdf: pdf, title: firstNonEmpty(novel.Title, "novel")}
	pdf.SetFont("novel", "", fontSize)
	_, h, _ := pdf.IsFitMultiCell(&gopdf.Rect{W: textW, H: 1000}, "测")
	w.lineH = h
	w.writeTitlePage(novel)
	for _, ch := range novel.Chapters {
		w.writeChapter(ch)
	}
	return pdf.WritePdf(path)
}

func (w *writer) addPage() {
	w.pdf.AddPage()
	w.pageNum++
	w.pdf.SetFont("novel", "", headerSize)
	w.pdf.SetTextColor(150, 150, 150)
	w.pdf.SetX(marginLeft)
	w.pdf.SetY(30)
	w.pdf.Cell(nil, w.title)
	if w.chTitle != "" {
		displayTitle := w.fitText(w.chTitle, textW/2)
		tw, _ := w.pdf.MeasureTextWidth(displayTitle)
		w.pdf.SetX(pageW - marginRight - tw)
		w.pdf.Cell(nil, displayTitle)
	}
	w.pdf.SetStrokeColor(200, 200, 200)
	w.pdf.SetLineWidth(0.5)
	w.pdf.Line(marginLeft, 48, pageW-marginRight, 48)
	w.pdf.Line(marginLeft, pageH-48, pageW-marginRight, pageH-48)
	w.pdf.SetFont("novel", "", headerSize)
	pageStr := fmt.Sprintf("- %d -", w.pageNum)
	tw, _ := w.pdf.MeasureTextWidth(pageStr)
	w.pdf.SetX((pageW - tw) / 2)
	w.pdf.SetY(pageH - 40)
	w.pdf.Cell(nil, pageStr)
	w.pdf.SetTextColor(0, 0, 0)
	w.pdf.SetFont("novel", "", fontSize)
	w.pdf.SetY(marginTop)
	w.pdf.SetX(marginLeft)
}

func (w *writer) maxY() float64 {
	return pageH - marginBottom
}

func (w *writer) writeTitlePage(novel *Novel) {
	w.chTitle = ""
	w.addPage()
	w.pdf.SetFont("novel", "", titleSize)
	w.writeCentered(firstNonEmpty(novel.Title, "novel"), titleSize+10)
	w.pdf.SetY(w.pdf.GetY() + 20)
	w.pdf.SetFont("novel", "", metaSize)
	for _, line := range []string{
		labelLine("作者", novel.Author),
		labelLine("分类", novel.Category),
		labelLine("状态", novel.Status),
	} {
		if strings.TrimSpace(line) == "" {
			continue
		}
		w.writeLine(line, metaSize+8)
	}
	if strings.TrimSpace(novel.Description) != "" {
		w.pdf.SetY(w.pdf.GetY() + 20)
		w.pdf.SetFont("novel", "", fontSize)
		for _, line := range splitParagraphs(novel.Description) {
			w.writeParagraph(line, false)
			w.pdf.SetY(w.pdf.GetY() + 4)
		}
	}
}

func (w *writer) writeChapter(ch Chapter) {
	w.chTitle = firstNonEmpty(ch.Title, "untitled")
	w.addPage()

	w.pdf.SetFont("novel", "", titleSize)
	_, titleLineH, _ := w.pdf.IsFitMultiCell(&gopdf.Rect{W: textW, H: 1000}, "测")
	w.pdf.SetY(marginTop)
	for _, line := range wrapText(w.pdf, w.chTitle, textW) {
		tw, _ := w.pdf.MeasureTextWidth(line)
		w.pdf.SetX((pageW - tw) / 2)
		w.pdf.Cell(nil, line)
		w.pdf.SetY(w.pdf.GetY() + titleLineH + 4)
	}
	w.pdf.SetY(w.pdf.GetY() + 28)
	w.pdf.SetFont("novel", "", fontSize)
	for _, line := range splitParagraphs(ch.Content) {
		w.writeParagraph(line, true)
		w.pdf.SetY(w.pdf.GetY() + 4)
	}
}

func (w *writer) writeParagraph(text string, indent bool) {
	lineH := w.lineH + lineSpacing
	if indent {
		text = "　　" + text
	}
	for _, line := range wrapText(w.pdf, text, textW) {
		if w.pdf.GetY()+lineH > w.maxY() {
			w.addPage()
		}
		w.pdf.SetX(marginLeft)
		w.pdf.Cell(nil, line)
		w.pdf.SetY(w.pdf.GetY() + lineH)
	}
}

func (w *writer) writeCentered(text string, lineH float64) {
	for _, line := range wrapText(w.pdf, text, textW) {
		tw, _ := w.pdf.MeasureTextWidth(line)
		w.pdf.SetX((pageW - tw) / 2)
		w.pdf.Cell(nil, line)
		w.pdf.SetY(w.pdf.GetY() + lineH)
	}
}

func (w *writer) writeLine(text string, lineH float64) {
	if w.pdf.GetY()+lineH > w.maxY() {
		w.addPage()
	}
	w.pdf.SetX(marginLeft)
	w.pdf.Cell(nil, text)
	w.pdf.SetY(w.pdf.GetY() + lineH)
}

func (w *writer) fitText(text string, maxW float64) string {
	tw, _ := w.pdf.MeasureTextWidth(text)
	if tw <= maxW {
		return text
	}
	runes := []rune(text)
	for i := len(runes); i > 0; i-- {
		s := string(runes[:i]) + "..."
		if cw, _ := w.pdf.MeasureTextWidth(s); cw <= maxW {
			return s
		}
	}
	return ""
}

func wrapText(pdf *gopdf.GoPdf, text string, width float64) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	lines := make([]string, 0, len(runes)/30+1)
	for len(runes) > 0 {
		end := len(runes)
		var lineWidth float64
		for i, r := range runes {
			rw, _ := pdf.MeasureTextWidth(string(r))
			if lineWidth+rw > width {
				end = i
				break
			}
			lineWidth += rw
		}
		if end <= 0 {
			end = 1
		}
		lines = append(lines, string(runes[:end]))
		runes = runes[end:]
	}
	return lines
}

func splitParagraphs(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func labelLine(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return label + ": " + value
}

func defaultFontPath() string {
	for _, path := range []string{
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/Library/Fonts/Arial Unicode.ttf",
	} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
