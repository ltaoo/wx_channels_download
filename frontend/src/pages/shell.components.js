import { ErrorMessage } from "@/dmui.js";
import { format_time } from "@/utils.js";

function certificate_source_label(source) {
  var labels = {
    sunny_net: "内置 SunnyNet",
    mitmproxy: "mitmproxy",
    configured: "自定义证书文件",
    generated: "本机生成",
  };
  return labels[source] || source || "未知来源";
}

function certificate_date_label(value) {
  return format_time(value, value ? String(value) : "未提供", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function certificate_list_label(value) {
  return Array.isArray(value) && value.length > 0 ? value.join("、") : "未提供";
}

function certificate_status(data) {
  var detail = (data && data.detail) || {};
  if (detail.expired) {
    return { label: "证书已过期", tone: "danger", icon: "circle-alert" };
  }
  if (data && data.trusted) {
    return { label: "已安装并受信任", tone: "success", icon: "check" };
  }
  if (data && data.installed) {
    return {
      label: "已安装，尚未受信任",
      tone: "warning",
      icon: "circle-alert",
    };
  }
  return { label: "尚未安装", tone: "danger", icon: "circle-x" };
}

function CertificateDetailItem(props) {
  return View({ class: "min-w-0" }, [
    View({ class: "mb-1 text-color-text-secondary text-caption leading-snug" }, [props.label]),
    View(
      {
        class: Timeless.classNames([
          "min-w-0 overflow-wrap-anywhere text-color-text-primary text-sm font-medium leading-base",
          props.mono ? "font-mono text-xs font-regular" : null,
        ]),
      },
      [props.value],
    ),
  ]);
}

function CertificateSettingsDetails(props) {
  var certificate_ = props.certificate;
  var detail_ = Timeless.computed(certificate_, function (data) {
    return (data && data.detail) || {};
  });
  var configured_ = Timeless.computed(certificate_, function (data) {
    return (data && data.configured) || {};
  });
  var status_ = Timeless.computed(certificate_, certificate_status);

  return View({ class: "flex min-w-0 flex-col gap-4" }, [
    View({ class: "relative grid min-w-0 items-center grid-cols-[auto_minmax(0,_1fr)_auto] gap-4 p-4-5 overflow-hidden bg-[linear-gradient(_135deg,_var(--dm-color-bg-surface),_color-mix(_in_srgb,_var(--dm-color-primary-soft)_54%,_var(--dm-color-bg-surface)_)_)] border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-card [&::before]:content-[\"\"] [&::before]:absolute [&::before]:inset-[0_auto_0_0] [&::before]:w-1 [&::before]:bg-color-primary max-520:grid-cols-[auto_minmax(0,_1fr)]" }, [
      View({ class: "grid w-50px h-50px place-items-center text-color-primary bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border-strong)] rounded-content" }, [
        Timeless.Icon({ name: "file-lock", size: 25 }),
      ]),
      View({ class: "min-w-0" }, [
        View({ class: "text-color-text-secondary text-caption font-semibold tracking-wide" }, ["当前实际使用"]),
        View({ class: "mt-1 overflow-hidden text-color-text-primary font-display text-lg font-bold leading-tight text-ellipsis whitespace-nowrap" }, [
          Timeless.computed(certificate_, function (data) {
            return (data && data.name) || "未命名证书";
          }),
        ]),
        View({ class: "mt-1 text-color-text-secondary text-sm" }, [
          Timeless.computed(certificate_, function (data) {
            return certificate_source_label(data && data.source);
          }),
        ]),
      ]),
      View(
        {
          class: "flex items-center gap-2 max-520:grid-column-[1_/_-1] max-520:justify-self-start max-520:ml-[calc(50px_+_var(--dm-space-4))]",
          attributes: { n: "settings-certificate-actions" },
        },
        [
          Tag(
            {
              class: Timeless.classNames([
                "border-[var(--dm-border-width)_solid_currentColor]",
                Timeless.computed(status_, function (status) {
                  return "settings-certificate-status--" + status.tone;
                }),
              ]),
              attributes: { n: "settings-certificate-status" },
            },
            [
              Show({
                when: Timeless.computed(status_, function (status) {
                  return status.icon === "check";
                }),
                ok() {
                  return Timeless.Icon({
                    name: "check",
                    size: 14,
                    attributes: {
                      n: "settings-certificate-status-trusted-icon",
                    },
                  });
                },
                else() {
                  return Show({
                    when: Timeless.computed(status_, function (status) {
                      return status.icon === "circle-x";
                    }),
                    ok() {
                      return Timeless.Icon({
                        name: "circle-x",
                        size: 14,
                        attributes: {
                          n: "settings-certificate-status-uninstalled-icon",
                        },
                      });
                    },
                    else() {
                      return Timeless.Icon({
                        name: "circle-alert",
                        size: 14,
                        attributes: {
                          n: "settings-certificate-status-warning-icon",
                        },
                      });
                    },
                  });
                },
              }),
              Timeless.computed(status_, function (status) {
                return status.label;
              }),
            ],
          ),
          Button(
            {
              store: props.delete_button,
              prefix: Timeless.Icon({
                name: "trash2",
                size: 14,
                attributes: { n: "settings-certificate-delete-icon" },
              }),
              attributes: {
                n: "settings-certificate-delete-button",
                "aria-label": "删除当前证书",
              },
            },
            ["删除"],
          ),
        ],
      ),
    ]),

    Show({
      when: Timeless.computed(certificate_, function (data) {
        return (
          Array.isArray(data && data.risk_warnings) &&
          data.risk_warnings.length > 0
        );
      }),
      ok() {
        return View({ class: "flex flex-col gap-2" }, [
          For({
            each: Timeless.computed(certificate_, function (data) {
              return data.risk_warnings;
            }),
            render(warning) {
              return View(
                {
                  class:
                    "flex items-start gap-2 p-3 text-sm leading-base border-[var(--dm-border-width)_solid_transparent] rounded-content [&_>_svg]:flex-[0_0_auto] [&_>_svg]:mt-[0.1em] text-color-warning bg-color-warning-soft border-color-[color-mix(in_srgb,_var(--dm-color-warning)_32%,_transparent)]",
                },
                [Timeless.Icon({ name: "circle-alert", size: 16 }), warning],
              );
            },
          }),
        ]);
      },
    }),
    View({ class: "p-4 bg-color-bg-surface border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card" }, [
      View({ class: "mb-3 text-color-text-primary text-sm font-semibold" }, ["证书身份"]),
      View({ class: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-[var(--dm-space-4)_var(--dm-space-5)] max-520:grid-cols-[minmax(0,_1fr)]" }, [
        CertificateDetailItem({
          label: "使用名称",
          value: Timeless.computed(certificate_, function (data) {
            return (data && data.name) || "未提供";
          }),
        }),
        CertificateDetailItem({
          label: "主题名称（CN）",
          value: Timeless.computed(detail_, function (detail) {
            return detail.subject_common_name || "未提供";
          }),
        }),
        CertificateDetailItem({
          label: "颁发者",
          value: Timeless.computed(detail_, function (detail) {
            return detail.issuer_common_name || "未提供";
          }),
        }),
        CertificateDetailItem({
          label: "组织",
          value: Timeless.computed(detail_, function (detail) {
            return certificate_list_label(detail.organizations);
          }),
        }),
        CertificateDetailItem({
          label: "序列号",
          mono: true,
          value: Timeless.computed(detail_, function (detail) {
            return detail.serial_number || "未提供";
          }),
        }),
        CertificateDetailItem({
          label: "证书类型",
          value: Timeless.computed(detail_, function (detail) {
            return detail.is_ca ? "根证书颁发机构（CA）" : "普通证书";
          }),
        }),
      ]),
    ]),
    View({ class: "p-4 bg-color-bg-surface border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card" }, [
      View({ class: "mb-3 text-color-text-primary text-sm font-semibold" }, ["有效期与信任"]),
      View({ class: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-[var(--dm-space-4)_var(--dm-space-5)] max-520:grid-cols-[minmax(0,_1fr)]" }, [
        CertificateDetailItem({
          label: "生效时间",
          value: Timeless.computed(detail_, function (detail) {
            return certificate_date_label(detail.not_before);
          }),
        }),
        CertificateDetailItem({
          label: "到期时间",
          value: Timeless.computed(detail_, function (detail) {
            return certificate_date_label(detail.not_after);
          }),
        }),
        CertificateDetailItem({
          label: "系统证书库",
          value: Timeless.computed(certificate_, function (data) {
            return data && data.installed ? "已安装" : "未安装";
          }),
        }),
        CertificateDetailItem({
          label: "系统信任",
          value: Timeless.computed(certificate_, function (data) {
            return data && data.trusted ? "受信任" : "未受信任";
          }),
        }),
        CertificateDetailItem({
          label: "DNS 名称",
          value: Timeless.computed(detail_, function (detail) {
            return certificate_list_label(detail.dns_names);
          }),
        }),
        CertificateDetailItem({
          label: "证书来源",
          value: Timeless.computed(certificate_, function (data) {
            return certificate_source_label(data && data.source);
          }),
        }),
      ]),
    ]),
    View({ class: "relative p-[var(--dm-space-4)_var(--dm-space-4)_var(--dm-space-4)_var(--dm-space-5)] overflow-hidden text-color-media-text bg-color-media-canvas border-[var(--dm-border-width)_solid_var(--dm-color-media-border)] rounded-content [&::before]:content-[\"\"] [&::before]:absolute [&::before]:inset-[0_auto_0_0] [&::before]:w-3px [&::before]:bg-color-primary" }, [
      View({ class: "mb-2 text-color-media-muted text-caption font-semibold tracking-wide" }, [
        "SHA-256 指纹",
      ]),
      View({ as: "code", class: "block text-inherit font-mono text-xs leading-relaxed overflow-wrap-anywhere" }, [
        Timeless.computed(detail_, function (detail) {
          return detail.fingerprint_sha256 || "未提供";
        }),
      ]),
    ]),
    View({ class: "p-4 bg-color-bg-surface border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card" }, [
      View({ class: "mb-3 text-color-text-primary text-sm font-semibold" }, ["配置来源"]),
      View({ class: "grid gap-3" }, [
        CertificateDetailItem({
          label: "配置名称",
          value: Timeless.computed(configured_, function (configured) {
            return configured.name || "未配置";
          }),
        }),
        CertificateDetailItem({
          label: "证书文件",
          mono: true,
          value: Timeless.computed(configured_, function (configured) {
            return configured.file || "使用自动检测或内置证书";
          }),
        }),
        CertificateDetailItem({
          label: "私钥文件",
          mono: true,
          value: Timeless.computed(configured_, function (configured) {
            return configured.key || "使用自动检测或内置私钥";
          }),
        }),
      ]),
    ]),

    Show({
      when: Timeless.computed(certificate_, function (data) {
        return Boolean(data && data.pem);
      }),
      ok() {
        return View({ as: "details", class: "overflow-hidden bg-color-bg-subtle border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card" }, [
          View({ as: "summary", class: "p-[var(--dm-space-3)_var(--dm-space-4)] text-color-text-secondary text-sm font-medium cursor-pointer user-select-none hover:text-color-text-primary hover:bg-color-bg-interactive" }, [
            "查看 PEM 原文",
          ]),
          View({ as: "pre", class: "max-h-260px m-0 p-4 overflow-auto text-color-media-text font-mono text-xs leading-relaxed whitespace-pre-wrap overflow-wrap-anywhere bg-color-media-canvas border-top-[var(--dm-border-width)_solid_var(--dm-color-media-border)]" }, [
            Timeless.computed(certificate_, function (data) {
              return data.pem;
            }),
          ]),
        ]);
      },
    }),
  ]);
}

function AboutSettingsDetails(props) {
  return View({ class: "flex min-w-0 flex-col gap-4" }, [
    View({ class: "grid min-w-0 items-center grid-cols-[auto_minmax(0,_1fr)_auto] gap-4 p-5 bg-[linear-gradient(_120deg,_color-mix(in_srgb,_var(--dm-color-primary-soft)_72%,_transparent),_transparent_62%_),_var(--dm-color-bg-surface)] border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-card max-520:grid-cols-[auto_minmax(0,_1fr)] max-520:p-4" }, [
      View({ class: "grid w-72px h-72px place-items-center overflow-hidden bg-brand-sidebar border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-content shadow-sm" }, [
        Img({
          class: "block w-56px h-56px object-contain user-select-none",
          src: "public/logo.png?v=logo-only-v4",
          alt: "D&M",
          attributes: { draggable: "false" },
        }),
      ]),
      View({ class: "min-w-0" }, [
        View({ class: "text-color-primary text-caption font-semibold tracking-wide" }, ["微信视频号下载工具"]),
        View({ class: "mt-1 overflow-hidden text-color-text-primary font-display text-xl font-bold leading-tight text-ellipsis whitespace-nowrap" }, ["wx_channels_download"]),
        View({ class: "mt-1-5 text-color-text-secondary text-sm leading-base" }, [
          "在本机抓取、下载和管理内容。",
        ]),
      ]),
      Tag(
        { name: "settings-about__version", class: "text-color-text-primary bg-[color-mix(in_srgb,_var(--dm-color-bg-surface)_88%,_transparent)] border-[var(--dm-border-width)_solid_var(--dm-color-border-strong)] max-520:grid-column-[1_/_-1] max-520:justify-self-start" },
        [
          View({ as: "span", class: "text-color-text-secondary text-caption font-medium" }, [
            "版本",
          ]),
          View({ as: "code", class: "text-inherit font-mono text-sm font-semibold" }, [
            props.version,
          ]),
        ],
      ),
    ]),
    View({ class: "grid gap-3" }, [
      Link(
        {
          class: "grid min-w-0 items-center grid-cols-[auto_minmax(0,_1fr)_auto] gap-3 p-4 text-color-text-primary text-decoration-none bg-color-bg-surface border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card transition-[background-color_var(--dm-duration-fast)_var(--dm-ease-standard),_border-color_var(--dm-duration-fast)_var(--dm-ease-standard),_transform_var(--dm-duration-fast)_var(--dm-ease-standard)] hover:bg-color-bg-interactive hover:border-color-color-primary-border-strong hover:transform-[translateY(-1px)] [&_>_svg]:text-color-text-secondary motion-reduce:transition-none motion-reduce:hover:transform-none dm-focus-ring",
          href: "https://github.com/ltaoo/wx_channels_download",
          target: "_blank",
          attributes: {
            rel: "noopener noreferrer",
            "aria-label": "打开 GitHub 仓库（新窗口）",
          },
        },
        [
          View({ class: "grid w-control-height-base h-control-height-base place-items-center text-color-primary bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-content" }, [
            Timeless.Icon({ name: "git-fork", size: 20 }),
          ]),
          View({ class: "min-w-0" }, [
            View({ class: "text-sm font-semibold leading-snug" }, ["GitHub 仓库"]),
            View({ class: "mt-1 overflow-hidden text-color-text-secondary font-mono text-xs leading-base text-ellipsis whitespace-nowrap" }, [
              "github.com/ltaoo/wx_channels_download",
            ]),
          ]),
          Timeless.Icon({ name: "external-link", size: 16 }),
        ],
      ),
      Link(
        {
          class: "grid min-w-0 items-center grid-cols-[auto_minmax(0,_1fr)_auto] gap-3 p-4 text-color-text-primary text-decoration-none bg-color-bg-surface border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card transition-[background-color_var(--dm-duration-fast)_var(--dm-ease-standard),_border-color_var(--dm-duration-fast)_var(--dm-ease-standard),_transform_var(--dm-duration-fast)_var(--dm-ease-standard)] hover:bg-color-bg-interactive hover:border-color-color-primary-border-strong hover:transform-[translateY(-1px)] [&_>_svg]:text-color-text-secondary motion-reduce:transition-none motion-reduce:hover:transform-none dm-focus-ring",
          href: "https://ltaoo.github.io/wx_channels_download/guide/start.html",
          target: "_blank",
          attributes: {
            rel: "noopener noreferrer",
            "aria-label": "打开使用文档（新窗口）",
          },
        },
        [
          View({ class: "grid w-control-height-base h-control-height-base place-items-center text-color-primary bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-content" }, [
            Timeless.Icon({ name: "file-text", size: 20 }),
          ]),
          View({ class: "min-w-0" }, [
            View({ class: "text-sm font-semibold leading-snug" }, ["使用文档"]),
            View({ class: "mt-1 overflow-hidden text-color-text-secondary font-mono text-xs leading-base text-ellipsis whitespace-nowrap" }, [
              "ltaoo.github.io/wx_channels_download/guide/start.html",
            ]),
          ]),
          Timeless.Icon({ name: "external-link", size: 16 }),
        ],
      ),
    ]),
  ]);
}

function MCPUsageGuide(props) {
  return View({ class: "min-w-0 overflow-hidden bg-color-bg-surface border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card" }, [
    View({ class: "flex items-center gap-3 p-4 bg-color-bg-subtle border-bottom-[var(--dm-border-width)_solid_var(--dm-color-border)]" }, [
      View({ class: "grid w-control-height-base h-control-height-base flex-[0_0_auto] place-items-center text-color-primary bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-content" }, [
        Timeless.Icon({ name: "file-code", size: 18 }),
      ]),
      View({}, [
        View({ class: "text-color-text-primary text-sm font-semibold" }, ["让 Agent 创建下载任务"]),
        View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
          "完成连接后，Agent 会按以下顺序调用工具。",
        ]),
      ]),
    ]),
    View({ as: "ol", class: "flex m-0 p-4 flex-col gap-4 list-style-none" }, [
      View({ as: "li", class: "grid min-w-0 grid-cols-[28px_minmax(0,_1fr)] gap-3" }, [
        View({ class: "grid w-28px h-28px place-items-center text-color-primary text-caption font-bold bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-badge" }, ["1"]),
        View({ class: "min-w-0" }, [
          View({ class: "text-color-text-primary text-sm font-semibold leading-28px" }, ["添加 MCP Server"]),
          View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
            "在 Agent 的 MCP Servers 或工具集成设置中新增 Streamable HTTP 服务。",
          ]),
          View({ as: "pre", class: "m-[var(--dm-space-3)_0_0] p-3 overflow-x-auto text-color-media-text font-mono text-xs leading-relaxed bg-color-media-canvas border-[var(--dm-border-width)_solid_var(--dm-color-media-border)] rounded-content whitespace-pre" }, [
            View({ as: "code" }, [
              "名称: dm\n传输: Streamable HTTP\nURL: ",
              props.endpoint,
            ]),
          ]),
        ]),
      ]),
      View({ as: "li", class: "grid min-w-0 grid-cols-[28px_minmax(0,_1fr)] gap-3" }, [
        View({ class: "grid w-28px h-28px place-items-center text-color-primary text-caption font-bold bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-badge" }, ["2"]),
        View({ class: "min-w-0" }, [
          View({ class: "text-color-text-primary text-sm font-semibold leading-28px" }, ["让 Agent 解析链接"]),
          View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
            "连接成功后，Agent 先检查平台状态，再解析链接并保留返回的 job_id。",
          ]),
          View({ class: "flex min-w-0 items-center gap-2 mt-3 overflow-x-auto text-color-text-secondary scrollbar-width-thin [&_code]:flex-[0_0_auto] [&_code]:p-[var(--dm-space-2)_var(--dm-space-2-5)] [&_code]:text-color-primary [&_code]:font-mono [&_code]:text-xs [&_code]:bg-color-primary-soft [&_code]:border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] [&_code]:rounded-content" }, [
            View({ as: "code" }, ["get_platform_status"]),
            Timeless.Icon({ name: "arrow-right", size: 14 }),
            View({ as: "code" }, ['fetch_content({ "url": "<内容链接>" })']),
          ]),
        ]),
      ]),
      View({ as: "li", class: "grid min-w-0 grid-cols-[28px_minmax(0,_1fr)] gap-3" }, [
        View({ class: "grid w-28px h-28px place-items-center text-color-primary text-caption font-bold bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-badge" }, ["3"]),
        View({ class: "min-w-0" }, [
          View({ class: "text-color-text-primary text-sm font-semibold leading-28px" }, ["确认并创建下载任务"]),
          View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
            "查看解析结果并确认下载后，让 Agent 使用 job_id 创建任务；等待模式会在完成后返回任务和文件信息。",
          ]),
          View({ as: "pre", class: "m-[var(--dm-space-3)_0_0] p-3 overflow-x-auto text-color-media-text font-mono text-xs leading-relaxed bg-color-media-canvas border-[var(--dm-border-width)_solid_var(--dm-color-media-border)] rounded-content whitespace-pre" }, [
            View({ as: "code" }, [
              'download_content({\n  "job_id": "<job_id>",\n  "wait_for_completion": true\n})',
            ]),
          ]),
        ]),
      ]),
      View({ as: "li", class: "grid min-w-0 grid-cols-[28px_minmax(0,_1fr)] gap-3" }, [
        View({ class: "grid w-28px h-28px place-items-center text-color-primary text-caption font-bold bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-badge" }, ["4"]),
        View({ class: "min-w-0" }, [
          View({ class: "text-color-text-primary text-sm font-semibold leading-28px" }, ["视频号外部下载与解密"]),
          View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
            "fetch_content 的 download_resources 提供下载地址和可选 key。可交给 aria2 等工具下载到运行本服务的机器；仅 requires_decryption 为 true 时，使用绝对路径原地解密。",
          ]),
          View({ as: "pre", class: "m-[var(--dm-space-3)_0_0] p-3 overflow-x-auto text-color-media-text font-mono text-xs leading-relaxed bg-color-media-canvas border-[var(--dm-border-width)_solid_var(--dm-color-media-border)] rounded-content whitespace-pre" }, [
            View({ as: "code" }, [
              'decrypt_wxchannels_video({\n  "file_path": "<已下载文件的绝对路径>",\n  "key": "<decode_key>"\n})',
            ]),
          ]),
        ]),
      ]),
    ]),
    View({ class: "m-[0_var(--dm-space-4)_var(--dm-space-4)_calc(28px_+_var(--dm-space-7))] p-[var(--dm-space-3)_var(--dm-space-4)] bg-[color-mix(_in_srgb,_var(--dm-color-primary-soft)_42%,_var(--dm-color-bg-surface)_)] border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-card max-520:mr-4 max-520:ml-4" }, [
      View({ class: "text-color-primary text-caption font-semibold" }, ["可以直接对 Agent 说"]),
      View({ as: "blockquote", class: "m-[var(--dm-space-2)_0_0] text-color-text-primary text-sm leading-relaxed" }, [
        "使用 dm MCP 处理这个链接：<内容链接>。先检查平台状态并解析内容，向我展示标题和下载项；得到我的确认后创建下载任务。若微信视频号改用 aria2 等外部下载器，请使用 download_resources 的地址下载到服务所在机器，并仅在存在 decode_key 时调用解密工具。",
      ]),
    ]),
  ]);
}

