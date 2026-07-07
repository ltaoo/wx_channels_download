/* global Fragment, NumberInput, Switch, Textarea */
import {
  disableSystemProxy,
  enableSystemProxy,
  fetchProxyStatus,
  generateProxyCertificate,
  installProxyCertificate,
  restartProxyService,
  startService,
  stopService,
  uninstallProxyCertificate,
  updateProxyConfig,
} from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";

const cn = Timeless.classNames;

function asText(value, fallback = "-") {
  if (value === undefined || value === null) return fallback;
  const text = String(value).trim();
  return text || fallback;
}

function boolText(value) {
  return value ? "开启" : "关闭";
}

function serviceStatusText(status) {
  if (status === "running") return "运行中";
  if (status === "starting") return "启动中";
  if (status === "stopping") return "停止中";
  if (status === "error") return "异常";
  if (status === "stopped") return "已停止";
  return "未知";
}

function toneClass(tone) {
  if (tone === "success") {
    return "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300";
  }
  if (tone === "warning") {
    return "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300";
  }
  if (tone === "danger") {
    return "border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300";
  }
  return "border-zinc-200 bg-zinc-50 text-zinc-600 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-300";
}

function isRefLike(value) {
  return value && typeof value === "object" && "value" in value;
}

function displayValue(value) {
  if (isRefLike(value)) {
    return computed(value, (v) => asText(v));
  }
  return asText(value);
}

function displayTitle(value) {
  if (isRefLike(value)) {
    return computed(value, (v) => String(v || ""));
  }
  return String(value || "");
}

function mappedClass(value, mapper) {
  if (isRefLike(value)) {
    return computed(value, mapper);
  }
  return mapper(value);
}

function statusTone(status) {
  if (status === "running") return "success";
  if (status === "starting" || status === "stopping") return "warning";
  if (status === "error") return "danger";
  return "neutral";
}

function Pill(props, children) {
  return View(
    {
      dataset: { t: "proxy-page-pill" },
      class: mappedClass(props.tone, (tone) =>
        cn([
          "inline-flex h-7 items-center gap-1.5 rounded-md border px-2.5 text-xs font-medium",
          toneClass(tone),
          props.class,
        ]),
      ),
    },
    children,
  );
}

