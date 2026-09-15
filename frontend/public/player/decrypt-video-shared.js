var Vl = Object.defineProperty;
var Dl = (t, e, r) =>
  e in t
    ? Vl(t, e, {
        enumerable: !0,
        configurable: !0,
        writable: !0,
        value: r,
      })
    : (t[e] = r);
var ye = (t, e, r) => Dl(t, typeof e != "symbol" ? e + "" : e, r);
var lp =
  typeof globalThis < "u"
    ? globalThis
    : typeof window < "u"
    ? window
    : typeof global < "u"
    ? global
    : typeof self < "u"
    ? self
    : {};
function $l(t) {
  return t && t.__esModule && Object.prototype.hasOwnProperty.call(t, "default")
    ? t.default
    : t;
}
function cp(t) {
  if (t.__esModule) return t;
  var e = t.default;
  if (typeof e == "function") {
    var r = function n() {
      return this instanceof n
        ? Reflect.construct(e, arguments, this.constructor)
        : e.apply(this, arguments);
    };
    r.prototype = e.prototype;
  } else r = {};
  return (
    Object.defineProperty(r, "__esModule", {
      value: !0,
    }),
    Object.keys(t).forEach(function (n) {
      var s = Object.getOwnPropertyDescriptor(t, n);
      Object.defineProperty(
        r,
        n,
        s.get
          ? s
          : {
              enumerable: !0,
              get: function () {
                return t[n];
              },
            }
      );
    }),
    r
  );
}
/**
 * @vue/shared v3.5.12
 * (c) 2018-present Yuxi (Evan) You and Vue contributors
 * @license MIT
 **/
/*! #__NO_SIDE_EFFECTS__ */
function As(t) {
  const e = Object.create(null);
  for (const r of t.split(",")) e[r] = 1;
  return (r) => r in e;
}
const Y = {},
  Vt = [],
  Ke = () => {},
  Fl = () => !1,
  ln = (t) =>
    t.charCodeAt(0) === 111 &&
    t.charCodeAt(1) === 110 &&
    (t.charCodeAt(2) > 122 || t.charCodeAt(2) < 97),
  Os = (t) => t.startsWith("onUpdate:"),
  ie = Object.assign,
  Ps = (t, e) => {
    const r = t.indexOf(e);
    r > -1 && t.splice(r, 1);
  },
  Ul = Object.prototype.hasOwnProperty,
  G = (t, e) => Ul.call(t, e),
  L = Array.isArray,
  Dt = (t) => Jt(t) === "[object Map]",
  cn = (t) => Jt(t) === "[object Set]",
  pi = (t) => Jt(t) === "[object Date]",
  jl = (t) => Jt(t) === "[object RegExp]",
  $ = (t) => typeof t == "function",
  re = (t) => typeof t == "string",
  $e = (t) => typeof t == "symbol",
  J = (t) => t !== null && typeof t == "object",
  ko = (t) => (J(t) || $(t)) && $(t.then) && $(t.catch),
  Vo = Object.prototype.toString,
  Jt = (t) => Vo.call(t),
  Hl = (t) => Jt(t).slice(8, -1),
  Do = (t) => Jt(t) === "[object Object]",
  Ms = (t) =>
    re(t) && t !== "NaN" && t[0] !== "-" && "" + parseInt(t, 10) === t,
  ir = As(
    ",key,ref,ref_for,ref_key,onVnodeBeforeMount,onVnodeMounted,onVnodeBeforeUpdate,onVnodeUpdated,onVnodeBeforeUnmount,onVnodeUnmounted"
  ),
  un = (t) => {
    const e = Object.create(null);
    return (r) => e[r] || (e[r] = t(r));
  },
  Bl = /-(\w)/g,
  ke = un((t) => t.replace(Bl, (e, r) => (r ? r.toUpperCase() : ""))),
  ql = /\B([A-Z])/g,
  ht = un((t) => t.replace(ql, "-$1").toLowerCase()),
  fn = un((t) => t.charAt(0).toUpperCase() + t.slice(1)),
  $r = un((t) => (t ? "on".concat(fn(t)) : "")),
  ut = (t, e) => !Object.is(t, e),
  $t = (t, ...e) => {
    for (let r = 0; r < t.length; r++) t[r](...e);
  },
  $o = (t, e, r, n = !1) => {
    Object.defineProperty(t, e, {
      configurable: !0,
      enumerable: !1,
      writable: n,
      value: r,
    });
  },
  ls = (t) => {
    const e = parseFloat(t);
    return isNaN(e) ? t : e;
  },
  Kl = (t) => {
    const e = re(t) ? Number(t) : NaN;
    return isNaN(e) ? t : e;
  };
let gi;
const dn = () =>
  gi ||
  (gi =
    typeof globalThis < "u"
      ? globalThis
      : typeof self < "u"
      ? self
      : typeof window < "u"
      ? window
      : typeof global < "u"
      ? global
      : {});
function Ls(t) {
  if (L(t)) {
    const e = {};
    for (let r = 0; r < t.length; r++) {
      const n = t[r],
        s = re(n) ? Jl(n) : Ls(n);
      if (s) for (const i in s) e[i] = s[i];
    }
    return e;
  } else if (re(t) || J(t)) return t;
}
const Wl = /;(?![^(]*\))/g,
  Gl = /:([^]+)/,
  zl = /\/\*[^]*?\*\//g;
function Jl(t) {
  const e = {};
  return (
    t
      .replace(zl, "")
      .split(Wl)
      .forEach((r) => {
        if (r) {
          const n = r.split(Gl);
          n.length > 1 && (e[n[0].trim()] = n[1].trim());
        }
      }),
    e
  );
}
function Ns(t) {
  let e = "";
  if (re(t)) e = t;
  else if (L(t))
    for (let r = 0; r < t.length; r++) {
      const n = Ns(t[r]);
      n && (e += n + " ");
    }
  else if (J(t)) for (const r in t) t[r] && (e += r + " ");
  return e.trim();
}
const Yl =
    "itemscope,allowfullscreen,formnovalidate,ismap,nomodule,novalidate,readonly",
  Xl = As(Yl);
function Fo(t) {
  return !!t || t === "";
}
function Ql(t, e) {
  if (t.length !== e.length) return !1;
  let r = !0;
  for (let n = 0; r && n < t.length; n++) r = jt(t[n], e[n]);
  return r;
}
function jt(t, e) {
  if (t === e) return !0;
  let r = pi(t),
    n = pi(e);
  if (r || n) return r && n ? t.getTime() === e.getTime() : !1;
  if (((r = $e(t)), (n = $e(e)), r || n)) return t === e;
  if (((r = L(t)), (n = L(e)), r || n)) return r && n ? Ql(t, e) : !1;
  if (((r = J(t)), (n = J(e)), r || n)) {
    if (!r || !n) return !1;
    const s = Object.keys(t).length,
      i = Object.keys(e).length;
    if (s !== i) return !1;
    for (const o in t) {
      const a = t.hasOwnProperty(o),
        l = e.hasOwnProperty(o);
      if ((a && !l) || (!a && l) || !jt(t[o], e[o])) return !1;
    }
  }
  return String(t) === String(e);
}
function Uo(t, e) {
  return t.findIndex((r) => jt(r, e));
}
const jo = (t) => !!(t && t.__v_isRef === !0),
  Zl = (t) =>
    re(t)
      ? t
      : t == null
      ? ""
      : L(t) || (J(t) && (t.toString === Vo || !$(t.toString)))
      ? jo(t)
        ? Zl(t.value)
        : JSON.stringify(t, Ho, 2)
      : String(t),
  Ho = (t, e) =>
    jo(e)
      ? Ho(t, e.value)
      : Dt(e)
      ? {
          ["Map(".concat(e.size, ")")]: [...e.entries()].reduce(
            (r, [n, s], i) => ((r[Pn(n, i) + " =>"] = s), r),
            {}
          ),
        }
      : cn(e)
      ? {
          ["Set(".concat(e.size, ")")]: [...e.values()].map((r) => Pn(r)),
        }
      : $e(e)
      ? Pn(e)
      : J(e) && !L(e) && !Do(e)
      ? String(e)
      : e,
  Pn = (t, e = "") => {
    var r;
    return $e(t)
      ? "Symbol(".concat((r = t.description) != null ? r : e, ")")
      : t;
  };
/**
 * @vue/reactivity v3.5.12
 * (c) 2018-present Yuxi (Evan) You and Vue contributors
 * @license MIT
 **/
let we;
class Bo {
  constructor(e = !1) {
    (this.detached = e),
      (this._active = !0),
      (this.effects = []),
      (this.cleanups = []),
      (this._isPaused = !1),
      (this.parent = we),
      !e && we && (this.index = (we.scopes || (we.scopes = [])).push(this) - 1);
  }
  get active() {
    return this._active;
  }
  pause() {
    if (this._active) {
      this._isPaused = !0;
      let e, r;
      if (this.scopes)
        for (e = 0, r = this.scopes.length; e < r; e++) this.scopes[e].pause();
      for (e = 0, r = this.effects.length; e < r; e++) this.effects[e].pause();
    }
  }
  resume() {
    if (this._active && this._isPaused) {
      this._isPaused = !1;
      let e, r;
      if (this.scopes)
        for (e = 0, r = this.scopes.length; e < r; e++) this.scopes[e].resume();
      for (e = 0, r = this.effects.length; e < r; e++) this.effects[e].resume();
    }
  }
  run(e) {
    if (this._active) {
      const r = we;
      try {
        return (we = this), e();
      } finally {
        we = r;
      }
    }
  }
  on() {
    we = this;
  }
  off() {
    we = this.parent;
  }
  stop(e) {
    if (this._active) {
      let r, n;
      for (r = 0, n = this.effects.length; r < n; r++) this.effects[r].stop();
      for (r = 0, n = this.cleanups.length; r < n; r++) this.cleanups[r]();
      if (this.scopes)
        for (r = 0, n = this.scopes.length; r < n; r++) this.scopes[r].stop(!0);
      if (!this.detached && this.parent && !e) {
        const s = this.parent.scopes.pop();
        s &&
          s !== this &&
          ((this.parent.scopes[this.index] = s), (s.index = this.index));
      }
      (this.parent = void 0), (this._active = !1);
    }
  }
}
function up(t) {
  return new Bo(t);
}
function ec() {
  return we;
}
function fp(t, e = !1) {
  we && we.cleanups.push(t);
}
let Z;
const Mn = new WeakSet();
class qo {
  constructor(e) {
    (this.fn = e),
      (this.deps = void 0),
      (this.depsTail = void 0),
      (this.flags = 5),
      (this.next = void 0),
      (this.cleanup = void 0),
      (this.scheduler = void 0),
      we && we.active && we.effects.push(this);
  }
  pause() {
    this.flags |= 64;
  }
  resume() {
    this.flags & 64 &&
      ((this.flags &= -65), Mn.has(this) && (Mn.delete(this), this.trigger()));
  }
  notify() {
    (this.flags & 2 && !(this.flags & 32)) || this.flags & 8 || Wo(this);
  }
  run() {
    if (!(this.flags & 1)) return this.fn();
    (this.flags |= 2), mi(this), Go(this);
    const e = Z,
      r = De;
    (Z = this), (De = !0);
    try {
      return this.fn();
    } finally {
      zo(this), (Z = e), (De = r), (this.flags &= -3);
    }
  }
  stop() {
    if (this.flags & 1) {
      for (let e = this.deps; e; e = e.nextDep) Ds(e);
      (this.deps = this.depsTail = void 0),
        mi(this),
        this.onStop && this.onStop(),
        (this.flags &= -2);
    }
  }
  trigger() {
    this.flags & 64
      ? Mn.add(this)
      : this.scheduler
      ? this.scheduler()
      : this.runIfDirty();
  }
  runIfDirty() {
    cs(this) && this.run();
  }
  get dirty() {
    return cs(this);
  }
}
let Ko = 0,
  or,
  ar;
function Wo(t, e = !1) {
  if (((t.flags |= 8), e)) {
    (t.next = ar), (ar = t);
    return;
  }
  (t.next = or), (or = t);
}
function ks() {
  Ko++;
}
function Vs() {
  if (--Ko > 0) return;
  if (ar) {
    let e = ar;
    for (ar = void 0; e; ) {
      const r = e.next;
      (e.next = void 0), (e.flags &= -9), (e = r);
    }
  }
  let t;
  for (; or; ) {
    let e = or;
    for (or = void 0; e; ) {
      const r = e.next;
      if (((e.next = void 0), (e.flags &= -9), e.flags & 1))
        try {
          e.trigger();
        } catch (n) {
          t || (t = n);
        }
      e = r;
    }
  }
  if (t) throw t;
}
function Go(t) {
  for (let e = t.deps; e; e = e.nextDep)
    (e.version = -1),
      (e.prevActiveLink = e.dep.activeLink),
      (e.dep.activeLink = e);
}
function zo(t) {
  let e,
    r = t.depsTail,
    n = r;
  for (; n; ) {
    const s = n.prevDep;
    n.version === -1 ? (n === r && (r = s), Ds(n), tc(n)) : (e = n),
      (n.dep.activeLink = n.prevActiveLink),
      (n.prevActiveLink = void 0),
      (n = s);
  }
  (t.deps = e), (t.depsTail = r);
}
function cs(t) {
  for (let e = t.deps; e; e = e.nextDep)
    if (
      e.dep.version !== e.version ||
      (e.dep.computed && (Jo(e.dep.computed) || e.dep.version !== e.version))
    )
      return !0;
  return !!t._dirty;
}
function Jo(t) {
  if (
    (t.flags & 4 && !(t.flags & 16)) ||
    ((t.flags &= -17), t.globalVersion === pr)
  )
    return;
  t.globalVersion = pr;
  const e = t.dep;
  if (((t.flags |= 2), e.version > 0 && !t.isSSR && t.deps && !cs(t))) {
    t.flags &= -3;
    return;
  }
  const r = Z,
    n = De;
  (Z = t), (De = !0);
  try {
    Go(t);
    const s = t.fn(t._value);
    (e.version === 0 || ut(s, t._value)) && ((t._value = s), e.version++);
  } catch (s) {
    throw (e.version++, s);
  } finally {
    (Z = r), (De = n), zo(t), (t.flags &= -3);
  }
}
function Ds(t, e = !1) {
  const { dep: r, prevSub: n, nextSub: s } = t;
  if (
    (n && ((n.nextSub = s), (t.prevSub = void 0)),
    s && ((s.prevSub = n), (t.nextSub = void 0)),
    r.subs === t && ((r.subs = n), !n && r.computed))
  ) {
    r.computed.flags &= -5;
    for (let i = r.computed.deps; i; i = i.nextDep) Ds(i, !0);
  }
  !e && !--r.sc && r.map && r.map.delete(r.key);
}
function tc(t) {
  const { prevDep: e, nextDep: r } = t;
  e && ((e.nextDep = r), (t.prevDep = void 0)),
    r && ((r.prevDep = e), (t.nextDep = void 0));
}
let De = !0;
const Yo = [];
function pt() {
  Yo.push(De), (De = !1);
}
function gt() {
  const t = Yo.pop();
  De = t === void 0 ? !0 : t;
}
function mi(t) {
  const { cleanup: e } = t;
  if (((t.cleanup = void 0), e)) {
    const r = Z;
    Z = void 0;
    try {
      e();
    } finally {
      Z = r;
    }
  }
}
let pr = 0;
class rc {
  constructor(e, r) {
    (this.sub = e),
      (this.dep = r),
      (this.version = r.version),
      (this.nextDep =
        this.prevDep =
        this.nextSub =
        this.prevSub =
        this.prevActiveLink =
          void 0);
  }
}
class hn {
  constructor(e) {
    (this.computed = e),
      (this.version = 0),
      (this.activeLink = void 0),
      (this.subs = void 0),
      (this.map = void 0),
      (this.key = void 0),
      (this.sc = 0);
  }
  track(e) {
    if (!Z || !De || Z === this.computed) return;
    let r = this.activeLink;
    if (r === void 0 || r.sub !== Z)
      (r = this.activeLink = new rc(Z, this)),
        Z.deps
          ? ((r.prevDep = Z.depsTail),
            (Z.depsTail.nextDep = r),
            (Z.depsTail = r))
          : (Z.deps = Z.depsTail = r),
        Xo(r);
    else if (r.version === -1 && ((r.version = this.version), r.nextDep)) {
      const n = r.nextDep;
      (n.prevDep = r.prevDep),
        r.prevDep && (r.prevDep.nextDep = n),
        (r.prevDep = Z.depsTail),
        (r.nextDep = void 0),
        (Z.depsTail.nextDep = r),
        (Z.depsTail = r),
        Z.deps === r && (Z.deps = n);
    }
    return r;
  }
  trigger(e) {
    this.version++, pr++, this.notify(e);
  }
  notify(e) {
    ks();
    try {
      for (let r = this.subs; r; r = r.prevSub)
        r.sub.notify() && r.sub.dep.notify();
    } finally {
      Vs();
    }
  }
}
function Xo(t) {
  if ((t.dep.sc++, t.sub.flags & 4)) {
    const e = t.dep.computed;
    if (e && !t.dep.subs) {
      e.flags |= 20;
      for (let n = e.deps; n; n = n.nextDep) Xo(n);
    }
    const r = t.dep.subs;
    r !== t && ((t.prevSub = r), r && (r.nextSub = t)), (t.dep.subs = t);
  }
}
const Jr = new WeakMap(),
  St = Symbol(""),
  us = Symbol(""),
  gr = Symbol("");
function ve(t, e, r) {
  if (De && Z) {
    let n = Jr.get(t);
    n || Jr.set(t, (n = new Map()));
    let s = n.get(r);
    s || (n.set(r, (s = new hn())), (s.map = n), (s.key = r)), s.track();
  }
}
function Qe(t, e, r, n, s, i) {
  const o = Jr.get(t);
  if (!o) {
    pr++;
    return;
  }
  const a = (l) => {
    l && l.trigger();
  };
  if ((ks(), e === "clear")) o.forEach(a);
  else {
    const l = L(t),
      c = l && Ms(r);
    if (l && r === "length") {
      const u = Number(n);
      o.forEach((d, p) => {
        (p === "length" || p === gr || (!$e(p) && p >= u)) && a(d);
      });
    } else
      switch (
        ((r !== void 0 || o.has(void 0)) && a(o.get(r)), c && a(o.get(gr)), e)
      ) {
        case "add":
          l ? c && a(o.get("length")) : (a(o.get(St)), Dt(t) && a(o.get(us)));
          break;
        case "delete":
          l || (a(o.get(St)), Dt(t) && a(o.get(us)));
          break;
        case "set":
          Dt(t) && a(o.get(St));
          break;
      }
  }
  Vs();
}
function nc(t, e) {
  const r = Jr.get(t);
  return r && r.get(e);
}
function Mt(t) {
  const e = K(t);
  return e === t ? e : (ve(e, "iterate", gr), Ne(t) ? e : e.map(xe));
}
function pn(t) {
  return ve((t = K(t)), "iterate", gr), t;
}
const sc = {
  __proto__: null,
  [Symbol.iterator]() {
    return Ln(this, Symbol.iterator, xe);
  },
  concat(...t) {
    return Mt(this).concat(...t.map((e) => (L(e) ? Mt(e) : e)));
  },
  entries() {
    return Ln(this, "entries", (t) => ((t[1] = xe(t[1])), t));
  },
  every(t, e) {
    return ze(this, "every", t, e, void 0, arguments);
  },
  filter(t, e) {
    return ze(this, "filter", t, e, (r) => r.map(xe), arguments);
  },
  find(t, e) {
    return ze(this, "find", t, e, xe, arguments);
  },
  findIndex(t, e) {
    return ze(this, "findIndex", t, e, void 0, arguments);
  },
  findLast(t, e) {
    return ze(this, "findLast", t, e, xe, arguments);
  },
  findLastIndex(t, e) {
    return ze(this, "findLastIndex", t, e, void 0, arguments);
  },
  forEach(t, e) {
    return ze(this, "forEach", t, e, void 0, arguments);
  },
  includes(...t) {
    return Nn(this, "includes", t);
  },
  indexOf(...t) {
    return Nn(this, "indexOf", t);
  },
  join(t) {
    return Mt(this).join(t);
  },
  lastIndexOf(...t) {
    return Nn(this, "lastIndexOf", t);
  },
  map(t, e) {
    return ze(this, "map", t, e, void 0, arguments);
  },
  pop() {
    return er(this, "pop");
  },
  push(...t) {
    return er(this, "push", t);
  },
  reduce(t, ...e) {
    return vi(this, "reduce", t, e);
  },
  reduceRight(t, ...e) {
    return vi(this, "reduceRight", t, e);
  },
  shift() {
    return er(this, "shift");
  },
  some(t, e) {
    return ze(this, "some", t, e, void 0, arguments);
  },
  splice(...t) {
    return er(this, "splice", t);
  },
  toReversed() {
    return Mt(this).toReversed();
  },
  toSorted(t) {
    return Mt(this).toSorted(t);
  },
  toSpliced(...t) {
    return Mt(this).toSpliced(...t);
  },
  unshift(...t) {
    return er(this, "unshift", t);
  },
  values() {
    return Ln(this, "values", xe);
  },
};
function Ln(t, e, r) {
  const n = pn(t),
    s = n[e]();
  return (
    n !== t &&
      !Ne(t) &&
      ((s._next = s.next),
      (s.next = () => {
        const i = s._next();
        return i.value && (i.value = r(i.value)), i;
      })),
    s
  );
}
const ic = Array.prototype;
function ze(t, e, r, n, s, i) {
  const o = pn(t),
    a = o !== t && !Ne(t),
    l = o[e];
  if (l !== ic[e]) {
    const d = l.apply(t, i);
    return a ? xe(d) : d;
  }
  let c = r;
  o !== t &&
    (a
      ? (c = function (d, p) {
          return r.call(this, xe(d), p, t);
        })
      : r.length > 2 &&
        (c = function (d, p) {
          return r.call(this, d, p, t);
        }));
  const u = l.call(o, c, n);
  return a && s ? s(u) : u;
}
function vi(t, e, r, n) {
  const s = pn(t);
  let i = r;
  return (
    s !== t &&
      (Ne(t)
        ? r.length > 3 &&
          (i = function (o, a, l) {
            return r.call(this, o, a, l, t);
          })
        : (i = function (o, a, l) {
            return r.call(this, o, xe(a), l, t);
          })),
    s[e](i, ...n)
  );
}
function Nn(t, e, r) {
  const n = K(t);
  ve(n, "iterate", gr);
  const s = n[e](...r);
  return (s === -1 || s === !1) && Us(r[0])
    ? ((r[0] = K(r[0])), n[e](...r))
    : s;
}
function er(t, e, r = []) {
  pt(), ks();
  const n = K(t)[e].apply(t, r);
  return Vs(), gt(), n;
}
const oc = As("__proto__,__v_isRef,__isVue"),
  Qo = new Set(
    Object.getOwnPropertyNames(Symbol)
      .filter((t) => t !== "arguments" && t !== "caller")
      .map((t) => Symbol[t])
      .filter($e)
  );
function ac(t) {
  $e(t) || (t = String(t));
  const e = K(this);
  return ve(e, "has", t), e.hasOwnProperty(t);
}
class Zo {
  constructor(e = !1, r = !1) {
    (this._isReadonly = e), (this._isShallow = r);
  }
  get(e, r, n) {
    const s = this._isReadonly,
      i = this._isShallow;
    if (r === "__v_isReactive") return !s;
    if (r === "__v_isReadonly") return s;
    if (r === "__v_isShallow") return i;
    if (r === "__v_raw")
      return n === (s ? (i ? vc : na) : i ? ra : ta).get(e) ||
        Object.getPrototypeOf(e) === Object.getPrototypeOf(n)
        ? e
        : void 0;
    const o = L(e);
    if (!s) {
      let l;
      if (o && (l = sc[r])) return l;
      if (r === "hasOwnProperty") return ac;
    }
    const a = Reflect.get(e, r, fe(e) ? e : n);
    return ($e(r) ? Qo.has(r) : oc(r)) || (s || ve(e, "get", r), i)
      ? a
      : fe(a)
      ? o && Ms(r)
        ? a
        : a.value
      : J(a)
      ? s
        ? sa(a)
        : gn(a)
      : a;
  }
}
class ea extends Zo {
  constructor(e = !1) {
    super(!1, e);
  }
  set(e, r, n, s) {
    let i = e[r];
    if (!this._isShallow) {
      const l = Rt(i);
      if (
        (!Ne(n) && !Rt(n) && ((i = K(i)), (n = K(n))), !L(e) && fe(i) && !fe(n))
      )
        return l ? !1 : ((i.value = n), !0);
    }
    const o = L(e) && Ms(r) ? Number(r) < e.length : G(e, r),
      a = Reflect.set(e, r, n, fe(e) ? e : s);
    return (
      e === K(s) && (o ? ut(n, i) && Qe(e, "set", r, n) : Qe(e, "add", r, n)), a
    );
  }
  deleteProperty(e, r) {
    const n = G(e, r);
    e[r];
    const s = Reflect.deleteProperty(e, r);
    return s && n && Qe(e, "delete", r, void 0), s;
  }
  has(e, r) {
    const n = Reflect.has(e, r);
    return (!$e(r) || !Qo.has(r)) && ve(e, "has", r), n;
  }
  ownKeys(e) {
    return ve(e, "iterate", L(e) ? "length" : St), Reflect.ownKeys(e);
  }
}
class lc extends Zo {
  constructor(e = !1) {
    super(!0, e);
  }
  set(e, r) {
    return !0;
  }
  deleteProperty(e, r) {
    return !0;
  }
}
const cc = new ea(),
  uc = new lc(),
  fc = new ea(!0);
const fs = (t) => t,
  Pr = (t) => Reflect.getPrototypeOf(t);
function dc(t, e, r) {
  return function (...n) {
    const s = this.__v_raw,
      i = K(s),
      o = Dt(i),
      a = t === "entries" || (t === Symbol.iterator && o),
      l = t === "keys" && o,
      c = s[t](...n),
      u = r ? fs : e ? ds : xe;
    return (
      !e && ve(i, "iterate", l ? us : St),
      {
        next() {
          const { value: d, done: p } = c.next();
          return p
            ? {
                value: d,
                done: p,
              }
            : {
                value: a ? [u(d[0]), u(d[1])] : u(d),
                done: p,
              };
        },
        [Symbol.iterator]() {
          return this;
        },
      }
    );
  };
}
function Mr(t) {
  return function (...e) {
    return t === "delete" ? !1 : t === "clear" ? void 0 : this;
  };
}
function hc(t, e) {
  const r = {
    get(s) {
      const i = this.__v_raw,
        o = K(i),
        a = K(s);
      t || (ut(s, a) && ve(o, "get", s), ve(o, "get", a));
      const { has: l } = Pr(o),
        c = e ? fs : t ? ds : xe;
      if (l.call(o, s)) return c(i.get(s));
      if (l.call(o, a)) return c(i.get(a));
      i !== o && i.get(s);
    },
    get size() {
      const s = this.__v_raw;
      return !t && ve(K(s), "iterate", St), Reflect.get(s, "size", s);
    },
    has(s) {
      const i = this.__v_raw,
        o = K(i),
        a = K(s);
      return (
        t || (ut(s, a) && ve(o, "has", s), ve(o, "has", a)),
        s === a ? i.has(s) : i.has(s) || i.has(a)
      );
    },
    forEach(s, i) {
      const o = this,
        a = o.__v_raw,
        l = K(a),
        c = e ? fs : t ? ds : xe;
      return (
        !t && ve(l, "iterate", St),
        a.forEach((u, d) => s.call(i, c(u), c(d), o))
      );
    },
  };
  return (
    ie(
      r,
      t
        ? {
            add: Mr("add"),
            set: Mr("set"),
            delete: Mr("delete"),
            clear: Mr("clear"),
          }
        : {
            add(s) {
              !e && !Ne(s) && !Rt(s) && (s = K(s));
              const i = K(this);
              return (
                Pr(i).has.call(i, s) || (i.add(s), Qe(i, "add", s, s)), this
              );
            },
            set(s, i) {
              !e && !Ne(i) && !Rt(i) && (i = K(i));
              const o = K(this),
                { has: a, get: l } = Pr(o);
              let c = a.call(o, s);
              c || ((s = K(s)), (c = a.call(o, s)));
              const u = l.call(o, s);
              return (
                o.set(s, i),
                c ? ut(i, u) && Qe(o, "set", s, i) : Qe(o, "add", s, i),
                this
              );
            },
            delete(s) {
              const i = K(this),
                { has: o, get: a } = Pr(i);
              let l = o.call(i, s);
              l || ((s = K(s)), (l = o.call(i, s))), a && a.call(i, s);
              const c = i.delete(s);
              return l && Qe(i, "delete", s, void 0), c;
            },
            clear() {
              const s = K(this),
                i = s.size !== 0,
                o = s.clear();
              return i && Qe(s, "clear", void 0, void 0), o;
            },
          }
    ),
    ["keys", "values", "entries", Symbol.iterator].forEach((s) => {
      r[s] = dc(s, t, e);
    }),
    r
  );
}
function $s(t, e) {
  const r = hc(t, e);
  return (n, s, i) =>
    s === "__v_isReactive"
      ? !t
      : s === "__v_isReadonly"
      ? t
      : s === "__v_raw"
      ? n
      : Reflect.get(G(r, s) && s in n ? r : n, s, i);
}
const pc = {
    get: $s(!1, !1),
  },
  gc = {
    get: $s(!1, !0),
  },
  mc = {
    get: $s(!0, !1),
  };
const ta = new WeakMap(),
  ra = new WeakMap(),
  na = new WeakMap(),
  vc = new WeakMap();
function xc(t) {
  switch (t) {
    case "Object":
    case "Array":
      return 1;
    case "Map":
    case "Set":
    case "WeakMap":
    case "WeakSet":
      return 2;
    default:
      return 0;
  }
}
function Ec(t) {
  return t.__v_skip || !Object.isExtensible(t) ? 0 : xc(Hl(t));
}
function gn(t) {
  return Rt(t) ? t : Fs(t, !1, cc, pc, ta);
}
function yc(t) {
  return Fs(t, !1, fc, gc, ra);
}
function sa(t) {
  return Fs(t, !0, uc, mc, na);
}
function Fs(t, e, r, n, s) {
  if (!J(t) || (t.__v_raw && !(e && t.__v_isReactive))) return t;
  const i = s.get(t);
  if (i) return i;
  const o = Ec(t);
  if (o === 0) return t;
  const a = new Proxy(t, o === 2 ? n : r);
  return s.set(t, a), a;
}
function Ft(t) {
  return Rt(t) ? Ft(t.__v_raw) : !!(t && t.__v_isReactive);
}
function Rt(t) {
  return !!(t && t.__v_isReadonly);
}
function Ne(t) {
  return !!(t && t.__v_isShallow);
}
function Us(t) {
  return t ? !!t.__v_raw : !1;
}
function K(t) {
  const e = t && t.__v_raw;
  return e ? K(e) : t;
}
function bc(t) {
  return (
    !G(t, "__v_skip") && Object.isExtensible(t) && $o(t, "__v_skip", !0), t
  );
}
const xe = (t) => (J(t) ? gn(t) : t),
  ds = (t) => (J(t) ? sa(t) : t);
function fe(t) {
  return t ? t.__v_isRef === !0 : !1;
}
function Fr(t) {
  return ia(t, !1);
}
function dp(t) {
  return ia(t, !0);
}
function ia(t, e) {
  return fe(t) ? t : new wc(t, e);
}
class wc {
  constructor(e, r) {
    (this.dep = new hn()),
      (this.__v_isRef = !0),
      (this.__v_isShallow = !1),
      (this._rawValue = r ? e : K(e)),
      (this._value = r ? e : xe(e)),
      (this.__v_isShallow = r);
  }
  get value() {
    return this.dep.track(), this._value;
  }
  set value(e) {
    const r = this._rawValue,
      n = this.__v_isShallow || Ne(e) || Rt(e);
    (e = n ? e : K(e)),
      ut(e, r) &&
        ((this._rawValue = e),
        (this._value = n ? e : xe(e)),
        this.dep.trigger());
  }
}
function oa(t) {
  return fe(t) ? t.value : t;
}
function hp(t) {
  return $(t) ? t() : oa(t);
}
const _c = {
  get: (t, e, r) => (e === "__v_raw" ? t : oa(Reflect.get(t, e, r))),
  set: (t, e, r, n) => {
    const s = t[e];
    return fe(s) && !fe(r) ? ((s.value = r), !0) : Reflect.set(t, e, r, n);
  },
};
function aa(t) {
  return Ft(t) ? t : new Proxy(t, _c);
}
class Sc {
  constructor(e) {
    (this.__v_isRef = !0), (this._value = void 0);
    const r = (this.dep = new hn()),
      { get: n, set: s } = e(r.track.bind(r), r.trigger.bind(r));
    (this._get = n), (this._set = s);
  }
  get value() {
    return (this._value = this._get());
  }
  set value(e) {
    this._set(e);
  }
}
function pp(t) {
  return new Sc(t);
}
function gp(t) {
  const e = L(t) ? new Array(t.length) : {};
  for (const r in t) e[r] = la(t, r);
  return e;
}
class Ic {
  constructor(e, r, n) {
    (this._object = e),
      (this._key = r),
      (this._defaultValue = n),
      (this.__v_isRef = !0),
      (this._value = void 0);
  }
  get value() {
    const e = this._object[this._key];
    return (this._value = e === void 0 ? this._defaultValue : e);
  }
  set value(e) {
    this._object[this._key] = e;
  }
  get dep() {
    return nc(K(this._object), this._key);
  }
}
class Tc {
  constructor(e) {
    (this._getter = e),
      (this.__v_isRef = !0),
      (this.__v_isReadonly = !0),
      (this._value = void 0);
  }
  get value() {
    return (this._value = this._getter());
  }
}
function mp(t, e, r) {
  return fe(t)
    ? t
    : $(t)
    ? new Tc(t)
    : J(t) && arguments.length > 1
    ? la(t, e, r)
    : Fr(t);
}
function la(t, e, r) {
  const n = t[e];
  return fe(n) ? n : new Ic(t, e, r);
}
class Cc {
  constructor(e, r, n) {
    (this.fn = e),
      (this.setter = r),
      (this._value = void 0),
      (this.dep = new hn(this)),
      (this.__v_isRef = !0),
      (this.deps = void 0),
      (this.depsTail = void 0),
      (this.flags = 16),
      (this.globalVersion = pr - 1),
      (this.next = void 0),
      (this.effect = this),
      (this.__v_isReadonly = !r),
      (this.isSSR = n);
  }
  notify() {
    if (((this.flags |= 16), !(this.flags & 8) && Z !== this))
      return Wo(this, !0), !0;
  }
  get value() {
    const e = this.dep.track();
    return Jo(this), e && (e.version = this.dep.version), this._value;
  }
  set value(e) {
    this.setter && this.setter(e);
  }
}
function Rc(t, e, r = !1) {
  let n, s;
  return $(t) ? (n = t) : ((n = t.get), (s = t.set)), new Cc(n, s, r);
}
const Lr = {},
  Yr = new WeakMap();
let _t;
function Ac(t, e = !1, r = _t) {
  if (r) {
    let n = Yr.get(r);
    n || Yr.set(r, (n = [])), n.push(t);
  }
}
function Oc(t, e, r = Y) {
  const {
      immediate: n,
      deep: s,
      once: i,
      scheduler: o,
      augmentJob: a,
      call: l,
    } = r,
    c = (y) => (s ? y : Ne(y) || s === !1 || s === 0 ? Ze(y, 1) : Ze(y));
  let u,
    d,
    p,
    g,
    v = !1,
    _ = !1;
  if (
    (fe(t)
      ? ((d = () => t.value), (v = Ne(t)))
      : Ft(t)
      ? ((d = () => c(t)), (v = !0))
      : L(t)
      ? ((_ = !0),
        (v = t.some((y) => Ft(y) || Ne(y))),
        (d = () =>
          t.map((y) => {
            if (fe(y)) return y.value;
            if (Ft(y)) return c(y);
            if ($(y)) return l ? l(y, 2) : y();
          })))
      : $(t)
      ? e
        ? (d = l ? () => l(t, 2) : t)
        : (d = () => {
            if (p) {
              pt();
              try {
                p();
              } finally {
                gt();
              }
            }
            const y = _t;
            _t = u;
            try {
              return l ? l(t, 3, [g]) : t(g);
            } finally {
              _t = y;
            }
          })
      : (d = Ke),
    e && s)
  ) {
    const y = d,
      D = s === !0 ? 1 / 0 : s;
    d = () => Ze(y(), D);
  }
  const T = ec(),
    N = () => {
      u.stop(), T && Ps(T.effects, u);
    };
  if (i && e) {
    const y = e;
    e = (...D) => {
      y(...D), N();
    };
  }
  let R = _ ? new Array(t.length).fill(Lr) : Lr;
  const S = (y) => {
    if (!(!(u.flags & 1) || (!u.dirty && !y)))
      if (e) {
        const D = u.run();
        if (s || v || (_ ? D.some((U, j) => ut(U, R[j])) : ut(D, R))) {
          p && p();
          const U = _t;
          _t = u;
          try {
            const j = [D, R === Lr ? void 0 : _ && R[0] === Lr ? [] : R, g];
            l ? l(e, 3, j) : e(...j), (R = D);
          } finally {
            _t = U;
          }
        }
      } else u.run();
  };
  return (
    a && a(S),
    (u = new qo(d)),
    (u.scheduler = o ? () => o(S, !1) : S),
    (g = (y) => Ac(y, !1, u)),
    (p = u.onStop =
      () => {
        const y = Yr.get(u);
        if (y) {
          if (l) l(y, 4);
          else for (const D of y) D();
          Yr.delete(u);
        }
      }),
    e ? (n ? S(!0) : (R = u.run())) : o ? o(S.bind(null, !0), !0) : u.run(),
    (N.pause = u.pause.bind(u)),
    (N.resume = u.resume.bind(u)),
    (N.stop = N),
    N
  );
}
function Ze(t, e = 1 / 0, r) {
  if (e <= 0 || !J(t) || t.__v_skip || ((r = r || new Set()), r.has(t)))
    return t;
  if ((r.add(t), e--, fe(t))) Ze(t.value, e, r);
  else if (L(t)) for (let n = 0; n < t.length; n++) Ze(t[n], e, r);
  else if (cn(t) || Dt(t))
    t.forEach((n) => {
      Ze(n, e, r);
    });
  else if (Do(t)) {
    for (const n in t) Ze(t[n], e, r);
    for (const n of Object.getOwnPropertySymbols(t))
      Object.prototype.propertyIsEnumerable.call(t, n) && Ze(t[n], e, r);
  }
  return t;
}
/**
 * @vue/runtime-core v3.5.12
 * (c) 2018-present Yuxi (Evan) You and Vue contributors
 * @license MIT
 **/
