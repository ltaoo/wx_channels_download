const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

const frontend = path.join(__dirname, "..");
const html = fs.readFileSync(path.join(frontend, "index.html"), "utf8");
const config_script = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)]
  .map((match) => match[1])
  .find((script) => script.includes("window.__d_config ="));

for (const page_url of [
  "http://192.168.0.118:2022/preview?id=1",
  "https://downloads.example.test/preview?id=1",
  "http://127.0.0.1:2022/preview?id=1",
  "http://[::1]:2022/preview?id=1",
]) {
  test(`preview and downloader URLs follow the page origin: ${page_url}`, () => {
    const location = new URL(page_url);
    const context = vm.createContext({
      window: { location }, URL,
    });
    context.global = context.window;
    // Legacy runtime configuration must not send a NAS page to loopback.
    vm.runInContext(config_script.replace("__WX_DOWNLOAD_CONFIG_JSON__", JSON.stringify({
      apiOrigin: "http://127.0.0.1:2022", apiHost: "127.0.0.1:2022",
      apiProtocol: "http", remoteServerEnabled: false,
    })), context);
    assert.equal(context.window.__d_config.apiOrigin, location.origin);
    assert.equal(context.window.__d_config.apiHost, location.host);
    assert.equal(context.window.__d_config.apiProtocol, location.protocol.slice(0, -1));
    context.window.API_ORIGIN = context.window.__d_config.apiOrigin;
    const preview = fs.readFileSync(path.join(frontend, "src/pages/preview.model.js"), "utf8");
    vm.runInContext(preview.slice(preview.indexOf("function file_url("), preview.indexOf("function playback_url(")), context);
    const media_url = context.file_url({
      file_url: "/api/file?path=%2FDownloads%2Fsynced%2Fvideo%23tag.mp4",
    });
    assert.equal(new URL(media_url).origin, location.origin);
    assert.equal(new URL(media_url).searchParams.get("path"), "/Downloads/synced/video#tag.mp4");
    const sdk = fs.readFileSync(path.join(frontend, "public/dl.sdk.js"), "utf8");
    vm.runInContext(sdk.slice(sdk.indexOf("function runtime_config_origin("), sdk.indexOf("function default_web_socket_url(")), context);
    assert.equal(context.runtime_config_origin(), location.origin);
  });
}

for (const remote_origin of [
  null,
  "http://100.78.198.69:2022",
  "https://downloads.example.test/",
  "http://[fd00::69]:2022",
]) {
  test(`injected remote downloads use the HTTPS/WSS relay without changing the configured target: ${remote_origin}`, () => {
    const local_origin = "http://127.0.0.1:2022";
    const context = vm.createContext({
      URL,
      location: new URL("https://channels.weixin.qq.com/web/pages/feed"),
      navigator: { userAgent: "test" },
      console: { log() {} },
    });
    context.window = context;
    context.global = context;
    context.__d_config = {
      apiOrigin: local_origin,
      apiHost: "127.0.0.1:2022",
      apiProtocol: "http",
      remoteServerEnabled: remote_origin !== null,
      remoteServerOrigin: remote_origin,
      assets_base_url: "/__assets",
    };
    const read = (file) => fs.readFileSync(path.join(frontend, file), "utf8");
    vm.runInContext(read("inject/env.js"), context);
    const assert_server = (expected_origin) => {
      const server = new URL(expected_origin);
      assert.equal(context.WXEnv.get("apiOrigin"), server.origin);
      assert.equal(context.WXEnv.get("apiHost"), server.host);
      assert.equal(context.WXEnv.get("apiProtocol"), server.protocol.slice(0, -1));
      assert.equal(context.WXEnv.get("downloaderOrigin"), server.origin);
      assert.equal(context.WXEnv.get("downloaderProtocol"), server.protocol.slice(0, -1));
      assert.equal(context.WXEnv.get("downloaderWSURL"),
        `${server.protocol === "https:" ? "wss:" : "ws:"}//${server.host}/ws/v1/download_task`);
      assert.equal(context.WXEnv.assetUrl("/inject/download/model.js"),
        "/__assets/inject/download/model.js");
    };
    const remote_proxy_origin = remote_origin ? "https://weixin110.qq.com" : null;
    assert_server(remote_proxy_origin || local_origin);
    vm.runInContext(read("inject/env.page.js"), context);
    assert_server(remote_proxy_origin || context.location.origin);
    assert.equal(context.__d_config.remoteServerOrigin, remote_origin);

    context.Timeless = {
      kit: {
        HttpClientCore: class { constructor(options) { this.hostname = options.hostname; } },
        request_factory() {},
      },
      web: { provide_http_client(client) { context.download_client = client; } },
    };
    const model = read("inject/download/model.js");
    vm.runInContext(model.slice(0, model.indexOf("function platform_preview_from_download_info(")), context);
    assert.equal(context.download_client.hostname,
      new URL(remote_proxy_origin || context.location.origin).origin);

    const sdk = read("public/dl.sdk.js");
    vm.runInContext(sdk.slice(sdk.indexOf("function runtime_config_origin("),
      sdk.indexOf("/**\n   * Download manager domain model.")), context);
    assert.equal(context.runtime_config_origin(), new URL(remote_proxy_origin || local_origin).origin);
    const server = new URL(remote_proxy_origin || local_origin);
    assert.equal(context.default_web_socket_url(),
      `${server.protocol === "https:" ? "wss:" : "ws:"}//${server.host}/ws/v1/download_task`);
  });
}