function MCPSettingsDetails(props) {
  var model = props.model;

  return Show({
    when: Timeless.combine(
      { loading: model.state.loading, data: model.state.data },
      function (state) {
        return state.loading && !state.data;
      },
    ),
    ok() {
      return View({ class: "flex min-h-360px items-center justify-center flex-col gap-3 p-7 text-color-text-secondary text-center bg-color-bg-subtle border-[var(--dm-border-width)_dashed_var(--dm-color-border-strong)] rounded-content" }, [
        View({ class: "grid w-control-height-lg h-control-height-lg place-items-center text-color-primary bg-color-primary-soft rounded-round animation-[settings-certificate-spin_1.1s_linear_infinite] motion-reduce:animation-none" }, [
          Timeless.Icon({ name: "refresh-cw", size: 22 }),
        ]),
        View({ class: "text-color-text-primary text-md font-semibold" }, ["正在读取 MCP 状态"]),
        View({ class: "max-w-440px text-sm leading-relaxed" }, [
          "正在检查 Agent 工具入口和当前配置。",
        ]),
      ]);
    },
    else() {
      return Show({
        when: Timeless.combine(
          { error: model.state.error, data: model.state.data },
          function (state) {
            return Boolean(state.error) && !state.data;
          },
        ),
        ok() {
          return View(
            { class: "flex min-h-360px items-center justify-center flex-col gap-3 p-7 text-color-text-secondary text-center bg-color-bg-subtle border-[var(--dm-border-width)_dashed_var(--dm-color-border-strong)] rounded-content [&_>_svg]:text-color-danger" },
            [
              Timeless.Icon({ name: "circle-alert", size: 28 }),
              View({ class: "text-color-text-primary text-md font-semibold" }, [
                "MCP 状态读取失败",
              ]),
              ErrorMessage({ tip: model.methods.tip, error: model.state.error }),
              Button(
                {
                  store: model.ui.retry_button$,
                  prefix: Timeless.Icon({ name: "refresh-cw", size: 14 }),
                },
                ["重新读取"],
              ),
            ],
          );
        },
        else() {
          return View({ class: "flex min-w-0 flex-col gap-4" }, [
            View({ class: "grid min-w-0 items-center grid-cols-[auto_minmax(0,_1fr)_auto] gap-4 p-4-5 bg-[linear-gradient(_135deg,_color-mix(in_srgb,_var(--dm-color-primary-soft)_64%,_transparent),_transparent_72%_),_var(--dm-color-bg-surface)] border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-card max-520:grid-cols-[auto_minmax(0,_1fr)]" }, [
              View({ class: "grid w-50px h-50px place-items-center text-color-primary bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border-strong)] rounded-content" }, [
                Timeless.Icon({ name: "braces", size: 24 }),
              ]),
              View({ class: "min-w-0" }, [
                View({ class: "text-color-text-primary font-display text-lg font-bold leading-tight" }, [
                  "Streamable HTTP",
                ]),
                View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
                  "允许兼容 MCP 的 Agent 查询平台、解析链接并创建下载任务。",
                ]),
              ]),
              Tag(
                {
                  name: "settings-mcp__status",
                  class: Timeless.classNames([
                    "border-[var(--dm-border-width)_solid_currentColor] max-520:grid-column-[1_/_-1] max-520:justify-self-start max-520:ml-[calc(50px_+_var(--dm-space-4))]",
                    Timeless.computed(model.state.enabled, function (enabled) {
                      return enabled
                        ? "text-color-success bg-color-success-soft"
                        : "text-color-text-secondary bg-color-bg-subtle";
                    }),
                  ]),
                },
                [
                  View({ class: "w-7px h-7px bg-currentColor rounded-round" }),
                  Timeless.computed(model.state.enabled, function (enabled) {
                    return enabled ? "运行中" : "已停用";
                  }),
                ],
              ),
            ]),

            View({ class: "flex items-center justify-between gap-4 p-4 bg-color-bg-surface border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card max-520:items-stretch max-520:flex-col" }, [
              View({ class: "min-w-0" }, [
                View({ class: "text-color-text-primary text-sm font-semibold" }, [
                  "允许 Agent 连接",
                ]),
                View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
                  "更改仅在当前运行期间生效；应用重启后默认关闭。",
                ]),
              ]),
              Button(
                {
                  store: model.ui.toggle_button$,
                  attributes: {
                    "aria-label": Timeless.computed(
                      model.state.enabled,
                      function (enabled) {
                        return enabled ? "关闭 MCP 服务" : "开启 MCP 服务";
                      },
                    ),
                  },
                },
                [
                  Timeless.computed(model.state.enabled, function (enabled) {
                    return enabled ? "关闭服务" : "开启服务";
                  }),
                ],
              ),
            ]),
            MCPUsageGuide({ endpoint: model.state.endpoint }),
            View({ class: "min-w-0 p-4 bg-color-bg-subtle border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card" }, [
              View({ class: "mb-2 text-color-text-secondary text-caption font-semibold tracking-wide" }, ["连接地址"]),
              View({ as: "code", class: "block p-3 overflow-x-auto text-color-media-text font-mono text-sm leading-base bg-color-media-canvas border-[var(--dm-border-width)_solid_var(--dm-color-media-border)] rounded-content whitespace-nowrap" }, [
                model.state.endpoint,
              ]),
              View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
                "将此 URL 填入支持 Streamable HTTP 的 MCP 客户端。服务默认只监听 API 配置的地址。",
              ]),
            ]),
            View({ class: "min-w-0 p-4 bg-color-bg-subtle border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card" }, [
              View({ class: "mb-2 text-color-text-secondary text-caption font-semibold tracking-wide" }, ["可用工具"]),
              View({ class: "flex flex-wrap gap-2 [&_code]:font-mono" }, [
                For({
                  each: Timeless.computed(model.state.data, function (data) {
                    return (data && data.tools) || [];
                  }),
                  render(tool) {
                    return Tag(
                      {
                        as: "code",
                        variant: "info",
                        name: "settings-mcp-tool",
                      },
                      [tool],
                    );
                  },
                }),
              ]),
            ]),
            View(
              {
                class:
                  "flex items-start gap-2 p-3 text-sm leading-base border-[var(--dm-border-width)_solid_transparent] rounded-content [&_>_svg]:flex-[0_0_auto] [&_>_svg]:mt-[0.1em] text-color-warning bg-color-warning-soft border-color-[color-mix(in_srgb,_var(--dm-color-warning)_32%,_transparent)]",
              },
              [
                Timeless.Icon({ name: "circle-alert", size: 16 }),
                "下载工具可以向本机目录写入文件。仅向可信 Agent 开放；若 API 监听局域网地址，请同时确认网络访问边界。",
              ],
            ),
          ]);
        },
      });
    },
  });
}

