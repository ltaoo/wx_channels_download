package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultURL = `https://rr8---sn-nu5gi0c-npoy.googlevideo.com/videoplayback?expire=1781648992&ei=AHoxasZpkYL1_A_HkvOABw&ip=101.127.248.36&id=o-AKEfeeaiOL9mOVzRUfCnYFK7cLVZ31AM7A4oNS-sRH4t&itag=18&source=youtube&requiressl=yes&xpc=EgVo2aDSNQ%3D%3D&cps=426&met=1781627392%2C&mh=2R&mm=31%2C29&mn=sn-nu5gi0c-npoy%2Csn-ojnpo5-5o&ms=au%2Crdu&mv=m&mvi=8&pl=21&rms=au%2Cau&initcwndbps=2687500&bui=ARmQxEWXntQYkWkyibM7PPQTDEuYf0ezlowhsFjsGUlz435Jq8AdoSjXs_pUsnQLzFsSD-xZrHE5CT30&spc=SQ-umrf6FUkrsF6NmHOcaug94pj6IcsvsTSrcwRecXmMvE1mWqr7kLOBcin0Ixlk2i6F5ype&vprv=1&svpuc=1&mime=video%2Fmp4&ns=7jyucd6r__LWvxUv7oyji8YV&rqh=1&gir=yes&clen=49976432&ratebypass=yes&dur=838.658&lmt=1752413352409248&mt=1781626898&fvip=4&fexp=51565116%2C51565681%2C51946838%2C51987686&c=WEB&sefc=1&txp=5538534&n=51IApSAOALotMDi-X&sparams=expire%2Cei%2Cip%2Cid%2Citag%2Csource%2Crequiressl%2Cxpc%2Cbui%2Cspc%2Cvprv%2Csvpuc%2Cmime%2Cns%2Crqh%2Cgir%2Cclen%2Cratebypass%2Cdur%2Clmt&sig=AHEqNM4wRAIgNYNwRxDFavwdUTq8J-_rZlbkOUe3-Vc8cK0mk9fjLpcCICK4_X9G3VLDVaKyC-JMdmkFn3phY-Aq2NrdB0WzlrbW&lsparams=cps%2Cmet%2Cmh%2Cmm%2Cmn%2Cms%2Cmv%2Cmvi%2Cpl%2Crms%2Cinitcwndbps&lsig=APaTxxMwRAIgY7Qvze-9865dna8Bb9XM2g6FE-HP4cqMCOlkJo-3WpwCIC2TjQ37O-YxII5D0fGWygIL0V2slmzz-05Wz7X6WrTr`

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.5 Safari/605.1.15"

func main() {
	rawURL := flag.String("url", defaultURL, "googlevideo videoplayback URL")
	output := flag.String("o", filepath.Join("_example", "youtube", "output.mp4"), "output file")
	cookie := flag.String("cookie", "", "optional Cookie header")
	timeout := flag.Duration("timeout", 15*time.Minute, "request timeout")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	if err := download(ctx, *rawURL, *output, *cookie); err != nil {
		fmt.Fprintf(os.Stderr, "download failed: %v\n", err)
		os.Exit(1)
	}
}

func download(ctx context.Context, rawURL, output, cookie string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("URL must be absolute")
	}
	printURLDiagnostics(parsed)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	setDownloadHeaders(req, cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("HTTP %s\n%s", resp.Status, strings.TrimSpace(string(snippet)))
	}

	total := resp.ContentLength
	if total <= 0 {
		total = int64Query(parsed, "clen")
	}

	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	partPath := output + ".part"
	file, err := os.Create(partPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer file.Close()

	start := time.Now()
	var downloaded int64
	buffer := make([]byte, 256*1024)
	lastPrint := time.Now()
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, err := file.Write(buffer[:n]); err != nil {
				return fmt.Errorf("write temp file: %w", err)
			}
			downloaded += int64(n)
			if time.Since(lastPrint) >= time.Second {
				printProgress(downloaded, total, start)
				lastPrint = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}
	}
	printProgress(downloaded, total, start)
	fmt.Println()

	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(partPath, output); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	fmt.Printf("saved: %s (%d bytes)\n", output, downloaded)
	return nil
}

func setDownloadHeaders(req *http.Request, cookie string) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://www.youtube.com")
	req.Header.Set("Referer", "https://www.youtube.com/")
	req.Header.Set("Range", "bytes=0-")
	req.Header.Set("Sec-Fetch-Dest", "video")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(cookie))
	}
}

func printURLDiagnostics(parsed *url.URL) {
	query := parsed.Query()
	fmt.Printf("host: %s\n", parsed.Host)
	if ip := query.Get("ip"); ip != "" {
		fmt.Printf("signed ip: %s\n", ip)
	}
	if expire := int64Query(parsed, "expire"); expire > 0 {
		expireAt := time.Unix(expire, 0)
		fmt.Printf("expires: %s (local: %s)\n", expireAt.UTC().Format(time.RFC3339), expireAt.Local().Format(time.RFC3339))
		if time.Now().After(expireAt) {
			fmt.Println("warning: signed URL is already expired")
		}
	}
	if clen := int64Query(parsed, "clen"); clen > 0 {
		fmt.Printf("expected size: %d bytes\n", clen)
	}
	if n := query.Get("n"); n != "" {
		fmt.Printf("n param: %s\n", n)
	}
	fmt.Println("starting download...")
}

func int64Query(parsed *url.URL, key string) int64 {
	value := parsed.Query().Get(key)
	if value == "" {
		return 0
	}
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}

func printProgress(downloaded, total int64, start time.Time) {
	elapsed := time.Since(start).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(downloaded) / elapsed
	}
	if total > 0 {
		percent := float64(downloaded) * 100 / float64(total)
		fmt.Printf("\r%.2f%% %d/%d bytes %.1f KiB/s", percent, downloaded, total, speed/1024)
		return
	}
	fmt.Printf("\r%d bytes %.1f KiB/s", downloaded, speed/1024)
}
