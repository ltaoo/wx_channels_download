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

func TestNavigateDouban(t *testing.T) {
	browser, err := NewMiniBrowser(120 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	page, err := browser.Navigate(ctx, "https://movie.douban.com/subject/1393859/", nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Final URL: %s", page.URL)
	t.Logf("Final Status: %d", page.StatusCode)
	t.Logf("HTML: %d, RenderedHTML: %d, Resources: %d", len(page.HTML), len(page.RenderedHTML), len(page.Resources))
	t.Logf("Navigation history: %v", page.NavigationHistory)

	for _, sf := range page.ScriptFailures {
		t.Logf("Script failure: %s -> %v", sf.URL, sf.Err)
	}

	output := page.RenderedHTML
	if output == "" {
		output = page.HTML
	}
	if err := os.WriteFile("douban_1393859.html", []byte(output), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("Saved to douban_1393859.html (%d bytes)", len(output))
}

func TestNavigateDoubanBook(t *testing.T) {
	browser, err := NewMiniBrowser(120 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	page, err := browser.Navigate(ctx, "https://book.douban.com/subject/38568894/?icn=index-latestbook-subject", nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Final URL: %s", page.URL)
	t.Logf("Final Status: %d", page.StatusCode)
	t.Logf("HTML: %d, RenderedHTML: %d, Resources: %d", len(page.HTML), len(page.RenderedHTML), len(page.Resources))
	t.Logf("Navigation history: %v", page.NavigationHistory)

	for _, sf := range page.ScriptFailures {
		t.Logf("Script failure: %s -> %v", sf.URL, sf.Err)
	}

	output := page.RenderedHTML
	if output == "" {
		output = page.HTML
	}
	if err := os.WriteFile("douban_book_38568894.html", []byte(output), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("Saved to douban_book_38568894.html (%d bytes)", len(output))
}

func TestNavigateDouban1418238(t *testing.T) {
	browser, err := NewMiniBrowser(120 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	page, err := browser.Navigate(ctx, "https://movie.douban.com/subject/1418238/", nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Final URL: %s", page.URL)
	t.Logf("Final Status: %d", page.StatusCode)
	t.Logf("HTML: %d, RenderedHTML: %d, Resources: %d", len(page.HTML), len(page.RenderedHTML), len(page.Resources))
	t.Logf("Navigation history: %v", page.NavigationHistory)

	for _, sf := range page.ScriptFailures {
		t.Logf("Script failure: %s -> %v", sf.URL, sf.Err)
	}

	output := page.RenderedHTML
	if output == "" {
		output = page.HTML
	}
	if err := os.WriteFile("douban_1418238.html", []byte(output), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("Saved to douban_1418238.html (%d bytes)", len(output))
}

func TestNavigateGitHubIssues(t *testing.T) {
	browser, err := NewMiniBrowser(120 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var capturedErrors []string
	page, err := browser.Navigate(ctx, "https://github.com/ltaoo/wx_channels_download/issues", nil, NavigateOptions{
		RuntimeInitializer: func(vm *goja.Runtime, p *Page) error {
			_, err := vm.RunString(`
				(function() {
					window.__minib_errors = [];
					window.__minib_mutations = [];
					var mutationCount = 0;

					// Intercept Error constructor to catch React's internal errors
					var _OrigError = Error;
					var _errorCount = 0;
					Error = function(message) {
						var err = new _OrigError(message);
						if (_errorCount < 20) {
							_errorCount++;
							var msg = String(message || '').substring(0, 500);
							var stack = '';
							try { stack = err.stack ? err.stack.split('\n').slice(0, 8).join('\n') : ''; } catch(e) {}
							window.__minib_errors.push('ERROR_NEW: ' + msg + '\nSTACK:\n' + stack);
						}
						return err;
					};
					Error.prototype = _OrigError.prototype;
					Error.captureStackTrace = _OrigError.captureStackTrace;

					// Inspect what behaviors.js sees when reading attributes
					window.__minib_attrs_debug = (function() {
						try {
							var attrs = document.documentElement.attributes;
							var spread = [...attrs];
							return { length: attrs.length, spreadLength: spread.length, names: spread.map(function(a) { return a.name; }) };
						} catch(e) {
							return { error: e.message };
						}
					})();

					// Log all uncaught errors during script execution
					window.addEventListener('error', function(event) {
						if (_errorCount < 20) {
							_errorCount++;
							window.__minib_errors.push('WINDOW_ERROR: ' + (event.message || '') + ' at ' + (event.filename || '') + ':' + (event.lineno || ''));
						}
					});

					// Intercept customElements.define to track what gets defined
					var _origDefine = customElements.define.bind(customElements);
					window.__minib_defined_elements = [];
					customElements.define = function(name, constructor, options) {
						window.__minib_defined_elements.push(name);
						return _origDefine(name, constructor, options);
					};

					// Watch for DOM mutations in the react root
					var observer = new MutationObserver(function(mutations) {
						mutations.forEach(function(mutation) {
							if (mutationCount >= 100) return;
							mutationCount++;
							var targetTag = mutation.target.tagName || mutation.target.nodeName;
							var targetClass = mutation.target.className ? String(mutation.target.className).substring(0, 80) : '';
							var info = mutation.type + ' on <' + targetTag + '>';
							if (targetClass) info += ' class=' + targetClass;
							if (mutation.type === 'childList') {
								info += ' added=' + mutation.addedNodes.length + ' removed=' + mutation.removedNodes.length;
								if (mutation.addedNodes.length > 0) {
									for (var i = 0; i < Math.min(mutation.addedNodes.length, 3); i++) {
										var added = mutation.addedNodes[i];
										info += ' [+' + (added.tagName || added.nodeName) + ']';
									}
								}
								if (mutation.removedNodes.length > 0) {
									for (var i = 0; i < Math.min(mutation.removedNodes.length, 3); i++) {
										var removed = mutation.removedNodes[i];
										info += ' [-' + (removed.tagName || removed.nodeName) + ']';
									}
								}
							}
							if (mutation.type === 'attributes') {
								info += ' attr=' + mutation.attributeName;
							}
							window.__minib_mutations.push(info);
						});
					});

					// Observe everything
					var reactAppRoot = document.querySelector('[data-target="react-app.reactRoot"]');
					if (reactAppRoot) {
						observer.observe(reactAppRoot, {
							childList: true,
							attributes: true,
							subtree: true,
							characterData: true
						});
					}

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
					window.addEventListener('unhandledrejection', function(event) {
						var reason = event.reason;
						var msg = reason && reason.message ? reason.message : String(reason);
						window.__minib_errors.push('UNHANDLED_REJECTION: ' + msg.substring(0, 500));
					});
				})();
			`)
			return err
		},
		RuntimeFinalizer: func(vm *goja.Runtime, p *Page) error {
			// Check defined elements
			defined, _ := vm.RunString(`JSON.stringify(window.__minib_defined_elements || [])`)
			if defined != nil {
				t.Logf("Defined custom elements: %s", defined.String())
			}

			// Deep inspect the React root children
			rootInspect, _ := vm.RunString(`(function() {
				var root = document.querySelector('[data-target="react-app.reactRoot"]');
				if (!root) return JSON.stringify({error: 'no root'});
				var children = [];
				for (var i = 0; i < Math.min(root.childNodes.length, 10); i++) {
					var child = root.childNodes[i];
					var info = {
						nodeType: child.nodeType,
						nodeName: child.nodeName,
					};
					if (child.nodeType === 3) {
						info.data = JSON.stringify(child.data);
						info.length = child.data.length;
						info.isWhitespace = /^\\s*$/.test(child.data);
					}
					if (child.nodeType === 8) info.data = child.data;
					if (child.nodeType === 1) {
						info.tagName = child.tagName;
						info.firstChildType = child.firstChild ? child.firstChild.nodeType : null;
						info.firstChildName = child.firstChild ? child.firstChild.nodeName : null;
						info.childCount = child.childNodes.length;
					}
					children.push(info);
				}
				return JSON.stringify({
					rootTagName: root.tagName,
					rootChildCount: root.childNodes.length,
					firstChildType: root.firstChild ? root.firstChild.nodeType : null,
					firstChildName: root.firstChild ? root.firstChild.nodeName : null,
					firstChildData: root.firstChild && root.firstChild.nodeType === 3 ? JSON.stringify(root.firstChild.data) : root.firstChild && root.firstChild.data,
					children: children
				});
			})()`)
			if rootInspect != nil {
				t.Logf("Root inspect: %s", rootInspect.String())
			}

			// Check if embedded data still exists
			dataCheck, _ := vm.RunString(`(function() {
				var embeddedData = document.querySelector('[data-target="react-app.embeddedData"]');
				var reactRoot = document.querySelector('[data-target="react-app.reactRoot"]');
				return JSON.stringify({
					embeddedDataExists: !!embeddedData,
					embeddedDataTextLength: embeddedData ? embeddedData.textContent.length : 0,
					reactRootExists: !!reactRoot,
					reactRootChildCount: reactRoot ? reactRoot.childNodes.length : 0,
					reactRootInnerHTMLLength: reactRoot ? reactRoot.innerHTML.length : 0,
					reactAppElement: !!document.querySelector('react-app'),
					reactAppDefined: typeof customElements.get('react-app'),
					allScripts: document.querySelectorAll('script[type="application/json"]').length
				});
			})()`)
			if dataCheck != nil {
				t.Logf("Data check: %s", dataCheck.String())
			}

			// Log attributes debug
			attrs_debug, _ := vm.RunString(`JSON.stringify(window.__minib_attrs_debug || {})`)
			if attrs_debug != nil {
				t.Logf("Attributes debug: %s", attrs_debug.String())
			}

			val, err := vm.RunString(`JSON.stringify(window.__minib_errors || [])`)
			if err == nil {
				var errors []string
				if json.Unmarshal([]byte(val.String()), &errors) == nil {
					capturedErrors = errors
				}
			}

			// Log mutations
			mutations, _ := vm.RunString(`JSON.stringify(window.__minib_mutations || [])`)
			if mutations != nil {
				var muts []string
				if json.Unmarshal([]byte(mutations.String()), &muts) == nil {
					t.Logf("=== DOM Mutations in React root (%d) ===", len(muts))
					for i, m := range muts {
						if i < 50 {
							t.Logf("  [%d] %s", i, m)
						}
					}
				}
			}
			// Check react-app root structure
			domCheck, _ := vm.RunString(`(function() {
				var reactRoot = document.querySelector('[data-target="react-app.reactRoot"]');
				if (!reactRoot) return JSON.stringify({error: 'no react root'});
				var result = {
					childCount: reactRoot.childNodes.length,
					children: [],
					innerHTML_length: reactRoot.innerHTML.length,
					innerHTML_start: reactRoot.innerHTML.substring(0, 200)
				};
				for (var i = 0; i < Math.min(reactRoot.childNodes.length, 20); i++) {
					var child = reactRoot.childNodes[i];
					var info = {
						nodeType: child.nodeType,
						nodeName: child.nodeName,
					};
					if (child.nodeType === 8) info.data = child.data;
					if (child.nodeType === 1) {
						info.tagName = child.tagName;
						info.className = child.className ? child.className.substring(0, 100) : '';
						info.childCount = child.childNodes.length;
					}
					result.children.push(info);
				}

				// Check if there's an error fallback
				var errorFallback = reactRoot.querySelector('[data-testid="list-error-fallback"]');
				result.hasErrorFallback = !!errorFallback;
				if (errorFallback) {
					result.errorFallbackParent = errorFallback.parentNode ? errorFallback.parentNode.tagName : 'none';
				}

				// Check for issue rows
				var issueRows = reactRoot.querySelectorAll('[class*="IssueRow"]');
				result.issueRowCount = issueRows.length;

				return JSON.stringify(result);
			})()`)
			if domCheck != nil {
				t.Logf("React root DOM: %s", domCheck.String())
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Final URL: %s", page.URL)
	t.Logf("Final Status: %d", page.StatusCode)
	t.Logf("HTML: %d, RenderedHTML: %d, Resources: %d", len(page.HTML), len(page.RenderedHTML), len(page.Resources))
	t.Logf("Navigation history: %v", page.NavigationHistory)

	for _, sf := range page.ScriptFailures {
		t.Logf("Script failure: %s -> %v", sf.URL, sf.Err)
	}

	t.Logf("=== Captured Errors/Logs (%d) ===", len(capturedErrors))
	for i, msg := range capturedErrors {
		t.Logf("  [%d] %s", i, msg)
	}

	// Save raw HTML (before JS execution)
	if err := os.WriteFile("github_issues_raw.html", []byte(page.HTML), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("Saved raw to github_issues_raw.html (%d bytes)", len(page.HTML))

	// Check if issue titles exist in raw vs rendered
	for _, title := range []string{"Initialization issue"} {
		inRaw := strings.Contains(page.HTML, title)
		inRendered := strings.Contains(page.RenderedHTML, title)
		t.Logf("Title %q: in raw HTML = %v, in rendered HTML = %v", title, inRaw, inRendered)
	}

	// Check error fallback
	hasErrorFallback := strings.Contains(page.RenderedHTML, "list-error-fallback")
	t.Logf("Has error fallback: %v", hasErrorFallback)

	output := page.RenderedHTML
	if output == "" {
		output = page.HTML
	}
	if err := os.WriteFile("github_issues.html", []byte(output), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("Saved to github_issues.html (%d bytes)", len(output))
}
