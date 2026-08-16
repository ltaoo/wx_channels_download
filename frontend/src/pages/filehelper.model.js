const file_helper_request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(response) {
    if (response.error) {
      return Timeless.Result.Err(response.error);
    }
    const payload = response.data || {};
    if (payload.code !== 0) {
      return Timeless.Result.Err(
        payload.msg || "请求失败",
        payload.code,
        payload.data,
      );
    }
    return Timeless.Result.Ok(payload.data || {});
  },
});

const file_helper_sync_request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(response) {
    if (response.error) {
      return Timeless.Result.Err(response.error);
    }
    return Timeless.Result.Ok(response.data || {});
  },
});

function error_message(error, fallback) {
  if (error && error.message) {
    return error.message;
  }
  return error ? String(error) : fallback;
}

function event_target_element(event) {
  const target = event && event.target;
  return target && typeof target.get$elm === "function"
    ? target.get$elm()
    : target;
}

function decode_and_clean_xml(content) {
  if (!content) {
    return "";
  }
  const textarea = window.document.createElement("textarea");
  textarea.innerHTML = content;
  return textarea.value.replace(/<br\s*\/?>/gi, "").replace(/&amp;/g, "&");
}

function parse_finder_feed(content) {
  try {
    const xml = decode_and_clean_xml(content);
    const document_ = new window.DOMParser().parseFromString(xml, "text/xml");
    if (document_.querySelector("parsererror")) {
      console.error("XML 解析失败:", xml.substring(0, 200));
      return null;
    }

    const finder_feed = document_.querySelector("finderFeed");
    if (!finder_feed) {
      return null;
    }

    function text_content(selector) {
      const element = finder_feed.querySelector(selector);
      return element ? element.textContent.trim() : "";
    }

    const media = finder_feed.querySelector("mediaList media");
    function media_text(selector) {
      const element = media && media.querySelector(selector);
      return element ? element.textContent.trim() : "";
    }

    return {
      nickname: text_content("nickname"),
      avatar: text_content("avatar"),
      desc: text_content("desc"),
      cover_url: media_text("coverUrl"),
      thumb_url: media_text("thumbUrl"),
      video_url: media_text("url"),
      duration: media_text("videoPlayDuration") || "0",
    };
  } catch (error) {
    console.error("解析视频号消息失败:", error);
    return null;
  }
}

function normalize_message(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const message_type = Number(source.MsgType);
  let type = "text";
  let text = source.Content || "";
  let finder_data = null;

  if (message_type === 3) {
    type = "image";
    text = "[图片]";
  } else if (message_type === 49) {
    if (Number(source.AppMsgType) === 6) {
      type = "file";
      text = `[文件: ${source.FileName || "未知文件"}]`;
    } else {
      finder_data = parse_finder_feed(source.Content);
      if (finder_data && finder_data.nickname) {
        type = "finder";
        text = finder_data.desc || "[视频号]";
      } else {
        type = "app";
        text = "[应用消息]";
      }
    }
  } else if (message_type !== 1) {
    type = "unknown";
    text = "[未知消息]";
  }

  return {
    ...source,
    is_mine: source.FromUserName !== "filehelper",
    type,
    text,
    finder_data,
  };
}