function SectionPanel(props, children) {
  return View(
    {
      dataset: { t: "proxy-page-section-panel" },
      class:
        "rounded-lg border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
    },
    [
      View(
        {
          dataset: { t: "proxy-page-section-panel-header" },
          class: "mb-5 flex flex-wrap items-start justify-between gap-3",
        },
        [
          View({ dataset: { t: "proxy-page-section-panel-title-row" }, class: "flex min-w-0 items-center gap-3" }, [
            View(
              {
                dataset: { t: "proxy-page-section-panel-icon" },
                class:
                  "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-zinc-100 text-zinc-700 dark:bg-zinc-900 dark:text-zinc-200",
              },
              [Icon({ name: props.icon, size: 18 })],
            ),
            View({ dataset: { t: "proxy-page-section-panel-title-text" }, class: "min-w-0" }, [
              View(
                {
                  dataset: { t: "proxy-page-section-panel-title" },
                  class:
                    "truncate text-base font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [props.title],
              ),
              props.subtitle
                ? View(
                    {
                      dataset: { t: "proxy-page-section-panel-subtitle" },
                      class:
                        "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                    },
                    [props.subtitle],
                  )
                : null,
            ]),
          ]),
          props.actions
            ? View({ dataset: { t: "proxy-page-section-panel-actions" }, class: "flex flex-wrap gap-2" }, props.actions)
            : null,
        ],
      ),
      Fragment({}, children),
    ],
  );
}

function InfoItem(label, value, icon, tone = "neutral") {
  return View(
    {
      dataset: { t: "proxy-page-info-item" },
      class:
        "min-w-0 rounded-md border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-900/70",
    },
    [
      View(
        {
          dataset: { t: "proxy-page-info-item-label" },
          class: "flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400",
        },
        [Icon({ name: icon, size: 13 }), label],
      ),
      View(
        {
          dataset: { t: "proxy-page-info-item-value" },
          class: mappedClass(tone, (t) =>
            cn([
              "mt-1 truncate text-sm font-medium",
              t === "success"
                ? "text-emerald-700 dark:text-emerald-300"
                : t === "danger"
                  ? "text-red-700 dark:text-red-300"
                  : "text-zinc-900 dark:text-zinc-100",
            ]),
          ),
          title: displayTitle(value),
        },
        [displayValue(value)],
      ),
    ],
  );
}

function FieldBlock(label, control, hint) {
  return View({ dataset: { t: "proxy-page-field-block" }, class: "min-w-0 space-y-2" }, [
    Label({ class: "block text-xs font-medium text-zinc-600 dark:text-zinc-300" }, [
      label,
    ]),
    control,
    hint
      ? View(
          {
            dataset: { t: "proxy-page-field-hint" },
            class: "text-xs leading-5 text-zinc-500 dark:text-zinc-400",
          },
          [hint],
        )
      : null,
  ]);
}

function SwitchField(label, switchStore, value_, icon) {
  return View(
    {
      dataset: { t: "proxy-page-switch-field" },
      class:
        "flex items-center justify-between gap-3 rounded-md border border-zinc-200 bg-zinc-50 px-3 py-2.5 dark:border-zinc-800 dark:bg-zinc-900/70",
    },
    [
      View({ dataset: { t: "proxy-page-switch-field-label-row" }, class: "flex min-w-0 items-center gap-2" }, [
        Icon({ name: icon, size: 15 }),
        View({ dataset: { t: "proxy-page-switch-field-label" }, class: "truncate text-sm text-zinc-800 dark:text-zinc-100" }, [
          label,
        ]),
        View(
          {
            dataset: { t: "proxy-page-switch-field-value" },
            class: "text-xs text-zinc-500 dark:text-zinc-400",
          },
          [computed(value_, boolText)],
        ),
      ]),
      Switch({ store: switchStore }),
    ],
  );
}

function ProxyPageModel(props) {
  const reqs = {
    status: new Timeless.RequestCore(fetchProxyStatus, { client: api_client$ }),
    update: new Timeless.RequestCore(updateProxyConfig, { client: api_client$ }),
    restart: new Timeless.RequestCore(restartProxyService, { client: api_client$ }),
    start: new Timeless.RequestCore(startService, { client: api_client$ }),
    stop: new Timeless.RequestCore(stopService, { client: api_client$ }),
    enableSystem: new Timeless.RequestCore(enableSystemProxy, { client: api_client$ }),
    disableSystem: new Timeless.RequestCore(disableSystemProxy, { client: api_client$ }),
    generateCert: new Timeless.RequestCore(generateProxyCertificate, { client: api_client$ }),
    installCert: new Timeless.RequestCore(installProxyCertificate, { client: api_client$ }),
    uninstallCert: new Timeless.RequestCore(uninstallProxyCertificate, { client: api_client$ }),
  };

  const state = {
    status: ref({}),
    loading: ref(false),
    busy: ref(false),
    error: ref(""),
    showPEM: ref(false),
    form: {
      hostname: ref("127.0.0.1"),
      port: ref(2023),
      system: ref(true),
      tun: ref(false),
      upstreamProxy: ref(""),
      defaultInterface: ref(""),
      skipInstallRootCert: ref(false),
      tcpRelayEnabled: ref(false),
      tcpRelayHostname: ref("127.0.0.1"),
      tcpRelayPort: ref(9900),
      certName: ref("wx_channels_download"),
      certYears: ref(10),
    },
  };

  const ui = {
    scroll: new Timeless.ui.ScrollViewCore({}),
    hostname: new Timeless.ui.InputCore({
      defaultValue: "127.0.0.1",
      placeholder: "127.0.0.1",
      onChange(value) {
        state.form.hostname.as(String(value || ""));
      },
    }),
    port: new Timeless.ui.NumberInputCore({
      defaultValue: 2023,
      min: 1,
      max: 65535,
      step: 1,
      precision: 0,
      onChange(value) {
        state.form.port.as(Number(value) || 0);
      },
    }),
    upstreamProxy: new Timeless.ui.InputCore({
      defaultValue: "",
      placeholder: "http://127.0.0.1:7890",
      onChange(value) {
        state.form.upstreamProxy.as(String(value || ""));
      },
    }),
    defaultInterface: new Timeless.ui.InputCore({
      defaultValue: "",
      placeholder: "auto",
      onChange(value) {
        state.form.defaultInterface.as(String(value || ""));
      },
    }),
    tcpRelayHostname: new Timeless.ui.InputCore({
      defaultValue: "127.0.0.1",
      placeholder: "127.0.0.1",
      onChange(value) {
        state.form.tcpRelayHostname.as(String(value || ""));
      },
    }),
    tcpRelayPort: new Timeless.ui.NumberInputCore({
      defaultValue: 9900,
      min: 1,
      max: 65535,
      step: 1,
      precision: 0,
      onChange(value) {
        state.form.tcpRelayPort.as(Number(value) || 0);
      },
    }),
    certName: new Timeless.ui.InputCore({
      defaultValue: "wx_channels_download",
      placeholder: "wx_channels_download",
      onChange(value) {
        state.form.certName.as(String(value || ""));
      },
    }),
    certYears: new Timeless.ui.NumberInputCore({
      defaultValue: 10,
      min: 1,
      max: 30,
      step: 1,
      precision: 0,
      onChange(value) {
        state.form.certYears.as(Number(value) || 10);
      },
    }),
    systemSwitch: Timeless.ui.SwitchCore({ defaultValue: true }),
    tunSwitch: Timeless.ui.SwitchCore({ defaultValue: false }),
    skipCertSwitch: Timeless.ui.SwitchCore({ defaultValue: false }),
    tcpRelaySwitch: Timeless.ui.SwitchCore({ defaultValue: false }),
  };

  ui.systemSwitch.onChange((value) => state.form.system.as(!!value));
  ui.tunSwitch.onChange((value) => state.form.tun.as(!!value));
  ui.skipCertSwitch.onChange((value) => state.form.skipInstallRootCert.as(!!value));
  ui.tcpRelaySwitch.onChange((value) => state.form.tcpRelayEnabled.as(!!value));

  function setInput(input, value) {
    input.setValue(value, { silence: true });
  }

  function syncForm(data) {
    const cfg = data?.config || {};
    const relay = cfg.tcp_relay || {};
    const cert = cfg.cert || data?.certificate?.configured || {};
    state.form.hostname.as(cfg.hostname || "127.0.0.1");
    state.form.port.as(Number(cfg.port) || 2023);
    state.form.system.as(!!cfg.system);
    state.form.tun.as(!!cfg.tun);
    state.form.upstreamProxy.as(cfg.upstream_proxy || "");
    state.form.defaultInterface.as(cfg.default_interface || "");
    state.form.skipInstallRootCert.as(!!cfg.skip_install_root_cert);
    state.form.tcpRelayEnabled.as(!!relay.enabled);
    state.form.tcpRelayHostname.as(relay.hostname || "127.0.0.1");
    state.form.tcpRelayPort.as(Number(relay.port) || 9900);
    state.form.certName.as(cert.name || data?.certificate?.name || "wx_channels_download");

    setInput(ui.hostname, state.form.hostname.value);
    ui.port.setValue(state.form.port.value, { silence: true });
    ui.systemSwitch.setValue(state.form.system.value);
    ui.tunSwitch.setValue(state.form.tun.value);
    setInput(ui.upstreamProxy, state.form.upstreamProxy.value);
    setInput(ui.defaultInterface, state.form.defaultInterface.value);
    ui.skipCertSwitch.setValue(state.form.skipInstallRootCert.value);
    ui.tcpRelaySwitch.setValue(state.form.tcpRelayEnabled.value);
    setInput(ui.tcpRelayHostname, state.form.tcpRelayHostname.value);
    ui.tcpRelayPort.setValue(state.form.tcpRelayPort.value, { silence: true });
    setInput(ui.certName, state.form.certName.value);
  }

  function buildValues() {
    return {
      "proxy.hostname": state.form.hostname.value.trim() || "127.0.0.1",
      "proxy.port": Number(state.form.port.value) || 2023,
      "proxy.system": !!state.form.system.value,
      "proxy.tun": !!state.form.tun.value,
      "proxy.defaultInterface": state.form.defaultInterface.value.trim(),
      "proxy.upstreamProxy": state.form.upstreamProxy.value.trim(),
      "proxy.skipInstallRootCert": !!state.form.skipInstallRootCert.value,
      "proxy.tcpRelay.enabled": !!state.form.tcpRelayEnabled.value,
      "proxy.tcpRelay.hostname": state.form.tcpRelayHostname.value.trim() || "127.0.0.1",
      "proxy.tcpRelay.port": Number(state.form.tcpRelayPort.value) || 9900,
    };
  }

  async function run(action, successText, syncStatus = true) {
    state.busy.as(true);
    state.error.as("");
    const r = await action();
    state.busy.as(false);
    if (r.error) {
      const msg = r.error.message || String(r.error);
      state.error.as(msg);
      props.app.tip?.({ type: "error", text: [msg] });
      return null;
    }
    if (syncStatus && r.data) {
      state.status.as(r.data);
      syncForm(r.data);
    }
    if (successText) {
      props.app.tip?.({ type: "success", text: [successText] });
    }
    return r.data || {};
  }

  const methods = {
    async load() {
      state.loading.as(true);
      state.error.as("");
      const r = await reqs.status.run();
      state.loading.as(false);
      if (r.error) {
        const msg = r.error.message || String(r.error);
        state.error.as(msg);
        return;
      }
      state.status.as(r.data || {});
      syncForm(r.data || {});
    },
    save(restart = false) {
      return run(
        () => reqs.update.run({ values: buildValues(), restart }),
        restart ? "代理配置已保存并重启" : "代理配置已保存",
      );
    },
    restart() {
      return run(() => reqs.restart.run(), "代理服务已重启");
    },
    start() {
      return run(
        () => reqs.start.run({ name: "interceptor" }),
        "代理服务已启动",
        false,
      ).then(() => methods.load());
    },
    stop() {
      return run(
        () => reqs.stop.run({ name: "interceptor" }),
        "代理服务已停止",
        false,
      ).then(() => methods.load());
    },
    enableSystemProxy() {
      return run(() => reqs.enableSystem.run(), "系统代理已设置");
    },
    disableSystemProxy() {
      return run(() => reqs.disableSystem.run(), "系统代理已取消");
    },
    installCert() {
      return run(() => reqs.installCert.run(), "根证书已安装/信任");
    },
    uninstallCert() {
      return run(() => reqs.uninstallCert.run(), "根证书已移除");
    },
    generateCert(install) {
      return run(
        () =>
          reqs.generateCert.run({
            name: state.form.certName.value.trim(),
            valid_years: Number(state.form.certYears.value) || 10,
            install: !!install,
            restart: false,
          }),
        install ? "根证书已生成并安装" : "根证书已生成",
      );
    },
    togglePEM() {
      state.showPEM.as(!state.showPEM.value);
    },
    downloadPEM() {
      const base = String(api_client$.hostname || "").replace(/\/$/, "");
      window.open(`${base}/api/proxy/certificate/pem`, "_blank");
    },
  };

  return { state, ui, methods };
}

function ServicePanel(vm$) {
  const service_ = computed(vm$.state.status, (s) => s.service || {});
  const status_ = computed(service_, (s) => s.status || "unknown");
  return SectionPanel(
    {
      title: "代理服务",
      subtitle: computed(service_, (s) => s.addr || "-"),
      icon: "radio-tower",
      actions: [
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              variant: "outline",
              onClick() {
                vm$.methods.load();
              },
            }),
            disabled: vm$.state.busy,
          },
          [Icon({ name: "refresh-cw", size: 15 }), "刷新"],
        ),
      ],
    },
    [
      View({ dataset: { t: "proxy-page-service-summary-grid" }, class: "grid gap-3 md:grid-cols-3" }, [
        InfoItem("状态", computed(status_, serviceStatusText), "activity", computed(status_, statusTone)),
        InfoItem("监听", computed(service_, (s) => (s.listening ? "端口可用" : "未监听")), "network", computed(service_, (s) => (s.listening ? "success" : "danger"))),
        InfoItem("地址", computed(service_, (s) => s.addr || "-"), "link"),
      ]),
      View({ dataset: { t: "proxy-page-service-actions" }, class: "mt-4 flex flex-wrap gap-2" }, [
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              onClick() {
                vm$.methods.start();
              },
            }),
            disabled: vm$.state.busy,
          },
          [Icon({ name: "play", size: 15 }), "启动"],
        ),
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              variant: "outline",
              onClick() {
                vm$.methods.stop();
              },
            }),
            disabled: vm$.state.busy,
          },
          [Icon({ name: "square", size: 15 }), "停止"],
        ),
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              variant: "secondary",
              onClick() {
                vm$.methods.restart();
              },
            }),
            disabled: vm$.state.busy,
          },
          [Icon({ name: "rotate-cw", size: 15 }), "重启代理"],
        ),
      ]),
    ],
  );
}

