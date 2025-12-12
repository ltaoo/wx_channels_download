var __wx_channels_tip__ = {};
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
let decrypt_js_loaded = false;
/** 获取 decrypt_array */
async function __wx_channels_decrypt(seed) {
  if (!decrypt_js_loaded) {
    await __wx_load_script(
      "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/decrypt-video-core/1.3.0/wasm_video_decode.js"
    );
    decrypt_js_loaded = true;
  }
  await sleep();
  decryptor = new Module.WxIsaac64(seed);
  // 调用该方法时，会调用 wasm_isaac_generate 方法
  // 131072 是 decryptor_array 的长度
  decryptor.generate(131072);
  // decryptor.delete();
  // const r = Uint8ArrayToBase64(decryptor_array);
  // decryptor_array = undefined;
  return decryptor_array;
}

var ChannelsUtil = (() => {
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
      window.__wx_channels_cur_video &&
      typeof window.__wx_channels_cur_video.player.play === "function"
    ) {
      window.__wx_channels_cur_video.player.play();
    }
  }
  function __wx_channels_pause_cur_video() {
    if (
      window.__wx_channels_cur_video &&
      typeof window.__wx_channels_cur_video.player.pause === "function"
    ) {
      window.__wx_channels_cur_video.player.pause();
    }
  }
  /**
   * @param {LogMsg} params
   */
  function __wx_log(params) {
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
    fetch("/__wx_channels_api/error", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(params),
    });
    if (!params.alert) {
      return;
    }
    alert(params.msg);
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
   * @param {() => HTMLElement} getter
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
   * @param {ChannelsMediaProfile} profile
   * @param {ChannelsMediaSpec} spec
   * @param {string} template
   */
  function __wx_build_filename(profile, spec, template) {
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
    if (spec) {
      params.spec = spec.fileFormat;
    }
    var filename = template
      ? template.replace(/\{\{([^}]+)\}\}/g, (match, key) => params[key])
      : default_name;
    if (window.beforeFilename) {
      return window.beforeFilename(filename, params, profile, spec);
    }
    return filename;
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
    reader.onloadend = function () {
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
        function (blob, duration, rec) {
          resolve([null, blob]);
        },
        function (msg) {
          resolve([new Error(msg || "Conversion failed"), null]);
        }
      );
    });
  }
  function __wx_channels_video_decrypt(t, e, arr) {
    for (
      var r = new Uint8Array(t), n = 0;
      n < t.byteLength && e + n < arr.length;
      n++
    ) {
      r[n] ^= arr[n];
    }
    return r;
  }

  /** 在终端展示下载进度 */
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
            ? ((loaded_size / total_size) * 100).toFixed(2)
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
   * @param {{ silence: boolean }} opt
   * @returns {[boolean, ChannelsMediaProfile]}
   */
  function __wx_check_profile_existing(opt) {
    var profile = __wx_channels_store__.profile;
    if (!profile) {
      ChannelsUtil.error({
        alert: !opt.silence,
        msg: "检测不到视频，请提交 issue 反馈",
      });
      return [true, null];
    }
    return [false, profile];
  }
  /**
   *
   * @param {RequestInfo | URL} url
   * @returns {Promise<[Error | null, Response | null]>}
   */
  async function __wx_fetch(url) {
    try {
      const r = await fetch(url);
      return [null, r];
    } catch (err) {
      return [err, null];
    }
  }

  return {
    /**
     * 类型转换相关
     */
    async media_buffer_to_wav(...args) {
      await __wx_load_script(
        "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/recorder.min.js"
      );
      return mediaBufferToWav(...args);
    },
    wav_to_mp3_blob: wavBlobToMP3,
    build_decrypt_arr: __wx_channels_decrypt,
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
    // decrypt_video: __wx_channels_video_decrypt,
    async decrypt_video(buf, key) {
      var tip = "前端解密失败，尝试使用 decrypt 命令解密";
      try {
        const r = await __wx_channels_decrypt(key);
        if (r) {
          const media_buf = __wx_channels_video_decrypt(buf, 0, r);
          return [null, media_buf];
        }
        return [new Error(tip), null];
      } catch (err) {
        return [new Error(tip), null];
      }
    },
    download_with_progress,
    /**  */
    uid: __wx_uid__,
    build_filename: __wx_build_filename,
    load_script: __wx_load_script,
    find_elm: __wx_find_elm,
    /**
     * 提示相关
     */
    copy: __wx_channels_copy,
    log: __wx_log,
    error: __wx_error,
    loading() {
      if (typeof __wx_channels_tip__.loading === "function") {
        return window.__wx_channels_tip__.loading("下载中");
      }
      return {
        hide() {},
      };
    },
    toast(text) {
      if (typeof __wx_channels_tip__.toast === "function") {
        return window.__wx_channels_tip__.toast(text, 1e3);
      }
      return {
        hide() {},
      };
    },
    get cur_video() {
      return __wx_channels_cur_video;
    },
    pause_cur_video: __wx_channels_pause_cur_video,
    play_cur_video: __wx_channels_play_cur_video,
    check_profile_existing: __wx_check_profile_existing,
    fetch: __wx_fetch,
    async save(blob, filename) {
      await __wx_load_script(
        "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/FileSaver.min.js"
      );
      saveAs(blob, filename);
    },
    async Zip() {
      await __wx_load_script(
        "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/jszip.min.js"
      );
      const zip = new JSZip();
      return zip;
    },
  };
})();
