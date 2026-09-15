/*! For license information please see worker_release.js.LICENSE.txt */
(() => {
  function t(t, e) {
    var r = Object.keys(t);
    if (Object.getOwnPropertySymbols) {
      var n = Object.getOwnPropertySymbols(t);
      e &&
        (n = n.filter(function (e) {
          return Object.getOwnPropertyDescriptor(t, e).enumerable;
        })),
        r.push.apply(r, n);
    }
    return r;
  }
  function e(e) {
    for (var r = 1; r < arguments.length; r++) {
      var n = null != arguments[r] ? arguments[r] : {};
      r % 2
        ? t(Object(n), !0).forEach(function (t) {
            var r, a, o;
            (r = e),
              (a = t),
              (o = n[t]),
              (a = l(a)) in r
                ? Object.defineProperty(r, a, {
                    value: o,
                    enumerable: !0,
                    configurable: !0,
                    writable: !0,
                  })
                : (r[a] = o);
          })
        : Object.getOwnPropertyDescriptors
        ? Object.defineProperties(e, Object.getOwnPropertyDescriptors(n))
        : t(Object(n)).forEach(function (t) {
            Object.defineProperty(e, t, Object.getOwnPropertyDescriptor(n, t));
          });
    }
    return e;
  }
  function r(t, e) {
    return (
      (function (t) {
        if (Array.isArray(t)) return t;
      })(t) ||
      (function (t, e) {
        var r =
          null == t
            ? null
            : ("undefined" != typeof Symbol && t[Symbol.iterator]) ||
              t["@@iterator"];
        if (null != r) {
          var n,
            a,
            o,
            i,
            u = [],
            s = !0,
            c = !1;
          try {
            if (((o = (r = r.call(t)).next), 0 === e)) {
              if (Object(r) !== r) return;
              s = !1;
            } else
              for (
                ;
                !(s = (n = o.call(r)).done) &&
                (u.push(n.value), u.length !== e);
                s = !0
              );
          } catch (t) {
            (c = !0), (a = t);
          } finally {
            try {
              if (!s && null != r.return && ((i = r.return()), Object(i) !== i))
                return;
            } finally {
              if (c) throw a;
            }
          }
          return u;
        }
      })(t, e) ||
      (function (t, e) {
        if (t) {
          if ("string" == typeof t) return n(t, e);
          var r = Object.prototype.toString.call(t).slice(8, -1);
          return (
            "Object" === r && t.constructor && (r = t.constructor.name),
            "Map" === r || "Set" === r
              ? Array.from(t)
              : "Arguments" === r ||
                /^(?:Ui|I)nt(?:8|16|32)(?:Clamped)?Array$/.test(r)
              ? n(t, e)
              : void 0
          );
        }
      })(t, e) ||
      (function () {
        throw new TypeError(
          "Invalid attempt to destructure non-iterable instance.\nIn order to be iterable, non-array objects must have a [Symbol.iterator]() method."
        );
      })()
    );
  }
  function n(t, e) {
    (null == e || e > t.length) && (e = t.length);
    for (var r = 0, n = new Array(e); r < e; r++) n[r] = t[r];
    return n;
  }
  function a() {
    "use strict";
    a = function () {
      return e;
    };
    var t,
      e = {},
      r = Object.prototype,
      n = r.hasOwnProperty,
      o =
        Object.defineProperty ||
        function (t, e, r) {
          t[e] = r.value;
        },
      i = "function" == typeof Symbol ? Symbol : {},
      s = i.iterator || "@@iterator",
      c = i.asyncIterator || "@@asyncIterator",
      f = i.toStringTag || "@@toStringTag";
    function l(t, e, r) {
      return (
        Object.defineProperty(t, e, {
          value: r,
          enumerable: !0,
          configurable: !0,
          writable: !0,
        }),
        t[e]
      );
    }
    try {
      l({}, "");
    } catch (t) {
      l = function (t, e, r) {
        return (t[e] = r);
      };
    }
    function p(t, e, r, n) {
      var a = e && e.prototype instanceof b ? e : b,
        i = Object.create(a.prototype),
        u = new I(n || []);
      return o(i, "_invoke", { value: T(t, r, u) }), i;
    }
    function h(t, e, r) {
      try {
        return { type: "normal", arg: t.call(e, r) };
      } catch (t) {
        return { type: "throw", arg: t };
      }
    }
    e.wrap = p;
    var m = "suspendedStart",
      d = "suspendedYield",
      w = "executing",
      y = "completed",
      g = {};
    function b() {}
    function v() {}
    function _() {}
    var k = {};
    l(k, s, function () {
      return this;
    });
    var E = Object.getPrototypeOf,
      R = E && E(E(M([])));
    R && R !== r && n.call(R, s) && (k = R);
    var x = (_.prototype = b.prototype = Object.create(k));
    function S(t) {
      ["next", "throw", "return"].forEach(function (e) {
        l(t, e, function (t) {
          return this._invoke(e, t);
        });
      });
    }
    function B(t, e) {
      function r(a, o, i, s) {
        var c = h(t[a], t, o);
        if ("throw" !== c.type) {
          var f = c.arg,
            l = f.value;
          return l && "object" == u(l) && n.call(l, "__await")
            ? e.resolve(l.__await).then(
                function (t) {
                  r("next", t, i, s);
                },
                function (t) {
                  r("throw", t, i, s);
                }
              )
            : e.resolve(l).then(
                function (t) {
                  (f.value = t), i(f);
                },
                function (t) {
                  return r("throw", t, i, s);
                }
              );
        }
        s(c.arg);
      }
      var a;
      o(this, "_invoke", {
        value: function (t, n) {
          function o() {
            return new e(function (e, a) {
              r(t, n, e, a);
            });
          }
          return (a = a ? a.then(o, o) : o());
        },
      });
    }
    function T(e, r, n) {
      var a = m;
      return function (o, i) {
        if (a === w) throw new Error("Generator is already running");
        if (a === y) {
          if ("throw" === o) throw i;
          return { value: t, done: !0 };
        }
        for (n.method = o, n.arg = i; ; ) {
          var u = n.delegate;
          if (u) {
            var s = O(u, n);
            if (s) {
              if (s === g) continue;
              return s;
            }
          }
          if ("next" === n.method) n.sent = n._sent = n.arg;
          else if ("throw" === n.method) {
            if (a === m) throw ((a = y), n.arg);
            n.dispatchException(n.arg);
          } else "return" === n.method && n.abrupt("return", n.arg);
          a = w;
          var c = h(e, r, n);
          if ("normal" === c.type) {
            if (((a = n.done ? y : d), c.arg === g)) continue;
            return { value: c.arg, done: n.done };
          }
          "throw" === c.type &&
            ((a = y), (n.method = "throw"), (n.arg = c.arg));
        }
      };
    }
    function O(e, r) {
      var n = r.method,
        a = e.iterator[n];
      if (a === t)
        return (
          (r.delegate = null),
          ("throw" === n &&
            e.iterator.return &&
            ((r.method = "return"),
            (r.arg = t),
            O(e, r),
            "throw" === r.method)) ||
            ("return" !== n &&
              ((r.method = "throw"),
              (r.arg = new TypeError(
                "The iterator does not provide a '" + n + "' method"
              )))),
          g
        );
      var o = h(a, e.iterator, r.arg);
      if ("throw" === o.type)
        return (r.method = "throw"), (r.arg = o.arg), (r.delegate = null), g;
      var i = o.arg;
      return i
        ? i.done
          ? ((r[e.resultName] = i.value),
            (r.next = e.nextLoc),
            "return" !== r.method && ((r.method = "next"), (r.arg = t)),
            (r.delegate = null),
            g)
          : i
        : ((r.method = "throw"),
          (r.arg = new TypeError("iterator result is not an object")),
          (r.delegate = null),
          g);
    }
    function C(t) {
      var e = { tryLoc: t[0] };
      1 in t && (e.catchLoc = t[1]),
        2 in t && ((e.finallyLoc = t[2]), (e.afterLoc = t[3])),
        this.tryEntries.push(e);
    }
    function L(t) {
      var e = t.completion || {};
      (e.type = "normal"), delete e.arg, (t.completion = e);
    }
    function I(t) {
      (this.tryEntries = [{ tryLoc: "root" }]),
        t.forEach(C, this),
        this.reset(!0);
    }
    function M(e) {
      if (e || "" === e) {
        var r = e[s];
        if (r) return r.call(e);
        if ("function" == typeof e.next) return e;
        if (!isNaN(e.length)) {
          var a = -1,
            o = function r() {
              for (; ++a < e.length; )
                if (n.call(e, a)) return (r.value = e[a]), (r.done = !1), r;
              return (r.value = t), (r.done = !0), r;
            };
          return (o.next = o);
        }
      }
      throw new TypeError(u(e) + " is not iterable");
    }
    return (
      (v.prototype = _),
      o(x, "constructor", { value: _, configurable: !0 }),
      o(_, "constructor", { value: v, configurable: !0 }),
      (v.displayName = l(_, f, "GeneratorFunction")),
      (e.isGeneratorFunction = function (t) {
        var e = "function" == typeof t && t.constructor;
        return (
          !!e && (e === v || "GeneratorFunction" === (e.displayName || e.name))
        );
      }),
      (e.mark = function (t) {
        return (
          Object.setPrototypeOf
            ? Object.setPrototypeOf(t, _)
            : ((t.__proto__ = _), l(t, f, "GeneratorFunction")),
          (t.prototype = Object.create(x)),
          t
        );
      }),
      (e.awrap = function (t) {
        return { __await: t };
      }),
      S(B.prototype),
      l(B.prototype, c, function () {
        return this;
      }),
      (e.AsyncIterator = B),
      (e.async = function (t, r, n, a, o) {
        void 0 === o && (o = Promise);
        var i = new B(p(t, r, n, a), o);
        return e.isGeneratorFunction(r)
          ? i
          : i.next().then(function (t) {
              return t.done ? t.value : i.next();
            });
      }),
      S(x),
      l(x, f, "Generator"),
      l(x, s, function () {
        return this;
      }),
      l(x, "toString", function () {
        return "[object Generator]";
      }),
      (e.keys = function (t) {
        var e = Object(t),
          r = [];
        for (var n in e) r.push(n);
        return (
          r.reverse(),
          function t() {
            for (; r.length; ) {
              var n = r.pop();
              if (n in e) return (t.value = n), (t.done = !1), t;
            }
            return (t.done = !0), t;
          }
        );
      }),
      (e.values = M),
      (I.prototype = {
        constructor: I,
        reset: function (e) {
          if (
            ((this.prev = 0),
            (this.next = 0),
            (this.sent = this._sent = t),
            (this.done = !1),
            (this.delegate = null),
            (this.method = "next"),
            (this.arg = t),
            this.tryEntries.forEach(L),
            !e)
          )
            for (var r in this)
              "t" === r.charAt(0) &&
                n.call(this, r) &&
                !isNaN(+r.slice(1)) &&
                (this[r] = t);
        },
        stop: function () {
          this.done = !0;
          var t = this.tryEntries[0].completion;
          if ("throw" === t.type) throw t.arg;
          return this.rval;
        },
        dispatchException: function (e) {
          if (this.done) throw e;
          var r = this;
          function a(n, a) {
            return (
              (u.type = "throw"),
              (u.arg = e),
              (r.next = n),
              a && ((r.method = "next"), (r.arg = t)),
              !!a
            );
          }
          for (var o = this.tryEntries.length - 1; o >= 0; --o) {
            var i = this.tryEntries[o],
              u = i.completion;
            if ("root" === i.tryLoc) return a("end");
            if (i.tryLoc <= this.prev) {
              var s = n.call(i, "catchLoc"),
                c = n.call(i, "finallyLoc");
              if (s && c) {
                if (this.prev < i.catchLoc) return a(i.catchLoc, !0);
                if (this.prev < i.finallyLoc) return a(i.finallyLoc);
              } else if (s) {
                if (this.prev < i.catchLoc) return a(i.catchLoc, !0);
              } else {
                if (!c)
                  throw new Error("try statement without catch or finally");
                if (this.prev < i.finallyLoc) return a(i.finallyLoc);
              }
            }
          }
        },
        abrupt: function (t, e) {
          for (var r = this.tryEntries.length - 1; r >= 0; --r) {
            var a = this.tryEntries[r];
            if (
              a.tryLoc <= this.prev &&
              n.call(a, "finallyLoc") &&
              this.prev < a.finallyLoc
            ) {
              var o = a;
              break;
            }
          }
          o &&
            ("break" === t || "continue" === t) &&
            o.tryLoc <= e &&
            e <= o.finallyLoc &&
            (o = null);
          var i = o ? o.completion : {};
          return (
            (i.type = t),
            (i.arg = e),
            o
              ? ((this.method = "next"), (this.next = o.finallyLoc), g)
              : this.complete(i)
          );
        },
        complete: function (t, e) {
          if ("throw" === t.type) throw t.arg;
          return (
            "break" === t.type || "continue" === t.type
              ? (this.next = t.arg)
              : "return" === t.type
              ? ((this.rval = this.arg = t.arg),
                (this.method = "return"),
                (this.next = "end"))
              : "normal" === t.type && e && (this.next = e),
            g
          );
        },
        finish: function (t) {
          for (var e = this.tryEntries.length - 1; e >= 0; --e) {
            var r = this.tryEntries[e];
            if (r.finallyLoc === t)
              return this.complete(r.completion, r.afterLoc), L(r), g;
          }
        },
        catch: function (t) {
          for (var e = this.tryEntries.length - 1; e >= 0; --e) {
            var r = this.tryEntries[e];
            if (r.tryLoc === t) {
              var n = r.completion;
              if ("throw" === n.type) {
                var a = n.arg;
                L(r);
              }
              return a;
            }
          }
          throw new Error("illegal catch attempt");
        },
        delegateYield: function (e, r, n) {
          return (
            (this.delegate = { iterator: M(e), resultName: r, nextLoc: n }),
            "next" === this.method && (this.arg = t),
            g
          );
        },
      }),
      e
    );
  }
  function o(t, e, r, n, a, o, i) {
    try {
      var u = t[o](i),
        s = u.value;
    } catch (t) {
      return void r(t);
    }
    u.done ? e(s) : Promise.resolve(s).then(n, a);
  }
  function i(t) {
    return function () {
      var e = this,
        r = arguments;
      return new Promise(function (n, a) {
        var i = t.apply(e, r);
        function u(t) {
          o(i, n, a, u, s, "next", t);
        }
        function s(t) {
          o(i, n, a, u, s, "throw", t);
        }
        u(void 0);
      });
    };
  }
  function u(t) {
    return (
      (u =
        "function" == typeof Symbol && "symbol" == typeof Symbol.iterator
          ? function (t) {
              return typeof t;
            }
          : function (t) {
              return t &&
                "function" == typeof Symbol &&
                t.constructor === Symbol &&
                t !== Symbol.prototype
                ? "symbol"
                : typeof t;
            }),
      u(t)
    );
  }
  function s(t, e) {
    if (!(t instanceof e))
      throw new TypeError("Cannot call a class as a function");
  }
  function c(t, e) {
    for (var r = 0; r < e.length; r++) {
      var n = e[r];
      (n.enumerable = n.enumerable || !1),
        (n.configurable = !0),
        "value" in n && (n.writable = !0),
        Object.defineProperty(t, l(n.key), n);
    }
  }
  function f(t, e, r) {
    return (
      e && c(t.prototype, e),
      r && c(t, r),
      Object.defineProperty(t, "prototype", { writable: !1 }),
      t
    );
  }
  function l(t) {
    var e = (function (t, e) {
      if ("object" !== u(t) || null === t) return t;
      var r = t[Symbol.toPrimitive];
      if (void 0 !== r) {
        var n = r.call(t, "string");
        if ("object" !== u(n)) return n;
        throw new TypeError("@@toPrimitive must return a primitive value.");
      }
      return String(t);
    })(t);
    return "symbol" === u(e) ? e : String(e);
  }
  var p;
  p =
    "undefined" != typeof WorkerGlobalScope && self instanceof WorkerGlobalScope
      ? self
      : window;
  var h = {
      networkTimeout: 6e3,
      networkRetryTimeout: 1e4,
      networkRetryTimes: 5,
      winLargeSize: 1048576,
      lruCacheSizeTimes: 4,
    },
    m = {
      AUTO_CUT: "AUTO_CUT",
      CUT_ENDED: "CUT_ENDED",
      ON_REQUEST_SUCCESS: "ON_REQUEST_SUCCESS",
    },
    d = {
      MSE_ERROR: "MSE_ERROR",
      WORKER_ERROR: "WORKER_ERROR",
      FFMPEG_ERROR: "FFMPEG_ERROR",
      NETWORK_TIMEOUT_ERROR: "NETWORK_TIMEOUT_ERROR",
      NETWORK_TIMEOUT_RETRY: "NETWORK_TIMEOUT_RETRY",
      NETWORK_REQUEST_ERROR: "NETWORK_REQUEST_ERROR",
      NETWORK_STATUS_CODE_ERROR: "NETWORK_STATUS_CODE_ERROR",
      WASM_ERROR: "WASM_ERROR",
    },
    w = (function () {
      function t(e) {
        s(this, t),
          (this.maxSize = e),
          (this.cacheMap = {}),
          (this.currentByteLen = 0),
          (this.allHitCount = 0),
          (this.allGetCount = 0);
      }
      return (
        f(t, [
          {
            key: "updateLruMaxSize",
            value: function (t) {
              "number" == typeof t && t > 0 && (this.maxSize = t);
            },
          },
          {
            key: "dispose",
            value: function () {
              (this.cacheMap = {}),
                (this.currentByteLen = 0),
                (this.allHitCount = 0),
                (this.allGetCount = 0);
            },
          },
          {
            key: "setData",
            value: function (t, e, r) {
              var n = this.genKey(t, e);
              y.log("[lruCache]setData: ", n, ", byteLen:", r.byteLength),
                (this.cacheMap[n] = {
                  buffer: r,
                  hitCount: 0,
                  lastAccessTime: Date.now(),
                }),
                (this.currentByteLen = this.currentByteLen + r.byteLength);
            },
          },
          {
            key: "genKey",
            value: function (t, e) {
              return "".concat(t, "-").concat(e);
            },
          },
          {
            key: "getBuffer",
            value: function (t, e) {
              var r = this.get(t, e);
              return r ? r.buffer : null;
            },
          },
          {
            key: "getHitRate",
            value: function () {
              return (this.allHitCount / this.allGetCount).toFixed(2);
            },
          },
          {
            key: "get",
            value: function (t, e) {
              var r = this.genKey(t, e),
                n = this.cacheMap[r];
              return (
                (this.allGetCount = this.allGetCount + 1),
                n
                  ? ((n.hitCount = n.hitCount + 1),
                    (n.lastAccessTime = Date.now()),
                    (this.allHitCount = this.allHitCount + 1),
                    y.log(
                      "[lruCache]getBufferSuccess: ",
                      r,
                      " hitCount: ",
                      n.hitCount,
                      " hitRate: ".concat(100 * this.getHitRate(), "%")
                    ),
                    n)
                  : null
              );
            },
          },
          {
            key: "delItem",
            value: function (t) {
              for (
                var e = this,
                  r = Object.keys(this.cacheMap).sort(function (t, r) {
                    return (
                      e.cacheMap[t].lastAccessTime -
                      e.cacheMap[r].lastAccessTime
                    );
                  }),
                  n = r.slice(0, Math.ceil(r.length / 2)),
                  a = [],
                  o = 0;
                o < n.length;
                o++
              ) {
                var i = n[o];
                a.push({
                  key: i,
                  hitCount: this.cacheMap[i].hitCount,
                  byteLength: this.cacheMap[i].buffer.byteLength,
                });
              }
              a.sort(function (t, e) {
                return t.hitCount - e.hitCount;
              });
              for (var u = 0, s = 0; u < t && s < a.length; s++) {
                var c = a[s];
                (u += c.byteLength),
                  delete this.cacheMap[c.key],
                  (this.currentByteLen -= c.byteLength);
              }
              u < t && this.delItem(t - u);
            },
          },
          {
            key: "set",
            value: function (t, e, r) {
              var n = r.byteLength;
              if (this.currentByteLen + n > this.maxSize) {
                var a = this.currentByteLen + n - this.maxSize;
                this.delItem(a);
              }
              this.setData(t, e, r);
            },
          },
        ]),
        t
      );
    })(),
    y = new ((function () {
      function t() {
        s(this, t), (this.TAG = "[Decrypt Worker]");
      }
      return (
        f(t, [
          {
            key: "time",
            value: function (t) {
              p.taskInfo.debugTimeLog && console.time(t);
            },
          },
          {
            key: "timeEnd",
            value: function (t) {
              p.taskInfo.debugTimeLog && console.timeEnd(t);
            },
          },
          {
            key: "log",
            value: function () {
              if (p.taskInfo && p.taskInfo.debug) {
                for (
                  var t, e = arguments.length, r = new Array(e), n = 0;
                  n < e;
                  n++
                )
                  r[n] = arguments[n];
                (t = console).log.apply(t, [this.TAG].concat(r));
              }
            },
          },
          {
            key: "error",
            value: function () {
              for (
                var t, e = arguments.length, r = new Array(e), n = 0;
                n < e;
                n++
              )
                r[n] = arguments[n];
              (t = console).error.apply(t, [this.TAG].concat(r));
            },
          },
          {
            key: "warn",
            value: function () {
              for (
                var t, e = arguments.length, r = new Array(e), n = 0;
                n < e;
                n++
              )
                r[n] = arguments[n];
              (t = console).warn.apply(t, [this.TAG].concat(r));
            },
          },
        ]),
        t
      );
    })())();
  function g() {
    p.lruCache && (p.lruCache.dispose(), (p.lruCache = null));
  }
  function b() {
    (p.files = {}),
      (p.fetchBuffer = void 0),
      (p.fetchBufferSize = 5242880),
      (p.lastBytesRead = 0),
      (p.taskInfo = p.TEST_TASK_INFO || {
        url: "",
        size: 0,
        seed: "",
        disableDecrypt: !1,
        debug: !1,
        debugTimeLog: !1,
      }),
      (p.decryptor = null),
      (p.fmp4ReadyCallBack = null),
      (p.firstWinBufferOkCb = null),
      (p.winBufStart = -1),
      (p.winBufEnd = -1),
      (p.winSize = 524288),
      (p.winBuf = null),
      (p.winBufContentLength = 0),
      (p.winRawBuf = null),
      (p.firstWinBuf = null),
      (p.winLargeSize = h.winLargeSize),
      (p.winSmallSize = 1048576),
      (p.winPreservePastSize = 1048576),
      p.lruCache && p.lruCache.dispose(),
      (p.segmentDuration = 0.5),
      (p.segmentSmallDuration = 0.5),
      (p.segmentSeekDuration = 1),
      (p.segmentSmallCount = 0),
      (p.segmentLargeDuration = 5),
      (p.fetchControllerList = []),
      (p.ffmpeg_errno = 0),
      (p.ffmpeg_errmsg = "OK"),
      (p.io_errmsg = "OK"),
      (p.last_ss = -1),
      (p.fmp4Index = 0),
      (p.info_file = { next_ss: "0", next_dts_v: "0", next_dts_a: "0" }),
      (p.cutFmp4Idle = !0),
      (p.seekTaskCb = null),
      (p.pauseProcess = !1),
      (p.disposingProcess = !1),
      (p.downloading = !1),
      (p.networkTimeout = h.networkTimeout),
      (p.networkRetryTimes = h.networkRetryTimes);
  }
  function v(t) {
    p.winSize = t;
  }
  function _(t) {
    p.segmentDuration = t;
  }
  function k(t) {
    p.taskInfo.size = t;
  }
  function E(t) {
    p.downloading = t;
  }
  function R(t) {
    return x.apply(this, arguments);
  }
  function x() {
    return (x = i(
      a().mark(function t(e) {
        var r, n, o, i;
        return a().wrap(
          function (t) {
            for (;;)
              switch ((t.prev = t.next)) {
                case 0:
                  if (
                    ((r = e.url),
                    (t.prev = 1),
                    !(p.winBufStart < 0 && p.winBufEnd < 0) &&
                      p.winBuf &&
                      p.firstWinBuf)
                  ) {
                    t.next = 26;
                    break;
                  }
                  return E(!0), (t.next = 6), this.getContentLen(r);
                case 6:
                  if (((n = t.sent), (o = 0), !p.winRawBuf)) {
                    t.next = 13;
                    break;
                  }
                  (i = Math.min(n, p.winRawBuf.byteLength - 1)),
                    p.taskInfo.disableDecrypt
                      ? (p.winBuf = new Uint8Array(p.winRawBuf))
                      : (p.winBuf = I(p.winRawBuf, 0)),
                    (t.next = 17);
                  break;
                case 13:
                  return (
                    (i = Math.min(n, p.winSize - 1)),
                    (t.next = 16),
                    N({ url: r, start: o, end: i })
                  );
                case 16:
                  p.winBuf = t.sent;
                case 17:
                  if (
                    ((p.winBufContentLength = p.winBuf.byteLength),
                    E(!1),
                    p.winBuf)
                  ) {
                    t.next = 21;
                    break;
                  }
                  return t.abrupt("return", !1);
                case 21:
                  return (
                    (p.winBufStart = o),
                    (p.winBufEnd = i),
                    (p.firstWinBuf = p.winBuf.slice(0)),
                    p.firstWinBufferOkCb && p.firstWinBufferOkCb(),
                    t.abrupt("return", !0)
                  );
                case 26:
                  return t.abrupt("return", !1);
                case 29:
                  throw ((t.prev = 29), (t.t0 = t.catch(1)), E(!1), t.t0);
                case 33:
                case "end":
                  return t.stop();
              }
          },
          t,
          this,
          [[1, 29]]
        );
      })
    )).apply(this, arguments);
  }
  function S(t) {
    return B.apply(this, arguments);
  }
  function B() {
    return (B = i(
      a().mark(function t(e) {
        var r, n, o, i, u, s, c, f, l, h, m, d, w, g, b, v;
        return a().wrap(
          function (t) {
            for (;;)
              switch ((t.prev = t.next)) {
                case 0:
                  return (
                    (r = e.url),
                    (n = e.start),
                    (o = e.end),
                    (i = e.stream_id),
                    (t.next = 3),
                    this.getContentLen(r)
                  );
                case 3:
                  return (u = t.sent), (t.next = 6), R(e);
                case 6:
                  if (
                    ((m = !1),
                    (d = p.winBufStart),
                    (w = p.winBufEnd),
                    !(n >= 0 && o < p.firstWinBuf.byteLength))
                  ) {
                    t.next = 12;
                    break;
                  }
                  return (
                    (h = p.firstWinBuf.subarray(n, o + 1)),
                    t.abrupt("return", h)
                  );
                case 12:
                  if (!(n < p.winBufStart)) {
                    t.next = 20;
                    break;
                  }
                  (s = n),
                    (g = Math.min(Math.max(o, s + p.winSize - 1), u - 1)) <
                      p.winBufStart || g > p.winBufEnd
                      ? (y.log(
                          "@@@ case1.1 for "
                            .concat(i, " | start: ")
                            .concat(n, ", end: ")
                            .concat(o, ", offsetStart: ")
                            .concat(n - p.winBufStart, ", offsetEnd: ")
                            .concat(o - p.winBufStart, ", winBufStart: ")
                            .concat(p.winBufStart, ", winBufEnd: ")
                            .concat(p.winBufEnd)
                        ),
                        (c = g))
                      : (y.log(
                          "@@@ case1.2 for "
                            .concat(i, " | start: ")
                            .concat(n, ", end: ")
                            .concat(o, ", offsetStart: ")
                            .concat(n - p.winBufStart, ", offsetEnd: ")
                            .concat(o - p.winBufStart, ", winBufStart: ")
                            .concat(p.winBufStart, ", winBufEnd: ")
                            .concat(p.winBufEnd)
                        ),
                        (c = p.winBufStart - 1),
                        (l = p.winBuf.subarray(0, g - p.winBufStart + 1)),
                        (m = !1)),
                    (d = n),
                    (w = g),
                    (t.next = 50);
                  break;
                case 20:
                  if (!(n >= p.winBufStart && o <= p.winBufEnd)) {
                    t.next = 29;
                    break;
                  }
                  return (
                    y.log(
                      "@@@ case2 for "
                        .concat(i, " | start: ")
                        .concat(n, ", end: ")
                        .concat(o, ", offsetStart: ")
                        .concat(n - p.winBufStart, ", offsetEnd: ")
                        .concat(o - p.winBufStart, ", winBufStart: ")
                        .concat(p.winBufStart, ", winBufEnd: ")
                        .concat(p.winBufEnd, ", byteLen: ")
                        .concat(p.winBufContentLength)
                    ),
                    (h = p.winBuf.subarray(
                      n - p.winBufStart,
                      o - p.winBufStart + 1
                    )),
                    (l = null),
                    (d = p.winBufStart),
                    (w = p.winBufEnd),
                    t.abrupt("return", h)
                  );
                case 29:
                  if (
                    !(n >= p.winBufStart && n <= p.winBufEnd && o > p.winBufEnd)
                  ) {
                    t.next = 40;
                    break;
                  }
                  y.log(
                    "@@@ case3 for "
                      .concat(i, " | start: ")
                      .concat(n, ", end: ")
                      .concat(o, ", offsetStart: ")
                      .concat(n - p.winBufStart, ", offsetEnd: ")
                      .concat(o - p.winBufStart, ", winBufStart: ")
                      .concat(p.winBufStart, ", winBufEnd: ")
                      .concat(p.winBufEnd)
                  ),
                    (b = Math.max(o, p.winBufEnd + p.winSize)),
                    (m = !0),
                    (d = Math.max(n - p.winPreservePastSize, p.winBufStart)),
                    (l = p.winBuf.subarray(
                      d - p.winBufStart,
                      p.winBufContentLength
                    )),
                    (s = p.winBufEnd + 1),
                    (c = Math.min(b, u - 1)),
                    (w = c),
                    (t.next = 50);
                  break;
                case 40:
                  if (!(n > p.winBufEnd)) {
                    t.next = 49;
                    break;
                  }
                  y.log(
                    "@@@ case4 for "
                      .concat(i, " | start: ")
                      .concat(n, ", end: ")
                      .concat(o, ", offsetStart: ")
                      .concat(n - p.winBufStart, ", offsetEnd: ")
                      .concat(o - p.winBufStart, ", winBufStart: ")
                      .concat(p.winBufStart, ", winBufEnd: ")
                      .concat(p.winBufEnd)
                  ),
                    (s = n),
                    (c = Math.min(Math.max(o, n + p.winSize - 1), u - 1)),
                    (l = null),
                    (d = s),
                    (w = c),
                    (t.next = 50);
                  break;
                case 49:
                  throw new Error(
                    "getBufferFromWindow error, start: "
                      .concat(n, ", end: ")
                      .concat(o, ", winBufStart: ")
                      .concat(p.winBufStart, ", winBufEnd: ")
                      .concat(p.winBufEnd)
                  );
                case 50:
                  return (t.next = 52), N({ url: r, start: s, end: c });
                case 52:
                  if ((f = t.sent)) {
                    t.next = 55;
                    break;
                  }
                  return t.abrupt("return", null);
                case 55:
                  return (
                    l
                      ? ((p.winBufContentLength = l.byteLength + f.byteLength),
                        p.winBuf.byteLength < p.winBufContentLength &&
                          (p.winBuf = new Uint8Array(p.winBufContentLength)),
                        m
                          ? (p.winBuf.set(l), p.winBuf.set(f, l.byteLength))
                          : (p.winBuf.set(l, f.byteLength), p.winBuf.set(f)))
                      : (f.byteLength > p.winBuf.byteLength
                          ? (p.winBuf = f)
                          : p.winBuf.set(f),
                        (p.winBufContentLength = f.byteLength)),
                    (p.winBufStart = d),
                    (p.winBufEnd = w),
                    (v = p.winBuf.subarray(
                      n - p.winBufStart,
                      o - p.winBufStart + 1
                    )),
                    t.abrupt("return", v)
                  );
                case 60:
                case "end":
                  return t.stop();
              }
          },
          t,
          this
        );
      })
    )).apply(this, arguments);
  }
  function T(t) {
    return "string" == typeof t
      ? t
      : err
      ? t instanceof Error
        ? t.toString()
        : JSON.stringify(t)
      : "";
  }
  function O(t, e, r, n) {
    (n = n || {}),
      r && y.error(t, e, r),
      (stack = r && r.stack ? r.stack : ""),
      (e = "[WorkerError]["
        .concat(t, "]")
        .concat(e, " ")
        .concat(p.taskInfo.url)),
      0 !== p.ffmpeg_errno &&
        y.error(
          "[FFMPEG INTERNAL] "
            .concat(p.ffmpeg_errno, " (")
            .concat(p.ffmpeg_errmsg, ")")
        );
    var a = { __status__: "__WORKER_ERROR__", errType: t, errMsg: e, stack };
    return Object.assign(a, n), self.postMessage(a), a;
  }
  function C(t) {
    return L.apply(this, arguments);
  }
  function L() {
    return (
      (L = i(
        a().mark(function t(e) {
          var r,
            n,
            o = arguments;
          return a().wrap(
            function (t) {
              for (;;)
                switch ((t.prev = t.next)) {
                  case 0:
                    return (
                      (r = o.length > 1 && void 0 !== o[1] ? o[1] : 3),
                      (t.prev = 1),
                      (t.next = 4),
                      fetch(e, { method: "HEAD" })
                    );
                  case 4:
                    return (n = t.sent), t.abrupt("return", n.headers);
                  case 8:
                    if (((t.prev = 8), (t.t0 = t.catch(1)), !((r -= 1) > 0))) {
                      t.next = 17;
                      break;
                    }
                    return (t.next = 14), C(e, r);
                  case 14:
                    return t.abrupt("return", t.sent);
                  case 17:
                    return (
                      O(
                        d.NETWORK_REQUEST_ERROR,
                        "getHeaders error: " + T(t.t0),
                        t.t0
                      ),
                      t.abrupt("return", null)
                    );
                  case 19:
                  case "end":
                    return t.stop();
                }
            },
            t,
            null,
            [[1, 8]]
          );
        })
      )),
      L.apply(this, arguments)
    );
  }
  function I(t, e) {
    for (
      var r = new Uint8Array(t), n = 0;
      n < t.byteLength && e + n < p.decryptor_array.length;
      n++
    )
      r[n] ^= p.decryptor_array[n];
    return r;
  }
  function M(t) {
    return P.apply(this, arguments);
  }
  function P() {
    return (P = i(
      a().mark(function t(e) {
        var r, n, o, i;
        return a().wrap(
          function (t) {
            for (;;)
              switch ((t.prev = t.next)) {
                case 0:
                  if (
                    ((p.disposingProcess = !0),
                    (r = e.disposeOptions || {}),
                    (n = r.terminateWorker),
                    (t.prev = 3),
                    !p.fetchControllerList)
                  ) {
                    t.next = 18;
                    break;
                  }
                  o = 0;
                case 6:
                  if (!(o < p.fetchControllerList.length)) {
                    t.next = 18;
                    break;
                  }
                  (t.prev = 7),
                    (i = p.fetchControllerList[o]) && i.abort(),
                    (t.next = 15);
                  break;
                case 12:
                  return (
                    (t.prev = 12), (t.t0 = t.catch(7)), t.abrupt("continue", 15)
                  );
                case 15:
                  o++, (t.next = 6);
                  break;
                case 18:
                  e = e || {};
                case 19:
                  if (((t.prev = 19), n)) {
                    t.next = 23;
                    break;
                  }
                  return (t.next = 23), G();
                case 23:
                  return (
                    b(),
                    g(),
                    (p.disposingProcess = !1),
                    V({}, Z(e)),
                    n && ((p.outputBuffer = null), p.close && p.close()),
                    t.finish(19)
                  );
                case 29:
                case "end":
                  return t.stop();
              }
          },
          t,
          null,
          [
            [3, , 19, 29],
            [7, 12],
          ]
        );
      })
    )).apply(this, arguments);
  }
  function U(t) {
    if (t && p.fetchControllerList && p.fetchControllerList.length) {
      for (var e = [], r = 0; r < p.fetchControllerList.length; r++) {
        var n = p.fetchControllerList[r];
        if (n.__mid__ !== t.__mid__) e.push(n);
        else
          try {
            n && n.abort();
          } catch (t) {
            continue;
          }
      }
      p.fetchControllerList = e;
    }
  }
  function F(t) {
    return "number" == typeof p.networkRetryTimes && "number" == typeof t
      ? p.networkRetryTimes - t
      : 0;
  }
  function A(t, e) {
    if (!e || !t) return t;
    var r = (function (t) {
      if (!t) return "";
      try {
        return new URL(t).hostname;
      } catch (t) {
        return "";
      }
    })(t);
    if (r !== e) {
      var n = t.replace(r, e);
      return console.log("[sniffer]切换到备用域名: ", e, "原域名: ", r), n;
    }
    return t;
  }
  function W(t, e) {
    if (
      p.fallbackHost &&
      p.networkRetryTimes - p.retryTimesToSwitchHost === e
    ) {
      var r = t.url,
        n = A(r, p.fallbackHost);
      r !== n &&
        ((t.url = n),
        r.includes(p.taskInfo.url) &&
          (p.taskInfo.url = A(p.taskInfo.url, p.fallbackHost)));
    }
  }
  function N(t, e, r) {
    return new Promise(function (n, o) {
      var u,
        s,
        c = t.start,
        f = t.end,
        l = t.url || "",
        h =
          ((u = Date.now()),
          (s = Math.random().toString(36).substring(2, 8)),
          "".concat(u, "-").concat(s)),
        w = ""
          .concat(l)
          .concat(l.includes("?") ? "&" : "?", "_pUid_=")
          .concat(h);
      void 0 === e && (e = p.networkRetryTimes);
      var g,
        b = r ? p.networkRetryTimeout : p.networkTimeout,
        v = Math.abs(f - c) + 1,
        _ = f ? "bytes=".concat(c, "-").concat(f) : "bytes=".concat(c, "-"),
        k = Date.now();
      try {
        var E = new XMLHttpRequest();
        (g = E) &&
          ((p.fetchControllerList = p.fetchControllerList || []),
          (g.__mid__ = Date.now()),
          p.fetchControllerList.push(g)),
          E.open("GET", w),
          E.setRequestHeader("range", _),
          (E.timeout = b),
          (E.responseType = "arraybuffer"),
          (E.onreadystatechange = function (t) {
            if (p.disposingProcess) return U(E), void o("abort");
            if (4 === E.readyState) {
              if (E.status >= 200 && E.status < 300) {
                var a = new Uint8Array(E.response);
                if (p.taskInfo.disableDecrypt) n(a);
                else {
                  var i = I(a, c);
                  n(i);
                }
                var u = Date.now() - k,
                  s = (function (t) {
                    try {
                      var e = performance
                        .getEntriesByType("resource")
                        .find(function (e) {
                          return e.name === t;
                        });
                      return e
                        ? {
                            total: e.responseEnd - e.startTime,
                            dnsLookup: e.domainLookupEnd - e.domainLookupStart,
                            tcpConnect: e.connectEnd - e.connectStart,
                            firstByte: e.responseStart - e.startTime,
                            download: e.responseEnd - e.responseStart,
                            redirect: e.redirectEnd - e.redirectStart,
                          }
                        : {};
                    } catch (t) {
                      return { errMsg: t.toString() };
                    }
                  })(w),
                  f = (function (t) {
                    var e = t
                        .getAllResponseHeaders()
                        .trim()
                        .split(/[\r\n]+/),
                      r = {};
                    return (
                      e.forEach(function (t) {
                        var e = t.split(": "),
                          n = e.shift().toLowerCase(),
                          a = e.join(": ");
                        r[n] = a;
                      }),
                      r
                    );
                  })(E);
                (h =
                  (h = {
                    url: w,
                    startIdx: c,
                    size: a.byteLength,
                    cost: u,
                    timeout: b,
                    startTs: k,
                    isRetry: !!r,
                    retryTime: F(e),
                    perf: s,
                    worker: !0,
                    respHeaders: f,
                  }) || {}),
                  V(Object.assign(h, { cmd: m.ON_REQUEST_SUCCESS }));
              } else if (E.status >= 400) {
                var l = "[NetWork Error]url: "
                  .concat(w, ", status: ")
                  .concat(E.status);
                O(
                  d.NETWORK_STATUS_CODE_ERROR,
                  l,
                  {},
                  {
                    networkInfo: { status: E.status },
                    size: v,
                    timeout: b,
                    startTs: k,
                    startIdx: c,
                  }
                ),
                  o(l);
              }
              U(E);
            }
            var h;
          }),
          (E.ontimeout = i(
            a().mark(function r() {
              var i, u, s;
              return a().wrap(
                function (r) {
                  for (;;)
                    switch ((r.prev = r.next)) {
                      case 0:
                        if (
                          ((i = "[worker xhr ontimeout]".concat(w)),
                          (u = "[NetWork Error]".concat(i)),
                          U(E),
                          !p.disposingProcess)
                        ) {
                          r.next = 6;
                          break;
                        }
                        return o(u), r.abrupt("return");
                      case 6:
                        if (!(e <= 0)) {
                          r.next = 11;
                          break;
                        }
                        O(
                          d.NETWORK_TIMEOUT_ERROR,
                          i,
                          {},
                          { startTs: k, size: v, startIdx: c, timeout: b }
                        ),
                          o(u),
                          (r.next = 28);
                        break;
                      case 11:
                        return (
                          e--,
                          (r.prev = 12),
                          O(
                            d.NETWORK_TIMEOUT_RETRY,
                            i,
                            {},
                            { startTs: k, size: v, startIdx: c, timeout: b }
                          ),
                          y.log("@@@start retry xhr timeout: ", e),
                          W(t, e),
                          (r.next = 18),
                          N(t, e, !0)
                        );
                      case 18:
                        (s = r.sent), n(s), (r.next = 25);
                        break;
                      case 22:
                        (r.prev = 22), (r.t0 = r.catch(12)), o(r.t0);
                      case 25:
                        return (
                          (r.prev = 25),
                          y.log("@@@end retry xhr timeout: ", e),
                          r.finish(25)
                        );
                      case 28:
                      case "end":
                        return r.stop();
                    }
                },
                r,
                null,
                [[12, 22, 25, 28]]
              );
            })
          )),
          E.addEventListener(
            "error",
            (function () {
              var r = i(
                a().mark(function r(i) {
                  var u, s, f;
                  return a().wrap(
                    function (r) {
                      for (;;)
                        switch ((r.prev = r.next)) {
                          case 0:
                            if (
                              ((u = "[worker xhr onerror]"
                                .concat(i.type, ": ")
                                .concat(i.loaded, ", ")
                                .concat(w)),
                              (s = "[NetWork Error]".concat(u)),
                              U(E),
                              !p.disposingProcess)
                            ) {
                              r.next = 6;
                              break;
                            }
                            return o(s), r.abrupt("return");
                          case 6:
                            if (!(e <= 0)) {
                              r.next = 11;
                              break;
                            }
                            O(
                              d.NETWORK_REQUEST_ERROR,
                              u,
                              {},
                              { startTs: k, size: v, startIdx: c, timeout: b }
                            ),
                              o(s),
                              (r.next = 23);
                            break;
                          case 11:
                            return (
                              e--,
                              W(t, e),
                              (r.prev = 13),
                              (r.next = 16),
                              N(t, e)
                            );
                          case 16:
                            (f = r.sent), n(f), (r.next = 23);
                            break;
                          case 20:
                            (r.prev = 20), (r.t0 = r.catch(13)), o(r.t0);
                          case 23:
                          case "end":
                            return r.stop();
                        }
                    },
                    r,
                    null,
                    [[13, 20]]
                  );
                })
              );
              return function (t) {
                return r.apply(this, arguments);
              };
            })()
          ),
          E.send();
      } catch (t) {
        O(d.NETWORK_REQUEST_ERROR, "".concat(T(t), ", ").concat(w), t, {
          startTs: k,
          size: v,
          timeout: b,
          startIdx: c,
        }),
          o(t);
      }
    });
  }
  function D() {
    var t = p.info_file.next_ss;
    return -1 === Number(t) || p.disposingProcess;
  }
  function z() {
    V({ cmd: m.CUT_ENDED });
  }
  function j(t) {
    return H.apply(this, arguments);
  }
  function H() {
    return (H = i(
      a().mark(function t(e) {
        var r, n, o, i, u, s, c, f, l, h, m, w;
        return a().wrap(
          function (t) {
            for (;;)
              switch ((t.prev = t.next)) {
                case 0:
                  if (
                    ((p.cutFmp4Idle = !1),
                    (t.prev = 2),
                    p.fmp4Index++,
                    (r = p.fmp4Index),
                    v(p.winLargeSize),
                    (n = e && "number" == typeof e.timestampOffset),
                    (o = n ? 1e3 * Math.ceil(e.timestampOffset) * 1e3 : void 0),
                    (i = !(1 !== r && (!n || 0 != o))),
                    (n || 1 === r) && (p.segmentSmallCount = 3),
                    p.segmentSmallCount > 0
                      ? (_(p.segmentSmallDuration), p.segmentSmallCount--)
                      : _(p.segmentLargeDuration),
                    n &&
                      0 === o &&
                      ((p.info_file.next_ss = "0"),
                      (p.info_file.next_dts_v = "0"),
                      (p.info_file.next_dts_a = "0")),
                    (u = "wasm:outputStream_".concat(r)),
                    (s = n && o > 0 ? "".concat(o) : p.info_file.next_ss),
                    (c = n && o > 0 ? "null" : p.info_file.next_dts_v),
                    (f = n && o > 0 ? "null" : p.info_file.next_dts_a),
                    y.log(
                      "###[worker] next_ss=".concat(s, " timestamp=").concat(o)
                    ),
                    D() && !n)
                  ) {
                    t.next = 52;
                    break;
                  }
                  return (
                    (p.last_ss = s),
                    y.time("@@@wxTransSingleFmp4EncryptJs"),
                    (t.next = 22),
                    Module.wxTransSingleFmp4EncryptJs(
                      "wasm:inputStream",
                      u,
                      i,
                      "null" == c,
                      p.segmentDuration,
                      s,
                      c,
                      f
                    )
                  );
                case 22:
                  if (
                    ((p.info_file = t.sent),
                    y.timeEnd("@@@wxTransSingleFmp4EncryptJs"),
                    "number" == typeof (l = p.info_file.ffmpeg_ret) && 0 === l)
                  ) {
                    t.next = 30;
                    break;
                  }
                  return (
                    (h = "ffmpeg return error: ".concat(
                      JSON.stringify(p.info_file)
                    )),
                    (m = O(d.FFMPEG_ERROR, h)),
                    (p.cutFmp4Idle = !0),
                    t.abrupt("return", { success: !1, ffmpegRetInfo: m })
                  );
                case 30:
                  if ("OK" === p.io_errmsg) {
                    t.next = 34;
                    break;
                  }
                  return (
                    (p.cutFmp4Idle = !0),
                    (p.io_errmsg = "OK"),
                    t.abrupt("return", { success: !1 })
                  );
                case 34:
                  if (!p.disposingProcess) {
                    t.next = 37;
                    break;
                  }
                  return (
                    (p.cutFmp4Idle = !0), t.abrupt("return", { success: !0 })
                  );
                case 37:
                  if (!p.seekTaskCb) {
                    t.next = 41;
                    break;
                  }
                  return (
                    p.seekTaskCb(),
                    (p.seekTaskCb = null),
                    t.abrupt("return", { success: !0 })
                  );
                case 41:
                  if (!D()) {
                    t.next = 45;
                    break;
                  }
                  return z(), (p.cutFmp4Idle = !0), t.abrupt("return");
                case 45:
                  if (!p.pauseProcess) {
                    t.next = 48;
                    break;
                  }
                  return (
                    (p.cutFmp4Idle = !0), t.abrupt("return", { success: !0 })
                  );
                case 48:
                  return (e = void 0), t.abrupt("continue", 0);
                case 52:
                  z(), (p.cutFmp4Idle = !0);
                case 54:
                  return t.abrupt("return", { success: !0 });
                case 57:
                  return (
                    (t.prev = 57),
                    (t.t0 = t.catch(2)),
                    O(d.WASM_ERROR, T(t.t0), t.t0),
                    (p.cutFmp4Idle = !0),
                    (w = Object.assign({}, p.info_file, {
                      message: T(t.t0),
                      stack: t.t0.stack || "",
                    })),
                    t.abrupt("return", { success: !1, ffmpegRetInfo: w })
                  );
                case 63:
                  t.next = 0;
                  break;
                case 65:
                case "end":
                  return t.stop();
              }
          },
          t,
          null,
          [[2, 57]]
        );
      })
    )).apply(this, arguments);
  }
  function G() {
    return K.apply(this, arguments);
  }
  function K() {
    return (K = i(
      a().mark(function t() {
        return a().wrap(function (t) {
          for (;;)
            switch ((t.prev = t.next)) {
              case 0:
                if (!p.cutFmp4Idle || p.downloading) {
                  t.next = 2;
                  break;
                }
                return t.abrupt("return", !0);
              case 2:
                return t.abrupt(
                  "return",
                  new Promise(function (t) {
                    var e = setInterval(function () {
                      p.cutFmp4Idle &&
                        !p.downloading &&
                        (t(!0), clearInterval(e));
                    }, 10);
                  })
                );
              case 3:
              case "end":
                return t.stop();
            }
        }, t);
      })
    )).apply(this, arguments);
  }
  function Q(t) {
    p.pauseProcess = t;
  }
  function Y(t) {
    (t.firstRawBuf = null),
      p.registFmp4ReadyCb(function (r, n) {
        t
          ? (y.time("@@@registFmp4ReadyCb"),
            p.postMessage(
              e(
                e({ success: !0, buffer: r.buffer }, t),
                {},
                {
                  decryptor_array: p.decryptor_array,
                  fmp4Index: p.fmp4Index,
                  duration: n,
                }
              ),
              [r.buffer]
            ),
            (t = null))
          : p.postMessage(
              {
                success: !0,
                buffer: r.buffer,
                cmd: m.AUTO_CUT,
                decryptor_array: p.decryptor_array,
                fmp4Index: p.fmp4Index,
              },
              [r.buffer]
            );
      });
  }
  function J(t, r) {
    if (t && !t.success) {
      var n = Object.assign(
        e(
          e({}, r),
          {},
          { decryptor_array: p.decryptor_array, fmp4Index: p.fmp4Index }
        ),
        t
      );
      p.postMessage(n);
    }
  }
  function q(t) {
    console.log("@@@updateGlobalConfig: ", t);
    var e,
      r,
      n = (t = t || {}),
      a = n.timeout,
      o = n.winBufSize,
      i = n.networkRetryTimes,
      s = n.retryTimeout,
      c = n.useLruCache,
      f = n.lruCacheSize;
    (e = n.fallbackHostConfig)
      ? ((p.fallbackHostConfig = p.fallbackHostConfig || {}),
        "object" === u(e) &&
          (Object.assign(p.fallbackHostConfig, e),
          (p.fallbackHost = p.fallbackHostConfig.fallbackHost || ""),
          (p.retryTimesToSwitchHost =
            p.fallbackHostConfig.retryTimesToSwitchHost || 1)))
      : (p.fallbackHostConfig = {}),
      "number" == typeof a &&
        a > 0 &&
        (function (t) {
          "number" == typeof t && t > 0 && (p.networkTimeout = t);
        })(a),
      "number" == typeof s &&
        s > 0 &&
        (function (t) {
          "number" == typeof t && t > 0 && (p.networkRetryTimeout = t);
        })(s),
      "number" == typeof i &&
        i > 0 &&
        "number" == typeof (r = i) &&
        r > 0 &&
        (p.networkRetryTimes = r),
      "number" == typeof o &&
        o > 0 &&
        (function (t) {
          "number" == typeof t && t > 0 && (p.winLargeSize = t);
        })(o),
      "boolean" == typeof c &&
        c &&
        p.lruCache &&
        p.lruCache.updateLruMaxSize(f);
  }
  function X(t) {
    return $.apply(this, arguments);
  }
  function $() {
    return ($ = i(
      a().mark(function t(e) {
        var r, n, o, u, s, c, f, l, m, d, _, E, x;
        return a().wrap(function (t) {
          for (;;)
            switch ((t.prev = t.next)) {
              case 0:
                if (
                  ((r = e.url),
                  (n = e.seed),
                  (o = e.disableDecrypt),
                  (u = e.first),
                  (s = e.timestampOffset),
                  (c = e.segmentDuration),
                  (f = e.firstRawBuf),
                  (l = e.contentLen),
                  (m = e.debug),
                  (d = e.debugTimeLog),
                  (_ = e.useLruCache),
                  (E = e.lruCacheSize),
                  y.log("@@@lru config: ", _, E, u),
                  !p.disposingProcess)
                ) {
                  t.next = 4;
                  break;
                }
                return t.abrupt("return");
              case 4:
                if (((x = !!("number" == typeof s && s >= 0)), !u && x)) {
                  t.next = 16;
                  break;
                }
                if (!p.cutFmp4Idle) {
                  t.next = 12;
                  break;
                }
                b(),
                  _
                    ? (("number" == typeof (B = E) && B > 0) ||
                        (B = p.winLargeSize * h.lruCacheSizeTimes),
                      (p.lruCache = new w(B)))
                    : g(),
                  q(e),
                  (t.next = 14);
                break;
              case 12:
                return (
                  y.error("startCut: WORKER_CMD==CUT but wasm is running"),
                  t.abrupt("return")
                );
              case 14:
                t.next = 17;
                break;
              case 16:
                Q(!1);
              case 17:
                if (
                  (f && ((S = f), (p.winRawBuf = S)),
                  l && k(l),
                  (p.taskInfo.debug = m),
                  (p.taskInfo.debugTimeLog = d),
                  (p.taskInfo.url = r),
                  (p.taskInfo.seed = n),
                  (p.taskInfo.disableDecrypt = o),
                  (p.segmentLargeDuration = c || 5),
                  p.decryptor || o)
                ) {
                  t.next = 28;
                  break;
                }
                return (t.next = 28), p.initDecrtptor(n);
              case 28:
                if (u || !x) {
                  t.next = 40;
                  break;
                }
                if (!p.cutFmp4Idle) {
                  t.next = 37;
                  break;
                }
                return Y(e), (t.next = 33), j(e);
              case 33:
                J(t.sent, e), (t.next = 38);
                break;
              case 37:
                p.registSeekTaskCb(
                  i(
                    a().mark(function t() {
                      return a().wrap(function (t) {
                        for (;;)
                          switch ((t.prev = t.next)) {
                            case 0:
                              return Y(e), (t.next = 3), j(e);
                            case 3:
                              J(t.sent, e);
                            case 5:
                            case "end":
                              return t.stop();
                          }
                      }, t);
                    })
                  )
                );
              case 38:
                t.next = 41;
                break;
              case 40:
                p.firstWinBuf ||
                  (y.time("@@@firstWinBuf"),
                  p.registFirstWinBufferOkCb(
                    i(
                      a().mark(function t() {
                        return a().wrap(function (t) {
                          for (;;)
                            switch ((t.prev = t.next)) {
                              case 0:
                                return (
                                  y.timeEnd("@@@firstWinBuf"),
                                  Y(e),
                                  (t.next = 4),
                                  j(e)
                                );
                              case 4:
                                J(t.sent, e);
                              case 6:
                              case "end":
                                return t.stop();
                            }
                        }, t);
                      })
                    )
                  ),
                  v(p.winSmallSize),
                  R({ url: r }));
              case 41:
                y.time("@@@registFmp4ReadyCb");
              case 42:
              case "end":
                return t.stop();
            }
          var S, B;
        }, t);
      })
    )).apply(this, arguments);
  }
  function V(t, e) {
    (t = t || {}),
      Object.assign(t, { fmp4Index, __cbId__: e }),
      self.postMessage && self.postMessage(t);
  }
  function Z(t) {
    return (t = t || {}).__cbId__;
  }
  function tt(t) {
    return et.apply(this, arguments);
  }
  function et() {
    return (et = i(
      a().mark(function t(e) {
        var r, n, o;
        return a().wrap(function (t) {
          for (;;)
            switch ((t.prev = t.next)) {
              case 0:
                return (
                  (r = e.buf),
                  (n = e.seed),
                  (o = e.start),
                  (t.next = 3),
                  p.initDecrtptor(n)
                );
              case 3:
                V({ buffer: I(r, o) }, Z(e));
              case 5:
              case "end":
                return t.stop();
            }
        }, t);
      })
    )).apply(this, arguments);
  }
  b(),
    (p.fallbackHostConfig = {}),
    (p.outputBufferIncSize = 16777216),
    (p.outputBuffer = new Uint8Array(p.outputBufferIncSize)),
    (p.fallbackHostConfig = {}),
    (p.fallbackHost = ""),
    (p.retryTimesToSwitchHost = 1),
    (p.registSeekTaskCb = function (t) {
      p.seekTaskCb = t;
    }),
    (p.registFirstWinBufferOkCb = function (t) {
      p.firstWinBufferOkCb = t;
    }),
    (p.registFmp4ReadyCb = function (t) {
      p.fmp4ReadyCallBack = t;
    }),
    (p.initDecrtptor = (function () {
      var t = i(
        a().mark(function t(e) {
          return a().wrap(function (t) {
            for (;;)
              switch ((t.prev = t.next)) {
                case 0:
                  return (t.next = 2), new Module.WxIsaac64(e);
                case 2:
                  return (
                    (p.decryptor = t.sent),
                    (t.next = 5),
                    p.decryptor.generate(131072)
                  );
                case 5:
                  return (t.next = 7), p.decryptor.delete();
                case 7:
                case "end":
                  return t.stop();
              }
          }, t);
        })
      );
      return function (e) {
        return t.apply(this, arguments);
      };
    })()),
    (self.onerror = function (t) {
      O(d.WORKER_ERROR, t.toString(), t);
    }),
    (p.getContentLen = (function () {
      var t = i(
        a().mark(function t(e) {
          var r, n, o, i, u;
          return a().wrap(
            function (t) {
              for (;;)
                switch ((t.prev = t.next)) {
                  case 0:
                    if (!p.taskInfo.size) {
                      t.next = 2;
                      break;
                    }
                    return t.abrupt("return", p.taskInfo.size);
                  case 2:
                    return (r = null), (t.prev = 3), (t.next = 6), C(e);
                  case 6:
                    if ((n = t.sent)) {
                      t.next = 9;
                      break;
                    }
                    return t.abrupt("return", null);
                  case 9:
                    (o = n.get("content-length") || n.get("CONTENT-LENGTH"))
                      ? (r = Number(o))
                      : "string" ==
                          typeof (i =
                            n.get("content-range") || n.get("CONTENT-RANGE")) &&
                        i.includes("/") &&
                        ((u = i.split("/")[1]), (r = Number(u))),
                      "number" != typeof r
                        ? O(
                            d.NETWORK_REQUEST_ERROR,
                            "can not get content-length/content-range"
                          )
                        : (p.taskInfo.size = r),
                      (t.next = 17);
                    break;
                  case 14:
                    (t.prev = 14),
                      (t.t0 = t.catch(3)),
                      O(
                        d.NETWORK_REQUEST_ERROR,
                        "get content-length error: " + T(t.t0),
                        t.t0
                      );
                  case 17:
                    return t.abrupt("return", r);
                  case 18:
                  case "end":
                    return t.stop();
                }
            },
            t,
            null,
            [[3, 14]]
          );
        })
      );
      return function (e) {
        return t.apply(this, arguments);
      };
    })()),
    (p.wasm_ffmpeg_fopen = (function () {
      var t = i(
        a().mark(function t(e, r, n) {
          var o, i, u, s;
          return a().wrap(
            function (t) {
              for (;;)
                switch ((t.prev = t.next)) {
                  case 0:
                    return (
                      (o = Object.keys(files).length),
                      (o += 1),
                      (t.prev = 2),
                      (i = new Uint8Array(Module.HEAPU8.buffer, e, r)),
                      (u = String.fromCharCode.apply(null, i)),
                      (t.next = 7),
                      getContentLen(p.taskInfo.url)
                    );
                  case 7:
                    return (
                      (s = t.sent),
                      (files[o] = 0 == n ? [u, n, 0, s] : [u, n, 0, 0]),
                      t.abrupt("return", o)
                    );
                  case 12:
                    return (
                      (t.prev = 12),
                      (t.t0 = t.catch(2)),
                      (p.io_errmsg = "wasm_ffmpeg_fopen"),
                      O(d.FFMPEG_ERROR, T(t.t0), t.t0),
                      t.abrupt("return", -1)
                    );
                  case 17:
                  case "end":
                    return t.stop();
                }
            },
            t,
            null,
            [[2, 12]]
          );
        })
      );
      return function (e, r, n) {
        return t.apply(this, arguments);
      };
    })()),
    (p.wasm_ffmpeg_fread = (function () {
      var t = i(
        a().mark(function t(e, n, o, i) {
          var u, s, c, f, l, h, m, w, g;
          return a().wrap(
            function (t) {
              for (;;)
                switch ((t.prev = t.next)) {
                  case 0:
                    if (files[e]) {
                      t.next = 3;
                      break;
                    }
                    return (
                      O(
                        d.FFMPEG_ERROR,
                        "[wasm_ffmpeg_fread] can't find fd data"
                      ),
                      t.abrupt("return", 0)
                    );
                  case 3:
                    if (
                      ((t.prev = 3),
                      (u = r(files[e], 4))[0],
                      (s = u[1]),
                      (c = u[2]),
                      u[3],
                      (f = new Uint8Array(Module.HEAPU8.buffer, n, o)),
                      0 == s)
                    ) {
                      t.next = 8;
                      break;
                    }
                    throw "must be read-only mode";
                  case 8:
                    if (!p.disposingProcess) {
                      t.next = 10;
                      break;
                    }
                    return t.abrupt("return", 0);
                  case 10:
                    return (t.next = 12), getContentLen(p.taskInfo.url);
                  case 12:
                    if (
                      ((l = t.sent),
                      (o = Math.min(o, l - c)),
                      (h = c),
                      (m = c + o - 1),
                      p.lruCache && (w = p.lruCache.getBuffer(h, m)),
                      y.log(
                        "@@@[fread]start: "
                          .concat(h, ", end: ")
                          .concat(m, ", size: ")
                          .concat(o / 1024, ", hitLruCache: ")
                          .concat(!!w)
                      ),
                      w)
                    ) {
                      t.next = 24;
                      break;
                    }
                    return (
                      (g = {
                        url: p.taskInfo.url,
                        start: h,
                        end: m,
                        stream_id: i,
                      }),
                      (t.next = 22),
                      S(g)
                    );
                  case 22:
                    (w = t.sent),
                      p.lruCache && p.lruCache.set(h, m, w.slice(0, o));
                  case 24:
                    if (w) {
                      t.next = 26;
                      break;
                    }
                    return t.abrupt("return", -1);
                  case 26:
                    return (
                      f.set(w.subarray(0, o)),
                      (files[e][2] += o),
                      (p.lastBytesRead = c),
                      t.abrupt("return", o)
                    );
                  case 32:
                    if (
                      ((t.prev = 32), (t.t0 = t.catch(3)), !p.disposingProcess)
                    ) {
                      t.next = 37;
                      break;
                    }
                    return (
                      console.warn("@@@[wasm_ffmpeg_fread] disposing", t.t0),
                      t.abrupt("return", 0)
                    );
                  case 37:
                    return (
                      (p.io_errmsg = "wasm_ffmpeg_fread"),
                      O(
                        d.FFMPEG_ERROR,
                        "[wasm_ffmpeg_fread] ".concat(T(t.t0)),
                        t.t0
                      ),
                      t.abrupt("return", -1)
                    );
                  case 40:
                  case "end":
                    return t.stop();
                }
            },
            t,
            null,
            [[3, 32]]
          );
        })
      );
      return function (e, r, n, a) {
        return t.apply(this, arguments);
      };
    })()),
    (p.wasm_ffmpeg_fwrite = function (t, e, n) {
      if (!files[t])
        return O(d.FFMPEG_ERROR, "[wasm_ffmpeg_fwrite] can't find fd data"), n;
      if (p.disposingProcess)
        return console.warn("@@@[wasm_ffmpeg_fwrite] disposing"), n;
      try {
        var a = r(files[t], 4),
          o = (a[0], a[1], a[2]),
          i = (a[3], new Uint8Array(Module.HEAPU8.buffer, e, n));
        if (o + n >= p.outputBuffer.byteLength) {
          var u = new Uint8Array(o + n + p.outputBufferIncSize);
          u.set(p.outputBuffer), (p.outputBuffer = u);
        }
        for (var s = 0; s < n; s++) p.outputBuffer[o + s] = i[s];
        return (files[t][2] = o + n), n;
      } catch (t) {
        return (
          (p.io_errmsg = "wasm_ffmpeg_fwrite"),
          O(d.FFMPEG_ERROR, "[wasm_ffmpeg_fwrite]: ".concat(T(t)), t),
          -1
        );
      }
    }),
    (p.wasm_ffmpeg_fseek = function (t, e, n, a, o) {
      if (files[t])
        try {
          var i = new Uint8Array(Module.HEAPU8.buffer, e, n),
            u = parseInt(String.fromCharCode.apply(null, i)),
            s = r(files[t], 4),
            c = (s[0], s[1], s[2], s[3]);
          if (0 == a) files[t][2] = u;
          else if (1 == a) files[t][2] += u;
          else {
            if (2 != a) throw "unknown whence in seek";
            files[t][2] = c + u;
          }
          for (
            var f = files[t][2].toString(),
              l = new Uint8Array(Module.HEAPU8.buffer, o, 32),
              h = 0;
            h < f.length;
            h++
          )
            l[h] = f.charCodeAt(h);
        } catch (t) {
          return (
            (p.io_errmsg = "wasm_ffmpeg_fseek"),
            void O(d.FFMPEG_ERROR, "[wasm_ffmpeg_fseek] ".concat(T(t)), t)
          );
        }
      else O(d.FFMPEG_ERROR, "[wasm_ffmpeg_fseek] can't find fd data");
    }),
    (p.wasm_ffmpeg_fclose = function (t, e) {
      var n = p.files;
      if (n[t])
        try {
          var a = r(n[t], 4),
            o = (a[0], a[1]),
            i = a[2];
          if ((a[3], 0 != o && i > 0)) {
            var u = p.outputBuffer.slice(0, i);
            (p.info_file.duration_v = p.info_file.duration_a = e),
              p.disposingProcess ||
                (p.fmp4ReadyCallBack && p.fmp4ReadyCallBack(u, e));
          }
        } catch (t) {
          (p.io_errmsg = "wasm_ffmpeg_fclose"),
            O(d.FFMPEG_ERROR, "[wasm_ffmpeg_fclose] ".concat(T(t)), t);
        }
      else O(d.FFMPEG_ERROR, "[wasm_ffmpeg_fclose] can't find fd data");
    }),
    (p.wasm_ffmpeg_fsize = function (t, e) {
      if (files[t])
        for (
          var n = r(files[t], 4),
            a = (n[0], n[1]),
            o = n[2],
            i = n[3],
            u = (0 != a ? o : i).toString(),
            s = new Uint8Array(Module.HEAPU8.buffer, e, 32),
            c = 0;
          c < u.length;
          c++
        )
          s[c] = u.charCodeAt(c);
      else O(d.FFMPEG_ERROR, "[wasm_ffmpeg_fsize] can't find fd data");
    }),
    (p.wasm_ffmpeg_error_report = function (t, e, r) {
      p.ffmpeg_errno = t;
      var n = new Uint8Array(Module.HEAPU8.buffer, e, r);
      p.ffmpeg_errmsg = String.fromCharCode.apply(null, n);
    }),
    (p.wasm_isaac_generate = function (t, e) {
      p.decryptor_array = new Uint8Array(e);
      var r = new Uint8Array(Module.HEAPU8.buffer, t, e);
      p.decryptor_array.set(r.reverse());
    }),
    (p.onmessage = (function () {
      var t = i(
        a().mark(function t(e) {
          var r, n;
          return a().wrap(function (t) {
            for (;;)
              switch ((t.prev = t.next)) {
                case 0:
                  if (((r = e.data), "CUT" !== (n = r.cmd))) {
                    t.next = 6;
                    break;
                  }
                  X(r), (t.next = 13);
                  break;
                case 6:
                  if ("DISPOSE" !== n) {
                    t.next = 12;
                    break;
                  }
                  return (
                    y.log("@@@[decryptWorker]worker start dispose"),
                    (t.next = 10),
                    M(r)
                  );
                case 10:
                  t.next = 13;
                  break;
                case 12:
                  "PAUSE" === n
                    ? (p.pauseProcess || Q(!0), V({}, Z(r)))
                    : "RESUME" === n
                    ? (p.pauseProcess && !D() && (Q(!1), j()), V({}, Z(r)))
                    : "UPDATE_CONFIG" === n
                    ? q(r)
                    : "DECRYPT_BUFFER" === n && tt(r);
                case 13:
                case "end":
                  return t.stop();
              }
          }, t);
        })
      );
      return function (e) {
        return t.apply(this, arguments);
      };
    })());
})();