function ConfigPanel(vm$) {
  return SectionPanel(
    {
      title: "端口与模式",
      subtitle: "HTTP/HTTPS 代理、TUN、TCP Relay",
      icon: "sliders-horizontal",
      actions: [
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              variant: "outline",
              onClick() {
                vm$.methods.save(false);
              },
            }),
            disabled: vm$.state.busy,
          },
          [Icon({ name: "save", size: 15 }), "保存"],
        ),
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              onClick() {
                vm$.methods.save(true);
              },
            }),
            disabled: vm$.state.busy,
          },
          [Icon({ name: "rotate-cw", size: 15 }), "保存并重启"],
        ),
      ],
    },
    [
      View({ dataset: { t: "proxy-page-config-form-grid" }, class: "grid gap-4 lg:grid-cols-2" }, [
        FieldBlock("代理主机", Input({ store: vm$.ui.hostname })),
        FieldBlock("代理端口", NumberInput({ store: vm$.ui.port })),
        FieldBlock("上游代理", Input({ store: vm$.ui.upstreamProxy })),
        FieldBlock("TUN 默认网卡", Input({ store: vm$.ui.defaultInterface })),
      ]),
      View({ dataset: { t: "proxy-page-config-switches" }, class: "mt-4 grid gap-3 lg:grid-cols-3" }, [
        SwitchField("随服务设置系统代理", vm$.ui.systemSwitch, vm$.state.form.system, "power"),
        SwitchField("TUN 模式", vm$.ui.tunSwitch, vm$.state.form.tun, "route"),
        SwitchField("跳过根证书安装", vm$.ui.skipCertSwitch, vm$.state.form.skipInstallRootCert, "shield-off"),
      ]),
      View(
        {
          dataset: { t: "proxy-page-tcp-relay-config" },
          class: "mt-5 border-t border-zinc-100 pt-5 dark:border-zinc-900",
        },
        [
          View({ dataset: { t: "proxy-page-tcp-relay-row" }, class: "grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,160px)]" }, [
            SwitchField("TCP Relay", vm$.ui.tcpRelaySwitch, vm$.state.form.tcpRelayEnabled, "cable"),
            FieldBlock("Relay 主机", Input({ store: vm$.ui.tcpRelayHostname })),
            FieldBlock("Relay 端口", NumberInput({ store: vm$.ui.tcpRelayPort })),
          ]),
        ],
      ),
    ],
  );
}

