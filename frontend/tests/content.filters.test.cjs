const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const path = require("node:path");
const test = require("node:test");

async function load_content_model() {
  const source = readFileSync(
    path.join(__dirname, "../src/pages/content.model.js"),
    "utf8",
  )
    .replace(
      /^import \{ proxy_image_url \} from "@\/image-proxy\.model\.js";$/m,
      "const proxy_image_url = (_platform_id, url) => url;",
    )
    .replace(
      /^import \{ task_status \} from "\.\/content_detail\.model\.js";$/m,
      'const task_status = () => ({ tone: "success", label: "成功" });',
    );
  const module_url = `data:text/javascript;base64,${Buffer.from(source).toString("base64")}`;
  return import(module_url);
}

test("saved filters are normalized and matched by their criteria", async () => {
  const {
    content_type_icon,
    normalize_content_layout,
    normalize_saved_filters,
    saved_filters_match,
  } = await load_content_model();
  const filters = normalize_saved_filters([
    {
      id: " one ",
      name: "  微信视频  ",
      keyword: " foo ",
      platform_id: "wxchannels",
      scope: "all",
      extra: "ignored",
    },
    null,
    { id: "one", name: "duplicate" },
    { name: "missing id" },
  ]);

  assert.deepEqual(filters, [
    {
      id: "one",
      name: "微信视频",
      platform_name: "",
      account_name: "",
      keyword: "foo",
      content_type: "",
      platform_id: "wxchannels",
      account_id: "",
      scope: "all",
    },
  ]);
  assert.equal(
    saved_filters_match(filters[0], {
      keyword: "foo",
      content_type: "",
      platform_id: "wxchannels",
      account_id: "",
      scope: "all",
    }),
    true,
  );
  assert.equal(
    saved_filters_match(filters[0], { ...filters[0], account_id: "1" }),
    false,
  );
  assert.equal(normalize_content_layout("card"), "card");
  assert.equal(normalize_content_layout("bad-value"), "table");
  globalThis.window = {
    CONTENT_TYPE_ICONS: {
      html: "sprite#html",
      article: "sprite#article",
      default: "sprite#default",
    },
  };
  assert.equal(content_type_icon("article", "html"), "sprite#html");
  assert.equal(content_type_icon("article", ""), "sprite#article");
  assert.equal(content_type_icon("unknown", ""), "sprite#default");
  delete globalThis.window;
});
