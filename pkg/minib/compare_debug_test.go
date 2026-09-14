package minib

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestComparePageDebug(t *testing.T) {
	browser, err := NewMiniBrowser(2 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), "http://127.0.0.1:18765/compare.html?case=storage-wait-pick-up-order-mld-uwp%2Fmld__uwp%2Fstorage-wait-pick-up-order%2Ffill-all-fields-query%2Freset-request", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page.RenderedHTML, "WMS 快照对比") {
		t.Skip("compare server is unavailable on 127.0.0.1:18765")
	}
	_ = os.WriteFile("/tmp/compare_page_rendered.html", []byte(page.RenderedHTML), 0600)
	body := page.RenderedHTML
	if index := strings.Index(body, "<body"); index >= 0 {
		body = body[index:]
	}
	if len(body) > 600 {
		body = body[:600]
	}
	t.Logf("rendered bytes=%d scripts=%d resources=%d body_prefix=%s", len(page.RenderedHTML), page.ExecutedScripts, len(page.Resources), body)
	for _, failure := range page.ScriptFailures {
		t.Logf("script failure %s: %v", failure.URL, failure.Err)
	}
	for _, message := range page.ConsoleMessages {
		t.Logf("console: %s", strings.SplitN(message, "\n", 2)[0])
	}
	for _, resource := range page.Resources {
		if resource.Err != nil {
			t.Logf("resource failure %s: %v", resource.URL, resource.Err)
		}
	}
}
