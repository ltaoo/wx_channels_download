var WXBase64 = (() => {
  function bytesToString(bytes) {
    let encoded = "";
    for (let i = 0; i < bytes.length; i += 1) {
      encoded += `%${bytes[i].toString(16)}`;
    }
    return decodeURIComponent(encoded);
  }

  function stringToArrayBuffer(str) {
    const bytes = [];
    const n = str.length;
    for (let i = 0; i < n; i += 1) {
      let codePoint = str.charCodeAt(i);
      if (codePoint >= 55296 && codePoint <= 56319 && n > i + 1) {
        const next = str.charCodeAt(i + 1);
        if (next >= 56320 && next <= 57343) {
          codePoint = (codePoint - 55296) * 1024 + next - 56320 + 65536;
          i += 1;
        }
      }

      if (codePoint < 128) {
        bytes.push(codePoint);
        continue;
      }
      if (codePoint < 2048) {
        bytes.push((codePoint >> 6) | 192);
        bytes.push((codePoint & 63) | 128);
        continue;
      }
      if (codePoint < 55296 || (codePoint >= 57344 && codePoint < 65536)) {
        bytes.push((codePoint >> 12) | 224);
        bytes.push(((codePoint >> 6) & 63) | 128);
        bytes.push((codePoint & 63) | 128);
        continue;
      }
      if (codePoint >= 65536 && codePoint <= 1114111) {
        bytes.push((codePoint >> 18) | 240);
        bytes.push(((codePoint >> 12) & 63) | 128);
        bytes.push(((codePoint >> 6) & 63) | 128);
        bytes.push((codePoint & 63) | 128);
        continue;
      }
      bytes.push(239, 191, 189);
    }
    return new Uint8Array(bytes).buffer;
  }

  function urlEncode(base64Std) {
    return base64Std
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=+$/g, "");
  }

  function urlDecode(base64UrlNoPad) {
    let s = base64UrlNoPad.replace(/-/g, "+").replace(/_/g, "/");
    while (s.length % 4) {
      s += "=";
    }
    return s;
  }

  function base64ToArrayBuffer(base64Std) {
    const bin = window.atob(base64Std);
    const n = bin.length;
    const out = new Uint8Array(n);
    for (let i = 0; i < n; i += 1) {
      out[i] = bin.charCodeAt(i);
    }
    return out.buffer;
  }

  function arrayBufferToBase64(buf) {
    let bin = "";
    const u8 = new Uint8Array(buf);
    for (let i = 0; i < u8.byteLength; i += 1) {
      bin += String.fromCharCode(u8[i]);
    }
    return window.btoa(bin);
  }

  function decodeBase64JSON(base64Std) {
    try {
      const ab = base64ToArrayBuffer(base64Std);
      const s = bytesToString(new Uint8Array(ab));
      return JSON.parse(s);
    } catch (e) {
      void e;
      return {};
    }
  }

  function encodeStringBase64(value) {
    try {
      const s = typeof value === "string" ? value : JSON.stringify(value);
      const ab = stringToArrayBuffer(s);
      return arrayBufferToBase64(ab);
    } catch (e) {
      void e;
      return "";
    }
  }

  function decodeBase64String(base64Std) {
    try {
      const ab = base64ToArrayBuffer(base64Std);
      return bytesToString(new Uint8Array(ab));
    } catch (e) {
      void e;
      return "";
    }
  }

  function encodeUint64ToBase64(decimalUint64) {
    try {
      let n = BigInt(decimalUint64);
      if (n < 0n || n > 18446744073709551615n) {
        return "";
      }
      const bytes = new Uint8Array(8);
      for (let i = 7; i >= 0; i -= 1) {
        bytes[i] = Number(n & 255n);
        n >>= 8n;
      }
      return urlEncode(arrayBufferToBase64(bytes.buffer));
    } catch (e) {
      void e;
      return "";
    }
  }

  function decodeBase64ToUint64String(base64UrlNoPad) {
    try {
      const base64Std = urlDecode(base64UrlNoPad);
      const ab = base64ToArrayBuffer(base64Std);
      let bytes = new Uint8Array(ab);
      if (bytes.length === 0) {
        return "";
      }
      if (bytes.length < 8) {
        const padded = new Uint8Array(8);
        padded.set(bytes, 8 - bytes.length);
        bytes = padded;
      } else if (bytes.length > 8) {
        bytes = bytes.subarray(0, 8);
      }
      let n = 0n;
      for (let i = 0; i < 8; i += 1) {
        n = (n << 8n) | BigInt(bytes[i]);
      }
      return n.toString(10);
    } catch (e) {
      void e;
      return "";
    }
  }

  const api = {
    urlEncode,
    urlDecode,
    base64ToArrayBuffer,
    arrayBufferToBase64,
    decodeBase64JSON,
    encodeStringBase64,
    decodeBase64String,
    encodeUint64ToBase64,
    decodeBase64ToUint64String,
  };

  try {
    if (typeof window !== "undefined") {
      window.WXBase64 = api;
    }
  } catch (e) {
    void e;
  }

  try {
    if (typeof WXE !== "undefined" && WXE && WXE.emit && WXE.Events) {
      WXE.emit(WXE.Events.UtilsLoaded, {
        encodeUint64ToBase64,
        decodeBase64ToUint64String,
      });
    }
  } catch (e) {
    void e;
  }

  try {
    if (typeof WXU !== "undefined" && WXU && WXU.API) {
      Object.assign(WXU.API, {
        urlEncode,
        urlDecode,
        base64ToArrayBuffer,
        arrayBufferToBase64,
        decodeBase64JSON,
        encodeStringBase64,
        decodeBase64String,
        encodeUint64ToBase64,
        decodeBase64ToUint64String,
      });
    }
  } catch (e) {
    void e;
  }

  return api;
})();
