package minib

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestGitHubHydrationDebug(t *testing.T) {
	browser, err := NewMiniBrowser(120 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	page, err := browser.Navigate(ctx, "https://github.com/ltaoo/wx_channels_download/issues", nil, NavigateOptions{
		RuntimeInitializer: func(vm *goja.Runtime, p *Page) error {
			_, err := vm.RunString(`
				(function() {
					window.__debug = {
						errorFallbackStack: null,
						fiberError: null,
						consoleErrors: []
					};

					// Intercept setAttribute to capture error fallback
					var _origSetAttr = Element.prototype.setAttribute;
					Element.prototype.setAttribute = function(name, value) {
						if (name === 'data-testid' && value === 'list-error-fallback') {
							window.__debug.errorFallbackStack = new Error().stack;

							// Walk up DOM to find React fiber
							var node = this;
							while (node) {
								var keys = Object.getOwnPropertyNames(node);
								for (var i = 0; i < keys.length; i++) {
									var key = keys[i];
									if (key.indexOf('__reactFiber') === 0 || key.indexOf('__reactContainer') === 0 || key.indexOf('__reactInternalInstance') === 0) {
										var fiber = node[key];
										var visited = 0;
										while (fiber && visited < 100) {
											visited++;
											// Look for error boundary (class component with getDerivedStateFromError)
											if (fiber.tag === 1 && fiber.stateNode && fiber.stateNode.state) {
												var state = fiber.stateNode.state;
												if (state.error || state.hasError) {
													window.__debug.fiberError = {
														message: state.error ? (state.error.message || String(state.error)) : 'hasError=true',
														stack: state.error && state.error.stack ? state.error.stack.substring(0, 800) : '',
														componentName: fiber.type ? (fiber.type.displayName || fiber.type.name || '') : ''
													};
													break;
												}
											}
											fiber = fiber['return'];
										}
										break;
									}
								}
								node = node.parentNode;
							}
						}
						return _origSetAttr.call(this, name, value);
					};

					// Intercept console.error
					var _origConsoleError = console.error;
					console.error = function() {
						var msg = Array.prototype.slice.call(arguments).map(function(a) {
							if (a instanceof Error) return 'Error: ' + a.message + '\n' + (a.stack || '').substring(0, 500);
							if (typeof a === 'string') return a;
							if (a && a.message) return a.message;
							try { return JSON.stringify(a).substring(0, 200); } catch(e) { return String(a); }
						}).join(' ');
						if (msg.length > 0 && window.__debug.consoleErrors.length < 50) {
							window.__debug.consoleErrors.push(msg.substring(0, 1000));
						}
						return _origConsoleError.apply(console, arguments);
					};
				})();
			`)
			return err
		},
		RuntimeFinalizer: func(vm *goja.Runtime, p *Page) error {
			val, _ := vm.RunString(`JSON.stringify(window.__debug || {})`)
			if val != nil {
				var debug struct {
					ErrorFallbackStack string      `json:"errorFallbackStack"`
					FiberError         interface{} `json:"fiberError"`
					ConsoleErrors      []string    `json:"consoleErrors"`
				}
				if json.Unmarshal([]byte(val.String()), &debug) == nil {
					if debug.ErrorFallbackStack != "" {
						t.Logf("=== ERROR FALLBACK STACK ===\n%s", debug.ErrorFallbackStack)
					}
					if debug.FiberError != nil {
						fe, _ := json.MarshalIndent(debug.FiberError, "", "  ")
						t.Logf("=== FIBER ERROR ===\n%s", string(fe))
					} else {
						t.Log("No fiber error found via stateNode.state")
					}
					t.Logf("=== Console errors (%d) ===", len(debug.ConsoleErrors))
					for i, e := range debug.ConsoleErrors {
						t.Logf("  [%d] %s", i, e)
					}
				}
			}

			// Try to find error on fiber tree starting from react root
			fiberWalk, _ := vm.RunString(`(function() {
				var root = document.querySelector('[data-target="react-app.reactRoot"]');
				if (!root) return JSON.stringify({error: 'no root'});

				// Find the React fiber key
				var fiberKey = null;
				var names = Object.getOwnPropertyNames(root);
				for (var i = 0; i < names.length; i++) {
					if (names[i].indexOf('__reactFiber') === 0 || names[i].indexOf('__reactContainer') === 0) { fiberKey = names[i]; break; }
				}
				if (!fiberKey) return JSON.stringify({error: 'no fiber key', keys: names.filter(function(k) { return k.indexOf('__') === 0; })});

				var fiber = root[fiberKey];
				var errorBoundaries = [];
				var queue = [fiber];
				var visited = 0;

				while (queue.length > 0 && visited < 500) {
					var f = queue.shift();
					if (!f) continue;
					visited++;

					// Check if this is an error boundary with error state
					if (f.tag === 1 && f.stateNode) {
						var s = f.stateNode;
						try {
							if (s.state) {
								var sk = Object.keys(s.state);
								for (var j = 0; j < sk.length; j++) {
									var v = s.state[sk[j]];
									if (v instanceof Error || (v && v.message && v.stack)) {
										errorBoundaries.push({
											name: f.type ? (f.type.displayName || f.type.name || 'ClassComp') : 'unknown',
											stateKey: sk[j],
											errorMessage: v.message ? v.message.substring(0, 500) : String(v).substring(0, 200),
											errorStack: v.stack ? v.stack.substring(0, 800) : ''
										});
									}
								}
							}
						} catch(e) {}
					}

					if (f.child) queue.push(f.child);
					if (f.sibling) queue.push(f.sibling);
				}

				return JSON.stringify({
					fiberKey: fiberKey,
					visited: visited,
					errorBoundaries: errorBoundaries
				});
			})()`)
			if fiberWalk != nil {
				t.Logf("Fiber walk: %s", fiberWalk.String())
			}

			domCheck, _ := vm.RunString(`(function() {
				var rr = document.querySelector('[data-target="react-app.reactRoot"]');
				return JSON.stringify({
					hasErrorFallback: rr && !!rr.querySelector('[data-testid="list-error-fallback"]'),
					hasIssueRows: rr && rr.innerHTML.indexOf('IssueRow') !== -1
				});
			})()`)
			if domCheck != nil {
				t.Logf("Final: %s", domCheck.String())
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Status: %d", page.StatusCode)
}
