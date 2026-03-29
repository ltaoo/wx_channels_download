/**
 * @file 所有的工具函数 + API + 事件总线
 */
var FakeLocalAPIServerAddr = "localapi.weixin.qq.com";
var FakeRemoteAPIServerAddr = "remoteapi.weixin.qq.com";
var FakeOfficialAccountServerAddr = "official.weixin.qq.com";
var FakeRemoteAPIServerProtocol = "https";
var FakeLocalAPIServerProtocol = "https";
var WSServerProtocol = "wss";
if (typeof window.__wx_channels_config__ === "undefined") {
  __wx_channels_config__ = {};
}
if (typeof window.WXVariable === "undefined") {
  WXVariable = {};
}
var __wx_assets_base = (function () {
  // var cfg = __wx_channels_config__;
  // if (cfg && cfg.apiServerProtocol && cfg.apiServerAddr) {
  //   return (
  //     cfg.apiServerProtocol +
  //     "://" +
  //     cfg.apiServerAddr +
  //     "/__wx_channels_assets"
  //   );
  // }
  return "https://channels.weixin.qq.com/__wx_channels_assets";
})();
var __wx_username;
var __wx_channels_tip__ = {};
var __wx_channels_cur_video = null;
/** 全局的存储 */
var __wx_channels_store__ = {
  profile: null,
  buffers: [],
};
var __wx_channels_live_store__ = {
  profile: null,
};
function __wx_channels_video_decrypt(t, e, p) {
  for (
    var r = new Uint8Array(t), n = 0;
    n < t.byteLength && e + n < p.decryptor_array.length;
    n++
  )
    r[n] ^= p.decryptor_array[n];
  return r;
}
window.VTS_WASM_URL =
  "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/decrypt-video-core/1.3.0/wasm_video_decode.wasm";