export function SettingsDialog(props) {
  return Dialog(
    {
      store: props.dialog,
      class: "dm-dialog--settings",
      closeLabel: "关闭设置",
      attributes: { n: "settings-dialog" },
    },
    [
      DialogTitle({ class: "flex items-center gap-3 flex-[0_0_auto] p-[var(--dm-space-4)_var(--dm-space-6)] border-bottom-[var(--dm-border-width)_solid_var(--dm-color-border)] max-520:p-[var(--dm-space-3)_var(--dm-space-4)]" }, [
        View({ class: "grid w-control-height-base h-control-height-base flex-[0_0_auto] place-items-center text-color-primary bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-content" }, [
          Timeless.Icon({ name: "settings", size: 18 }),
        ]),
        View({ class: "flex min-w-0 flex-col gap-0-5" }, [
          View({ as: "span", class: "text-color-text-primary font-display text-lg font-bold leading-tight" }, [
            "设置",
          ]),
          View({ as: "span", class: "text-color-text-secondary font-sans text-sm font-regular leading-base max-520:hidden" }, [
            "查看当前运行环境和安全配置",
          ]),
        ]),
      ]),
      DialogBody({ class: "[&.dm-dialog-body]:grid [&.dm-dialog-body]:min-h-0 [&.dm-dialog-body]:flex-[1_1_auto] [&.dm-dialog-body]:grid-cols-[184px_minmax(0,_1fr)] [&.dm-dialog-body]:p-0 [&.dm-dialog-body]:overflow-hidden max-760:[&.dm-dialog-body]:grid-cols-[minmax(0,_1fr)] max-760:[&.dm-dialog-body]:grid-rows-[auto_minmax(0,_1fr)]" }, [
        View(
          {
            as: "nav",
            class: "flex min-w-0 flex-col gap-1 p-[var(--dm-space-4)_var(--dm-space-3)] bg-color-bg-subtle border-right-[var(--dm-border-width)_solid_var(--dm-color-border)] max-760:flex-row max-760:p-[var(--dm-space-2)_var(--dm-space-3)] max-760:overflow-x-auto max-760:border-right-0 max-760:border-bottom-[var(--dm-border-width)_solid_var(--dm-color-border)]",
            ariaLabel: "设置菜单",
          },
          [
            Button(
              {
                store: props.certificate_menu_button,
                class: Timeless.classNames([
                  "dm-button--sidebar-nav",
                  Timeless.computed(props.section, function (section) {
                    return section === "certificate" ? "is-active" : null;
                  }),
                ]),
                attributes: {
                  n: "settings-certificate-menu-action",
                  "aria-current": Timeless.computed(
                    props.section,
                    function (section) {
                      return section === "certificate" ? "page" : undefined;
                    },
                  ),
                },
              },
              [
                Timeless.Icon({ name: "file-lock", size: 17 }),
                View({ as: "span" }, ["证书"]),
              ],
            ),
            Button(
              {
                store: props.mcp_menu_button,
                class: Timeless.classNames([
                  "dm-button--sidebar-nav",
                  Timeless.computed(props.section, function (section) {
                    return section === "mcp" ? "is-active" : null;
                  }),
                ]),
                attributes: {
                  n: "settings-mcp-menu-action",
                  "aria-current": Timeless.computed(
                    props.section,
                    function (section) {
                      return section === "mcp" ? "page" : undefined;
                    },
                  ),
                },
              },
              [
                Timeless.Icon({ name: "braces", size: 17 }),
                View({ as: "span" }, ["MCP"]),
              ],
            ),
            Button(
              {
                store: props.about_menu_button,
                class: Timeless.classNames([
                  "dm-button--sidebar-nav",
                  Timeless.computed(props.section, function (section) {
                    return section === "about" ? "is-active" : null;
                  }),
                ]),
                attributes: {
                  n: "settings-about-menu-action",
                  "aria-current": Timeless.computed(
                    props.section,
                    function (section) {
                      return section === "about" ? "page" : undefined;
                    },
                  ),
                },
              },
              [
                Timeless.Icon({ name: "file-box", size: 17 }),
                View({ as: "span" }, ["关于"]),
              ],
            ),
          ],
        ),
        View({ class: "min-w-0 min-h-0 overflow-auto scrollbar-gutter-stable" }, [
          Show({
            when: Timeless.computed(props.section, function (section) {
              return section === "certificate";
            }),
            ok() {
              return View({ class: "w-full p-[var(--dm-space-5)_var(--dm-space-6)_var(--dm-space-7)] max-760:p-[var(--dm-space-4)_var(--dm-space-4)_var(--dm-space-6)]" }, [
                View({ class: "flex items-start justify-between gap-4 mb-5 max-520:items-center" }, [
                  View({}, [
                    View({ class: "text-color-text-primary font-display text-lg font-bold leading-tight" }, ["证书"]),
                    View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
                      "代理服务当前加载并用于 HTTPS 解密的根证书。",
                    ]),
                  ]),
                  Button(
                    {
                      store: props.refresh_button,
                      prefix: Timeless.Icon({ name: "refresh-cw", size: 14 }),
                      attributes: { "aria-label": "刷新证书信息" },
                    },
                    ["刷新"],
                  ),
                ]),
                Show({
                  when: props.loading,
                  ok() {
                    return View({ class: "flex min-h-360px items-center justify-center flex-col gap-3 p-7 text-color-text-secondary text-center bg-color-bg-subtle border-[var(--dm-border-width)_dashed_var(--dm-color-border-strong)] rounded-content" }, [
                      View({ class: "grid w-control-height-lg h-control-height-lg place-items-center text-color-primary bg-color-primary-soft rounded-round animation-[settings-certificate-spin_1.1s_linear_infinite] motion-reduce:animation-none" }, [
                        Timeless.Icon({ name: "refresh-cw", size: 22 }),
                      ]),
                      View({ class: "text-color-text-primary text-md font-semibold" }, [
                        "正在读取证书",
                      ]),
                      View({ class: "max-w-440px text-sm leading-relaxed" }, [
                        "正在检查证书来源、系统信任和有效期。",
                      ]),
                    ]);
                  },
                  else() {
                    return Show({
                      when: Timeless.computed(props.error, function (error) {
                        return Boolean(error);
                      }),
                      ok() {
                        return View(
                          {
                            class:
                              "flex min-h-360px items-center justify-center flex-col gap-3 p-7 text-color-text-secondary text-center bg-color-bg-subtle border-[var(--dm-border-width)_dashed_var(--dm-color-border-strong)] rounded-content [&_>_svg]:text-color-danger",
                          },
                          [
                            Timeless.Icon({ name: "circle-alert", size: 28 }),
                            View({ class: "text-color-text-primary text-md font-semibold" }, [
                              "证书信息读取失败",
                            ]),
                            ErrorMessage({ tip: props.tip, error: props.error }),
                            Button(
                              {
                                store: props.retry_button,
                                prefix: Timeless.Icon({
                                  name: "refresh-cw",
                                  size: 14,
                                }),
                              },
                              ["重新读取"],
                            ),
                          ],
                        );
                      },
                      else() {
                        return Show({
                          when: Timeless.computed(
                            props.certificate,
                            function (data) {
                              return Boolean(data);
                            },
                          ),
                          ok() {
                            return CertificateSettingsDetails({
                              certificate: props.certificate,
                              delete_button: props.delete_button,
                            });
                          },
                        });
                      },
                    });
                  },
                }),
              ]);
            },
          }),
          Show({
            when: Timeless.computed(props.section, function (section) {
              return section === "mcp";
            }),
            ok() {
              return View({ class: "w-full p-[var(--dm-space-5)_var(--dm-space-6)_var(--dm-space-7)] max-760:p-[var(--dm-space-4)_var(--dm-space-4)_var(--dm-space-6)]" }, [
                View({ class: "flex items-start justify-between gap-4 mb-5 max-520:items-center" }, [
                  View({}, [
                    View({ class: "text-color-text-primary font-display text-lg font-bold leading-tight" }, ["MCP"]),
                    View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
                      "管理供本机或局域网 Agent 使用的工具服务。",
                    ]),
                  ]),
                  Button(
                    {
                      store: props.mcp_refresh_button,
                      prefix: Timeless.Icon({ name: "refresh-cw", size: 14 }),
                      attributes: { "aria-label": "刷新 MCP 状态" },
                    },
                    ["刷新"],
                  ),
                ]),
                MCPSettingsDetails({ model: props.mcp_model }),
              ]);
            },
          }),
          Show({
            when: Timeless.computed(props.section, function (section) {
              return section === "about";
            }),
            ok() {
              return View({ class: "w-full p-[var(--dm-space-5)_var(--dm-space-6)_var(--dm-space-7)] max-760:p-[var(--dm-space-4)_var(--dm-space-4)_var(--dm-space-6)]" }, [
                View({ class: "flex items-start justify-between gap-4 mb-5 max-520:items-center" }, [
                  View({}, [
                    View({ class: "text-color-text-primary font-display text-lg font-bold leading-tight" }, ["关于"]),
                    View({ class: "mt-1 text-color-text-secondary text-sm leading-base" }, [
                      "查看当前版本，以及项目的代码和使用文档。",
                    ]),
                  ]),
                ]),
                AboutSettingsDetails({ version: props.version }),
              ]);
            },
          }),
        ]),
      ]),
    ],
  );
}

