

const third_party_downloader_notice_classes = {
  __proto__: null,
  "success": "border-color-success-soft bg-color-success-soft text-color-success",
  "danger": "border-color-color-danger-border bg-color-danger-soft text-color-danger",
  "progress": "border-color-color-info-border bg-color-info-soft text-color-info",
};

function ThirdPartyDownloaderField(props) {
  return View(
    {
      class: Timeless.classNames([
        "grid min-w-0 content-start gap-1-5",
        props.wide ? "col-span-full max-620:grid-column-auto" : "",
      ]),
    },
    [
      View({ as: "label", class: "flex items-center gap-1-5 text-sm font-semibold leading-snug text-color-text-secondary" }, [
        props.label,
        props.optional
          ? View(
              { as: "span", class: "text-caption font-regular text-color-text-disabled" },
              ["选填"],
            )
          : null,
      ].filter(Boolean)),
      props.control,
      props.hint
        ? View({ class: "text-caption leading-base text-color-text-secondary" }, [props.hint])
        : null,
    ].filter(Boolean),
  );
}

function ThirdPartyDownloaderIdentity(props) {
  const option_ = props.store.state.option;
  return View({ class: "flex min-w-0 items-center gap-3" }, [
    View({ class: "grid w-38px h-38px flex-none place-items-center rounded-content bg-color-bg-inverse text-color-text-inverse" }, [
      Timeless.Icon({ name: "hard-drive", size: 21 }),
    ]),
    View({ class: "grid min-w-0 gap-0-5" }, [
      View({ class: "text-base font-bold leading-snug text-color-text-primary" }, [
        computed(option_, (option) => option.label),
      ]),
      View({ class: "text-xs leading-base text-color-text-secondary" }, [
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
        "flex min-w-0 items-start py-2.5 px-3 gap-2-5 border-default border-solid border-color-default rounded-content bg-color-bg-subtle text-color-text-secondary",
        computed(
          notice_,
          (notice) => `${third_party_downloader_notice_classes[notice.tone || "neutral"] || ""}`,
        ),
      ]),
      attributes: {
        role: "status",
        "aria-live": "polite",
      },
    },
    [
      View({ class: "grid w-18px h-20px flex-none place-items-center" }, [
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
                      class: "animate-spin motion-reduce:animation-none",
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
      View({ class: "grid min-w-0 gap-0-5" }, [
        View({ class: "text-sm font-bold leading-snug text-currentcolor" }, [
          computed(notice_, (notice) => notice.title),
        ]),
        View({ class: "font-mono text-caption leading-base overflow-wrap-anywhere text-currentcolor" }, [
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
          class: "grid min-w-0 p-3 gap-2 border-default border-solid border-color-info rounded-content bg-color-info-soft",
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
          View({ class: "flex min-w-0 items-center justify-between gap-3" }, [
            View({ class: "text-sm font-bold text-color-text-primary" }, [
              computed(progress_, (progress) => progress.status_text),
            ]),
            View({ class: "font-mono text-sm font-variant-numeric-tabular-nums font-bold text-color-info" }, [
              computed(progress_, (progress) => progress.percent_text),
            ]),
          ]),
          View({ class: "h-7px overflow-hidden rounded-round bg-info/14" }, [
            View({
              class: "w-0 h-full rounded-inherit bg-color-info transition-width",
              style: computed(progress_, (progress) => ({
                width: `${progress.percent || 0}%`,
              })),
            }),
          ]),
          View({ class: "flex min-w-0 items-center justify-start gap-3 font-mono text-caption text-color-text-secondary progress-meta " }, [
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
                  class: "inline-flex min-w-0 items-center gap-1-5 overflow-hidden font-mono text-caption whitespace-nowrap text-ellipsis text-color-text-secondary",
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
                      `inline-flex min-w-0 items-center gap-1-5 text-caption font-semibold text-color-info progress-status-label is-${progress.decryption_status || "pending"}`,
                  ),
                },
                [
                  Timeless.Icon({ name: "refresh-cw", size: 13 }),
                  computed(progress_, (progress) => progress.decryption_status === "failed"
                    ? "微信解密失败" : progress.decryption_text),
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
  return View({ class: "grid min-w-0 grid-cols-2 max-620:grid-cols-1 pt-3 gap-y-3 gap-x-4 border-t-default border-t-solid border-t-color-default" }, [
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
      class: ["dm-dialog--wide ", props.class]
        .filter(Boolean)
        .join(" "),
      closeLabel: "关闭三方下载器面板",
    },
    [
      DialogHeader({ class: "dialog-header-leading-icon " }, [
        View({ class: "grid max-620:hidden w-40px h-40px flex-none place-items-center border-default border-solid border-color-primary-border rounded-content bg-color-primary-soft text-color-primary" }, [
          Timeless.Icon({ name: "download", size: 22 }),
        ]),
        View({ class: "grid min-w-0 gap-1-5" }, [
          DialogTitle({}, ["发送到三方下载器"]),
          DialogDescription({}, [
            "连接本机 aria2、Motrix 或 Gopeed，直接创建下载任务。",
          ]),
        ]),
      ]),
      DialogBody({ class: "dialog-body-grid " }, [
        View({ class: "grid p-4 max-620:p-3 gap-4 border-default border-solid border-color-default rounded-card bg-surface-tinted" }, [
          View({ class: "flex min-w-0 max-620:flex-col items-center justify-between max-620:items-stretch gap-4" }, [
            ThirdPartyDownloaderIdentity({ store: vm$ }),
            Select({
              store: vm$.ui.select_kind$,
              class: "w-132px max-620:w-full grow-0 shrink-0 basis-33 max-620:basis-auto bg-color-bg-surface",
              attributes: { "aria-label": "选择三方下载器" },
            }),
          ]),
          View({ class: "grid min-w-0 grid-cols-2 max-620:grid-cols-1 gap-y-3 gap-x-4" }, [
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
          View({ class: "flex items-start gap-2 text-caption leading-base text-color-text-secondary notice-icon " }, [
            Timeless.Icon({ name: "circle-alert", size: 14 }),
            computed(option_, (option) => option.setup),
          ]),
        ]),
        Show({
          when: computed(vm$.state.notice, (notice) => notice.tone !== "danger"),
          ok() { return ThirdPartyDownloaderNotice({ store: vm$ }); },
        }),
        ThirdPartyDownloaderProgress({ store: vm$ }),
        View({ class: "grid p-4 max-620:p-3 gap-4 border-default border-solid border-color-default rounded-card bg-color-bg-surface" }, [
          View({ class: "grid gap-1" }, [
            View({ class: "text-base font-bold leading-snug text-color-text-primary" }, [
              "下载任务",
            ]),
            View(
              { class: "text-xs leading-base text-color-text-secondary" },
              [computed(option_, (option) => option.protocols)],
            ),
          ]),
          View({ class: "grid min-w-0 grid-cols-2 max-620:grid-cols-1 gap-y-3 gap-x-4" }, [
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
              class: "w-max-content justify-self-start px-0 text-color-text-secondary",
              prefix: Timeless.Icon({
                name: "chevron-down",
                size: 15,
                class: Timeless.classNames([
                  "transition-transform motion-reduce:transition-none icon-disclosure",
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
      DialogFooter({ class: "dialog-footer-actions " }, [
        View({ class: "inline-flex min-w-0 items-center gap-2 text-xs leading-base text-color-text-secondary" }, [
          Timeless.Icon({ name: "hard-drive", size: 15 }),
          "任务由三方下载器管理，不会加入本页任务列表。",
        ]),
        View({ class: "inline-flex flex-none items-center max-620:justify-end gap-2" }, [
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