window.MAX_HEAP_SIZE = 33554432;
var decryptor_array;
let decryptor;
/** t 是要解码的视频内容长度    e 是 decryptor_array 的长度 */
function wasm_isaac_generate(t, e) {
  decryptor_array = new Uint8Array(e);
  var r = new Uint8Array(Module.HEAPU8.buffer, t, e);
  decryptor_array.set(r.reverse());
  if (decryptor) {
    decryptor.delete();
  }
}
let loaded = false;
/** 获取 decrypt_array */
async function __wx_channels_decrypt(seed) {
  if (!loaded) {
    await WXU.load_script(
      "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/decrypt-video-core/1.3.0/wasm_video_decode.js",
    );
    loaded = true;
  }
  await WXU.sleep();
  decryptor = new Module.WxIsaac64(seed);
  // 调用该方法时，会调用 wasm_isaac_generate 方法
  // 131072 是 decryptor_array 的长度
  decryptor.generate(131072);
  // decryptor.delete();
  // const r = Uint8ArrayToBase64(decryptor_array);
  // decryptor_array = undefined;
  return decryptor_array;
}
var WXU = (() => {
  var defaultRandomAlphabet =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  function __wx_uid__() {
    return random_string(12);
  }
  /**
   * 返回一个指定长度的随机字符串
   * @param length
   * @returns
   */
  function random_string(length) {
    return random_string_with_alphabet(length, defaultRandomAlphabet);
  }
  function random_string_with_alphabet(length, alphabet) {
    let b = new Array(length);
    let max = alphabet.length;
    for (let i = 0; i < b.length; i++) {
      let n = Math.floor(Math.random() * max);
      b[i] = alphabet[n];
    }
    return b.join("");
  }
  function sleep() {
    return new Promise((resolve) => {
      setTimeout(() => {
        resolve();
      }, 1000);
    });
  }
  /**
   * 格式化 FeedProfile，增加了一些字段
   * @param {ChannelsFeed} feed
   * @returns {FeedProfile | null}
   */
  function format_feed(feed) {
    if (feed.liveInfo) {
      return {
        ...feed,
        type: "live",
        // id: feed.id,
        title: feed.description || "直播",
        url: feed.liveInfo.streamUrl,
        cover_url: (() => {
          if (feed.anchorContact) {
            return feed.anchorContact.liveCoverImgUrl;
          }
          if (
            feed.objectDesc &&
            feed.objectDesc.media &&
            feed.objectDesc.media[0]
          ) {
            return feed.objectDesc.media[0].coverUrl;
          }
          return "";
        })(),
        contact: (() => {
          if (feed.anchorContact) {
            return {
              id: feed.anchorContact.username,
              avatar_url: feed.anchorContact.headUrl,
              nickname: feed.anchorContact.nickname,
            };
          }
          return {
            id: feed.contact.username,
            nickname: feed.contact.nickname,
            avatar_url: feed.contact.headUrl,
          };
        })(),
      };
    }
    if (!feed.objectDesc) {
      return null;
    }
    var type = feed.objectDesc.mediaType;
    if (type === 9) {
      // 直播没有 media
      return null;
    }
    var media = feed.objectDesc.media[0];
    if (type === 2) {
      // 图片视频
      return {
        ...feed,
        type: "picture",
        id: feed.id,
        nonce_id: feed.objectNonceId,
        cover_url: media.coverUrl,
        title: feed.objectDesc.description,
        files: feed.objectDesc.media,
        url: "",
        key: 0,
        spec: [],
        contact: {
          id: feed.contact.username,
          avatar_url: feed.contact.headUrl,
          nickname: feed.contact.nickname,
        },
      };
    }
    if (type === 4) {
      return {
        ...feed,
        type: "media",
        id: feed.id,
        nonce_id: feed.objectNonceId,
        title: feed.objectDesc.description,
        url: media.url + media.urlToken,
        key: media.decodeKey,
        cover_url: media.coverUrl,
        createtime: feed.createtime,
        spec: media.spec,
        size: media.fileSize,
        duration: media.videoPlayLen,
        contact: {
          id: feed.contact.username,
          avatar_url: feed.contact.headUrl,
          nickname: feed.contact.nickname,
        },
      };
    }
    return null;
  }
  /**
   * @param {string} text
   */
  function __wx_channels_copy(text) {
    var textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.cssText = "position: absolute; top: -999px; left: -999px;";
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand("copy");
    document.body.removeChild(textArea);
  }
  function __wx_channels_play_cur_video() {
    if (
      __wx_channels_cur_video &&
      typeof __wx_channels_cur_video.player.play === "function"
    ) {
      __wx_channels_cur_video.player.play();
    }
  }
  function __wx_channels_pause_cur_video() {
    if (
      __wx_channels_cur_video &&
      typeof __wx_channels_cur_video.player.pause === "function"
    ) {
      __wx_channels_cur_video.player.pause();
    }
  }
  /**
   * @param {LogMsg} params
   */
  function __wx_log(params) {
    console.log("[log]", params);
    fetch("/__wx_channels_api/tip", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(params),
    });
  }
  /**
   * @param {ErrorMsg} params
   */
  function __wx_error(params) {
    var _alert = params.alert ?? 1;
    fetch("/__wx_channels_api/error", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(params),
    });
    if (_alert) {
      weui.topTips(params.msg);
    }
  }
  const script_loaded_map = {};
  function __wx_load_script(src) {
    const existing = script_loaded_map[src];
    if (existing) {
      return existing;
    }
    const p = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.type = "text/javascript";
      script.src = src;
      script.onload = resolve;
      script.onerror = reject;
      document.head.appendChild(script);
    });
    script_loaded_map[src] = p;
    return p;
  }

  /**
   * @param {() => HTMLElement} selector
   * @returns
   */
  function __wx_find_elm(selector) {
    return new Promise((resolve) => {
      var __count = 0;
      var __timer = setInterval(() => {
        __count += 1;
        var $elm = selector();
        if (!$elm) {
          if (__count >= 5) {
            clearInterval(__timer);
            __timer = null;
            resolve(null);
          }
          return;
        }
        resolve($elm);
        return;
      }, 200);
    });
  }

  /**
   * 构建文件名
   * @param {FeedProfile} profile
   * @param {string} spec
   * @param {string} template
   */
  function build_filename(profile, spec, template) {
    var default_name = (() => {
      if (profile.title) {
        return profile.title;
      }
      if (profile.id) {
        return profile.id;
      }
      return new Date().valueOf();
    })();
    var params = {
      filename: default_name,
      id: profile.id,
      title: profile.title,
      spec: "original",
      created_at: profile.createtime,
      download_at: (new Date().valueOf() / 1000).toFixed(0),
    };
    if (profile.contact) {
      params.author = profile.contact.nickname;
    }
    if (spec && profile.spec) {
      var matched = profile.spec.find((item) => item.fileFormat === spec);
      if (matched) {
        params.spec = matched.fileFormat;
      }
    }
    var filename = template
      ? template.replace(/\{\{([^}]+)\}\}/g, (match, key) => params[key])
      : default_name;
    if (typeof window.beforeFilename === "function") {
      return window.beforeFilename(filename, params, profile, spec);
    }
    return filename;
  }
  function remove_zero(num) {
    let result = Number(num);
    if (String(num).indexOf(".") > -1) {
      result = parseFloat(num.toString().replace(/0+?$/g, ""));
    }
    return result;
  }
  function bytes_to_size(bytes) {
    if (!bytes) {
      return "0KB";
    }
    if (bytes === 0) {
      return "0KB";
    }
    const symbols = ["bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
    let exp = Math.floor(Math.log(bytes) / Math.log(1024));
    if (exp < 1) {
      return bytes + " " + symbols[0];
    }
    bytes = Number((bytes / Math.pow(1024, exp)).toFixed(2));
    const size = bytes;
    const unit = symbols[exp];
    if (Number.isInteger(size)) {
      return `${size}${unit}`;
    }
    return `${remove_zero(size.toFixed(2))}${unit}`;
  }
  function get_queries(href) {
    var [pathname, search] = decodeURIComponent(href).split("?");
    var queries = decodeURIComponent(search)
      .split("&")
      .map((item) => {
        const [key, value] = item.split("=");
        return {
          [key]: value,
        };
      })
      .reduce(
        (prev, cur) => ({
          ...prev,
          ...cur,
        }),
        {},
      );
    return queries;
  }

  function mediaBufferToWav(abuffer, len) {
    len = len || abuffer.length;
    var num_of_chan = abuffer.numberOfChannels;
    var length = len * num_of_chan * 2 + 44;
    var buffer = new ArrayBuffer(length);
    var view = new DataView(buffer);
    var channels = [];
    var i;
    var sample;
    var offset = 0;
    var pos = 0;

    function setUint16(data) {
      view.setUint16(pos, data, true);
      pos += 2;
    }
    function setUint32(data) {
      view.setUint32(pos, data, true);
      pos += 4;
    }

    setUint32(0x46464952);
    setUint32(length - 8);
    setUint32(0x45564157);
    setUint32(0x20746d66);
    setUint32(16);
    setUint16(1);
    setUint16(num_of_chan);
    setUint32(abuffer.sampleRate);
    setUint32(abuffer.sampleRate * num_of_chan * 2);
    setUint16(num_of_chan * 2);
    setUint16(16);
    setUint32(0x61746164);
    setUint32(length - pos - 4);

    for (i = 0; i < abuffer.numberOfChannels; i += 1) {
      channels.push(abuffer.getChannelData(i));
    }
    while (pos < length) {
      for (i = 0; i < num_of_chan; i += 1) {
        sample = Math.max(-1, Math.min(1, channels[i][offset]));
        sample = (0.5 + sample < 0 ? sample * 32768 : sample * 32767) | 0;
        view.setInt16(pos, sample, true);
        pos += 2;
      }
      offset += 1;
    }
    return new Blob([buffer], { type: "audio/wav" });
  }
  // https://blog.csdn.net/qq_18643245/article/details/141157149
  function wav2Other(newSet, wavBlob, True, False) {
    const reader = new FileReader();
    reader.onloadend = async function () {
      //检测wav文件头
      const wavView = new Uint8Array(reader.result);
      const eq = function (p, s) {
        for (var i = 0; i < s.length; i++) {
          if (wavView[p + i] != s.charCodeAt(i)) {
            return false;
          }
        }
        return true;
      };
      let pcm;
      if (eq(0, "RIFF") && eq(8, "WAVEfmt ")) {
        var numCh = wavView[22];
        if (wavView[20] == 1 && (numCh == 1 || numCh == 2)) {
          //raw pcm 单或双声道
          var sampleRate =
            wavView[24] +
            (wavView[25] << 8) +
            (wavView[26] << 16) +
            (wavView[27] << 24);
          var bitRate = wavView[34] + (wavView[35] << 8);
          //搜索data块的位置
          var dataPos = 0; // 44 或有更多块
          for (var i = 12, iL = wavView.length - 8; i < iL; ) {
            if (
              wavView[i] == 100 &&
              wavView[i + 1] == 97 &&
              wavView[i + 2] == 116 &&
              wavView[i + 3] == 97
            ) {
              //eq(i,"data")
              dataPos = i + 8;
              break;
            }
            i += 4;
            i +=
              4 +
              wavView[i] +
              (wavView[i + 1] << 8) +
              (wavView[i + 2] << 16) +
              (wavView[i + 3] << 24);
          }
          console.log("wav info", sampleRate, bitRate, numCh, dataPos);
          if (dataPos) {
            if (bitRate == 16) {
              pcm = new Int16Array(wavView.buffer.slice(dataPos));
            } else if (bitRate == 8) {
              pcm = new Int16Array(wavView.length - dataPos);
              //8位转成16位
              for (var j = dataPos, d = 0; j < wavView.length; j++, d++) {
                var b = wavView[j];
                pcm[d] = (b - 128) << 8;
              }
            }
          }
          if (pcm && numCh == 2) {
            //双声道简单转单声道
            var pcm1 = new Int16Array(pcm.length / 2);
            for (var i = 0; i < pcm1.length; i++) {
              pcm1[i] = (pcm[i * 2] + pcm[i * 2 + 1]) / 2;
            }
            pcm = pcm1;
          }
        }
      }
      if (!pcm) {
        False && False("非单或双声道wav raw pcm格式音频，无法转码");
        return;
      }
      await __wx_load_script(__wx_assets_base + "/lib/recorder.min.js");
      var rec = Recorder(newSet).mock(pcm, sampleRate);
      rec.stop(function (blob, duration) {
        True(blob, duration, rec);
      }, False);
    };
    reader.readAsArrayBuffer(wavBlob);
  }

  async function wavBlobToMP3(wavBlob) {
    return new Promise((resolve) => {
      if (!wavBlob) {
        resolve([new Error("Missing the wav blob"), null]);
        return;
      }
      var set = {
        type: "mp3",
        sampleRate: 48000,
        bitRate: 96,
      };
      wav2Other(
        set,
        wavBlob,
        function (blob) {
          resolve([null, blob]);
        },
        function (msg) {
          resolve([new Error(msg || "Conversion failed"), null]);
        },
      );
    });
  }
  /**
   * 支持回调的下载
   * @param {Response} response
   * @param {{ onStart: (v: { total_size: number }) => void, onProgress: (v: { loaded_size: number, progress: number | null }) => void, onEnd: (v: { blob: Blob }) => void }} handlers
   */
  async function download_with_progress(response, handlers) {
    var content_length = response.headers.get("Content-Length");
    var chunks = [];
    var total_size = content_length ? parseInt(content_length, 10) : 0;
    if (total_size) {
      if (handlers.onStart) {
        handlers.onStart({ total_size });
      }
    }
    var loaded_size = 0;
    var reader = response.body.getReader();
    while (true) {
      var { done, value } = await reader.read();
      if (done) {
        break;
      }
      chunks.push(value);
      loaded_size += value.length;
      if (handlers.onProgress) {
        handlers.onProgress({
          loaded_size,
          progress: total_size
            ? Number(((loaded_size / total_size) * 100).toFixed(2))
            : null,
        });
      }
    }
    var blob = new Blob(chunks);
    if (handlers.onEnd) {
      handlers.onEnd({ blob });
    }
    return blob;
  }

  /**
   * 检查是否存在视频
   * @param {{ silence?: boolean }} opt
   * @returns {[boolean, FeedProfile]}
   */
  function __wx_check_feed_existing(opt = {}) {
    var profile = __wx_channels_store__.profile;
    if (!profile) {
      WXU.error({
        alert: Number(!opt.silence),
        msg: "检测不到视频，请提交 issue 反馈",
      });
      return [true, null];
    }
    return [false, profile];
  }

  /**
   * @param {RequestInfo | URL} url
   * @returns {Promise<[null, Response] | [Error, null]>}
   */
  async function __wx_fetch(url) {
    try {
      const r = await fetch(url);
      return [null, r];
    } catch (err) {
      return [/** @type {Error} */ (err), null];
    }
  }

  var before_menus_items = [];
  var after_menus_items = [];
  var before_level2_menus_items = [];
  var after_level2_menus_items = [];
  var WXAPI = {};
  var WXAPI2 = {};
  var WXAPI3 = {};
  var WXAPI4 = {};

  WXE.onAPILoaded((variables) => {
    const keys = Object.keys(variables);
    for (let i = 0; i < keys.length; i++) {
      (() => {
        const variable = keys[i];
        const methods = variables[variable];
        // console.log("variable", {
        //   api: typeof methods.finderGetCommentDetail,
        //   api2: typeof methods.finderSearch,
        //   api3: typeof methods.finderLiveUserPage,
        //   api4: typeof methods.finderGetFollowList,
        // });
        if (typeof methods.finderGetFollowList === "function") {
          WXAPI4 = methods;
          return;
        }
        if (typeof methods.finderGetCommentDetail === "function") {
          WXAPI = methods;
          return;
        }
        if (typeof methods.finderSearch === "function") {
          WXAPI2 = methods;
          return;
        }
        if (typeof methods.finderLiveUserPage === "function") {
          WXAPI3 = methods;
          return;
        }
      })();
    }
  });
  WXE.onUtilsLoaded((methods) => {
    Object.assign(WXAPI, methods);
  });
  return {
    ...WXE,
    get API() {
      return WXAPI;
    },
    get API2() {
      return WXAPI2;
    },
    get API3() {
      return WXAPI3;
    },
    get API4() {
      return WXAPI4;
    },
    downloader: {
      show() {},
      hide() {},
      toggle() {},
      /**
       * 提交下载任务
       * @param {FeedProfile} feed
       * @param {object} opt 配置
       * @param {string} [opt.spec] 规格
       * @param {string} [opt.suffix] 后缀
       */
      async create(feed, opt = {}) {
        console.log("[downloader.create]create", feed);
        var spec = (() => {
          if (opt.spec) {
            return opt.spec;
          }
          if (WXU.config.defaultHighest || opt.spec === null) {
            return "original";
          }
          if (feed.spec[0]) {
            return feed.spec[0].fileFormat;
          }
          return "original";
        })();
        var filename = WXU.build_filename(
          feed,
          spec,
          WXU.config.downloadFilenameTemplate,
        );
        if (!filename) {
          return [new Error("filename 为空"), null];
        }
        if (feed.type === "picture") {
          opt.suffix = ".zip";
          feed.url = `zip://weixin.qq.com?files=${encodeURIComponent(
            JSON.stringify(
              feed.files.map((f, idx) => {
                return {
                  url: f.url,
                  filename: `${idx + 1}.jpg`,
                };
              }),
            ),
          )}`;
          console.log("[]feed.url", feed.url);
        }
        if (opt.suffix !== ".jpg") {
          feed.url = feed.url + "&X-snsvideoflag=" + spec;
        }
        // console.log("[downloader.create]before WXU.request");
        var [err, data] = await WXU.request({
          method: "POST",
          url:
            APIServerProtocol + "://" + FakeAPIServerAddr + "/api/task/create",
          body: {
            id: feed.id,
            url: feed.url,
            title: feed.title,
            filename: filename,
            key: Number(feed.key),
            spec,
            suffix: opt.suffix,
            source_url: WXRouter.buildJumpUrl(feed),
          },
        });
        WXU.downloader.show();
        if (err) {
          return [err, null];
        }
        return [null, data];
      },
      /**
       * 批量提交下载任务
       * @param {FeedProfile[]} feeds
       * @param {object} opt 配置
       * @param {string} [opt.spec] 规格
       * @param {string} [opt.suffix] 后缀
       * @returns
       */
      async create_batch(feeds, opt = {}) {
        var body = {
          feeds: [],
        };
        for (let i = 0; i < feeds.length; i += 1) {
          var feed = feeds[i];
          var spec = (() => {
            if (opt.spec) {
              return opt.spec;
            }
            if (WXU.config.defaultHighest || opt.spec === null) {
              return "original";
            }
            if (feed.spec[0]) {
              return feed.spec[0].fileFormat;
            }
            return "original";
          })();
          var filename = WXU.build_filename(
            feed,
            spec,
            WXU.config.downloadFilenameTemplate,
          );
          if (filename) {
            if (feed.type === "picture") {
              opt.suffix = ".zip";
              feed.url = `zip://weixin.qq.com?files=${encodeURIComponent(
                JSON.stringify(
                  feed.files.map((f, idx) => {
                    return {
                      url: f.url,
                      filename: `${idx + 1}.jpg`,
                    };
                  }),
                ),
              )}`;
            }
            if (opt.suffix !== ".jpg") {
              feed.url = feed.url + "&X-snsvideoflag=" + spec;
            }
            body.feeds.push({
              id: feed.id,
              url: feed.url,
              title: feed.title,
              key: Number(feed.key),
              filename,
              spec,
              suffix: opt.suffix,
              source_url: WXRouter.buildJumpUrl(feed),
            });
          }
        }
        WXU.downloader.show();
        var [err, data] = await WXU.request({
          method: "POST",
          url:
            APIServerProtocol +
            "://" +
            FakeAPIServerAddr +
            "/api/task/create_batch",
          body,
        });
        if (err) {
          return [err, null];
        }
        return [null, data];
      },
    },
    /**
     * 视频解密
     */
    build_decrypt_arr: __wx_channels_decrypt,
    video_decrypt: __wx_channels_video_decrypt,
    async decrypt_video(buf, key) {
      try {
        const r = await __wx_channels_decrypt(key);
        if (r) {
          buf = __wx_channels_video_decrypt(buf, 0, {
            decryptor_array: r,
          });
          return [null, buf];
        }
        return [new Error("前端解密失败"), null];
      } catch (err) {
        return [err, null];
      }
    },
    /**
     * 类型转换相关
     */
    async media_buffer_to_wav(...args) {
      await __wx_load_script(__wx_assets_base + "/lib/recorder.min.js");
      return mediaBufferToWav(...args);
    },
    wav_to_mp3_blob: wavBlobToMP3,
    async media_to_mp3(buf) {
      var audioCtx = new AudioContext();
      return new Promise((resolve) => {
        audioCtx.decodeAudioData(buf, async function (buffer) {
          var blob = await mediaBufferToWav(buffer);
          var [err, data] = await wavBlobToMP3(blob);
          if (err) {
            return resolve([err, null]);
          }
          return resolve([null, data]);
        });
      });
    },
    download_with_progress,
    /**  */
    sleep,
    resultify(fn) {
      return (...args) => {
        return new Promise((resolve) => {
          fn(...args)
            .then((data) => {
              resolve([null, data]);
            })
            .catch((err) => {
              resolve([err, null]);
            });
        });
      };
    },
    uid: __wx_uid__,
    bytes_to_size,
    parseJSON(v) {
      try {
        var r = JSON.parse(v);
        return [null, r];
      } catch (err) {
        return [err, null];
      }
    },
    build_filename,
    load_script: __wx_load_script,
    find_elm: __wx_find_elm,
    get_queries,
    /**
     * 提示相关
     */
    copy: __wx_channels_copy,
    log: __wx_log,
    error: __wx_error,
    loading() {
      return weui.loading();
    },
    toast(text) {
      return weui.toast(text);
    },
    append_media_buf(buf) {
      __wx_channels_store__.buffers.push(buf);
    },
    set_cur_video() {
      setTimeout(() => {
        window.__wx_channels_cur_video = document.querySelector(
          ".feed-video.video-js",
        );
      }, 800);
    },
    /**
     * @param {ChannelsFeed} feed
     */
    set_feed(feed) {
      var profile = format_feed(feed);
      if (!profile) {
        return;
      }
      fetch("/__wx_channels_api/profile", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(profile),
      });
      __wx_channels_store__.profile = profile;
    },
    /**
     *
     * @param {ChannelsFeed} feed
     */
    set_live_feed(feed) {
      var profile = format_feed(feed);
      if (!profile) {
        return;
      }
      fetch("/__wx_channels_api/profile", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(profile),
      });
      __wx_channels_live_store__.profile = profile;
    },
    /**
     *
     * @param {ChannelsFeed} feed
     * @returns
     */
    format_feed,
    get cur_video() {
      return __wx_channels_cur_video;
    },
    pause_cur_video: __wx_channels_pause_cur_video,
    play_cur_video: __wx_channels_play_cur_video,
    check_feed_existing: __wx_check_feed_existing,
    fetch: __wx_fetch,
    observe_node(selector, cb) {
      var $existing = document.querySelector(selector);
      if ($existing) {
        cb($existing);
        return;
      }
      var observer = new MutationObserver((mutations, obs) => {
        mutations.forEach((mutation) => {
          if (mutation.type === "childList") {
            mutation.addedNodes.forEach((node) => {
              if (node.nodeType === 1) {
                if (node.matches(selector) || node.querySelector(selector)) {
                  cb(
                    node.matches(selector)
                      ? node
                      : node.querySelector(selector),
                  );
                  if (document.querySelector(selector)) {
                    obs.disconnect();
                  }
                }
              }
            });
          }
        });
      });
      WXU.onWindowLoaded(() => {
        const $root = document.getElementById("app");
        if (!$root) {
          return;
        }
        observer.observe($root, {
          childList: true,
          subtree: true,
        });
      });
    },
    /**
     * @param {{ url: string; method: 'GET' | 'POST'; body?: any }} opt
     */
    async request(opt) {
      return new Promise((resolve, reject) => {
        var xhr = new XMLHttpRequest();
        xhr.open(opt.method, opt.url);
        xhr.setRequestHeader("Content-Type", "application/json");
        xhr.onload = async function () {
          // console.log("[request]xhr.responseText", xhr.responseText);
          try {
            var data = JSON.parse(xhr.responseText);
            if (data.code !== 0) {
              resolve([new Error(data.msg), null]);
              return;
            }
            resolve([null, data.data]);
          } catch (e) {
            // ignore
          }
          resolve([null, xhr.responseText]);
        };
        xhr.onerror = function (err) {
          // console.log("[request]xhr.onerror", err);
          resolve([new Error(err.type), null]);
        };
        xhr.send(JSON.stringify(opt.body));
      });
    },
    async save(blob, filename) {
      await __wx_load_script(__wx_assets_base + "/lib/FileSaver.min.js");
      saveAs(blob, filename);
    },
    async Zip() {
      await __wx_load_script(__wx_assets_base + "/lib/jszip.min.js");
      const zip = new JSZip();
      return zip;
    },
    /**
     * 向菜单前面插入额外菜单
     * @param {{label: string; onClick?:(event: { profile: ChannelsMedia }) => void}[]} items
     */
    unshiftMenuItems(items) {
      before_menus_items = items.concat(before_menus_items);
    },
    /**
     * 向菜单后面插入额外菜单
     * @param {{label: string; onClick?:(event: { profile: ChannelsMedia }) => void}[]} items
     */
    pushMenuItems(items) {
      after_menus_items = after_menus_items.concat(items);
    },
    get before_menu_items() {
      return before_menus_items;
    },
    get after_menu_items() {
      return after_menus_items;
    },
    /**
     * @returns {ChannelsConfig}
     */
    get config() {
      /** @type {ChannelsConfig} */
      return {
        ...(__wx_channels_config__ || {}),
        ...(WXVariable || {}),
      };
    },
    get version() {
      return __wx_channels_version__;
    },
    env: {
      get isChannels() {
        return window.location.href.includes("weixin.qq.com");
      },
      get isWxwork() {
        return window.ua.includes("wxwork");
      },
    },
  };
})();

