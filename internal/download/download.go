package download

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/decrypt"
)

// 解密读取器
type DecryptReader struct {
	reader   io.Reader
	ctx      *decrypt.RandCtx64
	limit    uint64
	consumed uint64
	ks       [8]byte
	ksPos    int
}

func NewDecryptReader(reader io.Reader, key uint64, offset uint64, limit uint64) *DecryptReader {
	ctx := decrypt.CreateISAacInst(key)
	dr := &DecryptReader{
		reader:   reader,
		ctx:      ctx,
		limit:    limit,
		consumed: 0,
		ksPos:    8,
	}
	if limit > 0 {
		// 将 consumed 对齐到文件偏移，超出加密区则设置为加密区末尾
		if offset >= limit {
			dr.consumed = limit
		} else {
			dr.consumed = offset
			// 跳过完整的 8 字节块
			skipBlocks := offset / 8
			for i := uint64(0); i < skipBlocks; i++ {
				_ = dr.ctx.ISAacRandom()
			}
			// 生成当前块并设置起始位置
			randNumber := dr.ctx.ISAacRandom()
			binary.BigEndian.PutUint64(dr.ks[:], randNumber)
			dr.ksPos = int(offset % 8)
		}
	}
	return dr
}

func (dr *DecryptReader) Read(p []byte) (int, error) {
	n, err := dr.reader.Read(p)
	if n <= 0 {
		return n, err
	}
	if dr.limit == 0 || dr.consumed >= dr.limit {
		return n, err
	}

	toDecrypt := uint64(n)
	remaining := dr.limit - dr.consumed
	if toDecrypt > remaining {
		toDecrypt = remaining
	}
	// 逐字节异或，维护 keystream 位置
	for i := uint64(0); i < toDecrypt; i++ {
		if dr.ksPos >= 8 {
			randNumber := dr.ctx.ISAacRandom()
			binary.BigEndian.PutUint64(dr.ks[:], randNumber)
			dr.ksPos = 0
		}
		p[i] ^= dr.ks[dr.ksPos]
		dr.ksPos++
	}
	dr.consumed += toDecrypt
	return n, err
}

type MediaProxyWithDecrypt struct {
	client *http.Client
}

func NewMediaProxyWithDecrypt() *MediaProxyWithDecrypt {
	tr := &http.Transport{
		TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	return &MediaProxyWithDecrypt{
		client: &http.Client{Transport: tr},
	}
}

func (mp *MediaProxyWithDecrypt) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	targetURL := q.Get("url")
	if targetURL == "" {
		http.Error(w, "missing targetURL", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://" + targetURL
	}
	if _, err := url.Parse(targetURL); err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	filename := q.Get("filename")
	if filename == "" {
		if u, err := url.Parse(targetURL); err == nil {
			if base := path.Base(u.Path); base != "" && base != "/" {
				filename = base
			}
		}
		if filename == "" {
			filename = "download.mp4"
		}
	}
	decryptKeyStr := q.Get("key")
	toMP3 := q.Get("mp3")
	if decryptKeyStr != "" {
		decryptKey, err := strconv.ParseUint(decryptKeyStr, 0, 64)
		if err != nil {
			http.Error(w, "invalid decryptKey", http.StatusBadRequest)
			return
		}
		if toMP3 == "1" {
			mp.convertWithDecrypt(w, targetURL, decryptKey, 131072, filename)
			return
		}
		mp.decryptOnly(w, r, targetURL, decryptKey, 131072, filename)
		return
	}
	mp.convertOnly(targetURL, w, filename, "mp3")
}

func (mp *MediaProxyWithDecrypt) convertWithDecrypt(w http.ResponseWriter, targetURL string, key uint64, encLimit uint64, filename string) {
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

func (mp *MediaProxyWithDecrypt) decryptOnly(w http.ResponseWriter, r *http.Request, targetURL string, key uint64, encLimit uint64, filename string) {
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

func (mp *MediaProxyWithDecrypt) convertOnly(targetURL string, w http.ResponseWriter, filename string, format string) {
	if format != "mp3" {
		mp.simpleProxy(targetURL, w, nil)
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

func (mp *MediaProxyWithDecrypt) prepareRequest(method, targetURL string, header http.Header) (*http.Request, error) {
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

func (mp *MediaProxyWithDecrypt) simpleProxy(targetURL string, w http.ResponseWriter, r *http.Request) {
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

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range, Accept, Origin, X-Requested-With")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Accept-Ranges, Content-Type, Content-Length, Content-Disposition")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
