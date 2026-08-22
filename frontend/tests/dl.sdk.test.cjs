const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const sdk_source = fs.readFileSync(
  path.join(__dirname, "../public/dl.sdk.js"),
  "utf8",
);

function reference(initial_value) {
  return {
    value: initial_value,
    as(next_value) {
      this.value = next_value;
      return this;
    },
  };
}

function plain(value) {
  return JSON.parse(JSON.stringify(value));
}

function load_sdk(handle_request) {
  class RequestCore {
    constructor(run) {
      this.run_request = run;
    }

    run(params) {
      return this.run_request(params);
    }
  }

  class ChannelCore {
    onMessage(listener) {
      this.message_listener = listener;
    }

    onStateChange(listener) {
      this.state_listener = listener;
    }

    onReconnected(listener) {
      this.reconnected_listener = listener;
    }

    async connect() {
      return { data: true };
    }

    async reconnect() {
      return { data: true };
    }

    async disconnect() {
      return { data: true };
    }

    destroy() {}
  }

  const window = {
    Timeless: {
      Result: {
        Err(error) {
          return { error };
        },
        Ok(data) {
          return { data };
        },
      },
      kit: {
        RequestCore,
        SocketClientCore: class SocketClientCore {},
        ChannelCore,
        request_factory() {
          return {
            get(request_path, params) {
              return handle_request("GET", request_path, params);
            },
            post(request_path, body) {
              return handle_request("POST", request_path, body);
            },
          };
        },
      },
      ref: reference,
      refarr: reference,
      refobj: reference,
    },
    location: {
      href: "http://localhost/",
      origin: "http://localhost",
    },
    queueMicrotask,
    setTimeout,
    clearTimeout,
  };
  vm.runInNewContext(sdk_source, { URL, console, window });
  return window;
}

test("create(feed, options) maps platform and skip to the create request", async () => {
  const requests = [];
  const window = load_sdk((method, request_path, body) => {
    requests.push({ method, request_path, body });
    return Promise.resolve({
      data: {
        tasks: [
          {
            code: 0,
            data: {
              id: 12,
              status: 1,
              resources: [
                {
                  id: 21,
                  download_dir: "/downloads",
                  name: "video.mp4",
                },
              ],
            },
          },
        ],
      },
    });
  });
  const dl$ = window.DL({ client: {}, socket_client: {}, auto_start: false });
  const feed = { id: "feed-id", description: "test" };

  const task$ = await dl$.create(feed, {
    platform: "wxchannels",
    skip: true,
  });

  assert.deepEqual(plain(requests), [
    {
      method: "POST",
      request_path: "/api/v1/download_task/create",
      body: {
        objects: [
          {
            platform: "wxchannels",
            content: feed,
            config: { existing_action: "skip" },
          },
        ],
      },
    },
  ]);
  assert.equal(task$.files[0].output_path, "/downloads/video.mp4");
});

test("onSuccess receives final absolute output paths", async () => {
  const window = load_sdk(() =>
    Promise.resolve({
      data: {
        tasks: [{ code: 0, data: { id: 13, status: 2, resources: [] } }],
      },
    }),
  );
  const dl$ = window.DL({ client: {}, socket_client: {}, auto_start: false });
  const task$ = await dl$.create({
    platform: "wxchannels",
    content: { id: "feed-id" },
  });
  let completed_task = null;
  task$.onSuccess((task) => {
    completed_task = task;
  });

  task$._update({
    id: 13,
    status: 5,
    files: [
      {
        id: 31,
        download_dir: "/downloads/wxchannels",
        name: "final-video.mp4",
      },
    ],
  });

  assert.equal(completed_task, task$);
  assert.equal(
    completed_task.files[0].output_path,
    "/downloads/wxchannels/final-video.mp4",
  );
});

test("onFailed receives the Error directly", async () => {
  const window = load_sdk(() =>
    Promise.resolve({
      data: {
        tasks: [{ code: 0, data: { id: 14, status: 2, resources: [] } }],
      },
    }),
  );
  const dl$ = window.DL({ client: {}, socket_client: {}, auto_start: false });
  const task$ = await dl$.create({
    platform: "wxchannels",
    content: { id: "feed-id" },
  });
  let received_error = null;
  task$.onFailed((error) => {
    received_error = error;
  });

  task$._update({ id: 14, status: 6, error: "download failed" });

  assert.equal(received_error && received_error.name, "Error");
  assert.equal(received_error.message, "download failed");
});

test("skip hydrates an already finished task before returning", async () => {
  const requests = [];
  const window = load_sdk((method, request_path, body) => {
    requests.push({ method, request_path, body });
    if (method === "POST") {
      return Promise.resolve({
        data: {
          tasks: [
            {
              code: 0,
              data: { id: 15, name: "existing", skipped: true, action: "skip" },
            },
          ],
        },
      });
    }
    return Promise.resolve({
      data: {
        id: 15,
        name: "existing",
        status: 5,
        files: [
          {
            id: 41,
            download_dir: "/downloads",
            name: "existing.mp4",
          },
        ],
      },
    });
  });
  const dl$ = window.DL({ client: {}, socket_client: {}, auto_start: false });
  const task$ = await dl$.create(
    { id: "feed-id" },
    { platform: "wxchannels", skip: true },
  );
  let completed_task = null;

  task$.onSuccess((task) => {
    completed_task = task;
  });
  await new Promise((resolve) => setImmediate(resolve));

  assert.deepEqual(plain(requests[1]), {
    method: "GET",
    request_path: "/api/v1/download_task/list",
    body: { task_id: 15 },
  });
  assert.equal(completed_task, task$);
  assert.equal(task$.files[0].output_path, "/downloads/existing.mp4");
});
