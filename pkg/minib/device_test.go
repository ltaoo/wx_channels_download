package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const device_media_page = `<!doctype html><html><head>
<style>
.box { color: red; }
@media (min-width: 1000px) { .box { color: green; } }
@media (max-width: 500px) { .box { color: blue; } }
</style>
</head><body><div class="box">content</div><script>
document.body.setAttribute('data-viewport', JSON.stringify({
  innerWidth: window.innerWidth,
  innerHeight: window.innerHeight,
  outerScreen: [screen.width, screen.height, screen.availWidth, screen.availHeight].join(','),
  ratio: window.devicePixelRatio,
  platform: navigator.platform,
  touch: navigator.maxTouchPoints,
  agent: navigator.userAgent,
  desktopWide: matchMedia('(min-width: 768px)').matches,
  narrow: matchMedia('(max-width: 1080px)').matches,
  portrait: matchMedia('(orientation: portrait)').matches,
  landscape: matchMedia('(orientation: landscape)').matches,
  hoverHover: matchMedia('(hover: hover)').matches,
  hoverNone: matchMedia('(hover: none)').matches,
  pointerFine: matchMedia('(pointer: fine)').matches,
  pointerCoarse: matchMedia('(pointer: coarse)').matches,
  reducedMotion: matchMedia('(prefers-reduced-motion: reduce)').matches,
  colorScheme: matchMedia('(prefers-color-scheme: dark)').matches,
  computedColor: getComputedStyle(document.querySelector('.box')).color
}));
</script></body></html>`

func navigate_device_page(t *testing.T, server *httptest.Server, options NavigateOptions) map[string]any {
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
	value, err := browser.ExecuteJS(context.Background(), `document.body.getAttribute('data-viewport')`)
	if err != nil {
		t.Fatal(err)
	}
	if value == nil || value.String() == "" {
		t.Fatal("page did not record viewport state")
	}
	return decode_device_state(t, browser, value.String())
}

func decode_device_state(t *testing.T, browser *MiniBrowser, encoded string) map[string]any {
	t.Helper()
	value, err := browser.ExecuteJS(context.Background(), "(function(){ return "+encoded+"; })()")
	if err != nil {
		t.Fatal(err)
	}
	exported, ok := value.Export().(map[string]any)
	if !ok {
		t.Fatalf("viewport state has unexpected type %T", value.Export())
	}
	return exported
}

func device_number(t *testing.T, state map[string]any, key string) float64 {
	t.Helper()
	switch number := state[key].(type) {
	case int64:
		return float64(number)
	case int:
		return float64(number)
	case float64:
		return number
	default:
		t.Fatalf("viewport field %q = %#v, expected number", key, state[key])
		return 0
	}
}

func device_bool(t *testing.T, state map[string]any, key string) bool {
	t.Helper()
	flag, ok := state[key].(bool)
	if !ok {
		t.Fatalf("viewport field %q = %#v, expected bool", key, state[key])
	}
	return flag
}

func device_string(t *testing.T, state map[string]any, key string) string {
	t.Helper()
	text, ok := state[key].(string)
	if !ok {
		t.Fatalf("viewport field %q = %#v, expected string", key, state[key])
	}
	return text
}

func TestNavigateDefaultDeviceKeepsLegacyViewport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, device_media_page)
	}))
	defer server.Close()

	state := navigate_device_page(t, server, NavigateOptions{})
	if got := device_number(t, state, "innerWidth"); got != 1440 {
		t.Fatalf("legacy innerWidth = %v, want 1440", got)
	}
	if got := device_number(t, state, "innerHeight"); got != 900 {
		t.Fatalf("legacy innerHeight = %v, want 900", got)
	}
	if got := device_string(t, state, "outerScreen"); got != "1440,900,1440,875" {
		t.Fatalf("legacy screen = %q, want 1440,900,1440,875", got)
	}
	if got := device_number(t, state, "ratio"); got != 2 {
		t.Fatalf("legacy devicePixelRatio = %v, want 2", got)
	}
	if got := device_string(t, state, "platform"); got != "MacIntel" {
		t.Fatalf("legacy platform = %q, want MacIntel", got)
	}
	if got := device_number(t, state, "touch"); got != 0 {
		t.Fatalf("legacy maxTouchPoints = %v, want 0", got)
	}
	if !device_bool(t, state, "desktopWide") {
		t.Fatal("legacy matchMedia('(min-width: 768px)') = false, want true at 1440")
	}
	if device_bool(t, state, "narrow") {
		t.Fatal("legacy matchMedia('(max-width: 1080px)') = true, want false at 1440")
	}
	if !device_bool(t, state, "hoverHover") || !device_bool(t, state, "pointerFine") {
		t.Fatal("legacy desktop should keep hover:hover and pointer:fine matching")
	}
	if got := device_string(t, state, "computedColor"); got != "green" {
		t.Fatalf("legacy computed color = %q, want green from min-width 1000px block", got)
	}
	if !strings.Contains(device_string(t, state, "agent"), "Mozilla/5.0") {
		t.Fatalf("legacy user agent = %q, expected desktop Chrome UA", device_string(t, state, "agent"))
	}
}

