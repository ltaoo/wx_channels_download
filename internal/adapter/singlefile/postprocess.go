package singlefileadapter

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/clawreq"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/minib"
	"wx_channel/pkg/scraper/singlefile"
)

// Postprocess replaces the saved HTML only after all required assets are embedded.
func (a *SinglefileAdapter) Postprocess(ctx context.Context, info *hermes.TaskJob, _ adapter.PostprocessDeps) error {
	if info == nil {
		return fmt.Errorf("singlefile: task is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	browser, err := minib.NewMiniBrowser(30*time.Second, a.runtime_cookie_provider())
	if err != nil {
		return err
	}
	defer browser.Close()
	for resource_index := range info.Resources {
		resource := &info.Resources[resource_index]
		if resource.Extra["singlefile_postprocess"] != "inline" || resource.Extra["postprocessed"] == "true" {
			continue
		}
		data, err := os.ReadFile(resource.FilePath)
		if err != nil {
			return fmt.Errorf("singlefile read HTML: %w", err)
		}
		processed, err := inline_html(ctx, browser, string(data), resource.Extra["source_url"])
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := replace_html_file(resource.FilePath, []byte(processed)); err != nil {
			return fmt.Errorf("singlefile save HTML: %w", err)
		}
		resource.Kind = "text/html"
		resource.Size = int64(len(processed))
		resource.Downloaded = resource.Size
		resource.Extra["postprocessed"] = "true"
	}
	return nil
}

func inline_html(ctx context.Context, browser *minib.MiniBrowser, source string, source_url string) (string, error) {
	base_url, err := singlefile.ParseURL(source_url)
	if err != nil {
		return "", err
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(source))
	if err != nil {
		return "", err
	}
	if base_href, exists := document.Find("base[href]").First().Attr("href"); exists {
		base_url, err = base_url.Parse(strings.TrimSpace(base_href))
		if err != nil {
			return "", fmt.Errorf("singlefile base URL: %w", err)
		}
	}
	document.Find("script").Remove()
	responses := make(map[string]*clawreq.Response)
	fetch_asset := func(reference string) (*clawreq.Response, error) {
		asset_url, err := base_url.Parse(strings.TrimSpace(reference))
		if err != nil {
			return nil, err
		}
		if response := responses[asset_url.String()]; response != nil {
			return response, nil
		}
		response, err := download_asset(ctx, browser, asset_url.String(), source_url)
		if err != nil {
			return nil, err
		}
		responses[asset_url.String()] = response
		return response, nil
	}
	for _, image_node := range document.Find("img").Nodes {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		image := goquery.NewDocumentFromNode(image_node).Selection
		source := strings.TrimSpace(image.AttrOr("src", ""))
		if source == "" {
			source = strings.TrimSpace(image.AttrOr("data-src", ""))
		}
		if source == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(source), "data:image/") {
			response, err := fetch_asset(source)
			if err != nil {
				return "", fmt.Errorf("singlefile image %q: %w", source, err)
			}
			media_type, _, _ := mime.ParseMediaType(response.ContentType())
			if !strings.HasPrefix(media_type, "image/") {
				media_type = http.DetectContentType(response.Body)
			}
			if !strings.HasPrefix(media_type, "image/") {
				return "", fmt.Errorf("singlefile: %q is not an image (%s)", source, media_type)
			}
			source = "data:" + media_type + ";base64," + base64.StdEncoding.EncodeToString(response.Body)
		}
		image.SetAttr("src", source).RemoveAttr("srcset").RemoveAttr("sizes").RemoveAttr("data-src").RemoveAttr("data-srcset").RemoveAttr("loading")
		image.ParentFiltered("picture").Find("source").Remove()
	}
	for _, link_node := range document.Find("link[href]").Nodes {
		link := goquery.NewDocumentFromNode(link_node).Selection
		is_stylesheet := false
		for _, relation := range strings.Fields(strings.ToLower(link.AttrOr("rel", ""))) {
			is_stylesheet = is_stylesheet || relation == "stylesheet"
		}
		if !is_stylesheet {
			continue
		}
		response, err := fetch_asset(link.AttrOr("href", ""))
		if err != nil {
			return "", fmt.Errorf("singlefile stylesheet: %w", err)
		}
		css, err := response.Text()
		if err != nil {
			return "", fmt.Errorf("singlefile decode CSS: %w", err)
		}
		// ponytail: CSS url()/@import dependencies remain external; add CSS asset traversal when needed.
		style := &html.Node{Type: html.ElementNode, Data: "style", DataAtom: atom.Style,
			Attr: []html.Attribute{{Key: "data-n", Val: "inlined-stylesheet"}}}
		for _, attribute := range link_node.Attr {
			switch attribute.Key {
			case "media", "title", "disabled", "nonce":
				style.Attr = append(style.Attr, attribute)
			}
		}
		style.AppendChild(&html.Node{Type: html.TextNode, Data: css})
		link_node.Parent.InsertBefore(style, link_node)
		link_node.Parent.RemoveChild(link_node)
	}
	return document.Html()
}

func download_asset(ctx context.Context, browser *minib.MiniBrowser, raw_url string, referer string) (*clawreq.Response, error) {
	request_ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for redirect_count := 0; redirect_count <= 10; redirect_count++ {
		request_url, err := singlefile.ParseURL(raw_url)
		if err != nil {
			return nil, err
		}
		response, err := browser.Get(request_ctx, raw_url, http.Header{"Referer": {referer}})
		if err != nil {
			return nil, err
		}
		switch response.StatusCode {
		case 301, 302, 303, 307, 308:
			location := response.Header.Get("Location")
			if location == "" {
				return nil, fmt.Errorf("asset redirect has no Location")
			}
			next_url, err := request_url.Parse(location)
			if err != nil {
				return nil, err
			}
			raw_url = next_url.String()
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("asset %q returned HTTP %d", raw_url, response.StatusCode)
		}
		return response, nil
	}
	return nil, fmt.Errorf("singlefile: too many asset redirects")
}

func replace_html_file(file_path string, data []byte) error {
	file_info, err := os.Stat(file_path)
	if err != nil {
		return err
	}
	temporary_file, err := os.CreateTemp(filepath.Dir(file_path), ".singlefile-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temporary_file.Name())
	defer temporary_file.Close()
	if err := temporary_file.Chmod(file_info.Mode().Perm()); err != nil {
		return err
	}
	if _, err := temporary_file.Write(data); err != nil {
		return err
	}
	if err := temporary_file.Sync(); err != nil {
		return err
	}
	if err := temporary_file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary_file.Name(), file_path)
}
