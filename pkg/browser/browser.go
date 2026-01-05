package browser

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type OfficialAccountBrowser struct {
	Cookies  []*http.Cookie
	Accounts []*OfficialAccount
}
type OfficialAccount struct {
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatar_url"`
	Biz         string `json:"biz"`
	Uin         string `json:"uin"`
	Key         string `json:"key"`
	PassTicket  string `json:"pass_ticket"`
	AppmsgToken string `json:"appmsg_token"`
}

func NewOfficialAccountBrowser() *OfficialAccountBrowser {
	return &OfficialAccountBrowser{}
}

func (b *OfficialAccountBrowser) BuildURL(uu string, params map[string]string) string {
	u, _ := url.Parse(uu)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	target_url := u.String()
	return target_url
}
func (b *OfficialAccountBrowser) Open(target_url string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", target_url, nil)
	if err != nil {
		// log.Fatal(err)
		return nil, err
	}
	// client := &http.Client{Timeout: 15 * time.Second}
	// req, err := http.NewRequest("GET", target_url, nil)
	// if err != nil {
	// 	return nil, err
	// }
	for _, cookie := range b.Cookies {
		req.AddCookie(cookie)
	}
	// req.Header.Set("Accept", "*/*")
	// req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// req.Header.Set("Priority", "u=1, i")
	// // req.Header.Set("Referer", target_url)
	// req.Header.Set("Sec-Fetch-Dest", "empty")
	// req.Header.Set("Sec-Fetch-Mode", "cors")
	// req.Header.Set("Sec-Fetch-Site", "same-origin")
	// req.Header.Set("Upgrade-Insecure-Requests", "1")
	// req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) MacWechat/3.8.7(0x13080712) UnifiedPCMacWechat(0xf2640619) XWEB/14304 Flue")
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("priority", "u=1, i")
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63090a13) UnifiedPCWindowsWechat(0xf2541022) XWEB/16467 Flue")
	// req.Header.Set("cookie", "rewardsn=; wxtokenkey=777; wxuin=722784289; lang=zh_CN; appmsg_token=1355_7spG0oCbHVG9eaMm5oJrSLP1efMock6mJCLeE5nulTwfch03QdtRtKSxfcmqkiDUENO08tftaMUVrO-c; pass_ticket=69W9uFzA9kEiofxBss4/9yLBbtBgSCxveYsCh8h9yMtpSl6Ppa0VCGUxMxeM5J3x; devicetype=android-29; version=28004252; wap_sid2=CKGg09gCEooBeV9IQTJraVJxcm85LWpFdGtxaU8wLUpWV0Q3YjliQTh2OWpCNVBScGg0SUJsdmdZV2Fra1pJa0RIdlZPUUd1em5ndjhJMjVPeEpKdFhHSlJIZkZSNnFkal84TmpBUGI5amQxRy1LODJRcGNIcjB3TTBxOEk2OHVHeE9UV1dxaV9uNDJpa1RBQUF+MLuh78oGOA1AlU4=")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	cookies := resp.Cookies()
	b.Cookies = cookies
	resp_bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return resp_bytes, nil
}

// 刷新指定公众号的凭证信息
func (b *OfficialAccountBrowser) RefreshAccount(body *OfficialAccount) (*OfficialAccount, error) {
	// https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=MzA5MDM1MTcyNQ==&scene=124&uin=Mjk5Mzg5NDE4Mg==&key=daf9bdc5abc4e8d00f609f167aabb6af06ea4dbc232b89ea3625ff4c8356d231d2253abff286be5d3657e0ae2bbed4fc6a12dd1454c7b92ffd89cf0de5ff414c59393f87751427d77cba09a8a42330e4514dd8830773e8c23a3998093ea6daedde2bb1c297f46752095f66be9ed5a5cec88599695f1ad7cffcedb1f68433e70b&devicetype=UnifiedPCWindows&version=f2541022&lang=zh_CN&a8scene=1&acctmode=0&pass_ticket=5wosi2ITX0uw6quyzee0f9Fj+sGMluU7CwaH1V+dReYpDmcYMjjw7983I839W8FG
	u := "https://mp.weixin.qq.com/mp/profile_ext"
	query := map[string]string{
		"action":      "home",
		"__biz":       body.Biz,
		"scene":       "124",
		"uin":         body.Uin,
		"key":         body.Key,
		"pass_ticket": body.PassTicket,
		"devicetype":  "UnifiedPCWindows",
		"version":     "f2541022",
		"lang":        "zh_CN",
		"a8scene":     "1",
		"acctmode":    "0",
	}
	target_url := b.BuildURL(u, query)
	fmt.Println("[OfficialAccountAPI]refresh account - url", target_url)
	_, err := b.Open(target_url)
	if err != nil {
		return nil, err
	}
	if len(b.Cookies) == 0 {
		return nil, errors.New("refresh account failed, the cookies is empty")
	}
	new_one := &OfficialAccount{
		Nickname:  body.Nickname,
		AvatarURL: body.AvatarURL,
		Biz:       body.Biz,
		Uin:       body.Uin,
		Key:       body.Key,
	}
	for _, ck := range b.Cookies {
		switch ck.Name {
		case "pass_ticket":
			new_one.PassTicket = ck.Value
		case "key":
			// 貌似拿不到新的 key
			new_one.Key = ck.Value
		case "wxuin", "uin":
			if new_one.Uin == "" {
				new_one.Uin = base64.StdEncoding.EncodeToString([]byte(ck.Value))
			}
		}
	}
	return new_one, nil
}

func (b *OfficialAccountBrowser) FetchAccountHomeWithoutToken(body *OfficialAccount) ([]byte, error) {
	query := map[string]string{
		"action": "home",
		"__biz":  body.Biz,
		"scene":  "124",
	}
	target_url := b.BuildURL("https://mp.weixin.qq.com/mp/profile_ext", query)
	return b.Open(target_url)
}

func (b *OfficialAccountBrowser) FetchMsgList(acct *OfficialAccount) ([]byte, error) {
	u := "https://mp.weixin.qq.com/mp/profile_ext"
	query := map[string]string{
		"action":       "getmsg",
		"__biz":        acct.Biz,
		"uin":          "777",
		"key":          "777",
		"pass_ticket":  acct.PassTicket,
		"wxtoken":      "",
		"appmsg_token": acct.AppmsgToken,
		"x5":           "0",
		"count":        "10",
		"offset":       "0",
		"f":            "json",
	}
	target_url := b.BuildURL(u, query)
	// target_url = "https://mp.weixin.qq.com/mp/profile_ext?action=getmsg&__biz=MzI2NDk5NzA0Mw==&uin=777&key=777&pass_ticket=gHG24efz4b9tx5qvHIU6GDehIDaCtu1l914dcm3CQPcWupHu6qFB2UljnxoTI7lS&wxtoken=&appmsg_token=1355_a1sh8%2BPRc5MljS4rpjODrejTXqrrITmMatblkw~~&x5=0&count=10&offset=0&f=json"
	fmt.Printf("[OfficialAccountAPI]fetch msg list - url %s\n", target_url)
	fmt.Println("[OfficialAccountAPI]fetch msg list - cookies", len(b.Cookies))
	resp, err := b.Open(target_url)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (b *OfficialAccountBrowser) CookiesToString() string {
	var cookie_parts []string
	for _, cookie := range b.Cookies {
		cookie_parts = append(cookie_parts, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	return strings.Join(cookie_parts, "; ")
}
