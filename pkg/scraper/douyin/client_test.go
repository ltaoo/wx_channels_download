package douyin

import (
	"encoding/json"
	"testing"
)

func TestWebResultRetainsVideoRepresentations(t *testing.T) {
	var response DouyinWebVideoProfileResp
	if err := json.Unmarshal([]byte(`{"aweme_detail":{"aweme_id":"123","video":{
		"play_addr_h264":{"url_list":["https://media.example/playwm/default.mp4"]},
		"bit_rate":[{"gear_name":"normal_720","bit_rate":1000000,"is_h265":1,
			"play_addr":{"width":720,"height":1280,"data_size":12345,"url_list":["https://media.example/720.mp4"]}}]
	}}}`), &response); err != nil {
		t.Fatal(err)
	}
	info := convert_web_resp(&response)
	if info.URL != "https://media.example/play/default.mp4" || info.Video == nil || len(info.Video.BitRate) != 1 {
		t.Fatalf("web conversion must preserve the default and all representations: %+v", info)
	}
	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var restored VideoInfo
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	stream := restored.Video.BitRate[0]
	if stream.GearName != "normal_720" || stream.PlayAddr.DataSize != 12345 || stream.IsH265 != 1 {
		t.Fatalf("representations must survive JSON serialization: %+v", stream)
	}
}
