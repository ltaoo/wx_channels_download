import { SyncPageModel } from "./sync.model.js";

function SyncServerCard(props) {
  const server = props.server;
  return View(
    {
      as: "article",
      class: "sync-server-card",
      attributes: { n: "sync-server-card" },
    },
    [
      View(
        {
          class: "sync-server-card__identity",
          attributes: { n: "sync-server-identity" },
        },
        [
          View(
            {
              class: "sync-server-card__icon",
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
              class: "sync-server-card__text",
              attributes: { n: "sync-server-text" },
            },
            [
              View(
                { class: "sync-server-card__title-row" },
                [
                  View(
                    {
                      as: "h2",
                      class: "sync-server-card__name",
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
                  class: "sync-server-card__url",
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
        when: server.message,
        ok() {
          return View(
            {
              class: Timeless.classNames([
                "sync-server-card__message",
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
      class: "content-toolbar content-filter-form",
      attributes: { n: "sync-toolbar", role: "search" },
      onSubmit(event) {
        event.preventDefault();
      },
    },
    [
      View(
        {
          class: "content-filter-fields dm-flex dm-items-center dm-gap-2",
          attributes: { n: "sync-toolbar-fields" },
        },
        [
          View(
            {
              class: "content-filter-search",
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
          class: "content-filter-actions dm-flex dm-items-center dm-gap-2",
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
      class: "content-page sync-page page",
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
          class: "content-toolbar-wrap sync-toolbar-wrap",
          attributes: { n: "sync-toolbar-wrap" },
        },
        [SyncPageToolbar({ store: model })],
      ),
      View(
        {
          class: "content-main sync-main",
          attributes: { n: "sync-main" },
        },
        [
          Show({
            when: model.state.error,
            ok() {
              return View(
                {
                  class: "sync-page__error",
                  attributes: { n: "sync-page-error", role: "alert" },
                },
                [model.state.error.value],
              );
            },
          }),
          Show({
            when: computed(
              model.state.visible_servers,
              (servers) => servers.length > 0,
            ),
            ok() {
              return View(
                {
                  class: "sync-server-list",
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
                  class: "sync-page__empty",
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
