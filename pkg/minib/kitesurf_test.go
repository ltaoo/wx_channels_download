package minib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// kitesurf_corpus mirrors Cloudflare's public 14-URL browser corpus:
// https://kitesurf.cloudflare.app/corpus.txt
var kitesurf_corpus = []struct {
	name                string
	url                 string
	required_selectors  []string
	todomvc_interaction bool
}{
	{name: "example", url: "https://example.com/", required_selectors: []string{"h1", "a[href]"}},
	{name: "hacker_news", url: "https://news.ycombinator.com/", required_selectors: []string{".titleline > a"}},
	{name: "cloudflare_docs", url: "https://developers.cloudflare.com/"},
	{name: "cloudflare_blog", url: "https://blog.cloudflare.com/"},
	{name: "wikipedia", url: "https://en.wikipedia.org/"},
	{name: "mdn", url: "https://developer.mozilla.org/en-US/"},
	{name: "elmundo", url: "https://www.elmundo.es/"},
	{name: "rtp", url: "https://www.rtp.pt/noticias/"},
	{name: "guardian", url: "https://www.theguardian.com/international"},
	{name: "todomvc_javascript", url: "https://todomvc.com/examples/javascript-es6/dist/", required_selectors: []string{".todoapp", "input.new-todo"}, todomvc_interaction: true},
	{name: "todomvc_react", url: "https://todomvc.com/examples/react/dist/index.html", required_selectors: []string{".todoapp", "input.new-todo"}, todomvc_interaction: true},
	{name: "todomvc_vue", url: "https://todomvc.com/examples/vue/dist/", required_selectors: []string{".todoapp", "input.new-todo"}, todomvc_interaction: true},
	{name: "todomvc_angular", url: "https://todomvc.com/examples/angular/dist/browser/", required_selectors: []string{".todoapp", "input.new-todo"}, todomvc_interaction: true},
	{name: "todomvc_preact", url: "https://todomvc.com/examples/preact/dist/", required_selectors: []string{".todoapp", "input.new-todo"}, todomvc_interaction: true},
}

type kitesurf_page_state struct {
	URL                string `json:"url"`
	Title              string `json:"title"`
	ReadyState         string `json:"ready_state"`
	ElementCount       int    `json:"element_count"`
	OuterHTMLUTF16     int    `json:"outer_html_utf16"`
	OuterHTMLFNV1A32   string `json:"outer_html_fnv1a32"`
	BodyTextUTF16      int    `json:"body_text_utf16"`
	BodyTextFNV1A32    string `json:"body_text_fnv1a32"`
	LinkCount          int    `json:"link_count"`
	ImageCount         int    `json:"image_count"`
	FormControlCount   int    `json:"form_control_count"`
	InlineStyleCount   int    `json:"inline_style_count"`
	StylesheetCount    int    `json:"stylesheet_count"`
	DocumentVisibility string `json:"document_visibility"`
	TimerType          string `json:"timer_type"`
	ZoneTimerType      string `json:"zone_timer_type"`
	ZonePromiseType    string `json:"zone_promise_type"`
	ZonePromiseThen    string `json:"zone_promise_then_type"`
	ZoneType           string `json:"zone_type"`
}

type kitesurf_capture_report struct {
	Name             string                      `json:"name"`
	RequestedURL     string                      `json:"requested_url"`
	StatusCode       int                         `json:"status_code"`
	DurationMS       int64                       `json:"duration_ms"`
	HTMLBytes        int                         `json:"html_bytes"`
	Resources        int                         `json:"resources"`
	ExecutedScripts  int                         `json:"executed_scripts"`
	ScriptFailures   int                         `json:"script_failures"`
	FailureDetails   []string                    `json:"failure_details,omitempty"`
	ConsoleMessages  []string                    `json:"console_messages,omitempty"`
	XHRRequests      int                         `json:"xhr_requests"`
	FetchRequests    int                         `json:"fetch_requests"`
	CapturedRequests int                         `json:"captured_requests"`
	State            kitesurf_page_state         `json:"state"`
	Interaction      *kitesurf_interaction_state `json:"interaction,omitempty"`
}

type kitesurf_interaction_state struct {
	InputFound  bool   `json:"input_found"`
	BeforeItems int    `json:"before_items"`
	AfterItems  int    `json:"after_items"`
	CreatedText string `json:"created_text"`
	InputValue  string `json:"input_value"`
}

