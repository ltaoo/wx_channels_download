

const third_party_downloader_notice_classes = {
  __proto__: null,
  "success": "border-color-[color-mix(in_srgb,_var(--dm-color-success)_32%,_transparent)] bg-color-success-soft text-color-success",
  "danger": "border-color-color-danger-border bg-color-danger-soft text-color-danger",
  "progress": "border-color-color-info-border bg-color-info-soft text-color-info",
};

function ThirdPartyDownloaderField(props) {
  return View(
    {
      class: Timeless.classNames([
        "min-w-0 grid content-start gap-1-5",
        props.wide ? "grid-column-[1_/_-1] max-620:grid-column-auto" : "",
      ]),
    },
    [
      View({ as: "label", class: "flex items-center gap-1-5 text-color-text-secondary text-sm font-semibold leading-snug" }, [
        props.label,
        props.optional
          ? View(
              { as: "span", class: "text-color-text-disabled text-caption font-regular" },
              ["选填"],
            )
          : null,
      ].filter(Boolean)),
      props.control,
      props.hint
        ? View({ class: "text-color-text-secondary text-caption leading-base" }, [props.hint])
        : null,
    ].filter(Boolean),
  );
}

function ThirdPartyDownloaderIdentity(props) {
  const option_ = props.store.state.option;
  return View({ class: "min-w-0 flex items-center gap-3" }, [
    View({ class: "w-38px h-38px flex-[0_0_auto] grid place-items-center rounded-content bg-color-bg-inverse text-color-text-inverse" }, [
      Timeless.Icon({ name: "hard-drive", size: 21 }),
    ]),
    View({ class: "min-w-0 grid gap-0-5" }, [
      View({ class: "text-color-text-primary text-base font-bold leading-snug" }, [
        computed(option_, (option) => option.label),
      ]),
      View({ class: "text-color-text-secondary text-xs leading-base" }, [
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
        "min-w-0 flex items-start gap-2-5 p-[var(--dm-space-2-5)_var(--dm-space-3)] border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-content bg-color-bg-subtle text-color-text-secondary",
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
      View({ class: "w-18px h-20px flex-[0_0_auto] grid place-items-center" }, [
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
                      class: "animation-[dm-spin_var(--dm-duration-loop)_linear_infinite] motion-reduce:animation-none",
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
      View({ class: "min-w-0 grid gap-0-5" }, [
        View({ class: "text-currentcolor text-sm font-bold leading-snug" }, [
          computed(notice_, (notice) => notice.title),
        ]),
        View({ class: "overflow-wrap-anywhere text-currentcolor font-mono text-caption leading-base" }, [
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
          class: "min-w-0 grid gap-2 p-3 border-[var(--dm-border-width)_solid_var(--dm-color-info-border)] rounded-content bg-color-info-soft",
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
          View({ class: "min-w-0 flex items-center justify-between gap-3" }, [
            View({ class: "text-color-text-primary text-sm font-bold" }, [
              computed(progress_, (progress) => progress.status_text),
            ]),
            View({ class: "text-color-info font-mono text-sm font-variant-numeric-tabular-nums font-bold" }, [
              computed(progress_, (progress) => progress.percent_text),
            ]),
          ]),
          View({ class: "h-7px overflow-hidden rounded-round bg-[color-mix(in_srgb,_var(--dm-color-info)_14%,_transparent)]" }, [
            View({
              class: "w-0 h-full rounded-inherit bg-color-info transition-[width_var(--dm-duration-base)_var(--dm-ease-standard)]",
              style: computed(progress_, (progress) => ({
                width: `${progress.percent || 0}%`,
              })),
            }),
          ]),
          View({ class: "min-w-0 flex items-center gap-3 justify-start text-color-text-secondary font-mono text-caption [&_>_div]:min-w-0 [&_>_div]:inline-flex [&_>_div]:items-center [&_>_div]:gap-1-5" }, [
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
                  class: "min-w-0 inline-flex items-center gap-1-5 overflow-hidden text-color-text-secondary font-mono text-caption text-ellipsis whitespace-nowrap",
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
                      `min-w-0 inline-flex items-center gap-1-5 text-color-info text-caption font-semibold [&.is-completed]:text-color-success [&.is-failed]:text-color-danger is-${progress.decryption_status || "pending"}`,
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
  return View({ class: "min-w-0 grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-[var(--dm-space-3)_var(--dm-space-4)] pt-3 border-top-[var(--dm-border-width)_solid_var(--dm-color-border)] max-620:grid-cols-[minmax(0,_1fr)]" }, [
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
      class: ["[--third-party-downloader-panel-soft:color-mix(_in_srgb,_var(--dm-color-bg-subtle)_88%,_var(--dm-color-bg-surface)_)] dm-dialog--wide", props.class]
        .filter(Boolean)
        .join(" "),
      closeLabel: "关闭三方下载器面板",
    },
    [
      DialogHeader({ class: "[&.dm-dialog-header]:flex-row [&.dm-dialog-header]:items-start [&.dm-dialog-header]:gap-3 [&.dm-dialog-header]:pr-12 max-620:[&.dm-dialog-header]:p-[var(--dm-space-4)_var(--dm-space-10)_0_var(--dm-space-4)]" }, [
        View({ class: "w-40px h-40px flex-[0_0_auto] grid place-items-center border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-content bg-color-primary-soft text-color-primary max-620:hidden" }, [
          Timeless.Icon({ name: "download", size: 22 }),
        ]),
        View({ class: "min-w-0 grid gap-1-5" }, [
          DialogTitle({}, ["发送到三方下载器"]),
          DialogDescription({}, [
            "连接本机 aria2、Motrix 或 Gopeed，直接创建下载任务。",
          ]),
        ]),
      ]),
      DialogBody({ class: "[&.dm-dialog-body]:grid [&.dm-dialog-body]:gap-4 [&.dm-dialog-body]:pt-4 max-620:[&.dm-dialog-body]:p-4" }, [
        View({ class: "grid gap-4 p-4 border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card bg-[var(--third-party-downloader-panel-soft)] max-620:p-3" }, [
          View({ class: "min-w-0 flex items-center justify-between gap-4 max-620:items-stretch max-620:flex-col" }, [
            ThirdPartyDownloaderIdentity({ store: vm$ }),
            Select({
              store: vm$.ui.select_kind$,
              class: "w-132px flex-[0_0_132px] bg-color-bg-surface max-620:w-full max-620:basis-auto",
              attributes: { "aria-label": "选择三方下载器" },
            }),
          ]),
          View({ class: "min-w-0 grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-[var(--dm-space-3)_var(--dm-space-4)] max-620:grid-cols-[minmax(0,_1fr)]" }, [
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
          View({ class: "flex items-start gap-2 text-color-text-secondary text-caption leading-base [&_>_svg]:flex-[0_0_auto] [&_>_svg]:mt-1px [&_>_svg]:text-color-warning" }, [
            Timeless.Icon({ name: "circle-alert", size: 14 }),
            computed(option_, (option) => option.setup),
          ]),
        ]),
        Show({
          when: computed(vm$.state.notice, (notice) => notice.tone !== "danger"),
          ok() { return ThirdPartyDownloaderNotice({ store: vm$ }); },
        }),
        ThirdPartyDownloaderProgress({ store: vm$ }),
        View({ class: "grid gap-4 p-4 border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card bg-color-bg-surface max-620:p-3" }, [
          View({ class: "grid gap-1" }, [
            View({ class: "text-color-text-primary text-base font-bold leading-snug" }, [
              "下载任务",
            ]),
            View(
              { class: "text-color-text-secondary text-xs leading-base" },
              [computed(option_, (option) => option.protocols)],
            ),
          ]),
          View({ class: "min-w-0 grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-[var(--dm-space-3)_var(--dm-space-4)] max-620:grid-cols-[minmax(0,_1fr)]" }, [
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
                  "transition-[transform_var(--dm-duration-base)_var(--dm-ease-standard)] [&.is-open]:transform-[rotate(180deg)] motion-reduce:transition-none",
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
      DialogFooter({ class: "[&.dm-dialog-footer]:flex-[0_0_auto] [&.dm-dialog-footer]:flex [&.dm-dialog-footer]:items-center [&.dm-dialog-footer]:justify-between [&.dm-dialog-footer]:gap-4 [&.dm-dialog-footer]:p-[var(--dm-space-3)_var(--dm-space-5)] [&.dm-dialog-footer]:bg-color-bg-surface [&.dm-dialog-footer]:border-top-[var(--dm-border-width)_solid_var(--dm-color-border)] [&.dm-dialog-footer]:shadow-[0_calc(-1_*_var(--dm-space-2))_var(--dm-space-4)_color-mix(in_srgb,_var(--dm-color-text-primary)_8%,_transparent)] max-620:[&.dm-dialog-footer]:items-stretch max-620:[&.dm-dialog-footer]:flex-col max-620:[&.dm-dialog-footer]:p-[var(--dm-space-3)_var(--dm-space-4)]" }, [
        View({ class: "min-w-0 inline-flex items-center gap-2 text-color-text-secondary text-xs leading-base" }, [
          Timeless.Icon({ name: "hard-drive", size: 15 }),
          "任务由三方下载器管理，不会加入本页任务列表。",
        ]),
        View({ class: "flex-[0_0_auto] inline-flex items-center gap-2 max-620:justify-end" }, [
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
