package wxchannels

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
)

func TestMediaItemNumericCompatibility(t *testing.T) {
	for _, quoted := range []bool{false, true} {
		t.Run(fmt.Sprintf("quoted=%t", quoted), func(t *testing.T) {
			number := func(value string) string {
				if quoted {
					return strconv.Quote(value)
				}
				return value
			}
			payload := fmt.Sprintf(`{"mediaType":%s,"videoPlayLen":%s,"fileSize":%s,"decodeKey":9007199254740993,"spec":[{"videoBitrate":8000000,"levelOrder":9}]}`,
				number("4"), number("129"), number("1024"))
			var media ChannelsMediaItem
			if err := json.Unmarshal([]byte(payload), &media); err != nil {
				t.Fatal(err)
			}
			if media.MediaType != MediaTypeVideo || media.VideoPlayLen != 129 || media.FileSize != 1024 || media.DecodeKey != "9007199254740993" {
				t.Fatalf("media values changed: %+v", media)
			}
			if len(media.Spec) != 1 || media.Spec[0].VideoBitrate != 8000000 || media.Spec[0].LevelOrder != 9 {
				t.Fatalf("video ranking fields lost: %+v", media.Spec)
			}
		})
	}
}

func TestMediaItemLargeFileSize(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("large file sizes require a 64-bit int")
	}
	want, err := strconv.Atoi("9007199254740993")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{`9007199254740993`, `"9007199254740993"`} {
		var media ChannelsMediaItem
		if err := json.Unmarshal([]byte(`{"fileSize":`+value+`}`), &media); err != nil {
			t.Fatal(err)
		}
		if media.FileSize != want {
			t.Fatalf("file size lost precision: got %d, want %d", media.FileSize, want)
		}
	}
}

func TestMediaItemPartialAndInvalidNumbers(t *testing.T) {
	media := ChannelsMediaItem{MediaType: MediaTypeVideo, VideoPlayLen: 129, FileSize: 1024}
	if err := json.Unmarshal([]byte(`{"mediaType":null,"fileSize":null}`), &media); err != nil {
		t.Fatal(err)
	}
	if media.MediaType != MediaTypeVideo || media.VideoPlayLen != 129 || media.FileSize != 1024 {
		t.Fatalf("missing or null fields overwrote existing values: %+v", media)
	}
	for _, field := range []string{"mediaType", "videoPlayLen", "fileSize"} {
		for _, value := range []string{`"bad"`, `""`, `1.5`, `true`, `"999999999999999999999999999"`} {
			var invalid ChannelsMediaItem
			if err := json.Unmarshal([]byte(fmt.Sprintf(`{"%s":%s}`, field, value)), &invalid); err == nil {
				t.Errorf("accepted invalid %s: %s", field, value)
			}
		}
	}
}
