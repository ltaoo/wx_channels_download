self.importScripts(
  new URL("src/pages/wxchannels.crypto.js", self.location).href,
);

const ENCRYPTED_LENGTH = 131072;
const STREAM_MARKER = "/wxchannels-stream/";
const STREAM_CONFIG_URL = new URL(
  "wxchannels-stream/config",
  self.location,
).href;

self.addEventListener("install", (event) => {
  event.waitUntil(self.skipWaiting());
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("message", (event) => {
  const message = event.data || {};
  if (message.type === "play") {
    event.waitUntil(save_stream(message));
  } else if (message.type === "revoke") {
    event.waitUntil(revoke_stream(message.token));
  }
});

self.addEventListener("fetch", (event) => {
  const token = stream_token_for_request(event.request);
  if (!token) return;
  event.respondWith(
    (async () => {
      const stream = await load_stream(token);
      if (!stream) {
        return new Response("WxChannels stream not found", { status: 404 });
      }
      return decrypt_stream_response(event.request, stream);
    })(),
  );
});

function stream_token_for_request(request) {
  if (!["GET", "HEAD"].includes(request.method)) return null;
  const pathname = new URL(request.url).pathname;
  const marker_index = pathname.lastIndexOf(STREAM_MARKER);
  if (marker_index < 0) return null;
  const token = pathname.slice(marker_index + STREAM_MARKER.length);
  return token && token !== "config" ? token : null;
}

async function save_stream(message) {
  const cache = await caches.open("wxchannels-streams");
  await cache.put(
    STREAM_CONFIG_URL,
    new Response(
      JSON.stringify({
        token: message.token,
        url: message.url,
        key: message.key.toString(),
      }),
      { headers: { "content-type": "application/json" } },
    ),
  );
}

async function load_stream(token) {
  const cache = await caches.open("wxchannels-streams");
  const response = await cache.match(STREAM_CONFIG_URL);
  if (!response) return null;
  const stream = await response.json();
  if (stream.token !== token) return null;
  return { ...stream, key: BigInt(stream.key) };
}

async function revoke_stream(token) {
  const stream = await load_stream(token);
  if (!stream) return;
  const cache = await caches.open("wxchannels-streams");
  await cache.delete(STREAM_CONFIG_URL);
}

async function decrypt_stream_response(request, stream) {
  try {
    const range = request.headers.get("Range");
    const response = await fetch(stream.url, {
      cache: "reload",
      mode: "cors",
      headers: range ? { Range: range } : {},
    });
    if (!response.ok) return response;

    const headers = new Headers();
    for (const name of [
      "accept-ranges",
      "content-length",
      "content-range",
      "content-type",
      "etag",
      "last-modified",
    ]) {
      const value = response.headers.get(name);
      if (value) headers.set(name, value);
    }
    headers.set("accept-ranges", "bytes");
    if (request.method === "HEAD" || !response.body) {
      return new Response(null, {
        status: response.status,
        statusText: response.statusText,
        headers,
      });
    }

    const decryptor = globalThis.create_wxchannels_decryptor(
      stream.key,
      response_offset(request, response),
      ENCRYPTED_LENGTH,
    );
    const reader = response.body.getReader();
    const body = new ReadableStream({
      async pull(controller) {
        const { done, value } = await reader.read();
        if (done) {
          controller.close();
          return;
        }
        controller.enqueue(decryptor.decrypt_chunk(value));
      },
      cancel(reason) {
        return reader.cancel(reason);
      },
    });

    return new Response(body, {
      status: response.status,
      statusText: response.statusText,
      headers,
    });
  } catch (error) {
    report_stream_error(
      stream.token,
      error.message === "Failed to fetch"
        ? "无法流式读取视频：该地址需要允许浏览器跨域访问（CORS）"
        : error.message || String(error),
    );
    return new Response("WxChannels stream failed", { status: 502 });
  }
}

function response_offset(request, response) {
  if (response.status !== 206) return 0;
  const requested = (request.headers.get("range") || "").match(/^bytes=(\d+)-/);
  if (requested) return Number(requested[1]);
  const returned = (response.headers.get("content-range") || "").match(
    /^bytes (\d+)-/,
  );
  return returned ? Number(returned[1]) : 0;
}

function report_stream_error(token, message) {
  self.clients.matchAll().then((clients) => {
    for (const client of clients) {
      client.postMessage({ type: "error", token, message });
    }
  });
}
