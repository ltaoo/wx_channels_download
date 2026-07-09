package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	// "os"
	"strings"
	"time"
)

var c = "_zap=e4a7215a-4c23-4a57-9ab1-10a54f9852cd; d_c0=tvATXFSdzRqPTvpUelCjUpmZAoRG9GpZPpk=|1753255158; __snaker__id=C4YRGfhK1oDmUjE0; q_c1=c7f1917f442340c4b1e15a4ba65a2bd5|1753255178000|1753255178000; _xsrf=vR6qBslDfAMyDygZ6arwFI4TvbyNjtaK; edu_user_uuid=edu-v1|f3dda443-085b-4b2d-ac40-e1868e16c059; HMACCOUNT=B52A63979FC81C9E; captcha_session_v2=2|1:0|10:1778467080|18:captcha_session_v2|88:YmRnL0t6YnZjVWFlTWFZY3lUVlBNQTduWVNBQ3hqVzZ1cEtvalpwK3l3M05YWWU4Q1BqS0NvV3ZDc1FzZlltcQ==|cd1cbdb150ed2d9c105f7a625492a6c1a8dcf7d8fd4310ed83e92e97210fa53d; gdxidpyhxdE=VnmrT1PsvEeYKxK1CSWfDGqISG%2FlmUAan6S970%2B%5CaqwmvPu%2FZ4G10Jeve2v93nohdTn%2FQsCfrEe%5CeZoQj4%2B70dE1JOYSNYRU7NWo2o1oLjgEX0kaMb3HutVwS7TaLSbTcP%2BmfKOBeskHoL3qoYdktVuHWweaBGRNuKGjoZW0LZNe8kOb%3A1778467981404; DATE=1753255159128; cmci9xde=U2FsdGVkX19Cm8dLQ0tHGFgrHeuIPQ5534v6kc8ZUXQI3FgKzmukjYATmEJ929cSgQ34dOtYX2uqVKyq+PaeoA==; pmck9xge=U2FsdGVkX18/aJa1X28n34kp7NJ7E6ea2EOQe6hF+UU=; crystal=U2FsdGVkX1+qdcaJtB1h/igfovly9kzXU4qHOcOuQ3kmmyJ9C+p2OPl91Y/JrKA6TzC9Hw/aAT6mgArYvRb9S0xOIX0BeQDNz4QWIF59dvq9L7tnIqEkRAxWGYCfeCAK56X6AD/41UVACBBOHyqjdcwoCnZLZu4DelUivFnCJ2KqHjr4lM9BtgrHOOyg90Rf6X1j+1/EeenWmlg5nZGJRLLBdXWi9h4mUouF65bt3dn2ZnkT87jx1gFmdrv+tBOJ; assva6=U2FsdGVkX1+0DNOd0dkBhptAn+/4jsS3SzW4Oz/mIhg=; assva5=U2FsdGVkX19jMX8UpUNa0IuKiOXOVVukMghEodeHUqD6bsIaC7EkvUIN/jxzNOAAH3O/5v6FeUM+HYDDsx3bQg==; vmce9xdq=U2FsdGVkX1+plTpX0piWJB7gjjl5cNn4cNlUShMVWQAUKoBAxMKFJTzReXlTTxDx86dcdYmRpWZEgbStJ8RNol+l+Jdo062Jukr0ItbpW/w88fayGDp5CgvMpVEhjJzLYbpd2yxR46+HCtVTRzgj7Co96kG8wtxt7nskZNyz1PI=; captcha_ticket_v2=2|1:0|10:1778467098|17:captcha_ticket_v2|728:eyJ2YWxpZGF0ZSI6IkNOMzFfY2c0UipBZlp2MVpxekYqOVkqU1NLd19pV2J4b1NCOFEyajM0ekNKVU54R2gxd19mcTZOczZ6VDB0UVcqc1hJNHNBOVhwUGpOdGRQYWpoMmcxOWthbVA0WDZUcGZWS0o2TlFXekVIVlpNT3laYmY1bGV6T0JLbi5OWnBCMzhvYXNXQiphWlVxcFhjeDJlUW9JR1pPalFPUFBma1ZwVWlzLkdyNnRTYkJNY3hYLkVNVWNMWUpXaFAxS0xfR3ZXKmkuNGIqQ1JfbTQ1M2cyRktQQWVmZXl1OXl5NEI2M05EUDRJWWs1LkdQbWtCVkcwajJrVWtMcFJ2cEVEXzZ0RWFuQWdlQXlfT3NHVXZKeGRiRy5UbHVDRGYybDVicVVWV2ZfWURNNnB2bkFUWkg4Z29sOEVfaXlKZ3VHRkgzZWFBd0sxTlVmY1l2eVhyTzlwdmt4VVpGT0RoTVZzc0ZtT3EqODNzdmI1UypCSFhIbE9Wek5UYWRWMlFjcG94ZVV1V0RtTXV0dUs5dFVRamhpLjJWUGFVd0JkXzVQY0tTRzkuSDAuSnhRQ0M0SkZ5Tnp6alAyaWJPaE9nSThITmpQRlFSKndFbHJMajNPcVltcHN5U1YqNjE4WE42bjk4dmJRTk1scl94UVYxQ1ZQckdzb1NQbnE5ci5KZDk0T2tjVEJvMXhJc2xZUlk3N192X2lfMSJ9|5275c778c59c757b825ae084b683bd3ad369e76b384b4a7bb90c3e40db6cf988; Hm_lvt_98beee57fd2ef70ccdd5ca52b9740c49=1780039329; __zse_ck=005_22IMgkJmNKjSeduT7rK9/iX3poIiL39=/i1DHo85eUxrQtQwTvgicn9KQ/P9eFn4tNrn2SczUlLv/SG6aNKQWmlwV64MZ=ngS28nTlqW2Uz0hdbLT3uXBWm0SQ0dEXEw-H1XFayBtf9+u8emz5jR5ez7LDIUy2UOePoj41G/m+G5Gda9BtYFE0tLsJDuhx1G5jBWaoc5PTNLb7Biv/s4gs6gFZn6I3gWwmsVpGnoc3Y/BqAdZktX4ZIGd7tYYDZZW; z_c0=2|1:0|10:1780294191|4:z_c0|92:Mi4xRGYxYUJ3QUFBQUMyOEJOY1ZKM05HaVlBQUFCZ0FsVk5MM0FLYXdBVkRTdzVNcmliM2ktb1hMbzFXZHFRT001OFRR|282632340a88b63ca07e70deb9f5c880733102461fe52de116fe147dc9518163; SESSIONID=EeIL1ylxIPTh9mo1MBuKSRCQxrMYwjELkrlKYngY34J; JOID=V1wTA088wpN7XxJcP9diw8IJipEmeKXcM29qMkF0k8UVYn4yDOgBjhRfEVg9rLKdTmq_iMiVWPZT4SJW1u3Eh-U=; osd=UF0TBE87w5N8XxVdP9BixMMJjZEheaXbM2hrMkZ0lMQVZX41DegGjhNeEV89q7OdSWq4iciSWPFS4SVW0ezEgOU=; Hm_lpvt_98beee57fd2ef70ccdd5ca52b9740c49=1780472276; BEC=51ad4fb09a26f47ed033b85d422ac8dd"

