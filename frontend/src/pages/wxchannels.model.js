import "./wxchannels.crypto.js";

const WX_CHANNELS_ENCRYPTED_LENGTH = 131072;
const stream_worker_url = new URL(
  "../../wxchannels.stream.js",
  import.meta.url,
);

export function WxChannelsPlayerViewModel() {
  const Timeless = window.Timeless;
  const url_ = Timeless.ref("");
  const decode_key_ = Timeless.ref("");
  const playback_url_ = Timeless.ref("");
  const loading_ = Timeless.ref(false);
  const error_ = Timeless.ref("");
  const status_ = Timeless.ref("");
  let request_sequence = 0;
  let abort_controller = null;
  let playback_token = "";
  let stream_worker = null;
  let stream_worker_registration = null;
  let object_url = "";

  const submit_disabled_ = Timeless.combine(
    { url: url_, decode_key: decode_key_, loading: loading_ },
    (state) =>
      !state.url.trim() ||
      !state.decode_key.trim() ||
      Boolean(state.loading),
  );
  const submit_text_ = Timeless.combine(
    { loading: loading_ },
    (state) => (state.loading ? "解密中..." : "播放"),
  );

  function revoke_playback_url() {
    if (object_url) {
      URL.revokeObjectURL(object_url);
      object_url = "";
    }
    playback_url_.as("");
    if (playback_token && stream_worker) {
      stream_worker.postMessage({
        type: "revoke",
        token: playback_token,
      });
    }
    playback_token = "";
  }

  function parse_inputs() {
    let parsed_url;
    try {
      parsed_url = new URL(url_.value.trim());
    } catch {
      throw new Error("请输入有效的视频 URL");
    }
    if (!["http:", "https:"].includes(parsed_url.protocol)) {
      throw new Error("视频 URL 仅支持 http 或 https");
    }

    const key_text = decode_key_.value.trim();
    if (!/^(?:0x[0-9a-f]+|[0-9]+)$/i.test(key_text)) {
      throw new Error("decodeKey 必须是十进制或 0x 十六进制数字");
    }
    const key = BigInt(key_text);
    if (key < 0n || key > 0xffffffffffffffffn) {
      throw new Error("decodeKey 必须是无符号 64 位整数");
    }
    return { url: parsed_url.href, key };
  }

  async function ensure_stream_worker() {
    if (stream_worker_registration) return stream_worker_registration;
    if (!navigator.serviceWorker) {
      throw new Error("当前环境不支持 Service Worker");
    }
    stream_worker_registration = await navigator.serviceWorker.register(
      stream_worker_url,
      { scope: "./" },
    );
    await navigator.serviceWorker.ready;
    stream_worker = stream_worker_registration.active;
    if (!stream_worker) throw new Error("流式播放 Worker 未激活");
    stream_worker.addEventListener("message", handle_stream_message);
    return stream_worker_registration;
  }

  function handle_stream_message(event) {
    const message = event.data || {};
    if (
      message.type === "error" &&
      message.token === playback_token &&
      message.message
    ) {
      error_.as(message.message);
      status_.as("");
    }
  }

  async function start_stream_playback(input) {
    try {
      await ensure_stream_worker();
      playback_token = create_playback_token();
      stream_worker.postMessage({
        type: "play",
        token: playback_token,
        url: input.url,
        key: input.key,
      });
      const media_url = new URL(
        `wxchannels-stream/${playback_token}`,
        stream_worker_url,
      ).href;
      playback_url_.as(media_url);
      status_.as("流式播放中");
      return true;
    } catch {
      status_.as("流式播放不可用，已回退为完整下载...");
      return false;
    }
  }

  function create_playback_token() {
    return crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`;
  }

  async function download_playback(input, sequence) {
    abort_controller = new AbortController();
    const response = await fetch(input.url, {
      signal: abort_controller.signal,
      cache: "reload",
      mode: "cors",
    });
    if (!response.ok) throw new Error(`视频下载失败：HTTP ${response.status}`);
    const encrypted_data = new Uint8Array(await response.arrayBuffer());
    if (sequence !== request_sequence) return null;

    globalThis.decrypt_wxchannels_data(
      encrypted_data,
      input.key,
      WX_CHANNELS_ENCRYPTED_LENGTH,
    );
    const video_blob = new Blob([encrypted_data], { type: "video/mp4" });
    object_url = URL.createObjectURL(video_blob);
    playback_url_.as(object_url);
    status_.as("解密完成，可以播放");
    return object_url;
  }

  async function submit() {
    if (loading_.value) return null;

    let input;
    try {
      input = parse_inputs();
    } catch (error) {
      error_.as(error.message || String(error));
      return null;
    }

    const sequence = ++request_sequence;
    revoke_playback_url();
    loading_.as(true);
    error_.as("");
    status_.as("正在启动流式播放...");

    try {
      if (await start_stream_playback(input)) return playback_url_.value;
      return await download_playback(input, sequence);
    } catch (error) {
      if (error.name === "AbortError" || sequence !== request_sequence) {
        return null;
      }
      error_.as(
        error.message === "Failed to fetch"
          ? "无法读取视频：该地址需要允许浏览器跨域访问（CORS）"
          : error.message || String(error),
      );
      status_.as("");
      return null;
    } finally {
      if (sequence === request_sequence) {
        loading_.as(false);
        abort_controller = null;
      }
    }
  }

  function destroy() {
    request_sequence += 1;
    abort_controller?.abort();
    abort_controller = null;
    revoke_playback_url();
  }

  function media_error(event) {
    if (error_.value) {
      status_.as("");
      return;
    }
    const code = event.target.error ? event.target.error.code : 0;
    error_.as(
      code === 2
        ? "视频网络读取失败"
        : code === 3
          ? "视频解码失败"
          : "视频无法播放，请检查地址和 decodeKey",
    );
    status_.as("");
  }

  const ui = {
    input_url$: new Timeless.vm.InputCore({
      defaultValue: "",
      placeholder: "输入加密视频 URL",
      type: "url",
      allowClear: true,
      onChange(value) {
        url_.as(value || "");
      },
      onEnter() {
        return submit();
      },
    }),
    input_decode_key$: new Timeless.vm.InputCore({
      defaultValue: "",
      placeholder: "输入 decodeKey",
      allowClear: true,
      onChange(value) {
        decode_key_.as(value || "");
      },
      onEnter() {
        return submit();
      },
    }),
    btn_submit$: new Timeless.vm.ButtonCore({
      disabled: submit_disabled_.value,
      loading: loading_.value,
      variant: "primary",
      onClick() {
        return submit();
      },
    }),
  };

  submit_disabled_.subscribe({
    onChange(disabled) {
      if (disabled) ui.btn_submit$.disable();
      else ui.btn_submit$.enable();
    },
  });
  loading_.subscribe({
    onChange(loading) {
      ui.btn_submit$.setLoading(Boolean(loading));
    },
  });

  return {
    state: {
      error: error_,
      loading: loading_,
      playback_url: playback_url_,
      status: status_,
      submit_text: submit_text_,
    },
    ui,
    methods: { destroy, media_error, submit },
  };
}
