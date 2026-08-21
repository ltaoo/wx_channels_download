function ThirdPartyDownloaderField(props) {
  return View(
    {
      class: Timeless.classNames([
        "third-party-downloader-field",
        props.wide ? "third-party-downloader-field--wide" : "",
      ]),
    },
    [
      View({ as: "label", class: "third-party-downloader-field__label" }, [
        props.label,
        props.optional
          ? View(
              { as: "span", class: "third-party-downloader-field__optional" },
              ["选填"],
            )
          : null,
      ].filter(Boolean)),
      props.control,
      props.hint
        ? View({ class: "third-party-downloader-field__hint" }, [props.hint])
        : null,
    ].filter(Boolean),
  );
}

function ThirdPartyDownloaderIdentity(props) {
  const option_ = props.store.state.option;
  return View({ class: "third-party-downloader-identity" }, [
    View({ class: "third-party-downloader-identity__icon" }, [
      Timeless.Icon({ name: "hard-drive", size: 21 }),
    ]),
    View({ class: "third-party-downloader-identity__copy" }, [
      View({ class: "third-party-downloader-identity__name" }, [
        computed(option_, (option) => option.label),
      ]),
      View({ class: "third-party-downloader-identity__description" }, [
        computed(option_, (option) => option.description),
      ]),
    ]),
  ]);
}

function ThirdPartyDownloaderNotice(props) {
  const notice_ = props.store.state.notice;
  return View(
    {
      class: Timeless.classNames([
        "third-party-downloader-notice",
        computed(
          notice_,
          (notice) => `third-party-downloader-notice--${notice.tone || "neutral"}`,
        ),
      ]),
      attributes: {
        role: "status",
        "aria-live": "polite",
      },
    },
    [
      View({ class: "third-party-downloader-notice__icon" }, [
        Show({
          when: computed(notice_, (notice) => notice.tone === "success"),
          ok() {
            return Timeless.Icon({ name: "check", size: 16 });
          },
          else() {
            return Show({
              when: computed(notice_, (notice) => notice.tone === "danger"),
              ok() {
                return Timeless.Icon({ name: "circle-alert", size: 16 });
              },
              else() {
                return Show({
                  when: computed(
                    notice_,
                    (notice) => notice.tone === "progress",
                  ),
                  ok() {
                    return Timeless.Icon({
                      name: "refresh-cw",
                      size: 16,
                      class: "third-party-downloader-spin",
                    });
                  },
                  else() {
                    return Timeless.Icon({ name: "hard-drive", size: 16 });
                  },
                });
              },
            });
          },
        }),
      ]),
      View({ class: "third-party-downloader-notice__copy" }, [
        View({ class: "third-party-downloader-notice__title" }, [
          computed(notice_, (notice) => notice.title),
        ]),
        View({ class: "third-party-downloader-notice__message" }, [
          computed(notice_, (notice) => notice.message),
        ]),
      ]),
    ],
  );
}

