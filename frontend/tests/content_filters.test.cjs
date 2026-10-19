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
  for (const file of ["src/utils.js", "src/biz/request.js"]) {
    vm.runInContext(read(file).replace(/^export /gm, ""), context);
  }
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
  return { model, popover, storage, tips, timers };
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
