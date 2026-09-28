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
                {
                  as: "h2",
                  class: "sync-server-card__name",
                  attributes: { n: "sync-server-name" },
                },
                [server.name],
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
          class: "sync-page__header container",
          attributes: { n: "sync-page-header" },
        },
        [
          View(
            {
              class: "sync-page__heading",
              attributes: { n: "sync-page-heading" },
            },
            [
              View(
                {
                  as: "h1",
                  class: "sync-page__title",
                  attributes: { n: "sync-page-title" },
                },
                ["下载服务同步"],
              ),
              View(
                {
                  class: "sync-page__description",
                  attributes: { n: "sync-page-description" },
                },
                ["将当前服务的下载记录、内容记录和已下载文件同步到另一台下载服务。"],
              ),
            ],
          ),
          Button(
            {
              store: model.ui.refresh_button$,
              attributes: { n: "sync-page-refresh", type: "button" },
              prefix: Timeless.Icon({
                name: "refresh-cw",
                size: 16,
                attributes: { n: "sync-page-refresh-icon" },
              }),
            },
            ["刷新"],
          ),
        ],
      ),
      View(
        {
          class: "sync-page__body container",
          attributes: { n: "sync-page-body" },
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
            when: computed(model.state.servers, (servers) => servers.length > 0),
            ok() {
              return View(
                {
                  class: "sync-server-list",
                  attributes: { n: "sync-server-list" },
                },
                [
                  For({
                    each: model.state.servers,
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
