package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wx_channel/pkg/officialaccount"
)

const targetURL = "https://mp.weixin.qq.com/s/eBb3fVQVCM0WCTicndMw4A"

func main() {
	url := flag.String("url", targetURL, "WeChat official account article URL")
	outDir := flag.String("out", "officialaccount-output", "output directory")
	compress := flag.Bool("compress", false, "compress embedded images")
	flag.Parse()

	path, article, err := downloadArticle(*url, *outDir, *compress)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("saved %q to %s\n", article.Title, path)
}

func downloadArticle(url, outDir string, compress bool) (string, *officialaccount.WechatOfficialArticle, error) {
	oa := &officialaccount.OfficialAccountDownload{}
	article, err := oa.FetchArticle(url)
	if err != nil {
		return "", nil, err
	}
	content, err := oa.BuildHTMLFromArticle(article, compress)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", nil, err
	}
	path := filepath.Join(outDir, sanitizeFilename(article.Title)+".html")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", nil, err
	}
	return path, article, nil
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	name = strings.TrimSpace(replacer.Replace(name))
	if name == "" {
		return "article"
	}
	return name
}