function SystemProxyPanel(vm$) {
  const system_ = computed(vm$.state.status, (s) => s.system_proxy || {});
  return SectionPanel(
    {
      title: "系统代理",
      subtitle: computed(system_, (s) => {
        const cur = s.current;
        return cur ? `${cur.hostname}:${cur.port}` : "未设置";
      }),
      icon: "monitor-cog",
    },
    [
      View({ dataset: { t: "proxy-page-system-proxy-grid" }, class: "grid gap-3 md:grid-cols-3" }, [
        InfoItem("配置", computed(system_, (s) => boolText(s.configured)), "settings"),
        InfoItem("当前", computed(system_, (s) => (s.enabled ? "已设置" : "未设置")), "power", computed(system_, (s) => (s.enabled ? "success" : "neutral"))),
        InfoItem("匹配", computed(system_, (s) => (s.matched ? "匹配代理端口" : "不匹配")), "check-circle", computed(system_, (s) => (s.matched ? "success" : "warning"))),
      ]),
      Show({
        when: computed(system_, (s) => !!s.error),
        ok() {
          return View(
            {
              dataset: { t: "proxy-page-system-proxy-error" },
              class:
                "mt-3 rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300",
            },
            [computed(system_, (s) => s.error || "")],
          );
        },
      }),
      View({ dataset: { t: "proxy-page-system-proxy-actions" }, class: "mt-4 flex flex-wrap gap-2" }, [
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              onClick() {
                vm$.methods.enableSystemProxy();
              },
            }),
            disabled: vm$.state.busy,
          },
          [Icon({ name: "power", size: 15 }), "设置系统代理"],
        ),
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              variant: "outline",
              onClick() {
                vm$.methods.disableSystemProxy();
              },
            }),
            disabled: vm$.state.busy,
          },
          [Icon({ name: "power-off", size: 15 }), "取消系统代理"],
        ),
      ]),
    ],
  );
}

