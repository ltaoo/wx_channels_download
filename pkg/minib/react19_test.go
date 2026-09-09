package minib

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestReact19SSR(t *testing.T) {
	browser, err := NewMiniBrowser(120 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var capturedErrors []string
	page, err := browser.Navigate(ctx, "http://localhost:3000/", nil, NavigateOptions{
		RuntimeInitializer: func(vm *goja.Runtime, p *Page) error {
			_, err := vm.RunString(`
				(function() {
					window.__minib_errors = [];
					window.__minib_hydrated = false;
					window.__minib_scripts_executed = [];

					// Intercept console.error — React 的 hydration 警告
					var origError = console.error;
					console.error = function() {
						var args = Array.prototype.slice.call(arguments);
						var msg = args.map(function(a) {
							if (typeof a === 'string') return a;
							if (a && a.message) return a.message;
							try { return JSON.stringify(a); } catch(e) { return String(a); }
						}).join(' ');
						if (msg.length > 0) {
							window.__minib_errors.push('console.error: ' + msg.substring(0, 2000));
						}
						return origError.apply(console, args);
					};

					// Intercept console.warn
					var origWarn = console.warn;
					console.warn = function() {
						var args = Array.prototype.slice.call(arguments);
						var msg = args.map(function(a) {
							return typeof a === 'string' ? a : String(a);
						}).join(' ');
						if (msg.length > 0) {
							window.__minib_errors.push('console.warn: ' + msg.substring(0, 1000));
						}
						return origWarn.apply(console, args);
					};

					// Intercept console.log to detect hydration
					var origLog = console.log;
					console.log = function() {
						var args = Array.prototype.slice.call(arguments);
						var msg = args.map(function(a) {
							return typeof a === 'string' ? a : String(a);
						}).join(' ');
						if (msg.indexOf('hydrat') >= 0 || msg.indexOf('React') >= 0 || msg.indexOf('vite') >= 0) {
							window.__minib_errors.push('console.log: ' + msg.substring(0, 300));
						}
						return origLog.apply(console, args);
					};

					// Trap property accesses to detect what React is probing
					var probed_properties = {};
					var orig_getOwnPropDesc = Object.getOwnPropertyDescriptor;

					window.addEventListener('unhandledrejection', function(event) {
						var reason = event.reason;
						var msg = reason && reason.message ? reason.message : String(reason);
						var stack = reason && reason.stack ? reason.stack.substring(0, 2000) : '';
						window.__minib_errors.push('UNHANDLED_REJECTION: ' + msg.substring(0, 500) + '\nSTACK: ' + stack);
					});

					window.addEventListener('error', function(event) {
						var msg = event.message || '';
						var stack = event.error && event.error.stack ? event.error.stack.substring(0, 2000) : '';
						window.__minib_errors.push('WINDOW_ERROR: ' + msg.substring(0, 500) + '\nSTACK: ' + stack);
					});
				})();
			`)
			return err
		},
		RuntimeFinalizer: func(vm *goja.Runtime, p *Page) error {
			val, err := vm.RunString(`JSON.stringify(window.__minib_errors || [])`)
			if err == nil {
				var errors []string
				if json.Unmarshal([]byte(val.String()), &errors) == nil {
					capturedErrors = errors
				}
			}

			// Deep DOM inspection for hydration debugging
			domInspect, _ := vm.RunString(`(function() {
				var root = document.getElementById('root');
				if (!root) return JSON.stringify({error: 'no root'});

				function inspectNode(node, depth) {
					if (depth > 5) return '...';
					if (!node) return null;
					var info = {};
					info.nodeType = node.nodeType;
					info.nodeName = node.nodeName;
					if (node.nodeType === 3) { // text node
						info.textContent = node.textContent;
						info.data = node.data;
						info.length = node.length;
					}
					if (node.nodeType === 8) { // comment
						info.data = node.data;
					}
					if (node.nodeType === 1) { // element
						info.tagName = node.tagName;
						info.innerHTML_length = node.innerHTML ? node.innerHTML.length : -1;
						info.childNodes_length = node.childNodes ? node.childNodes.length : -1;
						info.children_length = node.children ? node.children.length : -1;
						info.attributes_length = node.attributes ? node.attributes.length : -1;
						// Check some properties React uses
						info.hasOwnerDocument = !!node.ownerDocument;
						info.namespaceURI = node.namespaceURI || 'undefined';
						info.firstChild_type = node.firstChild ? node.firstChild.nodeType : null;
						info.firstChild_name = node.firstChild ? node.firstChild.nodeName : null;
						if (depth < 3 && node.childNodes) {
							info.children = [];
							for (var i = 0; i < Math.min(node.childNodes.length, 10); i++) {
								info.children.push(inspectNode(node.childNodes[i], depth + 1));
							}
						}
					}
					return info;
				}

				var result = {
					root: inspectNode(root, 0),
					root_innerHTML: root.innerHTML.substring(0, 500),
					root_childNodes_length: root.childNodes.length,
					root_firstChild: root.firstChild ? {
						nodeType: root.firstChild.nodeType,
						nodeName: root.firstChild.nodeName,
						tagName: root.firstChild.tagName
					} : null,
					// Check document properties React uses
					doc_nodeType: document.nodeType,
					doc_documentElement_tagName: document.documentElement.tagName,
					doc_head_exists: !!document.head,
					doc_body_exists: !!document.body,
					doc_createTreeWalker: typeof document.createTreeWalker,
					doc_createComment: typeof document.createComment,
					node_prototype_checks: {
						compareDocumentPosition: typeof (root.compareDocumentPosition),
						contains: typeof (root.contains),
						getRootNode: typeof (root.getRootNode),
						isConnected: root.isConnected,
					}
				};
				return JSON.stringify(result);
			})()`);
			if domInspect != nil {
				t.Logf("DOM Inspection: %s", domInspect.String())
			}

			// 检查 React 是否存在
			checks, _ := vm.RunString(`JSON.stringify({
				hasReact: typeof window.__REACT_DEVTOOLS_GLOBAL_HOOK__ !== 'undefined',
				hasReactDOM: typeof window.ReactDOM !== 'undefined',
				rootHasReactFiber: (function() {
					var root = document.getElementById('root');
					if (!root) return 'no root';
					var keys = Object.keys(root);
					var reactKeys = keys.filter(function(k) { return k.indexOf('__react') >= 0; });
					return reactKeys.length > 0 ? reactKeys.join(',') : 'no react keys found, keys: ' + keys.slice(0, 10).join(',');
				})(),
				moduleScriptCount: document.querySelectorAll('script[type="module"]').length,
				allScriptCount: document.querySelectorAll('script').length
			})`)
			if checks != nil {
				t.Logf("React checks: %s", checks.String())
			}

			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("URL: %s, Status: %d", page.URL, page.StatusCode)
	t.Logf("HTML: %d bytes, RenderedHTML: %d bytes", len(page.HTML), len(page.RenderedHTML))
	t.Logf("Resources: %d, ScriptFailures: %d", len(page.Resources), len(page.ScriptFailures))

	// 打印加载的资源
	for i, r := range page.Resources {
		status := "OK"
		if r.Err != nil {
			status = r.Err.Error()
		}
		t.Logf("  Resource[%d]: %v %s (%d bytes) %s", i, r.Kind, r.URL, len(r.Body), status)
	}

	for _, sf := range page.ScriptFailures {
		t.Logf("Script failure: %s -> %v", sf.URL, sf.Err)
	}

	t.Logf("=== Captured Errors/Logs (%d) ===", len(capturedErrors))
	for i, msg := range capturedErrors {
		t.Logf("  [%d] %s", i, msg)
	}

	// 保存 HTML
	output := page.RenderedHTML
	if output == "" {
		output = page.HTML
	}
	_ = os.WriteFile("react19_rendered.html", []byte(output), 0644)

	// 检查 SSR 内容是否保留
	for _, pattern := range []string{"JSX Expressions", "useState", "Suspense"} {
		inRaw := strings.Contains(page.HTML, pattern)
		inRendered := strings.Contains(page.RenderedHTML, pattern)
		if inRaw != inRendered {
			t.Errorf("MISMATCH %q: raw=%v rendered=%v", pattern, inRaw, inRendered)
		}
	}
}
