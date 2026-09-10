package minib

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"wx_channel/pkg/cookies"
)

func TestStarlinkPipelineExecutionDebug(t *testing.T) {
	browser, err := NewMiniBrowser(2*time.Minute, cookies.NewPersistentReader("/tmp/starlink-cookies"))
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), "https://starlink.mayfair-inc.com/app/x-uwp-ui/pipelineExecution/1125ed71-118d-4c2a-a40b-0aa87db7bb42#01M25DEFWVPVA9Z63YD1SENK5P", nil)
	if err != nil {
		t.Fatal(err)
	}
	body := page.RenderedHTML
	if index := strings.Index(body, "<body"); index >= 0 {
		body = body[index:]
	}
	if len(body) > 1000 {
		body = body[:1000]
	}
	t.Logf("rendered bytes=%d scripts=%d resources=%d body_prefix=%s", len(page.RenderedHTML), page.ExecutedScripts, len(page.Resources), body)
	_ = os.WriteFile("starlink_pipeline_rendered.html", []byte(page.RenderedHTML), 0600)
	for _, resource := range page.Resources {
		if len(resource.Body) > 0 {
			_ = os.WriteFile("/tmp/starlink_res_"+strings.ReplaceAll(strings.ReplaceAll(resource.URL, "https://", ""), "/", "_"), resource.Body, 0600)
		}
	}
	for _, failure := range page.ScriptFailures {
		t.Logf("script failure %s: %s", failure.URL, strings.SplitN(failure.Err.Error(), "\n", 2)[0])
	}
	for _, message := range page.ConsoleMessages {
		t.Logf("console: %s", message)
	}
	for _, resource := range page.Resources {
		if resource.Err != nil {
			t.Logf("resource failure %s: %v", resource.URL, resource.Err)
		}
	}
}
