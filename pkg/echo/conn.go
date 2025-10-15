package echo

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	ContentTypeTextPlain       = "text/plain"
	ContentTypeTextHTML        = "text/html"
	ContentTypeTextCSS         = "text/css"
	ContentTypeTextJavaScript  = "text/javascript"
	ContentTypeApplicationJSON = "application/json"
	ContentTypeApplicationXML  = "application/xml"
	ContentTypeApplicationPDF  = "application/pdf"
	ContentTypeImageJPEG       = "image/jpeg"
	ContentTypeImagePNG        = "image/png"
	ContentTypeImageGIF        = "image/gif"
	ContentTypeImageSVG        = "image/svg+xml"
	ContentTypeAudioMPEG       = "audio/mpeg"
	ContentTypeVideoMP4        = "video/mp4"
	ContentTypeMultipartForm   = "multipart/form-data"
	ContentTypeFormURLEncoded  = "application/x-www-form-urlencoded"
	ContentTypeOctetStream     = "application/octet-stream"
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
func (t *EchoConn) GetResponseBody() []byte {
	body, err := io.ReadAll(t.resp.Body)
	if err != nil {
		return []byte{}
	}
	t.resp.Body.Close()
	var decoded_body []byte
	switch t.resp.Header.Get("Content-Encoding") {
	case "gzip":
		decoded_body, err = gzipDecode(body)
		if err != nil {
			decoded_body = body
		}
	case "deflate":
		decoded_body, err = zlibDecode(body)
		if err != nil {
			decoded_body = body
		}
	default:
		decoded_body = body
	}
	t.resp.Body = io.NopCloser(bytes.NewReader(body))
	return decoded_body
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
func (t *EchoConn) SetBody(body string) error {
	fmt.Println("[LOG][CONN]SetBody")
	_, err := fmt.Fprintf(t.conn, "Content-Length: %d\r\n", len(body))
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(t.conn, "\r\n")
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(t.conn, body)
	if err != nil {
		return err
	}
	fmt.Println("[LOG][CONN]before set_has_tmp_resp")
	t.direct_resp = true
	return nil
}

func (t *EchoConn) ResponseWithoutRequest(status_code int, body []byte, headers http.Header) {

}

func (t *EchoConn) SetJSONResponse(body string) error {
	fmt.Println("[LOG]SetJSONResponse")
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
func (t *EchoConn) SetJavaScriptResponse(body string) error {
	if err := t.SetStatus(http.StatusOK); err != nil {
		return err
	}
	if err := t.SetContentType(ContentTypeTextJavaScript); err != nil {
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
	// io.Copy(t.resp, decoded_body)
	t.resp.Body = io.NopCloser(bytes.NewReader(body))
	t.resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	t.resp.Header.Del("Content-Encoding")
	t.modify_resp = true
	return nil
}