function br(t, e, r, n) {
  try {
    return n ? t(...n) : t();
  } catch (s) {
    wr(s, e, r);
  }
}
function Fe(t, e, r, n) {
  if ($(t)) {
    const s = br(t, e, r, n);
    return (
      s &&
        ko(s) &&
        s.catch((i) => {
          wr(i, e, r);
        }),
      s
    );
  }
  if (L(t)) {
    const s = [];
    for (let i = 0; i < t.length; i++) s.push(Fe(t[i], e, r, n));
    return s;
  }
}
function wr(t, e, r, n = !0) {
  const s = e ? e.vnode : null,
    { errorHandler: i, throwUnhandledErrorInProduction: o } =
      (e && e.appContext.config) || Y;
  if (e) {
    let a = e.parent;
    const l = e.proxy,
      c = "https://vuejs.org/error-reference/#runtime-".concat(r);
    for (; a; ) {
      const u = a.ec;
      if (u) {
        for (let d = 0; d < u.length; d++) if (u[d](t, l, c) === !1) return;
      }
      a = a.parent;
    }
    if (i) {
      pt(), br(i, null, 10, [t, l, c]), gt();
      return;
    }
  }
  Pc(t, r, s, n, o);
}
function Pc(t, e, r, n = !0, s = !1) {
  if (s) throw t;
}
const _e = [];
let Be = -1;
const Ut = [];
let it = null,
  kt = 0;
const ca = Promise.resolve();
let Xr = null;
function Mc(t) {
  const e = Xr || ca;
  return t ? e.then(this ? t.bind(this) : t) : e;
}
function Lc(t) {
  let e = Be + 1,
    r = _e.length;
  for (; e < r; ) {
    const n = (e + r) >>> 1,
      s = _e[n],
      i = mr(s);
    i < t || (i === t && s.flags & 2) ? (e = n + 1) : (r = n);
  }
  return e;
}
function js(t) {
  if (!(t.flags & 1)) {
    const e = mr(t),
      r = _e[_e.length - 1];
    !r || (!(t.flags & 2) && e >= mr(r)) ? _e.push(t) : _e.splice(Lc(e), 0, t),
      (t.flags |= 1),
      ua();
  }
}
function ua() {
  Xr || (Xr = ca.then(da));
}
function Nc(t) {
  L(t)
    ? Ut.push(...t)
    : it && t.id === -1
    ? it.splice(kt + 1, 0, t)
    : t.flags & 1 || (Ut.push(t), (t.flags |= 1)),
    ua();
}
function xi(t, e, r = Be + 1) {
  for (; r < _e.length; r++) {
    const n = _e[r];
    if (n && n.flags & 2) {
      if (t && n.id !== t.uid) continue;
      _e.splice(r, 1),
        r--,
        n.flags & 4 && (n.flags &= -2),
        n(),
        n.flags & 4 || (n.flags &= -2);
    }
  }
}
function fa(t) {
  if (Ut.length) {
    const e = [...new Set(Ut)].sort((r, n) => mr(r) - mr(n));
    if (((Ut.length = 0), it)) {
      it.push(...e);
      return;
    }
    for (it = e, kt = 0; kt < it.length; kt++) {
      const r = it[kt];
      r.flags & 4 && (r.flags &= -2), r.flags & 8 || r(), (r.flags &= -2);
    }
    (it = null), (kt = 0);
  }
}
const mr = (t) => (t.id == null ? (t.flags & 2 ? -1 : 1 / 0) : t.id);
function da(t) {
  try {
    for (Be = 0; Be < _e.length; Be++) {
      const e = _e[Be];
      e &&
        !(e.flags & 8) &&
        (e.flags & 4 && (e.flags &= -2),
        br(e, e.i, e.i ? 15 : 14),
        e.flags & 4 || (e.flags &= -2));
    }
  } finally {
    for (; Be < _e.length; Be++) {
      const e = _e[Be];
      e && (e.flags &= -2);
    }
    (Be = -1),
      (_e.length = 0),
      fa(),
      (Xr = null),
      (_e.length || Ut.length) && da();
  }
}
let le = null,
  ha = null;
function Qr(t) {
  const e = le;
  return (le = t), (ha = (t && t.type.__scopeId) || null), e;
}
function kc(t, e = le, r) {
  if (!e || t._n) return t;
  const n = (...s) => {
    n._d && Oi(-1);
    const i = Qr(e);
    let o;
    try {
      o = t(...s);
    } finally {
      Qr(i), n._d && Oi(1);
    }
    return o;
  };
  return (n._n = !0), (n._c = !0), (n._d = !0), n;
}
function vp(t, e) {
  if (le === null) return t;
  const r = bn(le),
    n = t.dirs || (t.dirs = []);
  for (let s = 0; s < e.length; s++) {
    let [i, o, a, l = Y] = e[s];
    i &&
      ($(i) &&
        (i = {
          mounted: i,
          updated: i,
        }),
      i.deep && Ze(o),
      n.push({
        dir: i,
        instance: r,
        value: o,
        oldValue: void 0,
        arg: a,
        modifiers: l,
      }));
  }
  return t;
}
function yt(t, e, r, n) {
  const s = t.dirs,
    i = e && e.dirs;
  for (let o = 0; o < s.length; o++) {
    const a = s[o];
    i && (a.oldValue = i[o].value);
    let l = a.dir[n];
    l && (pt(), Fe(l, r, 8, [t.el, a, t, e]), gt());
  }
}
const pa = Symbol("_vte"),
  ga = (t) => t.__isTeleport,
  lr = (t) => t && (t.disabled || t.disabled === ""),
  Vc = (t) => t && (t.defer || t.defer === ""),
  Ei = (t) => typeof SVGElement < "u" && t instanceof SVGElement,
  yi = (t) => typeof MathMLElement == "function" && t instanceof MathMLElement,
  hs = (t, e) => {
    const r = t && t.to;
    return re(r) ? (e ? e(r) : null) : r;
  },
  Dc = {
    name: "Teleport",
    __isTeleport: !0,
    process(t, e, r, n, s, i, o, a, l, c) {
      const {
          mc: u,
          pc: d,
          pbc: p,
          o: { insert: g, querySelector: v, createText: _, createComment: T },
        } = c,
        N = lr(e.props);
      let { shapeFlag: R, children: S, dynamicChildren: y } = e;
      if (t == null) {
        const D = (e.el = _("")),
          U = (e.anchor = _(""));
        g(D, r, n), g(U, r, n);
        const j = (k, H) => {
            R & 16 &&
              (s && s.isCE && (s.ce._teleportTarget = k),
              u(S, k, H, s, i, o, a, l));
          },
          B = () => {
            const k = (e.target = hs(e.props, v)),
              H = ma(k, e, _, g);
            k &&
              (o !== "svg" && Ei(k)
                ? (o = "svg")
                : o !== "mathml" && yi(k) && (o = "mathml"),
              N || (j(k, H), Ur(e, !1)));
          };
        N && (j(r, U), Ur(e, !0)), Vc(e.props) ? ue(B, i) : B();
      } else {
        (e.el = t.el), (e.targetStart = t.targetStart);
        const D = (e.anchor = t.anchor),
          U = (e.target = t.target),
          j = (e.targetAnchor = t.targetAnchor),
          B = lr(t.props),
          k = B ? r : U,
          H = B ? D : j;
        if (
          (o === "svg" || Ei(U)
            ? (o = "svg")
            : (o === "mathml" || yi(U)) && (o = "mathml"),
          y
            ? (p(t.dynamicChildren, y, k, s, i, o, a), Xs(t, e, !0))
            : l || d(t, e, k, H, s, i, o, a, !1),
          N)
        )
          B
            ? e.props &&
              t.props &&
              e.props.to !== t.props.to &&
              (e.props.to = t.props.to)
            : Nr(e, r, D, c, 1);
        else if ((e.props && e.props.to) !== (t.props && t.props.to)) {
          const W = (e.target = hs(e.props, v));
          W && Nr(e, W, null, c, 0);
        } else B && Nr(e, U, j, c, 1);
        Ur(e, N);
      }
    },
    remove(t, e, r, { um: n, o: { remove: s } }, i) {
      const {
        shapeFlag: o,
        children: a,
        anchor: l,
        targetStart: c,
        targetAnchor: u,
        target: d,
        props: p,
      } = t;
      if ((d && (s(c), s(u)), i && s(l), o & 16)) {
        const g = i || !lr(p);
        for (let v = 0; v < a.length; v++) {
          const _ = a[v];
          n(_, e, r, g, !!_.dynamicChildren);
        }
      }
    },
    move: Nr,
    hydrate: $c,
  };
function Nr(t, e, r, { o: { insert: n }, m: s }, i = 2) {
  i === 0 && n(t.targetAnchor, e, r);
  const { el: o, anchor: a, shapeFlag: l, children: c, props: u } = t,
    d = i === 2;
  if ((d && n(o, e, r), (!d || lr(u)) && l & 16))
    for (let p = 0; p < c.length; p++) s(c[p], e, r, 2);
  d && n(a, e, r);
}
function $c(
  t,
  e,
  r,
  n,
  s,
  i,
  {
    o: {
      nextSibling: o,
      parentNode: a,
      querySelector: l,
      insert: c,
      createText: u,
    },
  },
  d
) {
  const p = (e.target = hs(e.props, l));
  if (p) {
    const g = lr(e.props),
      v = p._lpa || p.firstChild;
    if (e.shapeFlag & 16)
      if (g)
        (e.anchor = d(o(t), e, a(t), r, n, s, i)),
          (e.targetStart = v),
          (e.targetAnchor = v && o(v));
      else {
        e.anchor = o(t);
        let _ = v;
        for (; _; ) {
          if (_ && _.nodeType === 8) {
            if (_.data === "teleport start anchor") e.targetStart = _;
            else if (_.data === "teleport anchor") {
              (e.targetAnchor = _),
                (p._lpa = e.targetAnchor && o(e.targetAnchor));
              break;
            }
          }
          _ = o(_);
        }
        e.targetAnchor || ma(p, e, u, c), d(v && o(v), e, p, r, n, s, i);
      }
    Ur(e, g);
  }
  return e.anchor && o(e.anchor);
}
const xp = Dc;
function Ur(t, e) {
  const r = t.ctx;
  if (r && r.ut) {
    let n, s;
    for (
      e
        ? ((n = t.el), (s = t.anchor))
        : ((n = t.targetStart), (s = t.targetAnchor));
      n && n !== s;

    )
      n.nodeType === 1 && n.setAttribute("data-v-owner", r.uid),
        (n = n.nextSibling);
    r.ut();
  }
}
function ma(t, e, r, n) {
  const s = (e.targetStart = r("")),
    i = (e.targetAnchor = r(""));
  return (s[pa] = i), t && (n(s, t), n(i, t)), i;
}
const ot = Symbol("_leaveCb"),
  kr = Symbol("_enterCb");
function va() {
  const t = {
    isMounted: !1,
    isLeaving: !1,
    isUnmounting: !1,
    leavingVNodes: new Map(),
  };
  return (
    vn(() => {
      t.isMounted = !0;
    }),
    Ks(() => {
      t.isUnmounting = !0;
    }),
    t
  );
}
const Pe = [Function, Array],
  xa = {
    mode: String,
    appear: Boolean,
    persisted: Boolean,
    onBeforeEnter: Pe,
    onEnter: Pe,
    onAfterEnter: Pe,
    onEnterCancelled: Pe,
    onBeforeLeave: Pe,
    onLeave: Pe,
    onAfterLeave: Pe,
    onLeaveCancelled: Pe,
    onBeforeAppear: Pe,
    onAppear: Pe,
    onAfterAppear: Pe,
    onAppearCancelled: Pe,
  },
  Ea = (t) => {
    const e = t.subTree;
    return e.component ? Ea(e.component) : e;
  },
  Fc = {
    name: "BaseTransition",
    props: xa,
    setup(t, { slots: e }) {
      const r = Sr(),
        n = va();
      return () => {
        const s = e.default && Hs(e.default(), !0);
        if (!s || !s.length) return;
        const i = ya(s),
          o = K(t),
          { mode: a } = o;
        if (n.isLeaving) return kn(i);
        const l = bi(i);
        if (!l) return kn(i);
        let c = vr(l, o, n, r, (p) => (c = p));
        l.type !== Ee && ft(l, c);
        const u = r.subTree,
          d = u && bi(u);
        if (d && d.type !== Ee && !lt(l, d) && Ea(r).type !== Ee) {
          const p = vr(d, o, n, r);
          if ((ft(d, p), a === "out-in" && l.type !== Ee))
            return (
              (n.isLeaving = !0),
              (p.afterLeave = () => {
                (n.isLeaving = !1),
                  r.job.flags & 8 || r.update(),
                  delete p.afterLeave;
              }),
              kn(i)
            );
          a === "in-out" &&
            l.type !== Ee &&
            (p.delayLeave = (g, v, _) => {
              const T = ba(n, d);
              (T[String(d.key)] = d),
                (g[ot] = () => {
                  v(), (g[ot] = void 0), delete c.delayedLeave;
                }),
                (c.delayedLeave = _);
            });
        }
        return i;
      };
    },
  };
function ya(t) {
  let e = t[0];
  if (t.length > 1) {
    for (const r of t)
      if (r.type !== Ee) {
        e = r;
        break;
      }
  }
  return e;
}
const Uc = Fc;
function ba(t, e) {
  const { leavingVNodes: r } = t;
  let n = r.get(e.type);
  return n || ((n = Object.create(null)), r.set(e.type, n)), n;
}
function vr(t, e, r, n, s) {
  const {
      appear: i,
      mode: o,
      persisted: a = !1,
      onBeforeEnter: l,
      onEnter: c,
      onAfterEnter: u,
      onEnterCancelled: d,
      onBeforeLeave: p,
      onLeave: g,
      onAfterLeave: v,
      onLeaveCancelled: _,
      onBeforeAppear: T,
      onAppear: N,
      onAfterAppear: R,
      onAppearCancelled: S,
    } = e,
    y = String(t.key),
    D = ba(r, t),
    U = (k, H) => {
      k && Fe(k, n, 9, H);
    },
    j = (k, H) => {
      const W = H[1];
      U(k, H),
        L(k) ? k.every((P) => P.length <= 1) && W() : k.length <= 1 && W();
    },
    B = {
      mode: o,
      persisted: a,
      beforeEnter(k) {
        let H = l;
        if (!r.isMounted)
          if (i) H = T || l;
          else return;
        k[ot] && k[ot](!0);
        const W = D[y];
        W && lt(t, W) && W.el[ot] && W.el[ot](), U(H, [k]);
      },
      enter(k) {
        let H = c,
          W = u,
          P = d;
        if (!r.isMounted)
          if (i) (H = N || c), (W = R || u), (P = S || d);
          else return;
        let ee = !1;
        const pe = (k[kr] = (vt) => {
          ee ||
            ((ee = !0),
            vt ? U(P, [k]) : U(W, [k]),
            B.delayedLeave && B.delayedLeave(),
            (k[kr] = void 0));
        });
        H ? j(H, [k, pe]) : pe();
      },
      leave(k, H) {
        const W = String(t.key);
        if ((k[kr] && k[kr](!0), r.isUnmounting)) return H();
        U(p, [k]);
        let P = !1;
        const ee = (k[ot] = (pe) => {
          P ||
            ((P = !0),
            H(),
            pe ? U(_, [k]) : U(v, [k]),
            (k[ot] = void 0),
            D[W] === t && delete D[W]);
        });
        (D[W] = t), g ? j(g, [k, ee]) : ee();
      },
      clone(k) {
        const H = vr(k, e, r, n, s);
        return s && s(H), H;
      },
    };
  return B;
}
function kn(t) {
  if (_r(t)) return (t = tt(t)), (t.children = null), t;
}
function bi(t) {
  if (!_r(t)) return ga(t.type) && t.children ? ya(t.children) : t;
  const { shapeFlag: e, children: r } = t;
  if (r) {
    if (e & 16) return r[0];
    if (e & 32 && $(r.default)) return r.default();
  }
}
function ft(t, e) {
  t.shapeFlag & 6 && t.component
    ? ((t.transition = e), ft(t.component.subTree, e))
    : t.shapeFlag & 128
    ? ((t.ssContent.transition = e.clone(t.ssContent)),
      (t.ssFallback.transition = e.clone(t.ssFallback)))
    : (t.transition = e);
}
function Hs(t, e = !1, r) {
  let n = [],
    s = 0;
  for (let i = 0; i < t.length; i++) {
    let o = t[i];
    const a = r == null ? o.key : String(r) + String(o.key != null ? o.key : i);
    o.type === Se
      ? (o.patchFlag & 128 && s++, (n = n.concat(Hs(o.children, e, a))))
      : (e || o.type !== Ee) &&
        n.push(
          a != null
            ? tt(o, {
                key: a,
              })
            : o
        );
  }
  if (s > 1) for (let i = 0; i < n.length; i++) n[i].patchFlag = -2;
  return n;
}
/*! #__NO_SIDE_EFFECTS__ */
function jc(t, e) {
  return $(t)
    ? ie(
        {
          name: t.name,
        },
        e,
        {
          setup: t,
        }
      )
    : t;
}
function Bs(t) {
  t.ids = [t.ids[0] + t.ids[2]++ + "-", 0, 0];
}
function ps(t, e, r, n, s = !1) {
  if (L(t)) {
    t.forEach((v, _) => ps(v, e && (L(e) ? e[_] : e), r, n, s));
    return;
  }
  if (It(n) && !s) return;
  const i = n.shapeFlag & 4 ? bn(n.component) : n.el,
    o = s ? null : i,
    { i: a, r: l } = t,
    c = e && e.r,
    u = a.refs === Y ? (a.refs = {}) : a.refs,
    d = a.setupState,
    p = K(d),
    g = d === Y ? () => !1 : (v) => G(p, v);
  if (
    (c != null &&
      c !== l &&
      (re(c)
        ? ((u[c] = null), g(c) && (d[c] = null))
        : fe(c) && (c.value = null)),
    $(l))
  )
    br(l, a, 12, [o, u]);
  else {
    const v = re(l),
      _ = fe(l);
    if (v || _) {
      const T = () => {
        if (t.f) {
          const N = v ? (g(l) ? d[l] : u[l]) : l.value;
          s
            ? L(N) && Ps(N, i)
            : L(N)
            ? N.includes(i) || N.push(i)
            : v
            ? ((u[l] = [i]), g(l) && (d[l] = u[l]))
            : ((l.value = [i]), t.k && (u[t.k] = l.value));
        } else
          v
            ? ((u[l] = o), g(l) && (d[l] = o))
            : _ && ((l.value = o), t.k && (u[t.k] = o));
      };
      o ? ((T.id = -1), ue(T, r)) : T();
    }
  }
}
const wi = (t) => t.nodeType === 8;
dn().requestIdleCallback;
dn().cancelIdleCallback;
function Hc(t, e) {
  if (wi(t) && t.data === "[") {
    let r = 1,
      n = t.nextSibling;
    for (; n; ) {
      if (n.nodeType === 1) {
        if (e(n) === !1) break;
      } else if (wi(n))
        if (n.data === "]") {
          if (--r === 0) break;
        } else n.data === "[" && r++;
      n = n.nextSibling;
    }
  } else e(t);
}
const It = (t) => !!t.type.__asyncLoader;
/*! #__NO_SIDE_EFFECTS__ */
function Ep(t) {
  $(t) &&
    (t = {
      loader: t,
    });
  const {
    loader: e,
    loadingComponent: r,
    errorComponent: n,
    delay: s = 200,
    hydrate: i,
    timeout: o,
    suspensible: a = !0,
    onError: l,
  } = t;
  let c = null,
    u,
    d = 0;
  const p = () => (d++, (c = null), g()),
    g = () => {
      let v;
      return (
        c ||
        (v = c =
          e()
            .catch((_) => {
              if (((_ = _ instanceof Error ? _ : new Error(String(_))), l))
                return new Promise((T, N) => {
                  l(
                    _,
                    () => T(p()),
                    () => N(_),
                    d + 1
                  );
                });
              throw _;
            })
            .then((_) =>
              v !== c && c
                ? c
                : (_ &&
                    (_.__esModule || _[Symbol.toStringTag] === "Module") &&
                    (_ = _.default),
                  (u = _),
                  _)
            ))
      );
    };
  return jc({
    name: "AsyncComponentWrapper",
    __asyncLoader: g,
    __asyncHydrate(v, _, T) {
      const N = i
        ? () => {
            const R = i(T, (S) => Hc(v, S));
            R && (_.bum || (_.bum = [])).push(R);
          }
        : T;
      u ? N() : g().then(() => !_.isUnmounted && N());
    },
    get __asyncResolved() {
      return u;
    },
    setup() {
      const v = ae;
      if ((Bs(v), u)) return () => Vn(u, v);
      const _ = (S) => {
        (c = null), wr(S, v, 13, !n);
      };
      if ((a && v.suspense) || Bt)
        return g()
          .then((S) => () => Vn(S, v))
          .catch(
            (S) => (
              _(S),
              () =>
                n
                  ? oe(n, {
                      error: S,
                    })
                  : null
            )
          );
      const T = Fr(!1),
        N = Fr(),
        R = Fr(!!s);
      return (
        s &&
          setTimeout(() => {
            R.value = !1;
          }, s),
        o != null &&
          setTimeout(() => {
            if (!T.value && !N.value) {
              const S = new Error(
                "Async component timed out after ".concat(o, "ms.")
              );
              _(S), (N.value = S);
            }
          }, o),
        g()
          .then(() => {
            (T.value = !0), v.parent && _r(v.parent.vnode) && v.parent.update();
          })
          .catch((S) => {
            _(S), (N.value = S);
          }),
        () => {
          if (T.value && u) return Vn(u, v);
          if (N.value && n)
            return oe(n, {
              error: N.value,
            });
          if (r && !R.value) return oe(r);
        }
      );
    },
  });
}
function Vn(t, e) {
  const { ref: r, props: n, children: s, ce: i } = e.vnode,
    o = oe(t, n, s);
  return (o.ref = r), (o.ce = i), delete e.vnode.ce, o;
}
const _r = (t) => t.type.__isKeepAlive,
  Bc = {
    name: "KeepAlive",
    __isKeepAlive: !0,
    props: {
      include: [String, RegExp, Array],
      exclude: [String, RegExp, Array],
      max: [String, Number],
    },
    setup(t, { slots: e }) {
      const r = Sr(),
        n = r.ctx;
      if (!n.renderer)
        return () => {
          const R = e.default && e.default();
          return R && R.length === 1 ? R[0] : R;
        };
      const s = new Map(),
        i = new Set();
      let o = null;
      const a = r.suspense,
        {
          renderer: {
            p: l,
            m: c,
            um: u,
            o: { createElement: d },
          },
        } = n,
        p = d("div");
      (n.activate = (R, S, y, D, U) => {
        const j = R.component;
        c(R, S, y, 0, a),
          l(j.vnode, R, S, y, j, a, D, R.slotScopeIds, U),
          ue(() => {
            (j.isDeactivated = !1), j.a && $t(j.a);
            const B = R.props && R.props.onVnodeMounted;
            B && Le(B, j.parent, R);
          }, a);
      }),
        (n.deactivate = (R) => {
          const S = R.component;
          en(S.m),
            en(S.a),
            c(R, p, null, 1, a),
            ue(() => {
              S.da && $t(S.da);
              const y = R.props && R.props.onVnodeUnmounted;
              y && Le(y, S.parent, R), (S.isDeactivated = !0);
            }, a);
        });
      function g(R) {
        Dn(R), u(R, r, a, !0);
      }
      function v(R) {
        s.forEach((S, y) => {
          const D = ws(S.type);
          D && !R(D) && _(y);
        });
      }
      function _(R) {
        const S = s.get(R);
        S && (!o || !lt(S, o)) ? g(S) : o && Dn(o), s.delete(R), i.delete(R);
      }
      Hr(
        () => [t.include, t.exclude],
        ([R, S]) => {
          R && v((y) => nr(R, y)), S && v((y) => !nr(S, y));
        },
        {
          flush: "post",
          deep: !0,
        }
      );
      let T = null;
      const N = () => {
        T != null &&
          (tn(r.subTree.type)
            ? ue(() => {
                s.set(T, Vr(r.subTree));
              }, r.subTree.suspense)
            : s.set(T, Vr(r.subTree)));
      };
      return (
        vn(N),
        qs(N),
        Ks(() => {
          s.forEach((R) => {
            const { subTree: S, suspense: y } = r,
              D = Vr(S);
            if (R.type === D.type && R.key === D.key) {
              Dn(D);
              const U = D.component.da;
              U && ue(U, y);
              return;
            }
            g(R);
          });
        }),
        () => {
          if (((T = null), !e.default)) return (o = null);
          const R = e.default(),
            S = R[0];
          if (R.length > 1) return (o = null), R;
          if (!Ht(S) || (!(S.shapeFlag & 4) && !(S.shapeFlag & 128)))
            return (o = null), S;
          let y = Vr(S);
          if (y.type === Ee) return (o = null), y;
          const D = y.type,
            U = ws(It(y) ? y.type.__asyncResolved || {} : D),
            { include: j, exclude: B, max: k } = t;
          if ((j && (!U || !nr(j, U))) || (B && U && nr(B, U)))
            return (y.shapeFlag &= -257), (o = y), S;
          const H = y.key == null ? D : y.key,
            W = s.get(H);
          return (
            y.el && ((y = tt(y)), S.shapeFlag & 128 && (S.ssContent = y)),
            (T = H),
            W
              ? ((y.el = W.el),
                (y.component = W.component),
                y.transition && ft(y, y.transition),
                (y.shapeFlag |= 512),
                i.delete(H),
                i.add(H))
              : (i.add(H),
                k && i.size > parseInt(k, 10) && _(i.values().next().value)),
            (y.shapeFlag |= 256),
            (o = y),
            tn(S.type) ? S : y
          );
        }
      );
    },
  },
  yp = Bc;
function nr(t, e) {
  return L(t)
    ? t.some((r) => nr(r, e))
    : re(t)
    ? t.split(",").includes(e)
    : jl(t)
    ? ((t.lastIndex = 0), t.test(e))
    : !1;
}
function qc(t, e) {
  wa(t, "a", e);
}
function Kc(t, e) {
  wa(t, "da", e);
}
function wa(t, e, r = ae) {
  const n =
    t.__wdc ||
    (t.__wdc = () => {
      let s = r;
      for (; s; ) {
        if (s.isDeactivated) return;
        s = s.parent;
      }
      return t();
    });
  if ((mn(e, n, r), r)) {
    let s = r.parent;
    for (; s && s.parent; )
      _r(s.parent.vnode) && Wc(n, e, r, s), (s = s.parent);
  }
}
function Wc(t, e, r, n) {
  const s = mn(e, t, n, !0);
  Ws(() => {
    Ps(n[e], s);
  }, r);
}
function Dn(t) {
  (t.shapeFlag &= -257), (t.shapeFlag &= -513);
}
function Vr(t) {
  return t.shapeFlag & 128 ? t.ssContent : t;
}
function mn(t, e, r = ae, n = !1) {
  if (r) {
    const s = r[t] || (r[t] = []),
      i =
        e.__weh ||
        (e.__weh = (...o) => {
          pt();
          const a = Ir(r),
            l = Fe(e, r, t, o);
          return a(), gt(), l;
        });
    return n ? s.unshift(i) : s.push(i), i;
  }
}
const rt =
    (t) =>
    (e, r = ae) => {
      (!Bt || t === "sp") && mn(t, (...n) => e(...n), r);
    },
  _a = rt("bm"),
  vn = rt("m"),
  Gc = rt("bu"),
  qs = rt("u"),
  Ks = rt("bum"),
  Ws = rt("um"),
  zc = rt("sp"),
  Jc = rt("rtg"),
  Yc = rt("rtc");
function Xc(t, e = ae) {
  mn("ec", t, e);
}
const Gs = "components",
  Qc = "directives";
function bp(t, e) {
  return zs(Gs, t, !0, e) || t;
}
const Sa = Symbol.for("v-ndc");
function wp(t) {
  return re(t) ? zs(Gs, t, !1) || t : t || Sa;
}
function _p(t) {
  return zs(Qc, t);
}
function zs(t, e, r = !0, n = !1) {
  const s = le || ae;
  if (s) {
    const i = s.type;
    if (t === Gs) {
      const a = ws(i, !1);
      if (a && (a === e || a === ke(e) || a === fn(ke(e)))) return i;
    }
    const o = _i(s[t] || i[t], e) || _i(s.appContext[t], e);
    return !o && n ? i : o;
  }
}
function _i(t, e) {
  return t && (t[e] || t[ke(e)] || t[fn(ke(e))]);
}
function Sp(t, e, r, n) {
  let s;
  const i = r,
    o = L(t);
  if (o || re(t)) {
    const a = o && Ft(t);
    let l = !1;
    a && ((l = !Ne(t)), (t = pn(t))), (s = new Array(t.length));
    for (let c = 0, u = t.length; c < u; c++)
      s[c] = e(l ? xe(t[c]) : t[c], c, void 0, i);
  } else if (typeof t == "number") {
    s = new Array(t);
    for (let a = 0; a < t; a++) s[a] = e(a + 1, a, void 0, i);
  } else if (J(t))
    if (t[Symbol.iterator]) s = Array.from(t, (a, l) => e(a, l, void 0, i));
    else {
      const a = Object.keys(t);
      s = new Array(a.length);
      for (let l = 0, c = a.length; l < c; l++) {
        const u = a[l];
        s[l] = e(t[u], u, l, i);
      }
    }
  else s = [];
  return s;
}
function Ip(t, e) {
  for (let r = 0; r < e.length; r++) {
    const n = e[r];
    if (L(n)) for (let s = 0; s < n.length; s++) t[n[s].name] = n[s].fn;
    else
      n &&
        (t[n.name] = n.key
          ? (...s) => {
              const i = n.fn(...s);
              return i && (i.key = n.key), i;
            }
          : n.fn);
  }
  return t;
}
function Tp(t, e, r = {}, n, s) {
  if (le.ce || (le.parent && It(le.parent) && le.parent.ce))
    return (
      e !== "default" && (r.name = e),
      Es(),
      ys(Se, null, [oe("slot", r, n && n())], 64)
    );
  let i = t[e];
  i && i._c && (i._d = !1), Es();
  const o = i && Ia(i(r)),
    a = r.key || (o && o.key),
    l = ys(
      Se,
      {
        key: (a && !$e(a) ? a : "_".concat(e)) + (!o && n ? "_fb" : ""),
      },
      o || (n ? n() : []),
      o && t._ === 1 ? 64 : -2
    );
  return (
    !s && l.scopeId && (l.slotScopeIds = [l.scopeId + "-s"]),
    i && i._c && (i._d = !0),
    l
  );
}
function Ia(t) {
  return t.some((e) =>
    Ht(e) ? !(e.type === Ee || (e.type === Se && !Ia(e.children))) : !0
  )
    ? t
    : null;
}
function Cp(t, e) {
  const r = {};
  for (const n in t) r[$r(n)] = t[n];
  return r;
}
const gs = (t) => (t ? (Ba(t) ? bn(t) : gs(t.parent)) : null),
  cr = ie(Object.create(null), {
    $: (t) => t,
    $el: (t) => t.vnode.el,
    $data: (t) => t.data,
    $props: (t) => t.props,
    $attrs: (t) => t.attrs,
    $slots: (t) => t.slots,
    $refs: (t) => t.refs,
    $parent: (t) => gs(t.parent),
    $root: (t) => gs(t.root),
    $host: (t) => t.ce,
    $emit: (t) => t.emit,
    $options: (t) => Js(t),
    $forceUpdate: (t) =>
      t.f ||
      (t.f = () => {
        js(t.update);
      }),
    $nextTick: (t) => t.n || (t.n = Mc.bind(t.proxy)),
    $watch: (t) => wu.bind(t),
  }),
  $n = (t, e) => t !== Y && !t.__isScriptSetup && G(t, e),
  Zc = {
    get({ _: t }, e) {
      if (e === "__v_skip") return !0;
      const {
        ctx: r,
        setupState: n,
        data: s,
        props: i,
        accessCache: o,
        type: a,
        appContext: l,
      } = t;
      let c;
      if (e[0] !== "$") {
        const g = o[e];
        if (g !== void 0)
          switch (g) {
            case 1:
              return n[e];
            case 2:
              return s[e];
            case 4:
              return r[e];
            case 3:
              return i[e];
          }
        else {
          if ($n(n, e)) return (o[e] = 1), n[e];
          if (s !== Y && G(s, e)) return (o[e] = 2), s[e];
          if ((c = t.propsOptions[0]) && G(c, e)) return (o[e] = 3), i[e];
          if (r !== Y && G(r, e)) return (o[e] = 4), r[e];
          ms && (o[e] = 0);
        }
      }
      const u = cr[e];
      let d, p;
      if (u) return e === "$attrs" && ve(t.attrs, "get", ""), u(t);
      if ((d = a.__cssModules) && (d = d[e])) return d;
      if (r !== Y && G(r, e)) return (o[e] = 4), r[e];
      if (((p = l.config.globalProperties), G(p, e))) return p[e];
    },
    set({ _: t }, e, r) {
      const { data: n, setupState: s, ctx: i } = t;
      return $n(s, e)
        ? ((s[e] = r), !0)
        : n !== Y && G(n, e)
        ? ((n[e] = r), !0)
        : G(t.props, e) || (e[0] === "$" && e.slice(1) in t)
        ? !1
        : ((i[e] = r), !0);
    },
    has(
      {
        _: {
          data: t,
          setupState: e,
          accessCache: r,
          ctx: n,
          appContext: s,
          propsOptions: i,
        },
      },
      o
    ) {
      let a;
      return (
        !!r[o] ||
        (t !== Y && G(t, o)) ||
        $n(e, o) ||
        ((a = i[0]) && G(a, o)) ||
        G(n, o) ||
        G(cr, o) ||
        G(s.config.globalProperties, o)
      );
    },
    defineProperty(t, e, r) {
      return (
        r.get != null
          ? (t._.accessCache[e] = 0)
          : G(r, "value") && this.set(t, e, r.value, null),
        Reflect.defineProperty(t, e, r)
      );
    },
  };
