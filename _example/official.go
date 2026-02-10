package main

import (
	"fmt"
	"os"

	officialaccountdownload "github.com/GopeedLab/gopeed/pkg/officialaccount"
)

func main() {
	client := &officialaccountdownload.OfficialAccountDownload{}

	// url := "https://mp.weixin.qq.com/s/dlWe4sStQMKhKXphf8kC2Q"
	// url := "https://mp.weixin.qq.com/s/tEeZD35wa92VzEQmN6zXDg"
	url := "https://mp.weixin.qq.com/s/906EyV6zAvdCPTUFn2oehw"
	article, err := client.FetchArticle(url)
	if err != nil {
		fmt.Println(err)
		return
	}
	content, err := client.BuildHTMLFromArticle(article, false)
	if err != nil {
		fmt.Println(err)
		return
	}
	os.WriteFile("article.html", []byte(content), 0644)
}
