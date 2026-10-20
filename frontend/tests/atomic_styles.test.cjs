const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test: node_test } = require("node:test");
const postcss = require("postcss");
const selectors = require("postcss-selector-parser");
const espree = require("espree");
let compile;
let Element;

const frontend = path.join(__dirname, "..");
const read = (file) => fs.readFileSync(path.join(frontend, file), "utf8");
const sheet = postcss.parse(read("src/style.css"));

function walk(node, visit) {
  if (!node || typeof node !== "object") return;
  visit(node);
  for (const value of Object.values(node)) {
    if (Array.isArray(value)) value.forEach((child) => walk(child, visit));
    else if (value && typeof value === "object") walk(value, visit);
  }
}

function source(file) {
  const text = read(`src/${file}`);
  return { text, ast: espree.parse(text, { ecmaVersion: "latest", sourceType: "module" }) };
}

// Read class expressions from the actual views, including their dynamic maps.
function view_classes(file, name, context = {}) {
  const { text, ast } = source(file);
  const scope = { ...context };
  for (const statement of ast.body) {
    if (statement.type !== "VariableDeclaration") continue;
    for (const declaration of statement.declarations) {
      if (!declaration.id.name?.endsWith("_classes")) continue;
      scope[declaration.id.name] = vm.runInNewContext(
        `(${text.slice(declaration.init.start, declaration.init.end)})`,
      );
    }
  }
  let expression;
  walk(ast, (node) => {
    if (node.type !== "ObjectExpression") return;
    const attrs = node.properties.find((p) => p.key?.name === "attributes")?.value;
    const id = attrs?.properties?.find((p) => p.key?.name === "n")?.value ||
      node.properties.find((p) => p.key?.name === "name")?.value;
    if (id?.value !== name) return;
    expression = node.properties.find((p) => p.key?.name === "class")?.value;
  });
  assert.ok(expression, `${file}: missing ${name} class expression`);
  return vm.runInNewContext(`(${text.slice(expression.start, expression.end)})`, scope);
}

function specificity(node) {
  return (node.nodes || []).reduce((total, part) => {
    if (part.type === "id") return total + 10000;
    if (["class", "attribute"].includes(part.type)) return total + 100;
    if (part.type === "tag") return total + 1;
    if (part.type !== "pseudo" || part.value === ":where") return total;
    if (part.value.startsWith("::")) return total + 1;
    if ([":is", ":not", ":has"].includes(part.value)) {
      return total + Math.max(0, ...part.nodes.map(specificity));
    }
    return total + 100;
  }, 0);
}

const rules = [];
const ready = (async () => {
  const [css_select, dom_handler] = await Promise.all([
    import("css-select"), import("domhandler"),
  ]);
  compile = css_select.compile;
  Element = dom_handler.Element;
  for (const css of [read("src/dmui.css"), read("src/style.css")]) {
    postcss.parse(css).walkRules((rule) => {
      if (rule.parent.type === "atrule" && /keyframes$/.test(rule.parent.name)) return;
      const media = [];
      for (let parent = rule.parent; parent && parent.type !== "root"; parent = parent.parent) {
        if (parent.type === "atrule") media.push(parent.params);
      }
      selectors((root) => root.each((selector) => {
        const text = selector.toString();
        // Pseudo-elements have a separate cascade from their originating element.
        if (text.includes("::")) return;
        rules.push({
          match: compile(text, { pseudos: {
            hover: (element) => !!element.hover,
            active: () => false,
            focus: () => false,
            "focus-visible": () => false,
            "focus-within": () => false,
          } }),
          specificity: specificity(selector), media,
          declarations: rule.nodes.filter((node) => node.type === "decl"),
        });
      })).processSync(rule.selector);
    });
  }
})();

function test(name, body) {
  return node_test(name, async (context) => {
    await ready;
    return body(context);
  });
}

function element(classes, parent) {
  const node = new Element("div", { class: classes }, []);
  if (parent) {
    parent.children.push(node);
    node.parent = parent;
  }
  return node;
}