function Rp() {
  return eu().slots;
}
function eu() {
  const t = Sr();
  return t.setupContext || (t.setupContext = Ka(t));
}
function Si(t) {
  return L(t) ? t.reduce((e, r) => ((e[r] = null), e), {}) : t;
}
let ms = !0;
function tu(t) {
  const e = Js(t),
    r = t.proxy,
    n = t.ctx;
  (ms = !1), e.beforeCreate && Ii(e.beforeCreate, t, "bc");
  const {
    data: s,
    computed: i,
    methods: o,
    watch: a,
    provide: l,
    inject: c,
    created: u,
    beforeMount: d,
    mounted: p,
    beforeUpdate: g,
    updated: v,
    activated: _,
    deactivated: T,
    beforeDestroy: N,
    beforeUnmount: R,
    destroyed: S,
    unmounted: y,
    render: D,
    renderTracked: U,
    renderTriggered: j,
    errorCaptured: B,
    serverPrefetch: k,
    expose: H,
    inheritAttrs: W,
    components: P,
    directives: ee,
    filters: pe,
  } = e;
  if ((c && ru(c, n, null), o))
    for (const ne in o) {
      const X = o[ne];
      $(X) && (n[ne] = X.bind(r));
    }
  if (s) {
    const ne = s.call(r, r);
    J(ne) && (t.data = gn(ne));
  }
  if (((ms = !0), i))
    for (const ne in i) {
      const X = i[ne],
        xt = $(X) ? X.bind(r, r) : $(X.get) ? X.get.bind(r, r) : Ke,
        Ar = !$(X) && $(X.set) ? X.set.bind(r) : Ke,
        Et = Hu({
          get: xt,
          set: Ar,
        });
      Object.defineProperty(n, ne, {
        enumerable: !0,
        configurable: !0,
        get: () => Et.value,
        set: (je) => (Et.value = je),
      });
    }
  if (a) for (const ne in a) Ta(a[ne], n, r, ne);
  if (l) {
    const ne = $(l) ? l.call(r) : l;
    Reflect.ownKeys(ne).forEach((X) => {
      lu(X, ne[X]);
    });
  }
  u && Ii(u, t, "c");
  function ce(ne, X) {
    L(X) ? X.forEach((xt) => ne(xt.bind(r))) : X && ne(X.bind(r));
  }
  if (
    (ce(_a, d),
    ce(vn, p),
    ce(Gc, g),
    ce(qs, v),
    ce(qc, _),
    ce(Kc, T),
    ce(Xc, B),
    ce(Yc, U),
    ce(Jc, j),
    ce(Ks, R),
    ce(Ws, y),
    ce(zc, k),
    L(H))
  )
    if (H.length) {
      const ne = t.exposed || (t.exposed = {});
      H.forEach((X) => {
        Object.defineProperty(ne, X, {
          get: () => r[X],
          set: (xt) => (r[X] = xt),
        });
      });
    } else t.exposed || (t.exposed = {});
  D && t.render === Ke && (t.render = D),
    W != null && (t.inheritAttrs = W),
    P && (t.components = P),
    ee && (t.directives = ee),
    k && Bs(t);
}
function ru(t, e, r = Ke) {
  L(t) && (t = vs(t));
  for (const n in t) {
    const s = t[n];
    let i;
    J(s)
      ? "default" in s
        ? (i = jr(s.from || n, s.default, !0))
        : (i = jr(s.from || n))
      : (i = jr(s)),
      fe(i)
        ? Object.defineProperty(e, n, {
            enumerable: !0,
            configurable: !0,
            get: () => i.value,
            set: (o) => (i.value = o),
          })
        : (e[n] = i);
  }
}
function Ii(t, e, r) {
  Fe(L(t) ? t.map((n) => n.bind(e.proxy)) : t.bind(e.proxy), e, r);
}
function Ta(t, e, r, n) {
  let s = n.includes(".") ? $a(r, n) : () => r[n];
  if (re(t)) {
    const i = e[t];
    $(i) && Hr(s, i);
  } else if ($(t)) Hr(s, t.bind(r));
  else if (J(t))
    if (L(t)) t.forEach((i) => Ta(i, e, r, n));
    else {
      const i = $(t.handler) ? t.handler.bind(r) : e[t.handler];
      $(i) && Hr(s, i, t);
    }
}
function Js(t) {
  const e = t.type,
    { mixins: r, extends: n } = e,
    {
      mixins: s,
      optionsCache: i,
      config: { optionMergeStrategies: o },
    } = t.appContext,
    a = i.get(e);
  let l;
  return (
    a
      ? (l = a)
      : !s.length && !r && !n
      ? (l = e)
      : ((l = {}), s.length && s.forEach((c) => Zr(l, c, o, !0)), Zr(l, e, o)),
    J(e) && i.set(e, l),
    l
  );
}
function Zr(t, e, r, n = !1) {
  const { mixins: s, extends: i } = e;
  i && Zr(t, i, r, !0), s && s.forEach((o) => Zr(t, o, r, !0));
  for (const o in e)
    if (!(n && o === "expose")) {
      const a = nu[o] || (r && r[o]);
      t[o] = a ? a(t[o], e[o]) : e[o];
    }
  return t;
}
const nu = {
  data: Ti,
  props: Ci,
  emits: Ci,
  methods: sr,
  computed: sr,
  beforeCreate: be,
  created: be,
  beforeMount: be,
  mounted: be,
  beforeUpdate: be,
  updated: be,
  beforeDestroy: be,
  beforeUnmount: be,
  destroyed: be,
  unmounted: be,
  activated: be,
  deactivated: be,
  errorCaptured: be,
  serverPrefetch: be,
  components: sr,
  directives: sr,
  watch: iu,
  provide: Ti,
  inject: su,
};
function Ti(t, e) {
  return e
    ? t
      ? function () {
          return ie(
            $(t) ? t.call(this, this) : t,
            $(e) ? e.call(this, this) : e
          );
        }
      : e
    : t;
}
function su(t, e) {
  return sr(vs(t), vs(e));
}
function vs(t) {
  if (L(t)) {
    const e = {};
    for (let r = 0; r < t.length; r++) e[t[r]] = t[r];
    return e;
  }
  return t;
}
function be(t, e) {
  return t ? [...new Set([].concat(t, e))] : e;
}
function sr(t, e) {
  return t ? ie(Object.create(null), t, e) : e;
}
function Ci(t, e) {
  return t
    ? L(t) && L(e)
      ? [...new Set([...t, ...e])]
      : ie(Object.create(null), Si(t), Si(e != null ? e : {}))
    : e;
}
function iu(t, e) {
  if (!t) return e;
  if (!e) return t;
  const r = ie(Object.create(null), t);
  for (const n in e) r[n] = be(t[n], e[n]);
  return r;
}
function Ca() {
  return {
    app: null,
    config: {
      isNativeTag: Fl,
      performance: !1,
      globalProperties: {},
      optionMergeStrategies: {},
      errorHandler: void 0,
      warnHandler: void 0,
      compilerOptions: {},
    },
    mixins: [],
    components: {},
    directives: {},
    provides: Object.create(null),
    optionsCache: new WeakMap(),
    propsCache: new WeakMap(),
    emitsCache: new WeakMap(),
  };
}
let ou = 0;
function au(t, e) {
  return function (n, s = null) {
    $(n) || (n = ie({}, n)), s != null && !J(s) && (s = null);
    const i = Ca(),
      o = new WeakSet(),
      a = [];
    let l = !1;
    const c = (i.app = {
      _uid: ou++,
      _component: n,
      _props: s,
      _container: null,
      _context: i,
      _instance: null,
      version: qu,
      get config() {
        return i.config;
      },
      set config(u) {},
      use(u, ...d) {
        return (
          o.has(u) ||
            (u && $(u.install)
              ? (o.add(u), u.install(c, ...d))
              : $(u) && (o.add(u), u(c, ...d))),
          c
        );
      },
      mixin(u) {
        return i.mixins.includes(u) || i.mixins.push(u), c;
      },
      component(u, d) {
        return d ? ((i.components[u] = d), c) : i.components[u];
      },
      directive(u, d) {
        return d ? ((i.directives[u] = d), c) : i.directives[u];
      },
      mount(u, d, p) {
        if (!l) {
          const g = c._ceVNode || oe(n, s);
          return (
            (g.appContext = i),
            p === !0 ? (p = "svg") : p === !1 && (p = void 0),
            d && e ? e(g, u) : t(g, u, p),
            (l = !0),
            (c._container = u),
            (u.__vue_app__ = c),
            bn(g.component)
          );
        }
      },
      onUnmount(u) {
        a.push(u);
      },
      unmount() {
        l &&
          (Fe(a, c._instance, 16),
          t(null, c._container),
          delete c._container.__vue_app__);
      },
      provide(u, d) {
        return (i.provides[u] = d), c;
      },
      runWithContext(u) {
        const d = Tt;
        Tt = c;
        try {
          return u();
        } finally {
          Tt = d;
        }
      },
    });
    return c;
  };
}
let Tt = null;
function lu(t, e) {
  if (ae) {
    let r = ae.provides;
    const n = ae.parent && ae.parent.provides;
    n === r && (r = ae.provides = Object.create(n)), (r[t] = e);
  }
}
function jr(t, e, r = !1) {
  const n = ae || le;
  if (n || Tt) {
    const s = Tt
      ? Tt._context.provides
      : n
      ? n.parent == null
        ? n.vnode.appContext && n.vnode.appContext.provides
        : n.parent.provides
      : void 0;
    if (s && t in s) return s[t];
    if (arguments.length > 1) return r && $(e) ? e.call(n && n.proxy) : e;
  }
}
function Ap() {
  return !!(ae || le || Tt);
}
const Ra = {},
  Aa = () => Object.create(Ra),
  Oa = (t) => Object.getPrototypeOf(t) === Ra;
function cu(t, e, r, n = !1) {
  const s = {},
    i = Aa();
  (t.propsDefaults = Object.create(null)), Pa(t, e, s, i);
  for (const o in t.propsOptions[0]) o in s || (s[o] = void 0);
  r ? (t.props = n ? s : yc(s)) : t.type.props ? (t.props = s) : (t.props = i),
    (t.attrs = i);
}
function uu(t, e, r, n) {
  const {
      props: s,
      attrs: i,
      vnode: { patchFlag: o },
    } = t,
    a = K(s),
    [l] = t.propsOptions;
  let c = !1;
  if ((n || o > 0) && !(o & 16)) {
    if (o & 8) {
      const u = t.vnode.dynamicProps;
      for (let d = 0; d < u.length; d++) {
        let p = u[d];
        if (En(t.emitsOptions, p)) continue;
        const g = e[p];
        if (l)
          if (G(i, p)) g !== i[p] && ((i[p] = g), (c = !0));
          else {
            const v = ke(p);
            s[v] = xs(l, a, v, g, t, !1);
          }
        else g !== i[p] && ((i[p] = g), (c = !0));
      }
    }
  } else {
    Pa(t, e, s, i) && (c = !0);
    let u;
    for (const d in a)
      (!e || (!G(e, d) && ((u = ht(d)) === d || !G(e, u)))) &&
        (l
          ? r &&
            (r[d] !== void 0 || r[u] !== void 0) &&
            (s[d] = xs(l, a, d, void 0, t, !0))
          : delete s[d]);
    if (i !== a) for (const d in i) (!e || !G(e, d)) && (delete i[d], (c = !0));
  }
  c && Qe(t.attrs, "set", "");
}
function Pa(t, e, r, n) {
  const [s, i] = t.propsOptions;
  let o = !1,
    a;
  if (e)
    for (let l in e) {
      if (ir(l)) continue;
      const c = e[l];
      let u;
      s && G(s, (u = ke(l)))
        ? !i || !i.includes(u)
          ? (r[u] = c)
          : ((a || (a = {}))[u] = c)
        : En(t.emitsOptions, l) ||
          ((!(l in n) || c !== n[l]) && ((n[l] = c), (o = !0)));
    }
  if (i) {
    const l = K(r),
      c = a || Y;
    for (let u = 0; u < i.length; u++) {
      const d = i[u];
      r[d] = xs(s, l, d, c[d], t, !G(c, d));
    }
  }
  return o;
}
function xs(t, e, r, n, s, i) {
  const o = t[r];
  if (o != null) {
    const a = G(o, "default");
    if (a && n === void 0) {
      const l = o.default;
      if (o.type !== Function && !o.skipFactory && $(l)) {
        const { propsDefaults: c } = s;
        if (r in c) n = c[r];
        else {
          const u = Ir(s);
          (n = c[r] = l.call(null, e)), u();
        }
      } else n = l;
      s.ce && s.ce._setProp(r, n);
    }
    o[0] &&
      (i && !a ? (n = !1) : o[1] && (n === "" || n === ht(r)) && (n = !0));
  }
  return n;
}
const fu = new WeakMap();
function Ma(t, e, r = !1) {
  const n = r ? fu : e.propsCache,
    s = n.get(t);
  if (s) return s;
  const i = t.props,
    o = {},
    a = [];
  let l = !1;
  if (!$(t)) {
    const u = (d) => {
      l = !0;
      const [p, g] = Ma(d, e, !0);
      ie(o, p), g && a.push(...g);
    };
    !r && e.mixins.length && e.mixins.forEach(u),
      t.extends && u(t.extends),
      t.mixins && t.mixins.forEach(u);
  }
  if (!i && !l) return J(t) && n.set(t, Vt), Vt;
  if (L(i))
    for (let u = 0; u < i.length; u++) {
      const d = ke(i[u]);
      Ri(d) && (o[d] = Y);
    }
  else if (i)
    for (const u in i) {
      const d = ke(u);
      if (Ri(d)) {
        const p = i[u],
          g = (o[d] =
            L(p) || $(p)
              ? {
                  type: p,
                }
              : ie({}, p)),
          v = g.type;
        let _ = !1,
          T = !0;
        if (L(v))
          for (let N = 0; N < v.length; ++N) {
            const R = v[N],
              S = $(R) && R.name;
            if (S === "Boolean") {
              _ = !0;
              break;
            } else S === "String" && (T = !1);
          }
        else _ = $(v) && v.name === "Boolean";
        (g[0] = _), (g[1] = T), (_ || G(g, "default")) && a.push(d);
      }
    }
  const c = [o, a];
  return J(t) && n.set(t, c), c;
}
function Ri(t) {
  return t[0] !== "$" && !ir(t);
}
const La = (t) => t[0] === "_" || t === "$stable",
  Ys = (t) => (L(t) ? t.map(qe) : [qe(t)]),
  du = (t, e, r) => {
    if (e._n) return e;
    const n = kc((...s) => Ys(e(...s)), r);
    return (n._c = !1), n;
  },
  Na = (t, e, r) => {
    const n = t._ctx;
    for (const s in t) {
      if (La(s)) continue;
      const i = t[s];
      if ($(i)) e[s] = du(s, i, n);
      else if (i != null) {
        const o = Ys(i);
        e[s] = () => o;
      }
    }
  },
  ka = (t, e) => {
    const r = Ys(e);
    t.slots.default = () => r;
  },
  Va = (t, e, r) => {
    for (const n in e) (r || n !== "_") && (t[n] = e[n]);
  },
  hu = (t, e, r) => {
    const n = (t.slots = Aa());
    if (t.vnode.shapeFlag & 32) {
      const s = e._;
      s ? (Va(n, e, r), r && $o(n, "_", s, !0)) : Na(e, n);
    } else e && ka(t, e);
  },
  pu = (t, e, r) => {
    const { vnode: n, slots: s } = t;
    let i = !0,
      o = Y;
    if (n.shapeFlag & 32) {
      const a = e._;
      a
        ? r && a === 1
          ? (i = !1)
          : Va(s, e, r)
        : ((i = !e.$stable), Na(e, s)),
        (o = e);
    } else
      e &&
        (ka(t, e),
        (o = {
          default: 1,
        }));
    if (i) for (const a in s) !La(a) && o[a] == null && delete s[a];
  },
  ue = Au;
function gu(t) {
  return mu(t);
}
function mu(t, e) {
  const r = dn();
  r.__VUE__ = !0;
  const {
      insert: n,
      remove: s,
      patchProp: i,
      createElement: o,
      createText: a,
      createComment: l,
      setText: c,
      setElementText: u,
      parentNode: d,
      nextSibling: p,
      setScopeId: g = Ke,
      insertStaticContent: v,
    } = t,
    _ = (
      f,
      h,
      m,
      b = null,
      x = null,
      E = null,
      A = void 0,
      C = null,
      I = !!h.dynamicChildren
    ) => {
      if (f === h) return;
      f && !lt(f, h) && ((b = Or(f)), je(f, x, E, !0), (f = null)),
        h.patchFlag === -2 && ((I = !1), (h.dynamicChildren = null));
      const { type: w, ref: V, shapeFlag: O } = h;
      switch (w) {
        case yn:
          T(f, h, m, b);
          break;
        case Ee:
          N(f, h, m, b);
          break;
        case ur:
          f == null && R(h, m, b, A);
          break;
        case Se:
          P(f, h, m, b, x, E, A, C, I);
          break;
        default:
          O & 1
            ? D(f, h, m, b, x, E, A, C, I)
            : O & 6
            ? ee(f, h, m, b, x, E, A, C, I)
            : (O & 64 || O & 128) && w.process(f, h, m, b, x, E, A, C, I, Qt);
      }
      V != null && x && ps(V, f && f.ref, E, h || f, !h);
    },
    T = (f, h, m, b) => {
      if (f == null) n((h.el = a(h.children)), m, b);
      else {
        const x = (h.el = f.el);
        h.children !== f.children && c(x, h.children);
      }
    },
    N = (f, h, m, b) => {
      f == null ? n((h.el = l(h.children || "")), m, b) : (h.el = f.el);
    },
    R = (f, h, m, b) => {
      [f.el, f.anchor] = v(f.children, h, m, b, f.el, f.anchor);
    },
    S = ({ el: f, anchor: h }, m, b) => {
      let x;
      for (; f && f !== h; ) (x = p(f)), n(f, m, b), (f = x);
      n(h, m, b);
    },
    y = ({ el: f, anchor: h }) => {
      let m;
      for (; f && f !== h; ) (m = p(f)), s(f), (f = m);
      s(h);
    },
    D = (f, h, m, b, x, E, A, C, I) => {
      h.type === "svg" ? (A = "svg") : h.type === "math" && (A = "mathml"),
        f == null ? U(h, m, b, x, E, A, C, I) : k(f, h, x, E, A, C, I);
    },
    U = (f, h, m, b, x, E, A, C) => {
      let I, w;
      const { props: V, shapeFlag: O, transition: M, dirs: F } = f;
      if (
        ((I = f.el = o(f.type, E, V && V.is, V)),
        O & 8
          ? u(I, f.children)
          : O & 16 && B(f.children, I, null, b, x, Fn(f, E), A, C),
        F && yt(f, null, b, "created"),
        j(I, f, f.scopeId, A, b),
        V)
      ) {
        for (const Q in V) Q !== "value" && !ir(Q) && i(I, Q, null, V[Q], E, b);
        "value" in V && i(I, "value", null, V.value, E),
          (w = V.onVnodeBeforeMount) && Le(w, b, f);
      }
      F && yt(f, null, b, "beforeMount");
      const q = vu(x, M);
      q && M.beforeEnter(I),
        n(I, h, m),
        ((w = V && V.onVnodeMounted) || q || F) &&
          ue(() => {
            w && Le(w, b, f), q && M.enter(I), F && yt(f, null, b, "mounted");
          }, x);
    },
    j = (f, h, m, b, x) => {
      if ((m && g(f, m), b)) for (let E = 0; E < b.length; E++) g(f, b[E]);
      if (x) {
        let E = x.subTree;
        if (
          h === E ||
          (tn(E.type) && (E.ssContent === h || E.ssFallback === h))
        ) {
          const A = x.vnode;
          j(f, A, A.scopeId, A.slotScopeIds, x.parent);
        }
      }
    },
    B = (f, h, m, b, x, E, A, C, I = 0) => {
      for (let w = I; w < f.length; w++) {
        const V = (f[w] = C ? at(f[w]) : qe(f[w]));
        _(null, V, h, m, b, x, E, A, C);
      }
    },
    k = (f, h, m, b, x, E, A) => {
      const C = (h.el = f.el);
      let { patchFlag: I, dynamicChildren: w, dirs: V } = h;
      I |= f.patchFlag & 16;
      const O = f.props || Y,
        M = h.props || Y;
      let F;
      if (
        (m && bt(m, !1),
        (F = M.onVnodeBeforeUpdate) && Le(F, m, h, f),
        V && yt(h, f, m, "beforeUpdate"),
        m && bt(m, !0),
        ((O.innerHTML && M.innerHTML == null) ||
          (O.textContent && M.textContent == null)) &&
          u(C, ""),
        w
          ? H(f.dynamicChildren, w, C, m, b, Fn(h, x), E)
          : A || X(f, h, C, null, m, b, Fn(h, x), E, !1),
        I > 0)
      ) {
        if (I & 16) W(C, O, M, m, x);
        else if (
          (I & 2 && O.class !== M.class && i(C, "class", null, M.class, x),
          I & 4 && i(C, "style", O.style, M.style, x),
          I & 8)
        ) {
          const q = h.dynamicProps;
          for (let Q = 0; Q < q.length; Q++) {
            const z = q[Q],
              Te = O[z],
              ge = M[z];
            (ge !== Te || z === "value") && i(C, z, Te, ge, x, m);
          }
        }
        I & 1 && f.children !== h.children && u(C, h.children);
      } else !A && w == null && W(C, O, M, m, x);
      ((F = M.onVnodeUpdated) || V) &&
        ue(() => {
          F && Le(F, m, h, f), V && yt(h, f, m, "updated");
        }, b);
    },
    H = (f, h, m, b, x, E, A) => {
      for (let C = 0; C < h.length; C++) {
        const I = f[C],
          w = h[C],
          V =
            I.el && (I.type === Se || !lt(I, w) || I.shapeFlag & 70)
              ? d(I.el)
              : m;
        _(I, w, V, null, b, x, E, A, !0);
      }
    },
    W = (f, h, m, b, x) => {
      if (h !== m) {
        if (h !== Y)
          for (const E in h) !ir(E) && !(E in m) && i(f, E, h[E], null, x, b);
        for (const E in m) {
          if (ir(E)) continue;
          const A = m[E],
            C = h[E];
          A !== C && E !== "value" && i(f, E, C, A, x, b);
        }
        "value" in m && i(f, "value", h.value, m.value, x);
      }
    },
    P = (f, h, m, b, x, E, A, C, I) => {
      const w = (h.el = f ? f.el : a("")),
        V = (h.anchor = f ? f.anchor : a(""));
      let { patchFlag: O, dynamicChildren: M, slotScopeIds: F } = h;
      F && (C = C ? C.concat(F) : F),
        f == null
          ? (n(w, m, b), n(V, m, b), B(h.children || [], m, V, x, E, A, C, I))
          : O > 0 && O & 64 && M && f.dynamicChildren
          ? (H(f.dynamicChildren, M, m, x, E, A, C),
            (h.key != null || (x && h === x.subTree)) && Xs(f, h, !0))
          : X(f, h, m, V, x, E, A, C, I);
    },
    ee = (f, h, m, b, x, E, A, C, I) => {
      (h.slotScopeIds = C),
        f == null
          ? h.shapeFlag & 512
            ? x.ctx.activate(h, m, b, A, I)
            : pe(h, m, b, x, E, A, I)
          : vt(f, h, I);
    },
    pe = (f, h, m, b, x, E, A) => {
      const C = (f.component = Du(f, b, x));
      if ((_r(f) && (C.ctx.renderer = Qt), $u(C, !1, A), C.asyncDep)) {
        if ((x && x.registerDep(C, ce, A), !f.el)) {
          const I = (C.subTree = oe(Ee));
          N(null, I, h, m);
        }
      } else ce(C, f, h, m, x, E, A);
    },
    vt = (f, h, m) => {
      const b = (h.component = f.component);
      if (Cu(f, h, m))
        if (b.asyncDep && !b.asyncResolved) {
          ne(b, h, m);
          return;
        } else (b.next = h), b.update();
      else (h.el = f.el), (b.vnode = h);
    },
    ce = (f, h, m, b, x, E, A) => {
      const C = () => {
        if (f.isMounted) {
          let { next: O, bu: M, u: F, parent: q, vnode: Q } = f;
          {
            const Ce = Da(f);
            if (Ce) {
              O && ((O.el = Q.el), ne(f, O, A)),
                Ce.asyncDep.then(() => {
                  f.isUnmounted || C();
                });
              return;
            }
          }
          let z = O,
            Te;
          bt(f, !1),
            O ? ((O.el = Q.el), ne(f, O, A)) : (O = Q),
            M && $t(M),
            (Te = O.props && O.props.onVnodeBeforeUpdate) && Le(Te, q, O, Q),
            bt(f, !0);
          const ge = Un(f),
            Ve = f.subTree;
          (f.subTree = ge),
            _(Ve, ge, d(Ve.el), Or(Ve), f, x, E),
            (O.el = ge.el),
            z === null && Ru(f, ge.el),
            F && ue(F, x),
            (Te = O.props && O.props.onVnodeUpdated) &&
              ue(() => Le(Te, q, O, Q), x);
        } else {
          let O;
          const { el: M, props: F } = h,
            { bm: q, m: Q, parent: z, root: Te, type: ge } = f,
            Ve = It(h);
          if (
            (bt(f, !1),
            q && $t(q),
            !Ve && (O = F && F.onVnodeBeforeMount) && Le(O, z, h),
            bt(f, !0),
            M && fi)
          ) {
            const Ce = () => {
              (f.subTree = Un(f)), fi(M, f.subTree, f, x, null);
            };
            Ve && ge.__asyncHydrate ? ge.__asyncHydrate(M, f, Ce) : Ce();
          } else {
            Te.ce && Te.ce._injectChildStyle(ge);
            const Ce = (f.subTree = Un(f));
            _(null, Ce, m, b, f, x, E), (h.el = Ce.el);
          }
          if ((Q && ue(Q, x), !Ve && (O = F && F.onVnodeMounted))) {
            const Ce = h;
            ue(() => Le(O, z, Ce), x);
          }
          (h.shapeFlag & 256 ||
            (z && It(z.vnode) && z.vnode.shapeFlag & 256)) &&
            f.a &&
            ue(f.a, x),
            (f.isMounted = !0),
            (h = m = b = null);
        }
      };
      f.scope.on();
      const I = (f.effect = new qo(C));
      f.scope.off();
      const w = (f.update = I.run.bind(I)),
        V = (f.job = I.runIfDirty.bind(I));
      (V.i = f), (V.id = f.uid), (I.scheduler = () => js(V)), bt(f, !0), w();
    },
    ne = (f, h, m) => {
      h.component = f;
      const b = f.vnode.props;
      (f.vnode = h),
        (f.next = null),
        uu(f, h.props, b, m),
        pu(f, h.children, m),
        pt(),
        xi(f),
        gt();
    },
    X = (f, h, m, b, x, E, A, C, I = !1) => {
      const w = f && f.children,
        V = f ? f.shapeFlag : 0,
        O = h.children,
        { patchFlag: M, shapeFlag: F } = h;
      if (M > 0) {
        if (M & 128) {
          Ar(w, O, m, b, x, E, A, C, I);
          return;
        } else if (M & 256) {
          xt(w, O, m, b, x, E, A, C, I);
          return;
        }
      }
      F & 8
        ? (V & 16 && Xt(w, x, E), O !== w && u(m, O))
        : V & 16
        ? F & 16
          ? Ar(w, O, m, b, x, E, A, C, I)
          : Xt(w, x, E, !0)
        : (V & 8 && u(m, ""), F & 16 && B(O, m, b, x, E, A, C, I));
    },
    xt = (f, h, m, b, x, E, A, C, I) => {
      (f = f || Vt), (h = h || Vt);
      const w = f.length,
        V = h.length,
        O = Math.min(w, V);
      let M;
      for (M = 0; M < O; M++) {
        const F = (h[M] = I ? at(h[M]) : qe(h[M]));
        _(f[M], F, m, null, x, E, A, C, I);
      }
      w > V ? Xt(f, x, E, !0, !1, O) : B(h, m, b, x, E, A, C, I, O);
    },
    Ar = (f, h, m, b, x, E, A, C, I) => {
      let w = 0;
      const V = h.length;
      let O = f.length - 1,
        M = V - 1;
      for (; w <= O && w <= M; ) {
        const F = f[w],
          q = (h[w] = I ? at(h[w]) : qe(h[w]));
        if (lt(F, q)) _(F, q, m, null, x, E, A, C, I);
        else break;
        w++;
      }
      for (; w <= O && w <= M; ) {
        const F = f[O],
          q = (h[M] = I ? at(h[M]) : qe(h[M]));
        if (lt(F, q)) _(F, q, m, null, x, E, A, C, I);
        else break;
        O--, M--;
      }
      if (w > O) {
        if (w <= M) {
          const F = M + 1,
            q = F < V ? h[F].el : b;
          for (; w <= M; )
            _(null, (h[w] = I ? at(h[w]) : qe(h[w])), m, q, x, E, A, C, I), w++;
        }
      } else if (w > M) for (; w <= O; ) je(f[w], x, E, !0), w++;
      else {
        const F = w,
          q = w,
          Q = new Map();
        for (w = q; w <= M; w++) {
          const Re = (h[w] = I ? at(h[w]) : qe(h[w]));
          Re.key != null && Q.set(Re.key, w);
        }
        let z,
          Te = 0;
        const ge = M - q + 1;
        let Ve = !1,
          Ce = 0;
        const Zt = new Array(ge);
        for (w = 0; w < ge; w++) Zt[w] = 0;
        for (w = F; w <= O; w++) {
          const Re = f[w];
          if (Te >= ge) {
            je(Re, x, E, !0);
            continue;
          }
          let He;
          if (Re.key != null) He = Q.get(Re.key);
          else
            for (z = q; z <= M; z++)
              if (Zt[z - q] === 0 && lt(Re, h[z])) {
                He = z;
                break;
              }
          He === void 0
            ? je(Re, x, E, !0)
            : ((Zt[He - q] = w + 1),
              He >= Ce ? (Ce = He) : (Ve = !0),
              _(Re, h[He], m, null, x, E, A, C, I),
              Te++);
        }
        const di = Ve ? xu(Zt) : Vt;
        for (z = di.length - 1, w = ge - 1; w >= 0; w--) {
          const Re = q + w,
            He = h[Re],
            hi = Re + 1 < V ? h[Re + 1].el : b;
          Zt[w] === 0
            ? _(null, He, m, hi, x, E, A, C, I)
            : Ve && (z < 0 || w !== di[z] ? Et(He, m, hi, 2) : z--);
        }
      }
    },
    Et = (f, h, m, b, x = null) => {
      const { el: E, type: A, transition: C, children: I, shapeFlag: w } = f;
      if (w & 6) {
        Et(f.component.subTree, h, m, b);
        return;
      }
      if (w & 128) {
        f.suspense.move(h, m, b);
        return;
      }
      if (w & 64) {
        A.move(f, h, m, Qt);
        return;
      }
      if (A === Se) {
        n(E, h, m);
        for (let O = 0; O < I.length; O++) Et(I[O], h, m, b);
        n(f.anchor, h, m);
        return;
      }
      if (A === ur) {
        S(f, h, m);
        return;
      }
      if (b !== 2 && w & 1 && C)
        if (b === 0) C.beforeEnter(E), n(E, h, m), ue(() => C.enter(E), x);
        else {
          const { leave: O, delayLeave: M, afterLeave: F } = C,
            q = () => n(E, h, m),
            Q = () => {
              O(E, () => {
                q(), F && F();
              });
            };
          M ? M(E, q, Q) : Q();
        }
      else n(E, h, m);
    },
    je = (f, h, m, b = !1, x = !1) => {
      const {
        type: E,
        props: A,
        ref: C,
        children: I,
        dynamicChildren: w,
        shapeFlag: V,
        patchFlag: O,
        dirs: M,
        cacheIndex: F,
      } = f;
      if (
        (O === -2 && (x = !1),
        C != null && ps(C, null, m, f, !0),
        F != null && (h.renderCache[F] = void 0),
        V & 256)
      ) {
        h.ctx.deactivate(f);
        return;
      }
      const q = V & 1 && M,
        Q = !It(f);
      let z;
      if ((Q && (z = A && A.onVnodeBeforeUnmount) && Le(z, h, f), V & 6))
        kl(f.component, m, b);
      else {
        if (V & 128) {
          f.suspense.unmount(m, b);
          return;
        }
        q && yt(f, null, h, "beforeUnmount"),
          V & 64
            ? f.type.remove(f, h, m, Qt, b)
            : w && !w.hasOnce && (E !== Se || (O > 0 && O & 64))
            ? Xt(w, h, m, !1, !0)
            : ((E === Se && O & 384) || (!x && V & 16)) && Xt(I, h, m),
          b && li(f);
      }
      ((Q && (z = A && A.onVnodeUnmounted)) || q) &&
        ue(() => {
          z && Le(z, h, f), q && yt(f, null, h, "unmounted");
        }, m);
    },
    li = (f) => {
      const { type: h, el: m, anchor: b, transition: x } = f;
      if (h === Se) {
        Nl(m, b);
        return;
      }
      if (h === ur) {
        y(f);
        return;
      }
      const E = () => {
        s(m), x && !x.persisted && x.afterLeave && x.afterLeave();
      };
      if (f.shapeFlag & 1 && x && !x.persisted) {
        const { leave: A, delayLeave: C } = x,
          I = () => A(m, E);
        C ? C(f.el, E, I) : I();
      } else E();
    },
    Nl = (f, h) => {
      let m;
      for (; f !== h; ) (m = p(f)), s(f), (f = m);
      s(h);
    },
    kl = (f, h, m) => {
      const { bum: b, scope: x, job: E, subTree: A, um: C, m: I, a: w } = f;
      en(I),
        en(w),
        b && $t(b),
        x.stop(),
        E && ((E.flags |= 8), je(A, f, h, m)),
        C && ue(C, h),
        ue(() => {
          f.isUnmounted = !0;
        }, h),
        h &&
          h.pendingBranch &&
          !h.isUnmounted &&
          f.asyncDep &&
          !f.asyncResolved &&
          f.suspenseId === h.pendingId &&
          (h.deps--, h.deps === 0 && h.resolve());
    },
    Xt = (f, h, m, b = !1, x = !1, E = 0) => {
      for (let A = E; A < f.length; A++) je(f[A], h, m, b, x);
    },
    Or = (f) => {
      if (f.shapeFlag & 6) return Or(f.component.subTree);
      if (f.shapeFlag & 128) return f.suspense.next();
      const h = p(f.anchor || f.el),
        m = h && h[pa];
      return m ? p(m) : h;
    };
  let On = !1;
  const ci = (f, h, m) => {
      f == null
        ? h._vnode && je(h._vnode, null, null, !0)
        : _(h._vnode || null, f, h, null, null, null, m),
        (h._vnode = f),
        On || ((On = !0), xi(), fa(), (On = !1));
    },
    Qt = {
      p: _,
      um: je,
      m: Et,
      r: li,
      mt: pe,
      mc: B,
      pc: X,
      pbc: H,
      n: Or,
      o: t,
    };
  let ui, fi;
  return {
    render: ci,
    hydrate: ui,
    createApp: au(ci, ui),
  };
}
function Fn({ type: t, props: e }, r) {
  return (r === "svg" && t === "foreignObject") ||
    (r === "mathml" &&
      t === "annotation-xml" &&
      e &&
      e.encoding &&
      e.encoding.includes("html"))
    ? void 0
    : r;
}
function bt({ effect: t, job: e }, r) {
  r ? ((t.flags |= 32), (e.flags |= 4)) : ((t.flags &= -33), (e.flags &= -5));
}
function vu(t, e) {
  return (!t || (t && !t.pendingBranch)) && e && !e.persisted;
}
function Xs(t, e, r = !1) {
  const n = t.children,
    s = e.children;
  if (L(n) && L(s))
    for (let i = 0; i < n.length; i++) {
      const o = n[i];
      let a = s[i];
      a.shapeFlag & 1 &&
        !a.dynamicChildren &&
        ((a.patchFlag <= 0 || a.patchFlag === 32) &&
          ((a = s[i] = at(s[i])), (a.el = o.el)),
        !r && a.patchFlag !== -2 && Xs(o, a)),
        a.type === yn && (a.el = o.el);
    }
}
function xu(t) {
  const e = t.slice(),
    r = [0];
  let n, s, i, o, a;
  const l = t.length;
  for (n = 0; n < l; n++) {
    const c = t[n];
    if (c !== 0) {
      if (((s = r[r.length - 1]), t[s] < c)) {
        (e[n] = s), r.push(n);
        continue;
      }
      for (i = 0, o = r.length - 1; i < o; )
        (a = (i + o) >> 1), t[r[a]] < c ? (i = a + 1) : (o = a);
      c < t[r[i]] && (i > 0 && (e[n] = r[i - 1]), (r[i] = n));
    }
  }
  for (i = r.length, o = r[i - 1]; i-- > 0; ) (r[i] = o), (o = e[o]);
  return r;
}
function Da(t) {
  const e = t.subTree.component;
  if (e) return e.asyncDep && !e.asyncResolved ? e : Da(e);
}
function en(t) {
  if (t) for (let e = 0; e < t.length; e++) t[e].flags |= 8;
}
const Eu = Symbol.for("v-scx"),
  yu = () => jr(Eu);
