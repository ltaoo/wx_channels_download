package echo

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
)

type Echo struct {
	ca_cert      *x509.Certificate
	ca_key       *rsa.PrivateKey
	ca_key_pair  *tls.Certificate
	cert_cache   sync.Map
	timeout      time.Duration
	filter       []string
	http_handler func(conn *EchoConn)
}

func (h *Echo) SetHTTPHandler(handler func(conn *EchoConn)) {
	h.http_handler = handler
}
func (h *Echo) SetTimeout(timeout time.Duration) {
	h.timeout = timeout
}
func (h *Echo) SetHttpRequestFilter(filter []string) {
	h.filter = filter
}
func (h *Echo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// fmt.Println(r.URL.Host, r.URL.Hostname())
	// match_filter := false
	// for _, f := range h.filter {
	// 	if strings.HasSuffix(r.URL.Hostname(), f) {
	// 		match_filter = true
	// 	}
	// }
	// fmt.Println(match_filter)
	// if match_filter {
	// }
	if r.Method == http.MethodConnect {
		h.handle_https(w, r)
	} else {
		h.handle_http(w, r)
	}

}

func NewEcho(cerfile []byte, keyfile []byte) (*Echo, error) {
	ca_key_pair, err := tls.X509KeyPair(cerfile, keyfile)
	if err != nil {
		return nil, err
	}
	ca_cert, err := x509.ParseCertificate(ca_key_pair.Certificate[0])
	if err != nil {
		return nil, err
	}
	ca_key := ca_key_pair.PrivateKey.(*rsa.PrivateKey)
	return &Echo{
		ca_cert:     ca_cert,
		ca_key_pair: &ca_key_pair,
		ca_key:      ca_key,
		timeout:     30 * time.Second, // 默认30秒超时
	}, nil
}

func (h *Echo) handle_http(w http.ResponseWriter, r *http.Request) {
	req := r
	tls_helper := &EchoConn{step: HttpConnectStepBeforeRequest}
	tls_helper.BindRequest(req)
	h.http_handler(tls_helper)
	// fmt.Println("after http_handler", tls_helper.direct_resp)
	if tls_helper.direct_resp {
		return
	}
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	tls_helper.BindResponse(resp)
	// fmt.Println("[LOG][EVENT]after request")
	// event:after request
	h.http_handler(tls_helper)
	// remove_hop_headers(resp.Header)
	copy_header(w.Header(), resp.Header)

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Echo) handle_https(w http.ResponseWriter, r *http.Request) {
	host_port := r.URL.Host
	if !strings.Contains(host_port, ":") {
		host_port = net.JoinHostPort(host_port, "443")
	}

	// 劫持客户端连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	client_conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer client_conn.Close()
	// 告诉客户端连接已建立
	client_conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	// 为目标主机生成证书
	cert, err := h.generate_cert(r.URL.Hostname())
	if err != nil {
		// log.SetOutput()
		log.Printf("Failed to generate certificate: %v", err)
		return
	}
	// 创建TLS配置
	tls_config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS13,
	}
	// 在客户端和代理之间建立TLS连接
	tls_conn := tls.Server(client_conn, tls_config)
	defer tls_conn.Close()
	tls_helper := &EchoConn{conn: tls_conn}
	// 设置超时
	tls_conn.SetDeadline(time.Now().Add(h.timeout))
	// 等待TLS握手完成
	if err := tls_conn.Handshake(); err != nil {
		// log.Printf("TLS handshake error: %v", err)
		return
	}
	// 读取客户端请求
	reader := bufio.NewReader(tls_conn)
	for {
		// 重置超时
		tls_conn.SetDeadline(time.Now().Add(h.timeout))
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err == io.EOF {
				// log.Println("Client closed connection")
				return
			}
			if net_err, ok := err.(net.Error); ok && net_err.Timeout() {
				// log.Println("Read timeout")
				return
			}
			if strings.Contains(err.Error(), "connection reset by peer") {
				// log.Println("Client reset connection")
				return
			}
			// log.Printf("Error reading request: %v", err)
			return
		}
		// 修改请求以指向正确的目标
		req.URL.Scheme = "https"
		req.URL.Host = host_port
		req.RequestURI = ""
		// remove_hop_headers(req.Header)
		tls_helper.BindRequest(req)
		// fmt.Println("[LOG][EVENT]before request")
		// event:before request
		h.http_handler(tls_helper)
		if tls_helper.direct_resp {
			return
		}
		// 转发请求到目标服务器
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			// log.Printf("Error forwarding request: %v", err)
			return
		}
		tls_helper.BindResponse(resp)
		// fmt.Println("[LOG][EVENT]after request")
		// event:after request
		h.http_handler(tls_helper)
		remove_hop_headers(resp.Header)
		// fmt.Println("Before write resp to tls connection")
		// fmt.Println(req.URL.Path)
		// fmt.Println("Content-Length", resp.Header.Get("Content-Length"))
		// fmt.Println()
		if err := resp.Write(tls_conn); err != nil {
			// log.Printf("Error writing response: %v", err)
			// fmt.Printf("Error writing response: %v\n", err)
			// fmt.Println(req.URL.Path)
			return
		}
		// 如果不是keep-alive连接，则退出循环
		if !isKeepAlive(req, resp) {
			return
		}
	}
}

