const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function setup(filters = []) {
  const frontend = path.join(__dirname, "..");
  const storage = new Map([["content.saved_filters", JSON.stringify(filters)]]);
  const tips = [];
  const timers = new Map();
  let timer_id = 0;
  const context = vm.createContext({
    setTimeout(fn) { timers.set(++timer_id, fn); return timer_id; },
    clearTimeout(id) { timers.delete(id); },
    setInterval() {},
    clearInterval() {},
    localStorage: {
      getItem(key) { return storage.get(key) || null; },
      setItem(key, value) { storage.set(key, value); },
    },
  });
  context.window = context;
  const read = (file) => fs.readFileSync(path.join(frontend, file), "utf8");
  vm.runInContext(read("public/timeless/0.33.1/timeless.umd.min.js"), context);
  for (const name of ["ref", "refarr", "refobj", "computed", "combine"]) {
    context[name] = context.Timeless[name];
  }
  for (const file of ["src/utils.js", "src/biz/request.js", "src/image-proxy.model.js"]) {
    vm.runInContext(read(file).replace(/^export /gm, ""), context);
  }
  const requests = [];
  context.Timeless = { ...context.Timeless, kit: { ...context.Timeless.kit,
    RequestCore: class {
      constructor(request) { this.request = request; }
      async run(params) {
        const url = this.request(params).url.split("?")[0];
        requests.push({ url, params: JSON.parse(JSON.stringify(params)) });
        return { data: { list: url === "/api/account/list" ? [{ id: "account", nickname: "测试账号" }] : [],
          total: 96, page: params.page, page_size: params.page_size } };
      }
    },
  } };
  const source = read("src/pages/content.model.js").replace(/^import[\s\S]*?;\n/gm, "");
  vm.runInContext(source.slice(0, source.lastIndexOf("\nexport {")), context);
  const model = context.ContentViewModel({ client: {}, tip(tip) { tips.push(tip); } });
  const node = (props, children = []) => ({ props, children });
  for (const name of ["View", "Popover", "IconButton", "Input", "Button"]) context[name] = node;
  context.Timeless = { ...context.Timeless, Icon: node };
  const view = read("src/pages/content.js");
  vm.runInContext(view.slice(view.indexOf("function ContentSaveFilterPopover("),
    view.indexOf("function ContentSavedFilterPanel(")), context);
  const popover = context.ContentSaveFilterPopover({ store: model });
  return { model, popover, storage, tips, timers, requests };
}

test("hover opens the save controls and crossing into the popover preserves the edited name", () => {
  const { model, popover, timers } = setup();
  assert.equal(popover.children[0].children[0].props.name, "plus");
  popover.props.onTriggerMouseEnter();
  assert.equal(model.ui.save_filter_popover$.visible, true);
  assert.equal(model.ui.input_filter_name$.value, "筛选1");
  model.ui.input_filter_name$.setValue("自定义名称");
  popover.props.onTriggerMouseLeave();
  popover.props.onContentMouseEnter();
  for (const fn of [...timers.values()]) fn();
  assert.equal(model.ui.save_filter_popover$.visible, true);
  assert.equal(model.ui.input_filter_name$.value, "自定义名称");
  popover.props.onContentMouseLeave();
  for (const fn of [...timers.values()]) fn();
  assert.equal(model.ui.save_filter_popover$.visible, false);
  popover.props.onTriggerMouseLeave();
  const pending = timers.size;
  popover.props.onUnmounted();
  assert.equal(timers.size, pending - 1);
});