function Op(t, e) {
  return xn(t, null, e);
}
function bu(t, e) {
  return xn(t, null, {
    flush: "post",
  });
}
function Hr(t, e, r) {
  return xn(t, e, r);
}
function xn(t, e, r = Y) {
  const { immediate: n, deep: s, flush: i, once: o } = r,
    a = ie({}, r),
    l = (e && n) || (!e && i !== "post");
  let c;
  if (Bt) {
    if (i === "sync") {
      const g = yu();
      c = g.__watcherHandles || (g.__watcherHandles = []);
    } else if (!l) {
      const g = () => {};
      return (g.stop = Ke), (g.resume = Ke), (g.pause = Ke), g;
    }
  }
  const u = ae;
  a.call = (g, v, _) => Fe(g, u, v, _);
  let d = !1;
  i === "post"
    ? (a.scheduler = (g) => {
        ue(g, u && u.suspense);
      })
    : i !== "sync" &&
      ((d = !0),
      (a.scheduler = (g, v) => {
        v ? g() : js(g);
      })),
    (a.augmentJob = (g) => {
      e && (g.flags |= 4),
        d && ((g.flags |= 2), u && ((g.id = u.uid), (g.i = u)));
    });
  const p = Oc(t, e, a);
  return Bt && (c ? c.push(p) : l && p()), p;
}
function wu(t, e, r) {
  const n = this.proxy,
    s = re(t) ? (t.includes(".") ? $a(n, t) : () => n[t]) : t.bind(n, n);
  let i;
  $(e) ? (i = e) : ((i = e.handler), (r = e));
  const o = Ir(this),
    a = xn(s, i.bind(n), r);
  return o(), a;
}
function $a(t, e) {
  const r = e.split(".");
  return () => {
    let n = t;
    for (let s = 0; s < r.length && n; s++) n = n[r[s]];
    return n;
  };
}
const _u = (t, e) =>
  e === "modelValue" || e === "model-value"
    ? t.modelModifiers
    : t["".concat(e, "Modifiers")] ||
      t["".concat(ke(e), "Modifiers")] ||
      t["".concat(ht(e), "Modifiers")];
function Su(t, e, ...r) {
  if (t.isUnmounted) return;
  const n = t.vnode.props || Y;
  let s = r;
  const i = e.startsWith("update:"),
    o = i && _u(n, e.slice(7));
  o &&
    (o.trim && (s = r.map((u) => (re(u) ? u.trim() : u))),
    o.number && (s = r.map(ls)));
  let a,
    l = n[(a = $r(e))] || n[(a = $r(ke(e)))];
  !l && i && (l = n[(a = $r(ht(e)))]), l && Fe(l, t, 6, s);
  const c = n[a + "Once"];
  if (c) {
    if (!t.emitted) t.emitted = {};
    else if (t.emitted[a]) return;
    (t.emitted[a] = !0), Fe(c, t, 6, s);
  }
}
function Fa(t, e, r = !1) {
  const n = e.emitsCache,
    s = n.get(t);
  if (s !== void 0) return s;
  const i = t.emits;
  let o = {},
    a = !1;
  if (!$(t)) {
    const l = (c) => {
      const u = Fa(c, e, !0);
      u && ((a = !0), ie(o, u));
    };
    !r && e.mixins.length && e.mixins.forEach(l),
      t.extends && l(t.extends),
      t.mixins && t.mixins.forEach(l);
  }
  return !i && !a
    ? (J(t) && n.set(t, null), null)
    : (L(i) ? i.forEach((l) => (o[l] = null)) : ie(o, i),
      J(t) && n.set(t, o),
      o);
}
function En(t, e) {
  return !t || !ln(e)
    ? !1
    : ((e = e.slice(2).replace(/Once$/, "")),
      G(t, e[0].toLowerCase() + e.slice(1)) || G(t, ht(e)) || G(t, e));
}
function Un(t) {
  const {
      type: e,
      vnode: r,
      proxy: n,
      withProxy: s,
      propsOptions: [i],
      slots: o,
      attrs: a,
      emit: l,
      render: c,
      renderCache: u,
      props: d,
      data: p,
      setupState: g,
      ctx: v,
      inheritAttrs: _,
    } = t,
    T = Qr(t);
  let N, R;
  try {
    if (r.shapeFlag & 4) {
      const y = s || n,
        D = y;
      (N = qe(c.call(D, y, u, d, g, p, v))), (R = a);
    } else {
      const y = e;
      (N = qe(
        y.length > 1
          ? y(d, {
              attrs: a,
              slots: o,
              emit: l,
            })
          : y(d, null)
      )),
        (R = e.props ? a : Iu(a));
    }
  } catch (y) {
    (fr.length = 0), wr(y, t, 1), (N = oe(Ee));
  }
  let S = N;
  if (R && _ !== !1) {
    const y = Object.keys(R),
      { shapeFlag: D } = S;
    y.length &&
      D & 7 &&
      (i && y.some(Os) && (R = Tu(R, i)), (S = tt(S, R, !1, !0)));
  }
  return (
    r.dirs &&
      ((S = tt(S, null, !1, !0)),
      (S.dirs = S.dirs ? S.dirs.concat(r.dirs) : r.dirs)),
    r.transition && ft(S, r.transition),
    (N = S),
    Qr(T),
    N
  );
}
const Iu = (t) => {
    let e;
    for (const r in t)
      (r === "class" || r === "style" || ln(r)) && ((e || (e = {}))[r] = t[r]);
    return e;
  },
  Tu = (t, e) => {
    const r = {};
    for (const n in t) (!Os(n) || !(n.slice(9) in e)) && (r[n] = t[n]);
    return r;
  };
function Cu(t, e, r) {
  const { props: n, children: s, component: i } = t,
    { props: o, children: a, patchFlag: l } = e,
    c = i.emitsOptions;
  if (e.dirs || e.transition) return !0;
  if (r && l >= 0) {
    if (l & 1024) return !0;
    if (l & 16) return n ? Ai(n, o, c) : !!o;
    if (l & 8) {
      const u = e.dynamicProps;
      for (let d = 0; d < u.length; d++) {
        const p = u[d];
        if (o[p] !== n[p] && !En(c, p)) return !0;
      }
    }
  } else
    return (s || a) && (!a || !a.$stable)
      ? !0
      : n === o
      ? !1
      : n
      ? o
        ? Ai(n, o, c)
        : !0
      : !!o;
  return !1;
}
function Ai(t, e, r) {
  const n = Object.keys(e);
  if (n.length !== Object.keys(t).length) return !0;
  for (let s = 0; s < n.length; s++) {
    const i = n[s];
    if (e[i] !== t[i] && !En(r, i)) return !0;
  }
  return !1;
}
function Ru({ vnode: t, parent: e }, r) {
  for (; e; ) {
    const n = e.subTree;
    if ((n.suspense && n.suspense.activeBranch === t && (n.el = t.el), n === t))
      ((t = e.vnode).el = r), (e = e.parent);
    else break;
  }
}
const tn = (t) => t.__isSuspense;
function Au(t, e) {
  e && e.pendingBranch
    ? L(t)
      ? e.effects.push(...t)
      : e.effects.push(t)
    : Nc(t);
}
const Se = Symbol.for("v-fgt"),
  yn = Symbol.for("v-txt"),
  Ee = Symbol.for("v-cmt"),
  ur = Symbol.for("v-stc"),
  fr = [];
let Ae = null;
function Es(t = !1) {
  fr.push((Ae = t ? null : []));
}
function Ou() {
  fr.pop(), (Ae = fr[fr.length - 1] || null);
}
let xr = 1;
function Oi(t) {
  (xr += t), t < 0 && Ae && (Ae.hasOnce = !0);
}
function Ua(t) {
  return (
    (t.dynamicChildren = xr > 0 ? Ae || Vt : null),
    Ou(),
    xr > 0 && Ae && Ae.push(t),
    t
  );
}
function Pp(t, e, r, n, s, i) {
  return Ua(Ha(t, e, r, n, s, i, !0));
}
function ys(t, e, r, n, s) {
  return Ua(oe(t, e, r, n, s, !0));
}
function Ht(t) {
  return t ? t.__v_isVNode === !0 : !1;
}
function lt(t, e) {
  return t.type === e.type && t.key === e.key;
}
const ja = ({ key: t }) => (t != null ? t : null),
  Br = ({ ref: t, ref_key: e, ref_for: r }) => (
    typeof t == "number" && (t = "" + t),
    t != null
      ? re(t) || fe(t) || $(t)
        ? {
            i: le,
            r: t,
            k: e,
            f: !!r,
          }
        : t
      : null
  );
function Ha(
  t,
  e = null,
  r = null,
  n = 0,
  s = null,
  i = t === Se ? 0 : 1,
  o = !1,
  a = !1
) {
  const l = {
    __v_isVNode: !0,
    __v_skip: !0,
    type: t,
    props: e,
    key: e && ja(e),
    ref: e && Br(e),
    scopeId: ha,
    slotScopeIds: null,
    children: r,
    component: null,
    suspense: null,
    ssContent: null,
    ssFallback: null,
    dirs: null,
    transition: null,
    el: null,
    anchor: null,
    target: null,
    targetStart: null,
    targetAnchor: null,
    staticCount: 0,
    shapeFlag: i,
    patchFlag: n,
    dynamicProps: s,
    dynamicChildren: null,
    appContext: null,
    ctx: le,
  };
  return (
    a
      ? (Qs(l, r), i & 128 && t.normalize(l))
      : r && (l.shapeFlag |= re(r) ? 8 : 16),
    xr > 0 &&
      !o &&
      Ae &&
      (l.patchFlag > 0 || i & 6) &&
      l.patchFlag !== 32 &&
      Ae.push(l),
    l
  );
}
const oe = Pu;
function Pu(t, e = null, r = null, n = 0, s = null, i = !1) {
  if (((!t || t === Sa) && (t = Ee), Ht(t))) {
    const a = tt(t, e, !0);
    return (
      r && Qs(a, r),
      xr > 0 &&
        !i &&
        Ae &&
        (a.shapeFlag & 6 ? (Ae[Ae.indexOf(t)] = a) : Ae.push(a)),
      (a.patchFlag = -2),
      a
    );
  }
  if ((ju(t) && (t = t.__vccOpts), e)) {
    e = Mu(e);
    let { class: a, style: l } = e;
    a && !re(a) && (e.class = Ns(a)),
      J(l) && (Us(l) && !L(l) && (l = ie({}, l)), (e.style = Ls(l)));
  }
  const o = re(t) ? 1 : tn(t) ? 128 : ga(t) ? 64 : J(t) ? 4 : $(t) ? 2 : 0;
  return Ha(t, e, r, n, s, o, i, !0);
}
function Mu(t) {
  return t ? (Us(t) || Oa(t) ? ie({}, t) : t) : null;
}
function tt(t, e, r = !1, n = !1) {
  const { props: s, ref: i, patchFlag: o, children: a, transition: l } = t,
    c = e ? Nu(s || {}, e) : s,
    u = {
      __v_isVNode: !0,
      __v_skip: !0,
      type: t.type,
      props: c,
      key: c && ja(c),
      ref:
        e && e.ref
          ? r && i
            ? L(i)
              ? i.concat(Br(e))
              : [i, Br(e)]
            : Br(e)
          : i,
      scopeId: t.scopeId,
      slotScopeIds: t.slotScopeIds,
      children: a,
      target: t.target,
      targetStart: t.targetStart,
      targetAnchor: t.targetAnchor,
      staticCount: t.staticCount,
      shapeFlag: t.shapeFlag,
      patchFlag: e && t.type !== Se ? (o === -1 ? 16 : o | 16) : o,
      dynamicProps: t.dynamicProps,
      dynamicChildren: t.dynamicChildren,
      appContext: t.appContext,
      dirs: t.dirs,
      transition: l,
      component: t.component,
      suspense: t.suspense,
      ssContent: t.ssContent && tt(t.ssContent),
      ssFallback: t.ssFallback && tt(t.ssFallback),
      el: t.el,
      anchor: t.anchor,
      ctx: t.ctx,
      ce: t.ce,
    };
  return l && n && ft(u, l.clone(u)), u;
}
function Lu(t = " ", e = 0) {
  return oe(yn, null, t, e);
}
function Mp(t, e) {
  const r = oe(ur, null, t);
  return (r.staticCount = e), r;
}
function Lp(t = "", e = !1) {
  return e ? (Es(), ys(Ee, null, t)) : oe(Ee, null, t);
}
function qe(t) {
  return t == null || typeof t == "boolean"
    ? oe(Ee)
    : L(t)
    ? oe(Se, null, t.slice())
    : Ht(t)
    ? at(t)
    : oe(yn, null, String(t));
}
function at(t) {
  return (t.el === null && t.patchFlag !== -1) || t.memo ? t : tt(t);
}
function Qs(t, e) {
  let r = 0;
  const { shapeFlag: n } = t;
  if (e == null) e = null;
  else if (L(e)) r = 16;
  else if (typeof e == "object")
    if (n & 65) {
      const s = e.default;
      s && (s._c && (s._d = !1), Qs(t, s()), s._c && (s._d = !0));
      return;
    } else {
      r = 32;
      const s = e._;
      !s && !Oa(e)
        ? (e._ctx = le)
        : s === 3 &&
          le &&
          (le.slots._ === 1 ? (e._ = 1) : ((e._ = 2), (t.patchFlag |= 1024)));
    }
  else
    $(e)
      ? ((e = {
          default: e,
          _ctx: le,
        }),
        (r = 32))
      : ((e = String(e)), n & 64 ? ((r = 16), (e = [Lu(e)])) : (r = 8));
  (t.children = e), (t.shapeFlag |= r);
}
function Nu(...t) {
  const e = {};
  for (let r = 0; r < t.length; r++) {
    const n = t[r];
    for (const s in n)
      if (s === "class")
        e.class !== n.class && (e.class = Ns([e.class, n.class]));
      else if (s === "style") e.style = Ls([e.style, n.style]);
      else if (ln(s)) {
        const i = e[s],
          o = n[s];
        o &&
          i !== o &&
          !(L(i) && i.includes(o)) &&
          (e[s] = i ? [].concat(i, o) : o);
      } else s !== "" && (e[s] = n[s]);
  }
  return e;
}
function Le(t, e, r, n = null) {
  Fe(t, e, 7, [r, n]);
}
const ku = Ca();
let Vu = 0;
function Du(t, e, r) {
  const n = t.type,
    s = (e ? e.appContext : t.appContext) || ku,
    i = {
      uid: Vu++,
      vnode: t,
      type: n,
      parent: e,
      appContext: s,
      root: null,
      next: null,
      subTree: null,
      effect: null,
      update: null,
      job: null,
      scope: new Bo(!0),
      render: null,
      proxy: null,
      exposed: null,
      exposeProxy: null,
      withProxy: null,
      provides: e ? e.provides : Object.create(s.provides),
      ids: e ? e.ids : ["", 0, 0],
      accessCache: null,
      renderCache: [],
      components: null,
      directives: null,
      propsOptions: Ma(n, s),
      emitsOptions: Fa(n, s),
      emit: null,
      emitted: null,
      propsDefaults: Y,
      inheritAttrs: n.inheritAttrs,
      ctx: Y,
      data: Y,
      props: Y,
      attrs: Y,
      slots: Y,
      refs: Y,
      setupState: Y,
      setupContext: null,
      suspense: r,
      suspenseId: r ? r.pendingId : 0,
      asyncDep: null,
      asyncResolved: !1,
      isMounted: !1,
      isUnmounted: !1,
      isDeactivated: !1,
      bc: null,
      c: null,
      bm: null,
      m: null,
      bu: null,
      u: null,
      um: null,
      bum: null,
      da: null,
      a: null,
      rtg: null,
      rtc: null,
      ec: null,
      sp: null,
    };
  return (
    (i.ctx = {
      _: i,
    }),
    (i.root = e ? e.root : i),
    (i.emit = Su.bind(null, i)),
    t.ce && t.ce(i),
    i
  );
}
let ae = null;
const Sr = () => ae || le;
let rn, bs;
{
  const t = dn(),
    e = (r, n) => {
      let s;
      return (
        (s = t[r]) || (s = t[r] = []),
        s.push(n),
        (i) => {
          s.length > 1 ? s.forEach((o) => o(i)) : s[0](i);
        }
      );
    };
  (rn = e("__VUE_INSTANCE_SETTERS__", (r) => (ae = r))),
    (bs = e("__VUE_SSR_SETTERS__", (r) => (Bt = r)));
}
const Ir = (t) => {
    const e = ae;
    return (
      rn(t),
      t.scope.on(),
      () => {
        t.scope.off(), rn(e);
      }
    );
  },
  Pi = () => {
    ae && ae.scope.off(), rn(null);
  };
function Ba(t) {
  return t.vnode.shapeFlag & 4;
}
let Bt = !1;
function $u(t, e = !1, r = !1) {
  e && bs(e);
  const { props: n, children: s } = t.vnode,
    i = Ba(t);
  cu(t, n, i, e), hu(t, s, r);
  const o = i ? Fu(t, e) : void 0;
  return e && bs(!1), o;
}
function Fu(t, e) {
  const r = t.type;
  (t.accessCache = Object.create(null)), (t.proxy = new Proxy(t.ctx, Zc));
  const { setup: n } = r;
  if (n) {
    pt();
    const s = (t.setupContext = n.length > 1 ? Ka(t) : null),
      i = Ir(t),
      o = br(n, t, 0, [t.props, s]),
      a = ko(o);
    if ((gt(), i(), (a || t.sp) && !It(t) && Bs(t), a)) {
      if ((o.then(Pi, Pi), e))
        return o
          .then((l) => {
            Mi(t, l, e);
          })
          .catch((l) => {
            wr(l, t, 0);
          });
      t.asyncDep = o;
    } else Mi(t, o, e);
  } else qa(t, e);
}
function Mi(t, e, r) {
  $(e)
    ? t.type.__ssrInlineRender
      ? (t.ssrRender = e)
      : (t.render = e)
    : J(e) && (t.setupState = aa(e)),
    qa(t, r);
}
let Li;
function qa(t, e, r) {
  const n = t.type;
  if (!t.render) {
    if (!e && Li && !n.render) {
      const s = n.template || Js(t).template;
      if (s) {
        const { isCustomElement: i, compilerOptions: o } = t.appContext.config,
          { delimiters: a, compilerOptions: l } = n,
          c = ie(
            ie(
              {
                isCustomElement: i,
                delimiters: a,
              },
              o
            ),
            l
          );
        n.render = Li(s, c);
      }
    }
    t.render = n.render || Ke;
  }
  {
    const s = Ir(t);
    pt();
    try {
      tu(t);
    } finally {
      gt(), s();
    }
  }
}
const Uu = {
  get(t, e) {
    return ve(t, "get", ""), t[e];
  },
};
function Ka(t) {
  const e = (r) => {
    t.exposed = r || {};
  };
  return {
    attrs: new Proxy(t.attrs, Uu),
    slots: t.slots,
    emit: t.emit,
    expose: e,
  };
}
function bn(t) {
  return t.exposed
    ? t.exposeProxy ||
        (t.exposeProxy = new Proxy(aa(bc(t.exposed)), {
          get(e, r) {
            if (r in e) return e[r];
            if (r in cr) return cr[r](t);
          },
          has(e, r) {
            return r in e || r in cr;
          },
        }))
    : t.proxy;
}
function ws(t, e = !0) {
  return $(t) ? t.displayName || t.name : t.name || (e && t.__name);
}
function ju(t) {
  return $(t) && "__vccOpts" in t;
}
const Hu = (t, e) => Rc(t, e, Bt);
function Bu(t, e, r) {
  const n = arguments.length;
  return n === 2
    ? J(e) && !L(e)
      ? Ht(e)
        ? oe(t, null, [e])
        : oe(t, e)
      : oe(t, null, e)
    : (n > 3
        ? (r = Array.prototype.slice.call(arguments, 2))
        : n === 3 && Ht(r) && (r = [r]),
      oe(t, e, r));
}
const qu = "3.5.12";
/**
 * @vue/runtime-dom v3.5.12
 * (c) 2018-present Yuxi (Evan) You and Vue contributors
 * @license MIT
 **/
let _s;
const Ni = typeof window < "u" && window.trustedTypes;
if (Ni)
  try {
    _s = Ni.createPolicy("vue", {
      createHTML: (t) => t,
    });
  } catch (t) {}
const Wa = _s ? (t) => _s.createHTML(t) : (t) => t,
  Ku = "http://www.w3.org/2000/svg",
  Wu = "http://www.w3.org/1998/Math/MathML",
  Xe = typeof document < "u" ? document : null,
  ki = Xe && Xe.createElement("template"),
  Gu = {
    insert: (t, e, r) => {
      e.insertBefore(t, r || null);
    },
    remove: (t) => {
      const e = t.parentNode;
      e && e.removeChild(t);
    },
    createElement: (t, e, r, n) => {
      const s =
        e === "svg"
          ? Xe.createElementNS(Ku, t)
          : e === "mathml"
          ? Xe.createElementNS(Wu, t)
          : r
          ? Xe.createElement(t, {
              is: r,
            })
          : Xe.createElement(t);
      return (
        t === "select" &&
          n &&
          n.multiple != null &&
          s.setAttribute("multiple", n.multiple),
        s
      );
    },
    createText: (t) => Xe.createTextNode(t),
    createComment: (t) => Xe.createComment(t),
    setText: (t, e) => {
      t.nodeValue = e;
    },
    setElementText: (t, e) => {
      t.textContent = e;
    },
    parentNode: (t) => t.parentNode,
    nextSibling: (t) => t.nextSibling,
    querySelector: (t) => Xe.querySelector(t),
    setScopeId(t, e) {
      t.setAttribute(e, "");
    },
    insertStaticContent(t, e, r, n, s, i) {
      const o = r ? r.previousSibling : e.lastChild;
      if (s && (s === i || s.nextSibling))
        for (
          ;
          e.insertBefore(s.cloneNode(!0), r),
            !(s === i || !(s = s.nextSibling));

        );
      else {
        ki.innerHTML = Wa(
          n === "svg"
            ? "<svg>".concat(t, "</svg>")
            : n === "mathml"
            ? "<math>".concat(t, "</math>")
            : t
        );
        const a = ki.content;
        if (n === "svg" || n === "mathml") {
          const l = a.firstChild;
          for (; l.firstChild; ) a.appendChild(l.firstChild);
          a.removeChild(l);
        }
        e.insertBefore(a, r);
      }
      return [
        o ? o.nextSibling : e.firstChild,
        r ? r.previousSibling : e.lastChild,
      ];
    },
  },
  nt = "transition",
  tr = "animation",
  qt = Symbol("_vtc"),
  Ga = {
    name: String,
    type: String,
    css: {
      type: Boolean,
      default: !0,
    },
    duration: [String, Number, Object],
    enterFromClass: String,
    enterActiveClass: String,
    enterToClass: String,
    appearFromClass: String,
    appearActiveClass: String,
    appearToClass: String,
    leaveFromClass: String,
    leaveActiveClass: String,
    leaveToClass: String,
  },
  za = ie({}, xa, Ga),
  zu = (t) => ((t.displayName = "Transition"), (t.props = za), t),
  Np = zu((t, { slots: e }) => Bu(Uc, Ja(t), e)),
  wt = (t, e = []) => {
    L(t) ? t.forEach((r) => r(...e)) : t && t(...e);
  },
  Vi = (t) => (t ? (L(t) ? t.some((e) => e.length > 1) : t.length > 1) : !1);
function Ja(t) {
  const e = {};
  for (const P in t) P in Ga || (e[P] = t[P]);
  if (t.css === !1) return e;
  const {
      name: r = "v",
      type: n,
      duration: s,
      enterFromClass: i = "".concat(r, "-enter-from"),
      enterActiveClass: o = "".concat(r, "-enter-active"),
      enterToClass: a = "".concat(r, "-enter-to"),
      appearFromClass: l = i,
      appearActiveClass: c = o,
      appearToClass: u = a,
      leaveFromClass: d = "".concat(r, "-leave-from"),
      leaveActiveClass: p = "".concat(r, "-leave-active"),
      leaveToClass: g = "".concat(r, "-leave-to"),
    } = t,
    v = Ju(s),
    _ = v && v[0],
    T = v && v[1],
    {
      onBeforeEnter: N,
      onEnter: R,
      onEnterCancelled: S,
      onLeave: y,
      onLeaveCancelled: D,
      onBeforeAppear: U = N,
      onAppear: j = R,
      onAppearCancelled: B = S,
    } = e,
    k = (P, ee, pe) => {
      st(P, ee ? u : a), st(P, ee ? c : o), pe && pe();
    },
    H = (P, ee) => {
      (P._isLeaving = !1), st(P, d), st(P, g), st(P, p), ee && ee();
    },
    W = (P) => (ee, pe) => {
      const vt = P ? j : R,
        ce = () => k(ee, P, pe);
      wt(vt, [ee, ce]),
        Di(() => {
          st(ee, P ? l : i), Je(ee, P ? u : a), Vi(vt) || $i(ee, n, _, ce);
        });
    };
  return ie(e, {
    onBeforeEnter(P) {
      wt(N, [P]), Je(P, i), Je(P, o);
    },
    onBeforeAppear(P) {
      wt(U, [P]), Je(P, l), Je(P, c);
    },
    onEnter: W(!1),
    onAppear: W(!0),
    onLeave(P, ee) {
      P._isLeaving = !0;
      const pe = () => H(P, ee);
      Je(P, d),
        Je(P, p),
        Xa(),
        Di(() => {
          P._isLeaving && (st(P, d), Je(P, g), Vi(y) || $i(P, n, T, pe));
        }),
        wt(y, [P, pe]);
    },
    onEnterCancelled(P) {
      k(P, !1), wt(S, [P]);
    },
    onAppearCancelled(P) {
      k(P, !0), wt(B, [P]);
    },
    onLeaveCancelled(P) {
      H(P), wt(D, [P]);
    },
  });
}
function Ju(t) {
  if (t == null) return null;
  if (J(t)) return [jn(t.enter), jn(t.leave)];
  {
    const e = jn(t);
    return [e, e];
  }
}
function jn(t) {
  return Kl(t);
}
function Je(t, e) {
  e.split(/\s+/).forEach((r) => r && t.classList.add(r)),
    (t[qt] || (t[qt] = new Set())).add(e);
}
function st(t, e) {
  e.split(/\s+/).forEach((n) => n && t.classList.remove(n));
  const r = t[qt];
  r && (r.delete(e), r.size || (t[qt] = void 0));
}
function Di(t) {
  requestAnimationFrame(() => {
    requestAnimationFrame(t);
  });
}
let Yu = 0;
function $i(t, e, r, n) {
  const s = (t._endId = ++Yu),
    i = () => {
      s === t._endId && n();
    };
  if (r != null) return setTimeout(i, r);
  const { type: o, timeout: a, propCount: l } = Ya(t, e);
  if (!o) return n();
  const c = o + "end";
  let u = 0;
  const d = () => {
      t.removeEventListener(c, p), i();
    },
    p = (g) => {
      g.target === t && ++u >= l && d();
    };
  setTimeout(() => {
    u < l && d();
  }, a + 1),
    t.addEventListener(c, p);
}
function Ya(t, e) {
  const r = window.getComputedStyle(t),
    n = (v) => (r[v] || "").split(", "),
    s = n("".concat(nt, "Delay")),
    i = n("".concat(nt, "Duration")),
    o = Fi(s, i),
    a = n("".concat(tr, "Delay")),
    l = n("".concat(tr, "Duration")),
    c = Fi(a, l);
  let u = null,
    d = 0,
    p = 0;
  e === nt
    ? o > 0 && ((u = nt), (d = o), (p = i.length))
    : e === tr
    ? c > 0 && ((u = tr), (d = c), (p = l.length))
    : ((d = Math.max(o, c)),
      (u = d > 0 ? (o > c ? nt : tr) : null),
      (p = u ? (u === nt ? i.length : l.length) : 0));
  const g =
    u === nt &&
    /\b(transform|all)(,|$)/.test(n("".concat(nt, "Property")).toString());
  return {
    type: u,
    timeout: d,
    propCount: p,
    hasTransform: g,
  };
}
function Fi(t, e) {
  for (; t.length < e.length; ) t = t.concat(t);
  return Math.max(...e.map((r, n) => Ui(r) + Ui(t[n])));
}
function Ui(t) {
  return t === "auto" ? 0 : Number(t.slice(0, -1).replace(",", ".")) * 1e3;
}
function Xa() {
  return document.body.offsetHeight;
}
function Xu(t, e, r) {
  const n = t[qt];
  n && (e = (e ? [e, ...n] : [...n]).join(" ")),
    e == null
      ? t.removeAttribute("class")
      : r
      ? t.setAttribute("class", e)
      : (t.className = e);
}
const nn = Symbol("_vod"),
  Qa = Symbol("_vsh"),
  kp = {
    beforeMount(t, { value: e }, { transition: r }) {
      (t[nn] = t.style.display === "none" ? "" : t.style.display),
        r && e ? r.beforeEnter(t) : rr(t, e);
    },
    mounted(t, { value: e }, { transition: r }) {
      r && e && r.enter(t);
    },
    updated(t, { value: e, oldValue: r }, { transition: n }) {
      !e != !r &&
        (n
          ? e
            ? (n.beforeEnter(t), rr(t, !0), n.enter(t))
            : n.leave(t, () => {
                rr(t, !1);
              })
          : rr(t, e));
    },
    beforeUnmount(t, { value: e }) {
      rr(t, e);
    },
  };
function rr(t, e) {
  (t.style.display = e ? t[nn] : "none"), (t[Qa] = !e);
}
const Za = Symbol("");
function Vp(t) {
  const e = Sr();
  if (!e) return;
  const r = (e.ut = (s = t(e.proxy)) => {
      Array.from(
        document.querySelectorAll('[data-v-owner="'.concat(e.uid, '"]'))
      ).forEach((i) => sn(i, s));
    }),
    n = () => {
      const s = t(e.proxy);
      e.ce ? sn(e.ce, s) : Ss(e.subTree, s), r(s);
    };
  _a(() => {
    bu(n);
  }),
    vn(() => {
      const s = new MutationObserver(n);
      s.observe(e.subTree.el.parentNode, {
        childList: !0,
      }),
        Ws(() => s.disconnect());
    });
}
function Ss(t, e) {
  if (t.shapeFlag & 128) {
    const r = t.suspense;
    (t = r.activeBranch),
      r.pendingBranch &&
        !r.isHydrating &&
        r.effects.push(() => {
          Ss(r.activeBranch, e);
        });
  }
  for (; t.component; ) t = t.component.subTree;
  if (t.shapeFlag & 1 && t.el) sn(t.el, e);
  else if (t.type === Se) t.children.forEach((r) => Ss(r, e));
  else if (t.type === ur) {
    let { el: r, anchor: n } = t;
    for (; r && (sn(r, e), r !== n); ) r = r.nextSibling;
  }
}
function sn(t, e) {
  if (t.nodeType === 1) {
    const r = t.style;
    let n = "";
    for (const s in e)
      r.setProperty("--".concat(s), e[s]),
        (n += "--".concat(s, ": ").concat(e[s], ";"));
    r[Za] = n;
  }
}
const Qu = /(^|;)\s*display\s*:/;
function Zu(t, e, r) {
  const n = t.style,
    s = re(r);
  let i = !1;
  if (r && !s) {
    if (e)
      if (re(e))
        for (const o of e.split(";")) {
          const a = o.slice(0, o.indexOf(":")).trim();
          r[a] == null && qr(n, a, "");
        }
      else for (const o in e) r[o] == null && qr(n, o, "");
    for (const o in r) o === "display" && (i = !0), qr(n, o, r[o]);
  } else if (s) {
    if (e !== r) {
      const o = n[Za];
      o && (r += ";" + o), (n.cssText = r), (i = Qu.test(r));
    }
  } else e && t.removeAttribute("style");
  nn in t && ((t[nn] = i ? n.display : ""), t[Qa] && (n.display = "none"));
}
const ji = /\s*!important$/;
function qr(t, e, r) {
  if (L(r)) r.forEach((n) => qr(t, e, n));
  else if ((r == null && (r = ""), e.startsWith("--"))) t.setProperty(e, r);
  else {
    const n = ef(t, e);
    ji.test(r)
      ? t.setProperty(ht(n), r.replace(ji, ""), "important")
      : (t[n] = r);
  }
}
const Hi = ["Webkit", "Moz", "ms"],
  Hn = {};
function ef(t, e) {
  const r = Hn[e];
  if (r) return r;
  let n = ke(e);
  if (n !== "filter" && n in t) return (Hn[e] = n);
  n = fn(n);
  for (let s = 0; s < Hi.length; s++) {
    const i = Hi[s] + n;
    if (i in t) return (Hn[e] = i);
  }
  return e;
}
const Bi = "http://www.w3.org/1999/xlink";
function qi(t, e, r, n, s, i = Xl(e)) {
  n && e.startsWith("xlink:")
    ? r == null
      ? t.removeAttributeNS(Bi, e.slice(6, e.length))
      : t.setAttributeNS(Bi, e, r)
    : r == null || (i && !Fo(r))
    ? t.removeAttribute(e)
    : t.setAttribute(e, i ? "" : $e(r) ? String(r) : r);
}
function Ki(t, e, r, n, s) {
  if (e === "innerHTML" || e === "textContent") {
    r != null && (t[e] = e === "innerHTML" ? Wa(r) : r);
    return;
  }
  const i = t.tagName;
  if (e === "value" && i !== "PROGRESS" && !i.includes("-")) {
    const a = i === "OPTION" ? t.getAttribute("value") || "" : t.value,
      l = r == null ? (t.type === "checkbox" ? "on" : "") : String(r);
    (a !== l || !("_value" in t)) && (t.value = l),
      r == null && t.removeAttribute(e),
      (t._value = r);
    return;
  }
  let o = !1;
  if (r === "" || r == null) {
    const a = typeof t[e];
    a === "boolean"
      ? (r = Fo(r))
      : r == null && a === "string"
      ? ((r = ""), (o = !0))
      : a === "number" && ((r = 0), (o = !0));
  }
  try {
    t[e] = r;
  } catch (a) {}
  o && t.removeAttribute(s || e);
}
function ct(t, e, r, n) {
  t.addEventListener(e, r, n);
}
function tf(t, e, r, n) {
  t.removeEventListener(e, r, n);
}
const Wi = Symbol("_vei");
function rf(t, e, r, n, s = null) {
  const i = t[Wi] || (t[Wi] = {}),
    o = i[e];
  if (n && o) o.value = n;
  else {
    const [a, l] = nf(e);
    if (n) {
      const c = (i[e] = af(n, s));
      ct(t, a, c, l);
    } else o && (tf(t, a, o, l), (i[e] = void 0));
  }
}
const Gi = /(?:Once|Passive|Capture)$/;
function nf(t) {
  let e;
  if (Gi.test(t)) {
    e = {};
    let n;
    for (; (n = t.match(Gi)); )
      (t = t.slice(0, t.length - n[0].length)), (e[n[0].toLowerCase()] = !0);
  }
  return [t[2] === ":" ? t.slice(3) : ht(t.slice(2)), e];
}
let Bn = 0;
const sf = Promise.resolve(),
  of = () => Bn || (sf.then(() => (Bn = 0)), (Bn = Date.now()));
function af(t, e) {
  const r = (n) => {
    if (!n._vts) n._vts = Date.now();
    else if (n._vts <= r.attached) return;
    Fe(lf(n, r.value), e, 5, [n]);
  };
  return (r.value = t), (r.attached = of()), r;
}
function lf(t, e) {
  if (L(e)) {
    const r = t.stopImmediatePropagation;
    return (
      (t.stopImmediatePropagation = () => {
        r.call(t), (t._stopped = !0);
      }),
      e.map((n) => (s) => !s._stopped && n && n(s))
    );
  } else return e;
}
const zi = (t) =>
    t.charCodeAt(0) === 111 &&
    t.charCodeAt(1) === 110 &&
    t.charCodeAt(2) > 96 &&
    t.charCodeAt(2) < 123,
  cf = (t, e, r, n, s, i) => {
    const o = s === "svg";
    e === "class"
      ? Xu(t, n, o)
      : e === "style"
      ? Zu(t, r, n)
      : ln(e)
      ? Os(e) || rf(t, e, r, n, i)
      : (
          e[0] === "."
            ? ((e = e.slice(1)), !0)
            : e[0] === "^"
            ? ((e = e.slice(1)), !1)
            : uf(t, e, n, o)
        )
      ? (Ki(t, e, n),
        !t.tagName.includes("-") &&
          (e === "value" || e === "checked" || e === "selected") &&
          qi(t, e, n, o, i, e !== "value"))
      : t._isVueCE && (/[A-Z]/.test(e) || !re(n))
      ? Ki(t, ke(e), n, i, e)
      : (e === "true-value"
          ? (t._trueValue = n)
          : e === "false-value" && (t._falseValue = n),
        qi(t, e, n, o));
  };