function format_message_time(value) {
  const timestamp = Number(value);
  if (!Number.isFinite(timestamp) || timestamp <= 0) {
    return "";
  }
  return new Date(timestamp * 1000).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function FileHelperViewModel(props) {
  const logged_in_ = ref(false);
  const login_stage_ = ref("loading");
  const qrcode_url_ = ref("");
  const scanned_avatar_ = ref("");
  const login_tip_ = ref("请使用微信扫描二维码登录\n登录后可同步接收和发送消息");
  const channels_status_ = ref("idle");
  const messages_ = refarr([]);
  const message_text_ = ref("");
  const sending_ = ref(false);
  const logout_loading_ = ref(false);
  const toast_visible_ = ref(false);
  const toast_message_ = ref("");

  const connection_text_ = computed(logged_in_, (logged_in) =>
    logged_in ? "已连接" : "未登录",
  );
  const channels_status_text_ = computed(channels_status_, (status) => {
    const labels = {
      checking: "检测中...",
      available: "可用",
      unavailable: "不可用",
      error: "检测失败",
    };
    return labels[status] || "";
  });
  const send_disabled_ = combine(
    { text: message_text_, sending: sending_ },
    (state) => state.sending || !String(state.text || "").trim(),
  );

  const ui = {
    btn_logout$: new Timeless.vm.ButtonCore({
      disabled: logout_loading_.value,
      loading: logout_loading_.value,
      variant: "ghost",
      size: "sm",
      onClick() {
        return logout();
      },
    }),
    btn_refresh_qrcode$: new Timeless.vm.ButtonCore({
      variant: "primary",
      onClick() {
        return refresh_qrcode();
      },
    }),
    input_message$: new Timeless.vm.InputCore({
      defaultValue: message_text_.value,
      disabled: sending_.value,
      allowClear: false,
      ignoreEnterEvent: true,
      placeholder: "输入消息...",
      onChange(value) {
        message_text_.as(value);
      },
      onKeyDown(event) {
        handle_message_key_down(event);
      },
    }),
    btn_open_image_picker$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      size: "icon-sm",
      onClick() {
        open_image_picker();
      },
    }),
    btn_open_file_picker$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      size: "icon-sm",
      onClick() {
        open_file_picker();
      },
    }),
    btn_send_message$: new Timeless.vm.ButtonCore({
      disabled: send_disabled_.value,
      loading: sending_.value,
      variant: "primary",
      onClick() {
        return send_message();
      },
    }),
    input_image_file$: new Timeless.vm.InputCore({
      defaultValue: null,
      type: "file",
      allowClear: false,
    }),
    input_file$: new Timeless.vm.InputCore({
      defaultValue: null,
      type: "file",
      allowClear: false,
    }),
  };

  function sync_button_disabled(button, disabled) {
    if (button.state.disabled === Boolean(disabled)) {
      return;
    }
    if (disabled) {
      button.disable();
    } else {
      button.enable();
    }
  }

  message_text_.subscribe({
    onChange(value) {
      if (ui.input_message$.value !== value) {
        ui.input_message$.setValue(value, { silence: true });
      }
    },
  });
  sending_.subscribe({
    onChange(sending) {
      ui.input_message$.disabled = Boolean(sending);
      ui.input_message$.emitStateChange?.();
      ui.input_message$.setValue(ui.input_message$.value, { silence: true });
      ui.btn_send_message$.setLoading(Boolean(sending));
    },
  });
  send_disabled_.subscribe({
    onChange(disabled) {
      sync_button_disabled(ui.btn_send_message$, disabled);
    },
  });
  logout_loading_.subscribe({
    onChange(loading) {
      sync_button_disabled(ui.btn_logout$, loading);
      ui.btn_logout$.setLoading(Boolean(loading));
    },
  });

  const status_request = new Timeless.kit.RequestCore(
    () => file_helper_request.get("/api/filehelper/status"),
    { client: props.client },
  );
  const qrcode_request = new Timeless.kit.RequestCore(
    () => file_helper_request.get("/api/filehelper/qrcode"),
    { client: props.client },
  );
  const login_wait_request = new Timeless.kit.RequestCore(
    () => file_helper_request.get("/api/filehelper/login/wait"),
    { client: props.client },
  );
  const channels_status_request = new Timeless.kit.RequestCore(
    () => file_helper_request.get("/api/status"),
    { client: props.client },
  );
  const sync_check_request = new Timeless.kit.RequestCore(
    () => file_helper_request.get("/api/filehelper/synccheck"),
    { client: props.client },
  );
  const sync_request = new Timeless.kit.RequestCore(
    () => file_helper_sync_request.get("/api/filehelper/sync"),
    { client: props.client },
  );
  const send_request = new Timeless.kit.RequestCore(
    (params) => file_helper_request.post("/api/filehelper/send", params),
    { client: props.client },
  );
  const logout_request = new Timeless.kit.RequestCore(
    () => file_helper_request.post("/api/filehelper/logout"),
    { client: props.client },
  );

  let active = true;
  let login_polling = false;
  let sync_polling = false;
  let login_sequence = 0;
  let sync_sequence = 0;
  let qrcode_sequence = 0;
  let toast_timer = 0;
  let scroll_frame = 0;
  let message_list_element = null;
  let image_input_element = null;
  let file_input_element = null;
  const pause_timers = new Map();

  function pause(duration) {
    return new Promise((resolve) => {
      const timer = window.setTimeout(() => {
        pause_timers.delete(timer);
        resolve();
      }, duration);
      pause_timers.set(timer, resolve);
    });
  }

  function scroll_to_latest_message() {
    if (scroll_frame) {
      window.cancelAnimationFrame(scroll_frame);
    }
    scroll_frame = window.requestAnimationFrame(() => {
      scroll_frame = 0;
      if (message_list_element) {
        message_list_element.scrollTop = message_list_element.scrollHeight;
      }
    });
  }

  function show_toast(message, duration = 2000) {
    if (toast_timer) {
      window.clearTimeout(toast_timer);
    }
    toast_message_.as(message);
    toast_visible_.as(true);
    toast_timer = window.setTimeout(() => {
      toast_timer = 0;
      toast_visible_.as(false);
    }, duration);
  }

  function stop_login_polling() {
    login_sequence += 1;
    login_polling = false;
  }

  function stop_sync_polling() {
    sync_sequence += 1;
    sync_polling = false;
  }

  function show_login_section() {
    stop_sync_polling();
    logged_in_.as(false);
    channels_status_.as("idle");
  }

  async function check_channels_status() {
    channels_status_.as("checking");
    const result = await channels_status_request.run();
    if (!active || !logged_in_.value) {
      return result;
    }
    if (result.error) {
      channels_status_.as("error");
      return result;
    }
    const channels = result.data && result.data.channels;
    if (!channels || typeof channels.available !== "boolean") {
      channels_status_.as("error");
      return result;
    }
    channels_status_.as(
      channels.available ? "available" : "unavailable",
    );
    return result;
  }

  function show_chat_section() {
    stop_login_polling();
    logged_in_.as(true);
    check_channels_status();
    start_sync_check_polling();
  }

  async function refresh_qrcode() {
    stop_login_polling();
    const sequence = ++qrcode_sequence;
    login_stage_.as("loading");
    qrcode_url_.as("");
    scanned_avatar_.as("");
    login_tip_.as(
      "请使用微信扫描二维码登录\n登录后可同步接收和发送消息",
    );

    const result = await qrcode_request.run();
    if (!active || sequence !== qrcode_sequence || logged_in_.value) {
      return result;
    }
    if (result.error) {
      login_stage_.as("error");
      show_toast(`获取二维码失败: ${error_message(result.error, "请求失败")}`);
      return result;
    }

    qrcode_url_.as(result.data.qrcode_url || "");
    login_stage_.as("waiting");
    start_wait_login();
    return result;
  }

  async function start_wait_login() {
    if (login_polling || !active || logged_in_.value) {
      return;
    }
    login_polling = true;
    const sequence = ++login_sequence;

    while (
      active &&
      login_polling &&
      sequence === login_sequence &&
      !logged_in_.value
    ) {
      const result = await login_wait_request.run();
      if (!active || sequence !== login_sequence) {
        break;
      }
      if (result.error) {
        console.error("等待登录失败:", result.error);
        await pause(2000);
        continue;
      }

      const status = result.data && result.data.status;
      if (status === "waiting") {
        login_stage_.as("waiting");
        await pause(2000);
        continue;
      }
      if (status === "scanned") {
        scanned_avatar_.as(result.data.user_avatar || "");
        login_stage_.as("scanned");
        login_tip_.as("请在手机上确认登录");
        await pause(2000);
        continue;
      }
      if (status === "expired") {
        login_stage_.as("expired");
        break;
      }
      if (status === "logged_in") {
        show_toast("登录成功");
        show_chat_section();
        break;
      }
      await pause(2000);
    }

    if (sequence === login_sequence) {
      login_polling = false;
    }
  }

  function append_messages(raw_messages) {
    const next = messages_.value.slice();
    const ids = new Set(next.map((message) => String(message.MsgId || "")));

    for (const raw of raw_messages) {
      if (
        !raw ||
        (raw.FromUserName !== "filehelper" && raw.ToUserName !== "filehelper")
      ) {
        continue;
      }
      const id = String(raw.MsgId || "");
      if (id && ids.has(id)) {
        continue;
      }
      if (id) {
        ids.add(id);
      }
      next.push(normalize_message(raw));
    }

    messages_.as(next.slice(-200), { reset: true });
    scroll_to_latest_message();
  }

  async function fetch_and_render_messages() {
    const result = await sync_request.run();
    if (!active || !logged_in_.value) {
      return result;
    }
    if (result.error) {
      console.error("获取消息失败:", result.error);
      return result;
    }
    const response = result.data || {};
    if (response.BaseResponse && response.BaseResponse.Ret !== 0) {
      console.error("同步消息失败:", response);
      return result;
    }
    const messages = Array.isArray(response.AddMsgList)
      ? response.AddMsgList
      : [];
    if (messages.length > 0) {
      append_messages(messages);
    }
    return result;
  }

  async function start_sync_check_polling() {
    if (sync_polling || !active || !logged_in_.value) {
      return;
    }
    sync_polling = true;
    const sequence = ++sync_sequence;

    while (
      active &&
      sync_polling &&
      sequence === sync_sequence &&
      logged_in_.value
    ) {
      const result = await sync_check_request.run();
      if (!active || sequence !== sync_sequence) {
        break;
      }
      if (result.error) {
        const status_result = await status_request.run();
        if (!active || sequence !== sync_sequence) {
          break;
        }
        if (
          status_result.error ||
          (status_result.data && status_result.data.logged_in)
        ) {
          await pause(2000);
          continue;
        }
        show_login_section();
        refresh_qrcode();
        break;
      }

      const status = result.data && result.data.status;
      if (status === "hasMsg") {
        await fetch_and_render_messages();
      } else if (status === "logout") {
        show_toast("登录已过期");
        show_login_section();
        refresh_qrcode();
        break;
      }
    }

    if (sequence === sync_sequence) {
      sync_polling = false;
    }
  }

  async function check_status() {
    const result = await status_request.run();
    if (!active) {
      return result;
    }
    if (!result.error && result.data && result.data.logged_in) {
      show_chat_section();
    } else {
      show_login_section();
      refresh_qrcode();
    }
    return result;
  }

  async function send_message() {
    const text = String(message_text_.value || "").trim();
    if (!text || sending_.value) {
      return null;
    }
    sending_.as(true);
    const result = await send_request.run({ text });
    sending_.as(false);
    if (!active) {
      return result;
    }
    if (result.error) {
      show_toast(`发送失败: ${error_message(result.error, "请求失败")}`);
      return result;
    }

    message_text_.as("");
    append_messages([
      {
        MsgId: String(Date.now()),
        MsgType: 1,
        Content: text,
        FromUserName: "self",
        ToUserName: "filehelper",
        CreateTime: Math.floor(Date.now() / 1000),
      },
    ]);
    return result;
  }

  function handle_message_key_down(event) {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      send_message();
    }
  }

  function open_image_picker() {
    if (image_input_element) {
      image_input_element.click();
    }
  }

  function open_file_picker() {
    if (file_input_element) {
      file_input_element.click();
    }
  }

  async function logout() {
    if (
      logout_loading_.value ||
      !window.confirm("确定要退出登录吗？")
    ) {
      return null;
    }
    logout_loading_.as(true);
    const result = await logout_request.run();
    logout_loading_.as(false);
    if (!active) {
      return result;
    }
    if (result.error) {
      show_toast(`退出失败: ${error_message(result.error, "请求失败")}`);
      return result;
    }

    stop_login_polling();
    stop_sync_polling();
    messages_.as([], { reset: true });
    show_login_section();
    refresh_qrcode();
    return result;
  }

  function destroy() {
    active = false;
    stop_login_polling();
    stop_sync_polling();
    qrcode_sequence += 1;
    if (toast_timer) {
      window.clearTimeout(toast_timer);
    }
    if (scroll_frame) {
      window.cancelAnimationFrame(scroll_frame);
    }
    for (const [timer, resolve] of pause_timers) {
      window.clearTimeout(timer);
      resolve();
    }
    pause_timers.clear();
    message_list_element = null;
    image_input_element = null;
    file_input_element = null;
  }

  const methods = {
    ready() {
      props.app.setTitle("微信文件传输助手");
      return check_status();
    },
    destroy,
    refreshQRCode: refresh_qrcode,
    sendMessage: send_message,
    logout,
    setMessageText(value) {
      message_text_.as(value);
    },
    handleMessageKeyDown: handle_message_key_down,
    setMessageListElement(event) {
      message_list_element = event ? event_target_element(event) : null;
      if (message_list_element) {
        scroll_to_latest_message();
      }
    },
    setImageInputElement(event) {
      image_input_element = event ? event_target_element(event) : null;
    },
    setFileInputElement(event) {
      file_input_element = event ? event_target_element(event) : null;
    },
    openImagePicker: open_image_picker,
    openFilePicker: open_file_picker,
    handleImageSelect(event) {
      const target = event_target_element(event);
      if (target && target.files && target.files[0]) {
        show_toast("图片发送功能开发中...");
        target.value = "";
      }
    },
    handleFileSelect(event) {
      const target = event_target_element(event);
      if (target && target.files && target.files[0]) {
        show_toast("文件发送功能开发中...");
        target.value = "";
      }
    },
    formatMessageTime: format_message_time,
  };

  const state = {
    logged_in: logged_in_,
    login_stage: login_stage_,
    qrcode_url: qrcode_url_,
    scanned_avatar: scanned_avatar_,
    login_tip: login_tip_,
    channels_status: channels_status_,
    channels_status_text: channels_status_text_,
    connection_text: connection_text_,
    messages: messages_,
    message_text: message_text_,
    sending: sending_,
    send_disabled: send_disabled_,
    logout_loading: logout_loading_,
    toast_visible: toast_visible_,
    toast_message: toast_message_,
  };

  return { state, ui, methods };
}

export {
  FileHelperViewModel,
  event_target_element,
};
