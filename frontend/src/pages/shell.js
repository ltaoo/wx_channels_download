import {
  SettingsDialog,
  UpdateDialog,
} from "./shell.components.js";
import { ShellViewModel } from "./shell.model.js";

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

export default function SiderLayoutView(props) {
  var model = ShellViewModel(props);

  return View(
    {
      class: "app-shell page",
      onMounted: function () {
        model.methods.ready();
      },
      onUnmounted: function () {
        model.methods.destroy();
      },
    },
    [
      View({ as: "aside", class: "app-sider dm-flex dm-flex-col" }, [
        View({ class: "app-brand" }, [
          Img({
            class: "app-brand__logo",
            src: "public/logo.png?v=logo-only-v4",
            alt: "D&M",
            attributes: {
              draggable: "false",
            },
          }),
          View({ class: "app-brand__version" }, [
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
                        title: `当前版本 ${current_version}，发现新版本 ${latest_version}`,
                        "aria-label": `当前版本 ${current_version}，发现新版本 ${latest_version}，点击查看`,
                      },
                    },
                    [current_version],
                  ),
                  View({
                    class: "app-brand__version-dot",
                    attributes: { "aria-hidden": "true" },
                  }),
                ];
              },
              else() {
                return View({ class: "app-brand__version-text" }, [
                  model.models.update.state.current_version.value,
                ]);
              },
            }),
          ]),
        ]),
        View(
          {
            as: "nav",
            class: "app-menu dm-flex dm-flex-col dm-gap-1",
            ariaLabel: "页面导航",
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
                  },
                  [
                    View({ class: "app-menu__icon" }, [
                      Timeless.Icon({ name: menu.icon, size: 17 }),
                    ]),
                    View({ as: "span", class: "app-menu__label" }, [
                      menu.title,
                    ]),
                  ],
                );
              },
            }),
            View(
              {
                class: "app-menu__settings",
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
                    },
                  },
                  [
                    View({ class: "app-menu__icon" }, [
                      Timeless.Icon({ name: "settings", size: 17 }),
                    ]),
                    View({ as: "span", class: "app-menu__label" }, ["设置"]),
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
          class: "app-content dm-min-w-0",
          attributes: { n: "app-content" },
        },
        [
          Timeless.ui.KeepAliveSubViews({
            app: props.app,
            client: props.client,
            history: props.history,
            storage: props.storage,
            view: props.view,
            views: props.views,
            placeholder: LoadingView,
            ErrorFallback: ErrorFallbackView,
          }),
        ],
      ),
      SettingsDialog({
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