type kitesurf_suite_report struct {
	CorpusURL string                    `json:"corpus_url"`
	Selected  int                       `json:"selected"`
	Passed    int                       `json:"passed"`
	Failed    int                       `json:"failed"`
	Cases     []kitesurf_capture_report `json:"cases"`
}

const kitesurf_state_expression = `(function() {
  var root = document.documentElement, body = document.body;
  var dom = root ? root.outerHTML : '';
  var bodyText = body ? (body.textContent || '').replace(/\s+/g, ' ').trim() : '';
  function hash(value) {
    var result = 2166136261;
    for (var index = 0; index < value.length; index++) {
      result ^= value.charCodeAt(index);
      result = Math.imul(result, 16777619);
    }
    return ('00000000' + (result >>> 0).toString(16)).slice(-8);
  }
  return JSON.stringify({
    url: location.href,
    title: document.title,
    ready_state: document.readyState,
    element_count: document.getElementsByTagName('*').length,
    outer_html_utf16: dom.length,
    outer_html_fnv1a32: hash(dom),
    body_text_utf16: bodyText.length,
    body_text_fnv1a32: hash(bodyText),
    link_count: document.querySelectorAll('a[href]').length,
    image_count: document.querySelectorAll('img').length,
    form_control_count: document.querySelectorAll('form,fieldset,input,button,select,textarea').length,
    inline_style_count: document.querySelectorAll('style').length,
    stylesheet_count: document.styleSheets.length,
    document_visibility: document.visibilityState,
    timer_type: typeof globalThis.setTimeout,
    zone_timer_type: typeof globalThis.__zone_symbol__setTimeout,
    zone_promise_type: typeof globalThis.__zone_symbol__Promise,
    zone_promise_then_type: typeof (globalThis.__zone_symbol__Promise && globalThis.__zone_symbol__Promise.prototype.__zone_symbol__then),
    zone_type: typeof globalThis.Zone
  });
})()`

const kitesurf_todomvc_interaction_expression = `(function() {
  var task = 'minib kitesurf task';
  var input = document.querySelector('input.new-todo');
  var before = document.querySelectorAll('.todo-list li').length;
  if (!input) return JSON.stringify({ input_found: false, before_items: before });
  input.value = task;
  input.dispatchEvent(new InputEvent('input', { bubbles: true, data: task, inputType: 'insertText' }));
  input.dispatchEvent(new Event('change', { bubbles: true }));
  return JSON.stringify({ input_found: true, before_items: before });
})()`

const kitesurf_todomvc_keyboard_expression = `(function() {
  var input = document.querySelector('input.new-todo');
  if (!input) return false;
  input.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Enter', code: 'Enter', keyCode: 13, which: 13 }));
  input.dispatchEvent(new KeyboardEvent('keyup', { bubbles: true, cancelable: true, key: 'Enter', code: 'Enter', keyCode: 13, which: 13 }));
  return true;
})()`

const kitesurf_todomvc_observation_expression = `(function() {
  var input = document.querySelector('input.new-todo');
  var items = document.querySelectorAll('.todo-list li');
  var created = items.length ? items[items.length - 1] : null;
  return JSON.stringify({
    input_found: !!input,
    after_items: items.length,
    created_text: created ? (created.textContent || '').replace(/\s+/g, ' ').trim() : '',
    input_value: input ? input.value : ''
  });
})()`

func TestKitesurfCorpusDefinition(t *testing.T) {
	if len(kitesurf_corpus) != 14 {
		t.Fatalf("Kitesurf corpus contains %d URLs, want 14", len(kitesurf_corpus))
	}
	seen_names := make(map[string]bool)
	seen_urls := make(map[string]bool)
	for _, test_case := range kitesurf_corpus {
		if seen_names[test_case.name] {
			t.Fatalf("duplicate Kitesurf corpus name %q", test_case.name)
		}
		if seen_urls[test_case.url] {
			t.Fatalf("duplicate Kitesurf corpus URL %q", test_case.url)
		}
		seen_names[test_case.name] = true
		seen_urls[test_case.url] = true
		parsed_url, err := url.Parse(test_case.url)
		if err != nil || parsed_url.Scheme != "https" || parsed_url.Host == "" {
			t.Fatalf("invalid Kitesurf corpus URL %q", test_case.url)
		}
	}
}