function ThirdPartyDownloaderProgress(props) {
  const vm$ = props.store;
  const progress_ = vm$.state.progress;
  return Show({
    when: computed(progress_, (progress) => Boolean(progress.visible)),
    ok() {
      return View(
        {
          class: "third-party-downloader-progress",
          attributes: {
            role: "progressbar",
            "aria-label": "三方下载进度",
            "aria-valuemin": "0",
            "aria-valuemax": "100",
            "aria-valuenow": computed(progress_, (progress) =>
              String(Math.round(progress.percent || 0)),
            ),
          },
        },
        [
          View({ class: "third-party-downloader-progress__head" }, [
            View({ class: "third-party-downloader-progress__status" }, [
              computed(progress_, (progress) => progress.status_text),
            ]),
            View({ class: "third-party-downloader-progress__percent" }, [
              computed(progress_, (progress) => progress.percent_text),
            ]),
          ]),
          View({ class: "third-party-downloader-progress__track" }, [
            View({
              class: "third-party-downloader-progress__bar",
              style: computed(progress_, (progress) => ({
                width: `${progress.percent || 0}%`,
              })),
            }),
          ]),
          View({ class: "third-party-downloader-progress__metrics" }, [
            View({}, [
              Timeless.Icon({ name: "hard-drive", size: 13 }),
              computed(progress_, (progress) => progress.bytes_text),
            ]),
            View({}, [
              Timeless.Icon({ name: "gauge", size: 13 }),
              computed(progress_, (progress) => progress.speed_text),
            ]),
          ]),
          Show({
            when: computed(progress_, (progress) => Boolean(progress.file_path)),
            ok() {
              return View(
                {
                  class: "third-party-downloader-progress__path",
                  attributes: {
                    title: computed(
                      progress_,
                      (progress) => progress.file_path,
                    ),
                  },
                },
                [
                  Timeless.Icon({ name: "file", size: 13 }),
                  computed(progress_, (progress) => progress.file_path),
                ],
              );
            },
          }),
          Show({
            when: computed(progress_, (progress) =>
              Boolean(progress.decryption_text),
            ),
            ok() {
              return View(
                {
                  class: computed(
                    progress_,
                    (progress) =>
                      `third-party-downloader-progress__decrypt is-${progress.decryption_status || "pending"}`,
                  ),
                },
                [
                  Timeless.Icon({ name: "refresh-cw", size: 13 }),
                  computed(progress_, (progress) => progress.decryption_text),
                ],
              );
            },
          }),
        ],
      );
    },
  });
}

function ThirdPartyDownloaderAdvancedFields(props) {
  const vm$ = props.store;
  return View({ class: "third-party-downloader-advanced" }, [
    ThirdPartyDownloaderField({
      label: "Referer",
      optional: true,
      control: Input({
        store: vm$.ui.input_referer$,
        attributes: {
          type: "url",
          value: vm$.state.referer,
          inputmode: "url",
          autocomplete: "off",
          "aria-label": "Referer",
        },
      }),
    }),
    ThirdPartyDownloaderField({
      label: "User-Agent",
      optional: true,
      control: Input({
        store: vm$.ui.input_user_agent$,
        attributes: {
          value: vm$.state.user_agent,
          autocomplete: "off",
          "aria-label": "User-Agent",
        },
      }),
    }),
    ThirdPartyDownloaderField({
      label: "Cookie",
      optional: true,
      wide: true,
      control: Input({
        store: vm$.ui.input_cookie$,
        attributes: {
          value: vm$.state.cookie,
          autocomplete: "off",
          "aria-label": "Cookie",
        },
      }),
      hint: "仅随当前下载任务发送，不会保存到连接配置。",
    }),
  ]);
}

