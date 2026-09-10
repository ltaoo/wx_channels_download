const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const path = require("node:path");
const test = require("node:test");

async function load_detail_model() {
  const source = readFileSync(
    path.join(__dirname, "../src/pages/content_detail.model.js"),
    "utf8",
  ).replace(
    /^import \{ PreviewViewModel, normalize_file \} from "\.\/preview\.model\.js";$/m,
    [
      "const PreviewViewModel = () => ({ methods: { destroy() {} } });",
      "const normalize_file = (file) => file;",
    ].join("\n"),
  );
  const module_url = `data:text/javascript;base64,${Buffer.from(source).toString("base64")}`;
  return import(module_url);
}

function install_minimal_runtime() {
  const make_ref = (value) => {
    const subscribers = [];
    const source = { value };
    return {
      get value() {
        return source.value;
      },
      as(next) {
        source.value = next;
        subscribers.forEach((subscriber) => subscriber.onChange(source.value));
      },
      subscribe(subscriber) {
        subscribers.push(subscriber);
        return () => {
          const index = subscribers.indexOf(subscriber);
          if (index >= 0) subscribers.splice(index, 1);
        };
      },
    };
  };
  globalThis.ref = make_ref;
  globalThis.computed = (dependency, calculate) => {
    const result = make_ref(calculate(dependency.value));
    dependency.subscribe({
      onChange(value) {
        result.as(calculate(value));
      },
    });
    return result;
  };
  globalThis.Timeless = {
    vm: {
      ButtonCore: class {
        constructor(options = {}) {
          this.disabled = Boolean(options.disabled);
        }
        enable() {
          this.disabled = false;
        }
        disable() {
          this.disabled = true;
        }
      },
    },
  };
}

test("content detail normalizes associated tags", async () => {
  const { normalize_content_detail } = await load_detail_model();
  const detail = normalize_content_detail({
    id: "content-1",
    title: "Example",
    type: "article",
    tags: [
      { id: 3, name: " Go " },
      { ID: "bad", Name: "" },
      null,
    ],
  });

  assert.deepEqual(detail.tags, [{ id: 3, name: "Go" }]);
});

test("content extension model navigates files with boundary states", async () => {
  const { ContentDetailExtensionModel } = await load_detail_model();
  install_minimal_runtime();
  const model = ContentDetailExtensionModel([
    { key: "one" },
    { key: "two", available: true },
    { key: "three" },
  ]);

  assert.equal(model.state.selected.value.key, "two");
  assert.equal(model.ui.btn_previous$.disabled, false);
  assert.equal(model.ui.btn_next$.disabled, false);

  assert.equal(model.methods.previous().key, "one");
  assert.equal(model.ui.btn_previous$.disabled, true);
  assert.equal(model.methods.next().key, "two");
  assert.equal(model.methods.next().key, "three");
  assert.equal(model.ui.btn_next$.disabled, true);
  assert.equal(model.state.selected.value.key, "three");

  delete globalThis.ref;
  delete globalThis.computed;
  delete globalThis.Timeless;
});