func TestNavigatePCDeviceViewport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, device_media_page)
	}))
	defer server.Close()

	state := navigate_device_page(t, server, NavigateOptions{Device: DevicePC})
	if got := device_number(t, state, "innerWidth"); got != 1080 {
		t.Fatalf("PC innerWidth = %v, want 1080", got)
	}
	if got := device_number(t, state, "innerHeight"); got != 880 {
		t.Fatalf("PC innerHeight = %v, want 880", got)
	}
	if got := device_string(t, state, "outerScreen"); got != "1080,880,1080,855" {
		t.Fatalf("PC screen = %q, want 1080,880,1080,855", got)
	}
	if got := device_number(t, state, "ratio"); got != 2 {
		t.Fatalf("PC devicePixelRatio = %v, want 2", got)
	}
	if !device_bool(t, state, "desktopWide") {
		t.Fatal("PC matchMedia('(min-width: 768px)') = false, want true at 1080")
	}
	if !device_bool(t, state, "narrow") {
		t.Fatal("PC matchMedia('(max-width: 1080px)') = false, want true at 1080")
	}
	if !device_bool(t, state, "landscape") || device_bool(t, state, "portrait") {
		t.Fatal("PC orientation should be landscape (1080x880)")
	}
	if !device_bool(t, state, "hoverHover") || device_bool(t, state, "hoverNone") {
		t.Fatal("PC should match hover:hover but not hover:none")
	}
	if !device_bool(t, state, "pointerFine") || device_bool(t, state, "pointerCoarse") {
		t.Fatal("PC should match pointer:fine but not pointer:coarse")
	}
	if device_bool(t, state, "reducedMotion") || device_bool(t, state, "colorScheme") {
		t.Fatal("matchMedia prefers-* queries should stay false")
	}
	if got := device_string(t, state, "computedColor"); got != "green" {
		t.Fatalf("PC computed color = %q, want green from min-width 1000px block", got)
	}
}

func TestNavigateMobileDeviceViewport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, device_media_page)
	}))
	defer server.Close()

	state := navigate_device_page(t, server, NavigateOptions{Device: DeviceMobile})
	if got := device_number(t, state, "innerWidth"); got != 390 {
		t.Fatalf("mobile innerWidth = %v, want 390", got)
	}
	if got := device_number(t, state, "innerHeight"); got != 844 {
		t.Fatalf("mobile innerHeight = %v, want 844", got)
	}
	if got := device_number(t, state, "ratio"); got != 3 {
		t.Fatalf("mobile devicePixelRatio = %v, want 3", got)
	}
	if got := device_string(t, state, "platform"); got != "iPhone" {
		t.Fatalf("mobile platform = %q, want iPhone", got)
	}
	if got := device_number(t, state, "touch"); got != 5 {
		t.Fatalf("mobile maxTouchPoints = %v, want 5", got)
	}
	if device_bool(t, state, "desktopWide") {
		t.Fatal("mobile matchMedia('(min-width: 768px)') = true, want false at 390")
	}
	if !device_bool(t, state, "portrait") || device_bool(t, state, "landscape") {
		t.Fatal("mobile orientation should be portrait (390x844)")
	}
	if device_bool(t, state, "hoverHover") || !device_bool(t, state, "hoverNone") {
		t.Fatal("mobile should match hover:none but not hover:hover")
	}
	if !device_bool(t, state, "pointerCoarse") || device_bool(t, state, "pointerFine") {
		t.Fatal("mobile should match pointer:coarse but not pointer:fine")
	}
	if got := device_string(t, state, "computedColor"); got != "blue" {
		t.Fatalf("mobile computed color = %q, want blue from max-width 500px block", got)
	}
	agent := device_string(t, state, "agent")
	if !strings.Contains(agent, "iPhone") || !strings.Contains(agent, "Mobile") {
		t.Fatalf("mobile navigator.userAgent = %q, expected iPhone mobile UA", agent)
	}
}

func TestNavigateMobileDeviceSubstitutesUserAgent(t *testing.T) {
	var request_mutex sync.Mutex
	var document_user_agents []string
	var client_hint_values []string
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			response_writer.WriteHeader(http.StatusNotFound)
			return
		}
		request_mutex.Lock()
		document_user_agents = append(document_user_agents, request.Header.Get("User-Agent"))
		client_hint_values = append(client_hint_values, request.Header.Get("Sec-Ch-Ua-Mobile"))
		request_mutex.Unlock()
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><title>ua</title>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	if _, err := browser.Navigate(context.Background(), server.URL, nil, NavigateOptions{Device: DeviceMobile}); err != nil {
		t.Fatal(err)
	}
	request_mutex.Lock()
	observed_agents := append([]string(nil), document_user_agents...)
	observed_hints := append([]string(nil), client_hint_values...)
	request_mutex.Unlock()
	if len(observed_agents) != 1 {
		t.Fatalf("document requests = %d, want 1", len(observed_agents))
	}
	if !strings.Contains(observed_agents[0], "iPhone") {
		t.Fatalf("mobile document User-Agent = %q, expected iPhone UA", observed_agents[0])
	}
	if observed_hints[0] != "" {
		t.Fatalf("mobile document Sec-Ch-Ua-Mobile = %q, want it removed", observed_hints[0])
	}

	custom_headers := http.Header{}
	custom_headers.Set("User-Agent", "CustomAgent/1.0")
	if _, err := browser.Navigate(context.Background(), server.URL, custom_headers, NavigateOptions{Device: DeviceMobile}); err != nil {
		t.Fatal(err)
	}
	request_mutex.Lock()
	observed_agents = append([]string(nil), document_user_agents...)
	request_mutex.Unlock()
	if len(observed_agents) != 2 {
		t.Fatalf("document requests = %d, want 2", len(observed_agents))
	}
	if observed_agents[1] != "CustomAgent/1.0" {
		t.Fatalf("explicit User-Agent = %q, want it preserved", observed_agents[1])
	}
}

func TestNavigateRejectsInvalidDevice(t *testing.T) {
	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	_, err = browser.Navigate(context.Background(), "https://example.com", nil, NavigateOptions{Device: Device("tablet")})
	if err == nil || !strings.Contains(err.Error(), "unsupported Device") {
		t.Fatalf("invalid device error = %v, want unsupported Device rejection", err)
	}
}
