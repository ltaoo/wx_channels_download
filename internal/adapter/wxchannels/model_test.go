package wxchannelsadapter

import (
	"encoding/json"
	"testing"

	"wx_channel/pkg/scraper/wxchannels"
)

func TestResolveVideoDownloadSpecUsesLevelOrderForDefaultHighest(t *testing.T) {
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal([]byte(`{"objectDesc":{"media":[{"spec":[
		{"fileFormat":"xWR112","levelOrder":200,"videoBitrate":3200},
		{"fileFormat":"xWR111","levelOrder":100,"videoBitrate":2340},
		{"fileFormat":"xWR113","levelOrder":300,"videoBitrate":2600}
	]}]}}`), &obj); err != nil {
		t.Fatalf("unmarshal issue reproduction: %v", err)
	}

	if got := resolve_video_download_spec(&obj, nil, true); got != "xWR111" {
		t.Fatalf("defaultHighest selected %q, want xWR111 (best levelOrder despite lower bitrate)", got)
	}
	if got := resolve_video_download_spec(&obj, nil, false); got != "xWR112" {
		t.Fatalf("default selection changed to %q, want first listed spec xWR112", got)
	}
}

func TestResolveVideoDownloadSpecFallsBackToOriginalWithoutRankedSpecs(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		Spec: []wxchannels.ChannelsMediaSpec{{FileFormat: "xWR111", VideoBitrate: 2340}},
	}
	if got := resolve_video_download_spec(obj, nil, true); got != "" {
		t.Fatalf("defaultHighest selected %q without levelOrder metadata, want original request", got)
	}
}
