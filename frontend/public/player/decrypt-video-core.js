import {
  f as H,
  ar as yu,
  as as Su,
  at as bu,
  au as wu,
  av as Us,
  ay as wg,
  aP as Eg,
  aw as xs,
  ax as Ug,
  g as xg,
} from "./decrypt-video-shared.js";
var ps = {},
  Ts = {},
  Jt = {};
(function (g) {
  Object.defineProperty(g, "__esModule", {
    value: !0,
  }),
    (g.WORKER_STATUS = g.LOG_LEVEL = void 0),
    (function (f) {
      (f[(f.DEBUG = 1)] = "DEBUG"),
        (f[(f.LOG = 2)] = "LOG"),
        (f[(f.WARN = 3)] = "WARN"),
        (f[(f.ERROR = 4)] = "ERROR");
    })(g.LOG_LEVEL || (g.LOG_LEVEL = {})),
    (function (f) {
      (f[(f.IDLE = 0)] = "IDLE"),
        (f[(f.BUSY = 1)] = "BUSY"),
        (f[(f.RECYCLED = 2)] = "RECYCLED");
    })(g.WORKER_STATUS || (g.WORKER_STATUS = {}));
})(Jt);
var Eu = {};
(function (g) {
  var f = (function () {
    var e = new Date(),
      r = 4,
      n = 3,
      a = 2,
      l = 1,
      m = r,
      p = {
        setLogLevel: function (S) {
          S == this.debug
            ? (m = l)
            : S == this.info
            ? (m = a)
            : S == this.warn
            ? (m = n)
            : (S == this.error, (m = r));
        },
        debug: function (S, R) {
          console.debug === void 0 && (console.debug = console.log), l >= m;
        },
        log: function (S, R) {
          this.debug(S.msg);
        },
        info: function (S, R) {
          a >= m;
        },
        warn: function (S, R) {
          n >= m;
        },
        error: function (S, R) {
          r >= m;
        },
      };
    return p;
  })();
  (f.getDurationString = function (e, r) {
    var n;
    function a(O, P) {
      for (var y = "" + O, F = y.split("."); F[0].length < P; )
        F[0] = "0" + F[0];
      return F.join(".");
    }
    e < 0 ? ((n = !0), (e = -e)) : (n = !1);
    var l = r || 1,
      m = e / l,
      p = Math.floor(m / 3600);
    m -= p * 3600;
    var S = Math.floor(m / 60);
    m -= S * 60;
    var R = m * 1e3;
    return (
      (m = Math.floor(m)),
      (R -= m * 1e3),
      (R = Math.floor(R)),
      (n ? "-" : "") + p + ":" + a(S, 2) + ":" + a(m, 2) + "." + a(R, 3)
    );
  }),
    (f.printRanges = function (e) {
      var r = e.length;
      if (r > 0) {
        for (var n = "", a = 0; a < r; a++)
          a > 0 && (n += ","),
            (n +=
              "[" +
              f.getDurationString(e.start(a)) +
              "," +
              f.getDurationString(e.end(a)) +
              "]");
        return n;
      } else return "(empty)";
    }),
    (g.Log = f);
  var h = function (e) {
    if (e instanceof ArrayBuffer)
      (this.buffer = e), (this.dataview = new DataView(e));
    else throw "Needs an array buffer";
    this.position = 0;
  };
  (h.prototype.getPosition = function () {
    return this.position;
  }),
    (h.prototype.getEndPosition = function () {
      return this.buffer.byteLength;
    }),
    (h.prototype.getLength = function () {
      return this.buffer.byteLength;
    }),
    (h.prototype.seek = function (e) {
      var r = Math.max(0, Math.min(this.buffer.byteLength, e));
      return (this.position = isNaN(r) || !isFinite(r) ? 0 : r), !0;
    }),
    (h.prototype.isEos = function () {
      return this.getPosition() >= this.getEndPosition();
    }),
    (h.prototype.readAnyInt = function (e, r) {
      var n = 0;
      if (this.position + e <= this.buffer.byteLength) {
        switch (e) {
          case 1:
            r
              ? (n = this.dataview.getInt8(this.position))
              : (n = this.dataview.getUint8(this.position));
            break;
          case 2:
            r
              ? (n = this.dataview.getInt16(this.position))
              : (n = this.dataview.getUint16(this.position));
            break;
          case 3:
            if (r) throw "No method for reading signed 24 bits values";
            (n = this.dataview.getUint8(this.position) << 16),
              (n |= this.dataview.getUint8(this.position + 1) << 8),
              (n |= this.dataview.getUint8(this.position + 2));
            break;
          case 4:
            r
              ? (n = this.dataview.getInt32(this.position))
              : (n = this.dataview.getUint32(this.position));
            break;
          case 8:
            if (r) throw "No method for reading signed 64 bits values";
            (n = this.dataview.getUint32(this.position) << 32),
              (n |= this.dataview.getUint32(this.position + 4));
            break;
          default:
            throw "readInt method not implemented for size: " + e;
        }
        return (this.position += e), n;
      } else throw "Not enough bytes in buffer";
    }),
    (h.prototype.readUint8 = function () {
      return this.readAnyInt(1, !1);
    }),
    (h.prototype.readUint16 = function () {
      return this.readAnyInt(2, !1);
    }),
    (h.prototype.readUint24 = function () {
      return this.readAnyInt(3, !1);
    }),
    (h.prototype.readUint32 = function () {
      return this.readAnyInt(4, !1);
    }),
    (h.prototype.readUint64 = function () {
      return this.readAnyInt(8, !1);
    }),
    (h.prototype.readString = function (e) {
      if (this.position + e <= this.buffer.byteLength) {
        for (var r = "", n = 0; n < e; n++)
          r += String.fromCharCode(this.readUint8());
        return r;
      } else throw "Not enough bytes in buffer";
    }),
    (h.prototype.readCString = function () {
      for (var e = []; ; ) {
        var r = this.readUint8();
        if (r !== 0) e.push(r);
        else break;
      }
      return String.fromCharCode.apply(null, e);
    }),
    (h.prototype.readInt8 = function () {
      return this.readAnyInt(1, !0);
    }),
    (h.prototype.readInt16 = function () {
      return this.readAnyInt(2, !0);
    }),
    (h.prototype.readInt32 = function () {
      return this.readAnyInt(4, !0);
    }),
    (h.prototype.readInt64 = function () {
      return this.readAnyInt(8, !1);
    }),
    (h.prototype.readUint8Array = function (e) {
      for (var r = new Uint8Array(e), n = 0; n < e; n++)
        r[n] = this.readUint8();
      return r;
    }),
    (h.prototype.readInt16Array = function (e) {
      for (var r = new Int16Array(e), n = 0; n < e; n++)
        r[n] = this.readInt16();
      return r;
    }),
    (h.prototype.readUint16Array = function (e) {
      for (var r = new Int16Array(e), n = 0; n < e; n++)
        r[n] = this.readUint16();
      return r;
    }),
    (h.prototype.readUint32Array = function (e) {
      for (var r = new Uint32Array(e), n = 0; n < e; n++)
        r[n] = this.readUint32();
      return r;
    }),
    (h.prototype.readInt32Array = function (e) {
      for (var r = new Int32Array(e), n = 0; n < e; n++)
        r[n] = this.readInt32();
      return r;
    }),
    (g.MP4BoxStream = h);
  var d = function (e, r, n) {
    (this._byteOffset = r || 0),
      e instanceof ArrayBuffer
        ? (this.buffer = e)
        : typeof e == "object"
        ? ((this.dataView = e), r && (this._byteOffset += r))
        : (this.buffer = new ArrayBuffer(e || 0)),
      (this.position = 0),
      (this.endianness = n == null ? d.LITTLE_ENDIAN : n);
  };
  (d.prototype = {}),
    (d.prototype.getPosition = function () {
      return this.position;
    }),
    (d.prototype._realloc = function (e) {
      if (this._dynamicSize) {
        var r = this._byteOffset + this.position + e,
          n = this._buffer.byteLength;
        if (r <= n) {
          r > this._byteLength && (this._byteLength = r);
          return;
        }
        for (n < 1 && (n = 1); r > n; ) n *= 2;
        var a = new ArrayBuffer(n),
          l = new Uint8Array(this._buffer),
          m = new Uint8Array(a, 0, l.length);
        m.set(l), (this.buffer = a), (this._byteLength = r);
      }
    }),
    (d.prototype._trimAlloc = function () {
      if (this._byteLength != this._buffer.byteLength) {
        var e = new ArrayBuffer(this._byteLength),
          r = new Uint8Array(e),
          n = new Uint8Array(this._buffer, 0, r.length);
        r.set(n), (this.buffer = e);
      }
    }),
    (d.BIG_ENDIAN = !1),
    (d.LITTLE_ENDIAN = !0),
    (d.prototype._byteLength = 0),
    Object.defineProperty(d.prototype, "byteLength", {
      get: function () {
        return this._byteLength - this._byteOffset;
      },
    }),
    Object.defineProperty(d.prototype, "buffer", {
      get: function () {
        return this._trimAlloc(), this._buffer;
      },
      set: function (e) {
        (this._buffer = e),
          (this._dataView = new DataView(this._buffer, this._byteOffset)),
          (this._byteLength = this._buffer.byteLength);
      },
    }),
    Object.defineProperty(d.prototype, "byteOffset", {
      get: function () {
        return this._byteOffset;
      },
      set: function (e) {
        (this._byteOffset = e),
          (this._dataView = new DataView(this._buffer, this._byteOffset)),
          (this._byteLength = this._buffer.byteLength);
      },
    }),
    Object.defineProperty(d.prototype, "dataView", {
      get: function () {
        return this._dataView;
      },
      set: function (e) {
        (this._byteOffset = e.byteOffset),
          (this._buffer = e.buffer),
          (this._dataView = new DataView(this._buffer, this._byteOffset)),
          (this._byteLength = this._byteOffset + e.byteLength);
      },
    }),
    (d.prototype.seek = function (e) {
      var r = Math.max(0, Math.min(this.byteLength, e));
      this.position = isNaN(r) || !isFinite(r) ? 0 : r;
    }),
    (d.prototype.isEof = function () {
      return this.position >= this._byteLength;
    }),
    (d.prototype.mapUint8Array = function (e) {
      this._realloc(e * 1);
      var r = new Uint8Array(this._buffer, this.byteOffset + this.position, e);
      return (this.position += e * 1), r;
    }),
    (d.prototype.readInt32Array = function (e, r) {
      e = e == null ? this.byteLength - this.position / 4 : e;
      var n = new Int32Array(e);
      return (
        d.memcpy(
          n.buffer,
          0,
          this.buffer,
          this.byteOffset + this.position,
          e * n.BYTES_PER_ELEMENT
        ),
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += n.byteLength),
        n
      );
    }),
    (d.prototype.readInt16Array = function (e, r) {
      e = e == null ? this.byteLength - this.position / 2 : e;
      var n = new Int16Array(e);
      return (
        d.memcpy(
          n.buffer,
          0,
          this.buffer,
          this.byteOffset + this.position,
          e * n.BYTES_PER_ELEMENT
        ),
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += n.byteLength),
        n
      );
    }),
    (d.prototype.readInt8Array = function (e) {
      e = e == null ? this.byteLength - this.position : e;
      var r = new Int8Array(e);
      return (
        d.memcpy(
          r.buffer,
          0,
          this.buffer,
          this.byteOffset + this.position,
          e * r.BYTES_PER_ELEMENT
        ),
        (this.position += r.byteLength),
        r
      );
    }),
    (d.prototype.readUint32Array = function (e, r) {
      e = e == null ? this.byteLength - this.position / 4 : e;
      var n = new Uint32Array(e);
      return (
        d.memcpy(
          n.buffer,
          0,
          this.buffer,
          this.byteOffset + this.position,
          e * n.BYTES_PER_ELEMENT
        ),
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += n.byteLength),
        n
      );
    }),
    (d.prototype.readUint16Array = function (e, r) {
      e = e == null ? this.byteLength - this.position / 2 : e;
      var n = new Uint16Array(e);
      return (
        d.memcpy(
          n.buffer,
          0,
          this.buffer,
          this.byteOffset + this.position,
          e * n.BYTES_PER_ELEMENT
        ),
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += n.byteLength),
        n
      );
    }),
    (d.prototype.readUint8Array = function (e) {
      e = e == null ? this.byteLength - this.position : e;
      var r = new Uint8Array(e);
      return (
        d.memcpy(
          r.buffer,
          0,
          this.buffer,
          this.byteOffset + this.position,
          e * r.BYTES_PER_ELEMENT
        ),
        (this.position += r.byteLength),
        r
      );
    }),
    (d.prototype.readFloat64Array = function (e, r) {
      e = e == null ? this.byteLength - this.position / 8 : e;
      var n = new Float64Array(e);
      return (
        d.memcpy(
          n.buffer,
          0,
          this.buffer,
          this.byteOffset + this.position,
          e * n.BYTES_PER_ELEMENT
        ),
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += n.byteLength),
        n
      );
    }),
    (d.prototype.readFloat32Array = function (e, r) {
      e = e == null ? this.byteLength - this.position / 4 : e;
      var n = new Float32Array(e);
      return (
        d.memcpy(
          n.buffer,
          0,
          this.buffer,
          this.byteOffset + this.position,
          e * n.BYTES_PER_ELEMENT
        ),
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += n.byteLength),
        n
      );
    }),
    (d.prototype.readInt32 = function (e) {
      var r = this._dataView.getInt32(
        this.position,
        e == null ? this.endianness : e
      );
      return (this.position += 4), r;
    }),
    (d.prototype.readInt16 = function (e) {
      var r = this._dataView.getInt16(
        this.position,
        e == null ? this.endianness : e
      );
      return (this.position += 2), r;
    }),
    (d.prototype.readInt8 = function () {
      var e = this._dataView.getInt8(this.position);
      return (this.position += 1), e;
    }),
    (d.prototype.readUint32 = function (e) {
      var r = this._dataView.getUint32(
        this.position,
        e == null ? this.endianness : e
      );
      return (this.position += 4), r;
    }),
    (d.prototype.readUint16 = function (e) {
      var r = this._dataView.getUint16(
        this.position,
        e == null ? this.endianness : e
      );
      return (this.position += 2), r;
    }),
    (d.prototype.readUint8 = function () {
      var e = this._dataView.getUint8(this.position);
      return (this.position += 1), e;
    }),
    (d.prototype.readFloat32 = function (e) {
      var r = this._dataView.getFloat32(
        this.position,
        e == null ? this.endianness : e
      );
      return (this.position += 4), r;
    }),
    (d.prototype.readFloat64 = function (e) {
      var r = this._dataView.getFloat64(
        this.position,
        e == null ? this.endianness : e
      );
      return (this.position += 8), r;
    }),
    (d.endianness = new Int8Array(new Int16Array([1]).buffer)[0] > 0),
    (d.memcpy = function (e, r, n, a, l) {
      var m = new Uint8Array(e, r, l),
        p = new Uint8Array(n, a, l);
      m.set(p);
    }),
    (d.arrayToNative = function (e, r) {
      return r == this.endianness ? e : this.flipArrayEndianness(e);
    }),
    (d.nativeToEndian = function (e, r) {
      return this.endianness == r ? e : this.flipArrayEndianness(e);
    }),
    (d.flipArrayEndianness = function (e) {
      for (
        var r = new Uint8Array(e.buffer, e.byteOffset, e.byteLength), n = 0;
        n < e.byteLength;
        n += e.BYTES_PER_ELEMENT
      )
        for (var a = n + e.BYTES_PER_ELEMENT - 1, l = n; a > l; a--, l++) {
          var m = r[l];
          (r[l] = r[a]), (r[a] = m);
        }
      return e;
    }),
    (d.prototype.failurePosition = 0),
    (String.fromCharCodeUint8 = function (e) {
      for (var r = [], n = 0; n < e.length; n++) r[n] = e[n];
      return String.fromCharCode.apply(null, r);
    }),
    (d.prototype.readString = function (e, r) {
      return r == null || r == "ASCII"
        ? String.fromCharCodeUint8.apply(null, [
            this.mapUint8Array(e == null ? this.byteLength - this.position : e),
          ])
        : new TextDecoder(r).decode(this.mapUint8Array(e));
    }),
    (d.prototype.readCString = function (e) {
      var r = this.byteLength - this.position,
        n = new Uint8Array(this._buffer, this._byteOffset + this.position),
        a = r;
      e != null && (a = Math.min(e, r));
      for (var l = 0; l < a && n[l] !== 0; l++);
      var m = String.fromCharCodeUint8.apply(null, [this.mapUint8Array(l)]);
      return (
        e != null ? (this.position += a - l) : l != r && (this.position += 1), m
      );
    });
  var U = Math.pow(2, 32);
  (d.prototype.readInt64 = function () {
    return this.readInt32() * U + this.readUint32();
  }),
    (d.prototype.readUint64 = function () {
      return this.readUint32() * U + this.readUint32();
    }),
    (d.prototype.readInt64 = function () {
      return this.readUint32() * U + this.readUint32();
    }),
    (d.prototype.readUint24 = function () {
      return (
        (this.readUint8() << 16) + (this.readUint8() << 8) + this.readUint8()
      );
    }),
    (g.DataStream = d),
    (d.prototype.save = function (e) {
      var r = new Blob([this.buffer]);
      if (window.URL && URL.createObjectURL) {
        var n = window.URL.createObjectURL(r),
          a = document.createElement("a");
        document.body.appendChild(a),
          a.setAttribute("href", n),
          a.setAttribute("download", e),
          a.setAttribute("target", "_self"),
          a.click(),
          window.URL.revokeObjectURL(n);
      } else throw "DataStream.save: Can't create object URL.";
    }),
    (d.prototype._dynamicSize = !0),
    Object.defineProperty(d.prototype, "dynamicSize", {
      get: function () {
        return this._dynamicSize;
      },
      set: function (e) {
        e || this._trimAlloc(), (this._dynamicSize = e);
      },
    }),
    (d.prototype.shift = function (e) {
      var r = new ArrayBuffer(this._byteLength - e),
        n = new Uint8Array(r),
        a = new Uint8Array(this._buffer, e, n.length);
      n.set(a), (this.buffer = r), (this.position -= e);
    }),
    (d.prototype.writeInt32Array = function (e, r) {
      if (
        (this._realloc(e.length * 4),
        e instanceof Int32Array &&
          this.byteOffset + (this.position % e.BYTES_PER_ELEMENT) === 0)
      )
        d.memcpy(
          this._buffer,
          this.byteOffset + this.position,
          e.buffer,
          0,
          e.byteLength
        ),
          this.mapInt32Array(e.length, r);
      else for (var n = 0; n < e.length; n++) this.writeInt32(e[n], r);
    }),
    (d.prototype.writeInt16Array = function (e, r) {
      if (
        (this._realloc(e.length * 2),
        e instanceof Int16Array &&
          this.byteOffset + (this.position % e.BYTES_PER_ELEMENT) === 0)
      )
        d.memcpy(
          this._buffer,
          this.byteOffset + this.position,
          e.buffer,
          0,
          e.byteLength
        ),
          this.mapInt16Array(e.length, r);
      else for (var n = 0; n < e.length; n++) this.writeInt16(e[n], r);
    }),
    (d.prototype.writeInt8Array = function (e) {
      if (
        (this._realloc(e.length * 1),
        e instanceof Int8Array &&
          this.byteOffset + (this.position % e.BYTES_PER_ELEMENT) === 0)
      )
        d.memcpy(
          this._buffer,
          this.byteOffset + this.position,
          e.buffer,
          0,
          e.byteLength
        ),
          this.mapInt8Array(e.length);
      else for (var r = 0; r < e.length; r++) this.writeInt8(e[r]);
    }),
    (d.prototype.writeUint32Array = function (e, r) {
      if (
        (this._realloc(e.length * 4),
        e instanceof Uint32Array &&
          this.byteOffset + (this.position % e.BYTES_PER_ELEMENT) === 0)
      )
        d.memcpy(
          this._buffer,
          this.byteOffset + this.position,
          e.buffer,
          0,
          e.byteLength
        ),
          this.mapUint32Array(e.length, r);
      else for (var n = 0; n < e.length; n++) this.writeUint32(e[n], r);
    }),
    (d.prototype.writeUint16Array = function (e, r) {
      if (
        (this._realloc(e.length * 2),
        e instanceof Uint16Array &&
          this.byteOffset + (this.position % e.BYTES_PER_ELEMENT) === 0)
      )
        d.memcpy(
          this._buffer,
          this.byteOffset + this.position,
          e.buffer,
          0,
          e.byteLength
        ),
          this.mapUint16Array(e.length, r);
      else for (var n = 0; n < e.length; n++) this.writeUint16(e[n], r);
    }),
    (d.prototype.writeUint8Array = function (e) {
      if (
        (this._realloc(e.length * 1),
        e instanceof Uint8Array &&
          this.byteOffset + (this.position % e.BYTES_PER_ELEMENT) === 0)
      )
        d.memcpy(
          this._buffer,
          this.byteOffset + this.position,
          e.buffer,
          0,
          e.byteLength
        ),
          this.mapUint8Array(e.length);
      else for (var r = 0; r < e.length; r++) this.writeUint8(e[r]);
    }),
    (d.prototype.writeFloat64Array = function (e, r) {
      if (
        (this._realloc(e.length * 8),
        e instanceof Float64Array &&
          this.byteOffset + (this.position % e.BYTES_PER_ELEMENT) === 0)
      )
        d.memcpy(
          this._buffer,
          this.byteOffset + this.position,
          e.buffer,
          0,
          e.byteLength
        ),
          this.mapFloat64Array(e.length, r);
      else for (var n = 0; n < e.length; n++) this.writeFloat64(e[n], r);
    }),
    (d.prototype.writeFloat32Array = function (e, r) {
      if (
        (this._realloc(e.length * 4),
        e instanceof Float32Array &&
          this.byteOffset + (this.position % e.BYTES_PER_ELEMENT) === 0)
      )
        d.memcpy(
          this._buffer,
          this.byteOffset + this.position,
          e.buffer,
          0,
          e.byteLength
        ),
          this.mapFloat32Array(e.length, r);
      else for (var n = 0; n < e.length; n++) this.writeFloat32(e[n], r);
    }),
    (d.prototype.writeInt32 = function (e, r) {
      this._realloc(4),
        this._dataView.setInt32(
          this.position,
          e,
          r == null ? this.endianness : r
        ),
        (this.position += 4);
    }),
    (d.prototype.writeInt16 = function (e, r) {
      this._realloc(2),
        this._dataView.setInt16(
          this.position,
          e,
          r == null ? this.endianness : r
        ),
        (this.position += 2);
    }),
    (d.prototype.writeInt8 = function (e) {
      this._realloc(1),
        this._dataView.setInt8(this.position, e),
        (this.position += 1);
    }),
    (d.prototype.writeUint32 = function (e, r) {
      this._realloc(4),
        this._dataView.setUint32(
          this.position,
          e,
          r == null ? this.endianness : r
        ),
        (this.position += 4);
    }),
    (d.prototype.writeUint16 = function (e, r) {
      this._realloc(2),
        this._dataView.setUint16(
          this.position,
          e,
          r == null ? this.endianness : r
        ),
        (this.position += 2);
    }),
    (d.prototype.writeUint8 = function (e) {
      this._realloc(1),
        this._dataView.setUint8(this.position, e),
        (this.position += 1);
    }),
    (d.prototype.writeFloat32 = function (e, r) {
      this._realloc(4),
        this._dataView.setFloat32(
          this.position,
          e,
          r == null ? this.endianness : r
        ),
        (this.position += 4);
    }),
    (d.prototype.writeFloat64 = function (e, r) {
      this._realloc(8),
        this._dataView.setFloat64(
          this.position,
          e,
          r == null ? this.endianness : r
        ),
        (this.position += 8);
    }),
    (d.prototype.writeUCS2String = function (e, r, n) {
      n == null && (n = e.length);
      for (var a = 0; a < e.length && a < n; a++)
        this.writeUint16(e.charCodeAt(a), r);
      for (; a < n; a++) this.writeUint16(0);
    }),
    (d.prototype.writeString = function (e, r, n) {
      var a = 0;
      if (r == null || r == "ASCII")
        if (n != null) {
          var l = Math.min(e.length, n);
          for (a = 0; a < l; a++) this.writeUint8(e.charCodeAt(a));
          for (; a < n; a++) this.writeUint8(0);
        } else for (a = 0; a < e.length; a++) this.writeUint8(e.charCodeAt(a));
      else this.writeUint8Array(new TextEncoder(r).encode(e.substring(0, n)));
    }),
    (d.prototype.writeCString = function (e, r) {
      var n = 0;
      if (r != null) {
        var a = Math.min(e.length, r);
        for (n = 0; n < a; n++) this.writeUint8(e.charCodeAt(n));
        for (; n < r; n++) this.writeUint8(0);
      } else {
        for (n = 0; n < e.length; n++) this.writeUint8(e.charCodeAt(n));
        this.writeUint8(0);
      }
    }),
    (d.prototype.writeStruct = function (e, r) {
      for (var n = 0; n < e.length; n += 2) {
        var a = e[n + 1];
        this.writeType(a, r[e[n]], r);
      }
    }),
    (d.prototype.writeType = function (e, r, n) {
      var a;
      if (typeof e == "function") return e(this, r);
      if (typeof e == "object" && !(e instanceof Array))
        return e.set(this, r, n);
      var l = null,
        m = "ASCII",
        p = this.position;
      switch (
        (typeof e == "string" &&
          /:/.test(e) &&
          ((a = e.split(":")), (e = a[0]), (l = parseInt(a[1]))),
        typeof e == "string" &&
          /,/.test(e) &&
          ((a = e.split(",")), (e = a[0]), (m = parseInt(a[1]))),
        e)
      ) {
        case "uint8":
          this.writeUint8(r);
          break;
        case "int8":
          this.writeInt8(r);
          break;
        case "uint16":
          this.writeUint16(r, this.endianness);
          break;
        case "int16":
          this.writeInt16(r, this.endianness);
          break;
        case "uint32":
          this.writeUint32(r, this.endianness);
          break;
        case "int32":
          this.writeInt32(r, this.endianness);
          break;
        case "float32":
          this.writeFloat32(r, this.endianness);
          break;
        case "float64":
          this.writeFloat64(r, this.endianness);
          break;
        case "uint16be":
          this.writeUint16(r, d.BIG_ENDIAN);
          break;
        case "int16be":
          this.writeInt16(r, d.BIG_ENDIAN);
          break;
        case "uint32be":
          this.writeUint32(r, d.BIG_ENDIAN);
          break;
        case "int32be":
          this.writeInt32(r, d.BIG_ENDIAN);
          break;
        case "float32be":
          this.writeFloat32(r, d.BIG_ENDIAN);
          break;
        case "float64be":
          this.writeFloat64(r, d.BIG_ENDIAN);
          break;
        case "uint16le":
          this.writeUint16(r, d.LITTLE_ENDIAN);
          break;
        case "int16le":
          this.writeInt16(r, d.LITTLE_ENDIAN);
          break;
        case "uint32le":
          this.writeUint32(r, d.LITTLE_ENDIAN);
          break;
        case "int32le":
          this.writeInt32(r, d.LITTLE_ENDIAN);
          break;
        case "float32le":
          this.writeFloat32(r, d.LITTLE_ENDIAN);
          break;
        case "float64le":
          this.writeFloat64(r, d.LITTLE_ENDIAN);
          break;
        case "cstring":
          this.writeCString(r, l);
          break;
        case "string":
          this.writeString(r, m, l);
          break;
        case "u16string":
          this.writeUCS2String(r, this.endianness, l);
          break;
        case "u16stringle":
          this.writeUCS2String(r, d.LITTLE_ENDIAN, l);
          break;
        case "u16stringbe":
          this.writeUCS2String(r, d.BIG_ENDIAN, l);
          break;
        default:
          if (e.length == 3) {
            for (var S = e[1], R = 0; R < r.length; R++)
              this.writeType(S, r[R]);
            break;
          } else {
            this.writeStruct(e, r);
            break;
          }
      }
      l != null &&
        ((this.position = p), this._realloc(l), (this.position = p + l));
    }),
    (d.prototype.writeUint64 = function (e) {
      var r = Math.floor(e / U);
      this.writeUint32(r), this.writeUint32(e & 4294967295);
    }),
    (d.prototype.writeUint24 = function (e) {
      this.writeUint8((e & 16711680) >> 16),
        this.writeUint8((e & 65280) >> 8),
        this.writeUint8(e & 255);
    }),
    (d.prototype.adjustUint32 = function (e, r) {
      var n = this.position;
      this.seek(e), this.writeUint32(r), this.seek(n);
    }),
    (d.prototype.mapInt32Array = function (e, r) {
      this._realloc(e * 4);
      var n = new Int32Array(this._buffer, this.byteOffset + this.position, e);
      return (
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += e * 4),
        n
      );
    }),
    (d.prototype.mapInt16Array = function (e, r) {
      this._realloc(e * 2);
      var n = new Int16Array(this._buffer, this.byteOffset + this.position, e);
      return (
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += e * 2),
        n
      );
    }),
    (d.prototype.mapInt8Array = function (e) {
      this._realloc(e * 1);
      var r = new Int8Array(this._buffer, this.byteOffset + this.position, e);
      return (this.position += e * 1), r;
    }),
    (d.prototype.mapUint32Array = function (e, r) {
      this._realloc(e * 4);
      var n = new Uint32Array(this._buffer, this.byteOffset + this.position, e);
      return (
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += e * 4),
        n
      );
    }),
    (d.prototype.mapUint16Array = function (e, r) {
      this._realloc(e * 2);
      var n = new Uint16Array(this._buffer, this.byteOffset + this.position, e);
      return (
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += e * 2),
        n
      );
    }),
    (d.prototype.mapFloat64Array = function (e, r) {
      this._realloc(e * 8);
      var n = new Float64Array(
        this._buffer,
        this.byteOffset + this.position,
        e
      );
      return (
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += e * 8),
        n
      );
    }),
    (d.prototype.mapFloat32Array = function (e, r) {
      this._realloc(e * 4);
      var n = new Float32Array(
        this._buffer,
        this.byteOffset + this.position,
        e
      );
      return (
        d.arrayToNative(n, r == null ? this.endianness : r),
        (this.position += e * 4),
        n
      );
    });
  var x = function (e) {
    (this.buffers = []),
      (this.bufferIndex = -1),
      e && (this.insertBuffer(e), (this.bufferIndex = 0));
  };
  (x.prototype = new d(new ArrayBuffer(), 0, d.BIG_ENDIAN)),
    (x.prototype.initialized = function () {
      var e;
      return this.bufferIndex > -1
        ? !0
        : this.buffers.length > 0
        ? ((e = this.buffers[0]),
          e.fileStart === 0
            ? ((this.buffer = e),
              (this.bufferIndex = 0),
              f.debug("MultiBufferStream", "Stream ready for parsing"),
              !0)
            : (f.warn(
                "MultiBufferStream",
                "The first buffer should have a fileStart of 0"
              ),
              this.logBufferLevel(),
              !1))
        : (f.warn("MultiBufferStream", "No buffer to start parsing from"),
          this.logBufferLevel(),
          !1);
    }),
    (ArrayBuffer.concat = function (e, r) {
      f.debug(
        "ArrayBuffer",
        "Trying to create a new buffer of size: " +
          (e.byteLength + r.byteLength)
      );
      var n = new Uint8Array(e.byteLength + r.byteLength);
      return (
        n.set(new Uint8Array(e), 0),
        n.set(new Uint8Array(r), e.byteLength),
        n.buffer
      );
    }),
    (x.prototype.reduceBuffer = function (e, r, n) {
      var a;
      return (
        (a = new Uint8Array(n)),
        a.set(new Uint8Array(e, r, n)),
        (a.buffer.fileStart = e.fileStart + r),
        (a.buffer.usedBytes = 0),
        a.buffer
      );
    }),
    (x.prototype.insertBuffer = function (e) {
      for (var r = !0, n = 0; n < this.buffers.length; n++) {
        var a = this.buffers[n];
        if (e.fileStart <= a.fileStart) {
          if (e.fileStart === a.fileStart)
            if (e.byteLength > a.byteLength) {
              this.buffers.splice(n, 1), n--;
              continue;
            } else
              f.warn(
                "MultiBufferStream",
                "Buffer (fileStart: " +
                  e.fileStart +
                  " - Length: " +
                  e.byteLength +
                  ") already appended, ignoring"
              );
          else
            e.fileStart + e.byteLength <= a.fileStart ||
              (e = this.reduceBuffer(e, 0, a.fileStart - e.fileStart)),
              f.debug(
                "MultiBufferStream",
                "Appending new buffer (fileStart: " +
                  e.fileStart +
                  " - Length: " +
                  e.byteLength +
                  ")"
              ),
              this.buffers.splice(n, 0, e),
              n === 0 && (this.buffer = e);
          r = !1;
          break;
        } else if (e.fileStart < a.fileStart + a.byteLength) {
          var l = a.fileStart + a.byteLength - e.fileStart,
            m = e.byteLength - l;
          if (m > 0) e = this.reduceBuffer(e, l, m);
          else {
            r = !1;
            break;
          }
        }
      }
      r &&
        (f.debug(
          "MultiBufferStream",
          "Appending new buffer (fileStart: " +
            e.fileStart +
            " - Length: " +
            e.byteLength +
            ")"
        ),
        this.buffers.push(e),
        n === 0 && (this.buffer = e));
    }),
    (x.prototype.logBufferLevel = function (e) {
      var r,
        n,
        a,
        l,
        m = [],
        p,
        S = "";
      for (a = 0, l = 0, r = 0; r < this.buffers.length; r++)
        (n = this.buffers[r]),
          r === 0
            ? ((p = {}),
              m.push(p),
              (p.start = n.fileStart),
              (p.end = n.fileStart + n.byteLength),
              (S += "[" + p.start + "-"))
            : p.end === n.fileStart
            ? (p.end = n.fileStart + n.byteLength)
            : ((p = {}),
              (p.start = n.fileStart),
              (S += m[m.length - 1].end - 1 + "], [" + p.start + "-"),
              (p.end = n.fileStart + n.byteLength),
              m.push(p)),
          (a += n.usedBytes),
          (l += n.byteLength);
      m.length > 0 && (S += p.end - 1 + "]");
      var R = e ? f.info : f.debug;
      this.buffers.length === 0
        ? R("MultiBufferStream", "No more buffer in memory")
        : R(
            "MultiBufferStream",
            "" +
              this.buffers.length +
              " stored buffer(s) (" +
              a +
              "/" +
              l +
              " bytes), continuous ranges: " +
              S
          );
    }),
    (x.prototype.cleanBuffers = function () {
      var e, r;
      for (e = 0; e < this.buffers.length; e++)
        (r = this.buffers[e]),
          r.usedBytes === r.byteLength &&
            (f.debug("MultiBufferStream", "Removing buffer #" + e),
            this.buffers.splice(e, 1),
            e--);
    }),
    (x.prototype.mergeNextBuffer = function () {
      var e;
      if (this.bufferIndex + 1 < this.buffers.length)
        if (
          ((e = this.buffers[this.bufferIndex + 1]),
          e.fileStart === this.buffer.fileStart + this.buffer.byteLength)
        ) {
          var r = this.buffer.byteLength,
            n = this.buffer.usedBytes,
            a = this.buffer.fileStart;
          return (
            (this.buffers[this.bufferIndex] = ArrayBuffer.concat(
              this.buffer,
              e
            )),
            (this.buffer = this.buffers[this.bufferIndex]),
            this.buffers.splice(this.bufferIndex + 1, 1),
            (this.buffer.usedBytes = n),
            (this.buffer.fileStart = a),
            f.debug(
              "ISOFile",
              "Concatenating buffer for box parsing (length: " +
                r +
                "->" +
                this.buffer.byteLength +
                ")"
            ),
            !0
          );
        } else return !1;
      else return !1;
    }),
    (x.prototype.findPosition = function (e, r, n) {
      var a,
        l = null,
        m = -1;
      for (
        e === !0 ? (a = 0) : (a = this.bufferIndex);
        a < this.buffers.length && ((l = this.buffers[a]), l.fileStart <= r);

      ) {
        (m = a),
          n &&
            (l.fileStart + l.byteLength <= r
              ? (l.usedBytes = l.byteLength)
              : (l.usedBytes = r - l.fileStart),
            this.logBufferLevel());
        a++;
      }
      return m !== -1
        ? ((l = this.buffers[m]),
          l.fileStart + l.byteLength >= r
            ? (f.debug(
                "MultiBufferStream",
                "Found position in existing buffer #" + m
              ),
              m)
            : -1)
        : -1;
    }),
    (x.prototype.findEndContiguousBuf = function (e) {
      var r,
        n,
        a,
        l = e !== void 0 ? e : this.bufferIndex;
      if (((n = this.buffers[l]), this.buffers.length > l + 1))
        for (
          r = l + 1;
          r < this.buffers.length &&
          ((a = this.buffers[r]), a.fileStart === n.fileStart + n.byteLength);
          r++
        )
          n = a;
      return n.fileStart + n.byteLength;
    }),
    (x.prototype.getEndFilePositionAfter = function (e) {
      var r = this.findPosition(!0, e, !1);
      return r !== -1 ? this.findEndContiguousBuf(r) : e;
    }),
    (x.prototype.addUsedBytes = function (e) {
      (this.buffer.usedBytes += e), this.logBufferLevel();
    }),
    (x.prototype.setAllUsedBytes = function () {
      (this.buffer.usedBytes = this.buffer.byteLength), this.logBufferLevel();
    }),
    (x.prototype.seek = function (e, r, n) {
      var a;
      return (
        (a = this.findPosition(r, e, n)),
        a !== -1
          ? ((this.buffer = this.buffers[a]),
            (this.bufferIndex = a),
            (this.position = e - this.buffer.fileStart),
            f.debug(
              "MultiBufferStream",
              "Repositioning parser at buffer position: " + this.position
            ),
            !0)
          : (f.debug(
              "MultiBufferStream",
              "Position " + e + " not found in buffered data"
            ),
            !1)
      );
    }),
    (x.prototype.getPosition = function () {
      if (this.bufferIndex === -1 || this.buffers[this.bufferIndex] === null)
        throw "Error accessing position in the MultiBufferStream";
      return this.buffers[this.bufferIndex].fileStart + this.position;
    }),
    (x.prototype.getLength = function () {
      return this.byteLength;
    }),
    (x.prototype.getEndPosition = function () {
      if (this.bufferIndex === -1 || this.buffers[this.bufferIndex] === null)
        throw "Error accessing position in the MultiBufferStream";
      return this.buffers[this.bufferIndex].fileStart + this.byteLength;
    }),
    (g.MultiBufferStream = x);
  var A = function () {
    var e = 3,
      r = 4,
      n = 5,
      a = 6,
      l = [];
    (l[e] = "ES_Descriptor"),
      (l[r] = "DecoderConfigDescriptor"),
      (l[n] = "DecoderSpecificInfo"),
      (l[a] = "SLConfigDescriptor"),
      (this.getDescriptorName = function (S) {
        return l[S];
      });
    var m = this,
      p = {};
    return (
      (this.parseOneDescriptor = function (S) {
        var R = 0,
          O,
          P,
          y;
        for (O = S.readUint8(), y = S.readUint8(); y & 128; )
          (R = (y & 127) << 7), (y = S.readUint8());
        return (
          (R += y & 127),
          f.debug(
            "MPEG4DescriptorParser",
            "Found " +
              (l[O] || "Descriptor " + O) +
              ", size " +
              R +
              " at position " +
              S.getPosition()
          ),
          l[O] ? (P = new p[l[O]](R)) : (P = new p.Descriptor(R)),
          P.parse(S),
          P
        );
      }),
      (p.Descriptor = function (S, R) {
        (this.tag = S), (this.size = R), (this.descs = []);
      }),
      (p.Descriptor.prototype.parse = function (S) {
        this.data = S.readUint8Array(this.size);
      }),
      (p.Descriptor.prototype.findDescriptor = function (S) {
        for (var R = 0; R < this.descs.length; R++)
          if (this.descs[R].tag == S) return this.descs[R];
        return null;
      }),
      (p.Descriptor.prototype.parseRemainingDescriptors = function (S) {
        for (var R = S.position; S.position < R + this.size; ) {
          var O = m.parseOneDescriptor(S);
          this.descs.push(O);
        }
      }),
      (p.ES_Descriptor = function (S) {
        p.Descriptor.call(this, e, S);
      }),
      (p.ES_Descriptor.prototype = new p.Descriptor()),
      (p.ES_Descriptor.prototype.parse = function (S) {
        if (
          ((this.ES_ID = S.readUint16()),
          (this.flags = S.readUint8()),
          (this.size -= 3),
          this.flags & 128
            ? ((this.dependsOn_ES_ID = S.readUint16()), (this.size -= 2))
            : (this.dependsOn_ES_ID = 0),
          this.flags & 64)
        ) {
          var R = S.readUint8();
          (this.URL = S.readString(R)), (this.size -= R + 1);
        } else this.URL = "";
        this.flags & 32
          ? ((this.OCR_ES_ID = S.readUint16()), (this.size -= 2))
          : (this.OCR_ES_ID = 0),
          this.parseRemainingDescriptors(S);
      }),
      (p.ES_Descriptor.prototype.getOTI = function (S) {
        var R = this.findDescriptor(r);
        return R ? R.oti : 0;
      }),
      (p.ES_Descriptor.prototype.getAudioConfig = function (S) {
        var R = this.findDescriptor(r);
        if (!R) return null;
        var O = R.findDescriptor(n);
        if (O && O.data) {
          var P = (O.data[0] & 248) >> 3;
          return (
            P === 31 &&
              O.data.length >= 2 &&
              (P = 32 + ((O.data[0] & 7) << 3) + ((O.data[1] & 224) >> 5)),
            P
          );
        } else return null;
      }),
      (p.DecoderConfigDescriptor = function (S) {
        p.Descriptor.call(this, r, S);
      }),
      (p.DecoderConfigDescriptor.prototype = new p.Descriptor()),
      (p.DecoderConfigDescriptor.prototype.parse = function (S) {
        (this.oti = S.readUint8()),
          (this.streamType = S.readUint8()),
          (this.bufferSize = S.readUint24()),
          (this.maxBitrate = S.readUint32()),
          (this.avgBitrate = S.readUint32()),
          (this.size -= 13),
          this.parseRemainingDescriptors(S);
      }),
      (p.DecoderSpecificInfo = function (S) {
        p.Descriptor.call(this, n, S);
      }),
      (p.DecoderSpecificInfo.prototype = new p.Descriptor()),
      (p.SLConfigDescriptor = function (S) {
        p.Descriptor.call(this, a, S);
      }),
      (p.SLConfigDescriptor.prototype = new p.Descriptor()),
      this
    );
  };
  g.MPEG4DescriptorParser = A;
  var o = {
    ERR_INVALID_DATA: -1,
    ERR_NOT_ENOUGH_DATA: 0,
    OK: 1,
    BASIC_BOXES: ["mdat", "idat", "free", "skip", "meco", "strk"],
    FULL_BOXES: ["hmhd", "nmhd", "iods", "xml ", "bxml", "ipro", "mere"],
    CONTAINER_BOXES: [
      ["moov", ["trak", "pssh"]],
      ["trak"],
      ["edts"],
      ["mdia"],
      ["minf"],
      ["dinf"],
      ["stbl", ["sgpd", "sbgp"]],
      ["mvex", ["trex"]],
      ["moof", ["traf"]],
      ["traf", ["trun", "sgpd", "sbgp"]],
      ["vttc"],
      ["tref"],
      ["iref"],
      ["mfra", ["tfra"]],
      ["meco"],
      ["hnti"],
      ["hinf"],
      ["strk"],
      ["strd"],
      ["sinf"],
      ["rinf"],
      ["schi"],
      ["trgr"],
      ["udta", ["kind"]],
      ["iprp", ["ipma"]],
      ["ipco"],
    ],
    boxCodes: [],
    fullBoxCodes: [],
    containerBoxCodes: [],
    sampleEntryCodes: {},
    sampleGroupEntryCodes: [],
    trackGroupTypes: [],
    UUIDBoxes: {},
    UUIDs: [],
    initialize: function () {
      (o.FullBox.prototype = new o.Box()),
        (o.ContainerBox.prototype = new o.Box()),
        (o.SampleEntry.prototype = new o.Box()),
        (o.TrackGroupTypeBox.prototype = new o.FullBox()),
        o.BASIC_BOXES.forEach(function (e) {
          o.createBoxCtor(e);
        }),
        o.FULL_BOXES.forEach(function (e) {
          o.createFullBoxCtor(e);
        }),
        o.CONTAINER_BOXES.forEach(function (e) {
          o.createContainerBoxCtor(e[0], null, e[1]);
        });
    },
    Box: function (e, r, n) {
      (this.type = e), (this.size = r), (this.uuid = n);
    },
    FullBox: function (e, r, n) {
      o.Box.call(this, e, r, n), (this.flags = 0), (this.version = 0);
    },
    ContainerBox: function (e, r, n) {
      o.Box.call(this, e, r, n), (this.boxes = []);
    },
    SampleEntry: function (e, r, n, a) {
      o.ContainerBox.call(this, e, r), (this.hdr_size = n), (this.start = a);
    },
    SampleGroupEntry: function (e) {
      this.grouping_type = e;
    },
    TrackGroupTypeBox: function (e, r) {
      o.FullBox.call(this, e, r);
    },
    createBoxCtor: function (e, r) {
      o.boxCodes.push(e),
        (o[e + "Box"] = function (n) {
          o.Box.call(this, e, n);
        }),
        (o[e + "Box"].prototype = new o.Box()),
        r && (o[e + "Box"].prototype.parse = r);
    },
    createFullBoxCtor: function (e, r) {
      (o[e + "Box"] = function (n) {
        o.FullBox.call(this, e, n);
      }),
        (o[e + "Box"].prototype = new o.FullBox()),
        (o[e + "Box"].prototype.parse = function (n) {
          this.parseFullHeader(n), r && r.call(this, n);
        });
    },
    addSubBoxArrays: function (e) {
      if (e) {
        this.subBoxNames = e;
        for (var r = e.length, n = 0; n < r; n++) this[e[n] + "s"] = [];
      }
    },
    createContainerBoxCtor: function (e, r, n) {
      (o[e + "Box"] = function (a) {
        o.ContainerBox.call(this, e, a), o.addSubBoxArrays.call(this, n);
      }),
        (o[e + "Box"].prototype = new o.ContainerBox()),
        r && (o[e + "Box"].prototype.parse = r);
    },
    createMediaSampleEntryCtor: function (e, r, n) {
      (o.sampleEntryCodes[e] = []),
        (o[e + "SampleEntry"] = function (a, l) {
          o.SampleEntry.call(this, a, l), o.addSubBoxArrays.call(this, n);
        }),
        (o[e + "SampleEntry"].prototype = new o.SampleEntry()),
        r && (o[e + "SampleEntry"].prototype.parse = r);
    },
    createSampleEntryCtor: function (e, r, n, a) {
      o.sampleEntryCodes[e].push(r),
        (o[r + "SampleEntry"] = function (l) {
          o[e + "SampleEntry"].call(this, r, l),
            o.addSubBoxArrays.call(this, a);
        }),
        (o[r + "SampleEntry"].prototype = new o[e + "SampleEntry"]()),
        n && (o[r + "SampleEntry"].prototype.parse = n);
    },
    createEncryptedSampleEntryCtor: function (e, r, n) {
      o.createSampleEntryCtor.call(this, e, r, n, ["sinf"]);
    },
    createSampleGroupCtor: function (e, r) {
      (o[e + "SampleGroupEntry"] = function (n) {
        o.SampleGroupEntry.call(this, e, n);
      }),
        (o[e + "SampleGroupEntry"].prototype = new o.SampleGroupEntry()),
        r && (o[e + "SampleGroupEntry"].prototype.parse = r);
    },
    createTrackGroupCtor: function (e, r) {
      (o[e + "TrackGroupTypeBox"] = function (n) {
        o.TrackGroupTypeBox.call(this, e, n);
      }),
        (o[e + "TrackGroupTypeBox"].prototype = new o.TrackGroupTypeBox()),
        r && (o[e + "TrackGroupTypeBox"].prototype.parse = r);
    },
    createUUIDBox: function (e, r, n, a) {
      o.UUIDs.push(e),
        (o.UUIDBoxes[e] = function (l) {
          r
            ? o.FullBox.call(this, "uuid", l, e)
            : n
            ? o.ContainerBox.call(this, "uuid", l, e)
            : o.Box.call(this, "uuid", l, e);
        }),
        (o.UUIDBoxes[e].prototype = r
          ? new o.FullBox()
          : n
          ? new o.ContainerBox()
          : new o.Box()),
        a &&
          (r
            ? (o.UUIDBoxes[e].prototype.parse = function (l) {
                this.parseFullHeader(l), a && a.call(this, l);
              })
            : (o.UUIDBoxes[e].prototype.parse = a));
    },
  };
  o.initialize(),
    (o.TKHD_FLAG_ENABLED = 1),
    (o.TKHD_FLAG_IN_MOVIE = 2),
    (o.TKHD_FLAG_IN_PREVIEW = 4),
    (o.TFHD_FLAG_BASE_DATA_OFFSET = 1),
    (o.TFHD_FLAG_SAMPLE_DESC = 2),
    (o.TFHD_FLAG_SAMPLE_DUR = 8),
    (o.TFHD_FLAG_SAMPLE_SIZE = 16),
    (o.TFHD_FLAG_SAMPLE_FLAGS = 32),
    (o.TFHD_FLAG_DUR_EMPTY = 65536),
    (o.TFHD_FLAG_DEFAULT_BASE_IS_MOOF = 131072),
    (o.TRUN_FLAGS_DATA_OFFSET = 1),
    (o.TRUN_FLAGS_FIRST_FLAG = 4),
    (o.TRUN_FLAGS_DURATION = 256),
    (o.TRUN_FLAGS_SIZE = 512),
    (o.TRUN_FLAGS_FLAGS = 1024),
    (o.TRUN_FLAGS_CTS_OFFSET = 2048),
    (o.Box.prototype.add = function (e) {
      return this.addBox(new o[e + "Box"]());
    }),
    (o.Box.prototype.addBox = function (e) {
      return (
        this.boxes.push(e),
        this[e.type + "s"] ? this[e.type + "s"].push(e) : (this[e.type] = e),
        e
      );
    }),
    (o.Box.prototype.set = function (e, r) {
      return (this[e] = r), this;
    }),
    (o.Box.prototype.addEntry = function (e, r) {
      var n = r || "entries";
      return this[n] || (this[n] = []), this[n].push(e), this;
    }),
    (g.BoxParser = o),
    (o.parseUUID = function (e) {
      return o.parseHex16(e);
    }),
    (o.parseHex16 = function (e) {
      for (var r = "", n = 0; n < 16; n++) {
        var a = e.readUint8().toString(16);
        r += a.length === 1 ? "0" + a : a;
      }
      return r;
    }),
    (o.parseOneBox = function (e, r, n) {
      var a,
        l = e.getPosition(),
        m = 0,
        p,
        S;
      if (e.getEndPosition() - l < 8)
        return (
          f.debug(
            "BoxParser",
            "Not enough data in stream to parse the type and size of the box"
          ),
          {
            code: o.ERR_NOT_ENOUGH_DATA,
          }
        );
      if (n && n < 8)
        return (
          f.debug(
            "BoxParser",
            "Not enough bytes left in the parent box to parse a new box"
          ),
          {
            code: o.ERR_NOT_ENOUGH_DATA,
          }
        );
      var R = e.readUint32(),
        O = e.readString(4),
        P = O;
      if (
        (f.debug(
          "BoxParser",
          "Found box of type '" + O + "' and size " + R + " at position " + l
        ),
        (m = 8),
        O == "uuid")
      ) {
        if (e.getEndPosition() - e.getPosition() < 16 || n - m < 16)
          return (
            e.seek(l),
            f.debug(
              "BoxParser",
              "Not enough bytes left in the parent box to parse a UUID box"
            ),
            {
              code: o.ERR_NOT_ENOUGH_DATA,
            }
          );
        (S = o.parseUUID(e)), (m += 16), (P = S);
      }
      if (R == 1) {
        if (e.getEndPosition() - e.getPosition() < 8 || (n && n - m < 8))
          return (
            e.seek(l),
            f.warn(
              "BoxParser",
              'Not enough data in stream to parse the extended size of the "' +
                O +
                '" box'
            ),
            {
              code: o.ERR_NOT_ENOUGH_DATA,
            }
          );
        (R = e.readUint64()), (m += 8);
      } else if (R === 0) {
        if (n) R = n;
        else if (O !== "mdat")
          return (
            f.error(
              "BoxParser",
              "Unlimited box size not supported for type: '" + O + "'"
            ),
            (a = new o.Box(O, R)),
            {
              code: o.OK,
              box: a,
              size: a.size,
            }
          );
      }
      return R !== 0 && R < m
        ? (f.error(
            "BoxParser",
            "Box of type " +
              O +
              " has an invalid size " +
              R +
              " (too small to be a box)"
          ),
          {
            code: o.ERR_NOT_ENOUGH_DATA,
            type: O,
            size: R,
            hdr_size: m,
            start: l,
          })
        : R !== 0 && n && R > n
        ? (f.error(
            "BoxParser",
            "Box of type '" +
              O +
              "' has a size " +
              R +
              " greater than its container size " +
              n
          ),
          {
            code: o.ERR_NOT_ENOUGH_DATA,
            type: O,
            size: R,
            hdr_size: m,
            start: l,
          })
        : R !== 0 && l + R > e.getEndPosition()
        ? (e.seek(l),
          f.info(
            "BoxParser",
            "Not enough data in stream to parse the entire '" + O + "' box"
          ),
          {
            code: o.ERR_NOT_ENOUGH_DATA,
            type: O,
            size: R,
            hdr_size: m,
            start: l,
          })
        : r
        ? {
            code: o.OK,
            type: O,
            size: R,
            hdr_size: m,
            start: l,
          }
        : (o[O + "Box"]
            ? (a = new o[O + "Box"](R))
            : O !== "uuid"
            ? (f.warn("BoxParser", "Unknown box type: '" + O + "'"),
              (a = new o.Box(O, R)),
              (a.has_unparsed_data = !0))
            : o.UUIDBoxes[S]
            ? (a = new o.UUIDBoxes[S](R))
            : (f.warn("BoxParser", "Unknown uuid type: '" + S + "'"),
              (a = new o.Box(O, R)),
              (a.uuid = S),
              (a.has_unparsed_data = !0)),
          (a.hdr_size = m),
          (a.start = l),
          a.write === o.Box.prototype.write &&
            a.type !== "mdat" &&
            (f.info(
              "BoxParser",
              "'" +
                P +
                "' box writing not yet implemented, keeping unparsed data in memory for later write"
            ),
            a.parseDataAndRewind(e)),
          a.parse(e),
          (p = e.getPosition() - (a.start + a.size)),
          p < 0
            ? (f.warn(
                "BoxParser",
                "Parsing of box '" +
                  P +
                  "' did not read the entire indicated box data size (missing " +
                  -p +
                  " bytes), seeking forward"
              ),
              e.seek(a.start + a.size))
            : p > 0 &&
              (f.error(
                "BoxParser",
                "Parsing of box '" +
                  P +
                  "' read " +
                  p +
                  " more bytes than the indicated box data size, seeking backwards"
              ),
              a.size !== 0 && e.seek(a.start + a.size)),
          {
            code: o.OK,
            box: a,
            size: a.size,
          });
    }),
    (o.Box.prototype.parse = function (e) {
      this.type != "mdat"
        ? (this.data = e.readUint8Array(this.size - this.hdr_size))
        : this.size === 0
        ? e.seek(e.getEndPosition())
        : e.seek(this.start + this.size);
    }),
    (o.Box.prototype.parseDataAndRewind = function (e) {
      (this.data = e.readUint8Array(this.size - this.hdr_size)),
        (e.position -= this.size - this.hdr_size);
    }),
    (o.FullBox.prototype.parseDataAndRewind = function (e) {
      this.parseFullHeader(e),
        (this.data = e.readUint8Array(this.size - this.hdr_size)),
        (this.hdr_size -= 4),
        (e.position -= this.size - this.hdr_size);
    }),
    (o.FullBox.prototype.parseFullHeader = function (e) {
      (this.version = e.readUint8()),
        (this.flags = e.readUint24()),
        (this.hdr_size += 4);
    }),
    (o.FullBox.prototype.parse = function (e) {
      this.parseFullHeader(e),
        (this.data = e.readUint8Array(this.size - this.hdr_size));
    }),
    (o.ContainerBox.prototype.parse = function (e) {
      for (var r, n; e.getPosition() < this.start + this.size; )
        if (
          ((r = o.parseOneBox(
            e,
            !1,
            this.size - (e.getPosition() - this.start)
          )),
          r.code === o.OK)
        )
          if (
            ((n = r.box),
            this.boxes.push(n),
            this.subBoxNames && this.subBoxNames.indexOf(n.type) != -1)
          )
            this[this.subBoxNames[this.subBoxNames.indexOf(n.type)] + "s"].push(
              n
            );
          else {
            var a = n.type !== "uuid" ? n.type : n.uuid;
            this[a]
              ? f.warn(
                  "Box of type " + a + " already stored in field of this type"
                )
              : (this[a] = n);
          }
        else return;
    }),
    (o.Box.prototype.parseLanguage = function (e) {
      this.language = e.readUint16();
      var r = [];
      (r[0] = (this.language >> 10) & 31),
        (r[1] = (this.language >> 5) & 31),
        (r[2] = this.language & 31),
        (this.languageString = String.fromCharCode(
          r[0] + 96,
          r[1] + 96,
          r[2] + 96
        ));
    }),
    (o.SAMPLE_ENTRY_TYPE_VISUAL = "Visual"),
    (o.SAMPLE_ENTRY_TYPE_AUDIO = "Audio"),
    (o.SAMPLE_ENTRY_TYPE_HINT = "Hint"),
    (o.SAMPLE_ENTRY_TYPE_METADATA = "Metadata"),
    (o.SAMPLE_ENTRY_TYPE_SUBTITLE = "Subtitle"),
    (o.SAMPLE_ENTRY_TYPE_SYSTEM = "System"),
    (o.SAMPLE_ENTRY_TYPE_TEXT = "Text"),
    (o.SampleEntry.prototype.parseHeader = function (e) {
      e.readUint8Array(6),
        (this.data_reference_index = e.readUint16()),
        (this.hdr_size += 8);
    }),
    (o.SampleEntry.prototype.parse = function (e) {
      this.parseHeader(e),
        (this.data = e.readUint8Array(this.size - this.hdr_size));
    }),
    (o.SampleEntry.prototype.parseDataAndRewind = function (e) {
      this.parseHeader(e),
        (this.data = e.readUint8Array(this.size - this.hdr_size)),
        (this.hdr_size -= 8),
        (e.position -= this.size - this.hdr_size);
    }),
    (o.SampleEntry.prototype.parseFooter = function (e) {
      o.ContainerBox.prototype.parse.call(this, e);
    }),
    o.createMediaSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_HINT),
    o.createMediaSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_METADATA),
    o.createMediaSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_SUBTITLE),
    o.createMediaSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_SYSTEM),
    o.createMediaSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_TEXT),
    o.createMediaSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, function (e) {
      var r;
      this.parseHeader(e),
        e.readUint16(),
        e.readUint16(),
        e.readUint32Array(3),
        (this.width = e.readUint16()),
        (this.height = e.readUint16()),
        (this.horizresolution = e.readUint32()),
        (this.vertresolution = e.readUint32()),
        e.readUint32(),
        (this.frame_count = e.readUint16()),
        (r = Math.min(31, e.readUint8())),
        (this.compressorname = e.readString(r)),
        r < 31 && e.readString(31 - r),
        (this.depth = e.readUint16()),
        e.readUint16(),
        this.parseFooter(e);
    }),
    o.createMediaSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_AUDIO, function (e) {
      this.parseHeader(e),
        e.readUint32Array(2),
        (this.channel_count = e.readUint16()),
        (this.samplesize = e.readUint16()),
        e.readUint16(),
        e.readUint16(),
        (this.samplerate = e.readUint32() / 65536),
        this.parseFooter(e);
    }),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "avc1"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "avc2"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "avc3"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "avc4"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "av01"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "hvc1"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "hev1"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "vvc1"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "vvi1"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "vvs1"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "vvcN"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "vp08"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "vp09"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_AUDIO, "mp4a"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_AUDIO, "ac-3"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_AUDIO, "ec-3"),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_AUDIO, "Opus"),
    o.createEncryptedSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_VISUAL, "encv"),
    o.createEncryptedSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_AUDIO, "enca"),
    o.createEncryptedSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_SUBTITLE, "encu"),
    o.createEncryptedSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_SYSTEM, "encs"),
    o.createEncryptedSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_TEXT, "enct"),
    o.createEncryptedSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_METADATA, "encm"),
    o.createBoxCtor("a1lx", function (e) {
      var r = e.readUint8() & 1,
        n = ((r & 1) + 1) * 16;
      this.layer_size = [];
      for (var a = 0; a < 3; a++)
        n == 16
          ? (this.layer_size[a] = e.readUint16())
          : (this.layer_size[a] = e.readUint32());
    }),
    o.createBoxCtor("a1op", function (e) {
      this.op_index = e.readUint8();
    }),
    o.createFullBoxCtor("auxC", function (e) {
      this.aux_type = e.readCString();
      var r = this.size - this.hdr_size - (this.aux_type.length + 1);
      this.aux_subtype = e.readUint8Array(r);
    }),
    o.createBoxCtor("av1C", function (e) {
      var r = e.readUint8();
      if ((r >> 7) & !1) {
        f.error("av1C marker problem");
        return;
      }
      if (((this.version = r & 127), this.version !== 1)) {
        f.error("av1C version " + this.version + " not supported");
        return;
      }
      if (
        ((r = e.readUint8()),
        (this.seq_profile = (r >> 5) & 7),
        (this.seq_level_idx_0 = r & 31),
        (r = e.readUint8()),
        (this.seq_tier_0 = (r >> 7) & 1),
        (this.high_bitdepth = (r >> 6) & 1),
        (this.twelve_bit = (r >> 5) & 1),
        (this.monochrome = (r >> 4) & 1),
        (this.chroma_subsampling_x = (r >> 3) & 1),
        (this.chroma_subsampling_y = (r >> 2) & 1),
        (this.chroma_sample_position = r & 3),
        (r = e.readUint8()),
        (this.reserved_1 = (r >> 5) & 7),
        this.reserved_1 !== 0)
      ) {
        f.error("av1C reserved_1 parsing problem");
        return;
      }
      if (
        ((this.initial_presentation_delay_present = (r >> 4) & 1),
        this.initial_presentation_delay_present === 1)
      )
        this.initial_presentation_delay_minus_one = r & 15;
      else if (((this.reserved_2 = r & 15), this.reserved_2 !== 0)) {
        f.error("av1C reserved_2 parsing problem");
        return;
      }
      var n = this.size - this.hdr_size - 4;
      this.configOBUs = e.readUint8Array(n);
    }),
    o.createBoxCtor("avcC", function (e) {
      var r, n;
      for (
        this.configurationVersion = e.readUint8(),
          this.AVCProfileIndication = e.readUint8(),
          this.profile_compatibility = e.readUint8(),
          this.AVCLevelIndication = e.readUint8(),
          this.lengthSizeMinusOne = e.readUint8() & 3,
          this.nb_SPS_nalus = e.readUint8() & 31,
          n = this.size - this.hdr_size - 6,
          this.SPS = [],
          r = 0;
        r < this.nb_SPS_nalus;
        r++
      )
        (this.SPS[r] = {}),
          (this.SPS[r].length = e.readUint16()),
          (this.SPS[r].nalu = e.readUint8Array(this.SPS[r].length)),
          (n -= 2 + this.SPS[r].length);
      for (
        this.nb_PPS_nalus = e.readUint8(), n--, this.PPS = [], r = 0;
        r < this.nb_PPS_nalus;
        r++
      )
        (this.PPS[r] = {}),
          (this.PPS[r].length = e.readUint16()),
          (this.PPS[r].nalu = e.readUint8Array(this.PPS[r].length)),
          (n -= 2 + this.PPS[r].length);
      n > 0 && (this.ext = e.readUint8Array(n));
    }),
    o.createBoxCtor("btrt", function (e) {
      (this.bufferSizeDB = e.readUint32()),
        (this.maxBitrate = e.readUint32()),
        (this.avgBitrate = e.readUint32());
    }),
    o.createBoxCtor("clap", function (e) {
      (this.cleanApertureWidthN = e.readUint32()),
        (this.cleanApertureWidthD = e.readUint32()),
        (this.cleanApertureHeightN = e.readUint32()),
        (this.cleanApertureHeightD = e.readUint32()),
        (this.horizOffN = e.readUint32()),
        (this.horizOffD = e.readUint32()),
        (this.vertOffN = e.readUint32()),
        (this.vertOffD = e.readUint32());
    }),
    o.createBoxCtor("clli", function (e) {
      (this.max_content_light_level = e.readUint16()),
        (this.max_pic_average_light_level = e.readUint16());
    }),
    o.createFullBoxCtor("co64", function (e) {
      var r, n;
      if (((r = e.readUint32()), (this.chunk_offsets = []), this.version === 0))
        for (n = 0; n < r; n++) this.chunk_offsets.push(e.readUint64());
    }),
    o.createFullBoxCtor("CoLL", function (e) {
      (this.maxCLL = e.readUint16()), (this.maxFALL = e.readUint16());
    }),
    o.createBoxCtor("colr", function (e) {
      if (((this.colour_type = e.readString(4)), this.colour_type === "nclx")) {
        (this.colour_primaries = e.readUint16()),
          (this.transfer_characteristics = e.readUint16()),
          (this.matrix_coefficients = e.readUint16());
        var r = e.readUint8();
        this.full_range_flag = r >> 7;
      } else this.colour_type === "rICC" ? (this.ICC_profile = e.readUint8Array(this.size - 4)) : this.colour_type === "prof" && (this.ICC_profile = e.readUint8Array(this.size - 4));
    }),
    o.createFullBoxCtor("cprt", function (e) {
      this.parseLanguage(e), (this.notice = e.readCString());
    }),
    o.createFullBoxCtor("cslg", function (e) {
      this.version === 0 &&
        ((this.compositionToDTSShift = e.readInt32()),
        (this.leastDecodeToDisplayDelta = e.readInt32()),
        (this.greatestDecodeToDisplayDelta = e.readInt32()),
        (this.compositionStartTime = e.readInt32()),
        (this.compositionEndTime = e.readInt32()));
    }),
    o.createFullBoxCtor("ctts", function (e) {
      var r, n;
      if (
        ((r = e.readUint32()),
        (this.sample_counts = []),
        (this.sample_offsets = []),
        this.version === 0)
      )
        for (n = 0; n < r; n++) {
          this.sample_counts.push(e.readUint32());
          var a = e.readInt32();
          a < 0 &&
            f.warn(
              "BoxParser",
              "ctts box uses negative values without using version 1"
            ),
            this.sample_offsets.push(a);
        }
      else if (this.version == 1)
        for (n = 0; n < r; n++)
          this.sample_counts.push(e.readUint32()),
            this.sample_offsets.push(e.readInt32());
    }),
    o.createBoxCtor("dac3", function (e) {
      var r = e.readUint8(),
        n = e.readUint8(),
        a = e.readUint8();
      (this.fscod = r >> 6),
        (this.bsid = (r >> 1) & 31),
        (this.bsmod = ((r & 1) << 2) | ((n >> 6) & 3)),
        (this.acmod = (n >> 3) & 7),
        (this.lfeon = (n >> 2) & 1),
        (this.bit_rate_code = (n & 3) | ((a >> 5) & 7));
    }),
    o.createBoxCtor("dec3", function (e) {
      var r = e.readUint16();
      (this.data_rate = r >> 3),
        (this.num_ind_sub = r & 7),
        (this.ind_subs = []);
      for (var n = 0; n < this.num_ind_sub + 1; n++) {
        var a = {};
        this.ind_subs.push(a);
        var l = e.readUint8(),
          m = e.readUint8(),
          p = e.readUint8();
        (a.fscod = l >> 6),
          (a.bsid = (l >> 1) & 31),
          (a.bsmod = ((l & 1) << 4) | ((m >> 4) & 15)),
          (a.acmod = (m >> 1) & 7),
          (a.lfeon = m & 1),
          (a.num_dep_sub = (p >> 1) & 15),
          a.num_dep_sub > 0 && (a.chan_loc = ((p & 1) << 8) | e.readUint8());
      }
    }),
    o.createFullBoxCtor("dfLa", function (e) {
      var r = 127,
        n = 128,
        a = [],
        l = [
          "STREAMINFO",
          "PADDING",
          "APPLICATION",
          "SEEKTABLE",
          "VORBIS_COMMENT",
          "CUESHEET",
          "PICTURE",
          "RESERVED",
        ];
      this.parseFullHeader(e);
      do {
        var m = e.readUint8(),
          p = Math.min(m & r, l.length - 1);
        if (
          (p
            ? e.readUint8Array(e.readUint24())
            : (e.readUint8Array(13),
              (this.samplerate = e.readUint32() >> 12),
              e.readUint8Array(20)),
          a.push(l[p]),
          m & n)
        )
          break;
      } while (!0);
      this.numMetadataBlocks = a.length + " (" + a.join(", ") + ")";
    }),
    o.createBoxCtor("dimm", function (e) {
      this.bytessent = e.readUint64();
    }),
    o.createBoxCtor("dmax", function (e) {
      this.time = e.readUint32();
    }),
    o.createBoxCtor("dmed", function (e) {
      this.bytessent = e.readUint64();
    }),
    o.createBoxCtor("dOps", function (e) {
      if (
        ((this.Version = e.readUint8()),
        (this.OutputChannelCount = e.readUint8()),
        (this.PreSkip = e.readUint16()),
        (this.InputSampleRate = e.readUint32()),
        (this.OutputGain = e.readInt16()),
        (this.ChannelMappingFamily = e.readUint8()),
        this.ChannelMappingFamily !== 0)
      ) {
        (this.StreamCount = e.readUint8()),
          (this.CoupledCount = e.readUint8()),
          (this.ChannelMapping = []);
        for (var r = 0; r < this.OutputChannelCount; r++)
          this.ChannelMapping[r] = e.readUint8();
      }
    }),
    o.createFullBoxCtor("dref", function (e) {
      var r, n;
      this.entries = [];
      for (var a = e.readUint32(), l = 0; l < a; l++)
        if (
          ((r = o.parseOneBox(
            e,
            !1,
            this.size - (e.getPosition() - this.start)
          )),
          r.code === o.OK)
        )
          (n = r.box), this.entries.push(n);
        else return;
    }),
    o.createBoxCtor("drep", function (e) {
      this.bytessent = e.readUint64();
    }),
    o.createFullBoxCtor("elng", function (e) {
      this.extended_language = e.readString(this.size - this.hdr_size);
    }),
    o.createFullBoxCtor("elst", function (e) {
      this.entries = [];
      for (var r = e.readUint32(), n = 0; n < r; n++) {
        var a = {};
        this.entries.push(a),
          this.version === 1
            ? ((a.segment_duration = e.readUint64()),
              (a.media_time = e.readInt64()))
            : ((a.segment_duration = e.readUint32()),
              (a.media_time = e.readInt32())),
          (a.media_rate_integer = e.readInt16()),
          (a.media_rate_fraction = e.readInt16());
      }
    }),
    o.createFullBoxCtor("emsg", function (e) {
      this.version == 1
        ? ((this.timescale = e.readUint32()),
          (this.presentation_time = e.readUint64()),
          (this.event_duration = e.readUint32()),
          (this.id = e.readUint32()),
          (this.scheme_id_uri = e.readCString()),
          (this.value = e.readCString()))
        : ((this.scheme_id_uri = e.readCString()),
          (this.value = e.readCString()),
          (this.timescale = e.readUint32()),
          (this.presentation_time_delta = e.readUint32()),
          (this.event_duration = e.readUint32()),
          (this.id = e.readUint32()));
      var r =
        this.size -
        this.hdr_size -
        (4 * 4 + (this.scheme_id_uri.length + 1) + (this.value.length + 1));
      this.version == 1 && (r -= 4), (this.message_data = e.readUint8Array(r));
    }),
    o.createFullBoxCtor("esds", function (e) {
      var r = e.readUint8Array(this.size - this.hdr_size);
      if (typeof A < "u") {
        var n = new A();
        this.esd = n.parseOneDescriptor(new d(r.buffer, 0, d.BIG_ENDIAN));
      }
    }),
    o.createBoxCtor("fiel", function (e) {
      (this.fieldCount = e.readUint8()), (this.fieldOrdering = e.readUint8());
    }),
    o.createBoxCtor("frma", function (e) {
      this.data_format = e.readString(4);
    }),
    o.createBoxCtor("ftyp", function (e) {
      var r = this.size - this.hdr_size;
      (this.major_brand = e.readString(4)),
        (this.minor_version = e.readUint32()),
        (r -= 8),
        (this.compatible_brands = []);
      for (var n = 0; r >= 4; )
        (this.compatible_brands[n] = e.readString(4)), (r -= 4), n++;
    }),
    o.createFullBoxCtor("hdlr", function (e) {
      this.version === 0 &&
        (e.readUint32(),
        (this.handler = e.readString(4)),
        e.readUint32Array(3),
        (this.name = e.readString(this.size - this.hdr_size - 20)),
        this.name[this.name.length - 1] === "\0" &&
          (this.name = this.name.slice(0, -1)));
    }),
    o.createBoxCtor("hvcC", function (e) {
      var r, n, a, l;
      (this.configurationVersion = e.readUint8()),
        (l = e.readUint8()),
        (this.general_profile_space = l >> 6),
        (this.general_tier_flag = (l & 32) >> 5),
        (this.general_profile_idc = l & 31),
        (this.general_profile_compatibility = e.readUint32()),
        (this.general_constraint_indicator = e.readUint8Array(6)),
        (this.general_level_idc = e.readUint8()),
        (this.min_spatial_segmentation_idc = e.readUint16() & 4095),
        (this.parallelismType = e.readUint8() & 3),
        (this.chroma_format_idc = e.readUint8() & 3),
        (this.bit_depth_luma_minus8 = e.readUint8() & 7),
        (this.bit_depth_chroma_minus8 = e.readUint8() & 7),
        (this.avgFrameRate = e.readUint16()),
        (l = e.readUint8()),
        (this.constantFrameRate = l >> 6),
        (this.numTemporalLayers = (l & 13) >> 3),
        (this.temporalIdNested = (l & 4) >> 2),
        (this.lengthSizeMinusOne = l & 3),
        (this.nalu_arrays = []);
      var m = e.readUint8();
      for (r = 0; r < m; r++) {
        var p = [];
        this.nalu_arrays.push(p),
          (l = e.readUint8()),
          (p.completeness = (l & 128) >> 7),
          (p.nalu_type = l & 63);
        var S = e.readUint16();
        for (n = 0; n < S; n++) {
          var R = {};
          p.push(R), (a = e.readUint16()), (R.data = e.readUint8Array(a));
        }
      }
    }),
    o.createFullBoxCtor("iinf", function (e) {
      var r;
      this.version === 0
        ? (this.entry_count = e.readUint16())
        : (this.entry_count = e.readUint32()),
        (this.item_infos = []);
      for (var n = 0; n < this.entry_count; n++)
        if (
          ((r = o.parseOneBox(
            e,
            !1,
            this.size - (e.getPosition() - this.start)
          )),
          r.code === o.OK)
        )
          r.box.type !== "infe" &&
            f.error("BoxParser", "Expected 'infe' box, got " + r.box.type),
            (this.item_infos[n] = r.box);
        else return;
    }),
    o.createFullBoxCtor("iloc", function (e) {
      var r;
      (r = e.readUint8()),
        (this.offset_size = (r >> 4) & 15),
        (this.length_size = r & 15),
        (r = e.readUint8()),
        (this.base_offset_size = (r >> 4) & 15),
        this.version === 1 || this.version === 2
          ? (this.index_size = r & 15)
          : (this.index_size = 0),
        (this.items = []);
      var n = 0;
      if (this.version < 2) n = e.readUint16();
      else if (this.version === 2) n = e.readUint32();
      else throw "version of iloc box not supported";
      for (var a = 0; a < n; a++) {
        var l = {};
        if ((this.items.push(l), this.version < 2)) l.item_ID = e.readUint16();
        else if (this.version === 2) l.item_ID = e.readUint16();
        else throw "version of iloc box not supported";
        switch (
          (this.version === 1 || this.version === 2
            ? (l.construction_method = e.readUint16() & 15)
            : (l.construction_method = 0),
          (l.data_reference_index = e.readUint16()),
          this.base_offset_size)
        ) {
          case 0:
            l.base_offset = 0;
            break;
          case 4:
            l.base_offset = e.readUint32();
            break;
          case 8:
            l.base_offset = e.readUint64();
            break;
          default:
            throw "Error reading base offset size";
        }
        var m = e.readUint16();
        l.extents = [];
        for (var p = 0; p < m; p++) {
          var S = {};
          if ((l.extents.push(S), this.version === 1 || this.version === 2))
            switch (this.index_size) {
              case 0:
                S.extent_index = 0;
                break;
              case 4:
                S.extent_index = e.readUint32();
                break;
              case 8:
                S.extent_index = e.readUint64();
                break;
              default:
                throw "Error reading extent index";
            }
          switch (this.offset_size) {
            case 0:
              S.extent_offset = 0;
              break;
            case 4:
              S.extent_offset = e.readUint32();
              break;
            case 8:
              S.extent_offset = e.readUint64();
              break;
            default:
              throw "Error reading extent index";
          }
          switch (this.length_size) {
            case 0:
              S.extent_length = 0;
              break;
            case 4:
              S.extent_length = e.readUint32();
              break;
            case 8:
              S.extent_length = e.readUint64();
              break;
            default:
              throw "Error reading extent index";
          }
        }
      }
    }),
    o.createBoxCtor("imir", function (e) {
      var r = e.readUint8();
      (this.reserved = r >> 7), (this.axis = r & 1);
    }),
    o.createFullBoxCtor("infe", function (e) {
      if (
        ((this.version === 0 || this.version === 1) &&
          ((this.item_ID = e.readUint16()),
          (this.item_protection_index = e.readUint16()),
          (this.item_name = e.readCString()),
          (this.content_type = e.readCString()),
          (this.content_encoding = e.readCString())),
        this.version === 1)
      ) {
        (this.extension_type = e.readString(4)),
          f.warn("BoxParser", "Cannot parse extension type"),
          e.seek(this.start + this.size);
        return;
      }
      this.version >= 2 &&
        (this.version === 2
          ? (this.item_ID = e.readUint16())
          : this.version === 3 && (this.item_ID = e.readUint32()),
        (this.item_protection_index = e.readUint16()),
        (this.item_type = e.readString(4)),
        (this.item_name = e.readCString()),
        this.item_type === "mime"
          ? ((this.content_type = e.readCString()),
            (this.content_encoding = e.readCString()))
          : this.item_type === "uri " &&
            (this.item_uri_type = e.readCString()));
    }),
    o.createFullBoxCtor("ipma", function (e) {
      var r, n;
      for (
        entry_count = e.readUint32(), this.associations = [], r = 0;
        r < entry_count;
        r++
      ) {
        var a = {};
        this.associations.push(a),
          this.version < 1 ? (a.id = e.readUint16()) : (a.id = e.readUint32());
        var l = e.readUint8();
        for (a.props = [], n = 0; n < l; n++) {
          var m = e.readUint8(),
            p = {};
          a.props.push(p),
            (p.essential = (m & 128) >> 7 === 1),
            this.flags & 1
              ? (p.property_index = ((m & 127) << 8) | e.readUint8())
              : (p.property_index = m & 127);
        }
      }
    }),
    o.createFullBoxCtor("iref", function (e) {
      var r, n;
      for (this.references = []; e.getPosition() < this.start + this.size; )
        if (
          ((r = o.parseOneBox(
            e,
            !0,
            this.size - (e.getPosition() - this.start)
          )),
          r.code === o.OK)
        )
          this.version === 0
            ? (n = new o.SingleItemTypeReferenceBox(
                r.type,
                r.size,
                r.hdr_size,
                r.start
              ))
            : (n = new o.SingleItemTypeReferenceBoxLarge(
                r.type,
                r.size,
                r.hdr_size,
                r.start
              )),
            n.write === o.Box.prototype.write &&
              n.type !== "mdat" &&
              (f.warn(
                "BoxParser",
                n.type +
                  " box writing not yet implemented, keeping unparsed data in memory for later write"
              ),
              n.parseDataAndRewind(e)),
            n.parse(e),
            this.references.push(n);
        else return;
    }),
    o.createBoxCtor("irot", function (e) {
      this.angle = e.readUint8() & 3;
    }),
    o.createFullBoxCtor("ispe", function (e) {
      (this.image_width = e.readUint32()), (this.image_height = e.readUint32());
    }),
    o.createFullBoxCtor("kind", function (e) {
      (this.schemeURI = e.readCString()), (this.value = e.readCString());
    }),
    o.createFullBoxCtor("leva", function (e) {
      var r = e.readUint8();
      this.levels = [];
      for (var n = 0; n < r; n++) {
        var a = {};
        (this.levels[n] = a), (a.track_ID = e.readUint32());
        var l = e.readUint8();
        switch (
          ((a.padding_flag = l >> 7),
          (a.assignment_type = l & 127),
          a.assignment_type)
        ) {
          case 0:
            a.grouping_type = e.readString(4);
            break;
          case 1:
            (a.grouping_type = e.readString(4)),
              (a.grouping_type_parameter = e.readUint32());
            break;
          case 2:
            break;
          case 3:
            break;
          case 4:
            a.sub_track_id = e.readUint32();
            break;
          default:
            f.warn("BoxParser", "Unknown leva assignement type");
        }
      }
    }),
    o.createBoxCtor("lsel", function (e) {
      this.layer_id = e.readUint16();
    }),
    o.createBoxCtor("maxr", function (e) {
      (this.period = e.readUint32()), (this.bytes = e.readUint32());
    }),
    o.createBoxCtor("mdcv", function (e) {
      (this.display_primaries = []),
        (this.display_primaries[0] = {}),
        (this.display_primaries[0].x = e.readUint16()),
        (this.display_primaries[0].y = e.readUint16()),
        (this.display_primaries[1] = {}),
        (this.display_primaries[1].x = e.readUint16()),
        (this.display_primaries[1].y = e.readUint16()),
        (this.display_primaries[2] = {}),
        (this.display_primaries[2].x = e.readUint16()),
        (this.display_primaries[2].y = e.readUint16()),
        (this.white_point = {}),
        (this.white_point.x = e.readUint16()),
        (this.white_point.y = e.readUint16()),
        (this.max_display_mastering_luminance = e.readUint32()),
        (this.min_display_mastering_luminance = e.readUint32());
    }),
    o.createFullBoxCtor("mdhd", function (e) {
      this.version == 1
        ? ((this.creation_time = e.readUint64()),
          (this.modification_time = e.readUint64()),
          (this.timescale = e.readUint32()),
          (this.duration = e.readUint64()))
        : ((this.creation_time = e.readUint32()),
          (this.modification_time = e.readUint32()),
          (this.timescale = e.readUint32()),
          (this.duration = e.readUint32())),
        this.parseLanguage(e),
        e.readUint16();
    }),
    o.createFullBoxCtor("mehd", function (e) {
      this.flags & 1 &&
        (f.warn(
          "BoxParser",
          "mehd box incorrectly uses flags set to 1, converting version to 1"
        ),
        (this.version = 1)),
        this.version == 1
          ? (this.fragment_duration = e.readUint64())
          : (this.fragment_duration = e.readUint32());
    }),
    o.createFullBoxCtor("meta", function (e) {
      (this.boxes = []), o.ContainerBox.prototype.parse.call(this, e);
    }),
    o.createFullBoxCtor("mfhd", function (e) {
      this.sequence_number = e.readUint32();
    }),
    o.createFullBoxCtor("mfro", function (e) {
      this._size = e.readUint32();
    }),
    o.createFullBoxCtor("mvhd", function (e) {
      this.version == 1
        ? ((this.creation_time = e.readUint64()),
          (this.modification_time = e.readUint64()),
          (this.timescale = e.readUint32()),
          (this.duration = e.readUint64()))
        : ((this.creation_time = e.readUint32()),
          (this.modification_time = e.readUint32()),
          (this.timescale = e.readUint32()),
          (this.duration = e.readUint32())),
        (this.rate = e.readUint32()),
        (this.volume = e.readUint16() >> 8),
        e.readUint16(),
        e.readUint32Array(2),
        (this.matrix = e.readUint32Array(9)),
        e.readUint32Array(6),
        (this.next_track_id = e.readUint32());
    }),
    o.createBoxCtor("npck", function (e) {
      this.packetssent = e.readUint32();
    }),
    o.createBoxCtor("nump", function (e) {
      this.packetssent = e.readUint64();
    }),
    o.createFullBoxCtor("padb", function (e) {
      var r = e.readUint32();
      this.padbits = [];
      for (var n = 0; n < Math.floor((r + 1) / 2); n++)
        this.padbits = e.readUint8();
    }),
    o.createBoxCtor("pasp", function (e) {
      (this.hSpacing = e.readUint32()), (this.vSpacing = e.readUint32());
    }),
    o.createBoxCtor("payl", function (e) {
      this.text = e.readString(this.size - this.hdr_size);
    }),
    o.createBoxCtor("payt", function (e) {
      this.payloadID = e.readUint32();
      var r = e.readUint8();
      this.rtpmap_string = e.readString(r);
    }),
    o.createFullBoxCtor("pdin", function (e) {
      var r = (this.size - this.hdr_size) / 8;
      (this.rate = []), (this.initial_delay = []);
      for (var n = 0; n < r; n++)
        (this.rate[n] = e.readUint32()),
          (this.initial_delay[n] = e.readUint32());
    }),
    o.createFullBoxCtor("pitm", function (e) {
      this.version === 0
        ? (this.item_id = e.readUint16())
        : (this.item_id = e.readUint32());
    }),
    o.createFullBoxCtor("pixi", function (e) {
      var r;
      for (
        this.num_channels = e.readUint8(), this.bits_per_channels = [], r = 0;
        r < this.num_channels;
        r++
      )
        this.bits_per_channels[r] = e.readUint8();
    }),
    o.createBoxCtor("pmax", function (e) {
      this.bytes = e.readUint32();
    }),
    o.createFullBoxCtor("prft", function (e) {
      (this.ref_track_id = e.readUint32()),
        (this.ntp_timestamp = e.readUint64()),
        this.version === 0
          ? (this.media_time = e.readUint32())
          : (this.media_time = e.readUint64());
    }),
    o.createFullBoxCtor("pssh", function (e) {
      if (((this.system_id = o.parseHex16(e)), this.version > 0)) {
        var r = e.readUint32();
        this.kid = [];
        for (var n = 0; n < r; n++) this.kid[n] = o.parseHex16(e);
      }
      var a = e.readUint32();
      a > 0 && (this.data = e.readUint8Array(a));
    }),
    o.createFullBoxCtor("clef", function (e) {
      (this.width = e.readUint32()), (this.height = e.readUint32());
    }),
    o.createFullBoxCtor("enof", function (e) {
      (this.width = e.readUint32()), (this.height = e.readUint32());
    }),
    o.createFullBoxCtor("prof", function (e) {
      (this.width = e.readUint32()), (this.height = e.readUint32());
    }),
    o.createContainerBoxCtor("tapt", null, ["clef", "prof", "enof"]),
    o.createBoxCtor("rtp ", function (e) {
      (this.descriptionformat = e.readString(4)),
        (this.sdptext = e.readString(this.size - this.hdr_size - 4));
    }),
    o.createFullBoxCtor("saio", function (e) {
      this.flags & 1 &&
        ((this.aux_info_type = e.readUint32()),
        (this.aux_info_type_parameter = e.readUint32()));
      var r = e.readUint32();
      this.offset = [];
      for (var n = 0; n < r; n++)
        this.version === 0
          ? (this.offset[n] = e.readUint32())
          : (this.offset[n] = e.readUint64());
    }),
    o.createFullBoxCtor("saiz", function (e) {
      this.flags & 1 &&
        ((this.aux_info_type = e.readUint32()),
        (this.aux_info_type_parameter = e.readUint32())),
        (this.default_sample_info_size = e.readUint8());
      var r = e.readUint32();
      if (((this.sample_info_size = []), this.default_sample_info_size === 0))
        for (var n = 0; n < r; n++) this.sample_info_size[n] = e.readUint8();
    }),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_METADATA, "mett", function (e) {
      this.parseHeader(e),
        (this.content_encoding = e.readCString()),
        (this.mime_format = e.readCString()),
        this.parseFooter(e);
    }),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_METADATA, "metx", function (e) {
      this.parseHeader(e),
        (this.content_encoding = e.readCString()),
        (this.namespace = e.readCString()),
        (this.schema_location = e.readCString()),
        this.parseFooter(e);
    }),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_SUBTITLE, "sbtt", function (e) {
      this.parseHeader(e),
        (this.content_encoding = e.readCString()),
        (this.mime_format = e.readCString()),
        this.parseFooter(e);
    }),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_SUBTITLE, "stpp", function (e) {
      this.parseHeader(e),
        (this.namespace = e.readCString()),
        (this.schema_location = e.readCString()),
        (this.auxiliary_mime_types = e.readCString()),
        this.parseFooter(e);
    }),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_SUBTITLE, "stxt", function (e) {
      this.parseHeader(e),
        (this.content_encoding = e.readCString()),
        (this.mime_format = e.readCString()),
        this.parseFooter(e);
    }),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_SUBTITLE, "tx3g", function (e) {
      this.parseHeader(e),
        (this.displayFlags = e.readUint32()),
        (this.horizontal_justification = e.readInt8()),
        (this.vertical_justification = e.readInt8()),
        (this.bg_color_rgba = e.readUint8Array(4)),
        (this.box_record = e.readInt16Array(4)),
        (this.style_record = e.readUint8Array(12)),
        this.parseFooter(e);
    }),
    o.createSampleEntryCtor(o.SAMPLE_ENTRY_TYPE_METADATA, "wvtt", function (e) {
      this.parseHeader(e), this.parseFooter(e);
    }),
    o.createSampleGroupCtor("alst", function (e) {
      var r,
        n = e.readUint16();
      for (
        this.first_output_sample = e.readUint16(),
          this.sample_offset = [],
          r = 0;
        r < n;
        r++
      )
        this.sample_offset[r] = e.readUint32();
      var a = this.description_length - 4 - 4 * n;
      for (
        this.num_output_samples = [], this.num_total_samples = [], r = 0;
        r < a / 4;
        r++
      )
        (this.num_output_samples[r] = e.readUint16()),
          (this.num_total_samples[r] = e.readUint16());
    }),
    o.createSampleGroupCtor("avll", function (e) {
      (this.layerNumber = e.readUint8()),
        (this.accurateStatisticsFlag = e.readUint8()),
        (this.avgBitRate = e.readUint16()),
        (this.avgFrameRate = e.readUint16());
    }),
    o.createSampleGroupCtor("avss", function (e) {
      (this.subSequenceIdentifier = e.readUint16()),
        (this.layerNumber = e.readUint8());
      var r = e.readUint8();
      (this.durationFlag = r >> 7),
        (this.avgRateFlag = (r >> 6) & 1),
        this.durationFlag && (this.duration = e.readUint32()),
        this.avgRateFlag &&
          ((this.accurateStatisticsFlag = e.readUint8()),
          (this.avgBitRate = e.readUint16()),
          (this.avgFrameRate = e.readUint16())),
        (this.dependency = []);
      for (var n = e.readUint8(), a = 0; a < n; a++) {
        var l = {};
        this.dependency.push(l),
          (l.subSeqDirectionFlag = e.readUint8()),
          (l.layerNumber = e.readUint8()),
          (l.subSequenceIdentifier = e.readUint16());
      }
    }),
    o.createSampleGroupCtor("dtrt", function (e) {
      f.warn(
        "BoxParser",
        "Sample Group type: " + this.grouping_type + " not fully parsed"
      );
    }),
    o.createSampleGroupCtor("mvif", function (e) {
      f.warn(
        "BoxParser",
        "Sample Group type: " + this.grouping_type + " not fully parsed"
      );
    }),
    o.createSampleGroupCtor("prol", function (e) {
      this.roll_distance = e.readInt16();
    }),
    o.createSampleGroupCtor("rap ", function (e) {
      var r = e.readUint8();
      (this.num_leading_samples_known = r >> 7),
        (this.num_leading_samples = r & 127);
    }),
    o.createSampleGroupCtor("rash", function (e) {
      if (
        ((this.operation_point_count = e.readUint16()),
        this.description_length !==
          2 +
            (this.operation_point_count === 1
              ? 2
              : this.operation_point_count * 6) +
            9)
      )
        f.warn(
          "BoxParser",
          "Mismatch in " + this.grouping_type + " sample group length"
        ),
          (this.data = e.readUint8Array(this.description_length - 2));
      else {
        if (this.operation_point_count === 1)
          this.target_rate_share = e.readUint16();
        else {
          (this.target_rate_share = []), (this.available_bitrate = []);
          for (var r = 0; r < this.operation_point_count; r++)
            (this.available_bitrate[r] = e.readUint32()),
              (this.target_rate_share[r] = e.readUint16());
        }
        (this.maximum_bitrate = e.readUint32()),
          (this.minimum_bitrate = e.readUint32()),
          (this.discard_priority = e.readUint8());
      }
    }),
    o.createSampleGroupCtor("roll", function (e) {
      this.roll_distance = e.readInt16();
    }),
    (o.SampleGroupEntry.prototype.parse = function (e) {
      f.warn("BoxParser", "Unknown Sample Group type: " + this.grouping_type),
        (this.data = e.readUint8Array(this.description_length));
    }),
    o.createSampleGroupCtor("scif", function (e) {
      f.warn(
        "BoxParser",
        "Sample Group type: " + this.grouping_type + " not fully parsed"
      );
    }),
    o.createSampleGroupCtor("scnm", function (e) {
      f.warn(
        "BoxParser",
        "Sample Group type: " + this.grouping_type + " not fully parsed"
      );
    }),
    o.createSampleGroupCtor("seig", function (e) {
      this.reserved = e.readUint8();
      var r = e.readUint8();
      (this.crypt_byte_block = r >> 4),
        (this.skip_byte_block = r & 15),
        (this.isProtected = e.readUint8()),
        (this.Per_Sample_IV_Size = e.readUint8()),
        (this.KID = o.parseHex16(e)),
        (this.constant_IV_size = 0),
        (this.constant_IV = 0),
        this.isProtected === 1 &&
          this.Per_Sample_IV_Size === 0 &&
          ((this.constant_IV_size = e.readUint8()),
          (this.constant_IV = e.readUint8Array(this.constant_IV_size)));
    }),
    o.createSampleGroupCtor("stsa", function (e) {
      f.warn(
        "BoxParser",
        "Sample Group type: " + this.grouping_type + " not fully parsed"
      );
    }),
    o.createSampleGroupCtor("sync", function (e) {
      var r = e.readUint8();
      this.NAL_unit_type = r & 63;
    }),
    o.createSampleGroupCtor("tele", function (e) {
      var r = e.readUint8();
      this.level_independently_decodable = r >> 7;
    }),
    o.createSampleGroupCtor("tsas", function (e) {
      f.warn(
        "BoxParser",
        "Sample Group type: " + this.grouping_type + " not fully parsed"
      );
    }),
    o.createSampleGroupCtor("tscl", function (e) {
      f.warn(
        "BoxParser",
        "Sample Group type: " + this.grouping_type + " not fully parsed"
      );
    }),
    o.createSampleGroupCtor("vipr", function (e) {
      f.warn(
        "BoxParser",
        "Sample Group type: " + this.grouping_type + " not fully parsed"
      );
    }),
    o.createFullBoxCtor("sbgp", function (e) {
      (this.grouping_type = e.readString(4)),
        this.version === 1
          ? (this.grouping_type_parameter = e.readUint32())
          : (this.grouping_type_parameter = 0),
        (this.entries = []);
      for (var r = e.readUint32(), n = 0; n < r; n++) {
        var a = {};
        this.entries.push(a),
          (a.sample_count = e.readInt32()),
          (a.group_description_index = e.readInt32());
      }
    }),
    o.createFullBoxCtor("schm", function (e) {
      (this.scheme_type = e.readString(4)),
        (this.scheme_version = e.readUint32()),
        this.flags & 1 &&
          (this.scheme_uri = e.readString(this.size - this.hdr_size - 8));
    }),
    o.createBoxCtor("sdp ", function (e) {
      this.sdptext = e.readString(this.size - this.hdr_size);
    }),
    o.createFullBoxCtor("sdtp", function (e) {
      var r,
        n = this.size - this.hdr_size;
      (this.is_leading = []),
        (this.sample_depends_on = []),
        (this.sample_is_depended_on = []),
        (this.sample_has_redundancy = []);
      for (var a = 0; a < n; a++)
        (r = e.readUint8()),
          (this.is_leading[a] = r >> 6),
          (this.sample_depends_on[a] = (r >> 4) & 3),
          (this.sample_is_depended_on[a] = (r >> 2) & 3),
          (this.sample_has_redundancy[a] = r & 3);
    }),
    o.createFullBoxCtor("senc"),
    o.createFullBoxCtor("sgpd", function (e) {
      (this.grouping_type = e.readString(4)),
        f.debug(
          "BoxParser",
          "Found Sample Groups of type " + this.grouping_type
        ),
        this.version === 1
          ? (this.default_length = e.readUint32())
          : (this.default_length = 0),
        this.version >= 2 &&
          (this.default_group_description_index = e.readUint32()),
        (this.entries = []);
      for (var r = e.readUint32(), n = 0; n < r; n++) {
        var a;
        o[this.grouping_type + "SampleGroupEntry"]
          ? (a = new o[this.grouping_type + "SampleGroupEntry"](
              this.grouping_type
            ))
          : (a = new o.SampleGroupEntry(this.grouping_type)),
          this.entries.push(a),
          this.version === 1
            ? this.default_length === 0
              ? (a.description_length = e.readUint32())
              : (a.description_length = this.default_length)
            : (a.description_length = this.default_length),
          a.write === o.SampleGroupEntry.prototype.write &&
            (f.info(
              "BoxParser",
              "SampleGroup for type " +
                this.grouping_type +
                " writing not yet implemented, keeping unparsed data in memory for later write"
            ),
            (a.data = e.readUint8Array(a.description_length)),
            (e.position -= a.description_length)),
          a.parse(e);
      }
    }),
    o.createFullBoxCtor("sidx", function (e) {
      (this.reference_ID = e.readUint32()),
        (this.timescale = e.readUint32()),
        this.version === 0
          ? ((this.earliest_presentation_time = e.readUint32()),
            (this.first_offset = e.readUint32()))
          : ((this.earliest_presentation_time = e.readUint64()),
            (this.first_offset = e.readUint64())),
        e.readUint16(),
        (this.references = []);
      for (var r = e.readUint16(), n = 0; n < r; n++) {
        var a = {};
        this.references.push(a);
        var l = e.readUint32();
        (a.reference_type = (l >> 31) & 1),
          (a.referenced_size = l & 2147483647),
          (a.subsegment_duration = e.readUint32()),
          (l = e.readUint32()),
          (a.starts_with_SAP = (l >> 31) & 1),
          (a.SAP_type = (l >> 28) & 7),
          (a.SAP_delta_time = l & 268435455);
      }
    }),
    (o.SingleItemTypeReferenceBox = function (e, r, n, a) {
      o.Box.call(this, e, r), (this.hdr_size = n), (this.start = a);
    }),
    (o.SingleItemTypeReferenceBox.prototype = new o.Box()),
    (o.SingleItemTypeReferenceBox.prototype.parse = function (e) {
      this.from_item_ID = e.readUint16();
      var r = e.readUint16();
      this.references = [];
      for (var n = 0; n < r; n++) this.references[n] = e.readUint16();
    }),
    (o.SingleItemTypeReferenceBoxLarge = function (e, r, n, a) {
      o.Box.call(this, e, r), (this.hdr_size = n), (this.start = a);
    }),
    (o.SingleItemTypeReferenceBoxLarge.prototype = new o.Box()),
    (o.SingleItemTypeReferenceBoxLarge.prototype.parse = function (e) {
      this.from_item_ID = e.readUint32();
      var r = e.readUint16();
      this.references = [];
      for (var n = 0; n < r; n++) this.references[n] = e.readUint32();
    }),
    o.createFullBoxCtor("SmDm", function (e) {
      (this.primaryRChromaticity_x = e.readUint16()),
        (this.primaryRChromaticity_y = e.readUint16()),
        (this.primaryGChromaticity_x = e.readUint16()),
        (this.primaryGChromaticity_y = e.readUint16()),
        (this.primaryBChromaticity_x = e.readUint16()),
        (this.primaryBChromaticity_y = e.readUint16()),
        (this.whitePointChromaticity_x = e.readUint16()),
        (this.whitePointChromaticity_y = e.readUint16()),
        (this.luminanceMax = e.readUint32()),
        (this.luminanceMin = e.readUint32());
    }),
    o.createFullBoxCtor("smhd", function (e) {
      (this.balance = e.readUint16()), e.readUint16();
    }),
    o.createFullBoxCtor("ssix", function (e) {
      this.subsegments = [];
      for (var r = e.readUint32(), n = 0; n < r; n++) {
        var a = {};
        this.subsegments.push(a), (a.ranges = []);
        for (var l = e.readUint32(), m = 0; m < l; m++) {
          var p = {};
          a.ranges.push(p),
            (p.level = e.readUint8()),
            (p.range_size = e.readUint24());
        }
      }
    }),
    o.createFullBoxCtor("stco", function (e) {
      var r;
      if (((r = e.readUint32()), (this.chunk_offsets = []), this.version === 0))
        for (var n = 0; n < r; n++) this.chunk_offsets.push(e.readUint32());
    }),
    o.createFullBoxCtor("stdp", function (e) {
      var r = (this.size - this.hdr_size) / 2;
      this.priority = [];
      for (var n = 0; n < r; n++) this.priority[n] = e.readUint16();
    }),
    o.createFullBoxCtor("sthd"),
    o.createFullBoxCtor("stri", function (e) {
      (this.switch_group = e.readUint16()),
        (this.alternate_group = e.readUint16()),
        (this.sub_track_id = e.readUint32());
      var r = (this.size - this.hdr_size - 8) / 4;
      this.attribute_list = [];
      for (var n = 0; n < r; n++) this.attribute_list[n] = e.readUint32();
    }),
    o.createFullBoxCtor("stsc", function (e) {
      var r, n;
      if (
        ((r = e.readUint32()),
        (this.first_chunk = []),
        (this.samples_per_chunk = []),
        (this.sample_description_index = []),
        this.version === 0)
      )
        for (n = 0; n < r; n++)
          this.first_chunk.push(e.readUint32()),
            this.samples_per_chunk.push(e.readUint32()),
            this.sample_description_index.push(e.readUint32());
    }),
    o.createFullBoxCtor("stsd", function (e) {
      var r, n, a, l;
      for (this.entries = [], a = e.readUint32(), r = 1; r <= a; r++)
        if (
          ((n = o.parseOneBox(
            e,
            !0,
            this.size - (e.getPosition() - this.start)
          )),
          n.code === o.OK)
        )
          o[n.type + "SampleEntry"]
            ? ((l = new o[n.type + "SampleEntry"](n.size)),
              (l.hdr_size = n.hdr_size),
              (l.start = n.start))
            : (f.warn("BoxParser", "Unknown sample entry type: " + n.type),
              (l = new o.SampleEntry(n.type, n.size, n.hdr_size, n.start))),
            l.write === o.SampleEntry.prototype.write &&
              (f.info(
                "BoxParser",
                "SampleEntry " +
                  l.type +
                  " box writing not yet implemented, keeping unparsed data in memory for later write"
              ),
              l.parseDataAndRewind(e)),
            l.parse(e),
            this.entries.push(l);
        else return;
    }),
    o.createFullBoxCtor("stsg", function (e) {
      this.grouping_type = e.readUint32();
      var r = e.readUint16();
      this.group_description_index = [];
      for (var n = 0; n < r; n++)
        this.group_description_index[n] = e.readUint32();
    }),
    o.createFullBoxCtor("stsh", function (e) {
      var r, n;
      if (
        ((r = e.readUint32()),
        (this.shadowed_sample_numbers = []),
        (this.sync_sample_numbers = []),
        this.version === 0)
      )
        for (n = 0; n < r; n++)
          this.shadowed_sample_numbers.push(e.readUint32()),
            this.sync_sample_numbers.push(e.readUint32());
    }),
    o.createFullBoxCtor("stss", function (e) {
      var r, n;
      if (((n = e.readUint32()), this.version === 0))
        for (this.sample_numbers = [], r = 0; r < n; r++)
          this.sample_numbers.push(e.readUint32());
    }),
    o.createFullBoxCtor("stsz", function (e) {
      var r;
      if (((this.sample_sizes = []), this.version === 0))
        for (
          this.sample_size = e.readUint32(),
            this.sample_count = e.readUint32(),
            r = 0;
          r < this.sample_count;
          r++
        )
          this.sample_size === 0
            ? this.sample_sizes.push(e.readUint32())
            : (this.sample_sizes[r] = this.sample_size);
    }),
    o.createFullBoxCtor("stts", function (e) {
      var r, n, a;
      if (
        ((r = e.readUint32()),
        (this.sample_counts = []),
        (this.sample_deltas = []),
        this.version === 0)
      )
        for (n = 0; n < r; n++)
          this.sample_counts.push(e.readUint32()),
            (a = e.readInt32()),
            a < 0 &&
              (f.warn(
                "BoxParser",
                "File uses negative stts sample delta, using value 1 instead, sync may be lost!"
              ),
              (a = 1)),
            this.sample_deltas.push(a);
    }),
    o.createFullBoxCtor("stvi", function (e) {
      var r = e.readUint32();
      (this.single_view_allowed = r & 3), (this.stereo_scheme = e.readUint32());
      var n = e.readUint32();
      this.stereo_indication_type = e.readString(n);
      var a, l;
      for (this.boxes = []; e.getPosition() < this.start + this.size; )
        if (
          ((a = o.parseOneBox(
            e,
            !1,
            this.size - (e.getPosition() - this.start)
          )),
          a.code === o.OK)
        )
          (l = a.box), this.boxes.push(l), (this[l.type] = l);
        else return;
    }),
    o.createBoxCtor("styp", function (e) {
      o.ftypBox.prototype.parse.call(this, e);
    }),
    o.createFullBoxCtor("stz2", function (e) {
      var r, n;
      if (((this.sample_sizes = []), this.version === 0))
        if (
          ((this.reserved = e.readUint24()),
          (this.field_size = e.readUint8()),
          (n = e.readUint32()),
          this.field_size === 4)
        )
          for (r = 0; r < n; r += 2) {
            var a = e.readUint8();
            (this.sample_sizes[r] = (a >> 4) & 15),
              (this.sample_sizes[r + 1] = a & 15);
          }
        else if (this.field_size === 8)
          for (r = 0; r < n; r++) this.sample_sizes[r] = e.readUint8();
        else if (this.field_size === 16)
          for (r = 0; r < n; r++) this.sample_sizes[r] = e.readUint16();
        else f.error("BoxParser", "Error in length field in stz2 box");
    }),
    o.createFullBoxCtor("subs", function (e) {
      var r, n, a, l;
      for (a = e.readUint32(), this.entries = [], r = 0; r < a; r++) {
        var m = {};
        if (
          ((this.entries[r] = m),
          (m.sample_delta = e.readUint32()),
          (m.subsamples = []),
          (l = e.readUint16()),
          l > 0)
        )
          for (n = 0; n < l; n++) {
            var p = {};
            m.subsamples.push(p),
              this.version == 1
                ? (p.size = e.readUint32())
                : (p.size = e.readUint16()),
              (p.priority = e.readUint8()),
              (p.discardable = e.readUint8()),
              (p.codec_specific_parameters = e.readUint32());
          }
      }
    }),
    o.createFullBoxCtor("tenc", function (e) {
      if ((e.readUint8(), this.version === 0)) e.readUint8();
      else {
        var r = e.readUint8();
        (this.default_crypt_byte_block = (r >> 4) & 15),
          (this.default_skip_byte_block = r & 15);
      }
      (this.default_isProtected = e.readUint8()),
        (this.default_Per_Sample_IV_Size = e.readUint8()),
        (this.default_KID = o.parseHex16(e)),
        this.default_isProtected === 1 &&
          this.default_Per_Sample_IV_Size === 0 &&
          ((this.default_constant_IV_size = e.readUint8()),
          (this.default_constant_IV = e.readUint8Array(
            this.default_constant_IV_size
          )));
    }),
    o.createFullBoxCtor("tfdt", function (e) {
      this.version == 1
        ? (this.baseMediaDecodeTime = e.readUint64())
        : (this.baseMediaDecodeTime = e.readUint32());
    }),
    o.createFullBoxCtor("tfhd", function (e) {
      var r = 0;
      (this.track_id = e.readUint32()),
        this.size - this.hdr_size > r &&
        this.flags & o.TFHD_FLAG_BASE_DATA_OFFSET
          ? ((this.base_data_offset = e.readUint64()), (r += 8))
          : (this.base_data_offset = 0),
        this.size - this.hdr_size > r && this.flags & o.TFHD_FLAG_SAMPLE_DESC
          ? ((this.default_sample_description_index = e.readUint32()), (r += 4))
          : (this.default_sample_description_index = 0),
        this.size - this.hdr_size > r && this.flags & o.TFHD_FLAG_SAMPLE_DUR
          ? ((this.default_sample_duration = e.readUint32()), (r += 4))
          : (this.default_sample_duration = 0),
        this.size - this.hdr_size > r && this.flags & o.TFHD_FLAG_SAMPLE_SIZE
          ? ((this.default_sample_size = e.readUint32()), (r += 4))
          : (this.default_sample_size = 0),
        this.size - this.hdr_size > r && this.flags & o.TFHD_FLAG_SAMPLE_FLAGS
          ? ((this.default_sample_flags = e.readUint32()), (r += 4))
          : (this.default_sample_flags = 0);
    }),
    o.createFullBoxCtor("tfra", function (e) {
      (this.track_ID = e.readUint32()), e.readUint24();
      var r = e.readUint8();
      (this.length_size_of_traf_num = (r >> 4) & 3),
        (this.length_size_of_trun_num = (r >> 2) & 3),
        (this.length_size_of_sample_num = r & 3),
        (this.entries = []);
      for (var n = e.readUint32(), a = 0; a < n; a++)
        this.version === 1
          ? ((this.time = e.readUint64()), (this.moof_offset = e.readUint64()))
          : ((this.time = e.readUint32()), (this.moof_offset = e.readUint32())),
          (this.traf_number =
            e["readUint" + 8 * (this.length_size_of_traf_num + 1)]()),
          (this.trun_number =
            e["readUint" + 8 * (this.length_size_of_trun_num + 1)]()),
          (this.sample_number =
            e["readUint" + 8 * (this.length_size_of_sample_num + 1)]());
    }),
    o.createFullBoxCtor("tkhd", function (e) {
      this.version == 1
        ? ((this.creation_time = e.readUint64()),
          (this.modification_time = e.readUint64()),
          (this.track_id = e.readUint32()),
          e.readUint32(),
          (this.duration = e.readUint64()))
        : ((this.creation_time = e.readUint32()),
          (this.modification_time = e.readUint32()),
          (this.track_id = e.readUint32()),
          e.readUint32(),
          (this.duration = e.readUint32())),
        e.readUint32Array(2),
        (this.layer = e.readInt16()),
        (this.alternate_group = e.readInt16()),
        (this.volume = e.readInt16() >> 8),
        e.readUint16(),
        (this.matrix = e.readInt32Array(9)),
        (this.width = e.readUint32()),
        (this.height = e.readUint32());
    }),
    o.createBoxCtor("tmax", function (e) {
      this.time = e.readUint32();
    }),
    o.createBoxCtor("tmin", function (e) {
      this.time = e.readUint32();
    }),
    o.createBoxCtor("totl", function (e) {
      this.bytessent = e.readUint32();
    }),
    o.createBoxCtor("tpay", function (e) {
      this.bytessent = e.readUint32();
    }),
    o.createBoxCtor("tpyl", function (e) {
      this.bytessent = e.readUint64();
    }),
    (o.TrackGroupTypeBox.prototype.parse = function (e) {
      this.parseFullHeader(e), (this.track_group_id = e.readUint32());
    }),
    o.createTrackGroupCtor("msrc"),
    (o.TrackReferenceTypeBox = function (e, r, n, a) {
      o.Box.call(this, e, r), (this.hdr_size = n), (this.start = a);
    }),
    (o.TrackReferenceTypeBox.prototype = new o.Box()),
    (o.TrackReferenceTypeBox.prototype.parse = function (e) {
      this.track_ids = e.readUint32Array((this.size - this.hdr_size) / 4);
    }),
    (o.trefBox.prototype.parse = function (e) {
      for (var r, n; e.getPosition() < this.start + this.size; )
        if (
          ((r = o.parseOneBox(
            e,
            !0,
            this.size - (e.getPosition() - this.start)
          )),
          r.code === o.OK)
        )
          (n = new o.TrackReferenceTypeBox(
            r.type,
            r.size,
            r.hdr_size,
            r.start
          )),
            n.write === o.Box.prototype.write &&
              n.type !== "mdat" &&
              (f.info(
                "BoxParser",
                "TrackReference " +
                  n.type +
                  " box writing not yet implemented, keeping unparsed data in memory for later write"
              ),
              n.parseDataAndRewind(e)),
            n.parse(e),
            this.boxes.push(n);
        else return;
    }),
    o.createFullBoxCtor("trep", function (e) {
      for (
        this.track_ID = e.readUint32(), this.boxes = [];
        e.getPosition() < this.start + this.size;

      )
        if (
          ((ret = o.parseOneBox(
            e,
            !1,
            this.size - (e.getPosition() - this.start)
          )),
          ret.code === o.OK)
        )
          (box = ret.box), this.boxes.push(box);
        else return;
    }),
    o.createFullBoxCtor("trex", function (e) {
      (this.track_id = e.readUint32()),
        (this.default_sample_description_index = e.readUint32()),
        (this.default_sample_duration = e.readUint32()),
        (this.default_sample_size = e.readUint32()),
        (this.default_sample_flags = e.readUint32());
    }),
    o.createBoxCtor("trpy", function (e) {
      this.bytessent = e.readUint64();
    }),
    o.createFullBoxCtor("trun", function (e) {
      var r = 0;
      if (
        ((this.sample_count = e.readUint32()),
        (r += 4),
        this.size - this.hdr_size > r && this.flags & o.TRUN_FLAGS_DATA_OFFSET
          ? ((this.data_offset = e.readInt32()), (r += 4))
          : (this.data_offset = 0),
        this.size - this.hdr_size > r && this.flags & o.TRUN_FLAGS_FIRST_FLAG
          ? ((this.first_sample_flags = e.readUint32()), (r += 4))
          : (this.first_sample_flags = 0),
        (this.sample_duration = []),
        (this.sample_size = []),
        (this.sample_flags = []),
        (this.sample_composition_time_offset = []),
        this.size - this.hdr_size > r)
      )
        for (var n = 0; n < this.sample_count; n++)
          this.flags & o.TRUN_FLAGS_DURATION &&
            (this.sample_duration[n] = e.readUint32()),
            this.flags & o.TRUN_FLAGS_SIZE &&
              (this.sample_size[n] = e.readUint32()),
            this.flags & o.TRUN_FLAGS_FLAGS &&
              (this.sample_flags[n] = e.readUint32()),
            this.flags & o.TRUN_FLAGS_CTS_OFFSET &&
              (this.version === 0
                ? (this.sample_composition_time_offset[n] = e.readUint32())
                : (this.sample_composition_time_offset[n] = e.readInt32()));
    }),
    o.createFullBoxCtor("tsel", function (e) {
      this.switch_group = e.readUint32();
      var r = (this.size - this.hdr_size - 4) / 4;
      this.attribute_list = [];
      for (var n = 0; n < r; n++) this.attribute_list[n] = e.readUint32();
    }),
    o.createFullBoxCtor("txtC", function (e) {
      this.config = e.readCString();
    }),
    o.createFullBoxCtor("url ", function (e) {
      this.flags !== 1 && (this.location = e.readCString());
    }),
    o.createFullBoxCtor("urn ", function (e) {
      (this.name = e.readCString()),
        this.size - this.hdr_size - this.name.length - 1 > 0 &&
          (this.location = e.readCString());
    }),
    o.createUUIDBox("a5d40b30e81411ddba2f0800200c9a66", !0, !1, function (e) {
      this.LiveServerManifest = e
        .readString(this.size - this.hdr_size)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
    }),
    o.createUUIDBox("d08a4f1810f34a82b6c832d8aba183d3", !0, !1, function (e) {
      this.system_id = o.parseHex16(e);
      var r = e.readUint32();
      r > 0 && (this.data = e.readUint8Array(r));
    }),
    o.createUUIDBox("a2394f525a9b4f14a2446c427c648df4", !0, !1),
    o.createUUIDBox("8974dbce7be74c5184f97148f9882554", !0, !1, function (e) {
      (this.default_AlgorithmID = e.readUint24()),
        (this.default_IV_size = e.readUint8()),
        (this.default_KID = o.parseHex16(e));
    }),
    o.createUUIDBox("d4807ef2ca3946958e5426cb9e46a79f", !0, !1, function (e) {
      (this.fragment_count = e.readUint8()), (this.entries = []);
      for (var r = 0; r < this.fragment_count; r++) {
        var n = {},
          a = 0,
          l = 0;
        this.version === 1
          ? ((a = e.readUint64()), (l = e.readUint64()))
          : ((a = e.readUint32()), (l = e.readUint32())),
          (n.absolute_time = a),
          (n.absolute_duration = l),
          this.entries.push(n);
      }
    }),
    o.createUUIDBox("6d1d9b0542d544e680e2141daff757b2", !0, !1, function (e) {
      this.version === 1
        ? ((this.absolute_time = e.readUint64()),
          (this.duration = e.readUint64()))
        : ((this.absolute_time = e.readUint32()),
          (this.duration = e.readUint32()));
    }),
    o.createFullBoxCtor("vmhd", function (e) {
      (this.graphicsmode = e.readUint16()),
        (this.opcolor = e.readUint16Array(3));
    }),
    o.createFullBoxCtor("vpcC", function (e) {
      var r;
      this.version === 1
        ? ((this.profile = e.readUint8()),
          (this.level = e.readUint8()),
          (r = e.readUint8()),
          (this.bitDepth = r >> 4),
          (this.chromaSubsampling = (r >> 1) & 7),
          (this.videoFullRangeFlag = r & 1),
          (this.colourPrimaries = e.readUint8()),
          (this.transferCharacteristics = e.readUint8()),
          (this.matrixCoefficients = e.readUint8()),
          (this.codecIntializationDataSize = e.readUint16()),
          (this.codecIntializationData = e.readUint8Array(
            this.codecIntializationDataSize
          )))
        : ((this.profile = e.readUint8()),
          (this.level = e.readUint8()),
          (r = e.readUint8()),
          (this.bitDepth = (r >> 4) & 15),
          (this.colorSpace = r & 15),
          (r = e.readUint8()),
          (this.chromaSubsampling = (r >> 4) & 15),
          (this.transferFunction = (r >> 1) & 7),
          (this.videoFullRangeFlag = r & 1),
          (this.codecIntializationDataSize = e.readUint16()),
          (this.codecIntializationData = e.readUint8Array(
            this.codecIntializationDataSize
          )));
    }),
    o.createBoxCtor("vttC", function (e) {
      this.text = e.readString(this.size - this.hdr_size);
    }),
    o.createFullBoxCtor("vvcC", function (e) {
      var r,
        n,
        a = {
          held_bits: void 0,
          num_held_bits: 0,
          stream_read_1_bytes: function (I) {
            (this.held_bits = I.readUint8()), (this.num_held_bits = 8);
          },
          stream_read_2_bytes: function (I) {
            (this.held_bits = I.readUint16()), (this.num_held_bits = 16);
          },
          extract_bits: function (I) {
            var $ =
              (this.held_bits >> (this.num_held_bits - I)) & ((1 << I) - 1);
            return (this.num_held_bits -= I), $;
          },
        };
      if (
        (a.stream_read_1_bytes(e),
        a.extract_bits(5),
        (this.lengthSizeMinusOne = a.extract_bits(2)),
        (this.ptl_present_flag = a.extract_bits(1)),
        this.ptl_present_flag)
      ) {
        a.stream_read_2_bytes(e),
          (this.ols_idx = a.extract_bits(9)),
          (this.num_sublayers = a.extract_bits(3)),
          (this.constant_frame_rate = a.extract_bits(2)),
          (this.chroma_format_idc = a.extract_bits(2)),
          a.stream_read_1_bytes(e),
          (this.bit_depth_minus8 = a.extract_bits(3)),
          a.extract_bits(5);
        {
          if (
            (a.stream_read_2_bytes(e),
            a.extract_bits(2),
            (this.num_bytes_constraint_info = a.extract_bits(6)),
            (this.general_profile_idc = a.extract_bits(7)),
            (this.general_tier_flag = a.extract_bits(1)),
            (this.general_level_idc = e.readUint8()),
            a.stream_read_1_bytes(e),
            (this.ptl_frame_only_constraint_flag = a.extract_bits(1)),
            (this.ptl_multilayer_enabled_flag = a.extract_bits(1)),
            (this.general_constraint_info = new Uint8Array(
              this.num_bytes_constraint_info
            )),
            this.num_bytes_constraint_info)
          ) {
            for (r = 0; r < this.num_bytes_constraint_info - 1; r++) {
              var l = a.extract_bits(6);
              a.stream_read_1_bytes(e);
              var m = a.extract_bits(2);
              this.general_constraint_info[r] = (l << 2) | m;
            }
            this.general_constraint_info[this.num_bytes_constraint_info - 1] =
              a.extract_bits(6);
          } else a.extract_bits(6);
          for (
            a.stream_read_1_bytes(e),
              this.ptl_sublayer_present_mask = 0,
              n = this.num_sublayers - 2;
            n >= 0;
            --n
          ) {
            var p = a.extract_bits(1);
            this.ptl_sublayer_present_mask |= p << n;
          }
          for (n = this.num_sublayers; n <= 8 && this.num_sublayers > 1; ++n)
            a.extract_bits(1);
          for (n = this.num_sublayers - 2; n >= 0; --n)
            this.ptl_sublayer_present_mask & (1 << n) &&
              (this.sublayer_level_idc[n] = e.readUint8());
          if (
            ((this.ptl_num_sub_profiles = e.readUint8()),
            (this.general_sub_profile_idc = []),
            this.ptl_num_sub_profiles)
          )
            for (r = 0; r < this.ptl_num_sub_profiles; r++)
              this.general_sub_profile_idc.push(e.readUint32());
        }
        (this.max_picture_width = e.readUint16()),
          (this.max_picture_height = e.readUint16()),
          (this.avg_frame_rate = e.readUint16());
      }
      var S = 12,
        R = 13;
      this.nalu_arrays = [];
      var O = e.readUint8();
      for (r = 0; r < O; r++) {
        var P = [];
        this.nalu_arrays.push(P),
          a.stream_read_1_bytes(e),
          (P.completeness = a.extract_bits(1)),
          a.extract_bits(2),
          (P.nalu_type = a.extract_bits(5));
        var y = 1;
        for (
          P.nalu_type != R && P.nalu_type != S && (y = e.readUint16()), n = 0;
          n < y;
          n++
        ) {
          var F = e.readUint16();
          P.push({
            data: e.readUint8Array(F),
            length: F,
          });
        }
      }
    }),
    o.createFullBoxCtor("vvnC", function (e) {
      var r = strm.readUint8();
      this.lengthSizeMinusOne = r & 3;
    }),
    (o.SampleEntry.prototype.isVideo = function () {
      return !1;
    }),
    (o.SampleEntry.prototype.isAudio = function () {
      return !1;
    }),
    (o.SampleEntry.prototype.isSubtitle = function () {
      return !1;
    }),
    (o.SampleEntry.prototype.isMetadata = function () {
      return !1;
    }),
    (o.SampleEntry.prototype.isHint = function () {
      return !1;
    }),
    (o.SampleEntry.prototype.getCodec = function () {
      return this.type.replace(".", "");
    }),
    (o.SampleEntry.prototype.getWidth = function () {
      return "";
    }),
    (o.SampleEntry.prototype.getHeight = function () {
      return "";
    }),
    (o.SampleEntry.prototype.getChannelCount = function () {
      return "";
    }),
    (o.SampleEntry.prototype.getSampleRate = function () {
      return "";
    }),
    (o.SampleEntry.prototype.getSampleSize = function () {
      return "";
    }),
    (o.VisualSampleEntry.prototype.isVideo = function () {
      return !0;
    }),
    (o.VisualSampleEntry.prototype.getWidth = function () {
      return this.width;
    }),
    (o.VisualSampleEntry.prototype.getHeight = function () {
      return this.height;
    }),
    (o.AudioSampleEntry.prototype.isAudio = function () {
      return !0;
    }),
    (o.AudioSampleEntry.prototype.getChannelCount = function () {
      return this.channel_count;
    }),
    (o.AudioSampleEntry.prototype.getSampleRate = function () {
      return this.samplerate;
    }),
    (o.AudioSampleEntry.prototype.getSampleSize = function () {
      return this.samplesize;
    }),
    (o.SubtitleSampleEntry.prototype.isSubtitle = function () {
      return !0;
    }),
    (o.MetadataSampleEntry.prototype.isMetadata = function () {
      return !0;
    }),
    (o.decimalToHex = function (e, r) {
      var n = Number(e).toString(16);
      for (r = typeof r > "u" || r === null ? (r = 2) : r; n.length < r; )
        n = "0" + n;
      return n;
    }),
    (o.avc1SampleEntry.prototype.getCodec =
      o.avc2SampleEntry.prototype.getCodec =
      o.avc3SampleEntry.prototype.getCodec =
      o.avc4SampleEntry.prototype.getCodec =
        function () {
          var e = o.SampleEntry.prototype.getCodec.call(this);
          return this.avcC
            ? e +
                "." +
                o.decimalToHex(this.avcC.AVCProfileIndication) +
                o.decimalToHex(this.avcC.profile_compatibility) +
                o.decimalToHex(this.avcC.AVCLevelIndication)
            : e;
        }),
    (o.hev1SampleEntry.prototype.getCodec =
      o.hvc1SampleEntry.prototype.getCodec =
        function () {
          var e,
            r = o.SampleEntry.prototype.getCodec.call(this);
          if (this.hvcC) {
            switch (((r += "."), this.hvcC.general_profile_space)) {
              case 0:
                r += "";
                break;
              case 1:
                r += "A";
                break;
              case 2:
                r += "B";
                break;
              case 3:
                r += "C";
                break;
            }
            (r += this.hvcC.general_profile_idc), (r += ".");
            var n = this.hvcC.general_profile_compatibility,
              a = 0;
            for (e = 0; e < 32 && ((a |= n & 1), e != 31); e++)
              (a <<= 1), (n >>= 1);
            (r += o.decimalToHex(a, 0)),
              (r += "."),
              this.hvcC.general_tier_flag === 0 ? (r += "L") : (r += "H"),
              (r += this.hvcC.general_level_idc);
            var l = !1,
              m = "";
            for (e = 5; e >= 0; e--)
              (this.hvcC.general_constraint_indicator[e] || l) &&
                ((m =
                  "." +
                  o.decimalToHex(this.hvcC.general_constraint_indicator[e], 0) +
                  m),
                (l = !0));
            r += m;
          }
          return r;
        }),
    (o.vvc1SampleEntry.prototype.getCodec =
      o.vvi1SampleEntry.prototype.getCodec =
        function () {
          var e,
            r = o.SampleEntry.prototype.getCodec.call(this);
          if (this.vvcC) {
            (r += "." + this.vvcC.general_profile_idc),
              this.vvcC.general_tier_flag ? (r += ".H") : (r += ".L"),
              (r += this.vvcC.general_level_idc);
            var n = "";
            if (this.vvcC.general_constraint_info) {
              var a = [],
                l = 0;
              (l |= this.vvcC.ptl_frame_only_constraint << 7),
                (l |= this.vvcC.ptl_multilayer_enabled << 6);
              var m;
              for (e = 0; e < this.vvcC.general_constraint_info.length; ++e)
                (l |= (this.vvcC.general_constraint_info[e] >> 2) & 63),
                  a.push(l),
                  l && (m = e),
                  (l = (this.vvcC.general_constraint_info[e] >> 2) & 3);
              if (m === void 0) n = ".CA";
              else {
                n = ".C";
                var p = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567",
                  S = 0,
                  R = 0;
                for (e = 0; e <= m; ++e)
                  for (S = (S << 8) | a[e], R += 8; R >= 5; ) {
                    var O = (S >> (R - 5)) & 31;
                    (n += p[O]), (R -= 5), (S &= (1 << R) - 1);
                  }
                R && ((S <<= 5 - R), (n += p[S & 31]));
              }
            }
            r += n;
          }
          return r;
        }),
    (o.mp4aSampleEntry.prototype.getCodec = function () {
      var e = o.SampleEntry.prototype.getCodec.call(this);
      if (this.esds && this.esds.esd) {
        var r = this.esds.esd.getOTI(),
          n = this.esds.esd.getAudioConfig();
        return e + "." + o.decimalToHex(r) + (n ? "." + n : "");
      } else return e;
    }),
    (o.stxtSampleEntry.prototype.getCodec = function () {
      var e = o.SampleEntry.prototype.getCodec.call(this);
      return this.mime_format ? e + "." + this.mime_format : e;
    }),
    (o.vp08SampleEntry.prototype.getCodec =
      o.vp09SampleEntry.prototype.getCodec =
        function () {
          var e = o.SampleEntry.prototype.getCodec.call(this),
            r = this.vpcC.level;
          r == 0 && (r = "00");
          var n = this.vpcC.bitDepth;
          return (
            n == 8 && (n = "08"),
            e + ".0" + this.vpcC.profile + "." + r + "." + n
          );
        }),
    (o.av01SampleEntry.prototype.getCodec = function () {
      var e = o.SampleEntry.prototype.getCodec.call(this),
        r = this.av1C.seq_level_idx_0;
      r < 10 && (r = "0" + r);
      var n;
      return (
        this.av1C.seq_profile === 2 && this.av1C.high_bitdepth === 1
          ? (n = this.av1C.twelve_bit === 1 ? "12" : "10")
          : this.av1C.seq_profile <= 2 &&
            (n = this.av1C.high_bitdepth === 1 ? "10" : "08"),
        e +
          "." +
          this.av1C.seq_profile +
          "." +
          r +
          (this.av1C.seq_tier_0 ? "H" : "M") +
          "." +
          n
      );
    }),
    (o.Box.prototype.writeHeader = function (e, r) {
      (this.size += 8),
        this.size > U && (this.size += 8),
        this.type === "uuid" && (this.size += 16),
        f.debug(
          "BoxWriter",
          "Writing box " +
            this.type +
            " of size: " +
            this.size +
            " at position " +
            e.getPosition() +
            (r || "")
        ),
        this.size > U
          ? e.writeUint32(1)
          : ((this.sizePosition = e.getPosition()), e.writeUint32(this.size)),
        e.writeString(this.type, null, 4),
        this.type === "uuid" && e.writeUint8Array(this.uuid),
        this.size > U && e.writeUint64(this.size);
    }),
    (o.FullBox.prototype.writeHeader = function (e) {
      (this.size += 4),
        o.Box.prototype.writeHeader.call(
          this,
          e,
          " v=" + this.version + " f=" + this.flags
        ),
        e.writeUint8(this.version),
        e.writeUint24(this.flags);
    }),
    (o.Box.prototype.write = function (e) {
      this.type === "mdat"
        ? this.data &&
          ((this.size = this.data.length),
          this.writeHeader(e),
          e.writeUint8Array(this.data))
        : ((this.size = this.data ? this.data.length : 0),
          this.writeHeader(e),
          this.data && e.writeUint8Array(this.data));
    }),
    (o.ContainerBox.prototype.write = function (e) {
      (this.size = 0), this.writeHeader(e);
      for (var r = 0; r < this.boxes.length; r++)
        this.boxes[r] &&
          (this.boxes[r].write(e), (this.size += this.boxes[r].size));
      f.debug(
        "BoxWriter",
        "Adjusting box " + this.type + " with new size " + this.size
      ),
        e.adjustUint32(this.sizePosition, this.size);
    }),
    (o.TrackReferenceTypeBox.prototype.write = function (e) {
      (this.size = this.track_ids.length * 4),
        this.writeHeader(e),
        e.writeUint32Array(this.track_ids);
    }),
    (o.avcCBox.prototype.write = function (e) {
      var r;
      for (this.size = 7, r = 0; r < this.SPS.length; r++)
        this.size += 2 + this.SPS[r].length;
      for (r = 0; r < this.PPS.length; r++) this.size += 2 + this.PPS[r].length;
      for (
        this.ext && (this.size += this.ext.length),
          this.writeHeader(e),
          e.writeUint8(this.configurationVersion),
          e.writeUint8(this.AVCProfileIndication),
          e.writeUint8(this.profile_compatibility),
          e.writeUint8(this.AVCLevelIndication),
          e.writeUint8(this.lengthSizeMinusOne + 252),
          e.writeUint8(this.SPS.length + 224),
          r = 0;
        r < this.SPS.length;
        r++
      )
        e.writeUint16(this.SPS[r].length), e.writeUint8Array(this.SPS[r].nalu);
      for (e.writeUint8(this.PPS.length), r = 0; r < this.PPS.length; r++)
        e.writeUint16(this.PPS[r].length), e.writeUint8Array(this.PPS[r].nalu);
      this.ext && e.writeUint8Array(this.ext);
    }),
    (o.co64Box.prototype.write = function (e) {
      var r;
      for (
        this.version = 0,
          this.flags = 0,
          this.size = 4 + 8 * this.chunk_offsets.length,
          this.writeHeader(e),
          e.writeUint32(this.chunk_offsets.length),
          r = 0;
        r < this.chunk_offsets.length;
        r++
      )
        e.writeUint64(this.chunk_offsets[r]);
    }),
    (o.cslgBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 4 * 5),
        this.writeHeader(e),
        e.writeInt32(this.compositionToDTSShift),
        e.writeInt32(this.leastDecodeToDisplayDelta),
        e.writeInt32(this.greatestDecodeToDisplayDelta),
        e.writeInt32(this.compositionStartTime),
        e.writeInt32(this.compositionEndTime);
    }),
    (o.cttsBox.prototype.write = function (e) {
      var r;
      for (
        this.version = 0,
          this.flags = 0,
          this.size = 4 + 8 * this.sample_counts.length,
          this.writeHeader(e),
          e.writeUint32(this.sample_counts.length),
          r = 0;
        r < this.sample_counts.length;
        r++
      )
        e.writeUint32(this.sample_counts[r]),
          this.version === 1
            ? e.writeInt32(this.sample_offsets[r])
            : e.writeUint32(this.sample_offsets[r]);
    }),
    (o.drefBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 4),
        this.writeHeader(e),
        e.writeUint32(this.entries.length);
      for (var r = 0; r < this.entries.length; r++)
        this.entries[r].write(e), (this.size += this.entries[r].size);
      f.debug(
        "BoxWriter",
        "Adjusting box " + this.type + " with new size " + this.size
      ),
        e.adjustUint32(this.sizePosition, this.size);
    }),
    (o.elngBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = this.extended_language.length),
        this.writeHeader(e),
        e.writeString(this.extended_language);
    }),
    (o.elstBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 4 + 12 * this.entries.length),
        this.writeHeader(e),
        e.writeUint32(this.entries.length);
      for (var r = 0; r < this.entries.length; r++) {
        var n = this.entries[r];
        e.writeUint32(n.segment_duration),
          e.writeInt32(n.media_time),
          e.writeInt16(n.media_rate_integer),
          e.writeInt16(n.media_rate_fraction);
      }
    }),
    (o.emsgBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size =
          4 * 4 +
          this.message_data.length +
          (this.scheme_id_uri.length + 1) +
          (this.value.length + 1)),
        this.writeHeader(e),
        e.writeCString(this.scheme_id_uri),
        e.writeCString(this.value),
        e.writeUint32(this.timescale),
        e.writeUint32(this.presentation_time_delta),
        e.writeUint32(this.event_duration),
        e.writeUint32(this.id),
        e.writeUint8Array(this.message_data);
    }),
    (o.ftypBox.prototype.write = function (e) {
      (this.size = 8 + 4 * this.compatible_brands.length),
        this.writeHeader(e),
        e.writeString(this.major_brand, null, 4),
        e.writeUint32(this.minor_version);
      for (var r = 0; r < this.compatible_brands.length; r++)
        e.writeString(this.compatible_brands[r], null, 4);
    }),
    (o.hdlrBox.prototype.write = function (e) {
      (this.size = 5 * 4 + this.name.length + 1),
        (this.version = 0),
        (this.flags = 0),
        this.writeHeader(e),
        e.writeUint32(0),
        e.writeString(this.handler, null, 4),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeCString(this.name);
    }),
    (o.kindBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = this.schemeURI.length + 1 + (this.value.length + 1)),
        this.writeHeader(e),
        e.writeCString(this.schemeURI),
        e.writeCString(this.value);
    }),
    (o.mdhdBox.prototype.write = function (e) {
      (this.size = 4 * 4 + 2 * 2),
        (this.flags = 0),
        (this.version = 0),
        this.writeHeader(e),
        e.writeUint32(this.creation_time),
        e.writeUint32(this.modification_time),
        e.writeUint32(this.timescale),
        e.writeUint32(this.duration),
        e.writeUint16(this.language),
        e.writeUint16(0);
    }),
    (o.mehdBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 4),
        this.writeHeader(e),
        e.writeUint32(this.fragment_duration);
    }),
    (o.mfhdBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 4),
        this.writeHeader(e),
        e.writeUint32(this.sequence_number);
    }),
    (o.mvhdBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 23 * 4 + 2 * 2),
        this.writeHeader(e),
        e.writeUint32(this.creation_time),
        e.writeUint32(this.modification_time),
        e.writeUint32(this.timescale),
        e.writeUint32(this.duration),
        e.writeUint32(this.rate),
        e.writeUint16(this.volume << 8),
        e.writeUint16(0),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint32Array(this.matrix),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint32(this.next_track_id);
    }),
    (o.SampleEntry.prototype.writeHeader = function (e) {
      (this.size = 8),
        o.Box.prototype.writeHeader.call(this, e),
        e.writeUint8(0),
        e.writeUint8(0),
        e.writeUint8(0),
        e.writeUint8(0),
        e.writeUint8(0),
        e.writeUint8(0),
        e.writeUint16(this.data_reference_index);
    }),
    (o.SampleEntry.prototype.writeFooter = function (e) {
      for (var r = 0; r < this.boxes.length; r++)
        this.boxes[r].write(e), (this.size += this.boxes[r].size);
      f.debug(
        "BoxWriter",
        "Adjusting box " + this.type + " with new size " + this.size
      ),
        e.adjustUint32(this.sizePosition, this.size);
    }),
    (o.SampleEntry.prototype.write = function (e) {
      this.writeHeader(e),
        e.writeUint8Array(this.data),
        (this.size += this.data.length),
        f.debug(
          "BoxWriter",
          "Adjusting box " + this.type + " with new size " + this.size
        ),
        e.adjustUint32(this.sizePosition, this.size);
    }),
    (o.VisualSampleEntry.prototype.write = function (e) {
      this.writeHeader(e),
        (this.size += 2 * 7 + 6 * 4 + 32),
        e.writeUint16(0),
        e.writeUint16(0),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint16(this.width),
        e.writeUint16(this.height),
        e.writeUint32(this.horizresolution),
        e.writeUint32(this.vertresolution),
        e.writeUint32(0),
        e.writeUint16(this.frame_count),
        e.writeUint8(Math.min(31, this.compressorname.length)),
        e.writeString(this.compressorname, null, 31),
        e.writeUint16(this.depth),
        e.writeInt16(-1),
        this.writeFooter(e);
    }),
    (o.AudioSampleEntry.prototype.write = function (e) {
      this.writeHeader(e),
        (this.size += 2 * 4 + 3 * 4),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeUint16(this.channel_count),
        e.writeUint16(this.samplesize),
        e.writeUint16(0),
        e.writeUint16(0),
        e.writeUint32(this.samplerate << 16),
        this.writeFooter(e);
    }),
    (o.stppSampleEntry.prototype.write = function (e) {
      this.writeHeader(e),
        (this.size +=
          this.namespace.length +
          1 +
          this.schema_location.length +
          1 +
          this.auxiliary_mime_types.length +
          1),
        e.writeCString(this.namespace),
        e.writeCString(this.schema_location),
        e.writeCString(this.auxiliary_mime_types),
        this.writeFooter(e);
    }),
    (o.SampleGroupEntry.prototype.write = function (e) {
      e.writeUint8Array(this.data);
    }),
    (o.sbgpBox.prototype.write = function (e) {
      (this.version = 1),
        (this.flags = 0),
        (this.size = 12 + 8 * this.entries.length),
        this.writeHeader(e),
        e.writeString(this.grouping_type, null, 4),
        e.writeUint32(this.grouping_type_parameter),
        e.writeUint32(this.entries.length);
      for (var r = 0; r < this.entries.length; r++) {
        var n = this.entries[r];
        e.writeInt32(n.sample_count), e.writeInt32(n.group_description_index);
      }
    }),
    (o.sgpdBox.prototype.write = function (e) {
      var r, n;
      for (this.flags = 0, this.size = 12, r = 0; r < this.entries.length; r++)
        (n = this.entries[r]),
          this.version === 1 &&
            (this.default_length === 0 && (this.size += 4),
            (this.size += n.data.length));
      for (
        this.writeHeader(e),
          e.writeString(this.grouping_type, null, 4),
          this.version === 1 && e.writeUint32(this.default_length),
          this.version >= 2 &&
            e.writeUint32(this.default_sample_description_index),
          e.writeUint32(this.entries.length),
          r = 0;
        r < this.entries.length;
        r++
      )
        (n = this.entries[r]),
          this.version === 1 &&
            this.default_length === 0 &&
            e.writeUint32(n.description_length),
          n.write(e);
    }),
    (o.sidxBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 4 * 4 + 2 + 2 + 12 * this.references.length),
        this.writeHeader(e),
        e.writeUint32(this.reference_ID),
        e.writeUint32(this.timescale),
        e.writeUint32(this.earliest_presentation_time),
        e.writeUint32(this.first_offset),
        e.writeUint16(0),
        e.writeUint16(this.references.length);
      for (var r = 0; r < this.references.length; r++) {
        var n = this.references[r];
        e.writeUint32((n.reference_type << 31) | n.referenced_size),
          e.writeUint32(n.subsegment_duration),
          e.writeUint32(
            (n.starts_with_SAP << 31) | (n.SAP_type << 28) | n.SAP_delta_time
          );
      }
    }),
    (o.smhdBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 1),
        (this.size = 4),
        this.writeHeader(e),
        e.writeUint16(this.balance),
        e.writeUint16(0);
    }),
    (o.stcoBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 4 + 4 * this.chunk_offsets.length),
        this.writeHeader(e),
        e.writeUint32(this.chunk_offsets.length),
        e.writeUint32Array(this.chunk_offsets);
    }),
    (o.stscBox.prototype.write = function (e) {
      var r;
      for (
        this.version = 0,
          this.flags = 0,
          this.size = 4 + 12 * this.first_chunk.length,
          this.writeHeader(e),
          e.writeUint32(this.first_chunk.length),
          r = 0;
        r < this.first_chunk.length;
        r++
      )
        e.writeUint32(this.first_chunk[r]),
          e.writeUint32(this.samples_per_chunk[r]),
          e.writeUint32(this.sample_description_index[r]);
    }),
    (o.stsdBox.prototype.write = function (e) {
      var r;
      for (
        this.version = 0,
          this.flags = 0,
          this.size = 0,
          this.writeHeader(e),
          e.writeUint32(this.entries.length),
          this.size += 4,
          r = 0;
        r < this.entries.length;
        r++
      )
        this.entries[r].write(e), (this.size += this.entries[r].size);
      f.debug(
        "BoxWriter",
        "Adjusting box " + this.type + " with new size " + this.size
      ),
        e.adjustUint32(this.sizePosition, this.size);
    }),
    (o.stshBox.prototype.write = function (e) {
      var r;
      for (
        this.version = 0,
          this.flags = 0,
          this.size = 4 + 8 * this.shadowed_sample_numbers.length,
          this.writeHeader(e),
          e.writeUint32(this.shadowed_sample_numbers.length),
          r = 0;
        r < this.shadowed_sample_numbers.length;
        r++
      )
        e.writeUint32(this.shadowed_sample_numbers[r]),
          e.writeUint32(this.sync_sample_numbers[r]);
    }),
    (o.stssBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 4 + 4 * this.sample_numbers.length),
        this.writeHeader(e),
        e.writeUint32(this.sample_numbers.length),
        e.writeUint32Array(this.sample_numbers);
    }),
    (o.stszBox.prototype.write = function (e) {
      var r,
        n = !0;
      if (((this.version = 0), (this.flags = 0), this.sample_sizes.length > 0))
        for (r = 0; r + 1 < this.sample_sizes.length; )
          if (this.sample_sizes[r + 1] !== this.sample_sizes[0]) {
            n = !1;
            break;
          } else r++;
      else n = !1;
      (this.size = 8),
        n || (this.size += 4 * this.sample_sizes.length),
        this.writeHeader(e),
        n ? e.writeUint32(this.sample_sizes[0]) : e.writeUint32(0),
        e.writeUint32(this.sample_sizes.length),
        n || e.writeUint32Array(this.sample_sizes);
    }),
    (o.sttsBox.prototype.write = function (e) {
      var r;
      for (
        this.version = 0,
          this.flags = 0,
          this.size = 4 + 8 * this.sample_counts.length,
          this.writeHeader(e),
          e.writeUint32(this.sample_counts.length),
          r = 0;
        r < this.sample_counts.length;
        r++
      )
        e.writeUint32(this.sample_counts[r]),
          e.writeUint32(this.sample_deltas[r]);
    }),
    (o.tfdtBox.prototype.write = function (e) {
      var r = Math.pow(2, 32) - 1;
      (this.version = this.baseMediaDecodeTime > r ? 1 : 0),
        (this.flags = 0),
        (this.size = 4),
        this.version === 1 && (this.size += 4),
        this.writeHeader(e),
        this.version === 1
          ? e.writeUint64(this.baseMediaDecodeTime)
          : e.writeUint32(this.baseMediaDecodeTime);
    }),
    (o.tfhdBox.prototype.write = function (e) {
      (this.version = 0),
        (this.size = 4),
        this.flags & o.TFHD_FLAG_BASE_DATA_OFFSET && (this.size += 8),
        this.flags & o.TFHD_FLAG_SAMPLE_DESC && (this.size += 4),
        this.flags & o.TFHD_FLAG_SAMPLE_DUR && (this.size += 4),
        this.flags & o.TFHD_FLAG_SAMPLE_SIZE && (this.size += 4),
        this.flags & o.TFHD_FLAG_SAMPLE_FLAGS && (this.size += 4),
        this.writeHeader(e),
        e.writeUint32(this.track_id),
        this.flags & o.TFHD_FLAG_BASE_DATA_OFFSET &&
          e.writeUint64(this.base_data_offset),
        this.flags & o.TFHD_FLAG_SAMPLE_DESC &&
          e.writeUint32(this.default_sample_description_index),
        this.flags & o.TFHD_FLAG_SAMPLE_DUR &&
          e.writeUint32(this.default_sample_duration),
        this.flags & o.TFHD_FLAG_SAMPLE_SIZE &&
          e.writeUint32(this.default_sample_size),
        this.flags & o.TFHD_FLAG_SAMPLE_FLAGS &&
          e.writeUint32(this.default_sample_flags);
    }),
    (o.tkhdBox.prototype.write = function (e) {
      (this.version = 0),
        (this.size = 4 * 18 + 2 * 4),
        this.writeHeader(e),
        e.writeUint32(this.creation_time),
        e.writeUint32(this.modification_time),
        e.writeUint32(this.track_id),
        e.writeUint32(0),
        e.writeUint32(this.duration),
        e.writeUint32(0),
        e.writeUint32(0),
        e.writeInt16(this.layer),
        e.writeInt16(this.alternate_group),
        e.writeInt16(this.volume << 8),
        e.writeUint16(0),
        e.writeInt32Array(this.matrix),
        e.writeUint32(this.width),
        e.writeUint32(this.height);
    }),
    (o.trexBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size = 4 * 5),
        this.writeHeader(e),
        e.writeUint32(this.track_id),
        e.writeUint32(this.default_sample_description_index),
        e.writeUint32(this.default_sample_duration),
        e.writeUint32(this.default_sample_size),
        e.writeUint32(this.default_sample_flags);
    }),
    (o.trunBox.prototype.write = function (e) {
      (this.version = 0),
        (this.size = 4),
        this.flags & o.TRUN_FLAGS_DATA_OFFSET && (this.size += 4),
        this.flags & o.TRUN_FLAGS_FIRST_FLAG && (this.size += 4),
        this.flags & o.TRUN_FLAGS_DURATION &&
          (this.size += 4 * this.sample_duration.length),
        this.flags & o.TRUN_FLAGS_SIZE &&
          (this.size += 4 * this.sample_size.length),
        this.flags & o.TRUN_FLAGS_FLAGS &&
          (this.size += 4 * this.sample_flags.length),
        this.flags & o.TRUN_FLAGS_CTS_OFFSET &&
          (this.size += 4 * this.sample_composition_time_offset.length),
        this.writeHeader(e),
        e.writeUint32(this.sample_count),
        this.flags & o.TRUN_FLAGS_DATA_OFFSET &&
          ((this.data_offset_position = e.getPosition()),
          e.writeInt32(this.data_offset)),
        this.flags & o.TRUN_FLAGS_FIRST_FLAG &&
          e.writeUint32(this.first_sample_flags);
      for (var r = 0; r < this.sample_count; r++)
        this.flags & o.TRUN_FLAGS_DURATION &&
          e.writeUint32(this.sample_duration[r]),
          this.flags & o.TRUN_FLAGS_SIZE && e.writeUint32(this.sample_size[r]),
          this.flags & o.TRUN_FLAGS_FLAGS &&
            e.writeUint32(this.sample_flags[r]),
          this.flags & o.TRUN_FLAGS_CTS_OFFSET &&
            (this.version === 0
              ? e.writeUint32(this.sample_composition_time_offset[r])
              : e.writeInt32(this.sample_composition_time_offset[r]));
    }),
    (o["url Box"].prototype.write = function (e) {
      (this.version = 0),
        this.location
          ? ((this.flags = 0), (this.size = this.location.length + 1))
          : ((this.flags = 1), (this.size = 0)),
        this.writeHeader(e),
        this.location && e.writeCString(this.location);
    }),
    (o["urn Box"].prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 0),
        (this.size =
          this.name.length +
          1 +
          (this.location ? this.location.length + 1 : 0)),
        this.writeHeader(e),
        e.writeCString(this.name),
        this.location && e.writeCString(this.location);
    }),
    (o.vmhdBox.prototype.write = function (e) {
      (this.version = 0),
        (this.flags = 1),
        (this.size = 8),
        this.writeHeader(e),
        e.writeUint16(this.graphicsmode),
        e.writeUint16Array(this.opcolor);
    }),
    (o.cttsBox.prototype.unpack = function (e) {
      var r, n, a;
      for (a = 0, r = 0; r < this.sample_counts.length; r++)
        for (n = 0; n < this.sample_counts[r]; n++)
          (e[a].pts = e[a].dts + this.sample_offsets[r]), a++;
    }),
    (o.sttsBox.prototype.unpack = function (e) {
      var r, n, a;
      for (a = 0, r = 0; r < this.sample_counts.length; r++)
        for (n = 0; n < this.sample_counts[r]; n++)
          a === 0
            ? (e[a].dts = 0)
            : (e[a].dts = e[a - 1].dts + this.sample_deltas[r]),
            a++;
    }),
    (o.stcoBox.prototype.unpack = function (e) {
      var r;
      for (r = 0; r < this.chunk_offsets.length; r++)
        e[r].offset = this.chunk_offsets[r];
    }),
    (o.stscBox.prototype.unpack = function (e) {
      var r, n, a, l, m;
      for (l = 0, m = 0, r = 0; r < this.first_chunk.length; r++)
        for (
          n = 0;
          n <
          (r + 1 < this.first_chunk.length ? this.first_chunk[r + 1] : 1 / 0);
          n++
        )
          for (m++, a = 0; a < this.samples_per_chunk[r]; a++) {
            if (e[l])
              (e[l].description_index = this.sample_description_index[r]),
                (e[l].chunk_index = m);
            else return;
            l++;
          }
    }),
    (o.stszBox.prototype.unpack = function (e) {
      var r;
      for (r = 0; r < this.sample_sizes.length; r++)
        e[r].size = this.sample_sizes[r];
    }),
    (o.DIFF_BOXES_PROP_NAMES = [
      "boxes",
      "entries",
      "references",
      "subsamples",
      "items",
      "item_infos",
      "extents",
      "associations",
      "subsegments",
      "ranges",
      "seekLists",
      "seekPoints",
      "esd",
      "levels",
    ]),
    (o.DIFF_PRIMITIVE_ARRAY_PROP_NAMES = [
      "compatible_brands",
      "matrix",
      "opcolor",
      "sample_counts",
      "sample_counts",
      "sample_deltas",
      "first_chunk",
      "samples_per_chunk",
      "sample_sizes",
      "chunk_offsets",
      "sample_offsets",
      "sample_description_index",
      "sample_duration",
    ]),
    (o.boxEqualFields = function (e, r) {
      if (e && !r) return !1;
      var n;
      for (n in e)
        if (!(o.DIFF_BOXES_PROP_NAMES.indexOf(n) > -1)) {
          if (e[n] instanceof o.Box || r[n] instanceof o.Box) continue;
          if (typeof e[n] > "u" || typeof r[n] > "u") continue;
          if (typeof e[n] == "function" || typeof r[n] == "function") continue;
          if (
            (e.subBoxNames && e.subBoxNames.indexOf(n.slice(0, 4)) > -1) ||
            (r.subBoxNames && r.subBoxNames.indexOf(n.slice(0, 4)) > -1)
          )
            continue;
          if (
            n === "data" ||
            n === "start" ||
            n === "size" ||
            n === "creation_time" ||
            n === "modification_time"
          )
            continue;
          if (o.DIFF_PRIMITIVE_ARRAY_PROP_NAMES.indexOf(n) > -1) continue;
          if (e[n] !== r[n]) return !1;
        }
      return !0;
    }),
    (o.boxEqual = function (e, r) {
      if (!o.boxEqualFields(e, r)) return !1;
      for (var n = 0; n < o.DIFF_BOXES_PROP_NAMES.length; n++) {
        var a = o.DIFF_BOXES_PROP_NAMES[n];
        if (e[a] && r[a] && !o.boxEqual(e[a], r[a])) return !1;
      }
      return !0;
    });
  var M = function () {};
  M.prototype.parseSample = function (e) {
    var r = {},
      n;
    r.resources = [];
    var a = new h(e.data.buffer);
    if (!e.subsamples || e.subsamples.length === 0)
      r.documentString = a.readString(e.data.length);
    else if (
      ((r.documentString = a.readString(e.subsamples[0].size)),
      e.subsamples.length > 1)
    )
      for (n = 1; n < e.subsamples.length; n++)
        r.resources[n] = a.readUint8Array(e.subsamples[n].size);
    return (
      typeof DOMParser < "u" &&
        (r.document = new DOMParser().parseFromString(
          r.documentString,
          "application/xml"
        )),
      r
    );
  };
  var B = function () {};
  (B.prototype.parseSample = function (e) {
    var r,
      n = new h(e.data.buffer);
    return (r = n.readString(e.data.length)), r;
  }),
    (B.prototype.parseConfig = function (e) {
      var r,
        n = new h(e.buffer);
      return n.readUint32(), (r = n.readCString()), r;
    }),
    (g.XMLSubtitlein4Parser = M),
    (g.Textin4Parser = B);
  var w = function (e) {
    (this.stream = e || new x()),
      (this.boxes = []),
      (this.mdats = []),
      (this.moofs = []),
      (this.isProgressive = !1),
      (this.moovStartFound = !1),
      (this.onMoovStart = null),
      (this.moovStartSent = !1),
      (this.onReady = null),
      (this.readySent = !1),
      (this.onSegment = null),
      (this.onSamples = null),
      (this.onError = null),
      (this.sampleListBuilt = !1),
      (this.fragmentedTracks = []),
      (this.extractedTracks = []),
      (this.isFragmentationInitialized = !1),
      (this.sampleProcessingStarted = !1),
      (this.nextMoofNumber = 0),
      (this.itemListBuilt = !1),
      (this.onSidx = null),
      (this.sidxSent = !1);
  };
  (w.prototype.setSegmentOptions = function (e, r, n) {
    var a = this.getTrackById(e);
    if (a) {
      var l = {};
      this.fragmentedTracks.push(l),
        (l.id = e),
        (l.user = r),
        (l.trak = a),
        (a.nextSample = 0),
        (l.segmentStream = null),
        (l.nb_samples = 1e3),
        (l.rapAlignement = !0),
        n &&
          (n.nbSamples && (l.nb_samples = n.nbSamples),
          n.rapAlignement && (l.rapAlignement = n.rapAlignement));
    }
  }),
    (w.prototype.unsetSegmentOptions = function (e) {
      for (var r = -1, n = 0; n < this.fragmentedTracks.length; n++) {
        var a = this.fragmentedTracks[n];
        a.id == e && (r = n);
      }
      r > -1 && this.fragmentedTracks.splice(r, 1);
    }),
    (w.prototype.setExtractionOptions = function (e, r, n) {
      var a = this.getTrackById(e);
      if (a) {
        var l = {};
        this.extractedTracks.push(l),
          (l.id = e),
          (l.user = r),
          (l.trak = a),
          (a.nextSample = 0),
          (l.nb_samples = 1e3),
          (l.samples = []),
          n && n.nbSamples && (l.nb_samples = n.nbSamples);
      }
    }),
    (w.prototype.unsetExtractionOptions = function (e) {
      for (var r = -1, n = 0; n < this.extractedTracks.length; n++) {
        var a = this.extractedTracks[n];
        a.id == e && (r = n);
      }
      r > -1 && this.extractedTracks.splice(r, 1);
    }),
    (w.prototype.parse = function () {
      var e,
        r,
        n = !1;
      if (!(this.restoreParsePosition && !this.restoreParsePosition()))
        for (;;)
          if (this.hasIncompleteMdat && this.hasIncompleteMdat()) {
            if (this.processIncompleteMdat()) continue;
            return;
          } else if (
            (this.saveParsePosition && this.saveParsePosition(),
            (e = o.parseOneBox(this.stream, n)),
            e.code === o.ERR_NOT_ENOUGH_DATA)
          )
            if (this.processIncompleteBox) {
              if (this.processIncompleteBox(e)) continue;
              return;
            } else return;
          else {
            var a;
            switch (
              ((r = e.box),
              (a = r.type !== "uuid" ? r.type : r.uuid),
              this.boxes.push(r),
              a)
            ) {
              case "mdat":
                this.mdats.push(r);
                break;
              case "moof":
                this.moofs.push(r);
                break;
              case "moov":
                (this.moovStartFound = !0),
                  this.mdats.length === 0 && (this.isProgressive = !0);
              default:
                this[a] !== void 0 &&
                  f.warn(
                    "ISOFile",
                    "Duplicate Box of type: " +
                      a +
                      ", overriding previous occurrence"
                  ),
                  (this[a] = r);
                break;
            }
            this.updateUsedBytes && this.updateUsedBytes(r, e);
          }
    }),
    (w.prototype.checkBuffer = function (e) {
      if (e == null) throw "Buffer must be defined and non empty";
      if (e.fileStart === void 0) throw "Buffer must have a fileStart property";
      return e.byteLength === 0
        ? (f.warn(
            "ISOFile",
            "Ignoring empty buffer (fileStart: " + e.fileStart + ")"
          ),
          this.stream.logBufferLevel(),
          !1)
        : (f.info(
            "ISOFile",
            "Processing buffer (fileStart: " + e.fileStart + ")"
          ),
          (e.usedBytes = 0),
          this.stream.insertBuffer(e),
          this.stream.logBufferLevel(),
          this.stream.initialized()
            ? !0
            : (f.warn("ISOFile", "Not ready to start parsing"), !1));
    }),
    (w.prototype.appendBuffer = function (e, r) {
      var n;
      if (this.checkBuffer(e))
        return (
          this.parse(),
          this.moovStartFound &&
            !this.moovStartSent &&
            ((this.moovStartSent = !0), this.onMoovStart && this.onMoovStart()),
          this.moov
            ? (this.sampleListBuilt ||
                (this.buildSampleLists(), (this.sampleListBuilt = !0)),
              this.updateSampleLists(),
              this.onReady &&
                !this.readySent &&
                ((this.readySent = !0), this.onReady(this.getInfo())),
              this.processSamples(r),
              this.nextSeekPosition
                ? ((n = this.nextSeekPosition),
                  (this.nextSeekPosition = void 0))
                : (n = this.nextParsePosition),
              this.stream.getEndFilePositionAfter &&
                (n = this.stream.getEndFilePositionAfter(n)))
            : this.nextParsePosition
            ? (n = this.nextParsePosition)
            : (n = 0),
          this.sidx &&
            this.onSidx &&
            !this.sidxSent &&
            (this.onSidx(this.sidx), (this.sidxSent = !0)),
          this.meta &&
            (this.flattenItemInfo &&
              !this.itemListBuilt &&
              (this.flattenItemInfo(), (this.itemListBuilt = !0)),
            this.processItems && this.processItems(this.onItem)),
          this.stream.cleanBuffers &&
            (f.info(
              "ISOFile",
              "Done processing buffer (fileStart: " +
                e.fileStart +
                ") - next buffer to fetch should have a fileStart position of " +
                n
            ),
            this.stream.logBufferLevel(),
            this.stream.cleanBuffers(),
            this.stream.logBufferLevel(!0),
            f.info(
              "ISOFile",
              "Sample data size in memory: " + this.getAllocatedSampleDataSize()
            )),
          n
        );
    }),
    (w.prototype.getInfo = function () {
      var e,
        r,
        n = {},
        a,
        l,
        m,
        p,
        S = new Date("1904-01-01T00:00:00Z").getTime();
      if (this.moov)
        for (
          n.hasMoov = !0,
            n.duration = this.moov.mvhd.duration,
            n.timescale = this.moov.mvhd.timescale,
            n.isFragmented = this.moov.mvex != null,
            n.isFragmented &&
              this.moov.mvex.mehd &&
              (n.fragment_duration = this.moov.mvex.mehd.fragment_duration),
            n.isProgressive = this.isProgressive,
            n.hasIOD = this.moov.iods != null,
            n.brands = [],
            n.brands.push(this.ftyp.major_brand),
            n.brands = n.brands.concat(this.ftyp.compatible_brands),
            n.created = new Date(S + this.moov.mvhd.creation_time * 1e3),
            n.modified = new Date(S + this.moov.mvhd.modification_time * 1e3),
            n.tracks = [],
            n.audioTracks = [],
            n.videoTracks = [],
            n.subtitleTracks = [],
            n.metadataTracks = [],
            n.hintTracks = [],
            n.otherTracks = [],
            e = 0;
          e < this.moov.traks.length;
          e++
        ) {
          if (
            ((a = this.moov.traks[e]),
            (p = a.mdia.minf.stbl.stsd.entries[0]),
            (l = {}),
            n.tracks.push(l),
            (l.id = a.tkhd.track_id),
            (l.name = a.mdia.hdlr.name),
            (l.references = []),
            a.tref)
          )
            for (r = 0; r < a.tref.boxes.length; r++)
              (m = {}),
                l.references.push(m),
                (m.type = a.tref.boxes[r].type),
                (m.track_ids = a.tref.boxes[r].track_ids);
          a.edts && (l.edits = a.edts.elst.entries),
            (l.created = new Date(S + a.tkhd.creation_time * 1e3)),
            (l.modified = new Date(S + a.tkhd.modification_time * 1e3)),
            (l.movie_duration = a.tkhd.duration),
            (l.movie_timescale = n.timescale),
            (l.layer = a.tkhd.layer),
            (l.alternate_group = a.tkhd.alternate_group),
            (l.volume = a.tkhd.volume),
            (l.matrix = a.tkhd.matrix),
            (l.track_width = a.tkhd.width / 65536),
            (l.track_height = a.tkhd.height / 65536),
            (l.timescale = a.mdia.mdhd.timescale),
            (l.cts_shift = a.mdia.minf.stbl.cslg),
            (l.duration = a.mdia.mdhd.duration),
            (l.samples_duration = a.samples_duration),
            (l.codec = p.getCodec()),
            (l.kind =
              a.udta && a.udta.kinds.length
                ? a.udta.kinds[0]
                : {
                    schemeURI: "",
                    value: "",
                  }),
            (l.language = a.mdia.elng
              ? a.mdia.elng.extended_language
              : a.mdia.mdhd.languageString),
            (l.nb_samples = a.samples.length),
            (l.size = a.samples_size),
            (l.bitrate = (l.size * 8 * l.timescale) / l.samples_duration),
            p.isAudio()
              ? ((l.type = "audio"),
                n.audioTracks.push(l),
                (l.audio = {}),
                (l.audio.sample_rate = p.getSampleRate()),
                (l.audio.channel_count = p.getChannelCount()),
                (l.audio.sample_size = p.getSampleSize()))
              : p.isVideo()
              ? ((l.type = "video"),
                n.videoTracks.push(l),
                (l.video = {}),
                (l.video.width = p.getWidth()),
                (l.video.height = p.getHeight()))
              : p.isSubtitle()
              ? ((l.type = "subtitles"), n.subtitleTracks.push(l))
              : p.isHint()
              ? ((l.type = "metadata"), n.hintTracks.push(l))
              : p.isMetadata()
              ? ((l.type = "metadata"), n.metadataTracks.push(l))
              : ((l.type = "metadata"), n.otherTracks.push(l));
        }
      else n.hasMoov = !1;
      if (((n.mime = ""), n.hasMoov && n.tracks)) {
        for (
          n.videoTracks && n.videoTracks.length > 0
            ? (n.mime += 'video/mp4; codecs="')
            : n.audioTracks && n.audioTracks.length > 0
            ? (n.mime += 'audio/mp4; codecs="')
            : (n.mime += 'application/mp4; codecs="'),
            e = 0;
          e < n.tracks.length;
          e++
        )
          e !== 0 && (n.mime += ","), (n.mime += n.tracks[e].codec);
        (n.mime += '"; profiles="'),
          (n.mime += this.ftyp.compatible_brands.join()),
          (n.mime += '"');
      }
      return n;
    }),
    (w.prototype.processSamples = function (e) {
      var r, n;
      if (this.sampleProcessingStarted) {
        if (this.isFragmentationInitialized && this.onSegment !== null)
          for (r = 0; r < this.fragmentedTracks.length; r++) {
            var a = this.fragmentedTracks[r];
            for (
              n = a.trak;
              n.nextSample < n.samples.length && this.sampleProcessingStarted;

            ) {
              f.debug(
                "ISOFile",
                "Creating media fragment on track #" +
                  a.id +
                  " for sample " +
                  n.nextSample
              );
              var l = this.createFragment(a.id, n.nextSample, a.segmentStream);
              if (l) (a.segmentStream = l), n.nextSample++;
              else break;
              if (
                (n.nextSample % a.nb_samples === 0 ||
                  e ||
                  n.nextSample >= n.samples.length) &&
                (f.info(
                  "ISOFile",
                  "Sending fragmented data on track #" +
                    a.id +
                    " for samples [" +
                    Math.max(0, n.nextSample - a.nb_samples) +
                    "," +
                    (n.nextSample - 1) +
                    "]"
                ),
                f.info(
                  "ISOFile",
                  "Sample data size in memory: " +
                    this.getAllocatedSampleDataSize()
                ),
                this.onSegment &&
                  this.onSegment(
                    a.id,
                    a.user,
                    a.segmentStream.buffer,
                    n.nextSample,
                    e || n.nextSample >= n.samples.length
                  ),
                (a.segmentStream = null),
                a !== this.fragmentedTracks[r])
              )
                break;
            }
          }
        if (this.onSamples !== null)
          for (r = 0; r < this.extractedTracks.length; r++) {
            var m = this.extractedTracks[r];
            for (
              n = m.trak;
              n.nextSample < n.samples.length && this.sampleProcessingStarted;

            ) {
              f.debug(
                "ISOFile",
                "Exporting on track #" + m.id + " sample #" + n.nextSample
              );
              var p = this.getSample(n, n.nextSample);
              if (p) n.nextSample++, m.samples.push(p);
              else break;
              if (
                (n.nextSample % m.nb_samples === 0 ||
                  n.nextSample >= n.samples.length) &&
                (f.debug(
                  "ISOFile",
                  "Sending samples on track #" +
                    m.id +
                    " for sample " +
                    n.nextSample
                ),
                this.onSamples && this.onSamples(m.id, m.user, m.samples),
                (m.samples = []),
                m !== this.extractedTracks[r])
              )
                break;
            }
          }
      }
    }),
    (w.prototype.getBox = function (e) {
      var r = this.getBoxes(e, !0);
      return r.length ? r[0] : null;
    }),
    (w.prototype.getBoxes = function (e, r) {
      var n = [];
      return w._sweep.call(this, e, n, r), n;
    }),
    (w._sweep = function (e, r, n) {
      this.type && this.type == e && r.push(this);
      for (var a in this.boxes) {
        if (r.length && n) return;
        w._sweep.call(this.boxes[a], e, r, n);
      }
    }),
    (w.prototype.getTrackSamplesInfo = function (e) {
      var r = this.getTrackById(e);
      if (r) return r.samples;
    }),
    (w.prototype.getTrackSample = function (e, r) {
      var n = this.getTrackById(e),
        a = this.getSample(n, r);
      return a;
    }),
    (w.prototype.releaseUsedSamples = function (e, r) {
      var n = 0,
        a = this.getTrackById(e);
      a.lastValidSample || (a.lastValidSample = 0);
      for (var l = a.lastValidSample; l < r; l++) n += this.releaseSample(a, l);
      f.info(
        "ISOFile",
        "Track #" +
          e +
          " released samples up to " +
          r +
          " (released size: " +
          n +
          ", remaining: " +
          this.samplesDataSize +
          ")"
      ),
        (a.lastValidSample = r);
    }),
    (w.prototype.start = function () {
      (this.sampleProcessingStarted = !0), this.processSamples(!1);
    }),
    (w.prototype.stop = function () {
      this.sampleProcessingStarted = !1;
    }),
    (w.prototype.flush = function () {
      f.info("ISOFile", "Flushing remaining samples"),
        this.updateSampleLists(),
        this.processSamples(!0),
        this.stream.cleanBuffers(),
        this.stream.logBufferLevel(!0);
    }),
    (w.prototype.seekTrack = function (e, r, n) {
      var a,
        l,
        m = 1 / 0,
        p = 0,
        S = 0,
        R;
      if (n.samples.length === 0)
        return (
          f.info(
            "ISOFile",
            "No sample in track, cannot seek! Using time " +
              f.getDurationString(0, 1) +
              " and offset: 0"
          ),
          {
            offset: 0,
            time: 0,
          }
        );
      for (a = 0; a < n.samples.length; a++) {
        if (((l = n.samples[a]), a === 0)) (S = 0), (R = l.timescale);
        else if (l.cts > e * l.timescale) {
          S = a - 1;
          break;
        }
        r && l.is_sync && (p = a);
      }
      for (
        r && (S = p), e = n.samples[S].cts, n.nextSample = S;
        n.samples[S].alreadyRead === n.samples[S].size && n.samples[S + 1];

      )
        S++;
      return (
        (m = n.samples[S].offset + n.samples[S].alreadyRead),
        f.info(
          "ISOFile",
          "Seeking to " +
            (r ? "RAP" : "") +
            " sample #" +
            n.nextSample +
            " on track " +
            n.tkhd.track_id +
            ", time " +
            f.getDurationString(e, R) +
            " and offset: " +
            m
        ),
        {
          offset: m,
          time: e / R,
        }
      );
    }),
    (w.prototype.seek = function (e, r) {
      var n = this.moov,
        a,
        l,
        m,
        p = {
          offset: 1 / 0,
          time: 1 / 0,
        };
      if (this.moov) {
        for (m = 0; m < n.traks.length; m++)
          (a = n.traks[m]),
            (l = this.seekTrack(e, r, a)),
            l.offset < p.offset && (p.offset = l.offset),
            l.time < p.time && (p.time = l.time);
        return (
          f.info(
            "ISOFile",
            "Seeking at time " +
              f.getDurationString(p.time, 1) +
              " needs a buffer with a fileStart position of " +
              p.offset
          ),
          p.offset === 1 / 0
            ? (p = {
                offset: this.nextParsePosition,
                time: 0,
              })
            : (p.offset = this.stream.getEndFilePositionAfter(p.offset)),
          f.info(
            "ISOFile",
            "Adjusted seek position (after checking data already in buffer): " +
              p.offset
          ),
          p
        );
      } else throw "Cannot seek: moov not received!";
    }),
    (w.prototype.equal = function (e) {
      for (var r = 0; r < this.boxes.length && r < e.boxes.length; ) {
        var n = this.boxes[r],
          a = e.boxes[r];
        if (!o.boxEqual(n, a)) return !1;
        r++;
      }
      return !0;
    }),
    (g.ISOFile = w),
    (w.prototype.lastBoxStartPosition = 0),
    (w.prototype.parsingMdat = null),
    (w.prototype.nextParsePosition = 0),
    (w.prototype.discardMdatData = !1),
    (w.prototype.processIncompleteBox = function (e) {
      var r, n, a;
      return e.type === "mdat"
        ? ((r = new o[e.type + "Box"](e.size)),
          (this.parsingMdat = r),
          this.boxes.push(r),
          this.mdats.push(r),
          (r.start = e.start),
          (r.hdr_size = e.hdr_size),
          this.stream.addUsedBytes(r.hdr_size),
          (this.lastBoxStartPosition = r.start + r.size),
          (a = this.stream.seek(r.start + r.size, !1, this.discardMdatData)),
          a
            ? ((this.parsingMdat = null), !0)
            : (this.moovStartFound
                ? (this.nextParsePosition = this.stream.findEndContiguousBuf())
                : (this.nextParsePosition = r.start + r.size),
              !1))
        : (e.type === "moov" &&
            ((this.moovStartFound = !0),
            this.mdats.length === 0 && (this.isProgressive = !0)),
          (n = this.stream.mergeNextBuffer
            ? this.stream.mergeNextBuffer()
            : !1),
          n
            ? ((this.nextParsePosition = this.stream.getEndPosition()), !0)
            : (e.type
                ? this.moovStartFound
                  ? (this.nextParsePosition = this.stream.getEndPosition())
                  : (this.nextParsePosition =
                      this.stream.getPosition() + e.size)
                : (this.nextParsePosition = this.stream.getEndPosition()),
              !1));
    }),
    (w.prototype.hasIncompleteMdat = function () {
      return this.parsingMdat !== null;
    }),
    (w.prototype.processIncompleteMdat = function () {
      var e, r;
      return (
        (e = this.parsingMdat),
        (r = this.stream.seek(e.start + e.size, !1, this.discardMdatData)),
        r
          ? (f.debug("ISOFile", "Found 'mdat' end in buffered data"),
            (this.parsingMdat = null),
            !0)
          : ((this.nextParsePosition = this.stream.findEndContiguousBuf()), !1)
      );
    }),
    (w.prototype.restoreParsePosition = function () {
      return this.stream.seek(
        this.lastBoxStartPosition,
        !0,
        this.discardMdatData
      );
    }),
    (w.prototype.saveParsePosition = function () {
      this.lastBoxStartPosition = this.stream.getPosition();
    }),
    (w.prototype.updateUsedBytes = function (e, r) {
      this.stream.addUsedBytes &&
        (e.type === "mdat"
          ? (this.stream.addUsedBytes(e.hdr_size),
            this.discardMdatData &&
              this.stream.addUsedBytes(e.size - e.hdr_size))
          : this.stream.addUsedBytes(e.size));
    }),
    (w.prototype.add = o.Box.prototype.add),
    (w.prototype.addBox = o.Box.prototype.addBox),
    (w.prototype.init = function (e) {
      var r = e || {};
      this.add("ftyp")
        .set("major_brand", (r.brands && r.brands[0]) || "iso4")
        .set("minor_version", 0)
        .set("compatible_brands", r.brands || ["iso4"]);
      var n = this.add("moov");
      return (
        n
          .add("mvhd")
          .set("timescale", r.timescale || 600)
          .set("rate", r.rate || 65536)
          .set("creation_time", 0)
          .set("modification_time", 0)
          .set("duration", r.duration || 0)
          .set("volume", r.width ? 0 : 256)
          .set("matrix", [65536, 0, 0, 0, 65536, 0, 0, 0, 1073741824])
          .set("next_track_id", 1),
        n.add("mvex"),
        this
      );
    }),
    (w.prototype.addTrack = function (e) {
      this.moov || this.init(e);
      var r = e || {};
      (r.width = r.width || 320),
        (r.height = r.height || 320),
        (r.id = r.id || this.moov.mvhd.next_track_id),
        (r.type = r.type || "avc1");
      var n = this.moov.add("trak");
      (this.moov.mvhd.next_track_id = r.id + 1),
        n
          .add("tkhd")
          .set(
            "flags",
            o.TKHD_FLAG_ENABLED | o.TKHD_FLAG_IN_MOVIE | o.TKHD_FLAG_IN_PREVIEW
          )
          .set("creation_time", 0)
          .set("modification_time", 0)
          .set("track_id", r.id)
          .set("duration", r.duration || 0)
          .set("layer", r.layer || 0)
          .set("alternate_group", 0)
          .set("volume", 1)
          .set("matrix", [0, 0, 0, 0, 0, 0, 0, 0, 0])
          .set("width", r.width << 16)
          .set("height", r.height << 16);
      var a = n.add("mdia");
      a
        .add("mdhd")
        .set("creation_time", 0)
        .set("modification_time", 0)
        .set("timescale", r.timescale || 1)
        .set("duration", r.media_duration || 0)
        .set("language", r.language || "und"),
        a
          .add("hdlr")
          .set("handler", r.hdlr || "vide")
          .set("name", r.name || "Track created with MP4Box.js"),
        a.add("elng").set("extended_language", r.language || "fr-FR");
      var l = a.add("minf");
      if (o[r.type + "SampleEntry"] !== void 0) {
        var m = new o[r.type + "SampleEntry"]();
        m.data_reference_index = 1;
        var p = "";
        for (var S in o.sampleEntryCodes)
          for (var R = o.sampleEntryCodes[S], O = 0; O < R.length; O++)
            if (R.indexOf(r.type) > -1) {
              p = S;
              break;
            }
        switch (p) {
          case "Visual":
            if (
              (l.add("vmhd").set("graphicsmode", 0).set("opcolor", [0, 0, 0]),
              m
                .set("width", r.width)
                .set("height", r.height)
                .set("horizresolution", 72 << 16)
                .set("vertresolution", 72 << 16)
                .set("frame_count", 1)
                .set("compressorname", r.type + " Compressor")
                .set("depth", 24),
              r.avcDecoderConfigRecord)
            ) {
              var P = new o.avcCBox(),
                y = new h(r.avcDecoderConfigRecord);
              P.parse(y), m.addBox(P);
            }
            break;
          case "Audio":
            l.add("smhd").set("balance", r.balance || 0),
              m
                .set("channel_count", r.channel_count || 2)
                .set("samplesize", r.samplesize || 16)
                .set("samplerate", r.samplerate || 65536);
            break;
          case "Hint":
            l.add("hmhd");
            break;
          case "Subtitle":
            switch ((l.add("sthd"), r.type)) {
              case "stpp":
                m.set("namespace", r.namespace || "nonamespace")
                  .set("schema_location", r.schema_location || "")
                  .set("auxiliary_mime_types", r.auxiliary_mime_types || "");
                break;
            }
            break;
          case "Metadata":
            l.add("nmhd");
            break;
          case "System":
            l.add("nmhd");
            break;
          default:
            l.add("nmhd");
            break;
        }
        r.description && m.addBox(r.description),
          r.description_boxes &&
            r.description_boxes.forEach(function (I) {
              m.addBox(I);
            }),
          l
            .add("dinf")
            .add("dref")
            .addEntry(new o["url Box"]().set("flags", 1));
        var F = l.add("stbl");
        return (
          F.add("stsd").addEntry(m),
          F.add("stts").set("sample_counts", []).set("sample_deltas", []),
          F.add("stsc")
            .set("first_chunk", [])
            .set("samples_per_chunk", [])
            .set("sample_description_index", []),
          F.add("stco").set("chunk_offsets", []),
          F.add("stsz").set("sample_sizes", []),
          this.moov.mvex
            .add("trex")
            .set("track_id", r.id)
            .set(
              "default_sample_description_index",
              r.default_sample_description_index || 1
            )
            .set("default_sample_duration", r.default_sample_duration || 0)
            .set("default_sample_size", r.default_sample_size || 0)
            .set("default_sample_flags", r.default_sample_flags || 0),
          this.buildTrakSampleLists(n),
          r.id
        );
      }
    }),
    (o.Box.prototype.computeSize = function (e) {
      var r = e || new d();
      (r.endianness = d.BIG_ENDIAN), this.write(r);
    }),
    (w.prototype.addSample = function (e, r, n) {
      var a = n || {},
        l = {},
        m = this.getTrackById(e);
      if (m !== null) {
        (l.number = m.samples.length),
          (l.track_id = m.tkhd.track_id),
          (l.timescale = m.mdia.mdhd.timescale),
          (l.description_index = a.sample_description_index
            ? a.sample_description_index - 1
            : 0),
          (l.description = m.mdia.minf.stbl.stsd.entries[l.description_index]),
          (l.data = r),
          (l.size = r.byteLength),
          (l.alreadyRead = l.size),
          (l.duration = a.duration || 1),
          (l.cts = a.cts || 0),
          (l.dts = a.dts || 0),
          (l.is_sync = a.is_sync || !1),
          (l.is_leading = a.is_leading || 0),
          (l.depends_on = a.depends_on || 0),
          (l.is_depended_on = a.is_depended_on || 0),
          (l.has_redundancy = a.has_redundancy || 0),
          (l.degradation_priority = a.degradation_priority || 0),
          (l.offset = 0),
          (l.subsamples = a.subsamples),
          m.samples.push(l),
          (m.samples_size += l.size),
          (m.samples_duration += l.duration),
          m.first_dts || (m.first_dts = a.dts),
          this.processSamples();
        var p = this.createSingleSampleMoof(l);
        return (
          this.addBox(p),
          p.computeSize(),
          (p.trafs[0].truns[0].data_offset = p.size + 8),
          (this.add("mdat").data = new Uint8Array(r)),
          l
        );
      }
    }),
    (w.prototype.createSingleSampleMoof = function (e) {
      var r = 0;
      e.is_sync ? (r = 1 << 25) : (r = 65536);
      var n = new o.moofBox();
      n.add("mfhd").set("sequence_number", this.nextMoofNumber),
        this.nextMoofNumber++;
      var a = n.add("traf"),
        l = this.getTrackById(e.track_id);
      return (
        a
          .add("tfhd")
          .set("track_id", e.track_id)
          .set("flags", o.TFHD_FLAG_DEFAULT_BASE_IS_MOOF),
        a.add("tfdt").set("baseMediaDecodeTime", e.dts - (l.first_dts || 0)),
        a
          .add("trun")
          .set(
            "flags",
            o.TRUN_FLAGS_DATA_OFFSET |
              o.TRUN_FLAGS_DURATION |
              o.TRUN_FLAGS_SIZE |
              o.TRUN_FLAGS_FLAGS |
              o.TRUN_FLAGS_CTS_OFFSET
          )
          .set("data_offset", 0)
          .set("first_sample_flags", 0)
          .set("sample_count", 1)
          .set("sample_duration", [e.duration])
          .set("sample_size", [e.size])
          .set("sample_flags", [r])
          .set("sample_composition_time_offset", [e.cts - e.dts]),
        n
      );
    }),
    (w.prototype.lastMoofIndex = 0),
    (w.prototype.samplesDataSize = 0),
    (w.prototype.resetTables = function () {
      var e, r, n, a, l, m, p, S;
      for (
        this.initial_duration = this.moov.mvhd.duration,
          this.moov.mvhd.duration = 0,
          e = 0;
        e < this.moov.traks.length;
        e++
      ) {
        (r = this.moov.traks[e]),
          (r.tkhd.duration = 0),
          (r.mdia.mdhd.duration = 0),
          (n = r.mdia.minf.stbl.stco || r.mdia.minf.stbl.co64),
          (n.chunk_offsets = []),
          (a = r.mdia.minf.stbl.stsc),
          (a.first_chunk = []),
          (a.samples_per_chunk = []),
          (a.sample_description_index = []),
          (l = r.mdia.minf.stbl.stsz || r.mdia.minf.stbl.stz2),
          (l.sample_sizes = []),
          (m = r.mdia.minf.stbl.stts),
          (m.sample_counts = []),
          (m.sample_deltas = []),
          (p = r.mdia.minf.stbl.ctts),
          p && ((p.sample_counts = []), (p.sample_offsets = [])),
          (S = r.mdia.minf.stbl.stss);
        var R = r.mdia.minf.stbl.boxes.indexOf(S);
        R != -1 && (r.mdia.minf.stbl.boxes[R] = null);
      }
    }),
    (w.initSampleGroups = function (e, r, n, a, l) {
      var m, p, S, R;
      function O(P, y, F) {
        (this.grouping_type = P),
          (this.grouping_type_parameter = y),
          (this.sbgp = F),
          (this.last_sample_in_run = -1),
          (this.entry_index = -1);
      }
      for (
        r && (r.sample_groups_info = []),
          e.sample_groups_info || (e.sample_groups_info = []),
          p = 0;
        p < n.length;
        p++
      ) {
        for (
          R = n[p].grouping_type + "/" + n[p].grouping_type_parameter,
            S = new O(n[p].grouping_type, n[p].grouping_type_parameter, n[p]),
            r && (r.sample_groups_info[R] = S),
            e.sample_groups_info[R] || (e.sample_groups_info[R] = S),
            m = 0;
          m < a.length;
          m++
        )
          a[m].grouping_type === n[p].grouping_type &&
            ((S.description = a[m]), (S.description.used = !0));
        if (l)
          for (m = 0; m < l.length; m++)
            l[m].grouping_type === n[p].grouping_type &&
              ((S.fragment_description = l[m]),
              (S.fragment_description.used = !0),
              (S.is_fragment = !0));
      }
      if (r) {
        if (l)
          for (p = 0; p < l.length; p++)
            !l[p].used &&
              l[p].version >= 2 &&
              ((R = l[p].grouping_type + "/0"),
              (S = new O(l[p].grouping_type, 0)),
              (S.is_fragment = !0),
              r.sample_groups_info[R] || (r.sample_groups_info[R] = S));
      } else
        for (p = 0; p < a.length; p++)
          !a[p].used &&
            a[p].version >= 2 &&
            ((R = a[p].grouping_type + "/0"),
            (S = new O(a[p].grouping_type, 0)),
            e.sample_groups_info[R] || (e.sample_groups_info[R] = S));
    }),
    (w.setSampleGroupProperties = function (e, r, n, a) {
      var l, m;
      r.sample_groups = [];
      for (l in a)
        if (
          ((r.sample_groups[l] = {}),
          (r.sample_groups[l].grouping_type = a[l].grouping_type),
          (r.sample_groups[l].grouping_type_parameter =
            a[l].grouping_type_parameter),
          n >= a[l].last_sample_in_run &&
            (a[l].last_sample_in_run < 0 && (a[l].last_sample_in_run = 0),
            a[l].entry_index++,
            a[l].entry_index <= a[l].sbgp.entries.length - 1 &&
              (a[l].last_sample_in_run +=
                a[l].sbgp.entries[a[l].entry_index].sample_count)),
          a[l].entry_index <= a[l].sbgp.entries.length - 1
            ? (r.sample_groups[l].group_description_index =
                a[l].sbgp.entries[a[l].entry_index].group_description_index)
            : (r.sample_groups[l].group_description_index = -1),
          r.sample_groups[l].group_description_index !== 0)
        ) {
          var p;
          a[l].fragment_description
            ? (p = a[l].fragment_description)
            : (p = a[l].description),
            r.sample_groups[l].group_description_index > 0
              ? (r.sample_groups[l].group_description_index > 65535
                  ? (m = (r.sample_groups[l].group_description_index >> 16) - 1)
                  : (m = r.sample_groups[l].group_description_index - 1),
                p && m >= 0 && (r.sample_groups[l].description = p.entries[m]))
              : p &&
                p.version >= 2 &&
                p.default_group_description_index > 0 &&
                (r.sample_groups[l].description =
                  p.entries[p.default_group_description_index - 1]);
        }
    }),
    (w.process_sdtp = function (e, r, n) {
      r &&
        (e
          ? ((r.is_leading = e.is_leading[n]),
            (r.depends_on = e.sample_depends_on[n]),
            (r.is_depended_on = e.sample_is_depended_on[n]),
            (r.has_redundancy = e.sample_has_redundancy[n]))
          : ((r.is_leading = 0),
            (r.depends_on = 0),
            (r.is_depended_on = 0),
            (r.has_redundancy = 0)));
    }),
    (w.prototype.buildSampleLists = function () {
      var e, r;
      for (e = 0; e < this.moov.traks.length; e++)
        (r = this.moov.traks[e]), this.buildTrakSampleLists(r);
    }),
    (w.prototype.buildTrakSampleLists = function (e) {
      var r,
        n,
        a,
        l,
        m,
        p,
        S,
        R,
        O,
        P,
        y,
        F,
        I,
        $,
        J,
        _e,
        de,
        Te,
        be,
        we,
        Ie,
        Ke,
        ft,
        lt;
      if (
        ((e.samples = []),
        (e.samples_duration = 0),
        (e.samples_size = 0),
        (n = e.mdia.minf.stbl.stco || e.mdia.minf.stbl.co64),
        (a = e.mdia.minf.stbl.stsc),
        (l = e.mdia.minf.stbl.stsz || e.mdia.minf.stbl.stz2),
        (m = e.mdia.minf.stbl.stts),
        (p = e.mdia.minf.stbl.ctts),
        (S = e.mdia.minf.stbl.stss),
        (R = e.mdia.minf.stbl.stsd),
        (O = e.mdia.minf.stbl.subs),
        (F = e.mdia.minf.stbl.stdp),
        (P = e.mdia.minf.stbl.sbgps),
        (y = e.mdia.minf.stbl.sgpds),
        (Te = -1),
        (be = -1),
        (we = -1),
        (Ie = -1),
        (Ke = 0),
        (ft = 0),
        (lt = 0),
        w.initSampleGroups(e, null, P, y),
        !(typeof l > "u"))
      ) {
        for (r = 0; r < l.sample_sizes.length; r++) {
          var j = {};
          (j.number = r),
            (j.track_id = e.tkhd.track_id),
            (j.timescale = e.mdia.mdhd.timescale),
            (j.alreadyRead = 0),
            (e.samples[r] = j),
            (j.size = l.sample_sizes[r]),
            (e.samples_size += j.size),
            r === 0
              ? (($ = 1),
                (I = 0),
                (j.chunk_index = $),
                (j.chunk_run_index = I),
                (de = a.samples_per_chunk[I]),
                (_e = 0),
                I + 1 < a.first_chunk.length
                  ? (J = a.first_chunk[I + 1] - 1)
                  : (J = 1 / 0))
              : r < de
              ? ((j.chunk_index = $), (j.chunk_run_index = I))
              : ($++,
                (j.chunk_index = $),
                (_e = 0),
                $ <= J ||
                  (I++,
                  I + 1 < a.first_chunk.length
                    ? (J = a.first_chunk[I + 1] - 1)
                    : (J = 1 / 0)),
                (j.chunk_run_index = I),
                (de += a.samples_per_chunk[I])),
            (j.description_index =
              a.sample_description_index[j.chunk_run_index] - 1),
            (j.description = R.entries[j.description_index]),
            (j.offset = n.chunk_offsets[j.chunk_index - 1] + _e),
            (_e += j.size),
            r > Te && (be++, Te < 0 && (Te = 0), (Te += m.sample_counts[be])),
            r > 0
              ? ((e.samples[r - 1].duration = m.sample_deltas[be]),
                (e.samples_duration += e.samples[r - 1].duration),
                (j.dts = e.samples[r - 1].dts + e.samples[r - 1].duration))
              : (j.dts = 0),
            p
              ? (r >= we &&
                  (Ie++, we < 0 && (we = 0), (we += p.sample_counts[Ie])),
                (j.cts = e.samples[r].dts + p.sample_offsets[Ie]))
              : (j.cts = j.dts),
            S
              ? (r == S.sample_numbers[Ke] - 1
                  ? ((j.is_sync = !0), Ke++)
                  : ((j.is_sync = !1), (j.degradation_priority = 0)),
                O &&
                  O.entries[ft].sample_delta + lt == r + 1 &&
                  ((j.subsamples = O.entries[ft].subsamples),
                  (lt += O.entries[ft].sample_delta),
                  ft++))
              : (j.is_sync = !0),
            w.process_sdtp(e.mdia.minf.stbl.sdtp, j, j.number),
            F
              ? (j.degradation_priority = F.priority[r])
              : (j.degradation_priority = 0),
            O &&
              O.entries[ft].sample_delta + lt == r &&
              ((j.subsamples = O.entries[ft].subsamples),
              (lt += O.entries[ft].sample_delta)),
            (P.length > 0 || y.length > 0) &&
              w.setSampleGroupProperties(e, j, r, e.sample_groups_info);
        }
        r > 0 &&
          ((e.samples[r - 1].duration = Math.max(
            e.mdia.mdhd.duration - e.samples[r - 1].dts,
            0
          )),
          (e.samples_duration += e.samples[r - 1].duration));
      }
    }),
    (w.prototype.updateSampleLists = function () {
      var e, r, n, a, l, m, p, S, R, O, P, y, F, I, $;
      if (this.moov !== void 0) {
        for (; this.lastMoofIndex < this.moofs.length; )
          if (
            ((R = this.moofs[this.lastMoofIndex]),
            this.lastMoofIndex++,
            R.type == "moof")
          )
            for (O = R, e = 0; e < O.trafs.length; e++) {
              for (
                P = O.trafs[e],
                  y = this.getTrackById(P.tfhd.track_id),
                  F = this.getTrexById(P.tfhd.track_id),
                  P.tfhd.flags & o.TFHD_FLAG_SAMPLE_DESC
                    ? (a = P.tfhd.default_sample_description_index)
                    : (a = F ? F.default_sample_description_index : 1),
                  P.tfhd.flags & o.TFHD_FLAG_SAMPLE_DUR
                    ? (l = P.tfhd.default_sample_duration)
                    : (l = F ? F.default_sample_duration : 0),
                  P.tfhd.flags & o.TFHD_FLAG_SAMPLE_SIZE
                    ? (m = P.tfhd.default_sample_size)
                    : (m = F ? F.default_sample_size : 0),
                  P.tfhd.flags & o.TFHD_FLAG_SAMPLE_FLAGS
                    ? (p = P.tfhd.default_sample_flags)
                    : (p = F ? F.default_sample_flags : 0),
                  P.sample_number = 0,
                  P.sbgps.length > 0 &&
                    w.initSampleGroups(
                      y,
                      P,
                      P.sbgps,
                      y.mdia.minf.stbl.sgpds,
                      P.sgpds
                    ),
                  r = 0;
                r < P.truns.length;
                r++
              ) {
                var J = P.truns[r];
                for (n = 0; n < J.sample_count; n++) {
                  (I = {}),
                    (I.moof_number = this.lastMoofIndex),
                    (I.number_in_traf = P.sample_number),
                    P.sample_number++,
                    (I.number = y.samples.length),
                    (P.first_sample_index = y.samples.length),
                    y.samples.push(I),
                    (I.track_id = y.tkhd.track_id),
                    (I.timescale = y.mdia.mdhd.timescale),
                    (I.description_index = a - 1),
                    (I.description =
                      y.mdia.minf.stbl.stsd.entries[I.description_index]),
                    (I.size = m),
                    J.flags & o.TRUN_FLAGS_SIZE && (I.size = J.sample_size[n]),
                    (y.samples_size += I.size),
                    (I.duration = l),
                    J.flags & o.TRUN_FLAGS_DURATION &&
                      (I.duration = J.sample_duration[n]),
                    (y.samples_duration += I.duration),
                    y.first_traf_merged || n > 0
                      ? (I.dts =
                          y.samples[y.samples.length - 2].dts +
                          y.samples[y.samples.length - 2].duration)
                      : (P.tfdt
                          ? (I.dts = P.tfdt.baseMediaDecodeTime)
                          : (I.dts = 0),
                        (y.first_traf_merged = !0)),
                    (I.cts = I.dts),
                    J.flags & o.TRUN_FLAGS_CTS_OFFSET &&
                      (I.cts = I.dts + J.sample_composition_time_offset[n]),
                    ($ = p),
                    J.flags & o.TRUN_FLAGS_FLAGS
                      ? ($ = J.sample_flags[n])
                      : n === 0 &&
                        J.flags & o.TRUN_FLAGS_FIRST_FLAG &&
                        ($ = J.first_sample_flags),
                    (I.is_sync = !(($ >> 16) & 1)),
                    (I.is_leading = ($ >> 26) & 3),
                    (I.depends_on = ($ >> 24) & 3),
                    (I.is_depended_on = ($ >> 22) & 3),
                    (I.has_redundancy = ($ >> 20) & 3),
                    (I.degradation_priority = $ & 65535);
                  var _e = !!(P.tfhd.flags & o.TFHD_FLAG_BASE_DATA_OFFSET),
                    de = !!(P.tfhd.flags & o.TFHD_FLAG_DEFAULT_BASE_IS_MOOF),
                    Te = !!(J.flags & o.TRUN_FLAGS_DATA_OFFSET),
                    be = 0;
                  _e
                    ? (be = P.tfhd.base_data_offset)
                    : de || r === 0
                    ? (be = O.start)
                    : (be = S),
                    r === 0 && n === 0
                      ? Te
                        ? (I.offset = be + J.data_offset)
                        : (I.offset = be)
                      : (I.offset = S),
                    (S = I.offset + I.size),
                    (P.sbgps.length > 0 ||
                      P.sgpds.length > 0 ||
                      y.mdia.minf.stbl.sbgps.length > 0 ||
                      y.mdia.minf.stbl.sgpds.length > 0) &&
                      w.setSampleGroupProperties(
                        y,
                        I,
                        I.number_in_traf,
                        P.sample_groups_info
                      );
                }
              }
              if (P.subs) {
                y.has_fragment_subsamples = !0;
                var we = P.first_sample_index;
                for (r = 0; r < P.subs.entries.length; r++)
                  (we += P.subs.entries[r].sample_delta),
                    (I = y.samples[we - 1]),
                    (I.subsamples = P.subs.entries[r].subsamples);
              }
            }
      }
    }),
    (w.prototype.getSample = function (e, r) {
      var n,
        a = e.samples[r];
      if (!this.moov) return null;
      if (!a.data)
        (a.data = new Uint8Array(a.size)),
          (a.alreadyRead = 0),
          (this.samplesDataSize += a.size),
          f.debug(
            "ISOFile",
            "Allocating sample #" +
              r +
              " on track #" +
              e.tkhd.track_id +
              " of size " +
              a.size +
              " (total: " +
              this.samplesDataSize +
              ")"
          );
      else if (a.alreadyRead == a.size) return a;
      for (;;) {
        var l = this.stream.findPosition(!0, a.offset + a.alreadyRead, !1);
        if (l > -1) {
          n = this.stream.buffers[l];
          var m = n.byteLength - (a.offset + a.alreadyRead - n.fileStart);
          if (a.size - a.alreadyRead <= m)
            return (
              f.debug(
                "ISOFile",
                "Getting sample #" +
                  r +
                  " data (alreadyRead: " +
                  a.alreadyRead +
                  " offset: " +
                  (a.offset + a.alreadyRead - n.fileStart) +
                  " read size: " +
                  (a.size - a.alreadyRead) +
                  " full size: " +
                  a.size +
                  ")"
              ),
              d.memcpy(
                a.data.buffer,
                a.alreadyRead,
                n,
                a.offset + a.alreadyRead - n.fileStart,
                a.size - a.alreadyRead
              ),
              (n.usedBytes += a.size - a.alreadyRead),
              this.stream.logBufferLevel(),
              (a.alreadyRead = a.size),
              a
            );
          if (m === 0) return null;
          f.debug(
            "ISOFile",
            "Getting sample #" +
              r +
              " partial data (alreadyRead: " +
              a.alreadyRead +
              " offset: " +
              (a.offset + a.alreadyRead - n.fileStart) +
              " read size: " +
              m +
              " full size: " +
              a.size +
              ")"
          ),
            d.memcpy(
              a.data.buffer,
              a.alreadyRead,
              n,
              a.offset + a.alreadyRead - n.fileStart,
              m
            ),
            (a.alreadyRead += m),
            (n.usedBytes += m),
            this.stream.logBufferLevel();
        } else return null;
      }
    }),
    (w.prototype.releaseSample = function (e, r) {
      var n = e.samples[r];
      return n.data
        ? ((this.samplesDataSize -= n.size),
          (n.data = null),
          (n.alreadyRead = 0),
          n.size)
        : 0;
    }),
    (w.prototype.getAllocatedSampleDataSize = function () {
      return this.samplesDataSize;
    }),
    (w.prototype.getCodecs = function () {
      var e,
        r = "";
      for (e = 0; e < this.moov.traks.length; e++) {
        var n = this.moov.traks[e];
        e > 0 && (r += ","), (r += n.mdia.minf.stbl.stsd.entries[0].getCodec());
      }
      return r;
    }),
    (w.prototype.getTrexById = function (e) {
      var r;
      if (!this.moov || !this.moov.mvex) return null;
      for (r = 0; r < this.moov.mvex.trexs.length; r++) {
        var n = this.moov.mvex.trexs[r];
        if (n.track_id == e) return n;
      }
      return null;
    }),
    (w.prototype.getTrackById = function (e) {
      if (this.moov === void 0) return null;
      for (var r = 0; r < this.moov.traks.length; r++) {
        var n = this.moov.traks[r];
        if (n.tkhd.track_id == e) return n;
      }
      return null;
    }),
    (w.prototype.items = []),
    (w.prototype.itemsDataSize = 0),
    (w.prototype.flattenItemInfo = function () {
      var e = this.items,
        r,
        n,
        a,
        l = this.meta;
      if (l != null && l.hdlr !== void 0 && l.iinf !== void 0) {
        for (r = 0; r < l.iinf.item_infos.length; r++)
          (a = {}),
            (a.id = l.iinf.item_infos[r].item_ID),
            (e[a.id] = a),
            (a.ref_to = []),
            (a.name = l.iinf.item_infos[r].item_name),
            l.iinf.item_infos[r].protection_index > 0 &&
              (a.protection =
                l.ipro.protections[l.iinf.item_infos[r].protection_index - 1]),
            l.iinf.item_infos[r].item_type
              ? (a.type = l.iinf.item_infos[r].item_type)
              : (a.type = "mime"),
            (a.content_type = l.iinf.item_infos[r].content_type),
            (a.content_encoding = l.iinf.item_infos[r].content_encoding);
        if (l.iloc)
          for (r = 0; r < l.iloc.items.length; r++) {
            var m = l.iloc.items[r];
            switch (
              ((a = e[m.item_ID]),
              m.data_reference_index !== 0 &&
                (f.warn(
                  "Item storage with reference to other files: not supported"
                ),
                (a.source = l.dinf.boxes[m.data_reference_index - 1])),
              m.construction_method)
            ) {
              case 0:
                break;
              case 1:
                f.warn("Item storage with construction_method : not supported");
                break;
              case 2:
                f.warn("Item storage with construction_method : not supported");
                break;
            }
            for (a.extents = [], a.size = 0, n = 0; n < m.extents.length; n++)
              (a.extents[n] = {}),
                (a.extents[n].offset =
                  m.extents[n].extent_offset + m.base_offset),
                (a.extents[n].length = m.extents[n].extent_length),
                (a.extents[n].alreadyRead = 0),
                (a.size += a.extents[n].length);
          }
        if ((l.pitm && (e[l.pitm.item_id].primary = !0), l.iref))
          for (r = 0; r < l.iref.references.length; r++) {
            var p = l.iref.references[r];
            for (n = 0; n < p.references.length; n++)
              e[p.from_item_ID].ref_to.push({
                type: p.type,
                id: p.references[n],
              });
          }
        if (l.iprp)
          for (var S = 0; S < l.iprp.ipmas.length; S++) {
            var R = l.iprp.ipmas[S];
            for (r = 0; r < R.associations.length; r++) {
              var O = R.associations[r];
              for (
                a = e[O.id],
                  a.properties === void 0 &&
                    ((a.properties = {}), (a.properties.boxes = [])),
                  n = 0;
                n < O.props.length;
                n++
              ) {
                var P = O.props[n];
                if (
                  P.property_index > 0 &&
                  P.property_index - 1 < l.iprp.ipco.boxes.length
                ) {
                  var y = l.iprp.ipco.boxes[P.property_index - 1];
                  (a.properties[y.type] = y), a.properties.boxes.push(y);
                }
              }
            }
          }
      }
    }),
    (w.prototype.getItem = function (e) {
      var r, n;
      if (!this.meta) return null;
      if (((n = this.items[e]), !n.data && n.size))
        (n.data = new Uint8Array(n.size)),
          (n.alreadyRead = 0),
          (this.itemsDataSize += n.size),
          f.debug(
            "ISOFile",
            "Allocating item #" +
              e +
              " of size " +
              n.size +
              " (total: " +
              this.itemsDataSize +
              ")"
          );
      else if (n.alreadyRead === n.size) return n;
      for (var a = 0; a < n.extents.length; a++) {
        var l = n.extents[a];
        if (l.alreadyRead !== l.length) {
          var m = this.stream.findPosition(!0, l.offset + l.alreadyRead, !1);
          if (m > -1) {
            r = this.stream.buffers[m];
            var p = r.byteLength - (l.offset + l.alreadyRead - r.fileStart);
            if (l.length - l.alreadyRead <= p)
              f.debug(
                "ISOFile",
                "Getting item #" +
                  e +
                  " extent #" +
                  a +
                  " data (alreadyRead: " +
                  l.alreadyRead +
                  " offset: " +
                  (l.offset + l.alreadyRead - r.fileStart) +
                  " read size: " +
                  (l.length - l.alreadyRead) +
                  " full extent size: " +
                  l.length +
                  " full item size: " +
                  n.size +
                  ")"
              ),
                d.memcpy(
                  n.data.buffer,
                  n.alreadyRead,
                  r,
                  l.offset + l.alreadyRead - r.fileStart,
                  l.length - l.alreadyRead
                ),
                (r.usedBytes += l.length - l.alreadyRead),
                this.stream.logBufferLevel(),
                (n.alreadyRead += l.length - l.alreadyRead),
                (l.alreadyRead = l.length);
            else
              return (
                f.debug(
                  "ISOFile",
                  "Getting item #" +
                    e +
                    " extent #" +
                    a +
                    " partial data (alreadyRead: " +
                    l.alreadyRead +
                    " offset: " +
                    (l.offset + l.alreadyRead - r.fileStart) +
                    " read size: " +
                    p +
                    " full extent size: " +
                    l.length +
                    " full item size: " +
                    n.size +
                    ")"
                ),
                d.memcpy(
                  n.data.buffer,
                  n.alreadyRead,
                  r,
                  l.offset + l.alreadyRead - r.fileStart,
                  p
                ),
                (l.alreadyRead += p),
                (n.alreadyRead += p),
                (r.usedBytes += p),
                this.stream.logBufferLevel(),
                null
              );
          } else return null;
        }
      }
      return n.alreadyRead === n.size ? n : null;
    }),
    (w.prototype.releaseItem = function (e) {
      var r = this.items[e];
      if (r.data) {
        (this.itemsDataSize -= r.size), (r.data = null), (r.alreadyRead = 0);
        for (var n = 0; n < r.extents.length; n++) {
          var a = r.extents[n];
          a.alreadyRead = 0;
        }
        return r.size;
      } else return 0;
    }),
    (w.prototype.processItems = function (e) {
      for (var r in this.items) {
        var n = this.items[r];
        this.getItem(n.id),
          e && !n.sent && (e(n), (n.sent = !0), (n.data = null));
      }
    }),
    (w.prototype.hasItem = function (e) {
      for (var r in this.items) {
        var n = this.items[r];
        if (n.name === e) return n.id;
      }
      return -1;
    }),
    (w.prototype.getMetaHandler = function () {
      return this.meta ? this.meta.hdlr.handler : null;
    }),
    (w.prototype.getPrimaryItem = function () {
      return !this.meta || !this.meta.pitm
        ? null
        : this.getItem(this.meta.pitm.item_id);
    }),
    (w.prototype.itemToFragmentedTrackFile = function (e) {
      var r = e || {},
        n = null;
      if (
        (r.itemId ? (n = this.getItem(r.itemId)) : (n = this.getPrimaryItem()),
        n == null)
      )
        return null;
      var a = new w();
      a.discardMdatData = !1;
      var l = {
        type: n.type,
        description_boxes: n.properties.boxes,
      };
      n.properties.ispe &&
        ((l.width = n.properties.ispe.image_width),
        (l.height = n.properties.ispe.image_height));
      var m = a.addTrack(l);
      return m ? (a.addSample(m, n.data), a) : null;
    }),
    (w.prototype.write = function (e) {
      for (var r = 0; r < this.boxes.length; r++) this.boxes[r].write(e);
    }),
    (w.prototype.createFragment = function (e, r, n) {
      var a = this.getTrackById(e),
        l = this.getSample(a, r);
      if (l == null)
        return (
          (l = a.samples[r]),
          this.nextSeekPosition
            ? (this.nextSeekPosition = Math.min(
                l.offset + l.alreadyRead,
                this.nextSeekPosition
              ))
            : (this.nextSeekPosition = a.samples[r].offset + l.alreadyRead),
          null
        );
      var m = n || new d();
      m.endianness = d.BIG_ENDIAN;
      var p = this.createSingleSampleMoof(l);
      p.write(m),
        (p.trafs[0].truns[0].data_offset = p.size + 8),
        f.debug(
          "MP4Box",
          "Adjusting data_offset with new value " +
            p.trafs[0].truns[0].data_offset
        ),
        m.adjustUint32(
          p.trafs[0].truns[0].data_offset_position,
          p.trafs[0].truns[0].data_offset
        );
      var S = new o.mdatBox();
      return (S.data = l.data), S.write(m), m;
    }),
    (w.writeInitializationSegment = function (e, r, n, a) {
      var l;
      f.debug("ISOFile", "Generating initialization segment");
      var m = new d();
      (m.endianness = d.BIG_ENDIAN), e.write(m);
      var p = r.add("mvex");
      for (
        n && p.add("mehd").set("fragment_duration", n), l = 0;
        l < r.traks.length;
        l++
      )
        p.add("trex")
          .set("track_id", r.traks[l].tkhd.track_id)
          .set("default_sample_description_index", 1)
          .set("default_sample_duration", a)
          .set("default_sample_size", 0)
          .set("default_sample_flags", 65536);
      return r.write(m), m.buffer;
    }),
    (w.prototype.save = function (e) {
      var r = new d();
      (r.endianness = d.BIG_ENDIAN), this.write(r), r.save(e);
    }),
    (w.prototype.getBuffer = function () {
      var e = new d();
      return (e.endianness = d.BIG_ENDIAN), this.write(e), e.buffer;
    }),
    (w.prototype.initializeSegmentation = function () {
      var e, r, n, a;
      for (
        this.onSegment === null &&
          f.warn("MP4Box", "No segmentation callback set!"),
          this.isFragmentationInitialized ||
            ((this.isFragmentationInitialized = !0),
            (this.nextMoofNumber = 0),
            this.resetTables()),
          r = [],
          e = 0;
        e < this.fragmentedTracks.length;
        e++
      ) {
        var l = new o.moovBox();
        (l.mvhd = this.moov.mvhd),
          l.boxes.push(l.mvhd),
          (n = this.getTrackById(this.fragmentedTracks[e].id)),
          l.boxes.push(n),
          l.traks.push(n),
          (a = {}),
          (a.id = n.tkhd.track_id),
          (a.user = this.fragmentedTracks[e].user),
          (a.buffer = w.writeInitializationSegment(
            this.ftyp,
            l,
            this.moov.mvex && this.moov.mvex.mehd
              ? this.moov.mvex.mehd.fragment_duration
              : void 0,
            this.moov.traks[e].samples.length > 0
              ? this.moov.traks[e].samples[0].duration
              : 0
          )),
          r.push(a);
      }
      return r;
    }),
    (o.Box.prototype.printHeader = function (e) {
      (this.size += 8),
        this.size > U && (this.size += 8),
        this.type === "uuid" && (this.size += 16),
        e.log(e.indent + "size:" + this.size),
        e.log(e.indent + "type:" + this.type);
    }),
    (o.FullBox.prototype.printHeader = function (e) {
      (this.size += 4),
        o.Box.prototype.printHeader.call(this, e),
        e.log(e.indent + "version:" + this.version),
        e.log(e.indent + "flags:" + this.flags);
    }),
    (o.Box.prototype.print = function (e) {
      this.printHeader(e);
    }),
    (o.ContainerBox.prototype.print = function (e) {
      this.printHeader(e);
      for (var r = 0; r < this.boxes.length; r++)
        if (this.boxes[r]) {
          var n = e.indent;
          (e.indent += " "), this.boxes[r].print(e), (e.indent = n);
        }
    }),
    (w.prototype.print = function (e) {
      e.indent = "";
      for (var r = 0; r < this.boxes.length; r++)
        this.boxes[r] && this.boxes[r].print(e);
    }),
    (o.mvhdBox.prototype.print = function (e) {
      o.FullBox.prototype.printHeader.call(this, e),
        e.log(e.indent + "creation_time: " + this.creation_time),
        e.log(e.indent + "modification_time: " + this.modification_time),
        e.log(e.indent + "timescale: " + this.timescale),
        e.log(e.indent + "duration: " + this.duration),
        e.log(e.indent + "rate: " + this.rate),
        e.log(e.indent + "volume: " + (this.volume >> 8)),
        e.log(e.indent + "matrix: " + this.matrix.join(", ")),
        e.log(e.indent + "next_track_id: " + this.next_track_id);
    }),
    (o.tkhdBox.prototype.print = function (e) {
      o.FullBox.prototype.printHeader.call(this, e),
        e.log(e.indent + "creation_time: " + this.creation_time),
        e.log(e.indent + "modification_time: " + this.modification_time),
        e.log(e.indent + "track_id: " + this.track_id),
        e.log(e.indent + "duration: " + this.duration),
        e.log(e.indent + "volume: " + (this.volume >> 8)),
        e.log(e.indent + "matrix: " + this.matrix.join(", ")),
        e.log(e.indent + "layer: " + this.layer),
        e.log(e.indent + "alternate_group: " + this.alternate_group),
        e.log(e.indent + "width: " + this.width),
        e.log(e.indent + "height: " + this.height);
    });
  var k = {};
  (k.createFile = function (e, r) {
    var n = e !== void 0 ? e : !0,
      a = new w(r);
    return (a.discardMdatData = !n), a;
  }),
    (g.createFile = k.createFile);
})(Eu);
var _t = {};
Object.defineProperty(_t, "__esModule", {
  value: !0,
});
_t.setShowTimeCost = _t.setLoggerLevel = void 0;
const Ni = Jt;
let Wi = Ni.LOG_LEVEL.WARN;
const Tg = function (g) {
  Wi = g;
};
_t.setLoggerLevel = Tg;
let bs = !1;
const Cg = function (g) {
  bs = g;
};
_t.setShowTimeCost = Cg;
class Uu {
  static time(f) {}
  static timeEnd(f) {}
  static debug(...f) {
    Wi > Ni.LOG_LEVEL.DEBUG || (f = f || []);
  }
  static log(...f) {
    Wi > Ni.LOG_LEVEL.LOG || (f = f || []);
  }
  static error(...f) {
    Wi > Ni.LOG_LEVEL.ERROR || (f = f || []);
  }
  static warn(...f) {
    Wi > Ni.LOG_LEVEL.WARN || (f = f || []);
  }
}
_t.default = Uu;
Uu.TAG = "[DECRYPT_VIDEO]";
var Ee = {};
Object.defineProperty(Ee, "__esModule", {
  value: !0,
});
Ee.DEFAULT_OPTIONS =
  Ee.MOOV_FIRST_BUF_SIZE =
  Ee.DEFAULT_POOL_OPTIONS =
  Ee.MAIN_THREAD_CMD =
  Ee.WORKER_CMD =
  Ee.ERROR_TYPE =
  Ee.STALLED_ERROR_TYPE =
    void 0;
const Rg = Jt;
Ee.STALLED_ERROR_TYPE = {
  unknown: "unknown",
  bufferHole: "bufferHole",
  currentTimeOutOfBounds: "currentTimeOutOfBounds",
  currentTimeInBuffered: "currentTimeInBuffered",
};
Ee.ERROR_TYPE = {
  MSE_ERROR: "MSE_ERROR",
  WORKER_ERROR: "WORKER_ERROR",
  FFMPEG_ERROR: "FFMPEG_ERROR",
  NETWORK_TIMEOUT_ERROR: "NETWORK_TIMEOUT_ERROR",
  NETWORK_REQUEST_ERROR: "NETWORK_REQUEST_ERROR",
  NETWORK_TIMEOUT_RETRY: "NETWORK_TIMEOUT_RETRY",
  NETWORK_STATUS_CODE_ERROR: "NETWORK_STATUS_CODE_ERROR",
  WASM_ERROR: "WASM_ERROR",
  HEADER_ERROR: "HEADER_ERROR",
  FIRST_SEG_CALLBACK_ERROR: "FIRST_SEG_CALLBACK_ERROR",
  FIRST_SEG_REMUX_CALLBACK_ERROR: "FIRST_SEG_REMUX_CALLBACK_ERROR",
  ON_REQ_SUCCESS_CALLBACK_ERROR: "ON_REQ_SUCCESS_CALLBACK_ERROR",
  ON_DISPOSE_CALLBACK_ERROR: "ON_DISPOSE_CALLBACK_ERROR",
  LOAD_ERROR: "LOAD_ERROR",
};
Ee.WORKER_CMD = {
  RESUME: "RESUME",
  PAUSE: "PAUSE",
  CUT: "CUT",
  DISPOSE: "DISPOSE",
  INIT_FIRST_BUFFER: "INIT_FIRST_BUFFER",
  UPDATE_CONFIG: "UPDATE_CONFIG",
  DECRYPT_BUFFER: "DECRYPT_BUFFER",
};
Ee.MAIN_THREAD_CMD = {
  AUTO_CUT: "AUTO_CUT",
  CUT_ENDED: "CUT_ENDED",
  ON_REQUEST_SUCCESS: "ON_REQUEST_SUCCESS",
};
Ee.DEFAULT_POOL_OPTIONS = {
  workerLimitNum: 3,
};
Ee.MOOV_FIRST_BUF_SIZE = 1 * 1024 * 1024;
Ee.DEFAULT_OPTIONS = {
  url: "",
  maxBufferedTime: 25,
  minBufferedTime: 5,
  maxBackwardBufferedTime: 3 * 60,
  minBackwardBufferedTime: 2 * 60,
  cacheConfig: {
    useLruCache: !1,
    lruCacheSize: 0,
  },
  ffmpegConfig: {
    segmentDuration: 3,
  },
  networkConfig: {
    firstBufTimeout: 10 * 1e3,
    retryTimeout: 10 * 1e3,
    firstBufSize: 1 * 1024 * 1024,
    concurrentNum: 2,
    timeout: 10 * 1e3,
    useDynamicFirstBuf: !1,
    winBufSize: 1 * 1024 * 1024,
    networkRetryTimes: 5,
  },
  logConfig: {
    openDebugLog: !1,
    openTimeLog: !1,
    logLevel: Rg.LOG_LEVEL.WARN,
  },
};
var Cs = {},
  Re = {};
Object.defineProperty(Re, "__esModule", {
  value: !0,
});
Re.getHostFromUrl =
  Re.getBufferedRanges =
  Re.getPerformanceData =
  Re.getPerfDataList =
  Re.generateUniqueId =
  Re.calKBSpeed =
  Re.appendDataToBuffer =
    void 0;
const Ag = function (g) {
  let f = 0;
  for (let U = 0; U < g.length; U++) {
    const x = g[U];
    x && (f += x.byteLength);
  }
  const h = new Uint8Array(f);
  let d = 0;
  for (let U = 0; U < g.length; U++) {
    const x = g[U];
    x && (h.set(x, d), (d += x.byteLength));
  }
  return h;
};
Re.appendDataToBuffer = Ag;
const Bg = (g, f) =>
  typeof g == "number" && typeof f == "number" && f && g
    ? Math.round(((g / f) * 1e3) / 1024)
    : 0;
Re.calKBSpeed = Bg;
function Ig() {
  const g = Date.now(),
    f = Math.random().toString(36).substring(2, 8);
  return "".concat(g, "-").concat(f);
}
Re.generateUniqueId = Ig;
function Lg(g) {
  g = g || [];
  const f = [];
  for (let h = 0; h < g.length; h++) {
    const d = g[h],
      U = xu(d);
    f.push(
      Object.assign(
        {
          _idx: h,
        },
        U
      )
    );
  }
  return f;
}
Re.getPerfDataList = Lg;
function xu(g) {
  try {
    const h = performance
      .getEntriesByType("resource")
      .find((d) => d.name === g);
    return h
      ? {
          total: h.responseEnd - h.startTime,
          dnsLookup: h.domainLookupEnd - h.domainLookupStart,
          tcpConnect: h.connectEnd - h.connectStart,
          firstByte: h.responseStart - h.startTime,
          download: h.responseEnd - h.responseStart,
          redirect: h.redirectEnd - h.redirectStart,
        }
      : {};
  } catch (f) {
    return {
      errMsg: f.toString(),
    };
  }
}
Re.getPerformanceData = xu;
function Og(g) {
  try {
    if (!g) return [];
    const f = g.buffered;
    if (!f) return [];
    const h = [];
    for (let d = 0; d < f.length; d++)
      h.push({
        start: f.start(d),
        end: f.end(d),
      });
    return h;
  } catch (f) {
    return [];
  }
}
Re.getBufferedRanges = Og;
function Pg(g) {
  if (!g) return "";
  try {
    return new URL(g).hostname;
  } catch (f) {
    return "";
  }
}
Re.getHostFromUrl = Pg;
var Gi = {};
Object.defineProperty(Gi, "__esModule", {
  value: !0,
});
class kg extends Error {
  constructor(f) {
    super(f), (this.name = "DecryptVideoCoreError");
  }
}
Gi.default = kg;
var Rs = {},
  pi = {};
Object.defineProperty(pi, "__esModule", {
  value: !0,
});
pi.IS_BROWSER = pi.exportKey = void 0;
let Tu = "";
typeof location < "u" &&
  location.href &&
  typeof URL == "function" &&
  (Tu = new URL(location.href).searchParams.get("exportkey"));
pi.exportKey = Tu;
let ws = !1;
try {
  typeof window < "u" &&
    typeof document < "u" &&
    typeof localStorage < "u" &&
    typeof localStorage.getItem == "function" &&
    (ws = !0);
} catch (g) {
  ws = !1;
}
pi.IS_BROWSER = ws;
var qi = {};
Object.defineProperty(qi, "__esModule", {
  value: !0,
});
qi.BaseAdapter = void 0;
class Fg {
  constructor(f) {
    this.options = f;
  }
}
qi.BaseAdapter = Fg;
var jt = {};
(function (g) {
  g();
})(function () {
  function g(l, m) {
    if (!(l instanceof m))
      throw new TypeError("Cannot call a class as a function");
  }
  function f(l, m) {
    for (var p = 0; p < m.length; p++) {
      var S = m[p];
      (S.enumerable = S.enumerable || !1),
        (S.configurable = !0),
        "value" in S && (S.writable = !0),
        Object.defineProperty(l, S.key, S);
    }
  }
  function h(l, m, p) {
    return m && f(l.prototype, m), l;
  }
  function d(l, m) {
    if (typeof m != "function" && m !== null)
      throw new TypeError("Super expression must either be null or a function");
    (l.prototype = Object.create(m && m.prototype, {
      constructor: {
        value: l,
        writable: !0,
        configurable: !0,
      },
    })),
      m && x(l, m);
  }
  function U(l) {
    return (
      (U = Object.setPrototypeOf
        ? Object.getPrototypeOf
        : function (p) {
            return p.__proto__ || Object.getPrototypeOf(p);
          }),
      U(l)
    );
  }
  function x(l, m) {
    return (
      (x =
        Object.setPrototypeOf ||
        function (S, R) {
          return (S.__proto__ = R), S;
        }),
      x(l, m)
    );
  }
  function A() {
    if (typeof Reflect > "u" || !Reflect.construct || Reflect.construct.sham)
      return !1;
    if (typeof Proxy == "function") return !0;
    try {
      return (
        Boolean.prototype.valueOf.call(
          Reflect.construct(Boolean, [], function () {})
        ),
        !0
      );
    } catch (l) {
      return !1;
    }
  }
  function o(l) {
    if (l === void 0)
      throw new ReferenceError(
        "this hasn't been initialised - super() hasn't been called"
      );
    return l;
  }
  function M(l, m) {
    return m && (typeof m == "object" || typeof m == "function") ? m : o(l);
  }
  function B(l) {
    var m = A();
    return function () {
      var S = U(l),
        R;
      if (m) {
        var O = U(this).constructor;
        R = Reflect.construct(S, arguments, O);
      } else R = S.apply(this, arguments);
      return M(this, R);
    };
  }
  function w(l, m) {
    for (
      ;
      !Object.prototype.hasOwnProperty.call(l, m) && ((l = U(l)), l !== null);

    );
    return l;
  }
  function k(l, m, p) {
    return (
      typeof Reflect < "u" && Reflect.get
        ? (k = Reflect.get)
        : (k = function (R, O, P) {
            var y = w(R, O);
            if (y) {
              var F = Object.getOwnPropertyDescriptor(y, O);
              return F.get ? F.get.call(P) : F.value;
            }
          }),
      k(l, m, p || l)
    );
  }
  var e = (function () {
      function l() {
        g(this, l),
          Object.defineProperty(this, "listeners", {
            value: {},
            writable: !0,
            configurable: !0,
          });
      }
      return (
        h(l, [
          {
            key: "addEventListener",
            value: function (p, S, R) {
              p in this.listeners || (this.listeners[p] = []),
                this.listeners[p].push({
                  callback: S,
                  options: R,
                });
            },
          },
          {
            key: "removeEventListener",
            value: function (p, S) {
              if (p in this.listeners) {
                for (var R = this.listeners[p], O = 0, P = R.length; O < P; O++)
                  if (R[O].callback === S) {
                    R.splice(O, 1);
                    return;
                  }
              }
            },
          },
          {
            key: "dispatchEvent",
            value: function (p) {
              if (p.type in this.listeners) {
                for (
                  var S = this.listeners[p.type],
                    R = S.slice(),
                    O = 0,
                    P = R.length;
                  O < P;
                  O++
                ) {
                  var y = R[O];
                  try {
                    y.callback.call(this, p);
                  } catch (F) {
                    Promise.resolve().then(function () {
                      throw F;
                    });
                  }
                  y.options &&
                    y.options.once &&
                    this.removeEventListener(p.type, y.callback);
                }
                return !p.defaultPrevented;
              }
            },
          },
        ]),
        l
      );
    })(),
    r = (function (l) {
      d(p, l);
      var m = B(p);
      function p() {
        var S;
        return (
          g(this, p),
          (S = m.call(this)),
          S.listeners || e.call(o(S)),
          Object.defineProperty(o(S), "aborted", {
            value: !1,
            writable: !0,
            configurable: !0,
          }),
          Object.defineProperty(o(S), "onabort", {
            value: null,
            writable: !0,
            configurable: !0,
          }),
          S
        );
      }
      return (
        h(p, [
          {
            key: "toString",
            value: function () {
              return "[object AbortSignal]";
            },
          },
          {
            key: "dispatchEvent",
            value: function (R) {
              R.type === "abort" &&
                ((this.aborted = !0),
                typeof this.onabort == "function" &&
                  this.onabort.call(this, R)),
                k(U(p.prototype), "dispatchEvent", this).call(this, R);
            },
          },
        ]),
        p
      );
    })(e),
    n = (function () {
      function l() {
        g(this, l),
          Object.defineProperty(this, "signal", {
            value: new r(),
            writable: !0,
            configurable: !0,
          });
      }
      return (
        h(l, [
          {
            key: "abort",
            value: function () {
              var p;
              try {
                p = new Event("abort");
              } catch (S) {
                typeof document < "u"
                  ? document.createEvent
                    ? ((p = document.createEvent("Event")),
                      p.initEvent("abort", !1, !1))
                    : ((p = document.createEventObject()), (p.type = "abort"))
                  : (p = {
                      type: "abort",
                      bubbles: !1,
                      cancelable: !1,
                    });
              }
              this.signal.dispatchEvent(p);
            },
          },
          {
            key: "toString",
            value: function () {
              return "[object AbortController]";
            },
          },
        ]),
        l
      );
    })();
  typeof Symbol < "u" &&
    Symbol.toStringTag &&
    ((n.prototype[Symbol.toStringTag] = "AbortController"),
    (r.prototype[Symbol.toStringTag] = "AbortSignal"));
  function a(l) {
    return l.__FORCE_INSTALL_ABORTCONTROLLER_POLYFILL
      ? !0
      : (typeof l.Request == "function" &&
          !l.Request.prototype.hasOwnProperty("signal")) ||
          !l.AbortController;
  }
  (function (l) {
    a(l) && ((l.AbortController = n), (l.AbortSignal = r));
  })(typeof self < "u" ? self : H);
});
var As = {},
  Gr = {},
  pt = {};
Object.defineProperty(pt, "__esModule", {
  value: !0,
});
pt.CHECK_IS_PREFETCH =
  pt.IS_MINI_PROGRAM =
  pt.IS_NATIVE_LIKE_APP =
  pt.IS_LITE_APP =
    void 0;
pt.IS_LITE_APP = typeof lite < "u";
pt.IS_NATIVE_LIKE_APP =
  typeof wxNative < "u" && typeof wxNative.webTransfer == "function";
pt.IS_MINI_PROGRAM = typeof wx < "u" && typeof wx.request == "function";
const Dg = () =>
  typeof window < "u" &&
  typeof window.WeixinPrefecherJSBridge < "u" &&
  typeof window.WeixinPrefecherJSBridge.invoke == "function";
pt.CHECK_IS_PREFETCH = Dg;
(function (g) {
  var f =
      (H && H.__createBinding) ||
      (Object.create
        ? function (d, U, x, A) {
            A === void 0 && (A = x);
            var o = Object.getOwnPropertyDescriptor(U, x);
            (!o ||
              ("get" in o ? !U.__esModule : o.writable || o.configurable)) &&
              (o = {
                enumerable: !0,
                get: function () {
                  return U[x];
                },
              }),
              Object.defineProperty(d, A, o);
          }
        : function (d, U, x, A) {
            A === void 0 && (A = x), (d[A] = U[x]);
          }),
    h =
      (H && H.__exportStar) ||
      function (d, U) {
        for (var x in d)
          x !== "default" &&
            !Object.prototype.hasOwnProperty.call(U, x) &&
            f(U, d, x);
      };
  Object.defineProperty(g, "__esModule", {
    value: !0,
  }),
    (g.FingerPrintDeviceIdStoryKey = g.FingerPrintDeviceIdHeaderKey = void 0),
    h(pt, g),
    (g.FingerPrintDeviceIdHeaderKey = "finger-print-device-id"),
    (g.FingerPrintDeviceIdStoryKey = "_finger_print_device_id");
})(Gr);
(function (g) {
  var f =
    (H && H.__importDefault) ||
    function (o) {
      return o && o.__esModule
        ? o
        : {
            default: o,
          };
    };
  Object.defineProperty(g, "__esModule", {
    value: !0,
  }),
    (g.getAppId = g.cache = g.transformResponse = g.transformError = void 0);
  const h = f(yu()),
    d = Gr;
  function U(o, M, B) {
    o.errMsg.indexOf("request:fail abort") !== -1
      ? M((0, h.default)("Request aborted", B, "ECONNABORTED", ""))
      : o.errMsg.indexOf("timeout") !== -1
      ? M(
          (0, h.default)(
            "timeout of " + B.timeout + "ms exceeded",
            B,
            "ECONNABORTED",
            ""
          )
        )
      : M((0, h.default)("Network Error", B, null, ""));
  }
  g.transformError = U;
  function x(o, M, B) {
    const w = o.header,
      k = o.statusCode;
    let e = "";
    return (
      k === 200 ? (e = "OK") : k === 400 && (e = "Bad Request"),
      {
        data: o.data,
        status: k,
        statusText: e,
        headers: w,
        config: M,
        request: B,
      }
    );
  }
  (g.transformResponse = x), (g.cache = {});
  function A() {
    if (g.cache.appId) return g.cache.appId;
    try {
      if (d.IS_LITE_APP) {
        if (
          lite &&
          lite.system &&
          typeof lite.system.getSystemInfo == "function"
        ) {
          const { appId: o } = lite.system.getSystemInfo();
          g.cache.appId = o;
        }
      } else if (d.IS_MINI_PROGRAM && wx && wx.getSystemInfoSync) {
        const { host: o } = wx.getSystemInfoSync();
        g.cache.appId = o == null ? void 0 : o.appId;
      }
    } catch (o) {}
    return g.cache.appId;
  }
  g.getAppId = A;
})(As);
var Tt = {};
Object.defineProperty(Tt, "__esModule", {
  value: !0,
});
Tt.getLoginErrorCode = Tt.isLoginError = Tt.getUniqueId = void 0;
let Mg = 0;
const zg = () => "requestService_$".concat(Mg++);
Tt.getUniqueId = zg;
const Hi = (g) => (g >= 330 && g <= 350) || g === -14;
function Ng(g) {
  if (g.error && g.error.message === "页面校验失败") return !0;
  let f = +"".concat(Math.abs(g.errCode)).slice(-3);
  return (
    g.error &&
      g.error.rpcResult &&
      (f = +"".concat(Math.abs(g.error.rpcResult.returnCode)).slice(-3)),
    g.data &&
      g.data.baseResp &&
      g.data.baseResp.errcode &&
      (f = +"".concat(Math.abs(g.data.baseResp.errcode)).slice(-3)),
    g.data &&
      g.data.base_resp &&
      g.data.base_resp.errcode &&
      (f = +"".concat(Math.abs(g.data.base_resp.errcode)).slice(-3)),
    Hi(f)
  );
}
Tt.isLoginError = Ng;
function Wg(g) {
  if (g.error && g.error.message === "页面校验失败") return !0;
  let f = +"".concat(Math.abs(g.errCode)).slice(-3);
  if (Hi(f)) return g.errCode;
  if (
    g.error &&
    g.error.rpcResult &&
    ((f = +"".concat(Math.abs(g.error.rpcResult.returnCode)).slice(-3)), Hi(f))
  )
    return g.error.rpcResult.returnCode;
  if (
    g.data &&
    g.data.baseResp &&
    g.data.baseResp.errcode &&
    ((f = +"".concat(Math.abs(g.data.baseResp.errcode)).slice(-3)), Hi(f))
  )
    return g.data.baseResp.errcode;
  if (
    g.data &&
    g.data.base_resp &&
    g.data.base_resp.errcode &&
    ((f = +"".concat(Math.abs(g.data.base_resp.errcode)).slice(-3)), Hi(f))
  )
    return g.data.base_resp.errcode;
}
Tt.getLoginErrorCode = Wg;
var _s =
    (H && H.__awaiter) ||
    function (g, f, h, d) {
      function U(x) {
        return x instanceof h
          ? x
          : new h(function (A) {
              A(x);
            });
      }
      return new (h || (h = Promise))(function (x, A) {
        function o(w) {
          try {
            B(d.next(w));
          } catch (k) {
            A(k);
          }
        }
        function M(w) {
          try {
            B(d.throw(w));
          } catch (k) {
            A(k);
          }
        }
        function B(w) {
          w.done ? x(w.value) : U(w.value).then(o, M);
        }
        B((d = d.apply(g, f || [])).next());
      });
    },
  Yi =
    (H && H.__importDefault) ||
    function (g) {
      return g && g.__esModule
        ? g
        : {
            default: g,
          };
    };
Object.defineProperty(jt, "__esModule", {
  value: !0,
});
jt.FinderLiteAppAdapter = jt.LiteAppAdapter = void 0;
const pu = Yi(Su),
  Hg = Yi(bu()),
  Gg = Yi(wu),
  qg = Yi(Us()),
  Ft = Yi(yu()),
  Yg = As,
  $g = qi,
  Vg = Tt;
let zr, gs;
const Kg = 20 * 1e3;
class Bs extends $g.BaseAdapter {
  getLoginCode(f) {
    return _s(this, void 0, void 0, function* () {
      if (
        !this.options.disabledLiteAppAutoLogin &&
        !f.disabledLiteAppAutoLogin
      ) {
        if (!zr) {
          const h = this.options.getLoginParams
            ? this.options.getLoginParams(f)
            : {};
          zr = new Promise((d) => {
            lite.jsapi.invoke("login", h, (U) => {
              d(U);
            });
          });
        }
        yield zr;
      }
    });
  }
  doRequest(f) {
    return new Promise((h, d) =>
      _s(this, void 0, void 0, function* () {
        yield this.getLoginCode(f);
        const U = f.data || null,
          x = f.headers || {};
        let A = !0;
        if (f.auth) {
          const a = f.auth.username || "",
            l = f.auth.password || "";
          x.Authorization = "Basic " + btoa(a + ":" + l);
        }
        const o = {};
        pu.default.isUndefined(f.withCredentials) ||
          (o.credentials = "include");
        const M =
          typeof AbortController < "u"
            ? new AbortController()
            : {
                signal: null,
                abort: () => {},
              };
        f.cancelToken &&
          ((o.signal = M.signal),
          f.cancelToken.promise.then(function (l) {
            A && ((A = !1), M.abort(), d(l));
          }));
        function B() {
          if (f.timeout) {
            o.signal || (o.signal = M.signal);
            let a = "timeout of " + f.timeout + "ms exceeded";
            f.timeoutErrorMessage && (a = f.timeoutErrorMessage),
              setTimeout(function () {
                A &&
                  ((A = !1),
                  M.abort(),
                  d((0, Ft.default)(a, f, "ECONNABORTED", f, null)));
              }, f.timeout);
          }
        }
        const w = {};
        for (const a in x) x.hasOwnProperty(a) && (w[a] = x[a]);
        const k = (0, qg.default)(f.baseURL, f.url),
          e = Object.assign(
            {},
            o,
            Object.assign(
              {},
              {
                method: f.method.toUpperCase(),
                body: U,
                headers: w,
              },
              f.fetchOptions || {}
            )
          ),
          r = new Request(
            (0, Gg.default)(
              k,
              Object.assign(
                {
                  _loginAppId:
                    this.options.appId || f.appId || (0, Yg.getAppId)(),
                },
                f.params
              ),
              f.paramsSerializer
            ),
            e
          ),
          n = fetch(r.url, e);
        B(),
          n.then(
            function (l) {
              if (((A = !1), l.ok)) {
                pu.default.isFormData(U) && delete x["Content-Type"];
                const m = l.headers;
                let p = null;
                switch (f.responseType) {
                  case "arraybuffer":
                    p = l.arrayBuffer();
                    break;
                  case "text":
                    p = l.text();
                    break;
                  case "json":
                    p = l.json();
                    break;
                  case "blob":
                    p = l.blob();
                    break;
                  default:
                    p = l.json();
                    break;
                }
                p
                  ? p.then(
                      function (R) {
                        const O = {
                          data: R,
                          status: l.status,
                          statusText: l.statusText,
                          headers: m,
                          config: f,
                          request: r,
                          requestHeaders: x,
                        };
                        (0, Hg.default)(h, d, O);
                      },
                      function (R) {
                        d(
                          R ||
                            (0, Ft.default)(
                              "Stream decode error",
                              f,
                              l.statusText,
                              r,
                              l
                            )
                        );
                      }
                    )
                  : d(
                      (0, Ft.default)(
                        "Failed to resolve response stream.",
                        f,
                        "STREAM_FAILED",
                        r,
                        l
                      )
                    );
              } else
                l.status >= 500
                  ? d(
                      (0, Ft.default)(
                        "Server-side error: " + l.status + " / " + l.statusText,
                        f,
                        l.statusText,
                        r,
                        l
                      )
                    )
                  : l.status >= 400
                  ? d(
                      (0, Ft.default)(
                        "Client-side error: " + l.status + " / " + l.statusText,
                        f,
                        l.statusText,
                        r,
                        l
                      )
                    )
                  : d((0, Ft.default)("Unknown error", f, l.statusText, r, l));
            },
            function (l) {
              l instanceof Error
                ? d((0, Ft.default)(l.message, f, null, r, l))
                : d((0, Ft.default)("Network Error", f, null, r, l));
            }
          );
      })
    );
  }
  sendRequest(f) {
    return _s(this, void 0, void 0, function* () {
      const h =
          f.disabledLiteAppAutoLogin || this.options.disabledLiteAppAutoLogin,
        d = f.isSessionInvalid || this.options.isSessionInvalid;
      if (h || typeof d != "function") return this.doRequest(f);
      try {
        const U = yield this.doRequest(f);
        return d(U.data, f)
          ? ((!gs || gs.getTime() + Kg < Date.now()) &&
              ((zr = void 0), (gs = new Date())),
            this.doRequest(f))
          : U;
      } catch (U) {
        return Promise.reject(U);
      }
    });
  }
}
jt.LiteAppAdapter = Bs;
class Xg extends Bs {
  constructor(f) {
    super(
      Object.assign(
        {
          isSessionInvalid: Vg.isLoginError,
        },
        f
      )
    );
  }
}
jt.FinderLiteAppAdapter = Xg;
jt.default = new Bs({});
var Is = {},
  qr =
    (H && H.__importDefault) ||
    function (g) {
      return g && g.__esModule
        ? g
        : {
            default: g,
          };
    };
Object.defineProperty(Is, "__esModule", {
  value: !0,
});
const jg = qr(Su),
  Jg = qr(bu()),
  Zg = qr(wu),
  Qg = qr(Us()),
  _u = As,
  ev = console.warn;
function tv(g) {
  const f = wx.request.bind(wx);
  return new Promise((h, d) => {
    let U;
    const x = g.data,
      A = g.headers,
      M = {
        method: (g.method && g.method.toUpperCase()) || "GET",
        url: (0, Zg.default)(
          (0, Qg.default)(g.baseURL, g.url),
          g.params,
          g.paramsSerializer
        ),
        success: (B) => {
          const w = (0, _u.transformResponse)(B, g, M);
          (0, Jg.default)(h, d, w);
        },
        fail: (B) => {
          (0, _u.transformError)(B, d, g);
        },
        complete() {
          U = void 0;
        },
      };
    g.timeout !== 0 &&
      ev(
        'The "timeout" option is not supported by miniprogram. For more information about usage see "https://developers.weixin.qq.com/miniprogram/dev/framework/config.html#全局配置"'
      ),
      jg.default.forEach(A, function (w, k) {
        const e = k.toLowerCase();
        ((typeof x > "u" && e === "content-type") || e === "referer") &&
          delete A[k];
      }),
      (M.header = A),
      g.responseType && (M.responseType = g.responseType),
      g.cancelToken &&
        g.cancelToken.promise.then(function (w) {
          U && (U.abort(), d(w), (U = void 0));
        }),
      x !== void 0 && (M.data = x),
      (U = f(M));
  });
}
Is.default = tv;
var Es = {};
const iv = wg(Eg);
(function (g) {
  var f =
    (H && H.__importDefault) ||
    function (p) {
      return p && p.__esModule
        ? p
        : {
            default: p,
          };
    };
  Object.defineProperty(g, "__esModule", {
    value: !0,
  }),
    (g.isCancel =
      g.isRpcRequestXError =
      g.isLoginRequestXError =
      g.isNetworkRequestXError =
      g.isRequestXError =
      g.generateFinderErrorFromResponse =
      g.generateErrorFromResponse =
      g.RpcRequestXError =
      g.LoginRequestXError =
      g.NetworkRequestXError =
      g.RequestXError =
      g.NetworkRequestXErrorType =
        void 0);
  const h = iv,
    d = f(xs),
    U = Tt,
    x = f(Us());
  var A;
  (function (p) {
    (p.Network = "Network"),
      (p.Timeout = "Timeout"),
      (p.Abort = "Abort"),
      (p.HttpStatus = "HttpStatus"),
      (p.WebTransferJsApi = "WebTransferJsApi");
  })((A = g.NetworkRequestXErrorType || (g.NetworkRequestXErrorType = {})));
  class o extends h.RequestError {
    constructor(S, R, O) {
      var P, y;
      let F = "";
      O.config && (F = (0, x.default)(O.config.baseURL, O.config.url)),
        super(S, F, {
          errCode: R,
        }),
        (this.config = O.config),
        (this.name = "RequestXError"),
        (this.resp = O.response),
        (this.stack = O.stack),
        (this.rid = (P = O.config) === null || P === void 0 ? void 0 : P.rid),
        (this.displayTitle =
          (y = O.displayTitle) !== null && y !== void 0 ? y : ""),
        (this.displayContent = O.displayContent || "请求异常");
    }
    get merlin() {
      return {
        idx1: this.url,
        idx2: this.errCode ? "".concat(this.errCode) : void 0,
        extra: JSON.stringify(this.config),
      };
    }
    formatErrorMessage(S) {
      var R;
      let O =
        (R = S != null ? S : this.displayContent) !== null && R !== void 0
          ? R
          : "";
      return (
        this.errCode &&
          !O.endsWith("".concat(this.errCode)) &&
          !O.endsWith("(".concat(this.errCode, ")")) &&
          (O += "(".concat(this.errCode, ")")),
        this.rid && (O += "\n".concat(this.rid)),
        O
      );
    }
  }
  g.RequestXError = o;
  class M extends o {
    constructor(S) {
      var R;
      super(
        S.message || "网络异常",
        S.code ||
          ((R = S.response) === null || R === void 0 ? void 0 : R.status),
        {
          config: S.config,
          response: S.response,
          displayContent: "网络异常",
        }
      ),
        (this.name = "NetworkRequestXError"),
        (this.isCancel = !!S.isCancel),
        S.message === "Network Error"
          ? (this.type = A.Network)
          : S.message.indexOf("timeout") !== -1
          ? (this.type = A.Timeout)
          : S.code === "ECONNABORTED" || this.isCancel
          ? (this.type = A.Abort)
          : S.message.includes("webTransfer")
          ? (this.type = A.WebTransferJsApi)
          : (this.type = A.HttpStatus);
    }
    get merlin() {
      return {
        idx1: this.url,
        idx2: this.errCode ? "".concat(this.errCode) : void 0,
        idx3: "".concat(this.type),
        extra: JSON.stringify(this.config),
      };
    }
  }
  g.NetworkRequestXError = M;
  class B extends o {
    constructor(S, R, O) {
      super(S, R, O), (this.name = "LoginRequestXError");
    }
    get merlin() {
      return {
        idx1: this.url,
        idx2: this.errCode ? "".concat(this.errCode) : void 0,
        extra: JSON.stringify(this.config),
      };
    }
  }
  g.LoginRequestXError = B;
  class w extends o {
    constructor(S, R, O, P) {
      super(S, R, O), (this.name = "RpcRequestXError"), (this.rpcName = P);
    }
    get merlin() {
      return {
        idx1: this.url,
        idx2: this.errCode ? "".concat(this.errCode) : void 0,
        idx3: this.rpcName,
        extra: JSON.stringify(this.config),
      };
    }
  }
  g.RpcRequestXError = w;
  function k(p) {
    if (d.default.isAxiosError(p) || d.default.isCancel(p)) return new M(p);
  }
  g.generateErrorFromResponse = k;
  function e(p, S) {
    var R;
    if (d.default.isAxiosError(p) || d.default.isCancel(p)) return new M(p);
    if (p instanceof Error || (p.message && p.stack)) {
      const O = p;
      return new o(O.message, void 0, {
        config: S,
        response: p,
        stack: O.stack,
      });
    }
    if ((0, U.isLoginError)(p))
      return new B(
        ((R = p.error) === null || R === void 0 ? void 0 : R.message) ||
          p.errMsg ||
          "登录态异常: ".concat((0, U.getLoginErrorCode)(p)),
        (0, U.getLoginErrorCode)(p),
        {
          config: S,
          response: p,
        }
      );
    if (
      p.data &&
      p.data.baseResp &&
      typeof p.data.baseResp.errcode == "number" &&
      p.data.baseResp.errcode !== 0
    )
      return new o(
        p.data.baseResp.errmsg ||
          "baseResp.errcode: ".concat(p.data.baseResp.errcode),
        p.data.baseResp.errcode,
        {
          config: S,
          response: p,
          displayTitle: p.data.baseResp.errtitle,
          displayContent: p.data.baseResp.errmsg,
        }
      );
    if (
      p.data &&
      p.data.base_resp &&
      typeof p.data.base_resp.errcode == "number" &&
      p.data.base_resp.errcode !== 0
    )
      return new o(
        p.data.base_resp.errmsg ||
          "baseResp.errcode: ".concat(p.data.base_resp.errcode),
        p.data.base_resp.errcode,
        {
          config: S,
          response: p,
          displayTitle: p.data.base_resp.errtitle,
          displayContent: p.data.base_resp.errmsg,
        }
      );
    if (p.error) {
      if (p.error.rpcResult) {
        const P = p.error.rpcResult.returnCode || p.error.code || p.errCode,
          y =
            p.error.rpcResult.serverName && p.error.rpcResult.methodName
              ? ""
                  .concat(p.error.rpcResult.serverName, "::")
                  .concat(p.error.rpcResult.methodName)
              : void 0;
        return new w(
          p.error.message || "RPC 请求错误".concat(P ? ": ".concat(P) : ""),
          p.error.rpcResult.returnCode || p.error.code || p.errCode,
          {
            config: S,
            response: p,
          },
          y
        );
      }
      const O = p.error.code || p.errCode;
      return new o(
        p.error.message ||
          "".concat(p.error.name || "请求错误").concat(O ? ": ".concat(O) : ""),
        p.error.code || p.errCode,
        {
          config: S,
          response: p,
        }
      );
    }
    if (typeof p.errCode == "number" && p.errCode !== 0)
      return new o(p.errMsg || "errCode: ".concat(p.errCode), p.errCode, {
        config: S,
        response: p,
      });
  }
  g.generateFinderErrorFromResponse = e;
  function r(p) {
    return p instanceof o;
  }
  g.isRequestXError = r;
  function n(p) {
    return p instanceof M;
  }
  g.isNetworkRequestXError = n;
  function a(p) {
    return p instanceof B;
  }
  g.isLoginRequestXError = a;
  function l(p) {
    return p instanceof w;
  }
  g.isRpcRequestXError = l;
  function m(p) {
    return n(p) && p.isCancel;
  }
  g.isCancel = m;
})(Es);
var Nt = {};
Object.defineProperty(Nt, "__esModule", {
  value: !0,
});
Nt.generateRid = Nt.generateHex = Nt.timestampToHex = void 0;
function Cu() {
  return Math.floor(Date.now() / 1e3).toString(16);
}
Nt.timestampToHex = Cu;
function Ru() {
  return [...Array(8)]
    .map(() => Math.floor(Math.random() * 16).toString(16))
    .join("");
}
Nt.generateHex = Ru;
function rv() {
  return "".concat(Cu(), "-").concat(Ru());
}
Nt.generateRid = rv;
var Yr = {};
Object.defineProperty(Yr, "__esModule", {
  value: !0,
});
Yr.getPageUrl = void 0;
const vs = Gr,
  nv = () => {
    var g, f, h, d, U;
    let x = "";
    try {
      return (
        vs.IS_LITE_APP
          ? (x =
              (f =
                (g = lite == null ? void 0 : lite.router) === null ||
                g === void 0
                  ? void 0
                  : g.currentPages[lite.router.currentPages.length - 1]) ===
                null || f === void 0
                ? void 0
                : f.path)
          : vs.IS_NATIVE_LIKE_APP || vs.IS_MINI_PROGRAM
          ? (x =
              (d =
                (h = getCurrentPages == null ? void 0 : getCurrentPages()) ===
                  null || h === void 0
                  ? void 0
                  : h[0]) === null || d === void 0
                ? void 0
                : d.route)
          : typeof location == "object" && (x = location.href),
        (U = x.replace(/\?.*$/, "")) !== null && U !== void 0 ? U : ""
      );
    } catch (A) {
      return "";
    }
  };
Yr.getPageUrl = nv;
(function (g) {
  var f =
      (H && H.__createBinding) ||
      (Object.create
        ? function (P, y, F, I) {
            I === void 0 && (I = F);
            var $ = Object.getOwnPropertyDescriptor(y, F);
            (!$ ||
              ("get" in $ ? !y.__esModule : $.writable || $.configurable)) &&
              ($ = {
                enumerable: !0,
                get: function () {
                  return y[F];
                },
              }),
              Object.defineProperty(P, I, $);
          }
        : function (P, y, F, I) {
            I === void 0 && (I = F), (P[I] = y[F]);
          }),
    h =
      (H && H.__exportStar) ||
      function (P, y) {
        for (var F in P)
          F !== "default" &&
            !Object.prototype.hasOwnProperty.call(y, F) &&
            f(y, P, F);
      },
    d =
      (H && H.__awaiter) ||
      function (P, y, F, I) {
        function $(J) {
          return J instanceof F
            ? J
            : new F(function (_e) {
                _e(J);
              });
        }
        return new (F || (F = Promise))(function (J, _e) {
          function de(we) {
            try {
              be(I.next(we));
            } catch (Ie) {
              _e(Ie);
            }
          }
          function Te(we) {
            try {
              be(I.throw(we));
            } catch (Ie) {
              _e(Ie);
            }
          }
          function be(we) {
            we.done ? J(we.value) : $(we.value).then(de, Te);
          }
          be((I = I.apply(P, y || [])).next());
        });
      },
    U =
      (H && H.__rest) ||
      function (P, y) {
        var F = {};
        for (var I in P)
          Object.prototype.hasOwnProperty.call(P, I) &&
            y.indexOf(I) < 0 &&
            (F[I] = P[I]);
        if (P != null && typeof Object.getOwnPropertySymbols == "function")
          for (
            var $ = 0, I = Object.getOwnPropertySymbols(P);
            $ < I.length;
            $++
          )
            y.indexOf(I[$]) < 0 &&
              Object.prototype.propertyIsEnumerable.call(P, I[$]) &&
              (F[I[$]] = P[I[$]]);
        return F;
      },
    x =
      (H && H.__importDefault) ||
      function (P) {
        return P && P.__esModule
          ? P
          : {
              default: P,
            };
      };
  Object.defineProperty(g, "__esModule", {
    value: !0,
  }),
    (g.RequestService = void 0);
  const A = x(xs),
    o = x(Ug),
    M = pi,
    B = qi,
    w = x(jt),
    k = x(Is),
    e = Gr,
    r = Es,
    n = Nt;
  h(Es, g);
  const a = Yr;
  function l(P) {
    return new Promise((y) => {
      setTimeout(() => {
        y(void 0);
      }, P);
    });
  }
  const m = 3,
    p = 0,
    S = (P, y, F) => F || !0,
    R = (P) => P instanceof r.NetworkRequestXError;
  class O {
    static setGlobalConfig(y) {
      this.globalConfig = Object.assign(
        Object.assign({}, this.globalConfig),
        y
      );
    }
    constructor(y) {
      (this.baseURL = ""),
        (this.axiosInstance = void 0),
        (this.validateResult = S),
        (this.onRetry = R),
        (this.formatConfig = (we) => we),
        (this.requestMiddles = []),
        (this.cancelTokenMap = new Map());
      const F = Object.assign(Object.assign({}, O.globalConfig), y),
        {
          baseURL: I,
          validateResult: $,
          onRetry: J,
          formatConfig: _e,
          requestMiddles: de,
          adapter: Te,
        } = F,
        be = U(F, [
          "baseURL",
          "validateResult",
          "onRetry",
          "formatConfig",
          "requestMiddles",
          "adapter",
        ]);
      typeof $ == "function" && (this.validateResult = $),
        typeof _e == "function" && (this.formatConfig = _e),
        typeof J == "function" && (this.onRetry = J),
        Array.isArray(de) && (this.requestMiddles = [...de]),
        (this.instanceConfig = Object.assign(Object.assign({}, be), {
          adapter: Te,
        })),
        (this.baseURL = I),
        (this.axiosInstance = A.default.create(
          Object.assign(
            {
              baseURL: I,
              adapter:
                Te instanceof B.BaseAdapter ? Te.sendRequest.bind(Te) : Te,
            },
            be
          )
        ));
    }
    getDefaultAdapter() {
      let y;
      return (
        e.IS_LITE_APP
          ? (y = w.default)
          : (e.IS_NATIVE_LIKE_APP || e.IS_MINI_PROGRAM) && (y = k.default),
        y
      );
    }
    initRequestConfig(y) {
      const F = Object.assign(Object.assign({}, this.instanceConfig), y);
      if (
        (F.retry &&
          ((F._retryCount = 0),
          F.retry.count || (F.retry.count = m),
          F.retry.delay || (F.retry.delay = p)),
        !F.adapter)
      ) {
        const I = this.getDefaultAdapter();
        I && (F.adapter = I);
      }
      return this.baseURL && !F.baseURL && (F.baseURL = this.baseURL), F;
    }
    addMiddleware(y) {
      this.requestMiddles.push(y);
    }
    getRequestMiddles(y) {
      const F = [...this.requestMiddles];
      return (
        y.enableRid !== !1 && F.unshift(this.addRidMiddleware.bind(this)),
        !y.disabledPassExportKey &&
          M.exportKey &&
          F.push(this.addExportKeyMiddleware.bind(this)),
        y.enablePassPageUrl !== !1 &&
          F.push(this.addPageUrlMiddleware.bind(this)),
        y.retry && F.push(this.retry.bind(this)),
        F
      );
    }
    request(y) {
      return d(this, void 0, void 0, function* () {
        y = this.initRequestConfig(y);
        const F = this.getRequestMiddles(y),
          I = ($) =>
            d(this, void 0, void 0, function* () {
              return F.length === 0
                ? yield this.doRequest($)
                : yield F.shift().bind(this)($, I);
            });
        return I(y);
      });
    }
    rawRequest(y) {
      return d(this, void 0, void 0, function* () {
        y = this.initRequestConfig(y);
        const F = this.getRequestMiddles(y),
          I = ($) =>
            d(this, void 0, void 0, function* () {
              return F.length === 0
                ? yield this.doRawRequest($)
                : yield F.shift().bind(this)($, I);
            });
        return I(y);
      });
    }
    addExportKeyMiddleware(y, F) {
      return d(this, void 0, void 0, function* () {
        return (
          (y.params = y.params || {}), (y.params.exportkey = M.exportKey), F(y)
        );
      });
    }
    addRidMiddleware(y, F) {
      return d(this, void 0, void 0, function* () {
        return (
          (y.rid = (0, n.generateRid)()),
          y.params || (y.params = {}),
          (y.params._rid = y.rid),
          F(y)
        );
      });
    }
    addPageUrlMiddleware(y, F) {
      return d(this, void 0, void 0, function* () {
        return (
          (y.params = y.params || {}),
          (y.params._pageUrl = (0, a.getPageUrl)()),
          F(y)
        );
      });
    }
    retry(y, F) {
      return d(this, void 0, void 0, function* () {
        try {
          y._retryCount || (y._retryCount = 0);
          try {
            return yield F(y);
          } catch (I) {
            if (y._retryCount >= y.retry.count) return Promise.reject(I);
            const $ = this.onRetry(I, y, {
              currentRetryCount: y._retryCount + 1,
            });
            return $
              ? (typeof $ != "boolean"
                  ? yield l($.delay || y.retry.count || 0)
                  : yield l(y.retry.count || 0),
                y._retryCount++,
                this.retry(y, F))
              : Promise.reject(I);
          }
        } catch (I) {
          return Promise.reject(I);
        }
      });
    }
    reportSuccess(y, F, I) {
      y.reportConfig &&
        y.reportConfig.success &&
        y.reportConfig.success(y, F, I);
    }
    reportFail(y, F, I) {
      y.reportConfig && y.reportConfig.fail && y.reportConfig.fail(y, F, I);
    }
    doRequest(y) {
      return d(this, void 0, void 0, function* () {
        const F = new Date().getTime();
        try {
          y.method || (y.method = "POST"),
            this.formatConfig && (y = this.formatConfig(y)),
            y.method.toLocaleLowerCase() === "get"
              ? (y.params = y.params || y.data || {})
              : (y.data = y.data || y.params || {});
          const I = yield this.axiosInstance.request(
              Object.assign(Object.assign({}, y), {
                adapter:
                  y.adapter instanceof B.BaseAdapter
                    ? y.adapter.sendRequest.bind(y.adapter)
                    : y.adapter,
              })
            ),
            $ = (0, r.generateErrorFromResponse)(I.data),
            J = this.validateResult(I.data, y, $),
            de = {
              durationMs: new Date().getTime() - F,
            };
          return J !== !0
            ? (this.reportFail(y, J, de), Promise.reject(J))
            : (this.reportSuccess(y, I.data, de), I.data);
        } catch (I) {
          const J = {
            durationMs: new Date().getTime() - F,
          };
          A.default.isCancel(I) &&
            ((I.isCancel = !0), (I = (0, o.default)(I, y, "ECONNABORTED")));
          const _e = (0, r.generateErrorFromResponse)(I),
            de = this.validateResult(I, y, _e);
          return de !== !0
            ? (this.reportFail(y, de, J), Promise.reject(de))
            : (this.reportSuccess(y, I, J), I);
        }
      });
    }
    doRawRequest(y) {
      return d(this, void 0, void 0, function* () {
        const F = new Date().getTime();
        try {
          y.method || (y.method = "POST"),
            this.formatConfig && (y = this.formatConfig(y)),
            y.method.toLocaleLowerCase() === "get"
              ? (y.params = y.params || y.data || {})
              : (y.data = y.data || y.params || {});
          const I = yield this.axiosInstance.request(
              Object.assign(Object.assign({}, y), {
                adapter:
                  y.adapter instanceof B.BaseAdapter
                    ? y.adapter.sendRequest.bind(y.adapter)
                    : y.adapter,
              })
            ),
            $ = (0, r.generateErrorFromResponse)(I.data),
            J = this.validateResult(I.data, y, $),
            de = {
              durationMs: new Date().getTime() - F,
            };
          return J !== !0
            ? (this.reportFail(y, J, de), Promise.reject(J))
            : (this.reportSuccess(y, I.data, de), I);
        } catch (I) {
          const J = {
            durationMs: new Date().getTime() - F,
          };
          A.default.isCancel(I) &&
            ((I.isCancel = !0), (I = (0, o.default)(I, y, "ECONNABORTED")));
          const _e = (0, r.generateErrorFromResponse)(I),
            de = this.validateResult(I, y, _e);
          return de !== !0
            ? (this.reportFail(y, de, J), Promise.reject(de))
            : (this.reportSuccess(y, I, J), I);
        }
      });
    }
    get(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "GET"), yield this.request(y);
      });
    }
    rawGet(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "GET"), yield this.rawRequest(y);
      });
    }
    post(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "POST"), yield this.request(y);
      });
    }
    rawPost(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "POST"), yield this.rawRequest(y);
      });
    }
    delete(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "DELETE"), yield this.request(y);
      });
    }
    rawDelete(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "DELETE"), yield this.rawRequest(y);
      });
    }
    head(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "HEAD"), yield this.request(y);
      });
    }
    rawHead(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "HEAD"), yield this.rawRequest(y);
      });
    }
    options(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "OPTIONS"), yield this.request(y);
      });
    }
    rawOptions(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "OPTIONS"), yield this.rawRequest(y);
      });
    }
    put(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "PUT"), yield this.request(y);
      });
    }
    rawPut(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "PUT"), yield this.rawRequest(y);
      });
    }
    patch(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "PATCH"), yield this.request(y);
      });
    }
    rawPatch(y) {
      return d(this, void 0, void 0, function* () {
        return (y.method = "PATCH"), yield this.rawRequest(y);
      });
    }
  }
  (g.RequestService = O), (O.globalConfig = {});
})(Rs);
var Nr =
    (H && H.__awaiter) ||
    function (g, f, h, d) {
      function U(x) {
        return x instanceof h
          ? x
          : new h(function (A) {
              A(x);
            });
      }
      return new (h || (h = Promise))(function (x, A) {
        function o(w) {
          try {
            B(d.next(w));
          } catch (k) {
            A(k);
          }
        }
        function M(w) {
          try {
            B(d.throw(w));
          } catch (k) {
            A(k);
          }
        }
        function B(w) {
          w.done ? x(w.value) : U(w.value).then(o, M);
        }
        B((d = d.apply(g, f || [])).next());
      });
    },
  Au =
    (H && H.__importDefault) ||
    function (g) {
      return g && g.__esModule
        ? g
        : {
            default: g,
          };
    };
Object.defineProperty(Cs, "__esModule", {
  value: !0,
});
const sv = Re,
  ov = Au(Gi),
  av = Rs,
  Bu = Au(xs),
  { CancelToken: uv } = Bu.default;
class fv {
  constructor(f) {
    (this.abortControllerList = []),
      (this.defaultTimeout = 6 * 1e3),
      (this.abortControllerList = []),
      (this.config = f || {}),
      (this.fallbackHostConfig =
        (f == null ? void 0 : f.fallbackHostConfig) || {}),
      (this.reqService = new av.RequestService({
        enableRid: !1,
        enablePassPageUrl: !1,
        onRetry: (h, d, U) => {
          var x;
          if (
            (setTimeout(() => {
              this.config.onRetryCallback &&
                this.config.onRetryCallback(d, U.currentRetryCount);
            }, 0),
            this.fallbackHost &&
              U.currentRetryCount === this.retryTimesToSwitchHost)
          ) {
            d.url = d.url || "";
            const A = (0, sv.getHostFromUrl)(d.url);
            if (
              A &&
              !(
                !((x = d.url) === null || x === void 0) &&
                x.includes(this.fallbackHost)
              )
            ) {
              const o = d.url.replace(A, this.fallbackHost);
              d.url = o;
            }
          }
          return {
            delay: 0,
          };
        },
      }));
  }
  get fallbackHost() {
    var f;
    return (
      ((f = this.fallbackHostConfig) === null || f === void 0
        ? void 0
        : f.fallbackHost) || ""
    );
  }
  get retryTimesToSwitchHost() {
    var f;
    return (
      ((f = this.fallbackHostConfig) === null || f === void 0
        ? void 0
        : f.retryTimesToSwitchHost) || 1
    );
  }
  updateFallbackHostConfig(f) {
    (this.fallbackHostConfig = this.fallbackHostConfig || {}),
      (this.fallbackHostConfig = Object.assign(
        {},
        this.fallbackHostConfig,
        f || {}
      ));
  }
  timeoutPromise(f, h) {
    return new Promise((d) => {
      setTimeout(() => {
        d(h);
      }, f);
    });
  }
  _raceFetch(f, h, d, U) {
    return Nr(this, void 0, void 0, function* () {
      const x = d || this.defaultTimeout;
      let A = U;
      typeof A != "number" && (A = 3);
      try {
        return yield fetch(f, h);
      } catch (o) {
        if (((A = A - 1), A > 0))
          try {
            return yield this._raceFetch(f, h, x, A);
          } catch (M) {
            throw M;
          }
        else
          throw (
            ((o.message = "[raceFetch]".concat(o.message, ", url: ").concat(f)),
            o)
          );
      }
    });
  }
  getHeaders(f) {
    return Nr(this, void 0, void 0, function* () {
      const h = yield this._raceFetch(
        f,
        {
          method: "HEAD",
        },
        this.defaultTimeout,
        2
      );
      return h ? h.headers : null;
    });
  }
  getContentLen(f) {
    return Nr(this, void 0, void 0, function* () {
      const h = yield this.getHeaders(f);
      if (!h) return null;
      let d = null;
      const U = h.get("content-length") || h.get("CONTENT-LENGTH");
      if (U) d = Number(U);
      else {
        const x = h.get("content-range") || h.get("CONTENT-RANGE");
        if (typeof x == "string" && x.includes("/")) {
          const A = x.split("/")[1];
          d = Number(A);
        }
      }
      if (typeof d != "number") throw new ov.default("getContentLen fail");
      return d;
    });
  }
  downloadRangeBuffer(f) {
    var h;
    return Nr(this, void 0, void 0, function* () {
      const { url: d, end: U } = f;
      let x = f.start || 0;
      const A = f.timeout || this.defaultTimeout,
        o = U ? "bytes=".concat(x, "-").concat(U) : "bytes=".concat(x, "-");
      try {
        const M = uv.source();
        this.abortControllerList.push(M);
        const B = yield (h = this.reqService) === null || h === void 0
            ? void 0
            : h.rawGet({
                cancelToken: M.token,
                url: d,
                retry: {
                  count: 3,
                },
                timeout: A,
                headers: {
                  range: o,
                },
                responseType: "arraybuffer",
                disabledPassExportKey: !0,
              }),
          w = (B == null ? void 0 : B.headers) || {},
          k = B == null ? void 0 : B.data,
          e = new Uint8Array(k);
        return {
          respHeaders: w,
          data: e,
        };
      } catch (M) {
        if (Bu.default.isCancel(M) || M.name === "AbortError") return null;
        throw M;
      }
    });
  }
  dispose() {
    if (this.abortControllerList) {
      for (let f = 0; f < this.abortControllerList.length; f++) {
        const h = this.abortControllerList[f];
        try {
          h.cancel();
        } catch (d) {
          continue;
        }
      }
      this.abortControllerList = [];
    }
    (this.reqService = void 0), (this.config = {});
  }
}
Cs.default = fv;
var Iu = {};
(function (g) {
  Object.defineProperty(g, "__esModule", {
    value: !0,
  }),
    (g.SKIP_BUFFER_RANGE_START = g.SKIP_BUFFER_HOLE_STEP_SECONDS = void 0);
  const f = Ee,
    h = Re;
  (g.SKIP_BUFFER_HOLE_STEP_SECONDS = 0.1), (g.SKIP_BUFFER_RANGE_START = 0.05);
  class d {
    constructor(x) {
      (this.disposed = !1),
        (this.lastCurrentTime = 0),
        (this.hasReportStalled = !1),
        (this.alarmCount = 0),
        (this.lastStalledData = null),
        (this.config = Object.assign({}, x)),
        (this.mediaElement = this.config.mediaElement),
        this.initConfig();
    }
    startChecker() {
      if (this.disposed) {
        this.clearChecker();
        return;
      }
      this.checker ||
        (this.checker = setInterval(() => {
          this.check();
        }, this.config.checkIntervalTime));
    }
    stopChecker() {
      this.clearChecker();
    }
    dispose() {
      (this.disposed = !0),
        this.resetCheckData(),
        (this.mediaElement = null),
        this.clearChecker();
    }
    initConfig() {
      (this.config = this.config || {}),
        (this.config.maxBufferHole = this.config.maxBufferHole || 0.1),
        (this.config.checkIntervalTime = this.config.checkIntervalTime || 1e3),
        (this.config.alarmTimes = this.config.alarmTimes || 2);
    }
    isVideoCanNotPlay() {
      return !!(
        !this.mediaElement ||
        this.mediaElement.paused ||
        this.mediaElement.ended ||
        this.mediaElement.playbackRate === 0 ||
        this.mediaElement.buffered.length === 0
      );
    }
    searchTargetBuffered(x) {
      if (!this.mediaElement) return -1;
      const { buffered: A } = this.mediaElement;
      if (A.length <= 0) return -1;
      for (let o = 0; o < A.length; o++) {
        const M = A.start(o),
          B = A.end(o);
        if (x >= M && x <= B) return o;
      }
      return -1;
    }
    trySkipBufferHole() {
      var x;
      if (!this.mediaElement) return;
      const { config: A } = this,
        o =
          (x = this.mediaElement) === null || x === void 0
            ? void 0
            : x.buffered,
        { currentTime: M } = this.mediaElement;
      for (let B = 0; B < o.length; B++) {
        const w = o.start(B);
        if (M + A.maxBufferHole >= w && M < w) {
          const k = Math.max(
            w + g.SKIP_BUFFER_RANGE_START,
            this.mediaElement.currentTime + g.SKIP_BUFFER_HOLE_STEP_SECONDS
          );
          return (this.mediaElement.currentTime = k), k;
        }
      }
    }
    genStalledData(x) {
      var A, o, M;
      return {
        currentTime:
          (A = this.mediaElement) === null || A === void 0
            ? void 0
            : A.currentTime,
        buffered:
          (o = this.mediaElement) === null || o === void 0
            ? void 0
            : o.buffered,
        checkIntervalTime: this.config.checkIntervalTime,
        duration:
          ((M = this.mediaElement) === null || M === void 0
            ? void 0
            : M.duration) || 0,
        errorType: x,
      };
    }
    fixCurrentTimeInBuffered() {
      if (!this.mediaElement) return !1;
      const { currentTime: x } = this.mediaElement,
        { buffered: A } = this.mediaElement,
        { duration: o } = this.mediaElement;
      if (x >= o) return !1;
      if (typeof x == "number" && A.length > 0 && typeof o == "number") {
        if (this.searchTargetBuffered(x) === -1) return !1;
        const B = 1;
        return (
          x + B < o
            ? (this.mediaElement.currentTime = Math.floor(x + B))
            : (this.mediaElement.currentTime = o),
          !0
        );
      }
      return !1;
    }
    fixCurrentTimeLargerThanDuration() {
      if (!this.mediaElement) return !1;
      const { currentTime: x } = this.mediaElement,
        { duration: A } = this.mediaElement;
      return typeof x == "number" && typeof A == "number" && x > A
        ? (A > 1
            ? (this.mediaElement.currentTime = A - 1)
            : (this.mediaElement.currentTime = Math.floor(A)),
          !0)
        : !1;
    }
    resetCheckData() {
      (this.hasReportStalled = !1),
        (this.alarmCount = 0),
        (this.lastStalledData = null),
        (this.lastCurrentTime = 0);
    }
    check() {
      var x, A, o, M, B;
      if (this.disposed || this.isVideoCanNotPlay()) return;
      const w =
        (x = this.mediaElement) === null || x === void 0
          ? void 0
          : x.currentTime;
      if (this.lastCurrentTime !== w) {
        this.resetCheckData(), (this.lastCurrentTime = w);
        return;
      }
      if (this.alarmCount < this.config.alarmTimes) {
        this.alarmCount = this.alarmCount + 1;
        return;
      }
      if (this.hasReportStalled) return;
      if (this.lastStalledData) {
        this.hasReportStalled = !0;
        const r = this.lastStalledData || {};
        (r.bufferedList = (0, h.getBufferedRanges)(this.mediaElement)),
          (o = (A = this.config).playingStalledCallback) === null ||
            o === void 0 ||
            o.call(A, r);
        return;
      }
      let k = f.STALLED_ERROR_TYPE.unknown;
      const e = this.genStalledData(k);
      try {
        this.fixCurrentTimeLargerThanDuration()
          ? (k = f.STALLED_ERROR_TYPE.currentTimeOutOfBounds)
          : this.fixCurrentTimeInBuffered()
          ? (k = f.STALLED_ERROR_TYPE.currentTimeInBuffered)
          : typeof this.trySkipBufferHole() == "number" &&
            (k = f.STALLED_ERROR_TYPE.bufferHole),
          typeof ((M = this.mediaElement) === null || M === void 0
            ? void 0
            : M.currentTime) == "number" &&
            (this.lastCurrentTime =
              (B = this.mediaElement) === null || B === void 0
                ? void 0
                : B.currentTime);
      } finally {
        (e.errorType = k), (this.lastStalledData = e);
      }
    }
    clearChecker() {
      this.checker && clearInterval(this.checker), (this.checker = null);
    }
  }
  g.default = d;
})(Iu);
var Ls = {},
  Hr = {
    exports: {},
  };
/**
 * @license
 * Lodash <https://lodash.com/>
 * Copyright OpenJS Foundation and other contributors <https://openjsf.org/>
 * Released under MIT license <https://lodash.com/license>
 * Based on Underscore.js 1.8.3 <http://underscorejs.org/LICENSE>
 * Copyright Jeremy Ashkenas, DocumentCloud and Investigative Reporters & Editors
 */
Hr.exports;
(function (g, f) {
  (function () {
    var h,
      d = "4.17.21",
      U = 200,
      x = "Unsupported core-js use. Try https://npms.io/search?q=ponyfill.",
      A = "Expected a function",
      o = "Invalid `variable` option passed into `_.template`",
      M = "__lodash_hash_undefined__",
      B = 500,
      w = "__lodash_placeholder__",
      k = 1,
      e = 2,
      r = 4,
      n = 1,
      a = 2,
      l = 1,
      m = 2,
      p = 4,
      S = 8,
      R = 16,
      O = 32,
      P = 64,
      y = 128,
      F = 256,
      I = 512,
      $ = 30,
      J = "...",
      _e = 800,
      de = 16,
      Te = 1,
      be = 2,
      we = 3,
      Ie = 1 / 0,
      Ke = 9007199254740991,
      ft = 17976931348623157e292,
      lt = NaN,
      j = 4294967295,
      Ou = j - 1,
      Pu = j >>> 1,
      ku = [
        ["ary", y],
        ["bind", l],
        ["bindKey", m],
        ["curry", S],
        ["curryRight", R],
        ["flip", I],
        ["partial", O],
        ["partialRight", P],
        ["rearg", F],
      ],
      Zt = "[object Arguments]",
      Vi = "[object Array]",
      Fu = "[object AsyncFunction]",
      _i = "[object Boolean]",
      gi = "[object Date]",
      Du = "[object DOMException]",
      Ki = "[object Error]",
      Xi = "[object Function]",
      Fs = "[object GeneratorFunction]",
      rt = "[object Map]",
      vi = "[object Number]",
      Mu = "[object Null]",
      gt = "[object Object]",
      Ds = "[object Promise]",
      zu = "[object Proxy]",
      mi = "[object RegExp]",
      nt = "[object Set]",
      yi = "[object String]",
      ji = "[object Symbol]",
      Nu = "[object Undefined]",
      Si = "[object WeakMap]",
      Wu = "[object WeakSet]",
      bi = "[object ArrayBuffer]",
      Qt = "[object DataView]",
      $r = "[object Float32Array]",
      Vr = "[object Float64Array]",
      Kr = "[object Int8Array]",
      Xr = "[object Int16Array]",
      jr = "[object Int32Array]",
      Jr = "[object Uint8Array]",
      Zr = "[object Uint8ClampedArray]",
      Qr = "[object Uint16Array]",
      en = "[object Uint32Array]",
      Hu = /\b__p \+= '';/g,
      Gu = /\b(__p \+=) '' \+/g,
      qu = /(__e\(.*?\)|\b__t\)) \+\n'';/g,
      Ms = /&(?:amp|lt|gt|quot|#39);/g,
      zs = /[&<>"']/g,
      Yu = RegExp(Ms.source),
      $u = RegExp(zs.source),
      Vu = /<%-([\s\S]+?)%>/g,
      Ku = /<%([\s\S]+?)%>/g,
      Ns = /<%=([\s\S]+?)%>/g,
      Xu = /\.|\[(?:[^[\]]*|(["'])(?:(?!\1)[^\\]|\\.)*?\1)\]/,
      ju = /^\w*$/,
      Ju =
        /[^.[\]]+|\[(?:(-?\d+(?:\.\d+)?)|(["'])((?:(?!\2)[^\\]|\\.)*?)\2)\]|(?=(?:\.|\[\])(?:\.|\[\]|$))/g,
      tn = /[\\^$.*+?()[\]{}|]/g,
      Zu = RegExp(tn.source),
      rn = /^\s+/,
      Qu = /\s/,
      ef = /\{(?:\n\/\* \[wrapped with .+\] \*\/)?\n?/,
      tf = /\{\n\/\* \[wrapped with (.+)\] \*/,
      rf = /,? & /,
      nf = /[^\x00-\x2f\x3a-\x40\x5b-\x60\x7b-\x7f]+/g,
      sf = /[()=,{}\[\]\/\s]/,
      of = /\\(\\)?/g,
      af = /\$\{([^\\}]*(?:\\.[^\\}]*)*)\}/g,
      Ws = /\w*$/,
      uf = /^[-+]0x[0-9a-f]+$/i,
      ff = /^0b[01]+$/i,
      lf = /^\[object .+?Constructor\]$/,
      hf = /^0o[0-7]+$/i,
      df = /^(?:0|[1-9]\d*)$/,
      cf = /[\xc0-\xd6\xd8-\xf6\xf8-\xff\u0100-\u017f]/g,
      Ji = /($^)/,
      pf = /['\n\r\u2028\u2029\\]/g,
      Zi = "\\ud800-\\udfff",
      _f = "\\u0300-\\u036f",
      gf = "\\ufe20-\\ufe2f",
      vf = "\\u20d0-\\u20ff",
      Hs = _f + gf + vf,
      Gs = "\\u2700-\\u27bf",
      qs = "a-z\\xdf-\\xf6\\xf8-\\xff",
      mf = "\\xac\\xb1\\xd7\\xf7",
      yf = "\\x00-\\x2f\\x3a-\\x40\\x5b-\\x60\\x7b-\\xbf",
      Sf = "\\u2000-\\u206f",
      bf =
        " \\t\\x0b\\f\\xa0\\ufeff\\n\\r\\u2028\\u2029\\u1680\\u180e\\u2000\\u2001\\u2002\\u2003\\u2004\\u2005\\u2006\\u2007\\u2008\\u2009\\u200a\\u202f\\u205f\\u3000",
      Ys = "A-Z\\xc0-\\xd6\\xd8-\\xde",
      $s = "\\ufe0e\\ufe0f",
      Vs = mf + yf + Sf + bf,
      nn = "['’]",
      wf = "[" + Zi + "]",
      Ks = "[" + Vs + "]",
      Qi = "[" + Hs + "]",
      Xs = "\\d+",
      Ef = "[" + Gs + "]",
      js = "[" + qs + "]",
      Js = "[^" + Zi + Vs + Xs + Gs + qs + Ys + "]",
      sn = "\\ud83c[\\udffb-\\udfff]",
      Uf = "(?:" + Qi + "|" + sn + ")",
      Zs = "[^" + Zi + "]",
      on = "(?:\\ud83c[\\udde6-\\uddff]){2}",
      an = "[\\ud800-\\udbff][\\udc00-\\udfff]",
      ei = "[" + Ys + "]",
      Qs = "\\u200d",
      eo = "(?:" + js + "|" + Js + ")",
      xf = "(?:" + ei + "|" + Js + ")",
      to = "(?:" + nn + "(?:d|ll|m|re|s|t|ve))?",
      io = "(?:" + nn + "(?:D|LL|M|RE|S|T|VE))?",
      ro = Uf + "?",
      no = "[" + $s + "]?",
      Tf = "(?:" + Qs + "(?:" + [Zs, on, an].join("|") + ")" + no + ro + ")*",
      Cf = "\\d*(?:1st|2nd|3rd|(?![123])\\dth)(?=\\b|[A-Z_])",
      Rf = "\\d*(?:1ST|2ND|3RD|(?![123])\\dTH)(?=\\b|[a-z_])",
      so = no + ro + Tf,
      Af = "(?:" + [Ef, on, an].join("|") + ")" + so,
      Bf = "(?:" + [Zs + Qi + "?", Qi, on, an, wf].join("|") + ")",
      If = RegExp(nn, "g"),
      Lf = RegExp(Qi, "g"),
      un = RegExp(sn + "(?=" + sn + ")|" + Bf + so, "g"),
      Of = RegExp(
        [
          ei + "?" + js + "+" + to + "(?=" + [Ks, ei, "$"].join("|") + ")",
          xf + "+" + io + "(?=" + [Ks, ei + eo, "$"].join("|") + ")",
          ei + "?" + eo + "+" + to,
          ei + "+" + io,
          Rf,
          Cf,
          Xs,
          Af,
        ].join("|"),
        "g"
      ),
      Pf = RegExp("[" + Qs + Zi + Hs + $s + "]"),
      kf = /[a-z][A-Z]|[A-Z]{2}[a-z]|[0-9][a-zA-Z]|[a-zA-Z][0-9]|[^a-zA-Z0-9 ]/,
      Ff = [
        "Array",
        "Buffer",
        "DataView",
        "Date",
        "Error",
        "Float32Array",
        "Float64Array",
        "Function",
        "Int8Array",
        "Int16Array",
        "Int32Array",
        "Map",
        "Math",
        "Object",
        "Promise",
        "RegExp",
        "Set",
        "String",
        "Symbol",
        "TypeError",
        "Uint8Array",
        "Uint8ClampedArray",
        "Uint16Array",
        "Uint32Array",
        "WeakMap",
        "_",
        "clearTimeout",
        "isFinite",
        "parseInt",
        "setTimeout",
      ],
      Df = -1,
      pe = {};
    (pe[$r] =
      pe[Vr] =
      pe[Kr] =
      pe[Xr] =
      pe[jr] =
      pe[Jr] =
      pe[Zr] =
      pe[Qr] =
      pe[en] =
        !0),
      (pe[Zt] =
        pe[Vi] =
        pe[bi] =
        pe[_i] =
        pe[Qt] =
        pe[gi] =
        pe[Ki] =
        pe[Xi] =
        pe[rt] =
        pe[vi] =
        pe[gt] =
        pe[mi] =
        pe[nt] =
        pe[yi] =
        pe[Si] =
          !1);
    var ce = {};
    (ce[Zt] =
      ce[Vi] =
      ce[bi] =
      ce[Qt] =
      ce[_i] =
      ce[gi] =
      ce[$r] =
      ce[Vr] =
      ce[Kr] =
      ce[Xr] =
      ce[jr] =
      ce[rt] =
      ce[vi] =
      ce[gt] =
      ce[mi] =
      ce[nt] =
      ce[yi] =
      ce[ji] =
      ce[Jr] =
      ce[Zr] =
      ce[Qr] =
      ce[en] =
        !0),
      (ce[Ki] = ce[Xi] = ce[Si] = !1);
    var Mf = {
        À: "A",
        Á: "A",
        Â: "A",
        Ã: "A",
        Ä: "A",
        Å: "A",
        à: "a",
        á: "a",
        â: "a",
        ã: "a",
        ä: "a",
        å: "a",
        Ç: "C",
        ç: "c",
        Ð: "D",
        ð: "d",
        È: "E",
        É: "E",
        Ê: "E",
        Ë: "E",
        è: "e",
        é: "e",
        ê: "e",
        ë: "e",
        Ì: "I",
        Í: "I",
        Î: "I",
        Ï: "I",
        ì: "i",
        í: "i",
        î: "i",
        ï: "i",
        Ñ: "N",
        ñ: "n",
        Ò: "O",
        Ó: "O",
        Ô: "O",
        Õ: "O",
        Ö: "O",
        Ø: "O",
        ò: "o",
        ó: "o",
        ô: "o",
        õ: "o",
        ö: "o",
        ø: "o",
        Ù: "U",
        Ú: "U",
        Û: "U",
        Ü: "U",
        ù: "u",
        ú: "u",
        û: "u",
        ü: "u",
        Ý: "Y",
        ý: "y",
        ÿ: "y",
        Æ: "Ae",
        æ: "ae",
        Þ: "Th",
        þ: "th",
        ß: "ss",
        Ā: "A",
        Ă: "A",
        Ą: "A",
        ā: "a",
        ă: "a",
        ą: "a",
        Ć: "C",
        Ĉ: "C",
        Ċ: "C",
        Č: "C",
        ć: "c",
        ĉ: "c",
        ċ: "c",
        č: "c",
        Ď: "D",
        Đ: "D",
        ď: "d",
        đ: "d",
        Ē: "E",
        Ĕ: "E",
        Ė: "E",
        Ę: "E",
        Ě: "E",
        ē: "e",
        ĕ: "e",
        ė: "e",
        ę: "e",
        ě: "e",
        Ĝ: "G",
        Ğ: "G",
        Ġ: "G",
        Ģ: "G",
        ĝ: "g",
        ğ: "g",
        ġ: "g",
        ģ: "g",
        Ĥ: "H",
        Ħ: "H",
        ĥ: "h",
        ħ: "h",
        Ĩ: "I",
        Ī: "I",
        Ĭ: "I",
        Į: "I",
        İ: "I",
        ĩ: "i",
        ī: "i",
        ĭ: "i",
        į: "i",
        ı: "i",
        Ĵ: "J",
        ĵ: "j",
        Ķ: "K",
        ķ: "k",
        ĸ: "k",
        Ĺ: "L",
        Ļ: "L",
        Ľ: "L",
        Ŀ: "L",
        Ł: "L",
        ĺ: "l",
        ļ: "l",
        ľ: "l",
        ŀ: "l",
        ł: "l",
        Ń: "N",
        Ņ: "N",
        Ň: "N",
        Ŋ: "N",
        ń: "n",
        ņ: "n",
        ň: "n",
        ŋ: "n",
        Ō: "O",
        Ŏ: "O",
        Ő: "O",
        ō: "o",
        ŏ: "o",
        ő: "o",
        Ŕ: "R",
        Ŗ: "R",
        Ř: "R",
        ŕ: "r",
        ŗ: "r",
        ř: "r",
        Ś: "S",
        Ŝ: "S",
        Ş: "S",
        Š: "S",
        ś: "s",
        ŝ: "s",
        ş: "s",
        š: "s",
        Ţ: "T",
        Ť: "T",
        Ŧ: "T",
        ţ: "t",
        ť: "t",
        ŧ: "t",
        Ũ: "U",
        Ū: "U",
        Ŭ: "U",
        Ů: "U",
        Ű: "U",
        Ų: "U",
        ũ: "u",
        ū: "u",
        ŭ: "u",
        ů: "u",
        ű: "u",
        ų: "u",
        Ŵ: "W",
        ŵ: "w",
        Ŷ: "Y",
        ŷ: "y",
        Ÿ: "Y",
        Ź: "Z",
        Ż: "Z",
        Ž: "Z",
        ź: "z",
        ż: "z",
        ž: "z",
        Ĳ: "IJ",
        ĳ: "ij",
        Œ: "Oe",
        œ: "oe",
        ŉ: "'n",
        ſ: "s",
      },
      zf = {
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
      },
      Nf = {
        "&amp;": "&",
        "&lt;": "<",
        "&gt;": ">",
        "&quot;": '"',
        "&#39;": "'",
      },
      Wf = {
        "\\": "\\",
        "'": "'",
        "\n": "n",
        "\r": "r",
        "\u2028": "u2028",
        "\u2029": "u2029",
      },
      Hf = parseFloat,
      Gf = parseInt,
      oo = typeof H == "object" && H && H.Object === Object && H,
      qf = typeof self == "object" && self && self.Object === Object && self,
      Ae = oo || qf || Function("return this")(),
      fn = f && !f.nodeType && f,
      Wt = fn && !0 && g && !g.nodeType && g,
      ao = Wt && Wt.exports === fn,
      ln = ao && oo.process,
      Xe = (function () {
        try {
          var T = Wt && Wt.require && Wt.require("util").types;
          return T || (ln && ln.binding && ln.binding("util"));
        } catch (D) {}
      })(),
      uo = Xe && Xe.isArrayBuffer,
      fo = Xe && Xe.isDate,
      lo = Xe && Xe.isMap,
      ho = Xe && Xe.isRegExp,
      co = Xe && Xe.isSet,
      po = Xe && Xe.isTypedArray;
    function He(T, D, L) {
      switch (L.length) {
        case 0:
          return T.call(D);
        case 1:
          return T.call(D, L[0]);
        case 2:
          return T.call(D, L[0], L[1]);
        case 3:
          return T.call(D, L[0], L[1], L[2]);
      }
      return T.apply(D, L);
    }
    function Yf(T, D, L, q) {
      for (var Z = -1, ae = T == null ? 0 : T.length; ++Z < ae; ) {
        var Ue = T[Z];
        D(q, Ue, L(Ue), T);
      }
      return q;
    }
    function je(T, D) {
      for (
        var L = -1, q = T == null ? 0 : T.length;
        ++L < q && D(T[L], L, T) !== !1;

      );
      return T;
    }
    function $f(T, D) {
      for (var L = T == null ? 0 : T.length; L-- && D(T[L], L, T) !== !1; );
      return T;
    }
    function _o(T, D) {
      for (var L = -1, q = T == null ? 0 : T.length; ++L < q; )
        if (!D(T[L], L, T)) return !1;
      return !0;
    }
    function Ct(T, D) {
      for (
        var L = -1, q = T == null ? 0 : T.length, Z = 0, ae = [];
        ++L < q;

      ) {
        var Ue = T[L];
        D(Ue, L, T) && (ae[Z++] = Ue);
      }
      return ae;
    }
    function er(T, D) {
      var L = T == null ? 0 : T.length;
      return !!L && ti(T, D, 0) > -1;
    }
    function hn(T, D, L) {
      for (var q = -1, Z = T == null ? 0 : T.length; ++q < Z; )
        if (L(D, T[q])) return !0;
      return !1;
    }
    function ge(T, D) {
      for (var L = -1, q = T == null ? 0 : T.length, Z = Array(q); ++L < q; )
        Z[L] = D(T[L], L, T);
      return Z;
    }
    function Rt(T, D) {
      for (var L = -1, q = D.length, Z = T.length; ++L < q; ) T[Z + L] = D[L];
      return T;
    }
    function dn(T, D, L, q) {
      var Z = -1,
        ae = T == null ? 0 : T.length;
      for (q && ae && (L = T[++Z]); ++Z < ae; ) L = D(L, T[Z], Z, T);
      return L;
    }
    function Vf(T, D, L, q) {
      var Z = T == null ? 0 : T.length;
      for (q && Z && (L = T[--Z]); Z--; ) L = D(L, T[Z], Z, T);
      return L;
    }
    function cn(T, D) {
      for (var L = -1, q = T == null ? 0 : T.length; ++L < q; )
        if (D(T[L], L, T)) return !0;
      return !1;
    }
    var Kf = pn("length");
    function Xf(T) {
      return T.split("");
    }
    function jf(T) {
      return T.match(nf) || [];
    }
    function go(T, D, L) {
      var q;
      return (
        L(T, function (Z, ae, Ue) {
          if (D(Z, ae, Ue)) return (q = ae), !1;
        }),
        q
      );
    }
    function tr(T, D, L, q) {
      for (var Z = T.length, ae = L + (q ? 1 : -1); q ? ae-- : ++ae < Z; )
        if (D(T[ae], ae, T)) return ae;
      return -1;
    }
    function ti(T, D, L) {
      return D === D ? ul(T, D, L) : tr(T, vo, L);
    }
    function Jf(T, D, L, q) {
      for (var Z = L - 1, ae = T.length; ++Z < ae; ) if (q(T[Z], D)) return Z;
      return -1;
    }
    function vo(T) {
      return T !== T;
    }
    function mo(T, D) {
      var L = T == null ? 0 : T.length;
      return L ? gn(T, D) / L : lt;
    }
    function pn(T) {
      return function (D) {
        return D == null ? h : D[T];
      };
    }
    function _n(T) {
      return function (D) {
        return T == null ? h : T[D];
      };
    }
    function yo(T, D, L, q, Z) {
      return (
        Z(T, function (ae, Ue, he) {
          L = q ? ((q = !1), ae) : D(L, ae, Ue, he);
        }),
        L
      );
    }
    function Zf(T, D) {
      var L = T.length;
      for (T.sort(D); L--; ) T[L] = T[L].value;
      return T;
    }
    function gn(T, D) {
      for (var L, q = -1, Z = T.length; ++q < Z; ) {
        var ae = D(T[q]);
        ae !== h && (L = L === h ? ae : L + ae);
      }
      return L;
    }
    function vn(T, D) {
      for (var L = -1, q = Array(T); ++L < T; ) q[L] = D(L);
      return q;
    }
    function Qf(T, D) {
      return ge(D, function (L) {
        return [L, T[L]];
      });
    }
    function So(T) {
      return T && T.slice(0, Uo(T) + 1).replace(rn, "");
    }
    function Ge(T) {
      return function (D) {
        return T(D);
      };
    }
    function mn(T, D) {
      return ge(D, function (L) {
        return T[L];
      });
    }
    function wi(T, D) {
      return T.has(D);
    }
    function bo(T, D) {
      for (var L = -1, q = T.length; ++L < q && ti(D, T[L], 0) > -1; );
      return L;
    }
    function wo(T, D) {
      for (var L = T.length; L-- && ti(D, T[L], 0) > -1; );
      return L;
    }
    function el(T, D) {
      for (var L = T.length, q = 0; L--; ) T[L] === D && ++q;
      return q;
    }
    var tl = _n(Mf),
      il = _n(zf);
    function rl(T) {
      return "\\" + Wf[T];
    }
    function nl(T, D) {
      return T == null ? h : T[D];
    }
    function ii(T) {
      return Pf.test(T);
    }
    function sl(T) {
      return kf.test(T);
    }
    function ol(T) {
      for (var D, L = []; !(D = T.next()).done; ) L.push(D.value);
      return L;
    }
    function yn(T) {
      var D = -1,
        L = Array(T.size);
      return (
        T.forEach(function (q, Z) {
          L[++D] = [Z, q];
        }),
        L
      );
    }
    function Eo(T, D) {
      return function (L) {
        return T(D(L));
      };
    }
    function At(T, D) {
      for (var L = -1, q = T.length, Z = 0, ae = []; ++L < q; ) {
        var Ue = T[L];
        (Ue === D || Ue === w) && ((T[L] = w), (ae[Z++] = L));
      }
      return ae;
    }
    function ir(T) {
      var D = -1,
        L = Array(T.size);
      return (
        T.forEach(function (q) {
          L[++D] = q;
        }),
        L
      );
    }
    function al(T) {
      var D = -1,
        L = Array(T.size);
      return (
        T.forEach(function (q) {
          L[++D] = [q, q];
        }),
        L
      );
    }
    function ul(T, D, L) {
      for (var q = L - 1, Z = T.length; ++q < Z; ) if (T[q] === D) return q;
      return -1;
    }
    function fl(T, D, L) {
      for (var q = L + 1; q--; ) if (T[q] === D) return q;
      return q;
    }
    function ri(T) {
      return ii(T) ? hl(T) : Kf(T);
    }
    function st(T) {
      return ii(T) ? dl(T) : Xf(T);
    }
    function Uo(T) {
      for (var D = T.length; D-- && Qu.test(T.charAt(D)); );
      return D;
    }
    var ll = _n(Nf);
    function hl(T) {
      for (var D = (un.lastIndex = 0); un.test(T); ) ++D;
      return D;
    }
    function dl(T) {
      return T.match(un) || [];
    }
    function cl(T) {
      return T.match(Of) || [];
    }
    var pl = function T(D) {
        D = D == null ? Ae : ni.defaults(Ae.Object(), D, ni.pick(Ae, Ff));
        var L = D.Array,
          q = D.Date,
          Z = D.Error,
          ae = D.Function,
          Ue = D.Math,
          he = D.Object,
          Sn = D.RegExp,
          _l = D.String,
          Je = D.TypeError,
          rr = L.prototype,
          gl = ae.prototype,
          si = he.prototype,
          nr = D["__core-js_shared__"],
          sr = gl.toString,
          le = si.hasOwnProperty,
          vl = 0,
          xo = (function () {
            var t = /[^.]+$/.exec((nr && nr.keys && nr.keys.IE_PROTO) || "");
            return t ? "Symbol(src)_1." + t : "";
          })(),
          or = si.toString,
          ml = sr.call(he),
          yl = Ae._,
          Sl = Sn(
            "^" +
              sr
                .call(le)
                .replace(tn, "\\$&")
                .replace(
                  /hasOwnProperty|(function).*?(?=\\\()| for .+?(?=\\\])/g,
                  "$1.*?"
                ) +
              "$"
          ),
          ar = ao ? D.Buffer : h,
          Bt = D.Symbol,
          ur = D.Uint8Array,
          To = ar ? ar.allocUnsafe : h,
          fr = Eo(he.getPrototypeOf, he),
          Co = he.create,
          Ro = si.propertyIsEnumerable,
          lr = rr.splice,
          Ao = Bt ? Bt.isConcatSpreadable : h,
          Ei = Bt ? Bt.iterator : h,
          Ht = Bt ? Bt.toStringTag : h,
          hr = (function () {
            try {
              var t = Vt(he, "defineProperty");
              return t({}, "", {}), t;
            } catch (i) {}
          })(),
          bl = D.clearTimeout !== Ae.clearTimeout && D.clearTimeout,
          wl = q && q.now !== Ae.Date.now && q.now,
          El = D.setTimeout !== Ae.setTimeout && D.setTimeout,
          dr = Ue.ceil,
          cr = Ue.floor,
          bn = he.getOwnPropertySymbols,
          Ul = ar ? ar.isBuffer : h,
          Bo = D.isFinite,
          xl = rr.join,
          Tl = Eo(he.keys, he),
          xe = Ue.max,
          Le = Ue.min,
          Cl = q.now,
          Rl = D.parseInt,
          Io = Ue.random,
          Al = rr.reverse,
          wn = Vt(D, "DataView"),
          Ui = Vt(D, "Map"),
          En = Vt(D, "Promise"),
          oi = Vt(D, "Set"),
          xi = Vt(D, "WeakMap"),
          Ti = Vt(he, "create"),
          pr = xi && new xi(),
          ai = {},
          Bl = Kt(wn),
          Il = Kt(Ui),
          Ll = Kt(En),
          Ol = Kt(oi),
          Pl = Kt(xi),
          _r = Bt ? Bt.prototype : h,
          Ci = _r ? _r.valueOf : h,
          Lo = _r ? _r.toString : h;
        function _(t) {
          if (me(t) && !Q(t) && !(t instanceof se)) {
            if (t instanceof Ze) return t;
            if (le.call(t, "__wrapped__")) return Oa(t);
          }
          return new Ze(t);
        }
        var ui = (function () {
          function t() {}
          return function (i) {
            if (!ve(i)) return {};
            if (Co) return Co(i);
            t.prototype = i;
            var s = new t();
            return (t.prototype = h), s;
          };
        })();
        function gr() {}
        function Ze(t, i) {
          (this.__wrapped__ = t),
            (this.__actions__ = []),
            (this.__chain__ = !!i),
            (this.__index__ = 0),
            (this.__values__ = h);
        }
        (_.templateSettings = {
          escape: Vu,
          evaluate: Ku,
          interpolate: Ns,
          variable: "",
          imports: {
            _,
          },
        }),
          (_.prototype = gr.prototype),
          (_.prototype.constructor = _),
          (Ze.prototype = ui(gr.prototype)),
          (Ze.prototype.constructor = Ze);
        function se(t) {
          (this.__wrapped__ = t),
            (this.__actions__ = []),
            (this.__dir__ = 1),
            (this.__filtered__ = !1),
            (this.__iteratees__ = []),
            (this.__takeCount__ = j),
            (this.__views__ = []);
        }
        function kl() {
          var t = new se(this.__wrapped__);
          return (
            (t.__actions__ = Me(this.__actions__)),
            (t.__dir__ = this.__dir__),
            (t.__filtered__ = this.__filtered__),
            (t.__iteratees__ = Me(this.__iteratees__)),
            (t.__takeCount__ = this.__takeCount__),
            (t.__views__ = Me(this.__views__)),
            t
          );
        }
        function Fl() {
          if (this.__filtered__) {
            var t = new se(this);
            (t.__dir__ = -1), (t.__filtered__ = !0);
          } else (t = this.clone()), (t.__dir__ *= -1);
          return t;
        }
        function Dl() {
          var t = this.__wrapped__.value(),
            i = this.__dir__,
            s = Q(t),
            u = i < 0,
            c = s ? t.length : 0,
            v = Xh(0, c, this.__views__),
            b = v.start,
            E = v.end,
            C = E - b,
            z = u ? E : b - 1,
            N = this.__iteratees__,
            W = N.length,
            G = 0,
            Y = Le(C, this.__takeCount__);
          if (!s || (!u && c == C && Y == C)) return ia(t, this.__actions__);
          var K = [];
          e: for (; C-- && G < Y; ) {
            z += i;
            for (var te = -1, X = t[z]; ++te < W; ) {
              var ne = N[te],
                oe = ne.iteratee,
                $e = ne.type,
                De = oe(X);
              if ($e == be) X = De;
              else if (!De) {
                if ($e == Te) continue e;
                break e;
              }
            }
            K[G++] = X;
          }
          return K;
        }
        (se.prototype = ui(gr.prototype)), (se.prototype.constructor = se);
        function Gt(t) {
          var i = -1,
            s = t == null ? 0 : t.length;
          for (this.clear(); ++i < s; ) {
            var u = t[i];
            this.set(u[0], u[1]);
          }
        }
        function Ml() {
          (this.__data__ = Ti ? Ti(null) : {}), (this.size = 0);
        }
        function zl(t) {
          var i = this.has(t) && delete this.__data__[t];
          return (this.size -= i ? 1 : 0), i;
        }
        function Nl(t) {
          var i = this.__data__;
          if (Ti) {
            var s = i[t];
            return s === M ? h : s;
          }
          return le.call(i, t) ? i[t] : h;
        }
        function Wl(t) {
          var i = this.__data__;
          return Ti ? i[t] !== h : le.call(i, t);
        }
        function Hl(t, i) {
          var s = this.__data__;
          return (
            (this.size += this.has(t) ? 0 : 1),
            (s[t] = Ti && i === h ? M : i),
            this
          );
        }
        (Gt.prototype.clear = Ml),
          (Gt.prototype.delete = zl),
          (Gt.prototype.get = Nl),
          (Gt.prototype.has = Wl),
          (Gt.prototype.set = Hl);
        function vt(t) {
          var i = -1,
            s = t == null ? 0 : t.length;
          for (this.clear(); ++i < s; ) {
            var u = t[i];
            this.set(u[0], u[1]);
          }
        }
        function Gl() {
          (this.__data__ = []), (this.size = 0);
        }
        function ql(t) {
          var i = this.__data__,
            s = vr(i, t);
          if (s < 0) return !1;
          var u = i.length - 1;
          return s == u ? i.pop() : lr.call(i, s, 1), --this.size, !0;
        }
        function Yl(t) {
          var i = this.__data__,
            s = vr(i, t);
          return s < 0 ? h : i[s][1];
        }
        function $l(t) {
          return vr(this.__data__, t) > -1;
        }
        function Vl(t, i) {
          var s = this.__data__,
            u = vr(s, t);
          return u < 0 ? (++this.size, s.push([t, i])) : (s[u][1] = i), this;
        }
        (vt.prototype.clear = Gl),
          (vt.prototype.delete = ql),
          (vt.prototype.get = Yl),
          (vt.prototype.has = $l),
          (vt.prototype.set = Vl);
        function mt(t) {
          var i = -1,
            s = t == null ? 0 : t.length;
          for (this.clear(); ++i < s; ) {
            var u = t[i];
            this.set(u[0], u[1]);
          }
        }
        function Kl() {
          (this.size = 0),
            (this.__data__ = {
              hash: new Gt(),
              map: new (Ui || vt)(),
              string: new Gt(),
            });
        }
        function Xl(t) {
          var i = Ar(this, t).delete(t);
          return (this.size -= i ? 1 : 0), i;
        }
        function jl(t) {
          return Ar(this, t).get(t);
        }
        function Jl(t) {
          return Ar(this, t).has(t);
        }
        function Zl(t, i) {
          var s = Ar(this, t),
            u = s.size;
          return s.set(t, i), (this.size += s.size == u ? 0 : 1), this;
        }
        (mt.prototype.clear = Kl),
          (mt.prototype.delete = Xl),
          (mt.prototype.get = jl),
          (mt.prototype.has = Jl),
          (mt.prototype.set = Zl);
        function qt(t) {
          var i = -1,
            s = t == null ? 0 : t.length;
          for (this.__data__ = new mt(); ++i < s; ) this.add(t[i]);
        }
        function Ql(t) {
          return this.__data__.set(t, M), this;
        }
        function eh(t) {
          return this.__data__.has(t);
        }
        (qt.prototype.add = qt.prototype.push = Ql), (qt.prototype.has = eh);
        function ot(t) {
          var i = (this.__data__ = new vt(t));
          this.size = i.size;
        }
        function th() {
          (this.__data__ = new vt()), (this.size = 0);
        }
        function ih(t) {
          var i = this.__data__,
            s = i.delete(t);
          return (this.size = i.size), s;
        }
        function rh(t) {
          return this.__data__.get(t);
        }
        function nh(t) {
          return this.__data__.has(t);
        }
        function sh(t, i) {
          var s = this.__data__;
          if (s instanceof vt) {
            var u = s.__data__;
            if (!Ui || u.length < U - 1)
              return u.push([t, i]), (this.size = ++s.size), this;
            s = this.__data__ = new mt(u);
          }
          return s.set(t, i), (this.size = s.size), this;
        }
        (ot.prototype.clear = th),
          (ot.prototype.delete = ih),
          (ot.prototype.get = rh),
          (ot.prototype.has = nh),
          (ot.prototype.set = sh);
        function Oo(t, i) {
          var s = Q(t),
            u = !s && Xt(t),
            c = !s && !u && kt(t),
            v = !s && !u && !c && di(t),
            b = s || u || c || v,
            E = b ? vn(t.length, _l) : [],
            C = E.length;
          for (var z in t)
            (i || le.call(t, z)) &&
              !(
                b &&
                (z == "length" ||
                  (c && (z == "offset" || z == "parent")) ||
                  (v &&
                    (z == "buffer" ||
                      z == "byteLength" ||
                      z == "byteOffset")) ||
                  wt(z, C))
              ) &&
              E.push(z);
          return E;
        }
        function Po(t) {
          var i = t.length;
          return i ? t[Pn(0, i - 1)] : h;
        }
        function oh(t, i) {
          return Br(Me(t), Yt(i, 0, t.length));
        }
        function ah(t) {
          return Br(Me(t));
        }
        function Un(t, i, s) {
          ((s !== h && !at(t[i], s)) || (s === h && !(i in t))) && yt(t, i, s);
        }
        function Ri(t, i, s) {
          var u = t[i];
          (!(le.call(t, i) && at(u, s)) || (s === h && !(i in t))) &&
            yt(t, i, s);
        }
        function vr(t, i) {
          for (var s = t.length; s--; ) if (at(t[s][0], i)) return s;
          return -1;
        }
        function uh(t, i, s, u) {
          return (
            It(t, function (c, v, b) {
              i(u, c, s(c), b);
            }),
            u
          );
        }
        function ko(t, i) {
          return t && dt(i, Ce(i), t);
        }
        function fh(t, i) {
          return t && dt(i, Ne(i), t);
        }
        function yt(t, i, s) {
          i == "__proto__" && hr
            ? hr(t, i, {
                configurable: !0,
                enumerable: !0,
                value: s,
                writable: !0,
              })
            : (t[i] = s);
        }
        function xn(t, i) {
          for (var s = -1, u = i.length, c = L(u), v = t == null; ++s < u; )
            c[s] = v ? h : ss(t, i[s]);
          return c;
        }
        function Yt(t, i, s) {
          return (
            t === t &&
              (s !== h && (t = t <= s ? t : s),
              i !== h && (t = t >= i ? t : i)),
            t
          );
        }
        function Qe(t, i, s, u, c, v) {
          var b,
            E = i & k,
            C = i & e,
            z = i & r;
          if ((s && (b = c ? s(t, u, c, v) : s(t)), b !== h)) return b;
          if (!ve(t)) return t;
          var N = Q(t);
          if (N) {
            if (((b = Jh(t)), !E)) return Me(t, b);
          } else {
            var W = Oe(t),
              G = W == Xi || W == Fs;
            if (kt(t)) return sa(t, E);
            if (W == gt || W == Zt || (G && !c)) {
              if (((b = C || G ? {} : Ua(t)), !E))
                return C ? Nh(t, fh(b, t)) : zh(t, ko(b, t));
            } else {
              if (!ce[W]) return c ? t : {};
              b = Zh(t, W, E);
            }
          }
          v || (v = new ot());
          var Y = v.get(t);
          if (Y) return Y;
          v.set(t, b),
            Qa(t)
              ? t.forEach(function (X) {
                  b.add(Qe(X, i, s, X, t, v));
                })
              : Ja(t) &&
                t.forEach(function (X, ne) {
                  b.set(ne, Qe(X, i, s, ne, t, v));
                });
          var K = z ? (C ? Yn : qn) : C ? Ne : Ce,
            te = N ? h : K(t);
          return (
            je(te || t, function (X, ne) {
              te && ((ne = X), (X = t[ne])), Ri(b, ne, Qe(X, i, s, ne, t, v));
            }),
            b
          );
        }
        function lh(t) {
          var i = Ce(t);
          return function (s) {
            return Fo(s, t, i);
          };
        }
        function Fo(t, i, s) {
          var u = s.length;
          if (t == null) return !u;
          for (t = he(t); u--; ) {
            var c = s[u],
              v = i[c],
              b = t[c];
            if ((b === h && !(c in t)) || !v(b)) return !1;
          }
          return !0;
        }
        function Do(t, i, s) {
          if (typeof t != "function") throw new Je(A);
          return ki(function () {
            t.apply(h, s);
          }, i);
        }
        function Ai(t, i, s, u) {
          var c = -1,
            v = er,
            b = !0,
            E = t.length,
            C = [],
            z = i.length;
          if (!E) return C;
          s && (i = ge(i, Ge(s))),
            u
              ? ((v = hn), (b = !1))
              : i.length >= U && ((v = wi), (b = !1), (i = new qt(i)));
          e: for (; ++c < E; ) {
            var N = t[c],
              W = s == null ? N : s(N);
            if (((N = u || N !== 0 ? N : 0), b && W === W)) {
              for (var G = z; G--; ) if (i[G] === W) continue e;
              C.push(N);
            } else v(i, W, u) || C.push(N);
          }
          return C;
        }
        var It = la(ht),
          Mo = la(Cn, !0);
        function hh(t, i) {
          var s = !0;
          return (
            It(t, function (u, c, v) {
              return (s = !!i(u, c, v)), s;
            }),
            s
          );
        }
        function mr(t, i, s) {
          for (var u = -1, c = t.length; ++u < c; ) {
            var v = t[u],
              b = i(v);
            if (b != null && (E === h ? b === b && !Ye(b) : s(b, E)))
              var E = b,
                C = v;
          }
          return C;
        }
        function dh(t, i, s, u) {
          var c = t.length;
          for (
            s = ee(s),
              s < 0 && (s = -s > c ? 0 : c + s),
              u = u === h || u > c ? c : ee(u),
              u < 0 && (u += c),
              u = s > u ? 0 : tu(u);
            s < u;

          )
            t[s++] = i;
          return t;
        }
        function zo(t, i) {
          var s = [];
          return (
            It(t, function (u, c, v) {
              i(u, c, v) && s.push(u);
            }),
            s
          );
        }
        function Be(t, i, s, u, c) {
          var v = -1,
            b = t.length;
          for (s || (s = ed), c || (c = []); ++v < b; ) {
            var E = t[v];
            i > 0 && s(E)
              ? i > 1
                ? Be(E, i - 1, s, u, c)
                : Rt(c, E)
              : u || (c[c.length] = E);
          }
          return c;
        }
        var Tn = ha(),
          No = ha(!0);
        function ht(t, i) {
          return t && Tn(t, i, Ce);
        }
        function Cn(t, i) {
          return t && No(t, i, Ce);
        }
        function yr(t, i) {
          return Ct(i, function (s) {
            return Et(t[s]);
          });
        }
        function $t(t, i) {
          i = Ot(i, t);
          for (var s = 0, u = i.length; t != null && s < u; ) t = t[ct(i[s++])];
          return s && s == u ? t : h;
        }
        function Wo(t, i, s) {
          var u = i(t);
          return Q(t) ? u : Rt(u, s(t));
        }
        function ke(t) {
          return t == null
            ? t === h
              ? Nu
              : Mu
            : Ht && Ht in he(t)
            ? Kh(t)
            : ad(t);
        }
        function Rn(t, i) {
          return t > i;
        }
        function ch(t, i) {
          return t != null && le.call(t, i);
        }
        function ph(t, i) {
          return t != null && i in he(t);
        }
        function _h(t, i, s) {
          return t >= Le(i, s) && t < xe(i, s);
        }
        function An(t, i, s) {
          for (
            var u = s ? hn : er,
              c = t[0].length,
              v = t.length,
              b = v,
              E = L(v),
              C = 1 / 0,
              z = [];
            b--;

          ) {
            var N = t[b];
            b && i && (N = ge(N, Ge(i))),
              (C = Le(N.length, C)),
              (E[b] =
                !s && (i || (c >= 120 && N.length >= 120))
                  ? new qt(b && N)
                  : h);
          }
          N = t[0];
          var W = -1,
            G = E[0];
          e: for (; ++W < c && z.length < C; ) {
            var Y = N[W],
              K = i ? i(Y) : Y;
            if (((Y = s || Y !== 0 ? Y : 0), !(G ? wi(G, K) : u(z, K, s)))) {
              for (b = v; --b; ) {
                var te = E[b];
                if (!(te ? wi(te, K) : u(t[b], K, s))) continue e;
              }
              G && G.push(K), z.push(Y);
            }
          }
          return z;
        }
        function gh(t, i, s, u) {
          return (
            ht(t, function (c, v, b) {
              i(u, s(c), v, b);
            }),
            u
          );
        }
        function Bi(t, i, s) {
          (i = Ot(i, t)), (t = Ra(t, i));
          var u = t == null ? t : t[ct(tt(i))];
          return u == null ? h : He(u, t, s);
        }
        function Ho(t) {
          return me(t) && ke(t) == Zt;
        }
        function vh(t) {
          return me(t) && ke(t) == bi;
        }
        function mh(t) {
          return me(t) && ke(t) == gi;
        }
        function Ii(t, i, s, u, c) {
          return t === i
            ? !0
            : t == null || i == null || (!me(t) && !me(i))
            ? t !== t && i !== i
            : yh(t, i, s, u, Ii, c);
        }
        function yh(t, i, s, u, c, v) {
          var b = Q(t),
            E = Q(i),
            C = b ? Vi : Oe(t),
            z = E ? Vi : Oe(i);
          (C = C == Zt ? gt : C), (z = z == Zt ? gt : z);
          var N = C == gt,
            W = z == gt,
            G = C == z;
          if (G && kt(t)) {
            if (!kt(i)) return !1;
            (b = !0), (N = !1);
          }
          if (G && !N)
            return (
              v || (v = new ot()),
              b || di(t) ? ba(t, i, s, u, c, v) : $h(t, i, C, s, u, c, v)
            );
          if (!(s & n)) {
            var Y = N && le.call(t, "__wrapped__"),
              K = W && le.call(i, "__wrapped__");
            if (Y || K) {
              var te = Y ? t.value() : t,
                X = K ? i.value() : i;
              return v || (v = new ot()), c(te, X, s, u, v);
            }
          }
          return G ? (v || (v = new ot()), Vh(t, i, s, u, c, v)) : !1;
        }
        function Sh(t) {
          return me(t) && Oe(t) == rt;
        }
        function Bn(t, i, s, u) {
          var c = s.length,
            v = c,
            b = !u;
          if (t == null) return !v;
          for (t = he(t); c--; ) {
            var E = s[c];
            if (b && E[2] ? E[1] !== t[E[0]] : !(E[0] in t)) return !1;
          }
          for (; ++c < v; ) {
            E = s[c];
            var C = E[0],
              z = t[C],
              N = E[1];
            if (b && E[2]) {
              if (z === h && !(C in t)) return !1;
            } else {
              var W = new ot();
              if (u) var G = u(z, N, C, t, i, W);
              if (!(G === h ? Ii(N, z, n | a, u, W) : G)) return !1;
            }
          }
          return !0;
        }
        function Go(t) {
          if (!ve(t) || id(t)) return !1;
          var i = Et(t) ? Sl : lf;
          return i.test(Kt(t));
        }
        function bh(t) {
          return me(t) && ke(t) == mi;
        }
        function wh(t) {
          return me(t) && Oe(t) == nt;
        }
        function Eh(t) {
          return me(t) && Fr(t.length) && !!pe[ke(t)];
        }
        function qo(t) {
          return typeof t == "function"
            ? t
            : t == null
            ? We
            : typeof t == "object"
            ? Q(t)
              ? Vo(t[0], t[1])
              : $o(t)
            : du(t);
        }
        function In(t) {
          if (!Pi(t)) return Tl(t);
          var i = [];
          for (var s in he(t)) le.call(t, s) && s != "constructor" && i.push(s);
          return i;
        }
        function Uh(t) {
          if (!ve(t)) return od(t);
          var i = Pi(t),
            s = [];
          for (var u in t)
            (u == "constructor" && (i || !le.call(t, u))) || s.push(u);
          return s;
        }
        function Ln(t, i) {
          return t < i;
        }
        function Yo(t, i) {
          var s = -1,
            u = ze(t) ? L(t.length) : [];
          return (
            It(t, function (c, v, b) {
              u[++s] = i(c, v, b);
            }),
            u
          );
        }
        function $o(t) {
          var i = Vn(t);
          return i.length == 1 && i[0][2]
            ? Ta(i[0][0], i[0][1])
            : function (s) {
                return s === t || Bn(s, t, i);
              };
        }
        function Vo(t, i) {
          return Xn(t) && xa(i)
            ? Ta(ct(t), i)
            : function (s) {
                var u = ss(s, t);
                return u === h && u === i ? os(s, t) : Ii(i, u, n | a);
              };
        }
        function Sr(t, i, s, u, c) {
          t !== i &&
            Tn(
              i,
              function (v, b) {
                if ((c || (c = new ot()), ve(v))) xh(t, i, b, s, Sr, u, c);
                else {
                  var E = u ? u(Jn(t, b), v, b + "", t, i, c) : h;
                  E === h && (E = v), Un(t, b, E);
                }
              },
              Ne
            );
        }
        function xh(t, i, s, u, c, v, b) {
          var E = Jn(t, s),
            C = Jn(i, s),
            z = b.get(C);
          if (z) {
            Un(t, s, z);
            return;
          }
          var N = v ? v(E, C, s + "", t, i, b) : h,
            W = N === h;
          if (W) {
            var G = Q(C),
              Y = !G && kt(C),
              K = !G && !Y && di(C);
            (N = C),
              G || Y || K
                ? Q(E)
                  ? (N = E)
                  : ye(E)
                  ? (N = Me(E))
                  : Y
                  ? ((W = !1), (N = sa(C, !0)))
                  : K
                  ? ((W = !1), (N = oa(C, !0)))
                  : (N = [])
                : Fi(C) || Xt(C)
                ? ((N = E),
                  Xt(E) ? (N = iu(E)) : (!ve(E) || Et(E)) && (N = Ua(C)))
                : (W = !1);
          }
          W && (b.set(C, N), c(N, C, u, v, b), b.delete(C)), Un(t, s, N);
        }
        function Ko(t, i) {
          var s = t.length;
          if (s) return (i += i < 0 ? s : 0), wt(i, s) ? t[i] : h;
        }
        function Xo(t, i, s) {
          i.length
            ? (i = ge(i, function (v) {
                return Q(v)
                  ? function (b) {
                      return $t(b, v.length === 1 ? v[0] : v);
                    }
                  : v;
              }))
            : (i = [We]);
          var u = -1;
          i = ge(i, Ge(V()));
          var c = Yo(t, function (v, b, E) {
            var C = ge(i, function (z) {
              return z(v);
            });
            return {
              criteria: C,
              index: ++u,
              value: v,
            };
          });
          return Zf(c, function (v, b) {
            return Mh(v, b, s);
          });
        }
        function Th(t, i) {
          return jo(t, i, function (s, u) {
            return os(t, u);
          });
        }
        function jo(t, i, s) {
          for (var u = -1, c = i.length, v = {}; ++u < c; ) {
            var b = i[u],
              E = $t(t, b);
            s(E, b) && Li(v, Ot(b, t), E);
          }
          return v;
        }
        function Ch(t) {
          return function (i) {
            return $t(i, t);
          };
        }
        function On(t, i, s, u) {
          var c = u ? Jf : ti,
            v = -1,
            b = i.length,
            E = t;
          for (t === i && (i = Me(i)), s && (E = ge(t, Ge(s))); ++v < b; )
            for (
              var C = 0, z = i[v], N = s ? s(z) : z;
              (C = c(E, N, C, u)) > -1;

            )
              E !== t && lr.call(E, C, 1), lr.call(t, C, 1);
          return t;
        }
        function Jo(t, i) {
          for (var s = t ? i.length : 0, u = s - 1; s--; ) {
            var c = i[s];
            if (s == u || c !== v) {
              var v = c;
              wt(c) ? lr.call(t, c, 1) : Dn(t, c);
            }
          }
          return t;
        }
        function Pn(t, i) {
          return t + cr(Io() * (i - t + 1));
        }
        function Rh(t, i, s, u) {
          for (var c = -1, v = xe(dr((i - t) / (s || 1)), 0), b = L(v); v--; )
            (b[u ? v : ++c] = t), (t += s);
          return b;
        }
        function kn(t, i) {
          var s = "";
          if (!t || i < 1 || i > Ke) return s;
          do i % 2 && (s += t), (i = cr(i / 2)), i && (t += t);
          while (i);
          return s;
        }
        function ie(t, i) {
          return Zn(Ca(t, i, We), t + "");
        }
        function Ah(t) {
          return Po(ci(t));
        }
        function Bh(t, i) {
          var s = ci(t);
          return Br(s, Yt(i, 0, s.length));
        }
        function Li(t, i, s, u) {
          if (!ve(t)) return t;
          i = Ot(i, t);
          for (
            var c = -1, v = i.length, b = v - 1, E = t;
            E != null && ++c < v;

          ) {
            var C = ct(i[c]),
              z = s;
            if (C === "__proto__" || C === "constructor" || C === "prototype")
              return t;
            if (c != b) {
              var N = E[C];
              (z = u ? u(N, C, E) : h),
                z === h && (z = ve(N) ? N : wt(i[c + 1]) ? [] : {});
            }
            Ri(E, C, z), (E = E[C]);
          }
          return t;
        }
        var Zo = pr
            ? function (t, i) {
                return pr.set(t, i), t;
              }
            : We,
          Ih = hr
            ? function (t, i) {
                return hr(t, "toString", {
                  configurable: !0,
                  enumerable: !1,
                  value: us(i),
                  writable: !0,
                });
              }
            : We;
        function Lh(t) {
          return Br(ci(t));
        }
        function et(t, i, s) {
          var u = -1,
            c = t.length;
          i < 0 && (i = -i > c ? 0 : c + i),
            (s = s > c ? c : s),
            s < 0 && (s += c),
            (c = i > s ? 0 : (s - i) >>> 0),
            (i >>>= 0);
          for (var v = L(c); ++u < c; ) v[u] = t[u + i];
          return v;
        }
        function Oh(t, i) {
          var s;
          return (
            It(t, function (u, c, v) {
              return (s = i(u, c, v)), !s;
            }),
            !!s
          );
        }
        function br(t, i, s) {
          var u = 0,
            c = t == null ? u : t.length;
          if (typeof i == "number" && i === i && c <= Pu) {
            for (; u < c; ) {
              var v = (u + c) >>> 1,
                b = t[v];
              b !== null && !Ye(b) && (s ? b <= i : b < i)
                ? (u = v + 1)
                : (c = v);
            }
            return c;
          }
          return Fn(t, i, We, s);
        }
        function Fn(t, i, s, u) {
          var c = 0,
            v = t == null ? 0 : t.length;
          if (v === 0) return 0;
          i = s(i);
          for (
            var b = i !== i, E = i === null, C = Ye(i), z = i === h;
            c < v;

          ) {
            var N = cr((c + v) / 2),
              W = s(t[N]),
              G = W !== h,
              Y = W === null,
              K = W === W,
              te = Ye(W);
            if (b) var X = u || K;
            else
              z
                ? (X = K && (u || G))
                : E
                ? (X = K && G && (u || !Y))
                : C
                ? (X = K && G && !Y && (u || !te))
                : Y || te
                ? (X = !1)
                : (X = u ? W <= i : W < i);
            X ? (c = N + 1) : (v = N);
          }
          return Le(v, Ou);
        }
        function Qo(t, i) {
          for (var s = -1, u = t.length, c = 0, v = []; ++s < u; ) {
            var b = t[s],
              E = i ? i(b) : b;
            if (!s || !at(E, C)) {
              var C = E;
              v[c++] = b === 0 ? 0 : b;
            }
          }
          return v;
        }
        function ea(t) {
          return typeof t == "number" ? t : Ye(t) ? lt : +t;
        }
        function qe(t) {
          if (typeof t == "string") return t;
          if (Q(t)) return ge(t, qe) + "";
          if (Ye(t)) return Lo ? Lo.call(t) : "";
          var i = t + "";
          return i == "0" && 1 / t == -Ie ? "-0" : i;
        }
        function Lt(t, i, s) {
          var u = -1,
            c = er,
            v = t.length,
            b = !0,
            E = [],
            C = E;
          if (s) (b = !1), (c = hn);
          else if (v >= U) {
            var z = i ? null : qh(t);
            if (z) return ir(z);
            (b = !1), (c = wi), (C = new qt());
          } else C = i ? [] : E;
          e: for (; ++u < v; ) {
            var N = t[u],
              W = i ? i(N) : N;
            if (((N = s || N !== 0 ? N : 0), b && W === W)) {
              for (var G = C.length; G--; ) if (C[G] === W) continue e;
              i && C.push(W), E.push(N);
            } else c(C, W, s) || (C !== E && C.push(W), E.push(N));
          }
          return E;
        }
        function Dn(t, i) {
          return (
            (i = Ot(i, t)), (t = Ra(t, i)), t == null || delete t[ct(tt(i))]
          );
        }
        function ta(t, i, s, u) {
          return Li(t, i, s($t(t, i)), u);
        }
        function wr(t, i, s, u) {
          for (
            var c = t.length, v = u ? c : -1;
            (u ? v-- : ++v < c) && i(t[v], v, t);

          );
          return s
            ? et(t, u ? 0 : v, u ? v + 1 : c)
            : et(t, u ? v + 1 : 0, u ? c : v);
        }
        function ia(t, i) {
          var s = t;
          return (
            s instanceof se && (s = s.value()),
            dn(
              i,
              function (u, c) {
                return c.func.apply(c.thisArg, Rt([u], c.args));
              },
              s
            )
          );
        }
        function Mn(t, i, s) {
          var u = t.length;
          if (u < 2) return u ? Lt(t[0]) : [];
          for (var c = -1, v = L(u); ++c < u; )
            for (var b = t[c], E = -1; ++E < u; )
              E != c && (v[c] = Ai(v[c] || b, t[E], i, s));
          return Lt(Be(v, 1), i, s);
        }
        function ra(t, i, s) {
          for (var u = -1, c = t.length, v = i.length, b = {}; ++u < c; ) {
            var E = u < v ? i[u] : h;
            s(b, t[u], E);
          }
          return b;
        }
        function zn(t) {
          return ye(t) ? t : [];
        }
        function Nn(t) {
          return typeof t == "function" ? t : We;
        }
        function Ot(t, i) {
          return Q(t) ? t : Xn(t, i) ? [t] : La(fe(t));
        }
        var Ph = ie;
        function Pt(t, i, s) {
          var u = t.length;
          return (s = s === h ? u : s), !i && s >= u ? t : et(t, i, s);
        }
        var na =
          bl ||
          function (t) {
            return Ae.clearTimeout(t);
          };
        function sa(t, i) {
          if (i) return t.slice();
          var s = t.length,
            u = To ? To(s) : new t.constructor(s);
          return t.copy(u), u;
        }
        function Wn(t) {
          var i = new t.constructor(t.byteLength);
          return new ur(i).set(new ur(t)), i;
        }
        function kh(t, i) {
          var s = i ? Wn(t.buffer) : t.buffer;
          return new t.constructor(s, t.byteOffset, t.byteLength);
        }
        function Fh(t) {
          var i = new t.constructor(t.source, Ws.exec(t));
          return (i.lastIndex = t.lastIndex), i;
        }
        function Dh(t) {
          return Ci ? he(Ci.call(t)) : {};
        }
        function oa(t, i) {
          var s = i ? Wn(t.buffer) : t.buffer;
          return new t.constructor(s, t.byteOffset, t.length);
        }
        function aa(t, i) {
          if (t !== i) {
            var s = t !== h,
              u = t === null,
              c = t === t,
              v = Ye(t),
              b = i !== h,
              E = i === null,
              C = i === i,
              z = Ye(i);
            if (
              (!E && !z && !v && t > i) ||
              (v && b && C && !E && !z) ||
              (u && b && C) ||
              (!s && C) ||
              !c
            )
              return 1;
            if (
              (!u && !v && !z && t < i) ||
              (z && s && c && !u && !v) ||
              (E && s && c) ||
              (!b && c) ||
              !C
            )
              return -1;
          }
          return 0;
        }
        function Mh(t, i, s) {
          for (
            var u = -1,
              c = t.criteria,
              v = i.criteria,
              b = c.length,
              E = s.length;
            ++u < b;

          ) {
            var C = aa(c[u], v[u]);
            if (C) {
              if (u >= E) return C;
              var z = s[u];
              return C * (z == "desc" ? -1 : 1);
            }
          }
          return t.index - i.index;
        }
        function ua(t, i, s, u) {
          for (
            var c = -1,
              v = t.length,
              b = s.length,
              E = -1,
              C = i.length,
              z = xe(v - b, 0),
              N = L(C + z),
              W = !u;
            ++E < C;

          )
            N[E] = i[E];
          for (; ++c < b; ) (W || c < v) && (N[s[c]] = t[c]);
          for (; z--; ) N[E++] = t[c++];
          return N;
        }
        function fa(t, i, s, u) {
          for (
            var c = -1,
              v = t.length,
              b = -1,
              E = s.length,
              C = -1,
              z = i.length,
              N = xe(v - E, 0),
              W = L(N + z),
              G = !u;
            ++c < N;

          )
            W[c] = t[c];
          for (var Y = c; ++C < z; ) W[Y + C] = i[C];
          for (; ++b < E; ) (G || c < v) && (W[Y + s[b]] = t[c++]);
          return W;
        }
        function Me(t, i) {
          var s = -1,
            u = t.length;
          for (i || (i = L(u)); ++s < u; ) i[s] = t[s];
          return i;
        }
        function dt(t, i, s, u) {
          var c = !s;
          s || (s = {});
          for (var v = -1, b = i.length; ++v < b; ) {
            var E = i[v],
              C = u ? u(s[E], t[E], E, s, t) : h;
            C === h && (C = t[E]), c ? yt(s, E, C) : Ri(s, E, C);
          }
          return s;
        }
        function zh(t, i) {
          return dt(t, Kn(t), i);
        }
        function Nh(t, i) {
          return dt(t, wa(t), i);
        }
        function Er(t, i) {
          return function (s, u) {
            var c = Q(s) ? Yf : uh,
              v = i ? i() : {};
            return c(s, t, V(u, 2), v);
          };
        }
        function fi(t) {
          return ie(function (i, s) {
            var u = -1,
              c = s.length,
              v = c > 1 ? s[c - 1] : h,
              b = c > 2 ? s[2] : h;
            for (
              v = t.length > 3 && typeof v == "function" ? (c--, v) : h,
                b && Fe(s[0], s[1], b) && ((v = c < 3 ? h : v), (c = 1)),
                i = he(i);
              ++u < c;

            ) {
              var E = s[u];
              E && t(i, E, u, v);
            }
            return i;
          });
        }
        function la(t, i) {
          return function (s, u) {
            if (s == null) return s;
            if (!ze(s)) return t(s, u);
            for (
              var c = s.length, v = i ? c : -1, b = he(s);
              (i ? v-- : ++v < c) && u(b[v], v, b) !== !1;

            );
            return s;
          };
        }
        function ha(t) {
          return function (i, s, u) {
            for (var c = -1, v = he(i), b = u(i), E = b.length; E--; ) {
              var C = b[t ? E : ++c];
              if (s(v[C], C, v) === !1) break;
            }
            return i;
          };
        }
        function Wh(t, i, s) {
          var u = i & l,
            c = Oi(t);
          function v() {
            var b = this && this !== Ae && this instanceof v ? c : t;
            return b.apply(u ? s : this, arguments);
          }
          return v;
        }
        function da(t) {
          return function (i) {
            i = fe(i);
            var s = ii(i) ? st(i) : h,
              u = s ? s[0] : i.charAt(0),
              c = s ? Pt(s, 1).join("") : i.slice(1);
            return u[t]() + c;
          };
        }
        function li(t) {
          return function (i) {
            return dn(lu(fu(i).replace(If, "")), t, "");
          };
        }
        function Oi(t) {
          return function () {
            var i = arguments;
            switch (i.length) {
              case 0:
                return new t();
              case 1:
                return new t(i[0]);
              case 2:
                return new t(i[0], i[1]);
              case 3:
                return new t(i[0], i[1], i[2]);
              case 4:
                return new t(i[0], i[1], i[2], i[3]);
              case 5:
                return new t(i[0], i[1], i[2], i[3], i[4]);
              case 6:
                return new t(i[0], i[1], i[2], i[3], i[4], i[5]);
              case 7:
                return new t(i[0], i[1], i[2], i[3], i[4], i[5], i[6]);
            }
            var s = ui(t.prototype),
              u = t.apply(s, i);
            return ve(u) ? u : s;
          };
        }
        function Hh(t, i, s) {
          var u = Oi(t);
          function c() {
            for (var v = arguments.length, b = L(v), E = v, C = hi(c); E--; )
              b[E] = arguments[E];
            var z = v < 3 && b[0] !== C && b[v - 1] !== C ? [] : At(b, C);
            if (((v -= z.length), v < s))
              return va(t, i, Ur, c.placeholder, h, b, z, h, h, s - v);
            var N = this && this !== Ae && this instanceof c ? u : t;
            return He(N, this, b);
          }
          return c;
        }
        function ca(t) {
          return function (i, s, u) {
            var c = he(i);
            if (!ze(i)) {
              var v = V(s, 3);
              (i = Ce(i)),
                (s = function (E) {
                  return v(c[E], E, c);
                });
            }
            var b = t(i, s, u);
            return b > -1 ? c[v ? i[b] : b] : h;
          };
        }
        function pa(t) {
          return bt(function (i) {
            var s = i.length,
              u = s,
              c = Ze.prototype.thru;
            for (t && i.reverse(); u--; ) {
              var v = i[u];
              if (typeof v != "function") throw new Je(A);
              if (c && !b && Rr(v) == "wrapper") var b = new Ze([], !0);
            }
            for (u = b ? u : s; ++u < s; ) {
              v = i[u];
              var E = Rr(v),
                C = E == "wrapper" ? $n(v) : h;
              C &&
              jn(C[0]) &&
              C[1] == (y | S | O | F) &&
              !C[4].length &&
              C[9] == 1
                ? (b = b[Rr(C[0])].apply(b, C[3]))
                : (b = v.length == 1 && jn(v) ? b[E]() : b.thru(v));
            }
            return function () {
              var z = arguments,
                N = z[0];
              if (b && z.length == 1 && Q(N)) return b.plant(N).value();
              for (var W = 0, G = s ? i[W].apply(this, z) : N; ++W < s; )
                G = i[W].call(this, G);
              return G;
            };
          });
        }
        function Ur(t, i, s, u, c, v, b, E, C, z) {
          var N = i & y,
            W = i & l,
            G = i & m,
            Y = i & (S | R),
            K = i & I,
            te = G ? h : Oi(t);
          function X() {
            for (var ne = arguments.length, oe = L(ne), $e = ne; $e--; )
              oe[$e] = arguments[$e];
            if (Y)
              var De = hi(X),
                Ve = el(oe, De);
            if (
              (u && (oe = ua(oe, u, c, Y)),
              v && (oe = fa(oe, v, b, Y)),
              (ne -= Ve),
              Y && ne < z)
            ) {
              var Se = At(oe, De);
              return va(t, i, Ur, X.placeholder, s, oe, Se, E, C, z - ne);
            }
            var ut = W ? s : this,
              xt = G ? ut[t] : t;
            return (
              (ne = oe.length),
              E ? (oe = ud(oe, E)) : K && ne > 1 && oe.reverse(),
              N && C < ne && (oe.length = C),
              this && this !== Ae && this instanceof X && (xt = te || Oi(xt)),
              xt.apply(ut, oe)
            );
          }
          return X;
        }
        function _a(t, i) {
          return function (s, u) {
            return gh(s, t, i(u), {});
          };
        }
        function xr(t, i) {
          return function (s, u) {
            var c;
            if (s === h && u === h) return i;
            if ((s !== h && (c = s), u !== h)) {
              if (c === h) return u;
              typeof s == "string" || typeof u == "string"
                ? ((s = qe(s)), (u = qe(u)))
                : ((s = ea(s)), (u = ea(u))),
                (c = t(s, u));
            }
            return c;
          };
        }
        function Hn(t) {
          return bt(function (i) {
            return (
              (i = ge(i, Ge(V()))),
              ie(function (s) {
                var u = this;
                return t(i, function (c) {
                  return He(c, u, s);
                });
              })
            );
          });
        }
        function Tr(t, i) {
          i = i === h ? " " : qe(i);
          var s = i.length;
          if (s < 2) return s ? kn(i, t) : i;
          var u = kn(i, dr(t / ri(i)));
          return ii(i) ? Pt(st(u), 0, t).join("") : u.slice(0, t);
        }
        function Gh(t, i, s, u) {
          var c = i & l,
            v = Oi(t);
          function b() {
            for (
              var E = -1,
                C = arguments.length,
                z = -1,
                N = u.length,
                W = L(N + C),
                G = this && this !== Ae && this instanceof b ? v : t;
              ++z < N;

            )
              W[z] = u[z];
            for (; C--; ) W[z++] = arguments[++E];
            return He(G, c ? s : this, W);
          }
          return b;
        }
        function ga(t) {
          return function (i, s, u) {
            return (
              u && typeof u != "number" && Fe(i, s, u) && (s = u = h),
              (i = Ut(i)),
              s === h ? ((s = i), (i = 0)) : (s = Ut(s)),
              (u = u === h ? (i < s ? 1 : -1) : Ut(u)),
              Rh(i, s, u, t)
            );
          };
        }
        function Cr(t) {
          return function (i, s) {
            return (
              (typeof i == "string" && typeof s == "string") ||
                ((i = it(i)), (s = it(s))),
              t(i, s)
            );
          };
        }
        function va(t, i, s, u, c, v, b, E, C, z) {
          var N = i & S,
            W = N ? b : h,
            G = N ? h : b,
            Y = N ? v : h,
            K = N ? h : v;
          (i |= N ? O : P), (i &= ~(N ? P : O)), i & p || (i &= ~(l | m));
          var te = [t, i, c, Y, W, K, G, E, C, z],
            X = s.apply(h, te);
          return jn(t) && Aa(X, te), (X.placeholder = u), Ba(X, t, i);
        }
        function Gn(t) {
          var i = Ue[t];
          return function (s, u) {
            if (
              ((s = it(s)), (u = u == null ? 0 : Le(ee(u), 292)), u && Bo(s))
            ) {
              var c = (fe(s) + "e").split("e"),
                v = i(c[0] + "e" + (+c[1] + u));
              return (
                (c = (fe(v) + "e").split("e")), +(c[0] + "e" + (+c[1] - u))
              );
            }
            return i(s);
          };
        }
        var qh =
          oi && 1 / ir(new oi([, -0]))[1] == Ie
            ? function (t) {
                return new oi(t);
              }
            : hs;
        function ma(t) {
          return function (i) {
            var s = Oe(i);
            return s == rt ? yn(i) : s == nt ? al(i) : Qf(i, t(i));
          };
        }
        function St(t, i, s, u, c, v, b, E) {
          var C = i & m;
          if (!C && typeof t != "function") throw new Je(A);
          var z = u ? u.length : 0;
          if (
            (z || ((i &= ~(O | P)), (u = c = h)),
            (b = b === h ? b : xe(ee(b), 0)),
            (E = E === h ? E : ee(E)),
            (z -= c ? c.length : 0),
            i & P)
          ) {
            var N = u,
              W = c;
            u = c = h;
          }
          var G = C ? h : $n(t),
            Y = [t, i, s, u, c, N, W, v, b, E];
          if (
            (G && sd(Y, G),
            (t = Y[0]),
            (i = Y[1]),
            (s = Y[2]),
            (u = Y[3]),
            (c = Y[4]),
            (E = Y[9] = Y[9] === h ? (C ? 0 : t.length) : xe(Y[9] - z, 0)),
            !E && i & (S | R) && (i &= ~(S | R)),
            !i || i == l)
          )
            var K = Wh(t, i, s);
          else
            i == S || i == R
              ? (K = Hh(t, i, E))
              : (i == O || i == (l | O)) && !c.length
              ? (K = Gh(t, i, s, u))
              : (K = Ur.apply(h, Y));
          var te = G ? Zo : Aa;
          return Ba(te(K, Y), t, i);
        }
        function ya(t, i, s, u) {
          return t === h || (at(t, si[s]) && !le.call(u, s)) ? i : t;
        }
        function Sa(t, i, s, u, c, v) {
          return (
            ve(t) && ve(i) && (v.set(i, t), Sr(t, i, h, Sa, v), v.delete(i)), t
          );
        }
        function Yh(t) {
          return Fi(t) ? h : t;
        }
        function ba(t, i, s, u, c, v) {
          var b = s & n,
            E = t.length,
            C = i.length;
          if (E != C && !(b && C > E)) return !1;
          var z = v.get(t),
            N = v.get(i);
          if (z && N) return z == i && N == t;
          var W = -1,
            G = !0,
            Y = s & a ? new qt() : h;
          for (v.set(t, i), v.set(i, t); ++W < E; ) {
            var K = t[W],
              te = i[W];
            if (u) var X = b ? u(te, K, W, i, t, v) : u(K, te, W, t, i, v);
            if (X !== h) {
              if (X) continue;
              G = !1;
              break;
            }
            if (Y) {
              if (
                !cn(i, function (ne, oe) {
                  if (!wi(Y, oe) && (K === ne || c(K, ne, s, u, v)))
                    return Y.push(oe);
                })
              ) {
                G = !1;
                break;
              }
            } else if (!(K === te || c(K, te, s, u, v))) {
              G = !1;
              break;
            }
          }
          return v.delete(t), v.delete(i), G;
        }
        function $h(t, i, s, u, c, v, b) {
          switch (s) {
            case Qt:
              if (t.byteLength != i.byteLength || t.byteOffset != i.byteOffset)
                return !1;
              (t = t.buffer), (i = i.buffer);
            case bi:
              return !(
                t.byteLength != i.byteLength || !v(new ur(t), new ur(i))
              );
            case _i:
            case gi:
            case vi:
              return at(+t, +i);
            case Ki:
              return t.name == i.name && t.message == i.message;
            case mi:
            case yi:
              return t == i + "";
            case rt:
              var E = yn;
            case nt:
              var C = u & n;
              if ((E || (E = ir), t.size != i.size && !C)) return !1;
              var z = b.get(t);
              if (z) return z == i;
              (u |= a), b.set(t, i);
              var N = ba(E(t), E(i), u, c, v, b);
              return b.delete(t), N;
            case ji:
              if (Ci) return Ci.call(t) == Ci.call(i);
          }
          return !1;
        }
        function Vh(t, i, s, u, c, v) {
          var b = s & n,
            E = qn(t),
            C = E.length,
            z = qn(i),
            N = z.length;
          if (C != N && !b) return !1;
          for (var W = C; W--; ) {
            var G = E[W];
            if (!(b ? G in i : le.call(i, G))) return !1;
          }
          var Y = v.get(t),
            K = v.get(i);
          if (Y && K) return Y == i && K == t;
          var te = !0;
          v.set(t, i), v.set(i, t);
          for (var X = b; ++W < C; ) {
            G = E[W];
            var ne = t[G],
              oe = i[G];
            if (u) var $e = b ? u(oe, ne, G, i, t, v) : u(ne, oe, G, t, i, v);
            if (!($e === h ? ne === oe || c(ne, oe, s, u, v) : $e)) {
              te = !1;
              break;
            }
            X || (X = G == "constructor");
          }
          if (te && !X) {
            var De = t.constructor,
              Ve = i.constructor;
            De != Ve &&
              "constructor" in t &&
              "constructor" in i &&
              !(
                typeof De == "function" &&
                De instanceof De &&
                typeof Ve == "function" &&
                Ve instanceof Ve
              ) &&
              (te = !1);
          }
          return v.delete(t), v.delete(i), te;
        }
        function bt(t) {
          return Zn(Ca(t, h, Fa), t + "");
        }
        function qn(t) {
          return Wo(t, Ce, Kn);
        }
        function Yn(t) {
          return Wo(t, Ne, wa);
        }
        var $n = pr
          ? function (t) {
              return pr.get(t);
            }
          : hs;
        function Rr(t) {
          for (
            var i = t.name + "", s = ai[i], u = le.call(ai, i) ? s.length : 0;
            u--;

          ) {
            var c = s[u],
              v = c.func;
            if (v == null || v == t) return c.name;
          }
          return i;
        }
        function hi(t) {
          var i = le.call(_, "placeholder") ? _ : t;
          return i.placeholder;
        }
        function V() {
          var t = _.iteratee || fs;
          return (
            (t = t === fs ? qo : t),
            arguments.length ? t(arguments[0], arguments[1]) : t
          );
        }
        function Ar(t, i) {
          var s = t.__data__;
          return td(i) ? s[typeof i == "string" ? "string" : "hash"] : s.map;
        }
        function Vn(t) {
          for (var i = Ce(t), s = i.length; s--; ) {
            var u = i[s],
              c = t[u];
            i[s] = [u, c, xa(c)];
          }
          return i;
        }
        function Vt(t, i) {
          var s = nl(t, i);
          return Go(s) ? s : h;
        }
        function Kh(t) {
          var i = le.call(t, Ht),
            s = t[Ht];
          try {
            t[Ht] = h;
            var u = !0;
          } catch (v) {}
          var c = or.call(t);
          return u && (i ? (t[Ht] = s) : delete t[Ht]), c;
        }
        var Kn = bn
            ? function (t) {
                return t == null
                  ? []
                  : ((t = he(t)),
                    Ct(bn(t), function (i) {
                      return Ro.call(t, i);
                    }));
              }
            : ds,
          wa = bn
            ? function (t) {
                for (var i = []; t; ) Rt(i, Kn(t)), (t = fr(t));
                return i;
              }
            : ds,
          Oe = ke;
        ((wn && Oe(new wn(new ArrayBuffer(1))) != Qt) ||
          (Ui && Oe(new Ui()) != rt) ||
          (En && Oe(En.resolve()) != Ds) ||
          (oi && Oe(new oi()) != nt) ||
          (xi && Oe(new xi()) != Si)) &&
          (Oe = function (t) {
            var i = ke(t),
              s = i == gt ? t.constructor : h,
              u = s ? Kt(s) : "";
            if (u)
              switch (u) {
                case Bl:
                  return Qt;
                case Il:
                  return rt;
                case Ll:
                  return Ds;
                case Ol:
                  return nt;
                case Pl:
                  return Si;
              }
            return i;
          });
        function Xh(t, i, s) {
          for (var u = -1, c = s.length; ++u < c; ) {
            var v = s[u],
              b = v.size;
            switch (v.type) {
              case "drop":
                t += b;
                break;
              case "dropRight":
                i -= b;
                break;
              case "take":
                i = Le(i, t + b);
                break;
              case "takeRight":
                t = xe(t, i - b);
                break;
            }
          }
          return {
            start: t,
            end: i,
          };
        }
        function jh(t) {
          var i = t.match(tf);
          return i ? i[1].split(rf) : [];
        }
        function Ea(t, i, s) {
          i = Ot(i, t);
          for (var u = -1, c = i.length, v = !1; ++u < c; ) {
            var b = ct(i[u]);
            if (!(v = t != null && s(t, b))) break;
            t = t[b];
          }
          return v || ++u != c
            ? v
            : ((c = t == null ? 0 : t.length),
              !!c && Fr(c) && wt(b, c) && (Q(t) || Xt(t)));
        }
        function Jh(t) {
          var i = t.length,
            s = new t.constructor(i);
          return (
            i &&
              typeof t[0] == "string" &&
              le.call(t, "index") &&
              ((s.index = t.index), (s.input = t.input)),
            s
          );
        }
        function Ua(t) {
          return typeof t.constructor == "function" && !Pi(t) ? ui(fr(t)) : {};
        }
        function Zh(t, i, s) {
          var u = t.constructor;
          switch (i) {
            case bi:
              return Wn(t);
            case _i:
            case gi:
              return new u(+t);
            case Qt:
              return kh(t, s);
            case $r:
            case Vr:
            case Kr:
            case Xr:
            case jr:
            case Jr:
            case Zr:
            case Qr:
            case en:
              return oa(t, s);
            case rt:
              return new u();
            case vi:
            case yi:
              return new u(t);
            case mi:
              return Fh(t);
            case nt:
              return new u();
            case ji:
              return Dh(t);
          }
        }
        function Qh(t, i) {
          var s = i.length;
          if (!s) return t;
          var u = s - 1;
          return (
            (i[u] = (s > 1 ? "& " : "") + i[u]),
            (i = i.join(s > 2 ? ", " : " ")),
            t.replace(ef, "{\n/* [wrapped with " + i + "] */\n")
          );
        }
        function ed(t) {
          return Q(t) || Xt(t) || !!(Ao && t && t[Ao]);
        }
        function wt(t, i) {
          var s = typeof t;
          return (
            (i = i == null ? Ke : i),
            !!i &&
              (s == "number" || (s != "symbol" && df.test(t))) &&
              t > -1 &&
              t % 1 == 0 &&
              t < i
          );
        }
        function Fe(t, i, s) {
          if (!ve(s)) return !1;
          var u = typeof i;
          return (
            u == "number" ? ze(s) && wt(i, s.length) : u == "string" && i in s
          )
            ? at(s[i], t)
            : !1;
        }
        function Xn(t, i) {
          if (Q(t)) return !1;
          var s = typeof t;
          return s == "number" ||
            s == "symbol" ||
            s == "boolean" ||
            t == null ||
            Ye(t)
            ? !0
            : ju.test(t) || !Xu.test(t) || (i != null && t in he(i));
        }
        function td(t) {
          var i = typeof t;
          return i == "string" ||
            i == "number" ||
            i == "symbol" ||
            i == "boolean"
            ? t !== "__proto__"
            : t === null;
        }
        function jn(t) {
          var i = Rr(t),
            s = _[i];
          if (typeof s != "function" || !(i in se.prototype)) return !1;
          if (t === s) return !0;
          var u = $n(s);
          return !!u && t === u[0];
        }
        function id(t) {
          return !!xo && xo in t;
        }
        var rd = nr ? Et : cs;
        function Pi(t) {
          var i = t && t.constructor,
            s = (typeof i == "function" && i.prototype) || si;
          return t === s;
        }
        function xa(t) {
          return t === t && !ve(t);
        }
        function Ta(t, i) {
          return function (s) {
            return s == null ? !1 : s[t] === i && (i !== h || t in he(s));
          };
        }
        function nd(t) {
          var i = Pr(t, function (u) {
              return s.size === B && s.clear(), u;
            }),
            s = i.cache;
          return i;
        }
        function sd(t, i) {
          var s = t[1],
            u = i[1],
            c = s | u,
            v = c < (l | m | y),
            b =
              (u == y && s == S) ||
              (u == y && s == F && t[7].length <= i[8]) ||
              (u == (y | F) && i[7].length <= i[8] && s == S);
          if (!(v || b)) return t;
          u & l && ((t[2] = i[2]), (c |= s & l ? 0 : p));
          var E = i[3];
          if (E) {
            var C = t[3];
            (t[3] = C ? ua(C, E, i[4]) : E), (t[4] = C ? At(t[3], w) : i[4]);
          }
          return (
            (E = i[5]),
            E &&
              ((C = t[5]),
              (t[5] = C ? fa(C, E, i[6]) : E),
              (t[6] = C ? At(t[5], w) : i[6])),
            (E = i[7]),
            E && (t[7] = E),
            u & y && (t[8] = t[8] == null ? i[8] : Le(t[8], i[8])),
            t[9] == null && (t[9] = i[9]),
            (t[0] = i[0]),
            (t[1] = c),
            t
          );
        }
        function od(t) {
          var i = [];
          if (t != null) for (var s in he(t)) i.push(s);
          return i;
        }
        function ad(t) {
          return or.call(t);
        }
        function Ca(t, i, s) {
          return (
            (i = xe(i === h ? t.length - 1 : i, 0)),
            function () {
              for (
                var u = arguments, c = -1, v = xe(u.length - i, 0), b = L(v);
                ++c < v;

              )
                b[c] = u[i + c];
              c = -1;
              for (var E = L(i + 1); ++c < i; ) E[c] = u[c];
              return (E[i] = s(b)), He(t, this, E);
            }
          );
        }
        function Ra(t, i) {
          return i.length < 2 ? t : $t(t, et(i, 0, -1));
        }
        function ud(t, i) {
          for (var s = t.length, u = Le(i.length, s), c = Me(t); u--; ) {
            var v = i[u];
            t[u] = wt(v, s) ? c[v] : h;
          }
          return t;
        }
        function Jn(t, i) {
          if (
            !(i === "constructor" && typeof t[i] == "function") &&
            i != "__proto__"
          )
            return t[i];
        }
        var Aa = Ia(Zo),
          ki =
            El ||
            function (t, i) {
              return Ae.setTimeout(t, i);
            },
          Zn = Ia(Ih);
        function Ba(t, i, s) {
          var u = i + "";
          return Zn(t, Qh(u, fd(jh(u), s)));
        }
        function Ia(t) {
          var i = 0,
            s = 0;
          return function () {
            var u = Cl(),
              c = de - (u - s);
            if (((s = u), c > 0)) {
              if (++i >= _e) return arguments[0];
            } else i = 0;
            return t.apply(h, arguments);
          };
        }
        function Br(t, i) {
          var s = -1,
            u = t.length,
            c = u - 1;
          for (i = i === h ? u : i; ++s < i; ) {
            var v = Pn(s, c),
              b = t[v];
            (t[v] = t[s]), (t[s] = b);
          }
          return (t.length = i), t;
        }
        var La = nd(function (t) {
          var i = [];
          return (
            t.charCodeAt(0) === 46 && i.push(""),
            t.replace(Ju, function (s, u, c, v) {
              i.push(c ? v.replace(of, "$1") : u || s);
            }),
            i
          );
        });
        function ct(t) {
          if (typeof t == "string" || Ye(t)) return t;
          var i = t + "";
          return i == "0" && 1 / t == -Ie ? "-0" : i;
        }
        function Kt(t) {
          if (t != null) {
            try {
              return sr.call(t);
            } catch (i) {}
            try {
              return t + "";
            } catch (i) {}
          }
          return "";
        }
        function fd(t, i) {
          return (
            je(ku, function (s) {
              var u = "_." + s[0];
              i & s[1] && !er(t, u) && t.push(u);
            }),
            t.sort()
          );
        }
        function Oa(t) {
          if (t instanceof se) return t.clone();
          var i = new Ze(t.__wrapped__, t.__chain__);
          return (
            (i.__actions__ = Me(t.__actions__)),
            (i.__index__ = t.__index__),
            (i.__values__ = t.__values__),
            i
          );
        }
        function ld(t, i, s) {
          (s ? Fe(t, i, s) : i === h) ? (i = 1) : (i = xe(ee(i), 0));
          var u = t == null ? 0 : t.length;
          if (!u || i < 1) return [];
          for (var c = 0, v = 0, b = L(dr(u / i)); c < u; )
            b[v++] = et(t, c, (c += i));
          return b;
        }
        function hd(t) {
          for (
            var i = -1, s = t == null ? 0 : t.length, u = 0, c = [];
            ++i < s;

          ) {
            var v = t[i];
            v && (c[u++] = v);
          }
          return c;
        }
        function dd() {
          var t = arguments.length;
          if (!t) return [];
          for (var i = L(t - 1), s = arguments[0], u = t; u--; )
            i[u - 1] = arguments[u];
          return Rt(Q(s) ? Me(s) : [s], Be(i, 1));
        }
        var cd = ie(function (t, i) {
            return ye(t) ? Ai(t, Be(i, 1, ye, !0)) : [];
          }),
          pd = ie(function (t, i) {
            var s = tt(i);
            return (
              ye(s) && (s = h), ye(t) ? Ai(t, Be(i, 1, ye, !0), V(s, 2)) : []
            );
          }),
          _d = ie(function (t, i) {
            var s = tt(i);
            return ye(s) && (s = h), ye(t) ? Ai(t, Be(i, 1, ye, !0), h, s) : [];
          });
        function gd(t, i, s) {
          var u = t == null ? 0 : t.length;
          return u
            ? ((i = s || i === h ? 1 : ee(i)), et(t, i < 0 ? 0 : i, u))
            : [];
        }
        function vd(t, i, s) {
          var u = t == null ? 0 : t.length;
          return u
            ? ((i = s || i === h ? 1 : ee(i)),
              (i = u - i),
              et(t, 0, i < 0 ? 0 : i))
            : [];
        }
        function md(t, i) {
          return t && t.length ? wr(t, V(i, 3), !0, !0) : [];
        }
        function yd(t, i) {
          return t && t.length ? wr(t, V(i, 3), !0) : [];
        }
        function Sd(t, i, s, u) {
          var c = t == null ? 0 : t.length;
          return c
            ? (s && typeof s != "number" && Fe(t, i, s) && ((s = 0), (u = c)),
              dh(t, i, s, u))
            : [];
        }
        function Pa(t, i, s) {
          var u = t == null ? 0 : t.length;
          if (!u) return -1;
          var c = s == null ? 0 : ee(s);
          return c < 0 && (c = xe(u + c, 0)), tr(t, V(i, 3), c);
        }
        function ka(t, i, s) {
          var u = t == null ? 0 : t.length;
          if (!u) return -1;
          var c = u - 1;
          return (
            s !== h && ((c = ee(s)), (c = s < 0 ? xe(u + c, 0) : Le(c, u - 1))),
            tr(t, V(i, 3), c, !0)
          );
        }
        function Fa(t) {
          var i = t == null ? 0 : t.length;
          return i ? Be(t, 1) : [];
        }
        function bd(t) {
          var i = t == null ? 0 : t.length;
          return i ? Be(t, Ie) : [];
        }
        function wd(t, i) {
          var s = t == null ? 0 : t.length;
          return s ? ((i = i === h ? 1 : ee(i)), Be(t, i)) : [];
        }
        function Ed(t) {
          for (var i = -1, s = t == null ? 0 : t.length, u = {}; ++i < s; ) {
            var c = t[i];
            u[c[0]] = c[1];
          }
          return u;
        }
        function Da(t) {
          return t && t.length ? t[0] : h;
        }
        function Ud(t, i, s) {
          var u = t == null ? 0 : t.length;
          if (!u) return -1;
          var c = s == null ? 0 : ee(s);
          return c < 0 && (c = xe(u + c, 0)), ti(t, i, c);
        }
        function xd(t) {
          var i = t == null ? 0 : t.length;
          return i ? et(t, 0, -1) : [];
        }
        var Td = ie(function (t) {
            var i = ge(t, zn);
            return i.length && i[0] === t[0] ? An(i) : [];
          }),
          Cd = ie(function (t) {
            var i = tt(t),
              s = ge(t, zn);
            return (
              i === tt(s) ? (i = h) : s.pop(),
              s.length && s[0] === t[0] ? An(s, V(i, 2)) : []
            );
          }),
          Rd = ie(function (t) {
            var i = tt(t),
              s = ge(t, zn);
            return (
              (i = typeof i == "function" ? i : h),
              i && s.pop(),
              s.length && s[0] === t[0] ? An(s, h, i) : []
            );
          });
        function Ad(t, i) {
          return t == null ? "" : xl.call(t, i);
        }
        function tt(t) {
          var i = t == null ? 0 : t.length;
          return i ? t[i - 1] : h;
        }
        function Bd(t, i, s) {
          var u = t == null ? 0 : t.length;
          if (!u) return -1;
          var c = u;
          return (
            s !== h && ((c = ee(s)), (c = c < 0 ? xe(u + c, 0) : Le(c, u - 1))),
            i === i ? fl(t, i, c) : tr(t, vo, c, !0)
          );
        }
        function Id(t, i) {
          return t && t.length ? Ko(t, ee(i)) : h;
        }
        var Ld = ie(Ma);
        function Ma(t, i) {
          return t && t.length && i && i.length ? On(t, i) : t;
        }
        function Od(t, i, s) {
          return t && t.length && i && i.length ? On(t, i, V(s, 2)) : t;
        }
        function Pd(t, i, s) {
          return t && t.length && i && i.length ? On(t, i, h, s) : t;
        }
        var kd = bt(function (t, i) {
          var s = t == null ? 0 : t.length,
            u = xn(t, i);
          return (
            Jo(
              t,
              ge(i, function (c) {
                return wt(c, s) ? +c : c;
              }).sort(aa)
            ),
            u
          );
        });
        function Fd(t, i) {
          var s = [];
          if (!(t && t.length)) return s;
          var u = -1,
            c = [],
            v = t.length;
          for (i = V(i, 3); ++u < v; ) {
            var b = t[u];
            i(b, u, t) && (s.push(b), c.push(u));
          }
          return Jo(t, c), s;
        }
        function Qn(t) {
          return t == null ? t : Al.call(t);
        }
        function Dd(t, i, s) {
          var u = t == null ? 0 : t.length;
          return u
            ? (s && typeof s != "number" && Fe(t, i, s)
                ? ((i = 0), (s = u))
                : ((i = i == null ? 0 : ee(i)), (s = s === h ? u : ee(s))),
              et(t, i, s))
            : [];
        }
        function Md(t, i) {
          return br(t, i);
        }
        function zd(t, i, s) {
          return Fn(t, i, V(s, 2));
        }
        function Nd(t, i) {
          var s = t == null ? 0 : t.length;
          if (s) {
            var u = br(t, i);
            if (u < s && at(t[u], i)) return u;
          }
          return -1;
        }
        function Wd(t, i) {
          return br(t, i, !0);
        }
        function Hd(t, i, s) {
          return Fn(t, i, V(s, 2), !0);
        }
        function Gd(t, i) {
          var s = t == null ? 0 : t.length;
          if (s) {
            var u = br(t, i, !0) - 1;
            if (at(t[u], i)) return u;
          }
          return -1;
        }
        function qd(t) {
          return t && t.length ? Qo(t) : [];
        }
        function Yd(t, i) {
          return t && t.length ? Qo(t, V(i, 2)) : [];
        }
        function $d(t) {
          var i = t == null ? 0 : t.length;
          return i ? et(t, 1, i) : [];
        }
        function Vd(t, i, s) {
          return t && t.length
            ? ((i = s || i === h ? 1 : ee(i)), et(t, 0, i < 0 ? 0 : i))
            : [];
        }
        function Kd(t, i, s) {
          var u = t == null ? 0 : t.length;
          return u
            ? ((i = s || i === h ? 1 : ee(i)),
              (i = u - i),
              et(t, i < 0 ? 0 : i, u))
            : [];
        }
        function Xd(t, i) {
          return t && t.length ? wr(t, V(i, 3), !1, !0) : [];
        }
        function jd(t, i) {
          return t && t.length ? wr(t, V(i, 3)) : [];
        }
        var Jd = ie(function (t) {
            return Lt(Be(t, 1, ye, !0));
          }),
          Zd = ie(function (t) {
            var i = tt(t);
            return ye(i) && (i = h), Lt(Be(t, 1, ye, !0), V(i, 2));
          }),
          Qd = ie(function (t) {
            var i = tt(t);
            return (
              (i = typeof i == "function" ? i : h), Lt(Be(t, 1, ye, !0), h, i)
            );
          });
        function ec(t) {
          return t && t.length ? Lt(t) : [];
        }
        function tc(t, i) {
          return t && t.length ? Lt(t, V(i, 2)) : [];
        }
        function ic(t, i) {
          return (
            (i = typeof i == "function" ? i : h),
            t && t.length ? Lt(t, h, i) : []
          );
        }
        function es(t) {
          if (!(t && t.length)) return [];
          var i = 0;
          return (
            (t = Ct(t, function (s) {
              if (ye(s)) return (i = xe(s.length, i)), !0;
            })),
            vn(i, function (s) {
              return ge(t, pn(s));
            })
          );
        }
        function za(t, i) {
          if (!(t && t.length)) return [];
          var s = es(t);
          return i == null
            ? s
            : ge(s, function (u) {
                return He(i, h, u);
              });
        }
        var rc = ie(function (t, i) {
            return ye(t) ? Ai(t, i) : [];
          }),
          nc = ie(function (t) {
            return Mn(Ct(t, ye));
          }),
          sc = ie(function (t) {
            var i = tt(t);
            return ye(i) && (i = h), Mn(Ct(t, ye), V(i, 2));
          }),
          oc = ie(function (t) {
            var i = tt(t);
            return (i = typeof i == "function" ? i : h), Mn(Ct(t, ye), h, i);
          }),
          ac = ie(es);
        function uc(t, i) {
          return ra(t || [], i || [], Ri);
        }
        function fc(t, i) {
          return ra(t || [], i || [], Li);
        }
        var lc = ie(function (t) {
          var i = t.length,
            s = i > 1 ? t[i - 1] : h;
          return (s = typeof s == "function" ? (t.pop(), s) : h), za(t, s);
        });
        function Na(t) {
          var i = _(t);
          return (i.__chain__ = !0), i;
        }
        function hc(t, i) {
          return i(t), t;
        }
        function Ir(t, i) {
          return i(t);
        }
        var dc = bt(function (t) {
          var i = t.length,
            s = i ? t[0] : 0,
            u = this.__wrapped__,
            c = function (v) {
              return xn(v, t);
            };
          return i > 1 ||
            this.__actions__.length ||
            !(u instanceof se) ||
            !wt(s)
            ? this.thru(c)
            : ((u = u.slice(s, +s + (i ? 1 : 0))),
              u.__actions__.push({
                func: Ir,
                args: [c],
                thisArg: h,
              }),
              new Ze(u, this.__chain__).thru(function (v) {
                return i && !v.length && v.push(h), v;
              }));
        });
        function cc() {
          return Na(this);
        }
        function pc() {
          return new Ze(this.value(), this.__chain__);
        }
        function _c() {
          this.__values__ === h && (this.__values__ = eu(this.value()));
          var t = this.__index__ >= this.__values__.length,
            i = t ? h : this.__values__[this.__index__++];
          return {
            done: t,
            value: i,
          };
        }
        function gc() {
          return this;
        }
        function vc(t) {
          for (var i, s = this; s instanceof gr; ) {
            var u = Oa(s);
            (u.__index__ = 0),
              (u.__values__ = h),
              i ? (c.__wrapped__ = u) : (i = u);
            var c = u;
            s = s.__wrapped__;
          }
          return (c.__wrapped__ = t), i;
        }
        function mc() {
          var t = this.__wrapped__;
          if (t instanceof se) {
            var i = t;
            return (
              this.__actions__.length && (i = new se(this)),
              (i = i.reverse()),
              i.__actions__.push({
                func: Ir,
                args: [Qn],
                thisArg: h,
              }),
              new Ze(i, this.__chain__)
            );
          }
          return this.thru(Qn);
        }
        function yc() {
          return ia(this.__wrapped__, this.__actions__);
        }
        var Sc = Er(function (t, i, s) {
          le.call(t, s) ? ++t[s] : yt(t, s, 1);
        });
        function bc(t, i, s) {
          var u = Q(t) ? _o : hh;
          return s && Fe(t, i, s) && (i = h), u(t, V(i, 3));
        }
        function wc(t, i) {
          var s = Q(t) ? Ct : zo;
          return s(t, V(i, 3));
        }
        var Ec = ca(Pa),
          Uc = ca(ka);
        function xc(t, i) {
          return Be(Lr(t, i), 1);
        }
        function Tc(t, i) {
          return Be(Lr(t, i), Ie);
        }
        function Cc(t, i, s) {
          return (s = s === h ? 1 : ee(s)), Be(Lr(t, i), s);
        }
        function Wa(t, i) {
          var s = Q(t) ? je : It;
          return s(t, V(i, 3));
        }
        function Ha(t, i) {
          var s = Q(t) ? $f : Mo;
          return s(t, V(i, 3));
        }
        var Rc = Er(function (t, i, s) {
          le.call(t, s) ? t[s].push(i) : yt(t, s, [i]);
        });
        function Ac(t, i, s, u) {
          (t = ze(t) ? t : ci(t)), (s = s && !u ? ee(s) : 0);
          var c = t.length;
          return (
            s < 0 && (s = xe(c + s, 0)),
            Dr(t) ? s <= c && t.indexOf(i, s) > -1 : !!c && ti(t, i, s) > -1
          );
        }
        var Bc = ie(function (t, i, s) {
            var u = -1,
              c = typeof i == "function",
              v = ze(t) ? L(t.length) : [];
            return (
              It(t, function (b) {
                v[++u] = c ? He(i, b, s) : Bi(b, i, s);
              }),
              v
            );
          }),
          Ic = Er(function (t, i, s) {
            yt(t, s, i);
          });
        function Lr(t, i) {
          var s = Q(t) ? ge : Yo;
          return s(t, V(i, 3));
        }
        function Lc(t, i, s, u) {
          return t == null
            ? []
            : (Q(i) || (i = i == null ? [] : [i]),
              (s = u ? h : s),
              Q(s) || (s = s == null ? [] : [s]),
              Xo(t, i, s));
        }
        var Oc = Er(
          function (t, i, s) {
            t[s ? 0 : 1].push(i);
          },
          function () {
            return [[], []];
          }
        );
        function Pc(t, i, s) {
          var u = Q(t) ? dn : yo,
            c = arguments.length < 3;
          return u(t, V(i, 4), s, c, It);
        }
        function kc(t, i, s) {
          var u = Q(t) ? Vf : yo,
            c = arguments.length < 3;
          return u(t, V(i, 4), s, c, Mo);
        }
        function Fc(t, i) {
          var s = Q(t) ? Ct : zo;
          return s(t, kr(V(i, 3)));
        }
        function Dc(t) {
          var i = Q(t) ? Po : Ah;
          return i(t);
        }
        function Mc(t, i, s) {
          (s ? Fe(t, i, s) : i === h) ? (i = 1) : (i = ee(i));
          var u = Q(t) ? oh : Bh;
          return u(t, i);
        }
        function zc(t) {
          var i = Q(t) ? ah : Lh;
          return i(t);
        }
        function Nc(t) {
          if (t == null) return 0;
          if (ze(t)) return Dr(t) ? ri(t) : t.length;
          var i = Oe(t);
          return i == rt || i == nt ? t.size : In(t).length;
        }
        function Wc(t, i, s) {
          var u = Q(t) ? cn : Oh;
          return s && Fe(t, i, s) && (i = h), u(t, V(i, 3));
        }
        var Hc = ie(function (t, i) {
            if (t == null) return [];
            var s = i.length;
            return (
              s > 1 && Fe(t, i[0], i[1])
                ? (i = [])
                : s > 2 && Fe(i[0], i[1], i[2]) && (i = [i[0]]),
              Xo(t, Be(i, 1), [])
            );
          }),
          Or =
            wl ||
            function () {
              return Ae.Date.now();
            };
        function Gc(t, i) {
          if (typeof i != "function") throw new Je(A);
          return (
            (t = ee(t)),
            function () {
              if (--t < 1) return i.apply(this, arguments);
            }
          );
        }
        function Ga(t, i, s) {
          return (
            (i = s ? h : i),
            (i = t && i == null ? t.length : i),
            St(t, y, h, h, h, h, i)
          );
        }
        function qa(t, i) {
          var s;
          if (typeof i != "function") throw new Je(A);
          return (
            (t = ee(t)),
            function () {
              return (
                --t > 0 && (s = i.apply(this, arguments)), t <= 1 && (i = h), s
              );
            }
          );
        }
        var ts = ie(function (t, i, s) {
            var u = l;
            if (s.length) {
              var c = At(s, hi(ts));
              u |= O;
            }
            return St(t, u, i, s, c);
          }),
          Ya = ie(function (t, i, s) {
            var u = l | m;
            if (s.length) {
              var c = At(s, hi(Ya));
              u |= O;
            }
            return St(i, u, t, s, c);
          });
        function $a(t, i, s) {
          i = s ? h : i;
          var u = St(t, S, h, h, h, h, h, i);
          return (u.placeholder = $a.placeholder), u;
        }
        function Va(t, i, s) {
          i = s ? h : i;
          var u = St(t, R, h, h, h, h, h, i);
          return (u.placeholder = Va.placeholder), u;
        }
        function Ka(t, i, s) {
          var u,
            c,
            v,
            b,
            E,
            C,
            z = 0,
            N = !1,
            W = !1,
            G = !0;
          if (typeof t != "function") throw new Je(A);
          (i = it(i) || 0),
            ve(s) &&
              ((N = !!s.leading),
              (W = "maxWait" in s),
              (v = W ? xe(it(s.maxWait) || 0, i) : v),
              (G = "trailing" in s ? !!s.trailing : G));
          function Y(Se) {
            var ut = u,
              xt = c;
            return (u = c = h), (z = Se), (b = t.apply(xt, ut)), b;
          }
          function K(Se) {
            return (z = Se), (E = ki(ne, i)), N ? Y(Se) : b;
          }
          function te(Se) {
            var ut = Se - C,
              xt = Se - z,
              cu = i - ut;
            return W ? Le(cu, v - xt) : cu;
          }
          function X(Se) {
            var ut = Se - C,
              xt = Se - z;
            return C === h || ut >= i || ut < 0 || (W && xt >= v);
          }
          function ne() {
            var Se = Or();
            if (X(Se)) return oe(Se);
            E = ki(ne, te(Se));
          }
          function oe(Se) {
            return (E = h), G && u ? Y(Se) : ((u = c = h), b);
          }
          function $e() {
            E !== h && na(E), (z = 0), (u = C = c = E = h);
          }
          function De() {
            return E === h ? b : oe(Or());
          }
          function Ve() {
            var Se = Or(),
              ut = X(Se);
            if (((u = arguments), (c = this), (C = Se), ut)) {
              if (E === h) return K(C);
              if (W) return na(E), (E = ki(ne, i)), Y(C);
            }
            return E === h && (E = ki(ne, i)), b;
          }
          return (Ve.cancel = $e), (Ve.flush = De), Ve;
        }
        var qc = ie(function (t, i) {
            return Do(t, 1, i);
          }),
          Yc = ie(function (t, i, s) {
            return Do(t, it(i) || 0, s);
          });
        function $c(t) {
          return St(t, I);
        }
        function Pr(t, i) {
          if (typeof t != "function" || (i != null && typeof i != "function"))
            throw new Je(A);
          var s = function () {
            var u = arguments,
              c = i ? i.apply(this, u) : u[0],
              v = s.cache;
            if (v.has(c)) return v.get(c);
            var b = t.apply(this, u);
            return (s.cache = v.set(c, b) || v), b;
          };
          return (s.cache = new (Pr.Cache || mt)()), s;
        }
        Pr.Cache = mt;
        function kr(t) {
          if (typeof t != "function") throw new Je(A);
          return function () {
            var i = arguments;
            switch (i.length) {
              case 0:
                return !t.call(this);
              case 1:
                return !t.call(this, i[0]);
              case 2:
                return !t.call(this, i[0], i[1]);
              case 3:
                return !t.call(this, i[0], i[1], i[2]);
            }
            return !t.apply(this, i);
          };
        }
        function Vc(t) {
          return qa(2, t);
        }
        var Kc = Ph(function (t, i) {
            i =
              i.length == 1 && Q(i[0])
                ? ge(i[0], Ge(V()))
                : ge(Be(i, 1), Ge(V()));
            var s = i.length;
            return ie(function (u) {
              for (var c = -1, v = Le(u.length, s); ++c < v; )
                u[c] = i[c].call(this, u[c]);
              return He(t, this, u);
            });
          }),
          is = ie(function (t, i) {
            var s = At(i, hi(is));
            return St(t, O, h, i, s);
          }),
          Xa = ie(function (t, i) {
            var s = At(i, hi(Xa));
            return St(t, P, h, i, s);
          }),
          Xc = bt(function (t, i) {
            return St(t, F, h, h, h, i);
          });
        function jc(t, i) {
          if (typeof t != "function") throw new Je(A);
          return (i = i === h ? i : ee(i)), ie(t, i);
        }
        function Jc(t, i) {
          if (typeof t != "function") throw new Je(A);
          return (
            (i = i == null ? 0 : xe(ee(i), 0)),
            ie(function (s) {
              var u = s[i],
                c = Pt(s, 0, i);
              return u && Rt(c, u), He(t, this, c);
            })
          );
        }
        function Zc(t, i, s) {
          var u = !0,
            c = !0;
          if (typeof t != "function") throw new Je(A);
          return (
            ve(s) &&
              ((u = "leading" in s ? !!s.leading : u),
              (c = "trailing" in s ? !!s.trailing : c)),
            Ka(t, i, {
              leading: u,
              maxWait: i,
              trailing: c,
            })
          );
        }
        function Qc(t) {
          return Ga(t, 1);
        }
        function ep(t, i) {
          return is(Nn(i), t);
        }
        function tp() {
          if (!arguments.length) return [];
          var t = arguments[0];
          return Q(t) ? t : [t];
        }
        function ip(t) {
          return Qe(t, r);
        }
        function rp(t, i) {
          return (i = typeof i == "function" ? i : h), Qe(t, r, i);
        }
        function np(t) {
          return Qe(t, k | r);
        }
        function sp(t, i) {
          return (i = typeof i == "function" ? i : h), Qe(t, k | r, i);
        }
        function op(t, i) {
          return i == null || Fo(t, i, Ce(i));
        }
        function at(t, i) {
          return t === i || (t !== t && i !== i);
        }
        var ap = Cr(Rn),
          up = Cr(function (t, i) {
            return t >= i;
          }),
          Xt = Ho(
            (function () {
              return arguments;
            })()
          )
            ? Ho
            : function (t) {
                return me(t) && le.call(t, "callee") && !Ro.call(t, "callee");
              },
          Q = L.isArray,
          fp = uo ? Ge(uo) : vh;
        function ze(t) {
          return t != null && Fr(t.length) && !Et(t);
        }
        function ye(t) {
          return me(t) && ze(t);
        }
        function lp(t) {
          return t === !0 || t === !1 || (me(t) && ke(t) == _i);
        }
        var kt = Ul || cs,
          hp = fo ? Ge(fo) : mh;
        function dp(t) {
          return me(t) && t.nodeType === 1 && !Fi(t);
        }
        function cp(t) {
          if (t == null) return !0;
          if (
            ze(t) &&
            (Q(t) ||
              typeof t == "string" ||
              typeof t.splice == "function" ||
              kt(t) ||
              di(t) ||
              Xt(t))
          )
            return !t.length;
          var i = Oe(t);
          if (i == rt || i == nt) return !t.size;
          if (Pi(t)) return !In(t).length;
          for (var s in t) if (le.call(t, s)) return !1;
          return !0;
        }
        function pp(t, i) {
          return Ii(t, i);
        }
        function _p(t, i, s) {
          s = typeof s == "function" ? s : h;
          var u = s ? s(t, i) : h;
          return u === h ? Ii(t, i, h, s) : !!u;
        }
        function rs(t) {
          if (!me(t)) return !1;
          var i = ke(t);
          return (
            i == Ki ||
            i == Du ||
            (typeof t.message == "string" &&
              typeof t.name == "string" &&
              !Fi(t))
          );
        }
        function gp(t) {
          return typeof t == "number" && Bo(t);
        }
        function Et(t) {
          if (!ve(t)) return !1;
          var i = ke(t);
          return i == Xi || i == Fs || i == Fu || i == zu;
        }
        function ja(t) {
          return typeof t == "number" && t == ee(t);
        }
        function Fr(t) {
          return typeof t == "number" && t > -1 && t % 1 == 0 && t <= Ke;
        }
        function ve(t) {
          var i = typeof t;
          return t != null && (i == "object" || i == "function");
        }
        function me(t) {
          return t != null && typeof t == "object";
        }
        var Ja = lo ? Ge(lo) : Sh;
        function vp(t, i) {
          return t === i || Bn(t, i, Vn(i));
        }
        function mp(t, i, s) {
          return (s = typeof s == "function" ? s : h), Bn(t, i, Vn(i), s);
        }
        function yp(t) {
          return Za(t) && t != +t;
        }
        function Sp(t) {
          if (rd(t)) throw new Z(x);
          return Go(t);
        }
        function bp(t) {
          return t === null;
        }
        function wp(t) {
          return t == null;
        }
        function Za(t) {
          return typeof t == "number" || (me(t) && ke(t) == vi);
        }
        function Fi(t) {
          if (!me(t) || ke(t) != gt) return !1;
          var i = fr(t);
          if (i === null) return !0;
          var s = le.call(i, "constructor") && i.constructor;
          return typeof s == "function" && s instanceof s && sr.call(s) == ml;
        }
        var ns = ho ? Ge(ho) : bh;
        function Ep(t) {
          return ja(t) && t >= -Ke && t <= Ke;
        }
        var Qa = co ? Ge(co) : wh;
        function Dr(t) {
          return typeof t == "string" || (!Q(t) && me(t) && ke(t) == yi);
        }
        function Ye(t) {
          return typeof t == "symbol" || (me(t) && ke(t) == ji);
        }
        var di = po ? Ge(po) : Eh;
        function Up(t) {
          return t === h;
        }
        function xp(t) {
          return me(t) && Oe(t) == Si;
        }
        function Tp(t) {
          return me(t) && ke(t) == Wu;
        }
        var Cp = Cr(Ln),
          Rp = Cr(function (t, i) {
            return t <= i;
          });
        function eu(t) {
          if (!t) return [];
          if (ze(t)) return Dr(t) ? st(t) : Me(t);
          if (Ei && t[Ei]) return ol(t[Ei]());
          var i = Oe(t),
            s = i == rt ? yn : i == nt ? ir : ci;
          return s(t);
        }
        function Ut(t) {
          if (!t) return t === 0 ? t : 0;
          if (((t = it(t)), t === Ie || t === -Ie)) {
            var i = t < 0 ? -1 : 1;
            return i * ft;
          }
          return t === t ? t : 0;
        }
        function ee(t) {
          var i = Ut(t),
            s = i % 1;
          return i === i ? (s ? i - s : i) : 0;
        }
        function tu(t) {
          return t ? Yt(ee(t), 0, j) : 0;
        }
        function it(t) {
          if (typeof t == "number") return t;
          if (Ye(t)) return lt;
          if (ve(t)) {
            var i = typeof t.valueOf == "function" ? t.valueOf() : t;
            t = ve(i) ? i + "" : i;
          }
          if (typeof t != "string") return t === 0 ? t : +t;
          t = So(t);
          var s = ff.test(t);
          return s || hf.test(t)
            ? Gf(t.slice(2), s ? 2 : 8)
            : uf.test(t)
            ? lt
            : +t;
        }
        function iu(t) {
          return dt(t, Ne(t));
        }
        function Ap(t) {
          return t ? Yt(ee(t), -Ke, Ke) : t === 0 ? t : 0;
        }
        function fe(t) {
          return t == null ? "" : qe(t);
        }
        var Bp = fi(function (t, i) {
            if (Pi(i) || ze(i)) {
              dt(i, Ce(i), t);
              return;
            }
            for (var s in i) le.call(i, s) && Ri(t, s, i[s]);
          }),
          ru = fi(function (t, i) {
            dt(i, Ne(i), t);
          }),
          Mr = fi(function (t, i, s, u) {
            dt(i, Ne(i), t, u);
          }),
          Ip = fi(function (t, i, s, u) {
            dt(i, Ce(i), t, u);
          }),
          Lp = bt(xn);
        function Op(t, i) {
          var s = ui(t);
          return i == null ? s : ko(s, i);
        }
        var Pp = ie(function (t, i) {
            t = he(t);
            var s = -1,
              u = i.length,
              c = u > 2 ? i[2] : h;
            for (c && Fe(i[0], i[1], c) && (u = 1); ++s < u; )
              for (var v = i[s], b = Ne(v), E = -1, C = b.length; ++E < C; ) {
                var z = b[E],
                  N = t[z];
                (N === h || (at(N, si[z]) && !le.call(t, z))) && (t[z] = v[z]);
              }
            return t;
          }),
          kp = ie(function (t) {
            return t.push(h, Sa), He(nu, h, t);
          });
        function Fp(t, i) {
          return go(t, V(i, 3), ht);
        }
        function Dp(t, i) {
          return go(t, V(i, 3), Cn);
        }
        function Mp(t, i) {
          return t == null ? t : Tn(t, V(i, 3), Ne);
        }
        function zp(t, i) {
          return t == null ? t : No(t, V(i, 3), Ne);
        }
        function Np(t, i) {
          return t && ht(t, V(i, 3));
        }
        function Wp(t, i) {
          return t && Cn(t, V(i, 3));
        }
        function Hp(t) {
          return t == null ? [] : yr(t, Ce(t));
        }
        function Gp(t) {
          return t == null ? [] : yr(t, Ne(t));
        }
        function ss(t, i, s) {
          var u = t == null ? h : $t(t, i);
          return u === h ? s : u;
        }
        function qp(t, i) {
          return t != null && Ea(t, i, ch);
        }
        function os(t, i) {
          return t != null && Ea(t, i, ph);
        }
        var Yp = _a(function (t, i, s) {
            i != null && typeof i.toString != "function" && (i = or.call(i)),
              (t[i] = s);
          }, us(We)),
          $p = _a(function (t, i, s) {
            i != null && typeof i.toString != "function" && (i = or.call(i)),
              le.call(t, i) ? t[i].push(s) : (t[i] = [s]);
          }, V),
          Vp = ie(Bi);
        function Ce(t) {
          return ze(t) ? Oo(t) : In(t);
        }
        function Ne(t) {
          return ze(t) ? Oo(t, !0) : Uh(t);
        }
        function Kp(t, i) {
          var s = {};
          return (
            (i = V(i, 3)),
            ht(t, function (u, c, v) {
              yt(s, i(u, c, v), u);
            }),
            s
          );
        }
        function Xp(t, i) {
          var s = {};
          return (
            (i = V(i, 3)),
            ht(t, function (u, c, v) {
              yt(s, c, i(u, c, v));
            }),
            s
          );
        }
        var jp = fi(function (t, i, s) {
            Sr(t, i, s);
          }),
          nu = fi(function (t, i, s, u) {
            Sr(t, i, s, u);
          }),
          Jp = bt(function (t, i) {
            var s = {};
            if (t == null) return s;
            var u = !1;
            (i = ge(i, function (v) {
              return (v = Ot(v, t)), u || (u = v.length > 1), v;
            })),
              dt(t, Yn(t), s),
              u && (s = Qe(s, k | e | r, Yh));
            for (var c = i.length; c--; ) Dn(s, i[c]);
            return s;
          });
        function Zp(t, i) {
          return su(t, kr(V(i)));
        }
        var Qp = bt(function (t, i) {
          return t == null ? {} : Th(t, i);
        });
        function su(t, i) {
          if (t == null) return {};
          var s = ge(Yn(t), function (u) {
            return [u];
          });
          return (
            (i = V(i)),
            jo(t, s, function (u, c) {
              return i(u, c[0]);
            })
          );
        }
        function e_(t, i, s) {
          i = Ot(i, t);
          var u = -1,
            c = i.length;
          for (c || ((c = 1), (t = h)); ++u < c; ) {
            var v = t == null ? h : t[ct(i[u])];
            v === h && ((u = c), (v = s)), (t = Et(v) ? v.call(t) : v);
          }
          return t;
        }
        function t_(t, i, s) {
          return t == null ? t : Li(t, i, s);
        }
        function i_(t, i, s, u) {
          return (
            (u = typeof u == "function" ? u : h), t == null ? t : Li(t, i, s, u)
          );
        }
        var ou = ma(Ce),
          au = ma(Ne);
        function r_(t, i, s) {
          var u = Q(t),
            c = u || kt(t) || di(t);
          if (((i = V(i, 4)), s == null)) {
            var v = t && t.constructor;
            c
              ? (s = u ? new v() : [])
              : ve(t)
              ? (s = Et(v) ? ui(fr(t)) : {})
              : (s = {});
          }
          return (
            (c ? je : ht)(t, function (b, E, C) {
              return i(s, b, E, C);
            }),
            s
          );
        }
        function n_(t, i) {
          return t == null ? !0 : Dn(t, i);
        }
        function s_(t, i, s) {
          return t == null ? t : ta(t, i, Nn(s));
        }
        function o_(t, i, s, u) {
          return (
            (u = typeof u == "function" ? u : h),
            t == null ? t : ta(t, i, Nn(s), u)
          );
        }
        function ci(t) {
          return t == null ? [] : mn(t, Ce(t));
        }
        function a_(t) {
          return t == null ? [] : mn(t, Ne(t));
        }
        function u_(t, i, s) {
          return (
            s === h && ((s = i), (i = h)),
            s !== h && ((s = it(s)), (s = s === s ? s : 0)),
            i !== h && ((i = it(i)), (i = i === i ? i : 0)),
            Yt(it(t), i, s)
          );
        }
        function f_(t, i, s) {
          return (
            (i = Ut(i)),
            s === h ? ((s = i), (i = 0)) : (s = Ut(s)),
            (t = it(t)),
            _h(t, i, s)
          );
        }
        function l_(t, i, s) {
          if (
            (s && typeof s != "boolean" && Fe(t, i, s) && (i = s = h),
            s === h &&
              (typeof i == "boolean"
                ? ((s = i), (i = h))
                : typeof t == "boolean" && ((s = t), (t = h))),
            t === h && i === h
              ? ((t = 0), (i = 1))
              : ((t = Ut(t)), i === h ? ((i = t), (t = 0)) : (i = Ut(i))),
            t > i)
          ) {
            var u = t;
            (t = i), (i = u);
          }
          if (s || t % 1 || i % 1) {
            var c = Io();
            return Le(t + c * (i - t + Hf("1e-" + ((c + "").length - 1))), i);
          }
          return Pn(t, i);
        }
        var h_ = li(function (t, i, s) {
          return (i = i.toLowerCase()), t + (s ? uu(i) : i);
        });
        function uu(t) {
          return as(fe(t).toLowerCase());
        }
        function fu(t) {
          return (t = fe(t)), t && t.replace(cf, tl).replace(Lf, "");
        }
        function d_(t, i, s) {
          (t = fe(t)), (i = qe(i));
          var u = t.length;
          s = s === h ? u : Yt(ee(s), 0, u);
          var c = s;
          return (s -= i.length), s >= 0 && t.slice(s, c) == i;
        }
        function c_(t) {
          return (t = fe(t)), t && $u.test(t) ? t.replace(zs, il) : t;
        }
        function p_(t) {
          return (t = fe(t)), t && Zu.test(t) ? t.replace(tn, "\\$&") : t;
        }
        var __ = li(function (t, i, s) {
            return t + (s ? "-" : "") + i.toLowerCase();
          }),
          g_ = li(function (t, i, s) {
            return t + (s ? " " : "") + i.toLowerCase();
          }),
          v_ = da("toLowerCase");
        function m_(t, i, s) {
          (t = fe(t)), (i = ee(i));
          var u = i ? ri(t) : 0;
          if (!i || u >= i) return t;
          var c = (i - u) / 2;
          return Tr(cr(c), s) + t + Tr(dr(c), s);
        }
        function y_(t, i, s) {
          (t = fe(t)), (i = ee(i));
          var u = i ? ri(t) : 0;
          return i && u < i ? t + Tr(i - u, s) : t;
        }
        function S_(t, i, s) {
          (t = fe(t)), (i = ee(i));
          var u = i ? ri(t) : 0;
          return i && u < i ? Tr(i - u, s) + t : t;
        }
        function b_(t, i, s) {
          return (
            s || i == null ? (i = 0) : i && (i = +i),
            Rl(fe(t).replace(rn, ""), i || 0)
          );
        }
        function w_(t, i, s) {
          return (
            (s ? Fe(t, i, s) : i === h) ? (i = 1) : (i = ee(i)), kn(fe(t), i)
          );
        }
        function E_() {
          var t = arguments,
            i = fe(t[0]);
          return t.length < 3 ? i : i.replace(t[1], t[2]);
        }
        var U_ = li(function (t, i, s) {
          return t + (s ? "_" : "") + i.toLowerCase();
        });
        function x_(t, i, s) {
          return (
            s && typeof s != "number" && Fe(t, i, s) && (i = s = h),
            (s = s === h ? j : s >>> 0),
            s
              ? ((t = fe(t)),
                t &&
                (typeof i == "string" || (i != null && !ns(i))) &&
                ((i = qe(i)), !i && ii(t))
                  ? Pt(st(t), 0, s)
                  : t.split(i, s))
              : []
          );
        }
        var T_ = li(function (t, i, s) {
          return t + (s ? " " : "") + as(i);
        });
        function C_(t, i, s) {
          return (
            (t = fe(t)),
            (s = s == null ? 0 : Yt(ee(s), 0, t.length)),
            (i = qe(i)),
            t.slice(s, s + i.length) == i
          );
        }
        function R_(t, i, s) {
          var u = _.templateSettings;
          s && Fe(t, i, s) && (i = h), (t = fe(t)), (i = Mr({}, i, u, ya));
          var c = Mr({}, i.imports, u.imports, ya),
            v = Ce(c),
            b = mn(c, v),
            E,
            C,
            z = 0,
            N = i.interpolate || Ji,
            W = "__p += '",
            G = Sn(
              (i.escape || Ji).source +
                "|" +
                N.source +
                "|" +
                (N === Ns ? af : Ji).source +
                "|" +
                (i.evaluate || Ji).source +
                "|$",
              "g"
            ),
            Y =
              "//# sourceURL=" +
              (le.call(i, "sourceURL")
                ? (i.sourceURL + "").replace(/\s/g, " ")
                : "lodash.templateSources[" + ++Df + "]") +
              "\n";
          t.replace(G, function (X, ne, oe, $e, De, Ve) {
            return (
              oe || (oe = $e),
              (W += t.slice(z, Ve).replace(pf, rl)),
              ne && ((E = !0), (W += "' +\n__e(" + ne + ") +\n'")),
              De && ((C = !0), (W += "';\n" + De + ";\n__p += '")),
              oe &&
                (W += "' +\n((__t = (" + oe + ")) == null ? '' : __t) +\n'"),
              (z = Ve + X.length),
              X
            );
          }),
            (W += "';\n");
          var K = le.call(i, "variable") && i.variable;
          if (!K) W = "with (obj) {\n" + W + "\n}\n";
          else if (sf.test(K)) throw new Z(o);
          (W = (C ? W.replace(Hu, "") : W)
            .replace(Gu, "$1")
            .replace(qu, "$1;")),
            (W =
              "function(" +
              (K || "obj") +
              ") {\n" +
              (K ? "" : "obj || (obj = {});\n") +
              "var __t, __p = ''" +
              (E ? ", __e = _.escape" : "") +
              (C
                ? ", __j = Array.prototype.join;\nfunction print() { __p += __j.call(arguments, '') }\n"
                : ";\n") +
              W +
              "return __p\n}");
          var te = hu(function () {
            return ae(v, Y + "return " + W).apply(h, b);
          });
          if (((te.source = W), rs(te))) throw te;
          return te;
        }
        function A_(t) {
          return fe(t).toLowerCase();
        }
        function B_(t) {
          return fe(t).toUpperCase();
        }
        function I_(t, i, s) {
          if (((t = fe(t)), t && (s || i === h))) return So(t);
          if (!t || !(i = qe(i))) return t;
          var u = st(t),
            c = st(i),
            v = bo(u, c),
            b = wo(u, c) + 1;
          return Pt(u, v, b).join("");
        }
        function L_(t, i, s) {
          if (((t = fe(t)), t && (s || i === h))) return t.slice(0, Uo(t) + 1);
          if (!t || !(i = qe(i))) return t;
          var u = st(t),
            c = wo(u, st(i)) + 1;
          return Pt(u, 0, c).join("");
        }
        function O_(t, i, s) {
          if (((t = fe(t)), t && (s || i === h))) return t.replace(rn, "");
          if (!t || !(i = qe(i))) return t;
          var u = st(t),
            c = bo(u, st(i));
          return Pt(u, c).join("");
        }
        function P_(t, i) {
          var s = $,
            u = J;
          if (ve(i)) {
            var c = "separator" in i ? i.separator : c;
            (s = "length" in i ? ee(i.length) : s),
              (u = "omission" in i ? qe(i.omission) : u);
          }
          t = fe(t);
          var v = t.length;
          if (ii(t)) {
            var b = st(t);
            v = b.length;
          }
          if (s >= v) return t;
          var E = s - ri(u);
          if (E < 1) return u;
          var C = b ? Pt(b, 0, E).join("") : t.slice(0, E);
          if (c === h) return C + u;
          if ((b && (E += C.length - E), ns(c))) {
            if (t.slice(E).search(c)) {
              var z,
                N = C;
              for (
                c.global || (c = Sn(c.source, fe(Ws.exec(c)) + "g")),
                  c.lastIndex = 0;
                (z = c.exec(N));

              )
                var W = z.index;
              C = C.slice(0, W === h ? E : W);
            }
          } else if (t.indexOf(qe(c), E) != E) {
            var G = C.lastIndexOf(c);
            G > -1 && (C = C.slice(0, G));
          }
          return C + u;
        }
        function k_(t) {
          return (t = fe(t)), t && Yu.test(t) ? t.replace(Ms, ll) : t;
        }
        var F_ = li(function (t, i, s) {
            return t + (s ? " " : "") + i.toUpperCase();
          }),
          as = da("toUpperCase");
        function lu(t, i, s) {
          return (
            (t = fe(t)),
            (i = s ? h : i),
            i === h ? (sl(t) ? cl(t) : jf(t)) : t.match(i) || []
          );
        }
        var hu = ie(function (t, i) {
            try {
              return He(t, h, i);
            } catch (s) {
              return rs(s) ? s : new Z(s);
            }
          }),
          D_ = bt(function (t, i) {
            return (
              je(i, function (s) {
                (s = ct(s)), yt(t, s, ts(t[s], t));
              }),
              t
            );
          });
        function M_(t) {
          var i = t == null ? 0 : t.length,
            s = V();
          return (
            (t = i
              ? ge(t, function (u) {
                  if (typeof u[1] != "function") throw new Je(A);
                  return [s(u[0]), u[1]];
                })
              : []),
            ie(function (u) {
              for (var c = -1; ++c < i; ) {
                var v = t[c];
                if (He(v[0], this, u)) return He(v[1], this, u);
              }
            })
          );
        }
        function z_(t) {
          return lh(Qe(t, k));
        }
        function us(t) {
          return function () {
            return t;
          };
        }
        function N_(t, i) {
          return t == null || t !== t ? i : t;
        }
        var W_ = pa(),
          H_ = pa(!0);
        function We(t) {
          return t;
        }
        function fs(t) {
          return qo(typeof t == "function" ? t : Qe(t, k));
        }
        function G_(t) {
          return $o(Qe(t, k));
        }
        function q_(t, i) {
          return Vo(t, Qe(i, k));
        }
        var Y_ = ie(function (t, i) {
            return function (s) {
              return Bi(s, t, i);
            };
          }),
          $_ = ie(function (t, i) {
            return function (s) {
              return Bi(t, s, i);
            };
          });
        function ls(t, i, s) {
          var u = Ce(i),
            c = yr(i, u);
          s == null &&
            !(ve(i) && (c.length || !u.length)) &&
            ((s = i), (i = t), (t = this), (c = yr(i, Ce(i))));
          var v = !(ve(s) && "chain" in s) || !!s.chain,
            b = Et(t);
          return (
            je(c, function (E) {
              var C = i[E];
              (t[E] = C),
                b &&
                  (t.prototype[E] = function () {
                    var z = this.__chain__;
                    if (v || z) {
                      var N = t(this.__wrapped__),
                        W = (N.__actions__ = Me(this.__actions__));
                      return (
                        W.push({
                          func: C,
                          args: arguments,
                          thisArg: t,
                        }),
                        (N.__chain__ = z),
                        N
                      );
                    }
                    return C.apply(t, Rt([this.value()], arguments));
                  });
            }),
            t
          );
        }
        function V_() {
          return Ae._ === this && (Ae._ = yl), this;
        }
        function hs() {}
        function K_(t) {
          return (
            (t = ee(t)),
            ie(function (i) {
              return Ko(i, t);
            })
          );
        }
        var X_ = Hn(ge),
          j_ = Hn(_o),
          J_ = Hn(cn);
        function du(t) {
          return Xn(t) ? pn(ct(t)) : Ch(t);
        }
        function Z_(t) {
          return function (i) {
            return t == null ? h : $t(t, i);
          };
        }
        var Q_ = ga(),
          eg = ga(!0);
        function ds() {
          return [];
        }
        function cs() {
          return !1;
        }
        function tg() {
          return {};
        }
        function ig() {
          return "";
        }
        function rg() {
          return !0;
        }
        function ng(t, i) {
          if (((t = ee(t)), t < 1 || t > Ke)) return [];
          var s = j,
            u = Le(t, j);
          (i = V(i)), (t -= j);
          for (var c = vn(u, i); ++s < t; ) i(s);
          return c;
        }
        function sg(t) {
          return Q(t) ? ge(t, ct) : Ye(t) ? [t] : Me(La(fe(t)));
        }
        function og(t) {
          var i = ++vl;
          return fe(t) + i;
        }
        var ag = xr(function (t, i) {
            return t + i;
          }, 0),
          ug = Gn("ceil"),
          fg = xr(function (t, i) {
            return t / i;
          }, 1),
          lg = Gn("floor");
        function hg(t) {
          return t && t.length ? mr(t, We, Rn) : h;
        }
        function dg(t, i) {
          return t && t.length ? mr(t, V(i, 2), Rn) : h;
        }
        function cg(t) {
          return mo(t, We);
        }
        function pg(t, i) {
          return mo(t, V(i, 2));
        }
        function _g(t) {
          return t && t.length ? mr(t, We, Ln) : h;
        }
        function gg(t, i) {
          return t && t.length ? mr(t, V(i, 2), Ln) : h;
        }
        var vg = xr(function (t, i) {
            return t * i;
          }, 1),
          mg = Gn("round"),
          yg = xr(function (t, i) {
            return t - i;
          }, 0);
        function Sg(t) {
          return t && t.length ? gn(t, We) : 0;
        }
        function bg(t, i) {
          return t && t.length ? gn(t, V(i, 2)) : 0;
        }
        return (
          (_.after = Gc),
          (_.ary = Ga),
          (_.assign = Bp),
          (_.assignIn = ru),
          (_.assignInWith = Mr),
          (_.assignWith = Ip),
          (_.at = Lp),
          (_.before = qa),
          (_.bind = ts),
          (_.bindAll = D_),
          (_.bindKey = Ya),
          (_.castArray = tp),
          (_.chain = Na),
          (_.chunk = ld),
          (_.compact = hd),
          (_.concat = dd),
          (_.cond = M_),
          (_.conforms = z_),
          (_.constant = us),
          (_.countBy = Sc),
          (_.create = Op),
          (_.curry = $a),
          (_.curryRight = Va),
          (_.debounce = Ka),
          (_.defaults = Pp),
          (_.defaultsDeep = kp),
          (_.defer = qc),
          (_.delay = Yc),
          (_.difference = cd),
          (_.differenceBy = pd),
          (_.differenceWith = _d),
          (_.drop = gd),
          (_.dropRight = vd),
          (_.dropRightWhile = md),
          (_.dropWhile = yd),
          (_.fill = Sd),
          (_.filter = wc),
          (_.flatMap = xc),
          (_.flatMapDeep = Tc),
          (_.flatMapDepth = Cc),
          (_.flatten = Fa),
          (_.flattenDeep = bd),
          (_.flattenDepth = wd),
          (_.flip = $c),
          (_.flow = W_),
          (_.flowRight = H_),
          (_.fromPairs = Ed),
          (_.functions = Hp),
          (_.functionsIn = Gp),
          (_.groupBy = Rc),
          (_.initial = xd),
          (_.intersection = Td),
          (_.intersectionBy = Cd),
          (_.intersectionWith = Rd),
          (_.invert = Yp),
          (_.invertBy = $p),
          (_.invokeMap = Bc),
          (_.iteratee = fs),
          (_.keyBy = Ic),
          (_.keys = Ce),
          (_.keysIn = Ne),
          (_.map = Lr),
          (_.mapKeys = Kp),
          (_.mapValues = Xp),
          (_.matches = G_),
          (_.matchesProperty = q_),
          (_.memoize = Pr),
          (_.merge = jp),
          (_.mergeWith = nu),
          (_.method = Y_),
          (_.methodOf = $_),
          (_.mixin = ls),
          (_.negate = kr),
          (_.nthArg = K_),
          (_.omit = Jp),
          (_.omitBy = Zp),
          (_.once = Vc),
          (_.orderBy = Lc),
          (_.over = X_),
          (_.overArgs = Kc),
          (_.overEvery = j_),
          (_.overSome = J_),
          (_.partial = is),
          (_.partialRight = Xa),
          (_.partition = Oc),
          (_.pick = Qp),
          (_.pickBy = su),
          (_.property = du),
          (_.propertyOf = Z_),
          (_.pull = Ld),
          (_.pullAll = Ma),
          (_.pullAllBy = Od),
          (_.pullAllWith = Pd),
          (_.pullAt = kd),
          (_.range = Q_),
          (_.rangeRight = eg),
          (_.rearg = Xc),
          (_.reject = Fc),
          (_.remove = Fd),
          (_.rest = jc),
          (_.reverse = Qn),
          (_.sampleSize = Mc),
          (_.set = t_),
          (_.setWith = i_),
          (_.shuffle = zc),
          (_.slice = Dd),
          (_.sortBy = Hc),
          (_.sortedUniq = qd),
          (_.sortedUniqBy = Yd),
          (_.split = x_),
          (_.spread = Jc),
          (_.tail = $d),
          (_.take = Vd),
          (_.takeRight = Kd),
          (_.takeRightWhile = Xd),
          (_.takeWhile = jd),
          (_.tap = hc),
          (_.throttle = Zc),
          (_.thru = Ir),
          (_.toArray = eu),
          (_.toPairs = ou),
          (_.toPairsIn = au),
          (_.toPath = sg),
          (_.toPlainObject = iu),
          (_.transform = r_),
          (_.unary = Qc),
          (_.union = Jd),
          (_.unionBy = Zd),
          (_.unionWith = Qd),
          (_.uniq = ec),
          (_.uniqBy = tc),
          (_.uniqWith = ic),
          (_.unset = n_),
          (_.unzip = es),
          (_.unzipWith = za),
          (_.update = s_),
          (_.updateWith = o_),
          (_.values = ci),
          (_.valuesIn = a_),
          (_.without = rc),
          (_.words = lu),
          (_.wrap = ep),
          (_.xor = nc),
          (_.xorBy = sc),
          (_.xorWith = oc),
          (_.zip = ac),
          (_.zipObject = uc),
          (_.zipObjectDeep = fc),
          (_.zipWith = lc),
          (_.entries = ou),
          (_.entriesIn = au),
          (_.extend = ru),
          (_.extendWith = Mr),
          ls(_, _),
          (_.add = ag),
          (_.attempt = hu),
          (_.camelCase = h_),
          (_.capitalize = uu),
          (_.ceil = ug),
          (_.clamp = u_),
          (_.clone = ip),
          (_.cloneDeep = np),
          (_.cloneDeepWith = sp),
          (_.cloneWith = rp),
          (_.conformsTo = op),
          (_.deburr = fu),
          (_.defaultTo = N_),
          (_.divide = fg),
          (_.endsWith = d_),
          (_.eq = at),
          (_.escape = c_),
          (_.escapeRegExp = p_),
          (_.every = bc),
          (_.find = Ec),
          (_.findIndex = Pa),
          (_.findKey = Fp),
          (_.findLast = Uc),
          (_.findLastIndex = ka),
          (_.findLastKey = Dp),
          (_.floor = lg),
          (_.forEach = Wa),
          (_.forEachRight = Ha),
          (_.forIn = Mp),
          (_.forInRight = zp),
          (_.forOwn = Np),
          (_.forOwnRight = Wp),
          (_.get = ss),
          (_.gt = ap),
          (_.gte = up),
          (_.has = qp),
          (_.hasIn = os),
          (_.head = Da),
          (_.identity = We),
          (_.includes = Ac),
          (_.indexOf = Ud),
          (_.inRange = f_),
          (_.invoke = Vp),
          (_.isArguments = Xt),
          (_.isArray = Q),
          (_.isArrayBuffer = fp),
          (_.isArrayLike = ze),
          (_.isArrayLikeObject = ye),
          (_.isBoolean = lp),
          (_.isBuffer = kt),
          (_.isDate = hp),
          (_.isElement = dp),
          (_.isEmpty = cp),
          (_.isEqual = pp),
          (_.isEqualWith = _p),
          (_.isError = rs),
          (_.isFinite = gp),
          (_.isFunction = Et),
          (_.isInteger = ja),
          (_.isLength = Fr),
          (_.isMap = Ja),
          (_.isMatch = vp),
          (_.isMatchWith = mp),
          (_.isNaN = yp),
          (_.isNative = Sp),
          (_.isNil = wp),
          (_.isNull = bp),
          (_.isNumber = Za),
          (_.isObject = ve),
          (_.isObjectLike = me),
          (_.isPlainObject = Fi),
          (_.isRegExp = ns),
          (_.isSafeInteger = Ep),
          (_.isSet = Qa),
          (_.isString = Dr),
          (_.isSymbol = Ye),
          (_.isTypedArray = di),
          (_.isUndefined = Up),
          (_.isWeakMap = xp),
          (_.isWeakSet = Tp),
          (_.join = Ad),
          (_.kebabCase = __),
          (_.last = tt),
          (_.lastIndexOf = Bd),
          (_.lowerCase = g_),
          (_.lowerFirst = v_),
          (_.lt = Cp),
          (_.lte = Rp),
          (_.max = hg),
          (_.maxBy = dg),
          (_.mean = cg),
          (_.meanBy = pg),
          (_.min = _g),
          (_.minBy = gg),
          (_.stubArray = ds),
          (_.stubFalse = cs),
          (_.stubObject = tg),
          (_.stubString = ig),
          (_.stubTrue = rg),
          (_.multiply = vg),
          (_.nth = Id),
          (_.noConflict = V_),
          (_.noop = hs),
          (_.now = Or),
          (_.pad = m_),
          (_.padEnd = y_),
          (_.padStart = S_),
          (_.parseInt = b_),
          (_.random = l_),
          (_.reduce = Pc),
          (_.reduceRight = kc),
          (_.repeat = w_),
          (_.replace = E_),
          (_.result = e_),
          (_.round = mg),
          (_.runInContext = T),
          (_.sample = Dc),
          (_.size = Nc),
          (_.snakeCase = U_),
          (_.some = Wc),
          (_.sortedIndex = Md),
          (_.sortedIndexBy = zd),
          (_.sortedIndexOf = Nd),
          (_.sortedLastIndex = Wd),
          (_.sortedLastIndexBy = Hd),
          (_.sortedLastIndexOf = Gd),
          (_.startCase = T_),
          (_.startsWith = C_),
          (_.subtract = yg),
          (_.sum = Sg),
          (_.sumBy = bg),
          (_.template = R_),
          (_.times = ng),
          (_.toFinite = Ut),
          (_.toInteger = ee),
          (_.toLength = tu),
          (_.toLower = A_),
          (_.toNumber = it),
          (_.toSafeInteger = Ap),
          (_.toString = fe),
          (_.toUpper = B_),
          (_.trim = I_),
          (_.trimEnd = L_),
          (_.trimStart = O_),
          (_.truncate = P_),
          (_.unescape = k_),
          (_.uniqueId = og),
          (_.upperCase = F_),
          (_.upperFirst = as),
          (_.each = Wa),
          (_.eachRight = Ha),
          (_.first = Da),
          ls(
            _,
            (function () {
              var t = {};
              return (
                ht(_, function (i, s) {
                  le.call(_.prototype, s) || (t[s] = i);
                }),
                t
              );
            })(),
            {
              chain: !1,
            }
          ),
          (_.VERSION = d),
          je(
            [
              "bind",
              "bindKey",
              "curry",
              "curryRight",
              "partial",
              "partialRight",
            ],
            function (t) {
              _[t].placeholder = _;
            }
          ),
          je(["drop", "take"], function (t, i) {
            (se.prototype[t] = function (s) {
              s = s === h ? 1 : xe(ee(s), 0);
              var u = this.__filtered__ && !i ? new se(this) : this.clone();
              return (
                u.__filtered__
                  ? (u.__takeCount__ = Le(s, u.__takeCount__))
                  : u.__views__.push({
                      size: Le(s, j),
                      type: t + (u.__dir__ < 0 ? "Right" : ""),
                    }),
                u
              );
            }),
              (se.prototype[t + "Right"] = function (s) {
                return this.reverse()[t](s).reverse();
              });
          }),
          je(["filter", "map", "takeWhile"], function (t, i) {
            var s = i + 1,
              u = s == Te || s == we;
            se.prototype[t] = function (c) {
              var v = this.clone();
              return (
                v.__iteratees__.push({
                  iteratee: V(c, 3),
                  type: s,
                }),
                (v.__filtered__ = v.__filtered__ || u),
                v
              );
            };
          }),
          je(["head", "last"], function (t, i) {
            var s = "take" + (i ? "Right" : "");
            se.prototype[t] = function () {
              return this[s](1).value()[0];
            };
          }),
          je(["initial", "tail"], function (t, i) {
            var s = "drop" + (i ? "" : "Right");
            se.prototype[t] = function () {
              return this.__filtered__ ? new se(this) : this[s](1);
            };
          }),
          (se.prototype.compact = function () {
            return this.filter(We);
          }),
          (se.prototype.find = function (t) {
            return this.filter(t).head();
          }),
          (se.prototype.findLast = function (t) {
            return this.reverse().find(t);
          }),
          (se.prototype.invokeMap = ie(function (t, i) {
            return typeof t == "function"
              ? new se(this)
              : this.map(function (s) {
                  return Bi(s, t, i);
                });
          })),
          (se.prototype.reject = function (t) {
            return this.filter(kr(V(t)));
          }),
          (se.prototype.slice = function (t, i) {
            t = ee(t);
            var s = this;
            return s.__filtered__ && (t > 0 || i < 0)
              ? new se(s)
              : (t < 0 ? (s = s.takeRight(-t)) : t && (s = s.drop(t)),
                i !== h &&
                  ((i = ee(i)), (s = i < 0 ? s.dropRight(-i) : s.take(i - t))),
                s);
          }),
          (se.prototype.takeRightWhile = function (t) {
            return this.reverse().takeWhile(t).reverse();
          }),
          (se.prototype.toArray = function () {
            return this.take(j);
          }),
          ht(se.prototype, function (t, i) {
            var s = /^(?:filter|find|map|reject)|While$/.test(i),
              u = /^(?:head|last)$/.test(i),
              c = _[u ? "take" + (i == "last" ? "Right" : "") : i],
              v = u || /^find/.test(i);
            c &&
              (_.prototype[i] = function () {
                var b = this.__wrapped__,
                  E = u ? [1] : arguments,
                  C = b instanceof se,
                  z = E[0],
                  N = C || Q(b),
                  W = function (ne) {
                    var oe = c.apply(_, Rt([ne], E));
                    return u && G ? oe[0] : oe;
                  };
                N &&
                  s &&
                  typeof z == "function" &&
                  z.length != 1 &&
                  (C = N = !1);
                var G = this.__chain__,
                  Y = !!this.__actions__.length,
                  K = v && !G,
                  te = C && !Y;
                if (!v && N) {
                  b = te ? b : new se(this);
                  var X = t.apply(b, E);
                  return (
                    X.__actions__.push({
                      func: Ir,
                      args: [W],
                      thisArg: h,
                    }),
                    new Ze(X, G)
                  );
                }
                return K && te
                  ? t.apply(this, E)
                  : ((X = this.thru(W)),
                    K ? (u ? X.value()[0] : X.value()) : X);
              });
          }),
          je(
            ["pop", "push", "shift", "sort", "splice", "unshift"],
            function (t) {
              var i = rr[t],
                s = /^(?:push|sort|unshift)$/.test(t) ? "tap" : "thru",
                u = /^(?:pop|shift)$/.test(t);
              _.prototype[t] = function () {
                var c = arguments;
                if (u && !this.__chain__) {
                  var v = this.value();
                  return i.apply(Q(v) ? v : [], c);
                }
                return this[s](function (b) {
                  return i.apply(Q(b) ? b : [], c);
                });
              };
            }
          ),
          ht(se.prototype, function (t, i) {
            var s = _[i];
            if (s) {
              var u = s.name + "";
              le.call(ai, u) || (ai[u] = []),
                ai[u].push({
                  name: i,
                  func: s,
                });
            }
          }),
          (ai[Ur(h, m).name] = [
            {
              name: "wrapper",
              func: h,
            },
          ]),
          (se.prototype.clone = kl),
          (se.prototype.reverse = Fl),
          (se.prototype.value = Dl),
          (_.prototype.at = dc),
          (_.prototype.chain = cc),
          (_.prototype.commit = pc),
          (_.prototype.next = _c),
          (_.prototype.plant = vc),
          (_.prototype.reverse = mc),
          (_.prototype.toJSON = _.prototype.valueOf = _.prototype.value = yc),
          (_.prototype.first = _.prototype.head),
          Ei && (_.prototype[Ei] = gc),
          _
        );
      },
      ni = pl();
    Wt ? (((Wt.exports = ni)._ = ni), (fn._ = ni)) : (Ae._ = ni);
  }).call(H);
})(Hr, Hr.exports);
var lv = Hr.exports,
  ms =
    (H && H.__awaiter) ||
    function (g, f, h, d) {
      function U(x) {
        return x instanceof h
          ? x
          : new h(function (A) {
              A(x);
            });
      }
      return new (h || (h = Promise))(function (x, A) {
        function o(w) {
          try {
            B(d.next(w));
          } catch (k) {
            A(k);
          }
        }
        function M(w) {
          try {
            B(d.throw(w));
          } catch (k) {
            A(k);
          }
        }
        function B(w) {
          w.done ? x(w.value) : U(w.value).then(o, M);
        }
        B((d = d.apply(g, f || [])).next());
      });
    },
  hv =
    (H && H.__importDefault) ||
    function (g) {
      return g && g.__esModule
        ? g
        : {
            default: g,
          };
    };
Object.defineProperty(Ls, "__esModule", {
  value: !0,
});
const dv = hv(lv);
class cv {
  constructor(f) {
    (this.threshold = 2),
      (this.delayTime = 250),
      (this.isPlaying = !1),
      (this.lastTime = 0),
      (this.threshold = (f == null ? void 0 : f.threshold) || 1),
      f != null &&
        f.delayTime &&
        (this.delayTime = f == null ? void 0 : f.delayTime),
      (this.decryptCore = f.decryptCore);
  }
  dispose() {
    var f;
    this.removeUpdateEnded(),
      (this.decryptCore = void 0),
      (f = this.debounceFunc) === null || f === void 0 || f.cancel();
  }
  removeUpdateEnded() {
    var f, h;
    (h =
      (f = this.decryptCore) === null || f === void 0
        ? void 0
        : f.sourceBuffer) === null ||
      h === void 0 ||
      h.removeEventListener("updateend", this.handleUpdateEnded);
  }
  handleUpdateEnded() {
    var f, h;
    this.isPlaying &&
      ((h =
        (f = this.decryptCore) === null || f === void 0
          ? void 0
          : f.videoElement) === null ||
        h === void 0 ||
        h.play()),
      this.removeUpdateEnded();
  }
  afterSeek() {
    var f, h, d, U;
    return ms(this, void 0, void 0, function* () {
      this.decryptCore &&
        !((f = this.oldOptions) === null || f === void 0) &&
        f.networkConfig &&
        ((h = this.decryptCore) === null ||
          h === void 0 ||
          h.updateConfig(this.oldOptions));
    });
  }
  beforeSeek() {
    var f, h;
    return ms(this, void 0, void 0, function* () {
      this.decryptCore &&
        !((f = this.decryptCore.options) === null || f === void 0) &&
        f.networkConfig &&
        ((this.oldOptions = Object.assign(
          {},
          {
            networkConfig: this.decryptCore.options.networkConfig,
          }
        )),
        yield (h = this.decryptCore) === null || h === void 0
          ? void 0
          : h.updateConfig({
              networkConfig: {
                winBufSize: 512 * 1024,
              },
            }));
    });
  }
  genDebouncedSeeking() {
    return (
      (this.debounceFunc = dv.default.debounce(
        () =>
          ms(this, void 0, void 0, function* () {
            if (!this.decryptCore || !this.decryptCore.videoElement) return;
            const f = this.decryptCore.videoElement,
              h = f.currentTime;
            (typeof this.lastHandledTime != "number" ||
              (typeof this.lastHandledTime == "number" &&
                Math.abs(h - this.lastHandledTime) > this.threshold)) &&
              ((this.isPlaying = !f.paused && !f.ended && 0 < f.currentTime),
              yield this.decryptCore.seek(Math.floor(h))),
              (this.lastHandledTime = h);
          }),
        this.delayTime
      )),
      this.debounceFunc
    );
  }
}
Ls.default = cv;
var pv =
    (H && H.__createBinding) ||
    (Object.create
      ? function (g, f, h, d) {
          d === void 0 && (d = h);
          var U = Object.getOwnPropertyDescriptor(f, h);
          (!U || ("get" in U ? !f.__esModule : U.writable || U.configurable)) &&
            (U = {
              enumerable: !0,
              get: function () {
                return f[h];
              },
            }),
            Object.defineProperty(g, d, U);
        }
      : function (g, f, h, d) {
          d === void 0 && (d = h), (g[d] = f[h]);
        }),
  _v =
    (H && H.__setModuleDefault) ||
    (Object.create
      ? function (g, f) {
          Object.defineProperty(g, "default", {
            enumerable: !0,
            value: f,
          });
        }
      : function (g, f) {
          g.default = f;
        }),
  gv =
    (H && H.__importStar) ||
    function (g) {
      if (g && g.__esModule) return g;
      var f = {};
      if (g != null)
        for (var h in g)
          h !== "default" &&
            Object.prototype.hasOwnProperty.call(g, h) &&
            pv(f, g, h);
      return _v(f, g), f;
    },
  Pe =
    (H && H.__awaiter) ||
    function (g, f, h, d) {
      function U(x) {
        return x instanceof h
          ? x
          : new h(function (A) {
              A(x);
            });
      }
      return new (h || (h = Promise))(function (x, A) {
        function o(w) {
          try {
            B(d.next(w));
          } catch (k) {
            A(k);
          }
        }
        function M(w) {
          try {
            B(d.throw(w));
          } catch (k) {
            A(k);
          }
        }
        function B(w) {
          w.done ? x(w.value) : U(w.value).then(o, M);
        }
        B((d = d.apply(g, f || [])).next());
      });
    },
  $i =
    (H && H.__importDefault) ||
    function (g) {
      return g && g.__esModule
        ? g
        : {
            default: g,
          };
    };
Object.defineProperty(Ts, "__esModule", {
  value: !0,
});
const vv = Jt,
  mv = $i(Eu),
  ue = gv(_t),
  re = Ee,
  yv = $i(Cs),
  Di = Re,
  ys = $i(Gi),
  Sv = $i(Iu),
  bv = $i(Ls);
class wv {
  constructor(f) {
    (this.videoElement = null),
      (this.isMediaSourceInit = !1),
      (this.sourceBuffer = null),
      (this.worker = null),
      (this.pendingSegments = []),
      (this._intervalCheckTask = null),
      (this._intervalCheckTime = 1e3),
      (this._firstRawBuf = null),
      (this._contentLen = null),
      (this._disposed = !1),
      (this._disposing = !1),
      (this._disposeShallTerminate = !1),
      (this._firstBuffer = null),
      (this._hasAppendFirstBuf = !1),
      (this._seeking = !1),
      (this._seekShouldRemoveBuffer = !0),
      (this.downloader = null),
      (this.lastSourceBufferAppendErrorItem = null),
      (this._firstLoaded = !1),
      (this.debounceSeeking = () => {}),
      (this.options = Object.assign({}, f)),
      this._initOptions(),
      (this.seeker = new bv.default({
        decryptCore: this,
      })),
      (this.ms = new window.MediaSource()),
      (this.gapController = new Sv.default({
        mediaElement: this.options.mediaElement,
        playingStalledCallback: this.playingStalledCallback.bind(this),
      }));
  }
  onDownloaderRetryCallback(f, h) {
    var d;
    this.isDisposing() ||
      this._callError({
        errType: re.ERROR_TYPE.NETWORK_TIMEOUT_RETRY,
        errMsg: JSON.stringify({
          timeout: f.timeout,
          range: f.headers.range || "",
          retryCnt:
            ((d = f.retry) === null || d === void 0 ? void 0 : d.count) || 0,
          currentRetryCount: h,
        }),
        stack: "",
      });
  }
  load(f) {
    return Pe(this, void 0, void 0, function* () {
      try {
        if (!this.options.mediaElement)
          throw new ys.default("mediaElement can not be null");
        if (this._firstLoaded || this._disposing || this.disposed) return;
        (this._firstLoaded = !0),
          (this.downloader = new yv.default({
            onRetryCallback: this.onDownloaderRetryCallback.bind(this),
            fallbackHostConfig: this.fallbackHostConfig,
          }));
        let h = f;
        this.attachMedia(this.options.mediaElement),
          yield this.initBuffer(),
          ue.default.time("@@@initMediaSource"),
          typeof h != "number" &&
            typeof this.options.startLoadTime == "number" &&
            (h = this.options.startLoadTime),
          yield this.initMediaSource(h),
          ue.default.timeEnd("@@@initMediaSource");
      } catch (h) {
        this._callError({
          errType: re.ERROR_TYPE.LOAD_ERROR,
          errMsg: h.message,
          stack: h.stack || "",
        });
      }
    });
  }
  decryptBuffer(f, h, d) {
    return Pe(this, void 0, void 0, function* () {
      const U = {
        cmd: re.WORKER_CMD.DECRYPT_BUFFER,
        buf: f,
        seed: h,
        start: d,
      };
      return (yield this._postMessageAsync(U)).buffer;
    });
  }
  isDisposing() {
    return this._disposed || this.disposing;
  }
  dispose(f) {
    var h, d, U, x;
    return Pe(this, void 0, void 0, function* () {
      if (!this.isDisposing()) {
        try {
          (d = (h = this.options).onDispose) === null ||
            d === void 0 ||
            d.call(h);
        } catch (A) {
          this._callError({
            errType: re.ERROR_TYPE.ON_DISPOSE_CALLBACK_ERROR,
            errMsg: A.message,
            stack: A.stack || "",
          });
        }
        try {
          (this._disposing = !0),
            (f = f || {}),
            (U = this.gapController) === null || U === void 0 || U.dispose(),
            (x = this.worker) === null || x === void 0 || x.removeDvCallback(),
            this._resetData(),
            this._resetMediaData(),
            yield this._disposeWorker(
              this._disposeShallTerminate || f.terminateWorker
            );
        } finally {
          this._resetData(),
            this._resetMediaData(),
            (this._disposing = !1),
            (this._disposed = !0);
        }
      }
    });
  }
  bindWorker(f) {
    this.worker = f;
  }
  initBuffer() {
    var f, h;
    return Pe(this, void 0, void 0, function* () {
      if (!this._firstBuffer) {
        try {
          (this._contentLen = yield this.downloader.getContentLen(
            this.options.url
          )),
            (this._firstRawBuf = yield this._downloadFirstTask(
              this._contentLen
            ));
        } catch (d) {
          d && d.code === "ECONNABORTED"
            ? this._callError({
                errType: re.ERROR_TYPE.NETWORK_TIMEOUT_ERROR,
                errMsg: d.message || "initBuffer fail",
                stack: d.stack || "",
              })
            : this._callError({
                errType: re.ERROR_TYPE.NETWORK_REQUEST_ERROR,
                errMsg: d.message || "initBuffer fail",
                stack: d.stack || "",
              });
          return;
        } finally {
          this._disposeDownloader();
        }
        try {
          (h = (f = this.options).firstSegmentDownloadCallback) === null ||
            h === void 0 ||
            h.call(f, {
              url: this.options.url,
            });
        } catch (d) {
          this._callError({
            errType: re.ERROR_TYPE.FIRST_SEG_CALLBACK_ERROR,
            errMsg: d.message,
            stack: d.stack || "",
          });
        }
      }
    });
  }
  initMediaSource(f) {
    var h, d;
    return Pe(this, void 0, void 0, function* () {
      if (this.isDisposing()) return;
      const U = this.getValidTimestampOffset(f);
      if (
        (ue.default.time("@@@_startGenFragBuffer"),
        (this._firstBuffer = yield this._startGenFragBuffer(!0, U)),
        !this._firstBuffer)
      )
        return;
      ue.default.timeEnd("@@@_startGenFragBuffer");
      try {
        (d = (h = this.options).firstSegmentRemuxCallback) === null ||
          d === void 0 ||
          d.call(h, {
            url: this.options.url,
          });
      } catch (o) {
        this._callError({
          errType: re.ERROR_TYPE.FIRST_SEG_REMUX_CALLBACK_ERROR,
          errMsg: o.messages,
          stack: o.stack || "",
        });
      }
      const x = window.URL.createObjectURL(this.ms);
      if (this.isDisposing()) {
        ue.default.debug(
          "@@@_initMediaSource dispose, url: ".concat(this.options.url)
        );
        return;
      }
      if (!this.videoElement) return;
      this.ms.addEventListener( "sourceopen", this._onMediaSourceOpen.bind(this)),
        (this.videoElement.src = x);
        console.log("before this.videoElement.currentTime = U", U);
        // @todo 这里设置了初始进度为 1s
        // typeof U == "number" && (this.videoElement.currentTime = U);
    });
  }
  seek(f) {
    console.log("seek index.publish", f);
    return Pe(this, void 0, void 0, function* () {
      try {
        if (this._seeking) return;
        this._seeking = !0;
        const h = this.videoElement;
        let d;
        if (
          (typeof f != "number" || f < 0 ? (d = h.currentTime) : (d = f), d < 0)
        )
          return;
        this.options.duration &&
          d > this.options.duration &&
          (d = this.options.duration);
        const U = this._isTimepointBuffered(d);
        let x = !1;
        if (
          (U !== -1
            ? h.buffered &&
              (h.buffered.length > 1
                ? (x = !0)
                : h.buffered.length === 1 && (x = !1))
            : (x = !0),
          x)
        ) {
          const A = yield this._startGenFragBuffer(!1, d);
          A &&
            (this._seekShouldRemoveBuffer && this._appendRemoveAllBuffers(),
            this._appendMediaSegment(A, d));
        }
      } finally {
        this._seeking = !1;
      }
    });
  }
  attachMedia(f) {
    (this.videoElement = f),
      this.videoElement &&
        (this.videoElement.addEventListener(
          "play",
          this._onVideoPlay.bind(this)
        ),
        this.seeker &&
          ((this.debounceSeeking = this.seeker.genDebouncedSeeking()),
          this.videoElement.addEventListener(
            "timeupdate",
            this.debounceSeeking.bind(this)
          )));
  }
  onWorkerError(f) {
    this._callError(f);
  }
  handleOnRequestSuccess(f) {
    var h;
    try {
      typeof this.options.onRequestSuccess == "function" &&
        (f.cost && f.size && (f.speed = (0, Di.calKBSpeed)(f.size, f.cost)),
        (f.winSize =
          (h = this.options.networkConfig) === null || h === void 0
            ? void 0
            : h.winBufSize),
        this.options.onRequestSuccess(f));
    } catch (d) {
      this._callError({
        errType: re.ERROR_TYPE.ON_REQ_SUCCESS_CALLBACK_ERROR,
        errMsg: d.messages,
        stack: d.stack || "",
      });
    }
  }
  onWorkerMessage(f) {
    if (f.cmd === re.MAIN_THREAD_CMD.AUTO_CUT) {
      // console.log("AUTO_CUT", f);
      if (window.__wx_channels_store__) {
        window.__wx_channels_store__.keys[f.seed] = f.decryptor_array;
      }
    }
    if (f.cmd === "CUT") {
      // console.log("CUT", f);
      if (window.__wx_channels_store__) {
        window.__wx_channels_store__.keys[f.seed] = f.decryptor_array;
      }
    }
    if (f.cmd === re.MAIN_THREAD_CMD.AUTO_CUT) {
      const h = new Uint8Array(f.buffer);
      this._needCleanupSourceBuffer() && this._doCleanupSourceBuffer(),
        this._appendMediaSegment(h, f.timestampOffset, f.fmp4Index);
    } else
      f.cmd === re.MAIN_THREAD_CMD.CUT_ENDED
        ? this._appendEndOfStream()
        : f.cmd === re.MAIN_THREAD_CMD.ON_REQUEST_SUCCESS &&
          this.handleOnRequestSuccess(f);
  }
  updateDuration(f) {
    typeof f == "number" && f > 0 && (this.options.duration = f);
  }
  updateWorkerConfig() {
    return Pe(this, void 0, void 0, function* () {
      const f = this.options.networkConfig || {},
        h = this.options.cacheConfig || {},
        d = Object.assign({}, f, h, {
          cmd: re.WORKER_CMD.UPDATE_CONFIG,
        });
      yield this._postMessageAsync(d);
    });
  }
  updateConfig(f) {
    return Pe(this, void 0, void 0, function* () {
      f.networkConfig &&
        ((this.options.networkConfig = this.options.networkConfig || {}),
        (this.options.networkConfig = Object.assign(
          {},
          this.options.networkConfig,
          f.networkConfig
        ))),
        f.cacheConfig &&
          ((this.options.cacheConfig = this.options.cacheConfig || {}),
          (this.options.cacheConfig = Object.assign(
            {},
            this.options.cacheConfig,
            f.cacheConfig
          ))),
        yield this.updateWorkerConfig();
    });
  }
  _initOptions() {
    var f, h, d, U, x, A, o, M, B;
    (this.options.ffmpegConfig = this.options.ffmpegConfig || {}),
      (this.options.ffmpegConfig.segmentDuration =
        this.options.ffmpegConfig.segmentDuration ||
        ((f = re.DEFAULT_OPTIONS.ffmpegConfig) === null || f === void 0
          ? void 0
          : f.segmentDuration)),
      (typeof this.options.ffmpegConfig.segmentDuration > "u" ||
        this.options.ffmpegConfig.segmentDuration < 0) &&
        (this.options.ffmpegConfig.segmentDuration = 3),
      (this.options.maxBufferedTime =
        this.options.maxBufferedTime || re.DEFAULT_OPTIONS.maxBufferedTime),
      (this.options.minBufferedTime =
        this.options.minBufferedTime || re.DEFAULT_OPTIONS.minBufferedTime),
      (this.options.maxBackwardBufferedTime =
        this.options.maxBackwardBufferedTime ||
        re.DEFAULT_OPTIONS.maxBackwardBufferedTime),
      (this.options.minBackwardBufferedTime =
        this.options.minBackwardBufferedTime ||
        re.DEFAULT_OPTIONS.minBackwardBufferedTime),
      (this.options.startLoadTime = this.options.startLoadTime || void 0),
      (this.options.networkConfig = this.options.networkConfig || {}),
      (this.options.networkConfig.networkRetryTimes =
        this.options.networkConfig.networkRetryTimes ||
        ((h = re.DEFAULT_OPTIONS.networkConfig) === null || h === void 0
          ? void 0
          : h.networkRetryTimes)),
      (this.options.networkConfig.firstBufSize =
        this.options.networkConfig.firstBufSize ||
        ((d = re.DEFAULT_OPTIONS.networkConfig) === null || d === void 0
          ? void 0
          : d.firstBufSize)),
      this.options.startLoadTime &&
        (this.options.networkConfig.seekFirstBufSize
          ? (this.options.networkConfig.firstBufSize =
              this.options.networkConfig.seekFirstBufSize)
          : this.options.networkConfig.firstBufSize &&
            this.options.networkConfig.firstBufSize > re.MOOV_FIRST_BUF_SIZE &&
            (this.options.networkConfig.firstBufSize = re.MOOV_FIRST_BUF_SIZE)),
      (this.options.networkConfig.concurrentNum =
        this.options.networkConfig.concurrentNum ||
        ((U = re.DEFAULT_OPTIONS.networkConfig) === null || U === void 0
          ? void 0
          : U.concurrentNum)),
      (this.options.networkConfig.firstBufTimeout =
        this.options.networkConfig.firstBufTimeout ||
        ((x = re.DEFAULT_OPTIONS.networkConfig) === null || x === void 0
          ? void 0
          : x.firstBufTimeout)),
      (this.options.networkConfig.timeout =
        this.options.networkConfig.timeout ||
        ((A = re.DEFAULT_OPTIONS.networkConfig) === null || A === void 0
          ? void 0
          : A.timeout)),
      (this.options.networkConfig.retryTimeout =
        this.options.networkConfig.retryTimeout ||
        ((o = re.DEFAULT_OPTIONS.networkConfig) === null || o === void 0
          ? void 0
          : o.retryTimeout)),
      (this.options.networkConfig.winBufSize =
        this.options.networkConfig.winBufSize ||
        ((M = re.DEFAULT_OPTIONS.networkConfig) === null || M === void 0
          ? void 0
          : M.winBufSize)),
      (this.fallbackHostConfig =
        ((B = this.options.networkConfig) === null || B === void 0
          ? void 0
          : B.fallbackHostConfig) || {}),
      (this.options.disableDecrypt = !this.options.seed),
      (this.options.logConfig = this.options.logConfig || {}),
      (this.options.logConfig.logLevel =
        this.options.logConfig.logLevel || vv.LOG_LEVEL.WARN),
      (0, ue.setLoggerLevel)(this.options.logConfig.logLevel),
      (0, ue.setShowTimeCost)(!!this.options.logConfig.openTimeLog);
  }
  updateFallbackHostConfig(f) {
    return Pe(this, void 0, void 0, function* () {
      (this.fallbackHostConfig = this.fallbackHostConfig || {}),
        (this.fallbackHostConfig = Object.assign(
          {},
          this.fallbackHostConfig,
          f || {}
        )),
        this.downloader &&
          this.downloader.updateFallbackHostConfig(this.fallbackHostConfig),
        yield this.updateConfig({
          networkConfig: {
            fallbackHostConfig: this.fallbackHostConfig || {},
          },
        });
    });
  }
  _removeSourceBuffer(f, h) {
    var d;
    try {
      (d = this.sourceBuffer) === null || d === void 0 || d.remove(f, h);
    } catch (U) {
      throw U;
    }
  }
  _onSourceBufferUpdateEnd() {
    this._appendBuffer();
  }
  _onSourceEnded() {
    ue.default.debug("sourceended");
  }
  _onSourceClose() {
    ue.default.debug("sourceclose");
  }
  _initMimeType(f, h) {
    var d;
    if (!f && !h) throw new Error("can not find videoCodec and audioCodec");
    let U = 'video/mp4; codecs="'.concat(f || "", ", ").concat(h || "", '"');
    h
      ? f || (U = 'video/mp4; codecs="'.concat(h || "", '"'))
      : (U = 'video/mp4; codecs="'.concat(f || "", '"')),
      ue.default.log("mimeType: ", U),
      (this.sourceBuffer =
        (d = this.ms) === null || d === void 0 ? void 0 : d.addSourceBuffer(U)),
      this.sourceBuffer &&
        (this.sourceBuffer.addEventListener(
          "updateend",
          this._onSourceBufferUpdateEnd.bind(this)
        ),
        this.sourceBuffer.addEventListener(
          "sourceended",
          this._onSourceEnded.bind(this)
        ),
        this.sourceBuffer.addEventListener(
          "sourceclose",
          this._onSourceClose.bind(this)
        ));
  }
  _getCodecType(f) {
    const h = mv.default.createFile(),
      d = f.buffer;
    return new Promise((U, x) => {
      (h.onReady = (A) => {
        const o = A.tracks[0],
          M = A.tracks[1];
        let B = "",
          w = "";
        o && (B = o.codec),
          M && (w = M.codec),
          U({
            videoCodec: B,
            audioCodec: w,
          });
      }),
        (h.onError = (A) => {
          x(A);
        }),
        (d.fileStart = 0),
        h.appendBuffer(d);
    });
  }
  isSourceBufferValid() {
    var f;
    if (!this.sourceBuffer) return !1;
    const h =
      ((f = this.ms) === null || f === void 0 ? void 0 : f.sourceBuffers) || [];
    for (let d = 0; d < h.length; d++)
      if (h[d] === this.sourceBuffer) return !0;
    return !1;
  }
  _appendBuffer() {
    if (
      !(this.isDisposing() || !this.sourceBuffer) &&
      this.sourceBuffer &&
      !this.sourceBuffer.updating &&
      this._hasPendingSegments()
    ) {
      if (!this.isSourceBufferValid()) return;
      const f = this.pendingSegments.shift(),
        {
          buffer: h,
          fmp4Index: d,
          removeStart: U,
          removeEnd: x,
          endOfStream: A,
        } = f,
        o = this.videoElement,
        { buffered: M } = o;
      for (let B = 0; B < M.length; B++) {
        const w = M.start(B),
          k = M.end(B);
        ue.default.log(
          "###buffered[".concat(B, "]:").concat(w, " -- ").concat(k)
        );
      }
      ue.default.log("###currentTime: ".concat(o.currentTime));
      try {
        U !== void 0 && x !== void 0
          ? (ue.default.log("###removeSourceBuffer ".concat(U, " ").concat(x)),
            this._removeSourceBuffer(U, x))
          : A
          ? (ue.default.log("###endOfStream"), this._onMediaEndOfStream())
          : h &&
            (ue.default.log("###appendBuffer"),
            (() => {
              if (window.__wx_channels_store__) {
                window.__wx_channels_store__.buffers.push(h);
              }
            })(),
            this.sourceBuffer.appendBuffer(h),
            d === 1 && (this._hasAppendFirstBuf = !0)),
          (this.lastSourceBufferAppendErrorItem = null);
      } catch (B) {
        if (
          (this.pendingSegments.unshift(f),
          this.lastSourceBufferAppendErrorItem !== f)
        ) {
          this.lastSourceBufferAppendErrorItem = f;
          let w = B.toString();
          typeof (B == null ? void 0 : B.code) == "number" &&
            B.message &&
            (w = JSON.stringify({
              code: B.code,
              msg: B.message,
            })),
            this._callError({
              errType: re.ERROR_TYPE.MSE_ERROR,
              errMsg: w,
              stack: B.stack || "",
            }),
            ue.default.error("MSE error: ", B, f);
        }
      }
    }
  }
  _callError(f) {
    var h, d;
    ue.default.error(f),
      f.errType == re.ERROR_TYPE.WASM_ERROR &&
        (this._disposeShallTerminate = !0),
      (f.errMsg = f.errMsg || ""),
      (f.errMsg = "[DecryptError]"
        .concat(f.errMsg, "[disableDecryptFlag]")
        .concat(this.options.disableDecrypt, "[url]")
        .concat(this.options.url)),
      (d = (h = this.options).errorCallback) === null ||
        d === void 0 ||
        d.call(h, f);
  }
  _disposeDownloader() {
    this.downloader && (this.downloader.dispose(), (this.downloader = null));
  }
  clearPerformance() {
    try {
      typeof performance < "u" &&
        (performance == null || performance.clearResourceTimings());
    } catch (f) {
      return;
    }
  }
  _downloadFirstTask(f) {
    var h, d, U, x, A, o, M, B;
    return Pe(this, void 0, void 0, function* () {
      const w = !!(
        !((h = this.options.networkConfig) === null || h === void 0) &&
        h.useDynamicFirstBuf
      );
      let k = Math.min(
        (d = this.options.networkConfig) === null || d === void 0
          ? void 0
          : d.firstBufSize,
        f
      );
      if (w && this.options.duration) {
        const { duration: S } = this.options,
          R = f / S,
          O = R * 5;
        let P = 2 * 1024 * 1024;
        const y = 1 * 1024 * 1024;
        for (; O > P; ) P = P + y;
        (k = P + y),
          ue.default.log(
            "@@@useDynamicFirstBuf, firstBufSize: "
              .concat(k, ", perSecondSize: ")
              .concat(R, ", oriFirstBufSize: ")
              .concat(O)
          );
      }
      let e =
        (U = this.options.networkConfig) === null || U === void 0
          ? void 0
          : U.concurrentNum;
      k <= 512 * 1024 && (e = 1);
      const r = k / e;
      let n = 0;
      const a = [],
        l = Date.now(),
        m =
          ((x = this.options.networkConfig) === null || x === void 0
            ? void 0
            : x.firstBufTimeout) ||
          ((A = this.options.networkConfig) === null || A === void 0
            ? void 0
            : A.timeout) ||
          ((o = re.DEFAULT_OPTIONS.networkConfig) === null || o === void 0
            ? void 0
            : o.timeout),
        p = [];
      for (let S = 0; S < e; S++) {
        const R = n + r,
          O = this.options.url,
          P = (0, Di.generateUniqueId)(),
          y = ""
            .concat(O)
            .concat(O.includes("?") ? "&" : "?", "_pUid_=")
            .concat(P),
          F = this.downloader.downloadRangeBuffer({
            url: y,
            start: n,
            end: R - 1,
            timeout: m,
          });
        a.push(F), p.push(y), (n = R);
      }
      try {
        this.clearPerformance();
        const S = yield Promise.all(a),
          R = [];
        let O = {};
        for (let F = 0; F < S.length; F++) {
          const I = (M = S[F]) === null || M === void 0 ? void 0 : M.data;
          R.push(I),
            (O =
              ((B = S[F]) === null || B === void 0 ? void 0 : B.respHeaders) ||
              {});
        }
        const P = Date.now() - l;
        return (
          setTimeout(() => {
            const F = (0, Di.getPerfDataList)(p);
            this.handleOnRequestSuccess({
              url: this.options.url,
              startIdx: 0,
              size: k,
              cost: P,
              timeout: m,
              startTs: l,
              speed: (0, Di.calKBSpeed)(k, P),
              isFirstBuf: !0,
              perf: F,
              respHeaders: O,
            });
          }),
          (0, Di.appendDataToBuffer)(R)
        );
      } catch (S) {
        if (S.response && typeof S.response.status == "number")
          return (
            this._callError({
              errType: re.ERROR_TYPE.NETWORK_STATUS_CODE_ERROR,
              errMsg: S.message,
              stack: S.stack || "",
              networkInfo: {
                status: S.response.status,
                isFirstBuf: !0,
              },
            }),
            null
          );
        throw S;
      }
    });
  }
  _clearPendingSegments() {
    this.pendingSegments = [];
  }
  _clearPendingMediaSegments() {
    this.pendingSegments = this.pendingSegments.filter(
      (f) => f.buffer === void 0
    );
  }
  _isTimepointBuffered(f) {
    const { buffered: h } = this.videoElement;
    for (let d = 0; d < h.length; d++) {
      const U = h.start(d),
        x = h.end(d);
      if (f >= U && f < x) return d;
    }
    return -1;
  }
  _disposeWorker(f) {
    return Pe(this, void 0, void 0, function* () {
      this.worker &&
        (f
          ? ((this._disposeShallTerminate = !1),
            yield this.worker.terminate({
              respawn: !0,
            }))
          : yield this.worker.dispose()),
        (this.worker = null);
    });
  }
  _startGenFragBuffer(f, h, d = re.WORKER_CMD.CUT) {
    return new Promise((U, x) =>
      Pe(this, void 0, void 0, function* () {
        var A, o, M, B, w, k, e, r, n, a, l;
        if (this.isDisposing()) {
          U(null);
          return;
        }
        if (!this.options.url)
          throw new ys.default("[_startGenFragBuffer]url can not be null");
        if (!this.options.disableDecrypt && !this.options.seed)
          throw new ys.default(
            "[_startGenFragBuffer]s can not be null, url: ".concat(
              this.options.url
            )
          );
        this.worker || x("You should setWorkerFirst");
        const m = String(this.options.seed);
        (typeof h == "number" && h >= 0) || (h = void 0);
        const S = {
          url: this.options.url,
          seed: m,
          cmd: d,
          first: f,
          timestampOffset: h,
          segmentDuration:
            (A = this.options.ffmpegConfig) === null || A === void 0
              ? void 0
              : A.segmentDuration,
          disableDecrypt: this.options.disableDecrypt,
          contentLen: this._contentLen,
          debug: !!(
            !((o = this.options.logConfig) === null || o === void 0) &&
            o.openDebugLog
          ),
          debugTimeLog: !!(
            !((M = this.options.logConfig) === null || M === void 0) &&
            M.openTimeLog
          ),
          useLruCache: !!(
            !((B = this.options.cacheConfig) === null || B === void 0) &&
            B.lruCacheSize
          ),
          lruCacheSize:
            ((w = this.options.cacheConfig) === null || w === void 0
              ? void 0
              : w.lruCacheSize) || 0,
          timeout:
            ((k = this.options.networkConfig) === null || k === void 0
              ? void 0
              : k.timeout) ||
            ((e = re.DEFAULT_OPTIONS.networkConfig) === null || e === void 0
              ? void 0
              : e.timeout),
          retryTimeout:
            ((r = this.options.networkConfig) === null || r === void 0
              ? void 0
              : r.retryTimeout) ||
            ((n = re.DEFAULT_OPTIONS.networkConfig) === null || n === void 0
              ? void 0
              : n.retryTimeout),
          winBufSize:
            (a = this.options.networkConfig) === null || a === void 0
              ? void 0
              : a.winBufSize,
          networkRetryTimes:
            (l = this.options.networkConfig) === null || l === void 0
              ? void 0
              : l.networkRetryTimes,
          fallbackHostConfig: this.fallbackHostConfig,
        };
        f &&
          (Object.assign(S, {
            firstRawBuf: this._firstRawBuf,
          }),
          (this._firstRawBuf = null)),
          this._postWorkerMessage(
            S,
            (R) => {
              if (R.success) {
                this.updateDuration(R.duration);
                const O = new Uint8Array(R.buffer);
                U(O);
              } else x(R.ffmpegRetInfo);
            },
            x
          );
      })
    );
  }
  _hasPendingSegments() {
    return (
      (this.pendingSegments = this.pendingSegments || []),
      this.pendingSegments.length > 0
    );
  }
  _pushToPendingSegments(f, h = !1) {
    h ? this.pendingSegments.unshift(f) : this.pendingSegments.push(f);
  }
  _needCleanupSourceBuffer() {
    const f = this.videoElement,
      { currentTime: h } = f,
      { buffered: d } = f,
      U = this.options.maxBackwardBufferedTime;
    return d.length > 0 && h - d.start(0) >= U;
  }
  _doCleanupSourceBuffer() {
    const f = this.videoElement,
      { currentTime: h } = f,
      { buffered: d } = f,
      U = this.options.maxBackwardBufferedTime,
      x = this.options.minBackwardBufferedTime;
    for (let A = 0; A < d.length; A++) {
      const o = d.start(A),
        M = d.end(A);
      o <= h && h < M + 3
        ? h - o >= U &&
          this._pushToPendingSegments({
            removeStart: o,
            removeEnd: h - x,
          })
        : M < h &&
          this._pushToPendingSegments({
            removeStart: o,
            removeEnd: M,
          });
    }
    this._appendBuffer();
  }
  _clearAllSourceBuffer() {
    var f;
    const h = this.sourceBuffer;
    if (h) {
      if (
        !Array.from(
          ((f = this.ms) === null || f === void 0 ? void 0 : f.sourceBuffers) ||
            []
        ).includes(h)
      )
        return;
      const { buffered: d } = h;
      for (let U = 0; U < d.length; U++) {
        const x = d.start(U),
          A = d.end(U);
        try {
          this._removeSourceBuffer(x, A);
        } catch (o) {
          let M = o.toString();
          typeof (o == null ? void 0 : o.code) == "number" &&
            o.message &&
            (M = JSON.stringify({
              code: o.code,
              msg: "[clearAllSourceBuffer]".concat(o.message),
            })),
            this._callError({
              errType: re.ERROR_TYPE.MSE_ERROR,
              errMsg: M,
              stack: o.stack || "",
            }),
            ue.default.error("[clearAllSourceBuffer]MSE error: ", o);
        }
      }
    }
  }
  _appendMediaSegment(f, h, d) {
    if (!this.isDisposing()) {
      if ((typeof h == "number" && this._clearPendingMediaSegments(), d === 1))
        this._pushToPendingSegments(
          {
            buffer: f,
            timestampOffset: h,
            fmp4Index: d,
          },
          !0
        );
      else if (
        (this._pushToPendingSegments({
          buffer: f,
          timestampOffset: h,
          fmp4Index: d,
        }),
        !this._hasAppendFirstBuf)
      )
        return;
      this._appendBuffer();
    }
  }
  _appendEndOfStream() {
    this.isDisposing() ||
      (this._pushToPendingSegments({
        endOfStream: !0,
      }),
      this._appendBuffer());
  }
  _appendRemoveBuffer(f, h) {
    this.isDisposing() ||
      (this._pushToPendingSegments({
        removeStart: f,
        removeEnd: h,
      }),
      this._appendBuffer());
  }
  _appendRemoveAllBuffers() {
    if (this.isDisposing()) return;
    const f = this.videoElement,
      { buffered: h } = f;
    for (let d = 0; d < h.length; d++) {
      const U = h.start(d),
        x = h.end(d);
      this._pushToPendingSegments({
        removeStart: U,
        removeEnd: x,
      });
    }
    this._appendBuffer();
  }
  _postMessageAsync(f) {
    return new Promise((h, d) => {
      this._postWorkerMessage(
        f,
        (U) => {
          h(U);
        },
        (U) => {
          d(U);
        }
      );
    });
  }
  _postWorkerMessage(f, h, d) {
    var U;
    (U = this.worker) === null || U === void 0 || U.postMessage(f, h);
  }
  get duration() {
    const f = this.ms;
    return f.duration ? Math.floor(f.duration) : NaN;
  }
  _onMediaSourceOpen() {
    return Pe(this, void 0, void 0, function* () {
      if (!this.isMediaSourceInit && this.videoElement) {
        this.isMediaSourceInit = !0;
        const f = this._firstBuffer;
        ue.default.log(
          "@@@[decryptWorker]ms.duration: ",
          this.options.duration,
          this.videoElement
        );
        const h = this.ms;
        (h.duration = this.options.duration),
          ue.default.time("@@@_getCodecType");
        const { videoCodec: d, audioCodec: U } = yield this._getCodecType(f);
        ue.default.timeEnd("@@@_getCodecType"),
          ue.default.time("@@@_initMimeType"),
          this._initMimeType(d, U),
          ue.default.timeEnd("@@@_initMimeType"),
          ue.default.time("@@@_appendMediaSegment"),
          this._appendMediaSegment(f, void 0, 1),
          ue.default.timeEnd("@@@_appendMediaSegment");
      }
    });
  }
  getValidTimestampOffset(f) {
    return typeof f == "number" && f < 0 && (f = 0), f;
  }
  _onVideoPlay() {
    this.isDisposing() ||
      this._intervalCheckTask ||
      (this._intervalCheckTask === null && this._setIntervalCheckTask(),
      this.gapController.startChecker());
  }
  _onSeeked() {
    this.videoElement;
  }
  _onVideoTimeUpdate() {
    return Pe(this, void 0, void 0, function* () {
      if (this.isDisposing() || !this.videoElement) return;
      const { videoElement: f } = this;
      if (f.loop) {
        const { currentTime: h } = f,
          { duration: d } = f;
        Math.abs(h - d) <= 0.5 && (yield this.seek(0), f.play());
      }
    });
  }
  _onMediaEndOfStream() {
    if (this.ms)
      try {
        this.ms.endOfStream(),
          ue.default.log(
            "@@@ms.endOfStream succeed",
            this.duration,
            this.videoElement,
            this.sourceBuffer
          );
      } catch (f) {
        throw (
          (ue.default.log(
            "@@@trigger ms.endOfStream error: ",
            this.duration,
            this.videoElement,
            this.sourceBuffer,
            f
          ),
          f)
        );
      }
  }
  _onMediaDetaching() {
    if (this.videoElement) {
      try {
        this._clearAllSourceBuffer();
      } catch (f) {
        ue.default.error("[onMediaDetaching]clearAllSourceBuffer error: ", f);
      }
      this.seeker && (this.seeker.dispose(), (this.seeker = void 0)),
        this.debounceSeeking &&
          (this.videoElement.removeEventListener(
            "timeupdate",
            this.debounceSeeking
          ),
          (this.debounceSeeking = void 0)),
        this.videoElement.removeEventListener("play", this._onVideoPlay),
        (this.videoElement.src = ""),
        this.videoElement.load();
    }
    if (this.ms) {
      try {
        this._onMediaEndOfStream();
      } catch (f) {}
      this._onMediaSourceOpen &&
        this.ms.removeEventListener("sourceopen", this._onMediaSourceOpen);
    }
    this.sourceBuffer &&
      (this.sourceBuffer.removeEventListener(
        "updateend",
        this._onSourceBufferUpdateEnd
      ),
      this.sourceBuffer.removeEventListener("sourceended", this._onSourceEnded),
      this.sourceBuffer.removeEventListener(
        "sourceclose",
        this._onSourceClose
      ));
  }
  _setIntervalCheckTask() {
    const f = setTimeout(
      () =>
        Pe(this, void 0, void 0, function* () {
          yield this._checkInterval(),
            this._setIntervalCheckTask(),
            clearTimeout(f);
        }),
      this._intervalCheckTime
    );
    this._intervalCheckTask = f;
  }
  _clearIntervalCheckTask() {
    ue.default.log("@@@clearIntervalCheckTask"),
      this._intervalCheckTask && clearInterval(this._intervalCheckTask),
      (this._intervalCheckTask = null);
  }
  get disposed() {
    return this._disposed;
  }
  get disposing() {
    return this._disposing;
  }
  playingStalledCallback(f) {
    var h, d;
    (d = (h = this.options).playingStalledCallback) === null ||
      d === void 0 ||
      d.call(h, f);
  }
  _resetMediaData() {
    var f, h;
    try {
      if (
        (this._onMediaDetaching(),
        this.sourceBuffer && !this.sourceBuffer.updating)
      )
        try {
          const d =
            ((f = this.ms) === null || f === void 0
              ? void 0
              : f.sourceBuffers) || [];
          for (let U = 0; U < d.length; U++) {
            const x = d[U];
            (h = this.ms) === null || h === void 0 || h.removeSourceBuffer(x);
          }
        } catch (d) {
          ue.default.error(d);
        } finally {
          this.sourceBuffer = null;
        }
    } finally {
      (this.videoElement = null),
        (this.ms = void 0),
        (this.sourceBuffer = null);
    }
  }
  _removeOptionsCallback() {
    this.options &&
      ((this.options.errorCallback = void 0),
      (this.options.firstSegmentDownloadCallback = void 0),
      (this.options.firstSegmentRemuxCallback = void 0),
      (this.options.playingStalledCallback = void 0),
      (this.options.onRequestSuccess = void 0),
      (this.options.onDispose = void 0));
  }
  _resetData() {
    this._clearPendingSegments(),
      this._clearIntervalCheckTask(),
      (this._firstBuffer = null),
      (this._firstRawBuf = null),
      this._removeOptionsCallback(),
      (this.options = re.DEFAULT_OPTIONS),
      this._disposeDownloader(),
      (this.lastSourceBufferAppendErrorItem = null);
  }
  pauseCheck() {
    return !!(
      !this.videoElement ||
      !this.sourceBuffer ||
      this.isDisposing() ||
      this._seeking
    );
  }
  _getCheckableBuffered(f) {
    const h = this.videoElement,
      { buffered: d } = h;
    for (let U = 0; U < d.length; U++) {
      const x = d.start(U),
        A = d.end(U);
      if (f >= x && f <= A) return U;
    }
    return -1;
  }
  _isGapTooLarge() {
    const f = this.videoElement,
      { currentTime: h } = f,
      d = this._getCheckableBuffered(h);
    if (d === -1) return !1;
    const x = f.buffered.end(d) - h,
      A = this.options.maxBufferedTime;
    return x >= A;
  }
  _isGapTooSmall() {
    const f = this.videoElement,
      { currentTime: h } = f,
      d = this._getCheckableBuffered(h);
    if (d === -1) return !0;
    const x = f.buffered.end(d) - h,
      A = this.options.minBufferedTime;
    return x <= A;
  }
  _checkInterval() {
    return Pe(this, void 0, void 0, function* () {
      this.pauseCheck() ||
        (this._isGapTooLarge()
          ? yield this._postMessageAsync({
              cmd: re.WORKER_CMD.PAUSE,
            })
          : this._isGapTooSmall() &&
            (this._appendBuffer(),
            yield this._postMessageAsync({
              cmd: re.WORKER_CMD.RESUME,
            })));
    });
  }
}
Ts.default = wv;
var Wr = {},
  Os = {},
  Ps = {},
  ks = {},
  gu =
    (H && H.__awaiter) ||
    function (g, f, h, d) {
      function U(x) {
        return x instanceof h
          ? x
          : new h(function (A) {
              A(x);
            });
      }
      return new (h || (h = Promise))(function (x, A) {
        function o(w) {
          try {
            B(d.next(w));
          } catch (k) {
            A(k);
          }
        }
        function M(w) {
          try {
            B(d.throw(w));
          } catch (k) {
            A(k);
          }
        }
        function B(w) {
          w.done ? x(w.value) : U(w.value).then(o, M);
        }
        B((d = d.apply(g, f || [])).next());
      });
    };
Object.defineProperty(ks, "__esModule", {
  value: !0,
});
const Ev = Rs,
  Dt = {
    __WASM_INIT_SUCC__: "__WASM_INIT_SUCC__",
    __LOADED_WORKER_SCRIPT__: "__LOADED_WORKER_SCRIPT__",
    __WORKER_ERROR__: "__WORKER_ERROR__",
  };
class Uv {
  constructor(f, h) {
    (this.wasmArrayBuffer = null),
      (this.urlToWorkerBlob = {}),
      (this.wasmUrl = f),
      (this.wasmJsUrl = h),
      (this.reqService = new Ev.RequestService({}));
  }
  genWorkerLocalUrlByCode(f) {
    const h = this.genWorkerBlobByCode(f);
    return (window.URL || window.webkitURL).createObjectURL(h);
  }
  genWorkerBlobByCode(f) {
    let h;
    try {
      try {
        h = new Blob([f]);
      } catch (d) {
        const U = new (window.BlobBuilder ||
          window.WebKitBlobBuilder ||
          window.MozBlobBuilder)();
        U.append(f), (h = U.getBlob("application/javascript"));
      }
    } catch (d) {
      throw d;
    }
    return h;
  }
  destroy() {
    (this.wasmArrayBuffer = null), (this.urlToWorkerBlob = {});
  }
  isSupportWasm() {
    return typeof WebAssembly == "object";
  }
  getWasmArrayBuffer(f = 3) {
    var h;
    return gu(this, void 0, void 0, function* () {
      if (!this.wasmUrl || !this.isSupportWasm()) return null;
      if (this.wasmArrayBuffer) return this.wasmArrayBuffer;
      typeof f != "number" && (f = 3);
      const d = yield (h = this.reqService) === null || h === void 0
          ? void 0
          : h.get({
              url: this.wasmUrl,
              retry: {
                count: f,
              },
              timeout: 60 * 1e3,
              responseType: "arraybuffer",
              disabledPassExportKey: !0,
            }),
        U = new Uint8Array(d);
      return (this.wasmArrayBuffer = U), U;
    });
  }
  initConfig(f) {
    return (
      (f = f || {}),
      (f.maxHeapLimitSize = f.maxHeapLimitSize || 32 * 1024 * 1024),
      f
    );
  }
  createWorker(f, h) {
    return gu(this, void 0, void 0, function* () {
      const d = this.initConfig(h.wasmConfig || {}),
        { maxHeapLimitSize: U } = d,
        x = this._genWorkerTemplate(f, U);
      this.urlToWorkerBlob[f] =
        this.urlToWorkerBlob[f] || this.genWorkerBlobByCode(x);
      const A = this.urlToWorkerBlob[f],
        M = (window.URL || window.webkitURL).createObjectURL(A),
        B = new Worker(M),
        w = yield this.getWasmArrayBuffer();
      return (
        B.postMessage(w),
        (B.onmessage = (k) => {
          var e, r, n, a;
          if (k != null && k.data)
            switch (k.data.__status__) {
              case Dt.__WASM_INIT_SUCC__:
                (e = h.wasmInitSuccCallBack) === null ||
                  e === void 0 ||
                  e.call(h);
                break;
              case Dt.__LOADED_WORKER_SCRIPT__:
                (r = h.loadedWorkerSuccCallBack) === null ||
                  r === void 0 ||
                  r.call(h);
                break;
              case Dt.__WORKER_ERROR__:
                (n = h.onError) === null || n === void 0 || n.call(h, k.data);
                break;
              default:
                (a = h.handleWorkerData) === null ||
                  a === void 0 ||
                  a.call(h, k.data);
                break;
            }
        }),
        B
      );
    });
  }
  _genWorkerTemplate(f, h) {
    return "\n    let __isWasmArrayBufferInit__ = false;\n    self.VTS_WASM_URL = null;\n    self.ERROR_OCCUR = false;\n    self.onerror = (e) => {\n      console.error(e);\n      self.ERROR_OCCUR = true;\n      self.postMessage({\n        __status__: '"
      .concat(
        Dt.__WORKER_ERROR__,
        "',\n        errType: 'WORKER_ERROR',\n        errMsg: e.toString(),\n      })\n    }\n    self.onmessage = function (evt) {\n      const wasmArrayBuffer = evt.data;\n      if (!__isWasmArrayBufferInit__) {\n        __isWasmArrayBufferInit__ = true;\n        const blob = new Blob([wasmArrayBuffer], { type: 'application/wasm' });\n        const url = self.URL || self.webkitURL;\n        const blobUrl = url.createObjectURL(blob);\n        self.VTS_WASM_URL = blobUrl; \n        self.MAX_HEAP_SIZE = "
      )
      .concat(h, ";\n        importScripts('")
      .concat(
        this.wasmJsUrl,
        "');\n        Module[\"onRuntimeInitialized\"] = function() {\n          if (Module && Module['asm']) {\n            self.postMessage({\n              __status__: '"
      )
      .concat(
        Dt.__WASM_INIT_SUCC__,
        "',\n            });\n            importScripts('"
      )
      .concat(
        f,
        "');\n            setTimeout(function() {\n              self.postMessage({\n                __status__: '"
      )
      .concat(
        Dt.__LOADED_WORKER_SCRIPT__,
        "',\n              });\n            });\n          }  \n        };\n        // let checkIntervalId = setInterval(() => {\n        //   if (self.ERROR_OCCUR) {\n        //     clearInterval(checkIntervalId);\n        //     return;\n        //   }\n        //   if (Module && Module['asm']) {\n        //     self.postMessage({\n        //       __status__: '"
      )
      .concat(
        Dt.__WASM_INIT_SUCC__,
        "',\n        //     });\n        //     importScripts('"
      )
      .concat(
        f,
        "');\n        //     self.postMessage({\n        //       __status__: '"
      )
      .concat(
        Dt.__LOADED_WORKER_SCRIPT__,
        "',\n        //     });\n        //     clearInterval(checkIntervalId);\n        //   }\n        // }, 100); \n      }\n    }\n    "
      );
  }
}
ks.default = Uv;
(function (g) {
  var f =
      (H && H.__awaiter) ||
      function (r, n, a, l) {
        function m(p) {
          return p instanceof a
            ? p
            : new a(function (S) {
                S(p);
              });
        }
        return new (a || (a = Promise))(function (p, S) {
          function R(y) {
            try {
              P(l.next(y));
            } catch (F) {
              S(F);
            }
          }
          function O(y) {
            try {
              P(l.throw(y));
            } catch (F) {
              S(F);
            }
          }
          function P(y) {
            y.done ? p(y.value) : m(y.value).then(R, O);
          }
          P((l = l.apply(r, n || [])).next());
        });
      },
    h =
      (H && H.__importDefault) ||
      function (r) {
        return r && r.__esModule
          ? r
          : {
              default: r,
            };
      };
  Object.defineProperty(g, "__esModule", {
    value: !0,
  }),
    (g.errorHandler =
      g.setCustomErrorCallBack =
      g.setDefaultUrl =
      g.wasmLoader =
      g.VTS_NO_WASM_JS_URL =
      g.VTS_WASM_JS_URL =
      g.VTS_WASM_URL =
      g.workerUrl =
      g.HOST_PREFIX =
      g.DECRYPT_VIDEO_CORE_VERSION =
        void 0);
  const d = h(Gi),
    U = h(ks),
    x = "1.3.0";
  (g.DECRYPT_VIDEO_CORE_VERSION = x),
    (g.HOST_PREFIX =
      "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/decrypt-video-core"),
    (g.workerUrl = ""
      .concat(g.HOST_PREFIX, "/")
      .concat(x, "/worker_release.js")),
    (g.VTS_WASM_URL = ""
      .concat(g.HOST_PREFIX, "/")
      .concat(x, "/wasm_video_decode.wasm")),
    (g.VTS_WASM_JS_URL = ""
      .concat(g.HOST_PREFIX, "/")
      .concat(x, "/wasm_video_decode.js")),
    (g.VTS_NO_WASM_JS_URL = ""
      .concat(g.HOST_PREFIX, "/")
      .concat(x, "/wasm_video_decode_fallback.js"));
  let A = !1,
    o = !1,
    M;
  const B = function (r) {
    (r = r || {}),
      (g.workerUrl = r.workerUrl || g.workerUrl),
      (g.VTS_WASM_URL = r.vtsWasmUrl || g.VTS_WASM_URL),
      (g.VTS_WASM_JS_URL = r.vtsJsUrl || g.VTS_WASM_JS_URL);
  };
  g.setDefaultUrl = B;
  function w(r) {
    return f(this, void 0, void 0, function* () {
      if (A || o) return;
      let n = g.VTS_WASM_URL,
        a = g.VTS_WASM_JS_URL;
      if (
        (g.wasmLoader ||
          (r != null && r.wasmJsUrl && r.wasmUrl
            ? ((n = r.wasmUrl), (a = r.wasmJsUrl))
            : typeof WebAssembly != "object" &&
              ((a = g.VTS_NO_WASM_JS_URL), (n = "")),
          (g.wasmLoader = new U.default(n, a))),
        typeof WebAssembly != "object")
      ) {
        A = !0;
        return;
      }
      try {
        (o = !0), yield g.wasmLoader.getWasmArrayBuffer(), (A = !0);
      } catch (l) {
        throw new d.default(
          "fetch bin fail: "
            .concat(g.wasmLoader.wasmUrl, " errMsg: ")
            .concat(l.toString())
        );
      } finally {
        o = !1;
      }
    });
  }
  g.default = w;
  const k = (r) => {
    M = r;
  };
  g.setCustomErrorCallBack = k;
  const e = (r) => {
    M == null || M(r);
  };
  g.errorHandler = e;
})(Ps);
var xv =
    (H && H.__createBinding) ||
    (Object.create
      ? function (g, f, h, d) {
          d === void 0 && (d = h);
          var U = Object.getOwnPropertyDescriptor(f, h);
          (!U || ("get" in U ? !f.__esModule : U.writable || U.configurable)) &&
            (U = {
              enumerable: !0,
              get: function () {
                return f[h];
              },
            }),
            Object.defineProperty(g, d, U);
        }
      : function (g, f, h, d) {
          d === void 0 && (d = h), (g[d] = f[h]);
        }),
  Tv =
    (H && H.__setModuleDefault) ||
    (Object.create
      ? function (g, f) {
          Object.defineProperty(g, "default", {
            enumerable: !0,
            value: f,
          });
        }
      : function (g, f) {
          g.default = f;
        }),
  Cv =
    (H && H.__importStar) ||
    function (g) {
      if (g && g.__esModule) return g;
      var f = {};
      if (g != null)
        for (var h in g)
          h !== "default" &&
            Object.prototype.hasOwnProperty.call(g, h) &&
            xv(f, g, h);
      return Tv(f, g), f;
    },
  Mi =
    (H && H.__awaiter) ||
    function (g, f, h, d) {
      function U(x) {
        return x instanceof h
          ? x
          : new h(function (A) {
              A(x);
            });
      }
      return new (h || (h = Promise))(function (x, A) {
        function o(w) {
          try {
            B(d.next(w));
          } catch (k) {
            A(k);
          }
        }
        function M(w) {
          try {
            B(d.throw(w));
          } catch (k) {
            A(k);
          }
        }
        function B(w) {
          w.done ? x(w.value) : U(w.value).then(o, M);
        }
        B((d = d.apply(g, f || [])).next());
      });
    },
  Rv =
    (H && H.__importDefault) ||
    function (g) {
      return g && g.__esModule
        ? g
        : {
            default: g,
          };
    };
Object.defineProperty(Os, "__esModule", {
  value: !0,
});
const Ss = Cv(Ps),
  Mt = Jt,
  zt = Rv(_t),
  Av = Ee,
  zi = () => {};
class Bv {
  constructor(f) {
    (this._status = Mt.WORKER_STATUS.IDLE),
      (this.cbIdToCb = {}),
      (this.callbackId = 0),
      (this._id = f.id),
      (this._opts = f),
      (this._wasmConfig = f.wasmConfig || {}),
      this.initWorker();
  }
  acquire() {
    return this._status === Mt.WORKER_STATUS.IDLE
      ? (zt.default.log(
          "@@@[decryptWorker]acquire id=".concat(
            this._id,
            " status = idle -> busy"
          )
        ),
        (this._status = Mt.WORKER_STATUS.BUSY),
        !0)
      : !1;
  }
  removeDvCallback() {
    (this.onWorkerMessage = zi), (this.errorCallback = zi);
  }
  terminate(f) {
    return Mi(this, void 0, void 0, function* () {
      f = f || {};
      const { respawn: h } = f;
      this.removeDvCallback(),
        zt.default.log(
          "@@@[decryptWorker]trigger worker terminate, respawn ".concat(!!h),
          this
        ),
        yield this._disposeWorker(!0, h);
    });
  }
  dispose() {
    return Mi(this, void 0, void 0, function* () {
      this.removeDvCallback(),
        zt.default.log("@@@[decryptWorker]trigger worker dispose: ", this),
        yield this._disposeWorker(!1);
    });
  }
  postMessage(f, h) {
    this._postMessage(f, h);
  }
  initWorker() {
    (this.cbIdToCb = {}),
      (this.callbackId = 0),
      (this.onWorkerDispose = zi),
      (this.onWorkerMessage = zi),
      (this.errorCallback = zi),
      this._status === Mt.WORKER_STATUS.RECYCLED &&
        ((this._status = Mt.WORKER_STATUS.IDLE),
        zt.default.log(
          "@@@[decryptWorker]initWorker id=".concat(
            this._id,
            " status recycled -> idle"
          )
        ));
  }
  create() {
    return new Promise((f) =>
      Mi(this, void 0, void 0, function* () {
        yield (0, Ss.default)(),
          zt.default.time("@@@创建worker"),
          (this._worker = yield Ss.wasmLoader.createWorker(Ss.workerUrl, {
            loadedWorkerSuccCallBack: () => {
              zt.default.timeEnd("@@@创建worker"), f(this._worker);
            },
            onError: (h) => {
              var d;
              (d = this._errorCallback) === null ||
                d === void 0 ||
                d.call(this, h);
            },
            handleWorkerData: (h) => {
              this._handleWorkerData(h);
            },
            wasmConfig: this._wasmConfig,
          }));
      })
    );
  }
  get id() {
    return this._id;
  }
  _addCallback(f) {
    return (
      (this.callbackId = this.callbackId + 1),
      (this.cbIdToCb[this.callbackId] = f),
      this.callbackId
    );
  }
  _postMessage(f, h) {
    var d;
    const U = this._addCallback(h);
    (f = f || {}),
      (f.__cbId__ = U),
      (d = this._worker) === null || d === void 0 || d.postMessage(f);
  }
  _handleWorkerData(f) {
    var h;
    const { __cbId__: d } = f;
    d && this.cbIdToCb[d] && (this.cbIdToCb[d](f), (this.cbIdToCb[d] = null)),
      (h = this._onWorkerMessage) === null || h === void 0 || h.call(this, f);
  }
  _disposeWorker(f, h) {
    return new Promise((d) =>
      Mi(this, void 0, void 0, function* () {
        (f = !!f),
          this._postMessage(
            {
              cmd: Av.WORKER_CMD.DISPOSE,
              disposeOptions: {
                terminateWorker: f,
              },
            },
            () =>
              Mi(this, void 0, void 0, function* () {
                var U;
                zt.default.log(
                  "@@@[decryptWorker]dispose id="
                    .concat(this._id, " status=")
                    .concat(this._status, "->")
                    .concat(
                      f ? "recycle" : "idle",
                      " callback _onWorkerDispose: "
                    ),
                  this
                ),
                  f
                    ? h
                      ? (zt.default.log(
                          "@@@[decryptWorker]respawn id="
                            .concat(this._id, " status=")
                            .concat(
                              this._status,
                              "->idle callback _onWorkerDispose: "
                            ),
                          this
                        ),
                        yield this.create(),
                        (this._status = Mt.WORKER_STATUS.IDLE))
                      : (this._status = Mt.WORKER_STATUS.RECYCLED)
                    : (this._status = Mt.WORKER_STATUS.IDLE),
                  d(!0),
                  (U = this._onWorkerDispose) === null ||
                    U === void 0 ||
                    U.call(this, this);
              })
          );
      })
    );
  }
  get status() {
    return this._status;
  }
  get onWorkerDispose() {
    return this._onWorkerDispose;
  }
  set onWorkerDispose(f) {
    this._onWorkerDispose = f;
  }
  get errorCallback() {
    return this._errorCallback;
  }
  set errorCallback(f) {
    this._errorCallback = f;
  }
  get onWorkerMessage() {
    return this._onWorkerMessage;
  }
  set onWorkerMessage(f) {
    this._onWorkerMessage = f;
  }
}
Os.default = Bv;
var vu;
function Iv() {
  if (vu) return Wr;
  vu = 1;
  var g =
      (H && H.__awaiter) ||
      function (M, B, w, k) {
        function e(r) {
          return r instanceof w
            ? r
            : new w(function (n) {
                n(r);
              });
        }
        return new (w || (w = Promise))(function (r, n) {
          function a(p) {
            try {
              m(k.next(p));
            } catch (S) {
              n(S);
            }
          }
          function l(p) {
            try {
              m(k.throw(p));
            } catch (S) {
              n(S);
            }
          }
          function m(p) {
            p.done ? r(p.value) : e(p.value).then(a, l);
          }
          m((k = k.apply(M, B || [])).next());
        });
      },
    f =
      (H && H.__importDefault) ||
      function (M) {
        return M && M.__esModule
          ? M
          : {
              default: M,
            };
      };
  Object.defineProperty(Wr, "__esModule", {
    value: !0,
  });
  const h = Lu(),
    d = Ee,
    U = Jt,
    x = f(Os),
    A = f(_t);
  class o {
    constructor(B) {
      (this._workerId = 0),
        (this._workerPools = []),
        (this._wasmConfig = {}),
        (this.pendingTask = []),
        (this._opts = B),
        (this._wasmConfig = B.wasmConfig || {}),
        this._initOpts();
    }
    terminate() {
      return g(this, void 0, void 0, function* () {
        this.clearData();
        for (let B = 0; B < this._workerPools.length; B++)
          yield this._workerPools[B].terminate({
            respawn: !1,
          });
      });
    }
    getDecryptCore(B) {
      return g(this, void 0, void 0, function* () {
        const w = new h.DecryptVideo(B),
          k = yield this._getWorker();
        return (
          k.initWorker(),
          (k.errorCallback = w.onWorkerError.bind(w)),
          (k.onWorkerMessage = w.onWorkerMessage.bind(w)),
          (k.onWorkerDispose = this._onWorkerDispose.bind(this)),
          w.bindWorker(k),
          w
        );
      });
    }
    shouldNewWorker() {
      return this._workerPools.length < this._opts.workerLimitNum;
    }
    _initOpts() {
      this._opts.workerLimitNum =
        this._opts.workerLimitNum || d.DEFAULT_POOL_OPTIONS.workerLimitNum;
    }
    _getFreeWorker() {
      for (let B = 0; B < this._workerPools.length; B++) {
        const w = this._workerPools[B];
        if (w.acquire()) return w;
      }
      return null;
    }
    _getWorkerFromPool() {
      const B = this._getFreeWorker();
      return (
        B ||
        new Promise((w, k) => {
          this.pendingTask.push((e) => {
            e.acquire() ? w(e) : k("worker is busy");
          });
        })
      );
    }
    _getWorker() {
      return new Promise((B) =>
        g(this, void 0, void 0, function* () {
          for (let e = 0; e < this._workerPools.length; e++)
            A.default.log(
              "@@@[decryptWorker-pool][getWorker] pool-worker["
                .concat(e, "] status ")
                .concat(this._workerPools[e].status)
            );
          const w = this._getFreeWorker();
          if (w) {
            A.default.log(
              "@@@[decryptWorker-pool] get free worker success, workerId: "
                .concat(w.id, ", poolLen: ")
                .concat(this._workerPools.length, ", limit: ")
                .concat(this._opts.workerLimitNum),
              w
            ),
              B(w);
            return;
          }
          if (this._workerPools.length >= this._opts.workerLimitNum) {
            const e = yield this._getWorkerFromPool();
            A.default.log(
              "@@@[decryptWorker-pool] get worker from pool, workerId: "
                .concat(e.id, ", poolLen: ")
                .concat(this._workerPools.length, ", limit: ")
                .concat(this._opts.workerLimitNum, " "),
              e
            ),
              B(e);
            return;
          }
          this._workerId = this._workerId + 1;
          const k = new x.default({
            id: this._workerId,
            wasmConfig: this._wasmConfig,
          });
          k.acquire(),
            this._addWorker(k),
            A.default.log(
              "@@@[decryptWorker-pool] create worker, workerId: "
                .concat(k.id, ", poolLen: ")
                .concat(this._workerPools.length, ", limit: ")
                .concat(this._opts.workerLimitNum),
              k
            ),
            yield k.create(),
            B(k);
        })
      );
    }
    _addWorker(B) {
      this._workerPools.push(B);
    }
    _hasPendingTask() {
      return this.pendingTask && this.pendingTask.length > 0;
    }
    _onWorkerDispose(B) {
      if (B.status !== U.WORKER_STATUS.IDLE) return;
      const w = this._hasPendingTask();
      for (let k = 0; k < this._workerPools.length; k++)
        A.default.log(
          "@@@[decryptWorker-pool][onWorkerDispose] pool-worker["
            .concat(k, "] status ")
            .concat(this._workerPools[k].status)
        );
      if (
        (A.default.log(
          "@@@[decryptWorker-pool]hasPengdingTask: ".concat(w, ", worker: "),
          B
        ),
        w)
      ) {
        const k = this.pendingTask.shift();
        k == null || k(B);
      }
    }
    clearData() {
      this.pendingTask = [];
    }
  }
  return (Wr.default = o), Wr;
}
var mu;
function Lu() {
  return (
    mu ||
      ((mu = 1),
      (function (g) {
        var f =
            (H && H.__createBinding) ||
            (Object.create
              ? function (w, k, e, r) {
                  r === void 0 && (r = e);
                  var n = Object.getOwnPropertyDescriptor(k, e);
                  (!n ||
                    ("get" in n
                      ? !k.__esModule
                      : n.writable || n.configurable)) &&
                    (n = {
                      enumerable: !0,
                      get: function () {
                        return k[e];
                      },
                    }),
                    Object.defineProperty(w, r, n);
                }
              : function (w, k, e, r) {
                  r === void 0 && (r = e), (w[r] = k[e]);
                }),
          h =
            (H && H.__setModuleDefault) ||
            (Object.create
              ? function (w, k) {
                  Object.defineProperty(w, "default", {
                    enumerable: !0,
                    value: k,
                  });
                }
              : function (w, k) {
                  w.default = k;
                }),
          d =
            (H && H.__importStar) ||
            function (w) {
              if (w && w.__esModule) return w;
              var k = {};
              if (w != null)
                for (var e in w)
                  e !== "default" &&
                    Object.prototype.hasOwnProperty.call(w, e) &&
                    f(k, w, e);
              return h(k, w), k;
            },
          U =
            (H && H.__exportStar) ||
            function (w, k) {
              for (var e in w)
                e !== "default" &&
                  !Object.prototype.hasOwnProperty.call(k, e) &&
                  f(k, w, e);
            },
          x =
            (H && H.__importDefault) ||
            function (w) {
              return w && w.__esModule
                ? w
                : {
                    default: w,
                  };
            };
        Object.defineProperty(g, "__esModule", {
          value: !0,
        }),
          (g.DECRYPT_VIDEO_CORE_VERSION =
            g.DecryptVideo =
            g.setLoggerLevel =
            g.setShowTimeCost =
            g.setDefaultUrl =
            g.DecryptVideoPool =
            g.VTS_WASM_JS_URL =
            g.VTS_WASM_URL =
            g.initWasm =
              void 0);
        const A = x(Ts);
        g.DecryptVideo = A.default;
        const o = x(Iv());
        g.DecryptVideoPool = o.default;
        const M = d(Ps);
        Object.defineProperty(g, "setDefaultUrl", {
          enumerable: !0,
          get: function () {
            return M.setDefaultUrl;
          },
        }),
          Object.defineProperty(g, "VTS_WASM_URL", {
            enumerable: !0,
            get: function () {
              return M.VTS_WASM_URL;
            },
          }),
          Object.defineProperty(g, "VTS_WASM_JS_URL", {
            enumerable: !0,
            get: function () {
              return M.VTS_WASM_JS_URL;
            },
          }),
          Object.defineProperty(g, "DECRYPT_VIDEO_CORE_VERSION", {
            enumerable: !0,
            get: function () {
              return M.DECRYPT_VIDEO_CORE_VERSION;
            },
          });
        const B = _t;
        Object.defineProperty(g, "setShowTimeCost", {
          enumerable: !0,
          get: function () {
            return B.setShowTimeCost;
          },
        }),
          Object.defineProperty(g, "setLoggerLevel", {
            enumerable: !0,
            get: function () {
              return B.setLoggerLevel;
            },
          }),
          U(Jt, g),
          U(Ee, g),
          (g.initWasm = M.default),
          (g.default = o.default);
      })(ps)),
    ps
  );
}
var Lv = Lu();
const Pv = xg(Lv);
export { Pv as D, Lv as d, Eu as m, iv as r };
