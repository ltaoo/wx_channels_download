package download

import (
	"wx_channel/internal/manager"
)

type DownloadServer struct {
	*manager.HTTPServer
}

func NewDownloadServer(addr string) *DownloadServer {
	srv := manager.NewHTTPServer("下载服务", "download", addr)
	proxy := NewMediaProxyWithDecrypt()
	srv.SetHandler(withCORS(proxy))

	return &DownloadServer{
		HTTPServer: srv,
	}
}