/**
 * 用于下载已经播放的视频内容
 * @param {FeedProfile} profile 视频信息
 */
async function __wx_channels_download(profile) {
  console.log("__wx_channels_download");
  const data = profile.data;
  const blob = new Blob(data, { type: "video/mp4" });
  WXU.save(blob, profile.filename);
}
/**
 * 下载图片视频
 * @param {FeedProfile} profile 视频信息
 */
async function __wx_channels_download3(profile) {
  console.log("__wx_channels_download3");
  const files = profile.files;
  const zip = await WXU.Zip();
  zip.file("contact.txt", JSON.stringify(profile.contact, null, 2));
  const folder = zip.folder("images");
  const fetchPromises = files
    .map((f) => f.url)
    .map(async (url, index) => {
      const response = await fetch(url);
      const blob = await response.blob();
      folder.file(index + 1 + ".png", blob);
    });
  const ins = WXU.loading();
  try {
    await Promise.all(fetchPromises);
    const content = await zip.generateAsync({ type: "blob" });
    await WXU.save(content, profile.filename + ".zip");
  } catch (err) {
    WXU.error({ msg: "下载失败，" + err.message });
  }
  ins.hide();
}
/**
 * 下载加密视频
 * @param {FeedProfile} feed 视频信息
 * @param {object} opt 选项
 * @param {string} opt.spec 规格
 * @param {string} opt.suffix 后缀
 */
