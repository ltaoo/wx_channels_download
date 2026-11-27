const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function load_tags() {
  const intervals = new Set();
  const context = vm.createContext({
    console, setTimeout, clearTimeout, clearInterval,
    setInterval(fn, ms) { const timer = setInterval(fn, ms); timer.unref(); intervals.add(timer); return timer; },
  });
  const root = path.join(__dirname, "..");
  vm.runInContext(fs.readFileSync(path.join(root, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
  const requests = fs.readFileSync(path.join(root, "src/biz/request.js"), "utf8");
  vm.runInContext(requests.split("/**")[0].replace("export const", "const"), context);
  vm.runInContext('function setContentTags(body) { return request.post("/api/tag/content/set", body); }', context);
  vm.runInContext(fs.readFileSync(path.join(root, "src/tag.model.js"), "utf8")
    .replace(/^import[^\n]*\n/, "").replace("export function", "function"), context);
  return { context, dispose() { for (const timer of intervals) clearInterval(timer); } };
}

test("rapid changes from the picker and remove buttons are all persisted", async (t) => {
  const { context, dispose } = load_tags();
  t.after(dispose);
  const result = await vm.runInContext(`(async () => {
    const calls = [], callbacks = [];
    const tags_ref = Timeless.ref([]);
    let finish;
    const client = { post(url, body) {
      calls.push(JSON.parse(JSON.stringify(body)));
      return new Promise(resolve => { finish = () => resolve(Timeless.Result.Ok({})); });
    } };
    const write = next => persist_content_tags({ client, content_id: "one", tags_ref, next,
      on_change: selection => callbacks.push(selection.map(tag => tag.id)) });
    const first = write([{ id: 1 }]);
    await Promise.resolve();
    const second = write([{ id: 1 }, { id: 2 }]);
    const third = write([{ id: 2 }]);
    finish(); await first;
    await Promise.resolve(); finish(); await second;
    await Promise.resolve(); finish(); await third;
    return JSON.stringify({ calls, callbacks, selected: tags_ref.value });
  })()`, context);
  assert.deepEqual(JSON.parse(result), {
    calls: [
      { content_id: "one", tag_ids: [1] },
      { content_id: "one", tag_ids: [1, 2] },
      { content_id: "one", tag_ids: [2] },
    ], callbacks: [[2]], selected: [{ id: 2 }],
  });
});

test("failed writes restore the last confirmed selection and report the error", async (t) => {
  const { context, dispose } = load_tags();
  t.after(dispose);
  const result = await vm.runInContext(`(async () => {
    const tags_ref = Timeless.ref([{ id: 1 }]), tips = [];
    const client = { async post() { return Timeless.Result.Err("offline"); } };
    const failed = await persist_content_tags({ client, content_id: "one", tags_ref,
      next: [{ id: 2 }], tip: message => tips.push(message) });
    const failed_selection = tags_ref.value;
    client.post = async () => Timeless.Result.Ok({});
    await persist_content_tags({ client, content_id: "one", tags_ref, next: [{ id: 3 }] });
    return JSON.stringify({ failed: Boolean(failed.error), failed_selection, tips, selected: tags_ref.value });
  })()`, context);
  assert.deepEqual(JSON.parse(result), {
    failed: true, failed_selection: [{ id: 1 }], tips: [{ msg: ["offline"], type: "error" }], selected: [{ id: 3 }],
  });
});
