function getShortUri(data) {
  var u = new URL(decodeURIComponent(data.url));
  var pathname = u.pathname;
  var m = pathname.match(/\/sph\/([a-zA-Z0-9]{1,})/);
  if (m) {
    return m[1];
  }
  return u.searchParams.get("id");
}
async function fetchExportIdWithShareId(data) {
  if (!data.url) {
    return [new Error("missing url"), null];
  }
  var uri = getShortUri(data);
  if (!uri) {
    return [new Error("can't get the uri from url, " + data.url), null];
  }
  await WXU.load_script(__wx_asset_url("/lib/axios.min.js"));
  await WXU.load_script(__wx_asset_url("/lib/getFeedInfo.js"));
  // await WXU.load_script(__wx_asset_url("/lib/merlin.js"));
  if (typeof getFeedInfo !== "function") {
    return [new Error("the getFeedInfo is not a function"), null];
  }
  var payload = {
    baseReq: {
      generalToken: "",
    },
    shortUri: uri,
  };
  /** @type {SharedFeedProfileResp} */
  try {
    var shared = await getFeedInfo(payload);
    if (shared.data) {
      if (shared.data.sceneInfo) {
        if (shared.data.sceneInfo.dynamicExportId) {
          return [null, shared.data.sceneInfo.dynamicExportId];
        }
        return [new Error("missing 'sceneInfo.dynamicExportId'"), null];
      }
      if (shared.data.errMsg) {
        if (shared.data.errMsg.title) {
          return [new Error(shared.data.errMsg.title), null];
        }
      }
    }
    return [new Error("getFeedInfo failed"), null];
  } catch (err) {
    return [err, null];
  }
}
async function fetchFeedProfileWith(data) {
  if (data.url) {
    if (data.url.match(/sph/)) {
      var [err, eid] = await fetchExportIdWithShareId(data);
      if (err) {
        var m = data.url.match(/\/([a-zA-Z0-9]{1,})$/);
        if (m[1]) {
          data.eid = m[1];
        } else {
          return [err, null];
        }
      } else {
        data.eid = eid;
      }
    } else {
      var u = new URL(decodeURIComponent(data.url));
      data.oid = WXU.API.decodeBase64ToUint64String(u.searchParams.get("oid"));
      data.nid = WXU.API.decodeBase64ToUint64String(u.searchParams.get("nid"));
    }
  }
  let payload = {
    needObject: 1,
    lastBuffer: "",
    scene: data.eid ? 141 : 146,
    direction: 2,
    identityScene: 2,
    pullScene: 6,
    objectid: (() => {
      if (data.eid) {
        return undefined;
      }
      if (data.oid.includes("_")) {
        return data.oid.split("_")[0];
      }
      return data.oid;
    })(),
    objectNonceId: data.eid ? undefined : data.nid,
    encrypted_objectid: data.eid || "",
  };
  if (data.eid) {
    payload.traceBuffer = undefined;
  }
  try {
    var r = await WXU.API.finderGetCommentDetail(payload);
    return [null, r, payload];
  } catch (err) {
    return [err, null, null];
  }
}