function uf(t, e, r, n) {
  if (n)
    return !!(
      e === "innerHTML" ||
      e === "textContent" ||
      (e in t && zi(e) && $(r))
    );
  if (
    e === "spellcheck" ||
    e === "draggable" ||
    e === "translate" ||
    e === "form" ||
    (e === "list" && t.tagName === "INPUT") ||
    (e === "type" && t.tagName === "TEXTAREA")
  )
    return !1;
  if (e === "width" || e === "height") {
    const s = t.tagName;
    if (s === "IMG" || s === "VIDEO" || s === "CANVAS" || s === "SOURCE")
      return !1;
  }
  return zi(e) && re(r) ? !1 : e in t;
}
const el = new WeakMap(),
  tl = new WeakMap(),
  on = Symbol("_moveCb"),
  Ji = Symbol("_enterCb"),
  ff = (t) => (delete t.props.mode, t),
  df = ff({
    name: "TransitionGroup",
    props: ie({}, za, {
      tag: String,
      moveClass: String,
    }),
    setup(t, { slots: e }) {
      const r = Sr(),
        n = va();
      let s, i;
      return (
        qs(() => {
          if (!s.length) return;
          const o = t.moveClass || "".concat(t.name || "v", "-move");
          if (!mf(s[0].el, r.vnode.el, o)) return;
          s.forEach(hf), s.forEach(pf);
          const a = s.filter(gf);
          Xa(),
            a.forEach((l) => {
              const c = l.el,
                u = c.style;
              Je(c, o),
                (u.transform = u.webkitTransform = u.transitionDuration = "");
              const d = (c[on] = (p) => {
                (p && p.target !== c) ||
                  ((!p || /transform$/.test(p.propertyName)) &&
                    (c.removeEventListener("transitionend", d),
                    (c[on] = null),
                    st(c, o)));
              });
              c.addEventListener("transitionend", d);
            });
        }),
        () => {
          const o = K(t),
            a = Ja(o);
          let l = o.tag || Se;
          if (((s = []), i))
            for (let c = 0; c < i.length; c++) {
              const u = i[c];
              u.el &&
                u.el instanceof Element &&
                (s.push(u),
                ft(u, vr(u, a, n, r)),
                el.set(u, u.el.getBoundingClientRect()));
            }
          i = e.default ? Hs(e.default()) : [];
          for (let c = 0; c < i.length; c++) {
            const u = i[c];
            u.key != null && ft(u, vr(u, a, n, r));
          }
          return oe(l, null, i);
        }
      );
    },
  }),
  Dp = df;
function hf(t) {
  const e = t.el;
  e[on] && e[on](), e[Ji] && e[Ji]();
}
function pf(t) {
  tl.set(t, t.el.getBoundingClientRect());
}
function gf(t) {
  const e = el.get(t),
    r = tl.get(t),
    n = e.left - r.left,
    s = e.top - r.top;
  if (n || s) {
    const i = t.el.style;
    return (
      (i.transform = i.webkitTransform =
        "translate(".concat(n, "px,").concat(s, "px)")),
      (i.transitionDuration = "0s"),
      t
    );
  }
}
function mf(t, e, r) {
  const n = t.cloneNode(),
    s = t[qt];
  s &&
    s.forEach((a) => {
      a.split(/\s+/).forEach((l) => l && n.classList.remove(l));
    }),
    r.split(/\s+/).forEach((a) => a && n.classList.add(a)),
    (n.style.display = "none");
  const i = e.nodeType === 1 ? e : e.parentNode;
  i.appendChild(n);
  const { hasTransform: o } = Ya(n);
  return i.removeChild(n), o;
}
const Kt = (t) => {
  const e = t.props["onUpdate:modelValue"] || !1;
  return L(e) ? (r) => $t(e, r) : e;
};
function vf(t) {
  t.target.composing = !0;
}
function Yi(t) {
  const e = t.target;
  e.composing && ((e.composing = !1), e.dispatchEvent(new Event("input")));
}
const et = Symbol("_assign"),
  $p = {
    created(t, { modifiers: { lazy: e, trim: r, number: n } }, s) {
      t[et] = Kt(s);
      const i = n || (s.props && s.props.type === "number");
      ct(t, e ? "change" : "input", (o) => {
        if (o.target.composing) return;
        let a = t.value;
        r && (a = a.trim()), i && (a = ls(a)), t[et](a);
      }),
        r &&
          ct(t, "change", () => {
            t.value = t.value.trim();
          }),
        e ||
          (ct(t, "compositionstart", vf),
          ct(t, "compositionend", Yi),
          ct(t, "change", Yi));
    },
    mounted(t, { value: e }) {
      t.value = e == null ? "" : e;
    },
    beforeUpdate(
      t,
      { value: e, oldValue: r, modifiers: { lazy: n, trim: s, number: i } },
      o
    ) {
      if (((t[et] = Kt(o)), t.composing)) return;
      const a =
          (i || t.type === "number") && !/^0\d/.test(t.value)
            ? ls(t.value)
            : t.value,
        l = e == null ? "" : e;
      a !== l &&
        ((document.activeElement === t &&
          t.type !== "range" &&
          ((n && e === r) || (s && t.value.trim() === l))) ||
          (t.value = l));
    },
  },
  Fp = {
    deep: !0,
    created(t, e, r) {
      (t[et] = Kt(r)),
        ct(t, "change", () => {
          const n = t._modelValue,
            s = rl(t),
            i = t.checked,
            o = t[et];
          if (L(n)) {
            const a = Uo(n, s),
              l = a !== -1;
            if (i && !l) o(n.concat(s));
            else if (!i && l) {
              const c = [...n];
              c.splice(a, 1), o(c);
            }
          } else if (cn(n)) {
            const a = new Set(n);
            i ? a.add(s) : a.delete(s), o(a);
          } else o(nl(t, i));
        });
    },
    mounted: Xi,
    beforeUpdate(t, e, r) {
      (t[et] = Kt(r)), Xi(t, e, r);
    },
  };
function Xi(t, { value: e, oldValue: r }, n) {
  t._modelValue = e;
  let s;
  if (L(e)) s = Uo(e, n.props.value) > -1;
  else if (cn(e)) s = e.has(n.props.value);
  else {
    if (e === r) return;
    s = jt(e, nl(t, !0));
  }
  t.checked !== s && (t.checked = s);
}
const Up = {
  created(t, { value: e }, r) {
    (t.checked = jt(e, r.props.value)),
      (t[et] = Kt(r)),
      ct(t, "change", () => {
        t[et](rl(t));
      });
  },
  beforeUpdate(t, { value: e, oldValue: r }, n) {
    (t[et] = Kt(n)), e !== r && (t.checked = jt(e, n.props.value));
  },
};
function rl(t) {
  return "_value" in t ? t._value : t.value;
}
function nl(t, e) {
  const r = e ? "_trueValue" : "_falseValue";
  return r in t ? t[r] : e;
}
const xf = ["ctrl", "shift", "alt", "meta"],
  Ef = {
    stop: (t) => t.stopPropagation(),
    prevent: (t) => t.preventDefault(),
    self: (t) => t.target !== t.currentTarget,
    ctrl: (t) => !t.ctrlKey,
    shift: (t) => !t.shiftKey,
    alt: (t) => !t.altKey,
    meta: (t) => !t.metaKey,
    left: (t) => "button" in t && t.button !== 0,
    middle: (t) => "button" in t && t.button !== 1,
    right: (t) => "button" in t && t.button !== 2,
    exact: (t, e) => xf.some((r) => t["".concat(r, "Key")] && !e.includes(r)),
  },
  jp = (t, e) => {
    const r = t._withMods || (t._withMods = {}),
      n = e.join(".");
    return (
      r[n] ||
      (r[n] = (s, ...i) => {
        for (let o = 0; o < e.length; o++) {
          const a = Ef[e[o]];
          if (a && a(s, e)) return;
        }
        return t(s, ...i);
      })
    );
  },
  yf = {
    esc: "escape",
    space: " ",
    up: "arrow-up",
    left: "arrow-left",
    right: "arrow-right",
    down: "arrow-down",
    delete: "backspace",
  },
  Hp = (t, e) => {
    const r = t._withKeys || (t._withKeys = {}),
      n = e.join(".");
    return (
      r[n] ||
      (r[n] = (s) => {
        if (!("key" in s)) return;
        const i = ht(s.key);
        if (e.some((o) => o === i || yf[o] === i)) return t(s);
      })
    );
  },
  bf = ie(
    {
      patchProp: cf,
    },
    Gu
  );
let Qi;
function sl() {
  return Qi || (Qi = gu(bf));
}
const Bp = (...t) => {
    sl().render(...t);
  },
  qp = (...t) => {
    const e = sl().createApp(...t),
      { mount: r } = e;
    return (
      (e.mount = (n) => {
        const s = _f(n);
        if (!s) return;
        const i = e._component;
        !$(i) && !i.render && !i.template && (i.template = s.innerHTML),
          s.nodeType === 1 && (s.textContent = "");
        const o = r(s, !1, wf(s));
        return (
          s instanceof Element &&
            (s.removeAttribute("v-cloak"), s.setAttribute("data-v-app", "")),
          o
        );
      }),
      e
    );
  };
function wf(t) {
  if (t instanceof SVGElement) return "svg";
  if (typeof MathMLElement == "function" && t instanceof MathMLElement)
    return "mathml";
}
function _f(t) {
  return re(t) ? document.querySelector(t) : t;
}
class Is {
  constructor(e) {
    (this.onCatch = null), e && this.bindOnCatch(e);
  }
  bindOnCatch(e) {
    this.onCatch && this.unbindOnCatch(), (this.onCatch = e);
  }
  unbindOnCatch() {
    this.onCatch = null;
  }
  catch(e) {
    !this.onCatch || !e || this.onCatch(e);
  }
}
class il {}
class mt {
  constructor(e, r) {
    (this.info = e), (this.error = r);
  }
}
var dt;
(function (t) {
  (t.Fatal = "fatal"),
    (t.Danger = "danger"),
    (t.Default = "default"),
    (t.Minor = "minor");
})(dt || (dt = {}));
class At extends Error {
  constructor(e, r = {}) {
    super(e);
    const { cause: n, level: s = dt.Default, origin: i } = r;
    (this.name = "KnownError"),
      (this.cause = n),
      (this.level = s),
      (this.origin = i),
      (this.timestamp = Date.now()),
      (this.stack = n == null ? void 0 : n.stack);
  }
  toString() {
    var e, r, n;
    return !((e = this.stack) === null || e === void 0) &&
      e.startsWith(
        (r = this.cause) === null || r === void 0 ? void 0 : r.name
      ) &&
      !((n = this.cause) === null || n === void 0) &&
      n.name
      ? this.stack.replace(this.cause.name, this.name)
      : this.stack
      ? this.stack
      : "".concat(this.name, ": ").concat(this.message);
  }
  get merlin() {}
}
class wn {}
class Sf {
  constructor() {
    (this.catchers = []), (this.transformers = []), (this.consumers = []);
  }
}
function If(...t) {}
const ol = Object.prototype.toString;
function dr(t) {
  switch (ol.call(t)) {
    case "[object Error]":
    case "[object Exception]":
    case "[object DOMException]":
      return !0;
    default:
      return _n(t, Error);
  }
}
function Ot(t, e) {
  return ol.call(t) === "[object ".concat(e, "]");
}
function al(t) {
  return Ot(t, "ErrorEvent");
}
function Ts(t) {
  return Ot(t, "PromiseRejectionEvent");
}
function Tf(t) {
  return Ot(t, "DOMError");
}
function Cf(t) {
  return Ot(t, "DOMException");
}
function Rf(t) {
  return Ot(t, "String");
}
function Af(t) {
  return t === null || (typeof t != "object" && typeof t != "function");
}
function ll(t) {
  return Ot(t, "Object");
}
function Kr(t) {
  return typeof Event < "u" && _n(t, Event);
}
function cl(t) {
  return typeof Element < "u" && _n(t, Element);
}
function Of(t) {
  return Ot(t, "RegExp");
}
function Pf(t) {
  return !!(t != null && t.then && typeof t.then == "function");
}
function Mf(t) {
  return (
    ll(t) &&
    "nativeEvent" in t &&
    "preventDefault" in t &&
    "stopPropagation" in t
  );
}
function _n(t, e) {
  try {
    return t instanceof e;
  } catch (r) {
    return !1;
  }
}
class Lf extends wn {
  constructor(e) {
    super(), (this.detectors = e);
  }
  transform(e) {
    const r = e instanceof mt ? e.error : e;
    for (const n of this.detectors)
      if (n.check(r))
        return new n(dr(r) ? r.message : "unknown", {
          cause: r,
        });
    return null;
  }
}
var Wt;
(function (t) {
  (t.UnknownError = "UnknownError"),
    (t.EvalError = "EvalError"),
    (t.RangeError = "RangeError"),
    (t.ReferenceError = "ReferenceError"),
    (t.SyntaxError = "SyntaxError"),
    (t.TypeError = "TypeError"),
    (t.URIError = "URIError"),
    (t.IndexSizeError = "IndexSizeError"),
    (t.HierarchyRequestError = "HierarchyRequestError"),
    (t.WrongDocumentError = "WrongDocumentError"),
    (t.InvalidCharacterError = "InvalidCharacterError"),
    (t.NoModificationAllowedError = "NoModificationAllowedError"),
    (t.NotFoundError = "NotFoundError"),
    (t.NotSupportedError = "NotSupportedError"),
    (t.InUseAttributeError = "InUseAttributeError"),
    (t.InvalidStateError = "InvalidStateError"),
    (t.InvalidModificationError = "InvalidModificationError"),
    (t.NamespaceError = "NamespaceError"),
    (t.InvalidAccessError = "InvalidAccessError"),
    (t.SecurityError = "SecurityError"),
    (t.NetworkError = "NetworkError"),
    (t.AbortError = "AbortError"),
    (t.URLMismatchError = "URLMismatchError"),
    (t.QuotaExceededError = "QuotaExceededError"),
    (t.TimeoutError = "TimeoutError"),
    (t.InvalidNodeTypeError = "InvalidNodeTypeError"),
    (t.DataCloneError = "DataCloneError"),
    (t.EncodingError = "EncodingError"),
    (t.NotReadableError = "NotReadableError"),
    (t.ConstraintError = "ConstraintError"),
    (t.DataError = "DataError"),
    (t.TransactionInactiveError = "TransactionInactiveError"),
    (t.ReadOnlyError = "ReadOnlyError"),
    (t.VersionError = "VersionError"),
    (t.OperationError = "OperationError"),
    (t.NotAllowedError = "NotAllowedError");
})(Wt || (Wt = {}));
class ul extends At {
  constructor(e, r = {}) {
    super(e, r), (this.name = "ProgramError");
    const { cause: n, level: s = dt.Default } = r;
    (this.type =
      n != null && n.name && n.name in Wt
        ? n == null
          ? void 0
          : n.name
        : Wt.UnknownError),
      (this.level = s);
  }
  toString() {
    var e, r, n;
    return !((e = this.stack) === null || e === void 0) &&
      e.startsWith(
        (r = this.cause) === null || r === void 0 ? void 0 : r.name
      ) &&
      !((n = this.cause) === null || n === void 0) &&
      n.name
      ? this.stack.replace(
          this.cause.name,
          "".concat(this.name, "[").concat(this.type, "]")
        )
      : this.stack
      ? this.stack
      : "".concat(this.name, "[").concat(this.type, "]: ").concat(this.message);
  }
  get merlin() {
    return {
      idx1: this.type,
    };
  }
}
class Er extends At {
  constructor(e, r, n = {}) {
    super(e, n), (this.url = r), (this.name = "LoadError");
  }
  get merlin() {
    const e = {};
    return (
      this.level === dt.Fatal || this.level === dt.Danger
        ? (e.idx1 = this.url)
        : (e.extra = "url:".concat(this.url)),
      e
    );
  }
}
class fl extends Er {
  constructor(e, r, n = {}) {
    super(e, r, n), (this.name = "ImageLoadError");
    const { eleId: s = "" } = n;
    this.eleId = s;
  }
  get merlin() {
    return Object.assign(Object.assign({}, super.merlin), {
      idx3: this.eleId,
    });
  }
}
class dl extends Er {
  constructor(e, r, n, s = {}) {
    super(e, r, s), (this.type = n), (this.name = "ScriptLoadError");
    const { level: i = dt.Danger } = s;
    this.level = i;
  }
}
class Nf extends At {
  constructor(e, r, n = {}) {
    super(e, n), (this.url = r), (this.name = "RequestError");
    const { req: s, resp: i, errCode: o } = n;
    (this.req = s), (this.resp = i), (this.errCode = o);
  }
  get merlin() {
    return {
      idx1: this.url,
      idx2: "".concat(this.errCode !== void 0 ? this.errCode : ""),
    };
  }
}
class hl extends At {
  constructor(e, r, n = {}) {
    super(e, n), (this.scene = r), (this.name = "LogicError");
  }
  get merlin() {
    return {
      idx1: this.scene,
    };
  }
}
class Zi extends wn {
  transform(e) {
    return e instanceof At
      ? null
      : e instanceof Zs
      ? this.transformOnError(
          e.info.event,
          e.info.source,
          e.info.lineno,
          e.info.colno,
          e.error
        )
      : e instanceof an
      ? this.transformEvent(e.info.event)
      : e instanceof mt && e.error
      ? this.transformError(e.error)
      : this.transformError(e);
  }
  transformError(e, r) {
    return dr(e)
      ? e.name in Wt
        ? new ul(e.message, {
            cause: e,
            origin: r,
          })
        : e.name === "ResourceError" && e.liteUrl
        ? new Er("failed to load: ".concat(e.liteUrl), e.liteUrl)
        : null
      : null;
  }
  transformErrorEvent(e) {
    return this.transformError(e.error, {
      file: e.filename,
      line: e.lineno,
      col: e.colno,
    });
  }
  transformPromiseRejectionEvent(e) {
    var r;
    const n =
      e.reason || ((r = e.detail) === null || r === void 0 ? void 0 : r.reason);
    return n
      ? dr(n)
        ? this.transformError(n)
        : Kr(n) && e !== n && !Ts(n)
        ? this.transformEvent(n)
        : null
      : null;
  }
  transformEvent(e) {
    var r, n;
    if (Ts(e)) return this.transformPromiseRejectionEvent(e);
    if (al(e)) return this.transformErrorEvent(e);
    if (Kr(e) && cl(e.target)) {
      const s = e.target,
        i = s.getAttribute("src") || s.getAttribute("href") || "",
        o = (r = s.tagName) === null || r === void 0 ? void 0 : r.toUpperCase();
      if (o === "IMG" || o === "IMAGE") {
        let a = "";
        return (
          !((n = s.dataset) === null || n === void 0) && n.pandora
            ? (a += s.dataset.pandora)
            : (s.id && (a += "#".concat(s.id)),
              s.className && (a += ".".concat(s.className))),
          new fl("failed to load (".concat(a, "): ").concat(i), i, {
            eleId: a,
          })
        );
      }
      return o === "SCRIPT" || o === "LINK"
        ? new dl(
            "failed to load: ".concat(i),
            i,
            s.getAttribute("type") || s.getAttribute("as") || ""
          )
        : new Er("failed to load: ".concat(i), i);
    }
    return null;
  }
  transformOnError(e, r, n, s, i) {
    return dr(i)
      ? this.transformError(i, {
          file: r,
          line: n,
          col: s,
        })
      : Kr(e)
      ? this.transformEvent(e)
      : null;
  }
}
class Zs extends mt {}
class an extends mt {}
class pl {
  constructor(e) {
    (this.catchers = new Set()),
      (this.consumers = []),
      (this.running = !1),
      (this.thisCatch = this.catch.bind(this)),
      (this.transformers =
        e != null && e.disableBuiltinTransformers ? [] : [new Zi()]);
  }
  run(e) {
    if (this.running) return this;
    const {
        plugins: r = [],
        catchers: n = [],
        transformers: s = [],
        consumers: i = [],
        disableBuiltinTransformers: o,
      } = e || {},
      a = [],
      l = o ? [] : [new Zi()],
      c = [];
    return (
      r.forEach((u) => {
        a.push(...u.catchers),
          l.push(...u.transformers),
          c.push(...u.consumers);
      }),
      a.push(...n),
      l.push(...s),
      c.push(...i),
      a.forEach((u) => this.addCatcher(u)),
      this.setTransformers(l),
      this.setConsumers(c),
      (this.running = !0),
      this.catchers.forEach((u) => u.run()),
      this
    );
  }
  addCatcher(e) {
    return e instanceof Is
      ? (this.catchers.add(e),
        e.bindOnCatch(this.thisCatch),
        this.running && e.run(),
        this)
      : this;
  }
  removeCatcher(e) {
    return e instanceof Is
      ? (this.running && e.stop(),
        e.unbindOnCatch(),
        this.catchers.delete(e),
        this)
      : this;
  }
  setCatchers(e) {
    return (
      this.catchers.size &&
        (this.catchers.forEach((r) => r.unbindOnCatch()),
        this.catchers.clear()),
      e.forEach((r) => this.addCatcher(r)),
      this
    );
  }
  setTransformers(e) {
    return (this.transformers = e.filter((r) => r instanceof wn)), this;
  }
  setConsumers(e) {
    return (this.consumers = e.filter((r) => r instanceof il)), this;
  }
  transform(e) {
    let r = e;
    return (
      this.transformers.forEach((n) => {
        var s;
        r = (s = n.transform(r)) !== null && s !== void 0 ? s : r;
      }),
      r
    );
  }
  catch(e) {
    let r = this.transform(e);
    return (
      r instanceof mt && r.error && (r = r.error),
      this.consumers.forEach((n) => {
        r = n.consume(r);
      }),
      r
    );
  }
  stop() {
    if (this.running)
      return this.catchers.forEach((e) => e.stop()), (this.running = !1), this;
  }
}
function kf(t) {
  return function (e, r, n) {
    const s = n.value;
    n.value = function (i) {
      if (Array.isArray(t)) {
        for (const o of t) if (i instanceof o) return s.apply(this, i);
      } else if (i instanceof t) return s.apply(this, i);
      return i;
    };
  };
}
function Vf(t) {
  return function (e, r, n) {
    const s = n.value;
    n.value = function (i) {
      return t(i) ? s.apply(this, i) : i;
    };
  };
}
const Df = new pl(),
  Kp = Object.freeze(
    Object.defineProperty(
      {
        __proto__: null,
        Catcher: Is,
        CatcherIssue: mt,
        Consumer: il,
        DetectableTransformer: Lf,
        get ErrorLevel() {
          return dt;
        },
        EventCatcherIssue: an,
        ImageLoadError: fl,
        KnownError: At,
        LoadError: Er,
        LogicError: hl,
        OnErrorCatcherIssue: Zs,
        Pandora: pl,
        Plugin: Sf,
        ProgramError: ul,
        get ProgramErrorType() {
          return Wt;
        },
        RequestError: Nf,
        ScriptLoadError: dl,
        Transformer: wn,
        isDOMError: Tf,
        isDOMException: Cf,
        isElement: cl,
        isError: dr,
        isErrorEvent: al,
        isEvent: Kr,
        isInstanceOf: _n,
        isPlainObject: ll,
        isPrimitive: Af,
        isPromiseRejectionEvent: Ts,
        isRegExp: Of,
        isString: Rf,
        isSyntheticEvent: Mf,
        isThenable: Pf,
        logError: If,
        pandora: Df,
        runIf: Vf,
        runIs: kf,
      },
      Symbol.toStringTag,
      {
        value: "Module",
      }
    )
  );
function We(t, e, r, n) {
  function s(i) {
    return i instanceof r
      ? i
      : new r(function (o) {
          o(i);
        });
  }
  return new (r || (r = Promise))(function (i, o) {
    function a(u) {
      try {
        c(n.next(u));
      } catch (d) {
        o(d);
      }
    }
    function l(u) {
      try {
        c(n.throw(u));
      } catch (d) {
        o(d);
      }
    }
    function c(u) {
      u.done ? i(u.value) : s(u.value).then(a, l);
    }
    c((n = n.apply(t, [])).next());
  });
}
class gl extends Error {
  constructor(e) {
    super(e), (this.name = "MerlinError");
  }
}
class Sn {
  constructor() {
    this.merlin = void 0;
  }
  static isCollector(e) {
    return (
      e &&
      typeof e.init == "function" &&
      typeof e.settle == "function" &&
      typeof e.afterInit == "function"
    );
  }
  init(e) {
    this.merlin = e;
    try {
      this.afterInit();
    } catch (r) {
      e.core.adapter.errorHandler(
        new gl(
          "collector 初始化失败: ".concat(
            (r == null ? void 0 : r.message) || "",
            "}"
          )
        )
      );
    }
  }
  settle() {}
}
const eo = "__ml::aid",
  Ye = "__ml::page";
var to;
(function (t) {
  (t.OCCUR = "occur"), (t.RECOVER = "recover");
})(to || (to = {}));
var Ie;
(function (t) {
  (t.CUSTOM = "custom"),
    (t.PAGE_ENTER = "pageEnter"),
    (t.PAGE_LEAVE = "pageLeave"),
    (t.ELEMENT_EXPOSE = "elementExpose"),
    (t.ELEMENT_CONCEAL = "elementConceal"),
    (t.ELEMENT_CLICK = "elementClick"),
    (t.ELEMENT_HOVER = "elementHover"),
    (t.VIDEO_PLAY_START = "videoPlayStart"),
    (t.VIDEO_PLAY_PAUSE = "videoPlayPause"),
    (t.VIDEO_PLAY_WAITING = "videoPlayWaiting"),
    (t.VIDEO_PLAY_SEEK = "videoPlaySeek"),
    (t.VIDEO_PLAY_FINISH = "videoPlayFinish");
})(Ie || (Ie = {}));
var hr;
(function (t) {
  (t.OK = "ok"), (t.FAIL = "fail"), (t.ABORT = "abort");
})(hr || (hr = {}));
var he;
(function (t) {
  (t.ERROR = "error"), (t.BEHAVIOR = "behavior"), (t.PERFORMANCE = "perf");
})(he || (he = {}));
class ei {
  constructor() {
    (this.ctime = Date.now()),
      (this.context = {
        contextId: "",
        aid: "",
        env: {},
        page: {},
      });
  }
  updateContext(e) {
    this.context = e;
  }
}
class $f extends ei {
  constructor(e) {
    super(), (this.data = e), (this.type = he.ERROR);
  }
}
class Me extends ei {
  constructor(e) {
    super(), (this.data = e), (this.type = he.BEHAVIOR);
  }
}
class ro extends ei {
  static parseStatus(e) {
    return e === "fail" ? hr.FAIL : e === "abort" ? hr.ABORT : hr.OK;
  }
  constructor(e) {
    super(), (this.data = e), (this.type = he.PERFORMANCE);
  }
}
class Ff {
  constructor(e, r, n, s) {
    (this.reporter = e),
      (this.type = r),
      (this.action = n),
      (this.info = s),
      (this.startTime = Date.now()),
      (this.settled = !1);
  }
  resetStartTime(e) {
    return (this.startTime = e || Date.now()), this;
  }
  report(e = "ok", r, n) {
    var s, i, o, a;
    if (this.settled) return;
    this.settled = !0;
    const l = n == null ? void 0 : n.sampleRate;
    if (l !== void 0 && (l <= 0 || (l < 1 && Math.random() >= l))) return;
    const c = Date.now(),
      u = c > this.startTime ? c - this.startTime : 0;
    this.reporter.reportPerf(this.type, this.action, {
      duration: u,
      status: e,
      idx1:
        (r == null ? void 0 : r.idx1) ||
        ((s = this.info) === null || s === void 0 ? void 0 : s.idx1),
      idx2:
        (r == null ? void 0 : r.idx2) ||
        ((i = this.info) === null || i === void 0 ? void 0 : i.idx2),
      idx3:
        (r == null ? void 0 : r.idx3) ||
        ((o = this.info) === null || o === void 0 ? void 0 : o.idx3),
      log:
        (r == null ? void 0 : r.log) ||
        ((a = this.info) === null || a === void 0 ? void 0 : a.log),
    });
  }
}
class Tr {
  static isTransport(e) {
    return (
      e &&
      typeof e.init == "function" &&
      typeof e.receiveFromCore == "function" &&
      typeof e.flush == "function" &&
      typeof e.send == "function"
    );
  }
  constructor(e) {
    (this.sampleRate = 1),
      (this.filter = void 0),
      (this.buffer = []),
      (this.frequencyLimitRecordCount = 0);
    const {
      filter: r,
      sampleRate: n,
      bufferSize: s = 50,
      flushInterval: i = 1e4,
      frequencyLimit: o = {
        max: 0,
        perSeconds: 0,
      },
    } = e || {};
    r && (this.filter = r),
      typeof n == "number" && (this.sampleRate = n <= 0 || n > 1 ? 1 : n),
      (this.bufferSize = s),
      (this.flushInterval = i),
      (this.frequencyLimit = o);
  }
  init(e) {
    return We(this, void 0, void 0, function* () {
      (this.reporter = e),
        setInterval &&
          clearInterval &&
          (this.flushInterval !== 0 &&
            (this.flushIntervalTimer &&
              (clearInterval(this.flushIntervalTimer),
              (this.flushIntervalTimer = void 0)),
            (this.flushIntervalTimer = setInterval(() => {
              this.flush();
            }, this.flushInterval))),
          this.frequencyLimit.perSeconds !== 0 &&
            (this.frequencyLimitTimer &&
              (clearInterval(this.frequencyLimitTimer),
              (this.frequencyLimitTimer = void 0)),
            (this.frequencyLimitTimer = setInterval(() => {
              this.frequencyLimitRecordCount = 0;
            }, this.frequencyLimit.perSeconds * 1e3))));
    });
  }
  receiveFromCore(e, r = !1) {
    return We(this, void 0, void 0, function* () {
      let n = [...e].filter((s) => this.reportTypes.includes(s.type));
      this.filter && (n = n.filter(this.filter)),
        this.sampleRate < 1 &&
          (n = n.filter(() => Math.random() < this.sampleRate)),
        n.length && this.buffer.push(...n),
        (r || this.bufferSize === 0 || this.buffer.length >= this.bufferSize) &&
          (yield this.flush());
    });
  }
  flush() {
    return We(this, void 0, void 0, function* () {
      if (!this.buffer.length) return;
      const e = [...this.buffer];
      if (((this.buffer.length = 0), this.frequencyLimit.max !== 0)) {
        const r = this.frequencyLimit.max - this.frequencyLimitRecordCount;
        if (r <= 0) return;
        r < e.length && (e.length = r),
          (this.frequencyLimitRecordCount += e.length);
      }
      yield this.send(e);
    });
  }
}
let no;
function ti() {
  return (
    no ||
    (no =
      typeof globalThis < "u"
        ? globalThis
        : typeof self < "u"
        ? self
        : typeof window < "u"
        ? window
        : typeof global < "u"
        ? global
        : {})
  );
}
function Uf(t = location.href) {
  var e, r, n, s;
  const i = {},
    o = t.includes("#") ? t.split("#")[0] : t;
  let a = o.includes("?") ? o.split("?")[1] : o;
  if (a.includes("="))
    for (const c of a.split("&")) {
      const [u, d] = c.split("="),
        p =
          (e = u == null ? void 0 : u.replace(/\+/g, " ")) !== null &&
          e !== void 0
            ? e
            : "",
        g =
          (r = d == null ? void 0 : d.replace(/\+/g, " ")) !== null &&
          r !== void 0
            ? r
            : "";
      try {
        i[p] = decodeURIComponent(g);
      } catch (v) {
        i[p] = g;
      }
    }
  const l = t.includes("#") ? t.split("#")[1] : "";
  if (((a = l.includes("?") ? l.split("?")[1] : ""), a.includes("=")))
    for (const c of a.split("&")) {
      const [u, d] = c.split("="),
        p =
          (n = u == null ? void 0 : u.replace(/\+/g, " ")) !== null &&
          n !== void 0
            ? n
            : "",
        g =
          (s = d == null ? void 0 : d.replace(/\+/g, " ")) !== null &&
          s !== void 0
            ? s
            : "";
      try {
        i[p] = decodeURIComponent(g);
      } catch (v) {
        i[p] = g;
      }
    }
  return i;
}
function jf(t) {
  return /^[A-Z]/.test(t) ? t : t.replace(/([A-Z])/g, "_$1").toLowerCase();
}
function Hf(t) {
  return /^[A-Z]/.test(t)
    ? t
    : t.replace(/(_[a-z])/g, (e, r) => r.toUpperCase().slice(1));
}
function Bf(t, e = !0) {
  return Object.entries(t)
    .filter(([r, n]) => n != null)
    .map(([r, n]) => "".concat(e ? jf(r) : r, "=").concat(n))
    .join("&");
}
function qf(t) {
  return typeof t != "object"
    ? t
    : Object.fromEntries(Object.entries(t).map(([e, r]) => [Hf(e), r]));
}
function Ge() {
  try {
    const t = crypto;
    if (t != null && t.randomUUID) return t.randomUUID();
    if (t != null && t.getRandomValues && Uint8Array)
      return "10000000-1000-4000-8000-100000000000".replace(/[018]/g, (e) =>
        (
          Number(e) ^
          (t.getRandomValues(new Uint8Array(1))[0] & (15 >> (Number(e) / 4)))
        ).toString(16)
      );
  } catch (t) {}
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (t) => {
    const e = (Math.random() * 16) | 0;
    return (t === "x" ? e : (e & 3) | 8).toString(16);
  });
}
function Cr(t, e, r, n) {
  return {
    filename: t,
    func: e,
    lineno: r,
    colno: n,
  };
}
const Yt = "?",
  Kf =
    /^\s*at (?:(.*?) ?\((?:address at )?)?((?:file|https?|blob|chrome-extension|address|native|eval|webpack|<anonymous>|[-a-z]+:|.*bundle|\/).*?)(?::(\d+))?(?::(\d+))?\)?\s*$/i,
  Wf = /\((\S*)(?::(\d+))(?::(\d+))\)/,
  Gf = (t) => {
    const e = Kf.exec(t);
    if (e) {
      if (e[2] && e[2].indexOf("eval") === 0) {
        const i = Wf.exec(e[2]);
        i && ([e[2], e[3], e[4]] = [i[1], i[2], i[3]]);
      }
      const [n, s] = ml(e[1] || Yt, e[2]);
      return Cr(s, n, e[3] ? +e[3] : void 0, e[4] ? +e[4] : void 0);
    }
  },
  zf =
    /^\s*(.*?)(?:\((.*?)\))?(?:^|@)?((?:file|https?|blob|chrome|webpack|resource|moz-extension|capacitor).*?:\/.*?|\[native code\]|[^@]*(?:bundle|\d+\.js)|\/[\w\-. /=]+)(?::(\d+))?(?::(\d+))?\s*$/i,
  Jf = /(\S+) line (\d+)(?: > eval line \d+)* > eval/i,
  Yf = (t) => {
    const e = zf.exec(t);
    if (e) {
      if (e[3] && e[3].indexOf(" > eval") > -1) {
        const i = Jf.exec(e[3]);
        i &&
          ((e[1] = e[1] || "eval"), ([e[3], e[4]] = [i[1], i[2]]), (e[5] = ""));
      }
      let n = e[3],
        s = e[1] || Yt;
      return (
        ([s, n] = ml(s, n)),
        Cr(n, s, e[4] ? +e[4] : void 0, e[5] ? +e[5] : void 0)
      );
    }
  },
  Xf =
    /^\s*at (?:((?:\[object object\])?.+) )?\(?((?:file|ms-appx|https?|webpack|blob):.*?):(\d+)(?::(\d+))?\)?\s*$/i,
  Qf = (t) => {
    const e = Xf.exec(t);
    return e ? Cr(e[2], e[1] || Yt, +e[3], e[4] ? +e[4] : void 0) : void 0;
  },
  Zf = / line (\d+).*script (?:in )?(\S+)(?:: in function (\S+))?$/i,
  ed = (t) => {
    const e = Zf.exec(t);
    return e ? Cr(e[2], e[3] || Yt, +e[1]) : void 0;
  },
  td =
    / line (\d+), column (\d+)\s*(?:in (?:<anonymous function: ([^>]+)>|([^)]+))\(.*\))? in (.*):\s*$/i,
  rd = (t) => {
    const e = td.exec(t);
    return e ? Cr(e[5], e[3] || e[4] || Yt, +e[1], +e[2]) : void 0;
  },
  ml = (t, e) => {
    const r = t.indexOf("safari-extension") !== -1,
      n = t.indexOf("safari-web-extension") !== -1;
    return r || n
      ? [
          t.indexOf("@") !== -1 ? t.split("@")[0] : Yt,
          r ? "safari-extension:".concat(e) : "safari-web-extension:".concat(e),
        ]
      : [t, e];
  },
  nd = [ed, rd, Gf, Qf, Yf];
