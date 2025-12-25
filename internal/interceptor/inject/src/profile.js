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
    $btn.onclick = async () => {
      if (!WXU.API.finderUserPage) {
        WXU.error({
          msg: "API 未完成初始化",
        });
        return;
      }
      if (!my_username) {
        WXU.error({
          msg: "数据未完成初始化",
        });
        return;
      }
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
      const AllFeedsOfContact = [];
      let next_marker = "";
      let has_more = true;
      while (has_more) {
        var payload = {
          username: queries.username,
          finderUsername: my_username,
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
        AllFeedsOfContact.push(...r.data.object);
        if (
          !r.data.lastBuffer ||
          r.data.object.length < 15 ||
          r.data.object.length === 0
        ) {
          console.log("All feeds", AllFeedsOfContact);
          has_more = false;
          return;
        }
        next_marker = r.data.lastBuffer;
      }
    };
    $operation.appendChild($btn);
    return true;
  }
  WXU.onInit((data) => {
    my_username = data.mainFinderUsername;
  });
  setTimeout(() => {
    if (window.location.pathname !== "/web/pages/profile") {
      return;
    }
    const success = __wx_insert_batch_download_btn();
    if (success) {
      return;
    }
    WXU.error({
      msg: "插入下载按钮失败",
    });
  }, 3000);
})();
