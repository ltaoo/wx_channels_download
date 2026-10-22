import { FileHelperViewModel } from "./filehelper.model.js";

function FileHelperHeaderView(props) {
  const vm$ = props.store;
  return View({ class: "grid min-h-72px flex-none grid-cols-content-auto items-center py-3 px-5 max-520:px-3 gap-4 border-b border-b-solid border-b-color-default bg-color-bg-surface text-color-text-primary shadow-divider backdrop-blur-14px" }, [
    View({ class: "flex min-w-0 items-center gap-3" }, [
      View({ class: "grid w-control-height-base h-control-height-base flex-none place-items-center rounded-content bg-color-primary-fill text-color-on-primary shadow-signal" }, [
        Timeless.Icon({ name: "upload", size: 18 }),
      ]),
      View({}, [
        View({ class: "text-base font-semibold whitespace-nowrap text-color-text-primary" }, ["文件传输助手"]),
        View({ class: "max-520:hidden mt-0.5 text-xs text-color-text-secondary" }, [
          "连接微信，跨设备传送内容",
        ]),
      ]),
    ]),
    View({ class: "flex min-w-0 items-center justify-end gap-2 text-sm" }, [
      Show({
        when: computed(
          vm$.state.channels_status,
          (status) => status !== "idle",
        ),
        ok() {
          return Tag(
            {
              name: "filehelper-channels-status",
              variant: "info",
              class: "max-520:hidden whitespace-nowrap text-color-text-secondary status-summary",
            },
            [
              "视频号API: ",
              View(
                {
                  class: computed(
                    vm$.state.channels_status,
                    (status) => `is-${status}`,
                  ),
                },
                [vm$.state.channels_status_text],
              ),
            ],
          );
        },
      }),
      Tag(
        {
          name: "filehelper-status",
          class: computed(vm$.state.logged_in, (logged_in) =>
            logged_in ? "whitespace-nowrap text-color-primary" : "whitespace-nowrap text-color-text-secondary",
          ),
        },
        [vm$.state.connection_text],
      ),
      Show({
        when: vm$.state.logged_in,
        ok() {
          return Button(
            {
              store: vm$.ui.btn_logout$,
              attributes: { n: "file-helper-logout-action" },
            },
            [
              computed(vm$.state.logout_loading, (loading) =>
                loading ? "退出中..." : "退出",
              ),
            ],
          );
        },
      }),
    ]),
  ]);
}

function FileHelperQRCodeView(props) {
  const vm$ = props.store;
  return Show({
    when: computed(vm$.state.login_stage, (stage) => stage === "scanned"),
    ok() {
      return View({ class: "flex min-h-200px flex-col items-center justify-center mb-5 gap-3" }, [
        Show({
          when: computed(vm$.state.scanned_avatar, (avatar) => Boolean(avatar)),
          ok() {
            return Timeless.Img({
              class: "w-80px h-80px object-cover rounded-avatar",
              src: vm$.state.scanned_avatar,
              alt: "用户头像",
            });
          },
        }),
        View({ class: "text-base text-color-text-primary" }, ["请在手机上确认登录"]),
      ]);
    },
    else() {
      return View({ class: "flex relative w-220px h-220px max-520:w-200px max-520:h-200px flex-none items-center justify-center mt-5 mb-4 mx-auto overflow-hidden border border-solid rounded-content border-color-default bg-color-bg-surface shadow-sm" }, [
        Show({
          when: computed(vm$.state.login_stage, (stage) => stage === "loading"),
          ok() {
            return View({ class: "w-40px h-40px border-3 border-solid rounded-round border-color-default border-t-color-primary animate-spin-fast" });
          },
        }),
        Show({
          when: computed(vm$.state.qrcode_url, (url) => Boolean(url)),
          ok() {
            return Timeless.Img({
              class: "w-full h-full object-contain",
              src: vm$.state.qrcode_url,
              alt: "登录二维码",
            });
          },
        }),
        Show({
          when: computed(
            vm$.state.login_stage,
            (stage) => stage === "expired" || stage === "error",
          ),
          ok() {
            return View({ class: "flex absolute inset-0 flex-col items-center justify-center gap-2.5 bg-surface/90" }, [
              View({ class: "text-base text-color-text-secondary" }, [
                computed(vm$.state.login_stage, (stage) =>
                  stage === "expired" ? "二维码已过期" : "二维码加载失败",
                ),
              ]),
              Button(
                {
                  store: vm$.ui.btn_refresh_qrcode$,
                  attributes: { n: "file-helper-refresh-qr-action" },
                },
                ["刷新二维码"],
              ),
            ]);
          },
        }),
      ]);
    },
  });
}