function CertificatePanel(vm$) {
  const cert_ = computed(vm$.state.status, (s) => s.certificate || {});
  const detail_ = computed(cert_, (c) => c.detail || {});
  return SectionPanel(
    {
      title: "证书管理",
      subtitle: computed(cert_, (c) => c.name || "-"),
      icon: "shield-check",
      actions: [
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              variant: "outline",
              onClick() {
                vm$.methods.togglePEM();
              },
            }),
            disabled: vm$.state.busy,
          },
          [Icon({ name: "eye", size: 15 }), "查看 PEM"],
        ),
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              variant: "outline",
              onClick() {
                vm$.methods.downloadPEM();
              },
            }),
          },
          [Icon({ name: "download", size: 15 }), "下载"],
        ),
      ],
    },
    [
      View({ dataset: { t: "proxy-page-certificate-summary-grid" }, class: "grid gap-3 md:grid-cols-3" }, [
        InfoItem("信任状态", computed(cert_, (c) => (c.installed ? "已安装/信任" : "未安装")), "badge-check", computed(cert_, (c) => (c.installed ? "success" : "warning"))),
        InfoItem("有效期至", computed(detail_, (d) => asText(d.not_after)), "calendar"),
        InfoItem("根 CA", computed(detail_, (d) => boolText(d.is_ca)), "shield"),
      ]),
      View({ dataset: { t: "proxy-page-certificate-detail-grid" }, class: "mt-3 grid gap-3 md:grid-cols-2" }, [
        InfoItem("Subject CN", computed(detail_, (d) => d.subject_common_name || "-"), "user-round"),
        InfoItem("Issuer CN", computed(detail_, (d) => d.issuer_common_name || "-"), "stamp"),
        InfoItem("序列号", computed(detail_, (d) => d.serial_number || "-"), "hash"),
        InfoItem("SHA256", computed(detail_, (d) => d.fingerprint_sha256 || "-"), "fingerprint"),
      ]),
      Show({
        when: computed(cert_, (c) => !!c.install_status_error || !!c.parse_error),
        ok() {
          return View(
            {
              dataset: { t: "proxy-page-certificate-error" },
              class:
                "mt-3 rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300",
            },
            [
              computed(
                cert_,
                (c) => c.install_status_error || c.parse_error || "",
              ),
            ],
          );
        },
      }),
      View(
        {
          dataset: { t: "proxy-page-certificate-actions-and-generate" },
          class: "mt-5 border-t border-zinc-100 pt-5 dark:border-zinc-900",
        },
        [
          View({ dataset: { t: "proxy-page-certificate-generate-form" }, class: "grid gap-4 lg:grid-cols-[minmax(0,1fr)_160px]" }, [
            FieldBlock("证书名称", Input({ store: vm$.ui.certName })),
            FieldBlock("有效年限", NumberInput({ store: vm$.ui.certYears })),
          ]),
          View({ dataset: { t: "proxy-page-certificate-action-buttons" }, class: "mt-4 flex flex-wrap gap-2" }, [
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  onClick() {
                    vm$.methods.generateCert(false);
                  },
                }),
                disabled: vm$.state.busy,
              },
              [Icon({ name: "key-round", size: 15 }), "生成"],
            ),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  onClick() {
                    vm$.methods.generateCert(true);
                  },
                }),
                disabled: vm$.state.busy,
              },
              [Icon({ name: "badge-check", size: 15 }), "生成并安装"],
            ),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "secondary",
                  onClick() {
                    vm$.methods.installCert();
                  },
                }),
                disabled: vm$.state.busy,
              },
              [Icon({ name: "badge-check", size: 15 }), "安装/信任"],
            ),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  onClick() {
                    vm$.methods.uninstallCert();
                  },
                }),
                disabled: vm$.state.busy,
              },
              [Icon({ name: "trash-2", size: 15 }), "移除"],
            ),
          ]),
        ],
      ),
      Show({
        when: vm$.state.showPEM,
        ok() {
          return View(
            {
              dataset: { t: "proxy-page-certificate-pem-view" },
              class: "mt-4",
            },
            [
              Textarea({
                store: new Timeless.ui.InputCore({
                  defaultValue: cert_.value?.pem || "",
                  disabled: true,
                }),
                class: "min-h-56 font-mono text-xs",
              }),
            ],
          );
        },
      }),
    ],
  );
}

