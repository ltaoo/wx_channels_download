package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

type fake_wxchannels_runtime struct {
	response json.RawMessage
	err      error

	captured_ctx context.Context
	last_call    string

	keyword     string
	username    string
	next_marker string
	oid         string
	nid         string
	request_url string
	eid         string
	comment_id  string
}

func (r *fake_wxchannels_runtime) record(ctx context.Context, call string) {
	r.captured_ctx = ctx
	r.last_call = call
}

func (r *fake_wxchannels_runtime) SearchContact(ctx context.Context, keyword string, next_marker string) (json.RawMessage, error) {
	r.record(ctx, "SearchContact")
	r.keyword, r.next_marker = keyword, next_marker
	return r.response, r.err
}

func (r *fake_wxchannels_runtime) FeedListOfContact(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	r.record(ctx, "FeedListOfContact")
	r.username, r.next_marker = username, next_marker
	return r.response, r.err
}

func (r *fake_wxchannels_runtime) LiveReplayList(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	r.record(ctx, "LiveReplayList")
	r.username, r.next_marker = username, next_marker
	return r.response, r.err
}

func (r *fake_wxchannels_runtime) FeedProfile(ctx context.Context, oid string, nid string, request_url string, eid string) (json.RawMessage, error) {
	r.record(ctx, "FeedProfile")
	r.oid, r.nid, r.request_url, r.eid = oid, nid, request_url, eid
	return r.response, r.err
}

func (r *fake_wxchannels_runtime) FeedCommentList(ctx context.Context, oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error) {
	r.record(ctx, "FeedCommentList")
	r.oid, r.nid, r.comment_id, r.next_marker = oid, nid, comment_id, next_marker
	return r.response, r.err
}

func (r *fake_wxchannels_runtime) FeedShareUrl(ctx context.Context, oid string) (json.RawMessage, error) {
	r.record(ctx, "FeedShareUrl")
	r.oid = oid
	return r.response, r.err
}

func TestWXChannelsCapabilityValidation(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		invoke  func(*WXChannelsCapability) (json.RawMessage, error)
		wantErr string
	}{
		{
			name: "search_requires_keyword",
			invoke: func(c *WXChannelsCapability) (json.RawMessage, error) {
				return c.SearchContact(ctx, "   ", "")
			},
			wantErr: "keyword 不能为空",
		},
		{
			name: "feed_list_requires_username",
			invoke: func(c *WXChannelsCapability) (json.RawMessage, error) {
				return c.FeedListOfContact(ctx, "", "")
			},
			wantErr: "username 不能为空",
		},
		{
			name: "live_replay_requires_username",
			invoke: func(c *WXChannelsCapability) (json.RawMessage, error) {
				return c.LiveReplayList(ctx, "  ", "")
			},
			wantErr: "username 不能为空",
		},
		{
			name: "feed_profile_requires_selector",
			invoke: func(c *WXChannelsCapability) (json.RawMessage, error) {
				return c.FeedProfile(ctx, " ", "", "  ", "")
			},
			wantErr: "oid、url 和 eid 至少需要提供一个",
		},
		{
			name: "feed_comment_requires_oid",
			invoke: func(c *WXChannelsCapability) (json.RawMessage, error) {
				return c.FeedCommentList(ctx, "", "nid", "comment", "")
			},
			wantErr: "oid 不能为空",
		},
		{
			name: "feed_comment_requires_nid_or_comment_id",
			invoke: func(c *WXChannelsCapability) (json.RawMessage, error) {
				return c.FeedCommentList(ctx, "oid", " ", "", "")
			},
			wantErr: "nid 和 comment_id 至少需要提供一个",
		},
		{
			name: "feed_share_url_requires_oid",
			invoke: func(c *WXChannelsCapability) (json.RawMessage, error) {
				return c.FeedShareUrl(ctx, "")
			},
			wantErr: "oid 不能为空",
		},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			runtime := &fake_wxchannels_runtime{}
			_, err := test_case.invoke(NewWXChannelsCapability(runtime))
			if err == nil {
				t.Fatalf("期望返回 %q，实际为 nil", test_case.wantErr)
			}
			if err.Error() != test_case.wantErr {
				t.Errorf("错误文本 = %q, 期望 %q", err.Error(), test_case.wantErr)
			}
			if runtime.last_call != "" {
				t.Errorf("校验失败仍调用了 runtime: %s", runtime.last_call)
			}
		})
	}
}

func TestWXChannelsCapabilityTrimsAndForwards(t *testing.T) {
	runtime := &fake_wxchannels_runtime{response: json.RawMessage(`{"ok":true}`)}
	capability := NewWXChannelsCapability(runtime)
	ctx := context.WithValue(context.Background(), struct{}{}, "marker")

	if _, err := capability.SearchContact(ctx, "  keyword  ", "  cursor  "); err != nil {
		t.Fatalf("SearchContact 返回错误: %v", err)
	}
	if runtime.captured_ctx != ctx {
		t.Errorf("context 未透传到 runtime")
	}
	if runtime.keyword != "keyword" || runtime.next_marker != "  cursor  " {
		t.Errorf("参数 = %q/%q, 期望 keyword/未 trim 的 cursor", runtime.keyword, runtime.next_marker)
	}
}