func (h *Echo) generate_cert(hostname string) (tls.Certificate, error) {
	// 先检查缓存
	if cert, ok := h.cert_cache.Load(hostname); ok {
		return cert.(tls.Certificate), nil
	}

	// 创建证书模板
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: hostname,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		DNSNames:              []string{hostname},
	}

	// 生成私钥
	priv_key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	// 使用CA签名证书
	der_bytes, err := x509.CreateCertificate(rand.Reader, template, h.ca_cert, &priv_key.PublicKey, h.ca_key)
	if err != nil {
		return tls.Certificate{}, err
	}

	// 创建证书和私钥的PEM块
	cert_pem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der_bytes})
	key_pem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv_key)})

	// 创建TLS证书并存入缓存
	cert, err := tls.X509KeyPair(cert_pem, key_pem)
	if err != nil {
		return tls.Certificate{}, err
	}

	h.cert_cache.Store(hostname, cert)
	return cert, nil
}

func copy_header(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func remove_hop_headers(header http.Header) {
	// Hop-by-hop头列表
	hopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func isKeepAlive(req *http.Request, resp *http.Response) bool {
	// 检查请求和响应中的Connection头
	if strings.ToLower(req.Header.Get("Connection")) == "close" {
		return false
	}
	if strings.ToLower(resp.Header.Get("Connection")) == "close" {
		return false
	}

	// HTTP/1.1默认是keep-alive，除非明确声明close
	if resp.ProtoMajor == 1 && resp.ProtoMinor == 1 {
		return true
	}

	// HTTP/1.0需要明确声明keep-alive
	if resp.ProtoMajor == 1 && resp.ProtoMinor == 0 {
		return strings.ToLower(resp.Header.Get("Connection")) == "keep-alive"
	}

	return false
}

func gzip_decode(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func zlib_decode(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func encode_gzip(data []byte) (io.Reader, error) {
	var buf bytes.Buffer
	gz_writer := gzip.NewWriter(&buf)

	if _, err := gz_writer.Write(data); err != nil {
		return nil, fmt.Errorf("gzip write failed: %v", err)
	}
	if err := gz_writer.Close(); err != nil { // 必须 Close() 才能完成压缩
		return nil, fmt.Errorf("gzip close failed: %v", err)
	}

	return &buf, nil
}

func encode_deflate(data []byte) (io.Reader, error) {
	var buf bytes.Buffer
	zlib_writer := zlib.NewWriter(&buf)
	if _, err := zlib_writer.Write(data); err != nil {
		return nil, fmt.Errorf("deflate write failed: %v", err)
	}
	if err := zlib_writer.Close(); err != nil { // 必须 Close() 才能完成压缩
		return nil, fmt.Errorf("deflate close failed: %v", err)
	}

	return &buf, nil
}

func encode_br(data []byte) (io.Reader, error) {
	var buf bytes.Buffer
	br_writer := brotli.NewWriter(&buf)
	if _, err := br_writer.Write(data); err != nil {
		return nil, fmt.Errorf("failed to encode brotli: %v", err)
	}
	if err := br_writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize brotli: %v", err)
	}
	return &buf, nil
}