func main() {
	accountID := flag.String("account", "e40c4495e66937d3e085ff1b5f2d03a6", "zhihu member hash id")
	cookie := flag.String("cookie", c, "zhihu cookie, or set ZHIHU_COOKIE")
	bodyLimit := flag.Int64("body-limit", 8192, "max response body bytes to print")
	// accountID := "e40c4495e66937d3e085ff1b5f2d03a6"
	// cookie := ""
	// bodyLimit := 8192
	flag.Parse()

	id := strings.TrimSpace(*accountID)
	if id == "" {
		log.Fatal("missing -account")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://api.zhihu.com/people/"+url.PathEscape(id), nil)
	if err != nil {
		log.Fatal(err)
	}
	setZhihuPeopleHeaders(req)
	if strings.TrimSpace(*cookie) != "" {
		req.Header.Set("cookie", strings.TrimSpace(*cookie))
	}

	fmt.Println("Request:", req.Method, req.URL.String())
	fmt.Println("Has cookie:", req.Header.Get("cookie") != "")
	fmt.Println()

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Println("Status:", resp.Status)
	fmt.Println("Final URL:", resp.Request.URL.String())
	fmt.Println()
	fmt.Println("Response headers:")
	for name, values := range resp.Header {
		fmt.Printf("%s: %s\n", name, strings.Join(values, "; "))
	}
	fmt.Println()
	fmt.Printf("Body, first %d bytes:\n", *bodyLimit)
	body, err := io.ReadAll(io.LimitReader(resp.Body, *bodyLimit))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(body))
}

func setZhihuPeopleHeaders(req *http.Request) {
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("priority", "u=0, i")
	req.Header.Set("sec-ch-ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-fetch-dest", "document")
	req.Header.Set("sec-fetch-mode", "navigate")
	req.Header.Set("sec-fetch-site", "none")
	req.Header.Set("sec-fetch-user", "?1")
	req.Header.Set("upgrade-insecure-requests", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
}
