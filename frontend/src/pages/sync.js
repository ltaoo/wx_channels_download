import { SyncPageModel } from "./sync.model.js";

function SyncServerCard(props) {
  const server = props.server;
  return View(
    {
      as: "article",
      class: "grid grid-cols-content-auto max-640:grid-cols-1fr items-center max-640:items-stretch p-4 gap-4 border border-solid border-color-content rounded-lg bg-content-panel",
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
              class: "grid w-42px h-42px flex-none place-items-center rounded-md bg-content-panel-soft text-color-primary",
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
                      class: "min-w-0 m-0 overflow-hidden text-base whitespace-nowrap text-ellipsis text-content-text",
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
                  class: "mt-1 overflow-hidden font-mono text-sm whitespace-nowrap text-ellipsis text-content-muted",
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
                "col-span-full text-sm status-message ",
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
      class: "flex max-760:flex-wrap justify-between items-stretch gap-3 group-content-toolbar",
      attributes: { n: "sync-toolbar", role: "search" },
      onSubmit(event) {
        event.preventDefault();
      },
    },
    [
      View(
        {
          class: "min-w-0 grow shrink basis-0 max-760:grow max-760:shrink max-760:basis-full max-520:flex-wrap dm-items-center dm-gap-2 dm-flex",
          attributes: { n: "sync-toolbar-fields" },
        },
        [
          View(
            {
              class: "min-w-0 grow shrink basis-0 max-520:basis-full",
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
          class: "max-520:w-full flex-none dm-items-center max-760:ml-auto max-520:ml-0 dm-gap-2 dm-flex group-content-filter-actions toolbar-actions",
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
      class: "flex min-h-full min-w-0 flex-col font-sans bg-content-bg text-content-text group-content-page section-page",
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
          class: "pt-4 pb-0 px-4 group-content-toolbar-wrap section-toolbar-root",
          attributes: { n: "sync-toolbar-wrap" },
        },
        [SyncPageToolbar({ store: model })],
      ),
      View(
        {
          class: "min-w-0 flex-auto p-4 group-content-main section-main-root",
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
                  class: "p-8 text-center text-content-muted",
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