function styles(node, { width = 1440, reduced = false } = {}) {
  const result = new Map();
  for (const rule of rules) {
    if (!rule.media.every((media) => {
      const max = media.match(/max-width:\s*(\d+)px/);
      const min = media.match(/min-width:\s*(\d+)px/);
      return !(max && width > +max[1]) && !(min && width < +min[1]) &&
        (!media.includes("prefers-reduced-motion") || reduced) &&
        !media.includes("prefers-color-scheme");
    }) || !rule.match(node)) continue;
    for (const declaration of rule.declarations) {
      const prior = result.get(declaration.prop);
      const important = !!declaration.important;
      if (prior && (prior.important && !important ||
        prior.important === important && prior.specificity > rule.specificity)) continue;
      result.set(declaration.prop, {
        value: declaration.value.replace(/\s+/g, " ").trim(),
        important, specificity: rule.specificity,
      });
    }
  }
  return Object.fromEntries([...result].map(([key, value]) => [key, value.value]));
}

test("page utilities declare one property and have valid, consumed class names", () => {
  const used = new Set();
  function scan(directory) {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const filename = path.join(directory, entry.name);
      if (entry.isDirectory()) scan(filename);
      else if (filename.endsWith(".js")) {
        const ast = espree.parse(fs.readFileSync(filename, "utf8"), { ecmaVersion: "latest", sourceType: "module" });
        walk(ast, (node) => {
          const value = node.type === "Literal" ? node.value : node.type === "TemplateElement" ? node.value.raw : null;
          if (typeof value === "string") value.split(/\s+/).forEach((word) => used.add(word));
        });
      }
    }
  }
  scan(path.join(frontend, "src"));
  sheet.walkRules((rule) => {
    if (rule.parent.type === "atrule" && /keyframes$/.test(rule.parent.name)) return;
    if (["html", "body", "#root"].includes(rule.selector)) return;
    assert.equal(rule.nodes.filter((node) => node.type === "decl").length, 1, rule.selector);
    let utility;
    selectors((root) => root.walkClasses((node) => {
      if (!/^(group-|is-|has-|dm-)/.test(node.value)) utility = node.value;
    })).processSync(rule.selector);
    assert.ok(utility, `page rule needs a utility class: ${rule.selector}`);
    assert.ok(used.has(utility), `unused or incorrectly escaped utility: ${utility}`);
  });
  const animations = [];
  sheet.walkDecls(/^animation(?:-name)?$/, (declaration) => animations.push(declaration.value));
  sheet.walkAtRules(/keyframes$/, (rule) => {
    assert.ok(animations.some((value) => value.includes(rule.params)), `unused keyframes: ${rule.params}`);
  });
});

