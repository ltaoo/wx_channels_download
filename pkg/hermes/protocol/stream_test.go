package protocol

import (
	"testing"

	"wx_channel/pkg/hermes"
)

func TestBuildStreamFFmpegArgsUsesEndpointProxyForHTTP(t *testing.T) {
	const wantProxyURL = "http://stream-user:p%40ss@127.0.0.1:8080"
	args, err := buildStreamFFmpegArgs(hermes.Endpoint{
		URL: "https://media.example/live.flv",
		ProxyServer: hermes.ProxyServer{
			Address:  "127.0.0.1:8080",
			Username: "stream-user",
			Password: "p@ss",
		},
	}, "segment-%06d.mkv", 0, 10)
	if err != nil {
		t.Fatal(err)
	}

	proxyIndex := stringIndex(args, "-http_proxy")
	if proxyIndex < 0 || proxyIndex+1 >= len(args) || args[proxyIndex+1] != wantProxyURL {
		t.Fatalf("ffmpeg args do not contain task proxy: %v", args)
	}
	inputIndex := stringIndex(args, "-i")
	if inputIndex < 0 || proxyIndex > inputIndex {
		t.Fatalf("proxy option must be placed before the input: %v", args)
	}
}

func TestBuildStreamFFmpegArgsDoesNotApplyHTTPProxyToRTSP(t *testing.T) {
	args, err := buildStreamFFmpegArgs(hermes.Endpoint{
		URL:         "rtsp://media.example/live",
		ProxyServer: hermes.ProxyServer{Address: "http://127.0.0.1:8080"},
	}, "segment-%06d.mkv", 0, 10)
	if err != nil {
		t.Fatal(err)
	}

	if stringIndex(args, "-http_proxy") >= 0 {
		t.Fatalf("RTSP args unexpectedly contain HTTP proxy: %v", args)
	}
}

func stringIndex(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}