func TestKitesurfRealWorldCorpus(t *testing.T) {
	if os.Getenv("MINIB_KITESURF_LIVE_TEST") == "" {
		t.Skip("set MINIB_KITESURF_LIVE_TEST=1 to run the public Kitesurf real-world corpus")
	}
	case_filter := strings.TrimSpace(os.Getenv("MINIB_KITESURF_CASE"))
	output_dir := strings.TrimSpace(os.Getenv("MINIB_KITESURF_OUTPUT_DIR"))
	matched := 0
	passed := 0
	reports := make([]kitesurf_capture_report, 0)
	for _, test_case := range kitesurf_corpus {
		if case_filter != "" && !strings.Contains(test_case.name, case_filter) {
			continue
		}
		matched++
		var captured_report *kitesurf_capture_report
		case_passed := t.Run(test_case.name, func(t *testing.T) {
			browser, err := NewMiniBrowser(2 * time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			defer browser.Close()

			started_at := time.Now()
			page, err := browser.Navigate(context.Background(), test_case.url, nil, NavigateOptions{
				CaptureHAR: output_dir != "",
			})
			if err != nil {
				t.Fatal(err)
			}
			state_value, err := browser.ExecuteJS(context.Background(), kitesurf_state_expression)
			if err != nil {
				t.Fatal(err)
			}
			var state kitesurf_page_state
			if err := json.Unmarshal([]byte(state_value.String()), &state); err != nil {
				t.Fatalf("decode page state: %v; value=%s", err, state_value.String())
			}
			assert_kitesurf_page_state(t, page, state, test_case.required_selectors)
			var interaction *kitesurf_interaction_state
			if test_case.todomvc_interaction {
				interaction = run_kitesurf_todomvc_interaction(t, browser)
			}

			report := kitesurf_capture_report{
				Name:             test_case.name,
				RequestedURL:     test_case.url,
				StatusCode:       page.StatusCode,
				DurationMS:       time.Since(started_at).Milliseconds(),
				HTMLBytes:        len(page.RenderedHTML),
				Resources:        len(page.Resources),
				ExecutedScripts:  page.ExecutedScripts,
				ScriptFailures:   len(page.ScriptFailures),
				FailureDetails:   kitesurf_failure_details(page.ScriptFailures),
				ConsoleMessages:  kitesurf_console_messages(page.ConsoleMessages),
				XHRRequests:      len(page.XHRRequests),
				FetchRequests:    len(page.FetchRequests),
				CapturedRequests: har_entry_count(page.har_data),
				State:            state,
				Interaction:      interaction,
			}
			captured_report = &report
			if output_dir != "" {
				if err := save_kitesurf_capture(output_dir, test_case.name, page, report); err != nil {
					t.Fatal(err)
				}
			}
			report_json, _ := json.Marshal(report)
			t.Log(string(report_json))
		})
		if captured_report != nil {
			reports = append(reports, *captured_report)
		}
		if case_passed {
			passed++
		}
	}
	if matched == 0 {
		t.Fatalf("MINIB_KITESURF_CASE=%q did not match a corpus case", case_filter)
	}
	if output_dir != "" {
		if err := save_kitesurf_suite_report(output_dir, kitesurf_suite_report{
			CorpusURL: "https://kitesurf.cloudflare.app/corpus.txt",
			Selected:  matched,
			Passed:    passed,
			Failed:    matched - passed,
			Cases:     reports,
		}); err != nil {
			t.Error(err)
		}
	}
}

func run_kitesurf_todomvc_interaction(t *testing.T, browser *MiniBrowser) *kitesurf_interaction_state {
	t.Helper()
	value, err := browser.ExecuteJS(context.Background(), kitesurf_todomvc_interaction_expression)
	if err != nil {
		t.Fatalf("execute TodoMVC interaction: %v", err)
	}
	var state kitesurf_interaction_state
	if err := json.Unmarshal([]byte(value.String()), &state); err != nil {
		t.Fatalf("decode TodoMVC interaction state: %v; value=%s", err, value.String())
	}
	value, err = browser.ExecuteJS(context.Background(), kitesurf_todomvc_keyboard_expression)
	if err != nil {
		t.Fatalf("execute TodoMVC keyboard interaction: %v", err)
	}
	if !value.ToBoolean() {
		t.Fatal("TodoMVC keyboard interaction input was not found")
	}
	value, err = browser.ExecuteJS(context.Background(), kitesurf_todomvc_observation_expression)
	if err != nil {
		t.Fatalf("observe TodoMVC interaction: %v", err)
	}
	var observation kitesurf_interaction_state
	if err := json.Unmarshal([]byte(value.String()), &observation); err != nil {
		t.Fatalf("decode TodoMVC interaction observation: %v; value=%s", err, value.String())
	}
	state.InputFound = state.InputFound && observation.InputFound
	state.AfterItems = observation.AfterItems
	state.CreatedText = observation.CreatedText
	state.InputValue = observation.InputValue
	if !state.InputFound {
		t.Error("TodoMVC interaction input was not found")
	}
	if state.AfterItems <= state.BeforeItems {
		t.Errorf("TodoMVC interaction did not create an item: before=%d after=%d", state.BeforeItems, state.AfterItems)
	}
	if !strings.Contains(state.CreatedText, "minib kitesurf task") {
		t.Errorf("TodoMVC interaction created text %q, want task text", state.CreatedText)
	}
	return &state
}

func kitesurf_console_messages(messages []string) []string {
	const max_console_messages = 8
	if len(messages) > max_console_messages {
		messages = messages[:max_console_messages]
	}
	return append([]string(nil), messages...)
}

func assert_kitesurf_page_state(t *testing.T, page *Page, state kitesurf_page_state, required_selectors []string) {
	t.Helper()
	if page.StatusCode < 200 || page.StatusCode >= 400 {
		t.Errorf("status=%d, want 2xx or 3xx", page.StatusCode)
	}
	if state.URL == "" || state.Title == "" {
		t.Errorf("page identity is incomplete: url=%q title=%q", state.URL, state.Title)
	}
	if state.ReadyState != "complete" {
		t.Errorf("readyState=%q, want complete", state.ReadyState)
	}
	if state.ElementCount < 3 || state.OuterHTMLUTF16 == 0 || state.BodyTextUTF16 == 0 {
		t.Errorf("document did not produce meaningful content: %+v", state)
	}
	if len(state.OuterHTMLFNV1A32) != 8 || len(state.BodyTextFNV1A32) != 8 {
		t.Errorf("document fingerprints are invalid: html=%q text=%q", state.OuterHTMLFNV1A32, state.BodyTextFNV1A32)
	}
	if state.DocumentVisibility != "visible" {
		t.Errorf("visibilityState=%q, want visible", state.DocumentVisibility)
	}
	for _, selector := range required_selectors {
		if query_first(page.Document, selector) == nil {
			t.Errorf("rendered DOM is missing required selector %q", selector)
		}
	}
}

func kitesurf_failure_details(failures []ScriptFailure) []string {
	details := make([]string, 0, len(failures))
	for _, failure := range failures {
		details = append(details, fmt.Sprintf("%s: %v", failure.URL, failure.Err))
	}
	return details
}

func save_kitesurf_suite_report(output_dir string, report kitesurf_suite_report) error {
	if err := os.MkdirAll(output_dir, 0755); err != nil {
		return fmt.Errorf("minib: create Kitesurf output directory: %w", err)
	}
	report_data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("minib: encode Kitesurf suite report: %w", err)
	}
	if err := os.WriteFile(filepath.Join(output_dir, "summary.json"), append(report_data, '\n'), 0600); err != nil {
		return fmt.Errorf("minib: write Kitesurf suite report: %w", err)
	}
	return nil
}

func save_kitesurf_capture(output_dir string, name string, page *Page, report kitesurf_capture_report) error {
	case_dir := filepath.Join(output_dir, name)
	if err := page.SaveHTML(filepath.Join(case_dir, "page.html")); err != nil {
		return err
	}
	if err := page.SaveHAR(filepath.Join(case_dir, "page.har")); err != nil {
		return err
	}
	report_data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("minib: encode Kitesurf report: %w", err)
	}
	if err := os.WriteFile(filepath.Join(case_dir, "report.json"), append(report_data, '\n'), 0600); err != nil {
		return fmt.Errorf("minib: write Kitesurf report: %w", err)
	}
	return nil
}

func har_entry_count(har_data []byte) int {
	if len(har_data) == 0 {
		return 0
	}
	var document struct {
		Log struct {
			Entries []json.RawMessage `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(har_data, &document); err != nil {
		return 0
	}
	return len(document.Log.Entries)
}
