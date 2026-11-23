const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");

async function main() {
  const { source, mode } = JSON.parse(fs.readFileSync(0, "utf8"));
  const events = [];
  const scripts = [];
  const context = vm.createContext({
    WXU: { emit: (name, data) => events.push([name, data]) },
    console: { log() {} },
    document: {
      createElement(tag) {
        assert.equal(tag, "script");
        return { attributes: {}, setAttribute(name, value) { this.attributes[name] = value; } };
      },
      head: { appendChild(script) { scripts.push(script); } },
    },
  });
  context.window = context;
  // Convert only the fixture's ESM export statement so the proxy output can run in a VM.
  const runnable = source.replace(/export\s*\{([^}]*)\};?/g, (_, names) =>
    names.split(",").filter(Boolean).map((name) => `globalThis.${name}=${name};`).join(""));
  vm.runInContext(runnable, context);
  if (mode === "runtime-loader") {
    assert.equal(context.originalSDK, true);
    assert.equal(scripts.length, 1);
    assert.equal(scripts[0].attributes["data-n"], "wxchannels-liteapp-runtime");
    assert.equal(scripts[0].crossOrigin, "anonymous");
    assert.match(scripts[0].src, /^https:\/\/res\.wx\.qq\.com\/__assets\/wxchannels\/liteapp\/runtime\.js\?t=\d+$/);
  } else if (mode === "navigation") {
    const { handlers } = context;
    events.length = 0;
    for (const [name, event, id] of [
      ["goToNextFlowFeed", "channels:GotoNextFeed", "second"],
      ["goToPrevFlowFeed", "channels:GotoPrevFeed", "first"],
    ]) {
      const arg = {};
      const promise = handlers[name](arg);
      assert.equal(events.length, 0, "navigation event fired before completion");
      await promise;
      assert.equal(arg.done, true, "original navigation did not run");
      assert.equal(events.length, 1);
      assert.equal(events[0][0], event);
      assert.equal(events[0][1].id, id, "event did not read the updated closure ref");
      events.length = 0;
      const error = new Error("navigation failed");
      await assert.rejects(handlers[name]({ fail: error }), (e) => e === error);
      assert.equal(events.length, 0, "failed navigation emitted success");
    }
    if (source.includes("channels:HomeFeedChanged")) {
      const first = {}, second = {};
      await handlers.loadLocalPlaylist(first, second);
      assert.equal(context.loadArgs[0], first);
      assert.equal(context.loadArgs[1], second);
      assert.equal(events.length, 1);
      assert.equal(events[0][0], "channels:HomeFeedChanged");
      assert.equal(events[0][1].id, "playlist");
    }
  } else if (mode === "api") {
    const api = new context.API();
    const init = { data: { username: "user" } };
    const detail = { data: { object: { id: "detail" } } };
    const payload = { id: "requested" };
    api.request = async function (name, value) {
      assert.equal(this, api);
      if (name === "detail") assert.equal(value, payload);
      return name === "init" ? init : detail;
    };
    events.length = 0;
    assert.equal(await api.finderInit(), init);
    assert.equal(events.length, 1);
    assert.equal(events[0][0], "channels:Init");
    assert.equal(events[0][1], init.data);
    events.length = 0;
    assert.equal(await api.finderGetCommentDetail(payload), detail);
    assert.equal(events.length, 1);
    assert.equal(events[0][0], "channels:OnFeedProfileLoaded");
    assert.equal(events[0][1], detail.data.object);
    events.length = 0;
    const error = new Error("request failed");
    api.request = async () => { throw error; };
    await assert.rejects(api.finderGetCommentDetail(payload), (e) => e === error);
    assert.equal(events.length, 0);
  } else {
    context.preload();
    await new Promise(setImmediate);
    assert.equal(events.length, 1);
    assert.equal(events[0][0], "channels:PreloadFeeds");
    assert.equal(events[0][1][0].id, "preloaded");
  }
}

main().catch((error) => { console.error(error); process.exitCode = 1; });
