const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function setup() {
  const frontend = path.join(__dirname, "..");
  const context = vm.createContext({ setTimeout, clearTimeout, setInterval: () => 0 });
  context.window = context;
  vm.runInContext(fs.readFileSync(path.join(frontend, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
  const source = fs.readFileSync(path.join(frontend, "src/components/index.js"), "utf8");
  vm.runInContext(`const Runtime = Timeless; ${source.slice(
    source.indexOf("function AccountSelectListModel("),
    source.indexOf("function EntitySelect("),
  )}`, context);
  const entries = Array.from({ length: 1000 }, (_, index) =>
    new context.Timeless.vm.SelectItemCore({ label: `账号 ${index}`, value: String(index) }),
  );
  const store = new context.Timeless.vm.SelectCore({ options: entries });
  const options = context.Timeless.ref(entries);
  return { context, entries, store, options, model: context.AccountSelectListModel(store, options) };
}

test("keyboard navigation scrolls offscreen accounts into view and wraps", () => {
  const { entries, store, model } = setup();
  assert.equal(model.state.height_.value, "288px");
  model.methods.on_scroll({ scrollTop: 0, clientHeight: 288 });
  store.focusOption(entries[5].value);
  model.methods.move_focus(1);
  assert.equal(store.focused_item$, entries[6]);
  assert.equal(model.state.scroll_request_.value.top, 48);
  model.methods.on_scroll({ scrollTop: 48, clientHeight: 288 });
  store.focusOption(entries[0].value);
  model.methods.move_focus(-1);
  assert.equal(store.focused_item$, entries[999]);
  assert.equal(model.state.scroll_request_.value.top, 47712);
  model.methods.move_focus(1);
  assert.equal(store.focused_item$, entries[0]);
  assert.equal(model.state.scroll_request_.value.top, 0);
  model.methods.destroy();
});

test("filtering resets scrolling and keyboard selection stays within enabled results", () => {
  const { entries, store, options, model } = setup();
  model.methods.on_scroll({ scrollTop: 10000, clientHeight: 144 });
  store.focusOption(entries[500].value);
  entries[20].disabled = true;
  options.as([entries[10], entries[20], entries[30]]);
  assert.equal(model.state.scroll_request_.value.top, 0);
  assert.equal(model.state.height_.value, "144px");
  model.methods.select_focused();
  assert.equal(store.value, undefined);
  model.methods.move_focus(-1);
  assert.equal(store.focused_item$, entries[30]);
  model.methods.move_focus(-1);
  assert.equal(store.focused_item$, entries[10]);
  model.methods.select_focused();
  assert.equal(store.value, "10");
  options.as([]);
  model.methods.move_focus(1);
  model.methods.select_focused();
  assert.equal(store.value, "10");
  model.methods.destroy();
});

test("reloaded options check only the current value instead of the stale selected item", () => {
  const { context, store, options, model } = setup();
  store.setValue("0");
  const entries = Array.from({ length: 1000 }, (_, index) =>
    new context.Timeless.vm.SelectItemCore({ label: `新账号 ${index}`, value: String(index) }),
  );
  store.setOptions(entries);
  options.as(entries);
  store.setValue("900");
  assert.deepEqual(entries.filter((entry) => entry.selected).map((entry) => entry.value), ["900"]);
  assert.equal(store.selected_item$, entries[900]);
  store.setValue("0");
  assert.deepEqual(entries.filter((entry) => entry.selected).map((entry) => entry.value), ["0"]);
  model.methods.destroy();
});

test("opening reveals the selected value after search reset and handles missing values", () => {
  const { entries, store, options, model } = setup();
  store.setValue("900");
  options.as([entries[900]]);
  model.methods.reveal_selected();
  assert.equal(model.state.scroll_request_.value.top, 0);
  options.as(entries);
  model.methods.reveal_selected();
  assert.equal(model.state.scroll_request_.value.top, 900 * 48);
  model.methods.on_scroll({ scrollTop: 0, clientHeight: 144 });
  model.methods.reveal_selected();
  assert.equal(model.state.scroll_request_.value.top, 900 * 48);
  store.setValue("missing");
  model.methods.reveal_selected();
  assert.equal(model.state.scroll_request_.value.top, 0);
  assert.equal(entries.some((entry) => entry.selected), false);
  model.methods.destroy();
});

test("AccountSelect reopens at the selected value after reloading and selecting a search result", async () => {
  const { context, entries, model } = setup();
  model.methods.destroy();
  let list_props;
  context.Input = () => context.Timeless.View();
  context.VirtualListView = (props) => {
    list_props = props;
    return context.Timeless.View();
  };
  const source = fs.readFileSync(path.join(__dirname, "../src/components/index.js"), "utf8")
    .replace(/^import .*;\n/gm, "")
    .replace(/^export \{[^}]+\};\n/gm, "")
    .replace(/^export /gm, "");
  vm.runInContext(`(() => { ${source}\nwindow.AccountSelect = AccountSelect; })()`, context);
  const store = new context.Timeless.vm.SelectCore({
    defaultValue: "",
    options: [new context.Timeless.vm.SelectItemCore({ label: "全部账号", value: "" })],
    search: new context.Timeless.vm.InputCore({ defaultValue: "" }),
  });
  const view = context.AccountSelect({ store });
  store.setOptions(entries);
  store.setValue("900");
  await store.show();
  const element = { scrollTop: 0 };
  let unlisten = list_props.onMounted({ target: element });
  assert.equal(element.scrollTop, 900 * 48);
  assert.deepEqual(entries.filter((entry) => entry.selected).map((entry) => entry.value), ["900"]);
  unlisten();
  store.search_input$.setValue("账号 999");
  store.handleClickItem(entries[999]);
  await store.show();
  assert.equal(list_props.each.value.length, 1000);
  unlisten = list_props.onMounted({ target: element });
  assert.equal(element.scrollTop, 999 * 48);
  assert.deepEqual(entries.filter((entry) => entry.selected).map((entry) => entry.value), ["999"]);
  unlisten();
  view.onUnmounted();
});