function FileHelperLoginView(props) {
  const vm$ = props.store;
  return View({ class: "grid min-h-0 flex-1 max-820:grid-cols-1 grid-cols-main-90 items-center max-820:items-start p-responsive max-520:py-8 max-520:px-4 gap-responsive max-820:gap-8 bg-color-bg-surface" }, [
    View({ class: "min-w-0 max-820:text-center" }, [
      Tag(
        { name: "filehelper-login-eyebrow", class: "w-fit-content mb-4 max-820:mr-auto max-820:ml-auto tracking-wide text-transform-uppercase border-default border-solid border-color-primary-border bg-color-primary-soft text-color-primary" },
        ["微信连接"],
      ),
      View({ as: "h1", class: "max-w-520px m-0 max-820:mr-auto max-820:ml-auto font-display font-semibold leading-1.12 max-520:text-3xl text-fluid-display tracking-tight text-color-text-primary" }, [
        "把手机里的内容，直接送到工作台",
      ]),
      View({ class: "max-w-520px mt-5 max-820:mr-auto max-820:ml-auto text-base leading-relaxed text-color-text-secondary" }, [
        "扫码连接文件传输助手后，可在电脑与微信之间同步消息、图片、文件和视频号内容。",
      ]),
      View({ class: "grid max-820:max-w-460px mt-8 max-820:mr-auto max-820:ml-auto gap-3 max-820:text-left" }, [
        View({ class: "flex items-start gap-3" }, [
          View({ class: "grid w-control-height-sm h-control-height-sm flex-none place-items-center rounded-content bg-color-primary-soft text-color-primary" }, [
            Timeless.Icon({ name: "upload", size: 17 }),
          ]),
          View({}, [
            View({ class: "text-md font-semibold text-color-text-primary" }, ["双向传输"]),
            View({ class: "mt-1 text-sm text-color-text-secondary" }, [
              "发送和接收内容，无需额外中转。",
            ]),
          ]),
        ]),
        View({ class: "flex items-start gap-3" }, [
          View({ class: "grid w-control-height-sm h-control-height-sm flex-none place-items-center rounded-content bg-color-primary-soft text-color-primary" }, [
            Timeless.Icon({ name: "file", size: 17 }),
          ]),
          View({}, [
            View({ class: "text-md font-semibold text-color-text-primary" }, ["统一归档"]),
            View({ class: "mt-1 text-sm text-color-text-secondary" }, [
              "文件与媒体资源继续进入下载工作流。",
            ]),
          ]),
        ]),
      ]),
    ]),
    View({ class: "w-full max-820:max-w-380px max-820:my-0 max-820:mx-auto p-6 max-520:p-4 text-center border-default border-solid rounded-card border-color-default bg-color-bg-subtle shadow-md" }, [
      View({ class: "text-lg font-semibold text-color-text-primary" }, ["微信扫码登录"]),
      View({ class: "mt-1 text-sm text-color-text-secondary" }, [
        "使用手机微信扫描下方二维码",
      ]),
      FileHelperQRCodeView({ store: vm$ }),
      View({ class: "m-0 text-base leading-relaxed text-center whitespace-pre-line text-color-text-secondary" }, [vm$.state.login_tip]),
    ]),
  ]);
}