async function __wx_channels_download4(feed, opt) {
  console.log("__wx_channels_download4");
  if (!WXU.config.downloadInFrontend) {
    var [err, data] = await WXU.downloader.create(feed, opt);
    if (err) {
      WXU.error({ msg: err.message });
      return;
    }
    return;
  }
  var filename = WXU.build_filename(
    feed,
    opt.spec,
    WXU.config.downloadFilenameTemplate,
  );
  if (!filename) {
    WXU.error({ msg: "文件名生成失败" });
    return;
  }
  feed.filename = filename;
  if (feed.type === "picture") {
    __wx_channels_download3(feed);
    return;
  }
  if (opt.spec) {
    feed.url = feed.url + "&X-snsvideoflag=" + opt.spec;
  }
  if (WXU.config.downloadPauseWhenDownload) {
    WXU.pause_cur_video();
  }
  const ins = WXU.loading();
  var [err, response] = await WXU.fetch(feed.url);
  if (err) {
    WXU.error({ msg: err.message });
    return;
  }
  const media_blob = await WXU.download_with_progress(response, {
    onStart({ total_size }) {
      WXU.log({
        msg: `总大小 ${WXU.bytes_to_size(total_size)}`,
      });
    },
    onProgress({ loaded_size, progress }) {
      WXU.log({
        replace: 1,
        msg:
          progress === null
            ? `${WXU.bytes_to_size(loaded_size)}`
            : `${progress}%`,
      });
    },
    onEnd() {},
  });
  WXU.log({ ignore_prefix: 1, msg: "" });
  var media_buf = new Uint8Array(await media_blob.arrayBuffer());
  if (feed.key) {
    WXU.log({ msg: "下载完成，开始解密" });
    var [err, data] = await WXU.decrypt_video(media_buf, feed.key);
    if (err) {
      WXU.error({ msg: "解密失败，" + err.message, alert: 0 });
      WXU.error({ msg: "尝试使用 decrypt 命令解密", alert: 0 });
    } else {
      WXU.log({ msg: "解密成功" });
      media_buf = data;
    }
  }
  if (opt.suffix === ".mp3") {
    const [err, mp3_blob] = await WXU.media_to_mp3(media_buf.buffer);
    if (err) {
      WXU.error({ msg: err.message });
      return;
    }
    WXU.emit(WXU.Events.MP3Downloaded, feed);
    WXU.save(mp3_blob, feed.filename + opt.suffix);
  } else {
    WXU.emit(WXU.Events.MediaDownloaded, feed);
    const result = new Blob([media_buf], { type: "video/mp4" });
    WXU.save(result, feed.filename + opt.suffix);
  }
  ins.hide();
  if (WXU.config.downloadPauseWhenDownload) {
    WXU.play_cur_video();
  }
}
/** 复制当前页面地址 */
function __wx_channels_handle_copy__() {
  WXU.copy(location.href);
  WXU.toast("复制成功");
}
/**
 * 所有下载功能统一先调用该方法
 * 由该方法分发到具体的 download 方法中
 * @param {string | null} spec 规格信息
 * @param {boolean} mp3 是否转换为 MP3
 */
