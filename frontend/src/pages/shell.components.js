const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

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
  return window.format_time(value, value ? String(value) : "未提供", {
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
  return Array.isArray(value) && value.length > 0
    ? value.join("、")
    : "未提供";
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
  return View({ class: "settings-certificate-detail" }, [
    View({ class: "settings-certificate-detail__label" }, [props.label]),
    View(
      {
        class: Timeless.classNames([
          "settings-certificate-detail__value",
          props.mono ? "settings-certificate-detail__value--mono" : null,
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

  return View({ class: "settings-certificate" }, [
    View({ class: "settings-certificate-hero" }, [
      View({ class: "settings-certificate-hero__mark" }, [
        Timeless.Icon({ name: "file-lock", size: 25 }),
      ]),
      View({ class: "settings-certificate-hero__identity" }, [
        View({ class: "settings-certificate-hero__eyebrow" }, [
          "当前实际使用",
        ]),
        View({ class: "settings-certificate-hero__name" }, [
          Timeless.computed(certificate_, function (data) {
            return (data && data.name) || "未命名证书";
          }),
        ]),
        View({ class: "settings-certificate-hero__source" }, [
          Timeless.computed(certificate_, function (data) {
            return certificate_source_label(data && data.source);
          }),
        ]),
      ]),
      View(
        {
          class: Timeless.classNames([
            "settings-certificate-status",
            Timeless.computed(status_, function (status) {
              return "settings-certificate-status--" + status.tone;
            }),
          ]),
        },
        [
          Show({
            when: Timeless.computed(status_, function (status) {
              return status.icon === "check";
            }),
            ok() {
              return Timeless.Icon({ name: "check", size: 14 });
            },
            else() {
              return Show({
                when: Timeless.computed(status_, function (status) {
                  return status.icon === "circle-x";
                }),
                ok() {
                  return Timeless.Icon({ name: "circle-x", size: 14 });
                },
                else() {
                  return Timeless.Icon({ name: "circle-alert", size: 14 });
                },
              });
            },
          }),
          Timeless.computed(status_, function (status) {
            return status.label;
          }),
        ],
      ),
    ]),
    Show({
      when: Timeless.computed(certificate_, function (data) {
        return Boolean(data && data.install_status_error);
      }),
      ok() {
        return View(
          {
            class:
              "settings-certificate-notice settings-certificate-notice--danger",
          },
          [
            Timeless.Icon({ name: "circle-alert", size: 16 }),
            Timeless.computed(certificate_, function (data) {
              return data.install_status_error;
            }),
          ],
        );
      },
    }),
    Show({
      when: Timeless.computed(certificate_, function (data) {
        return (
          Array.isArray(data && data.risk_warnings) &&
          data.risk_warnings.length > 0
        );
      }),
      ok() {
        return View({ class: "settings-certificate-notices" }, [
          For({
            each: Timeless.computed(certificate_, function (data) {
              return data.risk_warnings;
            }),
            render(warning) {
              return View(
                {
                  class:
                    "settings-certificate-notice settings-certificate-notice--warning",
                },
                [Timeless.Icon({ name: "circle-alert", size: 16 }), warning],
              );
            },
          }),
        ]);
      },
    }),
    View({ class: "settings-certificate-section" }, [
      View({ class: "settings-certificate-section__title" }, ["证书身份"]),
      View({ class: "settings-certificate-grid" }, [
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
    View({ class: "settings-certificate-section" }, [
      View({ class: "settings-certificate-section__title" }, [
        "有效期与信任",
      ]),
      View({ class: "settings-certificate-grid" }, [
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
    View({ class: "settings-certificate-fingerprint" }, [
      View({ class: "settings-certificate-fingerprint__label" }, [
        "SHA-256 指纹",
      ]),
      View({ as: "code", class: "settings-certificate-fingerprint__value" }, [
        Timeless.computed(detail_, function (detail) {
          return detail.fingerprint_sha256 || "未提供";
        }),
      ]),
    ]),
    View({ class: "settings-certificate-section" }, [
      View({ class: "settings-certificate-section__title" }, ["配置来源"]),
      View({ class: "settings-certificate-config" }, [
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
        return Boolean(data && data.parse_error);
      }),
      ok() {
        return View(
          {
            class:
              "settings-certificate-notice settings-certificate-notice--danger",
          },
          [
            Timeless.Icon({ name: "circle-alert", size: 16 }),
            Timeless.computed(certificate_, function (data) {
              return "证书内容解析失败：" + data.parse_error;
            }),
          ],
        );
      },
    }),
    Show({
      when: Timeless.computed(certificate_, function (data) {
        return Boolean(data && data.pem);
      }),
      ok() {
        return View({ as: "details", class: "settings-certificate-pem" }, [
          View(
            { as: "summary", class: "settings-certificate-pem__summary" },
            ["查看 PEM 原文"],
          ),
          View({ as: "pre", class: "settings-certificate-pem__content" }, [
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
  return View({ class: "settings-about" }, [
    View({ class: "settings-about__hero" }, [
      View({ class: "settings-about__logo-wrap" }, [
        Img({
          class: "settings-about__logo",
          src: "public/logo.png?v=logo-only-v4",
          alt: "D&M",
          attributes: { draggable: "false" },
        }),
      ]),
      View({ class: "settings-about__identity" }, [
        View({ class: "settings-about__eyebrow" }, ["微信视频号下载工具"]),
        View({ class: "settings-about__name" }, ["wx_channels_download"]),
        View({ class: "settings-about__summary" }, [
          "在本机抓取、下载和管理内容。",
        ]),
      ]),
      View({ class: "settings-about__version" }, [
        View({ as: "span", class: "settings-about__version-label" }, [
          "版本",
        ]),
        View({ as: "code", class: "settings-about__version-value" }, [
          props.version,
        ]),
      ]),
    ]),
    View({ class: "settings-about__resources" }, [
      Link(
        {
          class: "settings-about__resource dm-focus-ring",
          href: "https://github.com/ltaoo/wx_channels_download",
          target: "_blank",
          attributes: {
            rel: "noopener noreferrer",
            "aria-label": "打开 GitHub 仓库（新窗口）",
          },
        },
        [
          View({ class: "settings-about__resource-icon" }, [
            Timeless.Icon({ name: "git-fork", size: 20 }),
          ]),
          View({ class: "settings-about__resource-copy" }, [
            View({ class: "settings-about__resource-title" }, [
              "GitHub 仓库",
            ]),
            View({ class: "settings-about__resource-url" }, [
              "github.com/ltaoo/wx_channels_download",
            ]),
          ]),
          Timeless.Icon({ name: "external-link", size: 16 }),
        ],
      ),
      Link(
        {
          class: "settings-about__resource dm-focus-ring",
          href: "https://ltaoo.github.io/wx_channels_download/guide/start.html",
          target: "_blank",
          attributes: {
            rel: "noopener noreferrer",
            "aria-label": "打开使用文档（新窗口）",
          },
        },
        [
          View({ class: "settings-about__resource-icon" }, [
            Timeless.Icon({ name: "file-text", size: 20 }),
          ]),
          View({ class: "settings-about__resource-copy" }, [
            View({ class: "settings-about__resource-title" }, ["使用文档"]),
            View({ class: "settings-about__resource-url" }, [
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
  return View({ class: "settings-mcp__guide" }, [
    View({ class: "settings-mcp__guide-header" }, [
      View({ class: "settings-mcp__guide-icon" }, [
        Timeless.Icon({ name: "file-code", size: 18 }),
      ]),
      View({}, [
        View({ class: "settings-mcp__guide-title" }, [
          "让 Agent 创建下载任务",
        ]),
        View({ class: "settings-mcp__guide-description" }, [
          "完成连接后，Agent 会按以下顺序调用工具。",
        ]),
      ]),
    ]),
    View({ as: "ol", class: "settings-mcp__steps" }, [
      View({ as: "li", class: "settings-mcp__step" }, [
        View({ class: "settings-mcp__step-number" }, ["1"]),
        View({ class: "settings-mcp__step-body" }, [
          View({ class: "settings-mcp__step-title" }, ["添加 MCP Server"]),
          View({ class: "settings-mcp__step-description" }, [
            "在 Agent 的 MCP Servers 或工具集成设置中新增 Streamable HTTP 服务。",
          ]),
          View({ as: "pre", class: "settings-mcp__example" }, [
            View({ as: "code" }, [
              "名称: dm\n传输: Streamable HTTP\nURL: ",
              props.endpoint,
            ]),
          ]),
        ]),
      ]),
      View({ as: "li", class: "settings-mcp__step" }, [
        View({ class: "settings-mcp__step-number" }, ["2"]),
        View({ class: "settings-mcp__step-body" }, [
          View({ class: "settings-mcp__step-title" }, ["让 Agent 解析链接"]),
          View({ class: "settings-mcp__step-description" }, [
            "连接成功后，Agent 先检查平台状态，再解析链接并保留返回的 job_id。",
          ]),
          View({ class: "settings-mcp__tool-flow" }, [
            View({ as: "code" }, ["get_platform_status"]),
            Timeless.Icon({ name: "arrow-right", size: 14 }),
            View({ as: "code" }, ['fetch_content({ "url": "<内容链接>" })']),
          ]),
        ]),
      ]),
      View({ as: "li", class: "settings-mcp__step" }, [
        View({ class: "settings-mcp__step-number" }, ["3"]),
        View({ class: "settings-mcp__step-body" }, [
          View({ class: "settings-mcp__step-title" }, ["确认并创建下载任务"]),
          View({ class: "settings-mcp__step-description" }, [
            "查看解析结果并确认下载后，让 Agent 使用 job_id 创建任务；等待模式会在完成后返回任务和文件信息。",
          ]),
          View({ as: "pre", class: "settings-mcp__example" }, [
            View({ as: "code" }, [
              'download_content({\n  "job_id": "<job_id>",\n  "wait_for_completion": true\n})',
            ]),
          ]),
        ]),
      ]),
      View({ as: "li", class: "settings-mcp__step" }, [
        View({ class: "settings-mcp__step-number" }, ["4"]),
        View({ class: "settings-mcp__step-body" }, [
          View({ class: "settings-mcp__step-title" }, [
            "视频号外部下载与解密",
          ]),
          View({ class: "settings-mcp__step-description" }, [
            "fetch_content 的 download_resources 提供下载地址和可选 key。可交给 aria2 等工具下载到运行本服务的机器；仅 requires_decryption 为 true 时，使用绝对路径原地解密。",
          ]),
          View({ as: "pre", class: "settings-mcp__example" }, [
            View({ as: "code" }, [
              'decrypt_wxchannels_video({\n  "file_path": "<已下载文件的绝对路径>",\n  "key": "<decode_key>"\n})',
            ]),
          ]),
        ]),
      ]),
    ]),
    View({ class: "settings-mcp__prompt" }, [
      View({ class: "settings-mcp__prompt-label" }, ["可以直接对 Agent 说"]),
      View({ as: "blockquote", class: "settings-mcp__prompt-content" }, [
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
      return View({ class: "settings-dialog__state" }, [
        View({ class: "settings-dialog__state-spinner" }, [
          Timeless.Icon({ name: "refresh-cw", size: 22 }),
        ]),
        View({ class: "settings-dialog__state-title" }, [
          "正在读取 MCP 状态",
        ]),
        View({ class: "settings-dialog__state-text" }, [
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
            { class: "settings-dialog__state settings-dialog__state--error" },
            [
              Timeless.Icon({ name: "circle-alert", size: 28 }),
              View({ class: "settings-dialog__state-title" }, [
                "MCP 状态读取失败",
              ]),
              View({ class: "settings-dialog__state-text" }, [
                model.state.error,
              ]),
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
          return View({ class: "settings-mcp" }, [
            View({ class: "settings-mcp__service" }, [
              View({ class: "settings-mcp__service-icon" }, [
                Timeless.Icon({ name: "braces", size: 24 }),
              ]),
              View({ class: "settings-mcp__service-copy" }, [
                View({ class: "settings-mcp__service-title" }, [
                  "Streamable HTTP",
                ]),
                View({ class: "settings-mcp__service-description" }, [
                  "允许兼容 MCP 的 Agent 查询平台、解析链接并创建下载任务。",
                ]),
              ]),
              View(
                {
                  class: Timeless.classNames([
                    "settings-mcp__status",
                    Timeless.computed(model.state.enabled, function (enabled) {
                      return enabled
                        ? "settings-mcp__status--running"
                        : "settings-mcp__status--stopped";
                    }),
                  ]),
                },
                [
                  View({ class: "settings-mcp__status-dot" }),
                  Timeless.computed(model.state.enabled, function (enabled) {
                    return enabled ? "运行中" : "已停用";
                  }),
                ],
              ),
            ]),
            Show({
              when: Timeless.computed(model.state.error, Boolean),
              ok() {
                return View(
                  {
                    class:
                      "settings-certificate-notice settings-certificate-notice--danger",
                  },
                  [
                    Timeless.Icon({ name: "circle-alert", size: 16 }),
                    model.state.error,
                  ],
                );
              },
            }),
            View({ class: "settings-mcp__control" }, [
              View({ class: "settings-mcp__control-copy" }, [
                View({ class: "settings-mcp__control-title" }, [
                  "允许 Agent 连接",
                ]),
                View({ class: "settings-mcp__control-description" }, [
                  "更改仅在当前运行期间生效；应用重启后默认开启。",
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
            View({ class: "settings-mcp__section" }, [
              View({ class: "settings-mcp__section-label" }, ["连接地址"]),
              View({ as: "code", class: "settings-mcp__endpoint" }, [
                model.state.endpoint,
              ]),
              View({ class: "settings-mcp__hint" }, [
                "将此 URL 填入支持 Streamable HTTP 的 MCP 客户端。服务默认只监听 API 配置的地址。",
              ]),
            ]),
            View({ class: "settings-mcp__section" }, [
              View({ class: "settings-mcp__section-label" }, ["可用工具"]),
              View({ class: "settings-mcp__tools" }, [
                For({
                  each: Timeless.computed(model.state.data, function (data) {
                    return (data && data.tools) || [];
                  }),
                  render(tool) {
                    return View({ as: "code" }, [tool]);
                  },
                }),
              ]),
            ]),
            MCPUsageGuide({ endpoint: model.state.endpoint }),
            View(
              {
                class:
                  "settings-certificate-notice settings-certificate-notice--warning",
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
      class: "settings-dialog",
      closeLabel: "关闭设置",
    },
    [
      DialogTitle({ class: "settings-dialog__heading" }, [
        View({ class: "settings-dialog__heading-icon" }, [
          Timeless.Icon({ name: "settings", size: 18 }),
        ]),
        View({ class: "settings-dialog__heading-copy" }, [
          View({ as: "span", class: "settings-dialog__heading-title" }, [
            "设置",
          ]),
          View(
            { as: "span", class: "settings-dialog__heading-description" },
            ["查看当前运行环境和安全配置"],
          ),
        ]),
      ]),
      DialogBody({ class: "settings-dialog__body" }, [
        View(
          {
            as: "nav",
            class: "settings-dialog__menus",
            ariaLabel: "设置菜单",
          },
          [
            Button(
              {
                store: props.certificate_menu_button,
                class: Timeless.classNames([
                  "settings-dialog__menu dm-justify-start",
                  Timeless.computed(props.section, function (section) {
                    return section === "certificate"
                      ? "settings-dialog__menu--active"
                      : null;
                  }),
                ]),
                attributes: {
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
                  "settings-dialog__menu dm-justify-start",
                  Timeless.computed(props.section, function (section) {
                    return section === "mcp"
                      ? "settings-dialog__menu--active"
                      : null;
                  }),
                ]),
                attributes: {
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
                  "settings-dialog__menu dm-justify-start",
                  Timeless.computed(props.section, function (section) {
                    return section === "about"
                      ? "settings-dialog__menu--active"
                      : null;
                  }),
                ]),
                attributes: {
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
        View({ class: "settings-dialog__panel" }, [
          Show({
            when: Timeless.computed(props.section, function (section) {
              return section === "certificate";
            }),
            ok() {
              return View({ class: "settings-dialog__panel-inner" }, [
                View({ class: "settings-dialog__panel-header" }, [
                  View({}, [
                    View({ class: "settings-dialog__panel-title" }, ["证书"]),
                    View({ class: "settings-dialog__panel-description" }, [
                      "代理服务当前加载并用于 HTTPS 解密的根证书。",
                    ]),
                  ]),
                  Button(
                    {
                      store: props.refresh_button,
                      class: "settings-dialog__refresh",
                      prefix: Timeless.Icon({ name: "refresh-cw", size: 14 }),
                      attributes: { "aria-label": "刷新证书信息" },
                    },
                    ["刷新"],
                  ),
                ]),
                Show({
                  when: props.loading,
                  ok() {
                    return View({ class: "settings-dialog__state" }, [
                      View({ class: "settings-dialog__state-spinner" }, [
                        Timeless.Icon({ name: "refresh-cw", size: 22 }),
                      ]),
                      View({ class: "settings-dialog__state-title" }, [
                        "正在读取证书",
                      ]),
                      View({ class: "settings-dialog__state-text" }, [
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
                              "settings-dialog__state settings-dialog__state--error",
                          },
                          [
                            Timeless.Icon({ name: "circle-alert", size: 28 }),
                            View({ class: "settings-dialog__state-title" }, [
                              "证书信息读取失败",
                            ]),
                            View({ class: "settings-dialog__state-text" }, [
                              props.error,
                            ]),
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
              return View({ class: "settings-dialog__panel-inner" }, [
                View({ class: "settings-dialog__panel-header" }, [
                  View({}, [
                    View({ class: "settings-dialog__panel-title" }, ["MCP"]),
                    View({ class: "settings-dialog__panel-description" }, [
                      "管理供本机或局域网 Agent 使用的工具服务。",
                    ]),
                  ]),
                  Button(
                    {
                      store: props.mcp_refresh_button,
                      class: "settings-dialog__refresh",
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
              return View({ class: "settings-dialog__panel-inner" }, [
                View({ class: "settings-dialog__panel-header" }, [
                  View({}, [
                    View({ class: "settings-dialog__panel-title" }, ["关于"]),
                    View({ class: "settings-dialog__panel-description" }, [
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
      class: "update-dialog",
      zIndex: 12000,
      showClose: false,
      attributes: { "aria-labelledby": "update-dialog-title" },
    },
    [
      DialogHeader({ class: "update-dialog__header" }, [
        View({ class: "update-dialog__mark" }, [
          Timeless.Icon({ name: "cloud-download", size: 23 }),
        ]),
        View({ class: "update-dialog__heading" }, [
          DialogTitle(
            { attributes: { id: "update-dialog-title" } },
            [model.state.phase_title],
          ),
          DialogDescription({}, [model.state.phase_message]),
        ]),
      ]),
      DialogBody({ class: "update-dialog__body" }, [
        View({ class: "update-dialog__versions" }, [
          View({ class: "update-dialog__version" }, [
            View({ class: "update-dialog__version-label" }, ["当前版本"]),
            View({ as: "code", class: "update-dialog__version-value" }, [
              Timeless.computed(snapshot_, function (snapshot) {
                return snapshot.current_version || "未知";
              }),
            ]),
          ]),
          View(
            {
              class: "update-dialog__version-arrow",
              attributes: { "aria-hidden": "true" },
            },
            ["→"],
          ),
          View({ class: "update-dialog__version" }, [
            View({ class: "update-dialog__version-label" }, ["最新版本"]),
            View({ as: "code", class: "update-dialog__version-value" }, [
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
            return View({ class: "update-dialog__meta" }, [
              Show({
                when: Timeless.computed(
                  model.state.published_text,
                  Boolean,
                ),
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
                class: "update-dialog__progress",
                attributes: {
                  role: "progressbar",
                  "aria-valuemin": "0",
                  "aria-valuemax": "100",
                  "aria-valuenow": model.state.percent,
                },
              },
              [
                View({ class: "update-dialog__progress-head" }, [
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
                View({ class: "update-dialog__progress-track" }, [
                  View({
                    class: Timeless.classNames([
                      "update-dialog__progress-value",
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
                          width: state.has_total
                            ? `${state.percent}%`
                            : "36%",
                        };
                      },
                    ),
                  }),
                ]),
                View({ class: "update-dialog__progress-detail" }, [
                  model.state.progress_text,
                ]),
              ],
            );
          },
        }),
        Show({
          when: Timeless.computed(model.state.status, function (status) {
            return status === "error";
          }),
          ok() {
            return View(
              {
                class: "update-dialog__error",
                attributes: { role: "alert" },
              },
              [
                Timeless.Icon({ name: "circle-alert", size: 17 }),
                model.state.phase_message,
              ],
            );
          },
        }),
        Show({
          when: Timeless.computed(snapshot_, function (snapshot) {
            return Boolean(snapshot.body);
          }),
          ok() {
            return View({ class: "update-dialog__notes" }, [
              View({ class: "update-dialog__notes-title" }, ["版本说明"]),
              View({ as: "pre", class: "update-dialog__notes-content" }, [
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
          return DialogFooter({ class: "update-dialog__footer" }, [
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
                    Timeless.computed(
                      model.state.status,
                      function (status) {
                        return status === "error" ? "重新下载" : "下载并更新";
                      },
                    ),
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