function FileHelperFinderMessageView(props) {
  const data = props.message.finder_data;
  const cover_url = data.cover_url || data.thumb_url || "";
  return View(
    { class: "w-240px max-w-full overflow-hidden rounded-card bg-color-bg-surface shadow-sm" },
    [
      cover_url
        ? LazyImg({
            class: "block w-full h-135px object-cover bg-color-bg-interactive",
            src: cover_url,
            alt: "封面",
          })
        : null,
      View({ class: "p-2.5" }, [
        View({ class: "-webkit-box mb-2 overflow-hidden text-base leading-1.4 -webkit-box-orient-vertical -webkit-line-clamp-2 text-color-text-primary" }, [data.desc || "[视频号]"]),
        View(
          { class: "flex items-center gap-1.5" },
          [
            data.avatar
              ? LazyImg({
                  class: "w-20px h-20px flex-none object-cover rounded-avatar",
                  src: data.avatar,
                  alt: "头像",
                })
              : null,
            View({ class: "min-w-0 overflow-hidden text-sm whitespace-nowrap text-ellipsis text-color-text-secondary" }, [data.nickname]),
            PlatformTag({
              class: "ml-auto bg-color-signal-soft text-color-primary",
              name: "filehelper-finder-platform",
              label: "视频号",
              favicon: (window.PLATFORM_FAVICONS || {}).wxchannels,
            }),
          ].filter(Boolean),
        ),
      ]),
    ].filter(Boolean),
  );
}

function FileHelperMessageView(props) {
  const vm$ = props.store;
  const message = props.message;
  const time = vm$.methods.formatMessageTime(message.CreateTime);
  return View(
    {
      class: message.is_mine
        ? "flex max-w-4/5 align-self-flex-start gap-2 message-row is-self"
        : "flex max-w-4/5 align-self-flex-start gap-2 message-row ",
    },
    [
      View({ class: "flex w-40px h-40px flex-none items-center justify-center text-base rounded-avatar bg-color-primary text-color-on-primary" }, [
        message.is_mine ? "我" : "文",
      ]),
      View(
        { class: "min-w-0" },
        [
          message.type === "finder" && message.finder_data
            ? FileHelperFinderMessageView({ message })
            : View({ class: "py-2.5 px-3 text-ui leading-1.5 overflow-wrap-anywhere whitespace-pre-wrap rounded-content bg-color-bg-surface shadow-sm message-bubble" }, [message.text]),
          time ? View({ class: "mt-1 text-xs text-right text-color-text-secondary message-time" }, [time]) : null,
        ].filter(Boolean),
      ),
    ],
  );
}

function FileHelperEmptyMessagesView() {
  return View({ class: "flex min-h-0 flex-1 flex-col items-center justify-center gap-2 text-base text-color-text-secondary empty-icon-muted" }, [
    Timeless.Icon({ name: "upload", size: 48 }),
    View({}, ["暂无消息"]),
    View({}, ["发送的文件将同步到手机微信"]),
  ]);
}

function FileHelperMessageListView(props) {
  const vm$ = props.store;
  return View(
    {
      class: "flex min-h-0 flex-1 flex-col p-4 gap-3 overflow-y-auto",
      onMounted(event) {
        vm$.methods.setMessageListElement(event);
      },
      onUnmounted() {
        vm$.methods.setMessageListElement(null);
      },
    },
    [
      Show({
        when: computed(vm$.state.messages, (messages) => messages.length > 0),
        ok() {
          return For({
            each: vm$.state.messages,
            render(message_) {
              const message =
                message_ && message_.value !== undefined
                  ? message_.value
                  : message_;
              return FileHelperMessageView({ store: vm$, message });
            },
          });
        },
        else() {
          return FileHelperEmptyMessagesView();
        },
      }),
    ],
  );
}

