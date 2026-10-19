import { SyncPageModel } from "./sync.model.js";

function SyncServerCard(props) {
  const server = props.server;
  return View(
    {
      as: "article",
      class: "grid grid-cols-[minmax(0,_1fr)_auto] items-center gap-4 p-4 border-[1px_solid_var(--dm-content-border)] rounded-lg bg-content-panel max-640:items-stretch max-640:grid-cols-1fr",
      attributes: { n: "sync-server-card" },
    },
    [
      View(
        {
          class: "flex min-w-0 items-center gap-3",
          attributes: { n: "sync-server-identity" },
        },
        [
          View(
            {
              class: "grid w-42px h-42px flex-[0_0_auto] place-items-center text-color-primary rounded-md bg-content-panel-soft",
              attributes: { n: "sync-server-icon", "aria-hidden": "true" },
            },
            [
              Timeless.Icon({
                name: "hard-drive",
                size: 22,
                attributes: { n: "sync-server-icon-glyph" },
              }),
            ],
          ),
          View(
            {
              class: "min-w-0",
              attributes: { n: "sync-server-text" },
            },
            [
              View(
                { class: "flex min-w-0 items-center gap-2" },
                [
                  View(
                    {
                      as: "h2",
                      class: "overflow-hidden min-w-0 m-0 text-content-text text-base text-ellipsis whitespace-nowrap",
                      attributes: { n: "sync-server-name" },
                    },
                    [server.name],
                  ),
                  View(
                    {
                      class: server.version_class,
                      attributes: { n: "sync-server-version" },
                    },
                    [server.version_text],
                  ),
                  View(
                    {
                      class: server.device_status_class,
                      attributes: {
                        n: "sync-server-status",
                        title: server.device_status_title,
                      },
                    },
                    [server.device_status_label],
                  ),
                ],
              ),
              View(
                {
                  class: "text-content-muted overflow-hidden mt-1 font-[var(--dm-font-family-mono,_monospace)] text-sm text-ellipsis whitespace-nowrap",
                  attributes: { n: "sync-server-url", title: server.url },
                },
                [server.url],
              ),
            ],
          ),
        ],
      ),
      Button(
        {
          store: server.button$,
          attributes: {
            n: "sync-server-action",
            type: "button",
            "aria-label": `同步到 ${server.name}`,
          },
          prefix: Timeless.Icon({
            name: "upload",
            size: 16,
            attributes: { n: "sync-server-action-icon" },
          }),
        },
        ["同步到该服务"],
      ),
      Show({
        when: computed(server.status, (status) => status === "success"),
        ok() {
          return View(
            {
              class: Timeless.classNames([
                "grid-column-[1_/_-1] text-sm [&.is-success]:text-color-success [&.is-error]:text-color-danger",
                server.message_class,
              ]),
              attributes: { n: "sync-server-status", role: "status" },
            },
            [server.message],
          );
        },
      }),
    ],
  );
}

function SyncPageToolbar(props) {
  const model = props.store;
  return View(
    {
      type: "form",
      class: "group-content-toolbar flex justify-between gap-3 max-760:flex-wrap items-stretch",
      attributes: { n: "sync-toolbar", role: "search" },
      onSubmit(event) {
        event.preventDefault();
      },
    },
    [
      View(
        {
          class: "min-w-0 flex-[1_1_0] max-760:flex-[1_1_100%] max-520:flex-wrap dm-flex dm-items-center dm-gap-2",
          attributes: { n: "sync-toolbar-fields" },
        },
        [
          View(
            {
              class: "min-w-0 flex-[1_1_0] max-520:basis-full",
              attributes: { n: "sync-search-field" },
            },
            [
              Input({
                store: model.ui.input_keyword$,
                rootAttributes: { n: "sync-search-control" },
                prefix: Timeless.Icon({
                  name: "search",
                  size: 16,
                  attributes: { n: "sync-search-icon" },
                }),
                attributes: {
                  n: "sync-search-input",
                  name: "keyword",
                  type: "search",
                  autocomplete: "off",
                  "aria-label": "搜索服务名称或地址",
                },
              }),
            ],
          ),
        ],
      ),
      View(
        {
          class: "group-content-filter-actions flex-[0_0_auto] max-760:ml-auto max-520:w-full max-520:ml-0 max-520:[&_>_.dm-button]:flex-[1_1_0] dm-flex dm-items-center dm-gap-2",
          attributes: { n: "sync-toolbar-actions" },
        },
        [
          Button(
            {
              store: model.ui.refresh_button$,
              attributes: { n: "sync-refresh", type: "button" },
              prefix: Timeless.Icon({
                name: "refresh-cw",
                size: 16,
                attributes: { n: "sync-refresh-icon" },
              }),
            },
            ["刷新"],
          ),
        ],
      ),
    ],
  );
}

export default function SyncPageView(props) {
  const model = SyncPageModel(props);
  return View(
    {
      class: "group-content-page min-h-full flex flex-col bg-content-bg text-content-text [&_>_.group-sync-toolbar-wrap]:flex-[0_0_auto] [&_>_.group-sync-main]:min-h-0 [&_>_.group-sync-main]:flex [&_>_.group-sync-main]:flex-[1_1_auto] [&_>_.group-sync-main]:flex-col [&_>_.group-sync-main]:gap-4 max-640:[&_.group-content-toolbar]:items-stretch max-640:[&_.group-content-toolbar]:flex-col max-640:[&_.group-content-filter-actions]:justify-end min-w-0 min-h-full font-sans",
      attributes: { n: "sync-page" },
      onMounted() {
        model.methods.ready();
      },
      onUnmounted() {
        model.methods.destroy();
      },
    },
    [
      View(
        {
          class: "group-content-toolbar-wrap p-[var(--dm-space-4)_var(--dm-space-4)_0] group-sync-toolbar-wrap",
          attributes: { n: "sync-toolbar-wrap" },
        },
        [SyncPageToolbar({ store: model })],
      ),
      View(
        {
          class: "group-content-main flex-[1_1_auto] min-w-0 p-4 group-sync-main",
          attributes: { n: "sync-main" },
        },
        [

          Show({
            when: computed(
              model.state.visible_servers,
              (servers) => servers.length > 0,
            ),
            ok() {
              return View(
                {
                  class: "grid gap-3",
                  attributes: { n: "sync-server-list" },
                },
                [
                  For({
                    each: model.state.visible_servers,
                    render(server) {
                      return SyncServerCard({ server });
                    },
                  }),
                ],
              );
            },
            else() {
              return View(
                {
                  class: "text-content-muted p-8 text-center",
                  attributes: { n: "sync-page-empty" },
                },
                [model.state.empty_message],
              );
            },
          }),
        ],
      ),
    ],
  );
}