export function UpdateDialog(props) {
  var model = props.store;
  var snapshot_ = model.state.snapshot;

  return Dialog(
    {
      store: model.ui.dialog$,
      class: "dm-dialog--md",
      zIndex: 12000,
      showClose: false,
      attributes: { "aria-labelledby": "update-dialog-title" },
    },
    [
      DialogHeader({ class: "[&.dm-dialog-header]:flex-row [&.dm-dialog-header]:items-start [&.dm-dialog-header]:gap-3 [&.dm-dialog-header]:p-[var(--dm-space-5)_var(--dm-space-5)_var(--dm-space-4)] [&.dm-dialog-header]:border-bottom-[var(--dm-border-width)_solid_var(--dm-color-border)] max-520:[&.dm-dialog-header]:px-4" }, [
        View({ class: "grid w-control-height-lg h-control-height-lg flex-[0_0_auto] place-items-center text-color-primary bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-content max-520:hidden" }, [
          Timeless.Icon({ name: "cloud-download", size: 23 }),
        ]),
        View({ class: "grid min-w-0 gap-1" }, [
          DialogTitle({ attributes: { id: "update-dialog-title" } }, [
            model.state.phase_title,
          ]),
          DialogDescription({}, [Timeless.combine(
            { status: model.state.status, message: model.state.phase_message }, (state) =>
            state.status === "error" ? "更新未完成，请重新尝试。" : state.message,
          )]),
        ]),
      ]),
      DialogBody({ class: "[&.dm-dialog-body]:grid [&.dm-dialog-body]:gap-4 [&.dm-dialog-body]:p-5 max-520:[&.dm-dialog-body]:px-4" }, [
        View({ class: "grid grid-cols-[minmax(0,_1fr)_auto_minmax(0,_1fr)] items-center gap-3 p-4 bg-color-bg-subtle border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-card max-520:p-3" }, [
          View({ class: "grid min-w-0 gap-1 [&:last-child]:text-right [&:last-child_.group-update-dialog__version-value]:text-color-primary" }, [
            View({ class: "text-color-text-secondary text-xs leading-base" }, ["当前版本"]),
            View({ as: "code", class: "group-update-dialog__version-value overflow-hidden text-color-text-primary font-mono text-base font-bold text-ellipsis whitespace-nowrap" }, [
              Timeless.computed(snapshot_, function (snapshot) {
                return snapshot.current_version || "未知";
              }),
            ]),
          ]),
          View(
            {
              class: "text-color-text-disabled text-xl",
              attributes: { "aria-hidden": "true" },
            },
            ["→"],
          ),
          View({ class: "grid min-w-0 gap-1 [&:last-child]:text-right [&:last-child_.group-update-dialog__version-value]:text-color-primary" }, [
            View({ class: "text-color-text-secondary text-xs leading-base" }, ["最新版本"]),
            View({ as: "code", class: "group-update-dialog__version-value overflow-hidden text-color-text-primary font-mono text-base font-bold text-ellipsis whitespace-nowrap" }, [
              Show({
                when: model.state.has_latest_version,
                ok() {
                  return model.state.latest_version.value;
                },
                else() {
                  return "未知";
                },
              }),
            ]),
          ]),
        ]),
        Show({
          when: Timeless.computed(snapshot_, function (snapshot) {
            return Boolean(snapshot.published_at || snapshot.asset_name);
          }),
          ok() {
            return View({ class: "flex flex-wrap gap-[var(--dm-space-2)_var(--dm-space-4)] text-color-text-secondary text-xs [&_>_span]:inline-flex [&_>_span]:min-w-0 [&_>_span]:items-center [&_>_span]:gap-1-5 [&_>_span]:overflow-wrap-anywhere" }, [
              Show({
                when: Timeless.computed(model.state.published_text, Boolean),
                ok() {
                  return View({ as: "span" }, [
                    Timeless.Icon({ name: "calendar", size: 14 }),
                    model.state.published_text,
                  ]);
                },
              }),
              Show({
                when: Timeless.computed(snapshot_, function (snapshot) {
                  return Boolean(snapshot.asset_name);
                }),
                ok() {
                  return View({ as: "span" }, [
                    Timeless.Icon({ name: "file-box", size: 14 }),
                    Timeless.computed(snapshot_, function (snapshot) {
                      return snapshot.asset_name;
                    }),
                  ]);
                },
              }),
            ]);
          },
        }),
        Show({
          when: Timeless.computed(model.state.status, function (status) {
            return ["downloading", "ready", "restarting"].includes(status);
          }),
          ok() {
            return View(
              {
                class: "grid gap-2 p-4 bg-color-info-soft border-[var(--dm-border-width)_solid_var(--dm-color-info-border)] rounded-card",
                attributes: {
                  role: "progressbar",
                  "aria-valuemin": "0",
                  "aria-valuemax": "100",
                  "aria-valuenow": model.state.percent,
                },
              },
              [
                View({ class: "flex items-center justify-between gap-3 text-color-text-primary text-sm font-semibold" }, [
                  View({ as: "span" }, [model.state.phase_title]),
                  View({ as: "span" }, [
                    Show({
                      when: model.state.has_total,
                      ok() {
                        return Timeless.computed(
                          model.state.percent,
                          function (percent) {
                            return `${Math.round(percent)}%`;
                          },
                        );
                      },
                      else() {
                        return "处理中";
                      },
                    }),
                  ]),
                ]),
                View({ class: "h-2 overflow-hidden bg-[color-mix(in_srgb,_var(--dm-color-info)_15%,_transparent)] rounded-round" }, [
                  View({
                    class: Timeless.classNames([
                      "h-full bg-color-info rounded-inherit transition-[width_var(--dm-duration-slow)_var(--dm-ease-standard)] [&.is-indeterminate]:animation-[update-dialog-progress_1.1s_ease-in-out_infinite] motion-reduce:transition-none motion-reduce:[&.is-indeterminate]:animation-none",
                      Timeless.computed(
                        model.state.has_total,
                        function (has_total) {
                          return has_total ? null : "is-indeterminate";
                        },
                      ),
                    ]),
                    style: Timeless.combine(
                      {
                        percent: model.state.percent,
                        has_total: model.state.has_total,
                      },
                      function (state) {
                        return {
                          width: state.has_total ? `${state.percent}%` : "36%",
                        };
                      },
                    ),
                  }),
                ]),
                View({ class: "text-color-text-secondary font-mono text-caption" }, [
                  model.state.progress_text,
                ]),
              ],
            );
          },
        }),

        Show({
          when: Timeless.computed(snapshot_, function (snapshot) {
            return Boolean(snapshot.body);
          }),
          ok() {
            return View({ class: "grid min-h-0 gap-2" }, [
              View({ class: "text-color-text-primary text-sm font-semibold" }, ["版本说明"]),
              View({ as: "pre", class: "max-h-180px m-0 p-3 overflow-auto text-color-text-secondary bg-color-bg-subtle border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-content font-sans text-xs leading-relaxed whitespace-pre-wrap overflow-wrap-anywhere" }, [
                Timeless.computed(snapshot_, function (snapshot) {
                  return snapshot.body;
                }),
              ]),
            ]);
          },
        }),
      ]),
      Show({
        when: Timeless.computed(model.state.busy, function (busy) {
          return !busy;
        }),
        ok() {
          return DialogFooter({ class: "[&.dm-dialog-footer]:p-[var(--dm-space-4)_var(--dm-space-5)] max-520:[&.dm-dialog-footer]:px-4" }, [
            Button({ store: model.ui.cancel_button$ }, ["稍后"]),
            Show({
              when: model.state.can_download,
              ok() {
                return Button(
                  {
                    store: model.ui.download_button$,
                    prefix: Timeless.Icon({ name: "download", size: 15 }),
                  },
                  [
                    Timeless.computed(model.state.status, function (status) {
                      return status === "error" ? "重新下载" : "下载并更新";
                    }),
                  ],
                );
              },
            }),
          ]);
        },
      }),
    ],
  );
}
