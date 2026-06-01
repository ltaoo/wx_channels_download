/**
 * Sandbox management page
 * @param {ViewComponentProps} props
 */
export default function SandboxesPageView(props) {
  const sandbox$ = new SandboxModel(props.client);
  const list$ = new Timeless.ReactiveVar([]);
  const selected$ = new Timeless.ReactiveVar(null);
  const vncTicket$ = new Timeless.ReactiveVar("");
  const loading$ = new Timeless.ReactiveVar(false);

  const statusColors = {
    running: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
    paused: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300",
    creating: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
    error: "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300",
    destroyed: "bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400",
  };

  async function refreshList() {
    loading$.set(true);
    try {
      const data = await sandbox$.list();
      list$.set(data || []);
    } catch (e) {
      props.app.tip({ type: "error", text: ["Failed to load sandboxes", String(e)] });
    } finally {
      loading$.set(false);
    }
  }

  async function createSandbox() {
    loading$.set(true);
    try {
      await sandbox$.create({});
      props.app.tip({ text: ["Sandbox created"] });
      await refreshList();
    } catch (e) {
      props.app.tip({ type: "error", text: ["Create failed", String(e)] });
    } finally {
      loading$.set(false);
    }
  }

  async function destroySandbox(id) {
    if (!confirm("Destroy this sandbox? This cannot be undone.")) return;
    try {
      await sandbox$.destroy(id);
      if (selected$.get()?.id === id) selected$.set(null);
      props.app.tip({ text: ["Sandbox destroyed"] });
      await refreshList();
    } catch (e) {
      props.app.tip({ type: "error", text: ["Destroy failed", String(e)] });
    }
  }

  async function togglePause(rec) {
    try {
      if (rec.status === "running") {
        await sandbox$.pause(rec.id);
      } else {
        await sandbox$.resume(rec.id);
      }
      await refreshList();
    } catch (e) {
      props.app.tip({ type: "error", text: ["Operation failed", String(e)] });
    }
  }

  async function selectSandbox(rec) {
    selected$.set(rec);
    if (rec.status === "running") {
      try {
        const resp = await sandbox$.applySession(rec.id, { ttl_sec: 3600 });
        vncTicket$.set(resp.ticket);
      } catch (e) {
        vncTicket$.set("");
      }
    }
  }

  async function takeScreenshot() {
    const rec = selected$.get();
    if (!rec) return;
    try {
      const result = await sandbox$.screenshot(rec.id, { format: "png" });
      if (result?.data) {
        // Open screenshot in new tab
        const w = window.open("", "_blank");
        if (w) {
          w.document.write(`<img src="data:image/${result.format};base64,${result.data}" />`);
        }
      }
    } catch (e) {
      props.app.tip({ type: "error", text: ["Screenshot failed", String(e)] });
    }
  }

  async function navigateURL() {
    const rec = selected$.get();
    if (!rec) return;
    const url = prompt("Enter URL:", rec.default_url || "https://example.com");
    if (!url) return;
    try {
      await sandbox$.actions(rec.id, [{ type: "navigate", url }]);
      props.app.tip({ text: ["Navigated to", url] });
    } catch (e) {
      props.app.tip({ type: "error", text: ["Navigation failed", String(e)] });
    }
  }

  // Auto-refresh on mount
  refreshList();

  function vncURL() {
    const rec = selected$.get();
    const ticket = vncTicket$.get();
    if (!rec || !ticket) return "";
    const proto = location.protocol === "https:" ? "wss" : "ws";
    return `${location.protocol}//${location.host}/api/v1/sandboxes/${rec.id}/session/vnc_lite.html?ticket=${ticket}`;
  }

  return View({ class: "sandboxes-page h-full flex flex-col" }, [
    // Header
    View({ class: "flex items-center justify-between px-6 py-4 border-b border-zinc-200 dark:border-zinc-800" }, [
      Text({ class: "text-lg font-semibold" }, "Sandboxes"),
      Flex({ items: "center", class: "gap-3" }, [
        Button({
          store: new Timeless.ui.ButtonCore({
            onClick: refreshList,
            variant: "outline",
            size: "sm",
          }),
        }, [Icon({ name: "refresh-cw", size: 16 }), " Refresh"]),
        Button({
          store: new Timeless.ui.ButtonCore({
            onClick: createSandbox,
            size: "sm",
          }),
        }, [Icon({ name: "plus", size: 16 }), " New Sandbox"]),
      ]),
    ]),

    // Main content: 3-column layout
    View({ class: "flex-1 flex overflow-hidden" }, [
      // Column 1: Container list
      View({ class: "w-64 border-r border-zinc-200 dark:border-zinc-800 overflow-y-auto p-3" }, [
        Show({
          when: computed(list$, (list) => list.length === 0),
          ok() {
            return View({ class: "text-center text-zinc-400 py-8" }, [
              Text({ class: "text-sm" }, "No sandboxes yet"),
              Text({ class: "text-xs mt-1" }, 'Click "New Sandbox" to create one'),
            ]);
          },
        }),
        Flex({ direction: "col", class: "gap-2" }, [
          ...(() => {
            const list = list$.get();
            return (list || []).map((rec) =>
              View({
                class: computed(selected$, (sel) => {
                  const base = "p-3 rounded-lg cursor-pointer transition-colors border ";
                  const active = sel?.id === rec.id
                    ? "border-blue-500 bg-blue-50 dark:bg-blue-950";
                  const inactive = "border-transparent hover:bg-zinc-100 dark:hover:bg-zinc-800";
                  return base + (sel?.id === rec.id ? active : inactive);
                }),
                onClick() { selectSandbox(rec); },
              }, [
                Flex({ items: "center", justify: "between", class: "mb-1" }, [
                  Text({ class: "text-sm font-medium truncate" }, rec.alias || rec.id),
                  View({
                    class: `text-xs px-2 py-0.5 rounded-full ${statusColors[rec.status] || statusColors.error}`,
                  }, rec.status),
                ]),
                Text({ class: "text-xs text-zinc-400 truncate" }, rec.id),
                Show({
                  when: computed(selected$, (sel) => sel?.id === rec.id),
                  ok() {
                    return Flex({ class: "gap-1 mt-2" }, [
                      Button({
                        store: new Timeless.ui.ButtonCore({
                          variant: "ghost",
                          size: "xs",
                          onClick() { togglePause(rec); },
                        }),
                      }, rec.status === "running" ? "Pause" : "Resume"),
                      Button({
                        store: new Timeless.ui.ButtonCore({
                          variant: "ghost",
                          size: "xs",
                          onClick() { destroySandbox(rec.id); },
                        }),
                      }, "Destroy"),
                    ]);
                  },
                }),
              ])
            );
          })(),
        ]),
      ]),

      // Column 2: Preview area
      View({ class: "flex-1 flex flex-col overflow-hidden" }, [
        Show({
          when: computed(selected$, (sel) => !sel),
          ok() {
            return View({ class: "flex-1 flex items-center justify-center text-zinc-400" }, [
              Text({ class: "text-sm" }, "Select a sandbox to preview"),
            ]);
          },
        }),
        Show({
          when: computed([selected$, vncTicket$], (sel, ticket) => sel && sel.status === "running" && ticket),
          ok() {
            return View({ class: "flex-1 bg-zinc-900" }, [
              // Use an iframe to embed noVNC
              View({
                tag: "iframe",
                class: "w-full h-full border-0",
                src: vncURL(),
              }),
            ]);
          },
        }),
        Show({
          when: computed(selected$, (sel) => sel && sel.status !== "running"),
          ok() {
            return View({ class: "flex-1 flex items-center justify-center" }, [
              Flex({ direction: "col", items: "center", class: "gap-3" }, [
                Text({ class: "text-zinc-400" }, "Sandbox is not running"),
                Button({
                  store: new Timeless.ui.ButtonCore({
                    onClick() {
                      const sel = selected$.get();
                      if (sel) togglePause(sel);
                    },
                  }),
                }, "Resume"),
              ]),
            ]);
          },
        }),
      ]),

      // Column 3: Action panel
      View({ class: "w-56 border-l border-zinc-200 dark:border-zinc-800 overflow-y-auto p-3" }, [
        Show({
          when: computed(selected$, (sel) => !sel),
          ok() { return null; },
        }),
        Show({
          when: computed(selected$, (sel) => !!sel),
          ok() {
            const sel = selected$.get();
            const isRunning = sel?.status === "running";
            return Flex({ direction: "col", class: "gap-2" }, [
              Text({ class: "text-sm font-medium mb-2" }, "Actions"),
              Button({
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  size: "sm",
                  disabled: !isRunning,
                  onClick: takeScreenshot,
                }),
              }, [Icon({ name: "camera", size: 14 }), " Screenshot"]),
              Button({
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  size: "sm",
                  disabled: !isRunning,
                  onClick: navigateURL,
                }),
              }, [Icon({ name: "globe", size: 14 }), " Navigate"]),
              Separator({ orientation: "horizontal", class: "my-2" }),
              Text({ class: "text-xs text-zinc-400" }, "Info"),
              Text({ class: "text-xs" }, `ID: ${sel?.id || "-"}`),
              Text({ class: "text-xs" }, `Kind: ${sel?.kind || "browser"}`),
              Text({ class: "text-xs" }, `Image: ${sel?.image || "-"}`),
              Show({
                when: sel?.endpoint?.cdp_host_port,
                ok() { return Text({ class: "text-xs" }, `CDP: :${sel.endpoint.cdp_host_port}`); },
              }),
              Show({
                when: sel?.endpoint?.session_host_port,
                ok() { return Text({ class: "text-xs" }, `VNC: :${sel.endpoint.session_host_port}`); },
              }),
            ]);
          },
        }),
      ]),
    ]),
  ]);
}
