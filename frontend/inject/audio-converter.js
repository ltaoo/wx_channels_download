/**
 * @file MP4 to MP3 audio conversion logic
 * Dependency: env.js must be loaded first (provides __wx_asset_url global function)
 */
var WXAudio = (() => {
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
          var sampleRate =
            wavView[24] +
            (wavView[25] << 8) +
            (wavView[26] << 16) +
            (wavView[27] << 24);
          var bitRate = wavView[34] + (wavView[35] << 8);
          var dataPos = 0;
          for (var i = 12, iL = wavView.length - 8; i < iL; ) {
            if (
              wavView[i] == 100 &&
              wavView[i + 1] == 97 &&
              wavView[i + 2] == 116 &&
              wavView[i + 3] == 97
            ) {
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
              for (var j = dataPos, d = 0; j < wavView.length; j++, d++) {
                var b = wavView[j];
                pcm[d] = (b - 128) << 8;
              }
            }
          }
          if (pcm && numCh == 2) {
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

  async function mediaToMp3(buf) {
    var audioCtx = new AudioContext();
    return new Promise((resolve) => {
      audioCtx.decodeAudioData(buf, async function (buffer) {
        var blob = mediaBufferToWav(buffer);
        var [err, data] = await wavBlobToMP3(blob);
        if (err) {
          return resolve([err, null]);
        }
        return resolve([null, data]);
      });
    });
  }

  return {
    mediaBufferToWav,
    wav2Other,
    wavBlobToMP3,
    mediaToMp3,
  };
})();