// TestWXChannelsCapabilityPreservesRawMessageBytes pins the fidelity contract:
// identifiers such as decode_key are 64-bit integers, and decoding a response
// into map[string]any turns them into float64. The capability must hand the
// runtime's bytes back untouched.
func TestWXChannelsCapabilityPreservesRawMessageBytes(t *testing.T) {
	raw := json.RawMessage(`{"data":{"id":7234567890123456789,"decode_key":"18446744073709551615"}}`)
	runtime := &fake_wxchannels_runtime{response: raw}
	got, err := NewWXChannelsCapability(runtime).FeedShareUrl(context.Background(), "oid")
	if err != nil {
		t.Fatalf("FeedShareUrl 返回错误: %v", err)
	}
	if !reflect.DeepEqual(got, raw) {
		t.Errorf("RawMessage 字节被修改\n实际: %s\n期望: %s", got, raw)
	}
	var as_map map[string]any
	if err := json.Unmarshal(got, &as_map); err != nil {
		t.Fatalf("Unmarshal 失败: %v", err)
	}
	if _, corrupted := as_map["data"].(map[string]any)["id"].(float64); !corrupted {
		t.Fatalf("测试前提失效: id 未按 float64 解析，无法证明保真设计")
	}
}

func TestWXChannelsCapabilityFeedProfileNormalization(t *testing.T) {
	cases := []struct {
		name        string
		oid         string
		nid         string
		request_url string
		eid         string
		want_oid    string
		want_nid    string
		want_url    string
		want_eid    string
	}{
		{
			name:        "url_eid_becomes_eid",
			request_url: "https://example.com/feed?eid=ENCRYPTED&x=1",
			want_eid:    "ENCRYPTED",
		},
		{
			name:        "oid_and_nid_clear_url",
			oid:         "OID",
			nid:         "NID",
			request_url: "https://example.com/feed",
			want_oid:    "OID",
			want_nid:    "NID",
		},
		{
			name:        "plain_url_stays_url",
			request_url: "https://example.com/feed",
			want_url:    "https://example.com/feed",
		},
		{
			name:     "eid_only_unchanged",
			eid:      "ENCRYPTED",
			want_eid: "ENCRYPTED",
		},
		{
			name:        "explicit_eid_wins_over_url",
			request_url: "https://example.com/feed?eid=FROM_URL",
			eid:         "EXPLICIT",
			want_url:    "https://example.com/feed?eid=FROM_URL",
			want_eid:    "EXPLICIT",
		},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			runtime := &fake_wxchannels_runtime{}
			_, err := NewWXChannelsCapability(runtime).FeedProfile(
				context.Background(),
				test_case.oid,
				test_case.nid,
				test_case.request_url,
				test_case.eid,
			)
			if err != nil {
				t.Fatalf("FeedProfile 返回错误: %v", err)
			}
			if runtime.oid != test_case.want_oid ||
				runtime.nid != test_case.want_nid ||
				runtime.request_url != test_case.want_url ||
				runtime.eid != test_case.want_eid {
				t.Errorf(
					"规范化参数 = %q/%q/%q/%q, 期望 %q/%q/%q/%q",
					runtime.oid, runtime.nid, runtime.request_url, runtime.eid,
					test_case.want_oid, test_case.want_nid, test_case.want_url, test_case.want_eid,
				)
			}
		})
	}
}

func TestWXChannelsCapabilityUninitialized(t *testing.T) {
	var capability *WXChannelsCapability
	if _, err := capability.FeedShareUrl(context.Background(), "oid"); err == nil {
		t.Fatal("nil capability 期望返回错误，实际为 nil")
	}
	if _, err := NewWXChannelsCapability(nil).SearchContact(context.Background(), "keyword", ""); err == nil {
		t.Fatal("nil runtime 期望返回错误，实际为 nil")
	}
}

type fake_wxmp_runtime struct {
	response     json.RawMessage
	captured_ctx context.Context
	username     string
	offset       string
}

func (r *fake_wxmp_runtime) BizMsgList(ctx context.Context, username string, offset string) (json.RawMessage, error) {
	r.captured_ctx = ctx
	r.username, r.offset = username, offset
	return r.response, nil
}

func TestWXMPCapabilityValidationAndForwarding(t *testing.T) {
	runtime := &fake_wxmp_runtime{response: json.RawMessage(`{"list":[]}`)}
	capability := NewWXMPCapability(runtime)
	if _, err := capability.BizMsgList(context.Background(), "   ", "0"); err == nil {
		t.Fatal("空 username 期望返回错误，实际为 nil")
	}
	if runtime.username != "" {
		t.Fatalf("校验失败仍调用了 runtime: username=%q", runtime.username)
	}

	ctx := context.WithValue(context.Background(), struct{}{}, "marker")
	raw, err := capability.BizMsgList(ctx, "  biz  ", "  cursor  ")
	if err != nil {
		t.Fatalf("BizMsgList 返回错误: %v", err)
	}
	if runtime.captured_ctx != ctx {
		t.Errorf("context 未透传到 runtime")
	}
	if runtime.username != "biz" || runtime.offset != "cursor" {
		t.Errorf("参数 = %q/%q, 期望 biz/cursor", runtime.username, runtime.offset)
	}
	if string(raw) != `{"list":[]}` {
		t.Errorf("RawMessage = %s, 期望原样返回", raw)
	}
}

func TestWXMPCapabilityUninitialized(t *testing.T) {
	var capability *WXMPCapability
	if _, err := capability.BizMsgList(context.Background(), "biz", ""); err == nil {
		t.Fatal("nil capability 期望返回错误，实际为 nil")
	}
	if _, err := NewWXMPCapability(nil).BizMsgList(context.Background(), "biz", ""); err == nil {
		t.Fatal("nil runtime 期望返回错误，实际为 nil")
	}
}
