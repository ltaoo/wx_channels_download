package minib

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMainstreamSiteCompatibility(t *testing.T) {
	if os.Getenv("MINIB_CROSS_SITE_LIVE_TEST") == "" {
		t.Skip("set MINIB_CROSS_SITE_LIVE_TEST=1 to run cross-site browser parity checks")
	}
	output_dir := strings.TrimSpace(os.Getenv("MINIB_CROSS_SITE_OUTPUT_DIR"))
	test_cases := []struct {
		name             string
		url              string
		warmup_url       string
		title            string
		markers          []string
		request_marker   string
		ego_html_length  int
		ego_xhr_requests int
	}{
		{name: "self_media_weibo", url: "https://weibo.com/u/2731935637?tabtype=newVideo", warmup_url: "https://www.weibo.com", title: "@田曦薇 的个人主页", markers: []string{"田曦薇", "全部视频"}, ego_html_length: 241494, ego_xhr_requests: 11},
		{name: "video_bilibili", url: "https://www.bilibili.com/v/popular/all", title: "哔哩哔哩热门", markers: []string{"综合热门", "排行榜", `class="video-card"`}, request_marker: "/x/web-interface/popular?", ego_html_length: 436649, ego_xhr_requests: 28},
		{name: "video_youtube_detail", url: "https://www.youtube.com/watch?v=4GtF4TuFps4", title: "Learn English With My Morning Routine (Natural Speaking Practice) - YouTube", markers: []string{"<ytd-watch-flexy", "<ytd-watch-metadata", "Learn English With My Morning Routine (Natural Speaking Practice)", "Miss Honey 🍯"}, ego_html_length: 2405507},
		{name: "podcast_ximalaya", url: "http://www.ximalaya.com/", title: "有声小说,听书,听小说,听故事,听广播 - 喜马拉雅", markers: []string{"喜马拉雅", "猜你喜欢"}, request_marker: "/revision/explore/guessYouLike", ego_html_length: 131225, ego_xhr_requests: 29},
		{name: "official_apple", url: "https://www.apple.com.cn/", title: "Apple (中国大陆) - 官方网站", markers: []string{"Apple", "iPhone"}, ego_html_length: 221164, ego_xhr_requests: 4},
	}
	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			browser, err := NewMiniBrowser(2 * time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			defer browser.Close()
			if test_case.warmup_url != "" {
				if _, err := browser.Navigate(context.Background(), test_case.warmup_url, nil); err != nil {
					t.Fatal(err)
				}
			}
			navigate_options := make([]NavigateOptions, 0, 1)
			if output_dir != "" {
				navigate_options = append(navigate_options, NavigateOptions{CaptureHAR: true})
			}
			page, err := browser.Navigate(context.Background(), test_case.url, nil, navigate_options...)
			if err != nil {
				t.Fatal(err)
			}
			if output_dir != "" {
				if err := os.MkdirAll(output_dir, 0755); err != nil {
					t.Fatal(err)
				}
				html_path := filepath.Join(output_dir, test_case.name+".html")
				if err := page.SaveHTML(html_path); err != nil {
					t.Fatal(err)
				}
				if err := page.SaveHAR(filepath.Join(output_dir, test_case.name+".har")); err != nil {
					t.Fatal(err)
				}
			}
			actual_title := text_content(find_element(page.Document, "title"))
			if actual_title != test_case.title {
				console_messages := page.ConsoleMessages
				if len(console_messages) > 8 {
					console_messages = console_messages[:8]
				}
				runtime_state := ""
				if value, runtime_err := browser.ExecuteJS(context.Background(), `(function(){var root=document.querySelector('#popular-app')||document.querySelector('#award')||document.querySelector('#app');return JSON.stringify({readyState:document.readyState,root:!!root,rootHTML:root&&root.innerHTML.length,vue:!!(root&&root.__vue__),location:location.href})})()`); runtime_err == nil {
					runtime_state = value.String()
				}
				t.Fatalf("title=%q, want %q; html=%d resources=%d scripts=%d xhr=%q fetch=%q failures=%+v console=%q runtime=%s", actual_title, test_case.title, len(page.RenderedHTML), len(page.Resources), page.ExecutedScripts, page.XHRRequests, page.FetchRequests, page.ScriptFailures, console_messages, runtime_state)
			}
			for _, marker := range test_case.markers {
				if !strings.Contains(page.RenderedHTML, marker) {
					runtime_state := ""
					if value, runtime_err := browser.ExecuteJS(context.Background(), `(function(){var app=document.querySelector('ytd-app')||document.querySelector('#app');return JSON.stringify({readyState:document.readyState,app:!!app,appHTML:app&&app.innerHTML.length,appReady:app&&app.__dataReady,appAttached:app&&app.isAttached,watch:!!document.querySelector('ytd-watch-flexy'),bodyText:document.body&&document.body.textContent.length})})()`); runtime_err == nil {
						runtime_state = value.String()
					}
					console_messages := page.ConsoleMessages
					if len(console_messages) > 4 {
						console_messages = console_messages[:4]
					}
					t.Fatalf("rendered HTML missing %q; html=%d resources=%d scripts=%d xhr=%d failures=%+v console=%q runtime=%s", marker, len(page.RenderedHTML), len(page.Resources), page.ExecutedScripts, len(page.XHRRequests), page.ScriptFailures, console_messages, runtime_state)
				}
			}
			if test_case.request_marker != "" {
				request_found := false
				for _, request_url := range append(append([]string{}, page.XHRRequests...), page.FetchRequests...) {
					if strings.Contains(request_url, test_case.request_marker) {
						request_found = true
						break
					}
				}
				if !request_found {
					runtime_state := ""
					if value, runtime_err := browser.ExecuteJS(context.Background(), `(function(){var app=document.querySelector('ytd-app');var appConstructor=customElements&&customElements.get('ytd-app');return JSON.stringify({readyState:document.readyState,title:document.title,app:!!app,appHTML:app&&app.innerHTML.length,appRegistered:!!appConstructor,appInstance:!!(app&&appConstructor&&app instanceof appConstructor),video:!!document.querySelector('video'),watch:!!document.querySelector('ytd-watch-flexy'),bodyText:document.body&&document.body.textContent.length,initialData:!!window.ytInitialData})})()`); runtime_err == nil {
						runtime_state = value.String()
					}
					failures := page.ScriptFailures
					if len(failures) > 8 {
						failures = failures[:8]
					}
					console_messages := page.ConsoleMessages
					if len(console_messages) > 8 {
						console_messages = console_messages[:8]
					}
					t.Fatalf("request missing %q; xhr=%d fetch=%d scripts=%d failures=%+v console=%q runtime=%s", test_case.request_marker, len(page.XHRRequests), len(page.FetchRequests), page.ExecutedScripts, failures, console_messages, runtime_state)
				}
			}
			t.Logf("ego_html=%d minib_html=%d ego_xhr=%d minib_xhr=%d links=%d images=%d resources=%d", test_case.ego_html_length, len(page.RenderedHTML), test_case.ego_xhr_requests, len(page.XHRRequests), len(find_by_tag(page.Document, "a")), len(find_by_tag(page.Document, "img")), len(page.Resources))
		})
	}
}