function FileHelperComposerView(props) {
  const vm$ = props.store;
  return View({ class: "flex flex-none items-end py-2.5 px-4 gap-2.5 border-t border-t-solid border-t-color-default bg-color-bg-surface" }, [
    View({ class: "w-full" }, [
      View({ class: "flex mb-2.5 gap-4" }, [
        Button(
          {
            store: vm$.ui.btn_open_image_picker$,
            attributes: {
              n: "open-image-picker-button",
              title: "发送图片",
              "aria-label": "发送图片",
            },
          },
          [Timeless.Icon({ name: "image", size: 18 })],
        ),
        Button(
          {
            store: vm$.ui.btn_open_file_picker$,
            attributes: {
              n: "open-file-picker-button",
              title: "发送文件",
              "aria-label": "发送文件",
            },
          },
          [Timeless.Icon({ name: "file", size: 18 })],
        ),
      ]),
      View({ class: "flex items-end gap-2.5" }, [
        View({ class: "min-w-0 flex-1" }, [
          Textarea({
            store: vm$.ui.input_message$,
            class: "dm-textarea--compact",
            attributes: { n: "file-helper-message-input", rows: "1" },
          }),
        ]),
        Button(
          {
            store: vm$.ui.btn_send_message$,
            attributes: { n: "file-helper-send-action" },
          },
          [
            computed(vm$.state.sending, (sending) =>
              sending ? "发送中..." : "发送",
            ),
          ],
        ),
      ]),
    ]),
  ]);
}

function FileHelperChatView(props) {
  const vm$ = props.store;
  return View({ class: "flex min-h-0 flex-1 flex-col" }, [
    FileHelperMessageListView({ store: vm$ }),
    FileHelperComposerView({ store: vm$ }),
  ]);
}

function FileHelperHiddenInputsView(props) {
  const vm$ = props.store;
  return [
    Input({
      store: vm$.ui.input_image_file$,
      class: "dm-sr-only",
      rootClass: "dm-sr-only",
      rootAttributes: { n: "file-helper-image-file-field" },
      attributes: {
        n: "file-helper-image-file-input",
        type: "file",
        accept: "image/*",
      },
      onMounted(event) {
        vm$.methods.setImageInputElement(event);
      },
      onUnmounted() {
        vm$.methods.setImageInputElement(null);
      },
      onChange(event) {
        vm$.methods.handleImageSelect(event);
      },
    }),
    Input({
      store: vm$.ui.input_file$,
      class: "dm-sr-only",
      rootClass: "dm-sr-only",
      rootAttributes: { n: "file-helper-file-field" },
      attributes: { n: "file-helper-file-input", type: "file" },
      onMounted(event) {
        vm$.methods.setFileInputElement(event);
      },
      onUnmounted() {
        vm$.methods.setFileInputElement(null);
      },
      onChange(event) {
        vm$.methods.handleFileSelect(event);
      },
    }),
  ];
}

function FileHelperPageView(props) {
  const vm$ = FileHelperViewModel(props);
  return View(
    {
      class: "flex w-full min-w-0 h-full min-h-0 items-center justify-center overflow-hidden max-820:overflow-auto font-sans bg-image-page-grid bg-size-grid bg-color-bg-canvas bg-color-canvas text-color-text-primary",
      onMounted() {
        vm$.methods.ready();
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      View({ class: "flex min-w-0 h-parent-inset max-h-205 min-h-0 max-820:h-auto max-820:min-h-parent-inset max-520:h-full flex-1 flex-col m-4 max-520:m-0 overflow-hidden rounded-card max-520:rounded-0 bg-color-bg-surface shadow-lg dm-panel" }, [
        FileHelperHeaderView({ store: vm$ }),
        Show({
          when: vm$.state.logged_in,
          ok() {
            return FileHelperChatView({ store: vm$ });
          },
          else() {
            return FileHelperLoginView({ store: vm$ });
          },
        }),
        ...FileHelperHiddenInputsView({ store: vm$ }),
      ]),
    ],
  );
}

export default FileHelperPageView;
