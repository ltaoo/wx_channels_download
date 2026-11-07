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