async function __wx_channels_handle_click_download__(spec, mp3) {
  const [err, feed] = WXU.check_feed_existing();
  if (err) return;
  const payload = { ...feed };
  payload.mp3 = !!mp3;
  payload.original_url = feed.url;
  payload.target_spec = spec;
  payload.source_url = location.href;
  WXU.log({
    msg: `${payload.source_url}
${payload.original_url}
${payload.key || ""}`,
  });
  WXU.emit(WXU.Events.BeforeDownloadMedia, payload);
  var suffix = ".mp4";
  if (mp3) {
    suffix = ".mp3";
  }
  if (payload.type === "picture") {
    suffix = ".zip";
  }
  __wx_channels_download4(payload, { spec, suffix });
}
/** 下载已加载的视频 */
function __wx_channels_download_cur__() {
  const [err, profile] = WXU.check_feed_existing();
  if (err) return;
  if (__wx_channels_store__.buffers.length === 0) {
    WXU.error({ msg: "没有可下载的内容" });
    return;
  }
  var filename = WXU.build_filename(
    profile,
    null,
    WXU.config.downloadFilenameTemplate,
  );
  if (!filename) {
    WXU.error({ msg: "文件名生成失败" });
    return;
  }
  profile.filename = filename;
  profile.data = __wx_channels_store__.buffers;
  __wx_channels_download(profile);
}
/** 打印下载原始文件命令 */
function __wx_channels_handle_print_download_command() {
  const [err, profile] = WXU.check_feed_existing();
  if (err) return;
  var _profile = { ...profile };
  var filename = WXU.build_filename(
    _profile,
    null,
    WXU.config.downloadFilenameTemplate,
  );
  if (!filename) {
    alert("文件名生成失败");
    return;
  }
  var command = `download --url "${_profile.url}"`;
  if (_profile.key) {
    command += ` --key ${_profile.key}`;
  }
  command += ` --filename "${filename}.mp4"`;
  WXU.log({ msg: command });
  WXU.toast("请在终端查看下载命令");
}
/** 下载视频封面 */
async function __wx_channels_handle_download_cover() {
  var [err, profile] = WXU.check_feed_existing();
  if (err) return;
  var url = profile.cover_url.replace(/^http:/, "https:");
  if (!WXU.config.downloadInFrontend) {
    var [err, data] = await WXU.downloader.create(
      {
        id: profile.id,
        url,
        title: profile.title,
        spec: profile.spec,
        contact: profile.contact,
      },
      {
        suffix: ".jpg",
      },
    );
    if (err) {
      WXU.error({ msg: err.message });
      return;
    }
    return;
  }
  var filename = WXU.build_filename(
    profile,
    null,
    WXU.config.downloadFilenameTemplate,
  );
  if (!filename) {
    WXU.error({ msg: "文件名生成失败" });
    return;
  }
  WXU.log({ msg: `下载封面\n${url}` });
  const ins = WXU.loading();
  var [err, response] = await WXU.fetch(url);
  ins.hide();
  if (err) {
    WXU.error({ msg: err.message });
    return;
  }
  const blob = await response.blob();
  WXU.save(blob, filename + ".jpg");
}
/**
 * 为指定按钮添加额外的下载选项菜单
 * @param {HTMLElement} trigger
 */
