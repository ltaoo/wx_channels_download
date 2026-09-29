package minib

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The bigmodel.cn coding-plan overview renders its usage bars through a cubic
// ease-out tween that reads performance.now() and drives itself with
// requestAnimationFrame timestamps. When the two clocks disagree the tween
// either never advances or never starts, and the bar freezes at width: 0%.
const tween_page = `<!doctype html>
<html><body>
<div id="bar" style="width: 0%"></div>
<script>
function tween(from, to, duration, onUpdate) {
  var start = performance.now();
  var frames = 0;
  window.__frameTypes = [];
  function frame(timestamp) {
    window.__frameTypes.push(typeof timestamp);
    frames++;
    var progress = Math.min((timestamp - start) / duration, 1);
    var eased = 1 - Math.pow(1 - progress, 3);
    onUpdate(Math.round(from + (to - from) * eased));
    if (progress < 1) {
      requestAnimationFrame(frame);
    }
  }
  requestAnimationFrame(frame);
}
tween(0, 72, 900, function(value) {
  document.getElementById('bar').style.width = value + '%';
  document.body.setAttribute('data-tween', String(value));
});
</script>
</body></html>`

func TestAnimationFrameTweenSettlesAtTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response_writer.Write([]byte(tween_page))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page.RenderedHTML, `data-tween="72"`) {
		t.Fatalf("tween did not settle at its target: %s", render_excerpt(page.RenderedHTML, "data-tween"))
	}
	if !strings.Contains(page.RenderedHTML, `width: 72%`) {
		t.Fatalf("animated width was not applied: %s", render_excerpt(page.RenderedHTML, "progress-bar"))
	}
	if strings.Contains(page.RenderedHTML, "NaN") {
		t.Fatalf("tween computed NaN, so rAF timestamps do not share the performance.now() clock")
	}
}

// The bigmodel quota response lands seconds after navigation, by which point
// ordinary timers have already spent the virtual clock budget. The tween has to
// start (and finish) from there instead of freezing at its initial value.
const late_tween_page = `<!doctype html>
<html><body>
<div id="bar" style="width: 0%"></div>
<script>
var clockBurner = setInterval(function() {}, 20);
function startTween() {
  var start = performance.now();
  document.body.setAttribute('data-start', String(start));
  function frame(timestamp) {
    var progress = Math.min((timestamp - start) / 900, 1);
    var eased = 1 - Math.pow(1 - progress, 3);
    var value = Math.round(72 * eased);
    document.getElementById('bar').style.width = value + '%';
    document.body.setAttribute('data-tween', String(value));
    if (progress < 1) {
      requestAnimationFrame(frame);
    }
  }
  requestAnimationFrame(frame);
}
setTimeout(function() {
  var request = new XMLHttpRequest();
  request.open('GET', '/quota', true);
  request.onload = startTween;
  request.send();
}, 3000);
</script>
</body></html>`

func TestAnimationFrameTweenStartedAfterLateResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/quota" {
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = response_writer.Write([]byte(`{"limits":[]}`))
			return
		}
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response_writer.Write([]byte(late_tween_page))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	started_at := attribute_value(page.RenderedHTML, "data-start")
	if started_at < 3000 {
		t.Fatalf("tween started at %dms, expected it to start after the timer budget was spent", started_at)
	}
	t.Logf("tween started at %dms of virtual time", started_at)
	if !strings.Contains(page.RenderedHTML, `data-tween="72"`) {
		t.Fatalf("late tween did not settle at its target: %s", render_excerpt(page.RenderedHTML, "data-tween"))
	}
}

// bigmodel_production_tween is copied verbatim from the shipped webpack chunk
// claude-usage~glm-coding-ent-usage-member~glm-coding-ent-usage-stats~subscribe-overview.b391df3e.js
// (module a967, export z). It is the exact function that animates
// UsageLimitCard.displayPercents from 0 to the API percentage, so this test
// breaks for the same reason the real page does.
const bigmodel_production_tween = `function J(t,n,e,r){var a=null,i=performance.now(),u=function(o){var l=Math.min((o-i)/e,1),c=1-Math.pow(1-l,3);r(t+(n-t)*c,1===l),l<1&&(a=requestAnimationFrame(u))};return a=requestAnimationFrame(u),function(){return cancelAnimationFrame(a)}}`

// bigmodel_production_tween_page mirrors how the real page uses J: the quota
// response arrives seconds after navigation, then each card tweens 0 -> the
// percentage returned by /api/monitor/usage/quota/limit (49 in this fixture).
const bigmodel_production_tween_page = `<!doctype html>
<html><body>
<div id="bar" class="progress-bar-value is-usage-blue" style="width: 0%"></div>
<script>
` + bigmodel_production_tween + `
var burner = setInterval(function() {}, 20);
setTimeout(function() {
  J(0, 49, 900, function(value, done) {
    var rounded = Math.round(value);
    document.getElementById('bar').style.width = rounded + '%';
    document.body.setAttribute('data-tween', String(rounded));
    document.body.setAttribute('data-done', String(done));
  });
}, 4000);
</script>
</body></html>`

func TestBigmodelProductionTweenSettles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response_writer.Write([]byte(bigmodel_production_tween_page))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page.RenderedHTML, "NaN") {
		t.Fatalf("production tween computed NaN: %s", render_excerpt(page.RenderedHTML, "data-tween"))
	}
	if !strings.Contains(page.RenderedHTML, `data-tween="49"`) {
		t.Fatalf("production tween did not settle at the API percentage: %s", render_excerpt(page.RenderedHTML, "data-tween"))
	}
	if !strings.Contains(page.RenderedHTML, `data-done="true"`) {
		t.Fatalf("production tween never reported completion: %s", render_excerpt(page.RenderedHTML, "data-done"))
	}
	if !strings.Contains(page.RenderedHTML, `width: 49%`) {
		t.Fatalf("bar width was never updated: %s", render_excerpt(page.RenderedHTML, "progress-bar-value"))
	}
}

func TestAnimationFramePassesHighResTimestamp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response_writer.Write([]byte(`<!doctype html><html><body><script>
var first = null;
requestAnimationFrame(function(ts) {
  first = ts;
  document.body.setAttribute('data-type', typeof ts);
  document.body.setAttribute('data-value', String(ts));
});
</script></body></html>`))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page.RenderedHTML, `data-type="number"`) {
		t.Fatalf("rAF callback did not receive a numeric timestamp: %s", render_excerpt(page.RenderedHTML, "data-type"))
	}
}

func attribute_value(html string, name string) int64 {
	marker := name + `="`
	index := strings.Index(html, marker)
	if index < 0 {
		return -1
	}
	rest := html[index+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return -1
	}
	value, err := strconv.ParseInt(rest[:end], 10, 64)
	if err != nil {
		return -1
	}
	return value
}

func render_excerpt(html string, marker string) string {
	index := strings.Index(html, marker)
	if index < 0 {
		return html
	}
	start := index - 120
	if start < 0 {
		start = 0
	}
	end := index + 120
	if end > len(html) {
		end = len(html)
	}
	return html[start:end]
}