function sd(...t) {
  return (e, r = 0, n = 0) => {
    const s = [];
    for (const i of e.split("\n").slice(r))
      for (const o of t) {
        const a = o(i);
        if (a) {
          if ((s.push(a), n > 0 && s.length >= n)) return s;
          break;
        }
      }
    return s;
  };
}
const id = sd(...nd);
function od(t) {
  for (const e of t)
    if (e.filename && e.lineno !== void 0 && e.colno !== void 0)
      return {
        file: e.filename,
        line: e.lineno,
        col: e.colno,
      };
  return {};
}
function ad(t) {
  return !!t.file && t.file !== "undefined" && t.line !== void 0;
}
function ld(t) {
  const e = t.stack ? id(t.stack) : [];
  let { file: r, line: n, col: s } = t;
  return (
    ad({
      file: r,
      line: n,
    }) || ({ file: r, col: s, line: n } = od(e)),
    {
      id: Ge(),
      name: t.name || "Unknown",
      message: t.message || "",
      stack: t.stack,
      stackFrames: e,
      file: r,
      line: n,
      col: s,
      eIdx1: t.eIdx1,
      eIdx2: t.eIdx2,
      eIdx3: t.eIdx3,
      eExtra: t.eExtra,
      idx1: t.idx1,
      idx2: t.idx2,
      idx3: t.idx3,
      log: t.log,
      status: t.status,
    }
  );
}
var so;
(function (t) {
  (t.UnsignedInt = "unsigned int"),
    (t.Int = "int"),
    (t.LongLong = "long long"),
    (t.Char1024 = "char[1024]"),
    (t.Uint64 = "uint64");
})(so || (so = {}));
class yr {
  static parseChars(e) {
    var r;
    try {
      return typeof e == "string"
        ? e.replace(/,/g, ";")
        : e && typeof e == "object"
        ? ((r = JSON.stringify(e)) === null || r === void 0
            ? void 0
            : r.replace(/,/g, ";")) || ""
        : (e == null ? void 0 : e.toString().replace(/,/g, ";")) || "";
    } catch (n) {
      return "";
    }
  }
  static parseUint(e) {
    try {
      return typeof e == "number" ? Math.floor(e) : 0;
    } catch (r) {
      return 0;
    }
  }
  static parseValue(e, r) {
    return r === "chars" || r === "char[1024]"
      ? this.parseChars(e)
      : this.parseUint(e);
  }
  constructor(e) {
    this.fields = e;
    const r = new Map();
    e.forEach((n) => r.set(n[0], n[1])), (this.fieldNameToType = r);
  }
  parseFields(e) {
    if (!e) return {};
    const r = {};
    return (
      Object.entries(e).forEach(([n, s]) => {
        const i = this.fieldNameToType.get(n);
        i && (r[n] = yr.parseValue(s, i));
      }),
      r
    );
  }
  serialize(e) {
    return this.fields.map((r) => yr.parseValue(e[r[0]], r[1])).join(",");
  }
}
function cd(t) {
  return t.map((e) => yr.parseChars(e)).join(",");
}
let io;
function Wp() {
  return (
    io ||
    (io = new yr([
      ["deviceModel", "chars"],
      ["deviceBrand", "chars"],
      ["osName", "chars"],
      ["osVersion", "chars"],
      ["languageVersion", "chars"],
      ["startTime", "uint"],
      ["endTime", "uint"],
      ["count", "uint"],
      ["clientIPV6", "chars"],
      ["bizId", "uint"],
      ["contextId", "chars"],
      ["entranceId", "chars"],
      ["ctime", "uint"],
      ["location", "chars"],
      ["network", "uint"],
      ["browser", "chars"],
      ["userInfo", "chars"],
      ["redDotInfo", "chars"],
      ["recInfo", "chars"],
      ["pageId", "chars"],
      ["refPageId", "chars"],
      ["accessId", "chars"],
      ["refAccessId", "chars"],
      ["refEleId", "chars"],
      ["step", "uint"],
      ["pageInfo", "chars"],
      ["eleId", "chars"],
      ["eleInfo", "chars"],
      ["actionType", "chars"],
      ["actionInfo", "chars"],
      ["windowInfo", "chars"],
      ["entranceInfo", "chars"],
    ]))
  );
}
function vl() {
  return new Promise((t) => {
    var e;
    !((e = ti().WeixinJSBridge) === null || e === void 0) && e.invoke
      ? t(null)
      : document.addEventListener("WeixinJSBridgeReady", t, !1);
  });
}
function ud(t) {
  var e, r;
  return !!(
    !(
      (r =
        (e = t == null ? void 0 : t.version) === null || e === void 0
          ? void 0
          : e.startsWith) === null || r === void 0
    ) && r.call(e, "3")
  );
}
function fd(t) {
  var e, r;
  return !!(
    !(
      (r =
        (e = t == null ? void 0 : t.version) === null || e === void 0
          ? void 0
          : e.startsWith) === null || r === void 0
    ) && r.call(e, "2")
  );
}
function In(t, e, r) {
  t &&
    (ud(t)
      ? t.directive(e, {
          mounted: r.mounted,
          updated: r.updated,
          unmounted: r.unmounted,
        })
      : fd(t) &&
        t.directive(e, {
          inserted: r.mounted,
          componentUpdated: r.updated,
          unbind: r.unmounted,
        }));
}
function Gt(t) {
  var e, r;
  return (e = t == null ? void 0 : t.props) !== null && e !== void 0
    ? e
    : (r = t == null ? void 0 : t.data) === null || r === void 0
    ? void 0
    : r.attrs;
}
class xl {
  constructor(e) {
    (this.ctx = {}),
      (this.contextEnv = {}),
      (this.fromPage = {}),
      (this.aIdGenerator = () => Ge()),
      (this.errorHandler = (s, i) => {});
    const { aIdGenerator: r, errorHandler: n } = e || {};
    r && (this.aIdGenerator = r), n && (this.errorHandler = n), (this.cfg = e);
  }
  init(e) {
    this.core = e;
    const {
      collectors: r = [],
      transports: n = [],
      userInfo: s,
      release: i = this.core.release,
      logError: o = this.core.logError,
    } = this.cfg || {};
    r.forEach((a) => {
      var l;
      return (l = this.core) === null || l === void 0 ? void 0 : l.collector(a);
    }),
      n.forEach((a) => {
        var l;
        return (l = this.core) === null || l === void 0
          ? void 0
          : l.transport(a);
      }),
      s && (this.core.userInfo = s),
      (this.core.release = i),
      (this.core.logError = o),
      this.initAid(),
      this.updateContext(),
      this.readLinkedData(),
      this.loadPageStackFromStorage(),
      this.afterInit();
  }
  afterInit() {}
  pushPage(e, r) {
    this.readLinkedData();
    const n = this.loadPageStackFromStorage();
    n
      ? (this.currentPage = {
          pageId: e,
          accessId: Ge(),
          step: n.step + 1,
          refAccessId: n.accessId,
          refPageId: n.pageId,
          fromElementId: this.fromPage.eleId,
          fromAccessId: this.fromPage.accessId,
          pageInfo: r,
        })
      : (this.currentPage = {
          pageId: e,
          accessId: Ge(),
          step: 1,
          pageInfo: r,
          fromElementId: this.fromPage.eleId,
          fromAccessId: this.fromPage.accessId,
        }),
      this.savePageStackToStorage();
  }
  updatePageInfo(e) {
    this.currentPage && (this.currentPage.pageInfo = e);
  }
  get contextPage() {
    if (!this.currentPage) return {};
    const {
      pageId: e,
      accessId: r,
      step: n,
      refAccessId: s,
      refPageId: i,
      fromElementId: o,
      fromAccessId: a,
      pageInfo: l,
    } = this.currentPage;
    return {
      pageId: e,
      accessId: r,
      step: n,
      refAccessId: s,
      refPageId: i,
      fromElementId: o,
      fromAccessId: a,
      pageInfo: l,
    };
  }
  getPageStorageKey(e) {
    var r;
    return (
      e === void 0 &&
        (e = (r = this.core) === null || r === void 0 ? void 0 : r.contextId),
      "".concat(Ye, "_").concat(e)
    );
  }
}
class dd extends xl {
  constructor() {
    super(...arguments),
      (this.storage = {
        supportSync: !1,
        setItem() {},
        getItem() {},
        removeItem() {},
      });
  }
  get env() {
    return "";
  }
  httpPost() {}
  get linkedData() {
    return {};
  }
  initAid() {}
  readLinkedData() {}
  serializeLinkedData() {
    return "";
  }
  settle() {}
  updateContext() {}
  loadPageStackFromStorage() {}
  savePageStackToStorage() {}
}
class hd extends Sn {
  constructor() {
    super(...arguments),
      (this.thisOnerrorHandler = this.onerrorHandler.bind(this)),
      (this.thisErrorListener = this.errorListener.bind(this)),
      (this.thisUnhandledrejectionListener =
        this.unhandledrejectionListener.bind(this)),
      (this.lastCaughtError = null);
  }
  afterInit() {
    (window.onerror = this.thisOnerrorHandler),
      window.addEventListener("error", this.thisErrorListener, !0),
      window.addEventListener(
        "unhandledrejection",
        this.thisUnhandledrejectionListener
      );
  }
  logError(...e) {
    var r;
    !((r = this.merlin) === null || r === void 0) && r.core.logError;
  }
  onerrorHandler(e, r, n, s, i) {
    var o;
    return (
      i === this.lastCaughtError ||
        ((this.lastCaughtError = i || null),
        this.logError(i || e),
        (o = this.merlin) === null ||
          o === void 0 ||
          o.catch(
            new Zs(
              {
                source: r,
                lineno: n,
                colno: s,
                event: e,
              },
              i
            )
          )),
      !0
    );
  }
  errorListener(e) {
    var r;
    const { error: n } = e;
    n !== this.lastCaughtError &&
      ((this.lastCaughtError = n || null),
      this.logError(n || e),
      (r = this.merlin) === null ||
        r === void 0 ||
        r.catch(
          new an(
            {
              event: e,
            },
            n
          )
        ));
  }
  unhandledrejectionListener(e) {
    var r, n;
    e.preventDefault();
    const s =
      e.reason || ((r = e.detail) === null || r === void 0 ? void 0 : r.reason);
    s !== this.lastCaughtError &&
      ((this.lastCaughtError = s || null),
      this.logError(s || e),
      (n = this.merlin) === null ||
        n === void 0 ||
        n.catch(
          new an(
            {
              event: e,
            },
            s
          )
        ));
  }
}
class pd extends Sn {
  constructor(e) {
    super(), (this.vue = e.vue);
  }
  afterInit() {
    this.vue.config &&
      "errorHandler" in this.vue.config &&
      (this.vue.config.errorHandler = (e, r, n) => {
        var s, i;
        !((s = this.merlin) === null || s === void 0) && s.core.logError,
          (i = this.merlin) === null || i === void 0 || i.catch(new mt({}, e));
      });
  }
}
var oo;
(function (t) {
  (t.TRUE = "true"), (t.FALSE = "false");
})(oo || (oo = {}));
var Ct;
(function (t) {
  (t.TRUE = "true"), (t.FALSE = "false");
})(Ct || (Ct = {}));
function gd(t, e) {
  return !t || !e ? !1 : t === e ? !0 : t.contains(e);
}
const ao = (t, e) => {
    var r;
    return (
      !!t &&
      !!e &&
      (t === document.body ||
        ((r = t.dataset) === null || r === void 0 ? void 0 : r.mlRoot) ===
          Ct.TRUE) &&
      t.contains(e)
    );
  },
  md = (t) => {
    if (!t) return "";
    const e = t.getAttribute("ml-key");
    if (e && e.length > 0) return e;
    const r = t.dataset.mlKey;
    return r && r.length > 0 ? r : "";
  },
  lo =
    typeof window == "object" &&
    "IntersectionObserver" in window &&
    "IntersectionObserverEntry" in window &&
    "intersectionRatio" in window.IntersectionObserverEntry.prototype;
