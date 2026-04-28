;var WXAPI5 = (() => {

var Hc = Object.defineProperty;
var qc = (t, e, r) => e in t ? Hc(t, e, {
    enumerable: !0,
    configurable: !0,
    writable: !0,
    value: r
}) : t[e] = r;
var gi = (t, e, r) => (qc(t, typeof e != "symbol" ? e + "" : e, r),
r);
(function() {
    const e = document.createElement("link").relList;
    if (e && e.supports && e.supports("modulepreload"))
        return;
    for (const i of document.querySelectorAll('link[rel="modulepreload"]'))
        n(i);
    new MutationObserver(i => {
        for (const s of i)
            if (s.type === "childList")
                for (const o of s.addedNodes)
                    o.tagName === "LINK" && o.rel === "modulepreload" && n(o)
    }
    ).observe(document, {
        childList: !0,
        subtree: !0
    });
    function r(i) {
        const s = {};
        return i.integrity && (s.integrity = i.integrity),
        i.referrerpolicy && (s.referrerPolicy = i.referrerpolicy),
        i.crossorigin === "use-credentials" ? s.credentials = "include" : i.crossorigin === "anonymous" ? s.credentials = "omit" : s.credentials = "same-origin",
        s
    }
    function n(i) {
        if (i.ep)
            return;
        i.ep = !0;
        const s = r(i);
        fetch(i.href, s)
    }
}
)();
/**
* @vue/shared v3.5.17
* (c) 2018-present Yuxi (Evan) You and Vue contributors
* @license MIT
**/
/*! #__NO_SIDE_EFFECTS__ */
function zs(t) {
    const e = Object.create(null);
    for (const r of t.split(","))
        e[r] = 1;
    return r => r in e
}
const ye = {}
  , ur = []
  , st = () => {}
  , Vc = () => !1
  , zn = t => t.charCodeAt(0) === 111 && t.charCodeAt(1) === 110 && (t.charCodeAt(2) > 122 || t.charCodeAt(2) < 97)
  , Gs = t => t.startsWith("onUpdate:")
  , Te = Object.assign
  , Ys = (t, e) => {
    const r = t.indexOf(e);
    r > -1 && t.splice(r, 1)
}
  , Kc = Object.prototype.hasOwnProperty
  , pe = (t, e) => Kc.call(t, e)
  , te = Array.isArray
  , cr = t => Gn(t) === "[object Map]"
  , Tl = t => Gn(t) === "[object Set]"
  , ie = t => typeof t == "function"
  , Se = t => typeof t == "string"
  , It = t => typeof t == "symbol"
  , we = t => t !== null && typeof t == "object"
  , Al = t => (we(t) || ie(t)) && ie(t.then) && ie(t.catch)
  , Rl = Object.prototype.toString
  , Gn = t => Rl.call(t)
  , Wc = t => Gn(t).slice(8, -1)
  , Ml = t => Gn(t) === "[object Object]"
  , Js = t => Se(t) && t !== "NaN" && t[0] !== "-" && "" + parseInt(t, 10) === t
  , Lr = zs(",key,ref,ref_for,ref_key,onVnodeBeforeMount,onVnodeMounted,onVnodeBeforeUpdate,onVnodeUpdated,onVnodeBeforeUnmount,onVnodeUnmounted")
  , Yn = t => {
    const e = Object.create(null);
    return r => e[r] || (e[r] = t(r))
}
  , zc = /-(\w)/g
  , et = Yn(t => t.replace(zc, (e, r) => r ? r.toUpperCase() : ""))
  , Gc = /\B([A-Z])/g
  , rr = Yn(t => t.replace(Gc, "-$1").toLowerCase())
  , Jn = Yn(t => t.charAt(0).toUpperCase() + t.slice(1))
  , mi = Yn(t => t ? `on${Jn(t)}` : "")
  , kt = (t, e) => !Object.is(t, e)
  , vi = (t, ...e) => {
    for (let r = 0; r < t.length; r++)
        t[r](...e)
}
  , Es = (t, e, r, n=!1) => {
    Object.defineProperty(t, e, {
        configurable: !0,
        enumerable: !1,
        writable: n,
        value: r
    })
}
  , Yc = t => {
    const e = parseFloat(t);
    return isNaN(e) ? t : e
}
  , Jc = t => {
    const e = Se(t) ? Number(t) : NaN;
    return isNaN(e) ? t : e
}
;
let No;
const sn = () => No || (No = typeof globalThis < "u" ? globalThis : typeof self < "u" ? self : typeof window < "u" ? window : typeof global < "u" ? global : {});
function He(t) {
    if (te(t)) {
        const e = {};
        for (let r = 0; r < t.length; r++) {
            const n = t[r]
              , i = Se(n) ? ef(n) : He(n);
            if (i)
                for (const s in i)
                    e[s] = i[s]
        }
        return e
    } else if (Se(t) || we(t))
        return t
}
const Qc = /;(?![^(]*\))/g
  , Xc = /:([^]+)/
  , Zc = /\/\*[^]*?\*\//g;
function ef(t) {
    const e = {};
    return t.replace(Zc, "").split(Qc).forEach(r => {
        if (r) {
            const n = r.split(Xc);
            n.length > 1 && (e[n[0].trim()] = n[1].trim())
        }
    }
    ),
    e
}
function Bt(t) {
    let e = "";
    if (Se(t))
        e = t;
    else if (te(t))
        for (let r = 0; r < t.length; r++) {
            const n = Bt(t[r]);
            n && (e += n + " ")
        }
    else if (we(t))
        for (const r in t)
            t[r] && (e += r + " ");
    return e.trim()
}
const tf = "itemscope,allowfullscreen,formnovalidate,ismap,nomodule,novalidate,readonly"
  , rf = zs(tf);
function Pl(t) {
    return !!t || t === ""
}
const Ol = t => !!(t && t.__v_isRef === !0)
  , Wr = t => Se(t) ? t : t == null ? "" : te(t) || we(t) && (t.toString === Rl || !ie(t.toString)) ? Ol(t) ? Wr(t.value) : JSON.stringify(t, Ll, 2) : String(t)
  , Ll = (t, e) => Ol(e) ? Ll(t, e.value) : cr(e) ? {
    [`Map(${e.size})`]: [...e.entries()].reduce( (r, [n,i], s) => (r[yi(n, s) + " =>"] = i,
    r), {})
} : Tl(e) ? {
    [`Set(${e.size})`]: [...e.values()].map(r => yi(r))
} : It(e) ? yi(e) : we(e) && !te(e) && !Ml(e) ? String(e) : e
  , yi = (t, e="") => {
    var r;
    return It(t) ? `Symbol(${(r = t.description) != null ? r : e})` : t
}
;
/**
* @vue/reactivity v3.5.17
* (c) 2018-present Yuxi (Evan) You and Vue contributors
* @license MIT
**/
let Ne;
class Nl {
    constructor(e=!1) {
        this.detached = e,
        this._active = !0,
        this._on = 0,
        this.effects = [],
        this.cleanups = [],
        this._isPaused = !1,
        this.parent = Ne,
        !e && Ne && (this.index = (Ne.scopes || (Ne.scopes = [])).push(this) - 1)
    }
    get active() {
        return this._active
    }
    pause() {
        if (this._active) {
            this._isPaused = !0;
            let e, r;
            if (this.scopes)
                for (e = 0,
                r = this.scopes.length; e < r; e++)
                    this.scopes[e].pause();
            for (e = 0,
            r = this.effects.length; e < r; e++)
                this.effects[e].pause()
        }
    }
    resume() {
        if (this._active && this._isPaused) {
            this._isPaused = !1;
            let e, r;
            if (this.scopes)
                for (e = 0,
                r = this.scopes.length; e < r; e++)
                    this.scopes[e].resume();
            for (e = 0,
            r = this.effects.length; e < r; e++)
                this.effects[e].resume()
        }
    }
    run(e) {
        if (this._active) {
            const r = Ne;
            try {
                return Ne = this,
                e()
            } finally {
                Ne = r
            }
        }
    }
    on() {
        ++this._on === 1 && (this.prevScope = Ne,
        Ne = this)
    }
    off() {
        this._on > 0 && --this._on === 0 && (Ne = this.prevScope,
        this.prevScope = void 0)
    }
    stop(e) {
        if (this._active) {
            this._active = !1;
            let r, n;
            for (r = 0,
            n = this.effects.length; r < n; r++)
                this.effects[r].stop();
            for (this.effects.length = 0,
            r = 0,
            n = this.cleanups.length; r < n; r++)
                this.cleanups[r]();
            if (this.cleanups.length = 0,
            this.scopes) {
                for (r = 0,
                n = this.scopes.length; r < n; r++)
                    this.scopes[r].stop(!0);
                this.scopes.length = 0
            }
            if (!this.detached && this.parent && !e) {
                const i = this.parent.scopes.pop();
                i && i !== this && (this.parent.scopes[this.index] = i,
                i.index = this.index)
            }
            this.parent = void 0
        }
    }
}
function kl(t) {
    return new Nl(t)
}
function Bl() {
    return Ne
}
function nf(t, e=!1) {
    Ne && Ne.cleanups.push(t)
}
let be;
const bi = new WeakSet;
class Dl {
    constructor(e) {
        this.fn = e,
        this.deps = void 0,
        this.depsTail = void 0,
        this.flags = 5,
        this.next = void 0,
        this.cleanup = void 0,
        this.scheduler = void 0,
        Ne && Ne.active && Ne.effects.push(this)
    }
    pause() {
        this.flags |= 64
    }
    resume() {
        this.flags & 64 && (this.flags &= -65,
        bi.has(this) && (bi.delete(this),
        this.trigger()))
    }
    notify() {
        this.flags & 2 && !(this.flags & 32) || this.flags & 8 || Ul(this)
    }
    run() {
        if (!(this.flags & 1))
            return this.fn();
        this.flags |= 2,
        ko(this),
        jl(this);
        const e = be
          , r = ot;
        be = this,
        ot = !0;
        try {
            return this.fn()
        } finally {
            $l(this),
            be = e,
            ot = r,
            this.flags &= -3
        }
    }
    stop() {
        if (this.flags & 1) {
            for (let e = this.deps; e; e = e.nextDep)
                Zs(e);
            this.deps = this.depsTail = void 0,
            ko(this),
            this.onStop && this.onStop(),
            this.flags &= -2
        }
    }
    trigger() {
        this.flags & 64 ? bi.add(this) : this.scheduler ? this.scheduler() : this.runIfDirty()
    }
    runIfDirty() {
        xs(this) && this.run()
    }
    get dirty() {
        return xs(this)
    }
}
let Fl = 0, Nr, kr;
function Ul(t, e=!1) {
    if (t.flags |= 8,
    e) {
        t.next = kr,
        kr = t;
        return
    }
    t.next = Nr,
    Nr = t
}
function Qs() {
    Fl++
}
function Xs() {
    if (--Fl > 0)
        return;
    if (kr) {
        let e = kr;
        for (kr = void 0; e; ) {
            const r = e.next;
            e.next = void 0,
            e.flags &= -9,
            e = r
        }
    }
    let t;
    for (; Nr; ) {
        let e = Nr;
        for (Nr = void 0; e; ) {
            const r = e.next;
            if (e.next = void 0,
            e.flags &= -9,
            e.flags & 1)
                try {
                    e.trigger()
                } catch (n) {
                    t || (t = n)
                }
            e = r
        }
    }
    if (t)
        throw t
}
function jl(t) {
    for (let e = t.deps; e; e = e.nextDep)
        e.version = -1,
        e.prevActiveLink = e.dep.activeLink,
        e.dep.activeLink = e
}
function $l(t) {
    let e, r = t.depsTail, n = r;
    for (; n; ) {
        const i = n.prevDep;
        n.version === -1 ? (n === r && (r = i),
        Zs(n),
        sf(n)) : e = n,
        n.dep.activeLink = n.prevActiveLink,
        n.prevActiveLink = void 0,
        n = i
    }
    t.deps = e,
    t.depsTail = r
}
function xs(t) {
    for (let e = t.deps; e; e = e.nextDep)
        if (e.dep.version !== e.version || e.dep.computed && (Hl(e.dep.computed) || e.dep.version !== e.version))
            return !0;
    return !!t._dirty
}
function Hl(t) {
    if (t.flags & 4 && !(t.flags & 16) || (t.flags &= -17,
    t.globalVersion === zr) || (t.globalVersion = zr,
    !t.isSSR && t.flags & 128 && (!t.deps && !t._dirty || !xs(t))))
        return;
    t.flags |= 2;
    const e = t.dep
      , r = be
      , n = ot;
    be = t,
    ot = !0;
    try {
        jl(t);
        const i = t.fn(t._value);
        (e.version === 0 || kt(i, t._value)) && (t.flags |= 128,
        t._value = i,
        e.version++)
    } catch (i) {
        throw e.version++,
        i
    } finally {
        be = r,
        ot = n,
        $l(t),
        t.flags &= -3
    }
}
function Zs(t, e=!1) {
    const {dep: r, prevSub: n, nextSub: i} = t;
    if (n && (n.nextSub = i,
    t.prevSub = void 0),
    i && (i.prevSub = n,
    t.nextSub = void 0),
    r.subs === t && (r.subs = n,
    !n && r.computed)) {
        r.computed.flags &= -5;
        for (let s = r.computed.deps; s; s = s.nextDep)
            Zs(s, !0)
    }
    !e && !--r.sc && r.map && r.map.delete(r.key)
}
function sf(t) {
    const {prevDep: e, nextDep: r} = t;
    e && (e.nextDep = r,
    t.prevDep = void 0),
    r && (r.prevDep = e,
    t.nextDep = void 0)
}
let ot = !0;
const ql = [];
function St() {
    ql.push(ot),
    ot = !1
}
function Ct() {
    const t = ql.pop();
    ot = t === void 0 ? !0 : t
}
function ko(t) {
    const {cleanup: e} = t;
    if (t.cleanup = void 0,
    e) {
        const r = be;
        be = void 0;
        try {
            e()
        } finally {
            be = r
        }
    }
}
let zr = 0;
class of {
    constructor(e, r) {
        this.sub = e,
        this.dep = r,
        this.version = r.version,
        this.nextDep = this.prevDep = this.nextSub = this.prevSub = this.prevActiveLink = void 0
    }
}
class eo {
    constructor(e) {
        this.computed = e,
        this.version = 0,
        this.activeLink = void 0,
        this.subs = void 0,
        this.map = void 0,
        this.key = void 0,
        this.sc = 0,
        this.__v_skip = !0
    }
    track(e) {
        if (!be || !ot || be === this.computed)
            return;
        let r = this.activeLink;
        if (r === void 0 || r.sub !== be)
            r = this.activeLink = new of(be,this),
            be.deps ? (r.prevDep = be.depsTail,
            be.depsTail.nextDep = r,
            be.depsTail = r) : be.deps = be.depsTail = r,
            Vl(r);
        else if (r.version === -1 && (r.version = this.version,
        r.nextDep)) {
            const n = r.nextDep;
            n.prevDep = r.prevDep,
            r.prevDep && (r.prevDep.nextDep = n),
            r.prevDep = be.depsTail,
            r.nextDep = void 0,
            be.depsTail.nextDep = r,
            be.depsTail = r,
            be.deps === r && (be.deps = n)
        }
        return r
    }
    trigger(e) {
        this.version++,
        zr++,
        this.notify(e)
    }
    notify(e) {
        Qs();
        try {
            for (let r = this.subs; r; r = r.prevSub)
                r.sub.notify() && r.sub.dep.notify()
        } finally {
            Xs()
        }
    }
}
function Vl(t) {
    if (t.dep.sc++,
    t.sub.flags & 4) {
        const e = t.dep.computed;
        if (e && !t.dep.subs) {
            e.flags |= 20;
            for (let n = e.deps; n; n = n.nextDep)
                Vl(n)
        }
        const r = t.dep.subs;
        r !== t && (t.prevSub = r,
        r && (r.nextSub = t)),
        t.dep.subs = t
    }
}
const Ln = new WeakMap
  , Yt = Symbol("")
  , Ss = Symbol("")
  , Gr = Symbol("");
function ke(t, e, r) {
    if (ot && be) {
        let n = Ln.get(t);
        n || Ln.set(t, n = new Map);
        let i = n.get(r);
        i || (n.set(r, i = new eo),
        i.map = n,
        i.key = r),
        i.track()
    }
}
function _t(t, e, r, n, i, s) {
    const o = Ln.get(t);
    if (!o) {
        zr++;
        return
    }
    const a = l => {
        l && l.trigger()
    }
    ;
    if (Qs(),
    e === "clear")
        o.forEach(a);
    else {
        const l = te(t)
          , u = l && Js(r);
        if (l && r === "length") {
            const c = Number(n);
            o.forEach( (g, d) => {
                (d === "length" || d === Gr || !It(d) && d >= c) && a(g)
            }
            )
        } else
            switch ((r !== void 0 || o.has(void 0)) && a(o.get(r)),
            u && a(o.get(Gr)),
            e) {
            case "add":
                l ? u && a(o.get("length")) : (a(o.get(Yt)),
                cr(t) && a(o.get(Ss)));
                break;
            case "delete":
                l || (a(o.get(Yt)),
                cr(t) && a(o.get(Ss)));
                break;
            case "set":
                cr(t) && a(o.get(Yt));
                break
            }
    }
    Xs()
}
function af(t, e) {
    const r = Ln.get(t);
    return r && r.get(e)
}
function sr(t) {
    const e = le(t);
    return e === t ? e : (ke(e, "iterate", Gr),
    Xe(t) ? e : e.map(Le))
}
function Qn(t) {
    return ke(t = le(t), "iterate", Gr),
    t
}
const lf = {
    __proto__: null,
    [Symbol.iterator]() {
        return wi(this, Symbol.iterator, Le)
    },
    concat(...t) {
        return sr(this).concat(...t.map(e => te(e) ? sr(e) : e))
    },
    entries() {
        return wi(this, "entries", t => (t[1] = Le(t[1]),
        t))
    },
    every(t, e) {
        return gt(this, "every", t, e, void 0, arguments)
    },
    filter(t, e) {
        return gt(this, "filter", t, e, r => r.map(Le), arguments)
    },
    find(t, e) {
        return gt(this, "find", t, e, Le, arguments)
    },
    findIndex(t, e) {
        return gt(this, "findIndex", t, e, void 0, arguments)
    },
    findLast(t, e) {
        return gt(this, "findLast", t, e, Le, arguments)
    },
    findLastIndex(t, e) {
        return gt(this, "findLastIndex", t, e, void 0, arguments)
    },
    forEach(t, e) {
        return gt(this, "forEach", t, e, void 0, arguments)
    },
    includes(...t) {
        return _i(this, "includes", t)
    },
    indexOf(...t) {
        return _i(this, "indexOf", t)
    },
    join(t) {
        return sr(this).join(t)
    },
    lastIndexOf(...t) {
        return _i(this, "lastIndexOf", t)
    },
    map(t, e) {
        return gt(this, "map", t, e, void 0, arguments)
    },
    pop() {
        return xr(this, "pop")
    },
    push(...t) {
        return xr(this, "push", t)
    },
    reduce(t, ...e) {
        return Bo(this, "reduce", t, e)
    },
    reduceRight(t, ...e) {
        return Bo(this, "reduceRight", t, e)
    },
    shift() {
        return xr(this, "shift")
    },
    some(t, e) {
        return gt(this, "some", t, e, void 0, arguments)
    },
    splice(...t) {
        return xr(this, "splice", t)
    },
    toReversed() {
        return sr(this).toReversed()
    },
    toSorted(t) {
        return sr(this).toSorted(t)
    },
    toSpliced(...t) {
        return sr(this).toSpliced(...t)
    },
    unshift(...t) {
        return xr(this, "unshift", t)
    },
    values() {
        return wi(this, "values", Le)
    }
};
function wi(t, e, r) {
    const n = Qn(t)
      , i = n[e]();
    return n !== t && !Xe(t) && (i._next = i.next,
    i.next = () => {
        const s = i._next();
        return s.value && (s.value = r(s.value)),
        s
    }
    ),
    i
}
const uf = Array.prototype;
function gt(t, e, r, n, i, s) {
    const o = Qn(t)
      , a = o !== t && !Xe(t)
      , l = o[e];
    if (l !== uf[e]) {
        const g = l.apply(t, s);
        return a ? Le(g) : g
    }
    let u = r;
    o !== t && (a ? u = function(g, d) {
        return r.call(this, Le(g), d, t)
    }
    : r.length > 2 && (u = function(g, d) {
        return r.call(this, g, d, t)
    }
    ));
    const c = l.call(o, u, n);
    return a && i ? i(c) : c
}
function Bo(t, e, r, n) {
    const i = Qn(t);
    let s = r;
    return i !== t && (Xe(t) ? r.length > 3 && (s = function(o, a, l) {
        return r.call(this, o, a, l, t)
    }
    ) : s = function(o, a, l) {
        return r.call(this, o, Le(a), l, t)
    }
    ),
    i[e](s, ...n)
}
function _i(t, e, r) {
    const n = le(t);
    ke(n, "iterate", Gr);
    const i = n[e](...r);
    return (i === -1 || i === !1) && no(r[0]) ? (r[0] = le(r[0]),
    n[e](...r)) : i
}
function xr(t, e, r=[]) {
    St(),
    Qs();
    const n = le(t)[e].apply(t, r);
    return Xs(),
    Ct(),
    n
}
const cf = zs("__proto__,__v_isRef,__isVue")
  , Kl = new Set(Object.getOwnPropertyNames(Symbol).filter(t => t !== "arguments" && t !== "caller").map(t => Symbol[t]).filter(It));
function ff(t) {
    It(t) || (t = String(t));
    const e = le(this);
    return ke(e, "has", t),
    e.hasOwnProperty(t)
}
class Wl {
    constructor(e=!1, r=!1) {
        this._isReadonly = e,
        this._isShallow = r
    }
    get(e, r, n) {
        if (r === "__v_skip")
            return e.__v_skip;
        const i = this._isReadonly
          , s = this._isShallow;
        if (r === "__v_isReactive")
            return !i;
        if (r === "__v_isReadonly")
            return i;
        if (r === "__v_isShallow")
            return s;
        if (r === "__v_raw")
            return n === (i ? s ? _f : Jl : s ? Yl : Gl).get(e) || Object.getPrototypeOf(e) === Object.getPrototypeOf(n) ? e : void 0;
        const o = te(e);
        if (!i) {
            let l;
            if (o && (l = lf[r]))
                return l;
            if (r === "hasOwnProperty")
                return ff
        }
        const a = Reflect.get(e, r, xe(e) ? e : n);
        return (It(r) ? Kl.has(r) : cf(r)) || (i || ke(e, "get", r),
        s) ? a : xe(a) ? o && Js(r) ? a : a.value : we(a) ? i ? Lt(a) : Xn(a) : a
    }
}
class zl extends Wl {
    constructor(e=!1) {
        super(!1, e)
    }
    set(e, r, n, i) {
        let s = e[r];
        if (!this._isShallow) {
            const l = Dt(s);
            if (!Xe(n) && !Dt(n) && (s = le(s),
            n = le(n)),
            !te(e) && xe(s) && !xe(n))
                return l ? !1 : (s.value = n,
                !0)
        }
        const o = te(e) && Js(r) ? Number(r) < e.length : pe(e, r)
          , a = Reflect.set(e, r, n, xe(e) ? e : i);
        return e === le(i) && (o ? kt(n, s) && _t(e, "set", r, n) : _t(e, "add", r, n)),
        a
    }
    deleteProperty(e, r) {
        const n = pe(e, r);
        e[r];
        const i = Reflect.deleteProperty(e, r);
        return i && n && _t(e, "delete", r, void 0),
        i
    }
    has(e, r) {
        const n = Reflect.has(e, r);
        return (!It(r) || !Kl.has(r)) && ke(e, "has", r),
        n
    }
    ownKeys(e) {
        return ke(e, "iterate", te(e) ? "length" : Yt),
        Reflect.ownKeys(e)
    }
}
class df extends Wl {
    constructor(e=!1) {
        super(!0, e)
    }
    set(e, r) {
        return !0
    }
    deleteProperty(e, r) {
        return !0
    }
}
const hf = new zl
  , pf = new df
  , gf = new zl(!0);
const Cs = t => t
  , pn = t => Reflect.getPrototypeOf(t);
function mf(t, e, r) {
    return function(...n) {
        const i = this.__v_raw
          , s = le(i)
          , o = cr(s)
          , a = t === "entries" || t === Symbol.iterator && o
          , l = t === "keys" && o
          , u = i[t](...n)
          , c = r ? Cs : e ? Nn : Le;
        return !e && ke(s, "iterate", l ? Ss : Yt),
        {
            next() {
                const {value: g, done: d} = u.next();
                return d ? {
                    value: g,
                    done: d
                } : {
                    value: a ? [c(g[0]), c(g[1])] : c(g),
                    done: d
                }
            },
            [Symbol.iterator]() {
                return this
            }
        }
    }
}
function gn(t) {
    return function(...e) {
        return t === "delete" ? !1 : t === "clear" ? void 0 : this
    }
}
function vf(t, e) {
    const r = {
        get(i) {
            const s = this.__v_raw
              , o = le(s)
              , a = le(i);
            t || (kt(i, a) && ke(o, "get", i),
            ke(o, "get", a));
            const {has: l} = pn(o)
              , u = e ? Cs : t ? Nn : Le;
            if (l.call(o, i))
                return u(s.get(i));
            if (l.call(o, a))
                return u(s.get(a));
            s !== o && s.get(i)
        },
        get size() {
            const i = this.__v_raw;
            return !t && ke(le(i), "iterate", Yt),
            Reflect.get(i, "size", i)
        },
        has(i) {
            const s = this.__v_raw
              , o = le(s)
              , a = le(i);
            return t || (kt(i, a) && ke(o, "has", i),
            ke(o, "has", a)),
            i === a ? s.has(i) : s.has(i) || s.has(a)
        },
        forEach(i, s) {
            const o = this
              , a = o.__v_raw
              , l = le(a)
              , u = e ? Cs : t ? Nn : Le;
            return !t && ke(l, "iterate", Yt),
            a.forEach( (c, g) => i.call(s, u(c), u(g), o))
        }
    };
    return Te(r, t ? {
        add: gn("add"),
        set: gn("set"),
        delete: gn("delete"),
        clear: gn("clear")
    } : {
        add(i) {
            !e && !Xe(i) && !Dt(i) && (i = le(i));
            const s = le(this);
            return pn(s).has.call(s, i) || (s.add(i),
            _t(s, "add", i, i)),
            this
        },
        set(i, s) {
            !e && !Xe(s) && !Dt(s) && (s = le(s));
            const o = le(this)
              , {has: a, get: l} = pn(o);
            let u = a.call(o, i);
            u || (i = le(i),
            u = a.call(o, i));
            const c = l.call(o, i);
            return o.set(i, s),
            u ? kt(s, c) && _t(o, "set", i, s) : _t(o, "add", i, s),
            this
        },
        delete(i) {
            const s = le(this)
              , {has: o, get: a} = pn(s);
            let l = o.call(s, i);
            l || (i = le(i),
            l = o.call(s, i)),
            a && a.call(s, i);
            const u = s.delete(i);
            return l && _t(s, "delete", i, void 0),
            u
        },
        clear() {
            const i = le(this)
              , s = i.size !== 0
              , o = i.clear();
            return s && _t(i, "clear", void 0, void 0),
            o
        }
    }),
    ["keys", "values", "entries", Symbol.iterator].forEach(i => {
        r[i] = mf(i, t, e)
    }
    ),
    r
}
function to(t, e) {
    const r = vf(t, e);
    return (n, i, s) => i === "__v_isReactive" ? !t : i === "__v_isReadonly" ? t : i === "__v_raw" ? n : Reflect.get(pe(r, i) && i in n ? r : n, i, s)
}
const yf = {
    get: to(!1, !1)
}
  , bf = {
    get: to(!1, !0)
}
  , wf = {
    get: to(!0, !1)
};
const Gl = new WeakMap
  , Yl = new WeakMap
  , Jl = new WeakMap
  , _f = new WeakMap;
function Ef(t) {
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
        return 0
    }
}
function xf(t) {
    return t.__v_skip || !Object.isExtensible(t) ? 0 : Ef(Wc(t))
}
function Xn(t) {
    return Dt(t) ? t : ro(t, !1, hf, yf, Gl)
}
function Sf(t) {
    return ro(t, !1, gf, bf, Yl)
}
function Lt(t) {
    return ro(t, !0, pf, wf, Jl)
}
function ro(t, e, r, n, i) {
    if (!we(t) || t.__v_raw && !(e && t.__v_isReactive))
        return t;
    const s = xf(t);
    if (s === 0)
        return t;
    const o = i.get(t);
    if (o)
        return o;
    const a = new Proxy(t,s === 2 ? n : r);
    return i.set(t, a),
    a
}
function xt(t) {
    return Dt(t) ? xt(t.__v_raw) : !!(t && t.__v_isReactive)
}
function Dt(t) {
    return !!(t && t.__v_isReadonly)
}
function Xe(t) {
    return !!(t && t.__v_isShallow)
}
function no(t) {
    return t ? !!t.__v_raw : !1
}
function le(t) {
    const e = t && t.__v_raw;
    return e ? le(e) : t
}
function io(t) {
    return !pe(t, "__v_skip") && Object.isExtensible(t) && Es(t, "__v_skip", !0),
    t
}
const Le = t => we(t) ? Xn(t) : t
  , Nn = t => we(t) ? Lt(t) : t;
function xe(t) {
    return t ? t.__v_isRef === !0 : !1
}
function Ie(t) {
    return Ql(t, !1)
}
function Wv(t) {
    return Ql(t, !0)
}
function Ql(t, e) {
    return xe(t) ? t : new Cf(t,e)
}
class Cf {
    constructor(e, r) {
        this.dep = new eo,
        this.__v_isRef = !0,
        this.__v_isShallow = !1,
        this._rawValue = r ? e : le(e),
        this._value = r ? e : Le(e),
        this.__v_isShallow = r
    }
    get value() {
        return this.dep.track(),
        this._value
    }
    set value(e) {
        const r = this._rawValue
          , n = this.__v_isShallow || Xe(e) || Dt(e);
        e = n ? e : le(e),
        kt(e, r) && (this._rawValue = e,
        this._value = n ? e : Le(e),
        this.dep.trigger())
    }
}
function Is(t) {
    return xe(t) ? t.value : t
}
const If = {
    get: (t, e, r) => e === "__v_raw" ? t : Is(Reflect.get(t, e, r)),
    set: (t, e, r, n) => {
        const i = t[e];
        return xe(i) && !xe(r) ? (i.value = r,
        !0) : Reflect.set(t, e, r, n)
    }
};
function Xl(t) {
    return xt(t) ? t : new Proxy(t,If)
}
function Tf(t) {
    const e = te(t) ? new Array(t.length) : {};
    for (const r in t)
        e[r] = Zl(t, r);
    return e
}
class Af {
    constructor(e, r, n) {
        this._object = e,
        this._key = r,
        this._defaultValue = n,
        this.__v_isRef = !0,
        this._value = void 0
    }
    get value() {
        const e = this._object[this._key];
        return this._value = e === void 0 ? this._defaultValue : e
    }
    set value(e) {
        this._object[this._key] = e
    }
    get dep() {
        return af(le(this._object), this._key)
    }
}
class Rf {
    constructor(e) {
        this._getter = e,
        this.__v_isRef = !0,
        this.__v_isReadonly = !0,
        this._value = void 0
    }
    get value() {
        return this._value = this._getter()
    }
}
function Mf(t, e, r) {
    return xe(t) ? t : ie(t) ? new Rf(t) : we(t) && arguments.length > 1 ? Zl(t, e, r) : Ie(t)
}
function Zl(t, e, r) {
    const n = t[e];
    return xe(n) ? n : new Af(t,e,r)
}
class Pf {
    constructor(e, r, n) {
        this.fn = e,
        this.setter = r,
        this._value = void 0,
        this.dep = new eo(this),
        this.__v_isRef = !0,
        this.deps = void 0,
        this.depsTail = void 0,
        this.flags = 16,
        this.globalVersion = zr - 1,
        this.next = void 0,
        this.effect = this,
        this.__v_isReadonly = !r,
        this.isSSR = n
    }
    notify() {
        if (this.flags |= 16,
        !(this.flags & 8) && be !== this)
            return Ul(this, !0),
            !0
    }
    get value() {
        const e = this.dep.track();
        return Hl(this),
        e && (e.version = this.dep.version),
        this._value
    }
    set value(e) {
        this.setter && this.setter(e)
    }
}
function Of(t, e, r=!1) {
    let n, i;
    return ie(t) ? n = t : (n = t.get,
    i = t.set),
    new Pf(n,i,r)
}
const mn = {}
  , kn = new WeakMap;
let Vt;
function Lf(t, e=!1, r=Vt) {
    if (r) {
        let n = kn.get(r);
        n || kn.set(r, n = []),
        n.push(t)
    }
}
function Nf(t, e, r=ye) {
    const {immediate: n, deep: i, once: s, scheduler: o, augmentJob: a, call: l} = r
      , u = E => i ? E : Xe(E) || i === !1 || i === 0 ? Et(E, 1) : Et(E);
    let c, g, d, m, v = !1, p = !1;
    if (xe(t) ? (g = () => t.value,
    v = Xe(t)) : xt(t) ? (g = () => u(t),
    v = !0) : te(t) ? (p = !0,
    v = t.some(E => xt(E) || Xe(E)),
    g = () => t.map(E => {
        if (xe(E))
            return E.value;
        if (xt(E))
            return u(E);
        if (ie(E))
            return l ? l(E, 2) : E()
    }
    )) : ie(t) ? e ? g = l ? () => l(t, 2) : t : g = () => {
        if (d) {
            St();
            try {
                d()
            } finally {
                Ct()
            }
        }
        const E = Vt;
        Vt = c;
        try {
            return l ? l(t, 3, [m]) : t(m)
        } finally {
            Vt = E
        }
    }
    : g = st,
    e && i) {
        const E = g
          , R = i === !0 ? 1 / 0 : i;
        g = () => Et(E(), R)
    }
    const f = Bl()
      , b = () => {
        c.stop(),
        f && f.active && Ys(f.effects, c)
    }
    ;
    if (s && e) {
        const E = e;
        e = (...R) => {
            E(...R),
            b()
        }
    }
    let _ = p ? new Array(t.length).fill(mn) : mn;
    const C = E => {
        if (!(!(c.flags & 1) || !c.dirty && !E))
            if (e) {
                const R = c.run();
                if (i || v || (p ? R.some( (T, I) => kt(T, _[I])) : kt(R, _))) {
                    d && d();
                    const T = Vt;
                    Vt = c;
                    try {
                        const I = [R, _ === mn ? void 0 : p && _[0] === mn ? [] : _, m];
                        _ = R,
                        l ? l(e, 3, I) : e(...I)
                    } finally {
                        Vt = T
                    }
                }
            } else
                c.run()
    }
    ;
    return a && a(C),
    c = new Dl(g),
    c.scheduler = o ? () => o(C, !1) : C,
    m = E => Lf(E, !1, c),
    d = c.onStop = () => {
        const E = kn.get(c);
        if (E) {
            if (l)
                l(E, 4);
            else
                for (const R of E)
                    R();
            kn.delete(c)
        }
    }
    ,
    e ? n ? C(!0) : _ = c.run() : o ? o(C.bind(null, !0), !0) : c.run(),
    b.pause = c.pause.bind(c),
    b.resume = c.resume.bind(c),
    b.stop = b,
    b
}
function Et(t, e=1 / 0, r) {
    if (e <= 0 || !we(t) || t.__v_skip || (r = r || new Set,
    r.has(t)))
        return t;
    if (r.add(t),
    e--,
    xe(t))
        Et(t.value, e, r);
    else if (te(t))
        for (let n = 0; n < t.length; n++)
            Et(t[n], e, r);
    else if (Tl(t) || cr(t))
        t.forEach(n => {
            Et(n, e, r)
        }
        );
    else if (Ml(t)) {
        for (const n in t)
            Et(t[n], e, r);
        for (const n of Object.getOwnPropertySymbols(t))
            Object.prototype.propertyIsEnumerable.call(t, n) && Et(t[n], e, r)
    }
    return t
}
/**
* @vue/runtime-core v3.5.17
* (c) 2018-present Yuxi (Evan) You and Vue contributors
* @license MIT
**/
function on(t, e, r, n) {
    try {
        return n ? t(...n) : t()
    } catch (i) {
        Zn(i, e, r)
    }
}
function lt(t, e, r, n) {
    if (ie(t)) {
        const i = on(t, e, r, n);
        return i && Al(i) && i.catch(s => {
            Zn(s, e, r)
        }
        ),
        i
    }
    if (te(t)) {
        const i = [];
        for (let s = 0; s < t.length; s++)
            i.push(lt(t[s], e, r, n));
        return i
    }
}
function Zn(t, e, r, n=!0) {
    const i = e ? e.vnode : null
      , {errorHandler: s, throwUnhandledErrorInProduction: o} = e && e.appContext.config || ye;
    if (e) {
        let a = e.parent;
        const l = e.proxy
          , u = `https://vuejs.org/error-reference/#runtime-${r}`;
        for (; a; ) {
            const c = a.ec;
            if (c) {
                for (let g = 0; g < c.length; g++)
                    if (c[g](t, l, u) === !1)
                        return
            }
            a = a.parent
        }
        if (s) {
            St(),
            on(s, null, 10, [t, l, u]),
            Ct();
            return
        }
    }
    kf(t, r, i, n, o)
}
function kf(t, e, r, n=!0, i=!1) {
    if (i)
        throw t
}
const je = [];
let ft = -1;
const fr = [];
let Mt = null
  , lr = 0;
const eu = Promise.resolve();
let Bn = null;
function tu(t) {
    const e = Bn || eu;
    return t ? e.then(this ? t.bind(this) : t) : e
}
function Bf(t) {
    let e = ft + 1
      , r = je.length;
    for (; e < r; ) {
        const n = e + r >>> 1
          , i = je[n]
          , s = Yr(i);
        s < t || s === t && i.flags & 2 ? e = n + 1 : r = n
    }
    return e
}
function so(t) {
    if (!(t.flags & 1)) {
        const e = Yr(t)
          , r = je[je.length - 1];
        !r || !(t.flags & 2) && e >= Yr(r) ? je.push(t) : je.splice(Bf(e), 0, t),
        t.flags |= 1,
        ru()
    }
}
function ru() {
    Bn || (Bn = eu.then(iu))
}
function Df(t) {
    te(t) ? fr.push(...t) : Mt && t.id === -1 ? Mt.splice(lr + 1, 0, t) : t.flags & 1 || (fr.push(t),
    t.flags |= 1),
    ru()
}
function Do(t, e, r=ft + 1) {
    for (; r < je.length; r++) {
        const n = je[r];
        if (n && n.flags & 2) {
            if (t && n.id !== t.uid)
                continue;
            je.splice(r, 1),
            r--,
            n.flags & 4 && (n.flags &= -2),
            n(),
            n.flags & 4 || (n.flags &= -2)
        }
    }
}
function nu(t) {
    if (fr.length) {
        const e = [...new Set(fr)].sort( (r, n) => Yr(r) - Yr(n));
        if (fr.length = 0,
        Mt) {
            Mt.push(...e);
            return
        }
        for (Mt = e,
        lr = 0; lr < Mt.length; lr++) {
            const r = Mt[lr];
            r.flags & 4 && (r.flags &= -2),
            r.flags & 8 || r(),
            r.flags &= -2
        }
        Mt = null,
        lr = 0
    }
}
const Yr = t => t.id == null ? t.flags & 2 ? -1 : 1 / 0 : t.id;
function iu(t) {
    const e = st;
    try {
        for (ft = 0; ft < je.length; ft++) {
            const r = je[ft];
            r && !(r.flags & 8) && (r.flags & 4 && (r.flags &= -2),
            on(r, r.i, r.i ? 15 : 14),
            r.flags & 4 || (r.flags &= -2))
        }
    } finally {
        for (; ft < je.length; ft++) {
            const r = je[ft];
            r && (r.flags &= -2)
        }
        ft = -1,
        je.length = 0,
        nu(),
        Bn = null,
        (je.length || fr.length) && iu()
    }
}
let Me = null
  , su = null;
function Dn(t) {
    const e = Me;
    return Me = t,
    su = t && t.type.__scopeId || null,
    e
}
function Fn(t, e=Me, r) {
    if (!e || t._n)
        return t;
    const n = (...i) => {
        n._d && Jo(-1);
        const s = Dn(e);
        let o;
        try {
            o = t(...i)
        } finally {
            Dn(s),
            n._d && Jo(1)
        }
        return o
    }
    ;
    return n._n = !0,
    n._c = !0,
    n._d = !0,
    n
}
function zv(t, e) {
    if (Me === null)
        return t;
    const r = ii(Me)
      , n = t.dirs || (t.dirs = []);
    for (let i = 0; i < e.length; i++) {
        let[s,o,a,l=ye] = e[i];
        s && (ie(s) && (s = {
            mounted: s,
            updated: s
        }),
        s.deep && Et(o),
        n.push({
            dir: s,
            instance: r,
            value: o,
            oldValue: void 0,
            arg: a,
            modifiers: l
        }))
    }
    return t
}
function jt(t, e, r, n) {
    const i = t.dirs
      , s = e && e.dirs;
    for (let o = 0; o < i.length; o++) {
        const a = i[o];
        s && (a.oldValue = s[o].value);
        let l = a.dir[n];
        l && (St(),
        lt(l, r, 8, [t.el, a, t, e]),
        Ct())
    }
}
const ou = Symbol("_vte")
  , au = t => t.__isTeleport
  , Br = t => t && (t.disabled || t.disabled === "")
  , Fo = t => t && (t.defer || t.defer === "")
  , Uo = t => typeof SVGElement < "u" && t instanceof SVGElement
  , jo = t => typeof MathMLElement == "function" && t instanceof MathMLElement
  , Ts = (t, e) => {
    const r = t && t.to;
    return Se(r) ? e ? e(r) : null : r
}
  , lu = {
    name: "Teleport",
    __isTeleport: !0,
    process(t, e, r, n, i, s, o, a, l, u) {
        const {mc: c, pc: g, pbc: d, o: {insert: m, querySelector: v, createText: p, createComment: f}} = u
          , b = Br(e.props);
        let {shapeFlag: _, children: C, dynamicChildren: E} = e;
        if (t == null) {
            const R = e.el = p("")
              , T = e.anchor = p("");
            m(R, r, n),
            m(T, r, n);
            const I = (y, w) => {
                _ & 16 && (i && i.isCE && (i.ce._teleportTarget = y),
                c(C, y, w, i, s, o, a, l))
            }
              , h = () => {
                const y = e.target = Ts(e.props, v)
                  , w = uu(y, e, p, m);
                y && (o !== "svg" && Uo(y) ? o = "svg" : o !== "mathml" && jo(y) && (o = "mathml"),
                b || (I(y, w),
                Cn(e, !1)))
            }
            ;
            b && (I(r, T),
            Cn(e, !0)),
            Fo(e.props) ? (e.el.__isMounted = !1,
            Ue( () => {
                h(),
                delete e.el.__isMounted
            }
            , s)) : h()
        } else {
            if (Fo(e.props) && t.el.__isMounted === !1) {
                Ue( () => {
                    lu.process(t, e, r, n, i, s, o, a, l, u)
                }
                , s);
                return
            }
            e.el = t.el,
            e.targetStart = t.targetStart;
            const R = e.anchor = t.anchor
              , T = e.target = t.target
              , I = e.targetAnchor = t.targetAnchor
              , h = Br(t.props)
              , y = h ? r : T
              , w = h ? R : I;
            if (o === "svg" || Uo(T) ? o = "svg" : (o === "mathml" || jo(T)) && (o = "mathml"),
            E ? (d(t.dynamicChildren, E, y, i, s, o, a),
            co(t, e, !0)) : l || g(t, e, y, w, i, s, o, a, !1),
            b)
                h ? e.props && t.props && e.props.to !== t.props.to && (e.props.to = t.props.to) : vn(e, r, R, u, 1);
            else if ((e.props && e.props.to) !== (t.props && t.props.to)) {
                const O = e.target = Ts(e.props, v);
                O && vn(e, O, null, u, 0)
            } else
                h && vn(e, T, I, u, 1);
            Cn(e, b)
        }
    },
    remove(t, e, r, {um: n, o: {remove: i}}, s) {
        const {shapeFlag: o, children: a, anchor: l, targetStart: u, targetAnchor: c, target: g, props: d} = t;
        if (g && (i(u),
        i(c)),
        s && i(l),
        o & 16) {
            const m = s || !Br(d);
            for (let v = 0; v < a.length; v++) {
                const p = a[v];
                n(p, e, r, m, !!p.dynamicChildren)
            }
        }
    },
    move: vn,
    hydrate: Ff
};
function vn(t, e, r, {o: {insert: n}, m: i}, s=2) {
    s === 0 && n(t.targetAnchor, e, r);
    const {el: o, anchor: a, shapeFlag: l, children: u, props: c} = t
      , g = s === 2;
    if (g && n(o, e, r),
    (!g || Br(c)) && l & 16)
        for (let d = 0; d < u.length; d++)
            i(u[d], e, r, 2);
    g && n(a, e, r)
}
function Ff(t, e, r, n, i, s, {o: {nextSibling: o, parentNode: a, querySelector: l, insert: u, createText: c}}, g) {
    const d = e.target = Ts(e.props, l);
    if (d) {
        const m = Br(e.props)
          , v = d._lpa || d.firstChild;
        if (e.shapeFlag & 16)
            if (m)
                e.anchor = g(o(t), e, a(t), r, n, i, s),
                e.targetStart = v,
                e.targetAnchor = v && o(v);
            else {
                e.anchor = o(t);
                let p = v;
                for (; p; ) {
                    if (p && p.nodeType === 8) {
                        if (p.data === "teleport start anchor")
                            e.targetStart = p;
                        else if (p.data === "teleport anchor") {
                            e.targetAnchor = p,
                            d._lpa = e.targetAnchor && o(e.targetAnchor);
                            break
                        }
                    }
                    p = o(p)
                }
                e.targetAnchor || uu(d, e, c, u),
                g(v && o(v), e, d, r, n, i, s)
            }
        Cn(e, m)
    }
    return e.anchor && o(e.anchor)
}
const Uf = lu;
function Cn(t, e) {
    const r = t.ctx;
    if (r && r.ut) {
        let n, i;
        for (e ? (n = t.el,
        i = t.anchor) : (n = t.targetStart,
        i = t.targetAnchor); n && n !== i; )
            n.nodeType === 1 && n.setAttribute("data-v-owner", r.uid),
            n = n.nextSibling;
        r.ut()
    }
}
function uu(t, e, r, n) {
    const i = e.targetStart = r("")
      , s = e.targetAnchor = r("");
    return i[ou] = s,
    t && (n(i, t),
    n(s, t)),
    s
}
const Pt = Symbol("_leaveCb")
  , yn = Symbol("_enterCb");
function jf() {
    const t = {
        isMounted: !1,
        isLeaving: !1,
        isUnmounting: !1,
        leavingVNodes: new Map
    };
    return _r( () => {
        t.isMounted = !0
    }
    ),
    oo( () => {
        t.isUnmounting = !0
    }
    ),
    t
}
const Je = [Function, Array]
  , cu = {
    mode: String,
    appear: Boolean,
    persisted: Boolean,
    onBeforeEnter: Je,
    onEnter: Je,
    onAfterEnter: Je,
    onEnterCancelled: Je,
    onBeforeLeave: Je,
    onLeave: Je,
    onAfterLeave: Je,
    onLeaveCancelled: Je,
    onBeforeAppear: Je,
    onAppear: Je,
    onAfterAppear: Je,
    onAppearCancelled: Je
}
  , fu = t => {
    const e = t.subTree;
    return e.component ? fu(e.component) : e
}
  , $f = {
    name: "BaseTransition",
    props: cu,
    setup(t, {slots: e}) {
        const r = qd()
          , n = jf();
        return () => {
            const i = e.default && pu(e.default(), !0);
            if (!i || !i.length)
                return;
            const s = du(i)
              , o = le(t)
              , {mode: a} = o;
            if (n.isLeaving)
                return Ei(s);
            const l = $o(s);
            if (!l)
                return Ei(s);
            let u = As(l, o, n, r, g => u = g);
            l.type !== Be && Jr(l, u);
            let c = r.subTree && $o(r.subTree);
            if (c && c.type !== Be && !Kt(l, c) && fu(r).type !== Be) {
                let g = As(c, o, n, r);
                if (Jr(c, g),
                a === "out-in" && l.type !== Be)
                    return n.isLeaving = !0,
                    g.afterLeave = () => {
                        n.isLeaving = !1,
                        r.job.flags & 8 || r.update(),
                        delete g.afterLeave,
                        c = void 0
                    }
                    ,
                    Ei(s);
                a === "in-out" && l.type !== Be ? g.delayLeave = (d, m, v) => {
                    const p = hu(n, c);
                    p[String(c.key)] = c,
                    d[Pt] = () => {
                        m(),
                        d[Pt] = void 0,
                        delete u.delayedLeave,
                        c = void 0
                    }
                    ,
                    u.delayedLeave = () => {
                        v(),
                        delete u.delayedLeave,
                        c = void 0
                    }
                }
                : c = void 0
            } else
                c && (c = void 0);
            return s
        }
    }
};
function du(t) {
    let e = t[0];
    if (t.length > 1) {
        for (const r of t)
            if (r.type !== Be) {
                e = r;
                break
            }
    }
    return e
}
const Hf = $f;
function hu(t, e) {
    const {leavingVNodes: r} = t;
    let n = r.get(e.type);
    return n || (n = Object.create(null),
    r.set(e.type, n)),
    n
}
function As(t, e, r, n, i) {
    const {appear: s, mode: o, persisted: a=!1, onBeforeEnter: l, onEnter: u, onAfterEnter: c, onEnterCancelled: g, onBeforeLeave: d, onLeave: m, onAfterLeave: v, onLeaveCancelled: p, onBeforeAppear: f, onAppear: b, onAfterAppear: _, onAppearCancelled: C} = e
      , E = String(t.key)
      , R = hu(r, t)
      , T = (y, w) => {
        y && lt(y, n, 9, w)
    }
      , I = (y, w) => {
        const O = w[1];
        T(y, w),
        te(y) ? y.every(A => A.length <= 1) && O() : y.length <= 1 && O()
    }
      , h = {
        mode: o,
        persisted: a,
        beforeEnter(y) {
            let w = l;
            if (!r.isMounted)
                if (s)
                    w = f || l;
                else
                    return;
            y[Pt] && y[Pt](!0);
            const O = R[E];
            O && Kt(t, O) && O.el[Pt] && O.el[Pt](),
            T(w, [y])
        },
        enter(y) {
            let w = u
              , O = c
              , A = g;
            if (!r.isMounted)
                if (s)
                    w = b || u,
                    O = _ || c,
                    A = C || g;
                else
                    return;
            let k = !1;
            const V = y[yn] = $ => {
                k || (k = !0,
                $ ? T(A, [y]) : T(O, [y]),
                h.delayedLeave && h.delayedLeave(),
                y[yn] = void 0)
            }
            ;
            w ? I(w, [y, V]) : V()
        },
        leave(y, w) {
            const O = String(t.key);
            if (y[yn] && y[yn](!0),
            r.isUnmounting)
                return w();
            T(d, [y]);
            let A = !1;
            const k = y[Pt] = V => {
                A || (A = !0,
                w(),
                V ? T(p, [y]) : T(v, [y]),
                y[Pt] = void 0,
                R[O] === t && delete R[O])
            }
            ;
            R[O] = t,
            m ? I(m, [y, k]) : k()
        },
        clone(y) {
            const w = As(y, e, r, n, i);
            return i && i(w),
            w
        }
    };
    return h
}
function Ei(t) {
    if (ei(t))
        return t = Ft(t),
        t.children = null,
        t
}
function $o(t) {
    if (!ei(t))
        return au(t.type) && t.children ? du(t.children) : t;
    if (t.component)
        return t.component.subTree;
    const {shapeFlag: e, children: r} = t;
    if (r) {
        if (e & 16)
            return r[0];
        if (e & 32 && ie(r.default))
            return r.default()
    }
}
function Jr(t, e) {
    t.shapeFlag & 6 && t.component ? (t.transition = e,
    Jr(t.component.subTree, e)) : t.shapeFlag & 128 ? (t.ssContent.transition = e.clone(t.ssContent),
    t.ssFallback.transition = e.clone(t.ssFallback)) : t.transition = e
}
function pu(t, e=!1, r) {
    let n = []
      , i = 0;
    for (let s = 0; s < t.length; s++) {
        let o = t[s];
        const a = r == null ? o.key : String(r) + String(o.key != null ? o.key : s);
        o.type === $e ? (o.patchFlag & 128 && i++,
        n = n.concat(pu(o.children, e, a))) : (e || o.type !== Be) && n.push(a != null ? Ft(o, {
            key: a
        }) : o)
    }
    if (i > 1)
        for (let s = 0; s < n.length; s++)
            n[s].patchFlag = -2;
    return n
}
/*! #__NO_SIDE_EFFECTS__ */
function nr(t, e) {
    return ie(t) ? ( () => Te({
        name: t.name
    }, e, {
        setup: t
    }))() : t
}
function gu(t) {
    t.ids = [t.ids[0] + t.ids[2]++ + "-", 0, 0]
}
function Dr(t, e, r, n, i=!1) {
    if (te(t)) {
        t.forEach( (v, p) => Dr(v, e && (te(e) ? e[p] : e), r, n, i));
        return
    }
    if (dr(n) && !i) {
        n.shapeFlag & 512 && n.type.__asyncResolved && n.component.subTree.component && Dr(t, e, r, n.component.subTree);
        return
    }
    const s = n.shapeFlag & 4 ? ii(n.component) : n.el
      , o = i ? null : s
      , {i: a, r: l} = t
      , u = e && e.r
      , c = a.refs === ye ? a.refs = {} : a.refs
      , g = a.setupState
      , d = le(g)
      , m = g === ye ? () => !1 : v => pe(d, v);
    if (u != null && u !== l && (Se(u) ? (c[u] = null,
    m(u) && (g[u] = null)) : xe(u) && (u.value = null)),
    ie(l))
        on(l, a, 12, [o, c]);
    else {
        const v = Se(l)
          , p = xe(l);
        if (v || p) {
            const f = () => {
                if (t.f) {
                    const b = v ? m(l) ? g[l] : c[l] : l.value;
                    i ? te(b) && Ys(b, s) : te(b) ? b.includes(s) || b.push(s) : v ? (c[l] = [s],
                    m(l) && (g[l] = c[l])) : (l.value = [s],
                    t.k && (c[t.k] = l.value))
                } else
                    v ? (c[l] = o,
                    m(l) && (g[l] = o)) : p && (l.value = o,
                    t.k && (c[t.k] = o))
            }
            ;
            o ? (f.id = -1,
            Ue(f, r)) : f()
        }
    }
}
sn().requestIdleCallback;
sn().cancelIdleCallback;
const dr = t => !!t.type.__asyncLoader
  , ei = t => t.type.__isKeepAlive;
function qf(t, e) {
    mu(t, "a", e)
}
function Vf(t, e) {
    mu(t, "da", e)
}
function mu(t, e, r=Oe) {
    const n = t.__wdc || (t.__wdc = () => {
        let i = r;
        for (; i; ) {
            if (i.isDeactivated)
                return;
            i = i.parent
        }
        return t()
    }
    );
    if (ti(e, n, r),
    r) {
        let i = r.parent;
        for (; i && i.parent; )
            ei(i.parent.vnode) && Kf(n, e, r, i),
            i = i.parent
    }
}
function Kf(t, e, r, n) {
    const i = ti(e, t, n, !0);
    vu( () => {
        Ys(n[e], i)
    }
    , r)
}
function ti(t, e, r=Oe, n=!1) {
    if (r) {
        const i = r[t] || (r[t] = [])
          , s = e.__weh || (e.__weh = (...o) => {
            St();
            const a = an(r)
              , l = lt(e, r, t, o);
            return a(),
            Ct(),
            l
        }
        );
        return n ? i.unshift(s) : i.push(s),
        s
    }
}
const Tt = t => (e, r=Oe) => {
    (!en || t === "sp") && ti(t, (...n) => e(...n), r)
}
  , Wf = Tt("bm")
  , _r = Tt("m")
  , zf = Tt("bu")
  , Gf = Tt("u")
  , oo = Tt("bum")
  , vu = Tt("um")
  , Yf = Tt("sp")
  , Jf = Tt("rtg")
  , Qf = Tt("rtc");
function Xf(t, e=Oe) {
    ti("ec", t, e)
}
const yu = "components";
function Zf(t, e) {
    return td(yu, t, !0, e) || t
}
const ed = Symbol.for("v-ndc");
function td(t, e, r=!0, n=!1) {
    const i = Me || Oe;
    if (i) {
        const s = i.type;
        if (t === yu) {
            const a = Gd(s, !1);
            if (a && (a === e || a === et(e) || a === Jn(et(e))))
                return s
        }
        const o = Ho(i[t] || s[t], e) || Ho(i.appContext[t], e);
        return !o && n ? s : o
    }
}
function Ho(t, e) {
    return t && (t[e] || t[et(e)] || t[Jn(et(e))])
}
function rd(t, e, r, n) {
    let i;
    const s = r && r[n]
      , o = te(t);
    if (o || Se(t)) {
        const a = o && xt(t);
        let l = !1
          , u = !1;
        a && (l = !Xe(t),
        u = Dt(t),
        t = Qn(t)),
        i = new Array(t.length);
        for (let c = 0, g = t.length; c < g; c++)
            i[c] = e(l ? u ? Nn(Le(t[c])) : Le(t[c]) : t[c], c, void 0, s && s[c])
    } else if (typeof t == "number") {
        i = new Array(t);
        for (let a = 0; a < t; a++)
            i[a] = e(a + 1, a, void 0, s && s[a])
    } else if (we(t))
        if (t[Symbol.iterator])
            i = Array.from(t, (a, l) => e(a, l, void 0, s && s[l]));
        else {
            const a = Object.keys(t);
            i = new Array(a.length);
            for (let l = 0, u = a.length; l < u; l++) {
                const c = a[l];
                i[l] = e(t[c], c, l, s && s[l])
            }
        }
    else
        i = [];
    return r && (r[n] = i),
    i
}
function nd(t, e, r={}, n, i) {
    if (Me.ce || Me.parent && dr(Me.parent) && Me.parent.ce)
        return e !== "default" && (r.name = e),
        ge(),
        Xr($e, null, [Ee("slot", r, n && n())], 64);
    let s = t[e];
    s && s._c && (s._d = !1),
    ge();
    const o = s && bu(s(r))
      , a = r.key || o && o.key
      , l = Xr($e, {
        key: (a && !It(a) ? a : `_${e}`) + (!o && n ? "_fb" : "")
    }, o || (n ? n() : []), o && t._ === 1 ? 64 : -2);
    return !i && l.scopeId && (l.slotScopeIds = [l.scopeId + "-s"]),
    s && s._c && (s._d = !0),
    l
}
function bu(t) {
    return t.some(e => Zr(e) ? !(e.type === Be || e.type === $e && !bu(e.children)) : !0) ? t : null
}
const Rs = t => t ? Du(t) ? ii(t) : Rs(t.parent) : null
  , Fr = Te(Object.create(null), {
    $: t => t,
    $el: t => t.vnode.el,
    $data: t => t.data,
    $props: t => t.props,
    $attrs: t => t.attrs,
    $slots: t => t.slots,
    $refs: t => t.refs,
    $parent: t => Rs(t.parent),
    $root: t => Rs(t.root),
    $host: t => t.ce,
    $emit: t => t.emit,
    $options: t => ao(t),
    $forceUpdate: t => t.f || (t.f = () => {
        so(t.update)
    }
    ),
    $nextTick: t => t.n || (t.n = tu.bind(t.proxy)),
    $watch: t => Td.bind(t)
})
  , xi = (t, e) => t !== ye && !t.__isScriptSetup && pe(t, e)
  , id = {
    get({_: t}, e) {
        if (e === "__v_skip")
            return !0;
        const {ctx: r, setupState: n, data: i, props: s, accessCache: o, type: a, appContext: l} = t;
        let u;
        if (e[0] !== "$") {
            const m = o[e];
            if (m !== void 0)
                switch (m) {
                case 1:
                    return n[e];
                case 2:
                    return i[e];
                case 4:
                    return r[e];
                case 3:
                    return s[e]
                }
            else {
                if (xi(n, e))
                    return o[e] = 1,
                    n[e];
                if (i !== ye && pe(i, e))
                    return o[e] = 2,
                    i[e];
                if ((u = t.propsOptions[0]) && pe(u, e))
                    return o[e] = 3,
                    s[e];
                if (r !== ye && pe(r, e))
                    return o[e] = 4,
                    r[e];
                Ms && (o[e] = 0)
            }
        }
        const c = Fr[e];
        let g, d;
        if (c)
            return e === "$attrs" && ke(t.attrs, "get", ""),
            c(t);
        if ((g = a.__cssModules) && (g = g[e]))
            return g;
        if (r !== ye && pe(r, e))
            return o[e] = 4,
            r[e];
        if (d = l.config.globalProperties,
        pe(d, e))
            return d[e]
    },
    set({_: t}, e, r) {
        const {data: n, setupState: i, ctx: s} = t;
        return xi(i, e) ? (i[e] = r,
        !0) : n !== ye && pe(n, e) ? (n[e] = r,
        !0) : pe(t.props, e) || e[0] === "$" && e.slice(1)in t ? !1 : (s[e] = r,
        !0)
    },
    has({_: {data: t, setupState: e, accessCache: r, ctx: n, appContext: i, propsOptions: s}}, o) {
        let a;
        return !!r[o] || t !== ye && pe(t, o) || xi(e, o) || (a = s[0]) && pe(a, o) || pe(n, o) || pe(Fr, o) || pe(i.config.globalProperties, o)
    },
    defineProperty(t, e, r) {
        return r.get != null ? t._.accessCache[e] = 0 : pe(r, "value") && this.set(t, e, r.value, null),
        Reflect.defineProperty(t, e, r)
    }
};
function qo(t) {
    return te(t) ? t.reduce( (e, r) => (e[r] = null,
    e), {}) : t
}
let Ms = !0;
function sd(t) {
    const e = ao(t)
      , r = t.proxy
      , n = t.ctx;
    Ms = !1,
    e.beforeCreate && Vo(e.beforeCreate, t, "bc");
    const {data: i, computed: s, methods: o, watch: a, provide: l, inject: u, created: c, beforeMount: g, mounted: d, beforeUpdate: m, updated: v, activated: p, deactivated: f, beforeDestroy: b, beforeUnmount: _, destroyed: C, unmounted: E, render: R, renderTracked: T, renderTriggered: I, errorCaptured: h, serverPrefetch: y, expose: w, inheritAttrs: O, components: A, directives: k, filters: V} = e;
    if (u && od(u, n, null),
    o)
        for (const Y in o) {
            const H = o[Y];
            ie(H) && (n[Y] = H.bind(r))
        }
    if (i) {
        const Y = i.call(r, r);
        we(Y) && (t.data = Xn(Y))
    }
    if (Ms = !0,
    s)
        for (const Y in s) {
            const H = s[Y]
              , re = ie(H) ? H.bind(r, r) : ie(H.get) ? H.get.bind(r, r) : st
              , he = !ie(H) && ie(H.set) ? H.set.bind(r) : st
              , Z = at({
                get: re,
                set: he
            });
            Object.defineProperty(n, Y, {
                enumerable: !0,
                configurable: !0,
                get: () => Z.value,
                set: se => Z.value = se
            })
        }
    if (a)
        for (const Y in a)
            wu(a[Y], n, r, Y);
    if (l) {
        const Y = ie(l) ? l.call(r) : l;
        Reflect.ownKeys(Y).forEach(H => {
            dd(H, Y[H])
        }
        )
    }
    c && Vo(c, t, "c");
    function D(Y, H) {
        te(H) ? H.forEach(re => Y(re.bind(r))) : H && Y(H.bind(r))
    }
    if (D(Wf, g),
    D(_r, d),
    D(zf, m),
    D(Gf, v),
    D(qf, p),
    D(Vf, f),
    D(Xf, h),
    D(Qf, T),
    D(Jf, I),
    D(oo, _),
    D(vu, E),
    D(Yf, y),
    te(w))
        if (w.length) {
            const Y = t.exposed || (t.exposed = {});
            w.forEach(H => {
                Object.defineProperty(Y, H, {
                    get: () => r[H],
                    set: re => r[H] = re
                })
            }
            )
        } else
            t.exposed || (t.exposed = {});
    R && t.render === st && (t.render = R),
    O != null && (t.inheritAttrs = O),
    A && (t.components = A),
    k && (t.directives = k),
    y && gu(t)
}
function od(t, e, r=st) {
    te(t) && (t = Ps(t));
    for (const n in t) {
        const i = t[n];
        let s;
        we(i) ? "default"in i ? s = Ur(i.from || n, i.default, !0) : s = Ur(i.from || n) : s = Ur(i),
        xe(s) ? Object.defineProperty(e, n, {
            enumerable: !0,
            configurable: !0,
            get: () => s.value,
            set: o => s.value = o
        }) : e[n] = s
    }
}
function Vo(t, e, r) {
    lt(te(t) ? t.map(n => n.bind(e.proxy)) : t.bind(e.proxy), e, r)
}
function wu(t, e, r, n) {
    let i = n.includes(".") ? Ou(r, n) : () => r[n];
    if (Se(t)) {
        const s = e[t];
        ie(s) && In(i, s)
    } else if (ie(t))
        In(i, t.bind(r));
    else if (we(t))
        if (te(t))
            t.forEach(s => wu(s, e, r, n));
        else {
            const s = ie(t.handler) ? t.handler.bind(r) : e[t.handler];
            ie(s) && In(i, s, t)
        }
}
function ao(t) {
    const e = t.type
      , {mixins: r, extends: n} = e
      , {mixins: i, optionsCache: s, config: {optionMergeStrategies: o}} = t.appContext
      , a = s.get(e);
    let l;
    return a ? l = a : !i.length && !r && !n ? l = e : (l = {},
    i.length && i.forEach(u => Un(l, u, o, !0)),
    Un(l, e, o)),
    we(e) && s.set(e, l),
    l
}
function Un(t, e, r, n=!1) {
    const {mixins: i, extends: s} = e;
    s && Un(t, s, r, !0),
    i && i.forEach(o => Un(t, o, r, !0));
    for (const o in e)
        if (!(n && o === "expose")) {
            const a = ad[o] || r && r[o];
            t[o] = a ? a(t[o], e[o]) : e[o]
        }
    return t
}
const ad = {
    data: Ko,
    props: Wo,
    emits: Wo,
    methods: Ar,
    computed: Ar,
    beforeCreate: Fe,
    created: Fe,
    beforeMount: Fe,
    mounted: Fe,
    beforeUpdate: Fe,
    updated: Fe,
    beforeDestroy: Fe,
    beforeUnmount: Fe,
    destroyed: Fe,
    unmounted: Fe,
    activated: Fe,
    deactivated: Fe,
    errorCaptured: Fe,
    serverPrefetch: Fe,
    components: Ar,
    directives: Ar,
    watch: ud,
    provide: Ko,
    inject: ld
};
function Ko(t, e) {
    return e ? t ? function() {
        return Te(ie(t) ? t.call(this, this) : t, ie(e) ? e.call(this, this) : e)
    }
    : e : t
}
function ld(t, e) {
    return Ar(Ps(t), Ps(e))
}
function Ps(t) {
    if (te(t)) {
        const e = {};
        for (let r = 0; r < t.length; r++)
            e[t[r]] = t[r];
        return e
    }
    return t
}
function Fe(t, e) {
    return t ? [...new Set([].concat(t, e))] : e
}
function Ar(t, e) {
    return t ? Te(Object.create(null), t, e) : e
}
function Wo(t, e) {
    return t ? te(t) && te(e) ? [...new Set([...t, ...e])] : Te(Object.create(null), qo(t), qo(e != null ? e : {})) : e
}
function ud(t, e) {
    if (!t)
        return e;
    if (!e)
        return t;
    const r = Te(Object.create(null), t);
    for (const n in e)
        r[n] = Fe(t[n], e[n]);
    return r
}
function _u() {
    return {
        app: null,
        config: {
            isNativeTag: Vc,
            performance: !1,
            globalProperties: {},
            optionMergeStrategies: {},
            errorHandler: void 0,
            warnHandler: void 0,
            compilerOptions: {}
        },
        mixins: [],
        components: {},
        directives: {},
        provides: Object.create(null),
        optionsCache: new WeakMap,
        propsCache: new WeakMap,
        emitsCache: new WeakMap
    }
}
let cd = 0;
function fd(t, e) {
    return function(n, i=null) {
        ie(n) || (n = Te({}, n)),
        i != null && !we(i) && (i = null);
        const s = _u()
          , o = new WeakSet
          , a = [];
        let l = !1;
        const u = s.app = {
            _uid: cd++,
            _component: n,
            _props: i,
            _container: null,
            _context: s,
            _instance: null,
            version: Qd,
            get config() {
                return s.config
            },
            set config(c) {},
            use(c, ...g) {
                return o.has(c) || (c && ie(c.install) ? (o.add(c),
                c.install(u, ...g)) : ie(c) && (o.add(c),
                c(u, ...g))),
                u
            },
            mixin(c) {
                return s.mixins.includes(c) || s.mixins.push(c),
                u
            },
            component(c, g) {
                return g ? (s.components[c] = g,
                u) : s.components[c]
            },
            directive(c, g) {
                return g ? (s.directives[c] = g,
                u) : s.directives[c]
            },
            mount(c, g, d) {
                if (!l) {
                    const m = u._ceVNode || Ee(n, i);
                    return m.appContext = s,
                    d === !0 ? d = "svg" : d === !1 && (d = void 0),
                    g && e ? e(m, c) : t(m, c, d),
                    l = !0,
                    u._container = c,
                    c.__vue_app__ = u,
                    ii(m.component)
                }
            },
            onUnmount(c) {
                a.push(c)
            },
            unmount() {
                l && (lt(a, u._instance, 16),
                t(null, u._container),
                delete u._container.__vue_app__)
            },
            provide(c, g) {
                return s.provides[c] = g,
                u
            },
            runWithContext(c) {
                const g = Jt;
                Jt = u;
                try {
                    return c()
                } finally {
                    Jt = g
                }
            }
        };
        return u
    }
}
let Jt = null;
function dd(t, e) {
    if (Oe) {
        let r = Oe.provides;
        const n = Oe.parent && Oe.parent.provides;
        n === r && (r = Oe.provides = Object.create(n)),
        r[t] = e
    }
}
function Ur(t, e, r=!1) {
    const n = Oe || Me;
    if (n || Jt) {
        let i = Jt ? Jt._context.provides : n ? n.parent == null || n.ce ? n.vnode.appContext && n.vnode.appContext.provides : n.parent.provides : void 0;
        if (i && t in i)
            return i[t];
        if (arguments.length > 1)
            return r && ie(e) ? e.call(n && n.proxy) : e
    }
}
function hd() {
    return !!(Oe || Me || Jt)
}
const Eu = {}
  , xu = () => Object.create(Eu)
  , Su = t => Object.getPrototypeOf(t) === Eu;
function pd(t, e, r, n=!1) {
    const i = {}
      , s = xu();
    t.propsDefaults = Object.create(null),
    Cu(t, e, i, s);
    for (const o in t.propsOptions[0])
        o in i || (i[o] = void 0);
    r ? t.props = n ? i : Sf(i) : t.type.props ? t.props = i : t.props = s,
    t.attrs = s
}
function gd(t, e, r, n) {
    const {props: i, attrs: s, vnode: {patchFlag: o}} = t
      , a = le(i)
      , [l] = t.propsOptions;
    let u = !1;
    if ((n || o > 0) && !(o & 16)) {
        if (o & 8) {
            const c = t.vnode.dynamicProps;
            for (let g = 0; g < c.length; g++) {
                let d = c[g];
                if (ri(t.emitsOptions, d))
                    continue;
                const m = e[d];
                if (l)
                    if (pe(s, d))
                        m !== s[d] && (s[d] = m,
                        u = !0);
                    else {
                        const v = et(d);
                        i[v] = Os(l, a, v, m, t, !1)
                    }
                else
                    m !== s[d] && (s[d] = m,
                    u = !0)
            }
        }
    } else {
        Cu(t, e, i, s) && (u = !0);
        let c;
        for (const g in a)
            (!e || !pe(e, g) && ((c = rr(g)) === g || !pe(e, c))) && (l ? r && (r[g] !== void 0 || r[c] !== void 0) && (i[g] = Os(l, a, g, void 0, t, !0)) : delete i[g]);
        if (s !== a)
            for (const g in s)
                (!e || !pe(e, g) && !0) && (delete s[g],
                u = !0)
    }
    u && _t(t.attrs, "set", "")
}
function Cu(t, e, r, n) {
    const [i,s] = t.propsOptions;
    let o = !1, a;
    if (e)
        for (let l in e) {
            if (Lr(l))
                continue;
            const u = e[l];
            let c;
            i && pe(i, c = et(l)) ? !s || !s.includes(c) ? r[c] = u : (a || (a = {}))[c] = u : ri(t.emitsOptions, l) || (!(l in n) || u !== n[l]) && (n[l] = u,
            o = !0)
        }
    if (s) {
        const l = le(r)
          , u = a || ye;
        for (let c = 0; c < s.length; c++) {
            const g = s[c];
            r[g] = Os(i, l, g, u[g], t, !pe(u, g))
        }
    }
    return o
}
function Os(t, e, r, n, i, s) {
    const o = t[r];
    if (o != null) {
        const a = pe(o, "default");
        if (a && n === void 0) {
            const l = o.default;
            if (o.type !== Function && !o.skipFactory && ie(l)) {
                const {propsDefaults: u} = i;
                if (r in u)
                    n = u[r];
                else {
                    const c = an(i);
                    n = u[r] = l.call(null, e),
                    c()
                }
            } else
                n = l;
            i.ce && i.ce._setProp(r, n)
        }
        o[0] && (s && !a ? n = !1 : o[1] && (n === "" || n === rr(r)) && (n = !0))
    }
    return n
}
const md = new WeakMap;
function Iu(t, e, r=!1) {
    const n = r ? md : e.propsCache
      , i = n.get(t);
    if (i)
        return i;
    const s = t.props
      , o = {}
      , a = [];
    let l = !1;
    if (!ie(t)) {
        const c = g => {
            l = !0;
            const [d,m] = Iu(g, e, !0);
            Te(o, d),
            m && a.push(...m)
        }
        ;
        !r && e.mixins.length && e.mixins.forEach(c),
        t.extends && c(t.extends),
        t.mixins && t.mixins.forEach(c)
    }
    if (!s && !l)
        return we(t) && n.set(t, ur),
        ur;
    if (te(s))
        for (let c = 0; c < s.length; c++) {
            const g = et(s[c]);
            zo(g) && (o[g] = ye)
        }
    else if (s)
        for (const c in s) {
            const g = et(c);
            if (zo(g)) {
                const d = s[c]
                  , m = o[g] = te(d) || ie(d) ? {
                    type: d
                } : Te({}, d)
                  , v = m.type;
                let p = !1
                  , f = !0;
                if (te(v))
                    for (let b = 0; b < v.length; ++b) {
                        const _ = v[b]
                          , C = ie(_) && _.name;
                        if (C === "Boolean") {
                            p = !0;
                            break
                        } else
                            C === "String" && (f = !1)
                    }
                else
                    p = ie(v) && v.name === "Boolean";
                m[0] = p,
                m[1] = f,
                (p || pe(m, "default")) && a.push(g)
            }
        }
    const u = [o, a];
    return we(t) && n.set(t, u),
    u
}
function zo(t) {
    return t[0] !== "$" && !Lr(t)
}
const lo = t => t[0] === "_" || t === "$stable"
  , uo = t => te(t) ? t.map(dt) : [dt(t)]
  , vd = (t, e, r) => {
    if (e._n)
        return e;
    const n = Fn( (...i) => uo(e(...i)), r);
    return n._c = !1,
    n
}
  , Tu = (t, e, r) => {
    const n = t._ctx;
    for (const i in t) {
        if (lo(i))
            continue;
        const s = t[i];
        if (ie(s))
            e[i] = vd(i, s, n);
        else if (s != null) {
            const o = uo(s);
            e[i] = () => o
        }
    }
}
  , Au = (t, e) => {
    const r = uo(e);
    t.slots.default = () => r
}
  , Ru = (t, e, r) => {
    for (const n in e)
        (r || !lo(n)) && (t[n] = e[n])
}
  , yd = (t, e, r) => {
    const n = t.slots = xu();
    if (t.vnode.shapeFlag & 32) {
        const i = e.__;
        i && Es(n, "__", i, !0);
        const s = e._;
        s ? (Ru(n, e, r),
        r && Es(n, "_", s, !0)) : Tu(e, n)
    } else
        e && Au(t, e)
}
  , bd = (t, e, r) => {
    const {vnode: n, slots: i} = t;
    let s = !0
      , o = ye;
    if (n.shapeFlag & 32) {
        const a = e._;
        a ? r && a === 1 ? s = !1 : Ru(i, e, r) : (s = !e.$stable,
        Tu(e, i)),
        o = e
    } else
        e && (Au(t, e),
        o = {
            default: 1
        });
    if (s)
        for (const a in i)
            !lo(a) && o[a] == null && delete i[a]
}
;
function wd() {
    typeof __VUE_PROD_HYDRATION_MISMATCH_DETAILS__ != "boolean" && (sn().__VUE_PROD_HYDRATION_MISMATCH_DETAILS__ = !1)
}
const Ue = Nd;
function _d(t) {
    return Ed(t)
}
function Ed(t, e) {
    wd();
    const r = sn();
    r.__VUE__ = !0;
    const {insert: n, remove: i, patchProp: s, createElement: o, createText: a, createComment: l, setText: u, setElementText: c, parentNode: g, nextSibling: d, setScopeId: m=st, insertStaticContent: v} = t
      , p = (S, M, N, F=null, B=null, U=null, G=void 0, W=null, K=!!M.dynamicChildren) => {
        if (S === M)
            return;
        S && !Kt(S, M) && (F = Ye(S),
        se(S, B, U, !0),
        S = null),
        M.patchFlag === -2 && (K = !1,
        M.dynamicChildren = null);
        const {type: j, ref: X, shapeFlag: J} = M;
        switch (j) {
        case ni:
            f(S, M, N, F);
            break;
        case Be:
            b(S, M, N, F);
            break;
        case Ii:
            S == null && _(M, N, F, G);
            break;
        case $e:
            A(S, M, N, F, B, U, G, W, K);
            break;
        default:
            J & 1 ? R(S, M, N, F, B, U, G, W, K) : J & 6 ? k(S, M, N, F, B, U, G, W, K) : (J & 64 || J & 128) && j.process(S, M, N, F, B, U, G, W, K, P)
        }
        X != null && B ? Dr(X, S && S.ref, U, M || S, !M) : X == null && S && S.ref != null && Dr(S.ref, null, U, S, !0)
    }
      , f = (S, M, N, F) => {
        if (S == null)
            n(M.el = a(M.children), N, F);
        else {
            const B = M.el = S.el;
            M.children !== S.children && u(B, M.children)
        }
    }
      , b = (S, M, N, F) => {
        S == null ? n(M.el = l(M.children || ""), N, F) : M.el = S.el
    }
      , _ = (S, M, N, F) => {
        [S.el,S.anchor] = v(S.children, M, N, F, S.el, S.anchor)
    }
      , C = ({el: S, anchor: M}, N, F) => {
        let B;
        for (; S && S !== M; )
            B = d(S),
            n(S, N, F),
            S = B;
        n(M, N, F)
    }
      , E = ({el: S, anchor: M}) => {
        let N;
        for (; S && S !== M; )
            N = d(S),
            i(S),
            S = N;
        i(M)
    }
      , R = (S, M, N, F, B, U, G, W, K) => {
        M.type === "svg" ? G = "svg" : M.type === "math" && (G = "mathml"),
        S == null ? T(M, N, F, B, U, G, W, K) : y(S, M, B, U, G, W, K)
    }
      , T = (S, M, N, F, B, U, G, W) => {
        let K, j;
        const {props: X, shapeFlag: J, transition: Q, dirs: ee} = S;
        if (K = S.el = o(S.type, U, X && X.is, X),
        J & 8 ? c(K, S.children) : J & 16 && h(S.children, K, null, F, B, Si(S, U), G, W),
        ee && jt(S, null, F, "created"),
        I(K, S, S.scopeId, G, F),
        X) {
            for (const me in X)
                me !== "value" && !Lr(me) && s(K, me, null, X[me], U, F);
            "value"in X && s(K, "value", null, X.value, U),
            (j = X.onVnodeBeforeMount) && ct(j, F, S)
        }
        ee && jt(S, null, F, "beforeMount");
        const oe = xd(B, Q);
        oe && Q.beforeEnter(K),
        n(K, M, N),
        ((j = X && X.onVnodeMounted) || oe || ee) && Ue( () => {
            j && ct(j, F, S),
            oe && Q.enter(K),
            ee && jt(S, null, F, "mounted")
        }
        , B)
    }
      , I = (S, M, N, F, B) => {
        if (N && m(S, N),
        F)
            for (let U = 0; U < F.length; U++)
                m(S, F[U]);
        if (B) {
            let U = B.subTree;
            if (M === U || Nu(U.type) && (U.ssContent === M || U.ssFallback === M)) {
                const G = B.vnode;
                I(S, G, G.scopeId, G.slotScopeIds, B.parent)
            }
        }
    }
      , h = (S, M, N, F, B, U, G, W, K=0) => {
        for (let j = K; j < S.length; j++) {
            const X = S[j] = W ? Ot(S[j]) : dt(S[j]);
            p(null, X, M, N, F, B, U, G, W)
        }
    }
      , y = (S, M, N, F, B, U, G) => {
        const W = M.el = S.el;
        let {patchFlag: K, dynamicChildren: j, dirs: X} = M;
        K |= S.patchFlag & 16;
        const J = S.props || ye
          , Q = M.props || ye;
        let ee;
        if (N && $t(N, !1),
        (ee = Q.onVnodeBeforeUpdate) && ct(ee, N, M, S),
        X && jt(M, S, N, "beforeUpdate"),
        N && $t(N, !0),
        (J.innerHTML && Q.innerHTML == null || J.textContent && Q.textContent == null) && c(W, ""),
        j ? w(S.dynamicChildren, j, W, N, F, Si(M, B), U) : G || H(S, M, W, null, N, F, Si(M, B), U, !1),
        K > 0) {
            if (K & 16)
                O(W, J, Q, N, B);
            else if (K & 2 && J.class !== Q.class && s(W, "class", null, Q.class, B),
            K & 4 && s(W, "style", J.style, Q.style, B),
            K & 8) {
                const oe = M.dynamicProps;
                for (let me = 0; me < oe.length; me++) {
                    const ce = oe[me]
                      , Ce = J[ce]
                      , Re = Q[ce];
                    (Re !== Ce || ce === "value") && s(W, ce, Ce, Re, B, N)
                }
            }
            K & 1 && S.children !== M.children && c(W, M.children)
        } else
            !G && j == null && O(W, J, Q, N, B);
        ((ee = Q.onVnodeUpdated) || X) && Ue( () => {
            ee && ct(ee, N, M, S),
            X && jt(M, S, N, "updated")
        }
        , F)
    }
      , w = (S, M, N, F, B, U, G) => {
        for (let W = 0; W < M.length; W++) {
            const K = S[W]
              , j = M[W]
              , X = K.el && (K.type === $e || !Kt(K, j) || K.shapeFlag & 198) ? g(K.el) : N;
            p(K, j, X, null, F, B, U, G, !0)
        }
    }
      , O = (S, M, N, F, B) => {
        if (M !== N) {
            if (M !== ye)
                for (const U in M)
                    !Lr(U) && !(U in N) && s(S, U, M[U], null, B, F);
            for (const U in N) {
                if (Lr(U))
                    continue;
                const G = N[U]
                  , W = M[U];
                G !== W && U !== "value" && s(S, U, W, G, B, F)
            }
            "value"in N && s(S, "value", M.value, N.value, B)
        }
    }
      , A = (S, M, N, F, B, U, G, W, K) => {
        const j = M.el = S ? S.el : a("")
          , X = M.anchor = S ? S.anchor : a("");
        let {patchFlag: J, dynamicChildren: Q, slotScopeIds: ee} = M;
        ee && (W = W ? W.concat(ee) : ee),
        S == null ? (n(j, N, F),
        n(X, N, F),
        h(M.children || [], N, X, B, U, G, W, K)) : J > 0 && J & 64 && Q && S.dynamicChildren ? (w(S.dynamicChildren, Q, N, B, U, G, W),
        (M.key != null || B && M === B.subTree) && co(S, M, !0)) : H(S, M, N, X, B, U, G, W, K)
    }
      , k = (S, M, N, F, B, U, G, W, K) => {
        M.slotScopeIds = W,
        S == null ? M.shapeFlag & 512 ? B.ctx.activate(M, N, F, G, K) : V(M, N, F, B, U, G, K) : $(S, M, K)
    }
      , V = (S, M, N, F, B, U, G) => {
        const W = S.component = Hd(S, F, B);
        if (ei(S) && (W.ctx.renderer = P),
        Vd(W, !1, G),
        W.asyncDep) {
            if (B && B.registerDep(W, D, G),
            !S.el) {
                const K = W.subTree = Ee(Be);
                b(null, K, M, N)
            }
        } else
            D(W, S, M, N, B, U, G)
    }
      , $ = (S, M, N) => {
        const F = M.component = S.component;
        if (Od(S, M, N))
            if (F.asyncDep && !F.asyncResolved) {
                Y(F, M, N);
                return
            } else
                F.next = M,
                F.update();
        else
            M.el = S.el,
            F.vnode = M
    }
      , D = (S, M, N, F, B, U, G) => {
        const W = () => {
            if (S.isMounted) {
                let {next: J, bu: Q, u: ee, parent: oe, vnode: me} = S;
                {
                    const Ve = Mu(S);
                    if (Ve) {
                        J && (J.el = me.el,
                        Y(S, J, G)),
                        Ve.asyncDep.then( () => {
                            S.isUnmounted || W()
                        }
                        );
                        return
                    }
                }
                let ce = J, Ce;
                $t(S, !1),
                J ? (J.el = me.el,
                Y(S, J, G)) : J = me,
                Q && vi(Q),
                (Ce = J.props && J.props.onVnodeBeforeUpdate) && ct(Ce, oe, J, me),
                $t(S, !0);
                const Re = Ci(S)
                  , rt = S.subTree;
                S.subTree = Re,
                p(rt, Re, g(rt.el), Ye(rt), S, B, U),
                J.el = Re.el,
                ce === null && Ld(S, Re.el),
                ee && Ue(ee, B),
                (Ce = J.props && J.props.onVnodeUpdated) && Ue( () => ct(Ce, oe, J, me), B)
            } else {
                let J;
                const {el: Q, props: ee} = M
                  , {bm: oe, m: me, parent: ce, root: Ce, type: Re} = S
                  , rt = dr(M);
                if ($t(S, !1),
                oe && vi(oe),
                !rt && (J = ee && ee.onVnodeBeforeMount) && ct(J, ce, M),
                $t(S, !0),
                Q && q) {
                    const Ve = () => {
                        S.subTree = Ci(S),
                        q(Q, S.subTree, S, B, null)
                    }
                    ;
                    rt && Re.__asyncHydrate ? Re.__asyncHydrate(Q, S, Ve) : Ve()
                } else {
                    Ce.ce && Ce.ce._def.shadowRoot !== !1 && Ce.ce._injectChildStyle(Re);
                    const Ve = S.subTree = Ci(S);
                    p(null, Ve, N, F, S, B, U),
                    M.el = Ve.el
                }
                if (me && Ue(me, B),
                !rt && (J = ee && ee.onVnodeMounted)) {
                    const Ve = M;
                    Ue( () => ct(J, ce, Ve), B)
                }
                (M.shapeFlag & 256 || ce && dr(ce.vnode) && ce.vnode.shapeFlag & 256) && S.a && Ue(S.a, B),
                S.isMounted = !0,
                M = N = F = null
            }
        }
        ;
        S.scope.on();
        const K = S.effect = new Dl(W);
        S.scope.off();
        const j = S.update = K.run.bind(K)
          , X = S.job = K.runIfDirty.bind(K);
        X.i = S,
        X.id = S.uid,
        K.scheduler = () => so(X),
        $t(S, !0),
        j()
    }
      , Y = (S, M, N) => {
        M.component = S;
        const F = S.vnode.props;
        S.vnode = M,
        S.next = null,
        gd(S, M.props, F, N),
        bd(S, M.children, N),
        St(),
        Do(S),
        Ct()
    }
      , H = (S, M, N, F, B, U, G, W, K=!1) => {
        const j = S && S.children
          , X = S ? S.shapeFlag : 0
          , J = M.children
          , {patchFlag: Q, shapeFlag: ee} = M;
        if (Q > 0) {
            if (Q & 128) {
                he(j, J, N, F, B, U, G, W, K);
                return
            } else if (Q & 256) {
                re(j, J, N, F, B, U, G, W, K);
                return
            }
        }
        ee & 8 ? (X & 16 && ne(j, B, U),
        J !== j && c(N, J)) : X & 16 ? ee & 16 ? he(j, J, N, F, B, U, G, W, K) : ne(j, B, U, !0) : (X & 8 && c(N, ""),
        ee & 16 && h(J, N, F, B, U, G, W, K))
    }
      , re = (S, M, N, F, B, U, G, W, K) => {
        S = S || ur,
        M = M || ur;
        const j = S.length
          , X = M.length
          , J = Math.min(j, X);
        let Q;
        for (Q = 0; Q < J; Q++) {
            const ee = M[Q] = K ? Ot(M[Q]) : dt(M[Q]);
            p(S[Q], ee, N, null, B, U, G, W, K)
        }
        j > X ? ne(S, B, U, !0, !1, J) : h(M, N, F, B, U, G, W, K, J)
    }
      , he = (S, M, N, F, B, U, G, W, K) => {
        let j = 0;
        const X = M.length;
        let J = S.length - 1
          , Q = X - 1;
        for (; j <= J && j <= Q; ) {
            const ee = S[j]
              , oe = M[j] = K ? Ot(M[j]) : dt(M[j]);
            if (Kt(ee, oe))
                p(ee, oe, N, null, B, U, G, W, K);
            else
                break;
            j++
        }
        for (; j <= J && j <= Q; ) {
            const ee = S[J]
              , oe = M[Q] = K ? Ot(M[Q]) : dt(M[Q]);
            if (Kt(ee, oe))
                p(ee, oe, N, null, B, U, G, W, K);
            else
                break;
            J--,
            Q--
        }
        if (j > J) {
            if (j <= Q) {
                const ee = Q + 1
                  , oe = ee < X ? M[ee].el : F;
                for (; j <= Q; )
                    p(null, M[j] = K ? Ot(M[j]) : dt(M[j]), N, oe, B, U, G, W, K),
                    j++
            }
        } else if (j > Q)
            for (; j <= J; )
                se(S[j], B, U, !0),
                j++;
        else {
            const ee = j
              , oe = j
              , me = new Map;
            for (j = oe; j <= Q; j++) {
                const Ke = M[j] = K ? Ot(M[j]) : dt(M[j]);
                Ke.key != null && me.set(Ke.key, j)
            }
            let ce, Ce = 0;
            const Re = Q - oe + 1;
            let rt = !1
              , Ve = 0;
            const Er = new Array(Re);
            for (j = 0; j < Re; j++)
                Er[j] = 0;
            for (j = ee; j <= J; j++) {
                const Ke = S[j];
                if (Ce >= Re) {
                    se(Ke, B, U, !0);
                    continue
                }
                let ut;
                if (Ke.key != null)
                    ut = me.get(Ke.key);
                else
                    for (ce = oe; ce <= Q; ce++)
                        if (Er[ce - oe] === 0 && Kt(Ke, M[ce])) {
                            ut = ce;
                            break
                        }
                ut === void 0 ? se(Ke, B, U, !0) : (Er[ut - oe] = j + 1,
                ut >= Ve ? Ve = ut : rt = !0,
                p(Ke, M[ut], N, null, B, U, G, W, K),
                Ce++)
            }
            const Oo = rt ? Sd(Er) : ur;
            for (ce = Oo.length - 1,
            j = Re - 1; j >= 0; j--) {
                const Ke = oe + j
                  , ut = M[Ke]
                  , Lo = Ke + 1 < X ? M[Ke + 1].el : F;
                Er[j] === 0 ? p(null, ut, N, Lo, B, U, G, W, K) : rt && (ce < 0 || j !== Oo[ce] ? Z(ut, N, Lo, 2) : ce--)
            }
        }
    }
      , Z = (S, M, N, F, B=null) => {
        const {el: U, type: G, transition: W, children: K, shapeFlag: j} = S;
        if (j & 6) {
            Z(S.component.subTree, M, N, F);
            return
        }
        if (j & 128) {
            S.suspense.move(M, N, F);
            return
        }
        if (j & 64) {
            G.move(S, M, N, P);
            return
        }
        if (G === $e) {
            n(U, M, N);
            for (let J = 0; J < K.length; J++)
                Z(K[J], M, N, F);
            n(S.anchor, M, N);
            return
        }
        if (G === Ii) {
            C(S, M, N);
            return
        }
        if (F !== 2 && j & 1 && W)
            if (F === 0)
                W.beforeEnter(U),
                n(U, M, N),
                Ue( () => W.enter(U), B);
            else {
                const {leave: J, delayLeave: Q, afterLeave: ee} = W
                  , oe = () => {
                    S.ctx.isUnmounted ? i(U) : n(U, M, N)
                }
                  , me = () => {
                    J(U, () => {
                        oe(),
                        ee && ee()
                    }
                    )
                }
                ;
                Q ? Q(U, oe, me) : me()
            }
        else
            n(U, M, N)
    }
      , se = (S, M, N, F=!1, B=!1) => {
        const {type: U, props: G, ref: W, children: K, dynamicChildren: j, shapeFlag: X, patchFlag: J, dirs: Q, cacheIndex: ee} = S;
        if (J === -2 && (B = !1),
        W != null && (St(),
        Dr(W, null, N, S, !0),
        Ct()),
        ee != null && (M.renderCache[ee] = void 0),
        X & 256) {
            M.ctx.deactivate(S);
            return
        }
        const oe = X & 1 && Q
          , me = !dr(S);
        let ce;
        if (me && (ce = G && G.onVnodeBeforeUnmount) && ct(ce, M, S),
        X & 6)
            ae(S.component, N, F);
        else {
            if (X & 128) {
                S.suspense.unmount(N, F);
                return
            }
            oe && jt(S, null, M, "beforeUnmount"),
            X & 64 ? S.type.remove(S, M, N, P, F) : j && !j.hasOnce && (U !== $e || J > 0 && J & 64) ? ne(j, M, N, !1, !0) : (U === $e && J & 384 || !B && X & 16) && ne(K, M, N),
            F && ve(S)
        }
        (me && (ce = G && G.onVnodeUnmounted) || oe) && Ue( () => {
            ce && ct(ce, M, S),
            oe && jt(S, null, M, "unmounted")
        }
        , N)
    }
      , ve = S => {
        const {type: M, el: N, anchor: F, transition: B} = S;
        if (M === $e) {
            Ae(N, F);
            return
        }
        if (M === Ii) {
            E(S);
            return
        }
        const U = () => {
            i(N),
            B && !B.persisted && B.afterLeave && B.afterLeave()
        }
        ;
        if (S.shapeFlag & 1 && B && !B.persisted) {
            const {leave: G, delayLeave: W} = B
              , K = () => G(N, U);
            W ? W(S.el, U, K) : K()
        } else
            U()
    }
      , Ae = (S, M) => {
        let N;
        for (; S !== M; )
            N = d(S),
            i(S),
            S = N;
        i(M)
    }
      , ae = (S, M, N) => {
        const {bum: F, scope: B, job: U, subTree: G, um: W, m: K, a: j, parent: X, slots: {__: J}} = S;
        Go(K),
        Go(j),
        F && vi(F),
        X && te(J) && J.forEach(Q => {
            X.renderCache[Q] = void 0
        }
        ),
        B.stop(),
        U && (U.flags |= 8,
        se(G, S, M, N)),
        W && Ue(W, M),
        Ue( () => {
            S.isUnmounted = !0
        }
        , M),
        M && M.pendingBranch && !M.isUnmounted && S.asyncDep && !S.asyncResolved && S.suspenseId === M.pendingId && (M.deps--,
        M.deps === 0 && M.resolve())
    }
      , ne = (S, M, N, F=!1, B=!1, U=0) => {
        for (let G = U; G < S.length; G++)
            se(S[G], M, N, F, B)
    }
      , Ye = S => {
        if (S.shapeFlag & 6)
            return Ye(S.component.subTree);
        if (S.shapeFlag & 128)
            return S.suspense.next();
        const M = d(S.anchor || S.el)
          , N = M && M[ou];
        return N ? d(N) : M
    }
    ;
    let z = !1;
    const x = (S, M, N) => {
        S == null ? M._vnode && se(M._vnode, null, null, !0) : p(M._vnode || null, S, M, null, null, null, N),
        M._vnode = S,
        z || (z = !0,
        Do(),
        nu(),
        z = !1)
    }
      , P = {
        p,
        um: se,
        m: Z,
        r: ve,
        mt: V,
        mc: h,
        pc: H,
        pbc: w,
        n: Ye,
        o: t
    };
    let L, q;
    return e && ([L,q] = e(P)),
    {
        render: x,
        hydrate: L,
        createApp: fd(x, L)
    }
}
function Si({type: t, props: e}, r) {
    return r === "svg" && t === "foreignObject" || r === "mathml" && t === "annotation-xml" && e && e.encoding && e.encoding.includes("html") ? void 0 : r
}
function $t({effect: t, job: e}, r) {
    r ? (t.flags |= 32,
    e.flags |= 4) : (t.flags &= -33,
    e.flags &= -5)
}
function xd(t, e) {
    return (!t || t && !t.pendingBranch) && e && !e.persisted
}
function co(t, e, r=!1) {
    const n = t.children
      , i = e.children;
    if (te(n) && te(i))
        for (let s = 0; s < n.length; s++) {
            const o = n[s];
            let a = i[s];
            a.shapeFlag & 1 && !a.dynamicChildren && ((a.patchFlag <= 0 || a.patchFlag === 32) && (a = i[s] = Ot(i[s]),
            a.el = o.el),
            !r && a.patchFlag !== -2 && co(o, a)),
            a.type === ni && (a.el = o.el),
            a.type === Be && !a.el && (a.el = o.el)
        }
}
function Sd(t) {
    const e = t.slice()
      , r = [0];
    let n, i, s, o, a;
    const l = t.length;
    for (n = 0; n < l; n++) {
        const u = t[n];
        if (u !== 0) {
            if (i = r[r.length - 1],
            t[i] < u) {
                e[n] = i,
                r.push(n);
                continue
            }
            for (s = 0,
            o = r.length - 1; s < o; )
                a = s + o >> 1,
                t[r[a]] < u ? s = a + 1 : o = a;
            u < t[r[s]] && (s > 0 && (e[n] = r[s - 1]),
            r[s] = n)
        }
    }
    for (s = r.length,
    o = r[s - 1]; s-- > 0; )
        r[s] = o,
        o = e[o];
    return r
}
function Mu(t) {
    const e = t.subTree.component;
    if (e)
        return e.asyncDep && !e.asyncResolved ? e : Mu(e)
}
function Go(t) {
    if (t)
        for (let e = 0; e < t.length; e++)
            t[e].flags |= 8
}
const Cd = Symbol.for("v-scx")
  , Id = () => Ur(Cd);
function In(t, e, r) {
    return Pu(t, e, r)
}
function Pu(t, e, r=ye) {
    const {immediate: n, deep: i, flush: s, once: o} = r
      , a = Te({}, r)
      , l = e && n || !e && s !== "post";
    let u;
    if (en) {
        if (s === "sync") {
            const m = Id();
            u = m.__watcherHandles || (m.__watcherHandles = [])
        } else if (!l) {
            const m = () => {}
            ;
            return m.stop = st,
            m.resume = st,
            m.pause = st,
            m
        }
    }
    const c = Oe;
    a.call = (m, v, p) => lt(m, c, v, p);
    let g = !1;
    s === "post" ? a.scheduler = m => {
        Ue(m, c && c.suspense)
    }
    : s !== "sync" && (g = !0,
    a.scheduler = (m, v) => {
        v ? m() : so(m)
    }
    ),
    a.augmentJob = m => {
        e && (m.flags |= 4),
        g && (m.flags |= 2,
        c && (m.id = c.uid,
        m.i = c))
    }
    ;
    const d = Nf(t, e, a);
    return en && (u ? u.push(d) : l && d()),
    d
}
function Td(t, e, r) {
    const n = this.proxy
      , i = Se(t) ? t.includes(".") ? Ou(n, t) : () => n[t] : t.bind(n, n);
    let s;
    ie(e) ? s = e : (s = e.handler,
    r = e);
    const o = an(this)
      , a = Pu(i, s.bind(n), r);
    return o(),
    a
}
function Ou(t, e) {
    const r = e.split(".");
    return () => {
        let n = t;
        for (let i = 0; i < r.length && n; i++)
            n = n[r[i]];
        return n
    }
}
const Ad = (t, e) => e === "modelValue" || e === "model-value" ? t.modelModifiers : t[`${e}Modifiers`] || t[`${et(e)}Modifiers`] || t[`${rr(e)}Modifiers`];
function Rd(t, e, ...r) {
    if (t.isUnmounted)
        return;
    const n = t.vnode.props || ye;
    let i = r;
    const s = e.startsWith("update:")
      , o = s && Ad(n, e.slice(7));
    o && (o.trim && (i = r.map(c => Se(c) ? c.trim() : c)),
    o.number && (i = r.map(Yc)));
    let a, l = n[a = mi(e)] || n[a = mi(et(e))];
    !l && s && (l = n[a = mi(rr(e))]),
    l && lt(l, t, 6, i);
    const u = n[a + "Once"];
    if (u) {
        if (!t.emitted)
            t.emitted = {};
        else if (t.emitted[a])
            return;
        t.emitted[a] = !0,
        lt(u, t, 6, i)
    }
}
function Lu(t, e, r=!1) {
    const n = e.emitsCache
      , i = n.get(t);
    if (i !== void 0)
        return i;
    const s = t.emits;
    let o = {}
      , a = !1;
    if (!ie(t)) {
        const l = u => {
            const c = Lu(u, e, !0);
            c && (a = !0,
            Te(o, c))
        }
        ;
        !r && e.mixins.length && e.mixins.forEach(l),
        t.extends && l(t.extends),
        t.mixins && t.mixins.forEach(l)
    }
    return !s && !a ? (we(t) && n.set(t, null),
    null) : (te(s) ? s.forEach(l => o[l] = null) : Te(o, s),
    we(t) && n.set(t, o),
    o)
}
function ri(t, e) {
    return !t || !zn(e) ? !1 : (e = e.slice(2).replace(/Once$/, ""),
    pe(t, e[0].toLowerCase() + e.slice(1)) || pe(t, rr(e)) || pe(t, e))
}
function Ci(t) {
    const {type: e, vnode: r, proxy: n, withProxy: i, propsOptions: [s], slots: o, attrs: a, emit: l, render: u, renderCache: c, props: g, data: d, setupState: m, ctx: v, inheritAttrs: p} = t
      , f = Dn(t);
    let b, _;
    try {
        if (r.shapeFlag & 4) {
            const E = i || n
              , R = E;
            b = dt(u.call(R, E, c, g, m, d, v)),
            _ = a
        } else {
            const E = e;
            b = dt(E.length > 1 ? E(g, {
                attrs: a,
                slots: o,
                emit: l
            }) : E(g, null)),
            _ = e.props ? a : Md(a)
        }
    } catch (E) {
        jr.length = 0,
        Zn(E, t, 1),
        b = Ee(Be)
    }
    let C = b;
    if (_ && p !== !1) {
        const E = Object.keys(_)
          , {shapeFlag: R} = C;
        E.length && R & 7 && (s && E.some(Gs) && (_ = Pd(_, s)),
        C = Ft(C, _, !1, !0))
    }
    return r.dirs && (C = Ft(C, null, !1, !0),
    C.dirs = C.dirs ? C.dirs.concat(r.dirs) : r.dirs),
    r.transition && Jr(C, r.transition),
    b = C,
    Dn(f),
    b
}
const Md = t => {
    let e;
    for (const r in t)
        (r === "class" || r === "style" || zn(r)) && ((e || (e = {}))[r] = t[r]);
    return e
}
  , Pd = (t, e) => {
    const r = {};
    for (const n in t)
        (!Gs(n) || !(n.slice(9)in e)) && (r[n] = t[n]);
    return r
}
;
function Od(t, e, r) {
    const {props: n, children: i, component: s} = t
      , {props: o, children: a, patchFlag: l} = e
      , u = s.emitsOptions;
    if (e.dirs || e.transition)
        return !0;
    if (r && l >= 0) {
        if (l & 1024)
            return !0;
        if (l & 16)
            return n ? Yo(n, o, u) : !!o;
        if (l & 8) {
            const c = e.dynamicProps;
            for (let g = 0; g < c.length; g++) {
                const d = c[g];
                if (o[d] !== n[d] && !ri(u, d))
                    return !0
            }
        }
    } else
        return (i || a) && (!a || !a.$stable) ? !0 : n === o ? !1 : n ? o ? Yo(n, o, u) : !0 : !!o;
    return !1
}
function Yo(t, e, r) {
    const n = Object.keys(e);
    if (n.length !== Object.keys(t).length)
        return !0;
    for (let i = 0; i < n.length; i++) {
        const s = n[i];
        if (e[s] !== t[s] && !ri(r, s))
            return !0
    }
    return !1
}
function Ld({vnode: t, parent: e}, r) {
    for (; e; ) {
        const n = e.subTree;
        if (n.suspense && n.suspense.activeBranch === t && (n.el = t.el),
        n === t)
            (t = e.vnode).el = r,
            e = e.parent;
        else
            break
    }
}
const Nu = t => t.__isSuspense;
function Nd(t, e) {
    e && e.pendingBranch ? te(t) ? e.effects.push(...t) : e.effects.push(t) : Df(t)
}
const $e = Symbol.for("v-fgt")
  , ni = Symbol.for("v-txt")
  , Be = Symbol.for("v-cmt")
  , Ii = Symbol.for("v-stc")
  , jr = [];
let ze = null;
function ge(t=!1) {
    jr.push(ze = t ? null : [])
}
function kd() {
    jr.pop(),
    ze = jr[jr.length - 1] || null
}
let Qr = 1;
function Jo(t, e=!1) {
    Qr += t,
    t < 0 && ze && e && (ze.hasOnce = !0)
}
function ku(t) {
    return t.dynamicChildren = Qr > 0 ? ze || ur : null,
    kd(),
    Qr > 0 && ze && ze.push(t),
    t
}
function _e(t, e, r, n, i, s) {
    return ku(ue(t, e, r, n, i, s, !0))
}
function Xr(t, e, r, n, i) {
    return ku(Ee(t, e, r, n, i, !0))
}
function Zr(t) {
    return t ? t.__v_isVNode === !0 : !1
}
function Kt(t, e) {
    return t.type === e.type && t.key === e.key
}
const Bu = ({key: t}) => t != null ? t : null
  , Tn = ({ref: t, ref_key: e, ref_for: r}) => (typeof t == "number" && (t = "" + t),
t != null ? Se(t) || xe(t) || ie(t) ? {
    i: Me,
    r: t,
    k: e,
    f: !!r
} : t : null);
function ue(t, e=null, r=null, n=0, i=null, s=t === $e ? 0 : 1, o=!1, a=!1) {
    const l = {
        __v_isVNode: !0,
        __v_skip: !0,
        type: t,
        props: e,
        key: e && Bu(e),
        ref: e && Tn(e),
        scopeId: su,
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
        shapeFlag: s,
        patchFlag: n,
        dynamicProps: i,
        dynamicChildren: null,
        appContext: null,
        ctx: Me
    };
    return a ? (fo(l, r),
    s & 128 && t.normalize(l)) : r && (l.shapeFlag |= Se(r) ? 8 : 16),
    Qr > 0 && !o && ze && (l.patchFlag > 0 || s & 6) && l.patchFlag !== 32 && ze.push(l),
    l
}
const Ee = Bd;
function Bd(t, e=null, r=null, n=0, i=null, s=!1) {
    if ((!t || t === ed) && (t = Be),
    Zr(t)) {
        const a = Ft(t, e, !0);
        return r && fo(a, r),
        Qr > 0 && !s && ze && (a.shapeFlag & 6 ? ze[ze.indexOf(t)] = a : ze.push(a)),
        a.patchFlag = -2,
        a
    }
    if (Yd(t) && (t = t.__vccOpts),
    e) {
        e = Dd(e);
        let {class: a, style: l} = e;
        a && !Se(a) && (e.class = Bt(a)),
        we(l) && (no(l) && !te(l) && (l = Te({}, l)),
        e.style = He(l))
    }
    const o = Se(t) ? 1 : Nu(t) ? 128 : au(t) ? 64 : we(t) ? 4 : ie(t) ? 2 : 0;
    return ue(t, e, r, n, i, o, s, !0)
}
function Dd(t) {
    return t ? no(t) || Su(t) ? Te({}, t) : t : null
}
function Ft(t, e, r=!1, n=!1) {
    const {props: i, ref: s, patchFlag: o, children: a, transition: l} = t
      , u = e ? Ud(i || {}, e) : i
      , c = {
        __v_isVNode: !0,
        __v_skip: !0,
        type: t.type,
        props: u,
        key: u && Bu(u),
        ref: e && e.ref ? r && s ? te(s) ? s.concat(Tn(e)) : [s, Tn(e)] : Tn(e) : s,
        scopeId: t.scopeId,
        slotScopeIds: t.slotScopeIds,
        children: a,
        target: t.target,
        targetStart: t.targetStart,
        targetAnchor: t.targetAnchor,
        staticCount: t.staticCount,
        shapeFlag: t.shapeFlag,
        patchFlag: e && t.type !== $e ? o === -1 ? 16 : o | 16 : o,
        dynamicProps: t.dynamicProps,
        dynamicChildren: t.dynamicChildren,
        appContext: t.appContext,
        dirs: t.dirs,
        transition: l,
        component: t.component,
        suspense: t.suspense,
        ssContent: t.ssContent && Ft(t.ssContent),
        ssFallback: t.ssFallback && Ft(t.ssFallback),
        el: t.el,
        anchor: t.anchor,
        ctx: t.ctx,
        ce: t.ce
    };
    return l && n && Jr(c, l.clone(c)),
    c
}
function Fd(t=" ", e=0) {
    return Ee(ni, null, t, e)
}
function Qt(t="", e=!1) {
    return e ? (ge(),
    Xr(Be, null, t)) : Ee(Be, null, t)
}
function dt(t) {
    return t == null || typeof t == "boolean" ? Ee(Be) : te(t) ? Ee($e, null, t.slice()) : Zr(t) ? Ot(t) : Ee(ni, null, String(t))
}
function Ot(t) {
    return t.el === null && t.patchFlag !== -1 || t.memo ? t : Ft(t)
}
function fo(t, e) {
    let r = 0;
    const {shapeFlag: n} = t;
    if (e == null)
        e = null;
    else if (te(e))
        r = 16;
    else if (typeof e == "object")
        if (n & 65) {
            const i = e.default;
            i && (i._c && (i._d = !1),
            fo(t, i()),
            i._c && (i._d = !0));
            return
        } else {
            r = 32;
            const i = e._;
            !i && !Su(e) ? e._ctx = Me : i === 3 && Me && (Me.slots._ === 1 ? e._ = 1 : (e._ = 2,
            t.patchFlag |= 1024))
        }
    else
        ie(e) ? (e = {
            default: e,
            _ctx: Me
        },
        r = 32) : (e = String(e),
        n & 64 ? (r = 16,
        e = [Fd(e)]) : r = 8);
    t.children = e,
    t.shapeFlag |= r
}
function Ud(...t) {
    const e = {};
    for (let r = 0; r < t.length; r++) {
        const n = t[r];
        for (const i in n)
            if (i === "class")
                e.class !== n.class && (e.class = Bt([e.class, n.class]));
            else if (i === "style")
                e.style = He([e.style, n.style]);
            else if (zn(i)) {
                const s = e[i]
                  , o = n[i];
                o && s !== o && !(te(s) && s.includes(o)) && (e[i] = s ? [].concat(s, o) : o)
            } else
                i !== "" && (e[i] = n[i])
    }
    return e
}
function ct(t, e, r, n=null) {
    lt(t, e, 7, [r, n])
}
const jd = _u();
let $d = 0;
function Hd(t, e, r) {
    const n = t.type
      , i = (e ? e.appContext : t.appContext) || jd
      , s = {
        uid: $d++,
        vnode: t,
        type: n,
        parent: e,
        appContext: i,
        root: null,
        next: null,
        subTree: null,
        effect: null,
        update: null,
        job: null,
        scope: new Nl(!0),
        render: null,
        proxy: null,
        exposed: null,
        exposeProxy: null,
        withProxy: null,
        provides: e ? e.provides : Object.create(i.provides),
        ids: e ? e.ids : ["", 0, 0],
        accessCache: null,
        renderCache: [],
        components: null,
        directives: null,
        propsOptions: Iu(n, i),
        emitsOptions: Lu(n, i),
        emit: null,
        emitted: null,
        propsDefaults: ye,
        inheritAttrs: n.inheritAttrs,
        ctx: ye,
        data: ye,
        props: ye,
        attrs: ye,
        slots: ye,
        refs: ye,
        setupState: ye,
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
        sp: null
    };
    return s.ctx = {
        _: s
    },
    s.root = e ? e.root : s,
    s.emit = Rd.bind(null, s),
    t.ce && t.ce(s),
    s
}
let Oe = null;
const qd = () => Oe || Me;
let jn, Ls;
{
    const t = sn()
      , e = (r, n) => {
        let i;
        return (i = t[r]) || (i = t[r] = []),
        i.push(n),
        s => {
            i.length > 1 ? i.forEach(o => o(s)) : i[0](s)
        }
    }
    ;
    jn = e("__VUE_INSTANCE_SETTERS__", r => Oe = r),
    Ls = e("__VUE_SSR_SETTERS__", r => en = r)
}
const an = t => {
    const e = Oe;
    return jn(t),
    t.scope.on(),
    () => {
        t.scope.off(),
        jn(e)
    }
}
  , Qo = () => {
    Oe && Oe.scope.off(),
    jn(null)
}
;
function Du(t) {
    return t.vnode.shapeFlag & 4
}
let en = !1;
function Vd(t, e=!1, r=!1) {
    e && Ls(e);
    const {props: n, children: i} = t.vnode
      , s = Du(t);
    pd(t, n, s, e),
    yd(t, i, r || e);
    const o = s ? Kd(t, e) : void 0;
    return e && Ls(!1),
    o
}
function Kd(t, e) {
    const r = t.type;
    t.accessCache = Object.create(null),
    t.proxy = new Proxy(t.ctx,id);
    const {setup: n} = r;
    if (n) {
        St();
        const i = t.setupContext = n.length > 1 ? zd(t) : null
          , s = an(t)
          , o = on(n, t, 0, [t.props, i])
          , a = Al(o);
        if (Ct(),
        s(),
        (a || t.sp) && !dr(t) && gu(t),
        a) {
            if (o.then(Qo, Qo),
            e)
                return o.then(l => {
                    Xo(t, l, e)
                }
                ).catch(l => {
                    Zn(l, t, 0)
                }
                );
            t.asyncDep = o
        } else
            Xo(t, o, e)
    } else
        Fu(t, e)
}
function Xo(t, e, r) {
    ie(e) ? t.type.__ssrInlineRender ? t.ssrRender = e : t.render = e : we(e) && (t.setupState = Xl(e)),
    Fu(t, r)
}
let Zo;
function Fu(t, e, r) {
    const n = t.type;
    if (!t.render) {
        if (!e && Zo && !n.render) {
            const i = n.template || ao(t).template;
            if (i) {
                const {isCustomElement: s, compilerOptions: o} = t.appContext.config
                  , {delimiters: a, compilerOptions: l} = n
                  , u = Te(Te({
                    isCustomElement: s,
                    delimiters: a
                }, o), l);
                n.render = Zo(i, u)
            }
        }
        t.render = n.render || st
    }
    {
        const i = an(t);
        St();
        try {
            sd(t)
        } finally {
            Ct(),
            i()
        }
    }
}
const Wd = {
    get(t, e) {
        return ke(t, "get", ""),
        t[e]
    }
};
function zd(t) {
    const e = r => {
        t.exposed = r || {}
    }
    ;
    return {
        attrs: new Proxy(t.attrs,Wd),
        slots: t.slots,
        emit: t.emit,
        expose: e
    }
}
function ii(t) {
    return t.exposed ? t.exposeProxy || (t.exposeProxy = new Proxy(Xl(io(t.exposed)),{
        get(e, r) {
            if (r in e)
                return e[r];
            if (r in Fr)
                return Fr[r](t)
        },
        has(e, r) {
            return r in e || r in Fr
        }
    })) : t.proxy
}
function Gd(t, e=!0) {
    return ie(t) ? t.displayName || t.name : t.name || e && t.__name
}
function Yd(t) {
    return ie(t) && "__vccOpts"in t
}
const at = (t, e) => Of(t, e, en);
function Jd(t, e, r) {
    const n = arguments.length;
    return n === 2 ? we(e) && !te(e) ? Zr(e) ? Ee(t, null, [e]) : Ee(t, e) : Ee(t, null, e) : (n > 3 ? r = Array.prototype.slice.call(arguments, 2) : n === 3 && Zr(r) && (r = [r]),
    Ee(t, e, r))
}
const Qd = "3.5.17";
/**
* @vue/runtime-dom v3.5.17
* (c) 2018-present Yuxi (Evan) You and Vue contributors
* @license MIT
**/
let Ns;
const ea = typeof window < "u" && window.trustedTypes;
if (ea)
    try {
        Ns = ea.createPolicy("vue", {
            createHTML: t => t
        })
    } catch {}
const Uu = Ns ? t => Ns.createHTML(t) : t => t
  , Xd = "http://www.w3.org/2000/svg"
  , Zd = "http://www.w3.org/1998/Math/MathML"
  , yt = typeof document < "u" ? document : null
  , ta = yt && yt.createElement("template")
  , eh = {
    insert: (t, e, r) => {
        e.insertBefore(t, r || null)
    }
    ,
    remove: t => {
        const e = t.parentNode;
        e && e.removeChild(t)
    }
    ,
    createElement: (t, e, r, n) => {
        const i = e === "svg" ? yt.createElementNS(Xd, t) : e === "mathml" ? yt.createElementNS(Zd, t) : r ? yt.createElement(t, {
            is: r
        }) : yt.createElement(t);
        return t === "select" && n && n.multiple != null && i.setAttribute("multiple", n.multiple),
        i
    }
    ,
    createText: t => yt.createTextNode(t),
    createComment: t => yt.createComment(t),
    setText: (t, e) => {
        t.nodeValue = e
    }
    ,
    setElementText: (t, e) => {
        t.textContent = e
    }
    ,
    parentNode: t => t.parentNode,
    nextSibling: t => t.nextSibling,
    querySelector: t => yt.querySelector(t),
    setScopeId(t, e) {
        t.setAttribute(e, "")
    },
    insertStaticContent(t, e, r, n, i, s) {
        const o = r ? r.previousSibling : e.lastChild;
        if (i && (i === s || i.nextSibling))
            for (; e.insertBefore(i.cloneNode(!0), r),
            !(i === s || !(i = i.nextSibling)); )
                ;
        else {
            ta.innerHTML = Uu(n === "svg" ? `<svg>${t}</svg>` : n === "mathml" ? `<math>${t}</math>` : t);
            const a = ta.content;
            if (n === "svg" || n === "mathml") {
                const l = a.firstChild;
                for (; l.firstChild; )
                    a.appendChild(l.firstChild);
                a.removeChild(l)
            }
            e.insertBefore(a, r)
        }
        return [o ? o.nextSibling : e.firstChild, r ? r.previousSibling : e.lastChild]
    }
}
  , At = "transition"
  , Sr = "animation"
  , tn = Symbol("_vtc")
  , ju = {
    name: String,
    type: String,
    css: {
        type: Boolean,
        default: !0
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
    leaveToClass: String
}
  , th = Te({}, cu, ju)
  , rh = t => (t.displayName = "Transition",
t.props = th,
t)
  , ks = rh( (t, {slots: e}) => Jd(Hf, nh(t), e))
  , Ht = (t, e=[]) => {
    te(t) ? t.forEach(r => r(...e)) : t && t(...e)
}
  , ra = t => t ? te(t) ? t.some(e => e.length > 1) : t.length > 1 : !1;
function nh(t) {
    const e = {};
    for (const A in t)
        A in ju || (e[A] = t[A]);
    if (t.css === !1)
        return e;
    const {name: r="v", type: n, duration: i, enterFromClass: s=`${r}-enter-from`, enterActiveClass: o=`${r}-enter-active`, enterToClass: a=`${r}-enter-to`, appearFromClass: l=s, appearActiveClass: u=o, appearToClass: c=a, leaveFromClass: g=`${r}-leave-from`, leaveActiveClass: d=`${r}-leave-active`, leaveToClass: m=`${r}-leave-to`} = t
      , v = ih(i)
      , p = v && v[0]
      , f = v && v[1]
      , {onBeforeEnter: b, onEnter: _, onEnterCancelled: C, onLeave: E, onLeaveCancelled: R, onBeforeAppear: T=b, onAppear: I=_, onAppearCancelled: h=C} = e
      , y = (A, k, V, $) => {
        A._enterCancelled = $,
        qt(A, k ? c : a),
        qt(A, k ? u : o),
        V && V()
    }
      , w = (A, k) => {
        A._isLeaving = !1,
        qt(A, g),
        qt(A, m),
        qt(A, d),
        k && k()
    }
      , O = A => (k, V) => {
        const $ = A ? I : _
          , D = () => y(k, A, V);
        Ht($, [k, D]),
        na( () => {
            qt(k, A ? l : s),
            mt(k, A ? c : a),
            ra($) || ia(k, n, p, D)
        }
        )
    }
    ;
    return Te(e, {
        onBeforeEnter(A) {
            Ht(b, [A]),
            mt(A, s),
            mt(A, o)
        },
        onBeforeAppear(A) {
            Ht(T, [A]),
            mt(A, l),
            mt(A, u)
        },
        onEnter: O(!1),
        onAppear: O(!0),
        onLeave(A, k) {
            A._isLeaving = !0;
            const V = () => w(A, k);
            mt(A, g),
            A._enterCancelled ? (mt(A, d),
            aa()) : (aa(),
            mt(A, d)),
            na( () => {
                !A._isLeaving || (qt(A, g),
                mt(A, m),
                ra(E) || ia(A, n, f, V))
            }
            ),
            Ht(E, [A, V])
        },
        onEnterCancelled(A) {
            y(A, !1, void 0, !0),
            Ht(C, [A])
        },
        onAppearCancelled(A) {
            y(A, !0, void 0, !0),
            Ht(h, [A])
        },
        onLeaveCancelled(A) {
            w(A),
            Ht(R, [A])
        }
    })
}
function ih(t) {
    if (t == null)
        return null;
    if (we(t))
        return [Ti(t.enter), Ti(t.leave)];
    {
        const e = Ti(t);
        return [e, e]
    }
}
function Ti(t) {
    return Jc(t)
}
function mt(t, e) {
    e.split(/\s+/).forEach(r => r && t.classList.add(r)),
    (t[tn] || (t[tn] = new Set)).add(e)
}
function qt(t, e) {
    e.split(/\s+/).forEach(n => n && t.classList.remove(n));
    const r = t[tn];
    r && (r.delete(e),
    r.size || (t[tn] = void 0))
}
function na(t) {
    requestAnimationFrame( () => {
        requestAnimationFrame(t)
    }
    )
}
let sh = 0;
function ia(t, e, r, n) {
    const i = t._endId = ++sh
      , s = () => {
        i === t._endId && n()
    }
    ;
    if (r != null)
        return setTimeout(s, r);
    const {type: o, timeout: a, propCount: l} = oh(t, e);
    if (!o)
        return n();
    const u = o + "end";
    let c = 0;
    const g = () => {
        t.removeEventListener(u, d),
        s()
    }
      , d = m => {
        m.target === t && ++c >= l && g()
    }
    ;
    setTimeout( () => {
        c < l && g()
    }
    , a + 1),
    t.addEventListener(u, d)
}
function oh(t, e) {
    const r = window.getComputedStyle(t)
      , n = v => (r[v] || "").split(", ")
      , i = n(`${At}Delay`)
      , s = n(`${At}Duration`)
      , o = sa(i, s)
      , a = n(`${Sr}Delay`)
      , l = n(`${Sr}Duration`)
      , u = sa(a, l);
    let c = null
      , g = 0
      , d = 0;
    e === At ? o > 0 && (c = At,
    g = o,
    d = s.length) : e === Sr ? u > 0 && (c = Sr,
    g = u,
    d = l.length) : (g = Math.max(o, u),
    c = g > 0 ? o > u ? At : Sr : null,
    d = c ? c === At ? s.length : l.length : 0);
    const m = c === At && /\b(transform|all)(,|$)/.test(n(`${At}Property`).toString());
    return {
        type: c,
        timeout: g,
        propCount: d,
        hasTransform: m
    }
}
function sa(t, e) {
    for (; t.length < e.length; )
        t = t.concat(t);
    return Math.max(...e.map( (r, n) => oa(r) + oa(t[n])))
}
function oa(t) {
    return t === "auto" ? 0 : Number(t.slice(0, -1).replace(",", ".")) * 1e3
}
function aa() {
    return document.body.offsetHeight
}
function ah(t, e, r) {
    const n = t[tn];
    n && (e = (e ? [e, ...n] : [...n]).join(" ")),
    e == null ? t.removeAttribute("class") : r ? t.setAttribute("class", e) : t.className = e
}
const $n = Symbol("_vod")
  , $u = Symbol("_vsh")
  , Gv = {
    beforeMount(t, {value: e}, {transition: r}) {
        t[$n] = t.style.display === "none" ? "" : t.style.display,
        r && e ? r.beforeEnter(t) : Cr(t, e)
    },
    mounted(t, {value: e}, {transition: r}) {
        r && e && r.enter(t)
    },
    updated(t, {value: e, oldValue: r}, {transition: n}) {
        !e != !r && (n ? e ? (n.beforeEnter(t),
        Cr(t, !0),
        n.enter(t)) : n.leave(t, () => {
            Cr(t, !1)
        }
        ) : Cr(t, e))
    },
    beforeUnmount(t, {value: e}) {
        Cr(t, e)
    }
};
function Cr(t, e) {
    t.style.display = e ? t[$n] : "none",
    t[$u] = !e
}
const lh = Symbol("")
  , uh = /(^|;)\s*display\s*:/;
function ch(t, e, r) {
    const n = t.style
      , i = Se(r);
    let s = !1;
    if (r && !i) {
        if (e)
            if (Se(e))
                for (const o of e.split(";")) {
                    const a = o.slice(0, o.indexOf(":")).trim();
                    r[a] == null && An(n, a, "")
                }
            else
                for (const o in e)
                    r[o] == null && An(n, o, "");
        for (const o in r)
            o === "display" && (s = !0),
            An(n, o, r[o])
    } else if (i) {
        if (e !== r) {
            const o = n[lh];
            o && (r += ";" + o),
            n.cssText = r,
            s = uh.test(r)
        }
    } else
        e && t.removeAttribute("style");
    $n in t && (t[$n] = s ? n.display : "",
    t[$u] && (n.display = "none"))
}
const la = /\s*!important$/;
function An(t, e, r) {
    if (te(r))
        r.forEach(n => An(t, e, n));
    else if (r == null && (r = ""),
    e.startsWith("--"))
        t.setProperty(e, r);
    else {
        const n = fh(t, e);
        la.test(r) ? t.setProperty(rr(n), r.replace(la, ""), "important") : t[n] = r
    }
}
const ua = ["Webkit", "Moz", "ms"]
  , Ai = {};
function fh(t, e) {
    const r = Ai[e];
    if (r)
        return r;
    let n = et(e);
    if (n !== "filter" && n in t)
        return Ai[e] = n;
    n = Jn(n);
    for (let i = 0; i < ua.length; i++) {
        const s = ua[i] + n;
        if (s in t)
            return Ai[e] = s
    }
    return e
}
const ca = "http://www.w3.org/1999/xlink";
function fa(t, e, r, n, i, s=rf(e)) {
    n && e.startsWith("xlink:") ? r == null ? t.removeAttributeNS(ca, e.slice(6, e.length)) : t.setAttributeNS(ca, e, r) : r == null || s && !Pl(r) ? t.removeAttribute(e) : t.setAttribute(e, s ? "" : It(r) ? String(r) : r)
}
function da(t, e, r, n, i) {
    if (e === "innerHTML" || e === "textContent") {
        r != null && (t[e] = e === "innerHTML" ? Uu(r) : r);
        return
    }
    const s = t.tagName;
    if (e === "value" && s !== "PROGRESS" && !s.includes("-")) {
        const a = s === "OPTION" ? t.getAttribute("value") || "" : t.value
          , l = r == null ? t.type === "checkbox" ? "on" : "" : String(r);
        (a !== l || !("_value"in t)) && (t.value = l),
        r == null && t.removeAttribute(e),
        t._value = r;
        return
    }
    let o = !1;
    if (r === "" || r == null) {
        const a = typeof t[e];
        a === "boolean" ? r = Pl(r) : r == null && a === "string" ? (r = "",
        o = !0) : a === "number" && (r = 0,
        o = !0)
    }
    try {
        t[e] = r
    } catch {}
    o && t.removeAttribute(i || e)
}
function dh(t, e, r, n) {
    t.addEventListener(e, r, n)
}
function hh(t, e, r, n) {
    t.removeEventListener(e, r, n)
}
const ha = Symbol("_vei");
function ph(t, e, r, n, i=null) {
    const s = t[ha] || (t[ha] = {})
      , o = s[e];
    if (n && o)
        o.value = n;
    else {
        const [a,l] = gh(e);
        if (n) {
            const u = s[e] = yh(n, i);
            dh(t, a, u, l)
        } else
            o && (hh(t, a, o, l),
            s[e] = void 0)
    }
}
const pa = /(?:Once|Passive|Capture)$/;
function gh(t) {
    let e;
    if (pa.test(t)) {
        e = {};
        let n;
        for (; n = t.match(pa); )
            t = t.slice(0, t.length - n[0].length),
            e[n[0].toLowerCase()] = !0
    }
    return [t[2] === ":" ? t.slice(3) : rr(t.slice(2)), e]
}
let Ri = 0;
const mh = Promise.resolve()
  , vh = () => Ri || (mh.then( () => Ri = 0),
Ri = Date.now());
function yh(t, e) {
    const r = n => {
        if (!n._vts)
            n._vts = Date.now();
        else if (n._vts <= r.attached)
            return;
        lt(bh(n, r.value), e, 5, [n])
    }
    ;
    return r.value = t,
    r.attached = vh(),
    r
}
function bh(t, e) {
    if (te(e)) {
        const r = t.stopImmediatePropagation;
        return t.stopImmediatePropagation = () => {
            r.call(t),
            t._stopped = !0
        }
        ,
        e.map(n => i => !i._stopped && n && n(i))
    } else
        return e
}
const ga = t => t.charCodeAt(0) === 111 && t.charCodeAt(1) === 110 && t.charCodeAt(2) > 96 && t.charCodeAt(2) < 123
  , wh = (t, e, r, n, i, s) => {
    const o = i === "svg";
    e === "class" ? ah(t, n, o) : e === "style" ? ch(t, r, n) : zn(e) ? Gs(e) || ph(t, e, r, n, s) : (e[0] === "." ? (e = e.slice(1),
    !0) : e[0] === "^" ? (e = e.slice(1),
    !1) : _h(t, e, n, o)) ? (da(t, e, n),
    !t.tagName.includes("-") && (e === "value" || e === "checked" || e === "selected") && fa(t, e, n, o, s, e !== "value")) : t._isVueCE && (/[A-Z]/.test(e) || !Se(n)) ? da(t, et(e), n, s, e) : (e === "true-value" ? t._trueValue = n : e === "false-value" && (t._falseValue = n),
    fa(t, e, n, o))
}
;
function _h(t, e, r, n) {
    if (n)
        return !!(e === "innerHTML" || e === "textContent" || e in t && ga(e) && ie(r));
    if (e === "spellcheck" || e === "draggable" || e === "translate" || e === "autocorrect" || e === "form" || e === "list" && t.tagName === "INPUT" || e === "type" && t.tagName === "TEXTAREA")
        return !1;
    if (e === "width" || e === "height") {
        const i = t.tagName;
        if (i === "IMG" || i === "VIDEO" || i === "CANVAS" || i === "SOURCE")
            return !1
    }
    return ga(e) && Se(r) ? !1 : e in t
}
const Eh = ["ctrl", "shift", "alt", "meta"]
  , xh = {
    stop: t => t.stopPropagation(),
    prevent: t => t.preventDefault(),
    self: t => t.target !== t.currentTarget,
    ctrl: t => !t.ctrlKey,
    shift: t => !t.shiftKey,
    alt: t => !t.altKey,
    meta: t => !t.metaKey,
    left: t => "button"in t && t.button !== 0,
    middle: t => "button"in t && t.button !== 1,
    right: t => "button"in t && t.button !== 2,
    exact: (t, e) => Eh.some(r => t[`${r}Key`] && !e.includes(r))
}
  , Sh = (t, e) => {
    const r = t._withMods || (t._withMods = {})
      , n = e.join(".");
    return r[n] || (r[n] = (i, ...s) => {
        for (let o = 0; o < e.length; o++) {
            const a = xh[e[o]];
            if (a && a(i, e))
                return
        }
        return t(i, ...s)
    }
    )
}
  , Ch = Te({
    patchProp: wh
}, eh);
let ma;
function Ih() {
    return ma || (ma = _d(Ch))
}
const Yv = (...t) => {
    const e = Ih().createApp(...t)
      , {mount: r} = e;
    return e.mount = n => {
        const i = Ah(n);
        if (!i)
            return;
        const s = e._component;
        !ie(s) && !s.render && !s.template && (s.template = i.innerHTML),
        i.nodeType === 1 && (i.textContent = "");
        const o = r(i, !1, Th(i));
        return i instanceof Element && (i.removeAttribute("v-cloak"),
        i.setAttribute("data-v-app", "")),
        o
    }
    ,
    e
}
;
function Th(t) {
    if (t instanceof SVGElement)
        return "svg";
    if (typeof MathMLElement == "function" && t instanceof MathMLElement)
        return "mathml"
}
function Ah(t) {
    return Se(t) ? document.querySelector(t) : t
}
const tt = (t, e) => {
    const r = t.__vccOpts || t;
    for (const [n,i] of e)
        r[n] = i;
    return r
}
;
var Rh = !1;
/*!
 * pinia v2.3.1
 * (c) 2025 Eduardo San Martin Morote
 * @license MIT
 */
let Hu;
const si = t => Hu = t
  , qu = Symbol();
function Bs(t) {
    return t && typeof t == "object" && Object.prototype.toString.call(t) === "[object Object]" && typeof t.toJSON != "function"
}
var $r;
(function(t) {
    t.direct = "direct",
    t.patchObject = "patch object",
    t.patchFunction = "patch function"
}
)($r || ($r = {}));
function Jv() {
    const t = kl(!0)
      , e = t.run( () => Ie({}));
    let r = []
      , n = [];
    const i = io({
        install(s) {
            si(i),
            i._a = s,
            s.provide(qu, i),
            s.config.globalProperties.$pinia = i,
            n.forEach(o => r.push(o)),
            n = []
        },
        use(s) {
            return !this._a && !Rh ? n.push(s) : r.push(s),
            this
        },
        _p: r,
        _a: null,
        _e: t,
        _s: new Map,
        state: e
    });
    return i
}
const Vu = () => {}
;
function va(t, e, r, n=Vu) {
    t.push(e);
    const i = () => {
        const s = t.indexOf(e);
        s > -1 && (t.splice(s, 1),
        n())
    }
    ;
    return !r && Bl() && nf(i),
    i
}
function or(t, ...e) {
    t.slice().forEach(r => {
        r(...e)
    }
    )
}
const Mh = t => t()
  , ya = Symbol()
  , Mi = Symbol();
function Ds(t, e) {
    t instanceof Map && e instanceof Map ? e.forEach( (r, n) => t.set(n, r)) : t instanceof Set && e instanceof Set && e.forEach(t.add, t);
    for (const r in e) {
        if (!e.hasOwnProperty(r))
            continue;
        const n = e[r]
          , i = t[r];
        Bs(i) && Bs(n) && t.hasOwnProperty(r) && !xe(n) && !xt(n) ? t[r] = Ds(i, n) : t[r] = n
    }
    return t
}
const Ph = Symbol();
function Oh(t) {
    return !Bs(t) || !t.hasOwnProperty(Ph)
}
const {assign: Rt} = Object;
function Lh(t) {
    return !!(xe(t) && t.effect)
}
function Nh(t, e, r, n) {
    const {state: i, actions: s, getters: o} = e
      , a = r.state.value[t];
    let l;
    function u() {
        a || (r.state.value[t] = i ? i() : {});
        const c = Tf(r.state.value[t]);
        return Rt(c, s, Object.keys(o || {}).reduce( (g, d) => (g[d] = io(at( () => {
            si(r);
            const m = r._s.get(t);
            return o[d].call(m, m)
        }
        )),
        g), {}))
    }
    return l = Ku(t, u, e, r, n, !0),
    l
}
function Ku(t, e, r={}, n, i, s) {
    let o;
    const a = Rt({
        actions: {}
    }, r)
      , l = {
        deep: !0
    };
    let u, c, g = [], d = [], m;
    const v = n.state.value[t];
    !s && !v && (n.state.value[t] = {}),
    Ie({});
    let p;
    function f(h) {
        let y;
        u = c = !1,
        typeof h == "function" ? (h(n.state.value[t]),
        y = {
            type: $r.patchFunction,
            storeId: t,
            events: m
        }) : (Ds(n.state.value[t], h),
        y = {
            type: $r.patchObject,
            payload: h,
            storeId: t,
            events: m
        });
        const w = p = Symbol();
        tu().then( () => {
            p === w && (u = !0)
        }
        ),
        c = !0,
        or(g, y, n.state.value[t])
    }
    const b = s ? function() {
        const {state: y} = r
          , w = y ? y() : {};
        this.$patch(O => {
            Rt(O, w)
        }
        )
    }
    : Vu;
    function _() {
        o.stop(),
        g = [],
        d = [],
        n._s.delete(t)
    }
    const C = (h, y="") => {
        if (ya in h)
            return h[Mi] = y,
            h;
        const w = function() {
            si(n);
            const O = Array.from(arguments)
              , A = []
              , k = [];
            function V(Y) {
                A.push(Y)
            }
            function $(Y) {
                k.push(Y)
            }
            or(d, {
                args: O,
                name: w[Mi],
                store: R,
                after: V,
                onError: $
            });
            let D;
            try {
                D = h.apply(this && this.$id === t ? this : R, O)
            } catch (Y) {
                throw or(k, Y),
                Y
            }
            return D instanceof Promise ? D.then(Y => (or(A, Y),
            Y)).catch(Y => (or(k, Y),
            Promise.reject(Y))) : (or(A, D),
            D)
        };
        return w[ya] = !0,
        w[Mi] = y,
        w
    }
      , E = {
        _p: n,
        $id: t,
        $onAction: va.bind(null, d),
        $patch: f,
        $reset: b,
        $subscribe(h, y={}) {
            const w = va(g, h, y.detached, () => O())
              , O = o.run( () => In( () => n.state.value[t], A => {
                (y.flush === "sync" ? c : u) && h({
                    storeId: t,
                    type: $r.direct,
                    events: m
                }, A)
            }
            , Rt({}, l, y)));
            return w
        },
        $dispose: _
    }
      , R = Xn(E);
    n._s.set(t, R);
    const I = (n._a && n._a.runWithContext || Mh)( () => n._e.run( () => (o = kl()).run( () => e({
        action: C
    }))));
    for (const h in I) {
        const y = I[h];
        if (xe(y) && !Lh(y) || xt(y))
            s || (v && Oh(y) && (xe(y) ? y.value = v[h] : Ds(y, v[h])),
            n.state.value[t][h] = y);
        else if (typeof y == "function") {
            const w = C(y, h);
            I[h] = w,
            a.actions[h] = y
        }
    }
    return Rt(R, I),
    Rt(le(R), I),
    Object.defineProperty(R, "$state", {
        get: () => n.state.value[t],
        set: h => {
            f(y => {
                Rt(y, h)
            }
            )
        }
    }),
    n._p.forEach(h => {
        Rt(R, o.run( () => h({
            store: R,
            app: n._a,
            pinia: n,
            options: a
        })))
    }
    ),
    v && s && r.hydrate && r.hydrate(R.$state, v),
    u = !0,
    c = !0,
    R
}
/*! #__NO_SIDE_EFFECTS__ */
function kh(t, e, r) {
    let n, i;
    const s = typeof e == "function";
    typeof t == "string" ? (n = t,
    i = s ? r : e) : (i = t,
    n = t.id);
    function o(a, l) {
        const u = hd();
        return a = a || (u ? Ur(qu, null) : null),
        a && si(a),
        a = Hu,
        a._s.has(n) || (s ? Ku(n, e, i, a) : Nh(n, i, a)),
        a._s.get(n)
    }
    return o.$id = n,
    o
}
function Wu(t) {
    {
        const e = le(t)
          , r = {};
        for (const n in e) {
            const i = e[n];
            i.effect ? r[n] = at({
                get: () => t[n],
                set(s) {
                    t[n] = s
                }
            }) : (xe(i) || xt(i)) && (r[n] = Mf(t, n))
        }
        return r
    }
}
if (typeof window < "u") {
    let t = function() {
        var e = document.body
          , r = document.getElementById("__svg__icons__dom__1776825669894__");
        r || (r = document.createElementNS("http://www.w3.org/2000/svg", "svg"),
        r.style.position = "absolute",
        r.style.width = "0",
        r.style.height = "0",
        r.id = "__svg__icons__dom__1776825669894__",
        r.setAttribute("xmlns", "http://www.w3.org/2000/svg"),
        r.setAttribute("xmlns:link", "http://www.w3.org/1999/xlink")),
        r.innerHTML = '<symbol  viewBox="0 0 52 46" id="icon-channels"><defs><filter color-interpolation-filters="auto" id="icon-channels_a"><feColorMatrix in="SourceGraphic" values="0 0 0 0 0.980392 0 0 0 0 0.615686 0 0 0 0 0.231373 0 0 0 1.000000 0"></feColorMatrix></filter></defs><g transform="translate(-6 -9)" filter="url(#icon-channels_a)" fill="none" fill-rule="evenodd"><path d="M56.45 13.362c.714 1.792.966 4.48.86 7.643l-.074 1.491a67.203 67.203 0 0 1-.761 6.455l-.295 1.69a89.787 89.787 0 0 1-1.314 5.946l-.447 1.67c-.076.277-.154.552-.233.825l-.487 1.623c-2.412 7.743-5.67 13.962-8.634 13.962-2.219 0-4.341-1.613-6.943-4.958-1.044-1.341-2.148-2.955-3.31-4.826a97.375 97.375 0 0 1-1.987-3.36L32 40.04l-.017.033c-.465.85-.932 1.678-1.398 2.481l-.7 1.185-.698 1.144c-1.161 1.871-2.265 3.485-3.309 4.826-2.602 3.345-4.724 4.958-6.943 4.958-2.965 0-6.222-6.219-8.634-13.962l-.487-1.623a84.95 84.95 0 0 1-.233-.824l-.447-1.67a90.102 90.102 0 0 1-1.152-5.097l-.314-1.697a82.33 82.33 0 0 1-.143-.843l-.255-1.668a63.628 63.628 0 0 1-.506-4.787l-.074-1.491c-.106-3.163.146-5.851.86-7.643 1.395-3.506 4.236-4.849 7.482-3.532 2.112.858 4.411 2.826 7.065 5.954 1.475 1.739 3.04 3.818 4.686 6.218a137.187 137.187 0 0 1 4.132 6.423L32 30.239l.056-.094c.678-1.15 1.361-2.275 2.045-3.372l1.028-1.623 1.027-1.574 1.061-1.574c1.647-2.4 3.211-4.48 4.686-6.218 2.654-3.128 4.953-5.096 7.065-5.954 3.246-1.317 6.087.026 7.482 3.532Zm-44.33 1.242-.206.489c-.384.962-.56 2.55-.54 4.582l.035 1.271c.01.22.022.445.036.673l.105 1.416c.065.73.146 1.491.245 2.28l.22 1.613.265 1.677c.144.853.305 1.726.481 2.616l.183.894.377 1.731.417 1.758.342 1.333.365 1.326.384 1.308.397 1.278.403 1.24.405 1.188.401 1.126.581 1.551.366.926.503 1.197.43.937.239.462c.18.326.311.504.38.504.272 0 1.187-.75 2.295-2.008l.62-.735c.106-.13.213-.264.32-.402.936-1.203 1.95-2.687 3.026-4.42a92.84 92.84 0 0 0 1.893-3.199l.633-1.132.99-1.837.63-1.22-.648-1.141-.795-1.362a138.18 138.18 0 0 0-4.99-7.874c-1.557-2.27-3.03-4.227-4.395-5.838-2.132-2.512-3.965-4.118-5.25-4.64-.627-.254-.828-.244-1.143.432Zm38.617-.432c-1.285.522-3.118 2.128-5.25 4.64-1.366 1.61-2.838 3.568-4.395 5.838a128.13 128.13 0 0 0-3.217 4.941l-.806 1.315-1.37 2.306-1.046 1.817.815 1.557a102.33 102.33 0 0 0 3.337 5.829c1.076 1.733 2.09 3.217 3.026 4.42l.32.402.62.735.582.634c.837.871 1.49 1.374 1.712 1.374.056 0 .151-.114.28-.326l.215-.392.258-.529.297-.655.328-.772.355-.877.376-.97.39-1.054.401-1.127.405-1.187.403-1.239.397-1.278.383-1.306.366-1.325.174-.666.168-.666.416-1.758c.276-1.21.523-2.403.742-3.566l.307-1.72c.094-.565.182-1.121.262-1.667l.219-1.604c.033-.262.063-.52.092-.776l.152-1.495c.32-3.635.212-6.483-.365-7.932l-.206-.489c-.315-.676-.516-.686-1.143-.432Z" fill="#000" fill-rule="nonzero" /></g></symbol><symbol  viewBox="0 0 12 13" id="icon-delete"><path d="m2.516 2.267.541 9.098c.017.282.25.502.533.502h4.82c.283 0 .516-.22.533-.502l.541-9.098H2.516Zm7.77 0-.545 9.146a1.333 1.333 0 0 1-1.33 1.254H3.59c-.706 0-1.29-.55-1.331-1.254l-.545-9.146H.334V1.8c0-.184.149-.333.333-.333h10.666c.184 0 .334.149.334.333v.467h-1.381ZM5.133 4l.334 6h-.8l-.334-6h.8Zm2.534 0-.334 6h-.8l.334-6h.8Zm-.334-4c.184 0 .334.15.334.333V.8H4.333V.333c0-.184.15-.333.334-.333h2.666Z" fill="#000" fill-rule="evenodd" fill-opacity=".9" opacity=".55" /></symbol>',
        e.insertBefore(r, e.firstChild)
    };
    document.readyState === "loading" ? document.addEventListener("DOMContentLoaded", t) : t()
}
const Bh = nr({
    name: "SvgIcon",
    props: {
        prefix: {
            type: String,
            default: "icon"
        },
        name: {
            type: String,
            required: !0
        },
        color: {
            type: String,
            default: "#000"
        }
    },
    setup(t) {
        return {
            symbolId: at( () => `#${t.prefix}-${t.name}`)
        }
    }
});
const Dh = {
    "aria-hidden": "true",
    class: "svg-icon"
}
  , Fh = ["href", "fill"];
function Uh(t, e, r, n, i, s) {
    return ge(),
    _e("svg", Dh, [ue("use", {
        href: t.symbolId,
        fill: t.color
    }, null, 8, Fh)])
}
const Qv = tt(Bh, [["render", Uh], ["__scopeId", "data-v-75d8c7c3"]]);
function jh() {
    return /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent)
}
function $h() {
    return "ontouchstart"in window || navigator.maxTouchPoints > 0
}
const ho = () => {
    const t = Ie(!1)
      , e = Ie(!1)
      , r = Ie("")
      , n = Ie(null)
      , i = () => {
        t.value = jh(),
        e.value = $h(),
        window.innerWidth >= window.innerHeight ? r.value = "window-ratio-width" : r.value = "window-ratio-height"
    }
    ;
    return _r(async () => {
        i(),
        window.addEventListener("resize", i)
    }
    ),
    oo( () => {
        window.removeEventListener("resize", i)
    }
    ),
    {
        isMobile: Lt(t),
        isTouch: Lt(e),
        windowRatioClass: Lt(r),
        supportsH265: Lt(n)
    }
}
;
var Hh = Object.defineProperty
  , qh = Object.defineProperties
  , Vh = Object.getOwnPropertyDescriptors
  , ba = Object.getOwnPropertySymbols
  , Kh = Object.prototype.hasOwnProperty
  , Wh = Object.prototype.propertyIsEnumerable
  , wa = (t, e, r) => e in t ? Hh(t, e, {
    enumerable: !0,
    configurable: !0,
    writable: !0,
    value: r
}) : t[e] = r
  , zh = (t, e) => {
    for (var r in e || (e = {}))
        Kh.call(e, r) && wa(t, r, e[r]);
    if (ba)
        for (var r of ba(e))
            Wh.call(e, r) && wa(t, r, e[r]);
    return t
}
  , Gh = (t, e) => qh(t, Vh(e))
  , _a = class {
    constructor(t) {
        this.onCatch = null,
        t && this.bindOnCatch(t)
    }
    bindOnCatch(t) {
        this.onCatch && this.unbindOnCatch(),
        this.onCatch = t
    }
    unbindOnCatch() {
        this.onCatch = null
    }
    catch(t) {
        !this.onCatch || !t || this.onCatch(t)
    }
}
  , Yh = class {
}
  , zu = class {
}
  , Zt = class {
    constructor(t, e) {
        this.info = t,
        this.error = e
    }
}
  , po = (t => (t.Fatal = "fatal",
t.Danger = "danger",
t.Default = "default",
t.Minor = "minor",
t))(po || {})
  , ln = class extends Error {
    constructor(t, e={}) {
        super(t);
        const {cause: r, level: n="default", origin: i} = e;
        this.name = "KnownError",
        this.cause = r,
        this.level = n,
        this.origin = i,
        this.timestamp = Date.now(),
        this.stack = r == null ? void 0 : r.stack
    }
    toString() {
        var t, e, r;
        return ((e = this.stack) == null ? void 0 : e.startsWith((t = this.cause) == null ? void 0 : t.name)) && ((r = this.cause) == null ? void 0 : r.name) ? this.stack.replace(this.cause.name, this.name) : this.stack ? this.stack : `${this.name}: ${this.message}`
    }
    get merlin() {}
}
  , Gu = Object.prototype.toString;
function Pi(t) {
    switch (Gu.call(t)) {
    case "[object Error]":
    case "[object Exception]":
    case "[object DOMException]":
        return !0;
    default:
        return go(t, Error)
    }
}
function Yu(t, e) {
    return Gu.call(t) === `[object ${e}]`
}
function Jh(t) {
    return Yu(t, "ErrorEvent")
}
function Ea(t) {
    return Yu(t, "PromiseRejectionEvent")
}
function Oi(t) {
    return typeof Event < "u" && go(t, Event)
}
function Qh(t) {
    return typeof Element < "u" && go(t, Element)
}
function go(t, e) {
    try {
        return t instanceof e
    } catch {
        return !1
    }
}
var Hn = class extends ln {
    constructor(t, e, r={}) {
        super(t, r),
        this.url = e,
        this.name = "LoadError"
    }
    get merlin() {
        const t = {};
        return this.level === "fatal" || this.level === "danger" ? t.idx1 = this.url : t.extra = `url:${this.url}`,
        t
    }
}
  , Xh = class extends Hn {
    constructor(t, e, r={}) {
        super(t, e, r),
        this.name = "ImageLoadError";
        const {eleId: n=""} = r;
        this.eleId = n
    }
    get merlin() {
        return Gh(zh({}, super.merlin), {
            idx3: this.eleId
        })
    }
}
  , mo = (t => (t.UnknownError = "UnknownError",
t.EvalError = "EvalError",
t.RangeError = "RangeError",
t.ReferenceError = "ReferenceError",
t.SyntaxError = "SyntaxError",
t.TypeError = "TypeError",
t.URIError = "URIError",
t.IndexSizeError = "IndexSizeError",
t.HierarchyRequestError = "HierarchyRequestError",
t.WrongDocumentError = "WrongDocumentError",
t.InvalidCharacterError = "InvalidCharacterError",
t.NoModificationAllowedError = "NoModificationAllowedError",
t.NotFoundError = "NotFoundError",
t.NotSupportedError = "NotSupportedError",
t.InUseAttributeError = "InUseAttributeError",
t.InvalidStateError = "InvalidStateError",
t.InvalidModificationError = "InvalidModificationError",
t.NamespaceError = "NamespaceError",
t.InvalidAccessError = "InvalidAccessError",
t.SecurityError = "SecurityError",
t.NetworkError = "NetworkError",
t.AbortError = "AbortError",
t.URLMismatchError = "URLMismatchError",
t.QuotaExceededError = "QuotaExceededError",
t.TimeoutError = "TimeoutError",
t.InvalidNodeTypeError = "InvalidNodeTypeError",
t.DataCloneError = "DataCloneError",
t.EncodingError = "EncodingError",
t.NotReadableError = "NotReadableError",
t.ConstraintError = "ConstraintError",
t.DataError = "DataError",
t.TransactionInactiveError = "TransactionInactiveError",
t.ReadOnlyError = "ReadOnlyError",
t.VersionError = "VersionError",
t.OperationError = "OperationError",
t.NotAllowedError = "NotAllowedError",
t))(mo || {})
  , Zh = class extends ln {
    constructor(t, e={}) {
        super(t, e),
        this.name = "ProgramError";
        const {cause: r, level: n="default"} = e;
        this.type = (r == null ? void 0 : r.name) && r.name in mo ? r == null ? void 0 : r.name : "UnknownError",
        this.level = n
    }
    toString() {
        var t, e, r;
        return ((e = this.stack) == null ? void 0 : e.startsWith((t = this.cause) == null ? void 0 : t.name)) && ((r = this.cause) == null ? void 0 : r.name) ? this.stack.replace(this.cause.name, `${this.name}[${this.type}]`) : this.stack ? this.stack : `${this.name}[${this.type}]: ${this.message}`
    }
    get merlin() {
        return {
            idx1: this.type
        }
    }
}
  , ep = class extends ln {
    constructor(t, e, r={}) {
        super(t, r),
        this.url = e,
        this.name = "RequestError";
        const {req: n, resp: i, errCode: s} = r;
        this.req = n,
        this.resp = i,
        this.errCode = s
    }
    get merlin() {
        return {
            idx1: this.url,
            idx2: `${this.errCode !== void 0 ? this.errCode : ""}`
        }
    }
}
  , tp = class extends Hn {
    constructor(t, e, r, n={}) {
        super(t, e, n),
        this.type = r,
        this.name = "ScriptLoadError";
        const {level: i="danger"} = n;
        this.level = i
    }
}
  , xa = class extends zu {
    transform(t) {
        if (t instanceof ln)
            return null;
        if (t instanceof Ju)
            return this.transformOnError(t.info.event, t.info.source, t.info.lineno, t.info.colno, t.error);
        if (t instanceof Fs)
            return this.transformEvent(t.info.event);
        if (t instanceof Zt && t.error) {
            const {error: e, info: r} = t;
            let n;
            return typeof r == "object" && typeof r.level == "string" && (n = r.level),
            this.transformError(e, void 0, n)
        }
        return this.transformError(t)
    }
    transformError(t, e, r) {
        return Pi(t) ? t.name in mo ? new Zh(t.message,{
            cause: t,
            origin: e,
            level: r
        }) : t.name === "ResourceError" && t.liteUrl ? new Hn(`failed to load: ${t.liteUrl}`,t.liteUrl,{
            origin: e,
            level: r
        }) : null : null
    }
    transformErrorEvent(t) {
        return this.transformError(t.error, {
            file: t.filename,
            line: t.lineno,
            col: t.colno
        })
    }
    transformPromiseRejectionEvent(t) {
        var e;
        const r = t.reason || ((e = t.detail) == null ? void 0 : e.reason);
        return r ? Pi(r) ? this.transformError(r) : Oi(r) && t !== r && !Ea(r) ? this.transformEvent(r) : null : null
    }
    transformEvent(t) {
        var e, r;
        if (Ea(t))
            return this.transformPromiseRejectionEvent(t);
        if (Jh(t))
            return this.transformErrorEvent(t);
        if (Oi(t) && Qh(t.target)) {
            const n = t.target
              , i = n.getAttribute("src") || n.getAttribute("href") || ""
              , s = (e = n.tagName) == null ? void 0 : e.toUpperCase();
            if (s === "IMG" || s === "IMAGE") {
                let o = "";
                return (r = n.dataset) != null && r.pandora ? o += n.dataset.pandora : (n.id && (o += `#${n.id}`),
                n.className && (o += `.${n.className}`)),
                new Xh(`failed to load (${o}): ${i}`,i,{
                    eleId: o
                })
            }
            return s === "SCRIPT" || s === "LINK" ? new tp(`failed to load: ${i}`,i,n.getAttribute("type") || n.getAttribute("as") || "") : new Hn(`failed to load: ${i}`,i)
        }
        return null
    }
    transformOnError(t, e, r, n, i) {
        return Pi(i) ? this.transformError(i, {
            file: e,
            line: r,
            col: n
        }) : Oi(t) ? this.transformEvent(t) : null
    }
}
  , Ju = class extends Zt {
}
  , Fs = class extends Zt {
}
  , rp = class {
    constructor(t) {
        this.catchers = new Set,
        this.consumers = [],
        this.running = !1,
        this.thisCatch = this.catch.bind(this),
        this.transformers = t != null && t.disableBuiltinTransformers ? [] : [new xa]
    }
    run(t) {
        if (this.running)
            return this;
        const {plugins: e=[], catchers: r=[], transformers: n=[], consumers: i=[], disableBuiltinTransformers: s} = t || {}
          , o = []
          , a = s ? [] : [new xa]
          , l = [];
        return e.forEach(u => {
            o.push(...u.catchers),
            a.push(...u.transformers),
            l.push(...u.consumers)
        }
        ),
        o.push(...r),
        a.push(...n),
        l.push(...i),
        o.forEach(u => this.addCatcher(u)),
        this.setTransformers(a),
        this.setConsumers(l),
        this.running = !0,
        this.catchers.forEach(u => u.run()),
        this
    }
    addCatcher(t) {
        return t instanceof _a ? (this.catchers.add(t),
        t.bindOnCatch(this.thisCatch),
        this.running && t.run(),
        this) : this
    }
    removeCatcher(t) {
        return t instanceof _a ? (this.running && t.stop(),
        t.unbindOnCatch(),
        this.catchers.delete(t),
        this) : this
    }
    setCatchers(t) {
        return this.catchers.size && (this.catchers.forEach(e => e.unbindOnCatch()),
        this.catchers.clear()),
        t.forEach(e => this.addCatcher(e)),
        this
    }
    setTransformers(t) {
        return this.transformers = t.filter(e => e instanceof zu),
        this
    }
    setConsumers(t) {
        return this.consumers = t.filter(e => e instanceof Yh),
        this
    }
    transform(t) {
        let e = t;
        return this.transformers.forEach(r => {
            var n;
            e = (n = r.transform(e)) != null ? n : e
        }
        ),
        e
    }
    catch(t) {
        let e = this.transform(t);
        return e instanceof Zt && e.error && (e = e.error),
        this.consumers.forEach(r => {
            e = r.consume(e)
        }
        ),
        e
    }
    stop() {
        if (!!this.running)
            return this.catchers.forEach(t => t.stop()),
            this.running = !1,
            this
    }
}
  , Xv = new rp;
class Qu extends Error {
    constructor(e) {
        super(e),
        this.name = "MerlinError"
    }
}
class oi {
    constructor() {
        this.merlin = void 0
    }
    static isCollector(e) {
        return e && typeof e.init == "function" && typeof e.settle == "function" && typeof e.afterInit == "function"
    }
    init(e) {
        this.merlin = e;
        try {
            this.afterInit()
        } catch (r) {
            e.core.adapter.errorHandler(new Qu(`collector \u521D\u59CB\u5316\u5931\u8D25: ${(r == null ? void 0 : r.message) || ""}}`))
        }
    }
    settle() {}
}
const Sa = "__ml::aid"
  , vt = "__ml::page";
class ai {
    constructor(e) {
        this.sampleRate = 1,
        this.filter = void 0,
        this.buffer = [],
        this.frequencyLimitRecordCount = 0;
        const {filter: r, sampleRate: n, bufferSize: i=50, flushInterval: s=1e4, frequencyLimit: o={
            max: 0,
            perSeconds: 0
        }} = e || {};
        r && (this.filter = r),
        typeof n == "number" && (this.sampleRate = n <= 0 || n > 1 ? 1 : n),
        this.bufferSize = i,
        this.flushInterval = s,
        this.frequencyLimit = o
    }
    static isTransport(e) {
        return e && typeof e.init == "function" && typeof e.receiveFromCore == "function" && typeof e.flush == "function" && typeof e.send == "function"
    }
    initFlushInterval() {
        !!setInterval && !!clearInterval && this.flushInterval !== 0 && (this.flushIntervalTimer && (clearInterval(this.flushIntervalTimer),
        this.flushIntervalTimer = void 0),
        this.flushIntervalTimer = setInterval( () => {
            this.flush()
        }
        , this.flushInterval))
    }
    async init(e) {
        this.reporter = e,
        this.initFlushInterval(),
        !!setInterval && !!clearInterval && this.frequencyLimit.perSeconds !== 0 && (this.frequencyLimitTimer && (clearInterval(this.frequencyLimitTimer),
        this.frequencyLimitTimer = void 0),
        this.frequencyLimitTimer = setInterval( () => {
            this.frequencyLimitRecordCount = 0
        }
        , this.frequencyLimit.perSeconds * 1e3))
    }
    config(e) {
        typeof e.bufferSize == "number" && (this.bufferSize = e.bufferSize),
        typeof e.flushInterval == "number" && (this.flushInterval = e.flushInterval,
        this.initFlushInterval())
    }
    async receiveFromCore(e, r=!1) {
        let n = [...e].filter(i => this.reportTypes.includes(i.type));
        this.filter && (n = n.filter(this.filter)),
        this.sampleRate < 1 && (n = n.filter( () => Math.random() < this.sampleRate)),
        n.length && this.buffer.push(...n),
        (r || this.bufferSize === 0 || this.buffer.length >= this.bufferSize) && await this.flush()
    }
    async flush() {
        if (!this.buffer.length)
            return;
        const e = [...this.buffer];
        if (this.buffer.length = 0,
        this.frequencyLimit.max !== 0) {
            const r = this.frequencyLimit.max - this.frequencyLimitRecordCount;
            if (r <= 0)
                return;
            r < e.length && (e.length = r),
            this.frequencyLimitRecordCount += e.length
        }
        await this.send(e)
    }
}
let Ca;
function Xu() {
    return Ca || (Ca = typeof globalThis < "u" ? globalThis : typeof self < "u" ? self : typeof window < "u" ? window : typeof global < "u" ? global : {})
}
function np(t=location.href) {
    var s, o, a, l;
    const e = {}
      , r = t.includes("#") ? t.split("#")[0] : t;
    let n = r.includes("?") ? r.split("?")[1] : r;
    if (n.includes("="))
        for (const u of n.split("&")) {
            const [c,g] = u.split("=")
              , d = (s = c == null ? void 0 : c.replace(/\+/g, " ")) != null ? s : ""
              , m = (o = g == null ? void 0 : g.replace(/\+/g, " ")) != null ? o : "";
            try {
                e[d] = decodeURIComponent(m)
            } catch {
                e[d] = m
            }
        }
    const i = t.includes("#") ? t.split("#")[1] : "";
    if (n = i.includes("?") ? i.split("?")[1] : "",
    n.includes("="))
        for (const u of n.split("&")) {
            const [c,g] = u.split("=")
              , d = (a = c == null ? void 0 : c.replace(/\+/g, " ")) != null ? a : ""
              , m = (l = g == null ? void 0 : g.replace(/\+/g, " ")) != null ? l : "";
            try {
                e[d] = decodeURIComponent(m)
            } catch {
                e[d] = m
            }
        }
    return e
}
function ip(t) {
    return /^[A-Z]/.test(t) ? t : t.replace(/([A-Z])/g, "_$1").toLowerCase()
}
function sp(t) {
    return /^[A-Z]/.test(t) ? t : t.replace(/(_[a-z])/g, (e, r) => r.toUpperCase().slice(1))
}
function op(t, e=!0) {
    return Object.entries(t).filter( ([r,n]) => n != null).map( ([r,n]) => `${e ? ip(r) : r}=${n}`).join("&")
}
function ap(t) {
    return typeof t != "object" ? t : Object.fromEntries(Object.entries(t).map( ([e,r]) => [sp(e), r]))
}
function lp(t, e) {
    return Object.fromEntries(Object.entries(t).filter( ([r]) => !e.includes(r)))
}
function ht() {
    try {
        const t = crypto;
        if (t != null && t.randomUUID)
            return t.randomUUID();
        if ((t == null ? void 0 : t.getRandomValues) && Uint8Array)
            return "10000000-1000-4000-8000-100000000000".replace(/[018]/g, e => (Number(e) ^ t.getRandomValues(new Uint8Array(1))[0] & 15 >> Number(e) / 4).toString(16))
    } catch {}
    return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, t => {
        const e = Math.random() * 16 | 0;
        return (t === "x" ? e : e & 3 | 8).toString(16)
    }
    )
}
function gr(t, e, r, n) {
    return {
        filename: t,
        func: e,
        lineno: r,
        colno: n
    }
}
const er = "?"
  , up = /^\s*at (\S+?)(?::(\d+))(?::(\d+))\s*$/i
  , cp = /^\s*at (?:(.+?\)(?: \[.+\])?|.*?) ?\((?:address at )?)?(?:async )?((?:<anonymous>|[-a-z]+:|.*bundle|\/)?.*?)(?::(\d+))?(?::(\d+))?\)?\s*$/i
  , fp = /\((\S*)(?::(\d+))(?::(\d+))\)/
  , dp = t => {
    const e = up.exec(t);
    if (e) {
        const [,n,i,s] = e;
        return gr(n, er, +i, +s)
    }
    const r = cp.exec(t);
    if (r) {
        if (r[2] && r[2].indexOf("eval") === 0) {
            const o = fp.exec(r[2]);
            o && (r[2] = o[1],
            r[3] = o[2],
            r[4] = o[3])
        }
        const [i,s] = Zu(r[1] || er, r[2]);
        return gr(s, i, r[3] ? +r[3] : void 0, r[4] ? +r[4] : void 0)
    }
}
  , hp = /^\s*(.*?)(?:\((.*?)\))?(?:^|@)?((?:[-a-z]+)?:\/.*?|\[native code\]|[^@]*(?:bundle|\d+\.js)|\/[\w\-. /=]+)(?::(\d+))?(?::(\d+))?\s*$/i
  , pp = /(\S+) line (\d+)(?: > eval line \d+)* > eval/i
  , gp = t => {
    const e = hp.exec(t);
    if (e) {
        if (e[3] && e[3].indexOf(" > eval") > -1) {
            const s = pp.exec(e[3]);
            s && (e[1] = e[1] || "eval",
            [e[3],e[4]] = [s[1], s[2]],
            e[5] = "")
        }
        let n = e[3]
          , i = e[1] || er;
        return [i,n] = Zu(i, n),
        gr(n, i, e[4] ? +e[4] : void 0, e[5] ? +e[5] : void 0)
    }
}
  , mp = /^\s*at (?:((?:\[object object\])?.+) )?\(?((?:file|ms-appx|https?|webpack|blob):.*?):(\d+)(?::(\d+))?\)?\s*$/i
  , vp = t => {
    const e = mp.exec(t);
    return e ? gr(e[2], e[1] || er, +e[3], e[4] ? +e[4] : void 0) : void 0
}
  , yp = / line (\d+).*script (?:in )?(\S+)(?:: in function (\S+))?$/i
  , bp = t => {
    const e = yp.exec(t);
    return e ? gr(e[2], e[3] || er, +e[1]) : void 0
}
  , wp = / line (\d+), column (\d+)\s*(?:in (?:<anonymous function: ([^>]+)>|([^)]+))\(.*\))? in (.*):\s*$/i
  , _p = t => {
    const e = wp.exec(t);
    return e ? gr(e[5], e[3] || e[4] || er, +e[1], +e[2]) : void 0
}
  , Zu = (t, e) => {
    const r = t.indexOf("safari-extension") !== -1
      , n = t.indexOf("safari-web-extension") !== -1;
    return r || n ? [t.indexOf("@") !== -1 ? t.split("@")[0] : er, r ? `safari-extension:${e}` : `safari-web-extension:${e}`] : [t, e]
}
  , Ep = [bp, _p, dp, vp, gp];
function xp(...t) {
    return (e, r=0, n=0) => {
        const i = [];
        for (const s of e.split(`
`).slice(r))
            for (const o of t) {
                const a = o(s);
                if (a) {
                    if (i.push(a),
                    n > 0 && i.length >= n)
                        return i;
                    break
                }
            }
        return i
    }
}
const Sp = xp(...Ep);
function Cp(t) {
    for (const e of t)
        if (e.filename && e.lineno !== void 0 && e.colno !== void 0)
            return {
                file: e.filename,
                line: e.lineno,
                col: e.colno
            };
    return {}
}
function Ip(t) {
    return !!t.file && t.file !== "undefined" && t.line !== void 0
}
function Tp(t) {
    const e = t.stack ? Sp(t.stack) : [];
    let {file: r, line: n, col: i} = t;
    return Ip({
        file: r,
        line: n
    }) || ({file: r, col: i, line: n} = Cp(e)),
    {
        id: ht(),
        name: t.name || "Unknown",
        message: t.message || "",
        stack: t.stack,
        stackFrames: e,
        file: r,
        line: n,
        col: i,
        eIdx1: t.eIdx1,
        eIdx2: t.eIdx2,
        eIdx3: t.eIdx3,
        eExtra: t.eExtra,
        idx1: t.idx1,
        idx2: t.idx2,
        idx3: t.idx3,
        log: t.log,
        status: t.status,
        level: t.level || po.Default
    }
}
var We = (t => (t.CUSTOM = "custom",
t.PAGE_ENTER = "pageEnter",
t.PAGE_LEAVE = "pageLeave",
t.ELEMENT_EXPOSE = "elementExpose",
t.ELEMENT_CONCEAL = "elementConceal",
t.ELEMENT_CLICK = "elementClick",
t.ELEMENT_HOVER = "elementHover",
t.VIDEO_PLAY_START = "videoPlayStart",
t.VIDEO_PLAY_PAUSE = "videoPlayPause",
t.VIDEO_PLAY_WAITING = "videoPlayWaiting",
t.VIDEO_PLAY_SEEK = "videoPlaySeek",
t.VIDEO_PLAY_FINISH = "videoPlayFinish",
t))(We || {})
  , Rn = (t => (t.OK = "ok",
t.FAIL = "fail",
t.ABORT = "abort",
t))(Rn || {})
  , Ze = (t => (t.ERROR = "error",
t.BEHAVIOR = "behavior",
t.PERFORMANCE = "perf",
t))(Ze || {});
class vo {
    constructor() {
        this.ctime = Date.now(),
        this.context = {
            contextId: "",
            aid: "",
            env: {},
            page: {}
        }
    }
    updateContext(e) {
        this.context = e
    }
}
class Ap extends vo {
    constructor(e) {
        super(),
        this.data = e,
        this.type = "error"
    }
}
class Qe extends vo {
    constructor(e) {
        super(),
        this.data = e,
        this.type = "behavior"
    }
}
class Ia extends vo {
    constructor(e) {
        super(),
        this.data = e,
        this.type = "perf"
    }
    static parseStatus(e) {
        return e === "fail" ? Rn.FAIL : e === "abort" ? Rn.ABORT : Rn.OK
    }
}
class Rp {
    constructor(e, r, n, i) {
        this.reporter = e,
        this.type = r,
        this.action = n,
        this.info = i,
        this.startTime = Date.now(),
        this.settled = !1
    }
    resetStartTime(e) {
        return this.startTime = e || Date.now(),
        this
    }
    report(e="ok", r, n) {
        var a, l, u, c;
        if (this.settled)
            return;
        this.settled = !0;
        const i = n == null ? void 0 : n.sampleRate;
        if (i !== void 0 && (i <= 0 || i < 1 && Math.random() >= i))
            return;
        const s = Date.now()
          , o = s > this.startTime ? s - this.startTime : 0;
        this.reporter.reportPerf(this.type, this.action, {
            duration: o,
            status: e,
            idx1: (r == null ? void 0 : r.idx1) || ((a = this.info) == null ? void 0 : a.idx1),
            idx2: (r == null ? void 0 : r.idx2) || ((l = this.info) == null ? void 0 : l.idx2),
            idx3: (r == null ? void 0 : r.idx3) || ((u = this.info) == null ? void 0 : u.idx3),
            log: (r == null ? void 0 : r.log) || ((c = this.info) == null ? void 0 : c.log)
        })
    }
}
class qn {
    static parseChars(e) {
        var r;
        try {
            return typeof e == "string" ? e.replace(/,/g, ";") : !!e && typeof e == "object" ? ((r = JSON.stringify(e)) == null ? void 0 : r.replace(/,/g, ";")) || "" : (e == null ? void 0 : e.toString().replace(/,/g, ";")) || ""
        } catch {
            return ""
        }
    }
    static parseUint(e) {
        try {
            return typeof e == "number" ? Math.floor(e) : 0
        } catch {
            return 0
        }
    }
    static parseValue(e, r) {
        return r === "chars" || r === "char[1024]" ? this.parseChars(e) : this.parseUint(e)
    }
    constructor(e) {
        this.fields = e;
        const r = new Map;
        e.forEach(n => r.set(n[0], n[1])),
        this.fieldNameToType = r
    }
    parseFields(e) {
        if (!e)
            return {};
        const r = {};
        return Object.entries(e).forEach( ([n,i]) => {
            const s = this.fieldNameToType.get(n);
            s && (r[n] = qn.parseValue(i, s))
        }
        ),
        r
    }
    serialize(e) {
        return this.fields.map(r => qn.parseValue(e[r[0]], r[1])).join(",")
    }
}
let Ta;
function Mp() {
    return Ta || (Ta = new qn([["deviceModel", "chars"], ["deviceBrand", "chars"], ["osName", "chars"], ["osVersion", "chars"], ["languageVersion", "chars"], ["startTime", "uint"], ["endTime", "uint"], ["count", "uint"], ["clientIPV6", "chars"], ["bizId", "uint"], ["contextId", "chars"], ["entranceId", "chars"], ["ctime", "uint"], ["location", "chars"], ["network", "uint"], ["browser", "chars"], ["userInfo", "chars"], ["redDotInfo", "chars"], ["recInfo", "chars"], ["pageId", "chars"], ["refPageId", "chars"], ["accessId", "chars"], ["refAccessId", "chars"], ["refEleId", "chars"], ["step", "uint"], ["pageInfo", "chars"], ["eleId", "chars"], ["eleInfo", "chars"], ["actionType", "chars"], ["actionInfo", "chars"], ["windowInfo", "chars"], ["entranceInfo", "chars"]]))
}
function Pp(t) {
    if (!t)
        return 0;
    switch (t) {
    case "wifi":
        return 1;
    case "2g":
        return 3;
    case "3g":
        return 4;
    case "4g":
        return 5;
    case "5g":
        return 6;
    default:
        return 0
    }
}
function Op(t, e) {
    var b;
    const {data: r, ctime: n, context: i} = e
      , {env: s, userInfo: o, contextId: a, entranceId: l, redDotInfo: u, recInfo: c, page: g, entranceInfo: d} = i
      , {screenHeight: m, screenWidth: v, clientHeight: p, clientWidth: f} = s;
    return Mp().parseFields({
        deviceModel: s.model,
        deviceBrand: s.brand,
        osName: s.os,
        osVersion: s.osVer,
        languageVersion: s.lang,
        startTime: n / 1e3,
        endTime: n / 1e3,
        count: 1,
        clientIPV6: "",
        bizId: t,
        contextId: a,
        entranceId: l,
        ctime: n,
        location: s.location,
        network: Pp(s.network),
        browser: s.ua,
        userInfo: o,
        redDotInfo: u,
        recInfo: c,
        pageId: g.pageId,
        refPageId: g.refPageId,
        accessId: g.accessId,
        refAccessId: g.refAccessId,
        refEleId: g.fromElementId,
        step: (b = g.step) != null ? b : 1,
        pageInfo: g.pageInfo,
        eleId: r.eleId,
        eleInfo: r.eleInfo,
        actionType: r.behaviorType,
        actionInfo: lp(r, ["eleId", "eleInfo", "behaviorType"]),
        windowInfo: {
            screenHeight: m,
            screenWidth: v,
            clientHeight: p,
            clientWidth: f
        },
        entranceInfo: d
    })
}
let Li = !1;
function Lp() {
    return Li ? Promise.resolve(null) : new Promise(t => {
        var e;
        (e = Xu().WeixinJSBridge) != null && e.invoke ? (Li = !0,
        t(null)) : document.addEventListener("WeixinJSBridgeReady", () => {
            Li = !0,
            t(null)
        }
        , !1)
    }
    )
}
function Np(t) {
    var e, r;
    return !!((r = (e = t == null ? void 0 : t.version) == null ? void 0 : e.startsWith) != null && r.call(e, "3"))
}
function kp(t) {
    var e, r;
    return !!((r = (e = t == null ? void 0 : t.version) == null ? void 0 : e.startsWith) != null && r.call(e, "2"))
}
function li(t, e, r) {
    !t || (Np(t) ? t.directive(e, {
        created: r.created,
        mounted: r.mounted,
        updated: r.updated,
        unmounted: r.unmounted
    }) : kp(t) && t.directive(e, {
        bind: r.created,
        inserted: r.mounted,
        componentUpdated: r.updated,
        unbind: r.unmounted
    }))
}
function rn(t) {
    var e, r;
    return (r = t == null ? void 0 : t.props) != null ? r : (e = t == null ? void 0 : t.data) == null ? void 0 : e.attrs
}
class ec {
    constructor(e) {
        this.ctx = {},
        this.contextEnv = {},
        this.fromPage = {},
        this.aIdGenerator = () => ht(),
        this.errorHandler = (i, s) => {}
        ;
        const {aIdGenerator: r, errorHandler: n} = e || {};
        r && (this.aIdGenerator = r),
        n && (this.errorHandler = n),
        this.cfg = e
    }
    init(e) {
        this.core = e;
        const {collectors: r=[], transports: n=[], userInfo: i, release: s=this.core.release, logError: o=this.core.logError} = this.cfg || {};
        r.forEach(l => {
            var u;
            return (u = this.core) == null ? void 0 : u.collector(l)
        }
        ),
        n.forEach(l => {
            var u;
            return (u = this.core) == null ? void 0 : u.transport(l)
        }
        ),
        i && (this.core.userInfo = i),
        this.core.release = s,
        this.core.logError = o;
        const a = this.initAid();
        return this.updateContext(),
        this.readLinkedData(),
        this.loadPageStackFromStorage(),
        this.afterInit(),
        a
    }
    afterInit() {}
    pushPage(e, r) {
        this.readLinkedData();
        const n = this.loadPageStackFromStorage();
        n ? this.currentPage = {
            pageId: e,
            accessId: ht(),
            step: n.step + 1,
            refAccessId: n.accessId,
            refPageId: n.pageId,
            fromElementId: this.fromPage.eleId,
            fromAccessId: this.fromPage.accessId,
            pageInfo: r
        } : this.currentPage = {
            pageId: e,
            accessId: ht(),
            step: 1,
            pageInfo: r,
            fromElementId: this.fromPage.eleId,
            fromAccessId: this.fromPage.accessId
        },
        this.savePageStackToStorage()
    }
    updatePageInfo(e) {
        !this.currentPage || (this.currentPage.pageInfo = e)
    }
    get contextPage() {
        if (!this.currentPage)
            return {};
        const {pageId: e, accessId: r, step: n, refAccessId: i, refPageId: s, fromElementId: o, fromAccessId: a, pageInfo: l} = this.currentPage;
        return {
            pageId: e,
            accessId: r,
            step: n,
            refAccessId: i,
            refPageId: s,
            fromElementId: o,
            fromAccessId: a,
            pageInfo: l
        }
    }
    getPageStorageKey(e=(r => (r = this.core) == null ? void 0 : r.contextId)()) {
        return `${vt}_${e}`
    }
}
class Bp extends ec {
    constructor() {
        super(...arguments),
        this.storage = {
            supportSync: !1,
            setItem() {},
            getItem() {},
            removeItem() {}
        }
    }
    get env() {
        return ""
    }
    httpPost() {}
    get linkedData() {
        return {}
    }
    initAid() {}
    readLinkedData() {}
    serializeLinkedData() {
        return ""
    }
    settle() {}
    updateContext() {}
    loadPageStackFromStorage() {}
    savePageStackToStorage() {}
}
class Dp extends oi {
    constructor() {
        super(...arguments),
        this.thisOnerrorHandler = this.onerrorHandler.bind(this),
        this.thisErrorListener = this.errorListener.bind(this),
        this.thisUnhandledrejectionListener = this.unhandledrejectionListener.bind(this),
        this.lastCaughtError = null
    }
    afterInit() {
        window.onerror = this.thisOnerrorHandler,
        window.addEventListener("error", this.thisErrorListener, !0),
        window.addEventListener("unhandledrejection", this.thisUnhandledrejectionListener)
    }
    logError(...e) {
        var r;
        (r = this.merlin) != null && r.core.logError
    }
    onerrorHandler(e, r, n, i, s) {
        var o;
        return s === this.lastCaughtError || (this.lastCaughtError = s || null,
        this.logError(s || e),
        (o = this.merlin) == null || o.catch(new Ju({
            source: r,
            lineno: n,
            colno: i,
            event: e
        },s))),
        !0
    }
    errorListener(e) {
        var n;
        const {error: r} = e;
        r !== this.lastCaughtError && (this.lastCaughtError = r || null,
        this.logError(r || e),
        (n = this.merlin) == null || n.catch(new Fs({
            event: e
        },r)))
    }
    unhandledrejectionListener(e) {
        var n, i;
        e.preventDefault();
        const r = e.reason || ((n = e.detail) == null ? void 0 : n.reason);
        r !== this.lastCaughtError && (this.lastCaughtError = r || null,
        this.logError(r || e),
        (i = this.merlin) == null || i.catch(new Fs({
            event: e
        },r)))
    }
}
class Fp extends oi {
    constructor(e) {
        super(),
        this.vue = e.vue
    }
    afterInit() {
        this.vue.config && "errorHandler"in this.vue.config && (this.vue.config.errorHandler = (e, r, n) => {
            var i, s;
            (i = this.merlin) != null && i.core.logError,
            (s = this.merlin) == null || s.catch(new Zt({
                level: r ? void 0 : po.Danger
            },e))
        }
        )
    }
}
class bn {
    constructor(e) {
        this.Catcher = e,
        this.catcherMap = new Map
    }
    register(e, r, n) {
        const i = this.catcherMap.get(e);
        if (i)
            return i.updateExtInfo(r),
            n && i.updateConfig(n),
            i;
        const s = new this.Catcher({
            el: e,
            extInfo: r,
            ...n
        });
        return this.catcherMap.set(e, s),
        s
    }
    deregister(e) {
        const r = this.catcherMap.get(e);
        !r || (r.destroy(),
        this.catcherMap.delete(e))
    }
    get(e) {
        return this.catcherMap.get(e)
    }
    has(e) {
        return this.catcherMap.has(e)
    }
    find(e) {
        for (const r of this.catcherMap.values())
            if (e(r))
                return r
    }
    settle() {
        this.catcherMap.forEach( (e, r) => {
            e.destroy(),
            this.catcherMap.delete(r)
        }
        )
    }
}
var hr = (t => (t.TRUE = "true",
t.FALSE = "false",
t))(hr || {});
function Up(t, e) {
    return !t || !e ? !1 : t === e ? !0 : t.contains(e)
}
const Aa = (t, e) => {
    var r;
    return !!t && !!e && (t === document.body || ((r = t.dataset) == null ? void 0 : r.mlRoot) === hr.TRUE) && t.contains(e)
}
  , jp = t => {
    if (!t)
        return "";
    const e = t.getAttribute("ml-key");
    if (e && e.length > 0)
        return e;
    const r = t.dataset.mlKey;
    return r && r.length > 0 ? r : ""
}
  , Ra = ( () => typeof window == "object" && "IntersectionObserver"in window && "IntersectionObserverEntry"in window && "intersectionRatio"in window.IntersectionObserverEntry.prototype)();
class ui {
    constructor(e) {
        this.el = e.el,
        this.extInfoOrFn = e.extInfo,
        this.key = jp(e.el)
    }
    updateExtInfo(e) {
        this.extInfoOrFn = e
    }
    get extInfo() {
        try {
            return typeof this.extInfoOrFn == "function" ? this.extInfoOrFn() : this.extInfoOrFn
        } catch {
            return {}
        }
    }
}
class $p {
    constructor() {
        this.currentState = null,
        this.stateToTime = new Map,
        this.lastTimeStamp = 0
    }
    init(e, r) {
        e.forEach(n => {
            this.stateToTime.set(n, 0)
        }
        ),
        r !== void 0 && (this.lastTimeStamp = Date.now(),
        this.currentState = r)
    }
    getCurrent() {
        return this.currentState
    }
    switchTo(e) {
        if (!this.stateToTime.has(e) || this.currentState === e)
            return;
        const r = Date.now()
          , n = r - this.lastTimeStamp;
        if (this.currentState !== null) {
            const i = this.stateToTime.get(this.currentState) || 0;
            this.stateToTime.set(this.currentState, n + i)
        }
        this.currentState = e,
        this.lastTimeStamp = r
    }
    updateTime(e) {
        if (!this.stateToTime.has(e) || this.currentState !== e)
            return;
        const r = Date.now()
          , n = r - this.lastTimeStamp
          , i = this.stateToTime.get(this.currentState) || 0;
        this.stateToTime.set(this.currentState, n + i),
        this.lastTimeStamp = r
    }
    resetTime(e) {
        !this.stateToTime.has(e) || this.stateToTime.set(e, 0)
    }
    getTime(e, r=!1) {
        var n;
        return r && this.updateTime(e),
        (n = this.stateToTime.get(e)) != null ? n : 0
    }
}
class Hp extends ui {
    constructor(e) {
        var r;
        super(e),
        this.listener = () => {}
        ,
        e.localListener && (this.localListener = e.localListener,
        this.context = e.context,
        this.listener = this.privateClick.bind(this),
        (r = this.el) == null || r.addEventListener("click", this.listener))
    }
    updateConfig() {}
    destroy() {
        var e;
        this.localListener && ((e = this.el) == null || e.removeEventListener("click", this.listener))
    }
    privateClick() {
        var e, r;
        (r = (e = this.context) == null ? void 0 : e.reporter) == null || r.reportElementClick(this.key, this.extInfo)
    }
}
class qp extends ui {
    constructor(e) {
        super(e),
        this.name = e == null ? void 0 : e.name,
        this.exposeTargets = [],
        this.observer = e.observer,
        this.el.dataset.mlRoot !== hr.TRUE && (this.el.dataset.mlRoot = hr.TRUE),
        (e == null ? void 0 : e.name) && this.el.dataset.mlName !== e.name && (this.el.dataset.mlName = e.name)
    }
    updateConfig(e) {
        this.name = e.name,
        e.observer && (this.exposeTargets = [],
        this.observer = e.observer),
        this.el.dataset.mlRoot = hr.TRUE,
        e != null && e.name && (this.el.dataset.mlName = e.name)
    }
    unObserver(e) {
        var n;
        (n = this.observer) == null || n.unobserve(e.el);
        const r = this.exposeTargets.findIndex(i => i === e);
        r >= 0 && this.exposeTargets.splice(r, 1)
    }
    observe(e) {
        this.exposeTargets.findIndex(r => r.el === e.el) >= 0 || (this.observer.observe(e.el),
        this.exposeTargets.push(e))
    }
    destroy() {
        this.observer.disconnect(),
        this.exposeTargets = [],
        this.el.dataset.mlRoot && delete this.el.dataset.mlRoot,
        this.el.dataset.mlName && delete this.el.dataset.mlName
    }
}
class Vp extends ui {
    constructor(e) {
        super(e),
        this.isExposing = !1,
        this.lastExposeTime = 0,
        this.context = e.context,
        this.rootName = e.rootName,
        this.hidden = !!e.hidden,
        this.globalHidden = !!e.globalHidden,
        this.conceal = e.conceal,
        this.once = e.once;
        const r = ht();
        this.exposeId = r,
        this.firstExposeId = r,
        e.rootName && e.rootName.length > 0 && this.el.dataset.mlRootName !== e.rootName && (this.el.dataset.mlRootName = e.rootName)
    }
    get logicHidden() {
        return this.hidden || this.globalHidden
    }
    get hasExposeRoot() {
        return !!this.exposeRoot
    }
    get exposeRootEl() {
        var e;
        return (e = this.exposeRoot) == null ? void 0 : e.el
    }
    bindExposeRoot(e) {
        var n;
        let r;
        this.rootName && (r = e.find(i => i.name === this.rootName)),
        !r && e.catcherMap.size === 1 && (r = e.get(document.body)),
        r || (r = e.get((n = this.getCurrentExposeRootElement()) != null ? n : document.body)),
        r && (this.exposeRoot && this.exposeRoot.el === r.el || (this.exposeRoot && this.unbindExposeRoot(),
        this.exposeRoot = r,
        r.observe(this)))
    }
    unbindExposeRoot() {
        var e;
        (e = this.exposeRoot) == null || e.unObserver(this),
        this.exposeRoot = void 0
    }
    updateConfig(e) {
        this.rootName = e.rootName,
        this.updateLogicHidden({
            hidden: !!e.hidden
        }),
        e.rootName && e.rootName.length > 0 && this.el.dataset.mlRootName !== e.rootName && (this.el.dataset.mlRootName = e.rootName)
    }
    updateLogicHidden(e) {
        const {hidden: r=this.hidden, globalHidden: n=this.globalHidden} = e
          , i = this.logicHidden
          , s = r || n;
        this.isExposing && (i && !s ? this.localHandleExpose() : !i && s && this.localSettleExpose()),
        this.hidden = r,
        this.globalHidden = n
    }
    handleExpose() {
        this.isExposing = !0,
        this.logicHidden || this.localHandleExpose()
    }
    settleExpose() {
        !this.isExposing || (this.isExposing = !1,
        this.logicHidden || this.localSettleExpose())
    }
    destroy() {
        this.lastExposeTime && this.isExposing && this.settleExpose(),
        this.unbindExposeRoot()
    }
    localHandleExpose() {
        var e, r;
        this.once && this.exposeId !== this.firstExposeId || (this.lastExposeTime = Date.now(),
        (r = (e = this.context) == null ? void 0 : e.reporter) == null || r.reportElementExpose(this.key, this.extInfo, this.exposeId))
    }
    localSettleExpose() {
        var n, i;
        if (this.once && this.exposeId !== this.firstExposeId || this.lastExposeTime === 0)
            return;
        const e = Date.now() - this.lastExposeTime
          , r = this.exposeId;
        this.exposeId = ht(),
        this.lastExposeTime = 0,
        this.conceal && ((i = (n = this.context) == null ? void 0 : n.reporter) == null || i.reportElementConceal(this.key, e, this.extInfo, r))
    }
    getCurrentExposeRootElement() {
        const e = r => {
            var i;
            const n = r.parentElement || r.parentNode;
            return n != null && n.dataset ? n === document.body || ((i = n == null ? void 0 : n.dataset) == null ? void 0 : i.mlRoot) === hr.TRUE ? n : e(n) : null
        }
        ;
        return e(this.el)
    }
}
class Kp extends $p {
    constructor() {
        super(),
        this.init([0, 1, 2, 3], 0)
    }
}
class Wp extends ui {
    constructor(e) {
        super(e),
        this.playId = ht(),
        this.playCount = 0,
        this.maxTime = 0,
        this.finiteStateMachine = new Kp,
        this.currentTime = 0,
        this.gapTime = 1e3,
        this.context = e.context,
        this.waiting = e.waiting,
        this.startMonitor()
    }
    destroy() {
        this.stopMonitor()
    }
    updateConfig() {}
    startMonitor() {
        this.listenVideoPause(),
        this.listenVideoFinished(),
        this.listenVideoStart(),
        this.waiting && this.listenVideoWaiting(),
        this.listenVideoTimeUpdate()
    }
    stopMonitor() {
        this.stopListenVideoPause(),
        this.stopListenVideoStart(),
        this.stopListenVideoFinished(),
        this.stopListenVideoWaiting(),
        this.stopListenVideoTimeUpdate()
    }
    listenVideoTimeUpdate() {
        this.handleVideoTimeUpdate = e => {
            const r = e.target
              , {currentTime: n} = r;
            n > this.maxTime && (this.maxTime = n),
            this.currentTime = n
        }
        ,
        this.el.addEventListener("timeupdate", this.handleVideoTimeUpdate)
    }
    stopListenVideoTimeUpdate() {
        this.handleVideoTimeUpdate && this.el.removeEventListener("timeupdate", this.handleVideoTimeUpdate)
    }
    listenVideoStart() {
        this.handleVideoPlay = e => {
            var i;
            const r = e.target
              , {currentTime: n} = r;
            this.finiteStateMachine.switchTo(1),
            this.playCount += 1,
            (i = this.context.reporter) == null || i.reportVideoPlayStart(this.key, this.playId, this.playCount, n, this.extInfo)
        }
        ,
        this.handleVideoSeekingForVideoLoopStart = e => {
            var o;
            const r = e.target
              , {currentTime: n, duration: i} = r
              , s = this.el.hasAttribute("loop");
            i - this.currentTime < 1 && n === 0 && this.finiteStateMachine.getCurrent() === 1 && s && (this.playCount += 1,
            (o = this.context.reporter) == null || o.reportVideoPlayStart(this.key, this.playId, this.playCount, n, this.extInfo))
        }
        ,
        this.el.addEventListener("play", this.handleVideoPlay),
        this.el.addEventListener("seeking", this.handleVideoSeekingForVideoLoopStart)
    }
    stopListenVideoStart() {
        this.handleVideoPlay && this.el.removeEventListener("play", this.handleVideoPlay),
        this.handleVideoSeekingForVideoLoopStart && this.el.removeEventListener("seeking", this.handleVideoSeekingForVideoLoopStart)
    }
    listenVideoWaiting() {
        this.handleVideoWaiting = e => {
            var i;
            const r = e.target
              , {currentTime: n} = r;
            (i = this.context.reporter) == null || i.reportVideoPlayWaiting(this.key, this.playId, n, this.extInfo)
        }
        ,
        this.el.addEventListener("waiting", this.handleVideoWaiting)
    }
    stopListenVideoWaiting() {
        this.handleVideoWaiting && this.el.removeEventListener("waiting", this.handleVideoWaiting)
    }
    listenVideoFinished() {
        const e = () => {
            this.playId = ht(),
            this.playCount = 0,
            this.maxTime = 0,
            this.finiteStateMachine.resetTime(1)
        }
        ;
        this.handleVideoStop = r => {
            var o;
            const n = r.target
              , {currentTime: i} = n;
            this.finiteStateMachine.switchTo(2);
            const s = this.finiteStateMachine.getTime(1);
            (o = this.context.reporter) == null || o.reportVideoPlayFinish(this.key, this.playId, i, this.maxTime, s, this.extInfo),
            e()
        }
        ,
        this.handleVideoSeekingForVideoLoopFinished = r => {
            var a;
            const n = r.target
              , {currentTime: i, duration: s} = n
              , o = this.el.hasAttribute("loop");
            if (s - this.currentTime < 1 && i === 0 && this.finiteStateMachine.getCurrent() === 1 && o) {
                this.finiteStateMachine.switchTo(2);
                const l = this.finiteStateMachine.getTime(1);
                (a = this.context.reporter) == null || a.reportVideoPlayFinish(this.key, this.playId, i, this.maxTime, l, this.extInfo),
                e(),
                this.finiteStateMachine.switchTo(1)
            }
        }
        ,
        this.el.addEventListener("ended", this.handleVideoStop),
        this.el.addEventListener("seeking", this.handleVideoSeekingForVideoLoopFinished)
    }
    stopListenVideoFinished() {
        var r;
        this.handleVideoStop && this.el.removeEventListener("ended", this.handleVideoStop),
        this.handleVideoSeekingForVideoLoopFinished && this.el.removeEventListener("seeking", this.handleVideoSeekingForVideoLoopFinished);
        const e = this.finiteStateMachine.getCurrent();
        e && [1, 3].includes(e) && ((r = this.context.reporter) == null || r.reportVideoPlayFinish(this.key, this.playId, this.currentTime, this.maxTime, this.finiteStateMachine.getTime(1, !0), this.extInfo)),
        this.finiteStateMachine.switchTo(2),
        this.playId = ht(),
        this.playCount = 0
    }
    listenVideoPause() {
        this.handleVideoPause = e => {
            const r = e.target
              , {currentTime: n} = r;
            this.finiteStateMachine.switchTo(3),
            setTimeout( () => {
                var i;
                this.finiteStateMachine.getCurrent() === 3 && ((i = this.context.reporter) == null || i.reportVideoPlayPause(this.key, this.playId, n, this.extInfo))
            }
            , this.gapTime)
        }
        ,
        this.el.addEventListener("pause", this.handleVideoPause)
    }
    stopListenVideoPause() {
        this.handleVideoPause && this.el.removeEventListener("pause", this.handleVideoPause)
    }
}
class zp extends oi {
    constructor(e) {
        super(),
        this.isExposeKickStarted = !1,
        this.exposeTargetMap = new bn(Vp),
        this.exposeRootMap = new bn(qp),
        this.clickTargetMap = new bn(Hp),
        this.videoTargetMap = new bn(Wp),
        this.garbageMap = new WeakMap,
        this.handleBuffer = new Map,
        this.rootMutationNodeBuffer = new Map,
        this.mutationProcessList = [],
        this.context = {},
        this.mutationObserver = new MutationObserver(this.mutationCallback.bind(this)),
        this.globalHidden = !1;
        const {expose: r, video: n} = e || {}
          , {once: i=!1, conceal: s=!1, viewportAsDefaultRoot: o=!1} = r || {}
          , {waiting: a=!1} = n || {};
        this.config = {
            expose: {
                once: i,
                conceal: s,
                viewportAsDefaultRoot: o
            },
            video: {
                waiting: a
            }
        },
        this.batch = this.mutationBatch.bind(this)
    }
    afterInit() {
        this.context.reporter = this.merlin,
        this.listenClickEvent(),
        this.mutationObserver.observe(document.body, {
            childList: !0,
            subtree: !0
        })
    }
    settle() {
        super.settle(),
        this.videoTargetMap.settle(),
        this.exposeTargetMap.settle()
    }
    click(e, r, n) {
        this.clickTargetMap.register(e, r, {
            context: this.context,
            localListener: !!(n != null && n.localListener)
        })
    }
    deleteClick(e) {
        this.clickTargetMap.deregister(e)
    }
    video(e, r) {
        this.videoTargetMap.register(e, r, {
            context: this.context,
            waiting: this.config.video.waiting
        })
    }
    deleteVideo(e) {
        this.videoTargetMap.deregister(e)
    }
    expose(e, r) {
        if (!Ra)
            return;
        const {extInfo: n, rootName: i, hidden: s, once: o=this.config.expose.once, conceal: a=this.config.expose.conceal} = r || {}
          , l = this.exposeTargetMap.register(e, n, {
            rootName: i,
            hidden: s,
            globalHidden: this.globalHidden,
            context: {
                reporter: this.merlin
            },
            once: o,
            conceal: a
        });
        this.isExposeKickStarted && l.bindExposeRoot(this.exposeRootMap)
    }
    deleteExpose(e) {
        this.exposeTargetMap.deregister(e)
    }
    exposeRoot(e, r) {
        if (!Ra)
            return;
        const {name: n, exposeConfig: i} = r != null ? r : {}
          , s = this.exposeRootMap.register(e, void 0, this.exposeRootMap.has(e) ? {
            name: n
        } : {
            name: n,
            observer: new IntersectionObserver(o => {
                o.forEach(a => {
                    const l = this.exposeTargetMap.get(a.target);
                    !l || (a.isIntersecting ? l.handleExpose() : l.settleExpose())
                }
                )
            }
            ,this.config.expose.viewportAsDefaultRoot ? {
                root: e === document.body ? null : e,
                threshold: .2,
                ...i
            } : {
                root: e,
                threshold: .2,
                ...i
            })
        });
        this.isExposeKickStarted && this.exposeTargetMap.catcherMap.forEach(o => {
            !o.rootName && Aa(s.el, o.el) ? (!o.hasExposeRoot || !Up(s.el, o.exposeRootEl)) && o.bindExposeRoot(this.exposeRootMap) : o.rootName === s.name && Aa(s.el, o.el) && o.bindExposeRoot(this.exposeRootMap)
        }
        )
    }
    deleteExposeRoot(e) {
        this.exposeRootMap.deregister(e)
    }
    kickStartExpose() {
        this.exposeRoot(document.body, {
            name: "body"
        }),
        this.exposeTargetMap.catcherMap.forEach(e => {
            e.bindExposeRoot(this.exposeRootMap)
        }
        ),
        this.isExposeKickStarted = !0
    }
    setGlobalHidden(e) {
        this.globalHidden = e,
        this.exposeTargetMap.catcherMap.forEach(r => {
            r.updateLogicHidden({
                globalHidden: e
            })
        }
        )
    }
    listenClickEvent() {
        document.body.addEventListener("click", e => {
            this.clickTargetMap.catcherMap.forEach(r => {
                var i;
                const n = e.target;
                r.el.contains(n) && ((i = this.merlin) == null || i.reportElementClick(r.key, r.extInfo))
            }
            )
        }
        )
    }
    processRemovedNodes(e) {
        const r = e.length;
        for (let n = 0; n < r; n++) {
            const i = e[n];
            i instanceof HTMLElement && (this.rootMutationNodeBuffer.has(i) || this.rootMutationNodeBuffer.set(i, "removed"))
        }
    }
    processAddedNodes(e) {
        const r = e.length;
        for (let n = 0; n < r; n++) {
            const i = e[n];
            i instanceof HTMLElement && (this.rootMutationNodeBuffer.has(i) || this.rootMutationNodeBuffer.set(i, "added"))
        }
    }
    processMutationBuffer() {
        const e = Array.from(this.rootMutationNodeBuffer.keys());
        for (let r = e.length - 1; r > -1; r--) {
            const n = e[r];
            if (this.rootMutationNodeBuffer.get(n) === "removed")
                this.videoTargetMap.catcherMap.forEach( (i, s) => {
                    n.contains(s) && this.handleBuffer.set(s, !1)
                }
                );
            else {
                let i;
                n instanceof HTMLVideoElement ? i = [n] : i = Array.prototype.slice.call(n.getElementsByTagName("video")),
                i.forEach(s => {
                    (this.handleBuffer.has(s) || this.garbageMap.has(s)) && this.handleBuffer.set(s, !0)
                }
                )
            }
        }
        this.rootMutationNodeBuffer.clear()
    }
    processHandleBuffer() {
        this.handleBuffer.forEach( (e, r) => {
            if (r instanceof HTMLVideoElement)
                if (e) {
                    const n = this.garbageMap.get(r);
                    n && (this.videoTargetMap.catcherMap.set(r, n),
                    n.startMonitor(),
                    this.garbageMap.delete(r))
                } else {
                    const n = this.videoTargetMap.get(r);
                    n && (this.videoTargetMap.catcherMap.delete(r),
                    n.stopMonitor(),
                    this.garbageMap.set(r, n))
                }
        }
        ),
        this.handleBuffer.clear()
    }
    mutationBatch() {
        for (let e = this.mutationProcessList.length - 1; e > -1; e--) {
            const {removedNodes: r, addedNodes: n} = this.mutationProcessList[e];
            r.length && this.processRemovedNodes(r),
            n.length && this.processAddedNodes(n)
        }
        this.processMutationBuffer(),
        this.processHandleBuffer(),
        this.mutationProcessList.length = 0
    }
    mutationCallback(e) {
        this.mutationProcessList.length || requestAnimationFrame(this.batch),
        this.mutationProcessList.push(...e)
    }
}
function Gp(t, e) {
    li(t, "ml-click", {
        mounted(r, n, i) {
            var o;
            const s = (o = rn(i)) == null ? void 0 : o["ml-extends"];
            e.click(r, s, {
                localListener: !!n.modifiers.local
            })
        },
        updated(r, n, i) {
            var o;
            const s = (o = rn(i)) == null ? void 0 : o["ml-extends"];
            e.click(r, s, {
                localListener: !!n.modifiers.local
            })
        },
        unmounted(r) {
            e.deleteClick(r)
        }
    })
}
function Yp(t, e) {
    function r(n, i) {
        var c;
        const s = (c = rn(i)) == null ? void 0 : c["ml-extends"]
          , o = n.arg
          , a = n.modifiers["not-once"] ? !1 : n.modifiers.once
          , l = n.modifiers["not-conceal"] ? !1 : n.modifiers.conceal;
        let u = !1;
        return typeof n.value == "boolean" ? u = n.value : typeof n.value == "object" && n.value !== null && (u = !!n.value.hidden),
        {
            extInfo: s,
            rootName: o,
            hidden: u,
            once: a,
            conceal: l
        }
    }
    li(t, "ml-expose", {
        mounted(n, i, s) {
            e.expose(n, r(i, s))
        },
        updated(n, i, s) {
            e.expose(n, r(i, s))
        },
        unmounted(n) {
            e.deleteExpose(n)
        }
    })
}
function Jp(t, e) {
    li(t, "ml-exposeRoot", {
        mounted(r, n) {
            const i = n.value
              , s = `${n.arg}`;
            e.exposeRoot(r, {
                name: s,
                exposeConfig: i
            })
        },
        unmounted(r) {
            e.deleteExposeRoot(r)
        }
    })
}
function Qp(t, e) {
    li(t, "ml-video", {
        mounted(r, n, i) {
            var o;
            const s = (o = rn(i)) == null ? void 0 : o["ml-extends"];
            e.video(r, s)
        },
        updated(r, n, i) {
            var o;
            const s = (o = rn(i)) == null ? void 0 : o["ml-extends"];
            e.video(r, s)
        },
        unmounted(r) {
            e.deleteVideo(r)
        }
    })
}
const Xp = t => {
    const {options: e} = t;
    let r = !0;
    const n = i => {
        if (!!r) {
            if (!i.name) {
                r = !1;
                return
            }
            i.children && i.children.forEach(s => {
                n(s)
            }
            )
        }
    }
    ;
    return e.routes && e.routes.forEach(i => {
        n(i)
    }
    ),
    r
}
;
class Zp extends zp {
    constructor(e) {
        super(e),
        this.vue = e.vue,
        this.router = e == null ? void 0 : e.router
    }
    bindVue(e, r) {
        this.vue = e,
        r && (this.router = r)
    }
    afterInit() {
        super.afterInit(),
        this.vue && (Yp(this.vue, this),
        Jp(this.vue, this),
        Qp(this.vue, this),
        Gp(this.vue, this)),
        this.kickStartExpose(),
        this.registerPage()
    }
    registerPage() {
        if (!!this.router) {
            if (!Xp(this.router))
                throw new Qu("every route in Vue-router must have a name");
            this.router.afterEach( (e, r) => {
                var l, u, c, g, d, m;
                const n = new Date().getTime();
                let i = n;
                try {
                    if (sessionStorage.setItem(`${e.name}`, n.toString()),
                    r != null && r.name) {
                        const v = parseFloat(sessionStorage.getItem(`${r.name}`) || "");
                        Number.isNaN(v) || (i = v)
                    }
                } catch {}
                const s = n - i;
                ((l = this.merlin) == null ? void 0 : l.core.loadPage()) && ((u = this.merlin) == null || u.reportPageLeave(s));
                const a = (c = e.meta) == null ? void 0 : c.ml;
                (d = this.merlin) == null || d.pushPage((g = a == null ? void 0 : a.pageId) != null ? g : `${e.name}`, {
                    ...e.params,
                    ...e.query,
                    ...a == null ? void 0 : a.extInfo
                }),
                (m = this.merlin) == null || m.reportPageEnter({})
            }
            )
        }
    }
}
( () => {
    const {performance: t} = globalThis;
    if (!t || !t.now)
        return;
    const e = 3600 * 1e3
      , r = t.now()
      , n = Date.now()
      , i = t.timeOrigin ? Math.abs(t.timeOrigin + r - n) : e
      , s = i < e
      , o = t.timing && t.timing.navigationStart
      , l = typeof o == "number" ? Math.abs(o + r - n) : e
      , u = l < e;
    return s || u ? i <= l ? t.timeOrigin : o : n
}
)();
class eg extends ai {
    constructor(e) {
        super({
            flushInterval: 0,
            bufferSize: 0,
            ...e
        }),
        this.reportTypes = [Ze.BEHAVIOR, Ze.ERROR, Ze.PERFORMANCE];
        const {prefix: r="[Merlin]", parser: n=null, method: i="log"} = e || {};
        this.prefix = r,
        this.parser = n,
        this.method = i
    }
    formatTime(e) {
        const r = new Date(e)
          , n = r.getHours().toString().padStart(2, "0")
          , i = r.getMinutes().toString().padStart(2, "0")
          , s = r.getSeconds().toString().padStart(2, "0");
        return `${n}:${i}:${s}`
    }
    print(...e) {
        this.method === "info" || this.method
    }
    send(e) {
        e.forEach(r => {
            const n = `[${r.type}]${this.formatTime(r.ctime)}`;
            if (this.parser) {
                const i = this.parser(r);
                Array.isArray(i) ? this.print(`${this.prefix}${n}`, ...i) : this.print(`${this.prefix}${n}`, i)
            } else
                this.print(`${this.prefix}${n}`, r.context, r.data)
        }
        )
    }
}
class tg extends ec {
    constructor(e) {
        const {collectors: r=[], transports: n=[], generalError: i, vueError: s, log: o, vue: a, pageStackStorageLimit: l=20} = e || {};
        i !== !1 && r.push(new Dp),
        s !== !1 && a && r.push(new Fp({
            vue: a
        })),
        o !== !1 && n.push(new eg(o)),
        super({
            ...e,
            collectors: r,
            transports: n
        }),
        this.storage = {
            supportSync: !0,
            setItem(u, c) {
                try {
                    if (!(window != null && window.localStorage) || c == null)
                        return;
                    window.localStorage.setItem(u, JSON.stringify(c))
                } catch {}
            },
            getItem(u, c) {
                try {
                    if (!(window != null && window.localStorage))
                        return c;
                    const g = window.localStorage.getItem(u);
                    if (g == null)
                        return c;
                    try {
                        return JSON.parse(g)
                    } catch {
                        return g || c
                    }
                } catch {
                    return c
                }
            },
            removeItem(u) {
                try {
                    if (!(window != null && window.localStorage))
                        return;
                    window.localStorage.removeItem(u)
                } catch {}
            }
        },
        this.handleBeforeUnload = this.settleMerlinCore.bind(this),
        this.handlePageHide = this.settleMerlinCore.bind(this),
        this.handleOffline = this.privateHandleOffline.bind(this),
        this.handleOnline = this.privateHandleOnline.bind(this),
        this.pageStackStorageLimit = l
    }
    get env() {
        return "web"
    }
    afterInit() {
        window.addEventListener("beforeunload", this.handleBeforeUnload),
        window.addEventListener("pagehide", this.handlePageHide),
        window.addEventListener("online", this.handleOnline),
        window.addEventListener("offline", this.handleOffline),
        window.dispatchEvent(new CustomEvent("merlin-ready",{
            bubbles: !0,
            cancelable: !1
        }))
    }
    initAid() {
        var n, i;
        if (!this.core)
            return;
        let e = this.storage.getItem(Sa, void 0);
        (!e || typeof e != "string") && (e = this.aIdGenerator(),
        this.storage.setItem(Sa, e)),
        this.core.aid = e;
        const r = (i = (n = this.cfg) == null ? void 0 : n.reportAidToMmdata) != null ? i : !0;
        r && Lp().then( () => {
            var s;
            (s = Xu().WeixinJSBridge) == null || s.invoke("kvReport", {
                id: typeof r == "number" ? r : 28530,
                value: e,
                is_important: 1,
                is_report_now: 1
            })
        }
        )
    }
    readLinkedData() {
        if (!this.core || !window)
            return;
        const e = ap(np());
        e.contextId && (this.core.contextId = e.contextId),
        e.entranceId && (this.ctx.entranceId = e.entranceId),
        e.subEntranceid && (this.ctx.entranceInfo = {
            subEntranceid: e.subEntranceid
        }),
        e.reddotId && (this.ctx.redDotInfo = {
            id: e.reddotId
        }),
        e.fromAccessId && (this.fromPage.accessId = e.fromAccessId),
        e.fromElementId && (this.fromPage.eleId = e.fromElementId)
    }
    get linkedData() {
        var e, r, n, i;
        return {
            contextId: (e = this.core) == null ? void 0 : e.contextId,
            entranceId: this.ctx.entranceId,
            subEntranceid: (r = this.ctx.entranceInfo) == null ? void 0 : r.subEntranceid,
            reddotId: (n = this.ctx.redDotInfo) == null ? void 0 : n.id,
            fromAccessId: (i = this.currentPage) == null ? void 0 : i.accessId
        }
    }
    serializeLinkedData(e) {
        return op({
            ...this.linkedData,
            ...e
        })
    }
    async httpPost(e, r, n) {
        if (typeof (navigator == null ? void 0 : navigator.sendBeacon) == "function")
            try {
                if (navigator.sendBeacon(e, new Blob([r],{
                    type: n.contentType
                })))
                    return
            } catch {}
        try {
            await fetch(e, {
                method: "POST",
                headers: {
                    "Content-Type": n.contentType
                },
                body: r
            })
        } catch (i) {
            this.errorHandler(i)
        }
    }
    updateContext() {
        !window || (this.ctx.url = window.location.href,
        this.contextEnv.ua = window.navigator.userAgent,
        this.contextEnv.clientHeight = window.innerHeight,
        this.contextEnv.clientWidth = window.innerWidth,
        this.contextEnv.screenHeight = window.screen.availHeight,
        this.contextEnv.screenWidth = window.screen.availWidth,
        this.contextEnv.devicePixelRatio = window.devicePixelRatio)
    }
    loadPageStackFromStorage() {
        var n;
        const e = this.storage.getItem(vt, []);
        try {
            localStorage && e && Object.keys(localStorage).forEach(i => {
                i.startsWith(`${vt}_`) && !e.includes(i.replace(`${vt}_`, "")) && this.storage.removeItem(i)
            }
            )
        } catch (i) {
            this == null || this.errorHandler(i)
        }
        const r = this.storage.getItem(this.getPageStorageKey((n = this.core) == null ? void 0 : n.contextId), void 0);
        if (r && typeof r == "object" && Object.prototype.hasOwnProperty.call(r, "accessId") && Object.prototype.hasOwnProperty.call(r, "pageId") && Object.prototype.hasOwnProperty.call(r, "step"))
            return r
    }
    savePageStackToStorage() {
        var r, n;
        const e = (r = this.core) == null ? void 0 : r.contextId;
        if (!(!this.currentPage || !e))
            try {
                const i = this.storage.getItem(vt, []);
                if (i.includes(e))
                    i.indexOf(e) !== i.length - 1 && (i.splice(i.indexOf(e), 1),
                    i.push(e),
                    this.storage.setItem(vt, i));
                else {
                    if (i.length >= this.pageStackStorageLimit) {
                        const s = i.shift();
                        s && this.storage.removeItem(this.getPageStorageKey(s))
                    }
                    i.push(e),
                    this.storage.setItem(vt, i)
                }
                this.storage.setItem(this.getPageStorageKey(e), {
                    pageId: this.currentPage.pageId,
                    accessId: this.currentPage.accessId,
                    step: this.currentPage.step,
                    refAccessId: this.currentPage.refAccessId,
                    fromElementId: this.currentPage.fromElementId,
                    refPageId: this.currentPage.refPageId
                })
            } catch (i) {
                if ((i == null ? void 0 : i.code) === 22 || ((n = i == null ? void 0 : i.message) == null ? void 0 : n.toLowerCase().includes("quota"))) {
                    const s = this.storage.getItem(vt, [])
                      , o = Math.floor(s.length / 2)
                      , a = this.recycleStorage(s, o);
                    this.storage.setItem(vt, a)
                }
            }
    }
    recycleStorage(e, r) {
        if (e.length <= r)
            return e;
        const n = [...e];
        return n.splice(r).forEach(s => {
            this.storage.removeItem(this.getPageStorageKey(s))
        }
        ),
        n
    }
    settleMerlinCore() {
        var e;
        (e = this.core) == null || e.settle()
    }
    privateHandleOffline() {
        var e;
        (e = this.core) == null || e.enableBuffering([Ze.BEHAVIOR, Ze.ERROR, Ze.PERFORMANCE])
    }
    privateHandleOnline() {
        var e;
        (e = this.core) == null || e.settleBuffering()
    }
}
class Zv extends tg {
    constructor(e) {
        const {vue: r, vueRouter: n, collectors: i=[], vueBehavior: s} = e || {};
        let o;
        s !== !1 && r && (o = new Zp({
            ...s,
            vue: r,
            router: n
        }),
        i.push(o)),
        super({
            ...e,
            collectors: i
        }),
        this.builtinVueBehaviorCollector = o
    }
    get vueBehaviorCollector() {
        return this.builtinVueBehaviorCollector
    }
}
class Ni {
    constructor(e) {
        this.type = e,
        this.buffer = [],
        this.buffering = !0
    }
    enableBuffering(e) {
        (Array.isArray(e) && e.includes(this.type) || e === this.type) && (this.buffering = !0)
    }
    settleBuffering(e) {
        (!e || Array.isArray(e) && e.includes(this.type) || e === this.type) && (this.buffering = !1)
    }
    push(e) {
        e.type === this.type && this.buffer.push(e)
    }
    updatePageInfoIfBuffering(e) {
        this.buffering && this.buffer.forEach(r => r.updateContext({
            ...r.context,
            page: {
                ...r.context.page,
                pageInfo: e
            }
        }))
    }
    updateAidIfBuffering(e) {
        this.buffering && this.buffer.forEach(r => r.updateContext({
            ...r.context,
            aid: e
        }))
    }
    consume() {
        if (this.buffering)
            return [];
        const e = [...this.buffer];
        return this.buffer.length = 0,
        e
    }
    get length() {
        return this.buffer.length
    }
}
class rg {
    constructor() {
        this.adapter = new Bp,
        this.contextId = ht(),
        this.logError = !0,
        this.aid = "",
        this.userInfo = {},
        this.release = "",
        this.initialized = !1,
        this.collectors = [],
        this.transports = [],
        this.buffers = [new Ni(Ze.BEHAVIOR), new Ni(Ze.ERROR), new Ni(Ze.PERFORMANCE)]
    }
    collector(e) {
        !oi.isCollector(e) || (this.collectors.push(e),
        this.initialized && this.merlin && e.init(this.merlin))
    }
    transport(e) {
        !ai.isTransport(e) || (this.transports.push(e),
        this.initialized && this.merlin && e.init(this.merlin))
    }
    init(e, r, n) {
        if (!this.initialized)
            try {
                this.adapter = r,
                this.pandora = n;
                const i = this.adapter.init(this);
                this.initialized = !0,
                this.collectors.forEach(s => s.init(e)),
                this.transports.forEach(s => s.init(e)),
                i instanceof Promise ? i.then( () => {
                    this.buffers.forEach(s => s.updateAidIfBuffering(this.aid)),
                    this.settleBuffering()
                }
                ) : this.settleBuffering()
            } catch (i) {
                this.adapter.errorHandler(i)
            }
    }
    enableBuffering(e) {
        this.buffers.forEach(r => r.enableBuffering(e))
    }
    async settleBuffering(e) {
        this.readLinkedData(),
        this.buffers.forEach(r => r.settleBuffering(e));
        try {
            await this.flush()
        } catch (r) {
            this.adapter.errorHandler(r)
        }
    }
    report(e) {
        if (!!this.initialized)
            try {
                this.adapter.updateContext(),
                e.updateContext(this.context),
                this.buffers.forEach(r => r.push(e)),
                this.flush()
            } catch (r) {
                this.adapter.errorHandler(r, e)
            }
    }
    readLinkedData() {
        this.adapter.readLinkedData()
    }
    get objectLinkedData() {
        var e;
        return {
            fromAccessId: (e = this.adapter.currentPage) == null ? void 0 : e.accessId,
            ...this.adapter.linkedData
        }
    }
    get serializeLinkedData() {
        var e, r;
        return ((r = this.adapter) == null ? void 0 : r.serializeLinkedData({
            fromAccessId: (e = this.adapter.currentPage) == null ? void 0 : e.accessId
        })) || ""
    }
    loadPage() {
        return this.adapter.loadPageStackFromStorage()
    }
    pushPage(...e) {
        this.adapter.pushPage(...e)
    }
    getPage() {
        return this.adapter.currentPage
    }
    updatePage(...e) {
        this.adapter.updatePageInfo(...e),
        this.buffers.forEach(r => {
            var n;
            return r.updatePageInfoIfBuffering(((n = this.adapter.currentPage) == null ? void 0 : n.pageInfo) || {})
        }
        )
    }
    get supportSyncStorage() {
        return this.adapter.storage.supportSync
    }
    getStorageItem(...e) {
        var r;
        return (r = this.adapter.storage) == null ? void 0 : r.getItem(...e)
    }
    setStorageItem(...e) {
        var r;
        (r = this.adapter.storage) == null || r.setItem(...e)
    }
    removeStorageItem(...e) {
        var r;
        (r = this.adapter.storage) == null || r.removeItem(...e)
    }
    httpPost(...e) {
        this.adapter.httpPost(...e)
    }
    get env() {
        return this.adapter.env
    }
    async settle() {
        try {
            this.collectors.forEach(e => e.settle()),
            await this.flush(!0)
        } catch (e) {
            this.adapter.errorHandler(e)
        }
    }
    async flush(e=!1) {
        const r = this.buffers.flatMap(n => n.consume());
        return this.sendToTransports(r, e)
    }
    get context() {
        return {
            contextId: this.contextId,
            aid: this.aid,
            userInfo: this.userInfo,
            release: this.release,
            env: this.adapter.contextEnv,
            page: this.adapter.contextPage,
            ...this.adapter.ctx
        }
    }
    sendToTransports(e, r=!1) {
        return Promise.all(this.transports.map(n => n.receiveFromCore(e, r)))
    }
}
class ng {
    constructor(e=new rg) {
        this.core = e,
        this.running = !1
    }
    init(e, r) {
        this.running || (this.running = !0,
        this.core.init(this, e, r))
    }
    setUserInfo(e) {
        this.core.userInfo = e
    }
    collector(e) {
        this.core.collector(e)
    }
    transport(e) {
        this.core.transport(e)
    }
    enableBuffering(e) {
        this.core.enableBuffering(e)
    }
    async settleBuffering(e) {
        this.core.settleBuffering(e)
    }
    get linkedData() {
        return this.core.objectLinkedData
    }
    encodeContextUrl(e) {
        const {serializeLinkedData: r} = this.core;
        return r ? e.includes("?") ? e.endsWith("&") ? `${e}${r}` : `${e}&${r}` : `${e}?${r}` : e
    }
    pushPage(e, r) {
        this.core.pushPage(e, r)
    }
    updatePage(e) {
        this.core.updatePage(e)
    }
    getPage() {
        return this.core.getPage()
    }
    async settle() {
        await this.core.settle()
    }
    catch(e, r) {
        var a, l;
        let n;
        e instanceof Zt && (n = typeof e.info == "object" ? e.info.level : void 0);
        let i = ((a = this.core.pandora) == null ? void 0 : a.catch(e)) || e;
        const s = {
            log: r == null ? void 0 : r.log,
            idx1: r == null ? void 0 : r.idx1,
            idx2: r == null ? void 0 : r.idx2,
            idx3: r == null ? void 0 : r.idx3
        };
        if (i instanceof Zt && (i = i.error),
        i instanceof ln) {
            const {origin: u} = i;
            if (s.name = i.name,
            s.message = i.message,
            s.eExtra = `ts:${i.timestamp}`,
            s.level = i.level,
            s.stack = i.stack,
            s.file = u == null ? void 0 : u.file,
            s.line = u == null ? void 0 : u.line,
            s.col = u == null ? void 0 : u.col,
            i.merlin) {
                const {idx1: c, idx2: g, idx3: d, extra: m} = i.merlin;
                s.eIdx1 = c,
                s.eIdx2 = g,
                s.eIdx3 = d,
                m && (s.eExtra += `|${m}`)
            }
        } else if (s.eExtra = `ts:${Date.now()}`,
        i instanceof Error)
            s.name = "UnknownError",
            s.message = `${i.name ? `${i.name}:` : ""}${i.message}`,
            s.stack = i.stack;
        else {
            s.name = "UnknownObject";
            try {
                typeof i == "object" ? (l = this.core.adapter.cfg) != null && l.stringifyUnknownObject ? s.message = JSON.stringify(i).slice(0, 200) : s.message = `${i.constructor.name}:${Object.keys(i).join(",")}`.slice(0, 200) : s.message = `${i}`.slice(0, 200)
            } catch {}
        }
        n && (s.level = n);
        const o = Tp(s);
        this.core.report(new Ap(o))
    }
    try(e, r, n) {
        try {
            return e()
        } catch (i) {
            return this.catch(i, r),
            n === void 0 ? null : n
        }
    }
    reportPerf(e, r, n, i) {
        const s = i == null ? void 0 : i.sampleRate;
        s !== void 0 && (s <= 0 || s < 1 && Math.random() >= s) || this.core.report(new Ia({
            type: e,
            action: r,
            duration: (n == null ? void 0 : n.duration) || 0,
            status: Ia.parseStatus(n == null ? void 0 : n.status),
            log: n == null ? void 0 : n.log,
            idx1: n == null ? void 0 : n.idx1,
            idx2: n == null ? void 0 : n.idx2,
            idx3: n == null ? void 0 : n.idx3,
            extra: n == null ? void 0 : n.extra
        }))
    }
    createPerfTransaction(e, r, n) {
        return new Rp(this,e,r,n)
    }
    reportCustomBehavior(e, r, n) {
        this.core.report(new Qe({
            behaviorType: We.CUSTOM,
            eleId: n,
            customType: e,
            eleInfo: r
        }))
    }
    reportElementClick(e, r) {
        this.core.report(new Qe({
            behaviorType: We.ELEMENT_CLICK,
            eleId: e,
            eleInfo: r
        }))
    }
    reportElementHover(e, r) {
        this.core.report(new Qe({
            behaviorType: We.ELEMENT_HOVER,
            eleId: e,
            eleInfo: r
        }))
    }
    reportElementExpose(e, r, n) {
        this.core.report(new Qe({
            behaviorType: We.ELEMENT_EXPOSE,
            eleId: e,
            eleInfo: r,
            exposeId: n || ""
        }))
    }
    reportElementConceal(e, r, n, i) {
        this.core.report(new Qe({
            behaviorType: We.ELEMENT_CONCEAL,
            eleId: e,
            eleInfo: n,
            exposeId: i || "",
            allTime: r
        }))
    }
    reportPageEnter(e) {
        this.core.report(new Qe({
            behaviorType: We.PAGE_ENTER,
            eleInfo: e
        }))
    }
    reportPageLeave(e, r) {
        this.core.report(new Qe({
            behaviorType: We.PAGE_LEAVE,
            eleInfo: r,
            stayTime: e
        }))
    }
    reportVideoPlayStart(e, r, n, i, s) {
        this.core.report(new Qe({
            behaviorType: We.VIDEO_PLAY_START,
            eleInfo: s,
            eleId: e,
            playId: r,
            playCount: n,
            currentTime: i
        }))
    }
    reportVideoPlayWaiting(e, r, n, i) {
        this.core.report(new Qe({
            behaviorType: We.VIDEO_PLAY_WAITING,
            eleInfo: i,
            eleId: e,
            playId: r,
            currentTime: n
        }))
    }
    reportVideoPlayPause(e, r, n, i) {
        this.core.report(new Qe({
            behaviorType: We.VIDEO_PLAY_PAUSE,
            eleInfo: i,
            eleId: e,
            playId: r,
            currentTime: n
        }))
    }
    reportVideoPlaySeek(e, r, n, i, s) {
        this.core.report(new Qe({
            behaviorType: We.VIDEO_PLAY_SEEK,
            eleInfo: s,
            eleId: e,
            playId: r,
            position: i,
            currentTime: n
        }))
    }
    reportVideoPlayFinish(e, r, n, i, s, o) {
        this.core.report(new Qe({
            behaviorType: We.VIDEO_PLAY_FINISH,
            eleInfo: o,
            eleId: e,
            playId: r,
            currentTime: n,
            maxTime: i,
            allTime: s
        }))
    }
}
function mr(t) {
    var e, r;
    return t ? ((r = (e = t.split("#")) == null ? void 0 : e[0].split("?")) == null ? void 0 : r[0]) || t : ""
}
function ig(t, e) {
    var n, i;
    if (!t)
        return "";
    const r = (i = (n = t.split("#")) == null ? void 0 : n[0].split("?")) == null ? void 0 : i[1];
    if (r) {
        const s = new URLSearchParams(r).get(e);
        if (s)
            return `${mr(t)}?${e}=${s}`
    }
    return mr(t)
}
function sg(t, e) {
    var n, i;
    if (!t)
        return "";
    const r = (i = (n = t.split("#")) == null ? void 0 : n[0].split("?")) == null ? void 0 : i[1];
    if (r) {
        const s = new URLSearchParams(r)
          , o = [];
        if (e.forEach(a => {
            const l = s.get(a);
            l && o.push([a, l])
        }
        ),
        o.length)
            return `${mr(t)}?${o.map( ([a,l]) => `${a}=${l}`).join("&")}`
    }
    return mr(t)
}
function og(t) {
    let e = mr;
    return typeof t == "string" ? e = r => ig(r, t) : Array.isArray(t) ? e = r => sg(r, t) : typeof t == "function" && (e = t),
    e
}
function ag(t, e) {
    var s;
    if (!t)
        return "";
    const [r,n] = t.split("#")
      , i = (s = r.split("?")) == null ? void 0 : s[1];
    if (i) {
        const o = new URLSearchParams(i);
        e.forEach(u => {
            o.delete(u)
        }
        );
        const a = o.toString();
        let l = mr(t);
        return a && (l += `?${a}`),
        n && (l += `#${n}`),
        l
    }
    return t
}
function lg(t, e, r) {
    const n = t.map(i => {
        let s;
        i.filename ? s = i.filename === r ? "__FILE__" : i.filename : s = "<anonymous>";
        const o = i.lineno || 0
          , a = i.colno || 0;
        return i.func === "?" ? `    at ${s}:${o}:${a}` : `    at ${i.func} (${s}:${o}:${a})`
    }
    );
    return `${e ? `${e}
` : ""}${n.join(`
`)}`
}
function De(t) {
    if (typeof t == "string")
        return t;
    if (t !== void 0)
        return `${t}`
}
class ug extends ai {
    constructor(e) {
        super(e),
        this.reportTypes = [Ze.ERROR],
        this.referMapper = o => o;
        const {referFilter: r, pageMapper: n, alarmSources: i, uriMapper: s} = e;
        if (this.pageMapper = og(n || s),
        r === !0 ? this.referMapper = () => "" : Array.isArray(r) ? this.referMapper = o => ag(o, r) : typeof r == "function" && (this.referMapper = r),
        i)
            if (typeof i == "function")
                this.alarmSourcesFilter = i;
            else {
                const {include: o=[], exclude: a=[]} = i;
                this.alarmSourcesFilter = l => {
                    const {file: u, stack: c} = l.data;
                    return !u && !c ? !0 : !(o.length && !o.some(d => (u == null ? void 0 : u.includes(d)) || (c == null ? void 0 : c.includes(d))) || a.length && a.some(d => (u == null ? void 0 : u.includes(d)) || (c == null ? void 0 : c.includes(d))))
                }
            }
        this.userTokenMapper = e.userTokenMapper || ( () => {}
        )
    }
    commonParseRecord(e) {
        var g, d, m, v, p;
        const {data: r, context: n} = e
          , {release: i, aid: s, env: o, url: a, appId: l, md5: u} = n
          , c = {
            release: De(i),
            msg: De(r.message),
            refer: this.referMapper(a),
            page: this.pageMapper(a),
            user_token: this.userTokenMapper(a),
            aid: De(s),
            type: De(r.name),
            idx1: De(r.idx1),
            idx2: De(r.idx2),
            idx3: De(r.idx3),
            log: De(r.log),
            e_idx1: De(r.eIdx1),
            e_idx2: De(r.eIdx2),
            e_idx3: De(r.eIdx3),
            extra: De(r.eExtra),
            fingerprint: ""
        };
        return ((g = this.reporter) == null ? void 0 : g.core.env) === "lite" ? (c.stack = r.stack,
        c.client_ver = o.clientVer,
        c.lib = o.lib,
        c.network = o.network,
        c.os = o.os,
        c.os_ver = `${o.osVer}`,
        c.brand = o.brand,
        c.model = o.model,
        c.app_id = l,
        c.ua = o.ua,
        c.md5 = u) : (c.fingerprint = `${(d = r.file) != null ? d : ""}:${(m = r.line) != null ? m : ""}:${(v = r.col) != null ? v : ""}`,
        c.fingerprint === "::" && (c.fingerprint = ""),
        c.file = De(r.file),
        c.line = De(r.line),
        c.col = De(r.col),
        c.stack = (p = r.stackFrames) != null && p.length ? lg(r.stackFrames, r.message, r.file) : r.stack),
        this.alarmSourcesFilter ? c.level = this.alarmSourcesFilter(e) ? r.level : "minor" : c.level = r.level,
        c
    }
}
class ey extends ug {
    constructor(e) {
        const r = !e.url.includes("pf=");
        super({
            bufferSize: r ? 0 : 5,
            flushInterval: r ? 0 : 500,
            frequencyLimit: {
                max: 25,
                perSeconds: 900
            },
            ...e
        }),
        this.thisSend = this.sendNew.bind(this);
        const {url: n} = e;
        r && (this.thisSend = this.sendOld.bind(this)),
        this.url = n
    }
    sendOld(e) {
        e.map(n => {
            const {aid: i, release: s, appId: o} = n.context;
            let {stack: a=""} = n.data;
            const {eIdx1: l} = n.data;
            return l && !a && (a = l),
            o && (a += `
appId:${o}`),
            s && (a += `
r:${s}`),
            i && (a += `
aid:${i}`),
            {
                stack: a,
                error_msg: n.data.message,
                error_type: n.data.name,
                page_uri: this.pageMapper(n.context.url)
            }
        }
        ).forEach(n => {
            var i;
            (i = this.reporter) == null || i.core.httpPost(this.url, JSON.stringify(n), {
                contentType: "application/json"
            })
        }
        )
    }
    sendNew(e) {
        var n;
        const r = e.map(i => this.commonParseRecord(i));
        (n = this.reporter) == null || n.core.httpPost(this.url, JSON.stringify({
            items: r
        }), {
            contentType: "application/json"
        })
    }
    send(e) {
        this.thisSend(e)
    }
}
const ty = new ng;
var cg = (t => (t.H265 = "h265",
t.H264 = "h264",
t))(cg || {})
  , fg = (t => (t.INIT = "init",
t.LOADING = "loading",
t.FIRST_FRAME = "first_frame",
t.PLAYING = "playing",
t.ERROR = "error",
t.FALLBACK = "fallback",
t))(fg || {})
  , tc = (t => (t[t.VideoErrType_GLOBAL = 1001] = "VideoErrType_GLOBAL",
t))(tc || {});
let Ma = ""
  , ki = {};
function Hr(t=location.search) {
    if (t === Ma)
        return ki;
    Ma = t;
    const e = t.split("#")[0]
      , r = e.includes("?") ? e.split("?")[1] : e
      , n = {};
    if (!r.includes("="))
        return ki = {
            ...n
        },
        n;
    for (const i of r.split("&")) {
        const [s,o] = i.split("=")
          , a = o.replace(/\+/g, " ");
        try {
            n[s] = decodeURIComponent(a)
        } catch {
            n[s] = a
        }
    }
    return ki = {
        ...n
    },
    n
}
function dg(t) {
    var e, r;
    return (r = (e = t == null ? void 0 : t.match(/^[A-Za-z0-9]+/)) == null ? void 0 : e[0]) != null ? r : ""
}
let Rr = null;
function rc() {
    if (Rr)
        return Rr;
    const t = Hr();
    return Rr = {
        noAutoplay: t.no_autoplay === "1",
        noCustomNav: t.no_custom_nav === "1",
        noNavBack: t.no_nav_back === "1",
        noNavMore: t.no_nav_more === "1",
        noRedirect: !!t.no_redirect,
        hideLocation: t.hideLocation === "1" || t.hideLocation === "true" || t.hide_location === "1" || t.hide_location === "true",
        appStatusBarHeight: parseInt(t.app_status_bar_height || "0", 10) || 0
    },
    Rr
}
function ry() {
    return Rr = null,
    rc()
}
function hg(t, e) {
    var r = {};
    for (var n in t)
        Object.prototype.hasOwnProperty.call(t, n) && e.indexOf(n) < 0 && (r[n] = t[n]);
    if (t != null && typeof Object.getOwnPropertySymbols == "function")
        for (var i = 0, n = Object.getOwnPropertySymbols(t); i < n.length; i++)
            e.indexOf(n[i]) < 0 && Object.prototype.propertyIsEnumerable.call(t, n[i]) && (r[n[i]] = t[n[i]]);
    return r
}
function de(t, e, r, n) {
    function i(s) {
        return s instanceof r ? s : new r(function(o) {
            o(s)
        }
        )
    }
    return new (r || (r = Promise))(function(s, o) {
        function a(c) {
            try {
                u(n.next(c))
            } catch (g) {
                o(g)
            }
        }
        function l(c) {
            try {
                u(n.throw(c))
            } catch (g) {
                o(g)
            }
        }
        function u(c) {
            c.done ? s(c.value) : i(c.value).then(a, l)
        }
        u((n = n.apply(t, e || [])).next())
    }
    )
}
var pg = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : typeof self < "u" ? self : {};
function ir(t) {
    return t && t.__esModule && Object.prototype.hasOwnProperty.call(t, "default") ? t.default : t
}
var wn = {
    exports: {}
}, Bi, Pa;
function nc() {
    return Pa || (Pa = 1,
    Bi = function(e, r) {
        return function() {
            for (var i = new Array(arguments.length), s = 0; s < i.length; s++)
                i[s] = arguments[s];
            return e.apply(r, i)
        }
    }
    ),
    Bi
}
var Di, Oa;
function qe() {
    if (Oa)
        return Di;
    Oa = 1;
    var t = nc()
      , e = Object.prototype.toString;
    function r(h) {
        return e.call(h) === "[object Array]"
    }
    function n(h) {
        return typeof h > "u"
    }
    function i(h) {
        return h !== null && !n(h) && h.constructor !== null && !n(h.constructor) && typeof h.constructor.isBuffer == "function" && h.constructor.isBuffer(h)
    }
    function s(h) {
        return e.call(h) === "[object ArrayBuffer]"
    }
    function o(h) {
        return typeof FormData < "u" && h instanceof FormData
    }
    function a(h) {
        var y;
        return typeof ArrayBuffer < "u" && ArrayBuffer.isView ? y = ArrayBuffer.isView(h) : y = h && h.buffer && h.buffer instanceof ArrayBuffer,
        y
    }
    function l(h) {
        return typeof h == "string"
    }
    function u(h) {
        return typeof h == "number"
    }
    function c(h) {
        return h !== null && typeof h == "object"
    }
    function g(h) {
        if (e.call(h) !== "[object Object]")
            return !1;
        var y = Object.getPrototypeOf(h);
        return y === null || y === Object.prototype
    }
    function d(h) {
        return e.call(h) === "[object Date]"
    }
    function m(h) {
        return e.call(h) === "[object File]"
    }
    function v(h) {
        return e.call(h) === "[object Blob]"
    }
    function p(h) {
        return e.call(h) === "[object Function]"
    }
    function f(h) {
        return c(h) && p(h.pipe)
    }
    function b(h) {
        return typeof URLSearchParams < "u" && h instanceof URLSearchParams
    }
    function _(h) {
        return h.trim ? h.trim() : h.replace(/^\s+|\s+$/g, "")
    }
    function C() {
        return typeof navigator < "u" && (navigator.product === "ReactNative" || navigator.product === "NativeScript" || navigator.product === "NS") ? !1 : typeof window < "u" && typeof document < "u"
    }
    function E(h, y) {
        if (!(h === null || typeof h > "u"))
            if (typeof h != "object" && (h = [h]),
            r(h))
                for (var w = 0, O = h.length; w < O; w++)
                    y.call(null, h[w], w, h);
            else
                for (var A in h)
                    Object.prototype.hasOwnProperty.call(h, A) && y.call(null, h[A], A, h)
    }
    function R() {
        var h = {};
        function y(A, k) {
            g(h[k]) && g(A) ? h[k] = R(h[k], A) : g(A) ? h[k] = R({}, A) : r(A) ? h[k] = A.slice() : h[k] = A
        }
        for (var w = 0, O = arguments.length; w < O; w++)
            E(arguments[w], y);
        return h
    }
    function T(h, y, w) {
        return E(y, function(A, k) {
            w && typeof A == "function" ? h[k] = t(A, w) : h[k] = A
        }),
        h
    }
    function I(h) {
        return h.charCodeAt(0) === 65279 && (h = h.slice(1)),
        h
    }
    return Di = {
        isArray: r,
        isArrayBuffer: s,
        isBuffer: i,
        isFormData: o,
        isArrayBufferView: a,
        isString: l,
        isNumber: u,
        isObject: c,
        isPlainObject: g,
        isUndefined: n,
        isDate: d,
        isFile: m,
        isBlob: v,
        isFunction: p,
        isStream: f,
        isURLSearchParams: b,
        isStandardBrowserEnv: C,
        forEach: E,
        merge: R,
        extend: T,
        trim: _,
        stripBOM: I
    },
    Di
}
var Fi, La;
function yo() {
    if (La)
        return Fi;
    La = 1;
    var t = qe();
    function e(r) {
        return encodeURIComponent(r).replace(/%3A/gi, ":").replace(/%24/g, "$").replace(/%2C/gi, ",").replace(/%20/g, "+").replace(/%5B/gi, "[").replace(/%5D/gi, "]")
    }
    return Fi = function(n, i, s) {
        if (!i)
            return n;
        var o;
        if (s)
            o = s(i);
        else if (t.isURLSearchParams(i))
            o = i.toString();
        else {
            var a = [];
            t.forEach(i, function(c, g) {
                c === null || typeof c > "u" || (t.isArray(c) ? g = g + "[]" : c = [c],
                t.forEach(c, function(m) {
                    t.isDate(m) ? m = m.toISOString() : t.isObject(m) && (m = JSON.stringify(m)),
                    a.push(e(g) + "=" + e(m))
                }))
            }),
            o = a.join("&")
        }
        if (o) {
            var l = n.indexOf("#");
            l !== -1 && (n = n.slice(0, l)),
            n += (n.indexOf("?") === -1 ? "?" : "&") + o
        }
        return n
    }
    ,
    Fi
}
var Ui, Na;
function gg() {
    if (Na)
        return Ui;
    Na = 1;
    var t = qe();
    function e() {
        this.handlers = []
    }
    return e.prototype.use = function(n, i, s) {
        return this.handlers.push({
            fulfilled: n,
            rejected: i,
            synchronous: s ? s.synchronous : !1,
            runWhen: s ? s.runWhen : null
        }),
        this.handlers.length - 1
    }
    ,
    e.prototype.eject = function(n) {
        this.handlers[n] && (this.handlers[n] = null)
    }
    ,
    e.prototype.forEach = function(n) {
        t.forEach(this.handlers, function(s) {
            s !== null && n(s)
        })
    }
    ,
    Ui = e,
    Ui
}
var ji, ka;
function mg() {
    if (ka)
        return ji;
    ka = 1;
    var t = qe();
    return ji = function(r, n) {
        t.forEach(r, function(s, o) {
            o !== n && o.toUpperCase() === n.toUpperCase() && (r[n] = s,
            delete r[o])
        })
    }
    ,
    ji
}
var $i, Ba;
function bo() {
    return Ba || (Ba = 1,
    $i = function(e, r, n, i, s) {
        return e.config = r,
        n && (e.code = n),
        e.request = i,
        e.response = s,
        e.isAxiosError = !0,
        e.toJSON = function() {
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
                code: this.code
            }
        }
        ,
        e
    }
    ),
    $i
}
var Hi, Da;
function wo() {
    if (Da)
        return Hi;
    Da = 1;
    var t = bo();
    return Hi = function(r, n, i, s, o) {
        var a = new Error(r);
        return t(a, n, i, s, o)
    }
    ,
    Hi
}
var qi, Fa;
function ic() {
    if (Fa)
        return qi;
    Fa = 1;
    var t = wo();
    return qi = function(r, n, i) {
        var s = i.config.validateStatus;
        !i.status || !s || s(i.status) ? r(i) : n(t("Request failed with status code " + i.status, i.config, null, i.request, i))
    }
    ,
    qi
}
var Vi, Ua;
function sc() {
    if (Ua)
        return Vi;
    Ua = 1;
    var t = qe();
    return Vi = t.isStandardBrowserEnv() ? function() {
        return {
            write: function(n, i, s, o, a, l) {
                var u = [];
                u.push(n + "=" + encodeURIComponent(i)),
                t.isNumber(s) && u.push("expires=" + new Date(s).toGMTString()),
                t.isString(o) && u.push("path=" + o),
                t.isString(a) && u.push("domain=" + a),
                l === !0 && u.push("secure"),
                document.cookie = u.join("; ")
            },
            read: function(n) {
                var i = document.cookie.match(new RegExp("(^|;\\s*)(" + n + ")=([^;]*)"));
                return i ? decodeURIComponent(i[3]) : null
            },
            remove: function(n) {
                this.write(n, "", Date.now() - 864e5)
            }
        }
    }() : function() {
        return {
            write: function() {},
            read: function() {
                return null
            },
            remove: function() {}
        }
    }(),
    Vi
}
var Ki, ja;
function vg() {
    return ja || (ja = 1,
    Ki = function(e) {
        return /^([a-z][a-z\d\+\-\.]*:)?\/\//i.test(e)
    }
    ),
    Ki
}
var Wi, $a;
function yg() {
    return $a || ($a = 1,
    Wi = function(e, r) {
        return r ? e.replace(/\/+$/, "") + "/" + r.replace(/^\/+/, "") : e
    }
    ),
    Wi
}
var zi, Ha;
function oc() {
    if (Ha)
        return zi;
    Ha = 1;
    var t = vg()
      , e = yg();
    return zi = function(n, i) {
        return n && !t(i) ? e(n, i) : i
    }
    ,
    zi
}
var Gi, qa;
function bg() {
    if (qa)
        return Gi;
    qa = 1;
    var t = qe()
      , e = ["age", "authorization", "content-length", "content-type", "etag", "expires", "from", "host", "if-modified-since", "if-unmodified-since", "last-modified", "location", "max-forwards", "proxy-authorization", "referer", "retry-after", "user-agent"];
    return Gi = function(n) {
        var i = {}, s, o, a;
        return n && t.forEach(n.split(`
`), function(u) {
            if (a = u.indexOf(":"),
            s = t.trim(u.substr(0, a)).toLowerCase(),
            o = t.trim(u.substr(a + 1)),
            s) {
                if (i[s] && e.indexOf(s) >= 0)
                    return;
                s === "set-cookie" ? i[s] = (i[s] ? i[s] : []).concat([o]) : i[s] = i[s] ? i[s] + ", " + o : o
            }
        }),
        i
    }
    ,
    Gi
}
var Yi, Va;
function ac() {
    if (Va)
        return Yi;
    Va = 1;
    var t = qe();
    return Yi = t.isStandardBrowserEnv() ? function() {
        var r = /(msie|trident)/i.test(navigator.userAgent), n = document.createElement("a"), i;
        function s(o) {
            var a = o;
            return r && (n.setAttribute("href", a),
            a = n.href),
            n.setAttribute("href", a),
            {
                href: n.href,
                protocol: n.protocol ? n.protocol.replace(/:$/, "") : "",
                host: n.host,
                search: n.search ? n.search.replace(/^\?/, "") : "",
                hash: n.hash ? n.hash.replace(/^#/, "") : "",
                hostname: n.hostname,
                port: n.port,
                pathname: n.pathname.charAt(0) === "/" ? n.pathname : "/" + n.pathname
            }
        }
        return i = s(window.location.href),
        function(a) {
            var l = t.isString(a) ? s(a) : a;
            return l.protocol === i.protocol && l.host === i.host
        }
    }() : function() {
        return function() {
            return !0
        }
    }(),
    Yi
}
var Ji, Ka;
function Us() {
    if (Ka)
        return Ji;
    Ka = 1;
    var t = qe()
      , e = ic()
      , r = sc()
      , n = yo()
      , i = oc()
      , s = bg()
      , o = ac()
      , a = wo();
    return Ji = function(u) {
        return new Promise(function(g, d) {
            var m = u.data
              , v = u.headers
              , p = u.responseType;
            t.isFormData(m) && delete v["Content-Type"];
            var f = new XMLHttpRequest;
            if (u.auth) {
                var b = u.auth.username || ""
                  , _ = u.auth.password ? unescape(encodeURIComponent(u.auth.password)) : "";
                v.Authorization = "Basic " + btoa(b + ":" + _)
            }
            var C = i(u.baseURL, u.url);
            f.open(u.method.toUpperCase(), n(C, u.params, u.paramsSerializer), !0),
            f.timeout = u.timeout;
            function E() {
                if (!!f) {
                    var T = "getAllResponseHeaders"in f ? s(f.getAllResponseHeaders()) : null
                      , I = !p || p === "text" || p === "json" ? f.responseText : f.response
                      , h = {
                        data: I,
                        status: f.status,
                        statusText: f.statusText,
                        headers: T,
                        config: u,
                        request: f
                    };
                    e(g, d, h),
                    f = null
                }
            }
            if ("onloadend"in f ? f.onloadend = E : f.onreadystatechange = function() {
                !f || f.readyState !== 4 || f.status === 0 && !(f.responseURL && f.responseURL.indexOf("file:") === 0) || setTimeout(E)
            }
            ,
            f.onabort = function() {
                !f || (d(a("Request aborted", u, "ECONNABORTED", f)),
                f = null)
            }
            ,
            f.onerror = function() {
                d(a("Network Error", u, null, f)),
                f = null
            }
            ,
            f.ontimeout = function() {
                var I = "timeout of " + u.timeout + "ms exceeded";
                u.timeoutErrorMessage && (I = u.timeoutErrorMessage),
                d(a(I, u, u.transitional && u.transitional.clarifyTimeoutError ? "ETIMEDOUT" : "ECONNABORTED", f)),
                f = null
            }
            ,
            t.isStandardBrowserEnv()) {
                var R = (u.withCredentials || o(C)) && u.xsrfCookieName ? r.read(u.xsrfCookieName) : void 0;
                R && (v[u.xsrfHeaderName] = R)
            }
            "setRequestHeader"in f && t.forEach(v, function(I, h) {
                typeof m > "u" && h.toLowerCase() === "content-type" ? delete v[h] : f.setRequestHeader(h, I)
            }),
            t.isUndefined(u.withCredentials) || (f.withCredentials = !!u.withCredentials),
            p && p !== "json" && (f.responseType = u.responseType),
            typeof u.onDownloadProgress == "function" && f.addEventListener("progress", u.onDownloadProgress),
            typeof u.onUploadProgress == "function" && f.upload && f.upload.addEventListener("progress", u.onUploadProgress),
            u.cancelToken && u.cancelToken.promise.then(function(I) {
                !f || (f.abort(),
                d(I),
                f = null)
            }),
            m || (m = null),
            f.send(m)
        }
        )
    }
    ,
    Ji
}
var Qi, Wa;
function _o() {
    if (Wa)
        return Qi;
    Wa = 1;
    var t = qe()
      , e = mg()
      , r = bo()
      , n = {
        "Content-Type": "application/x-www-form-urlencoded"
    };
    function i(l, u) {
        !t.isUndefined(l) && t.isUndefined(l["Content-Type"]) && (l["Content-Type"] = u)
    }
    function s() {
        var l;
        return (typeof XMLHttpRequest < "u" || typeof process < "u" && Object.prototype.toString.call(process) === "[object process]") && (l = Us()),
        l
    }
    function o(l, u, c) {
        if (t.isString(l))
            try {
                return (u || JSON.parse)(l),
                t.trim(l)
            } catch (g) {
                if (g.name !== "SyntaxError")
                    throw g
            }
        return (c || JSON.stringify)(l)
    }
    var a = {
        transitional: {
            silentJSONParsing: !0,
            forcedJSONParsing: !0,
            clarifyTimeoutError: !1
        },
        adapter: s(),
        transformRequest: [function(u, c) {
            return e(c, "Accept"),
            e(c, "Content-Type"),
            t.isFormData(u) || t.isArrayBuffer(u) || t.isBuffer(u) || t.isStream(u) || t.isFile(u) || t.isBlob(u) ? u : t.isArrayBufferView(u) ? u.buffer : t.isURLSearchParams(u) ? (i(c, "application/x-www-form-urlencoded;charset=utf-8"),
            u.toString()) : t.isObject(u) || c && c["Content-Type"] === "application/json" ? (i(c, "application/json"),
            o(u)) : u
        }
        ],
        transformResponse: [function(u) {
            var c = this.transitional
              , g = c && c.silentJSONParsing
              , d = c && c.forcedJSONParsing
              , m = !g && this.responseType === "json";
            if (m || d && t.isString(u) && u.length)
                try {
                    return JSON.parse(u)
                } catch (v) {
                    if (m)
                        throw v.name === "SyntaxError" ? r(v, this, "E_JSON_PARSE") : v
                }
            return u
        }
        ],
        timeout: 0,
        xsrfCookieName: "XSRF-TOKEN",
        xsrfHeaderName: "X-XSRF-TOKEN",
        maxContentLength: -1,
        maxBodyLength: -1,
        validateStatus: function(u) {
            return u >= 200 && u < 300
        }
    };
    return a.headers = {
        common: {
            Accept: "application/json, text/plain, */*"
        }
    },
    t.forEach(["delete", "get", "head"], function(u) {
        a.headers[u] = {}
    }),
    t.forEach(["post", "put", "patch"], function(u) {
        a.headers[u] = t.merge(n)
    }),
    Qi = a,
    Qi
}
var Xi, za;
function wg() {
    if (za)
        return Xi;
    za = 1;
    var t = qe()
      , e = _o();
    return Xi = function(n, i, s) {
        var o = this || e;
        return t.forEach(s, function(l) {
            n = l.call(o, n, i)
        }),
        n
    }
    ,
    Xi
}
var Zi, Ga;
function lc() {
    return Ga || (Ga = 1,
    Zi = function(e) {
        return !!(e && e.__CANCEL__)
    }
    ),
    Zi
}
var es, Ya;
function _g() {
    if (Ya)
        return es;
    Ya = 1;
    var t = qe()
      , e = wg()
      , r = lc()
      , n = _o();
    function i(s) {
        s.cancelToken && s.cancelToken.throwIfRequested()
    }
    return es = function(o) {
        i(o),
        o.headers = o.headers || {},
        o.data = e.call(o, o.data, o.headers, o.transformRequest),
        o.headers = t.merge(o.headers.common || {}, o.headers[o.method] || {}, o.headers),
        t.forEach(["delete", "get", "head", "post", "put", "patch", "common"], function(u) {
            delete o.headers[u]
        });
        var a = o.adapter || n.adapter;
        return a(o).then(function(u) {
            return i(o),
            u.data = e.call(o, u.data, u.headers, o.transformResponse),
            u
        }, function(u) {
            return r(u) || (i(o),
            u && u.response && (u.response.data = e.call(o, u.response.data, u.response.headers, o.transformResponse))),
            Promise.reject(u)
        })
    }
    ,
    es
}
var ts, Ja;
function uc() {
    if (Ja)
        return ts;
    Ja = 1;
    var t = qe();
    return ts = function(r, n) {
        n = n || {};
        var i = {}
          , s = ["url", "method", "data"]
          , o = ["headers", "auth", "proxy", "params"]
          , a = ["baseURL", "transformRequest", "transformResponse", "paramsSerializer", "timeout", "timeoutMessage", "withCredentials", "adapter", "responseType", "xsrfCookieName", "xsrfHeaderName", "onUploadProgress", "onDownloadProgress", "decompress", "maxContentLength", "maxBodyLength", "maxRedirects", "transport", "httpAgent", "httpsAgent", "cancelToken", "socketPath", "responseEncoding"]
          , l = ["validateStatus"];
        function u(m, v) {
            return t.isPlainObject(m) && t.isPlainObject(v) ? t.merge(m, v) : t.isPlainObject(v) ? t.merge({}, v) : t.isArray(v) ? v.slice() : v
        }
        function c(m) {
            t.isUndefined(n[m]) ? t.isUndefined(r[m]) || (i[m] = u(void 0, r[m])) : i[m] = u(r[m], n[m])
        }
        t.forEach(s, function(v) {
            t.isUndefined(n[v]) || (i[v] = u(void 0, n[v]))
        }),
        t.forEach(o, c),
        t.forEach(a, function(v) {
            t.isUndefined(n[v]) ? t.isUndefined(r[v]) || (i[v] = u(void 0, r[v])) : i[v] = u(void 0, n[v])
        }),
        t.forEach(l, function(v) {
            v in n ? i[v] = u(r[v], n[v]) : v in r && (i[v] = u(void 0, r[v]))
        });
        var g = s.concat(o).concat(a).concat(l)
          , d = Object.keys(r).concat(Object.keys(n)).filter(function(v) {
            return g.indexOf(v) === -1
        });
        return t.forEach(d, c),
        i
    }
    ,
    ts
}
var Eg = "0.21.4", xg = {
    version: Eg
}, rs, Qa;
function Sg() {
    if (Qa)
        return rs;
    Qa = 1;
    var t = xg
      , e = {};
    ["object", "boolean", "number", "function", "string", "symbol"].forEach(function(o, a) {
        e[o] = function(u) {
            return typeof u === o || "a" + (a < 1 ? "n " : " ") + o
        }
    });
    var r = {}
      , n = t.version.split(".");
    function i(o, a) {
        for (var l = a ? a.split(".") : n, u = o.split("."), c = 0; c < 3; c++) {
            if (l[c] > u[c])
                return !0;
            if (l[c] < u[c])
                return !1
        }
        return !1
    }
    e.transitional = function(a, l, u) {
        var c = l && i(l);
        function g(d, m) {
            return "[Axios v" + t.version + "] Transitional option '" + d + "'" + m + (u ? ". " + u : "")
        }
        return function(d, m, v) {
            if (a === !1)
                throw new Error(g(m, " has been removed in " + l));
            return c && !r[m] && (r[m] = !0),
            a ? a(d, m, v) : !0
        }
    }
    ;
    function s(o, a, l) {
        if (typeof o != "object")
            throw new TypeError("options must be an object");
        for (var u = Object.keys(o), c = u.length; c-- > 0; ) {
            var g = u[c]
              , d = a[g];
            if (d) {
                var m = o[g]
                  , v = m === void 0 || d(m, g, o);
                if (v !== !0)
                    throw new TypeError("option " + g + " must be " + v);
                continue
            }
            if (l !== !0)
                throw Error("Unknown option " + g)
        }
    }
    return rs = {
        isOlderVersion: i,
        assertOptions: s,
        validators: e
    },
    rs
}
var ns, Xa;
function Cg() {
    if (Xa)
        return ns;
    Xa = 1;
    var t = qe()
      , e = yo()
      , r = gg()
      , n = _g()
      , i = uc()
      , s = Sg()
      , o = s.validators;
    function a(l) {
        this.defaults = l,
        this.interceptors = {
            request: new r,
            response: new r
        }
    }
    return a.prototype.request = function(u) {
        typeof u == "string" ? (u = arguments[1] || {},
        u.url = arguments[0]) : u = u || {},
        u = i(this.defaults, u),
        u.method ? u.method = u.method.toLowerCase() : this.defaults.method ? u.method = this.defaults.method.toLowerCase() : u.method = "get";
        var c = u.transitional;
        c !== void 0 && s.assertOptions(c, {
            silentJSONParsing: o.transitional(o.boolean, "1.0.0"),
            forcedJSONParsing: o.transitional(o.boolean, "1.0.0"),
            clarifyTimeoutError: o.transitional(o.boolean, "1.0.0")
        }, !1);
        var g = []
          , d = !0;
        this.interceptors.request.forEach(function(E) {
            typeof E.runWhen == "function" && E.runWhen(u) === !1 || (d = d && E.synchronous,
            g.unshift(E.fulfilled, E.rejected))
        });
        var m = [];
        this.interceptors.response.forEach(function(E) {
            m.push(E.fulfilled, E.rejected)
        });
        var v;
        if (!d) {
            var p = [n, void 0];
            for (Array.prototype.unshift.apply(p, g),
            p = p.concat(m),
            v = Promise.resolve(u); p.length; )
                v = v.then(p.shift(), p.shift());
            return v
        }
        for (var f = u; g.length; ) {
            var b = g.shift()
              , _ = g.shift();
            try {
                f = b(f)
            } catch (C) {
                _(C);
                break
            }
        }
        try {
            v = n(f)
        } catch (C) {
            return Promise.reject(C)
        }
        for (; m.length; )
            v = v.then(m.shift(), m.shift());
        return v
    }
    ,
    a.prototype.getUri = function(u) {
        return u = i(this.defaults, u),
        e(u.url, u.params, u.paramsSerializer).replace(/^\?/, "")
    }
    ,
    t.forEach(["delete", "get", "head", "options"], function(u) {
        a.prototype[u] = function(c, g) {
            return this.request(i(g || {}, {
                method: u,
                url: c,
                data: (g || {}).data
            }))
        }
    }),
    t.forEach(["post", "put", "patch"], function(u) {
        a.prototype[u] = function(c, g, d) {
            return this.request(i(d || {}, {
                method: u,
                url: c,
                data: g
            }))
        }
    }),
    ns = a,
    ns
}
var is, Za;
function cc() {
    if (Za)
        return is;
    Za = 1;
    function t(e) {
        this.message = e
    }
    return t.prototype.toString = function() {
        return "Cancel" + (this.message ? ": " + this.message : "")
    }
    ,
    t.prototype.__CANCEL__ = !0,
    is = t,
    is
}
var ss, el;
function Ig() {
    if (el)
        return ss;
    el = 1;
    var t = cc();
    function e(r) {
        if (typeof r != "function")
            throw new TypeError("executor must be a function.");
        var n;
        this.promise = new Promise(function(o) {
            n = o
        }
        );
        var i = this;
        r(function(o) {
            i.reason || (i.reason = new t(o),
            n(i.reason))
        })
    }
    return e.prototype.throwIfRequested = function() {
        if (this.reason)
            throw this.reason
    }
    ,
    e.source = function() {
        var n, i = new e(function(o) {
            n = o
        }
        );
        return {
            token: i,
            cancel: n
        }
    }
    ,
    ss = e,
    ss
}
var os, tl;
function Tg() {
    return tl || (tl = 1,
    os = function(e) {
        return function(n) {
            return e.apply(null, n)
        }
    }
    ),
    os
}
var as, rl;
function Ag() {
    return rl || (rl = 1,
    as = function(e) {
        return typeof e == "object" && e.isAxiosError === !0
    }
    ),
    as
}
var nl;
function Rg() {
    if (nl)
        return wn.exports;
    nl = 1;
    var t = qe()
      , e = nc()
      , r = Cg()
      , n = uc()
      , i = _o();
    function s(a) {
        var l = new r(a)
          , u = e(r.prototype.request, l);
        return t.extend(u, r.prototype, l),
        t.extend(u, l),
        u
    }
    var o = s(i);
    return o.Axios = r,
    o.create = function(l) {
        return s(n(o.defaults, l))
    }
    ,
    o.Cancel = cc(),
    o.CancelToken = Ig(),
    o.isCancel = lc(),
    o.all = function(l) {
        return Promise.all(l)
    }
    ,
    o.spread = Tg(),
    o.isAxiosError = Ag(),
    wn.exports = o,
    wn.exports.default = o,
    wn.exports
}
var ls, il;
function Mg() {
    return il || (il = 1,
    ls = Rg()),
    ls
}
var Pg = Mg()
  , qr = ir(Pg)
  , Og = bo()
  , sl = ir(Og);
let Mn = "";
if (typeof location < "u" && location.href && typeof URL == "function") {
    const t = new URL(location.href);
    if (Mn = t.searchParams.get("exportkey"),
    !Mn && t.hash) {
        const e = t.hash.substring(1)
          , r = e.indexOf("?");
        if (r !== -1) {
            const n = e.substring(r + 1);
            Mn = new URLSearchParams(n).get("exportkey")
        }
    }
}
const ol = Mn;
let js = !1;
try {
    typeof window < "u" && typeof document < "u" && typeof localStorage < "u" && typeof localStorage.getItem == "function" && (js = !0)
} catch {
    js = !1
}
const Lg = js;
class Pn {
    constructor(e) {
        this.options = e
    }
}
var al = {}, ll;
function Ng() {
    return ll || (ll = 1,
    function(t) {
        t()
    }(function() {
        function t(f, b) {
            if (!(f instanceof b))
                throw new TypeError("Cannot call a class as a function")
        }
        function e(f, b) {
            for (var _ = 0; _ < b.length; _++) {
                var C = b[_];
                C.enumerable = C.enumerable || !1,
                C.configurable = !0,
                "value"in C && (C.writable = !0),
                Object.defineProperty(f, C.key, C)
            }
        }
        function r(f, b, _) {
            return b && e(f.prototype, b),
            f
        }
        function n(f, b) {
            if (typeof b != "function" && b !== null)
                throw new TypeError("Super expression must either be null or a function");
            f.prototype = Object.create(b && b.prototype, {
                constructor: {
                    value: f,
                    writable: !0,
                    configurable: !0
                }
            }),
            b && s(f, b)
        }
        function i(f) {
            return i = Object.setPrototypeOf ? Object.getPrototypeOf : function(_) {
                return _.__proto__ || Object.getPrototypeOf(_)
            }
            ,
            i(f)
        }
        function s(f, b) {
            return s = Object.setPrototypeOf || function(C, E) {
                return C.__proto__ = E,
                C
            }
            ,
            s(f, b)
        }
        function o() {
            if (typeof Reflect > "u" || !Reflect.construct || Reflect.construct.sham)
                return !1;
            if (typeof Proxy == "function")
                return !0;
            try {
                return Boolean.prototype.valueOf.call(Reflect.construct(Boolean, [], function() {})),
                !0
            } catch {
                return !1
            }
        }
        function a(f) {
            if (f === void 0)
                throw new ReferenceError("this hasn't been initialised - super() hasn't been called");
            return f
        }
        function l(f, b) {
            return b && (typeof b == "object" || typeof b == "function") ? b : a(f)
        }
        function u(f) {
            var b = o();
            return function() {
                var C = i(f), E;
                if (b) {
                    var R = i(this).constructor;
                    E = Reflect.construct(C, arguments, R)
                } else
                    E = C.apply(this, arguments);
                return l(this, E)
            }
        }
        function c(f, b) {
            for (; !Object.prototype.hasOwnProperty.call(f, b) && (f = i(f),
            f !== null); )
                ;
            return f
        }
        function g(f, b, _) {
            return typeof Reflect < "u" && Reflect.get ? g = Reflect.get : g = function(E, R, T) {
                var I = c(E, R);
                if (!!I) {
                    var h = Object.getOwnPropertyDescriptor(I, R);
                    return h.get ? h.get.call(T) : h.value
                }
            }
            ,
            g(f, b, _ || f)
        }
        var d = function() {
            function f() {
                t(this, f),
                Object.defineProperty(this, "listeners", {
                    value: {},
                    writable: !0,
                    configurable: !0
                })
            }
            return r(f, [{
                key: "addEventListener",
                value: function(_, C, E) {
                    _ in this.listeners || (this.listeners[_] = []),
                    this.listeners[_].push({
                        callback: C,
                        options: E
                    })
                }
            }, {
                key: "removeEventListener",
                value: function(_, C) {
                    if (_ in this.listeners) {
                        for (var E = this.listeners[_], R = 0, T = E.length; R < T; R++)
                            if (E[R].callback === C) {
                                E.splice(R, 1);
                                return
                            }
                    }
                }
            }, {
                key: "dispatchEvent",
                value: function(_) {
                    if (_.type in this.listeners) {
                        for (var C = this.listeners[_.type], E = C.slice(), R = 0, T = E.length; R < T; R++) {
                            var I = E[R];
                            try {
                                I.callback.call(this, _)
                            } catch (h) {
                                Promise.resolve().then(function() {
                                    throw h
                                })
                            }
                            I.options && I.options.once && this.removeEventListener(_.type, I.callback)
                        }
                        return !_.defaultPrevented
                    }
                }
            }]),
            f
        }()
          , m = function(f) {
            n(_, f);
            var b = u(_);
            function _() {
                var C;
                return t(this, _),
                C = b.call(this),
                C.listeners || d.call(a(C)),
                Object.defineProperty(a(C), "aborted", {
                    value: !1,
                    writable: !0,
                    configurable: !0
                }),
                Object.defineProperty(a(C), "onabort", {
                    value: null,
                    writable: !0,
                    configurable: !0
                }),
                C
            }
            return r(_, [{
                key: "toString",
                value: function() {
                    return "[object AbortSignal]"
                }
            }, {
                key: "dispatchEvent",
                value: function(E) {
                    E.type === "abort" && (this.aborted = !0,
                    typeof this.onabort == "function" && this.onabort.call(this, E)),
                    g(i(_.prototype), "dispatchEvent", this).call(this, E)
                }
            }]),
            _
        }(d)
          , v = function() {
            function f() {
                t(this, f),
                Object.defineProperty(this, "signal", {
                    value: new m,
                    writable: !0,
                    configurable: !0
                })
            }
            return r(f, [{
                key: "abort",
                value: function() {
                    var _;
                    try {
                        _ = new Event("abort")
                    } catch {
                        typeof document < "u" ? document.createEvent ? (_ = document.createEvent("Event"),
                        _.initEvent("abort", !1, !1)) : (_ = document.createEventObject(),
                        _.type = "abort") : _ = {
                            type: "abort",
                            bubbles: !1,
                            cancelable: !1
                        }
                    }
                    this.signal.dispatchEvent(_)
                }
            }, {
                key: "toString",
                value: function() {
                    return "[object AbortController]"
                }
            }]),
            f
        }();
        typeof Symbol < "u" && Symbol.toStringTag && (v.prototype[Symbol.toStringTag] = "AbortController",
        m.prototype[Symbol.toStringTag] = "AbortSignal");
        function p(f) {
            return f.__FORCE_INSTALL_ABORTCONTROLLER_POLYFILL ? !0 : typeof f.Request == "function" && !f.Request.prototype.hasOwnProperty("signal") || !f.AbortController
        }
        (function(f) {
            !p(f) || (f.AbortController = v,
            f.AbortSignal = m)
        }
        )(typeof self < "u" ? self : pg)
    })),
    al
}
Ng();
var kg = qe()
  , $s = ir(kg)
  , Bg = ic()
  , fc = ir(Bg)
  , Dg = yo()
  , dc = ir(Dg)
  , Fg = oc()
  , Eo = ir(Fg)
  , Ug = wo()
  , it = ir(Ug);
const un = typeof lite < "u"
  , hc = typeof wxNative < "u" && typeof wxNative.webTransfer == "function"
  , cn = typeof wx < "u" && typeof wx.request == "function";
( () => {
    if (typeof navigator != "object" || !navigator)
        return !1;
    const t = navigator.userAgent || "";
    return t.includes("MicroMessenger") && (t.includes("WindowsWechat") || t.includes("MacWechat"))
}
)();
( () => {
    if (typeof navigator != "object" || !navigator)
        return !1;
    const t = navigator.userAgent || "";
    return t.includes("MicroMessenger") && t.includes("MacWechat") && !t.includes("UnifiedPC")
}
)();
function jg(t, e, r) {
    t.errMsg.indexOf("request:fail abort") !== -1 ? e(it("Request aborted", r, "ECONNABORTED", "")) : t.errMsg.indexOf("timeout") !== -1 ? e(it("timeout of " + r.timeout + "ms exceeded", r, "ECONNABORTED", "")) : e(it("Network Error", r, null, ""))
}
function $g(t, e, r) {
    const n = t.header
      , i = t.statusCode;
    let s = "";
    return i === 200 ? s = "OK" : i === 400 && (s = "Bad Request"),
    {
        data: t.data,
        status: i,
        statusText: s,
        headers: n,
        config: e,
        request: r
    }
}
const Ir = {};
function Hg() {
    if (Ir.appId)
        return Ir.appId;
    try {
        if (un) {
            if (lite && lite.system && typeof lite.system.getSystemInfo == "function") {
                const {appId: t} = lite.system.getSystemInfo();
                Ir.appId = t
            }
        } else if (cn && wx && wx.getSystemInfoSync) {
            const {host: t} = wx.getSystemInfoSync();
            Ir.appId = t == null ? void 0 : t.appId
        }
    } catch {}
    return Ir.appId
}
let _n, us;
class qg extends Pn {
    getLoginCode(e) {
        return de(this, void 0, void 0, function*() {
            if (!this.options.disabledLiteAppAutoLogin && !e.disabledLiteAppAutoLogin) {
                if (!_n) {
                    const r = this.options.getLoginParams ? this.options.getLoginParams(e) : {};
                    e.timeout && !r.timeout && (r.timeout = e.timeout),
                    _n = new Promise(n => {
                        lite.jsapi.invoke("login", r, i => {
                            n(i)
                        }
                        )
                    }
                    )
                }
                yield _n
            }
        })
    }
    doRequest(e) {
        const {transformResponse: r, disabledPassAppId: n} = this.options;
        return new Promise( (i, s) => de(this, void 0, void 0, function*() {
            const o = {};
            let a = !0;
            const l = typeof AbortController < "u" ? new AbortController : {
                signal: null,
                abort: () => {}
            };
            e.cancelToken && (o.signal = l.signal,
            e.cancelToken.promise.then(function(_) {
                !a || (a = !1,
                l.abort(),
                s(_))
            }));
            function u() {
                if (e.timeout) {
                    o.signal || (o.signal = l.signal);
                    let b = "timeout of " + e.timeout + "ms exceeded";
                    e.timeoutErrorMessage && (b = e.timeoutErrorMessage),
                    setTimeout(function() {
                        !a || (a = !1,
                        l.abort(),
                        s(it(b, e, "ECONNABORTED", e, null)))
                    }, e.timeout)
                }
            }
            u(),
            yield this.getLoginCode(e);
            const c = e.data || null
              , g = e.headers || {};
            if (e.auth) {
                const b = e.auth.username || ""
                  , _ = e.auth.password || "";
                g.Authorization = "Basic " + btoa(b + ":" + _)
            }
            $s.isUndefined(e.withCredentials) || (o.credentials = "include");
            const d = {};
            for (const b in g)
                g.hasOwnProperty(b) && (d[b] = g[b]);
            const m = Eo(e.baseURL, e.url)
              , v = Object.assign({}, o, Object.assign({}, {
                method: e.method.toUpperCase(),
                body: c,
                headers: d
            }, e.fetchOptions || {}))
              , p = new Request(dc(m, Object.assign({
                _loginAppId: n ? void 0 : this.options.appId || e.appId || Hg()
            }, e.params), e.paramsSerializer),v);
            fetch(p.url, v).then(function(_) {
                if (a = !1,
                _.ok) {
                    $s.isFormData(c) && delete g["Content-Type"];
                    const C = _.headers;
                    let E = null;
                    if (r && typeof r == "function")
                        E = r(_, e);
                    else
                        switch (e.responseType) {
                        case "arraybuffer":
                            E = _.arrayBuffer();
                            break;
                        case "text":
                            E = _.text();
                            break;
                        case "json":
                            E = _.json();
                            break;
                        case "blob":
                            E = _.blob();
                            break;
                        default:
                            E = _.json();
                            break
                        }
                    E ? E.then(function(T) {
                        const I = {
                            data: T,
                            status: _.status,
                            statusText: _.statusText,
                            headers: C,
                            config: e,
                            request: p,
                            requestHeaders: g
                        };
                        fc(i, s, I)
                    }, function(T) {
                        s(T || it("Stream decode error", e, _.statusText, p, _))
                    }) : s(it("Failed to resolve response stream.", e, "STREAM_FAILED", p, _))
                } else
                    _.status >= 500 ? s(it("Server-side error: " + _.status + " / " + _.statusText, e, _.statusText, p, _)) : _.status >= 400 ? s(it("Client-side error: " + _.status + " / " + _.statusText, e, _.statusText, p, _)) : s(it("Unknown error", e, _.statusText, p, _))
            }, function(_) {
                _ instanceof Error ? s(it(_.message, e, null, p, _)) : s(it("Network Error", e, null, p, _))
            })
        }))
    }
    sendRequest(e) {
        return de(this, void 0, void 0, function*() {
            const r = e.disabledLiteAppAutoLogin || this.options.disabledLiteAppAutoLogin
              , n = e.isSessionInvalid || this.options.isSessionInvalid;
            if (r || typeof n != "function")
                return this.doRequest(e);
            try {
                const i = yield this.doRequest(e);
                if (n(i.data, e)) {
                    const s = e.loginRetryDelay || this.options.loginRetryDelay || 1e4;
                    return (!us || us.getTime() + s < Date.now()) && (_n = void 0,
                    us = new Date),
                    this.doRequest(e)
                }
                return i
            } catch (i) {
                return Promise.reject(i)
            }
        })
    }
}
var Vg = new qg({});
const Kg = console.warn;
function Wg(t) {
    const e = wx.request.bind(wx);
    return new Promise( (r, n) => {
        let i;
        const s = t.data
          , o = t.headers
          , l = {
            method: t.method && t.method.toUpperCase() || "GET",
            url: dc(Eo(t.baseURL, t.url), t.params, t.paramsSerializer),
            success: u => {
                const c = $g(u, t, l);
                fc(r, n, c)
            }
            ,
            fail: u => {
                jg(u, n, t)
            }
            ,
            complete() {
                i = void 0
            }
        };
        t.timeout !== 0 && Kg('The "timeout" option is not supported by miniprogram. For more information about usage see "https://developers.weixin.qq.com/miniprogram/dev/framework/config.html#\u5168\u5C40\u914D\u7F6E"'),
        $s.forEach(o, function(c, g) {
            const d = g.toLowerCase();
            (typeof s > "u" && d === "content-type" || d === "referer") && delete o[g]
        }),
        l.header = o,
        t.responseType && (l.responseType = t.responseType),
        t.cancelToken && t.cancelToken.promise.then(function(c) {
            !i || (i.abort(),
            n(c),
            i = void 0)
        }),
        s !== void 0 && (l.data = s),
        i = e(l)
    }
    )
}
var Wt;
(function(t) {
    t.Network = "Network",
    t.Timeout = "Timeout",
    t.Abort = "Abort",
    t.HttpStatus = "HttpStatus",
    t.WebTransferJsApi = "WebTransferJsApi"
}
)(Wt || (Wt = {}));
class On extends ep {
    constructor(e, r, n) {
        var i, s;
        let o = "";
        n != null && n.config && (o = Eo(n.config.baseURL, n.config.url)),
        super(e, o, {
            errCode: r
        }),
        this.config = n == null ? void 0 : n.config,
        this.name = "RequestXError",
        this.resp = n == null ? void 0 : n.response,
        this.stack = n == null ? void 0 : n.stack,
        this.rid = (i = n == null ? void 0 : n.config) === null || i === void 0 ? void 0 : i.rid,
        this.displayTitle = (s = n == null ? void 0 : n.displayTitle) !== null && s !== void 0 ? s : "",
        this.displayContent = (n == null ? void 0 : n.displayContent) || "\u8BF7\u6C42\u5F02\u5E38"
    }
    get merlin() {
        return {
            idx1: this.url,
            idx2: this.errCode ? `${this.errCode}` : void 0,
            extra: JSON.stringify(this.config)
        }
    }
    formatErrorMessage(e) {
        var r;
        let n = (r = e != null ? e : this.displayContent) !== null && r !== void 0 ? r : "";
        return this.errCode && !n.endsWith(`${this.errCode}`) && !n.endsWith(`(${this.errCode})`) && (n += `(${this.errCode})`),
        this.rid && (n += `
${this.rid}`),
        n
    }
}
class pc extends On {
    constructor(e) {
        var r;
        super(e.message || "\u7F51\u7EDC\u5F02\u5E38", e.code || ((r = e.response) === null || r === void 0 ? void 0 : r.status), {
            config: e.config,
            response: e.response,
            displayContent: "\u7F51\u7EDC\u5F02\u5E38"
        }),
        this.name = "NetworkRequestXError",
        this.isCancel = !!e.isCancel,
        e.message === "Network Error" ? this.type = Wt.Network : e.message.indexOf("timeout") !== -1 ? this.type = Wt.Timeout : e.code === "ECONNABORTED" || this.isCancel ? this.type = Wt.Abort : e.message.includes("webTransfer") ? this.type = Wt.WebTransferJsApi : this.type = Wt.HttpStatus
    }
    get merlin() {
        return {
            idx1: this.url,
            idx2: this.errCode ? `${this.errCode}` : void 0,
            idx3: `${this.type}`,
            extra: JSON.stringify(this.config)
        }
    }
}
function Mr(t) {
    if (qr.isAxiosError(t) || qr.isCancel(t))
        return new pc(t)
}
function zg() {
    return Math.floor(Date.now() / 1e3).toString(16)
}
function Gg() {
    return [...Array(8)].map( () => Math.floor(Math.random() * 16).toString(16)).join("")
}
function Yg() {
    return `${zg()}-${Gg()}`
}
const Jg = () => {
    var t, e, r, n, i;
    let s = "";
    try {
        return un ? s = (e = (t = lite == null ? void 0 : lite.router) === null || t === void 0 ? void 0 : t.currentPages[lite.router.currentPages.length - 1]) === null || e === void 0 ? void 0 : e.path : hc || cn ? s = (n = (r = getCurrentPages == null ? void 0 : getCurrentPages()) === null || r === void 0 ? void 0 : r[0]) === null || n === void 0 ? void 0 : n.route : typeof location == "object" && (s = location.href),
        (i = s == null ? void 0 : s.replace(/\?.*$/, "")) !== null && i !== void 0 ? i : ""
    } catch {
        return ""
    }
}
  , Qg = "__ml::aid"
  , cs = "__rx::aid";
function Xg() {
    try {
        const t = crypto;
        if (t != null && t.randomUUID)
            return t.randomUUID();
        if ((t == null ? void 0 : t.getRandomValues) && Uint8Array)
            return "10000000-1000-4000-8000-100000000000".replace(/[018]/g, e => (Number(e) ^ t.getRandomValues(new Uint8Array(1))[0] & 15 >> Number(e) / 4).toString(16))
    } catch {}
    return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, t => {
        const e = Math.random() * 16 | 0;
        return (t === "x" ? e : e & 3 | 8).toString(16)
    }
    )
}
function ul(t) {
    return de(this, void 0, void 0, function*() {
        return un ? yield lite.kv.getItem(t) : cn ? wx.getStorageSync(t) : localStorage.getItem(t)
    })
}
function cl(t, e) {
    return de(this, void 0, void 0, function*() {
        un ? yield lite.kv.setItem(t, e) : cn ? wx.setStorageSync(t, e) : localStorage.setItem(t, e)
    })
}
function fl() {
    return de(this, void 0, void 0, function*() {
        if (!Lg)
            return "";
        let t = "";
        const [e,r] = yield Promise.all([ul(Qg), ul(cs)]);
        return e ? (t = e,
        yield cl(cs, t)) : r ? t = r : (t = Xg(),
        yield cl(cs, t)),
        t
    })
}
var fs, dl;
function gc() {
    if (dl)
        return fs;
    dl = 1;
    var t = String.prototype.replace
      , e = /%20/g
      , r = {
        RFC1738: "RFC1738",
        RFC3986: "RFC3986"
    };
    return fs = {
        default: r.RFC3986,
        formatters: {
            RFC1738: function(n) {
                return t.call(n, e, "+")
            },
            RFC3986: function(n) {
                return String(n)
            }
        },
        RFC1738: r.RFC1738,
        RFC3986: r.RFC3986
    },
    fs
}
var ds, hl;
function Zg() {
    if (hl)
        return ds;
    hl = 1;
    var t = gc()
      , e = Object.prototype.hasOwnProperty
      , r = Array.isArray
      , n = function() {
        for (var p = [], f = 0; f < 256; ++f)
            p.push("%" + ((f < 16 ? "0" : "") + f.toString(16)).toUpperCase());
        return p
    }()
      , i = function(f) {
        for (; f.length > 1; ) {
            var b = f.pop()
              , _ = b.obj[b.prop];
            if (r(_)) {
                for (var C = [], E = 0; E < _.length; ++E)
                    typeof _[E] < "u" && C.push(_[E]);
                b.obj[b.prop] = C
            }
        }
    }
      , s = function(f, b) {
        for (var _ = b && b.plainObjects ? Object.create(null) : {}, C = 0; C < f.length; ++C)
            typeof f[C] < "u" && (_[C] = f[C]);
        return _
    }
      , o = function p(f, b, _) {
        if (!b)
            return f;
        if (typeof b != "object") {
            if (r(f))
                f.push(b);
            else if (f && typeof f == "object")
                (_ && (_.plainObjects || _.allowPrototypes) || !e.call(Object.prototype, b)) && (f[b] = !0);
            else
                return [f, b];
            return f
        }
        if (!f || typeof f != "object")
            return [f].concat(b);
        var C = f;
        return r(f) && !r(b) && (C = s(f, _)),
        r(f) && r(b) ? (b.forEach(function(E, R) {
            if (e.call(f, R)) {
                var T = f[R];
                T && typeof T == "object" && E && typeof E == "object" ? f[R] = p(T, E, _) : f.push(E)
            } else
                f[R] = E
        }),
        f) : Object.keys(b).reduce(function(E, R) {
            var T = b[R];
            return e.call(E, R) ? E[R] = p(E[R], T, _) : E[R] = T,
            E
        }, C)
    }
      , a = function(f, b) {
        return Object.keys(b).reduce(function(_, C) {
            return _[C] = b[C],
            _
        }, f)
    }
      , l = function(p, f, b) {
        var _ = p.replace(/\+/g, " ");
        if (b === "iso-8859-1")
            return _.replace(/%[0-9a-f]{2}/gi, unescape);
        try {
            return decodeURIComponent(_)
        } catch {
            return _
        }
    }
      , u = function(f, b, _, C, E) {
        if (f.length === 0)
            return f;
        var R = f;
        if (typeof f == "symbol" ? R = Symbol.prototype.toString.call(f) : typeof f != "string" && (R = String(f)),
        _ === "iso-8859-1")
            return escape(R).replace(/%u[0-9a-f]{4}/gi, function(y) {
                return "%26%23" + parseInt(y.slice(2), 16) + "%3B"
            });
        for (var T = "", I = 0; I < R.length; ++I) {
            var h = R.charCodeAt(I);
            if (h === 45 || h === 46 || h === 95 || h === 126 || h >= 48 && h <= 57 || h >= 65 && h <= 90 || h >= 97 && h <= 122 || E === t.RFC1738 && (h === 40 || h === 41)) {
                T += R.charAt(I);
                continue
            }
            if (h < 128) {
                T = T + n[h];
                continue
            }
            if (h < 2048) {
                T = T + (n[192 | h >> 6] + n[128 | h & 63]);
                continue
            }
            if (h < 55296 || h >= 57344) {
                T = T + (n[224 | h >> 12] + n[128 | h >> 6 & 63] + n[128 | h & 63]);
                continue
            }
            I += 1,
            h = 65536 + ((h & 1023) << 10 | R.charCodeAt(I) & 1023),
            T += n[240 | h >> 18] + n[128 | h >> 12 & 63] + n[128 | h >> 6 & 63] + n[128 | h & 63]
        }
        return T
    }
      , c = function(f) {
        for (var b = [{
            obj: {
                o: f
            },
            prop: "o"
        }], _ = [], C = 0; C < b.length; ++C)
            for (var E = b[C], R = E.obj[E.prop], T = Object.keys(R), I = 0; I < T.length; ++I) {
                var h = T[I]
                  , y = R[h];
                typeof y == "object" && y !== null && _.indexOf(y) === -1 && (b.push({
                    obj: R,
                    prop: h
                }),
                _.push(y))
            }
        return i(b),
        f
    }
      , g = function(f) {
        return Object.prototype.toString.call(f) === "[object RegExp]"
    }
      , d = function(f) {
        return !f || typeof f != "object" ? !1 : !!(f.constructor && f.constructor.isBuffer && f.constructor.isBuffer(f))
    }
      , m = function(f, b) {
        return [].concat(f, b)
    }
      , v = function(f, b) {
        if (r(f)) {
            for (var _ = [], C = 0; C < f.length; C += 1)
                _.push(b(f[C]));
            return _
        }
        return b(f)
    };
    return ds = {
        arrayToObject: s,
        assign: a,
        combine: m,
        compact: c,
        decode: l,
        encode: u,
        isBuffer: d,
        isRegExp: g,
        maybeMap: v,
        merge: o
    },
    ds
}
var hs, pl;
function em() {
    if (pl)
        return hs;
    pl = 1;
    var t = Zg()
      , e = gc()
      , r = Object.prototype.hasOwnProperty
      , n = {
        brackets: function(p) {
            return p + "[]"
        },
        comma: "comma",
        indices: function(p, f) {
            return p + "[" + f + "]"
        },
        repeat: function(p) {
            return p
        }
    }
      , i = Array.isArray
      , s = String.prototype.split
      , o = Array.prototype.push
      , a = function(v, p) {
        o.apply(v, i(p) ? p : [p])
    }
      , l = Date.prototype.toISOString
      , u = e.default
      , c = {
        addQueryPrefix: !1,
        allowDots: !1,
        charset: "utf-8",
        charsetSentinel: !1,
        delimiter: "&",
        encode: !0,
        encoder: t.encode,
        encodeValuesOnly: !1,
        format: u,
        formatter: e.formatters[u],
        indices: !1,
        serializeDate: function(p) {
            return l.call(p)
        },
        skipNulls: !1,
        strictNullHandling: !1
    }
      , g = function(p) {
        return typeof p == "string" || typeof p == "number" || typeof p == "boolean" || typeof p == "symbol" || typeof p == "bigint"
    }
      , d = function v(p, f, b, _, C, E, R, T, I, h, y, w, O, A) {
        var k = p;
        if (typeof R == "function" ? k = R(f, k) : k instanceof Date ? k = h(k) : b === "comma" && i(k) && (k = t.maybeMap(k, function(ae) {
            return ae instanceof Date ? h(ae) : ae
        })),
        k === null) {
            if (_)
                return E && !O ? E(f, c.encoder, A, "key", y) : f;
            k = ""
        }
        if (g(k) || t.isBuffer(k)) {
            if (E) {
                var V = O ? f : E(f, c.encoder, A, "key", y);
                if (b === "comma" && O) {
                    for (var $ = s.call(String(k), ","), D = "", Y = 0; Y < $.length; ++Y)
                        D += (Y === 0 ? "" : ",") + w(E($[Y], c.encoder, A, "value", y));
                    return [w(V) + "=" + D]
                }
                return [w(V) + "=" + w(E(k, c.encoder, A, "value", y))]
            }
            return [w(f) + "=" + w(String(k))]
        }
        var H = [];
        if (typeof k > "u")
            return H;
        var re;
        if (b === "comma" && i(k))
            re = [{
                value: k.length > 0 ? k.join(",") || null : void 0
            }];
        else if (i(R))
            re = R;
        else {
            var he = Object.keys(k);
            re = T ? he.sort(T) : he
        }
        for (var Z = 0; Z < re.length; ++Z) {
            var se = re[Z]
              , ve = typeof se == "object" && typeof se.value < "u" ? se.value : k[se];
            if (!(C && ve === null)) {
                var Ae = i(k) ? typeof b == "function" ? b(f, se) : f : f + (I ? "." + se : "[" + se + "]");
                a(H, v(ve, Ae, b, _, C, E, R, T, I, h, y, w, O, A))
            }
        }
        return H
    }
      , m = function(p) {
        if (!p)
            return c;
        if (p.encoder !== null && typeof p.encoder < "u" && typeof p.encoder != "function")
            throw new TypeError("Encoder has to be a function.");
        var f = p.charset || c.charset;
        if (typeof p.charset < "u" && p.charset !== "utf-8" && p.charset !== "iso-8859-1")
            throw new TypeError("The charset option must be either utf-8, iso-8859-1, or undefined");
        var b = e.default;
        if (typeof p.format < "u") {
            if (!r.call(e.formatters, p.format))
                throw new TypeError("Unknown format option provided.");
            b = p.format
        }
        var _ = e.formatters[b]
          , C = c.filter;
        return (typeof p.filter == "function" || i(p.filter)) && (C = p.filter),
        {
            addQueryPrefix: typeof p.addQueryPrefix == "boolean" ? p.addQueryPrefix : c.addQueryPrefix,
            allowDots: typeof p.allowDots > "u" ? c.allowDots : !!p.allowDots,
            charset: f,
            charsetSentinel: typeof p.charsetSentinel == "boolean" ? p.charsetSentinel : c.charsetSentinel,
            delimiter: typeof p.delimiter > "u" ? c.delimiter : p.delimiter,
            encode: typeof p.encode == "boolean" ? p.encode : c.encode,
            encoder: typeof p.encoder == "function" ? p.encoder : c.encoder,
            encodeValuesOnly: typeof p.encodeValuesOnly == "boolean" ? p.encodeValuesOnly : c.encodeValuesOnly,
            filter: C,
            format: b,
            formatter: _,
            serializeDate: typeof p.serializeDate == "function" ? p.serializeDate : c.serializeDate,
            skipNulls: typeof p.skipNulls == "boolean" ? p.skipNulls : c.skipNulls,
            sort: typeof p.sort == "function" ? p.sort : null,
            strictNullHandling: typeof p.strictNullHandling == "boolean" ? p.strictNullHandling : c.strictNullHandling
        }
    };
    return hs = function(v, p) {
        var f = v, b = m(p), _, C;
        typeof b.filter == "function" ? (C = b.filter,
        f = C("", f)) : i(b.filter) && (C = b.filter,
        _ = C);
        var E = [];
        if (typeof f != "object" || f === null)
            return "";
        var R;
        p && p.arrayFormat in n ? R = p.arrayFormat : p && "indices"in p ? R = p.indices ? "indices" : "repeat" : R = "indices";
        var T = n[R];
        _ || (_ = Object.keys(f)),
        b.sort && _.sort(b.sort);
        for (var I = 0; I < _.length; ++I) {
            var h = _[I];
            b.skipNulls && f[h] === null || a(E, d(f[h], h, T, b.strictNullHandling, b.skipNulls, b.encode ? b.encoder : null, b.filter, b.sort, b.allowDots, b.serializeDate, b.format, b.formatter, b.encodeValuesOnly, b.charset))
        }
        var y = E.join(b.delimiter)
          , w = b.addQueryPrefix === !0 ? "?" : "";
        return b.charsetSentinel && (b.charset === "iso-8859-1" ? w += "utf8=%26%2310003%3B&" : w += "utf8=%E2%9C%93&"),
        y.length > 0 ? w + y : ""
    }
    ,
    hs
}
em();
Us();
ac();
sc();
var Pr = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : typeof self < "u" ? self : {};
function tm(t) {
    return t && t.__esModule && Object.prototype.hasOwnProperty.call(t, "default") ? t.default : t
}
var rm = 1 / 0
  , nm = 9007199254740991
  , im = "[object Arguments]"
  , sm = "[object Function]"
  , om = "[object GeneratorFunction]"
  , am = "[object Symbol]"
  , lm = typeof Pr == "object" && Pr && Pr.Object === Object && Pr
  , um = typeof self == "object" && self && self.Object === Object && self
  , cm = lm || um || Function("return this")();
function fm(t, e, r) {
    switch (r.length) {
    case 0:
        return t.call(e);
    case 1:
        return t.call(e, r[0]);
    case 2:
        return t.call(e, r[0], r[1]);
    case 3:
        return t.call(e, r[0], r[1], r[2])
    }
    return t.apply(e, r)
}
function dm(t, e) {
    for (var r = -1, n = t ? t.length : 0, i = Array(n); ++r < n; )
        i[r] = e(t[r], r, t);
    return i
}
function hm(t, e) {
    for (var r = -1, n = e.length, i = t.length; ++r < n; )
        t[i + r] = e[r];
    return t
}
var xo = Object.prototype
  , pm = xo.hasOwnProperty
  , So = xo.toString
  , gl = cm.Symbol
  , gm = xo.propertyIsEnumerable
  , ml = gl ? gl.isConcatSpreadable : void 0
  , vl = Math.max;
function mc(t, e, r, n, i) {
    var s = -1
      , o = t.length;
    for (r || (r = bm),
    i || (i = []); ++s < o; ) {
        var a = t[s];
        e > 0 && r(a) ? e > 1 ? mc(a, e - 1, r, n, i) : hm(i, a) : n || (i[i.length] = a)
    }
    return i
}
function mm(t, e) {
    return t = Object(t),
    vm(t, e, function(r, n) {
        return n in t
    })
}
function vm(t, e, r) {
    for (var n = -1, i = e.length, s = {}; ++n < i; ) {
        var o = e[n]
          , a = t[o];
        r(a, o) && (s[o] = a)
    }
    return s
}
function ym(t, e) {
    return e = vl(e === void 0 ? t.length - 1 : e, 0),
    function() {
        for (var r = arguments, n = -1, i = vl(r.length - e, 0), s = Array(i); ++n < i; )
            s[n] = r[e + n];
        n = -1;
        for (var o = Array(e + 1); ++n < e; )
            o[n] = r[n];
        return o[e] = s,
        fm(t, this, o)
    }
}
function bm(t) {
    return Em(t) || _m(t) || !!(ml && t && t[ml])
}
function wm(t) {
    if (typeof t == "string" || Am(t))
        return t;
    var e = t + "";
    return e == "0" && 1 / t == -rm ? "-0" : e
}
function _m(t) {
    return Sm(t) && pm.call(t, "callee") && (!gm.call(t, "callee") || So.call(t) == im)
}
var Em = Array.isArray;
function xm(t) {
    return t != null && Im(t.length) && !Cm(t)
}
function Sm(t) {
    return vc(t) && xm(t)
}
function Cm(t) {
    var e = Tm(t) ? So.call(t) : "";
    return e == sm || e == om
}
function Im(t) {
    return typeof t == "number" && t > -1 && t % 1 == 0 && t <= nm
}
function Tm(t) {
    var e = typeof t;
    return !!t && (e == "object" || e == "function")
}
function vc(t) {
    return !!t && typeof t == "object"
}
function Am(t) {
    return typeof t == "symbol" || vc(t) && So.call(t) == am
}
ym(function(t, e) {
    return t == null ? {} : mm(t, dm(mc(e, 1), wm))
});
function En(t) {
    throw new Error('Could not dynamically require "' + t + '". Please configure the dynamicRequireTargets or/and ignoreDynamicRequires option of @rollup/plugin-commonjs appropriately for this require call to work.')
}
var Rm = {
    exports: {}
};
(function(t, e) {
    (function(r) {
        t.exports = r()
    }
    )(function() {
        return function r(n, i, s) {
            function o(u, c) {
                if (!i[u]) {
                    if (!n[u]) {
                        var g = typeof En == "function" && En;
                        if (!c && g)
                            return g(u, !0);
                        if (a)
                            return a(u, !0);
                        throw new Error("Cannot find module '" + u + "'")
                    }
                    var d = i[u] = {
                        exports: {}
                    };
                    n[u][0].call(d.exports, function(m) {
                        var v = n[u][1][m];
                        return o(v || m)
                    }, d, d.exports, r, n, i, s)
                }
                return i[u].exports
            }
            for (var a = typeof En == "function" && En, l = 0; l < s.length; l++)
                o(s[l]);
            return o
        }({
            1: [function(r, n, i) {
                (function(s, o, a, l, u, c, g, d, m) {
                    var v = r("crypto");
                    function p(T, I) {
                        return function(h, y) {
                            var w;
                            if (w = y.algorithm !== "passthrough" ? v.createHash(y.algorithm) : new R,
                            w.write === void 0 && (w.write = w.update,
                            w.end = w.update),
                            E(y, w).dispatch(h),
                            w.update || w.end(""),
                            w.digest)
                                return w.digest(y.encoding === "buffer" ? void 0 : y.encoding);
                            var O = w.read();
                            return y.encoding !== "buffer" ? O.toString(y.encoding) : O
                        }(T, I = _(T, I))
                    }
                    (i = n.exports = p).sha1 = function(T) {
                        return p(T)
                    }
                    ,
                    i.keys = function(T) {
                        return p(T, {
                            excludeValues: !0,
                            algorithm: "sha1",
                            encoding: "hex"
                        })
                    }
                    ,
                    i.MD5 = function(T) {
                        return p(T, {
                            algorithm: "md5",
                            encoding: "hex"
                        })
                    }
                    ,
                    i.keysMD5 = function(T) {
                        return p(T, {
                            algorithm: "md5",
                            encoding: "hex",
                            excludeValues: !0
                        })
                    }
                    ;
                    var f = v.getHashes ? v.getHashes().slice() : ["sha1", "md5"];
                    f.push("passthrough");
                    var b = ["buffer", "hex", "binary", "base64"];
                    function _(T, I) {
                        I = I || {};
                        var h = {};
                        if (h.algorithm = I.algorithm || "sha1",
                        h.encoding = I.encoding || "hex",
                        h.excludeValues = !!I.excludeValues,
                        h.algorithm = h.algorithm.toLowerCase(),
                        h.encoding = h.encoding.toLowerCase(),
                        h.ignoreUnknown = I.ignoreUnknown === !0,
                        h.respectType = I.respectType !== !1,
                        h.respectFunctionNames = I.respectFunctionNames !== !1,
                        h.respectFunctionProperties = I.respectFunctionProperties !== !1,
                        h.unorderedArrays = I.unorderedArrays === !0,
                        h.unorderedSets = I.unorderedSets !== !1,
                        h.unorderedObjects = I.unorderedObjects !== !1,
                        h.replacer = I.replacer || void 0,
                        h.excludeKeys = I.excludeKeys || void 0,
                        T === void 0)
                            throw new Error("Object argument required.");
                        for (var y = 0; y < f.length; ++y)
                            f[y].toLowerCase() === h.algorithm.toLowerCase() && (h.algorithm = f[y]);
                        if (f.indexOf(h.algorithm) === -1)
                            throw new Error('Algorithm "' + h.algorithm + '"  not supported. supported values: ' + f.join(", "));
                        if (b.indexOf(h.encoding) === -1 && h.algorithm !== "passthrough")
                            throw new Error('Encoding "' + h.encoding + '"  not supported. supported values: ' + b.join(", "));
                        return h
                    }
                    function C(T) {
                        if (typeof T == "function")
                            return /^function\s+\w*\s*\(\s*\)\s*{\s+\[native code\]\s+}$/i.exec(Function.prototype.toString.call(T)) != null
                    }
                    function E(T, I, h) {
                        h = h || [];
                        function y(w) {
                            return I.update ? I.update(w, "utf8") : I.write(w, "utf8")
                        }
                        return {
                            dispatch: function(w) {
                                return T.replacer && (w = T.replacer(w)),
                                this["_" + (w === null ? "null" : typeof w)](w)
                            },
                            _object: function(w) {
                                var O = Object.prototype.toString.call(w)
                                  , A = /\[object (.*)\]/i.exec(O);
                                A = (A = A ? A[1] : "unknown:[" + O + "]").toLowerCase();
                                var k;
                                if (0 <= (k = h.indexOf(w)))
                                    return this.dispatch("[CIRCULAR:" + k + "]");
                                if (h.push(w),
                                a !== void 0 && a.isBuffer && a.isBuffer(w))
                                    return y("buffer:"),
                                    y(w);
                                if (A === "object" || A === "function" || A === "asyncfunction") {
                                    var V = Object.keys(w);
                                    T.unorderedObjects && (V = V.sort()),
                                    T.respectType === !1 || C(w) || V.splice(0, 0, "prototype", "__proto__", "constructor"),
                                    T.excludeKeys && (V = V.filter(function(D) {
                                        return !T.excludeKeys(D)
                                    })),
                                    y("object:" + V.length + ":");
                                    var $ = this;
                                    return V.forEach(function(D) {
                                        $.dispatch(D),
                                        y(":"),
                                        T.excludeValues || $.dispatch(w[D]),
                                        y(",")
                                    })
                                }
                                if (!this["_" + A]) {
                                    if (T.ignoreUnknown)
                                        return y("[" + A + "]");
                                    throw new Error('Unknown object type "' + A + '"')
                                }
                                this["_" + A](w)
                            },
                            _array: function(w, O) {
                                O = O !== void 0 ? O : T.unorderedArrays !== !1;
                                var A = this;
                                if (y("array:" + w.length + ":"),
                                !O || w.length <= 1)
                                    return w.forEach(function($) {
                                        return A.dispatch($)
                                    });
                                var k = []
                                  , V = w.map(function($) {
                                    var D = new R
                                      , Y = h.slice();
                                    return E(T, D, Y).dispatch($),
                                    k = k.concat(Y.slice(h.length)),
                                    D.read().toString()
                                });
                                return h = h.concat(k),
                                V.sort(),
                                this._array(V, !1)
                            },
                            _date: function(w) {
                                return y("date:" + w.toJSON())
                            },
                            _symbol: function(w) {
                                return y("symbol:" + w.toString())
                            },
                            _error: function(w) {
                                return y("error:" + w.toString())
                            },
                            _boolean: function(w) {
                                return y("bool:" + w.toString())
                            },
                            _string: function(w) {
                                y("string:" + w.length + ":"),
                                y(w.toString())
                            },
                            _function: function(w) {
                                y("fn:"),
                                C(w) ? this.dispatch("[native]") : this.dispatch(w.toString()),
                                T.respectFunctionNames !== !1 && this.dispatch("function-name:" + String(w.name)),
                                T.respectFunctionProperties && this._object(w)
                            },
                            _number: function(w) {
                                return y("number:" + w.toString())
                            },
                            _xml: function(w) {
                                return y("xml:" + w.toString())
                            },
                            _null: function() {
                                return y("Null")
                            },
                            _undefined: function() {
                                return y("Undefined")
                            },
                            _regexp: function(w) {
                                return y("regex:" + w.toString())
                            },
                            _uint8array: function(w) {
                                return y("uint8array:"),
                                this.dispatch(Array.prototype.slice.call(w))
                            },
                            _uint8clampedarray: function(w) {
                                return y("uint8clampedarray:"),
                                this.dispatch(Array.prototype.slice.call(w))
                            },
                            _int8array: function(w) {
                                return y("uint8array:"),
                                this.dispatch(Array.prototype.slice.call(w))
                            },
                            _uint16array: function(w) {
                                return y("uint16array:"),
                                this.dispatch(Array.prototype.slice.call(w))
                            },
                            _int16array: function(w) {
                                return y("uint16array:"),
                                this.dispatch(Array.prototype.slice.call(w))
                            },
                            _uint32array: function(w) {
                                return y("uint32array:"),
                                this.dispatch(Array.prototype.slice.call(w))
                            },
                            _int32array: function(w) {
                                return y("uint32array:"),
                                this.dispatch(Array.prototype.slice.call(w))
                            },
                            _float32array: function(w) {
                                return y("float32array:"),
                                this.dispatch(Array.prototype.slice.call(w))
                            },
                            _float64array: function(w) {
                                return y("float64array:"),
                                this.dispatch(Array.prototype.slice.call(w))
                            },
                            _arraybuffer: function(w) {
                                return y("arraybuffer:"),
                                this.dispatch(new Uint8Array(w))
                            },
                            _url: function(w) {
                                return y("url:" + w.toString())
                            },
                            _map: function(w) {
                                y("map:");
                                var O = Array.from(w);
                                return this._array(O, T.unorderedSets !== !1)
                            },
                            _set: function(w) {
                                y("set:");
                                var O = Array.from(w);
                                return this._array(O, T.unorderedSets !== !1)
                            },
                            _file: function(w) {
                                return y("file:"),
                                this.dispatch([w.name, w.size, w.type, w.lastModfied])
                            },
                            _blob: function() {
                                if (T.ignoreUnknown)
                                    return y("[blob]");
                                throw Error(`Hashing Blob objects is currently not supported
(see https://github.com/puleos/object-hash/issues/26)
Use "options.replacer" or "options.ignoreUnknown"
`)
                            },
                            _domwindow: function() {
                                return y("domwindow")
                            },
                            _bigint: function(w) {
                                return y("bigint:" + w.toString())
                            },
                            _process: function() {
                                return y("process")
                            },
                            _timer: function() {
                                return y("timer")
                            },
                            _pipe: function() {
                                return y("pipe")
                            },
                            _tcp: function() {
                                return y("tcp")
                            },
                            _udp: function() {
                                return y("udp")
                            },
                            _tty: function() {
                                return y("tty")
                            },
                            _statwatcher: function() {
                                return y("statwatcher")
                            },
                            _securecontext: function() {
                                return y("securecontext")
                            },
                            _connection: function() {
                                return y("connection")
                            },
                            _zlib: function() {
                                return y("zlib")
                            },
                            _context: function() {
                                return y("context")
                            },
                            _nodescript: function() {
                                return y("nodescript")
                            },
                            _httpparser: function() {
                                return y("httpparser")
                            },
                            _dataview: function() {
                                return y("dataview")
                            },
                            _signal: function() {
                                return y("signal")
                            },
                            _fsevent: function() {
                                return y("fsevent")
                            },
                            _tlswrap: function() {
                                return y("tlswrap")
                            }
                        }
                    }
                    function R() {
                        return {
                            buf: "",
                            write: function(T) {
                                this.buf += T
                            },
                            end: function(T) {
                                this.buf += T
                            },
                            read: function() {
                                return this.buf
                            }
                        }
                    }
                    i.writeToStream = function(T, I, h) {
                        return h === void 0 && (h = I,
                        I = {}),
                        E(I = _(T, I), h).dispatch(T)
                    }
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/fake_7eac155c.js", "/")
            }
            , {
                buffer: 3,
                crypto: 5,
                lYpoI2: 10
            }],
            2: [function(r, n, i) {
                (function(s, o, a, l, u, c, g, d, m) {
                    (function(v) {
                        var p = typeof Uint8Array < "u" ? Uint8Array : Array
                          , f = "+".charCodeAt(0)
                          , b = "/".charCodeAt(0)
                          , _ = "0".charCodeAt(0)
                          , C = "a".charCodeAt(0)
                          , E = "A".charCodeAt(0)
                          , R = "-".charCodeAt(0)
                          , T = "_".charCodeAt(0);
                        function I(h) {
                            var y = h.charCodeAt(0);
                            return y === f || y === R ? 62 : y === b || y === T ? 63 : y < _ ? -1 : y < _ + 10 ? y - _ + 26 + 26 : y < E + 26 ? y - E : y < C + 26 ? y - C + 26 : void 0
                        }
                        v.toByteArray = function(h) {
                            var y, w;
                            if (0 < h.length % 4)
                                throw new Error("Invalid string. Length must be a multiple of 4");
                            var O = h.length
                              , A = h.charAt(O - 2) === "=" ? 2 : h.charAt(O - 1) === "=" ? 1 : 0
                              , k = new p(3 * h.length / 4 - A)
                              , V = 0 < A ? h.length - 4 : h.length
                              , $ = 0;
                            function D(Y) {
                                k[$++] = Y
                            }
                            for (y = 0; y < V; y += 4,
                            0)
                                D((16711680 & (w = I(h.charAt(y)) << 18 | I(h.charAt(y + 1)) << 12 | I(h.charAt(y + 2)) << 6 | I(h.charAt(y + 3)))) >> 16),
                                D((65280 & w) >> 8),
                                D(255 & w);
                            return A == 2 ? D(255 & (w = I(h.charAt(y)) << 2 | I(h.charAt(y + 1)) >> 4)) : A == 1 && (D((w = I(h.charAt(y)) << 10 | I(h.charAt(y + 1)) << 4 | I(h.charAt(y + 2)) >> 2) >> 8 & 255),
                            D(255 & w)),
                            k
                        }
                        ,
                        v.fromByteArray = function(h) {
                            var y, w, O, A, k = h.length % 3, V = "";
                            function $(D) {
                                return "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/".charAt(D)
                            }
                            for (y = 0,
                            O = h.length - k; y < O; y += 3)
                                w = (h[y] << 16) + (h[y + 1] << 8) + h[y + 2],
                                V += $((A = w) >> 18 & 63) + $(A >> 12 & 63) + $(A >> 6 & 63) + $(63 & A);
                            switch (k) {
                            case 1:
                                V += $((w = h[h.length - 1]) >> 2),
                                V += $(w << 4 & 63),
                                V += "==";
                                break;
                            case 2:
                                V += $((w = (h[h.length - 2] << 8) + h[h.length - 1]) >> 10),
                                V += $(w >> 4 & 63),
                                V += $(w << 2 & 63),
                                V += "="
                            }
                            return V
                        }
                    }
                    )(i === void 0 ? this.base64js = {} : i)
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/gulp-browserify/node_modules/base64-js/lib/b64.js", "/node_modules/gulp-browserify/node_modules/base64-js/lib")
            }
            , {
                buffer: 3,
                lYpoI2: 10
            }],
            3: [function(r, n, i) {
                (function(s, o, f, l, u, c, g, d, m) {
                    var v = r("base64-js")
                      , p = r("ieee754");
                    function f(x, P, L) {
                        if (!(this instanceof f))
                            return new f(x,P,L);
                        var q, S, M, N, F, B = typeof x;
                        if (P === "base64" && B == "string")
                            for (x = (q = x).trim ? q.trim() : q.replace(/^\s+|\s+$/g, ""); x.length % 4 != 0; )
                                x += "=";
                        if (B == "number")
                            S = H(x);
                        else if (B == "string")
                            S = f.byteLength(x, P);
                        else {
                            if (B != "object")
                                throw new Error("First argument needs to be a number, array or string.");
                            S = H(x.length)
                        }
                        if (f._useTypedArrays ? M = f._augment(new Uint8Array(S)) : ((M = this).length = S,
                        M._isBuffer = !0),
                        f._useTypedArrays && typeof x.byteLength == "number")
                            M._set(x);
                        else if (re(F = x) || f.isBuffer(F) || F && typeof F == "object" && typeof F.length == "number")
                            for (N = 0; N < S; N++)
                                f.isBuffer(x) ? M[N] = x.readUInt8(N) : M[N] = x[N];
                        else if (B == "string")
                            M.write(x, 0, P);
                        else if (B == "number" && !f._useTypedArrays && !L)
                            for (N = 0; N < S; N++)
                                M[N] = 0;
                        return M
                    }
                    function b(x, P, L, q) {
                        return f._charsWritten = ve(function(S) {
                            for (var M = [], N = 0; N < S.length; N++)
                                M.push(255 & S.charCodeAt(N));
                            return M
                        }(P), x, L, q)
                    }
                    function _(x, P, L, q) {
                        return f._charsWritten = ve(function(S) {
                            for (var M, N, F, B = [], U = 0; U < S.length; U++)
                                M = S.charCodeAt(U),
                                N = M >> 8,
                                F = M % 256,
                                B.push(F),
                                B.push(N);
                            return B
                        }(P), x, L, q)
                    }
                    function C(x, P, L) {
                        var q = "";
                        L = Math.min(x.length, L);
                        for (var S = P; S < L; S++)
                            q += String.fromCharCode(x[S]);
                        return q
                    }
                    function E(x, P, L, q) {
                        q || (z(typeof L == "boolean", "missing or invalid endian"),
                        z(P != null, "missing offset"),
                        z(P + 1 < x.length, "Trying to read beyond buffer length"));
                        var S, M = x.length;
                        if (!(M <= P))
                            return L ? (S = x[P],
                            P + 1 < M && (S |= x[P + 1] << 8)) : (S = x[P] << 8,
                            P + 1 < M && (S |= x[P + 1])),
                            S
                    }
                    function R(x, P, L, q) {
                        q || (z(typeof L == "boolean", "missing or invalid endian"),
                        z(P != null, "missing offset"),
                        z(P + 3 < x.length, "Trying to read beyond buffer length"));
                        var S, M = x.length;
                        if (!(M <= P))
                            return L ? (P + 2 < M && (S = x[P + 2] << 16),
                            P + 1 < M && (S |= x[P + 1] << 8),
                            S |= x[P],
                            P + 3 < M && (S += x[P + 3] << 24 >>> 0)) : (P + 1 < M && (S = x[P + 1] << 16),
                            P + 2 < M && (S |= x[P + 2] << 8),
                            P + 3 < M && (S |= x[P + 3]),
                            S += x[P] << 24 >>> 0),
                            S
                    }
                    function T(x, P, L, q) {
                        if (q || (z(typeof L == "boolean", "missing or invalid endian"),
                        z(P != null, "missing offset"),
                        z(P + 1 < x.length, "Trying to read beyond buffer length")),
                        !(x.length <= P)) {
                            var S = E(x, P, L, !0);
                            return 32768 & S ? -1 * (65535 - S + 1) : S
                        }
                    }
                    function I(x, P, L, q) {
                        if (q || (z(typeof L == "boolean", "missing or invalid endian"),
                        z(P != null, "missing offset"),
                        z(P + 3 < x.length, "Trying to read beyond buffer length")),
                        !(x.length <= P)) {
                            var S = R(x, P, L, !0);
                            return 2147483648 & S ? -1 * (4294967295 - S + 1) : S
                        }
                    }
                    function h(x, P, L, q) {
                        return q || (z(typeof L == "boolean", "missing or invalid endian"),
                        z(P + 3 < x.length, "Trying to read beyond buffer length")),
                        p.read(x, P, L, 23, 4)
                    }
                    function y(x, P, L, q) {
                        return q || (z(typeof L == "boolean", "missing or invalid endian"),
                        z(P + 7 < x.length, "Trying to read beyond buffer length")),
                        p.read(x, P, L, 52, 8)
                    }
                    function w(x, P, L, q, S) {
                        S || (z(P != null, "missing value"),
                        z(typeof q == "boolean", "missing or invalid endian"),
                        z(L != null, "missing offset"),
                        z(L + 1 < x.length, "trying to write beyond buffer length"),
                        ae(P, 65535));
                        var M = x.length;
                        if (!(M <= L))
                            for (var N = 0, F = Math.min(M - L, 2); N < F; N++)
                                x[L + N] = (P & 255 << 8 * (q ? N : 1 - N)) >>> 8 * (q ? N : 1 - N)
                    }
                    function O(x, P, L, q, S) {
                        S || (z(P != null, "missing value"),
                        z(typeof q == "boolean", "missing or invalid endian"),
                        z(L != null, "missing offset"),
                        z(L + 3 < x.length, "trying to write beyond buffer length"),
                        ae(P, 4294967295));
                        var M = x.length;
                        if (!(M <= L))
                            for (var N = 0, F = Math.min(M - L, 4); N < F; N++)
                                x[L + N] = P >>> 8 * (q ? N : 3 - N) & 255
                    }
                    function A(x, P, L, q, S) {
                        S || (z(P != null, "missing value"),
                        z(typeof q == "boolean", "missing or invalid endian"),
                        z(L != null, "missing offset"),
                        z(L + 1 < x.length, "Trying to write beyond buffer length"),
                        ne(P, 32767, -32768)),
                        x.length <= L || w(x, 0 <= P ? P : 65535 + P + 1, L, q, S)
                    }
                    function k(x, P, L, q, S) {
                        S || (z(P != null, "missing value"),
                        z(typeof q == "boolean", "missing or invalid endian"),
                        z(L != null, "missing offset"),
                        z(L + 3 < x.length, "Trying to write beyond buffer length"),
                        ne(P, 2147483647, -2147483648)),
                        x.length <= L || O(x, 0 <= P ? P : 4294967295 + P + 1, L, q, S)
                    }
                    function V(x, P, L, q, S) {
                        S || (z(P != null, "missing value"),
                        z(typeof q == "boolean", "missing or invalid endian"),
                        z(L != null, "missing offset"),
                        z(L + 3 < x.length, "Trying to write beyond buffer length"),
                        Ye(P, 34028234663852886e22, -34028234663852886e22)),
                        x.length <= L || p.write(x, P, L, q, 23, 4)
                    }
                    function $(x, P, L, q, S) {
                        S || (z(P != null, "missing value"),
                        z(typeof q == "boolean", "missing or invalid endian"),
                        z(L != null, "missing offset"),
                        z(L + 7 < x.length, "Trying to write beyond buffer length"),
                        Ye(P, 17976931348623157e292, -17976931348623157e292)),
                        x.length <= L || p.write(x, P, L, q, 52, 8)
                    }
                    i.Buffer = f,
                    i.SlowBuffer = f,
                    i.INSPECT_MAX_BYTES = 50,
                    f.poolSize = 8192,
                    f._useTypedArrays = function() {
                        try {
                            var x = new ArrayBuffer(0)
                              , P = new Uint8Array(x);
                            return P.foo = function() {
                                return 42
                            }
                            ,
                            P.foo() === 42 && typeof P.subarray == "function"
                        } catch {
                            return !1
                        }
                    }(),
                    f.isEncoding = function(x) {
                        switch (String(x).toLowerCase()) {
                        case "hex":
                        case "utf8":
                        case "utf-8":
                        case "ascii":
                        case "binary":
                        case "base64":
                        case "raw":
                        case "ucs2":
                        case "ucs-2":
                        case "utf16le":
                        case "utf-16le":
                            return !0;
                        default:
                            return !1
                        }
                    }
                    ,
                    f.isBuffer = function(x) {
                        return !(x == null || !x._isBuffer)
                    }
                    ,
                    f.byteLength = function(x, P) {
                        var L;
                        switch (x += "",
                        P || "utf8") {
                        case "hex":
                            L = x.length / 2;
                            break;
                        case "utf8":
                        case "utf-8":
                            L = Z(x).length;
                            break;
                        case "ascii":
                        case "binary":
                        case "raw":
                            L = x.length;
                            break;
                        case "base64":
                            L = se(x).length;
                            break;
                        case "ucs2":
                        case "ucs-2":
                        case "utf16le":
                        case "utf-16le":
                            L = 2 * x.length;
                            break;
                        default:
                            throw new Error("Unknown encoding")
                        }
                        return L
                    }
                    ,
                    f.concat = function(x, P) {
                        if (z(re(x), `Usage: Buffer.concat(list, [totalLength])
list should be an Array.`),
                        x.length === 0)
                            return new f(0);
                        if (x.length === 1)
                            return x[0];
                        if (typeof P != "number")
                            for (S = P = 0; S < x.length; S++)
                                P += x[S].length;
                        for (var L = new f(P), q = 0, S = 0; S < x.length; S++) {
                            var M = x[S];
                            M.copy(L, q),
                            q += M.length
                        }
                        return L
                    }
                    ,
                    f.prototype.write = function(x, P, L, q) {
                        var S;
                        isFinite(P) ? isFinite(L) || (q = L,
                        L = void 0) : (S = q,
                        q = P,
                        P = L,
                        L = S),
                        P = Number(P) || 0;
                        var M, N, F, B, U, G, W, K, j, X = this.length - P;
                        switch ((!L || X < (L = Number(L))) && (L = X),
                        q = String(q || "utf8").toLowerCase()) {
                        case "hex":
                            M = function(J, Q, ee, oe) {
                                ee = Number(ee) || 0;
                                var me = J.length - ee;
                                (!oe || me < (oe = Number(oe))) && (oe = me);
                                var ce = Q.length;
                                z(ce % 2 == 0, "Invalid hex string"),
                                ce / 2 < oe && (oe = ce / 2);
                                for (var Ce = 0; Ce < oe; Ce++) {
                                    var Re = parseInt(Q.substr(2 * Ce, 2), 16);
                                    z(!isNaN(Re), "Invalid hex string"),
                                    J[ee + Ce] = Re
                                }
                                return f._charsWritten = 2 * Ce,
                                Ce
                            }(this, x, P, L);
                            break;
                        case "utf8":
                        case "utf-8":
                            G = this,
                            W = x,
                            K = P,
                            j = L,
                            M = f._charsWritten = ve(Z(W), G, K, j);
                            break;
                        case "ascii":
                        case "binary":
                            M = b(this, x, P, L);
                            break;
                        case "base64":
                            N = this,
                            F = x,
                            B = P,
                            U = L,
                            M = f._charsWritten = ve(se(F), N, B, U);
                            break;
                        case "ucs2":
                        case "ucs-2":
                        case "utf16le":
                        case "utf-16le":
                            M = _(this, x, P, L);
                            break;
                        default:
                            throw new Error("Unknown encoding")
                        }
                        return M
                    }
                    ,
                    f.prototype.toString = function(x, P, L) {
                        var q, S, M, N, F = this;
                        if (x = String(x || "utf8").toLowerCase(),
                        P = Number(P) || 0,
                        (L = L !== void 0 ? Number(L) : L = F.length) === P)
                            return "";
                        switch (x) {
                        case "hex":
                            q = function(B, U, G) {
                                var W = B.length;
                                (!U || U < 0) && (U = 0),
                                (!G || G < 0 || W < G) && (G = W);
                                for (var K = "", j = U; j < G; j++)
                                    K += he(B[j]);
                                return K
                            }(F, P, L);
                            break;
                        case "utf8":
                        case "utf-8":
                            q = function(B, U, G) {
                                var W = ""
                                  , K = "";
                                G = Math.min(B.length, G);
                                for (var j = U; j < G; j++)
                                    B[j] <= 127 ? (W += Ae(K) + String.fromCharCode(B[j]),
                                    K = "") : K += "%" + B[j].toString(16);
                                return W + Ae(K)
                            }(F, P, L);
                            break;
                        case "ascii":
                        case "binary":
                            q = C(F, P, L);
                            break;
                        case "base64":
                            S = F,
                            N = L,
                            q = (M = P) === 0 && N === S.length ? v.fromByteArray(S) : v.fromByteArray(S.slice(M, N));
                            break;
                        case "ucs2":
                        case "ucs-2":
                        case "utf16le":
                        case "utf-16le":
                            q = function(B, U, G) {
                                for (var W = B.slice(U, G), K = "", j = 0; j < W.length; j += 2)
                                    K += String.fromCharCode(W[j] + 256 * W[j + 1]);
                                return K
                            }(F, P, L);
                            break;
                        default:
                            throw new Error("Unknown encoding")
                        }
                        return q
                    }
                    ,
                    f.prototype.toJSON = function() {
                        return {
                            type: "Buffer",
                            data: Array.prototype.slice.call(this._arr || this, 0)
                        }
                    }
                    ,
                    f.prototype.copy = function(x, P, L, q) {
                        if (L = L || 0,
                        q || q === 0 || (q = this.length),
                        P = P || 0,
                        q !== L && x.length !== 0 && this.length !== 0) {
                            z(L <= q, "sourceEnd < sourceStart"),
                            z(0 <= P && P < x.length, "targetStart out of bounds"),
                            z(0 <= L && L < this.length, "sourceStart out of bounds"),
                            z(0 <= q && q <= this.length, "sourceEnd out of bounds"),
                            q > this.length && (q = this.length),
                            x.length - P < q - L && (q = x.length - P + L);
                            var S = q - L;
                            if (S < 100 || !f._useTypedArrays)
                                for (var M = 0; M < S; M++)
                                    x[M + P] = this[M + L];
                            else
                                x._set(this.subarray(L, L + S), P)
                        }
                    }
                    ,
                    f.prototype.slice = function(x, P) {
                        var L = this.length;
                        if (x = Y(x, L, 0),
                        P = Y(P, L, L),
                        f._useTypedArrays)
                            return f._augment(this.subarray(x, P));
                        for (var q = P - x, S = new f(q,void 0,!0), M = 0; M < q; M++)
                            S[M] = this[M + x];
                        return S
                    }
                    ,
                    f.prototype.get = function(x) {
                        return this.readUInt8(x)
                    }
                    ,
                    f.prototype.set = function(x, P) {
                        return this.writeUInt8(x, P)
                    }
                    ,
                    f.prototype.readUInt8 = function(x, P) {
                        if (P || (z(x != null, "missing offset"),
                        z(x < this.length, "Trying to read beyond buffer length")),
                        !(x >= this.length))
                            return this[x]
                    }
                    ,
                    f.prototype.readUInt16LE = function(x, P) {
                        return E(this, x, !0, P)
                    }
                    ,
                    f.prototype.readUInt16BE = function(x, P) {
                        return E(this, x, !1, P)
                    }
                    ,
                    f.prototype.readUInt32LE = function(x, P) {
                        return R(this, x, !0, P)
                    }
                    ,
                    f.prototype.readUInt32BE = function(x, P) {
                        return R(this, x, !1, P)
                    }
                    ,
                    f.prototype.readInt8 = function(x, P) {
                        if (P || (z(x != null, "missing offset"),
                        z(x < this.length, "Trying to read beyond buffer length")),
                        !(x >= this.length))
                            return 128 & this[x] ? -1 * (255 - this[x] + 1) : this[x]
                    }
                    ,
                    f.prototype.readInt16LE = function(x, P) {
                        return T(this, x, !0, P)
                    }
                    ,
                    f.prototype.readInt16BE = function(x, P) {
                        return T(this, x, !1, P)
                    }
                    ,
                    f.prototype.readInt32LE = function(x, P) {
                        return I(this, x, !0, P)
                    }
                    ,
                    f.prototype.readInt32BE = function(x, P) {
                        return I(this, x, !1, P)
                    }
                    ,
                    f.prototype.readFloatLE = function(x, P) {
                        return h(this, x, !0, P)
                    }
                    ,
                    f.prototype.readFloatBE = function(x, P) {
                        return h(this, x, !1, P)
                    }
                    ,
                    f.prototype.readDoubleLE = function(x, P) {
                        return y(this, x, !0, P)
                    }
                    ,
                    f.prototype.readDoubleBE = function(x, P) {
                        return y(this, x, !1, P)
                    }
                    ,
                    f.prototype.writeUInt8 = function(x, P, L) {
                        L || (z(x != null, "missing value"),
                        z(P != null, "missing offset"),
                        z(P < this.length, "trying to write beyond buffer length"),
                        ae(x, 255)),
                        P >= this.length || (this[P] = x)
                    }
                    ,
                    f.prototype.writeUInt16LE = function(x, P, L) {
                        w(this, x, P, !0, L)
                    }
                    ,
                    f.prototype.writeUInt16BE = function(x, P, L) {
                        w(this, x, P, !1, L)
                    }
                    ,
                    f.prototype.writeUInt32LE = function(x, P, L) {
                        O(this, x, P, !0, L)
                    }
                    ,
                    f.prototype.writeUInt32BE = function(x, P, L) {
                        O(this, x, P, !1, L)
                    }
                    ,
                    f.prototype.writeInt8 = function(x, P, L) {
                        L || (z(x != null, "missing value"),
                        z(P != null, "missing offset"),
                        z(P < this.length, "Trying to write beyond buffer length"),
                        ne(x, 127, -128)),
                        P >= this.length || (0 <= x ? this.writeUInt8(x, P, L) : this.writeUInt8(255 + x + 1, P, L))
                    }
                    ,
                    f.prototype.writeInt16LE = function(x, P, L) {
                        A(this, x, P, !0, L)
                    }
                    ,
                    f.prototype.writeInt16BE = function(x, P, L) {
                        A(this, x, P, !1, L)
                    }
                    ,
                    f.prototype.writeInt32LE = function(x, P, L) {
                        k(this, x, P, !0, L)
                    }
                    ,
                    f.prototype.writeInt32BE = function(x, P, L) {
                        k(this, x, P, !1, L)
                    }
                    ,
                    f.prototype.writeFloatLE = function(x, P, L) {
                        V(this, x, P, !0, L)
                    }
                    ,
                    f.prototype.writeFloatBE = function(x, P, L) {
                        V(this, x, P, !1, L)
                    }
                    ,
                    f.prototype.writeDoubleLE = function(x, P, L) {
                        $(this, x, P, !0, L)
                    }
                    ,
                    f.prototype.writeDoubleBE = function(x, P, L) {
                        $(this, x, P, !1, L)
                    }
                    ,
                    f.prototype.fill = function(x, P, L) {
                        if (x = x || 0,
                        P = P || 0,
                        L = L || this.length,
                        typeof x == "string" && (x = x.charCodeAt(0)),
                        z(typeof x == "number" && !isNaN(x), "value is not a number"),
                        z(P <= L, "end < start"),
                        L !== P && this.length !== 0) {
                            z(0 <= P && P < this.length, "start out of bounds"),
                            z(0 <= L && L <= this.length, "end out of bounds");
                            for (var q = P; q < L; q++)
                                this[q] = x
                        }
                    }
                    ,
                    f.prototype.inspect = function() {
                        for (var x = [], P = this.length, L = 0; L < P; L++)
                            if (x[L] = he(this[L]),
                            L === i.INSPECT_MAX_BYTES) {
                                x[L + 1] = "...";
                                break
                            }
                        return "<Buffer " + x.join(" ") + ">"
                    }
                    ,
                    f.prototype.toArrayBuffer = function() {
                        if (typeof Uint8Array > "u")
                            throw new Error("Buffer.toArrayBuffer not supported in this browser");
                        if (f._useTypedArrays)
                            return new f(this).buffer;
                        for (var x = new Uint8Array(this.length), P = 0, L = x.length; P < L; P += 1)
                            x[P] = this[P];
                        return x.buffer
                    }
                    ;
                    var D = f.prototype;
                    function Y(x, P, L) {
                        return typeof x != "number" ? L : P <= (x = ~~x) ? P : 0 <= x || 0 <= (x += P) ? x : 0
                    }
                    function H(x) {
                        return (x = ~~Math.ceil(+x)) < 0 ? 0 : x
                    }
                    function re(x) {
                        return (Array.isArray || function(P) {
                            return Object.prototype.toString.call(P) === "[object Array]"
                        }
                        )(x)
                    }
                    function he(x) {
                        return x < 16 ? "0" + x.toString(16) : x.toString(16)
                    }
                    function Z(x) {
                        for (var P = [], L = 0; L < x.length; L++) {
                            var q = x.charCodeAt(L);
                            if (q <= 127)
                                P.push(x.charCodeAt(L));
                            else {
                                var S = L;
                                55296 <= q && q <= 57343 && L++;
                                for (var M = encodeURIComponent(x.slice(S, L + 1)).substr(1).split("%"), N = 0; N < M.length; N++)
                                    P.push(parseInt(M[N], 16))
                            }
                        }
                        return P
                    }
                    function se(x) {
                        return v.toByteArray(x)
                    }
                    function ve(x, P, L, q) {
                        for (var S = 0; S < q && !(S + L >= P.length || S >= x.length); S++)
                            P[S + L] = x[S];
                        return S
                    }
                    function Ae(x) {
                        try {
                            return decodeURIComponent(x)
                        } catch {
                            return String.fromCharCode(65533)
                        }
                    }
                    function ae(x, P) {
                        z(typeof x == "number", "cannot write a non-number as a number"),
                        z(0 <= x, "specified a negative value for writing an unsigned value"),
                        z(x <= P, "value is larger than maximum value for type"),
                        z(Math.floor(x) === x, "value has a fractional component")
                    }
                    function ne(x, P, L) {
                        z(typeof x == "number", "cannot write a non-number as a number"),
                        z(x <= P, "value larger than maximum allowed value"),
                        z(L <= x, "value smaller than minimum allowed value"),
                        z(Math.floor(x) === x, "value has a fractional component")
                    }
                    function Ye(x, P, L) {
                        z(typeof x == "number", "cannot write a non-number as a number"),
                        z(x <= P, "value larger than maximum allowed value"),
                        z(L <= x, "value smaller than minimum allowed value")
                    }
                    function z(x, P) {
                        if (!x)
                            throw new Error(P || "Failed assertion")
                    }
                    f._augment = function(x) {
                        return x._isBuffer = !0,
                        x._get = x.get,
                        x._set = x.set,
                        x.get = D.get,
                        x.set = D.set,
                        x.write = D.write,
                        x.toString = D.toString,
                        x.toLocaleString = D.toString,
                        x.toJSON = D.toJSON,
                        x.copy = D.copy,
                        x.slice = D.slice,
                        x.readUInt8 = D.readUInt8,
                        x.readUInt16LE = D.readUInt16LE,
                        x.readUInt16BE = D.readUInt16BE,
                        x.readUInt32LE = D.readUInt32LE,
                        x.readUInt32BE = D.readUInt32BE,
                        x.readInt8 = D.readInt8,
                        x.readInt16LE = D.readInt16LE,
                        x.readInt16BE = D.readInt16BE,
                        x.readInt32LE = D.readInt32LE,
                        x.readInt32BE = D.readInt32BE,
                        x.readFloatLE = D.readFloatLE,
                        x.readFloatBE = D.readFloatBE,
                        x.readDoubleLE = D.readDoubleLE,
                        x.readDoubleBE = D.readDoubleBE,
                        x.writeUInt8 = D.writeUInt8,
                        x.writeUInt16LE = D.writeUInt16LE,
                        x.writeUInt16BE = D.writeUInt16BE,
                        x.writeUInt32LE = D.writeUInt32LE,
                        x.writeUInt32BE = D.writeUInt32BE,
                        x.writeInt8 = D.writeInt8,
                        x.writeInt16LE = D.writeInt16LE,
                        x.writeInt16BE = D.writeInt16BE,
                        x.writeInt32LE = D.writeInt32LE,
                        x.writeInt32BE = D.writeInt32BE,
                        x.writeFloatLE = D.writeFloatLE,
                        x.writeFloatBE = D.writeFloatBE,
                        x.writeDoubleLE = D.writeDoubleLE,
                        x.writeDoubleBE = D.writeDoubleBE,
                        x.fill = D.fill,
                        x.inspect = D.inspect,
                        x.toArrayBuffer = D.toArrayBuffer,
                        x
                    }
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/gulp-browserify/node_modules/buffer/index.js", "/node_modules/gulp-browserify/node_modules/buffer")
            }
            , {
                "base64-js": 2,
                buffer: 3,
                ieee754: 11,
                lYpoI2: 10
            }],
            4: [function(r, n, i) {
                (function(s, o, v, l, u, c, g, d, m) {
                    var v = r("buffer").Buffer
                      , p = 4
                      , f = new v(p);
                    f.fill(0),
                    n.exports = {
                        hash: function(b, _, C, E) {
                            return v.isBuffer(b) || (b = new v(b)),
                            function(R, T, I) {
                                for (var h = new v(T), y = I ? h.writeInt32BE : h.writeInt32LE, w = 0; w < R.length; w++)
                                    y.call(h, R[w], 4 * w, !0);
                                return h
                            }(_(function(R, T) {
                                var I;
                                R.length % p != 0 && (I = R.length + (p - R.length % p),
                                R = v.concat([R, f], I));
                                for (var h = [], y = T ? R.readInt32BE : R.readInt32LE, w = 0; w < R.length; w += p)
                                    h.push(y.call(R, w));
                                return h
                            }(b, E), 8 * b.length), C, E)
                        }
                    }
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/gulp-browserify/node_modules/crypto-browserify/helpers.js", "/node_modules/gulp-browserify/node_modules/crypto-browserify")
            }
            , {
                buffer: 3,
                lYpoI2: 10
            }],
            5: [function(r, n, i) {
                (function(s, o, v, l, u, c, g, d, m) {
                    var v = r("buffer").Buffer
                      , p = r("./sha")
                      , f = r("./sha256")
                      , b = r("./rng")
                      , _ = {
                        sha1: p,
                        sha256: f,
                        md5: r("./md5")
                    }
                      , C = 64
                      , E = new v(C);
                    function R(I, h) {
                        var y = _[I = I || "sha1"]
                          , w = [];
                        return y || T("algorithm:", I, "is not yet supported"),
                        {
                            update: function(O) {
                                return v.isBuffer(O) || (O = new v(O)),
                                w.push(O),
                                O.length,
                                this
                            },
                            digest: function(O) {
                                var A = v.concat(w)
                                  , k = h ? function(V, $, D) {
                                    v.isBuffer($) || ($ = new v($)),
                                    v.isBuffer(D) || (D = new v(D)),
                                    $.length > C ? $ = V($) : $.length < C && ($ = v.concat([$, E], C));
                                    for (var Y = new v(C), H = new v(C), re = 0; re < C; re++)
                                        Y[re] = 54 ^ $[re],
                                        H[re] = 92 ^ $[re];
                                    var he = V(v.concat([Y, D]));
                                    return V(v.concat([H, he]))
                                }(y, h, A) : y(A);
                                return w = null,
                                O ? k.toString(O) : k
                            }
                        }
                    }
                    function T() {
                        var I = [].slice.call(arguments).join(" ");
                        throw new Error([I, "we accept pull requests", "http://github.com/dominictarr/crypto-browserify"].join(`
`))
                    }
                    E.fill(0),
                    i.createHash = function(I) {
                        return R(I)
                    }
                    ,
                    i.createHmac = R,
                    i.randomBytes = function(I, h) {
                        if (!h || !h.call)
                            return new v(b(I));
                        try {
                            h.call(this, void 0, new v(b(I)))
                        } catch (y) {
                            h(y)
                        }
                    }
                    ,
                    function(I, h) {
                        for (var y in I)
                            h(I[y], y)
                    }(["createCredentials", "createCipher", "createCipheriv", "createDecipher", "createDecipheriv", "createSign", "createVerify", "createDiffieHellman", "pbkdf2"], function(I) {
                        i[I] = function() {
                            T("sorry,", I, "is not implemented yet")
                        }
                    })
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/gulp-browserify/node_modules/crypto-browserify/index.js", "/node_modules/gulp-browserify/node_modules/crypto-browserify")
            }
            , {
                "./md5": 6,
                "./rng": 7,
                "./sha": 8,
                "./sha256": 9,
                buffer: 3,
                lYpoI2: 10
            }],
            6: [function(r, n, i) {
                (function(s, o, a, l, u, c, g, d, m) {
                    var v = r("./helpers");
                    function p(T, I) {
                        T[I >> 5] |= 128 << I % 32,
                        T[14 + (I + 64 >>> 9 << 4)] = I;
                        for (var h = 1732584193, y = -271733879, w = -1732584194, O = 271733878, A = 0; A < T.length; A += 16) {
                            var k = h
                              , V = y
                              , $ = w
                              , D = O
                              , h = b(h, y, w, O, T[A + 0], 7, -680876936)
                              , O = b(O, h, y, w, T[A + 1], 12, -389564586)
                              , w = b(w, O, h, y, T[A + 2], 17, 606105819)
                              , y = b(y, w, O, h, T[A + 3], 22, -1044525330);
                            h = b(h, y, w, O, T[A + 4], 7, -176418897),
                            O = b(O, h, y, w, T[A + 5], 12, 1200080426),
                            w = b(w, O, h, y, T[A + 6], 17, -1473231341),
                            y = b(y, w, O, h, T[A + 7], 22, -45705983),
                            h = b(h, y, w, O, T[A + 8], 7, 1770035416),
                            O = b(O, h, y, w, T[A + 9], 12, -1958414417),
                            w = b(w, O, h, y, T[A + 10], 17, -42063),
                            y = b(y, w, O, h, T[A + 11], 22, -1990404162),
                            h = b(h, y, w, O, T[A + 12], 7, 1804603682),
                            O = b(O, h, y, w, T[A + 13], 12, -40341101),
                            w = b(w, O, h, y, T[A + 14], 17, -1502002290),
                            h = _(h, y = b(y, w, O, h, T[A + 15], 22, 1236535329), w, O, T[A + 1], 5, -165796510),
                            O = _(O, h, y, w, T[A + 6], 9, -1069501632),
                            w = _(w, O, h, y, T[A + 11], 14, 643717713),
                            y = _(y, w, O, h, T[A + 0], 20, -373897302),
                            h = _(h, y, w, O, T[A + 5], 5, -701558691),
                            O = _(O, h, y, w, T[A + 10], 9, 38016083),
                            w = _(w, O, h, y, T[A + 15], 14, -660478335),
                            y = _(y, w, O, h, T[A + 4], 20, -405537848),
                            h = _(h, y, w, O, T[A + 9], 5, 568446438),
                            O = _(O, h, y, w, T[A + 14], 9, -1019803690),
                            w = _(w, O, h, y, T[A + 3], 14, -187363961),
                            y = _(y, w, O, h, T[A + 8], 20, 1163531501),
                            h = _(h, y, w, O, T[A + 13], 5, -1444681467),
                            O = _(O, h, y, w, T[A + 2], 9, -51403784),
                            w = _(w, O, h, y, T[A + 7], 14, 1735328473),
                            h = C(h, y = _(y, w, O, h, T[A + 12], 20, -1926607734), w, O, T[A + 5], 4, -378558),
                            O = C(O, h, y, w, T[A + 8], 11, -2022574463),
                            w = C(w, O, h, y, T[A + 11], 16, 1839030562),
                            y = C(y, w, O, h, T[A + 14], 23, -35309556),
                            h = C(h, y, w, O, T[A + 1], 4, -1530992060),
                            O = C(O, h, y, w, T[A + 4], 11, 1272893353),
                            w = C(w, O, h, y, T[A + 7], 16, -155497632),
                            y = C(y, w, O, h, T[A + 10], 23, -1094730640),
                            h = C(h, y, w, O, T[A + 13], 4, 681279174),
                            O = C(O, h, y, w, T[A + 0], 11, -358537222),
                            w = C(w, O, h, y, T[A + 3], 16, -722521979),
                            y = C(y, w, O, h, T[A + 6], 23, 76029189),
                            h = C(h, y, w, O, T[A + 9], 4, -640364487),
                            O = C(O, h, y, w, T[A + 12], 11, -421815835),
                            w = C(w, O, h, y, T[A + 15], 16, 530742520),
                            h = E(h, y = C(y, w, O, h, T[A + 2], 23, -995338651), w, O, T[A + 0], 6, -198630844),
                            O = E(O, h, y, w, T[A + 7], 10, 1126891415),
                            w = E(w, O, h, y, T[A + 14], 15, -1416354905),
                            y = E(y, w, O, h, T[A + 5], 21, -57434055),
                            h = E(h, y, w, O, T[A + 12], 6, 1700485571),
                            O = E(O, h, y, w, T[A + 3], 10, -1894986606),
                            w = E(w, O, h, y, T[A + 10], 15, -1051523),
                            y = E(y, w, O, h, T[A + 1], 21, -2054922799),
                            h = E(h, y, w, O, T[A + 8], 6, 1873313359),
                            O = E(O, h, y, w, T[A + 15], 10, -30611744),
                            w = E(w, O, h, y, T[A + 6], 15, -1560198380),
                            y = E(y, w, O, h, T[A + 13], 21, 1309151649),
                            h = E(h, y, w, O, T[A + 4], 6, -145523070),
                            O = E(O, h, y, w, T[A + 11], 10, -1120210379),
                            w = E(w, O, h, y, T[A + 2], 15, 718787259),
                            y = E(y, w, O, h, T[A + 9], 21, -343485551),
                            h = R(h, k),
                            y = R(y, V),
                            w = R(w, $),
                            O = R(O, D)
                        }
                        return Array(h, y, w, O)
                    }
                    function f(T, I, h, y, w, O) {
                        return R((A = R(R(I, T), R(y, O))) << (k = w) | A >>> 32 - k, h);
                        var A, k
                    }
                    function b(T, I, h, y, w, O, A) {
                        return f(I & h | ~I & y, T, I, w, O, A)
                    }
                    function _(T, I, h, y, w, O, A) {
                        return f(I & y | h & ~y, T, I, w, O, A)
                    }
                    function C(T, I, h, y, w, O, A) {
                        return f(I ^ h ^ y, T, I, w, O, A)
                    }
                    function E(T, I, h, y, w, O, A) {
                        return f(h ^ (I | ~y), T, I, w, O, A)
                    }
                    function R(T, I) {
                        var h = (65535 & T) + (65535 & I);
                        return (T >> 16) + (I >> 16) + (h >> 16) << 16 | 65535 & h
                    }
                    n.exports = function(T) {
                        return v.hash(T, p, 16)
                    }
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/gulp-browserify/node_modules/crypto-browserify/md5.js", "/node_modules/gulp-browserify/node_modules/crypto-browserify")
            }
            , {
                "./helpers": 4,
                buffer: 3,
                lYpoI2: 10
            }],
            7: [function(r, n, i) {
                (function(s, o, a, l, u, c, g, d, m) {
                    var v;
                    v = function(p) {
                        for (var f, b = new Array(p), _ = 0; _ < p; _++)
                            (3 & _) == 0 && (f = 4294967296 * Math.random()),
                            b[_] = f >>> ((3 & _) << 3) & 255;
                        return b
                    }
                    ,
                    n.exports = v
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/gulp-browserify/node_modules/crypto-browserify/rng.js", "/node_modules/gulp-browserify/node_modules/crypto-browserify")
            }
            , {
                buffer: 3,
                lYpoI2: 10
            }],
            8: [function(r, n, i) {
                (function(s, o, a, l, u, c, g, d, m) {
                    var v = r("./helpers");
                    function p(_, C) {
                        _[C >> 5] |= 128 << 24 - C % 32,
                        _[15 + (C + 64 >> 9 << 4)] = C;
                        for (var E, R, T, I, h, y = Array(80), w = 1732584193, O = -271733879, A = -1732584194, k = 271733878, V = -1009589776, $ = 0; $ < _.length; $ += 16) {
                            for (var D = w, Y = O, H = A, re = k, he = V, Z = 0; Z < 80; Z++) {
                                y[Z] = Z < 16 ? _[$ + Z] : b(y[Z - 3] ^ y[Z - 8] ^ y[Z - 14] ^ y[Z - 16], 1);
                                var se = f(f(b(w, 5), (T = O,
                                I = A,
                                h = k,
                                (R = Z) < 20 ? T & I | ~T & h : !(R < 40) && R < 60 ? T & I | T & h | I & h : T ^ I ^ h)), f(f(V, y[Z]), (E = Z) < 20 ? 1518500249 : E < 40 ? 1859775393 : E < 60 ? -1894007588 : -899497514))
                                  , V = k
                                  , k = A
                                  , A = b(O, 30)
                                  , O = w
                                  , w = se
                            }
                            w = f(w, D),
                            O = f(O, Y),
                            A = f(A, H),
                            k = f(k, re),
                            V = f(V, he)
                        }
                        return Array(w, O, A, k, V)
                    }
                    function f(_, C) {
                        var E = (65535 & _) + (65535 & C);
                        return (_ >> 16) + (C >> 16) + (E >> 16) << 16 | 65535 & E
                    }
                    function b(_, C) {
                        return _ << C | _ >>> 32 - C
                    }
                    n.exports = function(_) {
                        return v.hash(_, p, 20, !0)
                    }
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/gulp-browserify/node_modules/crypto-browserify/sha.js", "/node_modules/gulp-browserify/node_modules/crypto-browserify")
            }
            , {
                "./helpers": 4,
                buffer: 3,
                lYpoI2: 10
            }],
            9: [function(r, n, i) {
                (function(s, o, a, l, u, c, g, d, m) {
                    function v(_, C) {
                        var E = (65535 & _) + (65535 & C);
                        return (_ >> 16) + (C >> 16) + (E >> 16) << 16 | 65535 & E
                    }
                    function p(_, C) {
                        return _ >>> C | _ << 32 - C
                    }
                    function f(_, C) {
                        var E, R, T, I, h, y, w, O, A, k, V = new Array(1116352408,1899447441,3049323471,3921009573,961987163,1508970993,2453635748,2870763221,3624381080,310598401,607225278,1426881987,1925078388,2162078206,2614888103,3248222580,3835390401,4022224774,264347078,604807628,770255983,1249150122,1555081692,1996064986,2554220882,2821834349,2952996808,3210313671,3336571891,3584528711,113926993,338241895,666307205,773529912,1294757372,1396182291,1695183700,1986661051,2177026350,2456956037,2730485921,2820302411,3259730800,3345764771,3516065817,3600352804,4094571909,275423344,430227734,506948616,659060556,883997877,958139571,1322822218,1537002063,1747873779,1955562222,2024104815,2227730452,2361852424,2428436474,2756734187,3204031479,3329325298), $ = new Array(1779033703,3144134277,1013904242,2773480762,1359893119,2600822924,528734635,1541459225), D = new Array(64);
                        _[C >> 5] |= 128 << 24 - C % 32,
                        _[15 + (C + 64 >> 9 << 4)] = C;
                        for (var Y, H, re, he, Z, se, ve, Ae, ae = 0; ae < _.length; ae += 16) {
                            E = $[0],
                            R = $[1],
                            T = $[2],
                            I = $[3],
                            h = $[4],
                            y = $[5],
                            w = $[6],
                            O = $[7];
                            for (var ne = 0; ne < 64; ne++)
                                D[ne] = ne < 16 ? _[ne + ae] : v(v(v((Ae = D[ne - 2],
                                p(Ae, 17) ^ p(Ae, 19) ^ Ae >>> 10), D[ne - 7]), (ve = D[ne - 15],
                                p(ve, 7) ^ p(ve, 18) ^ ve >>> 3)), D[ne - 16]),
                                A = v(v(v(v(O, p(se = h, 6) ^ p(se, 11) ^ p(se, 25)), (Z = h) & y ^ ~Z & w), V[ne]), D[ne]),
                                k = v(p(he = E, 2) ^ p(he, 13) ^ p(he, 22), (Y = E) & (H = R) ^ Y & (re = T) ^ H & re),
                                O = w,
                                w = y,
                                y = h,
                                h = v(I, A),
                                I = T,
                                T = R,
                                R = E,
                                E = v(A, k);
                            $[0] = v(E, $[0]),
                            $[1] = v(R, $[1]),
                            $[2] = v(T, $[2]),
                            $[3] = v(I, $[3]),
                            $[4] = v(h, $[4]),
                            $[5] = v(y, $[5]),
                            $[6] = v(w, $[6]),
                            $[7] = v(O, $[7])
                        }
                        return $
                    }
                    var b = r("./helpers");
                    n.exports = function(_) {
                        return b.hash(_, f, 32, !0)
                    }
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/gulp-browserify/node_modules/crypto-browserify/sha256.js", "/node_modules/gulp-browserify/node_modules/crypto-browserify")
            }
            , {
                "./helpers": 4,
                buffer: 3,
                lYpoI2: 10
            }],
            10: [function(r, n, i) {
                (function(s, o, a, l, u, c, g, d, m) {
                    function v() {}
                    (s = n.exports = {}).nextTick = function() {
                        var p = typeof window < "u" && window.setImmediate
                          , f = typeof window < "u" && window.postMessage && window.addEventListener;
                        if (p)
                            return function(_) {
                                return window.setImmediate(_)
                            }
                            ;
                        if (f) {
                            var b = [];
                            return window.addEventListener("message", function(_) {
                                var C = _.source;
                                C !== window && C !== null || _.data !== "process-tick" || (_.stopPropagation(),
                                0 < b.length && b.shift()())
                            }, !0),
                            function(_) {
                                b.push(_),
                                window.postMessage("process-tick", "*")
                            }
                        }
                        return function(_) {
                            setTimeout(_, 0)
                        }
                    }(),
                    s.title = "browser",
                    s.browser = !0,
                    s.env = {},
                    s.argv = [],
                    s.on = v,
                    s.addListener = v,
                    s.once = v,
                    s.off = v,
                    s.removeListener = v,
                    s.removeAllListeners = v,
                    s.emit = v,
                    s.binding = function(p) {
                        throw new Error("process.binding is not supported")
                    }
                    ,
                    s.cwd = function() {
                        return "/"
                    }
                    ,
                    s.chdir = function(p) {
                        throw new Error("process.chdir is not supported")
                    }
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/gulp-browserify/node_modules/process/browser.js", "/node_modules/gulp-browserify/node_modules/process")
            }
            , {
                buffer: 3,
                lYpoI2: 10
            }],
            11: [function(r, n, i) {
                (function(s, o, a, l, u, c, g, d, m) {
                    i.read = function(v, p, f, b, _) {
                        var C, E, R = 8 * _ - b - 1, T = (1 << R) - 1, I = T >> 1, h = -7, y = f ? _ - 1 : 0, w = f ? -1 : 1, O = v[p + y];
                        for (y += w,
                        C = O & (1 << -h) - 1,
                        O >>= -h,
                        h += R; 0 < h; C = 256 * C + v[p + y],
                        y += w,
                        h -= 8)
                            ;
                        for (E = C & (1 << -h) - 1,
                        C >>= -h,
                        h += b; 0 < h; E = 256 * E + v[p + y],
                        y += w,
                        h -= 8)
                            ;
                        if (C === 0)
                            C = 1 - I;
                        else {
                            if (C === T)
                                return E ? NaN : 1 / 0 * (O ? -1 : 1);
                            E += Math.pow(2, b),
                            C -= I
                        }
                        return (O ? -1 : 1) * E * Math.pow(2, C - b)
                    }
                    ,
                    i.write = function(v, p, f, b, _, C) {
                        var E, R, T, I = 8 * C - _ - 1, h = (1 << I) - 1, y = h >> 1, w = _ === 23 ? Math.pow(2, -24) - Math.pow(2, -77) : 0, O = b ? 0 : C - 1, A = b ? 1 : -1, k = p < 0 || p === 0 && 1 / p < 0 ? 1 : 0;
                        for (p = Math.abs(p),
                        isNaN(p) || p === 1 / 0 ? (R = isNaN(p) ? 1 : 0,
                        E = h) : (E = Math.floor(Math.log(p) / Math.LN2),
                        p * (T = Math.pow(2, -E)) < 1 && (E--,
                        T *= 2),
                        2 <= (p += 1 <= E + y ? w / T : w * Math.pow(2, 1 - y)) * T && (E++,
                        T /= 2),
                        h <= E + y ? (R = 0,
                        E = h) : 1 <= E + y ? (R = (p * T - 1) * Math.pow(2, _),
                        E += y) : (R = p * Math.pow(2, y - 1) * Math.pow(2, _),
                        E = 0)); 8 <= _; v[f + O] = 255 & R,
                        O += A,
                        R /= 256,
                        _ -= 8)
                            ;
                        for (E = E << _ | R,
                        I += _; 0 < I; v[f + O] = 255 & E,
                        O += A,
                        E /= 256,
                        I -= 8)
                            ;
                        v[f + O - A] |= 128 * k
                    }
                }
                ).call(this, r("lYpoI2"), typeof self < "u" ? self : typeof window < "u" ? window : {}, r("buffer").Buffer, arguments[3], arguments[4], arguments[5], arguments[6], "/node_modules/ieee754/index.js", "/node_modules/ieee754")
            }
            , {
                buffer: 3,
                lYpoI2: 10
            }]
        }, {}, [1])(1)
    })
}
)(Rm);
var ps, yl;
function Mm() {
    return yl || (yl = 1,
    ps = function(t) {
        t.prototype[Symbol.iterator] = function*() {
            for (let e = this.head; e; e = e.next)
                yield e.value
        }
    }
    ),
    ps
}
var Pm = fe;
fe.Node = tr;
fe.create = fe;
function fe(t) {
    var e = this;
    if (e instanceof fe || (e = new fe),
    e.tail = null,
    e.head = null,
    e.length = 0,
    t && typeof t.forEach == "function")
        t.forEach(function(i) {
            e.push(i)
        });
    else if (arguments.length > 0)
        for (var r = 0, n = arguments.length; r < n; r++)
            e.push(arguments[r]);
    return e
}
fe.prototype.removeNode = function(t) {
    if (t.list !== this)
        throw new Error("removing node which does not belong to this list");
    var e = t.next
      , r = t.prev;
    return e && (e.prev = r),
    r && (r.next = e),
    t === this.head && (this.head = e),
    t === this.tail && (this.tail = r),
    t.list.length--,
    t.next = null,
    t.prev = null,
    t.list = null,
    e
}
;
fe.prototype.unshiftNode = function(t) {
    if (t !== this.head) {
        t.list && t.list.removeNode(t);
        var e = this.head;
        t.list = this,
        t.next = e,
        e && (e.prev = t),
        this.head = t,
        this.tail || (this.tail = t),
        this.length++
    }
}
;
fe.prototype.pushNode = function(t) {
    if (t !== this.tail) {
        t.list && t.list.removeNode(t);
        var e = this.tail;
        t.list = this,
        t.prev = e,
        e && (e.next = t),
        this.tail = t,
        this.head || (this.head = t),
        this.length++
    }
}
;
fe.prototype.push = function() {
    for (var t = 0, e = arguments.length; t < e; t++)
        Lm(this, arguments[t]);
    return this.length
}
;
fe.prototype.unshift = function() {
    for (var t = 0, e = arguments.length; t < e; t++)
        Nm(this, arguments[t]);
    return this.length
}
;
fe.prototype.pop = function() {
    if (!!this.tail) {
        var t = this.tail.value;
        return this.tail = this.tail.prev,
        this.tail ? this.tail.next = null : this.head = null,
        this.length--,
        t
    }
}
;
fe.prototype.shift = function() {
    if (!!this.head) {
        var t = this.head.value;
        return this.head = this.head.next,
        this.head ? this.head.prev = null : this.tail = null,
        this.length--,
        t
    }
}
;
fe.prototype.forEach = function(t, e) {
    e = e || this;
    for (var r = this.head, n = 0; r !== null; n++)
        t.call(e, r.value, n, this),
        r = r.next
}
;
fe.prototype.forEachReverse = function(t, e) {
    e = e || this;
    for (var r = this.tail, n = this.length - 1; r !== null; n--)
        t.call(e, r.value, n, this),
        r = r.prev
}
;
fe.prototype.get = function(t) {
    for (var e = 0, r = this.head; r !== null && e < t; e++)
        r = r.next;
    if (e === t && r !== null)
        return r.value
}
;
fe.prototype.getReverse = function(t) {
    for (var e = 0, r = this.tail; r !== null && e < t; e++)
        r = r.prev;
    if (e === t && r !== null)
        return r.value
}
;
fe.prototype.map = function(t, e) {
    e = e || this;
    for (var r = new fe, n = this.head; n !== null; )
        r.push(t.call(e, n.value, this)),
        n = n.next;
    return r
}
;
fe.prototype.mapReverse = function(t, e) {
    e = e || this;
    for (var r = new fe, n = this.tail; n !== null; )
        r.push(t.call(e, n.value, this)),
        n = n.prev;
    return r
}
;
fe.prototype.reduce = function(t, e) {
    var r, n = this.head;
    if (arguments.length > 1)
        r = e;
    else if (this.head)
        n = this.head.next,
        r = this.head.value;
    else
        throw new TypeError("Reduce of empty list with no initial value");
    for (var i = 0; n !== null; i++)
        r = t(r, n.value, i),
        n = n.next;
    return r
}
;
fe.prototype.reduceReverse = function(t, e) {
    var r, n = this.tail;
    if (arguments.length > 1)
        r = e;
    else if (this.tail)
        n = this.tail.prev,
        r = this.tail.value;
    else
        throw new TypeError("Reduce of empty list with no initial value");
    for (var i = this.length - 1; n !== null; i--)
        r = t(r, n.value, i),
        n = n.prev;
    return r
}
;
fe.prototype.toArray = function() {
    for (var t = new Array(this.length), e = 0, r = this.head; r !== null; e++)
        t[e] = r.value,
        r = r.next;
    return t
}
;
fe.prototype.toArrayReverse = function() {
    for (var t = new Array(this.length), e = 0, r = this.tail; r !== null; e++)
        t[e] = r.value,
        r = r.prev;
    return t
}
;
fe.prototype.slice = function(t, e) {
    e = e || this.length,
    e < 0 && (e += this.length),
    t = t || 0,
    t < 0 && (t += this.length);
    var r = new fe;
    if (e < t || e < 0)
        return r;
    t < 0 && (t = 0),
    e > this.length && (e = this.length);
    for (var n = 0, i = this.head; i !== null && n < t; n++)
        i = i.next;
    for (; i !== null && n < e; n++,
    i = i.next)
        r.push(i.value);
    return r
}
;
fe.prototype.sliceReverse = function(t, e) {
    e = e || this.length,
    e < 0 && (e += this.length),
    t = t || 0,
    t < 0 && (t += this.length);
    var r = new fe;
    if (e < t || e < 0)
        return r;
    t < 0 && (t = 0),
    e > this.length && (e = this.length);
    for (var n = this.length, i = this.tail; i !== null && n > e; n--)
        i = i.prev;
    for (; i !== null && n > t; n--,
    i = i.prev)
        r.push(i.value);
    return r
}
;
fe.prototype.splice = function(t, e, ...r) {
    t > this.length && (t = this.length - 1),
    t < 0 && (t = this.length + t);
    for (var n = 0, i = this.head; i !== null && n < t; n++)
        i = i.next;
    for (var s = [], n = 0; i && n < e; n++)
        s.push(i.value),
        i = this.removeNode(i);
    i === null && (i = this.tail),
    i !== this.head && i !== this.tail && (i = i.prev);
    for (var n = 0; n < r.length; n++)
        i = Om(this, i, r[n]);
    return s
}
;
fe.prototype.reverse = function() {
    for (var t = this.head, e = this.tail, r = t; r !== null; r = r.prev) {
        var n = r.prev;
        r.prev = r.next,
        r.next = n
    }
    return this.head = e,
    this.tail = t,
    this
}
;
function Om(t, e, r) {
    var n = e === t.head ? new tr(r,null,e,t) : new tr(r,e,e.next,t);
    return n.next === null && (t.tail = n),
    n.prev === null && (t.head = n),
    t.length++,
    n
}
function Lm(t, e) {
    t.tail = new tr(e,t.tail,null,t),
    t.head || (t.head = t.tail),
    t.length++
}
function Nm(t, e) {
    t.head = new tr(e,null,t.head,t),
    t.tail || (t.tail = t.head),
    t.length++
}
function tr(t, e, r, n) {
    if (!(this instanceof tr))
        return new tr(t,e,r,n);
    this.list = n,
    this.value = t,
    e ? (e.next = this,
    this.prev = e) : this.prev = null,
    r ? (r.prev = this,
    this.next = r) : this.next = null
}
try {
    Mm()(fe)
} catch {}
const km = Pm
  , zt = Symbol("max")
  , wt = Symbol("length")
  , ar = Symbol("lengthCalculator")
  , Vr = Symbol("allowStale")
  , Gt = Symbol("maxAge")
  , bt = Symbol("dispose")
  , bl = Symbol("noDisposeOnSet")
  , Pe = Symbol("lruList")
  , nt = Symbol("cache")
  , yc = Symbol("updateAgeOnGet")
  , gs = () => 1;
class Bm {
    constructor(e) {
        if (typeof e == "number" && (e = {
            max: e
        }),
        e || (e = {}),
        e.max && (typeof e.max != "number" || e.max < 0))
            throw new TypeError("max must be a non-negative number");
        this[zt] = e.max || 1 / 0;
        const r = e.length || gs;
        if (this[ar] = typeof r != "function" ? gs : r,
        this[Vr] = e.stale || !1,
        e.maxAge && typeof e.maxAge != "number")
            throw new TypeError("maxAge must be a number");
        this[Gt] = e.maxAge || 0,
        this[bt] = e.dispose,
        this[bl] = e.noDisposeOnSet || !1,
        this[yc] = e.updateAgeOnGet || !1,
        this.reset()
    }
    set max(e) {
        if (typeof e != "number" || e < 0)
            throw new TypeError("max must be a non-negative number");
        this[zt] = e || 1 / 0,
        Tr(this)
    }
    get max() {
        return this[zt]
    }
    set allowStale(e) {
        this[Vr] = !!e
    }
    get allowStale() {
        return this[Vr]
    }
    set maxAge(e) {
        if (typeof e != "number")
            throw new TypeError("maxAge must be a non-negative number");
        this[Gt] = e,
        Tr(this)
    }
    get maxAge() {
        return this[Gt]
    }
    set lengthCalculator(e) {
        typeof e != "function" && (e = gs),
        e !== this[ar] && (this[ar] = e,
        this[wt] = 0,
        this[Pe].forEach(r => {
            r.length = this[ar](r.value, r.key),
            this[wt] += r.length
        }
        )),
        Tr(this)
    }
    get lengthCalculator() {
        return this[ar]
    }
    get length() {
        return this[wt]
    }
    get itemCount() {
        return this[Pe].length
    }
    rforEach(e, r) {
        r = r || this;
        for (let n = this[Pe].tail; n !== null; ) {
            const i = n.prev;
            wl(this, e, n, r),
            n = i
        }
    }
    forEach(e, r) {
        r = r || this;
        for (let n = this[Pe].head; n !== null; ) {
            const i = n.next;
            wl(this, e, n, r),
            n = i
        }
    }
    keys() {
        return this[Pe].toArray().map(e => e.key)
    }
    values() {
        return this[Pe].toArray().map(e => e.value)
    }
    reset() {
        this[bt] && this[Pe] && this[Pe].length && this[Pe].forEach(e => this[bt](e.key, e.value)),
        this[nt] = new Map,
        this[Pe] = new km,
        this[wt] = 0
    }
    dump() {
        return this[Pe].map(e => Vn(this, e) ? !1 : {
            k: e.key,
            v: e.value,
            e: e.now + (e.maxAge || 0)
        }).toArray().filter(e => e)
    }
    dumpLru() {
        return this[Pe]
    }
    set(e, r, n) {
        if (n = n || this[Gt],
        n && typeof n != "number")
            throw new TypeError("maxAge must be a number");
        const i = n ? Date.now() : 0
          , s = this[ar](r, e);
        if (this[nt].has(e)) {
            if (s > this[zt])
                return pr(this, this[nt].get(e)),
                !1;
            const l = this[nt].get(e).value;
            return this[bt] && (this[bl] || this[bt](e, l.value)),
            l.now = i,
            l.maxAge = n,
            l.value = r,
            this[wt] += s - l.length,
            l.length = s,
            this.get(e),
            Tr(this),
            !0
        }
        const o = new Dm(e,r,s,i,n);
        return o.length > this[zt] ? (this[bt] && this[bt](e, r),
        !1) : (this[wt] += o.length,
        this[Pe].unshift(o),
        this[nt].set(e, this[Pe].head),
        Tr(this),
        !0)
    }
    has(e) {
        if (!this[nt].has(e))
            return !1;
        const r = this[nt].get(e).value;
        return !Vn(this, r)
    }
    get(e) {
        return ms(this, e, !0)
    }
    peek(e) {
        return ms(this, e, !1)
    }
    pop() {
        const e = this[Pe].tail;
        return e ? (pr(this, e),
        e.value) : null
    }
    del(e) {
        pr(this, this[nt].get(e))
    }
    load(e) {
        this.reset();
        const r = Date.now();
        for (let n = e.length - 1; n >= 0; n--) {
            const i = e[n]
              , s = i.e || 0;
            if (s === 0)
                this.set(i.k, i.v);
            else {
                const o = s - r;
                o > 0 && this.set(i.k, i.v, o)
            }
        }
    }
    prune() {
        this[nt].forEach( (e, r) => ms(this, r, !1))
    }
}
const ms = (t, e, r) => {
    const n = t[nt].get(e);
    if (n) {
        const i = n.value;
        if (Vn(t, i)) {
            if (pr(t, n),
            !t[Vr])
                return
        } else
            r && (t[yc] && (n.value.now = Date.now()),
            t[Pe].unshiftNode(n));
        return i.value
    }
}
  , Vn = (t, e) => {
    if (!e || !e.maxAge && !t[Gt])
        return !1;
    const r = Date.now() - e.now;
    return e.maxAge ? r > e.maxAge : t[Gt] && r > t[Gt]
}
  , Tr = t => {
    if (t[wt] > t[zt])
        for (let e = t[Pe].tail; t[wt] > t[zt] && e !== null; ) {
            const r = e.prev;
            pr(t, e),
            e = r
        }
}
  , pr = (t, e) => {
    if (e) {
        const r = e.value;
        t[bt] && t[bt](r.key, r.value),
        t[wt] -= r.length,
        t[nt].delete(r.key),
        t[Pe].removeNode(e)
    }
}
;
class Dm {
    constructor(e, r, n, i, s) {
        this.key = e,
        this.value = r,
        this.length = n,
        this.now = i,
        this.maxAge = s || 0
    }
}
const wl = (t, e, r, n) => {
    let i = r.value;
    Vn(t, i) && (pr(t, r),
    t[Vr] || (i = void 0)),
    i && e.call(n, i.value, i.key, t)
}
;
var Fm = Bm;
const Um = 1e3 * 3;
new Fm({
    maxAge: Um
});
var _l;
(function(t) {
    t.BASE_REQ = "0",
    t.FINDER_UIN = "1"
}
)(_l || (_l = {}));
function El(t) {
    return new Promise(e => {
        setTimeout( () => {
            e(void 0)
        }
        , t)
    }
    )
}
const jm = 3
  , $m = 0
  , Hm = (t, e, r) => r || !0
  , qm = t => t instanceof pc;
function Vm(t) {
    return t.isAxiosError
}
class ci {
    static setGlobalConfig(e) {
        this.globalConfig = Object.assign(Object.assign({}, this.globalConfig), e)
    }
    constructor(e) {
        this.baseURL = "",
        this.axiosInstance = void 0,
        this.validateResult = Hm,
        this.onRetry = qm,
        this.formatConfig = c => c,
        this.generateAid = fl,
        this.requestMiddles = [],
        this.cancelTokenMap = new Map;
        const r = Object.assign(Object.assign({}, ci.globalConfig), e)
          , {baseURL: n, validateResult: i, onRetry: s, formatConfig: o, requestMiddles: a, adapter: l} = r
          , u = hg(r, ["baseURL", "validateResult", "onRetry", "formatConfig", "requestMiddles", "adapter"]);
        typeof i == "function" && (this.validateResult = i),
        typeof o == "function" && (this.formatConfig = o),
        typeof s == "function" && (this.onRetry = s),
        Array.isArray(a) && (this.requestMiddles = [...a]),
        this.instanceConfig = Object.assign(Object.assign({}, u), {
            adapter: l
        }),
        this.baseURL = n,
        this.generateAid = e.generateAid || fl,
        this.generateAidPromise = this.generateAid(),
        this.generateAidPromise.then(c => {
            this.aid = c
        }
        ),
        this.axiosInstance = qr.create(Object.assign({
            baseURL: n,
            adapter: l instanceof Pn ? l.sendRequest.bind(l) : l
        }, u))
    }
    getDefaultAdapter() {
        let e;
        return un ? e = Vg : (hc || cn) && (e = Wg),
        e
    }
    initRequestConfig(e) {
        const r = Object.assign(Object.assign({}, this.instanceConfig), e);
        if (r.retry && (r._retryCount = 0,
        r.retry.count || (r.retry.count = jm),
        r.retry.delay || (r.retry.delay = $m)),
        !r.adapter) {
            const n = this.getDefaultAdapter();
            n && (r.adapter = n)
        }
        return this.baseURL && !r.baseURL && (r.baseURL = this.baseURL),
        r
    }
    addMiddleware(e) {
        this.requestMiddles.push(e)
    }
    getRequestMiddles(e) {
        const r = [...this.requestMiddles];
        return e.enableRid !== !1 && r.unshift(this.addRidMiddleware.bind(this)),
        e.enableAid && r.unshift(this.addAidMiddleware.bind(this)),
        !e.disabledPassExportKey && !!ol && r.push(this.addExportKeyMiddleware.bind(this)),
        e.enablePassPageUrl !== !1 && r.push(this.addPageUrlMiddleware.bind(this)),
        e.retry && r.push(this.retry.bind(this)),
        r
    }
    request(e) {
        return de(this, void 0, void 0, function*() {
            e = this.initRequestConfig(e);
            const r = this.getRequestMiddles(e)
              , n = i => de(this, void 0, void 0, function*() {
                return r.length === 0 ? yield this.doRequest(i) : yield r.shift().bind(this)(i, n)
            });
            return n(e)
        })
    }
    rawRequest(e) {
        return de(this, void 0, void 0, function*() {
            e = this.initRequestConfig(e);
            const r = this.getRequestMiddles(e)
              , n = i => de(this, void 0, void 0, function*() {
                return r.length === 0 ? yield this.doRawRequest(i) : yield r.shift().bind(this)(i, n)
            });
            return n(e)
        })
    }
    addExportKeyMiddleware(e, r) {
        return de(this, void 0, void 0, function*() {
            return e.params = e.params || {},
            e.params.exportkey = ol,
            r(e)
        })
    }
    addRidMiddleware(e, r) {
        return de(this, void 0, void 0, function*() {
            return e.rid = Yg(),
            e.params || (e.params = {}),
            e.params._rid = e.rid,
            r(e)
        })
    }
    addAidMiddleware(e, r) {
        return de(this, void 0, void 0, function*() {
            return yield this.generateAidPromise,
            e.params || (e.params = {}),
            e.params._aid = this.aid.replace(/"/gi, ""),
            r(e)
        })
    }
    addPageUrlMiddleware(e, r) {
        return de(this, void 0, void 0, function*() {
            return e.params = e.params || {},
            e.params._pageUrl = Jg(),
            r(e)
        })
    }
    retry(e, r) {
        return de(this, void 0, void 0, function*() {
            try {
                e._retryCount || (e._retryCount = 0);
                try {
                    return yield r(e)
                } catch (n) {
                    if (e._retryCount >= e.retry.count)
                        return Promise.reject(n);
                    const i = this.onRetry(n, e, {
                        currentRetryCount: e._retryCount + 1
                    });
                    return i ? (typeof i != "boolean" ? yield El(i.delay || e.retry.count || 0) : yield El(e.retry.count || 0),
                    e._retryCount++,
                    this.retry(e, r)) : Promise.reject(n)
                }
            } catch (n) {
                return Promise.reject(n)
            }
        })
    }
    reportSuccess(e, r, n) {
        e.reportConfig && e.reportConfig.success && e.reportConfig.success(e, r, n)
    }
    reportFail(e, r, n) {
        e.reportConfig && e.reportConfig.fail && e.reportConfig.fail(e, r, n)
    }
    doRequest(e) {
        return de(this, void 0, void 0, function*() {
            const r = new Date().getTime();
            try {
                e.method || (e.method = "POST"),
                this.formatConfig && (e = this.formatConfig(e)),
                e.method.toLocaleLowerCase() === "get" ? e.params = e.params || e.data || {} : e.data = e.data || e.params || {};
                const n = yield this.axiosInstance.request(Object.assign(Object.assign({}, e), {
                    adapter: e.adapter instanceof Pn ? e.adapter.sendRequest.bind(e.adapter) : e.adapter
                }))
                  , i = Mr(n.data)
                  , s = this.validateResult(n.data, e, i)
                  , a = {
                    durationMs: new Date().getTime() - r
                };
                return s !== !0 ? (this.reportFail(e, s, a),
                Promise.reject(s)) : (this.reportSuccess(e, n.data, a),
                n.data)
            } catch (n) {
                const s = {
                    durationMs: new Date().getTime() - r
                };
                qr.isCancel(n) && (n.isCancel = !0,
                n = sl(n, e, "ECONNABORTED"));
                const o = Mr(n)
                  , a = this.validateResult(n, e, o);
                return a !== !0 ? (this.reportFail(e, a, s),
                Promise.reject(a)) : (this.reportSuccess(e, n, s),
                n)
            }
        })
    }
    doRawRequest(e) {
        return de(this, void 0, void 0, function*() {
            const r = new Date().getTime();
            try {
                e.method || (e.method = "POST"),
                this.formatConfig && (e = this.formatConfig(e)),
                e.method.toLocaleLowerCase() === "get" ? e.params = e.params || e.data || {} : e.data = e.data || e.params || {};
                const n = yield this.axiosInstance.request(Object.assign(Object.assign({}, e), {
                    adapter: e.adapter instanceof Pn ? e.adapter.sendRequest.bind(e.adapter) : e.adapter
                }))
                  , i = Mr(n.data)
                  , s = this.validateResult(n.data, e, i)
                  , a = {
                    durationMs: new Date().getTime() - r
                };
                return s !== !0 ? (this.reportFail(e, s, a),
                Promise.reject(s)) : (this.reportSuccess(e, n.data, a),
                n)
            } catch (n) {
                const s = {
                    durationMs: new Date().getTime() - r
                };
                qr.isCancel(n) && (n.isCancel = !0,
                n = sl(n, e, "ECONNABORTED"));
                const o = Mr(n)
                  , a = this.validateResult(n, e, o);
                return a !== !0 ? (this.reportFail(e, a, s),
                Promise.reject(a)) : (this.reportSuccess(e, n, s),
                n)
            }
        })
    }
    get(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "GET",
            yield this.request(e)
        })
    }
    rawGet(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "GET",
            yield this.rawRequest(e)
        })
    }
    post(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "POST",
            yield this.request(e)
        })
    }
    rawPost(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "POST",
            yield this.rawRequest(e)
        })
    }
    delete(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "DELETE",
            yield this.request(e)
        })
    }
    rawDelete(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "DELETE",
            yield this.rawRequest(e)
        })
    }
    head(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "HEAD",
            yield this.request(e)
        })
    }
    rawHead(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "HEAD",
            yield this.rawRequest(e)
        })
    }
    options(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "OPTIONS",
            yield this.request(e)
        })
    }
    rawOptions(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "OPTIONS",
            yield this.rawRequest(e)
        })
    }
    put(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "PUT",
            yield this.request(e)
        })
    }
    rawPut(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "PUT",
            yield this.rawRequest(e)
        })
    }
    patch(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "PATCH",
            yield this.request(e)
        })
    }
    rawPatch(e) {
        return de(this, void 0, void 0, function*() {
            return e.method = "PATCH",
            yield this.rawRequest(e)
        })
    }
}
ci.globalConfig = {};
var Xt = {}
  , Hs = {
    exports: {}
};
(function(t, e) {
    var r = typeof Reflect < "u" ? Reflect.construct : void 0
      , n = Object.defineProperty
      , i = Error.captureStackTrace;
    i === void 0 && (i = function(u) {
        var c = new Error;
        n(u, "stack", {
            configurable: !0,
            get: function() {
                var d = c.stack;
                return n(this, "stack", {
                    configurable: !0,
                    value: d,
                    writable: !0
                }),
                d
            },
            set: function(d) {
                n(u, "stack", {
                    configurable: !0,
                    value: d,
                    writable: !0
                })
            }
        })
    }
    );
    function s(l) {
        l !== void 0 && n(this, "message", {
            configurable: !0,
            value: l,
            writable: !0
        });
        var u = this.constructor.name;
        u !== void 0 && u !== this.name && n(this, "name", {
            configurable: !0,
            value: u,
            writable: !0
        }),
        i(this, this.constructor)
    }
    s.prototype = Object.create(Error.prototype, {
        constructor: {
            configurable: !0,
            value: s,
            writable: !0
        }
    });
    var o = function() {
        function l(c, g) {
            return n(c, "name", {
                configurable: !0,
                value: g
            })
        }
        try {
            var u = function() {};
            if (l(u, "foo"),
            u.name === "foo")
                return l
        } catch {}
    }();
    function a(l, u) {
        if (u == null || u === Error)
            u = s;
        else if (typeof u != "function")
            throw new TypeError("super_ should be a function");
        var c;
        if (typeof l == "string")
            c = l,
            l = r !== void 0 ? function() {
                return r(u, arguments, this.constructor)
            }
            : function() {
                u.apply(this, arguments)
            }
            ,
            o !== void 0 && (o(l, c),
            c = void 0);
        else if (typeof l != "function")
            throw new TypeError("constructor should be either a string or a function");
        l.super_ = l.super = u;
        var g = {
            constructor: {
                configurable: !0,
                value: l,
                writable: !0
            }
        };
        return c !== void 0 && (g.name = {
            configurable: !0,
            value: c,
            writable: !0
        }),
        l.prototype = Object.create(u.prototype, g),
        l
    }
    e = t.exports = a,
    e.BaseError = s
}
)(Hs, Hs.exports);
Object.defineProperty(Xt, "__esModule", {
    value: !0
});
var bc = Xt.ServerError = wc = Xt.LogicError = Xt.KnownError = void 0;
const Km = Hs.exports;
class fn extends Km.BaseError {
    static fromJSON(e) {
        return new fn(e.message || "")
    }
}
Xt.KnownError = fn;
class Co extends fn {
    constructor(e, r) {
        super(r),
        this.name = "LogicError",
        this.code = 0,
        this.code = e
    }
    static fromJSON(e) {
        return new Co(e.code || 0,e.message || "")
    }
}
var wc = Xt.LogicError = Co;
class Io extends fn {
    constructor() {
        super(...arguments),
        this.name = "ServerError"
    }
    static fromJSON(e) {
        return new Io(e.message || "")
    }
}
bc = Xt.ServerError = Io;
class Wm {
    constructor(e) {
        gi(this, "errorTypeMap", {});
        for (const r of e)
            this.errorTypeMap[r.name] = r
    }
    getError(e) {
        if (e.name) {
            const r = this.errorTypeMap[e.name];
            if (r)
                return r.fromJSON(e)
        }
        return e.message ? new Error(e.message) : new Error(JSON.stringify(e))
    }
}
const zm = new Wm([wc, bc]);
class _c extends ci {
    constructor(e) {
        super({
            ...e,
            timeout: 1e4,
            withCredentials: !0,
            validateResult: (r, n) => {
                const i = Mr(r);
                return i || (Vm(r) ? new On(r.message,-1,{
                    config: n
                }) : r.error ? new On(zm.getError(r.error.message).message,-1,{
                    config: n
                }) : typeof r.errCode == "number" && r.errCode !== 0 ? new On(r.errMsg,r.errCode,{
                    config: n
                }) : !0)
            }
        })
    }
}
const Ec = "/finder-preview";
class Gm extends _c {
    constructor() {
        super({
            baseURL: `${Ec}/api`
        })
    }
    get(e) {
        return super.get(e)
    }
    post(e) {
        return super.post(e)
    }
}
class Ym extends Gm {
    getFeedInfo(e) {
        return this.post({
            url: "feed/get_feed_info",
            data: {
                ...e
            }
        })
    }
    getProfileInfo(e) {
        return this.post({
            url: "feed/get_userpage",
            data: {
                ...e
            }
        })
    }
}
const Jm = new Ym;
function Qm(t) {
    if (typeof document > "u")
        return;
    const e = document.cookie.split(";");
    for (const r of e) {
        const [n,i] = r.trim().split("=");
        if (n === t)
            try {
                return decodeURIComponent(i)
            } catch {
                return i
            }
    }
}
const xc = kh("feed", () => {
    const t = Ie()
      , e = Ie()
      , r = Ie()
      , n = Ie()
      , i = async () => {
        try {
            const o = Hr().token || Qm("token") || ""
              , a = window.location.pathname.includes("/sph")
              , l = a ? dg(Hr().id) : Hr().eid;
            if (!l)
                return;
            const u = await Jm.getFeedInfo({
                baseReq: {
                    generalToken: o
                },
                ...a ? {
                    shortUri: l
                } : {
                    exportId: l
                }
            });
            t.value = u.data.feedInfo,
            e.value = u.data.authorInfo,
            r.value = u.data.errMsg,
            n.value = u.data.sceneInfo
        } catch {}
    }
      , s = at( () => !!r.value && r.value.type !== 0);
    return {
        feed: t,
        finderAcct: e,
        errMsg: r,
        sceneInfo: n,
        isWarning: s,
        getFeedDetail: i
    }
}
);
var dn = {}
  , Xm = function() {
    return typeof Promise == "function" && Promise.prototype && Promise.prototype.then
}
  , Sc = {}
  , Ge = {};
let To;
const Zm = [0, 26, 44, 70, 100, 134, 172, 196, 242, 292, 346, 404, 466, 532, 581, 655, 733, 815, 901, 991, 1085, 1156, 1258, 1364, 1474, 1588, 1706, 1828, 1921, 2051, 2185, 2323, 2465, 2611, 2761, 2876, 3034, 3196, 3362, 3532, 3706];
Ge.getSymbolSize = function(e) {
    if (!e)
        throw new Error('"version" cannot be null or undefined');
    if (e < 1 || e > 40)
        throw new Error('"version" should be in range from 1 to 40');
    return e * 4 + 17
}
;
Ge.getSymbolTotalCodewords = function(e) {
    return Zm[e]
}
;
Ge.getBCHDigit = function(t) {
    let e = 0;
    for (; t !== 0; )
        e++,
        t >>>= 1;
    return e
}
;
Ge.setToSJISFunction = function(e) {
    if (typeof e != "function")
        throw new Error('"toSJISFunc" is not a valid function.');
    To = e
}
;
Ge.isKanjiModeEnabled = function() {
    return typeof To < "u"
}
;
Ge.toSJIS = function(e) {
    return To(e)
}
;
var fi = {};
(function(t) {
    t.L = {
        bit: 1
    },
    t.M = {
        bit: 0
    },
    t.Q = {
        bit: 3
    },
    t.H = {
        bit: 2
    };
    function e(r) {
        if (typeof r != "string")
            throw new Error("Param is not a string");
        switch (r.toLowerCase()) {
        case "l":
        case "low":
            return t.L;
        case "m":
        case "medium":
            return t.M;
        case "q":
        case "quartile":
            return t.Q;
        case "h":
        case "high":
            return t.H;
        default:
            throw new Error("Unknown EC Level: " + r)
        }
    }
    t.isValid = function(n) {
        return n && typeof n.bit < "u" && n.bit >= 0 && n.bit < 4
    }
    ,
    t.from = function(n, i) {
        if (t.isValid(n))
            return n;
        try {
            return e(n)
        } catch {
            return i
        }
    }
}
)(fi);
function Cc() {
    this.buffer = [],
    this.length = 0
}
Cc.prototype = {
    get: function(t) {
        const e = Math.floor(t / 8);
        return (this.buffer[e] >>> 7 - t % 8 & 1) === 1
    },
    put: function(t, e) {
        for (let r = 0; r < e; r++)
            this.putBit((t >>> e - r - 1 & 1) === 1)
    },
    getLengthInBits: function() {
        return this.length
    },
    putBit: function(t) {
        const e = Math.floor(this.length / 8);
        this.buffer.length <= e && this.buffer.push(0),
        t && (this.buffer[e] |= 128 >>> this.length % 8),
        this.length++
    }
};
var e0 = Cc;
function hn(t) {
    if (!t || t < 1)
        throw new Error("BitMatrix size must be defined and greater than 0");
    this.size = t,
    this.data = new Uint8Array(t * t),
    this.reservedBit = new Uint8Array(t * t)
}
hn.prototype.set = function(t, e, r, n) {
    const i = t * this.size + e;
    this.data[i] = r,
    n && (this.reservedBit[i] = !0)
}
;
hn.prototype.get = function(t, e) {
    return this.data[t * this.size + e]
}
;
hn.prototype.xor = function(t, e, r) {
    this.data[t * this.size + e] ^= r
}
;
hn.prototype.isReserved = function(t, e) {
    return this.reservedBit[t * this.size + e]
}
;
var t0 = hn
  , Ic = {};
(function(t) {
    const e = Ge.getSymbolSize;
    t.getRowColCoords = function(n) {
        if (n === 1)
            return [];
        const i = Math.floor(n / 7) + 2
          , s = e(n)
          , o = s === 145 ? 26 : Math.ceil((s - 13) / (2 * i - 2)) * 2
          , a = [s - 7];
        for (let l = 1; l < i - 1; l++)
            a[l] = a[l - 1] - o;
        return a.push(6),
        a.reverse()
    }
    ,
    t.getPositions = function(n) {
        const i = []
          , s = t.getRowColCoords(n)
          , o = s.length;
        for (let a = 0; a < o; a++)
            for (let l = 0; l < o; l++)
                a === 0 && l === 0 || a === 0 && l === o - 1 || a === o - 1 && l === 0 || i.push([s[a], s[l]]);
        return i
    }
}
)(Ic);
var Tc = {};
const r0 = Ge.getSymbolSize
  , xl = 7;
Tc.getPositions = function(e) {
    const r = r0(e);
    return [[0, 0], [r - xl, 0], [0, r - xl]]
}
;
var Ac = {};
(function(t) {
    t.Patterns = {
        PATTERN000: 0,
        PATTERN001: 1,
        PATTERN010: 2,
        PATTERN011: 3,
        PATTERN100: 4,
        PATTERN101: 5,
        PATTERN110: 6,
        PATTERN111: 7
    };
    const e = {
        N1: 3,
        N2: 3,
        N3: 40,
        N4: 10
    };
    t.isValid = function(i) {
        return i != null && i !== "" && !isNaN(i) && i >= 0 && i <= 7
    }
    ,
    t.from = function(i) {
        return t.isValid(i) ? parseInt(i, 10) : void 0
    }
    ,
    t.getPenaltyN1 = function(i) {
        const s = i.size;
        let o = 0
          , a = 0
          , l = 0
          , u = null
          , c = null;
        for (let g = 0; g < s; g++) {
            a = l = 0,
            u = c = null;
            for (let d = 0; d < s; d++) {
                let m = i.get(g, d);
                m === u ? a++ : (a >= 5 && (o += e.N1 + (a - 5)),
                u = m,
                a = 1),
                m = i.get(d, g),
                m === c ? l++ : (l >= 5 && (o += e.N1 + (l - 5)),
                c = m,
                l = 1)
            }
            a >= 5 && (o += e.N1 + (a - 5)),
            l >= 5 && (o += e.N1 + (l - 5))
        }
        return o
    }
    ,
    t.getPenaltyN2 = function(i) {
        const s = i.size;
        let o = 0;
        for (let a = 0; a < s - 1; a++)
            for (let l = 0; l < s - 1; l++) {
                const u = i.get(a, l) + i.get(a, l + 1) + i.get(a + 1, l) + i.get(a + 1, l + 1);
                (u === 4 || u === 0) && o++
            }
        return o * e.N2
    }
    ,
    t.getPenaltyN3 = function(i) {
        const s = i.size;
        let o = 0
          , a = 0
          , l = 0;
        for (let u = 0; u < s; u++) {
            a = l = 0;
            for (let c = 0; c < s; c++)
                a = a << 1 & 2047 | i.get(u, c),
                c >= 10 && (a === 1488 || a === 93) && o++,
                l = l << 1 & 2047 | i.get(c, u),
                c >= 10 && (l === 1488 || l === 93) && o++
        }
        return o * e.N3
    }
    ,
    t.getPenaltyN4 = function(i) {
        let s = 0;
        const o = i.data.length;
        for (let l = 0; l < o; l++)
            s += i.data[l];
        return Math.abs(Math.ceil(s * 100 / o / 5) - 10) * e.N4
    }
    ;
    function r(n, i, s) {
        switch (n) {
        case t.Patterns.PATTERN000:
            return (i + s) % 2 === 0;
        case t.Patterns.PATTERN001:
            return i % 2 === 0;
        case t.Patterns.PATTERN010:
            return s % 3 === 0;
        case t.Patterns.PATTERN011:
            return (i + s) % 3 === 0;
        case t.Patterns.PATTERN100:
            return (Math.floor(i / 2) + Math.floor(s / 3)) % 2 === 0;
        case t.Patterns.PATTERN101:
            return i * s % 2 + i * s % 3 === 0;
        case t.Patterns.PATTERN110:
            return (i * s % 2 + i * s % 3) % 2 === 0;
        case t.Patterns.PATTERN111:
            return (i * s % 3 + (i + s) % 2) % 2 === 0;
        default:
            throw new Error("bad maskPattern:" + n)
        }
    }
    t.applyMask = function(i, s) {
        const o = s.size;
        for (let a = 0; a < o; a++)
            for (let l = 0; l < o; l++)
                s.isReserved(l, a) || s.xor(l, a, r(i, l, a))
    }
    ,
    t.getBestMask = function(i, s) {
        const o = Object.keys(t.Patterns).length;
        let a = 0
          , l = 1 / 0;
        for (let u = 0; u < o; u++) {
            s(u),
            t.applyMask(u, i);
            const c = t.getPenaltyN1(i) + t.getPenaltyN2(i) + t.getPenaltyN3(i) + t.getPenaltyN4(i);
            t.applyMask(u, i),
            c < l && (l = c,
            a = u)
        }
        return a
    }
}
)(Ac);
var di = {};
const Nt = fi
  , xn = [1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 1, 2, 2, 4, 1, 2, 4, 4, 2, 4, 4, 4, 2, 4, 6, 5, 2, 4, 6, 6, 2, 5, 8, 8, 4, 5, 8, 8, 4, 5, 8, 11, 4, 8, 10, 11, 4, 9, 12, 16, 4, 9, 16, 16, 6, 10, 12, 18, 6, 10, 17, 16, 6, 11, 16, 19, 6, 13, 18, 21, 7, 14, 21, 25, 8, 16, 20, 25, 8, 17, 23, 25, 9, 17, 23, 34, 9, 18, 25, 30, 10, 20, 27, 32, 12, 21, 29, 35, 12, 23, 34, 37, 12, 25, 34, 40, 13, 26, 35, 42, 14, 28, 38, 45, 15, 29, 40, 48, 16, 31, 43, 51, 17, 33, 45, 54, 18, 35, 48, 57, 19, 37, 51, 60, 19, 38, 53, 63, 20, 40, 56, 66, 21, 43, 59, 70, 22, 45, 62, 74, 24, 47, 65, 77, 25, 49, 68, 81]
  , Sn = [7, 10, 13, 17, 10, 16, 22, 28, 15, 26, 36, 44, 20, 36, 52, 64, 26, 48, 72, 88, 36, 64, 96, 112, 40, 72, 108, 130, 48, 88, 132, 156, 60, 110, 160, 192, 72, 130, 192, 224, 80, 150, 224, 264, 96, 176, 260, 308, 104, 198, 288, 352, 120, 216, 320, 384, 132, 240, 360, 432, 144, 280, 408, 480, 168, 308, 448, 532, 180, 338, 504, 588, 196, 364, 546, 650, 224, 416, 600, 700, 224, 442, 644, 750, 252, 476, 690, 816, 270, 504, 750, 900, 300, 560, 810, 960, 312, 588, 870, 1050, 336, 644, 952, 1110, 360, 700, 1020, 1200, 390, 728, 1050, 1260, 420, 784, 1140, 1350, 450, 812, 1200, 1440, 480, 868, 1290, 1530, 510, 924, 1350, 1620, 540, 980, 1440, 1710, 570, 1036, 1530, 1800, 570, 1064, 1590, 1890, 600, 1120, 1680, 1980, 630, 1204, 1770, 2100, 660, 1260, 1860, 2220, 720, 1316, 1950, 2310, 750, 1372, 2040, 2430];
di.getBlocksCount = function(e, r) {
    switch (r) {
    case Nt.L:
        return xn[(e - 1) * 4 + 0];
    case Nt.M:
        return xn[(e - 1) * 4 + 1];
    case Nt.Q:
        return xn[(e - 1) * 4 + 2];
    case Nt.H:
        return xn[(e - 1) * 4 + 3];
    default:
        return
    }
}
;
di.getTotalCodewordsCount = function(e, r) {
    switch (r) {
    case Nt.L:
        return Sn[(e - 1) * 4 + 0];
    case Nt.M:
        return Sn[(e - 1) * 4 + 1];
    case Nt.Q:
        return Sn[(e - 1) * 4 + 2];
    case Nt.H:
        return Sn[(e - 1) * 4 + 3];
    default:
        return
    }
}
;
var Rc = {}
  , hi = {};
const Kr = new Uint8Array(512)
  , Kn = new Uint8Array(256);
(function() {
    let e = 1;
    for (let r = 0; r < 255; r++)
        Kr[r] = e,
        Kn[e] = r,
        e <<= 1,
        e & 256 && (e ^= 285);
    for (let r = 255; r < 512; r++)
        Kr[r] = Kr[r - 255]
}
)();
hi.log = function(e) {
    if (e < 1)
        throw new Error("log(" + e + ")");
    return Kn[e]
}
;
hi.exp = function(e) {
    return Kr[e]
}
;
hi.mul = function(e, r) {
    return e === 0 || r === 0 ? 0 : Kr[Kn[e] + Kn[r]]
}
;
(function(t) {
    const e = hi;
    t.mul = function(n, i) {
        const s = new Uint8Array(n.length + i.length - 1);
        for (let o = 0; o < n.length; o++)
            for (let a = 0; a < i.length; a++)
                s[o + a] ^= e.mul(n[o], i[a]);
        return s
    }
    ,
    t.mod = function(n, i) {
        let s = new Uint8Array(n);
        for (; s.length - i.length >= 0; ) {
            const o = s[0];
            for (let l = 0; l < i.length; l++)
                s[l] ^= e.mul(i[l], o);
            let a = 0;
            for (; a < s.length && s[a] === 0; )
                a++;
            s = s.slice(a)
        }
        return s
    }
    ,
    t.generateECPolynomial = function(n) {
        let i = new Uint8Array([1]);
        for (let s = 0; s < n; s++)
            i = t.mul(i, new Uint8Array([1, e.exp(s)]));
        return i
    }
}
)(Rc);
const Mc = Rc;
function Ao(t) {
    this.genPoly = void 0,
    this.degree = t,
    this.degree && this.initialize(this.degree)
}
Ao.prototype.initialize = function(e) {
    this.degree = e,
    this.genPoly = Mc.generateECPolynomial(this.degree)
}
;
Ao.prototype.encode = function(e) {
    if (!this.genPoly)
        throw new Error("Encoder not initialized");
    const r = new Uint8Array(e.length + this.degree);
    r.set(e);
    const n = Mc.mod(r, this.genPoly)
      , i = this.degree - n.length;
    if (i > 0) {
        const s = new Uint8Array(this.degree);
        return s.set(n, i),
        s
    }
    return n
}
;
var n0 = Ao
  , Pc = {}
  , Ut = {}
  , Ro = {};
Ro.isValid = function(e) {
    return !isNaN(e) && e >= 1 && e <= 40
}
;
var pt = {};
const Oc = "[0-9]+"
  , i0 = "[A-Z $%*+\\-./:]+";
let nn = "(?:[u3000-u303F]|[u3040-u309F]|[u30A0-u30FF]|[uFF00-uFFEF]|[u4E00-u9FAF]|[u2605-u2606]|[u2190-u2195]|u203B|[u2010u2015u2018u2019u2025u2026u201Cu201Du2225u2260]|[u0391-u0451]|[u00A7u00A8u00B1u00B4u00D7u00F7])+";
nn = nn.replace(/u/g, "\\u");
const s0 = "(?:(?![A-Z0-9 $%*+\\-./:]|" + nn + `)(?:.|[\r
]))+`;
pt.KANJI = new RegExp(nn,"g");
pt.BYTE_KANJI = new RegExp("[^A-Z0-9 $%*+\\-./:]+","g");
pt.BYTE = new RegExp(s0,"g");
pt.NUMERIC = new RegExp(Oc,"g");
pt.ALPHANUMERIC = new RegExp(i0,"g");
const o0 = new RegExp("^" + nn + "$")
  , a0 = new RegExp("^" + Oc + "$")
  , l0 = new RegExp("^[A-Z0-9 $%*+\\-./:]+$");
pt.testKanji = function(e) {
    return o0.test(e)
}
;
pt.testNumeric = function(e) {
    return a0.test(e)
}
;
pt.testAlphanumeric = function(e) {
    return l0.test(e)
}
;
(function(t) {
    const e = Ro
      , r = pt;
    t.NUMERIC = {
        id: "Numeric",
        bit: 1 << 0,
        ccBits: [10, 12, 14]
    },
    t.ALPHANUMERIC = {
        id: "Alphanumeric",
        bit: 1 << 1,
        ccBits: [9, 11, 13]
    },
    t.BYTE = {
        id: "Byte",
        bit: 1 << 2,
        ccBits: [8, 16, 16]
    },
    t.KANJI = {
        id: "Kanji",
        bit: 1 << 3,
        ccBits: [8, 10, 12]
    },
    t.MIXED = {
        bit: -1
    },
    t.getCharCountIndicator = function(s, o) {
        if (!s.ccBits)
            throw new Error("Invalid mode: " + s);
        if (!e.isValid(o))
            throw new Error("Invalid version: " + o);
        return o >= 1 && o < 10 ? s.ccBits[0] : o < 27 ? s.ccBits[1] : s.ccBits[2]
    }
    ,
    t.getBestModeForData = function(s) {
        return r.testNumeric(s) ? t.NUMERIC : r.testAlphanumeric(s) ? t.ALPHANUMERIC : r.testKanji(s) ? t.KANJI : t.BYTE
    }
    ,
    t.toString = function(s) {
        if (s && s.id)
            return s.id;
        throw new Error("Invalid mode")
    }
    ,
    t.isValid = function(s) {
        return s && s.bit && s.ccBits
    }
    ;
    function n(i) {
        if (typeof i != "string")
            throw new Error("Param is not a string");
        switch (i.toLowerCase()) {
        case "numeric":
            return t.NUMERIC;
        case "alphanumeric":
            return t.ALPHANUMERIC;
        case "kanji":
            return t.KANJI;
        case "byte":
            return t.BYTE;
        default:
            throw new Error("Unknown mode: " + i)
        }
    }
    t.from = function(s, o) {
        if (t.isValid(s))
            return s;
        try {
            return n(s)
        } catch {
            return o
        }
    }
}
)(Ut);
(function(t) {
    const e = Ge
      , r = di
      , n = fi
      , i = Ut
      , s = Ro
      , o = 1 << 12 | 1 << 11 | 1 << 10 | 1 << 9 | 1 << 8 | 1 << 5 | 1 << 2 | 1 << 0
      , a = e.getBCHDigit(o);
    function l(d, m, v) {
        for (let p = 1; p <= 40; p++)
            if (m <= t.getCapacity(p, v, d))
                return p
    }
    function u(d, m) {
        return i.getCharCountIndicator(d, m) + 4
    }
    function c(d, m) {
        let v = 0;
        return d.forEach(function(p) {
            const f = u(p.mode, m);
            v += f + p.getBitsLength()
        }),
        v
    }
    function g(d, m) {
        for (let v = 1; v <= 40; v++)
            if (c(d, v) <= t.getCapacity(v, m, i.MIXED))
                return v
    }
    t.from = function(m, v) {
        return s.isValid(m) ? parseInt(m, 10) : v
    }
    ,
    t.getCapacity = function(m, v, p) {
        if (!s.isValid(m))
            throw new Error("Invalid QR Code version");
        typeof p > "u" && (p = i.BYTE);
        const f = e.getSymbolTotalCodewords(m)
          , b = r.getTotalCodewordsCount(m, v)
          , _ = (f - b) * 8;
        if (p === i.MIXED)
            return _;
        const C = _ - u(p, m);
        switch (p) {
        case i.NUMERIC:
            return Math.floor(C / 10 * 3);
        case i.ALPHANUMERIC:
            return Math.floor(C / 11 * 2);
        case i.KANJI:
            return Math.floor(C / 13);
        case i.BYTE:
        default:
            return Math.floor(C / 8)
        }
    }
    ,
    t.getBestVersionForData = function(m, v) {
        let p;
        const f = n.from(v, n.M);
        if (Array.isArray(m)) {
            if (m.length > 1)
                return g(m, f);
            if (m.length === 0)
                return 1;
            p = m[0]
        } else
            p = m;
        return l(p.mode, p.getLength(), f)
    }
    ,
    t.getEncodedBits = function(m) {
        if (!s.isValid(m) || m < 7)
            throw new Error("Invalid QR Code version");
        let v = m << 12;
        for (; e.getBCHDigit(v) - a >= 0; )
            v ^= o << e.getBCHDigit(v) - a;
        return m << 12 | v
    }
}
)(Pc);
var Lc = {};
const qs = Ge
  , Nc = 1 << 10 | 1 << 8 | 1 << 5 | 1 << 4 | 1 << 2 | 1 << 1 | 1 << 0
  , u0 = 1 << 14 | 1 << 12 | 1 << 10 | 1 << 4 | 1 << 1
  , Sl = qs.getBCHDigit(Nc);
Lc.getEncodedBits = function(e, r) {
    const n = e.bit << 3 | r;
    let i = n << 10;
    for (; qs.getBCHDigit(i) - Sl >= 0; )
        i ^= Nc << qs.getBCHDigit(i) - Sl;
    return (n << 10 | i) ^ u0
}
;
var kc = {};
const c0 = Ut;
function vr(t) {
    this.mode = c0.NUMERIC,
    this.data = t.toString()
}
vr.getBitsLength = function(e) {
    return 10 * Math.floor(e / 3) + (e % 3 ? e % 3 * 3 + 1 : 0)
}
;
vr.prototype.getLength = function() {
    return this.data.length
}
;
vr.prototype.getBitsLength = function() {
    return vr.getBitsLength(this.data.length)
}
;
vr.prototype.write = function(e) {
    let r, n, i;
    for (r = 0; r + 3 <= this.data.length; r += 3)
        n = this.data.substr(r, 3),
        i = parseInt(n, 10),
        e.put(i, 10);
    const s = this.data.length - r;
    s > 0 && (n = this.data.substr(r),
    i = parseInt(n, 10),
    e.put(i, s * 3 + 1))
}
;
var f0 = vr;
const d0 = Ut
  , vs = ["0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z", " ", "$", "%", "*", "+", "-", ".", "/", ":"];
function yr(t) {
    this.mode = d0.ALPHANUMERIC,
    this.data = t
}
yr.getBitsLength = function(e) {
    return 11 * Math.floor(e / 2) + 6 * (e % 2)
}
;
yr.prototype.getLength = function() {
    return this.data.length
}
;
yr.prototype.getBitsLength = function() {
    return yr.getBitsLength(this.data.length)
}
;
yr.prototype.write = function(e) {
    let r;
    for (r = 0; r + 2 <= this.data.length; r += 2) {
        let n = vs.indexOf(this.data[r]) * 45;
        n += vs.indexOf(this.data[r + 1]),
        e.put(n, 11)
    }
    this.data.length % 2 && e.put(vs.indexOf(this.data[r]), 6)
}
;
var h0 = yr;
const p0 = Ut;
function br(t) {
    this.mode = p0.BYTE,
    typeof t == "string" ? this.data = new TextEncoder().encode(t) : this.data = new Uint8Array(t)
}
br.getBitsLength = function(e) {
    return e * 8
}
;
br.prototype.getLength = function() {
    return this.data.length
}
;
br.prototype.getBitsLength = function() {
    return br.getBitsLength(this.data.length)
}
;
br.prototype.write = function(t) {
    for (let e = 0, r = this.data.length; e < r; e++)
        t.put(this.data[e], 8)
}
;
var g0 = br;
const m0 = Ut
  , v0 = Ge;
function wr(t) {
    this.mode = m0.KANJI,
    this.data = t
}
wr.getBitsLength = function(e) {
    return e * 13
}
;
wr.prototype.getLength = function() {
    return this.data.length
}
;
wr.prototype.getBitsLength = function() {
    return wr.getBitsLength(this.data.length)
}
;
wr.prototype.write = function(t) {
    let e;
    for (e = 0; e < this.data.length; e++) {
        let r = v0.toSJIS(this.data[e]);
        if (r >= 33088 && r <= 40956)
            r -= 33088;
        else if (r >= 57408 && r <= 60351)
            r -= 49472;
        else
            throw new Error("Invalid SJIS character: " + this.data[e] + `
Make sure your charset is UTF-8`);
        r = (r >>> 8 & 255) * 192 + (r & 255),
        t.put(r, 13)
    }
}
;
var y0 = wr
  , Bc = {
    exports: {}
};
(function(t) {
    var e = {
        single_source_shortest_paths: function(r, n, i) {
            var s = {}
              , o = {};
            o[n] = 0;
            var a = e.PriorityQueue.make();
            a.push(n, 0);
            for (var l, u, c, g, d, m, v, p, f; !a.empty(); ) {
                l = a.pop(),
                u = l.value,
                g = l.cost,
                d = r[u] || {};
                for (c in d)
                    d.hasOwnProperty(c) && (m = d[c],
                    v = g + m,
                    p = o[c],
                    f = typeof o[c] > "u",
                    (f || p > v) && (o[c] = v,
                    a.push(c, v),
                    s[c] = u))
            }
            if (typeof i < "u" && typeof o[i] > "u") {
                var b = ["Could not find a path from ", n, " to ", i, "."].join("");
                throw new Error(b)
            }
            return s
        },
        extract_shortest_path_from_predecessor_list: function(r, n) {
            for (var i = [], s = n; s; )
                i.push(s),
                r[s],
                s = r[s];
            return i.reverse(),
            i
        },
        find_path: function(r, n, i) {
            var s = e.single_source_shortest_paths(r, n, i);
            return e.extract_shortest_path_from_predecessor_list(s, i)
        },
        PriorityQueue: {
            make: function(r) {
                var n = e.PriorityQueue, i = {}, s;
                r = r || {};
                for (s in n)
                    n.hasOwnProperty(s) && (i[s] = n[s]);
                return i.queue = [],
                i.sorter = r.sorter || n.default_sorter,
                i
            },
            default_sorter: function(r, n) {
                return r.cost - n.cost
            },
            push: function(r, n) {
                var i = {
                    value: r,
                    cost: n
                };
                this.queue.push(i),
                this.queue.sort(this.sorter)
            },
            pop: function() {
                return this.queue.shift()
            },
            empty: function() {
                return this.queue.length === 0
            }
        }
    };
    t.exports = e
}
)(Bc);
(function(t) {
    const e = Ut
      , r = f0
      , n = h0
      , i = g0
      , s = y0
      , o = pt
      , a = Ge
      , l = Bc.exports;
    function u(b) {
        return unescape(encodeURIComponent(b)).length
    }
    function c(b, _, C) {
        const E = [];
        let R;
        for (; (R = b.exec(C)) !== null; )
            E.push({
                data: R[0],
                index: R.index,
                mode: _,
                length: R[0].length
            });
        return E
    }
    function g(b) {
        const _ = c(o.NUMERIC, e.NUMERIC, b)
          , C = c(o.ALPHANUMERIC, e.ALPHANUMERIC, b);
        let E, R;
        return a.isKanjiModeEnabled() ? (E = c(o.BYTE, e.BYTE, b),
        R = c(o.KANJI, e.KANJI, b)) : (E = c(o.BYTE_KANJI, e.BYTE, b),
        R = []),
        _.concat(C, E, R).sort(function(I, h) {
            return I.index - h.index
        }).map(function(I) {
            return {
                data: I.data,
                mode: I.mode,
                length: I.length
            }
        })
    }
    function d(b, _) {
        switch (_) {
        case e.NUMERIC:
            return r.getBitsLength(b);
        case e.ALPHANUMERIC:
            return n.getBitsLength(b);
        case e.KANJI:
            return s.getBitsLength(b);
        case e.BYTE:
            return i.getBitsLength(b)
        }
    }
    function m(b) {
        return b.reduce(function(_, C) {
            const E = _.length - 1 >= 0 ? _[_.length - 1] : null;
            return E && E.mode === C.mode ? (_[_.length - 1].data += C.data,
            _) : (_.push(C),
            _)
        }, [])
    }
    function v(b) {
        const _ = [];
        for (let C = 0; C < b.length; C++) {
            const E = b[C];
            switch (E.mode) {
            case e.NUMERIC:
                _.push([E, {
                    data: E.data,
                    mode: e.ALPHANUMERIC,
                    length: E.length
                }, {
                    data: E.data,
                    mode: e.BYTE,
                    length: E.length
                }]);
                break;
            case e.ALPHANUMERIC:
                _.push([E, {
                    data: E.data,
                    mode: e.BYTE,
                    length: E.length
                }]);
                break;
            case e.KANJI:
                _.push([E, {
                    data: E.data,
                    mode: e.BYTE,
                    length: u(E.data)
                }]);
                break;
            case e.BYTE:
                _.push([{
                    data: E.data,
                    mode: e.BYTE,
                    length: u(E.data)
                }])
            }
        }
        return _
    }
    function p(b, _) {
        const C = {}
          , E = {
            start: {}
        };
        let R = ["start"];
        for (let T = 0; T < b.length; T++) {
            const I = b[T]
              , h = [];
            for (let y = 0; y < I.length; y++) {
                const w = I[y]
                  , O = "" + T + y;
                h.push(O),
                C[O] = {
                    node: w,
                    lastCount: 0
                },
                E[O] = {};
                for (let A = 0; A < R.length; A++) {
                    const k = R[A];
                    C[k] && C[k].node.mode === w.mode ? (E[k][O] = d(C[k].lastCount + w.length, w.mode) - d(C[k].lastCount, w.mode),
                    C[k].lastCount += w.length) : (C[k] && (C[k].lastCount = w.length),
                    E[k][O] = d(w.length, w.mode) + 4 + e.getCharCountIndicator(w.mode, _))
                }
            }
            R = h
        }
        for (let T = 0; T < R.length; T++)
            E[R[T]].end = 0;
        return {
            map: E,
            table: C
        }
    }
    function f(b, _) {
        let C;
        const E = e.getBestModeForData(b);
        if (C = e.from(_, E),
        C !== e.BYTE && C.bit < E.bit)
            throw new Error('"' + b + '" cannot be encoded with mode ' + e.toString(C) + `.
 Suggested mode is: ` + e.toString(E));
        switch (C === e.KANJI && !a.isKanjiModeEnabled() && (C = e.BYTE),
        C) {
        case e.NUMERIC:
            return new r(b);
        case e.ALPHANUMERIC:
            return new n(b);
        case e.KANJI:
            return new s(b);
        case e.BYTE:
            return new i(b)
        }
    }
    t.fromArray = function(_) {
        return _.reduce(function(C, E) {
            return typeof E == "string" ? C.push(f(E, null)) : E.data && C.push(f(E.data, E.mode)),
            C
        }, [])
    }
    ,
    t.fromString = function(_, C) {
        const E = g(_, a.isKanjiModeEnabled())
          , R = v(E)
          , T = p(R, C)
          , I = l.find_path(T.map, "start", "end")
          , h = [];
        for (let y = 1; y < I.length - 1; y++)
            h.push(T.table[I[y]].node);
        return t.fromArray(m(h))
    }
    ,
    t.rawSplit = function(_) {
        return t.fromArray(g(_, a.isKanjiModeEnabled()))
    }
}
)(kc);
const pi = Ge
  , ys = fi
  , b0 = e0
  , w0 = t0
  , _0 = Ic
  , E0 = Tc
  , Vs = Ac
  , Ks = di
  , x0 = n0
  , Wn = Pc
  , S0 = Lc
  , C0 = Ut
  , bs = kc;
function I0(t, e) {
    const r = t.size
      , n = E0.getPositions(e);
    for (let i = 0; i < n.length; i++) {
        const s = n[i][0]
          , o = n[i][1];
        for (let a = -1; a <= 7; a++)
            if (!(s + a <= -1 || r <= s + a))
                for (let l = -1; l <= 7; l++)
                    o + l <= -1 || r <= o + l || (a >= 0 && a <= 6 && (l === 0 || l === 6) || l >= 0 && l <= 6 && (a === 0 || a === 6) || a >= 2 && a <= 4 && l >= 2 && l <= 4 ? t.set(s + a, o + l, !0, !0) : t.set(s + a, o + l, !1, !0))
    }
}
function T0(t) {
    const e = t.size;
    for (let r = 8; r < e - 8; r++) {
        const n = r % 2 === 0;
        t.set(r, 6, n, !0),
        t.set(6, r, n, !0)
    }
}
function A0(t, e) {
    const r = _0.getPositions(e);
    for (let n = 0; n < r.length; n++) {
        const i = r[n][0]
          , s = r[n][1];
        for (let o = -2; o <= 2; o++)
            for (let a = -2; a <= 2; a++)
                o === -2 || o === 2 || a === -2 || a === 2 || o === 0 && a === 0 ? t.set(i + o, s + a, !0, !0) : t.set(i + o, s + a, !1, !0)
    }
}
function R0(t, e) {
    const r = t.size
      , n = Wn.getEncodedBits(e);
    let i, s, o;
    for (let a = 0; a < 18; a++)
        i = Math.floor(a / 3),
        s = a % 3 + r - 8 - 3,
        o = (n >> a & 1) === 1,
        t.set(i, s, o, !0),
        t.set(s, i, o, !0)
}
function ws(t, e, r) {
    const n = t.size
      , i = S0.getEncodedBits(e, r);
    let s, o;
    for (s = 0; s < 15; s++)
        o = (i >> s & 1) === 1,
        s < 6 ? t.set(s, 8, o, !0) : s < 8 ? t.set(s + 1, 8, o, !0) : t.set(n - 15 + s, 8, o, !0),
        s < 8 ? t.set(8, n - s - 1, o, !0) : s < 9 ? t.set(8, 15 - s - 1 + 1, o, !0) : t.set(8, 15 - s - 1, o, !0);
    t.set(n - 8, 8, 1, !0)
}
function M0(t, e) {
    const r = t.size;
    let n = -1
      , i = r - 1
      , s = 7
      , o = 0;
    for (let a = r - 1; a > 0; a -= 2)
        for (a === 6 && a--; ; ) {
            for (let l = 0; l < 2; l++)
                if (!t.isReserved(i, a - l)) {
                    let u = !1;
                    o < e.length && (u = (e[o] >>> s & 1) === 1),
                    t.set(i, a - l, u),
                    s--,
                    s === -1 && (o++,
                    s = 7)
                }
            if (i += n,
            i < 0 || r <= i) {
                i -= n,
                n = -n;
                break
            }
        }
}
function P0(t, e, r) {
    const n = new b0;
    r.forEach(function(l) {
        n.put(l.mode.bit, 4),
        n.put(l.getLength(), C0.getCharCountIndicator(l.mode, t)),
        l.write(n)
    });
    const i = pi.getSymbolTotalCodewords(t)
      , s = Ks.getTotalCodewordsCount(t, e)
      , o = (i - s) * 8;
    for (n.getLengthInBits() + 4 <= o && n.put(0, 4); n.getLengthInBits() % 8 !== 0; )
        n.putBit(0);
    const a = (o - n.getLengthInBits()) / 8;
    for (let l = 0; l < a; l++)
        n.put(l % 2 ? 17 : 236, 8);
    return O0(n, t, e)
}
function O0(t, e, r) {
    const n = pi.getSymbolTotalCodewords(e)
      , i = Ks.getTotalCodewordsCount(e, r)
      , s = n - i
      , o = Ks.getBlocksCount(e, r)
      , a = n % o
      , l = o - a
      , u = Math.floor(n / o)
      , c = Math.floor(s / o)
      , g = c + 1
      , d = u - c
      , m = new x0(d);
    let v = 0;
    const p = new Array(o)
      , f = new Array(o);
    let b = 0;
    const _ = new Uint8Array(t.buffer);
    for (let I = 0; I < o; I++) {
        const h = I < l ? c : g;
        p[I] = _.slice(v, v + h),
        f[I] = m.encode(p[I]),
        v += h,
        b = Math.max(b, h)
    }
    const C = new Uint8Array(n);
    let E = 0, R, T;
    for (R = 0; R < b; R++)
        for (T = 0; T < o; T++)
            R < p[T].length && (C[E++] = p[T][R]);
    for (R = 0; R < d; R++)
        for (T = 0; T < o; T++)
            C[E++] = f[T][R];
    return C
}
function L0(t, e, r, n) {
    let i;
    if (Array.isArray(t))
        i = bs.fromArray(t);
    else if (typeof t == "string") {
        let u = e;
        if (!u) {
            const c = bs.rawSplit(t);
            u = Wn.getBestVersionForData(c, r)
        }
        i = bs.fromString(t, u || 40)
    } else
        throw new Error("Invalid data");
    const s = Wn.getBestVersionForData(i, r);
    if (!s)
        throw new Error("The amount of data is too big to be stored in a QR Code");
    if (!e)
        e = s;
    else if (e < s)
        throw new Error(`
The chosen QR Code version cannot contain this amount of data.
Minimum version required to store current data is: ` + s + `.
`);
    const o = P0(e, r, i)
      , a = pi.getSymbolSize(e)
      , l = new w0(a);
    return I0(l, e),
    T0(l),
    A0(l, e),
    ws(l, r, 0),
    e >= 7 && R0(l, e),
    M0(l, o),
    isNaN(n) && (n = Vs.getBestMask(l, ws.bind(null, l, r))),
    Vs.applyMask(n, l),
    ws(l, r, n),
    {
        modules: l,
        version: e,
        errorCorrectionLevel: r,
        maskPattern: n,
        segments: i
    }
}
Sc.create = function(e, r) {
    if (typeof e > "u" || e === "")
        throw new Error("No input text");
    let n = ys.M, i, s;
    return typeof r < "u" && (n = ys.from(r.errorCorrectionLevel, ys.M),
    i = Wn.from(r.version),
    s = Vs.from(r.maskPattern),
    r.toSJISFunc && pi.setToSJISFunction(r.toSJISFunc)),
    L0(e, i, n, s)
}
;
var Dc = {}
  , Mo = {};
(function(t) {
    function e(r) {
        if (typeof r == "number" && (r = r.toString()),
        typeof r != "string")
            throw new Error("Color should be defined as hex string");
        let n = r.slice().replace("#", "").split("");
        if (n.length < 3 || n.length === 5 || n.length > 8)
            throw new Error("Invalid hex color: " + r);
        (n.length === 3 || n.length === 4) && (n = Array.prototype.concat.apply([], n.map(function(s) {
            return [s, s]
        }))),
        n.length === 6 && n.push("F", "F");
        const i = parseInt(n.join(""), 16);
        return {
            r: i >> 24 & 255,
            g: i >> 16 & 255,
            b: i >> 8 & 255,
            a: i & 255,
            hex: "#" + n.slice(0, 6).join("")
        }
    }
    t.getOptions = function(n) {
        n || (n = {}),
        n.color || (n.color = {});
        const i = typeof n.margin > "u" || n.margin === null || n.margin < 0 ? 4 : n.margin
          , s = n.width && n.width >= 21 ? n.width : void 0
          , o = n.scale || 4;
        return {
            width: s,
            scale: s ? 4 : o,
            margin: i,
            color: {
                dark: e(n.color.dark || "#000000ff"),
                light: e(n.color.light || "#ffffffff")
            },
            type: n.type,
            rendererOpts: n.rendererOpts || {}
        }
    }
    ,
    t.getScale = function(n, i) {
        return i.width && i.width >= n + i.margin * 2 ? i.width / (n + i.margin * 2) : i.scale
    }
    ,
    t.getImageWidth = function(n, i) {
        const s = t.getScale(n, i);
        return Math.floor((n + i.margin * 2) * s)
    }
    ,
    t.qrToImageData = function(n, i, s) {
        const o = i.modules.size
          , a = i.modules.data
          , l = t.getScale(o, s)
          , u = Math.floor((o + s.margin * 2) * l)
          , c = s.margin * l
          , g = [s.color.light, s.color.dark];
        for (let d = 0; d < u; d++)
            for (let m = 0; m < u; m++) {
                let v = (d * u + m) * 4
                  , p = s.color.light;
                if (d >= c && m >= c && d < u - c && m < u - c) {
                    const f = Math.floor((d - c) / l)
                      , b = Math.floor((m - c) / l);
                    p = g[a[f * o + b] ? 1 : 0]
                }
                n[v++] = p.r,
                n[v++] = p.g,
                n[v++] = p.b,
                n[v] = p.a
            }
    }
}
)(Mo);
(function(t) {
    const e = Mo;
    function r(i, s, o) {
        i.clearRect(0, 0, s.width, s.height),
        s.style || (s.style = {}),
        s.height = o,
        s.width = o,
        s.style.height = o + "px",
        s.style.width = o + "px"
    }
    function n() {
        try {
            return document.createElement("canvas")
        } catch {
            throw new Error("You need to specify a canvas element")
        }
    }
    t.render = function(s, o, a) {
        let l = a
          , u = o;
        typeof l > "u" && (!o || !o.getContext) && (l = o,
        o = void 0),
        o || (u = n()),
        l = e.getOptions(l);
        const c = e.getImageWidth(s.modules.size, l)
          , g = u.getContext("2d")
          , d = g.createImageData(c, c);
        return e.qrToImageData(d.data, s, l),
        r(g, u, c),
        g.putImageData(d, 0, 0),
        u
    }
    ,
    t.renderToDataURL = function(s, o, a) {
        let l = a;
        typeof l > "u" && (!o || !o.getContext) && (l = o,
        o = void 0),
        l || (l = {});
        const u = t.render(s, o, l)
          , c = l.type || "image/png"
          , g = l.rendererOpts || {};
        return u.toDataURL(c, g.quality)
    }
}
)(Dc);
var Fc = {};
const N0 = Mo;
function Cl(t, e) {
    const r = t.a / 255
      , n = e + '="' + t.hex + '"';
    return r < 1 ? n + " " + e + '-opacity="' + r.toFixed(2).slice(1) + '"' : n
}
function _s(t, e, r) {
    let n = t + e;
    return typeof r < "u" && (n += " " + r),
    n
}
function k0(t, e, r) {
    let n = ""
      , i = 0
      , s = !1
      , o = 0;
    for (let a = 0; a < t.length; a++) {
        const l = Math.floor(a % e)
          , u = Math.floor(a / e);
        !l && !s && (s = !0),
        t[a] ? (o++,
        a > 0 && l > 0 && t[a - 1] || (n += s ? _s("M", l + r, .5 + u + r) : _s("m", i, 0),
        i = 0,
        s = !1),
        l + 1 < e && t[a + 1] || (n += _s("h", o),
        o = 0)) : i++
    }
    return n
}
Fc.render = function(e, r, n) {
    const i = N0.getOptions(r)
      , s = e.modules.size
      , o = e.modules.data
      , a = s + i.margin * 2
      , l = i.color.light.a ? "<path " + Cl(i.color.light, "fill") + ' d="M0 0h' + a + "v" + a + 'H0z"/>' : ""
      , u = "<path " + Cl(i.color.dark, "stroke") + ' d="' + k0(o, s, i.margin) + '"/>'
      , c = 'viewBox="0 0 ' + a + " " + a + '"'
      , d = '<svg xmlns="http://www.w3.org/2000/svg" ' + (i.width ? 'width="' + i.width + '" height="' + i.width + '" ' : "") + c + ' shape-rendering="crispEdges">' + l + u + `</svg>
`;
    return typeof n == "function" && n(null, d),
    d
}
;
const B0 = Xm
  , Ws = Sc
  , Uc = Dc
  , D0 = Fc;
function Po(t, e, r, n, i) {
    const s = [].slice.call(arguments, 1)
      , o = s.length
      , a = typeof s[o - 1] == "function";
    if (!a && !B0())
        throw new Error("Callback required as last argument");
    if (a) {
        if (o < 2)
            throw new Error("Too few arguments provided");
        o === 2 ? (i = r,
        r = e,
        e = n = void 0) : o === 3 && (e.getContext && typeof i > "u" ? (i = n,
        n = void 0) : (i = n,
        n = r,
        r = e,
        e = void 0))
    } else {
        if (o < 1)
            throw new Error("Too few arguments provided");
        return o === 1 ? (r = e,
        e = n = void 0) : o === 2 && !e.getContext && (n = r,
        r = e,
        e = void 0),
        new Promise(function(l, u) {
            try {
                const c = Ws.create(r, n);
                l(t(c, e, n))
            } catch (c) {
                u(c)
            }
        }
        )
    }
    try {
        const l = Ws.create(r, n);
        i(null, t(l, e, n))
    } catch (l) {
        i(l)
    }
}
dn.create = Ws.create;
dn.toCanvas = Po.bind(null, Uc.render);
dn.toDataURL = Po.bind(null, Uc.renderToDataURL);
dn.toString = Po.bind(null, function(t, e, r) {
    return D0.render(t, r)
});
function F0() {
    const t = Hr()
      , e = {}
      , r = t.commentScene || t.comment_scene;
    r != null && r !== "" && (e.commentScene = Number(r));
    const n = t.entryScene || t.entry_scene;
    n != null && n !== "" && (e.entryScene = Number(n));
    const i = t.entryCardType || t.entry_card_type;
    i != null && i !== "" && (e.entryCardType = Number(i));
    const s = t.requestScene || t.request_scene;
    return s != null && s !== "" && (e.requestScene = Number(s)),
    e
}
function U0(t) {
    var r, n, i, s;
    const e = F0();
    return t ? {
        commentScene: (r = e.commentScene) != null ? r : t.commentScene,
        entryScene: (n = e.entryScene) != null ? n : t.entryScene,
        entryCardType: (i = e.entryCardType) != null ? i : t.entryCardType,
        requestScene: (s = e.requestScene) != null ? s : t.requestScene
    } : e
}
const j0 = 40;
function jc(t) {
    const {sceneInfoRef: e, refreshSceneInfo: r} = t
      , {isMobile: n} = ho()
      , i = Ie(new Map)
      , s = Ie(!1)
      , o = () => U0(e.value)
      , a = () => {
        i.value.clear()
    }
      , l = () => {
        const p = e.value;
        return !(p != null && p.dynamicExportId) || !(p != null && p.expiredTime) ? !0 : Math.floor(Date.now() / 1e3) >= p.expiredTime
    }
      , u = async () => {
        var p;
        if (!l())
            return e.value.dynamicExportId;
        try {
            await r()
        } catch {}
        return ((p = e.value) == null ? void 0 : p.dynamicExportId) || ""
    }
      , c = async (p=0, f="feed") => {
        var I;
        const b = await u();
        if (!b)
            return "";
        const C = (I = o().entryScene) != null ? I : j0
          , E = "https://channels.weixin.qq.com/mobile/commonFinderJsApi.html";
        let R;
        f === "profile" ? R = {
            action: "openFinderProfile",
            exportUsername: b,
            commentScene: C,
            reportExtraInfo: '{"sys_scene":1}',
            popWebView: 1,
            profileEnterActionType: p
        } : R = {
            action: "openFinderFeed",
            feedID: b,
            notGetReleatedList: 1,
            shareScene: C,
            commentScene: C,
            reportExtraInfo: '{"sys_scene":1}',
            popWebView: 1,
            feedEnterActionType: p
        };
        const T = new URLSearchParams({
            api: "openFinderView",
            extInfo: JSON.stringify(R)
        });
        return `${E}?${T.toString()}`
    }
      , g = async p => {
        if (!p)
            throw new Error("URL\u4E0D\u80FD\u4E3A\u7A7A");
        try {
            return s.value = !0,
            await dn.toDataURL(p, {
                width: 200,
                margin: 2,
                color: {
                    dark: "#000000",
                    light: "#FFFFFF"
                },
                errorCorrectionLevel: "M"
            })
        } catch {
            throw new Error("\u4E8C\u7EF4\u7801\u751F\u6210\u5931\u8D25")
        } finally {
            s.value = !1
        }
    }
      , d = async (p=0, f="feed") => {
        const b = `${f}-${p}`;
        if (i.value.has(b)) {
            if (!l())
                return i.value.get(b);
            i.value.delete(b)
        }
        try {
            const _ = await c(p, f);
            if (!_ || n.value)
                return "";
            const C = await g(_);
            return i.value.set(b, C),
            C
        } catch {
            return ""
        }
    }
      , m = async (p="feed") => {
        await d(0, p)
    }
      , v = async (p=0, f="feed") => {
        if (!n.value)
            return;
        const b = await u();
        if (!b)
            return;
        const _ = o()
          , C = R => {
            const T = {
                ...R
            };
            return _.commentScene !== void 0 && (T.commentScene = _.commentScene),
            _.entryScene !== void 0 && (T.entryScene = _.entryScene),
            _.entryCardType !== void 0 && (T.entryCardType = _.entryCardType),
            _.requestScene !== void 0 && (T.requestScene = _.requestScene),
            Object.entries(T).map( ([I,h]) => `${I}=${h}`).join("&")
        }
        ;
        let E = "";
        if (f === "profile") {
            const R = C({
                exportUsername: b,
                actionType: p
            });
            E = `weixin://biz/finder/openFinderProfile/${encodeURIComponent(R)}`
        } else {
            const R = C({
                exportId: b,
                actionType: p
            });
            E = `weixin://biz/finder/openFinderFeed/${encodeURIComponent(R)}`
        }
        try {
            const R = document.createElement("a");
            R.href = E,
            R.style.display = "none",
            document.body.appendChild(R),
            R.click(),
            document.body.removeChild(R)
        } catch {}
    }
    ;
    return {
        isGenerating: at( () => s.value),
        isMobile: n,
        generateWeixinUrl: c,
        generateQRCode: g,
        getQRCodeByScene: d,
        cacheQRCode: m,
        jumpToWeixin: v,
        clearQRCodeCache: a
    }
}
const $0 = () => {
    const t = Ie(0)
      , e = () => {
        const r = document.createElement("div");
        r.style.cssText = `
      position: fixed;
      top: 0;
      left: 0;
      width: 1px;
      height: 1px;
      padding-top: env(safe-area-inset-top);
      visibility: hidden;
      pointer-events: none;
    `,
        document.body.appendChild(r);
        const n = window.getComputedStyle(r)
          , {paddingTop: i} = n;
        document.body.removeChild(r);
        const s = parseFloat(i) || 0;
        if (s === 0) {
            const {appStatusBarHeight: o} = rc();
            return o && (/Android/i.test(navigator.userAgent) ? Math.round(o / window.devicePixelRatio) : o) || 24
        }
        return s
    }
    ;
    return _r( () => {
        t.value = e()
    }
    ),
    {
        safeAreaTop: Lt(t)
    }
}
  , ny = () => {
    const t = Ie(0)
      , e = () => {
        const r = document.createElement("div");
        r.style.cssText = `
      position: fixed;
      bottom: 0;
      left: 0;
      width: 1px;
      height: 1px;
      padding-bottom: env(safe-area-inset-bottom);
      visibility: hidden;
      pointer-events: none;
    `,
        document.body.appendChild(r);
        const n = window.getComputedStyle(r)
          , {paddingBottom: i} = n;
        return document.body.removeChild(r),
        parseFloat(i) || 0
    }
    ;
    return _r( () => {
        t.value = e()
    }
    ),
    {
        safeAreaBottom: Lt(t)
    }
}
;
var H0 = (t => (t.WECHAT = "WECHAT",
t.DEFAULT = "DEFAULT",
t))(H0 || {});
function iy() {
    const t = navigator.userAgent.toLowerCase();
    return t.includes("micromessenger") && !t.includes("wxwork") ? "WECHAT" : "DEFAULT"
}
function q0() {
    window.location.href = "webview-close://close"
}
function sy(t) {
    var e;
    (e = window.WeixinJSBridge) != null && e.invoke ? t() : document.addEventListener("WeixinJSBridgeReady", t, !1)
}
const V0 = {
    name: "ChannelsFilledColorfulIcon.vue",
    props: {
        width: {
            type: [String, Number],
            default: ""
        },
        height: {
            type: [String, Number],
            default: ""
        },
        color: {
            type: String,
            default: ""
        }
    },
    computed: {
        styles() {
            const t = Number(this.width) ? `calc(${this.width}px * var(--liteapp-font-scale, 1))` : this.width
              , e = Number(this.height) ? `calc(${this.height}px * var(--liteapp-font-scale, 1))` : this.height;
            return {
                width: t,
                height: e
            }
        }
    }
};
function K0(t, e, r, n, i, s) {
    return ge(),
    _e("i", {
        class: "i-weui:channels-filled-colorful",
        style: He(s.styles)
    }, null, 4)
}
const W0 = tt(V0, [["render", K0], ["__scopeId", "data-v-29ca2cb5"]])
  , z0 = {
    class: "finder-logo"
}
  , G0 = nr({
    __name: "FinderLogo",
    setup(t) {
        return (e, r) => (ge(),
        _e("div", z0, [Ee(W0, {
            width: "24px",
            height: "24px",
            color: "#fff"
        }), r[0] || (r[0] = ue("div", {
            class: "finder-logo-text"
        }, " \u89C6\u9891\u53F7 ", -1))]))
    }
});
const Y0 = tt(G0, [["__scopeId", "data-v-bf2e0991"]]);
const J0 = {
    name: "ArrowLeftRegularIcon.vue",
    props: {
        width: {
            type: [String, Number],
            default: ""
        },
        height: {
            type: [String, Number],
            default: ""
        },
        color: {
            type: String,
            default: ""
        }
    },
    computed: {
        styles() {
            const t = Number(this.width) ? `calc(${this.width}px * var(--liteapp-font-scale, 1))` : this.width
              , e = Number(this.height) ? `calc(${this.height}px * var(--liteapp-font-scale, 1))` : this.height;
            return {
                width: t,
                height: e,
                color: this.color,
                "-liteapp-svg-mask": this.color || "#000"
            }
        }
    }
};
function Q0(t, e, r, n, i, s) {
    return ge(),
    _e("i", {
        class: "i-weui:arrow-left-regular",
        style: He(s.styles)
    }, null, 4)
}
const X0 = tt(J0, [["render", Q0], ["__scopeId", "data-v-db7b7c5d"]]);
const Z0 = {
    name: "Dot3RegularIcon.vue",
    props: {
        width: {
            type: [String, Number],
            default: ""
        },
        height: {
            type: [String, Number],
            default: ""
        },
        color: {
            type: String,
            default: ""
        }
    },
    computed: {
        styles() {
            const t = Number(this.width) ? `calc(${this.width}px * var(--liteapp-font-scale, 1))` : this.width
              , e = Number(this.height) ? `calc(${this.height}px * var(--liteapp-font-scale, 1))` : this.height;
            return {
                width: t,
                height: e,
                color: this.color,
                "-liteapp-svg-mask": this.color || "#000"
            }
        }
    }
};
function ev(t, e, r, n, i, s) {
    return ge(),
    _e("i", {
        class: "i-weui:dot-3-regular",
        style: He(s.styles)
    }, null, 4)
}
const Il = tt(Z0, [["render", ev], ["__scopeId", "data-v-612962ca"]])
  , tv = {
    class: "navbar-content mobile-content"
}
  , rv = {
    class: "navbar-center"
}
  , nv = {
    class: "navbar-content pc-content"
}
  , iv = {
    class: "navbar-left"
}
  , sv = {
    key: 0,
    class: "dropdown-menu"
}
  , ov = nr({
    __name: "NavBar",
    props: {
        mode: {
            default: "transparent"
        },
        hideBack: {
            type: Boolean,
            default: !1
        },
        hideMore: {
            type: Boolean,
            default: !1
        }
    },
    emits: ["back", "more", "menuItemClick"],
    setup(t, {emit: e}) {
        const r = t
          , n = e
          , {isMobile: i} = ho()
          , {safeAreaTop: s} = $0()
          , o = Ie(!1)
          , a = () => {
            n("back"),
            q0()
        }
          , l = () => {
            n("more")
        }
          , u = () => {
            o.value = !o.value,
            o.value && n("more")
        }
          , c = () => {
            o.value = !1
        }
          , g = () => {
            c(),
            n("menuItemClick")
        }
        ;
        return (d, m) => Is(i) ? (ge(),
        _e("div", {
            key: 0,
            class: Bt(["custom-navbar mobile-navbar", {
                "navbar-solid": r.mode === "solid"
            }]),
            style: He({
                paddingTop: `${Is(s)}px`
            })
        }, [ue("div", tv, [ue("div", {
            class: "navbar-left",
            style: He(r.hideBack ? {
                visibility: "hidden"
            } : void 0),
            onClick: m[0] || (m[0] = v => !r.hideBack && a())
        }, [Ee(X0, {
            width: "12px",
            height: "24px",
            color: r.mode === "solid" ? "var(--weui-FG-0)" : "#fff"
        }, null, 8, ["color"])], 4), ue("div", rv, [nd(d.$slots, "center", {}, void 0, !0)]), ue("div", {
            class: "navbar-right",
            style: He(r.hideMore ? {
                visibility: "hidden"
            } : void 0),
            onClick: m[1] || (m[1] = v => !r.hideMore && l())
        }, [Ee(Il, {
            width: "24px",
            height: "24px",
            color: r.mode === "solid" ? "var(--weui-FG-0)" : "#fff"
        }, null, 8, ["color"])], 4)])], 6)) : (ge(),
        _e("div", {
            key: 1,
            class: Bt(["custom-navbar pc-navbar", {
                "navbar-solid": r.mode === "solid"
            }])
        }, [ue("div", nv, [ue("div", iv, [Ee(Y0)]), ue("div", {
            class: "navbar-right-wrapper",
            style: He(r.hideMore ? {
                visibility: "hidden"
            } : void 0)
        }, [ue("div", {
            class: "navbar-right",
            onClick: m[2] || (m[2] = v => !r.hideMore && u())
        }, [Ee(Il, {
            width: "24px",
            height: "24px",
            color: d.mode === "solid" ? "var(--weui-FG-0)" : "#fff"
        }, null, 8, ["color"])]), Ee(ks, {
            name: "menu-fade"
        }, {
            default: Fn( () => [o.value && !r.hideMore ? (ge(),
            _e("div", sv, [m[3] || (m[3] = ue("div", {
                class: "dropdown-arrow"
            }, null, -1)), ue("div", {
                class: "dropdown-menu-item",
                onClick: g
            }, " \u6295\u8BC9 ")])) : Qt("", !0)]),
            _: 1
        })], 4)]), o.value && !r.hideMore ? (ge(),
        _e("div", {
            key: 0,
            class: "menu-mask",
            onClick: c
        })) : Qt("", !0)], 2))
    }
});
const oy = tt(ov, [["__scopeId", "data-v-441a50a4"]])
  , av = t => t == null ? void 0 : t.replace(/^http:\/\//, "https://")
  , lv = ["src"]
  , uv = "https://res.wx.qq.com/t/fed_upload/c36cbe1c-cf10-40f2-8913-ffff5a59ac19/%E5%A4%B4%E5%83%8F%E5%8D%A0%E4%BD%8D%E5%9B%BE.svg"
  , cv = nr({
    __name: "Avatar",
    props: {
        src: {},
        size: {
            default: 44
        }
    },
    setup(t) {
        const e = t
          , r = at( () => av(e.src))
          , n = Ie(!1)
          , i = Ie(!1)
          , s = at( () => n.value || !r.value || r.value.startsWith("/") ? uv : r.value)
          , o = () => {
            n.value = !0
        }
          , a = () => {
            i.value = !0
        }
        ;
        return (l, u) => (ge(),
        _e("div", {
            class: "avatar",
            style: He({
                width: `${l.size}px`,
                height: `${l.size}px`
            })
        }, [ue("img", {
            src: s.value,
            style: He({
                width: `${l.size}px`,
                height: `${l.size}px`
            }),
            class: Bt({
                loading: !i.value
            }),
            draggable: "false",
            onError: o,
            onLoad: a
        }, null, 46, lv)], 4))
    }
});
const ay = tt(cv, [["__scopeId", "data-v-7bbf556d"]]);
const fv = {
    name: "HeartRegularIcon.vue",
    props: {
        width: {
            type: [String, Number],
            default: ""
        },
        height: {
            type: [String, Number],
            default: ""
        },
        color: {
            type: String,
            default: ""
        }
    },
    computed: {
        styles() {
            const t = Number(this.width) ? `calc(${this.width}px * var(--liteapp-font-scale, 1))` : this.width
              , e = Number(this.height) ? `calc(${this.height}px * var(--liteapp-font-scale, 1))` : this.height;
            return {
                width: t,
                height: e,
                color: this.color,
                "-liteapp-svg-mask": this.color || "#000"
            }
        }
    }
};
function dv(t, e, r, n, i, s) {
    return ge(),
    _e("i", {
        class: "i-weui:heart-regular",
        style: He(s.styles)
    }, null, 4)
}
const ly = tt(fv, [["render", dv], ["__scopeId", "data-v-6bb56e15"]]);
const hv = {
    name: "XmarkRegularIcon.vue",
    props: {
        width: {
            type: [String, Number],
            default: ""
        },
        height: {
            type: [String, Number],
            default: ""
        },
        color: {
            type: String,
            default: ""
        }
    },
    computed: {
        styles() {
            const t = Number(this.width) ? `calc(${this.width}px * var(--liteapp-font-scale, 1))` : this.width
              , e = Number(this.height) ? `calc(${this.height}px * var(--liteapp-font-scale, 1))` : this.height;
            return {
                width: t,
                height: e,
                color: this.color,
                "-liteapp-svg-mask": this.color || "#000"
            }
        }
    }
};
function pv(t, e, r, n, i, s) {
    return ge(),
    _e("i", {
        class: "i-weui:xmark-regular",
        style: He(s.styles)
    }, null, 4)
}
const gv = tt(hv, [["render", pv], ["__scopeId", "data-v-d013873d"]])
  , mv = {
    class: "qr-code-container"
}
  , vv = ["src"]
  , yv = {
    key: 1,
    class: "qr-code-placeholder"
}
  , bv = {
    class: "qr-modal-description"
}
  , wv = nr({
    __name: "QRCodeModal",
    props: {
        visible: {
            type: Boolean
        },
        qrCodeDataUrl: {},
        text: {
            default: "\u5F53\u524D\u4EC5\u652F\u6301\u6D4F\u89C8\u89C6\u9891\uFF0C\u66F4\u591A\u529F\u80FD\u53EF\u901A\u8FC7\u5FAE\u4FE1\u626B\u7801\u4F7F\u7528\u3002"
        }
    },
    emits: ["close"],
    setup(t, {emit: e}) {
        const r = e
          , n = () => {
            r("close")
        }
          , i = s => {
            s.target === s.currentTarget && n()
        }
        ;
        return (s, o) => (ge(),
        Xr(Uf, {
            to: "body"
        }, [s.visible ? (ge(),
        _e("div", {
            key: 0,
            class: "qr-modal-overlay",
            onClick: i
        }, [ue("div", {
            class: "qr-modal-content",
            onClick: o[0] || (o[0] = Sh( () => {}
            , ["stop"]))
        }, [ue("button", {
            class: "qr-modal-close",
            "aria-label": "\u5173\u95ED",
            onClick: n
        }, [Ee(gv, {
            width: "16",
            height: "16"
        })]), ue("div", mv, [s.qrCodeDataUrl ? (ge(),
        _e("img", {
            key: 0,
            src: s.qrCodeDataUrl,
            alt: "\u4E8C\u7EF4\u7801",
            class: "qr-code-image"
        }, null, 8, vv)) : (ge(),
        _e("div", yv, " \u4E8C\u7EF4\u7801\u52A0\u8F7D\u4E2D... "))]), ue("div", bv, Wr(s.text), 1)])])) : Qt("", !0)]))
    }
});
const uy = tt(wv, [["__scopeId", "data-v-4e4644cc"]])
  , _v = {
    key: 0,
    class: "weui-actionsheet weui-actionsheet_toggle"
}
  , Ev = {
    class: "weui-actionsheet__menu"
}
  , xv = ["onClick"]
  , Sv = {
    class: "weui-actionsheet__cell-label"
}
  , Cv = {
    key: 0,
    class: "weui-actionsheet__cell-desc"
}
  , Iv = nr({
    __name: "ActionSheet",
    props: {
        items: {}
    },
    emits: ["close"],
    setup(t, {expose: e, emit: r}) {
        const n = r
          , i = Ie(!1)
          , s = () => {
            i.value = !0
        }
          , o = () => {
            i.value = !1,
            n("close")
        }
          , a = () => {
            o()
        }
          , l = c => {
            var g;
            (g = c.onClick) == null || g.call(c),
            o()
        }
          , u = () => {
            o()
        }
        ;
        return e({
            show: s,
            hide: o
        }),
        (c, g) => (ge(),
        _e($e, null, [Ee(ks, {
            name: "weui-fade"
        }, {
            default: Fn( () => [i.value ? (ge(),
            _e("div", {
                key: 0,
                class: "weui-mask",
                onClick: a
            })) : Qt("", !0)]),
            _: 1
        }), Ee(ks, {
            name: "weui-actionsheet"
        }, {
            default: Fn( () => [i.value ? (ge(),
            _e("div", _v, [ue("div", Ev, [(ge(!0),
            _e($e, null, rd(c.items, (d, m) => (ge(),
            _e("div", {
                key: m,
                class: "weui-actionsheet__cell",
                onClick: v => l(d)
            }, [ue("div", Sv, Wr(d.label), 1), d.desc ? (ge(),
            _e("div", Cv, Wr(d.desc), 1)) : Qt("", !0)], 8, xv))), 128))]), ue("div", {
                class: "weui-actionsheet__action"
            }, [ue("div", {
                class: "weui-actionsheet__cell",
                onClick: u
            }, " \u53D6\u6D88 ")])])) : Qt("", !0)]),
            _: 1
        })], 64))
    }
});
const cy = tt(Iv, [["__scopeId", "data-v-d6884878"]]);
var $c = {
    exports: {}
};
/*!
 * weui.js v1.2.27 (https://weui.io)
 * Copyright 2025, wechat ui team
 * MIT license
 */
(function(t, e) {
    (function(r, n) {
        t.exports = n()
    }
    )(Pr, function() {
        return function(r) {
            function n(s) {
                if (i[s])
                    return i[s].exports;
                var o = i[s] = {
                    exports: {},
                    id: s,
                    loaded: !1
                };
                return r[s].call(o.exports, o, o.exports, n),
                o.loaded = !0,
                o.exports
            }
            var i = {};
            return n.m = r,
            n.c = i,
            n.p = "",
            n(0)
        }([function(r, n, i) {
            function s(Y) {
                return Y && Y.__esModule ? Y : {
                    default: Y
                }
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var o = i(1)
              , a = s(o)
              , l = i(7)
              , u = s(l)
              , c = i(8)
              , g = s(c)
              , d = i(9)
              , m = s(d)
              , v = i(11)
              , p = s(v)
              , f = i(13)
              , b = s(f)
              , _ = i(15)
              , C = s(_)
              , E = i(17)
              , R = s(E)
              , T = i(18)
              , I = s(T)
              , h = i(19)
              , y = s(h)
              , w = i(20)
              , O = s(w)
              , A = i(24)
              , k = i(30)
              , V = s(k)
              , $ = i(32)
              , D = s($);
            n.default = {
                dialog: a.default,
                alert: u.default,
                confirm: g.default,
                toast: m.default,
                loading: p.default,
                actionSheet: b.default,
                topTips: C.default,
                searchBar: R.default,
                tab: I.default,
                form: y.default,
                uploader: O.default,
                picker: A.picker,
                datePicker: A.datePicker,
                gallery: V.default,
                slider: D.default
            },
            r.exports = n.default
        }
        , function(r, n, i) {
            function s(d) {
                return d && d.__esModule ? d : {
                    default: d
                }
            }
            function o() {
                function d(C) {
                    d = l.default.noop,
                    _.addClass("weui-animate-fade-out"),
                    b.addClass("weui-animate-fade-out").on("animationend webkitAnimationEnd", function() {
                        f.remove(),
                        g = !1,
                        C && C()
                    })
                }
                function m(C) {
                    d(C)
                }
                var v = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : {};
                if (g)
                    return g;
                var p = l.default.os.android;
                v = l.default.extend({
                    title: null,
                    content: "",
                    className: "",
                    buttons: [{
                        label: "\u786E\u5B9A",
                        type: "primary",
                        onClick: l.default.noop
                    }],
                    isAndroid: p
                }, v);
                var f = (0,
                l.default)(l.default.render(c.default, v))
                  , b = f.find(".weui-dialog")
                  , _ = f.find(".weui-mask");
                return (0,
                l.default)("body").append(f),
                _.addClass("weui-animate-fade-in"),
                b.addClass("weui-animate-fade-in").on("animationend webkitAnimationEnd", function(C) {
                    C.target.focus()
                }),
                f.on("click", ".weui-dialog__btn", function(C) {
                    var E = (0,
                    l.default)(this).index();
                    v.buttons[E].onClick ? v.buttons[E].onClick.call(this, C) !== !1 && m() : m()
                }).on("touchmove", function(C) {
                    C.stopPropagation(),
                    C.preventDefault()
                }),
                g = f[0],
                g.hide = m,
                g
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a)
              , u = i(6)
              , c = s(u)
              , g = void 0;
            n.default = o,
            r.exports = n.default
        }
        , function(r, n, i) {
            function s(d) {
                return d && d.__esModule ? d : {
                    default: d
                }
            }
            function o(d) {
                var m = this.os = {}
                  , v = d.match(/(Android);?[\s\/]+([\d.]+)?/);
                v && (m.android = !0,
                m.version = v[2])
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = typeof Symbol == "function" && typeof Symbol.iterator == "symbol" ? function(d) {
                return typeof d
            }
            : function(d) {
                return d && typeof Symbol == "function" && d.constructor === Symbol && d !== Symbol.prototype ? "symbol" : typeof d
            }
            ;
            i(3);
            var l = i(4)
              , u = s(l)
              , c = i(5)
              , g = s(c);
            o.call(g.default, navigator.userAgent),
            (0,
            u.default)(g.default.fn, {
                append: function(d) {
                    return d instanceof HTMLElement || (d = d[0]),
                    this.forEach(function(m) {
                        m.appendChild(d)
                    }),
                    this
                },
                remove: function() {
                    return this.forEach(function(d) {
                        d.parentNode.removeChild(d)
                    }),
                    this
                },
                find: function(d) {
                    return (0,
                    g.default)(d, this)
                },
                addClass: function(d) {
                    return this.forEach(function(m) {
                        m.classList.add(d)
                    }),
                    this
                },
                removeClass: function(d) {
                    return this.forEach(function(m) {
                        m.classList.remove(d)
                    }),
                    this
                },
                eq: function(d) {
                    return (0,
                    g.default)(this[d])
                },
                show: function() {
                    return this.forEach(function(d) {
                        d.style.display = "block"
                    }),
                    this
                },
                hide: function() {
                    return this.forEach(function(d) {
                        d.style.display = "none"
                    }),
                    this
                },
                html: function(d) {
                    return this.forEach(function(m) {
                        m.innerHTML = d
                    }),
                    this
                },
                css: function(d) {
                    var m = this;
                    return Object.keys(d).forEach(function(v) {
                        m.forEach(function(p) {
                            p.style[v] = d[v]
                        })
                    }),
                    this
                },
                on: function(d, m, v) {
                    var p = typeof m == "string" && typeof v == "function";
                    return p || (v = m),
                    this.forEach(function(f) {
                        d.split(" ").forEach(function(b) {
                            f.addEventListener(b, function(_) {
                                p ? this.contains(_.target.closest(m)) && v.call(_.target, _) : v.call(this, _)
                            })
                        })
                    }),
                    this
                },
                off: function(d, m, v) {
                    return typeof m == "function" && (v = m,
                    m = null),
                    this.forEach(function(p) {
                        d.split(" ").forEach(function(f) {
                            typeof m == "string" ? p.querySelectorAll(m).forEach(function(b) {
                                b.removeEventListener(f, v)
                            }) : p.removeEventListener(f, v)
                        })
                    }),
                    this
                },
                index: function() {
                    var d = this[0]
                      , m = d.parentNode;
                    return Array.prototype.indexOf.call(m.children, d)
                },
                offAll: function() {
                    var d = this;
                    return this.forEach(function(m, v) {
                        var p = m.cloneNode(!0);
                        m.parentNode.replaceChild(p, m),
                        d[v] = p
                    }),
                    this
                },
                val: function() {
                    var d = arguments;
                    return arguments.length ? (this.forEach(function(m) {
                        m.value = d[0]
                    }),
                    this) : this[0].value
                },
                attr: function() {
                    var d = arguments;
                    if (a(arguments[0]) == "object") {
                        var m = arguments[0]
                          , v = this;
                        return Object.keys(m).forEach(function(p) {
                            v.forEach(function(f) {
                                f.setAttribute(p, m[p])
                            })
                        }),
                        this
                    }
                    return typeof arguments[0] == "string" && arguments.length < 2 ? this[0].getAttribute(arguments[0]) : (this.forEach(function(p) {
                        p.setAttribute(d[0], d[1])
                    }),
                    this)
                }
            }),
            (0,
            u.default)(g.default, {
                extend: u.default,
                noop: function() {},
                render: function(d, m) {
                    var v = "var p=[];with(this){p.push('" + d.replace(/[\r\t\n]/g, " ").split("<%").join("	").replace(/((^|%>)[^\t]*)'/g, "$1\r").replace(/\t=(.*?)%>/g, "',$1,'").split("	").join("');").split("%>").join("p.push('").split("\r").join("\\'") + "');}return p.join('');";
                    return new Function(v).apply(m)
                },
                getStyle: function(d, m) {
                    var v, p = (d.ownerDocument || document).defaultView;
                    return p && p.getComputedStyle ? (m = m.replace(/([A-Z])/g, "-$1").toLowerCase(),
                    p.getComputedStyle(d, null).getPropertyValue(m)) : d.currentStyle ? (m = m.replace(/\-(\w)/g, function(f, b) {
                        return b.toUpperCase()
                    }),
                    v = d.currentStyle[m],
                    /^\d+(em|pt|%|ex)?$/i.test(v) ? function(f) {
                        var b = d.style.left
                          , _ = d.runtimeStyle.left;
                        return d.runtimeStyle.left = d.currentStyle.left,
                        d.style.left = f || 0,
                        f = d.style.pixelLeft + "px",
                        d.style.left = b,
                        d.runtimeStyle.left = _,
                        f
                    }(v) : v) : void 0
                }
            }),
            n.default = g.default,
            r.exports = n.default
        }
        , function(r, n) {
            (function(i) {
                typeof i.matches != "function" && (i.matches = i.msMatchesSelector || i.mozMatchesSelector || i.webkitMatchesSelector || function(s) {
                    for (var o = this, a = (o.document || o.ownerDocument).querySelectorAll(s), l = 0; a[l] && a[l] !== o; )
                        ++l;
                    return Boolean(a[l])
                }
                ),
                typeof i.closest != "function" && (i.closest = function(s) {
                    for (var o = this; o && o.nodeType === 1; ) {
                        if (o.matches(s))
                            return o;
                        o = o.parentNode
                    }
                    return null
                }
                )
            }
            )(window.Element.prototype)
        }
        , function(r, n) {
            /*
object-assign
(c) Sindre Sorhus
@license MIT
*/
            function i(u) {
                if (u == null)
                    throw new TypeError("Object.assign cannot be called with null or undefined");
                return Object(u)
            }
            function s() {
                try {
                    if (!Object.assign)
                        return !1;
                    var u = new String("abc");
                    if (u[5] = "de",
                    Object.getOwnPropertyNames(u)[0] === "5")
                        return !1;
                    for (var c = {}, g = 0; g < 10; g++)
                        c["_" + String.fromCharCode(g)] = g;
                    var d = Object.getOwnPropertyNames(c).map(function(v) {
                        return c[v]
                    });
                    if (d.join("") !== "0123456789")
                        return !1;
                    var m = {};
                    return "abcdefghijklmnopqrst".split("").forEach(function(v) {
                        m[v] = v
                    }),
                    Object.keys(Object.assign({}, m)).join("") === "abcdefghijklmnopqrst"
                } catch {
                    return !1
                }
            }
            var o = Object.getOwnPropertySymbols
              , a = Object.prototype.hasOwnProperty
              , l = Object.prototype.propertyIsEnumerable;
            r.exports = s() ? Object.assign : function(u, c) {
                for (var g, d, m = i(u), v = 1; v < arguments.length; v++) {
                    g = Object(arguments[v]);
                    for (var p in g)
                        a.call(g, p) && (m[p] = g[p]);
                    if (o) {
                        d = o(g);
                        for (var f = 0; f < d.length; f++)
                            l.call(g, d[f]) && (m[d[f]] = g[d[f]])
                    }
                }
                return m
            }
        }
        , function(r, n, i) {
            var s, o;
            (function(a, l) {
                l = function(u, c, g) {
                    function d(m, v, p) {
                        return p = Object.create(d.fn),
                        m && p.push.apply(p, m[c] ? [m] : "" + m === m ? /</.test(m) ? ((v = u.createElement(v || c)).innerHTML = m,
                        v.children) : v ? (v = d(v)[0]) ? v[g](m) : p : u[g](m) : typeof m == "function" ? u.readyState[7] ? m() : u[c]("DOMContentLoaded", m) : m),
                        p
                    }
                    return d.fn = [],
                    d.one = function(m, v) {
                        return d(m, v)[0] || null
                    }
                    ,
                    d
                }(document, "addEventListener", "querySelectorAll"),
                s = [],
                o = function() {
                    return l
                }
                .apply(n, s),
                o !== void 0 && (r.exports = o)
            }
            )()
        }
        , function(r, n) {
            r.exports = `<div class="<%=className%>"> <div class=weui-mask></div> <div class=weui-dialog role=dialog aria-modal=true tabindex=-1> <% if (title) { %> <div class=weui-dialog__hd><strong class=weui-dialog__title><%=title%></strong></div> <% } %> <div class=weui-dialog__bd><%=content%></div> <div class=weui-dialog__ft> <% for(var i = 0; i < buttons.length; i++){ %> <a href=javascript:; class="weui-dialog__btn weui-dialog__btn_<%=buttons[i]['type']%>" role=button><%=buttons[i]['label']%></a> <% } %> </div> </div> </div> `
        }
        , function(r, n, i) {
            function s(d) {
                return d && d.__esModule ? d : {
                    default: d
                }
            }
            function o() {
                var d = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : ""
                  , m = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : u.default.noop
                  , v = arguments[2];
                return (typeof m > "u" ? "undefined" : a(m)) === "object" && (v = m,
                m = u.default.noop),
                v = u.default.extend({
                    content: d,
                    buttons: [{
                        label: "\u786E\u5B9A",
                        type: "primary",
                        onClick: m
                    }]
                }, v),
                (0,
                g.default)(v)
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = typeof Symbol == "function" && typeof Symbol.iterator == "symbol" ? function(d) {
                return typeof d
            }
            : function(d) {
                return d && typeof Symbol == "function" && d.constructor === Symbol && d !== Symbol.prototype ? "symbol" : typeof d
            }
              , l = i(2)
              , u = s(l)
              , c = i(1)
              , g = s(c);
            n.default = o,
            r.exports = n.default
        }
        , function(r, n, i) {
            function s(d) {
                return d && d.__esModule ? d : {
                    default: d
                }
            }
            function o() {
                var d = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : ""
                  , m = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : u.default.noop
                  , v = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : u.default.noop
                  , p = arguments[3];
                return (typeof m > "u" ? "undefined" : a(m)) === "object" ? (p = m,
                m = u.default.noop) : (typeof v > "u" ? "undefined" : a(v)) === "object" && (p = v,
                v = u.default.noop),
                p = u.default.extend({
                    content: d,
                    buttons: [{
                        label: "\u53D6\u6D88",
                        type: "default",
                        onClick: v
                    }, {
                        label: "\u786E\u5B9A",
                        type: "primary",
                        onClick: m
                    }]
                }, p),
                (0,
                g.default)(p)
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = typeof Symbol == "function" && typeof Symbol.iterator == "symbol" ? function(d) {
                return typeof d
            }
            : function(d) {
                return d && typeof Symbol == "function" && d.constructor === Symbol && d !== Symbol.prototype ? "symbol" : typeof d
            }
              , l = i(2)
              , u = s(l)
              , c = i(1)
              , g = s(c);
            n.default = o,
            r.exports = n.default
        }
        , function(r, n, i) {
            function s(d) {
                return d && d.__esModule ? d : {
                    default: d
                }
            }
            function o() {
                var d = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : ""
                  , m = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {};
                if (g)
                    return g;
                typeof m == "number" && (m = {
                    duration: m
                }),
                typeof m == "function" && (m = {
                    callback: m
                }),
                m = l.default.extend({
                    content: d,
                    duration: 3e3,
                    callback: l.default.noop,
                    className: "",
                    extClass: ""
                }, m);
                var v = (0,
                l.default)(l.default.render(c.default, m))
                  , p = v.find(".weui-toast")
                  , f = v.find(".weui-mask");
                return (0,
                l.default)("body").append(v),
                p.addClass("weui-animate-fade-in"),
                f.addClass("weui-animate-fade-in"),
                setTimeout(function() {
                    f.addClass("weui-animate-fade-out"),
                    p.addClass("weui-animate-fade-out").on("animationend webkitAnimationEnd", function() {
                        v.remove(),
                        g = !1,
                        m.callback()
                    })
                }, m.duration),
                g = v[0],
                v[0]
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a)
              , u = i(10)
              , c = s(u)
              , g = void 0;
            n.default = o,
            r.exports = n.default
        }
        , function(r, n) {
            r.exports = '<div class="<%= className %>" role=alert> <div class=weui-mask_transparent></div> <div class="weui-toast <%= extClass %>"> <i class="weui-icon_toast weui-icon-success-no-circle"></i> <p class=weui-toast__content><%=content%></p> </div> </div> '
        }
        , function(r, n, i) {
            function s(d) {
                return d && d.__esModule ? d : {
                    default: d
                }
            }
            function o() {
                function d(C) {
                    d = l.default.noop,
                    _.addClass("weui-animate-fade-out"),
                    b.addClass("weui-animate-fade-out").on("animationend webkitAnimationEnd", function() {
                        f.remove(),
                        g = !1,
                        C && C()
                    })
                }
                function m(C) {
                    d(C)
                }
                var v = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : ""
                  , p = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {};
                if (g)
                    return g;
                p = l.default.extend({
                    content: v,
                    className: ""
                }, p);
                var f = (0,
                l.default)(l.default.render(c.default, p))
                  , b = f.find(".weui-toast")
                  , _ = f.find(".weui-mask");
                return (0,
                l.default)("body").append(f),
                b.addClass("weui-animate-fade-in"),
                _.addClass("weui-animate-fade-in"),
                g = f[0],
                g.hide = m,
                g
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a)
              , u = i(12)
              , c = s(u)
              , g = void 0;
            n.default = o,
            r.exports = n.default
        }
        , function(r, n) {
            r.exports = '<div class="weui-loading_toast <%= className %>" role=alert> <div class=weui-mask_transparent></div> <div class=weui-toast> <span class="weui-primary-loading weui-icon_toast"> <span class=weui-primary-loading__dot></span> </span> <p class=weui-toast__content><%=content%></p> </div> </div> '
        }
        , function(r, n, i) {
            function s(d) {
                return d && d.__esModule ? d : {
                    default: d
                }
            }
            function o() {
                function d(E) {
                    d = l.default.noop,
                    _.addClass("weui-animate-slide-down"),
                    C.addClass("weui-animate-fade-out").on("animationend webkitAnimationEnd", function() {
                        b.remove(),
                        g = !1,
                        f.onClose(),
                        E && E()
                    })
                }
                function m(E) {
                    d(E)
                }
                var v = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : []
                  , p = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : []
                  , f = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : {};
                if (g)
                    return g;
                f = l.default.extend({
                    menus: v,
                    actions: p,
                    title: "",
                    className: "",
                    onClose: l.default.noop,
                    onClickMask: l.default.noop
                }, f);
                var b = (0,
                l.default)(l.default.render(c.default, f))
                  , _ = b.find(".weui-actionsheet")
                  , C = b.find(".weui-mask");
                return (0,
                l.default)("body").append(b),
                l.default.getStyle(_[0], "transform"),
                C.addClass("weui-animate-fade-in").on("click", function() {
                    f.onClickMask(),
                    m()
                }).on("touchmove", function(E) {
                    E.preventDefault()
                }),
                _.addClass("weui-animate-slide-up").on("animationend webkitAnimationEnd", function(E) {
                    E.target.focus()
                }),
                b.find(".weui-actionsheet__menu").on("click", ".weui-actionsheet__cell", function(E) {
                    var R = E.target.closest(".weui-actionsheet__cell");
                    if (R) {
                        var T = (0,
                        l.default)(R)
                          , I = T.index();
                        v[I].onClick.call(R, E),
                        m()
                    }
                }),
                b.find(".weui-actionsheet__action").on("click", ".weui-actionsheet__cell", function(E) {
                    var R = (0,
                    l.default)(this).index();
                    p[R].onClick.call(this, E),
                    m()
                }),
                b.find(".weui-actionsheet__close").on("click", function() {
                    f.onClickMask(),
                    m()
                }),
                b.on("touchmove", function(E) {
                    E.preventDefault()
                }),
                g = b[0],
                g.hide = m,
                g
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a)
              , u = i(14)
              , c = s(u)
              , g = void 0;
            n.default = o,
            r.exports = n.default
        }
        , function(r, n) {
            r.exports = '<div class="<%= className %>"> <div class=weui-mask></div> <div class=weui-actionsheet role=dialog aria-modal=true tabindex=-1> <button class="weui-hidden_abs weui-actionsheet__close">\u5173\u95ED</button> <% if(title){ %> <div class=weui-actionsheet__title> <p class=weui-actionsheet__title-text><%= title %></p> </div> <% } %> <div class=weui-actionsheet__menu> <% for(var i = 0; i < menus.length; i++){ %> <div class="weui-actionsheet__cell <%= menus[i].className %>" role=button> <%= menus[i].label %> <div class=weui-actionsheet__cell__tips><%= menus[i].desc %></div> </div> <% } %> </div> <div class=weui-actionsheet__action> <% for(var j = 0; j < actions.length; j++){ %> <div class="weui-actionsheet__cell <%= actions[j].className %>" role=button><%= actions[j].label %></div> <% } %> </div> </div> </div> '
        }
        , function(r, n, i) {
            function s(d) {
                return d && d.__esModule ? d : {
                    default: d
                }
            }
            function o(d) {
                function m(b) {
                    m = l.default.noop,
                    f.remove(),
                    b && b(),
                    p.callback(),
                    g = null
                }
                function v(b) {
                    m(b)
                }
                var p = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {};
                typeof p == "number" && (p = {
                    duration: p
                }),
                typeof p == "function" && (p = {
                    callback: p
                }),
                p = l.default.extend({
                    content: d,
                    duration: 3e3,
                    callback: l.default.noop,
                    className: ""
                }, p);
                var f = (0,
                l.default)(l.default.render(c.default, p));
                return (0,
                l.default)("body").append(f),
                g && (clearTimeout(g.timeout),
                g.hide()),
                g = {
                    hide: v
                },
                g.timeout = setTimeout(v, p.duration),
                f[0].hide = v,
                f[0]
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a)
              , u = i(16)
              , c = s(u)
              , g = null;
            n.default = o,
            r.exports = n.default
        }
        , function(r, n) {
            r.exports = '<div class="weui-toptips weui-toptips_warn <%= className %>" style=display:block role=alert><%= content %></div> '
        }
        , function(r, n, i) {
            function s(u) {
                return u && u.__esModule ? u : {
                    default: u
                }
            }
            function o(u) {
                var c = (0,
                l.default)(u);
                return c.forEach(function(g) {
                    function d() {
                        p.val(""),
                        m.removeClass("weui-search-bar_focusing")
                    }
                    var m = (0,
                    l.default)(g)
                      , v = m.find(".weui-search-bar__label")
                      , p = m.find(".weui-search-bar__input")
                      , f = m.find(".weui-icon-clear")
                      , b = m.find(".weui-search-bar__cancel-btn");
                    v.on("click", function() {
                        m.addClass("weui-search-bar_focusing"),
                        p[0].focus()
                    }),
                    p.on("blur", function() {
                        this.value.length || d()
                    }),
                    f.on("click", function() {
                        p.val(""),
                        p[0].focus()
                    }),
                    b.on("click", function() {
                        d(),
                        p[0].blur()
                    })
                }),
                c
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a);
            n.default = o,
            r.exports = n.default
        }
        , function(r, n, i) {
            function s(u) {
                return u && u.__esModule ? u : {
                    default: u
                }
            }
            function o(u) {
                var c = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {}
                  , g = (0,
                l.default)(u);
                return c = l.default.extend({
                    defaultIndex: 0,
                    onChange: l.default.noop
                }, c),
                g.forEach(function(d) {
                    var m = (0,
                    l.default)(d)
                      , v = m.find(".weui-navbar__item, .weui-tabbar__item")
                      , p = m.find(".weui-tab__content");
                    v.eq(c.defaultIndex).addClass("weui-bar__item_on"),
                    p.eq(c.defaultIndex).show(),
                    v.on("click", function() {
                        var f = (0,
                        l.default)(this)
                          , b = f.index();
                        v.removeClass("weui-bar__item_on"),
                        f.addClass("weui-bar__item_on"),
                        p.hide(),
                        p.eq(b).show(),
                        c.onChange.call(this, b)
                    })
                }),
                this
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a);
            n.default = o,
            r.exports = n.default
        }
        , function(r, n, i) {
            function s(f) {
                return f && f.__esModule ? f : {
                    default: f
                }
            }
            function o(f) {
                return f && f.classList ? f.classList.contains("weui-cell") ? f : o(f.parentNode) : null
            }
            function a(f, b, _) {
                var C = f[0]
                  , E = f.val();
                if (C.tagName == "INPUT" || C.tagName == "TEXTAREA") {
                    var R = C.getAttribute("pattern") || "";
                    if (C.type == "radio") {
                        for (var T = b.find('input[type="radio"][name="' + C.name + '"]'), I = 0, h = T.length; I < h; ++I)
                            if (T[I].checked)
                                return null;
                        return "empty"
                    }
                    if (C.type == "checkbox") {
                        if (R) {
                            var y = b.find('input[type="checkbox"][name="' + C.name + '"]')
                              , w = R.replace(/[{\s}]/g, "").split(",")
                              , O = 0;
                            if (w.length != 2)
                                throw C.outerHTML + " regexp is wrong.";
                            return y.forEach(function(A) {
                                A.checked && ++O
                            }),
                            w[1] === "" ? O >= parseInt(w[0]) ? null : O == 0 ? "empty" : "notMatch" : parseInt(w[0]) <= O && O <= parseInt(w[1]) ? null : O == 0 ? "empty" : "notMatch"
                        }
                        return C.checked ? null : "empty"
                    }
                    if (R) {
                        if (/^REG_/.test(R)) {
                            if (!_)
                                throw "RegExp " + R + " is empty.";
                            if (R = R.replace(/^REG_/, ""),
                            !_[R])
                                throw "RegExp " + R + " has not found.";
                            R = _[R]
                        }
                        return new RegExp(R).test(E) ? null : f.val().length ? "notMatch" : "empty"
                    }
                    return f.val().length ? null : "empty"
                }
                return E.length ? null : "empty"
            }
            function l(f) {
                var b = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : m.default.noop
                  , _ = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : {}
                  , C = (0,
                m.default)(f);
                return C.forEach(function(E) {
                    var R = (0,
                    m.default)(E)
                      , T = R.find("[required]");
                    typeof b != "function" && (b = c);
                    for (var I = 0, h = T.length; I < h; ++I) {
                        var y = T.eq(I)
                          , w = a(y, R, _.regexp)
                          , O = {
                            ele: y[0],
                            msg: w
                        };
                        if (w)
                            return void (b(O) || c(O))
                    }
                    b(null)
                }),
                this
            }
            function u(f) {
                var b = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {}
                  , _ = (0,
                m.default)(f);
                return _.forEach(function(C) {
                    var E = (0,
                    m.default)(C);
                    E.find("[required]").on("blur", function() {
                        if (this.type != "checkbox" && this.type != "radio") {
                            var R = (0,
                            m.default)(this);
                            if (!(R.val().length < 1)) {
                                var T = a(R, E, b.regexp);
                                T && c({
                                    ele: R[0],
                                    msg: T
                                })
                            }
                        }
                    }).on("focus", function() {
                        g(this)
                    })
                }),
                this
            }
            function c(f) {
                if (f) {
                    var b = (0,
                    m.default)(f.ele)
                      , _ = f.msg
                      , C = b.attr(_ + "Tips") || b.attr("tips") || b.attr("placeholder");
                    if (C && (0,
                    p.default)(C),
                    f.ele.type == "checkbox" || f.ele.type == "radio")
                        return;
                    var E = o(f.ele);
                    E && E.classList.add("weui-cell_warn")
                }
            }
            function g(f) {
                var b = o(f);
                b && b.classList.remove("weui-cell_warn")
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var d = i(2)
              , m = s(d)
              , v = i(15)
              , p = s(v);
            n.default = {
                showErrorTips: c,
                hideErrorTips: g,
                validate: l,
                checkIfBlur: u
            },
            r.exports = n.default
        }
        , function(r, n, i) {
            function s(p) {
                return p && p.__esModule ? p : {
                    default: p
                }
            }
            function o(p, f) {
                function b(A, k) {
                    var V = A.find('[data-id="' + k + '"]')
                      , $ = V.find(".weui-uploader__file-content");
                    return $.length || ($ = (0,
                    l.default)('<div class="weui-uploader__file-content"></div>'),
                    V.append($)),
                    V.addClass("weui-uploader__file_status"),
                    $
                }
                function _(A, k) {
                    var V = A.find('[data-id="' + k + '"]').removeClass("weui-uploader__file_status");
                    V.find(".weui-uploader__file-content").remove()
                }
                function C(A) {
                    A.url = R.createObjectURL(A),
                    A.status = "ready",
                    A.upload = function() {
                        (0,
                        m.default)(l.default.extend({
                            $uploader: E,
                            file: A
                        }, f))
                    }
                    ,
                    A.stop = function() {
                        this.xhr.abort()
                    }
                    ,
                    f.onQueued(A),
                    f.auto && A.upload()
                }
                var E = (0,
                l.default)(p)
                  , R = window.URL || window.webkitURL || window.mozURL;
                if (f = l.default.extend({
                    url: "",
                    auto: !0,
                    type: "file",
                    fileVal: "file",
                    xhrFields: {},
                    onBeforeQueued: l.default.noop,
                    onQueued: l.default.noop,
                    onBeforeSend: l.default.noop,
                    onSuccess: l.default.noop,
                    onProgress: l.default.noop,
                    onError: l.default.noop
                }, f),
                f.compress !== !1 && (f.compress = l.default.extend({
                    width: 1600,
                    height: 1600,
                    quality: .8
                }, f.compress)),
                f.onBeforeQueued) {
                    var T = f.onBeforeQueued;
                    f.onBeforeQueued = function(A, k) {
                        var V = T.call(A, k);
                        if (V === !1)
                            return !1;
                        if (V !== !0) {
                            var $ = (0,
                            l.default)(l.default.render(c.default, {
                                id: A.id
                            }));
                            E.find(".weui-uploader__files").append($)
                        }
                    }
                }
                if (f.onQueued) {
                    var I = f.onQueued;
                    f.onQueued = function(A) {
                        if (!I.call(A)) {
                            var k = E.find('[data-id="' + A.id + '"]');
                            k.css({
                                backgroundImage: 'url("' + (A.base64 || A.url) + '")'
                            }),
                            f.auto || _(E, A.id)
                        }
                    }
                }
                if (f.onBeforeSend) {
                    var h = f.onBeforeSend;
                    f.onBeforeSend = function(A, k, V) {
                        var $ = h.call(A, k, V);
                        if ($ === !1)
                            return !1
                    }
                }
                if (f.onSuccess) {
                    var y = f.onSuccess;
                    f.onSuccess = function(A, k) {
                        A.status = "success",
                        y.call(A, k) || _(E, A.id)
                    }
                }
                if (f.onProgress) {
                    var w = f.onProgress;
                    f.onProgress = function(A, k) {
                        w.call(A, k) || b(E, A.id).html(k + "%")
                    }
                }
                if (f.onError) {
                    var O = f.onError;
                    f.onError = function(A, k) {
                        A.status = "fail",
                        O.call(A, k) || b(E, A.id).html('<i class="weui-icon-warn"></i>')
                    }
                }
                E.find('input[type="file"]').on("change", function(A) {
                    var k = A.target.files;
                    k.length !== 0 && (f.compress === !1 && f.type == "file" ? Array.prototype.forEach.call(k, function(V) {
                        V.id = ++v,
                        f.onBeforeQueued(V, k) !== !1 && C(V)
                    }) : Array.prototype.forEach.call(k, function(V) {
                        V.id = ++v,
                        f.onBeforeQueued(V, k) !== !1 && (0,
                        g.compress)(V, f, function($) {
                            $ && C($)
                        })
                    }),
                    this.value = "")
                })
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a)
              , u = i(21)
              , c = s(u)
              , g = i(22)
              , d = i(23)
              , m = s(d)
              , v = 0;
            n.default = o,
            r.exports = n.default
        }
        , function(r, n) {
            r.exports = '<li class="weui-uploader__file weui-uploader__file_status" data-id="<%= id %>" role=img> <div class=weui-uploader__file-content> <i class=weui-loading style=width:30px;height:30px></i> </div> </li> '
        }
        , function(r, n) {
            function i(c) {
                var g, d = c.naturalHeight, m = document.createElement("canvas");
                m.width = 1,
                m.height = d;
                var v = m.getContext("2d");
                v.drawImage(c, 0, 0);
                try {
                    g = v.getImageData(0, 0, 1, d).data
                } catch {
                    return 1
                }
                for (var p = 0, f = d, b = d; b > p; ) {
                    var _ = g[4 * (b - 1) + 3];
                    _ === 0 ? f = b : p = b,
                    b = f + p >> 1
                }
                var C = b / d;
                return C === 0 ? 1 : C
            }
            function s(c) {
                for (var g = atob(c.split(",")[1]), d = new ArrayBuffer(g.length), m = new Uint8Array(d), v = 0; v < g.length; v++)
                    m[v] = g.charCodeAt(v);
                return d
            }
            function o(c) {
                var g = c.split(",")[0].split(":")[1].split(";")[0]
                  , d = s(c);
                return new Blob([d],{
                    type: g
                })
            }
            function a(c) {
                var g = new DataView(c);
                if (g.getUint16(0, !1) != 65496)
                    return -2;
                for (var d = g.byteLength, m = 2; m < d; ) {
                    var v = g.getUint16(m, !1);
                    if (m += 2,
                    v == 65505) {
                        if (g.getUint32(m += 2, !1) != 1165519206)
                            return -1;
                        var p = g.getUint16(m += 6, !1) == 18761;
                        m += g.getUint32(m + 4, p);
                        var f = g.getUint16(m, p);
                        m += 2;
                        for (var b = 0; b < f; b++)
                            if (g.getUint16(m + 12 * b, p) == 274)
                                return g.getUint16(m + 12 * b + 8, p)
                    } else {
                        if ((65280 & v) != 65280)
                            break;
                        m += g.getUint16(m, !1)
                    }
                }
                return -1
            }
            function l(c, g, d) {
                var m = c.width
                  , v = c.height;
                switch (d > 4 && (c.width = v,
                c.height = m),
                d) {
                case 2:
                    g.translate(m, 0),
                    g.scale(-1, 1);
                    break;
                case 3:
                    g.translate(m, v),
                    g.rotate(Math.PI);
                    break;
                case 4:
                    g.translate(0, v),
                    g.scale(1, -1);
                    break;
                case 5:
                    g.rotate(.5 * Math.PI),
                    g.scale(1, -1);
                    break;
                case 6:
                    g.rotate(.5 * Math.PI),
                    g.translate(0, -v);
                    break;
                case 7:
                    g.rotate(.5 * Math.PI),
                    g.translate(m, -v),
                    g.scale(-1, 1);
                    break;
                case 8:
                    g.rotate(-.5 * Math.PI),
                    g.translate(-m, 0)
                }
            }
            function u(c, g, d) {
                var m = new FileReader;
                m.onload = function(v) {
                    if (g.compress === !1)
                        return c.base64 = v.target.result,
                        void d(c);
                    var p = new Image;
                    p.onload = function() {
                        var f = i(p)
                          , b = a(s(p.src))
                          , _ = document.createElement("canvas")
                          , C = _.getContext("2d")
                          , E = g.compress.width
                          , R = g.compress.height
                          , T = p.width
                          , I = p.height
                          , h = void 0;
                        if (T < I && I > R ? (T = parseInt(R * p.width / p.height),
                        I = R) : T >= I && T > E && (I = parseInt(E * p.height / p.width),
                        T = E),
                        _.width = T,
                        _.height = I,
                        b > 0 && l(_, C, b),
                        C.drawImage(p, 0, 0, T, I / f),
                        h = /image\/jpeg/.test(c.type) || /image\/jpg/.test(c.type) ? _.toDataURL("image/jpeg", g.compress.quality) : _.toDataURL(c.type),
                        g.type == "file")
                            if (/;base64,null/.test(h) || /;base64,$/.test(h))
                                d(c);
                            else {
                                var y = o(h);
                                y.id = c.id,
                                y.name = c.name,
                                y.lastModified = c.lastModified,
                                y.lastModifiedDate = c.lastModifiedDate,
                                d(y)
                            }
                        else
                            /;base64,null/.test(h) || /;base64,$/.test(h) ? (g.onError(c, new Error("Compress fail, dataURL is " + h + ".")),
                            d()) : (c.base64 = h,
                            d(c))
                    }
                    ,
                    p.src = v.target.result
                }
                ,
                m.readAsDataURL(c)
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            }),
            n.default = {
                compress: u
            },
            r.exports = n.default
        }
        , function(r, n) {
            function i(s) {
                var o = s.url
                  , a = s.file
                  , l = s.fileVal
                  , u = s.onBeforeSend
                  , c = s.onProgress
                  , g = s.onError
                  , d = s.onSuccess
                  , m = s.xhrFields
                  , v = a.name
                  , p = a.type
                  , f = a.lastModifiedDate
                  , b = {
                    name: v,
                    type: p,
                    size: s.type == "file" ? a.size : a.base64.length,
                    lastModifiedDate: f
                }
                  , _ = {};
                if (u(a, b, _) !== !1) {
                    a.status = "progress",
                    c(a, 0);
                    var C = new FormData
                      , E = new XMLHttpRequest;
                    a.xhr = E,
                    Object.keys(b).forEach(function(R) {
                        C.append(R, b[R])
                    }),
                    s.type == "file" ? C.append(l, a, v) : C.append(l, a.base64),
                    E.onreadystatechange = function() {
                        if (E.readyState == 4)
                            if (E.status == 200)
                                try {
                                    var R = JSON.parse(E.responseText);
                                    d(a, R)
                                } catch (T) {
                                    g(a, T)
                                }
                            else
                                g(a, new Error("XMLHttpRequest response status is " + E.status))
                    }
                    ,
                    E.upload.addEventListener("progress", function(R) {
                        if (R.total != 0) {
                            var T = 100 * Math.ceil(R.loaded / R.total);
                            c(a, T)
                        }
                    }, !1),
                    E.open("POST", o),
                    Object.keys(m).forEach(function(R) {
                        E[R] = m[R]
                    }),
                    Object.keys(_).forEach(function(R) {
                        E.setRequestHeader(R, _[R])
                    }),
                    E.send(C)
                }
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            }),
            n.default = i,
            r.exports = n.default
        }
        , function(r, n, i) {
            function s(I) {
                if (I && I.__esModule)
                    return I;
                var h = {};
                if (I != null)
                    for (var y in I)
                        Object.prototype.hasOwnProperty.call(I, y) && (h[y] = I[y]);
                return h.default = I,
                h
            }
            function o(I) {
                return I && I.__esModule ? I : {
                    default: I
                }
            }
            function a(I) {
                (typeof I > "u" ? "undefined" : c(I)) != "object" && (I = {
                    label: I,
                    value: I
                }),
                d.default.extend(this, I)
            }
            function l() {
                function I() {
                    (0,
                    d.default)(A.container).append(H),
                    d.default.getStyle(H[0], "transform"),
                    H.find(".weui-mask").addClass("weui-animate-fade-in"),
                    H.find(".weui-picker").addClass("weui-animate-slide-up").on("animationend webkitAnimationEnd", function(ae) {
                        ae.target.focus()
                    })
                }
                function h(ae) {
                    h = d.default.noop,
                    H.find(".weui-mask").addClass("weui-animate-fade-out"),
                    H.find(".weui-picker").addClass("weui-animate-slide-down").on("animationend webkitAnimationEnd", function() {
                        H.remove(),
                        R = !1,
                        A.onClose(),
                        ae && ae()
                    })
                }
                function y(ae) {
                    h(ae)
                }
                function w(ae, ne) {
                    if (Y[ne] === void 0 && A.defaultValue && A.defaultValue[ne] !== void 0) {
                        var Ye = A.defaultValue[ne]
                          , z = 0
                          , x = ae.length;
                        if (c(ae[z]) == "object")
                            for (; z < x && Ye != ae[z].value; ++z)
                                ;
                        else
                            for (; z < x && Ye != ae[z]; ++z)
                                ;
                        z < x && (Y[ne] = z)
                    }
                    H.find(".weui-picker__group").eq(ne).scroll({
                        items: ae,
                        temp: Y[ne],
                        onChange: function(P, L) {
                            if (P) {
                                var q = H.find(".weui-picker__group").eq(ne);
                                q.find(".weui-picker__item").attr("aria-hidden", "true"),
                                d.default.os.android ? (q.attr("title", "\u6309\u4F4F\u4E0A\u4E0B\u53EF\u8C03"),
                                q.attr("aria-label", P.label)) : (q.find(".weui-picker__item").eq(L).attr("aria-hidden", "false"),
                                q.find(".weui-picker__item").eq(L)[0].focus()),
                                D[ne] = new a(P)
                            } else
                                D[ne] = null;
                            if (Y[ne] = L,
                            V)
                                D.length == Z && A.onChange(D);
                            else if (P.children && P.children.length > 0)
                                H.find(".weui-picker__group").eq(ne + 1).show(),
                                w(P.children, ne + 1);
                            else {
                                var S = H.find(".weui-picker__group");
                                S.forEach(function(M, N) {
                                    N > ne && (0,
                                    d.default)(M).hide()
                                }),
                                D.splice(ne + 1),
                                A.onChange(D)
                            }
                            H.find(".weui-picker__group").eq(ne)[0].focus(),
                            clearTimeout(ve),
                            ve = setTimeout(function() {
                                H.find("#weui-picker-aria-content").html("")
                            }, 100)
                        },
                        onScroll: function(P, L) {
                            if (P) {
                                var q = H.find(".weui-picker__group").eq(ne);
                                q.find(".weui-picker__item").attr("aria-hidden", "true"),
                                d.default.os.android ? (q.attr("title", "\u6309\u4F4F\u4E0A\u4E0B\u53EF\u8C03"),
                                q.attr("aria-label", P.label)) : (q.find(".weui-picker__item").eq(L).attr("aria-hidden", "false"),
                                q.find(".weui-picker__item").eq(L)[0].focus()),
                                D[ne] = new a(P)
                            } else
                                D[ne] = null;
                            Y[ne] = L,
                            d.default.os.android && (clearTimeout(ve),
                            ve = setTimeout(function() {
                                H.find("#weui-picker-aria-content").html(P.label).attr("role", "alert")
                            }, 50))
                        },
                        onConfirm: A.onConfirm
                    })
                }
                if (R)
                    return R;
                var O = arguments[arguments.length - 1]
                  , A = d.default.extend({
                    id: "default",
                    className: "",
                    container: "body",
                    title: "",
                    desc: "",
                    confirmText: "\u786E\u5B9A",
                    closeText: "\u5173\u95ED",
                    showClose: !0,
                    onChange: d.default.noop,
                    onConfirm: d.default.noop,
                    onClose: d.default.noop
                }, O)
                  , k = void 0
                  , V = !1;
                if (arguments.length > 2) {
                    var $ = 0;
                    for (k = []; $ < arguments.length - 1; )
                        k.push(arguments[$++]);
                    V = !0
                } else
                    k = arguments[0];
                T[A.id] = T[A.id] || [];
                for (var D = [], Y = T[A.id], H = (0,
                d.default)(d.default.render(_.default, A)), re = H.find("#weui-picker-confirm"), he = H.find(".weui-mask"), Z = O.depth || (V ? k.length : f.depthOf(k[0])), se = "", ve = void 0, Ae = Z; Ae--; )
                    se += E.default;
                return H.find(".weui-picker__bd").html(se),
                I(),
                V ? k.forEach(function(ae, ne) {
                    w(ae, ne)
                }) : w(k, 0),
                H.on("click", ".weui-mask", function() {
                    y()
                }).on("click", ".weui-picker__btn", function() {
                    y()
                }).on("click", ".weui-btn_icon", function() {
                    y()
                }).on("touchmove", ".weui-half-screen-dialog__hd", function(ae) {
                    ae.preventDefault()
                }).on("touchmove", ".weui-half-screen-dialog__ft", function(ae) {
                    ae.preventDefault()
                }),
                he.on("click", function() {
                    y()
                }).on("touchmove", function(ae) {
                    ae.preventDefault()
                }),
                re.on("click", function() {
                    A.onConfirm(D)
                }),
                R = H[0],
                R.hide = y,
                R
            }
            function u(I) {
                var h = new Date
                  , y = d.default.extend({
                    id: "datePicker",
                    onChange: d.default.noop,
                    onConfirm: d.default.noop,
                    start: h.getFullYear() - 20,
                    end: h.getFullYear() + 20,
                    defaultValue: [h.getFullYear(), h.getMonth() + 1, h.getDate()],
                    cron: "* * *",
                    depth: 3
                }, I);
                y.depth > 3 && (y.depth = 3),
                y.depth < 1 && (y.depth = 1),
                typeof y.start == "number" ? y.start = new Date(y.start + "/01/01") : typeof y.start == "string" && (y.start = new Date(y.start.replace(/-/g, "/"))),
                typeof y.end == "number" ? y.end = new Date(y.end + "/12/31") : typeof y.end == "string" && (y.end = new Date(y.end.replace(/-/g, "/")));
                var w = function(re, he, Z) {
                    for (var se = 0, ve = re.length; se < ve; se++) {
                        var Ae = re[se];
                        if (Ae[he] == Z)
                            return Ae
                    }
                }
                  , O = []
                  , A = v.default.parse(y.cron, y.start, y.end)
                  , k = void 0;
                do {
                    k = A.next();
                    var V = k.value.getFullYear()
                      , $ = k.value.getMonth() + 1
                      , D = k.value.getDate()
                      , Y = w(O, "value", V);
                    if (Y || (Y = {
                        label: V + "\u5E74",
                        value: V,
                        children: []
                    },
                    O.push(Y)),
                    y.depth > 1) {
                        var H = w(Y.children, "value", $);
                        H || (H = {
                            label: $ + "\u6708",
                            value: $,
                            children: []
                        },
                        Y.children.push(H)),
                        y.depth > 2 && H.children.push({
                            label: D + "\u65E5",
                            value: D
                        })
                    }
                } while (!k.done);
                return l(O, y)
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var c = typeof Symbol == "function" && typeof Symbol.iterator == "symbol" ? function(I) {
                return typeof I
            }
            : function(I) {
                return I && typeof Symbol == "function" && I.constructor === Symbol && I !== Symbol.prototype ? "symbol" : typeof I
            }
              , g = i(2)
              , d = o(g)
              , m = i(25)
              , v = o(m);
            i(26);
            var p = i(27)
              , f = s(p)
              , b = i(28)
              , _ = o(b)
              , C = i(29)
              , E = o(C);
            a.prototype.toString = function() {
                return this.value
            }
            ,
            a.prototype.valueOf = function() {
                return this.value
            }
            ;
            var R = void 0
              , T = {};
            n.default = {
                picker: l,
                datePicker: u
            },
            r.exports = n.default
        }
        , function(r, n) {
            function i(g, d) {
                if (!(g instanceof d))
                    throw new TypeError("Cannot call a class as a function")
            }
            function s(g, d) {
                var m = d[0]
                  , v = d[1]
                  , p = []
                  , f = void 0;
                g = g.replace(/\*/g, m + "-" + v);
                for (var b = g.split(","), _ = 0, C = b.length; _ < C; _++) {
                    var E = b[_];
                    E.match(l) && E.replace(l, function(R, T, I, h) {
                        h = parseInt(h) || 1,
                        T = Math.min(Math.max(m, ~~Math.abs(T)), v),
                        I = I ? Math.min(v, ~~Math.abs(I)) : T,
                        f = T;
                        do
                            p.push(f),
                            f += h;
                        while (f <= I)
                    })
                }
                return p
            }
            function o(g, d, m) {
                var v = g.replace(/^\s\s*|\s\s*$/g, "").split(/\s+/)
                  , p = [];
                return v.forEach(function(f, b) {
                    var _ = u[b];
                    p.push(s(f, _))
                }),
                new c(p,d,m)
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = function() {
                function g(d, m) {
                    for (var v = 0; v < m.length; v++) {
                        var p = m[v];
                        p.enumerable = p.enumerable || !1,
                        p.configurable = !0,
                        "value"in p && (p.writable = !0),
                        Object.defineProperty(d, p.key, p)
                    }
                }
                return function(d, m, v) {
                    return m && g(d.prototype, m),
                    v && g(d, v),
                    d
                }
            }()
              , l = /^(\d+)(?:-(\d+))?(?:\/(\d+))?$/g
              , u = [[1, 31], [1, 12], [0, 6]]
              , c = function() {
                function g(d, m, v) {
                    i(this, g),
                    this._dates = d[0],
                    this._months = d[1],
                    this._days = d[2],
                    this._start = m,
                    this._end = v,
                    this._pointer = m
                }
                return a(g, [{
                    key: "_findNext",
                    value: function() {
                        for (var d = void 0; ; ) {
                            if (this._end.getTime() - this._pointer.getTime() < 0)
                                throw new Error("out of range, end is " + this._end + ", current is " + this._pointer);
                            var m = this._pointer.getMonth()
                              , v = this._pointer.getDate()
                              , p = this._pointer.getDay();
                            if (this._months.indexOf(m + 1) !== -1)
                                if (this._dates.indexOf(v) !== -1) {
                                    if (this._days.indexOf(p) !== -1) {
                                        d = new Date(this._pointer);
                                        break
                                    }
                                    this._pointer.setDate(v + 1)
                                } else
                                    this._pointer.setDate(v + 1);
                            else
                                this._pointer.setMonth(m + 1),
                                this._pointer.setDate(1)
                        }
                        return d
                    }
                }, {
                    key: "next",
                    value: function() {
                        var d = this._findNext();
                        return this._pointer.setDate(this._pointer.getDate() + 1),
                        {
                            value: d,
                            done: !this.hasNext()
                        }
                    }
                }, {
                    key: "hasNext",
                    value: function() {
                        try {
                            return this._findNext(),
                            !0
                        } catch {
                            return !1
                        }
                    }
                }]),
                g
            }();
            n.default = {
                parse: o
            },
            r.exports = n.default
        }
        , function(r, n, i) {
            function s(p) {
                return p && p.__esModule ? p : {
                    default: p
                }
            }
            var o = typeof Symbol == "function" && typeof Symbol.iterator == "symbol" ? function(p) {
                return typeof p
            }
            : function(p) {
                return p && typeof Symbol == "function" && p.constructor === Symbol && p !== Symbol.prototype ? "symbol" : typeof p
            }
              , a = i(2)
              , l = s(a)
              , u = function(p, f) {
                return p.css({
                    "-webkit-transition": "all " + f + "s",
                    transition: "all " + f + "s"
                })
            }
              , c = function(p, f) {
                return p.css({
                    "-webkit-transform": "translate3d(0, " + f + "px, 0)",
                    transform: "translate3d(0, " + f + "px, 0)"
                })
            }
              , g = function(p) {
                for (var f = Math.floor(p.length / 2), b = 0; p[f] && p[f].disabled; )
                    if (f = ++f % p.length,
                    b++,
                    b > p.length)
                        throw new Error("No selectable item.");
                return f
            }
              , d = function(p, f, b) {
                var _ = g(b);
                return (p - _) * f
            }
              , m = function(p, f) {
                return p * f
            }
              , v = function(p, f, b) {
                return -(f * (b - p - 1))
            };
            l.default.fn.scroll = function(p) {
                function f(H) {
                    k += H,
                    k = Math.round(k / I.rowHeight) * I.rowHeight;
                    var re = m(I.offset, I.rowHeight)
                      , he = v(I.offset, I.rowHeight, I.items.length);
                    k > re && (k = re),
                    k < he && (k = he);
                    for (var Z = I.offset - k / I.rowHeight; I.items[Z] && I.items[Z].disabled; )
                        H > 0 ? ++Z : --Z;
                    k = (I.offset - Z) * I.rowHeight,
                    u(y, .3),
                    c(y, k),
                    Z !== V && (I.onScroll.call(this, I.items[Z], Z),
                    I.onChange.call(this, I.items[Z], Z)),
                    V = null
                }
                function b(H) {
                    w = H,
                    A = +new Date
                }
                function _(H) {
                    O = H;
                    var re = k + (O - w);
                    u(y, 0),
                    c(y, re),
                    A = +new Date,
                    $.push({
                        time: A,
                        y: O
                    }),
                    $.length > 40 && $.shift(),
                    re = Math.round(re / I.rowHeight) * I.rowHeight;
                    var he = m(I.offset, I.rowHeight)
                      , Z = v(I.offset, I.rowHeight, I.items.length);
                    if (!(re > he || re < Z)) {
                        var se = I.offset - re / I.rowHeight;
                        I.items[se] && I.items[se].disabled || se !== V && I.onScroll.call(this, I.items[se], se)
                    }
                }
                function C(H) {
                    if (w) {
                        var re = new Date().getTime()
                          , he = E[0].getBoundingClientRect().top + I.bodyHeight / 2;
                        if (O = H,
                        re - A > 100)
                            f(Math.abs(O - w) > 10 ? O - w : he - O);
                        else if (Math.abs(O - w) > 10) {
                            for (var Z = $.length - 1, se = Z, ve = Z; ve > 0 && A - $[ve].time < 100; ve--)
                                se = ve;
                            if (se !== Z) {
                                var Ae = $[Z]
                                  , ae = $[se]
                                  , ne = Ae.time - ae.time
                                  , Ye = Ae.y - ae.y
                                  , z = Ye / ne
                                  , x = 150 * z + (O - w);
                                f(x)
                            } else
                                f(0)
                        } else
                            f(he - O);
                        w = null
                    }
                }
                var E = (0,
                l.default)(this).offAll()
                  , R = E.find(".weui-picker__content")
                  , T = Math.round(R.find(".weui-picker__item")[0].clientHeight)
                  , I = l.default.extend({
                    items: [],
                    offset: 2,
                    rowHeight: T,
                    onChange: l.default.noop,
                    onScroll: l.default.noop,
                    temp: null,
                    bodyHeight: 5 * T
                }, p)
                  , h = I.items.map(function(H) {
                    return '<div role="option" title="\u6309\u4F4F\u4E0A\u4E0B\u53EF\u8C03" tabindex="0" class="weui-picker__item' + (H.disabled ? " weui-picker__item_disabled" : "") + '">' + ((typeof H > "u" ? "undefined" : o(H)) == "object" ? H.label : H) + "</div>"
                }).join("");
                E[0].parentElement.style.height = I.bodyHeight + "px",
                R.html(h);
                var y = R
                  , w = void 0
                  , O = void 0
                  , A = void 0
                  , k = void 0
                  , V = null
                  , $ = [];
                if (I.temp !== null && I.temp < I.items.length) {
                    var D = I.temp;
                    I.onChange.call(this, I.items[D], D),
                    k = (I.offset - D) * I.rowHeight
                } else {
                    var Y = g(I.items);
                    I.onChange.call(this, I.items[Y], Y),
                    k = d(I.offset, I.rowHeight, I.items)
                }
                c(y, k),
                E.on("touchstart", function(H) {
                    b(H.changedTouches[0].pageY)
                }).on("touchmove", function(H) {
                    _(H.changedTouches[0].pageY),
                    H.preventDefault()
                }).on("touchend", function(H) {
                    C(H.changedTouches[0].pageY)
                }),
                E.on("mousedown", function(H) {
                    b(H.pageY),
                    H.stopPropagation(),
                    H.preventDefault()
                }).on("mousemove", function(H) {
                    w && (_(H.pageY),
                    H.stopPropagation(),
                    H.preventDefault())
                }).on("mouseup mouseleave", function(H) {
                    C(H.pageY),
                    H.stopPropagation(),
                    H.preventDefault()
                })
            }
        }
        , function(r, n) {
            Object.defineProperty(n, "__esModule", {
                value: !0
            }),
            n.depthOf = function i(s) {
                var o = 1;
                return s.children && s.children[0] && (o = i(s.children[0]) + 1),
                o
            }
        }
        , function(r, n) {
            r.exports = '<div class="<%= className %>"> <div class=weui-mask></div> <div class="weui-half-screen-dialog weui-picker" role=dialog aria-modal=true tabindex=-1> <div class=weui-half-screen-dialog__hd> <% if(showClose){ %> <div class=weui-half-screen-dialog__hd__side> <button class="weui-btn_icon weui-wa-hotarea"><%= closeText %><i class=weui-icon-close-thin></i></button> </div> <% } %> <div class=weui-half-screen-dialog__hd__main> <strong class=weui-half-screen-dialog__title><%= title %></strong> <span class=weui-half-screen-dialog__subtitle><%= desc %></span> </div> </div> <div class=weui-half-screen-dialog__bd> <div class=weui-picker__bd></div> </div> <div class=weui-half-screen-dialog__ft> <div class=weui-hidden_abs id=weui-picker-aria-content></div> <a href=javascript:; class="weui-btn weui-btn_primary weui-picker__btn" id=weui-picker-confirm data-action=select role=button><%= confirmText %></a> </div> </div> </div> '
        }
        , function(r, n) {
            r.exports = "<div class=weui-picker__group role=listbox tabindex=0> <div class=weui-picker__mask></div> <div class=weui-picker__indicator></div> <div class=weui-picker__content> <div class=weui-picker__item>&nbsp;</div> </div> </div> "
        }
        , function(r, n, i) {
            function s(d) {
                return d && d.__esModule ? d : {
                    default: d
                }
            }
            function o(d) {
                function m(b) {
                    m = l.default.noop,
                    f.addClass("weui-animate-fade-out").on("animationend webkitAnimationEnd", function() {
                        f.remove(),
                        g = !1,
                        b && b()
                    })
                }
                function v(b) {
                    m(b)
                }
                var p = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {};
                if (g)
                    return g;
                p = l.default.extend({
                    className: "",
                    onDelete: l.default.noop
                }, p);
                var f = (0,
                l.default)(l.default.render(c.default, l.default.extend({
                    url: d
                }, p)));
                return (0,
                l.default)("body").append(f),
                f.find(".weui-gallery__img").on("click", function() {
                    v()
                }),
                f.find(".weui-gallery__close").on("click", function() {
                    v()
                }),
                f.find(".weui-gallery__del").on("click", function() {
                    p.onDelete.call(this, d)
                }),
                f.show().addClass("weui-animate-fade-in").on("animationend webkitAnimationEnd", function(b) {
                    b.target.focus()
                }),
                g = f[0],
                g.hide = v,
                g
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a)
              , u = i(31)
              , c = s(u)
              , g = void 0;
            n.default = o,
            r.exports = n.default
        }
        , function(r, n) {
            r.exports = '<div class="weui-gallery <%= className %>" role=dialog aria-modal=true tabindex=-1> <button class="weui-hidden_abs weui-gallery__close">\u5173\u95ED</button> <span class=weui-gallery__img style="background-image:url(<%= url %>)" role=img src="<%= url %>"></span> <div class=weui-gallery__opr> <a href=javascript: class=weui-gallery__del role=button aria-label=\u5220\u9664> <i class="weui-icon-delete weui-icon_gallery-delete"></i> </a> </div> </div> '
        }
        , function(r, n, i) {
            function s(u) {
                return u && u.__esModule ? u : {
                    default: u
                }
            }
            function o(u) {
                var c = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {}
                  , g = (0,
                l.default)(u);
                if (c = l.default.extend({
                    step: void 0,
                    defaultValue: 0,
                    onChange: l.default.noop
                }, c),
                c.step !== void 0 && (c.step = parseFloat(c.step),
                !c.step || c.step < 0))
                    throw new Error("Slider step must be a positive number.");
                if (c.defaultValue !== void 0 && c.defaultValue < 0 || c.defaultValue > 100)
                    throw new Error("Slider defaultValue must be >= 0 and <= 100.");
                return g.forEach(function(d) {
                    function m() {
                        var h = l.default.getStyle(_[0], "left");
                        return h = /%/.test(h) ? C * parseFloat(h) / 100 : parseFloat(h)
                    }
                    function v(h) {
                        var y = void 0
                          , w = void 0;
                        c.step && (h = Math.round(h / I) * I),
                        y = R + h,
                        y = y < 0 ? 0 : y > C ? C : y,
                        w = 100 * y / C,
                        b.css({
                            width: w + "%"
                        }),
                        _.css({
                            left: w + "%"
                        }),
                        c.onChange.call(d, w)
                    }
                    var p = (0,
                    l.default)(d)
                      , f = p.find(".weui-slider__inner")
                      , b = p.find(".weui-slider__track")
                      , _ = p.find(".weui-slider__handler")
                      , C = parseInt(l.default.getStyle(f[0], "width"))
                      , E = f[0].offsetLeft
                      , R = 0
                      , T = 0
                      , I = void 0;
                    c.step && (I = C * c.step / 100),
                    c.defaultValue && v(C * c.defaultValue / 100),
                    p.on("click", function(h) {
                        h.preventDefault(),
                        R = m(),
                        v(h.pageX - E - R)
                    }),
                    _.on("touchstart", function(h) {
                        R = m(),
                        T = h.changedTouches[0].clientX
                    }).on("touchmove", function(h) {
                        h.preventDefault(),
                        v(h.changedTouches[0].clientX - T)
                    })
                }),
                this
            }
            Object.defineProperty(n, "__esModule", {
                value: !0
            });
            var a = i(2)
              , l = s(a);
            n.default = o,
            r.exports = n.default
        }
        ])
    })
}
)($c);
const fy = tm($c.exports);
var Or = (t => (t[t.GeneralErrType_No_Err = 0] = "GeneralErrType_No_Err",
t[t.GeneralErrType_Err_Page = 1] = "GeneralErrType_Err_Page",
t[t.GeneralErrType_Warning_Page = 2] = "GeneralErrType_Warning_Page",
t[t.GeneralErrType_Info_Page = 3] = "GeneralErrType_Info_Page",
t[t.GeneralErrType_Redirect_App = 4] = "GeneralErrType_Redirect_App",
t[t.UNRECOGNIZED = -1] = "UNRECOGNIZED",
t))(Or || {})
  , Tv = (t => (t[t.FinderMediaCardShowStyle_Undefined = 0] = "FinderMediaCardShowStyle_Undefined",
t[t.FinderMediaCardShowStyle_Tile = 1] = "FinderMediaCardShowStyle_Tile",
t[t.FinderMediaCardShowStype_Center = 2] = "FinderMediaCardShowStype_Center",
t[t.UNRECOGNIZED = -1] = "UNRECOGNIZED",
t))(Tv || {});
const Av = {
    class: "channel_qrcode_area"
}
  , Rv = {
    class: "channel_qrcode_mod"
}
  , Mv = ["src"]
  , Pv = ["innerHTML"]
  , Ov = nr({
    __name: "QRCode",
    props: {
        title: {
            default: "\u53EF\u626B\u7801\u524D\u5F80\u5FAE\u4FE1\u89C2\u770B"
        }
    },
    setup(t) {
        const e = xc()
          , {sceneInfo: r} = Wu(e)
          , {getQRCodeByScene: n} = jc({
            sceneInfoRef: r,
            refreshSceneInfo: () => e.getFeedDetail()
        })
          , i = Ie("");
        return _r(async () => {
            i.value = await n(0, "feed")
        }
        ),
        (s, o) => (ge(),
        _e("div", Av, [ue("div", Rv, [ue("img", {
            class: "channel_qrcode_img",
            src: i.value
        }, null, 8, Mv), ue("h2", {
            class: "channel_qrcode_title",
            innerHTML: s.title
        }, null, 8, Pv)])]))
    }
});
const Lv = {
    name: "Errmsg",
    components: {
        QRCode: Ov
    },
    props: {
        errMsg: {
            type: Object,
            required: !0
        }
    },
    setup(t) {
        const {isMobile: e} = ho()
          , r = xc()
          , {sceneInfo: n} = Wu(r)
          , {jumpToWeixin: i} = jc({
            sceneInfoRef: n,
            refreshSceneInfo: () => r.getFeedDetail()
        })
          , s = at( () => t.errMsg.type === Or.GeneralErrType_Redirect_App || t.errMsg.type === tc.VideoErrType_GLOBAL)
          , o = at( () => t.errMsg.type === Or.GeneralErrType_Warning_Page || s.value ? "weui-icon-warn weui-icon_msg-primary" : t.errMsg.type === Or.GeneralErrType_Err_Page ? "weui-icon-warn weui-icon_msg" : t.errMsg.type === Or.GeneralErrType_Info_Page ? "weui-icon-info weui-icon_msg" : "")
          , a = at( () => e.value || !s.value);
        return {
            iconClass: o,
            isMobile: e,
            handleJumpToWeixin: () => {
                i(0, "feed")
            }
            ,
            jumpToWeixinButtonTitle: "\u524D\u5F80\u5FAE\u4FE1",
            showMsg: a,
            isNeedFallback: s
        }
    }
};
const Nv = {
    key: 0,
    class: "weui-msg"
}
  , kv = {
    class: "weui-msg__icon-area"
}
  , Bv = {
    class: "weui-msg__text-area"
}
  , Dv = ["innerHTML"]
  , Fv = ["innerHTML"]
  , Uv = {
    key: 0,
    class: "weui-msg__extra-area"
}
  , jv = {
    class: "weui-footer"
}
  , $v = {
    class: "weui-footer__links"
};
function Hv(t, e, r, n, i, s) {
    const o = Zf("QRCode");
    return ge(),
    _e("div", {
        class: Bt(["general-warning-msg", {
            "pc-mode": !n.isMobile
        }])
    }, [n.showMsg ? (ge(),
    _e("div", Nv, [ue("div", kv, [ue("i", {
        class: Bt(n.iconClass)
    }, null, 2)]), ue("div", Bv, [ue("h2", {
        class: "weui-msg__title",
        innerHTML: r.errMsg.title
    }, null, 8, Dv), ue("p", {
        class: "weui-msg__desc",
        innerHTML: r.errMsg.content
    }, null, 8, Fv)]), n.isNeedFallback ? (ge(),
    _e("div", Uv, [ue("div", jv, [ue("p", $v, [ue("button", {
        class: "weui-wa-hotarea weui-link weui-btn_reset weui-footer__link",
        type: "button",
        onClick: e[0] || (e[0] = (...a) => n.handleJumpToWeixin && n.handleJumpToWeixin(...a))
    }, Wr(n.jumpToWeixinButtonTitle), 1)])])])) : Qt("", !0)])) : (ge(),
    Xr(o, {
        key: 1,
        title: r.errMsg.title
    }, null, 8, ["title"]))], 2)
}
const dy = tt(Lv, [["render", Hv], ["__scopeId", "data-v-5d274351"]]);
class qv extends _c {
    report(e) {
        return this.post({
            url: `${Ec}/report/report-mmdata`,
            data: e
        })
    }
}
const Vv = new qv;
class hy extends ai {
    constructor() {
        super(...arguments);
        gi(this, "reportTypes", [Ze.BEHAVIOR])
    }
    async send(r) {
        const n = r.map(i => Op(46, i));
        return Vv.report({
            context: n
        })
    }
}

return {
	getFeedInfo: Jm.getFeedInfo.bind(Jm),
};

})();
