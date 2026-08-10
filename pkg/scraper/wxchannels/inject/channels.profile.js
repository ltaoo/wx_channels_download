/**
 * @file 用户主页
 */
(() => {
  var my_username = "";
  function __wx_insert_batch_download_btn() {
    const $operation = document.querySelector(".opr-area");
    if (!$operation) {
      return false;
    }
    const $btn = document.createElement("button");
    $btn.className = "button h-7 ml-2 weui-btn weui-btn_default weui-btn_mini";
    $btn.innerText = "批量下载";

    let is_running = false;
    let stop_signal = false;

    $btn.onclick = async () => {
      if (is_running) {
        stop_signal = true;
        $btn.innerText = "正在取消...";
        return;
      }
      is_running = true;
      stop_signal = false;

      $btn.innerText = "点击取消";
      $btn.classList.add("weui-btn_loading");
      // const $loading = document.createElement("i");
      // $loading.className = "weui-loading";
      // $btn.prepend($loading);

      const stop_loading = () => {
        $btn.classList.remove("weui-btn_loading");
        $btn.innerText = "批量下载";
        is_running = false;
      };

      try {
        if (!WXU.API.finderUserPage) {
          WXU.error({
            source: "channels.profile.js:41",
            msg: "API 未完成初始化",
          });
          return;
        }
        var { href } = window.location;
        if (!href) {
          WXU.error({ source: "channels.profile.js:48", msg: "当前 URL 为空" });
          return;
        }
        const queries = WXU.get_queries(href);
        WXU.log
          .Info()
          .Str("file", "channels.profile.js")
          .Str("href", href)
          .Msg("check has username in href");
        if (!queries.username) {
          WXU.error({
            source: "channels.profile.js:60",
            msg: "username 不能为空",
          });
          return;
        }
        let download_open = false;
        let next_marker = "";
        let has_more = true;
        let created_task_ids = [];
        while (has_more) {
          if (stop_signal) {
            has_more = false;
            break;
          }
          var payload = {
            username: queries.username,
            finderUsername: my_username || queries.username,
            lastBuffer: next_marker,
            needFansCount: 0,
            objectId: "0",
          };
          WXU.log
            .Info()
            .Str("file", "channels.profile.js")
            .JSON("payload", payload)
            .Msg("before WXU.API.finderUserPage");
          var r = await WXU.API.finderUserPage(payload);
          if (r.errCode !== 0) {
            WXU.error({
              source: "channels.profile.js:88",
              msg: r.errMsg,
              alert: 0,
            });
            has_more = false;
            return;
          }
          const feeds = r.data.object;
          var [err, data] = await WXU.downloader.create(feeds, {
            platform: "wxchannels",
            ignore_live_feed: true,
          });
          if (err) {
            WXU.error({ source: "channels.profile.js:107", msg: err.message });
            has_more = false;
            return;
          }
          if (!WXU.config.downloadForceCheckAllFeeds && data.ids.length === 0) {
            if (created_task_ids.length === 0) {
              return;
            }
            continue;
          }
          created_task_ids.push(...data.ids);
          if (!download_open) {
            download_open = true;
            WXU.downloader.show();
          }
          if (!r.data.lastBuffer || r.data.object.length < 15) {
            has_more = false;
            if (created_task_ids.length === 0) {
              return;
            }
            return;
          }
          next_marker = r.data.lastBuffer;
        }
      } finally {
        stop_loading();
      }
    };
    $operation.appendChild($btn);
    return true;
  }
  WXU.onInit((data) => {
    my_username = data.mainFinderUsername;
  });
  WXU.observe_node({
    selector: ".opr-area",
    container: "#app",
    onOk: () => {
      __wx_insert_batch_download_btn();
    },
  });
})();