function __wx_attach_download_dropdown_menu(trigger) {
  const { DropdownMenu, Menu, MenuItem } = WUI;
  const submenu$ = Menu({
    children: [],
  });
  const dropdown$ = DropdownMenu({
    $trigger: trigger,
    zIndex: 99999,
    children: [
      ...(() => {
        if (WXU.before_menu_items) {
          return render_extra_menu_items(WXU.before_menu_items, {
            hide() {
              dropdown$.hide();
            },
          });
        }
        return [];
      })(),
      MenuItem({
        label: "更多下载",
        submenu: submenu$,
        onMouseEnter() {
          submenu$.show();
        },
        onMouseLeave() {
          if (!submenu$.isHover) {
            submenu$.hide();
          }
        },
      }),
      MenuItem({
        label: "下载为MP3",
        onClick() {
          __wx_channels_handle_click_download__(null, true);
          dropdown$.hide();
        },
      }),
      MenuItem({
        label: "下载封面",
        onClick() {
          __wx_channels_handle_download_cover();
          dropdown$.hide();
        },
      }),
      MenuItem({
        label: "打印下载命令",
        onClick() {
          __wx_channels_handle_print_download_command();
          dropdown$.hide();
        },
      }),
      MenuItem({
        label: "复制页面链接",
        onClick() {
          __wx_channels_handle_copy__();
          dropdown$.hide();
        },
      }),
      ...(() => {
        if (WXU.after_menu_items) {
          return render_extra_menu_items(WXU.after_menu_items, {
            hide() {
              dropdown$.hide();
            },
          });
        }
        return [];
      })(),
    ],
    onMouseEnter() {
      if (submenu$.isOpen) {
        submenu$.hide();
      }
    },
  });
  dropdown$.ui.$trigger.onMouseEnter(() => {
    const download_menus = [
      MenuItem({
        label: "原始视频",
        onClick() {
          __wx_channels_handle_click_download__(null);
          dropdown$.hide();
        },
      }),
      ...(() => {
        const [err, profile] = WXU.check_feed_existing({
          silence: true,
        });
        if (err) {
          return [];
        }
        return profile.spec.map((spec) => {
          return MenuItem({
            label: spec.fileFormat,
            onClick() {
              __wx_channels_handle_click_download__(spec.fileFormat);
              dropdown$.hide();
            },
          });
        });
      })(),
    ];
    submenu$.setChildren(download_menus);
    dropdown$.show();
  });
  dropdown$.ui.$trigger.onMouseLeave(() => {
    if (dropdown$.isHover) {
      return;
    }
    dropdown$.hide();
  });
  return dropdown$;
}