function ChannelsWebsocketClient() {
  const methods = {
    connect_local_ws() {
      const ws_url = WXEnv.channelsLocalWSURL;
      const ws = new WebSocket(ws_url);
      ws.onclose = (e) => {
        WXU.error({
          msg: `channels ws连接已关闭，reason: ${e.reason}，code: ${e.code}`,
        });
      };
      ws.onerror = (e) => {
        WXU.error({ msg: "channels ws连接发生错误，" + JSON.stringify(e) });
      };
      ws.onmessage = (ev) => {
        const [err, msg] = WXU.parseJSON(ev.data);
        if (err) {
          return;
        }
        if (msg.type === "api_call") {
          this.__wx_handle_api_call(msg.data, ws);
        }
      };
    },
    async __wx_handle_api_call(msg, socket) {
      var { id, key, data } = msg;
      console.log("[DOWNLOADER]__wx_handle_api_call", id, key, data);
      function resp(body) {
        socket.send(
          JSON.stringify({
            id,
            data: body,
          }),
        );
      }
      if (key === "key:channels:contact_list") {
        let payload = {
          query: data.keyword,
          scene: 13,
          lastBuff: data.next_marker
            ? decodeURIComponent(data.next_marker)
            : "",
          requestId: String(new Date().valueOf()),
        };
        var r = await WXU.API2.finderSearch(payload);
        console.log("[DOWNLOADER]finderSearch", r, payload);
        /** @type {SearchResp} */
        var { infoList, objectList } = r.data;
        resp({
          ...r,
          payload,
        });
        return;
      }
      if (key === "key:channels:feed_list") {
        let payload = {
          username: data.username,
          finderUsername: __wx_username,
          lastBuffer: data.next_marker
            ? decodeURIComponent(data.next_marker)
            : "",
          needFansCount: 0,
          objectId: "0",
        };
        let r = await WXU.API.finderUserPage(payload);
        console.log("[DOWNLOADER]finderUserPage", r);
        /** @type {ChannelsObject[]} */
        const object = r.data.object || [];
        resp({
          ...r,
          payload,
        });
        return;
      }
      if (key === "key:channels:live_replay_list") {
        let payload = {
          username: data.username,
          finderUsername: __wx_username || data.username,
          lastBuffer: data.next_marker
            ? decodeURIComponent(data.next_marker)
            : "",
          needFansCount: 0,
          objectId: "0",
        };
        var r = await WXU.API3.finderLiveUserPage(payload);
        console.log("[DOWNLOADER]finderLiveUserPage", r);
        resp({
          ...r,
          payload,
        });
        return;
      }
      if (key === "key:channels:interactioned_list") {
        let payload = {
          lastBuffer: data.next_marker
            ? decodeURIComponent(data.next_marker)
            : "",
          tabFlag: data.flag ? Number(data.flag) : 7,
        };
        var r = await WXU.API4.finderGetInteractionedFeedList(payload);
        console.log("[DOWNLOADER]finderGetInteractionedFeedList", r);
        resp({
          ...r,
          payload,
        });
        return;
      }
      if (key === "key:channels:feed_profile") {
        console.log("before finderGetCommentProfile", data);
        var [err, r, payload] = await fetchFeedProfileWith(data);
        if (err) {
          resp({
            errCode: 1011,
            errMsg: err.message,
            payload: null,
          });
          return;
        }
        /** @type {MediaProfileResp} */
        var { object } = r.data;
        resp({
          ...r,
          payload,
        });
        return;
      }
      if (key === "key:channels:fetch_feed_comment_list") {
        // console.log("[DOWNLOADER]key:channels:fetch_feed_comment_list");
        if (!data.oid) {
          resp({
            errCode: 1011,
            errMsg: "missing oid",
            payload: null,
          });
          return;
        }
        if (!data.nid && !data.comment_id) {
          resp({
            errCode: 1011,
            errMsg: "missing nid or comment_id",
            payload: null,
          });
          return;
        }
        try {
          var payload = data.comment_id
            ? {
                direction: 2,
                identityScene: 2,
                objectId: data.oid,
                lastBuffer:
                  data.next_marker === "" ? undefined : data.next_marker,
                rootCommentId: data.comment_id,
              }
            : {
                finderBasereq: {
                  scene: 140,
                  ctxInfo: {
                    clientReportBuff: '{"entranceId":"1002"}',
                  },
                  objectBaseInfos: [],
                },
                objectId: data.oid,
                direction: 2,
                objectNonceId: data.nid,
                identityScene: 2,
                lastBuffer:
                  data.next_marker === "" ? undefined : data.next_marker,
                enterSessionId: String(Date.now()),
              };
          var r = await WXU.API.finderGetCommentList(payload);
          resp({
            ...r,
            payload,
          });
        } catch (err) {
          resp({
            errCode: 1011,
            errMsg: err.message,
            payload: null,
          });
        }
        return;
      }
      if (key === "key:channels:feed_share_url") {
        // console.log("[DOWNLOADER]fetchFeedShareUrl");
        if (!data.oid) {
          resp({
            errCode: 1011,
            errMsg: "missing oid",
            payload: null,
          });
          return;
        }
        var payload = {
          objectId: data.oid,
        };
        try {
          var r = await WXU.API.finderGetFeedH5Url(payload);
          resp({
            ...r,
            payload,
          });
        } catch (err) {
          resp({
            errCode: 1011,
            errMsg: err.message,
            payload,
          });
        }
        return;
      }
      if (key === "key:channels:reload") {
        console.log("[DOWNLOADER]reloading page");
        resp({
          msg: "reloading",
        });
        setTimeout(() => {
          window.location.reload();
        }, 500);
        return;
      }
      resp({
        errCode: 1000,
        errMsg: "未匹配的key",
        payload: msg,
      });
      return;
    },
  };
  return {
    methods,
  };
}

var ws_client$ = ChannelsWebsocketClient();
ws_client$.methods.connect_local_ws();
