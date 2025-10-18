package echo

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/andybalholm/brotli"
)

const (
	ContentTypeTextPlain             = "text/plain"
	ContentTypeTextHTML              = "text/html"
	ContentTypeTextCSS               = "text/css"
	ContentTypeApplicationJavaScript = "application/javascript"
	ContentTypeApplicationJSON       = "application/json"
	ContentTypeApplicationXML        = "application/xml"
	ContentTypeApplicationPDF        = "application/pdf"
	ContentTypeImageJPEG             = "image/jpeg"
	ContentTypeImagePNG              = "image/png"
	ContentTypeImageGIF              = "image/gif"
	ContentTypeImageSVG              = "image/svg+xml"
	ContentTypeAudioMPEG             = "audio/mpeg"
	ContentTypeVideoMP4              = "video/mp4"
	ContentTypeMultipartForm         = "multipart/form-data"
	ContentTypeFormURLEncoded        = "application/x-www-form-urlencoded"
	ContentTypeOctetStream           = "application/octet-stream"
)

const (
	HttpConnectStepBeforeRequest  = "before_request"
	HttpConnectStepAfterRequest   = "after_request"
	HttpConnectStepBeforeResponse = "before_response"
)

type EchoConn struct {
	step        string
	conn        *tls.Conn
	req         *http.Request
	direct_resp bool
	body        []byte
	modify_resp bool
	resp        *http.Response
}

func (t *EchoConn) IsBeforeRequest() bool {
	return t.step == HttpConnectStepBeforeRequest
}
func (t *EchoConn) IsAfterRequest() bool {
	return t.step == HttpConnectStepAfterRequest
}

func (t *EchoConn) URL() (*url.URL, error) {
	if t.req == nil {
		return nil, errors.New("missing the req")
	}
	return t.req.URL, nil
}

func (t *EchoConn) BindRequest(req *http.Request) {
	t.req = req
}
func (t *EchoConn) GetRequestBody() []byte {
	body, err := io.ReadAll(t.req.Body)
	if err != nil {
		return []byte{}
	}
	defer t.req.Body.Close()
	t.req.Body = io.NopCloser(bytes.NewReader(body))
	return body
}

func (t *EchoConn) BindResponse(resp *http.Response) {
	t.resp = resp
}
func (t *EchoConn) GetResponseBody() ([]byte, error) {
	var decoded_body []byte
	body, err := io.ReadAll(t.resp.Body)
	defer t.resp.Body.Close()

	if err != nil {
		fmt.Println("read resp body failed,", err.Error())
		return nil, err
	}

	switch t.resp.Header.Get("Content-Encoding") {
	case "gzip":
		r, err := gzip_decode(body)
		if err != nil {
			return decoded_body, err
		}
		decoded_body = r

	case "deflate":
		r, err := zlib_decode(body)
		if err != nil {
			return decoded_body, err
		}
		decoded_body = r
	case "br":
		br_reader := brotli.NewReader(io.NopCloser(bytes.NewReader(body)))
		r, err := io.ReadAll(br_reader)
		if err != nil {
			return decoded_body, nil
		}
		decoded_body = r
	default:
		decoded_body = body
		// ...
	}
	t.resp.Body = io.NopCloser(bytes.NewReader(body))
	return decoded_body, nil
}
func (t *EchoConn) GetResponseHeader() *http.Header {
	return &t.resp.Header
}
func (t *EchoConn) SetStatus(status_code int) error {
	_, err := fmt.Fprintf(t.conn, "HTTP/1.1 %d %s\r\n", status_code, http.StatusText(status_code))
	if err != nil {
		return err
	}
	return nil
}

func (t *EchoConn) SetContentType(content_type string) error {
	_, err := fmt.Fprintf(t.conn, "Content-Type: %v\r\n", content_type)
	if err != nil {
		return err
	}
	return nil
}

// 不请求，自己构造响应
func (t *EchoConn) SetBody(body []byte) error {
	// fmt.Println("[LOG][CONN]SetBody")
	_, err := fmt.Fprintf(t.conn, "Content-Length: %d\r\n", len(body))
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(t.conn, "\r\n")
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(t.conn, string(body))
	if err != nil {
		return err
	}
	// fmt.Println("[LOG][CONN]before set_has_tmp_resp")
	t.direct_resp = true
	return nil
}

func (t *EchoConn) ResponseWithoutRequest(status_code int, body []byte, headers http.Header) error {
	// _, err := fmt.Fprintf(t.conn, "HTTP/1.1 %d %s\r\n", status_code, http.StatusText(status_code))
	response := &http.Response{
		Status:        http.StatusText(status_code),
		StatusCode:    status_code,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        headers,
		Body:          nil, // 使用 Write 写入 Body
		ContentLength: -1,  // 自动计算长度
	}
	if err := response.Write(t.conn); err != nil {
		return err
	}
	_, err := t.conn.Write(body)
	if err != nil {
		return err
	}
	t.direct_resp = true
	// if err := t.SetStatus(http.StatusOK); err != nil {
	// 	return err
	// }
	// if err := t.SetContentType(ContentTypeApplicationJavaScript); err != nil {
	// 	return err
	// }
	// if err := t.SetBody(body); err != nil {
	// 	return err
	// }
	return nil
}

func (t *EchoConn) SetJSONResponse(body []byte) error {
	// fmt.Println("[LOG]SetJSONResponse")
	if err := t.SetStatus(http.StatusOK); err != nil {
		return err
	}
	if err := t.SetContentType(ContentTypeApplicationJSON); err != nil {
		return err
	}
	if err := t.SetBody(body); err != nil {
		return err
	}
	return nil
}
func (t *EchoConn) SetJavaScriptResponse(body []byte) error {
	if err := t.SetStatus(http.StatusOK); err != nil {
		return err
	}
	if err := t.SetContentType(ContentTypeApplicationJavaScript); err != nil {
		return err
	}
	if err := t.SetBody(body); err != nil {
		return err
	}
	return nil
}
func (t *EchoConn) ModifyResponseBody(body []byte) error {
	if t.resp == nil {
		return errors.New("missing the resp")
	}
	// t.resp.Body = io.NopCloser(bytes.NewReader(body))
	content_encoding := t.resp.Header.Get("Content-Encoding")
	// fmt.Println("before set content length", t.resp.Header.Get("Content-Length"), len(body), t.req.URL.Path, content_encoding)
	switch content_encoding {
	case "gzip":
		final_body, err := encode_gzip(body)
		if err != nil {
			return err
		}
		t.resp.Body = io.NopCloser(final_body)
		t.resp.ContentLength = int64(len(body))
		t.resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
		t.resp.Header.Set("Content-Encoding", content_encoding)
	case "deflate":
		final_body, err := encode_deflate(body)
		if err != nil {
			return err
		}
		t.resp.Body = io.NopCloser(final_body)
		t.resp.ContentLength = int64(len(body))
		t.resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
		t.resp.Header.Set("Content-Encoding", content_encoding)
	case "br":
		final_body, err := encode_br(body)
		if err != nil {
			return err
		}
		t.resp.Body = io.NopCloser(final_body)
		t.resp.ContentLength = int64(len(body))
		t.resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
		t.resp.Header.Set("Content-Encoding", content_encoding)
	default:
		t.resp.Body = io.NopCloser(bytes.NewReader(body))
		t.resp.ContentLength = int64(len(body))
		t.resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	}
	t.modify_resp = true
	return nil
}
