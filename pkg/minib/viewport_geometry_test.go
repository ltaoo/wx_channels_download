package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// viewport_geometry_page mimics how an element-plus el-table sizes itself: the
// root element's clientWidth distributes flex columns proportionally, and when
// the measured width cannot fit the configured minimums every column falls
// back to its minimum and the table scrolls instead. minib used to report 0
// for every element box, which pinned tables to their minimum width.
const viewport_geometry_page = `<!doctype html><html><head></head><body>
<div class="el-table"><table><colgroup></colgroup></table></div>
<script>
var root = document.querySelector('.el-table');
var mins = [120, 120, 120, 120];
var total = 0;
for (var i = 0; i < mins.length; i++) total += mins[i];
var bodyWidth = root.clientWidth;
var widths = mins;
var tableWidth = total;
if (bodyWidth > total) {
  widths = mins.map(function(w) { return Math.floor(w * bodyWidth / total); });
  tableWidth = bodyWidth;
}
var table = document.querySelector('table');
table.style.width = tableWidth + 'px';
document.querySelector('colgroup').innerHTML = widths.map(function(w) { return '<col width="' + w + '">'; }).join('');
var rect = root.getBoundingClientRect();
var detached = document.createElement('div');
var plain = document.createElement('div');
document.body.appendChild(plain);
if (typeof ResizeObserver === 'function') {
  new ResizeObserver(function(entries) {
    document.body.setAttribute('data-ro-width', entries[0].contentRect.width);
  }).observe(root);
}
document.body.setAttribute('data-geometry', JSON.stringify({
  rootClientWidth: root.clientWidth,
  rootOffsetWidth: root.offsetWidth,
  rootScrollWidth: root.scrollWidth,
  rootClientHeight: root.clientHeight,
  docClientWidth: document.documentElement.clientWidth,
  docClientHeight: document.documentElement.clientHeight,
  innerWidth: window.innerWidth,
  visualWidth: window.visualViewport ? window.visualViewport.width : -1,
  visualHeight: window.visualViewport ? window.visualViewport.height : -1,
  rectWidth: rect.width,
  rectRight: rect.right,
  detachedClientWidth: detached.clientWidth,
  detachedRectWidth: detached.getBoundingClientRect().width,
  plainClientHeight: plain.clientHeight,
  tableWidth: tableWidth,
  colWidths: widths.join(',')
}));
</script></body></html>`

func navigate_geometry_page(t *testing.T, server *httptest.Server, options NavigateOptions) map[string]any {
	t.Helper()
	browser, err := NewMiniBrowser(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	encoded, err := browser.ExecuteJS(context.Background(), `document.body.getAttribute('data-geometry')`)
	if err != nil {
		t.Fatal(err)
	}
	if encoded == nil || encoded.String() == "" {
		t.Fatal("page did not record geometry state")
	}
	state, err := browser.ExecuteJS(context.Background(), "(function(){ return "+encoded.String()+"; })()")
	if err != nil {
		t.Fatal(err)
	}
	exported, ok := state.Export().(map[string]any)
	if !ok {
		t.Fatalf("geometry state has unexpected type %T", state.Export())
	}
	return exported
}

func geometry_server(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, viewport_geometry_page)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestNavigateViewportGeometryPC(t *testing.T) {
	state := navigate_geometry_page(t, geometry_server(t), NavigateOptions{Device: DevicePC})
	for _, field := range []string{"rootClientWidth", "rootOffsetWidth", "rootScrollWidth", "docClientWidth", "innerWidth", "visualWidth", "rectWidth", "rectRight"} {
		if got := device_number(t, state, field); got != 1080 {
			t.Fatalf("%s = %v, want 1080", field, got)
		}
	}
	if got := device_number(t, state, "docClientHeight"); got != 880 {
		t.Fatalf("documentElement.clientHeight = %v, want 880", got)
	}
	if got := device_number(t, state, "visualHeight"); got != 880 {
		t.Fatalf("visualViewport.height = %v, want 880", got)
	}
	if got := device_number(t, state, "rootClientHeight"); got != 0 {
		t.Fatalf("content element clientHeight = %v, want 0", got)
	}
	if got := device_number(t, state, "plainClientHeight"); got != 0 {
		t.Fatalf("plain div clientHeight = %v, want 0", got)
	}
	if got := device_number(t, state, "detachedClientWidth"); got != 0 {
		t.Fatalf("detached clientWidth = %v, want 0", got)
	}
	if got := device_number(t, state, "detachedRectWidth"); got != 0 {
		t.Fatalf("detached getBoundingClientRect().width = %v, want 0", got)
	}
	// el-table distributes 4x120 minimums across the measured 1080px root.
	if got := device_number(t, state, "tableWidth"); got != 1080 {
		t.Fatalf("table width = %v, want 1080 (distributed, not minimum total)", got)
	}
	if got := device_string(t, state, "colWidths"); got != "270,270,270,270" {
		t.Fatalf("column widths = %q, want proportional 270,270,270,270", got)
	}
}

func TestNavigateViewportGeometryMobileFallsBackToMinimums(t *testing.T) {
	state := navigate_geometry_page(t, geometry_server(t), NavigateOptions{Device: DeviceMobile})
	// 390px viewport cannot fit 4x120 minimums, so the table keeps its
	// minimum-width layout like a real narrow browser window.
	if got := device_number(t, state, "tableWidth"); got != 480 {
		t.Fatalf("table width = %v, want 480 minimum total", got)
	}
	if got := device_string(t, state, "colWidths"); got != "120,120,120,120" {
		t.Fatalf("column widths = %q, want minimum 120,120,120,120", got)
	}
	if got := device_number(t, state, "rootClientWidth"); got != 390 {
		t.Fatalf("root clientWidth = %v, want 390", got)
	}
}

func TestNavigateViewportGeometryLegacy(t *testing.T) {
	state := navigate_geometry_page(t, geometry_server(t), NavigateOptions{})
	if got := device_number(t, state, "rootClientWidth"); got != 1440 {
		t.Fatalf("legacy root clientWidth = %v, want 1440", got)
	}
	if got := device_number(t, state, "docClientWidth"); got != 1440 {
		t.Fatalf("legacy documentElement.clientWidth = %v, want 1440", got)
	}
	if got := device_number(t, state, "tableWidth"); got != 1440 {
		t.Fatalf("legacy table width = %v, want 1440", got)
	}
}

func TestNavigateViewportGeometryResizeObserverSeesViewport(t *testing.T) {
	browser, err := NewMiniBrowser(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	server := geometry_server(t)
	page, err := browser.Navigate(context.Background(), server.URL, nil, NavigateOptions{Device: DevicePC})
	if err != nil {
		t.Fatal(err)
	}
	value, err := browser.ExecuteJS(context.Background(), `document.body.getAttribute('data-ro-width')`)
	if err != nil {
		t.Fatal(err)
	}
	if value == nil || value.String() != "1080" {
		t.Fatalf("ResizeObserver contentRect.width = %v, want 1080", value)
	}
	_ = page
}
