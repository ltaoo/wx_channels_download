import {
  SettingsDialog,
  UpdateDialog,
} from "./shell.components.js";
import { ShellViewModel } from "./shell.model.js";

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

function sidebar_menu_id(menu) {
  return `app-menu-${menu.name.split(".").pop()}`;
}

export default function SiderLayoutView(props) {
  var model = ShellViewModel(props);

  return View(
    {
      class: "grid w-full h-full min-w-0 min-h-full grid-cols-avatar-content max-640:grid-cols-1 max-640:grid-rows-content-footer font-sans bg-content-bg text-color-text-primary",
      onMounted: function () {
        model.methods.ready();
      },
      onUnmounted: function () {
        model.methods.destroy();
      },
    },
    [
      View(
        {
          as: "aside",
          class: "flex relative z-raised flex-col dm-flex-col max-640:grid-row-2 max-640:grid-column-1 py-4 px-3 max-640:py-1.5 max-640:px-2 gap-4.5 border-r-default border-r-solid border-r-color-sidebar max-640:border-t-default max-640:border-t-solid max-640:border-t-color-sidebar max-640:border-right-0 bg-brand-sidebar text-brand-sidebar-text shadow-sidebar-divider dm-flex",
          attributes: { n: "app-sider" },
        },
        [
          View(
            { class: "flex max-640:!hidden flex-col items-center pt-0 pb-4 px-2 gap-2 border-b-default border-b-solid border-b-color-sidebar", attributes: { n: "app-brand" } },
            [
              View(
                {
                  as: "button",
                  class: "block p-0 border-0 rounded-content focus-visible:outline-0 bg-transparent focus-visible:shadow-focus-ring cursor-pointer",
                  attributes: {
                    n: "app-brand-logo-action",
                    type: "button",
                    title: "返回首页",
                    "aria-label": "返回首页",
                  },
                  onClick() {
                    return model.methods.goHome();
                  },
                },
                [
                  Img({
                    class: "block w-auto max-w-full h-52px object-contain object-position-center user-select-none",
                    src: "public/logo.png?v=logo-only-v4",
                    alt: "D&M",
                    attributes: {
                      n: "app-brand-logo",
                      draggable: "false",
                    },
                  }),
                ],
              ),
              View(
                {
                  class: "relative w-fit-content min-w-0 flex-none my-0 mx-auto font-mono text-caption font-semibold leading-snug text-brand-sidebar-muted",
                  attributes: { n: "app-brand-version" },
                },
                [
                  Show({
                    when: model.models.update.state.notice_visible,
                    ok() {
                      var current_version =
                        model.models.update.state.current_version.value;
                      var latest_version =
                        model.models.update.state.latest_version.value;
                      return [
                        Button(
                          {
                            store: model.models.update.ui.notice_button$,
                            class: "dm-button--version",
                            attributes: {
                              n: "app-version-update-action",
                              type: "button",
                              title:
                                `当前版本 ${current_version}，发现新版本 ${latest_version}`,
                              "aria-label":
                                `当前版本 ${current_version}，发现新版本 ${latest_version}，点击查看`,
                            },
                          },
                          [current_version],
                        ),
                        View({
                          class: "absolute top--4px right--6px w-7px h-7px rounded-round bg-color-danger shadow-sidebar-focus pointer-events-none",
                          attributes: {
                            n: "app-brand-version-dot",
                            "aria-hidden": "true",
                          },
                        }),
                      ];
                    },
                    else() {
                      return View(
                        {
                          class: "max-w-full overflow-hidden whitespace-nowrap text-ellipsis",
                          attributes: { n: "app-brand-version-text" },
                        },
                        [
                          model.models.update.state.current_version.value,
                        ],
                      );
                    },
                  }),
                ],
              ),
            ],
          ),
        View(
          {
            as: "nav",
            class: "flex min-h-0 max-640:w-full max-640:min-h-auto flex-col flex-auto max-640:flex-auto max-640:!flex-row p-1 gap-2 max-640:gap-1 overflow-y-auto max-640:overflow-x-auto nav-icon-menu",
            ariaLabel: "页面导航",
            attributes: { n: "app-menu" },
          },
          [
            For({
              each: model.state.menu_items,
              render: function (item) {
                var menu = item.menu;
                return Button(
                  {
                    store: item.button$,
                    class: Timeless.classNames([
                      "dm-button--sidebar-nav",
                      Timeless.computed(
                        model.models.menu.cur,
                        function (current) {
                          return model.models.menu.isSelected(current, menu)
                            ? "is-active"
                            : null;
                        },
                      ),
                    ]),
                    attributes: { n: sidebar_menu_id(menu), title: menu.title },
                  },
                  [
                    View(
                      {
                        class: "grid w-6 h-6 shrink-0 place-items-center text-currentColor",
                        attributes: { n: `${sidebar_menu_id(menu)}-icon`, "aria-hidden": "true" },
                      },
                      [Timeless.Icon({ name: menu.icon, size: 24 })],
                    ),
                    View(
                      {
                        as: "span",
                        class: "max-w-full overflow-hidden whitespace-nowrap text-ellipsis",
                        attributes: { n: `${sidebar_menu_id(menu)}-label` },
                      },
                      [menu.title],
                    ),
                  ],
                );
              },
            }),
            View(
              {
                class: "flex max-640:min-w-52px max-640:grow max-640:shrink max-640:basis-0 mt-auto max-640:mt-0 nav-footer-action ",
                attributes: { n: "app-settings-action-container" },
              },
              [
                Button(
                  {
                    store: model.ui.settings_button$,
                    class: "dm-button--sidebar-nav",
                    attributes: {
                      n: "app-settings-action",
                      "aria-haspopup": "dialog",
                      "aria-label": "打开设置",
                      title: "打开设置",
                    },
                  },
                  [
                    View(
                      {
                        class: "grid w-6 h-6 shrink-0 place-items-center text-currentColor",
                        attributes: { n: "app-settings-action-icon", "aria-hidden": "true" },
                      },
                      [Timeless.Icon({ name: "settings", size: 24 })],
                    ),
                  ],
                ),
              ],
            ),
          ],
        ),
        // View({ class: "app-sider__footer" }, [
        //   View({ class: "app-sider__footer-icon" }, [
        //     Timeless.Icon({ name: "hard-drive", size: 16 }),
        //   ]),
        //   View({ class: "app-sider__footer-copy" }, [
        //     View({ class: "app-sider__footer-title" }, ["本地工作区"]),
        //     View({ class: "app-sider__footer-text" }, [
        //       "内容与任务保存在本机",
        //     ]),
        //   ]),
        // ]),
      ]),
      View(
        {
          as: "main",
          class: "relative z-1 min-w-0 h-full dm-min-w-0 max-640:grid-row-1 max-640:grid-column-1 overflow-auto overflow-x-hidden bg-content-bg bg-color-content bg-image-page-grid bg-position-grid bg-size-grid",
          attributes: { n: "app-content" },
        },
        [
          Timeless.ui.KeepAliveSubViews({
            app: props.app,
            tip: props.tip,
            client: props.client,
            history: props.history,
            hlsPlayer: props.hlsPlayer,
            storage: props.storage,
            view: props.view,
            views: props.views,
            placeholder: LoadingView,
            ErrorFallback: (error, name) => ErrorFallbackView(error, name, props.tip),
          }),
        ],
      ),
      SettingsDialog({
        tip: props.tip,
        dialog: model.ui.settings_dialog$,
        section: model.state.settings_section,
        version: model.state.version,
        certificate: model.state.certificate,
        loading: model.state.certificate_loading,
        error: model.state.certificate_error,
        certificate_menu_button: model.ui.certificate_menu_button$,
        mcp_menu_button: model.ui.mcp_menu_button$,
        about_menu_button: model.ui.about_menu_button$,
        mcp_model: model.models.mcp,
        mcp_refresh_button: model.models.mcp.ui.refresh_button$,
        refresh_button: model.ui.refresh_certificate_button$,
        retry_button: model.ui.retry_certificate_button$,
        delete_button: model.ui.delete_certificate_button$,
      }),
      UpdateDialog({ store: model.models.update }),
    ],
  );
}