/** 下载图标 按钮，点击时的处理函数 */
function __wx_download_btn_handler() {
  const [err, profile] = WXU.check_feed_existing();
  if (err) return;
  var spec = (() => {
    if (WXU.config.defaultHighest) {
      return null;
    }
    if (profile.spec[0]) {
      return profile.spec[0].fileFormat;
    }
    return null;
  })();
  __wx_channels_handle_click_download__(spec, false);
}

if (typeof window.Timeless !== undefined) {
  Object.assign(Timeless, Timeless.kit);
  Object.assign(Timeless, Timeless.headless);
  // Rendering
  window.h = Timeless.h;
  window.View = Timeless.View;
  window.Fragment = Timeless.Fragment;
  // Control flow
  window.Show = Timeless.Show;
  window.For = Timeless.For;
  window.Switch = Timeless.Switch;
  window.Match = Timeless.Match;
  // Reactivity
  window.ref = Timeless.ref;
  window.refobj = Timeless.refobj;
  window.refarr = Timeless.refarr;
  window.computed = Timeless.computed;
  window.combine = Timeless.combine;
  window.isElement = Timeless.isElement;
  // Styling
  window.cn = Timeless.cn;
  window.classNames = Timeless.classNames;
  // Primitives
  window.PopoverPrimitive = Timeless.PopoverPrimitive;
  window.DropdownMenuPrimitive = Timeless.DropdownMenuPrimitive;
  window.WaterfallPrimitive = Timeless.WaterfallPrimitive;
  window.ScrollViewPrimitive = Timeless.ScrollViewPrimitive;
  window.DialogPrimitive = Timeless.DialogPrimitive;
  // SVG helpers
  window.SVG = Timeless.SVG;
  window.Circle = Timeless.Circle;
  // HTML injection
  window.DangerouslyInnerHTML = Timeless.DangerouslyInnerHTML;
}

