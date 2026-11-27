import { fetchScraperPlatforms, fetchScraperPlatform } from "@/biz/request.js";

export function PlatformsPageModel(props) {
  const platforms_ = refarr([]);
  const keyword_ = ref("");
  const loading_ = ref(false);
  const catalog_error_ = ref("");
  const selected_ = ref(null);
  const operation_ = ref("");
  const busy_ = ref(false);
  const results = Object.fromEntries(["list", "detail"].map((operation) => {
    const result = ref(null);
    return [operation, {
      result,
      error: ref(""),
      elapsed: ref(""),
      busy: combine({ busy: busy_, operation: operation_ }, (state) => state.busy && state.operation === operation),
      has_result: computed(result, (value) => value !== null),
      result_text: computed(result, (value) => value === null ? "" : JSON.stringify(value, null, 2)),
    }];
  }));
  let request_version = 0;
  let disposed = false;
  const catalog_request = new Timeless.kit.RequestCore(fetchScraperPlatforms, { client: props.client });
  const fetch_request = new Timeless.kit.RequestCore(fetchScraperPlatform, { client: props.client });

  const ui = {
    keyword$: new Timeless.vm.InputCore({ placeholder: "搜索平台名称", allowClear: true, onChange(value) { keyword_.as(String(value || "")); } }),
    url$: new Timeless.vm.InputCore({ placeholder: "粘贴内容链接", defaultValue: "" }),
    profile$: new Timeless.vm.InputCore({ placeholder: "主页链接、用户名或账号 ID", defaultValue: "" }),
    user_id$: new Timeless.vm.InputCore({ placeholder: "X 的数字用户 ID", defaultValue: "" }),
    scope$: new Timeless.vm.SelectCore({ placeholder: "选择列表范围", position: "popper", options: [] }),
    page$: new Timeless.vm.InputCore({ placeholder: "留空获取第一页", defaultValue: "" }),
    refresh$: new Timeless.vm.ButtonCore({ variant: "outline", onClick: load }),
    detail$: new Timeless.vm.ButtonCore({ variant: "primary", onClick() { return run("detail"); } }),
    list$: new Timeless.vm.ButtonCore({ variant: "primary", onClick() { return run("list"); } }),
    next$: new Timeless.vm.ButtonCore({ variant: "outline", onClick() { return run("list", true); } }),
    copy_list$: new Timeless.vm.ButtonCore({ variant: "outline", onClick() { return copy_result("list"); } }),
    copy_detail$: new Timeless.vm.ButtonCore({ variant: "outline", onClick() { return copy_result("detail"); } }),
  };

  function sync_buttons() {
    const platform = selected_.value;
    [[ui.detail$, !busy_.value && platform?.detail], [ui.list$, !busy_.value && platform?.list], [ui.next$, !busy_.value && results.list.result.value?.next_marker]].forEach(([button, enabled]) => {
      if (enabled) button.enable();
      else button.disable();
    });
    ui.detail$.setLoading(busy_.value && operation_.value === "detail");
    ui.list$.setLoading(busy_.value && operation_.value === "list");
  }

  function invalidate() {
    request_version += 1;
    busy_.as(false);
    Object.values(results).forEach((state) => {
      state.error.as("");
      state.result.as(null);
      state.elapsed.as("");
    });
    operation_.as("");
    sync_buttons();
  }

  function open(platform) {
    invalidate();
    selected_.as(platform);
    [ui.url$, ui.profile$, ui.user_id$, ui.page$].forEach((input) => input.setValue(""));
    ui.scope$.setOptions((platform.tabs || []).map((tab) => new Timeless.vm.SelectItemCore({ label: tab.name, value: tab.scope })));
    ui.scope$.setValue(platform.tabs?.[0]?.scope || "");
    sync_buttons();
    const query = Timeless.utils.qs_stringify({ ...props.view.query, platform: platform.id });
    props.history.$router.replaceState(`${props.view.pathname}?${query}`);
  }

  async function load() {
    loading_.as(true);
    catalog_error_.as("");
    ui.refresh$.setLoading(true);
    try {
      const response = await catalog_request.run();
      if (disposed) return;
      if (response.error) throw response.error;
      platforms_.value.forEach((item) => item.button$.destroy());
      platforms_.as((response.data?.list || []).map((platform) => ({
        ...platform,
        favicon: window.PLATFORM_FAVICONS?.[platform.id] || "public/platform-icons.svg#default",
        capabilities: platform.list ? "详情 / 列表" : "详情",
        button$: new Timeless.vm.ButtonCore({ variant: "ghost", onClick() { open(platform); } }),
      })), { reset: true });
      if (!selected_.value) {
        const platform = platforms_.value.find((item) => item.id === props.view.query.platform);
        if (platform) open(platform);
      }
    } catch (error) {
      if (!disposed) catalog_error_.as(error.message || String(error));
    } finally {
      loading_.as(false);
      ui.refresh$.setLoading(false);
    }
  }

  async function run(operation, next = false) {
    if (busy_.value || !["detail", "list"].includes(operation) || !selected_.value?.[operation]) return;
    const state = results[operation];
    const body = {
      platform: selected_.value.id,
      operation,
      url: String(ui.url$.value || "").trim(),
      profile: String(ui.profile$.value || "").trim(),
      user_id: String(ui.user_id$.value || "").trim(),
      scope: String(ui.scope$.value || ""),
      page: String(ui.page$.value || "").trim(),
    };
    if (body.operation === "detail" ? !body.url : !body.profile) {
      state.error.as(body.operation === "detail" ? "请填写内容链接" : "请填写主页链接或账号 ID");
      return;
    }
    if (next) {
      const marker = results.list.result.value?.next_marker;
      if (!marker) return;
      body.page = marker;
      ui.page$.setValue(marker);
    }
    const version = ++request_version;
    const started = Date.now();
    operation_.as(operation);
    busy_.as(true);
    state.error.as("");
    state.result.as(null);
    state.elapsed.as("");
    sync_buttons();
    try {
      const response = await fetch_request.run(body);
      if (version !== request_version || disposed) return;
      if (response.error) throw response.error;
      state.result.as(response.data);
    } catch (error) {
      if (version === request_version && !disposed) state.error.as(error.message || String(error));
    } finally {
      if (version === request_version && !disposed) {
        busy_.as(false);
        state.elapsed.as(`${((Date.now() - started) / 1000).toFixed(2)} 秒`);
        sync_buttons();
      }
    }
  }

  async function copy_result(operation) {
    try {
      await props.app.copy(JSON.stringify(results[operation].result.value, null, 2));
      props.tip({ type: "success", msg: "已复制结果" });
    } catch (error) {
      props.tip({ type: "error", msg: error.message || "复制失败" });
    }
  }

  return {
    ui,
    state: {
      loading: loading_, catalog_error: catalog_error_, selected: selected_, busy: busy_,
      list_result: results.list, detail_result: results.detail,
      title: computed(selected_, (platform) => platform?.name || "平台详情"),
      count: computed(platforms_, (items) => `${items.length} 个平台`),
      visible: combine({ platforms: platforms_, keyword: keyword_ }, ({ platforms, keyword }) => platforms.filter((item) => `${item.name} ${item.id} ${item.source}`.toLowerCase().includes(keyword.trim().toLowerCase()))),
      needs_user_id: computed(selected_, (selected) => selected?.id === "x"),
      supports_list: computed(selected_, (platform) => Boolean(platform?.list)),
      next_marker: computed(results.list.result, (result) => result?.next_marker || ""),
    },
    methods: {
      load, open, run,
      destroy() {
        disposed = true;
        request_version += 1;
        platforms_.value.forEach((item) => item.button$.destroy());
        Object.values(ui).forEach((control) => control.destroy());
      },
    },
  };
}
