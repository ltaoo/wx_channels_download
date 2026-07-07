package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	contentdownload "wx_channel/pkg/contentplatform/download"
	contentyoutube "wx_channel/pkg/contentplatform/youtube"
)

func main() {
	rawURL := flag.String("url", "https://www.youtube.com/watch?v=3ryh7PNhz3E", "YouTube watch URL")
	outputDir := flag.String("dir", os.TempDir(), "download directory")
	filename := flag.String("filename", "youtube-e2e", "output filename without suffix")
	variantID := flag.String("variant", "", "optional variant id")
	probeOnly := flag.Bool("probe-only", false, "only probe and list variants")
	timeout := flag.Duration("timeout", 20*time.Minute, "download timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	router := contentdownload.NewRouter(contentyoutube.New(nil))
	probe, err := router.Probe(ctx, contentdownload.ProbeInput{URL: *rawURL})
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("content_id=%s default=%s suffix=%s warnings=%v\n", probe.ContentID, probe.Defaults.VariantID, probe.Defaults.Suffix, probe.Warnings)
	for _, variant := range probe.Variants {
		meta, _ := json.Marshal(variant.Metadata)
		fmt.Printf("variant id=%s type=%s spec=%s suffix=%s requires=%v meta=%s\n", variant.ID, variant.Type, variant.Spec, variant.Suffix, variant.Requires, string(meta))
	}
	if *probeOnly {
		return
	}

	options := contentdownload.Options{
		VariantID: *variantID,
		Filename:  *filename,
		Extra: map[string]any{
			"ffmpeg_available": true,
		},
	}
	resolved, err := router.Resolve(ctx, contentdownload.ResolveInput{
		URL:     *rawURL,
		Probe:   probe,
		Options: options,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("resolved protocol=%s suffix=%s url=%s metadata=%v\n", resolved.Download.Protocol, resolved.Suffix, resolved.Download.URL, resolved.Metadata)

	downloader := contentdownload.NewDownloader(router, *outputDir)
	task, err := downloader.CreateResolved(resolved)
	if err == nil {
		err = downloader.Start(ctx, task.ID)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "download failed: %v\n", err)
		if task != nil {
			fmt.Fprintf(os.Stderr, "task status: %s error=%s path=%s\n", task.Status, task.Error, task.FilePath)
		}
		os.Exit(1)
	}
	if task == nil {
		fmt.Fprintln(os.Stderr, "download failed: no task returned")
		os.Exit(1)
	}
	info, err := os.Stat(task.FilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat output failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("status=%s\npath=%s\nsize=%d\n", task.Status, filepath.Clean(task.FilePath), info.Size())
}
