// Tests adapted from dramadl; see ../../NOTICE and ../../LICENSE.
package source

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"wx_channel/pkg/hls"
)

var hls_policy_fixture = Policy{Media: []string{"cdn.example"}, RequireHTTPS: true}

func TestPlatformPolicyChecksSegmentsKeysAndInitializationObjects(t *testing.T) {
	for _, kind := range []string{"segment", "key", "init"} {
		playlist_for := func(address string) string {
			prefix := "#EXTM3U\n#EXT-X-TARGETDURATION:2\n"
			segment := "https://cdn.example/segment.ts"
			switch kind {
			case "segment":
				segment = address
			case "key":
				prefix += "#EXT-X-KEY:METHOD=AES-128,URI=\"" + address + "\"\n"
			case "init":
				prefix += "#EXT-X-MAP:URI=\"" + address + "\"\n"
			}
			return prefix + "#EXTINF:2,\n" + segment + "\n#EXT-X-ENDLIST\n"
		}
		t.Run(kind, func(t *testing.T) {
			if _, err := hls_policy_fixture.ParseHLS(playlist_for("https://sub.cdn.example/resource")); err != nil {
				t.Fatalf("official resource rejected: %v", err)
			}
			for _, address := range []string{
				"https://other.example/resource",
				"https://cdn.example.evil.example/resource",
				"http://cdn.example/resource",
				"https://user@cdn.example/resource",
				"https://cdn.example:8443/resource",
				"https://cdn.example/resource#fragment",
				"file:///tmp/resource",
				"relative/resource",
			} {
				text := playlist_for(address)
				if _, err := hls.ParseMedia(text); err != nil {
					t.Fatalf("generic parser applied a source policy to %q: %v", address, err)
				}
				if _, err := hls_policy_fixture.ParseHLS(text); err == nil {
					t.Errorf("platform accepted %s URI %q", kind, address)
				}
			}
		})
	}
}

func TestHLSMasterSelectsMatchingAudioAndRejectsTruncation(t *testing.T) {
	master := "#EXTM3U\n#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"main\",DEFAULT=YES,URI=\"audio.m3u8\"\n#EXT-X-STREAM-INF:BANDWIDTH=1000,AUDIO=\"main\"\nvideo.m3u8\n"
	api := mock_client(t, func(req *http.Request) (string, int) {
		if req.URL.Path == "/master.m3u8" {
			return master, 200
		}
		return "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nsegment.ts\n#EXT-X-ENDLIST\n", 200
	})
	client := &RequestClient{HTTPClient: api.HTTPClient, Policy: fixture_policy}
	video, audio, err := client.LoadHLSTracks(context.Background(), "https://pl-ali.youku.com/master.m3u8", 0)
	if err != nil || video == "" || audio == "" || !strings.Contains(audio, "https://pl-ali.youku.com/segment.ts") {
		t.Fatalf("lost separate audio: %v", err)
	}
	if err := fixture_policy.CheckPlaylistDuration(video, 1200); err == nil {
		t.Fatal("accepted 2-second preview of 1200-second video")
	}
	if _, err := fixture_policy.ParseHLS(strings.Replace(video, "#EXT-X-ENDLIST", "", 1)); err == nil {
		t.Fatal("accepted unterminated media")
	}
}