class Tn {
  constructor(e) {
    (this.el = e.el), (this.extInfoOrFn = e.extInfo), (this.key = md(e.el));
  }
  updateExtInfo(e) {
    this.extInfoOrFn = e;
  }
  get extInfo() {
    try {
      return typeof this.extInfoOrFn == "function"
        ? this.extInfoOrFn()
        : this.extInfoOrFn;
    } catch (e) {
      return {};
    }
  }
}
class vd {
  constructor() {
    (this.currentState = null),
      (this.stateToTime = new Map()),
      (this.lastTimeStamp = 0);
  }
  init(e, r) {
    e.forEach((n) => {
      this.stateToTime.set(n, 0);
    }),
      r !== void 0 &&
        ((this.lastTimeStamp = Date.now()), (this.currentState = r));
  }
  getCurrent() {
    return this.currentState;
  }
  switchTo(e) {
    if (!this.stateToTime.has(e) || this.currentState === e) return;
    const r = Date.now(),
      n = r - this.lastTimeStamp;
    if (this.currentState !== null) {
      const s = this.stateToTime.get(this.currentState) || 0;
      this.stateToTime.set(this.currentState, n + s);
    }
    (this.currentState = e), (this.lastTimeStamp = r);
  }
  updateTime(e) {
    if (!this.stateToTime.has(e) || this.currentState !== e) return;
    const r = Date.now(),
      n = r - this.lastTimeStamp,
      s = this.stateToTime.get(this.currentState) || 0;
    this.stateToTime.set(this.currentState, n + s), (this.lastTimeStamp = r);
  }
  resetTime(e) {
    this.stateToTime.has(e) && this.stateToTime.set(e, 0);
  }
  getTime(e, r = !1) {
    var n;
    return (
      r && this.updateTime(e),
      (n = this.stateToTime.get(e)) !== null && n !== void 0 ? n : 0
    );
  }
}
class xd extends Tn {
  constructor(e) {
    var r;
    super(e),
      (this.listener = () => {}),
      e.localListener &&
        ((this.localListener = e.localListener),
        (this.context = e.context),
        (this.listener = this.privateClick.bind(this)),
        (r = this.el) === null ||
          r === void 0 ||
          r.addEventListener("click", this.listener));
  }
  updateConfig() {}
  destroy() {
    var e;
    this.localListener &&
      ((e = this.el) === null ||
        e === void 0 ||
        e.removeEventListener("click", this.listener));
  }
  privateClick() {
    var e, r;
    (r = (e = this.context) === null || e === void 0 ? void 0 : e.reporter) ===
      null ||
      r === void 0 ||
      r.reportElementClick(this.key, this.extInfo);
  }
}
class Ed extends Tn {
  constructor(e) {
    super(e),
      (this.isExposing = !1),
      (this.lastExposeTime = 0),
      (this.firstExposeId = ""),
      (this.context = e.context),
      (this.rootName = e.rootName),
      (this.conceal = e.conceal),
      (this.once = e.once);
    const r = Ge();
    (this.exposeId = r),
      (this.firstExposeId = r),
      e.rootName &&
        e.rootName.length > 0 &&
        this.el.dataset.mlRootName !== e.rootName &&
        (this.el.dataset.mlRootName = e.rootName);
  }
  get hasExposeRoot() {
    return !!this.exposeRoot;
  }
  get exposeRootEl() {
    var e;
    return (e = this.exposeRoot) === null || e === void 0 ? void 0 : e.el;
  }
  bindExposeRoot(e) {
    var r;
    let n;
    this.rootName && (n = e.find((s) => s.name === this.rootName)),
      !n && e.catcherMap.size === 1 && (n = e.get(document.body)),
      n ||
        (n = e.get(
          (r = this.getCurrentExposeRootElement()) !== null && r !== void 0
            ? r
            : document.body
        )),
      n &&
        ((this.exposeRoot && this.exposeRoot.el === n.el) ||
          (this.exposeRoot && this.unbindExposeRoot(),
          (this.exposeRoot = n),
          n.observe(this)));
  }
  unbindExposeRoot() {
    var e;
    (e = this.exposeRoot) === null || e === void 0 || e.unObserver(this),
      (this.exposeRoot = void 0);
  }
  updateConfig(e) {
    (this.rootName = e.rootName),
      e.rootName &&
        e.rootName.length > 0 &&
        this.el.dataset.mlRootName !== e.rootName &&
        (this.el.dataset.mlRootName = e.rootName);
  }
  handleExpose() {
    var e, r;
    (this.once && this.exposeId !== this.firstExposeId) ||
      ((this.isExposing = !0),
      (this.lastExposeTime = Date.now()),
      (r =
        (e = this.context) === null || e === void 0 ? void 0 : e.reporter) ===
        null ||
        r === void 0 ||
        r.reportElementExpose(this.key, this.extInfo, this.exposeId));
  }
  settleExpose() {
    var e, r;
    if (
      (this.once && this.exposeId !== this.firstExposeId) ||
      this.lastExposeTime === 0 ||
      !this.isExposing
    )
      return;
    const n = Date.now() - this.lastExposeTime,
      s = this.exposeId;
    (this.exposeId = Ge()),
      (this.lastExposeTime = 0),
      (this.isExposing = !1),
      this.conceal &&
        ((r =
          (e = this.context) === null || e === void 0 ? void 0 : e.reporter) ===
          null ||
          r === void 0 ||
          r.reportElementConceal(this.key, n, this.extInfo, s));
  }
  destroy() {
    this.lastExposeTime && this.isExposing && this.settleExpose(),
      this.unbindExposeRoot();
  }
  getCurrentExposeRootElement() {
    const e = (r) => {
      var n;
      const s = r.parentElement || r.parentNode;
      return s != null && s.dataset
        ? s === document.body ||
          ((n = s == null ? void 0 : s.dataset) === null || n === void 0
            ? void 0
            : n.mlRoot) === Ct.TRUE
          ? s
          : e(s)
        : null;
    };
    return e(this.el);
  }
}
class yd extends Tn {
  constructor(e) {
    super(e),
      (this.name = e == null ? void 0 : e.name),
      (this.exposeTargets = []),
      (this.observer = e.observer),
      this.el.dataset.mlRoot !== Ct.TRUE && (this.el.dataset.mlRoot = Ct.TRUE),
      e != null &&
        e.name &&
        this.el.dataset.mlName !== e.name &&
        (this.el.dataset.mlName = e.name);
  }
  updateConfig(e) {
    (this.name = e.name),
      e.observer && ((this.exposeTargets = []), (this.observer = e.observer)),
      (this.el.dataset.mlRoot = Ct.TRUE),
      e != null && e.name && (this.el.dataset.mlName = e.name);
  }
  unObserver(e) {
    var r;
    (r = this.observer) === null || r === void 0 || r.unobserve(e.el);
    const n = this.exposeTargets.findIndex((s) => s === e);
    n >= 0 && this.exposeTargets.splice(n, 1);
  }
  observe(e) {
    this.exposeTargets.findIndex((r) => r.el === e.el) >= 0 ||
      (this.observer.observe(e.el), this.exposeTargets.push(e));
  }
  destroy() {
    this.observer.disconnect(),
      (this.exposeTargets = []),
      this.el.dataset.mlRoot && delete this.el.dataset.mlRoot,
      this.el.dataset.mlName && delete this.el.dataset.mlName;
  }
}
var se;
(function (t) {
  (t[(t.INIT = 0)] = "INIT"),
    (t[(t.PLAYING = 1)] = "PLAYING"),
    (t[(t.FINISHED = 2)] = "FINISHED"),
    (t[(t.PAUSING = 3)] = "PAUSING");
})(se || (se = {}));
class bd extends vd {
  constructor() {
    super(), this.init([se.INIT, se.PLAYING, se.FINISHED, se.PAUSING], se.INIT);
  }
}
class wd extends Tn {
  constructor(e) {
    super(e),
      (this.playId = Ge()),
      (this.playCount = 0),
      (this.maxTime = 0),
      (this.finiteStateMachine = new bd()),
      (this.currentTime = 0),
      (this.gapTime = 1e3),
      (this.context = e.context),
      (this.waiting = e.waiting),
      this.startMonitor();
  }
  destroy() {
    this.stopMonitor();
  }
  updateConfig() {}
  startMonitor() {
    this.listenVideoPause(),
      this.listenVideoFinished(),
      this.listenVideoStart(),
      this.waiting && this.listenVideoWaiting(),
      this.listenVideoTimeUpdate();
  }
  stopMonitor() {
    this.stopListenVideoPause(),
      this.stopListenVideoStart(),
      this.stopListenVideoFinished(),
      this.stopListenVideoWaiting(),
      this.stopListenVideoTimeUpdate();
  }
  listenVideoTimeUpdate() {
    (this.handleVideoTimeUpdate = (e) => {
      const r = e.target,
        { currentTime: n } = r;
      n > this.maxTime && (this.maxTime = n), (this.currentTime = n);
    }),
      this.el.addEventListener("timeupdate", this.handleVideoTimeUpdate);
  }
  stopListenVideoTimeUpdate() {
    this.handleVideoTimeUpdate &&
      this.el.removeEventListener("timeupdate", this.handleVideoTimeUpdate);
  }
  listenVideoStart() {
    (this.handleVideoPlay = (e) => {
      var r;
      const n = e.target,
        { currentTime: s } = n;
      this.finiteStateMachine.switchTo(se.PLAYING),
        (this.playCount += 1),
        (r = this.context.reporter) === null ||
          r === void 0 ||
          r.reportVideoPlayStart(
            this.key,
            this.playId,
            this.playCount,
            s,
            this.extInfo
          );
    }),
      (this.handleVideoSeekingForVideoLoopStart = (e) => {
        var r;
        const n = e.target,
          { currentTime: s, duration: i } = n,
          o = this.el.hasAttribute("loop");
        i - this.currentTime < 1 &&
          s === 0 &&
          this.finiteStateMachine.getCurrent() === se.PLAYING &&
          o &&
          ((this.playCount += 1),
          (r = this.context.reporter) === null ||
            r === void 0 ||
            r.reportVideoPlayStart(
              this.key,
              this.playId,
              this.playCount,
              s,
              this.extInfo
            ));
      }),
      this.el.addEventListener("play", this.handleVideoPlay),
      this.el.addEventListener(
        "seeking",
        this.handleVideoSeekingForVideoLoopStart
      );
  }
  stopListenVideoStart() {
    this.handleVideoPlay &&
      this.el.removeEventListener("play", this.handleVideoPlay),
      this.handleVideoSeekingForVideoLoopStart &&
        this.el.removeEventListener(
          "seeking",
          this.handleVideoSeekingForVideoLoopStart
        );
  }
  listenVideoWaiting() {
    (this.handleVideoWaiting = (e) => {
      var r;
      const n = e.target,
        { currentTime: s } = n;
      (r = this.context.reporter) === null ||
        r === void 0 ||
        r.reportVideoPlayWaiting(this.key, this.playId, s, this.extInfo);
    }),
      this.el.addEventListener("waiting", this.handleVideoWaiting);
  }
  stopListenVideoWaiting() {
    this.handleVideoWaiting &&
      this.el.removeEventListener("waiting", this.handleVideoWaiting);
  }
  listenVideoFinished() {
    const e = () => {
      (this.playId = Ge()),
        (this.playCount = 0),
        (this.maxTime = 0),
        this.finiteStateMachine.resetTime(se.PLAYING);
    };
    (this.handleVideoStop = (r) => {
      var n;
      const s = r.target,
        { currentTime: i } = s;
      this.finiteStateMachine.switchTo(se.FINISHED);
      const o = this.finiteStateMachine.getTime(se.PLAYING);
      (n = this.context.reporter) === null ||
        n === void 0 ||
        n.reportVideoPlayFinish(
          this.key,
          this.playId,
          i,
          this.maxTime,
          o,
          this.extInfo
        ),
        e();
    }),
      (this.handleVideoSeekingForVideoLoopFinished = (r) => {
        var n;
        const s = r.target,
          { currentTime: i, duration: o } = s,
          a = this.el.hasAttribute("loop");
        if (
          o - this.currentTime < 1 &&
          i === 0 &&
          this.finiteStateMachine.getCurrent() === se.PLAYING &&
          a
        ) {
          this.finiteStateMachine.switchTo(se.FINISHED);
          const l = this.finiteStateMachine.getTime(se.PLAYING);
          (n = this.context.reporter) === null ||
            n === void 0 ||
            n.reportVideoPlayFinish(
              this.key,
              this.playId,
              i,
              this.maxTime,
              l,
              this.extInfo
            ),
            e(),
            this.finiteStateMachine.switchTo(se.PLAYING);
        }
      }),
      this.el.addEventListener("ended", this.handleVideoStop),
      this.el.addEventListener(
        "seeking",
        this.handleVideoSeekingForVideoLoopFinished
      );
  }
  stopListenVideoFinished() {
    var e;
    this.handleVideoStop &&
      this.el.removeEventListener("ended", this.handleVideoStop),
      this.handleVideoSeekingForVideoLoopFinished &&
        this.el.removeEventListener(
          "seeking",
          this.handleVideoSeekingForVideoLoopFinished
        );
    const r = this.finiteStateMachine.getCurrent();
    r &&
      [se.PLAYING, se.PAUSING].includes(r) &&
      ((e = this.context.reporter) === null ||
        e === void 0 ||
        e.reportVideoPlayFinish(
          this.key,
          this.playId,
          this.currentTime,
          this.maxTime,
          this.finiteStateMachine.getTime(se.PLAYING, !0),
          this.extInfo
        )),
      this.finiteStateMachine.switchTo(se.FINISHED),
      (this.playId = Ge()),
      (this.playCount = 0);
  }
  listenVideoPause() {
    (this.handleVideoPause = (e) => {
      const r = e.target,
        { currentTime: n } = r;
      this.finiteStateMachine.switchTo(se.PAUSING),
        setTimeout(() => {
          var s;
          this.finiteStateMachine.getCurrent() === se.PAUSING &&
            ((s = this.context.reporter) === null ||
              s === void 0 ||
              s.reportVideoPlayPause(this.key, this.playId, n, this.extInfo));
        }, this.gapTime);
    }),
      this.el.addEventListener("pause", this.handleVideoPause);
  }
  stopListenVideoPause() {
    this.handleVideoPause &&
      this.el.removeEventListener("pause", this.handleVideoPause);
  }
}
class Dr {
  constructor(e) {
    (this.Catcher = e), (this.catcherMap = new Map());
  }
  register(e, r, n) {
    const s = this.catcherMap.get(e);
    if (s) return s.updateExtInfo(r), n && s.updateConfig(n), s;
    const i = new this.Catcher(
      Object.assign(
        {
          el: e,
          extInfo: r,
        },
        n
      )
    );
    return this.catcherMap.set(e, i), i;
  }
  deregister(e) {
    const r = this.catcherMap.get(e);
    r && (r.destroy(), this.catcherMap.delete(e));
  }
  get(e) {
    return this.catcherMap.get(e);
  }
  has(e) {
    return this.catcherMap.has(e);
  }
  find(e) {
    for (const r of this.catcherMap.values()) if (e(r)) return r;
  }
  settle() {
    this.catcherMap.forEach((e, r) => {
      e.destroy(), this.catcherMap.delete(r);
    });
  }
}
class _d extends Sn {
  constructor(e) {
    super(),
      (this.isExposeKickStarted = !1),
      (this.exposeTargetMap = new Dr(Ed)),
      (this.exposeRootMap = new Dr(yd)),
      (this.clickTargetMap = new Dr(xd)),
      (this.videoTargetMap = new Dr(wd)),
      (this.garbageMap = new WeakMap()),
      (this.handleBuffer = new Map()),
      (this.rootMutationNodeBuffer = new Map()),
      (this.mutationProcessList = []),
      (this.context = {}),
      (this.mutationObserver = new MutationObserver(
        this.mutationCallback.bind(this)
      ));
    const { expose: r, video: n } = e || {},
      {
        once: s = !1,
        conceal: i = !1,
        viewportAsDefaultRoot: o = !1,
      } = r || {},
      { waiting: a = !1 } = n || {};
    (this.config = {
      expose: {
        once: s,
        conceal: i,
        viewportAsDefaultRoot: o,
      },
      video: {
        waiting: a,
      },
    }),
      (this.batch = this.mutationBatch.bind(this));
  }
  afterInit() {
    (this.context.reporter = this.merlin),
      this.listenClickEvent(),
      this.mutationObserver.observe(document.body, {
        childList: !0,
        subtree: !0,
      });
  }
  settle() {
    super.settle(), this.videoTargetMap.settle(), this.exposeTargetMap.settle();
  }
  click(e, r, n) {
    this.clickTargetMap.register(e, r, {
      context: this.context,
      localListener: !!(n != null && n.localListener),
    });
  }
  deleteClick(e) {
    this.clickTargetMap.deregister(e);
  }
  video(e, r) {
    this.videoTargetMap.register(e, r, {
      context: this.context,
      waiting: this.config.video.waiting,
    });
  }
  deleteVideo(e) {
    this.videoTargetMap.deregister(e);
  }
  expose(e, r) {
    if (!lo) return;
    const {
        extInfo: n,
        rootName: s,
        once: i = this.config.expose.once,
        conceal: o = this.config.expose.conceal,
      } = r || {},
      a = this.exposeTargetMap.register(e, n, {
        rootName: s,
        context: {
          reporter: this.merlin,
        },
        once: i,
        conceal: o,
      });
    this.isExposeKickStarted && a.bindExposeRoot(this.exposeRootMap);
  }
  deleteExpose(e) {
    this.exposeTargetMap.deregister(e);
  }
  exposeRoot(e, r) {
    if (!lo) return;
    const { name: n, exposeConfig: s } = r != null ? r : {},
      i = this.exposeRootMap.register(
        e,
        void 0,
        this.exposeRootMap.has(e)
          ? {
              name: n,
            }
          : {
              name: n,
              observer: new IntersectionObserver(
                (o) => {
                  o.forEach((a) => {
                    const l = this.exposeTargetMap.get(a.target);
                    l &&
                      (a.isIntersecting ? l.handleExpose() : l.settleExpose());
                  });
                },
                this.config.expose.viewportAsDefaultRoot
                  ? Object.assign(
                      {
                        root: e === document.body ? null : e,
                        threshold: 0.2,
                      },
                      s
                    )
                  : Object.assign(
                      {
                        root: e,
                        threshold: 0.2,
                      },
                      s
                    )
              ),
            }
      );
    this.isExposeKickStarted &&
      this.exposeTargetMap.catcherMap.forEach((o) => {
        !o.rootName && ao(i.el, o.el)
          ? (!o.hasExposeRoot || !gd(i.el, o.exposeRootEl)) &&
            o.bindExposeRoot(this.exposeRootMap)
          : o.rootName === i.name &&
            ao(i.el, o.el) &&
            o.bindExposeRoot(this.exposeRootMap);
      });
  }
  deleteExposeRoot(e) {
    this.exposeRootMap.deregister(e);
  }
  kickStartExpose() {
    this.exposeRoot(document.body, {
      name: "body",
    }),
      this.exposeTargetMap.catcherMap.forEach((e) => {
        e.bindExposeRoot(this.exposeRootMap);
      }),
      (this.isExposeKickStarted = !0);
  }
  listenClickEvent() {
    document.body.addEventListener("click", (e) => {
      this.clickTargetMap.catcherMap.forEach((r) => {
        var n;
        const s = e.target;
        r.el.contains(s) &&
          ((n = this.merlin) === null ||
            n === void 0 ||
            n.reportElementClick(r.key, r.extInfo));
      });
    });
  }
  processRemovedNodes(e) {
    const r = e.length;
    for (let n = 0; n < r; n++) {
      const s = e[n];
      s instanceof HTMLElement &&
        (this.rootMutationNodeBuffer.has(s) ||
          this.rootMutationNodeBuffer.set(s, "removed"));
    }
  }
  processAddedNodes(e) {
    const r = e.length;
    for (let n = 0; n < r; n++) {
      const s = e[n];
      s instanceof HTMLElement &&
        (this.rootMutationNodeBuffer.has(s) ||
          this.rootMutationNodeBuffer.set(s, "added"));
    }
  }
  processMutationBuffer() {
    const e = Array.from(this.rootMutationNodeBuffer.keys());
    for (let r = e.length - 1; r > -1; r--) {
      const n = e[r];
      if (this.rootMutationNodeBuffer.get(n) === "removed")
        this.videoTargetMap.catcherMap.forEach((s, i) => {
          n.contains(i) && this.handleBuffer.set(i, !1);
        });
      else {
        let s;
        n instanceof HTMLVideoElement
          ? (s = [n])
          : (s = Array.prototype.slice.call(n.getElementsByTagName("video"))),
          s.forEach((i) => {
            (this.handleBuffer.has(i) || this.garbageMap.has(i)) &&
              this.handleBuffer.set(i, !0);
          });
      }
    }
    this.rootMutationNodeBuffer.clear();
  }
  processHandleBuffer() {
    this.handleBuffer.forEach((e, r) => {
      if (r instanceof HTMLVideoElement)
        if (e) {
          const n = this.garbageMap.get(r);
          n &&
            (this.videoTargetMap.catcherMap.set(r, n),
            n.startMonitor(),
            this.garbageMap.delete(r));
        } else {
          const n = this.videoTargetMap.get(r);
          n &&
            (this.videoTargetMap.catcherMap.delete(r),
            n.stopMonitor(),
            this.garbageMap.set(r, n));
        }
    }),
      this.handleBuffer.clear();
  }
  mutationBatch() {
    for (let e = this.mutationProcessList.length - 1; e > -1; e--) {
      const { removedNodes: r, addedNodes: n } = this.mutationProcessList[e];
      r.length && this.processRemovedNodes(r),
        n.length && this.processAddedNodes(n);
    }
    this.processMutationBuffer(),
      this.processHandleBuffer(),
      (this.mutationProcessList.length = 0);
  }
  mutationCallback(e) {
    this.mutationProcessList.length || requestAnimationFrame(this.batch),
      this.mutationProcessList.push(...e);
  }
}
function Sd(t, e) {
  In(t, "ml-click", {
    mounted(r, n, s) {
      var i;
      const o = (i = Gt(s)) === null || i === void 0 ? void 0 : i["ml-extends"];
      e.click(r, o, {
        localListener: !!n.modifiers.local,
      });
    },
    updated(r, n, s) {
      var i;
      const o = (i = Gt(s)) === null || i === void 0 ? void 0 : i["ml-extends"];
      e.click(r, o, {
        localListener: !!n.modifiers.local,
      });
    },
    unmounted(r) {
      e.deleteClick(r);
    },
  });
}
function Id(t, e) {
  In(t, "ml-expose", {
    mounted(r, n, s) {
      var i;
      const o = (i = Gt(s)) === null || i === void 0 ? void 0 : i["ml-extends"],
        a = n.arg,
        l = n.modifiers["not-once"] ? !1 : n.modifiers.once,
        c = n.modifiers["not-conceal"] ? !1 : n.modifiers.conceal;
      e.expose(r, {
        extInfo: o,
        rootName: a,
        once: l,
        conceal: c,
      });
    },
    updated(r, n, s) {
      var i;
      const o = (i = Gt(s)) === null || i === void 0 ? void 0 : i["ml-extends"],
        a = n.arg,
        l = n.modifiers["not-once"] ? !1 : n.modifiers.once,
        c = n.modifiers["not-conceal"] ? !1 : n.modifiers.conceal;
      e.expose(r, {
        extInfo: o,
        rootName: a,
        once: l,
        conceal: c,
      });
    },
    unmounted(r) {
      e.deleteExpose(r);
    },
  });
}
function Td(t, e) {
  In(t, "ml-exposeRoot", {
    mounted(r, n) {
      const s = n.value,
        i = "".concat(n.arg);
      e.exposeRoot(r, {
        name: i,
        exposeConfig: s,
      });
    },
    unmounted(r) {
      e.deleteExposeRoot(r);
    },
  });
}
function Cd(t, e) {
  In(t, "ml-video", {
    mounted(r, n, s) {
      var i;
      const o = (i = Gt(s)) === null || i === void 0 ? void 0 : i["ml-extends"];
      e.video(r, o);
    },
    updated(r, n, s) {
      var i;
      const o = (i = Gt(s)) === null || i === void 0 ? void 0 : i["ml-extends"];
      e.video(r, o);
    },
    unmounted(r) {
      e.deleteVideo(r);
    },
  });
}
const Rd = (t) => {
  const { options: e } = t;
  let r = !0;
  const n = (s) => {
    if (r) {
      if (!s.name) {
        r = !1;
        return;
      }
      s.children &&
        s.children.forEach((i) => {
          n(i);
        });
    }
  };
  return (
    e.routes &&
      e.routes.forEach((s) => {
        n(s);
      }),
    r
  );
};
class Gp extends _d {
  constructor(e) {
    super(e), (this.vue = e.vue), (this.router = e == null ? void 0 : e.router);
  }
  bindVue(e, r) {
    (this.vue = e), r && (this.router = r);
  }
  afterInit() {
    super.afterInit(),
      this.vue &&
        (Id(this.vue, this),
        Td(this.vue, this),
        Cd(this.vue, this),
        Sd(this.vue, this)),
      this.kickStartExpose(),
      this.registerPage();
  }
  registerPage() {
    if (this.router) {
      if (!Rd(this.router))
        throw new gl("every route in Vue-router must have a name");
      this.router.afterEach((e, r) => {
        var n, s, i, o, a, l;
        const c = new Date().getTime();
        let u = c;
        try {
          if (
            (sessionStorage.setItem("".concat(e.name), c.toString()),
            r != null && r.name)
          ) {
            const v = parseFloat(
              sessionStorage.getItem("".concat(r.name)) || ""
            );
            Number.isNaN(v) || (u = v);
          }
        } catch (v) {}
        const d = c - u;
        ((n = this.merlin) === null || n === void 0
          ? void 0
          : n.core.loadPage()) &&
          ((s = this.merlin) === null || s === void 0 || s.reportPageLeave(d));
        const g = (i = e.meta) === null || i === void 0 ? void 0 : i.ml;
        (o = this.merlin) === null ||
          o === void 0 ||
          o.pushPage(
            (a = g == null ? void 0 : g.pageId) !== null && a !== void 0
              ? a
              : "".concat(e.name),
            Object.assign(
              Object.assign(Object.assign({}, e.params), e.query),
              g == null ? void 0 : g.extInfo
            )
          ),
          (l = this.merlin) === null || l === void 0 || l.reportPageEnter({});
      });
    }
  }
}
class Ad extends Tr {
  constructor(e) {
    super(
      Object.assign(
        {
          flushInterval: 0,
          bufferSize: 0,
        },
        e
      )
    ),
      (this.reportTypes = [he.BEHAVIOR, he.ERROR, he.PERFORMANCE]);
    const {
      prefix: r = "[Merlin]",
      parser: n = null,
      method: s = "log",
    } = e || {};
    (this.prefix = r), (this.parser = n), (this.method = s);
  }
  print(...e) {
    this.method === "info" || this.method;
  }
  send(e) {
    e.forEach((r) => {
      const n = "["
        .concat(r.type, "]")
        .concat(new Date(r.ctime).toLocaleString());
      if (this.parser) {
        const s = this.parser(r);
        Array.isArray(s)
          ? this.print("".concat(this.prefix).concat(n), ...s)
          : this.print("".concat(this.prefix).concat(n), s);
      } else this.print("".concat(this.prefix).concat(n), r.context, r.data);
    });
  }
}
class zp extends xl {
  get env() {
    return "web";
  }
  constructor(e) {
    const {
      collectors: r = [],
      transports: n = [],
      generalError: s,
      vueError: i,
      log: o,
      vue: a,
      pageStackStorageLimit: l = 20,
    } = e || {};
    s !== !1 && r.push(new hd()),
      i !== !1 &&
        a &&
        r.push(
          new pd({
            vue: a,
          })
        ),
      o !== !1 && n.push(new Ad(o)),
      super(
        Object.assign(Object.assign({}, e), {
          collectors: r,
          transports: n,
        })
      ),
      (this.storage = {
        supportSync: !0,
        setItem(c, u) {
          try {
            if (!(window != null && window.localStorage) || u == null) return;
            window.localStorage.setItem(c, JSON.stringify(u));
          } catch (d) {}
        },
        getItem(c, u) {
          try {
            if (!(window != null && window.localStorage)) return u;
            const d = window.localStorage.getItem(c);
            if (d == null) return u;
            try {
              return JSON.parse(d);
            } catch (p) {
              return d || u;
            }
          } catch (d) {
            return u;
          }
        },
        removeItem(c) {
          try {
            if (!(window != null && window.localStorage)) return;
            window.localStorage.removeItem(c);
          } catch (u) {}
        },
      }),
      (this.handleBeforeUnload = this.settleMerlinCore.bind(this)),
      (this.handlePageHide = this.settleMerlinCore.bind(this)),
      (this.handleOffline = this.privateHandleOffline.bind(this)),
      (this.handleOnline = this.privateHandleOnline.bind(this)),
      (this.pageStackStorageLimit = l);
  }
  afterInit() {
    window.addEventListener("beforeunload", this.handleBeforeUnload),
      window.addEventListener("pagehide", this.handlePageHide),
      window.addEventListener("online", this.handleOnline),
      window.addEventListener("offline", this.handleOffline);
  }
  initAid() {
    var e, r;
    if (!this.core) return;
    let n = this.storage.getItem(eo, void 0);
    (!n || typeof n != "string") &&
      ((n = this.aIdGenerator()), this.storage.setItem(eo, n)),
      (this.core.aid = n);
    const s =
      (r =
        (e = this.cfg) === null || e === void 0
          ? void 0
          : e.reportAidToMmdata) !== null && r !== void 0
        ? r
        : !0;
    s &&
      vl().then(() => {
        var i;
        (i = ti().WeixinJSBridge) === null ||
          i === void 0 ||
          i.invoke("kvReport", {
            id: typeof s == "number" ? s : 28530,
            value: n,
            is_important: 1,
            is_report_now: 1,
          });
      });
  }
  readLinkedData() {
    if (!this.core || !window) return;
    const e = qf(Uf());
    e.contextId && (this.core.contextId = e.contextId),
      e.entranceId && (this.ctx.entranceId = e.entranceId),
      e.subEntranceid &&
        (this.ctx.entranceInfo = {
          subEntranceid: e.subEntranceid,
        }),
      e.reddotId &&
        (this.ctx.redDotInfo = {
          id: e.reddotId,
        }),
      e.fromAccessId && (this.fromPage.accessId = e.fromAccessId),
      e.fromElementId && (this.fromPage.eleId = e.fromElementId);
  }
  get linkedData() {
    var e, r, n, s;
    return {
      contextId:
        (e = this.core) === null || e === void 0 ? void 0 : e.contextId,
      entranceId: this.ctx.entranceId,
      subEntranceid:
        (r = this.ctx.entranceInfo) === null || r === void 0
          ? void 0
          : r.subEntranceid,
      reddotId:
        (n = this.ctx.redDotInfo) === null || n === void 0 ? void 0 : n.id,
      fromAccessId:
        (s = this.currentPage) === null || s === void 0 ? void 0 : s.accessId,
    };
  }
  serializeLinkedData(e) {
    return Bf(Object.assign(Object.assign({}, this.linkedData), e));
  }
  httpPost(e, r, n) {
    return We(this, void 0, void 0, function* () {
      if (
        typeof (navigator == null ? void 0 : navigator.sendBeacon) == "function"
      )
        try {
          if (
            navigator.sendBeacon(
              e,
              new Blob([r], {
                type: n.contentType,
              })
            )
          )
            return;
        } catch (s) {}
      try {
        yield fetch(e, {
          method: "POST",
          headers: {
            "Content-Type": n.contentType,
          },
          body: r,
        });
      } catch (s) {
        this.errorHandler(s);
      }
    });
  }
  updateContext() {
    window &&
      ((this.ctx.url = window.location.href),
      (this.contextEnv.ua = window.navigator.userAgent),
      (this.contextEnv.clientHeight = window.innerHeight),
      (this.contextEnv.clientWidth = window.innerWidth),
      (this.contextEnv.screenHeight = window.screen.availHeight),
      (this.contextEnv.screenWidth = window.screen.availWidth),
      (this.contextEnv.devicePixelRatio = window.devicePixelRatio));
  }
  loadPageStackFromStorage() {
    var e;
    const r = this.storage.getItem(Ye, []);
    try {
      localStorage &&
        r &&
        Object.keys(localStorage).forEach((s) => {
          s.startsWith("".concat(Ye, "_")) &&
            !r.includes(s.replace("".concat(Ye, "_"), "")) &&
            this.storage.removeItem(s);
        });
    } catch (s) {
      this === null || this === void 0 || this.errorHandler(s);
    }
    const n = this.storage.getItem(
      this.getPageStorageKey(
        (e = this.core) === null || e === void 0 ? void 0 : e.contextId
      ),
      void 0
    );
    if (
      n &&
      typeof n == "object" &&
      Object.prototype.hasOwnProperty.call(n, "accessId") &&
      Object.prototype.hasOwnProperty.call(n, "pageId") &&
      Object.prototype.hasOwnProperty.call(n, "step")
    )
      return n;
  }
  savePageStackToStorage() {
    var e, r;
    const n = (e = this.core) === null || e === void 0 ? void 0 : e.contextId;
    if (!(!this.currentPage || !n))
      try {
        const s = this.storage.getItem(Ye, []);
        if (s.includes(n))
          s.indexOf(n) !== s.length - 1 &&
            (s.splice(s.indexOf(n), 1), s.push(n), this.storage.setItem(Ye, s));
        else {
          if (s.length >= this.pageStackStorageLimit) {
            const i = s.shift();
            i && this.storage.removeItem(this.getPageStorageKey(i));
          }
          s.push(n), this.storage.setItem(Ye, s);
        }
        this.storage.setItem(this.getPageStorageKey(n), {
          pageId: this.currentPage.pageId,
          accessId: this.currentPage.accessId,
          step: this.currentPage.step,
          refAccessId: this.currentPage.refAccessId,
          fromElementId: this.currentPage.fromElementId,
          refPageId: this.currentPage.refPageId,
        });
      } catch (s) {
        if (
          (s == null ? void 0 : s.code) === 22 ||
          (!((r = s == null ? void 0 : s.message) === null || r === void 0) &&
            r.toLowerCase().includes("quota"))
        ) {
          const i = this.storage.getItem(Ye, []),
            o = Math.floor(i.length / 2),
            a = this.recycleStorage(i, o);
          this.storage.setItem(Ye, a);
        }
      }
  }
  recycleStorage(e, r) {
    if (e.length <= r) return e;
    const n = [...e];
    return (
      n.splice(r).forEach((i) => {
        this.storage.removeItem(this.getPageStorageKey(i));
      }),
      n
    );
  }
  settleMerlinCore() {
    var e;
    (e = this.core) === null || e === void 0 || e.settle();
  }
  privateHandleOffline() {
    var e;
    (e = this.core) === null ||
      e === void 0 ||
      e.enableBuffering([he.BEHAVIOR, he.ERROR, he.PERFORMANCE]);
  }
  privateHandleOnline() {
    var e;
    (e = this.core) === null || e === void 0 || e.settleBuffering();
  }
}
class qn {
  constructor(e) {
    (this.type = e), (this.buffer = []), (this.buffering = !0);
  }
  enableBuffering(e) {
    ((Array.isArray(e) && e.includes(this.type)) || e === this.type) &&
      (this.buffering = !0);
  }
  settleBuffering(e) {
    (!e || (Array.isArray(e) && e.includes(this.type)) || e === this.type) &&
      (this.buffering = !1);
  }
  push(e) {
    e.type === this.type && this.buffer.push(e);
  }
  updatePageInfoIfBuffering(e) {
    this.buffering &&
      this.buffer.forEach((r) =>
        r.updateContext(
          Object.assign(Object.assign({}, r.context), {
            page: Object.assign(Object.assign({}, r.context.page), {
              pageInfo: e,
            }),
          })
        )
      );
  }
  consume() {
    if (this.buffering) return [];
    const e = [...this.buffer];
    return (this.buffer.length = 0), e;
  }
  get length() {
    return this.buffer.length;
  }
}
class Od {
  constructor() {
    (this.adapter = new dd()),
      (this.contextId = Ge()),
      (this.logError = !0),
      (this.aid = ""),
      (this.userInfo = {}),
      (this.release = ""),
      (this.initialized = !1),
      (this.collectors = []),
      (this.transports = []),
      (this.buffers = [
        new qn(he.BEHAVIOR),
        new qn(he.ERROR),
        new qn(he.PERFORMANCE),
      ]);
  }
  collector(e) {
    Sn.isCollector(e) &&
      (this.collectors.push(e),
      this.initialized && this.merlin && e.init(this.merlin));
  }
  transport(e) {
    Tr.isTransport(e) &&
      (this.transports.push(e),
      this.initialized && this.merlin && e.init(this.merlin));
  }
  init(e, r, n) {
    if (!this.initialized)
      try {
        (this.adapter = r),
          (this.pandora = n),
          this.adapter.init(this),
          (this.initialized = !0),
          this.collectors.forEach((s) => s.init(e)),
          this.transports.forEach((s) => s.init(e)),
          this.settleBuffering();
      } catch (s) {
        this.adapter.errorHandler(s);
      }
  }
  enableBuffering(e) {
    this.buffers.forEach((r) => r.enableBuffering(e));
  }
  settleBuffering(e) {
    return We(this, void 0, void 0, function* () {
      this.readLinkedData(), this.buffers.forEach((r) => r.settleBuffering(e));
      try {
        yield this.flush();
      } catch (r) {
        this.adapter.errorHandler(r);
      }
    });
  }
  report(e) {
    if (this.initialized)
      try {
        this.adapter.updateContext(),
          e.updateContext(this.context),
          this.buffers.forEach((r) => r.push(e)),
          this.flush();
      } catch (r) {
        this.adapter.errorHandler(r, e);
      }
  }
  readLinkedData() {
    this.adapter.readLinkedData();
  }
  get objectLinkedData() {
    var e;
    return Object.assign(
      {
        fromAccessId:
          (e = this.adapter.currentPage) === null || e === void 0
            ? void 0
            : e.accessId,
      },
      this.adapter.linkedData
    );
  }
  get serializeLinkedData() {
    var e, r;
    return (
      ((e = this.adapter) === null || e === void 0
        ? void 0
        : e.serializeLinkedData({
            fromAccessId:
              (r = this.adapter.currentPage) === null || r === void 0
                ? void 0
                : r.accessId,
          })) || ""
    );
  }
  loadPage() {
    return this.adapter.loadPageStackFromStorage();
  }
  pushPage(...e) {
    this.adapter.pushPage(...e);
  }
  getPage() {
    return this.adapter.currentPage;
  }
  updatePage(...e) {
    this.adapter.updatePageInfo(...e),
      this.buffers.forEach((r) => {
        var n;
        return r.updatePageInfoIfBuffering(
          ((n = this.adapter.currentPage) === null || n === void 0
            ? void 0
            : n.pageInfo) || {}
        );
      });
  }
  get supportSyncStorage() {
    return this.adapter.storage.supportSync;
  }
  getStorageItem(...e) {
    var r;
    return (r = this.adapter.storage) === null || r === void 0
      ? void 0
      : r.getItem(...e);
  }
  setStorageItem(...e) {
    var r;
    (r = this.adapter.storage) === null || r === void 0 || r.setItem(...e);
  }
  removeStorageItem(...e) {
    var r;
    (r = this.adapter.storage) === null || r === void 0 || r.removeItem(...e);
  }
  httpPost(...e) {
    this.adapter.httpPost(...e);
  }
  get env() {
    return this.adapter.env;
  }
  settle() {
    return We(this, void 0, void 0, function* () {
      try {
        this.collectors.forEach((e) => e.settle()), yield this.flush(!0);
      } catch (e) {
        this.adapter.errorHandler(e);
      }
    });
  }
  flush(e = !1) {
    return We(this, void 0, void 0, function* () {
      const r = this.buffers.flatMap((n) => n.consume());
      return this.sendToTransports(r, e);
    });
  }
  get context() {
    return Object.assign(
      {
        contextId: this.contextId,
        aid: this.aid,
        userInfo: this.userInfo,
        release: this.release,
        env: this.adapter.contextEnv,
        page: this.adapter.contextPage,
      },
      this.adapter.ctx
    );
  }
  sendToTransports(e, r = !1) {
    return Promise.all(this.transports.map((n) => n.receiveFromCore(e, r)));
  }
}
class Pd {
  constructor(e = new Od()) {
    (this.core = e), (this.running = !1);
  }
  init(e, r) {
    this.running || ((this.running = !0), this.core.init(this, e, r));
  }
  setUserInfo(e) {
    this.core.userInfo = e;
  }
  collector(e) {
    this.core.collector(e);
  }
  transport(e) {
    this.core.transport(e);
  }
  enableBuffering(e) {
    this.core.enableBuffering(e);
  }
  settleBuffering(e) {
    return We(this, void 0, void 0, function* () {
      this.core.settleBuffering(e);
    });
  }
  get linkedData() {
    return this.core.objectLinkedData;
  }
  encodeContextUrl(e) {
    const { serializeLinkedData: r } = this.core;
    return r
      ? e.includes("?")
        ? e.endsWith("&")
          ? "".concat(e).concat(r)
          : "".concat(e, "&").concat(r)
        : "".concat(e, "?").concat(r)
      : e;
  }
  pushPage(e, r) {
    this.core.pushPage(e, r);
  }
  updatePage(e) {
    this.core.updatePage(e);
  }
  getPage() {
    return this.core.getPage();
  }
  settle() {
    return We(this, void 0, void 0, function* () {
      yield this.core.settle();
    });
  }
  catch(e, r) {
    var n;
    let s =
      ((n = this.core.pandora) === null || n === void 0
        ? void 0
        : n.catch(e)) || e;
    const i = {
      log: r == null ? void 0 : r.log,
      idx1: r == null ? void 0 : r.idx1,
      idx2: r == null ? void 0 : r.idx2,
      idx3: r == null ? void 0 : r.idx3,
    };
    if ((s instanceof mt && (s = s.error), s instanceof At)) {
      const { origin: a } = s;
      if (
        ((i.name = s.name),
        (i.message = s.message),
        (i.eExtra = "ts:".concat(s.timestamp)),
        (i.stack = s.stack),
        (i.file = a == null ? void 0 : a.file),
        (i.line = a == null ? void 0 : a.line),
        (i.col = a == null ? void 0 : a.col),
        s.merlin)
      ) {
        const { idx1: l, idx2: c, idx3: u, extra: d } = s.merlin;
        (i.eIdx1 = l),
          (i.eIdx2 = c),
          (i.eIdx3 = u),
          d && (i.eExtra += "|".concat(d));
      }
    } else if (((i.eExtra = "ts:".concat(Date.now())), s instanceof Error))
      (i.name = "UnknownError"),
        (i.message = ""
          .concat(s.name ? "".concat(s.name, ":") : "")
          .concat(s.message)),
        (i.stack = s.stack);
    else {
      i.name = "UnknownObject";
      try {
        i.message =
          typeof s == "object"
            ? "".concat(s.constructor.name, ":").concat(Object.keys(s))
            : "".concat(s);
      } catch (a) {}
    }
    const o = ld(i);
    this.core.report(new $f(o));
  }
  try(e, r, n) {
    try {
      return e();
    } catch (s) {
      return this.catch(s, r), n === void 0 ? null : n;
    }
  }
  reportPerf(e, r, n, s) {
    const i = s == null ? void 0 : s.sampleRate;
    (i !== void 0 && (i <= 0 || (i < 1 && Math.random() >= i))) ||
      this.core.report(
        new ro({
          type: e,
          action: r,
          duration: (n == null ? void 0 : n.duration) || 0,
          status: ro.parseStatus(n == null ? void 0 : n.status),
          log: n == null ? void 0 : n.log,
          idx1: n == null ? void 0 : n.idx1,
          idx2: n == null ? void 0 : n.idx2,
          idx3: n == null ? void 0 : n.idx3,
        })
      );
  }
  createPerfTransaction(e, r, n) {
    return new Ff(this, e, r, n);
  }
  reportCustomBehavior(e, r, n) {
    this.core.report(
      new Me({
        behaviorType: Ie.CUSTOM,
        eleId: n,
        customType: e,
        eleInfo: r,
      })
    );
  }
  reportElementClick(e, r) {
    this.core.report(
      new Me({
        behaviorType: Ie.ELEMENT_CLICK,
        eleId: e,
        eleInfo: r,
      })
    );
  }
  reportElementHover(e, r) {
    this.core.report(
      new Me({
        behaviorType: Ie.ELEMENT_HOVER,
        eleId: e,
        eleInfo: r,
      })
    );
  }
  reportElementExpose(e, r, n) {
    this.core.report(
      new Me({
        behaviorType: Ie.ELEMENT_EXPOSE,
        eleId: e,
        eleInfo: r,
        exposeId: n || "",
      })
    );
  }
  reportElementConceal(e, r, n, s) {
    this.core.report(
      new Me({
        behaviorType: Ie.ELEMENT_CONCEAL,
        eleId: e,
        eleInfo: n,
        exposeId: s || "",
        allTime: r,
      })
    );
  }
  reportPageEnter(e) {
    this.core.report(
      new Me({
        behaviorType: Ie.PAGE_ENTER,
        eleInfo: e,
      })
    );
  }
  reportPageLeave(e, r) {
    this.core.report(
      new Me({
        behaviorType: Ie.PAGE_LEAVE,
        eleInfo: r,
        stayTime: e,
      })
    );
  }
  reportVideoPlayStart(e, r, n, s, i) {
    this.core.report(
      new Me({
        behaviorType: Ie.VIDEO_PLAY_START,
        eleInfo: i,
        eleId: e,
        playId: r,
        playCount: n,
        currentTime: s,
      })
    );
  }
  reportVideoPlayWaiting(e, r, n, s) {
    this.core.report(
      new Me({
        behaviorType: Ie.VIDEO_PLAY_WAITING,
        eleInfo: s,
        eleId: e,
        playId: r,
        currentTime: n,
      })
    );
  }
  reportVideoPlayPause(e, r, n, s) {
    this.core.report(
      new Me({
        behaviorType: Ie.VIDEO_PLAY_PAUSE,
        eleInfo: s,
        eleId: e,
        playId: r,
        currentTime: n,
      })
    );
  }
  reportVideoPlaySeek(e, r, n, s, i) {
    this.core.report(
      new Me({
        behaviorType: Ie.VIDEO_PLAY_SEEK,
        eleInfo: i,
        eleId: e,
        playId: r,
        position: s,
        currentTime: n,
      })
    );
  }
  reportVideoPlayFinish(e, r, n, s, i, o) {
    this.core.report(
      new Me({
        behaviorType: Ie.VIDEO_PLAY_FINISH,
        eleInfo: o,
        eleId: e,
        playId: r,
        currentTime: n,
        maxTime: s,
        allTime: i,
      })
    );
  }
}
function zt(t) {
  var e, r;
  return t
    ? ((r =
        (e = t.split("#")) === null || e === void 0
          ? void 0
          : e[0].split("?")) === null || r === void 0
        ? void 0
        : r[0]) || t
    : "";
}
function Md(t, e) {
  var r, n;
  if (!t) return "";
  const s =
    (n =
      (r = t.split("#")) === null || r === void 0
        ? void 0
        : r[0].split("?")) === null || n === void 0
      ? void 0
      : n[1];
  if (s) {
    const i = new URLSearchParams(s).get(e);
    if (i) return "".concat(zt(t), "?").concat(e, "=").concat(i);
  }
  return zt(t);
}
function Ld(t, e) {
  var r, n;
  if (!t) return "";
  const s =
    (n =
      (r = t.split("#")) === null || r === void 0
        ? void 0
        : r[0].split("?")) === null || n === void 0
      ? void 0
      : n[1];
  if (s) {
    const i = new URLSearchParams(s),
      o = [];
    if (
      (e.forEach((a) => {
        const l = i.get(a);
        l && o.push([a, l]);
      }),
      o.length)
    )
      return ""
        .concat(zt(t), "?")
        .concat(o.map(([a, l]) => "".concat(a, "=").concat(l)).join("&"));
  }
  return zt(t);
}
function El(t) {
  let e = zt;
  return (
    typeof t == "string"
      ? (e = (r) => Md(r, t))
      : Array.isArray(t)
      ? (e = (r) => Ld(r, t))
      : typeof t == "function" && (e = t),
    e
  );
}
function Nd(t, e) {
  var r;
  if (!t) return "";
  const [n, s] = t.split("#"),
    i = (r = n.split("?")) === null || r === void 0 ? void 0 : r[1];
  if (i) {
    const o = new URLSearchParams(i);
    e.forEach((c) => {
      o.delete(c);
    });
    const a = o.toString();
    let l = zt(t);
    return a && (l += "?".concat(a)), s && (l += "#".concat(s)), l;
  }
  return t;
}
function kd(t, e, r) {
  const n = t.map((s) => {
    const i = s.filename === r ? "__FILE__" : s.filename;
    return s.func === "?"
      ? "    at ".concat(i, ":").concat(s.lineno, ":").concat(s.colno)
      : "    at "
          .concat(s.func, " (")
          .concat(i, ":")
          .concat(s.lineno, ":")
          .concat(s.colno, ")");
  });
  return "".concat(e ? "".concat(e, "\n") : "").concat(n.join("\n"));
}
function te(t) {
  if (typeof t == "string") return t;
  if (t !== void 0) return "".concat(t);
}
class Jp extends Tr {
  constructor(e) {
    const r = !e.url.includes("pf=");
    super(
      Object.assign(
        {
          bufferSize: r ? 0 : 5,
          flushInterval: r ? 0 : 500,
          frequencyLimit: {
            max: 25,
            perSeconds: 900,
          },
        },
        e
      )
    ),
      (this.reportTypes = [he.ERROR]),
      (this.referMapper = (a) => a),
      (this.thisSend = this.sendNew.bind(this));
    const { url: n, referFilter: s, pageMapper: i, uriMapper: o } = e;
    r && (this.thisSend = this.sendOld.bind(this)),
      (this.url = n),
      (this.pageMapper = El(i || o)),
      s === !0
        ? (this.referMapper = () => "")
        : Array.isArray(s)
        ? (this.referMapper = (a) => Nd(a, s))
        : typeof s == "function" && (this.referMapper = s),
      (this.userTokenMapper = e.userTokenMapper || (() => {}));
  }
  sendOld(e) {
    e.map((n) => {
      const { aid: s, release: i, appId: o } = n.context;
      let { stack: a } = n.data;
      const { eIdx1: l } = n.data;
      return (
        l && !a && (a = l),
        o && (a += "\nappId:".concat(o)),
        i && (a += "\nr:".concat(i)),
        s && (a += "\naid:".concat(s)),
        {
          stack: a,
          error_msg: n.data.message,
          error_type: n.data.name,
          page_uri: this.pageMapper(n.context.url),
        }
      );
    }).forEach((n) => {
      var s;
      (s = this.reporter) === null ||
        s === void 0 ||
        s.core.httpPost(this.url, JSON.stringify(n), {
          contentType: "application/json",
        });
    });
  }
  sendNew(e) {
    var r;
    const n = e.map((s) => {
      var i, o;
      const { data: a, context: l } = s,
        { aid: c, release: u, env: d, url: p, appId: g } = l,
        v = {
          release: te(u),
          msg: te(a.message),
          refer: this.referMapper(p),
          page: this.pageMapper(p),
          user_token: this.userTokenMapper(p),
          aid: te(c),
          type: te(a.name),
          idx1: te(a.idx1),
          idx2: te(a.idx2),
          idx3: te(a.idx3),
          log: te(a.log),
          e_idx1: te(a.eIdx1),
          e_idx2: te(a.eIdx2),
          e_idx3: te(a.eIdx3),
          extra: te(a.eExtra),
          fingerprint: "",
        };
      return (
        ((i = this.reporter) === null || i === void 0 ? void 0 : i.core.env) ===
        "lite"
          ? ((v.stack = a.stack),
            (v.client_ver = d.clientVer),
            (v.lib = d.lib),
            (v.network = d.network),
            (v.os = d.os),
            (v.os_ver = d.osVer),
            (v.brand = d.brand),
            (v.model = d.model),
            (v.app_id = g),
            (v.ua = d.ua))
          : (a.file !== void 0 && (v.fingerprint += "".concat(a.file)),
            a.line !== void 0 && (v.fingerprint += ":".concat(a.line)),
            a.col !== void 0 && (v.fingerprint += ":".concat(a.col)),
            (v.file = te(a.file)),
            (v.line = te(a.line)),
            (v.col = te(a.col)),
            (v.stack =
              !((o = a.stackFrames) === null || o === void 0) && o.length
                ? kd(a.stackFrames, a.message, a.file)
                : a.stack)),
        v
      );
    });
    (r = this.reporter) === null ||
      r === void 0 ||
      r.core.httpPost(
        this.url,
        JSON.stringify({
          items: n,
        }),
        {
          contentType: "application/json",
        }
      );
  }
  send(e) {
    this.thisSend(e);
  }
}
class Yp extends Tr {
  constructor(e) {
    super(
      Object.assign(
        {
          bufferSize: 10,
          flushInterval: 3e3,
        },
        e
      )
    ),
      (this.reportTypes = [he.PERFORMANCE]);
    const { url: r, pageMapper: n } = e;
    (this.url = r), (this.pageMapper = El(n));
  }
  send(e) {
    var r;
    const n = e.map((s) => {
      var i;
      const { data: o, context: a } = s,
        { release: l, aid: c, env: u, appId: d, url: p } = a,
        g = {
          release: te(l),
          type: te(o.type),
          action: te(o.action),
          status: te(o.status),
          duration: Math.round(o.duration) || 0,
          aid: te(c),
          idx1: te(o.idx1),
          idx2: te(o.idx2),
          idx3: te(o.idx3),
          log: te(o.log),
          page: this.pageMapper(p),
        };
      return (
        ((i = this.reporter) === null || i === void 0 ? void 0 : i.core.env) ===
          "lite" &&
          ((g.client_ver = u.clientVer),
          (g.lib = u.lib),
          (g.network = u.network),
          (g.os = u.os),
          (g.os_ver = u.osVer),
          (g.brand = u.brand),
          (g.model = u.model),
          (g.app_id = d),
          (g.ua = u.ua)),
        g
      );
    });
    (r = this.reporter) === null ||
      r === void 0 ||
      r.core.httpPost(
        this.url,
        JSON.stringify({
          items: n,
        }),
        {
          contentType: "application/json",
        }
      );
  }
}
class Xp extends Tr {
  constructor(e) {
    super(
      Object.assign(
        {
          bufferSize: 0,
          flushInterval: 0,
        },
        e
      )
    ),
      (this.reportTypes = [he.BEHAVIOR]),
      (this.logId = e.logId),
      (this.parser = e.parser);
  }
  send(e) {
    var r;
    return We(this, void 0, void 0, function* () {
      const n =
        (r = this.reporter) === null || r === void 0 ? void 0 : r.core.env;
      return (
        n === "web" && (yield vl()),
        e.map((s) => {
          var i;
          const o = this.parser(s);
          if (o === !1) return Promise.resolve();
          const a = cd(o);
          return (
            n === "lite"
              ? lite.reporter.reportKv(this.logId, a)
              : (i = ti().WeixinJSBridge) === null ||
                i === void 0 ||
                i.invoke("kvReport", {
                  id: this.logId,
                  value: a,
                  is_important: 1,
                  is_report_now: 1,
                }),
            Promise.resolve()
          );
        })
      );
    });
  }
}
const co = new Pd();
var ri = {
    exports: {},
  },
  yl = function (e, r) {
    return function () {
      for (var s = new Array(arguments.length), i = 0; i < s.length; i++)
        s[i] = arguments[i];
      return e.apply(r, s);
    };
  },
  Vd = yl,
  Pt = Object.prototype.toString;
function ni(t) {
  return Pt.call(t) === "[object Array]";
}
function Cs(t) {
  return typeof t > "u";
}
function Dd(t) {
  return (
    t !== null &&
    !Cs(t) &&
    t.constructor !== null &&
    !Cs(t.constructor) &&
    typeof t.constructor.isBuffer == "function" &&
    t.constructor.isBuffer(t)
  );
}
function $d(t) {
  return Pt.call(t) === "[object ArrayBuffer]";
}
function Fd(t) {
  return typeof FormData < "u" && t instanceof FormData;
}
function Ud(t) {
  var e;
  return (
    typeof ArrayBuffer < "u" && ArrayBuffer.isView
      ? (e = ArrayBuffer.isView(t))
      : (e = t && t.buffer && t.buffer instanceof ArrayBuffer),
    e
  );
}
function jd(t) {
  return typeof t == "string";
}
function Hd(t) {
  return typeof t == "number";
}
function bl(t) {
  return t !== null && typeof t == "object";
}
function Wr(t) {
  if (Pt.call(t) !== "[object Object]") return !1;
  var e = Object.getPrototypeOf(t);
  return e === null || e === Object.prototype;
}
function Bd(t) {
  return Pt.call(t) === "[object Date]";
}
function qd(t) {
  return Pt.call(t) === "[object File]";
}
function Kd(t) {
  return Pt.call(t) === "[object Blob]";
}
function wl(t) {
  return Pt.call(t) === "[object Function]";
}
function Wd(t) {
  return bl(t) && wl(t.pipe);
}
function Gd(t) {
  return typeof URLSearchParams < "u" && t instanceof URLSearchParams;
}
function zd(t) {
  return t.trim ? t.trim() : t.replace(/^\s+|\s+$/g, "");
}
function Jd() {
  return typeof navigator < "u" &&
    (navigator.product === "ReactNative" ||
      navigator.product === "NativeScript" ||
      navigator.product === "NS")
    ? !1
    : typeof window < "u" && typeof document < "u";
}
function si(t, e) {
  if (!(t === null || typeof t > "u"))
    if ((typeof t != "object" && (t = [t]), ni(t)))
      for (var r = 0, n = t.length; r < n; r++) e.call(null, t[r], r, t);
    else
      for (var s in t)
        Object.prototype.hasOwnProperty.call(t, s) && e.call(null, t[s], s, t);
}
function Rs() {
  var t = {};
  function e(s, i) {
    Wr(t[i]) && Wr(s)
      ? (t[i] = Rs(t[i], s))
      : Wr(s)
      ? (t[i] = Rs({}, s))
      : ni(s)
      ? (t[i] = s.slice())
      : (t[i] = s);
  }
  for (var r = 0, n = arguments.length; r < n; r++) si(arguments[r], e);
  return t;
}
function Yd(t, e, r) {
  return (
    si(e, function (s, i) {
      r && typeof s == "function" ? (t[i] = Vd(s, r)) : (t[i] = s);
    }),
    t
  );
}
function Xd(t) {
  return t.charCodeAt(0) === 65279 && (t = t.slice(1)), t;
}
var Oe = {
    isArray: ni,
    isArrayBuffer: $d,
    isBuffer: Dd,
    isFormData: Fd,
    isArrayBufferView: Ud,
    isString: jd,
    isNumber: Hd,
    isObject: bl,
    isPlainObject: Wr,
    isUndefined: Cs,
    isDate: Bd,
    isFile: qd,
    isBlob: Kd,
    isFunction: wl,
    isStream: Wd,
    isURLSearchParams: Gd,
    isStandardBrowserEnv: Jd,
    forEach: si,
    merge: Rs,
    extend: Yd,
    trim: zd,
    stripBOM: Xd,
  },
  Lt = Oe;
function uo(t) {
  return encodeURIComponent(t)
    .replace(/%3A/gi, ":")
    .replace(/%24/g, "$")
    .replace(/%2C/gi, ",")
    .replace(/%20/g, "+")
    .replace(/%5B/gi, "[")
    .replace(/%5D/gi, "]");
}
var _l = function (e, r, n) {
    if (!r) return e;
    var s;
    if (n) s = n(r);
    else if (Lt.isURLSearchParams(r)) s = r.toString();
    else {
      var i = [];
      Lt.forEach(r, function (l, c) {
        l === null ||
          typeof l > "u" ||
          (Lt.isArray(l) ? (c = c + "[]") : (l = [l]),
          Lt.forEach(l, function (d) {
            Lt.isDate(d)
              ? (d = d.toISOString())
              : Lt.isObject(d) && (d = JSON.stringify(d)),
              i.push(uo(c) + "=" + uo(d));
          }));
      }),
        (s = i.join("&"));
    }
    if (s) {
      var o = e.indexOf("#");
      o !== -1 && (e = e.slice(0, o)),
        (e += (e.indexOf("?") === -1 ? "?" : "&") + s);
    }
    return e;
  },
  Qd = Oe;
function Cn() {
  this.handlers = [];
}
Cn.prototype.use = function (e, r, n) {
  return (
    this.handlers.push({
      fulfilled: e,
      rejected: r,
      synchronous: n ? n.synchronous : !1,
      runWhen: n ? n.runWhen : null,
    }),
    this.handlers.length - 1
  );
};
Cn.prototype.eject = function (e) {
  this.handlers[e] && (this.handlers[e] = null);
};
Cn.prototype.forEach = function (e) {
  Qd.forEach(this.handlers, function (n) {
    n !== null && e(n);
  });
};
var Zd = Cn,
  eh = Oe,
  th = function (e, r) {
    eh.forEach(e, function (s, i) {
      i !== r &&
        i.toUpperCase() === r.toUpperCase() &&
        ((e[r] = s), delete e[i]);
    });
  },
  Sl = function (e, r, n, s, i) {
    return (
      (e.config = r),
      n && (e.code = n),
      (e.request = s),
      (e.response = i),
      (e.isAxiosError = !0),
      (e.toJSON = function () {
        return {
          message: this.message,
          name: this.name,
          description: this.description,
          number: this.number,
          fileName: this.fileName,
          lineNumber: this.lineNumber,
          columnNumber: this.columnNumber,
          stack: this.stack,
          config: this.config,
          code: this.code,
        };
      }),
      e
    );
  },
  Kn,
  fo;