test("the app loads local utilities once and routes have no page stylesheet dependency", () => {
  assert.deepEqual(
    [...read("index.html").matchAll(/<link[^>]+href="([^"]+\.css)[^"]*"/g)].map((match) => match[1]),
    ["src/dmui.css", "src/style.css"],
  );
  assert.doesNotMatch(read("src/store.js"), /src\/pages\/[^"\n]+\.css|load_style/);
  assert.doesNotMatch(read("package.json"), /tailwind/i);
  assert.doesNotMatch(read("src/style.css"), /@import/);
});

test("the account page retains its bounded layout and content region", () => {
  const page = element(view_classes("pages/account.js", "account-page"));
  const layout = styles(page);
  assert.equal(layout.height, "100%");
  assert.equal(layout["min-height"], "0");
  assert.equal(layout.overflow, "hidden");
  const main = element("group-content-main", page);
  assert.equal(styles(main).display, "flex");
  assert.equal(styles(main)["min-height"], "0");
});

test("the automation editor fills its page and keeps the inspector and toolbar fixed", () => {
  const page = element(view_classes("pages/flow_edit.js", "automation-page"));
  assert.equal(styles(page).height, "100%");
  assert.equal(styles(page)["min-height"], "0");
  assert.equal(styles(page).overflow, "hidden");
  const workspace = element(view_classes("pages/flow_edit.js", "automation-editor-workspace"), page);
  assert.equal(styles(workspace).position, "absolute");
  assert.equal(styles(workspace).inset, "0");
  const canvas = element("group-automation-flow-scroll", workspace);
  assert.equal(styles(canvas).height, "100%");
  assert.equal(styles(canvas)["min-height"], "0");
  const inspector = element(view_classes("pages/flow_edit.js", "automation-node-inspector"));
  assert.equal(styles(inspector).position, "fixed");
  assert.equal(styles(inspector).width, "min(420px, calc(100vw - 32px))");
  assert.match(styles(inspector).transition, /transform.*opacity.*visibility/);
  assert.equal(styles(inspector, { reduced: true }).transition, "none");
  const toolbar = element(view_classes("pages/flow.components.js", "automation-workspace-toolbar", { is_edit: true }));
  assert.equal(styles(toolbar).position, "fixed");
  assert.equal(styles(toolbar).left, "var(--dm-space-4)");
  assert.doesNotMatch(read("src/pages/flow_edit.js"), /AutomationRunHistory|automation-editor-toolbar-wrap/);
});

test("dynamic gallery variants retain single-image and three-image layouts", () => {
  for (const count of [1, 2, 3, 4]) {
    const gallery = element(view_classes("pages/account.js", "account-content-card-gallery", {
      visible_images: Array(count),
    }));
    const layout = styles(gallery);
    assert.equal(layout.display, count === 1 ? "block" : "grid");
    if (count === 1) assert.equal(layout["aspect-ratio"], "auto");
    const item = element(view_classes("pages/account.js", "account-content-card-gallery-item"), gallery);
    assert.equal(styles(item)["grid-row"], count === 3 ? "span 2" : undefined);
  }
});

test("dynamic content statistics retain their status colors", () => {
  for (const [key, tone] of Object.entries({
    running: "warning", paused: "warning", finished: "success", failed: "danger", files: "primary",
  })) {
    const node = element(view_classes("pages/content.js", "content-row-stat", { item: { key } }));
    assert.equal(styles(node).background, `var(--dm-color-${tone}-soft)`);
    assert.equal(styles(node).color, `var(--dm-color-${tone})`);
  }
});

test("dynamic log levels retain their foreground and background colors", () => {
  const { text, ast } = source("pages/logs.js");
  const table = ast.body.flatMap((node) => node.declarations || [])
    .find((node) => node.id.name === "logs_level_classes").init;
  const logs_level_classes = vm.runInNewContext(`(${text.slice(table.start, table.end)})`);
  const fn = ast.body.find((node) => node.type === "FunctionDeclaration" && node.id.name === "LogsPageLevelClass");
  const level_classes = vm.runInNewContext(`(${text.slice(fn.start, fn.end)})`, { logs_level_classes });
  for (const [level, color, background] of [
    ["debug", "--dm-content-muted", "--dm-content-panel-soft"],
    ["info", "--dm-color-info", "--dm-color-info-soft"],
    ["warn", "--dm-color-warning", "--dm-color-warning-soft"],
    ["error", "--dm-content-danger", "--dm-color-danger-soft"],
    ["unknown", "--dm-color-info", "--dm-color-info-soft"],
    ["constructor", "--dm-color-info", "--dm-color-info-soft"],
  ]) {
    const result = styles(element(level_classes(level)));
    assert.equal(result.color, `var(${color})`);
    assert.equal(result.background, `var(${background})`);
  }
});

test("compact navigation retains its bottom placement and hidden brand", () => {
  const sider = element(view_classes("pages/shell.js", "app-sider"));
  assert.equal(styles(sider, { width: 640 })["grid-row"], "2");
  assert.equal(styles(sider, { width: 1440 })["grid-row"], undefined);
  const brand = element(view_classes("pages/shell.js", "app-brand"));
  assert.equal(styles(brand, { width: 640 }).display, "none");
  assert.equal(styles(brand, { width: 1440 }).display, "flex");
});

test("sync status classes returned by the model retain their state variants", () => {
  const { text, ast } = source("pages/sync.model.js");
  let fn;
  walk(ast, (node) => {
    if (node.type === "FunctionDeclaration" && node.id.name === "device_status_class_names") fn = node;
  });
  const status_classes = vm.runInNewContext(`(${text.slice(fn.start, fn.end)})`);
  assert.equal(styles(element(status_classes("online"))).color, "var(--dm-color-success)");
  assert.equal(styles(element(status_classes("offline"))).color, "var(--dm-color-danger)");
  assert.equal(styles(element(status_classes("unknown"))).display, "none");
});

test("route motion is disabled when reduced motion is requested", () => {
  const { text, ast } = source("store.js");
  const declaration = ast.body.flatMap((node) => node.declarations || []).find((node) => node.id.name === "route_animation");
  const animation = vm.runInNewContext(text.slice(declaration.init.start, declaration.init.end));
  const node = element(animation.out);
  assert.match(styles(node).animation, /^dm-route-view-out /);
  assert.equal(styles(node)["pointer-events"], "none");
  assert.equal(styles(node, { reduced: true }).animation, "none");
});