export default function ProxyPageView(props) {
  const vm$ = ProxyPageModel(props);
  return View(
    {
      dataset: { t: "proxy-page-root" },
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.methods.load();
      },
    },
    [
      View(
        {
          dataset: { t: "proxy-page-header" },
          class:
            "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950",
        },
        [
          View({ dataset: { t: "proxy-page-header-row" }, class: "flex flex-wrap items-center justify-between gap-3" }, [
            View({ dataset: { t: "proxy-page-header-title-block" }, class: "min-w-0" }, [
              View(
                {
                  dataset: { t: "proxy-page-title" },
                  class:
                    "text-2xl font-semibold text-zinc-950 dark:text-zinc-50",
                },
                ["代理配置"],
              ),
              View(
                {
                  dataset: { t: "proxy-page-subtitle" },
                  class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400",
                },
                ["端口、系统代理、TUN 和根证书"],
              ),
            ]),
            View({ dataset: { t: "proxy-page-header-actions" }, class: "flex flex-wrap items-center gap-2" }, [
              Pill(
                {
                  tone: computed(vm$.state.status, (s) =>
                    statusTone(s.service?.status),
                  ),
                },
                [
                  Icon({ name: "activity", size: 13 }),
                  computed(vm$.state.status, (s) =>
                    serviceStatusText(s.service?.status),
                  ),
                ],
              ),
              Button(
                {
                  store: new Timeless.ui.ButtonCore({
                    variant: "outline",
                    onClick() {
                      vm$.methods.load();
                    },
                  }),
                  disabled: vm$.state.loading,
                },
                [Icon({ name: "refresh-cw", size: 15 }), "刷新"],
              ),
            ]),
          ]),
        ],
      ),
      ScrollView({ store: vm$.ui.scroll, class: "flex-1" }, [
        View({ dataset: { t: "proxy-page-content" }, class: "p-6" }, [
          Show({
            when: vm$.state.error,
            ok() {
              return View(
                {
                  dataset: { t: "proxy-page-error" },
                  class:
                    "mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
                },
                [vm$.state.error],
              );
            },
          }),
          Show({
            when: vm$.state.loading,
            ok() {
              return View(
                {
                  dataset: { t: "proxy-page-loading" },
                  class:
                    "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500",
                },
                [Icon({ name: "loader", size: 28 }), "加载中..."],
              );
            },
            else() {
              return View(
                {
                  dataset: { t: "proxy-page-grid" },
                  class: "grid gap-5 xl:grid-cols-2",
                },
                [
                  ServicePanel(vm$),
                  SystemProxyPanel(vm$),
                  ConfigPanel(vm$),
                  CertificatePanel(vm$),
                ],
              );
            },
          }),
        ]),
      ]),
    ],
  );
}
