package main

import (
	"fmt"
	"wx_channel/pkg/browser"
)

func main() {
	// {"biz":"MzI2NDk5NzA0Mw==","uin":"NzIyNzg0Mjg5","key":"daf9bdc5abc4e8d03245bfbd038530742d6154cdb778e846ecd1debf3aaf573992baac9354dfe8a1aab494ecc1184cdcfa9dc15f0082cd77b49db29d895cdf35ad007568f481835f6032dfc6c997b676e6d6392d872868eefe1111903b10d1c67d11325a060409ae2871fd12862f917b4e8ea9baa3663ab7763584050c7fa2ba","pass_ticket":"dRNavOowgTQyfK58/U+tHH5NEGl7WaWgdYh0j+kHxyvZRdljfFT7J8ltBwK7kHdK","appmsg_token":"1355_TV7SA7WLHhOzK4wAk_aJtJK1C7oJ2nlK79wSmQ~~"}
	body := &browser.OfficialAccount{
		Biz:         "MzI2NDk5NzA0Mw==",
		Uin:         "NzIyNzg0Mjg5",
		Key:         "daf9bdc5abc4e8d03245bfbd038530742d6154cdb778e846ecd1debf3aaf573992baac9354dfe8a1aab494ecc1184cdcfa9dc15f0082cd77b49db29d895cdf35ad007568f481835f6032dfc6c997b676e6d6392d872868eefe1111903b10d1c67d11325a060409ae2871fd12862f917b4e8ea9baa3663ab7763584050c7fa2ba",
		PassTicket:  "dRNavOowgTQyfK58/U+tHH5NEGl7WaWgdYh0j+kHxyvZRdljfFT7J8ltBwK7kHdK",
		AppmsgToken: "1355_TV7SA7WLHhOzK4wAk_aJtJK1C7oJ2nlK79wSmQ~~",
	}
	browser := browser.NewOfficialAccountBrowser()
	acct, err := browser.RefreshAccount(body)
	if err != nil {
		fmt.Println("Refresh Official Account Failed:", err)
		return
	}
	fmt.Println("Official Account:", acct)
	for _, cookie := range browser.Cookies {
		fmt.Println(cookie.Name, cookie.Value)
	}
	resp_bytes, err := browser.FetchMsgList(body)
	if err != nil {
		fmt.Println("Fetch Msg List Failed:", err)
		return
	}
	println(string(resp_bytes))
}
