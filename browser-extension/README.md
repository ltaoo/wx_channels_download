# WX Channels Browser Extension

这个目录是一个独立的浏览器扩展。扩展会在所有网站注入 content script，再由 `content/dispatcher.js` 根据当前域名运行对应站点逻辑。

## 结构

- `manifest.json`: Manifest V3 配置，`content_scripts.matches` 为 `<all_urls>`。
- `background.js`: 接收 content script 的上报消息，并在 content 直接上报失败时转发请求。
- `content/common.js`: 通用工具层，包含 DOM 监听、上报、日志、URL/JSON/属性解析等函数。
- `content/sites/zhihu.js`: 知乎首页推荐流采集逻辑，参考 `internal/platformbrowser/zhihu/scripts/topstory_recommend.js`。
- `content/dispatcher.js`: 执行已注册且匹配当前域名的站点脚本。

## 通用函数

站点脚本通过 `window.__wx_browser_extension__` 使用公共能力：

- `observeNode(selector, cb, options)`: 监听指定元素是否存在，找到后执行一次回调。
- `observeElements(selector, cb, options)`: 扫描并持续监听新增匹配元素，每个元素只回调一次。
- `reportProfile(payload, options)`: 上报内容数据到 `/__wx_channels_api/platform/browser`。
- `registerSite(site)`: 注册域名匹配和站点执行逻辑。
- `text`、`first`、`absoluteURL`、`parseJSON`、`attr`、`metaContent`、`metaContents`: 常用解析工具。

## 自动化操作

- `sleep(ms)`、`nextFrame()`、`waitUntil(check, options)`: 等待页面状态。
- `query(selector, root)`、`queryAll(selector, root)`、`findByText(selector, textOrRegExp, root)`: 查找元素。
- `scrollIntoView(target, options)`: 把元素滚动到视口中间。
- `scrollContainer(target, options)`: 自动滚动页面或容器，支持 `step`、`delay`、`maxTimes`、`until`。
- `click(target, options)`、`clickByText(selector, textOrRegExp, options)`: 点击元素或按文本点击按钮。
- `fill(target, value, options)`: 填写 `input`、`textarea` 或 `contenteditable`。
- `selectValue(target, value, options)`: 选择 `select` 的值。
- `submitForm(target, options)`: 提交指定表单或元素所在表单。
- `dispatchEvent(el, type, options)`、`dispatchMouseEvent(el, type, options)`: 主动派发事件。

示例：

```js
await WXExt.fill("input[name='q']", "keyword");
await WXExt.clickByText("button", /search/i);
await WXExt.scrollContainer(null, {
  step: 800,
  delay: 300,
  maxTimes: 10,
  until: function () {
    return document.querySelectorAll(".result-item").length >= 20;
  },
});
```

## 数据采集

- `isVisible(el)`、`elementText(el)`: 判断可见性和读取文本。
- `collectText(selector, options)`: 批量提取文本。
- `collectLinks(selector, options)`: 批量提取链接，返回 `{ text, href, title }`。
- `collectImages(selector, options)`: 批量提取图片，返回 `{ src, alt, width, height }`。
- `collectMeta(root)`: 提取页面 `meta`。
- `collectJSONLD(root)`: 提取 `application/ld+json` 结构化数据。
- `collectTable(table)`: 把表格转为对象数组或二维数组。
- `scrapeElements(selector, schema, options)`: 按 schema 从列表元素中提取结构化数据。

示例：

```js
var items = WXExt.scrapeElements(".result-item", {
  title: "h2",
  url: { selector: "a[href]", attr: "href", url: true },
  cover: { selector: "img", attr: "src", url: true },
  summary: ".summary",
});

items.forEach(function (item) {
  WXExt.reportProfile({
    platform_id: "example",
    platform_name: "Example",
    content_type: "article",
    content_external_id: "example:" + item.url,
    content_title: item.title,
    content_url: item.url,
    content_cover_url: item.cover,
  });
});
```

## 新增站点

新增 `content/sites/example.js`，按下面形式注册：

```js
(function () {
  var api = window.__wx_browser_extension__;
  if (!api) {
    return;
  }

  api.registerSite({
    id: "example",
    matches: function (loc) {
      return api.hostnameMatches(loc.hostname, ["example.com"]);
    },
    run: function (WXExt) {
      WXExt.observeNode(".target", function (el) {
        WXExt.reportProfile({
          platform_id: "example",
          platform_name: "Example",
          content_type: "article",
          content_external_id: "example:" + location.href,
          content_title: document.title,
          content_url: location.href,
        });
      });
    },
  });
})();
```

然后把新脚本加入 `manifest.json` 的 `content_scripts[0].js`，放在 `content/dispatcher.js` 之前。

## 安装调试

Chrome/Edge 打开扩展管理页，启用开发者模式，选择“加载已解压的扩展”，目录选 `browser-extension/`。

需要查看调试日志时，在页面控制台执行：

```js
localStorage.setItem("__wx_browser_extension_debug__", "1");
location.reload();
```
