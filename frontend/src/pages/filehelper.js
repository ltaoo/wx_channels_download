import { FileHelperViewModel } from "./filehelper.model.js";

function FileHelperHeaderView(props) {
  const vm$ = props.store;
  return View({ class: "grid flex-[0_0_auto] grid-cols-[minmax(0,_1fr)_auto] items-center gap-4 min-h-72px p-[var(--dm-space-3)_var(--dm-space-5)] bg-filehelper-white border-bottom-[1px_solid_var(--dm-filehelper-border)] max-520:px-3 text-color-text-primary shadow-divider -webkit-backdrop-filter-[blur(14px)] backdrop-filter-[blur(14px)]" }, [
    View({ class: "min-w-0 flex items-center gap-3" }, [
      View({ class: "w-control-height-base h-control-height-base grid flex-[0_0_auto] place-items-center text-color-on-primary bg-color-primary-fill rounded-content shadow-[0_var(--dm-space-2)_var(--dm-space-5)_var(--dm-color-signal-glow)]" }, [
        Timeless.Icon({ name: "upload", size: 18 }),
      ]),
      View({}, [
        View({ class: "text-filehelper-text text-base font-semibold whitespace-nowrap" }, ["文件传输助手"]),
        View({ class: "mt-0-5 text-filehelper-muted text-xs max-520:hidden" }, [
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
              class: "text-filehelper-muted whitespace-nowrap [&_.is-available]:text-filehelper-green [&_.is-unavailable]:text-color-danger [&_.is-error]:text-color-danger max-520:hidden",
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
            logged_in ? "text-filehelper-muted whitespace-nowrap [&.is-online]:text-filehelper-green is-online" : "text-filehelper-muted whitespace-nowrap [&.is-online]:text-filehelper-green",
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
      return View({ class: "flex min-h-200px flex-col items-center justify-center gap-3 mb-5" }, [
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
        View({ class: "text-filehelper-text text-base" }, ["请在手机上确认登录"]),
      ]);
    },
    else() {
      return View({ class: "relative flex w-220px h-220px flex-[0_0_auto] items-center justify-center m-[var(--dm-space-5)_auto_var(--dm-space-4)] overflow-hidden bg-filehelper-white border-[1px_solid_var(--dm-filehelper-border)] rounded-content shadow-sm max-520:w-200px max-520:h-200px" }, [
        Show({
          when: computed(vm$.state.login_stage, (stage) => stage === "loading"),
          ok() {
            return View({ class: "w-40px h-40px border-[3px_solid_var(--dm-filehelper-border)] border-top-color-filehelper-green rounded-round animation-[dm-filehelper-spin_1s_linear_infinite]" });
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
            return View({ class: "absolute inset-0 flex flex-col items-center justify-center gap-2-5 bg-[color-mix(in_srgb,_var(--dm-filehelper-white)_90%,_transparent)]" }, [
              View({ class: "text-filehelper-muted text-base" }, [
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
  return View({ class: "grid grid-cols-[minmax(0,_1fr)_360px] min-h-0 flex-1 items-center gap-[clamp(var(--dm-space-8),_7vw,_var(--dm-space-16))] p-[clamp(var(--dm-space-8),_7vw,_var(--dm-space-16))] bg-filehelper-white max-820:grid-cols-[minmax(0,_1fr)] max-820:items-start max-820:gap-8 max-520:p-[var(--dm-space-8)_var(--dm-space-4)]" }, [
    View({ class: "min-w-0 max-820:text-center" }, [
      Tag(
        { name: "filehelper-login-eyebrow", class: "w-fit-content mb-4 text-color-primary tracking-wide text-transform-uppercase bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] max-820:mr-auto max-820:ml-auto" },
        ["微信连接"],
      ),
      View({ as: "h1", class: "max-w-520px m-0 text-filehelper-text font-display text-[clamp(var(--dm-font-size-3xl),_3vw,_var(--dm-font-size-6xl))] font-semibold leading-[1.12] tracking-tight max-820:mr-auto max-820:ml-auto max-520:text-3xl" }, [
        "把手机里的内容，直接送到工作台",
      ]),
      View({ class: "max-w-520px mt-5 text-filehelper-muted text-base leading-relaxed max-820:mr-auto max-820:ml-auto" }, [
        "扫码连接文件传输助手后，可在电脑与微信之间同步消息、图片、文件和视频号内容。",
      ]),
      View({ class: "grid gap-3 mt-8 max-820:max-w-460px max-820:mr-auto max-820:ml-auto max-820:text-left" }, [
        View({ class: "flex items-start gap-3" }, [
          View({ class: "w-control-height-sm h-control-height-sm grid flex-[0_0_auto] place-items-center text-color-primary bg-color-primary-soft rounded-content" }, [
            Timeless.Icon({ name: "upload", size: 17 }),
          ]),
          View({}, [
            View({ class: "text-filehelper-text text-md font-semibold" }, ["双向传输"]),
            View({ class: "mt-1 text-filehelper-muted text-sm" }, [
              "发送和接收内容，无需额外中转。",
            ]),
          ]),
        ]),
        View({ class: "flex items-start gap-3" }, [
          View({ class: "w-control-height-sm h-control-height-sm grid flex-[0_0_auto] place-items-center text-color-primary bg-color-primary-soft rounded-content" }, [
            Timeless.Icon({ name: "file", size: 17 }),
          ]),
          View({}, [
            View({ class: "text-filehelper-text text-md font-semibold" }, ["统一归档"]),
            View({ class: "mt-1 text-filehelper-muted text-sm" }, [
              "文件与媒体资源继续进入下载工作流。",
            ]),
          ]),
        ]),
      ]),
    ]),
    View({ class: "w-full p-6 text-center bg-filehelper-bg-soft border-[var(--dm-border-width)_solid_var(--dm-filehelper-border)] rounded-card shadow-md max-820:max-w-380px max-820:m-[0_auto] max-520:p-4" }, [
      View({ class: "text-filehelper-text text-lg font-semibold" }, ["微信扫码登录"]),
      View({ class: "mt-1 text-filehelper-muted text-sm" }, [
        "使用手机微信扫描下方二维码",
      ]),
      FileHelperQRCodeView({ store: vm$ }),
      View({ class: "text-filehelper-muted text-base m-0 leading-relaxed text-center whitespace-pre-line" }, [vm$.state.login_tip]),
    ]),
  ]);
}

function FileHelperFinderMessageView(props) {
  const data = props.message.finder_data;
  const cover_url = data.cover_url || data.thumb_url || "";
  return View(
    { class: "w-240px max-w-full overflow-hidden bg-filehelper-white rounded-card shadow-sm" },
    [
      cover_url
        ? LazyImg({
            class: "block w-full h-135px object-cover bg-color-bg-interactive",
            src: cover_url,
            alt: "封面",
          })
        : null,
      View({ class: "p-2-5" }, [
        View({ class: "-webkit-box mb-2 overflow-hidden text-filehelper-text text-base leading-[1.4] -webkit-box-orient-vertical -webkit-line-clamp-2" }, [data.desc || "[视频号]"]),
        View(
          { class: "flex items-center gap-1-5" },
          [
            data.avatar
              ? LazyImg({
                  class: "w-20px h-20px flex-[0_0_auto] object-cover rounded-avatar",
                  src: data.avatar,
                  alt: "头像",
                })
              : null,
            View({ class: "min-w-0 overflow-hidden text-filehelper-muted text-sm text-ellipsis whitespace-nowrap" }, [data.nickname]),
            PlatformTag({
              class: "ml-auto text-filehelper-green bg-color-signal-soft",
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
        ? "flex max-w-[80%] align-self-flex-start gap-2 [&.is-self]:flex-row-reverse [&.is-self]:align-self-flex-end [&.is-self_.group-filehelper-message-bubble]:bg-filehelper-bubble-self [&.is-self_.group-filehelper-message-time]:text-left is-self"
        : "flex max-w-[80%] align-self-flex-start gap-2 [&.is-self]:flex-row-reverse [&.is-self]:align-self-flex-end [&.is-self_.group-filehelper-message-bubble]:bg-filehelper-bubble-self [&.is-self_.group-filehelper-message-time]:text-left",
    },
    [
      View({ class: "flex w-40px h-40px flex-[0_0_auto] items-center justify-center text-color-on-primary text-base bg-filehelper-green rounded-avatar" }, [
        message.is_mine ? "我" : "文",
      ]),
      View(
        { class: "min-w-0" },
        [
          message.type === "finder" && message.finder_data
            ? FileHelperFinderMessageView({ message })
            : View({ class: "group-filehelper-message-bubble p-[var(--dm-space-2-5)_var(--dm-space-3)] text-ui leading-[1.5] overflow-wrap-anywhere whitespace-pre-wrap bg-filehelper-white rounded-content shadow-sm" }, [message.text]),
          time ? View({ class: "group-filehelper-message-time mt-1 text-filehelper-muted text-xs text-right" }, [time]) : null,
        ].filter(Boolean),
      ),
    ],
  );
}

function FileHelperEmptyMessagesView() {
  return View({ class: "flex min-h-0 flex-1 flex-col items-center justify-center gap-2 text-filehelper-muted text-base [&_svg]:opacity-[0.5]" }, [
    Timeless.Icon({ name: "upload", size: 48 }),
    View({}, ["暂无消息"]),
    View({}, ["发送的文件将同步到手机微信"]),
  ]);
}

function FileHelperMessageListView(props) {
  const vm$ = props.store;
  return View(
    {
      class: "flex min-h-0 flex-1 flex-col gap-3 p-4 overflow-y-auto",
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
  return View({ class: "flex flex-[0_0_auto] items-end gap-2-5 p-[var(--dm-space-2-5)_var(--dm-space-4)] bg-filehelper-white border-top-[1px_solid_var(--dm-filehelper-border)]" }, [
    View({ class: "w-full" }, [
      View({ class: "flex gap-4 mb-2-5" }, [
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
      View({ class: "flex items-end gap-2-5" }, [
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
      class: "[--dm-filehelper-green:var(--dm-color-primary)] [--dm-filehelper-green-dark:var(--dm-color-primary-hover)] [--dm-filehelper-bg:var(--dm-color-bg-canvas)] [--dm-filehelper-bg-soft:var(--dm-color-bg-subtle)] [--dm-filehelper-white:var(--dm-color-bg-surface)] [--dm-filehelper-text:var(--dm-color-text-primary)] [--dm-filehelper-muted:var(--dm-color-text-secondary)] [--dm-filehelper-border:var(--dm-color-border)] [--dm-filehelper-bubble-self:var(--dm-color-signal-soft)] flex w-full min-w-0 h-full min-h-0 items-center justify-center overflow-hidden text-filehelper-text font-sans bg-filehelper-bg bg-image-[radial-gradient(_circle,_var(--dm-color-border-translucent)_var(--dm-border-width),_transparent_var(--dm-border-width)_)] background-size-[var(--dm-space-6)_var(--dm-space-6)] max-820:overflow-auto min-w-0 font-sans bg-color-bg-canvas",
      onMounted() {
        vm$.methods.ready();
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      View({ class: "flex flex-1 min-w-0 h-[min(calc(100%_-_var(--dm-space-8)),_820px)] min-h-0 m-4 flex-col overflow-hidden bg-filehelper-white rounded-card shadow-lg max-820:h-auto max-820:min-h-[calc(100%_-_var(--dm-space-8))] max-520:h-full max-520:m-0 max-520:rounded-0 dm-panel" }, [
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
