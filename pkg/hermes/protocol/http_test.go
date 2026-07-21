package protocol_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wx_channel/pkg/hermes"
	"wx_channel/pkg/hermes/protocol"
)

func TestHTTPPrepareDoesNotWaitForHEAD(t *testing.T) {
	headCalled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			headCalled <- struct{}{}
			<-r.Context().Done()
			return
		}
		if r.Header.Get("Range") != "bytes=0-0" {
			http.Error(w, "missing range probe", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", "bytes 0-0/1024")
		w.Header().Set("Content-Length", "1")
		w.Header().Set("Content-Type", "image/png; charset=binary")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte{0})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	prepared, err := protocol.NewHTTPDriver().Prepare(ctx, hermes.Endpoint{URL: server.URL})
	require.NoError(t, err)
	assert.Equal(t, int64(1024), prepared.Size)
	assert.True(t, prepared.SupportsRange)
	assert.Equal(t, "image/png; charset=binary", prepared.ContentType)
	select {
	case <-headCalled:
		t.Fatal("Prepare 不应发送 HEAD 请求")
	default:
	}
}

func TestHTTPOpenStreamsRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bytes=1-3", r.Header.Get("Range"))
		assert.Equal(t, "identity", r.Header.Get("Accept-Encoding"))
		w.Header().Set("Content-Range", "bytes 1-3/5")
		w.Header().Set("Content-Length", "3")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("ell"))
	}))
	defer server.Close()

	body, err := protocol.NewHTTPDriver().Open(context.Background(), hermes.Endpoint{URL: server.URL}, hermes.ReadRequest{
		OffsetStart: 1,
		OffsetEnd:   3,
		UseRange:    true,
	})
	require.NoError(t, err)
	defer body.Close()

	content, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, []byte("ell"), content)
}
