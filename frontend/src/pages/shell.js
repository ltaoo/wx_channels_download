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
      class: "grid grid-cols-[var(--dm-space-24)_minmax(0,_1fr)] w-full h-full bg-content-bg max-640:grid-cols-[minmax(0,_1fr)] max-640:grid-rows-[minmax(0,_1fr)_auto] min-w-0 min-h-full text-color-text-primary font-sans",
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
          class: "relative z-raised flex flex-col text-brand-sidebar-text bg-brand-sidebar border-right-[var(--dm-border-width)_solid_var(--dm-color-sidebar-border)] gap-4-5 p-[var(--dm-space-4)_var(--dm-space-3)] shadow-[1px_0_0_var(--dm-color-border-translucent)] max-640:grid-row-2 max-640:grid-column-1 max-640:p-[var(--dm-space-1-5)_var(--dm-space-2)] max-640:border-top-[var(--dm-border-width)_solid_var(--dm-color-sidebar-border)] max-640:border-right-0 dm-flex dm-flex-col",
          attributes: { n: "app-sider" },
        },
        [
          View(
            { class: "flex items-center flex-col border-bottom-[var(--dm-border-width)_solid_var(--dm-color-sidebar-border)] gap-2 p-[0_var(--dm-space-2)_var(--dm-space-4)_var(--dm-space-2)] max-640:!hidden", attributes: { n: "app-brand" } },
            [
              View(
                {
                  as: "button",
                  class: "block p-0 bg-transparent border-0 rounded-content cursor-pointer focus-visible:outline-0 focus-visible:shadow-focus-ring",
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
                  class: "relative w-fit-content min-w-0 flex-[0_0_auto] m-[0_auto] text-brand-sidebar-muted font-mono text-caption font-semibold leading-snug",
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
                          class: "absolute top--4px right--6px w-7px h-7px pointer-events-none bg-color-danger rounded-round shadow-[0_0_0_2px_var(--dm-brand-sidebar)]",
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
                          class: "max-w-full overflow-hidden text-ellipsis whitespace-nowrap",
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
            class: "flex flex-col gap-2 p-1 overflow-y-auto [&_.dm-button--sidebar-nav]:flex-col [&_.dm-button--sidebar-nav]:items-center [&_.dm-button--sidebar-nav]:justify-center [&_.dm-button--sidebar-nav]:gap-2 [&_.dm-button--sidebar-nav]:min-h-16 [&_.dm-button--sidebar-nav]:p-[var(--dm-space-1-5)_var(--dm-space-1)] [&_.dm-button--sidebar-nav]:text-sm [&_.dm-button--sidebar-nav]:font-semibold [&_.dm-button--sidebar-nav]:leading-snug [&_.dm-button--sidebar-nav]:text-center min-h-0 flex-[1_1_auto] max-640:w-full max-640:min-h-auto max-640:flex-[1_1_auto] max-640:!flex-row max-640:gap-1 max-640:overflow-x-auto",
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
                        class: "grid place-items-center w-6 h-6 shrink-0 text-currentColor",
                        attributes: { n: `${sidebar_menu_id(menu)}-icon`, "aria-hidden": "true" },
                      },
                      [Timeless.Icon({ name: menu.icon, size: 24 })],
                    ),
                    View(
                      {
                        as: "span",
                        class: "max-w-full overflow-hidden text-ellipsis whitespace-nowrap",
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
                class: "flex mt-auto [&_>_.dm-button--sidebar-nav]:w-full max-640:min-w-52px max-640:flex-[1_1_0] max-640:mt-0 max-640:[&_>_.dm-button--sidebar-nav]:w-auto",
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
                        class: "grid place-items-center w-6 h-6 shrink-0 text-currentColor",
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
          class: "relative z-1 min-w-0 h-full overflow-auto overflow-x-hidden bg-content-bg [background-color:var(--dm-content-bg)] bg-image-[radial-gradient(_circle,_var(--dm-color-border-translucent)_var(--dm-border-width),_transparent_var(--dm-border-width)_)] background-position-[var(--dm-space-3)_var(--dm-space-3)] background-size-[var(--dm-space-6)_var(--dm-space-6)] max-640:grid-row-1 max-640:grid-column-1 dm-min-w-0",
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