function Il() {
  if (fo) return Kn;
  fo = 1;
  var t = Sl;
  return (
    (Kn = function (r, n, s, i, o) {
      var a = new Error(r);
      return t(a, n, s, i, o);
    }),
    Kn
  );
}
var Wn, ho;
function rh() {
  if (ho) return Wn;
  ho = 1;
  var t = Il();
  return (
    (Wn = function (r, n, s) {
      var i = s.config.validateStatus;
      !s.status || !i || i(s.status)
        ? r(s)
        : n(
            t(
              "Request failed with status code " + s.status,
              s.config,
              null,
              s.request,
              s
            )
          );
    }),
    Wn
  );
}
var Gn, po;
function nh() {
  if (po) return Gn;
  po = 1;
  var t = Oe;
  return (
    (Gn = t.isStandardBrowserEnv()
      ? (function () {
          return {
            write: function (n, s, i, o, a, l) {
              var c = [];
              c.push(n + "=" + encodeURIComponent(s)),
                t.isNumber(i) && c.push("expires=" + new Date(i).toGMTString()),
                t.isString(o) && c.push("path=" + o),
                t.isString(a) && c.push("domain=" + a),
                l === !0 && c.push("secure"),
                (document.cookie = c.join("; "));
            },
            read: function (n) {
              var s = document.cookie.match(
                new RegExp("(^|;\\s*)(" + n + ")=([^;]*)")
              );
              return s ? decodeURIComponent(s[3]) : null;
            },
            remove: function (n) {
              this.write(n, "", Date.now() - 864e5);
            },
          };
        })()
      : (function () {
          return {
            write: function () {},
            read: function () {
              return null;
            },
            remove: function () {},
          };
        })()),
    Gn
  );
}
var zn, go;
function sh() {
  return (
    go ||
      ((go = 1),
      (zn = function (e) {
        return /^([a-z][a-z\d\+\-\.]*:)?\/\//i.test(e);
      })),
    zn
  );
}
var Jn, mo;
function ih() {
  return (
    mo ||
      ((mo = 1),
      (Jn = function (e, r) {
        return r ? e.replace(/\/+$/, "") + "/" + r.replace(/^\/+/, "") : e;
      })),
    Jn
  );
}
var Yn, vo;
function oh() {
  if (vo) return Yn;
  vo = 1;
  var t = sh(),
    e = ih();
  return (
    (Yn = function (n, s) {
      return n && !t(s) ? e(n, s) : s;
    }),
    Yn
  );
}
var Xn, xo;
function ah() {
  if (xo) return Xn;
  xo = 1;
  var t = Oe,
    e = [
      "age",
      "authorization",
      "content-length",
      "content-type",
      "etag",
      "expires",
      "from",
      "host",
      "if-modified-since",
      "if-unmodified-since",
      "last-modified",
      "location",
      "max-forwards",
      "proxy-authorization",
      "referer",
      "retry-after",
      "user-agent",
    ];
  return (
    (Xn = function (n) {
      var s = {},
        i,
        o,
        a;
      return (
        n &&
          t.forEach(n.split("\n"), function (c) {
            if (
              ((a = c.indexOf(":")),
              (i = t.trim(c.substr(0, a)).toLowerCase()),
              (o = t.trim(c.substr(a + 1))),
              i)
            ) {
              if (s[i] && e.indexOf(i) >= 0) return;
              i === "set-cookie"
                ? (s[i] = (s[i] ? s[i] : []).concat([o]))
                : (s[i] = s[i] ? s[i] + ", " + o : o);
            }
          }),
        s
      );
    }),
    Xn
  );
}
var Qn, Eo;
function lh() {
  if (Eo) return Qn;
  Eo = 1;
  var t = Oe;
  return (
    (Qn = t.isStandardBrowserEnv()
      ? (function () {
          var r = /(msie|trident)/i.test(navigator.userAgent),
            n = document.createElement("a"),
            s;
          function i(o) {
            var a = o;
            return (
              r && (n.setAttribute("href", a), (a = n.href)),
              n.setAttribute("href", a),
              {
                href: n.href,
                protocol: n.protocol ? n.protocol.replace(/:$/, "") : "",
                host: n.host,
                search: n.search ? n.search.replace(/^\?/, "") : "",
                hash: n.hash ? n.hash.replace(/^#/, "") : "",
                hostname: n.hostname,
                port: n.port,
                pathname:
                  n.pathname.charAt(0) === "/" ? n.pathname : "/" + n.pathname,
              }
            );
          }
          return (
            (s = i(window.location.href)),
            function (a) {
              var l = t.isString(a) ? i(a) : a;
              return l.protocol === s.protocol && l.host === s.host;
            }
          );
        })()
      : (function () {
          return function () {
            return !0;
          };
        })()),
    Qn
  );
}
var Zn, yo;
function bo() {
  if (yo) return Zn;
  yo = 1;
  var t = Oe,
    e = rh(),
    r = nh(),
    n = _l,
    s = oh(),
    i = ah(),
    o = lh(),
    a = Il();
  return (
    (Zn = function (c) {
      return new Promise(function (d, p) {
        var g = c.data,
          v = c.headers,
          _ = c.responseType;
        t.isFormData(g) && delete v["Content-Type"];
        var T = new XMLHttpRequest();
        if (c.auth) {
          var N = c.auth.username || "",
            R = c.auth.password
              ? unescape(encodeURIComponent(c.auth.password))
              : "";
          v.Authorization = "Basic " + btoa(N + ":" + R);
        }
        var S = s(c.baseURL, c.url);
        T.open(c.method.toUpperCase(), n(S, c.params, c.paramsSerializer), !0),
          (T.timeout = c.timeout);
        function y() {
          if (T) {
            var U =
                "getAllResponseHeaders" in T
                  ? i(T.getAllResponseHeaders())
                  : null,
              j =
                !_ || _ === "text" || _ === "json"
                  ? T.responseText
                  : T.response,
              B = {
                data: j,
                status: T.status,
                statusText: T.statusText,
                headers: U,
                config: c,
                request: T,
              };
            e(d, p, B), (T = null);
          }
        }
        if (
          ("onloadend" in T
            ? (T.onloadend = y)
            : (T.onreadystatechange = function () {
                !T ||
                  T.readyState !== 4 ||
                  (T.status === 0 &&
                    !(T.responseURL && T.responseURL.indexOf("file:") === 0)) ||
                  setTimeout(y);
              }),
          (T.onabort = function () {
            T && (p(a("Request aborted", c, "ECONNABORTED", T)), (T = null));
          }),
          (T.onerror = function () {
            p(a("Network Error", c, null, T)), (T = null);
          }),
          (T.ontimeout = function () {
            var j = "timeout of " + c.timeout + "ms exceeded";
            c.timeoutErrorMessage && (j = c.timeoutErrorMessage),
              p(
                a(
                  j,
                  c,
                  c.transitional && c.transitional.clarifyTimeoutError
                    ? "ETIMEDOUT"
                    : "ECONNABORTED",
                  T
                )
              ),
              (T = null);
          }),
          t.isStandardBrowserEnv())
        ) {
          var D =
            (c.withCredentials || o(S)) && c.xsrfCookieName
              ? r.read(c.xsrfCookieName)
              : void 0;
          D && (v[c.xsrfHeaderName] = D);
        }
        "setRequestHeader" in T &&
          t.forEach(v, function (j, B) {
            typeof g > "u" && B.toLowerCase() === "content-type"
              ? delete v[B]
              : T.setRequestHeader(B, j);
          }),
          t.isUndefined(c.withCredentials) ||
            (T.withCredentials = !!c.withCredentials),
          _ && _ !== "json" && (T.responseType = c.responseType),
          typeof c.onDownloadProgress == "function" &&
            T.addEventListener("progress", c.onDownloadProgress),
          typeof c.onUploadProgress == "function" &&
            T.upload &&
            T.upload.addEventListener("progress", c.onUploadProgress),
          c.cancelToken &&
            c.cancelToken.promise.then(function (j) {
              T && (T.abort(), p(j), (T = null));
            }),
          g || (g = null),
          T.send(g);
      });
    }),
    Zn
  );
}
var de = Oe,
  wo = th,
  ch = Sl,
  uh = {
    "Content-Type": "application/x-www-form-urlencoded",
  };
function _o(t, e) {
  !de.isUndefined(t) &&
    de.isUndefined(t["Content-Type"]) &&
    (t["Content-Type"] = e);
}
function fh() {
  var t;
  return (
    (typeof XMLHttpRequest < "u" ||
      (typeof process < "u" &&
        Object.prototype.toString.call(process) === "[object process]")) &&
      (t = bo()),
    t
  );
}
function dh(t, e, r) {
  if (de.isString(t))
    try {
      return (e || JSON.parse)(t), de.trim(t);
    } catch (n) {
      if (n.name !== "SyntaxError") throw n;
    }
  return (0, JSON.stringify)(t);
}
var Rn = {
  transitional: {
    silentJSONParsing: !0,
    forcedJSONParsing: !0,
    clarifyTimeoutError: !1,
  },
  adapter: fh(),
  transformRequest: [
    function (e, r) {
      return (
        wo(r, "Accept"),
        wo(r, "Content-Type"),
        de.isFormData(e) ||
        de.isArrayBuffer(e) ||
        de.isBuffer(e) ||
        de.isStream(e) ||
        de.isFile(e) ||
        de.isBlob(e)
          ? e
          : de.isArrayBufferView(e)
          ? e.buffer
          : de.isURLSearchParams(e)
          ? (_o(r, "application/x-www-form-urlencoded;charset=utf-8"),
            e.toString())
          : de.isObject(e) || (r && r["Content-Type"] === "application/json")
          ? (_o(r, "application/json"), dh(e))
          : e
      );
    },
  ],
  transformResponse: [
    function (e) {
      var r = this.transitional,
        n = r && r.silentJSONParsing,
        s = r && r.forcedJSONParsing,
        i = !n && this.responseType === "json";
      if (i || (s && de.isString(e) && e.length))
        try {
          return JSON.parse(e);
        } catch (o) {
          if (i)
            throw o.name === "SyntaxError" ? ch(o, this, "E_JSON_PARSE") : o;
        }
      return e;
    },
  ],
  timeout: 0,
  xsrfCookieName: "XSRF-TOKEN",
  xsrfHeaderName: "X-XSRF-TOKEN",
  maxContentLength: -1,
  maxBodyLength: -1,
  validateStatus: function (e) {
    return e >= 200 && e < 300;
  },
};
Rn.headers = {
  common: {
    Accept: "application/json, text/plain, */*",
  },
};
de.forEach(["delete", "get", "head"], function (e) {
  Rn.headers[e] = {};
});
de.forEach(["post", "put", "patch"], function (e) {
  Rn.headers[e] = de.merge(uh);
});
var ii = Rn,
  hh = Oe,
  ph = ii,
  gh = function (e, r, n) {
    var s = this || ph;
    return (
      hh.forEach(n, function (o) {
        e = o.call(s, e, r);
      }),
      e
    );
  },
  es,
  So;
function Tl() {
  return (
    So ||
      ((So = 1),
      (es = function (e) {
        return !!(e && e.__CANCEL__);
      })),
    es
  );
}
var Io = Oe,
  ts = gh,
  mh = Tl(),
  vh = ii;
function rs(t) {
  t.cancelToken && t.cancelToken.throwIfRequested();
}
var xh = function (e) {
    rs(e),
      (e.headers = e.headers || {}),
      (e.data = ts.call(e, e.data, e.headers, e.transformRequest)),
      (e.headers = Io.merge(
        e.headers.common || {},
        e.headers[e.method] || {},
        e.headers
      )),
      Io.forEach(
        ["delete", "get", "head", "post", "put", "patch", "common"],
        function (s) {
          delete e.headers[s];
        }
      );
    var r = e.adapter || vh.adapter;
    return r(e).then(
      function (s) {
        return (
          rs(e),
          (s.data = ts.call(e, s.data, s.headers, e.transformResponse)),
          s
        );
      },
      function (s) {
        return (
          mh(s) ||
            (rs(e),
            s &&
              s.response &&
              (s.response.data = ts.call(
                e,
                s.response.data,
                s.response.headers,
                e.transformResponse
              ))),
          Promise.reject(s)
        );
      }
    );
  },
  me = Oe,
  Cl = function (e, r) {
    r = r || {};
    var n = {},
      s = ["url", "method", "data"],
      i = ["headers", "auth", "proxy", "params"],
      o = [
        "baseURL",
        "transformRequest",
        "transformResponse",
        "paramsSerializer",
        "timeout",
        "timeoutMessage",
        "withCredentials",
        "adapter",
        "responseType",
        "xsrfCookieName",
        "xsrfHeaderName",
        "onUploadProgress",
        "onDownloadProgress",
        "decompress",
        "maxContentLength",
        "maxBodyLength",
        "maxRedirects",
        "transport",
        "httpAgent",
        "httpsAgent",
        "cancelToken",
        "socketPath",
        "responseEncoding",
      ],
      a = ["validateStatus"];
    function l(p, g) {
      return me.isPlainObject(p) && me.isPlainObject(g)
        ? me.merge(p, g)
        : me.isPlainObject(g)
        ? me.merge({}, g)
        : me.isArray(g)
        ? g.slice()
        : g;
    }
    function c(p) {
      me.isUndefined(r[p])
        ? me.isUndefined(e[p]) || (n[p] = l(void 0, e[p]))
        : (n[p] = l(e[p], r[p]));
    }
    me.forEach(s, function (g) {
      me.isUndefined(r[g]) || (n[g] = l(void 0, r[g]));
    }),
      me.forEach(i, c),
      me.forEach(o, function (g) {
        me.isUndefined(r[g])
          ? me.isUndefined(e[g]) || (n[g] = l(void 0, e[g]))
          : (n[g] = l(void 0, r[g]));
      }),
      me.forEach(a, function (g) {
        g in r ? (n[g] = l(e[g], r[g])) : g in e && (n[g] = l(void 0, e[g]));
      });
    var u = s.concat(i).concat(o).concat(a),
      d = Object.keys(e)
        .concat(Object.keys(r))
        .filter(function (g) {
          return u.indexOf(g) === -1;
        });
    return me.forEach(d, c), n;
  };
const Eh = "axios",
  yh = "0.21.4",
  bh = "Promise based HTTP client for the browser and node.js",
  wh = "index.js",
  _h = {
    test: "grunt test",
    start: "node ./sandbox/server.js",
    build: "NODE_ENV=production grunt build",
    preversion: "npm test",
    version:
      "npm run build && grunt version && git add -A dist && git add CHANGELOG.md bower.json package.json",
    postversion: "git push && git push --tags",
    examples: "node ./examples/server.js",
    coveralls:
      "cat coverage/lcov.info | ./node_modules/coveralls/bin/coveralls.js",
    fix: "eslint --fix lib/**/*.js",
  },
  Sh = {
    type: "git",
    url: "https://github.com/axios/axios.git",
  },
  Ih = ["xhr", "http", "ajax", "promise", "node"],
  Th = "Matt Zabriskie",
  Ch = "MIT",
  Rh = {
    url: "https://github.com/axios/axios/issues",
  },
  Ah = "https://axios-http.com",
  Oh = {
    coveralls: "^3.0.0",
    "es6-promise": "^4.2.4",
    grunt: "^1.3.0",
    "grunt-banner": "^0.6.0",
    "grunt-cli": "^1.2.0",
    "grunt-contrib-clean": "^1.1.0",
    "grunt-contrib-watch": "^1.0.0",
    "grunt-eslint": "^23.0.0",
    "grunt-karma": "^4.0.0",
    "grunt-mocha-test": "^0.13.3",
    "grunt-ts": "^6.0.0-beta.19",
    "grunt-webpack": "^4.0.2",
    "istanbul-instrumenter-loader": "^1.0.0",
    "jasmine-core": "^2.4.1",
    karma: "^6.3.2",
    "karma-chrome-launcher": "^3.1.0",
    "karma-firefox-launcher": "^2.1.0",
    "karma-jasmine": "^1.1.1",
    "karma-jasmine-ajax": "^0.1.13",
    "karma-safari-launcher": "^1.0.0",
    "karma-sauce-launcher": "^4.3.6",
    "karma-sinon": "^1.0.5",
    "karma-sourcemap-loader": "^0.3.8",
    "karma-webpack": "^4.0.2",
    "load-grunt-tasks": "^3.5.2",
    minimist: "^1.2.0",
    mocha: "^8.2.1",
    sinon: "^4.5.0",
    "terser-webpack-plugin": "^4.2.3",
    typescript: "^4.0.5",
    "url-search-params": "^0.10.0",
    webpack: "^4.44.2",
    "webpack-dev-server": "^3.11.0",
  },
  Ph = {
    "./lib/adapters/http.js": "./lib/adapters/xhr.js",
  },
  Mh = "dist/axios.min.js",
  Lh = "dist/axios.min.js",
  Nh = "./index.d.ts",
  kh = {
    "follow-redirects": "^1.14.0",
  },
  Vh = [
    {
      path: "./dist/axios.min.js",
      threshold: "5kB",
    },
  ],
  Dh = {
    name: Eh,
    version: yh,
    description: bh,
    main: wh,
    scripts: _h,
    repository: Sh,
    keywords: Ih,
    author: Th,
    license: Ch,
    bugs: Rh,
    homepage: Ah,
    devDependencies: Oh,
    browser: Ph,
    jsdelivr: Mh,
    unpkg: Lh,
    typings: Nh,
    dependencies: kh,
    bundlesize: Vh,
  };
var Rl = Dh,
  oi = {};
["object", "boolean", "number", "function", "string", "symbol"].forEach(
  function (t, e) {
    oi[t] = function (n) {
      return typeof n === t || "a" + (e < 1 ? "n " : " ") + t;
    };
  }
);
var To = {},
  $h = Rl.version.split(".");
function Al(t, e) {
  for (var r = e ? e.split(".") : $h, n = t.split("."), s = 0; s < 3; s++) {
    if (r[s] > n[s]) return !0;
    if (r[s] < n[s]) return !1;
  }
  return !1;
}
oi.transitional = function (e, r, n) {
  var s = r && Al(r);
  function i(o, a) {
    return (
      "[Axios v" +
      Rl.version +
      "] Transitional option '" +
      o +
      "'" +
      a +
      (n ? ". " + n : "")
    );
  }
  return function (o, a, l) {
    if (e === !1) throw new Error(i(a, " has been removed in " + r));
    return s && !To[a] && (To[a] = !0), e ? e(o, a, l) : !0;
  };
};
function Fh(t, e, r) {
  if (typeof t != "object") throw new TypeError("options must be an object");
  for (var n = Object.keys(t), s = n.length; s-- > 0; ) {
    var i = n[s],
      o = e[i];
    if (o) {
      var a = t[i],
        l = a === void 0 || o(a, i, t);
      if (l !== !0) throw new TypeError("option " + i + " must be " + l);
      continue;
    }
    if (r !== !0) throw Error("Unknown option " + i);
  }
}
var Uh = {
    isOlderVersion: Al,
    assertOptions: Fh,
    validators: oi,
  },
  Ol = Oe,
  jh = _l,
  Co = Zd,
  Ro = xh,
  An = Cl,
  Pl = Uh,
  Nt = Pl.validators;
function Rr(t) {
  (this.defaults = t),
    (this.interceptors = {
      request: new Co(),
      response: new Co(),
    });
}
Rr.prototype.request = function (e) {
  typeof e == "string"
    ? ((e = arguments[1] || {}), (e.url = arguments[0]))
    : (e = e || {}),
    (e = An(this.defaults, e)),
    e.method
      ? (e.method = e.method.toLowerCase())
      : this.defaults.method
      ? (e.method = this.defaults.method.toLowerCase())
      : (e.method = "get");
  var r = e.transitional;
  r !== void 0 &&
    Pl.assertOptions(
      r,
      {
        silentJSONParsing: Nt.transitional(Nt.boolean, "1.0.0"),
        forcedJSONParsing: Nt.transitional(Nt.boolean, "1.0.0"),
        clarifyTimeoutError: Nt.transitional(Nt.boolean, "1.0.0"),
      },
      !1
    );
  var n = [],
    s = !0;
  this.interceptors.request.forEach(function (p) {
    (typeof p.runWhen == "function" && p.runWhen(e) === !1) ||
      ((s = s && p.synchronous), n.unshift(p.fulfilled, p.rejected));
  });
  var i = [];
  this.interceptors.response.forEach(function (p) {
    i.push(p.fulfilled, p.rejected);
  });
  var o;
  if (!s) {
    var a = [Ro, void 0];
    for (
      Array.prototype.unshift.apply(a, n),
        a = a.concat(i),
        o = Promise.resolve(e);
      a.length;

    )
      o = o.then(a.shift(), a.shift());
    return o;
  }
  for (var l = e; n.length; ) {
    var c = n.shift(),
      u = n.shift();
    try {
      l = c(l);
    } catch (d) {
      u(d);
      break;
    }
  }
  try {
    o = Ro(l);
  } catch (d) {
    return Promise.reject(d);
  }
  for (; i.length; ) o = o.then(i.shift(), i.shift());
  return o;
};
Rr.prototype.getUri = function (e) {
  return (
    (e = An(this.defaults, e)),
    jh(e.url, e.params, e.paramsSerializer).replace(/^\?/, "")
  );
};
Ol.forEach(["delete", "get", "head", "options"], function (e) {
  Rr.prototype[e] = function (r, n) {
    return this.request(
      An(n || {}, {
        method: e,
        url: r,
        data: (n || {}).data,
      })
    );
  };
});
Ol.forEach(["post", "put", "patch"], function (e) {
  Rr.prototype[e] = function (r, n, s) {
    return this.request(
      An(s || {}, {
        method: e,
        url: r,
        data: n,
      })
    );
  };
});
var Hh = Rr,
  ns,
  Ao;
function Ml() {
  if (Ao) return ns;
  Ao = 1;
  function t(e) {
    this.message = e;
  }
  return (
    (t.prototype.toString = function () {
      return "Cancel" + (this.message ? ": " + this.message : "");
    }),
    (t.prototype.__CANCEL__ = !0),
    (ns = t),
    ns
  );
}
var ss, Oo;
function Bh() {
  if (Oo) return ss;
  Oo = 1;
  var t = Ml();
  function e(r) {
    if (typeof r != "function")
      throw new TypeError("executor must be a function.");
    var n;
    this.promise = new Promise(function (o) {
      n = o;
    });
    var s = this;
    r(function (o) {
      s.reason || ((s.reason = new t(o)), n(s.reason));
    });
  }
  return (
    (e.prototype.throwIfRequested = function () {
      if (this.reason) throw this.reason;
    }),
    (e.source = function () {
      var n,
        s = new e(function (o) {
          n = o;
        });
      return {
        token: s,
        cancel: n,
      };
    }),
    (ss = e),
    ss
  );
}
var is, Po;
function qh() {
  return (
    Po ||
      ((Po = 1),
      (is = function (e) {
        return function (n) {
          return e.apply(null, n);
        };
      })),
    is
  );
}
var os, Mo;
function Kh() {
  return (
    Mo ||
      ((Mo = 1),
      (os = function (e) {
        return typeof e == "object" && e.isAxiosError === !0;
      })),
    os
  );
}
var Lo = Oe,
  Wh = yl,
  Gr = Hh,
  Gh = Cl,
  zh = ii;
function Ll(t) {
  var e = new Gr(t),
    r = Wh(Gr.prototype.request, e);
  return Lo.extend(r, Gr.prototype, e), Lo.extend(r, e), r;
}
var Ue = Ll(zh);
Ue.Axios = Gr;
Ue.create = function (e) {
  return Ll(Gh(Ue.defaults, e));
};
Ue.Cancel = Ml();
Ue.CancelToken = Bh();
Ue.isCancel = Tl();
Ue.all = function (e) {
  return Promise.all(e);
};
Ue.spread = qh();
Ue.isAxiosError = Kh();
ri.exports = Ue;
ri.exports.default = Ue;
var Jh = ri.exports,
  Yh = Jh;
const Xh = $l(Yh),
  ai = Xh.create();
ai.interceptors.request.use((t) => ({
  ...t,
  withCredentials: !0,
  timeout: 5e3,
}));
ai.interceptors.response.use(
  (t) => {
    const e = t.data;
    if (t.status !== 200) {
      if (e.errCode === 4011 || e.errCode === 4012) return t;
    } else return e.errCode === 0 ? t : Promise.reject(e);
    return t;
  },
  (t) => Promise.reject(t)
);
class Qh {
  constructor(e, r) {
    ye(this, "host", "/web");
    ye(this, "api", new Map());
    ye(this, "interceptors", []);
    e.forEach((n) => {
      this.api.set(n.name, {
        ...n,
        url: "".concat(r || this.host).concat(n.path),
      });
    });
  }
  getApiUrl(e) {
    return this.api.get(e).url;
  }
  addInterceptor(e) {
    return this.interceptors.push(e);
  }
  async get(e) {
    return this.request({
      ...e,
      method: "GET",
    });
  }
  async post(e) {
    return this.request({
      ...e,
      method: "POST",
    });
  }
  async request(e) {
    const r = {
      ...e,
    };
    r.method || (r.method = "POST");
    const n = r.data || r.params || {};
    r.method.toLocaleLowerCase() === "get" ? (r.params = n) : (r.data = n);
    try {
      let s = await ai.request(r);
      return (s = this.interceptors.reduce((o, a) => a(o), s)), s.data;
    } catch (s) {
      return {
        errCode: -1,
        errMsg: s == null ? void 0 : s.message,
        data: null,
      };
    }
  }
}
class Zh extends Qh {
  constructor() {
    super([
      {
        name: "GetUserAgent",
        path: "/api/util/ua",
      },
    ]);
  }
  async getUserAgent() {
    const e = await this.post({
      url: this.getApiUrl("GetUserAgent"),
      data: {},
    });
    return e.errCode !== 0 || !e.data ? "" : e.data;
  }
}
const ep = new Zh();
var tp = ((t) => (
  (t[(t.pcWechat = 1)] = "pcWechat"),
  (t[(t.macWechat = 2)] = "macWechat"),
  (t[(t.wxwork = 3)] = "wxwork"),
  t
))(tp || {});
const zr = new Map();
function No(t) {
  const e = /UnifiedPC.+Wechat\((\S+)\)/.exec(t);
  return e ? e[1] : "";
}
class rp {
  constructor() {
    ye(this, "ua", navigator.userAgent);
    ye(this, "uaLowerCase", this.ua.toLowerCase());
    ye(this, "isUpdatingUa", !1);
  }
  async updateUserAgentFromNode() {
    if (this.isUpdatingUa) return;
    this.isUpdatingUa = !0;
    const e = await ep.getUserAgent();
    e && ((this.ua = e), (this.uaLowerCase = e.toLowerCase()), zr.clear()),
      (this.isUpdatingUa = !1);
  }
  get currentEnv() {
    const e = this.uaLowerCase;
    if (e.includes("unifiedpcwindows")) return 1;
    if (e.includes("unifiedpcmac")) return 2;
    if (e.includes("unifiedpclinux") || e.includes("unifiedpcohos")) return 1;
    if (e.includes("wxwork")) return 3;
    if (e.includes("macwechat")) return 2;
    if (e.includes("windowswechat")) return 1;
  }
  get deviceId() {
    const e = this.uaLowerCase;
    return e.includes("unifiedpcwindows")
      ? 37
      : e.includes("unifiedpcmac")
      ? 38
      : e.includes("unifiedpclinux")
      ? 39
      : e.includes("unifiedpcohos")
      ? 43
      : e.includes("macwechat")
      ? 14
      : e.includes("windowswechat")
      ? 15
      : 29;
  }
  get isMobile() {
    const e = this.uaLowerCase;
    return !!(
      e.includes("mobile") ||
      e.includes("android") ||
      e.includes("iphone")
    );
  }
  get isMobileWx() {
    const e = this.uaLowerCase,
      r = this.isMobile,
      n = e.includes("micromessenger") && !e.includes("wxwork");
    return r && n;
  }
  get isDesktopWechat() {
    const e = this.currentEnv;
    return e === 2 || e === 1;
  }
  get isWxwork() {
    return this.currentEnv === 3;
  }
  get isWindowsWechat() {
    return this.currentEnv === 1;
  }
  get isMacWechat() {
    return this.currentEnv === 2;
  }
  get macVersion() {
    var o;
    if (this.uaLowerCase.includes("unifiedpcmac")) {
      const a = (o = No(this.ua)) != null ? o : "0xf2600000";
      return parseInt(a, 16);
    }
    const e = this.uaLowerCase.match(/macwechat\/(\S+)/),
      r = e != null && e.length && e.length >= 1 ? e[1] : "",
      n = r == null ? void 0 : r.match(/\((.+?)\)/),
      s = n != null && n.length && n.length >= 1 ? n[1] : "0";
    return parseInt(s, 16);
  }
  get windowsVersion() {
    var e;
    if (this.uaLowerCase.includes("unifiedpc")) {
      const r = (e = No(this.ua)) != null ? e : "0xf2500000";
      return parseInt(r, 16);
    }
    try {
      const r = this.uaLowerCase.match(/windowswechat\((.+?)\)/),
        n = r != null && r.length && r.length >= 1 ? r[1] : "0";
      return parseInt(n, 16);
    } catch (r) {
      return 0;
    }
  }
  get isMultiTabs() {
    return this.ua.includes("Flue");
  }
  get xwebVersion() {
    var e;
    return parseInt(
      ((e = this.ua.match(/XWEB\/(\S+)/)) == null ? void 0 : e[1]) || "",
      10
    );
  }
  get isDebugXweb() {
    return this.xwebVersion === 1e3;
  }
  get disabledFlowVolumeStorage() {
    return this.xwebVersion === 6961;
  }
  get isUnified() {
    return this.uaLowerCase.includes("unified");
  }
  isCurrentEnvSupported(e) {
    var r, n, s, i;
    if (this.isWxwork) return !0;
    if (this.isUnified) {
      const o = this.uaLowerCase;
      let a = e.pc;
      o.includes("unifiedpcwindows")
        ? (a = (r = e.uniWin) != null ? r : e.pc)
        : o.includes("unifiedpcmac")
        ? (a = (n = e.uniMac) != null ? n : e.mac)
        : o.includes("unifiedpclinux")
        ? (a = (s = e.uniLinux) != null ? s : e.pc)
        : o.includes("unifiedpcohos") && (a = (i = e.uniOh) != null ? i : e.pc);
      const l = parseInt(a, 16);
      if ((this.currentEnv === 2 ? this.macVersion : this.windowsVersion) >= l)
        return !0;
    } else if (this.isWindowsWechat) {
      const o = this.windowsVersion,
        a = parseInt(e.pc, 16);
      if (o >= a) return !0;
    } else if (this.isMacWechat) {
      const o = this.macVersion,
        a = parseInt(e.mac, 16);
      if (o >= a) return !0;
    }
    return !1;
  }
  get isWin10() {
    return this.windowsOsVersion === "Windows 10";
  }
  get chromeMajorVersion() {
    const { ua: e } = this,
      r = /Chrome\/([\d.]+)/,
      n = e.match(r);
    return n && n[1] ? n[1].split(".")[0] : null;
  }
  get windowsOsVersion() {
    const e = this.ua,
      r = {
        "5.0": "Windows 2000",
        5.1: "Windows XP",
        5.2: "Windows XP x64 Edition",
        "6.0": "Windows Vista",
        6.1: "Windows 7",
        6.2: "Windows 8",
        6.3: "Windows 8.1",
        "10.0": "Windows 10",
      },
      n = e.match(/Windows NT (\d+\.\d+)/);
    return (n && n[1] in r && r[n[1]]) || "";
  }
}
const np = new Proxy(new rp(), {
    get(t, e, r) {
      var s;
      const n = Object.getPrototypeOf(t);
      if (
        n &&
        typeof ((s = Object.getOwnPropertyDescriptor(n, e)) == null
          ? void 0
          : s.get) == "function"
      ) {
        if (zr.has(e)) return zr.get(e);
        const i = Reflect.get(t, e, r);
        return zr.set(e, Reflect.get(t, e, r)), i;
      }
      return Reflect.get(t, e, r);
    },
  }),
  sp = gn(np);
var ip = ((t) => (
  (t[(t.SUCCEED = 0)] = "SUCCEED"),
  (t[(t.NOT_CONNECTED = 1)] = "NOT_CONNECTED"),
  (t[(t.NOT_SUPPORTED = 2)] = "NOT_SUPPORTED"),
  (t[(t.TIMEOUT = 3)] = "TIMEOUT"),
  (t[(t.INVALID_DATA = 4)] = "INVALID_DATA"),
  (t[(t.DISCONNECTED = 5)] = "DISCONNECTED"),
  t
))(ip || {});
class op {
  constructor(e) {
    ye(this, "nextMsgId", 1);
    ye(this, "isConnected", !1);
    ye(this, "callbackMap", new Map());
    ye(this, "timeout", 300);
    ye(this, "supportedApiSet", new Set(["hello"]));
    ye(this, "disconnected", !1);
    ye(this, "workerVersion", 0);
    ye(this, "support", !1);
    var i, o, a, l;
    const { workerPre: r } = window;
    window.workerPre = null;
    const n = r && typeof r == "object";
    if (
      (location.href.indexOf("/pages/home") === -1 &&
        !(sp.isUnified && location.href.indexOf("/pages/feed") !== -1)) ||
      !(
        (o = (i = window.xweb) == null ? void 0 : i.worker) != null && o.connect
      )
    )
      return;
    const s = co.createPerfTransaction("logic", "workerInit");
    if (
      ((this.support = !0),
      window.addEventListener("xwportdisconnect", (c) => {
        (this.disconnected = !0), (this.isConnected = !1);
      }),
      n || (l = (a = window.xweb.worker).connect) == null || l.call(a, e),
      (this.isConnected = !0),
      window.xweb.worker.port)
    ) {
      window.xweb.worker.port.onmessage = (u) => {
        var d;
        if ((d = u == null ? void 0 : u.data) != null && d.id) {
          const p = this.callbackMap.get(u.data.id);
          p && (p(u.data), this.callbackMap.delete(u.data.id));
        } else
          co.catch(new hl("无法解析 worker 响应体", "worker"), {
            log: JSON.stringify(u),
          });
      };
      const c = ({ data: u, code: d }) => {
        var p;
        Array.isArray(u)
          ? u != null && u.length
            ? (u.forEach((g) => {
                this.supportedApiSet.add(g);
              }),
              s.report(
                "ok",
                {
                  idx1: "".concat(n ? 1 : 0),
                  idx2: "".concat(d),
                  idx3: "".concat(this.workerVersion),
                },
                {
                  sampleRate: 0.05,
                }
              ))
            : s.report(
                "fail",
                {
                  idx1: "apiList",
                  idx2: "".concat(d),
                },
                {
                  sampleRate: 0.05,
                }
              )
          : (p = u == null ? void 0 : u.api) != null && p.length
          ? (u.api.forEach((g) => {
              this.supportedApiSet.add(g);
            }),
            (this.workerVersion = u.v),
            s.report(
              "ok",
              {
                idx1: "".concat(n ? 1 : 0),
                idx2: "".concat(d),
                idx3: "".concat(this.workerVersion),
              },
              {
                sampleRate: 0.05,
              }
            ))
          : s.report(
              "fail",
              {
                idx2: "".concat(d),
              },
              {
                sampleRate: 0.05,
              }
            );
      };
      n
        ? c(r)
        : this.invoke("hello", void 0, !0)
            .then(c)
            .catch((u) => {
              s.report(
                "fail",
                {
                  idx1: "resp",
                  log: JSON.stringify(u),
                },
                {
                  sampleRate: 0.05,
                }
              );
            });
    } else
      s.report(
        "fail",
        {
          idx1: "port",
        },
        {
          sampleRate: 0.05,
        }
      );
  }
  async invoke(e, r, n = !1) {
    if (!this.isConnected || !this.supportedApiSet.has(e)) {
      let s = 1;
      return (
        this.disconnected ? (s = 5) : this.supportedApiSet.has(e) || (s = 2),
        Promise.resolve({
          data: null,
          code: s,
        })
      );
    }
    return new Promise((s, i) => {
      var a, l, c, u;
      const o = this.nextMsgId++;
      n ||
        window.setTimeout(() => {
          this.callbackMap.has(o) &&
            (this.callbackMap.delete(o),
            s({
              data: null,
              code: 3,
            }));
        }, this.timeout),
        this.callbackMap.set(o, (d) => {
          const p = d;
          p.code === 0
            ? s({
                data: p.data,
                code: 0,
              })
            : i(p.msg);
        }),
        (u =
          (c =
            (l = (a = window.xweb) == null ? void 0 : a.worker) == null
              ? void 0
              : l.port) == null
            ? void 0
            : c.postMessage) == null ||
          u.call(c, {
            id: o,
            apiName: e,
            body: r,
          });
    });
  }
}
let as;
function Qp() {
  return as || (as = new op()), as;
}
export {
  vn as A,
  sa as B,
  pp as C,
  tp as D,
  Sr as E,
  dp as F,
  Bu as G,
  lu as H,
  co as I,
  Xh as J,
  Jp as K,
  hl as L,
  Df as M,
  Ie as N,
  Yp as O,
  he as P,
  vl as Q,
  Nf as R,
  Qh as S,
  Tr as T,
  yr as U,
  Gp as V,
  zp as W,
  Xp as X,
  Wp as Y,
  Ks as Z,
  ip as _,
  ys as a,
  Ha as a0,
  oe as a1,
  Se as a2,
  Sp as a3,
  Ns as a4,
  kc as a5,
  Ls as a6,
  Zl as a7,
  Lp as a8,
  xp as a9,
  Ip as aA,
  Up as aB,
  Hp as aC,
  Cp as aD,
  Ht as aE,
  Dp as aF,
  qs as aG,
  tt as aH,
  Ee as aI,
  qc as aJ,
  Kc as aK,
  Ws as aL,
  Bp as aM,
  _a as aN,
  Mp as aO,
  Kp as aP,
  dt as aQ,
  vp as aa,
  kp as ab,
  Np as ac,
  $p as ad,
  jp as ae,
  Tp as af,
  _p as ag,
  Lu as ah,
  Ep as ai,
  Vp as aj,
  Fp as ak,
  Gc as al,
  wp as am,
  yp as an,
  hp as ao,
  Rp as ap,
  Op as aq,
  Il as ar,
  Oe as as,
  rh as at,
  _l as au,
  oh as av,
  Yh as aw,
  Sl as ax,
  cp as ay,
  Nu as az,
  qp as b,
  Hu as c,
  jc as d,
  Qp as e,
  lp as f,
  $l as g,
  up as h,
  Fr as i,
  fe as j,
  Ft as k,
  mp as l,
  bc as m,
  jr as n,
  Es as o,
  gn as p,
  Ap as q,
  bp as r,
  ec as s,
  K as t,
  sp as u,
  fp as v,
  Hr as w,
  Mc as x,
  gp as y,
  oa as z,
};