export function ThirdPartyDownloaderPanel(props) {
  const vm$ = props.store;
  const option_ = vm$.state.option;

  return Dialog(
    {
      store: vm$.ui.dialog$,
      class: ["third-party-downloader-panel", props.class]
        .filter(Boolean)
        .join(" "),
      style: {
        width: "min(760px, calc(100vw - 32px))",
        ...(props.style || {}),
      },
      closeLabel: "关闭三方下载器面板",
    },
    [
      DialogHeader({ class: "third-party-downloader-heading" }, [
        View({ class: "third-party-downloader-heading__mark" }, [
          Timeless.Icon({ name: "download", size: 22 }),
        ]),
        View({ class: "third-party-downloader-heading__copy" }, [
          DialogTitle({}, ["发送到三方下载器"]),
          DialogDescription({}, [
            "连接本机 aria2、Motrix 或 Gopeed，直接创建下载任务。",
          ]),
        ]),
      ]),
      DialogBody({ class: "third-party-downloader-body" }, [
        View({ class: "third-party-downloader-connection" }, [
          View({ class: "third-party-downloader-connection__top" }, [
            ThirdPartyDownloaderIdentity({ store: vm$ }),
            Select({
              store: vm$.ui.select_kind$,
              class: "third-party-downloader-kind-select",
              attributes: { "aria-label": "选择三方下载器" },
            }),
          ]),
          View({ class: "third-party-downloader-grid" }, [
            ThirdPartyDownloaderField({
              label: "本机连接地址",
              control: Input({
                store: vm$.ui.input_endpoint$,
                attributes: {
                  type: "url",
                  value: vm$.state.endpoint,
                  inputmode: "url",
                  autocomplete: "off",
                  spellcheck: "false",
                  "aria-label": "本机下载器连接地址",
                },
              }),
              hint: "仅允许连接 localhost、127.0.0.1 或 ::1。",
            }),
            ThirdPartyDownloaderField({
              label: computed(option_, (option) => option.tokenLabel),
              optional: true,
              control: Input({
                store: vm$.ui.input_token$,
                attributes: {
                  type: "password",
                  value: vm$.state.token,
                  autocomplete: "off",
                  spellcheck: "false",
                  "aria-label": "下载器访问密钥",
                },
              }),
              hint: "连接配置只保存在当前浏览器的本机存储中。",
            }),
          ]),
          View({ class: "third-party-downloader-setup-hint" }, [
            Timeless.Icon({ name: "circle-alert", size: 14 }),
            computed(option_, (option) => option.setup),
          ]),
        ]),
        ThirdPartyDownloaderNotice({ store: vm$ }),
        ThirdPartyDownloaderProgress({ store: vm$ }),
        View({ class: "third-party-downloader-task" }, [
          View({ class: "third-party-downloader-section-heading" }, [
            View({ class: "third-party-downloader-section-heading__title" }, [
              "下载任务",
            ]),
            View(
              { class: "third-party-downloader-section-heading__description" },
              [computed(option_, (option) => option.protocols)],
            ),
          ]),
          View({ class: "third-party-downloader-grid" }, [
            ThirdPartyDownloaderField({
              label: "下载地址",
              wide: true,
              control: Input({
                store: vm$.ui.input_url$,
                attributes: {
                  type: "url",
                  value: vm$.state.url,
                  inputmode: "url",
                  autocomplete: "off",
                  spellcheck: "false",
                  "aria-label": "下载地址",
                },
              }),
            }),
            ThirdPartyDownloaderField({
              label: "文件名",
              optional: true,
              control: Input({
                store: vm$.ui.input_filename$,
                attributes: {
                  value: vm$.state.filename,
                  autocomplete: "off",
                  "aria-label": "下载文件名",
                },
              }),
            }),
            ThirdPartyDownloaderField({
              label: "保存目录",
              optional: true,
              control: Input({
                store: vm$.ui.input_directory$,
                attributes: {
                  value: vm$.state.directory,
                  autocomplete: "off",
                  spellcheck: "false",
                  "aria-label": "下载保存目录",
                },
              }),
            }),
          ]),
          Button(
            {
              store: vm$.ui.advanced_button$,
              class: "third-party-downloader-advanced-toggle",
              prefix: Timeless.Icon({
                name: "chevron-down",
                size: 15,
                class: Timeless.classNames([
                  "third-party-downloader-advanced-toggle__icon",
                  computed(vm$.state.advanced_open, (open) =>
                    open ? "is-open" : "",
                  ),
                ]),
              }),
              attributes: {
                "aria-expanded": computed(vm$.state.advanced_open, (open) =>
                  open ? "true" : "false",
                ),
              },
            },
            ["请求头选项"],
          ),
          Show({
            when: vm$.state.advanced_open,
            ok() {
              return ThirdPartyDownloaderAdvancedFields({ store: vm$ });
            },
          }),
        ]),
      ]),
      DialogFooter({ class: "third-party-downloader-footer" }, [
        View({ class: "third-party-downloader-footer__hint" }, [
          Timeless.Icon({ name: "hard-drive", size: 15 }),
          "任务由三方下载器管理，不会加入本页任务列表。",
        ]),
        View({ class: "third-party-downloader-footer__actions" }, [
          Button(
            {
              store: vm$.ui.refresh_button$,
              prefix: Timeless.Icon({ name: "activity", size: 15 }),
            },
            ["刷新进度"],
          ),
          Button(
            {
              store: vm$.ui.probe_button$,
              prefix: Timeless.Icon({ name: "refresh-cw", size: 15 }),
            },
            ["检测连接"],
          ),
          Button(
            {
              store: vm$.ui.submit_button$,
              prefix: Timeless.Icon({ name: "download", size: 15 }),
            },
            ["发送任务"],
          ),
        ]),
      ]),
    ],
  );
}
