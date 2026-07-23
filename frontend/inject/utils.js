/**
 * @file 所有的工具函数 + API + 事件总线
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before utils.js");
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
  function __wx_top_tip(text) {
    const tip = document.createElement("div");
    tip.className = "wx-top-tip";
    tip.textContent = text || "";
    document.body.appendChild(tip);
    setTimeout(() => {
      tip.remove();
    }, 3000);
    return {
      hide() {
        tip.remove();
      },
    };
  }
  function __wx_toast(text) {
    const toast = document.createElement("div");
    toast.className = "wx-toast";
    toast.textContent = text || "";
    document.body.appendChild(toast);
    setTimeout(() => {
      toast.remove();
    }, 2200);
    return {
      hide() {
        toast.remove();
      },
    };
  }
  function __wx_loading(text = "加载中") {
    const mask = document.createElement("div");
    mask.className = "wx-loading-mask";
    mask.innerHTML = `<div class="wx-loading-box"><span class="wx-loading-spinner"></span><span>${text}</span></div>`;
    document.body.appendChild(mask);
    return {
      hide() {
        mask.remove();
      },
    };
  }
  /**
   * @param {string} text
   */
  function __wx_copy(text) {
    var textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.cssText = "position: absolute; top: -999px; left: -999px;";
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand("copy");
    document.body.removeChild(textArea);
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
      __wx_top_tip(params.msg);
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
      await __wx_load_script(__wx_asset_url("/lib/recorder.min.js"));
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

  return {
    ...WXE,
    downloader: {
      show() {},
      hide() {},
      toggle() {},
      async create(feed, opt) {
        return [new Error("downloader not ready"), null];
      },
      async create_batch(feeds, opt) {
        return [new Error("downloader not ready"), null];
      },
    },
    /**
     * 类型转换相关
     */
    async media_buffer_to_wav(...args) {
      await __wx_load_script(__wx_asset_url("/lib/recorder.min.js"));
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
    remove_zero,
    parseJSON(v) {
      try {
        var r = JSON.parse(v);
        return [null, r];
      } catch (err) {
        return [err, null];
      }
    },
    load_script: __wx_load_script,
    download_with_progress,
    /**
     * @param {() => HTMLElement} selector
     * @returns
     */
    find_elm(selector) {
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
    },
    get_queries,
    /**
     * 提示相关
     */
    copy: __wx_copy,
    log: __wx_log,
    error: __wx_error,
    loading() {
      return __wx_loading();
    },
    toast(text) {
      return __wx_toast(text);
    },
    // menu_item: __wx_menu_item,
    // create_dropdown_menu: __wx_create_dropdown_menu,
    // create_popover: __wx_create_popover,
    fetch: __wx_fetch,
    observe_node(selector, cb, error_cb) {
      var $existing = document.querySelector(selector);
      if ($existing) {
        cb($existing);
        return;
      }
      var timer = null;
      if (error_cb) {
        timer = setTimeout(() => {
          observer.disconnect();
          error_cb();
        }, 5000);
      }
      var observer = new MutationObserver((mutations, obs) => {
        mutations.forEach((mutation) => {
          if (mutation.type === "childList") {
            mutation.addedNodes.forEach((node) => {
              if (node.nodeType === 1) {
                if (node.matches(selector) || node.querySelector(selector)) {
                  clearTimeout(timer);
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
              const err = new Error(data.msg);
              err.code = data.code;
              err.data = data.data;
              err.response = data;
              resolve([err, null]);
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
      await __wx_load_script(__wx_asset_url("/lib/FileSaver.min.js"));
      saveAs(blob, filename);
    },
    async Zip() {
      await __wx_load_script(__wx_asset_url("/lib/jszip.min.js"));
      const zip = new JSZip();
      return zip;
    },
    /**
     * @returns {ChannelsConfig}
     */
    get config() {
      return WXEnv.config;
    },
    env: {
      get isChannels() {
        return WXEnv.isChannels;
      },
      get isWxwork() {
        return WXEnv.isWxwork;
      },
    },
  };
})();

if (typeof window.Timeless !== "undefined") {
  const timeless = window.Timeless;
  Object.assign(timeless, timeless.kit);
  Object.assign(timeless, timeless.headless);
  // Rendering
  window.h = timeless.h;
  window.View = timeless.View;
  window.Fragment = timeless.Fragment;
  // Control flow
  window.Show = timeless.Show;
  window.For = timeless.For;
  window.Switch = timeless.Switch;
  window.Match = timeless.Match;
  // Reactivity
  window.ref = timeless.ref;
  window.refobj = timeless.refobj;
  window.refarr = timeless.refarr;
  window.computed = timeless.computed;
  window.combine = timeless.combine;
  window.isElement = timeless.isElement;
  // Styling
  window.cn = timeless.cn;
  window.classNames = timeless.classNames;
  // Primitives
  window.PopoverPrimitive = timeless.PopoverPrimitive;
  window.DropdownMenuPrimitive = timeless.DropdownMenuPrimitive;
  window.WaterfallPrimitive = timeless.WaterfallPrimitive;
  window.ScrollViewPrimitive = timeless.ScrollViewPrimitive;
  window.DialogPrimitive = timeless.DialogPrimitive;
  // SVG helpers
  window.SVG = timeless.SVG;
  window.Circle = timeless.Circle;
}