var FakeAPIServerAddr = WXU.config.remoteServerEnabled
  ? FakeRemoteAPIServerAddr
  : FakeLocalAPIServerAddr;
var APIServerProtocol = WXU.config.remoteServerEnabled
  ? FakeRemoteAPIServerProtocol
  : FakeLocalAPIServerProtocol;

function ChannelsWebsocketClient() {
  const methods = {
    connect_local_ws() {
      const ws_url =
        WSServerProtocol + "://" + FakeLocalAPIServerAddr + "/ws/channels";
      const ws = new WebSocket(ws_url);
      ws.onclose = (e) => {
        WXU.error({ msg: "channels ws连接已关闭，" + JSON.stringify(e) });
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
        try {
          if (data.url) {
            var u = new URL(decodeURIComponent(data.url));
            data.oid = WXU.API.decodeBase64ToUint64String(
              u.searchParams.get("oid"),
            );
            data.nid = WXU.API.decodeBase64ToUint64String(
              u.searchParams.get("nid"),
            );
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
          var r = await WXU.API.finderGetCommentDetail(payload);
          /** @type {MediaProfileResp} */
          var { object } = r.data;
          resp({
            ...r,
            payload,
          });
          return;
        } catch (err) {
          resp({
            errCode: 1011,
            errMsg: err.message,
            payload,
          });
          return;
        }
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
    },
  };
  return {
    methods,
  };
}

WXU.onInit((data) => {
  __wx_username = data.mainFinderUsername;
});

const ws_client$ = ChannelsWebsocketClient();
ws_client$.methods.connect_local_ws();
