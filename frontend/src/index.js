import {
  Button,
  Dialog,
  DialogBody,
  DialogTitle,
  createButtonStore,
} from "./components.js";

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

window.h = Timeless.h;
window.View = Timeless.View;
window.Fragment = Timeless.Fragment;
window.Img = Timeless.Img;
// Control flow
window.Show = Timeless.Show;
window.For = Timeless.For;
window.Switch = Timeless.Switch;
window.Match = Timeless.Match;
// Reactivity
window.ref = Timeless.ref;
window.refobj = Timeless.refobj;
window.refarr = Timeless.refarr;
window.computed = Timeless.computed;
window.combine = Timeless.combine;
window.isElement = Timeless.isElement;
// Styling
window.cn = Timeless.classNames;
window.classNames = Timeless.classNames;
// SVG helpers
window.SVG = Timeless.SVG;
window.Circle = Timeless.Circle;

window.PLATFORM_FAVICONS = Object.freeze({
  wxchannels:
    "https://res.wx.qq.com/t/wx_fed/finder/helper/finder-helper-web/res/favicon-v2.ico",
  wxmp: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
  officialaccount: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
  zhihu: "https://static.zhihu.com/heifetz/favicon.ico",
  douyin: "https://p-pc-weboff.byteimg.com/tos-cn-i-9r5gewecjs/favicon.png",
  youtube: "https://www.youtube.com/s/desktop/0084d708/img/favicon.ico",
  bilibili: "https://static.hdslb.com/images/favicon.ico",
});

