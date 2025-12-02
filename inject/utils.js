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
  console.log("__wx_channels_play_cur_video");
  if (window.__wx_channels_cur_video && typeof window.__wx_channels_cur_video.player.play === "function") {
    window.__wx_channels_cur_video.player.play();
  }
}
function __wx_channels_pause_cur_video() {
  console.log("__wx_channels_pause_cur_video");
  if (window.__wx_channels_cur_video && typeof window.__wx_channels_cur_video.player.pause === "function") {
    window.__wx_channels_cur_video.player.pause();
  }
}
function __wx_channel_loading() {
  if (window.__wx_channels_tip__ && window.__wx_channels_tip__.loading) {
    return window.__wx_channels_tip__.loading("下载中");
  }
  return {
    hide() { },
  };
}
function __wx_log(msg) {
  fetch("/__wx_channels_api/tip", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(msg),
  });
}

function __wx_load_script(src) {
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.type = "text/javascript";
    script.src = src;
    script.onload = resolve;
    script.onerror = reject;
    document.head.appendChild(script);
  });
}

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

/** 构建文件名 */
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
    spec: 'original',
    created_at: profile.createtime,
    download_at: (new Date().valueOf() / 1000).toFixed(0),
  };
  if (profile.contact) {
    params.author = profile.contact.nickname;
  }
  if (spec) {
    params.spec = spec.fileFormat;
  }
  var filename = template ? template.replace(/\{\{([^}]+)\}\}/g, (match, key) => params[key]) : default_name;
  if (window.beforeFilename) {
    return window.beforeFilename(filename, params, profile, spec);
  }
  return filename;
}

// var original_log = console.log;
// console.log = function (v) {
//   original_log.apply(console, arguments);
//   __wx_log({
//     msg: String(v).slice(0, 20),
//   });
// };

function icon_download1() {
  var icon_download_html = `<svg data-v-132dee25 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg>`;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-6548f11a data-v-132dee25 class="click-box op-item item-gap-combine" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;">${icon_download_html}<span data-v-132dee25="" class="text">下载</span></div>`;
  return $icon.firstChild;
}
function icon_download2() {
  var icon_download_base64 = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABwAAAAcCAYAAAByDd+UAAAB9ElEQVR4AeyVa3LCMAwGk16scDLCyaAnc3ddy2PnzTDDrzIW8UP6VhIh+Ro+/PkHbjY8pXTZPNw5ON1SAdijWELTOcs8Jr4n9g7HKWARe6AWVT2ZhzEdbnzdih/T7XEILCIKCriO4zi3EfkrdseEWj3T9bELbGHjH0joQomzJ2ZLhQ7E2Y2FnxubQIJsn5UNiFmB/ruGX0AvxDtf+K8Cca4wInLWXE+NArUTOdl50AIIzMxsidB7EZjHnVqjpUbn2wFxEGZmZujN4boLcIGffwmTcrlm0RW1uvMOyEl2oCphQtlaHYvMWy/ijdUWv2UFknVUE9m1Gi/PgXqjCfWvUhOsQBSjugCz9faI5LO2ai3QdTg478wOYI6aLQtbxiXVvTaIKq1Qq+cZSETdaANmcwPdam+WmB/GByMDSyaKffu1ZsWn7UBAjv46P61eBpYNKwiRstVfgPr7ttAjmAK5CGLVH1pgzoTSFdVx1QicsBi7vmhZgJZhClYgCgZ70N3GOr1hcXfmYtSpQBdYtMsniQmw9fqwMswbyuq6tndAqrTCgFopcSnDU0r5rX5w1VeQtoCZegd0A2j+jZgLNgEDbc0Z01czzsfjhE43FsA4LWCD4o3uo+rQiHMYJzTk6nUTWD2YoOAb/ZThvjtOAXcVXjz8OPAXAAD//5jl7kwAAAAGSURBVAMA8H8MSLsb1AoAAAAASUVORK5CYII=";
  var icon_download_html = `<div class="op-icon download-icon" data-v-1fe2ed37 style="background-image: url('${icon_download_base64}');"></div>`;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div class=""><div data-v-6548f11a data-v-1fe2ed37 class="click-box op-item" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;">${icon_download_html}<div data-v-1fe2ed37 class="op-text">下载</div></div></div>`;
  return $icon.firstChild
}
function icon_download3() {
  var icon_download_html = `<svg data-v-132dee25 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg>`;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-132dee25 class="context-menu__wrp item-gap-combine op-more-btn"><div class="context-menu__target"><div data-v-6548f11a data-v-132dee25 class="click-box op-item" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;"></div>${icon_download_html}</div></div>`;
  return $icon.firstChild
}
function icon_download4() {
  var icon_download_html = `<svg data-v-132dee25 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg>`;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-ecf44def="" class="click-box__btn small" ml-key="live-menu-share"><div data-v-ecf44def="" class="text-[20px]" style="height: 1em;">${icon_download_html}</div></div>`;
  return $icon.firstChild;
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
  setUint32(0x20746D66);
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
      view.setInt16(pos, sample, true)
      pos += 2;
    }
    offset += 1;
  }
  return new Blob([buffer], { type: 'audio/wav' });
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
        for (var i = 12, iL = wavView.length - 8; i < iL;) {
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
};

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
};
