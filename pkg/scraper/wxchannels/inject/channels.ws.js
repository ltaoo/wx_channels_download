var ChannelsWSURL = ChannelsEnv.get("channelsWSURL");

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
  await WXU.load_script(WXEnv.assetUrl("/public/axios.min.js"));
  await WXU.load_script(WXEnv.assetUrl("/wxchannels/inject/getFeedInfo.js"));
  // await WXU.load_script(WXEnv.assetUrl("/public/merlin.js"));
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
    if (data.url.match(/\/sph/)) {
      var [err, eid] = await fetchExportIdWithShareId(data);
      if (err) {
        var m = data.url.match(/\/([a-zA-Z0-9]{1,})$/);
        if (m && m[1]) {
          data.eid = m[1];
        } else {
          return [err, null];
        }
      } else {
        data.eid = eid;
      }
    } else {
      try {
        var u = new URL(decodeURIComponent(data.url), window.location.origin);
        console.log(
          "WXU.API.decodeBase64ToUint64String",
          WXU.Base64,
          WXU.API.decodeBase64ToUint64String,
        );
        data.oid = WXU.Base64.decodeBase64ToUint64String(
          u.searchParams.get("oid"),
        );
        data.nid = WXU.Base64.decodeBase64ToUint64String(
          u.searchParams.get("nid"),
        );
      } catch (parseErr) {
        return [
          new Error("failed to parse feed URL: " + parseErr.message),
          null,
        ];
      }
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
  const WEBSOCKET_RETRY_INTERVAL = 5000;
  const state = {
    connection_status: "disconnected",
  };
  let websocket_ = null;
  let connection_promise_ = null;
  let retry_timer_ = null;

  function set_connection_status(status) {
    state.connection_status = status;
  }

  function cancel_retry() {
    if (retry_timer_ === null) {
      return;
    }
    clearTimeout(retry_timer_);
    retry_timer_ = null;
  }

  function schedule_retry() {
    if (state.connection_status === "connected" || retry_timer_ !== null) {
      return;
    }
    retry_timer_ = setTimeout(() => {
      retry_timer_ = null;
      methods.connect_local_ws().catch(() => {});
    }, WEBSOCKET_RETRY_INTERVAL);
  }

  const methods = {
    connect_local_ws() {
      if (
        state.connection_status === "connected" &&
        websocket_ &&
        websocket_.readyState === WebSocket.OPEN
      ) {
        return Promise.resolve(true);
      }
      if (connection_promise_) {
        return connection_promise_;
      }
      cancel_retry();
      set_connection_status("connecting");
      const connection_promise = new Promise((resolve, reject) => {
        let ws;
        try {
          ws = new WebSocket(ChannelsWSURL);
        } catch (error) {
          set_connection_status("disconnected");
          schedule_retry();
          reject(error);
          return;
        }
        websocket_ = ws;
        let opened = false;
        ws.onopen = () => {
          if (websocket_ !== ws) {
            ws.close();
            return;
          }
          opened = true;
          set_connection_status("connected");
          cancel_retry();
          resolve(true);
        };
        ws.onclose = (e) => {
          if (websocket_ !== ws) {
            return;
          }
          websocket_ = null;
          set_connection_status("disconnected");
          schedule_retry();
          if (opened) {
            WXU.error({
              source: "channels.ws.js:onclose",
              msg: `channels ws连接已关闭，reason: "${e.reason}"，code: "${e.code}"`,
            });
            WXU.log.flushNow();
            return;
          }
          reject(new Error("channels websocket connection closed"));
        };
        ws.onerror = (e) => {
          if (websocket_ !== ws) {
            return;
          }
          set_connection_status("disconnected");
          schedule_retry();
          if (!opened) {
            websocket_ = null;
            reject(
              e instanceof Error
                ? e
                : new Error("channels websocket connection failed"),
            );
            try {
              ws.close();
            } catch (error) {}
          }
        };
        ws.onmessage = (ev) => {
          const [err, msg] = WXU.parseJSON(ev.data);
          if (err) {
            return;
          }
          if (msg.type === "api_call") {
            methods.__wx_handle_api_call(msg.data, ws);
          }
        };
      });
      connection_promise_ = connection_promise;
      connection_promise.then(
        () => {
          if (connection_promise_ === connection_promise) {
            connection_promise_ = null;
          }
        },
        () => {
          if (connection_promise_ === connection_promise) {
            connection_promise_ = null;
          }
        },
      );
      return connection_promise;
    },
    reconnect_local_ws() {
      cancel_retry();
      return methods.connect_local_ws();
    },
    async __wx_handle_api_call(msg, socket) {
      var { id, key, data } = msg;
      WXU.log
        .Info()
        .Str("file", "/channels.ws.js")
        .Str("id", String(id))
        .Str("key", key)
        .JSON("data", data)
        .Msg("__wx_handle_api_call");
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
      if (key === "key:channels:follow_list") {
        let payload = {
          finderUsername: __wx_username,
          lastBuffer: data.next_marker
            ? decodeURIComponent(data.next_marker)
            : "",
        };
        try {
          var r = await WXU.API4.finderGetFollowList(payload);
          console.log("[DOWNLOADER]finderGetFollowList", r, payload);
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
      if (key === "key:channels:play_history") {
        let payload = {
          finderUsername: __wx_username,
          lastBuffer: data.next_marker
            ? decodeURIComponent(data.next_marker)
            : "",
        };
        try {
          var r = await WXU.API4.finderGetPlayHistory(payload);
          console.log("[DOWNLOADER]finderGetPlayHistory", r, payload);
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
      if (key === "key:channels:live_info") {
        var payload = {
          clientStatus: {
            videoDecoderSupportMask: 1,
          },
          finderUsername: data.username,
          liveId: data.id,
          objectId: data.oid,
          objectNonceId: data.nid,
          scene: 2,
        };
        var liveAPI = WXU.LiveAPI;
        if (!liveAPI || typeof liveAPI.joinLive !== "function") {
          resp({
            errCode: 1011,
            errMsg: "joinLive API is unavailable",
            payload,
          });
          return;
        }
        try {
          var r = await liveAPI.joinLive(payload);
          console.log("[DOWNLOADER]joinLive", r, payload);
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
    state,
    methods,
  };
}

var ws_client$ = ChannelsWebsocketClient();
ws_client$.methods.connect_local_ws().catch(() => {});