test("save rejects empty and whitespace names with a prompt and keeps existing filters", () => {
  const { model, popover, storage, tips } = setup([{ id: "existing", name: "已有筛选" }]);
  const save = popover.props.content[0].children[1];
  let focused = 0;
  model.ui.input_filter_name$.focus = () => { focused++; };
  for (const name of ["", "   "]) {
    popover.props.onTriggerMouseEnter();
    model.ui.input_filter_name$.setValue(name);
    save.props.onClick({ preventDefault() {} });
    assert.equal(model.ui.save_filter_popover$.visible, true);
    assert.equal(model.state.saved_filters.value.length, 1);
    assert.equal(JSON.parse(storage.get("content.saved_filters")).length, 1);
    assert.equal(tips.at(-1).msg[0], "请输入筛选器名称");
  }
  assert.equal(focused, 2);
});

test("saving uses the existing count for the default name and persists the current criteria", () => {
  const { model, popover, storage, tips } = setup([{ id: "existing", name: "已有筛选" }]);
  const save = popover.props.content[0].children[1];
  assert.equal(save.children[0], "保存");
  model.methods.setKeyword("测试关键词");
  popover.props.onTriggerMouseEnter();
  assert.equal(model.ui.input_filter_name$.value, "筛选2");
  save.props.onClick({ preventDefault() {} });
  assert.equal(model.ui.save_filter_popover$.visible, false);
  const saved = JSON.parse(storage.get("content.saved_filters"));
  assert.equal(saved.length, 2);
  assert.equal(saved[0].name, "筛选2");
  assert.equal(saved[0].keyword, "测试关键词");
  popover.props.onTriggerMouseEnter();
  assert.equal(model.ui.input_filter_name$.value, "筛选3");
  assert.equal(tips.length, 0);
});

test("default numbering skips an existing name after filters are deleted", () => {
  const { model, popover } = setup([{ id: "existing", name: "筛选2" }]);
  popover.props.onTriggerMouseEnter();
  assert.equal(model.ui.input_filter_name$.value, "筛选3");
  model.methods.saveFilter();
  assert.equal(model.state.saved_filters.value.length, 2);
  assert.equal(model.state.saved_filters.value[1].name, "筛选2");
});

test("clicking the active quick filter clears its criteria and reloads the first page", async () => {
  const filter = { id: "video", name: "视频", keyword: "关键词", content_type: "video",
    platform_id: "platform", account_id: "account", scope: "all" };
  const { model, requests, storage } = setup([filter]);
  await model.methods.applyFilter(filter);
  assert.equal(model.state.active_filter_id.value, filter.id);
  assert.equal(model.ui.input_keyword$.value, filter.keyword);
  assert.equal(model.state.account_id.value, filter.account_id);
  assert.equal(model.ui.checkbox_all$.checked, true);
  await model.methods.changePage(2);
  assert.equal(model.state.page.value, 2);

  await model.methods.applyFilter(filter);
  assert.equal(model.state.active_filter_id.value, "");
  assert.equal(model.ui.input_keyword$.value, "");
  assert.equal(model.ui.select_content_type$.value, "");
  assert.equal(model.ui.select_platform$.value, "");
  assert.equal(model.ui.select_account$.value, "");
  assert.equal(model.ui.checkbox_all$.checked, false);
  assert.equal(model.state.page.value, 1);
  assert.deepEqual(requests.at(-1), { url: "/api/content/list", params: { page: 1, page_size: 48, scope: "task" } });
  assert.equal(JSON.parse(storage.get("content.saved_filters")).length, 1);

  await model.methods.applyFilter(filter);
  assert.equal(model.state.active_filter_id.value, filter.id);
});

test("switching quick filters applies the new one and default criteria can also be deselected", async () => {
  const filters = [{ id: "default", name: "默认" }, { id: "video", name: "视频", content_type: "video" }];
  const { model } = setup(filters);
  assert.equal(model.state.active_filter_id.value, "");
  await model.methods.applyFilter(filters[1]);
  assert.equal(model.state.active_filter_id.value, "video");
  await model.methods.applyFilter(filters[0]);
  assert.equal(model.state.active_filter_id.value, "default");
  assert.equal(model.ui.select_content_type$.value, "");
  await model.methods.applyFilter(filters[0]);
  assert.equal(model.state.active_filter_id.value, "");
});
