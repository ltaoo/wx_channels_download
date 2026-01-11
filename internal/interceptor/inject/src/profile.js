/**
 * @file 用户主页
 */
(() => {
  insert_channels_style();
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
      const $loading = document.createElement("i");
      $loading.className = "weui-loading";
      $btn.prepend($loading);

      const stop_loading = () => {
        $btn.classList.remove("weui-btn_loading");
        $btn.innerText = "批量下载";
        is_running = false;
      };

      try {
        if (!WXU.API.finderUserPage) {
          WXU.error({
            msg: "API 未完成初始化",
          });
          return;
        }
        // if (!my_username) {
        //   WXU.error({
        //     msg: "数据未完成初始化",
        //   });
        //   return;
        // }
        var { href } = window.location;
        if (!href) {
          WXU.error({
            msg: "当前 URL 为空",
          });
          return;
        }
        const queries = WXU.get_queries(href);
        if (!queries.username) {
          WXU.error({
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
          var r = await WXU.API.finderUserPage(payload);
          if (r.errCode !== 0) {
            WXU.error({
              msg: r.errMsg,
              alert: 0,
            });
            has_more = false;
            return;
          }
          const feeds = r.data.object
            .map((obj) => {
              return WXU.format_feed(obj);
            })
            .filter(Boolean);
          var [err, data] = await WXU.downloader.create_batch(feeds, {
            suffix: ".mp4",
          });
          if (err) {
            WXU.error({
              msg: err.message,
            });
            has_more = false;
            return;
          }
          if (!WXU.config.downloadForceCheckAllFeeds && data.ids.length === 0) {
            if (created_task_ids.length === 0) {
              WXU.toast("没有新的视频可以下载");
              WXU.downloader.hide();
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
              WXU.toast("没有新的视频可以下载");
              WXU.downloader.hide();
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
  WXU.observe_node(".opr-area", () => {
    __wx_insert_batch_download_btn();
  });
})();
