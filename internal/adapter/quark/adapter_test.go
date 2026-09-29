package quarkadapter

import (
	"encoding/json"
	"testing"
	"wx_channel/pkg/scraper/quark"
)

func TestBuildDownloadTaskFromFetchKeepsURLsLazy(t *testing.T) {
	share := &quark.Share{
		PwdID:  "share",
		Stoken: "token",
		Title:  "share",
		Files: []quark.File{{
			Fid:           "file",
			Name:          "a.mp4",
			ShareFidToken: "file-token",
		}},
	}

	result, err := NewQuarkAdapter().BuildDownloadTaskFromFetch(
		share,
		json.RawMessage("{}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Resources) != 1 || len(result.Resources[0].Endpoints) != 1 {
		t.Fatalf("unexpected resource graph: %#v", result.Resources)
	}
	if result.Resources[0].Endpoints[0].URL != "" {
		t.Fatal("preview should not resolve signed download URLs")
	}
}
