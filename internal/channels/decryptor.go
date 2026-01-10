package channels

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

type ChannelsVideoDecryptor struct {
	client *http.Client
}

func NewChannelsVideoDecryptor() *ChannelsVideoDecryptor {
	tr := &http.Transport{
		TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	return &ChannelsVideoDecryptor{
		client: &http.Client{Transport: tr},
	}
}

// func (mp *ChannelsVideoDecryptor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	handleDownload(w, r)
// }

func (mp *ChannelsVideoDecryptor) ConvertWithDecrypt(w http.ResponseWriter, targetURL string, key uint64, encLimit uint64, filename string) {
	req, err := mp.prepareRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := mp.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	decryptReader := NewDecryptReader(resp.Body, key, 0, encLimit)

	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-vn",
		"-acodec", "libmp3lame",
		"-ab", "192k",
		"-f", "mp3",
		"pipe:1",
	)
	cmd.Stdin = decryptReader
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	// 设置下载文件名,确保使用 .mp3 扩展名
	downloadFilename := filename
	if !strings.HasSuffix(strings.ToLower(downloadFilename), ".mp3") {
		downloadFilename = strings.TrimSuffix(downloadFilename, path.Ext(downloadFilename)) + ".mp3"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadFilename))
	bw := bufio.NewWriterSize(w, 64*1024)
	defer bw.Flush()
	if _, err := io.Copy(bw, stdout); err != nil {
		_ = cmd.Process.Kill()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = cmd.Wait()
}

func (mp *ChannelsVideoDecryptor) DecryptOnly(w http.ResponseWriter, r *http.Request, targetURL string, key uint64, encLimit uint64, filename string) {
	req, err := mp.prepareRequest(r.Method, targetURL, r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := mp.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var startOffset uint64 = 0
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		parts := strings.Split(cr, " ")
		if len(parts) == 2 {
			rangePart := parts[1]
			dash := strings.Index(rangePart, "-")
			if dash > 0 {
				if v, err := strconv.ParseUint(rangePart[:dash], 10, 64); err == nil {
					startOffset = v
				}
			}
		}
	}
	decryptReader := NewDecryptReader(resp.Body, key, startOffset, encLimit)

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if w.Header().Get("Accept-Ranges") == "" {
		w.Header().Set("Accept-Ranges", "bytes")
	}

	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	io.Copy(w, decryptReader)
}

// Inline playback variants (no attachment headers)
func (mp *ChannelsVideoDecryptor) DecryptOnlyInline(w http.ResponseWriter, r *http.Request, targetURL string, key uint64, encLimit uint64) {
	req, err := mp.prepareRequest(r.Method, targetURL, r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := mp.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	var startOffset uint64 = 0
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		parts := strings.Split(cr, " ")
		if len(parts) == 2 {
			rangePart := parts[1]
			dash := strings.Index(rangePart, "-")
			if dash > 0 {
				if v, err := strconv.ParseUint(rangePart[:dash], 10, 64); err == nil {
					startOffset = v
				}
			}
		}
	}
	decryptReader := NewDecryptReader(resp.Body, key, startOffset, encLimit)
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	if w.Header().Get("Accept-Ranges") == "" {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	io.Copy(w, decryptReader)
}

func (mp *ChannelsVideoDecryptor) ConvertWithDecryptInline(w http.ResponseWriter, targetURL string, key uint64, encLimit uint64) {
	req, err := mp.prepareRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := mp.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	decryptReader := NewDecryptReader(resp.Body, key, 0, encLimit)
	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-vn",
		"-acodec", "libmp3lame",
		"-ab", "192k",
		"-f", "mp3",
		"pipe:1",
	)
	cmd.Stdin = decryptReader
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	bw := bufio.NewWriterSize(w, 64*1024)
	defer bw.Flush()
	if _, err := io.Copy(bw, stdout); err != nil {
		_ = cmd.Process.Kill()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = cmd.Wait()
}

func (mp *ChannelsVideoDecryptor) ConvertOnlyInline(targetURL string, w http.ResponseWriter, format string) {
	if format != "mp3" {
		mp.SimpleProxy(targetURL, w, nil)
		return
	}
	req, err := mp.prepareRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := mp.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-vn",
		"-acodec", "libmp3lame",
		"-ab", "192k",
		"-f", "mp3",
		"pipe:1",
	)
	cmd.Stdin = resp.Body
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	bw := bufio.NewWriterSize(w, 64*1024)
	defer bw.Flush()
	if _, err := io.Copy(bw, stdout); err != nil {
		cmd.Process.Kill()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = cmd.Wait()
}
func (mp *ChannelsVideoDecryptor) ConvertOnly(targetURL string, w http.ResponseWriter, filename string, format string) {
	if format != "mp3" {
		mp.SimpleProxy(targetURL, w, nil)
		return
	}
	req, err := mp.prepareRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := mp.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-vn",
		"-acodec", "libmp3lame",
		"-ab", "192k",
		"-f", "mp3",
		"pipe:1",
	)
	cmd.Stdin = resp.Body
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	// 设置下载文件名,确保使用 .mp3 扩展名
	downloadFilename := filename
	if !strings.HasSuffix(strings.ToLower(downloadFilename), ".mp3") {
		downloadFilename = strings.TrimSuffix(downloadFilename, path.Ext(downloadFilename)) + ".mp3"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadFilename))
	bufferedWriter := bufio.NewWriterSize(w, 64*1024)
	defer bufferedWriter.Flush()
	if _, err := io.Copy(bufferedWriter, stdout); err != nil {
		cmd.Process.Kill()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = cmd.Wait()
}

func (mp *ChannelsVideoDecryptor) prepareRequest(method, targetURL string, header http.Header) (*http.Request, error) {
	if method != http.MethodGet && method != http.MethodHead {
		method = http.MethodGet
	}
	req, err := http.NewRequest(method, targetURL, nil)
	if err != nil {
		return nil, err
	}
	// Copy headers if provided
	if header != nil {
		if rangeHeader := header.Get("Range"); rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
		}
	}
	return req, nil
}

func (mp *ChannelsVideoDecryptor) SimpleProxy(targetURL string, w http.ResponseWriter, r *http.Request) {
	var header http.Header
	method := http.MethodGet
	if r != nil {
		header = r.Header
		method = r.Method
	}
	req, err := mp.prepareRequest(method, targetURL, header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp, err := mp.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	if w.Header().Get("Accept-Ranges") == "" {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	w.WriteHeader(resp.StatusCode)
	if method == http.MethodHead {
		return
	}
	io.Copy(w, resp.Body)
}
