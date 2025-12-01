package download

import (
	"wx_channel/internal/manager"
)

type DownloadServer struct {
	*manager.HTTPServer
}

func NewDownloadServer(addr string) *DownloadServer {
	srv := manager.NewHTTPServer("download", addr)
	proxy := NewMediaProxyWithDecrypt()
	srv.SetHandler(withCORS(proxy))

	return &DownloadServer{
		HTTPServer: srv,
	}
}
