import {
  FileHelperViewModel,
  event_target_element,
} from "./filehelper.model.js";

function FileHelperHeaderView(props) {
  const vm$ = props.store;
  return View({ class: "filehelper-header page-header" }, [
    View({ class: "filehelper-header-brand" }, [
      View({ class: "filehelper-header-icon" }, [
        Timeless.Icon({ name: "upload", size: 18 }),
      ]),
      View({}, [
        View({ class: "filehelper-header-title" }, ["文件传输助手"]),
        View({ class: "filehelper-header-subtitle" }, [
          "连接微信，跨设备传送内容",
        ]),
      ]),
    ]),
    View({ class: "filehelper-header-actions" }, [
      Show({
        when: computed(
          vm$.state.channels_status,
          (status) => status !== "idle",
        ),
        ok() {
          return View(
            { class: "filehelper-channels-status dm-badge dm-badge--info" },
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
      View(
        {
          class: computed(vm$.state.logged_in, (logged_in) =>
            logged_in
              ? "filehelper-status is-online"
              : "filehelper-status",
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
      return View({ class: "filehelper-scanned-user" }, [
        Show({
          when: computed(vm$.state.scanned_avatar, (avatar) => Boolean(avatar)),
          ok() {
            return Timeless.Img({
              class: "filehelper-scanned-avatar",
              src: vm$.state.scanned_avatar,
              alt: "用户头像",
            });
          },
        }),
        View({ class: "filehelper-scanned-tip" }, [
          "请在手机上确认登录",
        ]),
      ]);
    },
    else() {
      return View({ class: "filehelper-qrcode" }, [
        Show({
          when: computed(
            vm$.state.login_stage,
            (stage) => stage === "loading",
          ),
          ok() {
            return View({ class: "filehelper-loading-spinner" });
          },
        }),
        Show({
          when: computed(
            vm$.state.qrcode_url,
            (url) => Boolean(url),
          ),
          ok() {
            return Timeless.Img({
              class: "filehelper-qrcode-image",
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
            return View({ class: "filehelper-qrcode-expired" }, [
              View({ class: "filehelper-qrcode-expired-text" }, [
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
  return View({ class: "filehelper-login" }, [
    View({ class: "filehelper-login-copy" }, [
      View({ class: "filehelper-login-eyebrow" }, ["微信连接"]),
      View({ as: "h1", class: "filehelper-login-title" }, [
        "把手机里的内容，直接送到工作台",
      ]),
      View({ class: "filehelper-login-description" }, [
        "扫码连接文件传输助手后，可在电脑与微信之间同步消息、图片、文件和视频号内容。",
      ]),
      View({ class: "filehelper-login-features" }, [
        View({ class: "filehelper-login-feature" }, [
          View({ class: "filehelper-login-feature-icon" }, [
            Timeless.Icon({ name: "upload", size: 17 }),
          ]),
          View({}, [
            View({ class: "filehelper-login-feature-title" }, ["双向传输"]),
            View({ class: "filehelper-login-feature-text" }, [
              "发送和接收内容，无需额外中转。",
            ]),
          ]),
        ]),
        View({ class: "filehelper-login-feature" }, [
          View({ class: "filehelper-login-feature-icon" }, [
            Timeless.Icon({ name: "file", size: 17 }),
          ]),
          View({}, [
            View({ class: "filehelper-login-feature-title" }, ["统一归档"]),
            View({ class: "filehelper-login-feature-text" }, [
              "文件与媒体资源继续进入下载工作流。",
            ]),
          ]),
        ]),
      ]),
    ]),
    View({ class: "filehelper-login-panel" }, [
      View({ class: "filehelper-login-panel-title" }, ["微信扫码登录"]),
      View({ class: "filehelper-login-panel-caption" }, [
        "使用手机微信扫描下方二维码",
      ]),
      FileHelperQRCodeView({ store: vm$ }),
      View({ class: "filehelper-login-tip" }, [vm$.state.login_tip]),
    ]),
  ]);
}

function FileHelperFinderMessageView(props) {
  const data = props.message.finder_data;
  const cover_url = data.cover_url || data.thumb_url || "";
  return View({ class: "filehelper-finder-card" }, [
    cover_url
      ? Timeless.Img({
          class: "filehelper-finder-cover",
          src: cover_url,
          alt: "封面",
          onError(event) {
            const target = event_target_element(event);
            if (target) {
              target.style.display = "none";
            }
          },
        })
      : null,
    View({ class: "filehelper-finder-content" }, [
      View({ class: "filehelper-finder-desc" }, [
        data.desc || "[视频号]",
      ]),
      View({ class: "filehelper-finder-author" }, [
        data.avatar
          ? Timeless.Img({
              class: "filehelper-finder-avatar",
              src: data.avatar,
              alt: "头像",
              onError(event) {
                const target = event_target_element(event);
                if (target) {
                  target.style.display = "none";
                }
              },
            })
          : null,
        View({ class: "filehelper-finder-nickname" }, [data.nickname]),
        View({ class: "filehelper-finder-badge" }, ["视频号"]),
      ].filter(Boolean)),
    ]),
  ].filter(Boolean));
}

function FileHelperMessageView(props) {
  const vm$ = props.store;
  const message = props.message;
  const time = vm$.methods.formatMessageTime(message.CreateTime);
  return View(
    {
      class: message.is_mine
        ? "filehelper-message is-self"
        : "filehelper-message",
    },
    [
      View({ class: "filehelper-message-avatar" }, [
        message.is_mine ? "我" : "文",
      ]),
      View({ class: "filehelper-message-content" }, [
        message.type === "finder" && message.finder_data
          ? FileHelperFinderMessageView({ message })
          : View({ class: "filehelper-message-bubble" }, [message.text]),
        time
          ? View({ class: "filehelper-message-time" }, [time])
          : null,
      ].filter(Boolean)),
    ],
  );
}

function FileHelperEmptyMessagesView() {
  return View({ class: "filehelper-empty" }, [
    Timeless.Icon({ name: "upload", size: 48 }),
    View({}, ["暂无消息"]),
    View({}, ["发送的文件将同步到手机微信"]),
  ]);
}

function FileHelperMessageListView(props) {
  const vm$ = props.store;
  return View(
    {
      class: "filehelper-message-list",
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
  return View({ class: "filehelper-input-area" }, [
    View({ class: "filehelper-composer" }, [
      View({ class: "filehelper-toolbar" }, [
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
      View({ class: "filehelper-compose-row" }, [
        View({ class: "filehelper-input-wrapper" }, [
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
  return View({ class: "filehelper-chat" }, [
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
      class: "filehelper-page page",
      onMounted() {
        vm$.methods.ready();
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      View({ class: "filehelper-container dm-panel" }, [
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
      Show({
        when: vm$.state.toast_visible,
        ok() {
          return View(
            {
              class: "filehelper-toast dm-panel dm-shadow-popover",
              attributes: { role: "status", "aria-live": "polite" },
            },
            [vm$.state.toast_message],
          );
        },
      }),
    ],
  );
}

export default FileHelperPageView;
