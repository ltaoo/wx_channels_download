import "./wxchannels.crypto.js";
import {
  dispose_wxchannels_player_pool,
  mount_wxchannels_player,
  prepare_wxchannels_player,
} from "../player.js";

const WX_CHANNELS_ENCRYPTED_LENGTH = 131072;

export function WxChannelsPlayerViewModel() {
  const Timeless = window.Timeless;
  const url_ = Timeless.ref("");
  const decode_key_ = Timeless.ref("");
  const playback_url_ = Timeless.ref("");
  const playback_request_ = Timeless.ref(null);
  const stream_playback_ = Timeless.ref(false);
  const loading_ = Timeless.ref(false);
  const error_ = Timeless.ref("");
  const status_ = Timeless.ref("");
  let request_sequence = 0;
  let abort_controller = null;
  let stream_player_session = null;
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
    void unmount_stream_player();
    if (object_url) {
      URL.revokeObjectURL(object_url);
      object_url = "";
    }
    playback_url_.as("");
    playback_request_.as(null);
    stream_playback_.as(false);
  }

  function parse_inputs() {
    return parse_playback_input(url_.value, decode_key_.value);
  }

  function parse_playback_input(raw_url, raw_key) {
    let parsed_url;
    try {
      parsed_url = new URL(String(raw_url || "").trim());
    } catch {
      throw new Error("请输入有效的视频 URL");
    }
    if (!["http:", "https:"].includes(parsed_url.protocol)) {
      throw new Error("视频 URL 仅支持 http 或 https");
    }

    const key_text = String(raw_key || "").trim();
    if (!/^(?:0x[0-9a-f]+|[0-9]+)$/i.test(key_text)) {
      throw new Error("decodeKey 必须是十进制或 0x 十六进制数字");
    }
    const key = BigInt(key_text);
    if (key < 0n || key > 0xffffffffffffffffn) {
      throw new Error("decodeKey 必须是无符号 64 位整数");
    }
    return { url: parsed_url.href, key };
  }

  async function start_stream_playback(input, sequence) {
    try {
      console.log("[wxch-player] preparing stream player...");
      await prepare_wxchannels_player();
      if (sequence !== request_sequence) return true;
      playback_request_.as({ ...input, sequence });
      stream_playback_.as(true);
      playback_url_.as(input.url);
      status_.as("正在初始化流式播放...");
      console.log("[wxch-player] stream playback ready, waiting for video element mount");
      return true;
    } catch (err) {
      console.warn("[wxch-player] stream playback unavailable, falling back to download:", err);
      status_.as("流式播放不可用，已回退为完整下载...");
      return false;
    }
  }

  async function mount_stream_player(event) {
    const sequence = request_sequence;
    const input = playback_request_.value;
    const video = event_target_video(event);
    console.log("[wxch-player] mount_stream_player called", {
      hasVideo: Boolean(video),
      sequence,
      currentSequence: request_sequence,
      hasInput: Boolean(input),
      inputSequence: input?.sequence,
    });
    if (
      !video ||
      sequence !== request_sequence ||
      !input ||
      input.sequence !== sequence
    ) {
      console.warn("[wxch-player] mount_stream_player skipped: precondition failed");
      return null;
    }

    try {
      const session = await mount_wxchannels_player(video, input, {
        onError(error) {
          handle_stream_player_error(sequence, error);
        },
        onFirstSegmentDownload() {
          if (sequence === request_sequence) {
            console.log("[wxch-player] first segment downloaded");
            status_.as("已读取视频数据，正在解密...");
          }
        },
        onFirstSegmentRemux() {
          if (sequence === request_sequence) {
            console.log("[wxch-player] first segment remuxed, playing");
            status_.as("流式播放中");
            loading_.as(false);
          }
        },
      });
      if (sequence !== request_sequence) {
        await session.dispose();
        return null;
      }
      stream_player_session = session;
      console.log("[wxch-player] stream player session mounted");
      return session;
    } catch (error) {
      console.error("[wxch-player] mount_stream_player error:", error);
      if (sequence === request_sequence) {
        handle_stream_player_error(sequence, error);
      }
      return null;
    }
  }

  async function unmount_stream_player() {
    const session = stream_player_session;
    stream_player_session = null;
    await session?.dispose();
  }

  function handle_stream_player_error(sequence, error) {
    if (sequence !== request_sequence) return;
    const error_type = error?.errType || "";
    if (error_type === "NETWORK_TIMEOUT_RETRY") return;

    const message = error?.errMsg || error?.message || String(error);
    console.error("[wxch-player] stream player error:", { error_type, message });
    error_.as(
      error_type.startsWith("NETWORK")
        ? `无法流式读取视频：${message || "该地址需要允许浏览器跨域访问（CORS）"}`
        : `流式播放失败：${message}`,
    );
    status_.as("");
  }

  function event_target_video(event) {
    const target = event?.target;
    if (!target) return null;
    if (typeof target.get$elm === "function") return target.get$elm();
    return target.tagName === "VIDEO" ? target : null;
  }

  async function download_playback(input, sequence) {
    console.log("[wxch-player] download_playback start", { url: input.url, key: String(input.key), sequence });
    abort_controller = new AbortController();
    const response = await fetch(input.url, {
      signal: abort_controller.signal,
      cache: "reload",
      mode: "cors",
    });
    console.log("[wxch-player] fetch response", { status: response.status, ok: response.ok });
    if (!response.ok) throw new Error(`视频下载失败：HTTP ${response.status}`);
    const encrypted_data = new Uint8Array(await response.arrayBuffer());
    console.log("[wxch-player] downloaded bytes:", encrypted_data.byteLength);
    if (sequence !== request_sequence) return null;

    globalThis.decrypt_wxchannels_data(
      encrypted_data,
      input.key,
      WX_CHANNELS_ENCRYPTED_LENGTH,
    );
    const video_blob = new Blob([encrypted_data], { type: "video/mp4" });
    object_url = URL.createObjectURL(video_blob);
    console.log("[wxch-player] blob URL created:", object_url);
    stream_playback_.as(false);
    playback_url_.as(object_url);
    status_.as("解密完成，可以播放");
    return object_url;
  }

  async function submit() {
    try {
      return await play(parse_inputs());
    } catch (error) {
      error_.as(error.message || String(error));
      return null;
    }
  }

  async function play(source) {
    console.log("[wxch-player] play() called", { source, loading: loading_.value });
    if (loading_.value) return null;

    let input;
    try {
      input = parse_playback_input(source.url, source.decode_key ?? source.key);
    } catch (error) {
      console.warn("[wxch-player] parse_playback_input failed:", error.message);
      error_.as(error.message || String(error));
      return null;
    }

    console.log("[wxch-player] parsed input", { url: input.url, key: String(input.key) });
    const sequence = ++request_sequence;
    revoke_playback_url();
    loading_.as(true);
    error_.as("");
    status_.as("正在启动流式播放...");

    try {
      if (await start_stream_playback(input, sequence)) return playback_url_.value;
      return await download_playback(input, sequence);
    } catch (error) {
      console.error("[wxch-player] play() error:", error);
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
        abort_controller = null;
      }
    }
  }

  function destroy() {
    request_sequence += 1;
    abort_controller?.abort();
    abort_controller = null;
    revoke_playback_url();
    void dispose_wxchannels_player_pool();
  }

  function media_error(event) {
    const code = event.target.error ? event.target.error.code : 0;
    console.error("[wxch-player] media_error code=%d, existing error=%s", code, error_.value, event.target.error);
    if (error_.value) {
      status_.as("");
      return;
    }
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
      playback_request: playback_request_,
      stream_playback: stream_playback_,
      status: status_,
      submit_text: submit_text_,
    },
    ui,
    methods: { destroy, media_error, mount_stream_player, play, submit },
  };
}
