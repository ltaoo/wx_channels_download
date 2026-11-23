package hls

import (
	"strings"
	"testing"
)

func TestParseMediaPreservesURIsAndRewritesLocalReferences(t *testing.T) {
	text := "\ufeff#EXTM3U\r\n#EXT-X-VERSION:7\r\n" +
		"#EXT-X-KEY:METHOD=AES-128,URI=\"../key?token=a,b\"\r\n" +
		"#EXT-X-MAP:URI=\"https://cdn.example/init.mp4?token=init\"\r\n" +
		"#EXTINF:2.5,\r\npart.m4s?token=segment\r\n" +
		"#EXT-X-DISCONTINUITY\r\n#EXTINF:1.5,\r\npart.m4s?token=segment\r\n#EXT-X-ENDLIST\r\n"
	playlist, err := ParseMedia(text)
	if err != nil {
		t.Fatal(err)
	}
	if playlist.Duration != 4 || len(playlist.Resources) != 3 {
		t.Fatalf("lost timeline or resource deduplication: %+v", playlist)
	}
	want := []Resource{
		{Address: "../key?token=a,b", Filename: "resource-00000.key", Key: true},
		{Address: "https://cdn.example/init.mp4?token=init", Filename: "resource-00001.mp4"},
		{Address: "part.m4s?token=segment", Filename: "resource-00002.m4s"},
	}
	for index, resource := range playlist.Resources {
		if resource != want[index] {
			t.Fatalf("resource URI changed: got %+v, want %+v", resource, want[index])
		}
	}
	if strings.Contains(playlist.Text, "token=") || !strings.Contains(playlist.Text, `URI="resource-00000.key"`) || !strings.Contains(playlist.Text, `URI="resource-00001.mp4"`) || strings.Count(playlist.Text, "resource-00002.m4s") != 2 || !strings.Contains(playlist.Text, "#EXT-X-DISCONTINUITY") {
		t.Fatalf("local playlist lost references or timeline tags: %s", playlist.Text)
	}
}

func TestParseMediaRejectsIncompleteAndUnsupportedInputs(t *testing.T) {
	for name, text := range map[string]string{
		"missing header":         "#EXTINF:2,\nsegment.ts\n#EXT-X-ENDLIST\n",
		"live playlist":          "#EXTM3U\n#EXTINF:2,\nsegment.ts\n",
		"master playlist":        "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\nvideo.m3u8\n",
		"missing segment":        "#EXTM3U\n#EXTINF:2,\n#EXT-X-ENDLIST\n",
		"invalid duration":       "#EXTM3U\n#EXTINF:NaN,\nsegment.ts\n#EXT-X-ENDLIST\n",
		"segment after end":      "#EXTM3U\n#EXTINF:2,\nsegment.ts\n#EXT-X-ENDLIST\n#EXTINF:2,\nlate.ts\n",
		"unsupported encryption": "#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"key\"\n#EXTINF:2,\nsegment.ts\n#EXT-X-ENDLIST\n",
		"conflicting key URI":    "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"same.ts\"\n#EXTINF:2,\nsame.ts\n#EXT-X-ENDLIST\n",
		"malformed URI":          "#EXTM3U\n#EXTINF:2,\nhttps://[invalid/segment.ts\n#EXT-X-ENDLIST\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseMedia(text); err == nil {
				t.Fatal("accepted malformed or unsupported media playlist")
			}
		})
	}
}

func TestParseAttributesPreservesQuotedCommasAndRejectsAmbiguity(t *testing.T) {
	attributes, err := ParseAttributes(`TYPE=AUDIO,GROUP-ID="main",NAME="中文,立体声",DEFAULT=YES,URI="audio.m3u8?token=a,b"`)
	if err != nil || attributes["NAME"] != "中文,立体声" || attributes["URI"] != "audio.m3u8?token=a,b" || attributes["DEFAULT"] != "YES" {
		t.Fatalf("attribute values changed: %+v %v", attributes, err)
	}
	for _, value := range []string{`URI="one",URI="two"`, `URI="unterminated`, `URI="one"garbage`, `BAD ATTRIBUTE=yes`, `missing-equals`} {
		if _, err := ParseAttributes(value); err == nil {
			t.Errorf("accepted ambiguous attribute list %q", value)
		}
	}
}
