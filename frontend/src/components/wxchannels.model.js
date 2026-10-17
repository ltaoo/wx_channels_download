function wxchannels_play_url(input) {
  const base = window.API_ORIGIN || window.location.origin;
  const url = new URL("/play", base);
  url.searchParams.set("url", input.url);
  url.searchParams.set("key", String(input.key));
  return url.href;
}

export function WxChannelsPlayerViewModel() {
  const Timeless = window.Timeless;
  const url_ = Timeless.ref("");
  const decode_key_ = Timeless.ref("");
  const playback_url_ = Timeless.ref("");
  const loading_ = Timeless.ref(false);
  const error_ = Timeless.ref("");
  const status_ = Timeless.ref("");

  const submit_disabled_ = Timeless.combine(
    { url: url_, decode_key: decode_key_, loading: loading_ },
    (state) =>
      !state.url.trim() || !state.decode_key.trim() || Boolean(state.loading),
  );
  const submit_text_ = Timeless.combine({ loading: loading_ }, (state) =>
    state.loading ? "加载中..." : "播放",
  );

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

  async function submit() {
    try {
      return await play(parse_inputs());
    } catch (error) {
      error_.as(error.message || String(error));
      return null;
    }
  }

  async function play(source) {
    let input;
    try {
      input = parse_playback_input(source.url, source.decode_key ?? source.key);
    } catch (error) {
      error_.as(error.message || String(error));
      return null;
    }

    playback_url_.as("");
    error_.as("");
    status_.as("正在加载视频...");
    playback_url_.as(wxchannels_play_url(input));
    return playback_url_.value;
  }

  function media_mounted(event) {
    const target = event?.target;
    const video =
      typeof target?.get$elm === "function"
        ? target.get$elm()
        : target?.tagName === "VIDEO"
          ? target
          : null;
    video?.addEventListener("playing", () => status_.as(""), { once: true });
  }

  function destroy() {
    playback_url_.as("");
    error_.as("");
    status_.as("");
  }

  function media_error(event) {
    const code = event.target.error ? event.target.error.code : 0;
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
      status: status_,
      submit_text: submit_text_,
    },
    ui,
    methods: { destroy, media_error, media_mounted, play, submit },
  };
}
