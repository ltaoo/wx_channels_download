package wxchannels

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestChannelsMediaIntegerFields(t *testing.T) {
	for _, field := range []string{"mediaType", "videoPlayLen", "fileSize"} {
		for _, value := range []string{`42`, `"42"`, `null`} {
			t.Run(field+"/"+value, func(t *testing.T) {
				var object ChannelsObject
				body := `{"objectDesc":{"media":[{"` + field + `":` + value + `}]}}`
				if err := json.Unmarshal([]byte(body), &object); err != nil {
					t.Fatal(err)
				}
				media := object.ObjectDesc.Media[0]
				got := map[string]int{"mediaType": media.MediaType, "videoPlayLen": media.VideoPlayLen, "fileSize": media.FileSize}[field]
				want := 42
				if value == "null" {
					want = 0
				}
				if got != want {
					t.Fatalf("got %d, want %d", got, want)
				}
			})
		}
	}
}

func TestChannelsMediaInvalidIntegerFields(t *testing.T) {
	for _, value := range []string{`"invalid"`, `""`, `"1.5"`, `1.5`, `true`, `{}`, `[]`, `"9223372036854775808"`} {
		var media ChannelsMediaItem
		if err := json.Unmarshal([]byte(`{"fileSize":`+value+`}`), &media); err == nil {
			t.Fatalf("accepted invalid integer %s", value)
		}
	}
}

func TestChannelsMediaLargeFileSize(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("large file sizes require a 64-bit int")
	}
	var media ChannelsMediaItem
	if err := json.Unmarshal([]byte(`{"fileSize":"9007199254740993","decodeKey":12345678901234567890,"url":"https://example.com/video","width":1920,"height":1080}`), &media); err != nil {
		t.Fatal(err)
	}
	if int64(media.FileSize) != 9007199254740993 || media.DecodeKey != "12345678901234567890" || media.Width != 1920 || media.Height != 1080 || media.URL != "https://example.com/video" {
		t.Fatalf("unexpected media: %+v", media)
	}
	data, err := json.Marshal(media)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["fileSize"]) != "9007199254740993" {
		t.Fatalf("fileSize was not serialized as an exact JSON integer: %s", data)
	}
}