(function (global) {
  "use strict";

  var css_records = Object.create(null);
  var load_records = Object.create(null);

  function versioned_url(resource_url) {
    var resource = new URL(resource_url, document.baseURI);
    var version = String(
      (window.__d_config && window.__d_config.version) || "",
    ).trim();
    if (version) {
      resource.searchParams.set("v", version);
    }
    return resource.href;
  }

  /**
   *
   * @param {string} module_url
   * @param {string | undefined} css_url
   * @returns {Promise<HTMLLinkElement | null>}
   */
  function load_style(module_url, css_url) {
    if (!css_url) {
      return Promise.resolve(null);
    }

    var style_href = versioned_url(css_url);
    var current_record = css_records[style_href];
    if (current_record && current_record.status === "loaded") {
      return Promise.resolve(current_record.link);
    }
    if (current_record && current_record.status === "loading") {
      return current_record.promise;
    }

    /** @type {undefined | Function} */
    var resolve_load;
    /** @type {undefined | Function} */
    var reject_load;
    var promise = new Promise(function (resolve, reject) {
      resolve_load = resolve;
      reject_load = reject;
    });
    var link = document.createElement("link");
    var record = {
      link: link,
      promise: promise,
      status: "loading",
      url: css_url,
    };

    css_records[style_href] = record;
    link.rel = "stylesheet";
    link.href = style_href;
    link.dataset.module = module_url;
    link.onload = function () {
      record.status = "loaded";
      resolve_load(link);
    };
    link.onerror = function () {
      record.status = "failed";
      reject_load(new Error("页面样式加载失败：" + css_url));
    };
    document.head.appendChild(link);

    return promise;
  }

  /**
   *
   * @param {string} module_url
   * @param {string | undefined} css_url
   * @returns {Promise<Function>}
   */
  function load(module_url, css_url) {
    var module_href = versioned_url(module_url);
    var current_record = load_records[module_href];
    if (current_record && current_record.status === "loaded") {
      return load_style(module_url, css_url).then(function () {
        return current_record.component;
      });
    }
    if (current_record && current_record.status === "loading") {
      return current_record.promise;
    }

    var record = {
      component: null,
      promise: null,
      status: "loading",
      css_url: css_url,
      url: module_href,
    };

    load_records[module_href] = record;
    record.promise = Promise.all([
      load_style(module_url, css_url),
      import(module_href),
    ])
      .then(function (results) {
        var page_module = results[1];
        var component = page_module && page_module.default;
        if (typeof component !== "function") {
          throw new TypeError(
            "页面模块必须 default export View 工厂函数：" + module_url,
          );
        }
        record.component = component;
        record.status = "loaded";
        return component;
      })
      .catch(function (error) {
        record.status = "failed";
        throw error;
      });

    return record.promise;
  }

  /**
   *
   * @param {string} module_url
   * @param {string | undefined} css_url
   * @returns {() => Promise<Function>}
   */
  function lazy(module_url, css_url) {
    return function () {
      return load(module_url, css_url);
    };
  }

  var routes_configure = {
    filehelper: {
      title: "微信文件传输助手",
      pathname: "/filehelper",
      component: lazy("src/pages/filehelper.js", "src/pages/filehelper.css"),
    },
    preview: {
      title: "预览",
      pathname: "/preview",
      component: lazy("src/pages/preview.js", "src/pages/preview.css"),
    },
    shell: {
      title: "首页",
      pathname: "/",
      component: SiderLayoutView,
      children: {
        download: {
          is_default: true,
          title: "下载",
          pathname: "/download",
          component: lazy("src/pages/downloadv2.js"),
        },
        scraper: {
          title: "内容抓取",
          pathname: "/scraper",
          component: lazy("src/pages/scraper.js", "src/pages/scraper.css"),
        },
        content: {
          title: "内容管理",
          pathname: "/content",
          component: lazy("src/pages/content.js"),
        },
        content_detail: {
          title: "内容详情",
          pathname: "/content/detail",
          component: lazy(
            "src/pages/content_detail.js",
            "src/pages/content_detail.css",
          ),
        },
        browsehistory: {
          title: "浏览记录",
          pathname: "/browsehistory",
          component: lazy("src/pages/browsehistory.js"),
        },
        account: {
          title: "帐号管理",
          pathname: "/account",
          component: lazy("src/pages/account.js", "src/pages/account.css"),
        },
        // logs: {
        //   title: "日志",
        //   pathname: "/logs",
        //   component: lazy("src/pages/logs.js"),
        // },
      },
    },
  };
  var router = Timeless.kit.buildRoutes(routes_configure);
  var router$ = new Timeless.kit.NavigatorCore();
  var root_view_model = new Timeless.kit.RouteViewCore({
    name: "root",
    pathname: "/",
    title: "ROOT",
    visible: true,
    parent: null,
    views: [],
  });
  root_view_model.isRoot = true;
  var storage$ = new Timeless.kit.StorageCore({
    key: "wx_channels_download",
    defaultValues: {
      theme: "system",
    },
    values: {},
    client: global.localStorage,
  });
  var http_client$ = new Timeless.kit.HttpClientCore({
    headers: {
      "Content-Type": "application/json",
    },
  });
  const socket_client$ = new Timeless.kit.SocketClientCore();
  var history$ = new Timeless.kit.HistoryCore({
    view: root_view_model,
    router: router$,
    routes: router.routes,
    views: {
      root: root_view_model,
    },
  });
  var app$ = new Timeless.kit.ApplicationModel({
    clipboard: Timeless.kit.ClipboardModel(),
    storage: storage$,
    async beforeReady() {
      var route = router.routesWithPathname[router$.pathname];
      var route_name = route ? route.name : router.defaultRouteName;
      history$.push(route_name, router$.query, { ignore: true });
      return Timeless.Result.Ok(null);
    },
  });

  Timeless.web.provide_http_client(http_client$);
  Timeless.web.provide_socket_client(socket_client$, {
    WebSocket: WebSocket,
    // debug: debug === true,
  });
  if (!global.dl$) {
    global.dl$ = DL({
      client: http_client$,
      socket_client: socket_client$,
    });
  }
  Timeless.web.provide_history(history$);
  Timeless.web.provide_app(app$);

  history$.onRouteChange(function (/** @type {any} */ event) {
    if (event.view && event.view.title) {
      app$.setTitle(event.view.title);
    }
    if (event.ignore) {
      return;
    }
    if (event.reason === "push") {
      router$.pushState(String(event.href));
    }
    if (event.reason === "replace") {
      router$.replaceState(String(event.href));
    }
  });

  function LoadingView() {
    return View(
      {
        class:
          "route-loading dm-page dm-grid dm-place-center dm-text-muted dm-p-8",
        role: "status",
      },
      ["页面加载中…"],
    );
  }

  /**
   *
   * @param {Error} error
   * @param {string} view_name
   * @returns
   */
  function ErrorFallbackView(error, view_name) {
    return View({ class: "route-error dm-page dm-grid dm-place-center dm-p-8" }, [
      View({ as: "strong" }, ["页面加载失败"]),
      View({ as: "span" }, [view_name || "未知页面"]),
      View({ as: "pre" }, [error.message]),
    ]);
  }

  var settings_request = Timeless.kit.request_factory({
    headers: { "Content-Type": "application/json" },
    process(response) {
      if (response.error) {
        return Timeless.Result.Err(response.error);
      }
      var payload = response.data || {};
      if (payload.code !== 0) {
        return Timeless.Result.Err(
          payload.msg || "获取证书信息失败",
          payload.code,
          payload.data,
        );
      }
      return Timeless.Result.Ok(payload.data || {});
    },
  });

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
    if (!value) {
      return "未提供";
    }
    var date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return String(value);
    }
    return new Intl.DateTimeFormat("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).format(date);
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
      return { label: "已安装，尚未受信任", tone: "warning", icon: "circle-alert" };
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
          return View({ class: "settings-certificate-notice settings-certificate-notice--danger" }, [
            Timeless.Icon({ name: "circle-alert", size: 16 }),
            Timeless.computed(certificate_, function (data) {
              return data.install_status_error;
            }),
          ]);
        },
      }),
      Show({
        when: Timeless.computed(certificate_, function (data) {
          return Array.isArray(data && data.risk_warnings) && data.risk_warnings.length > 0;
        }),
        ok() {
          return View({ class: "settings-certificate-notices" }, [
            For({
              each: Timeless.computed(certificate_, function (data) {
                return data.risk_warnings;
              }),
              render(warning) {
                return View({ class: "settings-certificate-notice settings-certificate-notice--warning" }, [
                  Timeless.Icon({ name: "circle-alert", size: 16 }),
                  warning,
                ]);
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
        View({ class: "settings-certificate-section__title" }, ["有效期与信任"]),
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
          return View({ class: "settings-certificate-notice settings-certificate-notice--danger" }, [
            Timeless.Icon({ name: "circle-alert", size: 16 }),
            Timeless.computed(certificate_, function (data) {
              return "证书内容解析失败：" + data.parse_error;
            }),
          ]);
        },
      }),
      Show({
        when: Timeless.computed(certificate_, function (data) {
          return Boolean(data && data.pem);
        }),
        ok() {
          return View({ as: "details", class: "settings-certificate-pem" }, [
            View({ as: "summary", class: "settings-certificate-pem__summary" }, [
              "查看 PEM 原文",
            ]),
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
        View(
          {
            as: "a",
            class: "settings-about__resource dm-focus-ring",
            attributes: {
              href: "https://github.com/ltaoo/wx_channels_download",
              target: "_blank",
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
        View(
          {
            as: "a",
            class: "settings-about__resource dm-focus-ring",
            attributes: {
              href: "https://ltaoo.github.io/wx_channels_download/guide/start.html",
              target: "_blank",
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

  function SettingsDialog(props) {
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
            View({ as: "span", class: "settings-dialog__heading-description" }, [
              "查看当前运行环境和安全配置",
            ]),
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
                    "aria-current": Timeless.computed(props.section, function (section) {
                      return section === "certificate" ? "page" : undefined;
                    }),
                  },
                },
                [
                  Timeless.Icon({ name: "file-lock", size: 17 }),
                  View({ as: "span" }, ["证书"]),
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
                    "aria-current": Timeless.computed(props.section, function (section) {
                      return section === "about" ? "page" : undefined;
                    }),
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
                          return View({ class: "settings-dialog__state settings-dialog__state--error" }, [
                            Timeless.Icon({ name: "circle-alert", size: 28 }),
                            View({ class: "settings-dialog__state-title" }, [
                              "证书信息读取失败",
                            ]),
                            View({ class: "settings-dialog__state-text" }, [props.error]),
                            Button(
                              {
                                store: props.retry_button,
                                prefix: Timeless.Icon({ name: "refresh-cw", size: 14 }),
                              },
                              ["重新读取"],
                            ),
                          ]);
                        },
                        else() {
                          return Show({
                            when: Timeless.computed(props.certificate, function (data) {
                              return Boolean(data);
                            }),
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

  function SiderLayoutView(props) {
    var menu$ = Timeless.kit.RouteMenusModel({
      view: props.view,
      history: history$,
      menus: [
        { title: "下载", name: "root.shell.download", icon: "download" },
        { title: "Get", name: "root.shell.scraper", icon: "search" },
        { title: "内容管理", name: "root.shell.content", icon: "library" },
        { title: "浏览记录", name: "root.shell.browsehistory", icon: "history" },
        { title: "帐号管理", name: "root.shell.account", icon: "user" },
        // { title: "日志", name: "root.shell.logs", icon: "scroll-text" },
      ],
    });
    var settings_dialog$ = new Timeless.vm.DialogCore({ closeable: true, footer: false });
    var settings_section_ = Timeless.ref("certificate");
    var certificate_ = Timeless.ref(null);
    var certificate_loading_ = Timeless.ref(false);
    var certificate_error_ = Timeless.ref("");
    var certificate_request = new Timeless.kit.RequestCore(
      function () {
        return settings_request.get("/api/proxy/certificate/status");
      },
      { client: http_client$ },
    );
    var certificate_request_sequence = 0;

    async function load_certificate() {
      var sequence = ++certificate_request_sequence;
      certificate_loading_.as(true);
      certificate_error_.as("");
      var result = await certificate_request.run();
      if (sequence !== certificate_request_sequence) {
        return result;
      }
      certificate_loading_.as(false);
      if (result.error) {
        certificate_error_.as(result.error.message || String(result.error));
        return result;
      }
      certificate_.as(result.data || {});
      return result;
    }

    var settings_button$ = createButtonStore({
      variant: "ghost",
      onClick() {
        settings_dialog$.show();
        settings_section_.as("certificate");
        load_certificate();
      },
    });
    var certificate_menu_button$ = createButtonStore({
      variant: "ghost",
      onClick() {
        settings_section_.as("certificate");
        if (!certificate_.value && !certificate_loading_.value) {
          load_certificate();
        }
      },
    });
    var about_menu_button$ = createButtonStore({
      variant: "ghost",
      onClick() {
        settings_section_.as("about");
      },
    });
    var refresh_certificate_button$ = createButtonStore({
      variant: "outline",
      size: "sm",
      loading: certificate_loading_,
      onClick: load_certificate,
    });
    var retry_certificate_button$ = createButtonStore({
      variant: "primary",
      loading: certificate_loading_,
      onClick: load_certificate,
    });

    return View(
      {
        class: "app-shell dm-page",
        onUnmounted: function () {
          certificate_request_sequence += 1;
          certificate_request.destroy?.();
          settings_dialog$.destroy();
          menu$.destroy();
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
          ]),
          View(
            {
              as: "nav",
              class: "app-menu dm-flex dm-flex-col dm-gap-1",
              ariaLabel: "页面导航",
            },
            [
              For({
                each: menu$.menus,
                render: function (menu) {
                  return Button(
                    {
                      store: createButtonStore({
                        variant: "ghost",
                        onClick() {
                          menu$.handleClick(menu);
                        },
                      }),
                      class: Timeless.classNames([
                        "app-menu__item",
                        "dm-focus-ring",
                        "dm-justify-start",
                        Timeless.computed(menu$.cur, function (current) {
                          return menu$.isSelected(current, menu)
                            ? "app-menu__item--active"
                            : null;
                        }),
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
              Button(
                {
                  store: settings_button$,
                  class:
                    "app-menu__item app-menu__settings dm-focus-ring dm-justify-start",
                  attributes: {
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
            class: "app-content dm-min-w-0 dm-overflow-auto",
          },
          [
            Timeless.ui.StandardSubViews({
              app: props.app,
              client: props.client,
              ErrorFallback: ErrorFallbackView,
              history: props.history,
              placeholder: [LoadingView()],
              storage: props.storage,
              view: props.view,
              views: props.views,
            }),
          ],
        ),
        SettingsDialog({
          dialog: settings_dialog$,
          section: settings_section_,
          version:
            String(
              (window.__d_config && window.__d_config.version) || "",
            ).trim() || "开发版",
          certificate: certificate_,
          loading: certificate_loading_,
          error: certificate_error_,
          certificate_menu_button: certificate_menu_button$,
          about_menu_button: about_menu_button$,
          refresh_button: refresh_certificate_button$,
          retry_button: retry_certificate_button$,
        }),
      ],
    );
  }

  function ApplicationRootView() {
    return View({ class: "application-root dm-page" }, [
      Timeless.ui.StandardSubViews({
        app: app$,
        client: http_client$,
        history: history$,
        storage: storage$,
        view: history$.$view,
        views: router.views,
        placeholder: [LoadingView()],
        ErrorFallback: ErrorFallbackView,
      }),
    ]);
  }

  function bootstrap() {
    var $root = document.querySelector("#root");
    if (!$root) {
      throw new Error("应用无法启动：缺少 App Model 或根节点");
    }
    router$.prepare(global.location);
    app$.start({
      width: global.innerWidth,
      height: global.innerHeight,
    });
    Timeless.DOM.render(ApplicationRootView(), $root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bootstrap, { once: true });
  } else {
    bootstrap();
  }
})(window);
