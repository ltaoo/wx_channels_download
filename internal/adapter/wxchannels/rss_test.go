package wxchannelsadapter

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/pkg/scraper/wxchannels"
)

func TestChannelsRSSReturnsFetchedVideos(t *testing.T) {
	routes := NewWebsocketRoutes(0, nil, "")
	defer routes.Stop()
	engine := gin.New()
	engine.GET(ChannelsWebsocketPath, routes.handle_websocket)
	engine.GET("/rss/channels", routes.HandleFetchFeedListOfContactRSS)
	server := httptest.NewServer(engine)
	defer server.Close()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+ChannelsWebsocketPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetReadDeadline(time.Now().Add(5 * time.Second))

	bridge_result := make(chan error, 1)
	go func() {
		var message struct {
			Data struct {
				ID   string                          `json:"id"`
				Key  string                          `json:"key"`
				Body wxchannels.ChannelsFeedListBody `json:"data"`
			} `json:"data"`
		}
		if err := connection.ReadJSON(&message); err != nil {
			bridge_result <- err
			return
		}
		if message.Data.Key != "key:channels:feed_list" || message.Data.Body.Username != "test@finder" || message.Data.Body.NextMarker != "next+page=" {
			bridge_result <- fmt.Errorf("unexpected feed request: %+v", message.Data)
			return
		}
		bridge_result <- connection.WriteJSON(map[string]interface{}{
			"id": message.Data.ID,
			"data": json.RawMessage(`{"errCode":0,"data":{
				"BaseResponse":{"Ret":0},"contact":{"nickname":"作者 & 测试"},
				"userTags":[{"unrelated":"field"}],
				"object":[
					{"id":"14545102784038246591","objectNonceId":"17453311911488030792_0_140_2_0", "createtime":1733911369,
					 "contact":{"username":"test@finder","nickname":"作者 & 测试"},"objectDesc":{"description":"视频 <一> & 内容"}},
					{"id":"14545102784038246592","createtime":1733911379,"source_url":"https://example.com/feed?a=1&b=2"}
				]}}`),
		})
	}()

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rss/channels?username=test%40finder&next_marker=next%2Bpage%3D", nil))
	if err := <-bridge_result; err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/atom+xml") {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
	var feed atom_feed
	if err := xml.Unmarshal(recorder.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Entry) != 2 {
		t.Fatalf("expected two fetched videos in Atom XML, got %d: %s", len(feed.Entry), recorder.Body.String())
	}
	entry := feed.Entry[0]
	if feed.Title != "作者 & 测试" || entry.Title != "视频 <一> & 内容" || entry.Content.Body != entry.Title || entry.Author.Name != feed.Title {
		t.Fatalf("video metadata did not survive XML serialization: %+v", feed)
	}
	if entry.ID != "urn:wxchannels:feed:14545102784038246591" {
		t.Fatalf("video ID lost precision: %s", entry.ID)
	}
	parts, err := wxchannels.ParseFeedURL(entry.Link[0].Href)
	if err != nil || parts.Oid != "14545102784038246591" || parts.Nid != "17453311911488030792" {
		t.Fatalf("invalid video page link: %+v, %v", parts, err)
	}
	if feed.Entry[1].Link[0].Href != "https://example.com/feed?a=1&b=2" || feed.Entry[1].Title != "14545102784038246592" || feed.Entry[1].Author.Name != feed.Title {
		t.Fatalf("missing fallback metadata: %+v", feed.Entry[1])
	}
	if entry.Published != "2024-12-11T10:02:49Z" || feed.Updated != feed.Entry[1].Published {
		t.Fatalf("incorrect publication/update dates: %+v", feed)
	}
	if feed.Link[0].Href != "http://example.com/rss/channels?username=test%40finder&next_marker=next%2Bpage%3D" {
		t.Fatalf("self link lost request parameters: %+v", feed.Link)
	}
}

func TestChannelsRSSRejectsInvalidUpstreamResponses(t *testing.T) {
	for _, raw_response := range []string{
		`invalid JSON`,
		`{"errCode":1011,"errMsg":"not connected"}`,
		`{"errCode":0,"data":{"BaseResponse":{"Ret":-4011,"ErrMsg":{"String":"unavailable"}}}}`,
		`{"errCode":0}`,
	} {
		if _, err := build_channels_atom_feed(json.RawMessage(raw_response), "test@finder", "http://localhost/rss/channels"); err == nil {
			t.Errorf("upstream failure should not become an empty successful feed: %s", raw_response)
		}
	}
	feed, err := build_channels_atom_feed(json.RawMessage(`{"errCode":0,"data":{"BaseResponse":{"Ret":0},"object":[]}}`), "test@finder", "http://localhost/rss/channels")
	if err != nil || len(feed.Entry) != 0 || feed.Author.Name != "test@finder" {
		t.Fatalf("valid empty account should produce an empty feed: %+v, %v", feed, err)
	}
}
