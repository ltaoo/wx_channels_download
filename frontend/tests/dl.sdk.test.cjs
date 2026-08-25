const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

function loadSDK() {
  const channels = [];

  function ref(initial) {
    const listeners = new Set();
    return {
      value: initial,
      as(next) {
        this.value = typeof next === "function" ? next(this.value) : next;
        listeners.forEach((listener) => listener.onChange?.(this.value));
      },
      subscribe(listener) {
        listeners.add(listener);
        return () => listeners.delete(listener);
      },
    };
  }

  function refarr(initial) {
    const value = ref(initial);
    Object.defineProperty(value, "length", {
      get() {
        return value.value.length;
      },
    });
    return value;
  }

  class FakeChannelCore {
    constructor(url, options) {
      this.url = url;
      this.options = options;
      this.messageListeners = [];
      this.stateListeners = [];
      this.destroyed = false;
      channels.push(this);
    }

    onMessage(listener) {
      this.messageListeners.push(listener);
    }

    onStateChange(listener) {
      this.stateListeners.push(listener);
    }

    onReconnected() {}

    async connect() {
      this.stateListeners.forEach((listener) =>
        listener({ connected: true, connecting: false }),
      );
      return { data: true };
    }

    async disconnect() {
      this.stateListeners.forEach((listener) =>
        listener({ connected: false, connecting: false }),
      );
      return { data: true };
    }

    destroy() {
      this.destroyed = true;
    }

    emit(message) {
      this.messageListeners.forEach((listener) =>
        listener(this.options.process(JSON.stringify(message))),
      );
    }
  }

  class FakeRequestCore {
    constructor(run) {
      this.run = run;
    }
  }

  const sandbox = {
    clearTimeout,
    console,
    location: { origin: "http://127.0.0.1:2022" },
    queueMicrotask,
    setTimeout,
    Timeless: {
      ref,
      refobj: ref,
      refarr,
      Result: {
        Ok: (data) => ({ data }),
        Err: (error) => ({ error }),
      },
      kit: {
        RequestCore: FakeRequestCore,
        SocketClientCore: class {},
        ChannelCore: FakeChannelCore,
        request_factory: () => ({ get() {}, post() {} }),
      },
    },
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(
    fs.readFileSync(path.join(__dirname, "..", "public", "dl.sdk.js"), "utf8"),
    sandbox,
  );

  const owner = {
    socket_client: {},
    websocket_url: "/ws/v1/download_task",
    task_websocket_options: {
      reconnect: { enabled: true, interval: 5000 },
    },
    reqs: { download: {} },
  };

  return { channels, owner, DownloadTaskModel: sandbox.DownloadTaskModel };
}

function nextTurn() {
  return new Promise((resolve) => setImmediate(resolve));
}

test("task_upsert emits terminal callbacks once and replays late listeners", async () => {
  const { channels, owner, DownloadTaskModel } = loadSDK();
  const completedTask = DownloadTaskModel({
    owner,
    record: { id: 29, status: 2 },
  });
  const failedTask = DownloadTaskModel({
    owner,
    record: { id: 30, status: 2 },
  });
  let successCount = 0;
  let successRecord = null;
  let failureCount = 0;
  let failureError = null;
  completedTask.onSuccess((record) => {
    successCount += 1;
    successRecord = record;
  });
  failedTask.onFailed((error) => {
    failureCount += 1;
    failureError = error;
  });

  await nextTurn();
  channels[0].emit({
    type: "task_upsert",
    tasks: [{ id: 29, status: 5, files: [] }],
  });
  channels[1].emit({
    type: "task_upsert",
    tasks: [{ id: 30, status: 6, error: "network failed" }],
  });

  assert.equal(successCount, 1);
  assert.equal(successRecord.id, 29);
  assert.equal(completedTask.status.value, "finished");
  assert.equal(failureCount, 1);
  assert.match(failureError.message, /network failed/);
  assert.equal(failedTask.status.value, "failed");

  channels[0].emit({ type: "task_upsert", tasks: [{ id: 29, status: 5 }] });
  channels[1].emit({ type: "task_upsert", tasks: [{ id: 30, status: 6 }] });
  assert.equal(successCount, 1);
  assert.equal(failureCount, 1);

  let lateSuccessCount = 0;
  let lateFailureCount = 0;
  completedTask.onSuccess(() => {
    lateSuccessCount += 1;
  });
  failedTask.onFailed(() => {
    lateFailureCount += 1;
  });
  await Promise.resolve();
  assert.equal(lateSuccessCount, 1);
  assert.equal(lateFailureCount, 1);
});

test("task WebSocket subscribes by id and can be permanently disconnected", async () => {
  const { channels, owner, DownloadTaskModel } = loadSDK();
  const task = DownloadTaskModel({ owner, record: { id: 29, status: 2 } });

  await nextTurn();
  assert.equal(channels.length, 1);
  assert.equal(channels[0].url, "/ws/v1/download_task?task_id=29");
  assert.equal(task.websocket_connected.value, true);

  await task.disconnectWebSocket();
  assert.equal(channels[0].destroyed, true);
  assert.equal(task.websocket_connected.value, false);

  task._update({ id: 29, status: 2 });
  await nextTurn();
  assert.equal(channels.length, 1);
});
