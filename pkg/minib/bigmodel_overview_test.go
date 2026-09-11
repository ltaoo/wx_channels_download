package minib

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"wx_channel/pkg/cookies"
)

// Live check for the bigmodel.cn coding-plan overview usage bars.
//
// The bars are not rendered from a static value: UsageLimitCard tweens
// displayPercents from 0 to the API supplied percentage with a
// requestAnimationFrame loop, so a broken rAF/performance.now clock leaves them
// pinned at width: 0% even though the numbers (2.02万 / 2.8万) render fine.
//
// Skipped unless workdir/cookies.json holds a bigmodel.cn session (and an
// optional BIGMODEL_TOKEN adds the Authorization header when available):
//
//	go test ./pkg/minib/ -run BigmodelOverview -v
func TestBigmodelOverviewUsageBarsRender(t *testing.T) {
	if !bigmodel_cookies_available(t) {
		t.Skip("no bigmodel.cn cookies in workdir/cookies.json")
	}

	browser, err := NewMiniBrowser(3*time.Minute, cookies.NewPersistentReader(bigmodel_workdir))
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	token := strings.TrimSpace(os.Getenv("BIGMODEL_TOKEN"))
	ctx := with_request_header_modifier(context.Background(), func(request *http.Request) error {
		if !strings.HasSuffix(request.URL.Host, "bigmodel.cn") {
			return nil
		}
		if token != "" {
			request.Header.Set("Authorization", token)
		}
		request.Header.Set("Bigmodel-Organization", bigmodel_organization)
		request.Header.Set("Bigmodel-Project", bigmodel_project)
		return nil
	})

	page, err := browser.Navigate(ctx, "https://bigmodel.cn/coding-plan/personal/overview", nil, NavigateOptions{
		ResourceTimeout:   15 * time.Second,
		JavaScriptTimeout: 8 * time.Second,
		DisableImages:     true,
		DisableMedia:      true,
		// A returning Chrome profile has this marker, so the header's
		// InviteNewUser component skips its auto-open first-visit dialog.
		LocalStorage: map[string]string{
			"inviteDialogLastShowTime": strconv.FormatInt(time.Now().UnixMilli(), 10),
		},
	})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if page != nil {
		for _, failure := range page.ScriptFailures {
			t.Logf("script failure %s: %v", failure.URL, failure.Err)
		}
		for _, resource := range page.Resources {
			if resource.Err != nil {
				t.Logf("resource failure %s: %v", resource.URL, resource.Err)
			}
		}
	}

	widths := regexp.MustCompile(`class="progress-bar-value[^"]*" style="width: ([0-9.]+)%`).FindAllStringSubmatch(page.RenderedHTML, -1)
	percents := regexp.MustCompile(`class="percentage-value">([0-9]+)<`).FindAllStringSubmatch(page.RenderedHTML, -1)
	if len(widths) == 0 || len(percents) == 0 {
		t.Fatalf("usage bars did not render: %s", render_excerpt(page.RenderedHTML, "usage-limit-card"))
	}
	if len(widths) != len(percents) {
		t.Fatalf("bar count mismatch: %d widths vs %d percentages", len(widths), len(percents))
	}

	saw_non_zero := false
	for index := range widths {
		width := widths[index][1]
		percent := percents[index][1]
		if width != percent {
			t.Fatalf("bar %d width %s%% does not match rendered percentage %s%%", index, width, percent)
		}
		if value, err := strconv.ParseFloat(width, 64); err == nil && value > 0 {
			saw_non_zero = true
		}
	}
	if !saw_non_zero {
		t.Fatalf("every usage bar is still 0%%, the rAF tween never ran: %s", render_excerpt(page.RenderedHTML, "progress-bar-value"))
	}
	if invocations := strings.Count(page.RenderedHTML, "邀请海报"); invocations != 0 {
		t.Fatalf("first-visit invite dialog opened (%d 邀请海报 markers); localStorage seed did not take effect", invocations)
	}
	if strings.Contains(page.RenderedHTML, ">mmMwWLliI0O&amp;1<") {
		t.Fatalf("iframe font-enumeration probes leaked into the rendered HTML: %s", render_excerpt(page.RenderedHTML, "mmMwWLliI0O"))
	}
	t.Logf("usage bars settled at %v", widths_summary(widths))
}

const (
	bigmodel_organization = "org-0292A93A77f347d4ACd395dda5352DfA"
	bigmodel_project      = "proj_5B3343348E6F487281f0A7C929A347e8"
)

func widths_summary(widths [][]string) []string {
	values := make([]string, 0, len(widths))
	for _, width := range widths {
		values = append(values, width[1]+"%")
	}
	return values
}

// bigmodel_workdir is the application workdir whose cookies.json backs the
// persistent cookie provider.
var bigmodel_workdir = filepath.Join(".", "..", "..", "workdir")

// bigmodel_cookies_available reports whether workdir/cookies.json currently
// holds an unexpired bigmodel.cn session.
func bigmodel_cookies_available(t *testing.T) bool {
	header, err := cookies.NewPersistentReader(bigmodel_workdir).HeaderForDomain("bigmodel.cn")
	if err == nil && strings.TrimSpace(header) != "" {
		return true
	}
	if !errors.Is(err, cookies.ErrCookieNotFound) {
		t.Logf("read bigmodel cookies: %v", err)
	}
	return false
}
