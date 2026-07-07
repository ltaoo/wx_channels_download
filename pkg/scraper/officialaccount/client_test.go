package officialaccount

import (
	"net/url"
	"strings"
	"testing"
)

func TestOfficialAccountMergeFromKeepsAuthorID(t *testing.T) {
	acct := &OfficialAccount{
		Biz:      "biz",
		AuthorId: "old_author",
	}
	acct.MergeFrom(&OfficialAccount{
		Nickname: "nickname",
		AuthorId: "new_author",
	})

	if acct.Nickname != "nickname" {
		t.Fatalf("Nickname = %q", acct.Nickname)
	}
	if acct.AuthorId != "new_author" {
		t.Fatalf("AuthorId = %q", acct.AuthorId)
	}
}

func TestBuildMsgListReferer(t *testing.T) {
	acct := &OfficialAccount{
		Biz:        "MzkzNDY0MzE1Nw==",
		Uin:        "12345",
		Key:        "key_value",
		PassTicket: "ticket_value",
	}

	got := (&OfficialAccountClient{}).BuildMsgListReferer(acct)
	if strings.Contains(got, "${") || strings.Contains(got, "refererParams") {
		t.Fatalf("referer contains template text: %q", got)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "mp.weixin.qq.com" || parsed.Path != "/mp/profile_ext" {
		t.Fatalf("referer URL = %q", got)
	}
	if q.Get("action") != "home" {
		t.Fatalf("action = %q", q.Get("action"))
	}
	if q.Get("__biz") != acct.Biz {
		t.Fatalf("__biz = %q", q.Get("__biz"))
	}
	if q.Get("uin") != acct.Uin {
		t.Fatalf("uin = %q", q.Get("uin"))
	}
	if q.Get("key") != acct.Key {
		t.Fatalf("key = %q", q.Get("key"))
	}
	if q.Get("pass_ticket") != acct.PassTicket {
		t.Fatalf("pass_ticket = %q", q.Get("pass_ticket"))
	}
}
