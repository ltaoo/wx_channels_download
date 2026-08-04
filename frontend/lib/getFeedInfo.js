var Hc = Object.defineProperty;
var qc = (t, e, r) => e in t ? Hc(t, e, { enumerable: !0, configurable: !0, writable: !0, value: r }) : t[e] = r;
var gi = (t, e, r) => (qc(t, typeof e != "symbol" ? e + "" : e, r), r);

// Browser version - assumes axios is available globally via script tag

var qr = typeof window !== 'undefined' ? window.axios : (typeof axios !== 'undefined' ? axios : require('axios'));

const jm = 3, $m = 0, Hm = (t, e, r) => r || !0;

// Error class hierarchy
var Xt = {}, Hs = { exports: {} };
(function (t, e) {
    var r = typeof Reflect < "u" ? Reflect.construct : void 0, n = Object.defineProperty, i = Error.captureStackTrace;
    i === void 0 && (i = function (u) { var c = new Error(); n(u, "stack", { configurable: !0, get: function () { var d = c.stack; return n(this, "stack", { configurable: !0, value: d, writable: !0 }), d }, set: function (d) { n(u, "stack", { configurable: !0, value: d, writable: !0 }) } }) });
    function s(l) { l !== void 0 && n(this, "message", { configurable: !0, value: l, writable: !0 }); var u = this.constructor.name; u !== void 0 && u !== this.name && n(this, "name", { configurable: !0, value: u, writable: !0 }), i(this, this.constructor) }
    s.prototype = Object.create(Error.prototype, { constructor: { configurable: !0, value: s, writable: !0 } });
    var o = function () { function l(c, g) { return n(c, "name", { configurable: !0, value: g }) } try { var u = function () {}; if ((l(u, "foo"), u.name === "foo")) return l } catch {} }();
    function a(l, u) { if (u == null || u === Error) u = s; else if (typeof u != "function") throw new TypeError("super_ should be a function"); var c; if (typeof l == "string") ((c = l), (l = r !== void 0 ? function () { return r(u, arguments, this.constructor) } : function () { u.apply(this, arguments) }), o !== void 0 && (o(l, c), (c = void 0))); else if (typeof l != "function") throw new TypeError("constructor should be either a string or a function"); l.super_ = l.super = u; var g = { constructor: { configurable: !0, value: l, writable: !0 } }; return c !== void 0 && (g.name = { configurable: !0, value: c, writable: !0 }), (l.prototype = Object.create(u.prototype, g)), l }
    ((e = t.exports = a), (e.BaseError = s));
})(Hs, Hs.exports);

var bc = Xt.ServerError = wc = Xt.LogicError = Xt.KnownError = void 0;
const Km = Hs.exports;
class fn extends Km.BaseError { static fromJSON(e) { return new fn(e.message || "") } }
Xt.KnownError = fn;
class Co extends fn { constructor(e, r) { super(r), this.name = "LogicError", this.code = 0, this.code = e } static fromJSON(e) { return new Co(e.code || 0, e.message || "") } }
var wc = Xt.LogicError = Co;
class Io extends fn { constructor() { super(...arguments), this.name = "ServerError" } static fromJSON(e) { return new Io(e.message || "") } }
var bc = Xt.ServerError = Io;

class ln extends Error {
    constructor(t, e = {}) {
        super(t);
        const { cause: r, level: n = "default", origin: i } = e;
        ((this.name = "KnownError"), (this.cause = r), (this.level = n), (this.origin = i), (this.timestamp = Date.now()), (this.stack = r == null ? void 0 : r.stack));
    }
}

class ep extends ln {
    constructor(t, e, r = {}) {
        (super(t, r), (this.url = e), (this.name = "RequestError"));
        const { req: n, resp: i, errCode: s } = r;
        ((this.req = n), (this.resp = i), (this.errCode = s));
    }
}

class On extends ep {
    constructor(e, r, n) {
        var i, s;
        let o = "";
        (n != null && n.config && (o = Eo(n.config.baseURL, n.config.url)), super(e, o, { errCode: r }), (this.config = n == null ? void 0 : n.config), (this.name = "RequestXError"), (this.resp = n == null ? void 0 : n.response), (this.stack = n == null ? void 0 : n.stack), (this.rid = (i = n == null ? void 0 : n.config) === null || i === void 0 ? void 0 : i.rid), (this.displayTitle = (s = n == null ? void 0 : n.displayTitle) !== null && s !== void 0 ? s : ""), (this.displayContent = (n == null ? void 0 : n.displayContent) || "请求异常"));
    }
}

class Wm {
    constructor(e) {
        gi(this, "errorTypeMap", {});
        if (e) {
            for (const r of e) {
                if (r) this.errorTypeMap[r.name] = r;
            }
        }
    }
    getError(e) {
        if (e && e.name) {
            const r = this.errorTypeMap[e.name];
            if (r) return r.fromJSON(e);
        }
        return e && e.message ? new Error(e.message) : new Error(JSON.stringify(e));
    }
}

const zm = new Wm([wc, bc]);

function Vm(t) { return t.isAxiosError }

// Helper functions
function hg(t, e) {
    var r = {};
    for (var n in t) Object.prototype.hasOwnProperty.call(t, n) && e.indexOf(n) < 0 && (r[n] = t[n]);
    if (t != null && typeof Object.getOwnPropertySymbols == "function") for (var i = 0, n = Object.getOwnPropertySymbols(t); i < n.length; i++) e.indexOf(n[i]) < 0 && Object.prototype.propertyIsEnumerable.call(t, n[i]) && (r[n[i]] = t[n[i]]);
    return r;
}

function Eo(t, e) {
    if (!e) return t;
    if (e.startsWith("http")) return e;
    return t ? t.replace(/\/$/, "") + "/" + e.replace(/^\//, "") : e;
}

// Simplified ci class - uses axios directly
class ci {
    static setGlobalConfig(e) { this.globalConfig = Object.assign(Object.assign({}, this.globalConfig), e); }
    constructor(e) {
        this.baseURL = "";
        this.axiosInstance = void 0;
        this.validateResult = Hm;
        this.onRetry = () => false;
        this.formatConfig = c => c;
        this.requestMiddles = [];
        this.cancelTokenMap = new Map;
        const r = Object.assign(Object.assign({}, ci.globalConfig), e);
        const { baseURL: n, validateResult: i, onRetry: s, formatConfig: o, requestMiddles: a, adapter: l } = r;
        const u = hg(r, ["baseURL", "validateResult", "onRetry", "formatConfig", "requestMiddles", "adapter"]);
        typeof i == "function" && (this.validateResult = i);
        typeof o == "function" && (this.formatConfig = o);
        Array.isArray(a) && (this.requestMiddles = [...a]);
        this.instanceConfig = Object.assign(Object.assign({}, u), { adapter: l });
        this.baseURL = n;
        this.axiosInstance = qr.create(Object.assign({ baseURL: n }, u));
    }
    initRequestConfig(e) {
        const r = Object.assign(Object.assign({}, this.instanceConfig), e);
        if (r.retry && (r._retryCount = 0, r.retry.count || (r.retry.count = jm), r.retry.delay || (r.retry.delay = $m)), !r.adapter) { }
        return this.baseURL && !r.baseURL && (r.baseURL = this.baseURL), r;
    }
    addMiddleware(e) { this.requestMiddles.push(e); }
    getRequestMiddles(e) {
        const r = [...this.requestMiddles];
        return e.retry && r.push(this.retry.bind(this)), r;
    }
    async request(e) {
        e = this.initRequestConfig(e);
        const r = this.getRequestMiddles(e);
        let n = async (i) => r.length === 0 ? await this.doRequest(i) : await r.shift().bind(this)(i, n);
        return await n(e);
    }
    async rawRequest(e) {
        e = this.initRequestConfig(e);
        const r = this.getRequestMiddles(e);
        let n = async (i) => r.length === 0 ? await this.doRawRequest(i) : await r.shift().bind(this)(i, n);
        return await n(e);
    }
    async retry(e, r) {
        try {
            e._retryCount || (e._retryCount = 0);
            try { return await r(e); }
            catch (n) {
                if (e._retryCount >= e.retry.count) return Promise.reject(n);
                return e._retryCount++, this.retry(e, r);
            }
        } catch (n) { return Promise.reject(n); }
    }
    reportSuccess(e, r, n) { }
    reportFail(e, r, n) { }
    async doRequest(e) {
        const r = new Date().getTime();
        try {
            e.method || (e.method = "POST");
            this.formatConfig && (e = this.formatConfig(e));
            e.method.toLocaleLowerCase() === "get" ? e.params = e.params || e.data || {} : e.data = e.data || e.params || {};
            const n = await this.axiosInstance.request(e);
            const i = Vm(n.data) ? new On(n.data.message, -1, { config: n.config }) : null;
            const s = i || (n.data.error ? new On(zm.getError(n.data.error.message).message, -1, { config: n.config }) : typeof n.data.errCode == "number" && n.data.errCode !== 0 ? new On(n.data.errMsg, n.data.errCode, { config: n.config }) : !0);
            const a = { durationMs: new Date().getTime() - r };
            return s !== !0 ? (this.reportFail(e, s, a), Promise.reject(s)) : (this.reportSuccess(e, n.data, a), n.data);
        } catch (n) {
            const s = { durationMs: new Date().getTime() - r };
            const o = Vm(n) ? new On(n.message, -1, { config: n.config, response: n.response }) : typeof n.response?.data?.errCode == "number" ? new On(n.response.data.errMsg, n.response.data.errCode, { config: n.config, response: n.response }) : new On(n.message, -1, { config: n.config });
            const a = this.validateResult(n, e, o);
            return a !== !0 ? (this.reportFail(e, a, s), Promise.reject(a)) : (this.reportSuccess(e, n, s), n);
        }
    }
    async doRawRequest(e) {
        const r = new Date().getTime();
        try {
            e.method || (e.method = "POST");
            this.formatConfig && (e = this.formatConfig(e));
            e.method.toLocaleLowerCase() === "get" ? e.params = e.params || e.data || {} : e.data = e.data || e.params || {};
            const n = await this.axiosInstance.request(e);
            const i = Vm(n.data) ? new On(n.data.message, -1, { config: n.config }) : null;
            const s = i || (n.data.error ? new On(zm.getError(n.data.error.message).message, -1, { config: n.config }) : typeof n.data.errCode == "number" && n.data.errCode !== 0 ? new On(n.data.errMsg, n.data.errCode, { config: n.config }) : !0);
            const a = { durationMs: new Date().getTime() - r };
            return s !== !0 ? (this.reportFail(e, s, a), Promise.reject(s)) : (this.reportSuccess(e, n.data, a), n);
        } catch (n) {
            const s = { durationMs: new Date().getTime() - r };
            const o = Vm(n) ? new On(n.message, -1, { config: n.config, response: n.response }) : typeof n.response?.data?.errCode == "number" ? new On(n.response.data.errMsg, n.response.data.errCode, { config: n.config, response: n.response }) : new On(n.message, -1, { config: n.config });
            const a = this.validateResult(n, e, o);
            return a !== !0 ? (this.reportFail(e, a, s), Promise.reject(a)) : (this.reportSuccess(e, n, s), n);
        }
    }
    async get(e) { return e.method = "GET", await this.request(e); }
    async rawGet(e) { return e.method = "GET", await this.rawRequest(e); }
    async post(e) { return e.method = "POST", await this.request(e); }
    async rawPost(e) { return e.method = "POST", await this.rawRequest(e); }
}
ci.globalConfig = {};

// _c extends ci
class _c extends ci {
    constructor(e) {
        super({
            ...e,
            timeout: 1e4,
            withCredentials: !0,
            validateResult: (r, n) => {
                const i = Vm(r) ? new On(r.message, -1, { config: n }) : r.error ? new On(zm.getError(r.error.message).message, -1, { config: n }) : typeof r.errCode == "number" && r.errCode !== 0 ? new On(r.errMsg, r.errCode, { config: n }) : !0;
                return i;
            }
        });
    }
}

// Gm extends _c
const Ec = "/finder-preview";
class Gm extends _c {
    constructor() { super({ baseURL: `${Ec}/api` }); }
    get(e) { return super.get(e); }
    post(e) { return super.post(e); }
}

// Ym extends Gm - contains getFeedInfo
class Ym extends Gm {
    getFeedInfo(e) {
        return this.post({
            url: "feed/get_feed_info",
            data: { ...e }
        });
    }
    getProfileInfo(e) {
        return this.post({
            url: "feed/get_userpage",
            data: { ...e }
        });
    }
}

const Jm = new Ym();

// Export for browser - can be used as window.getFeedInfo or via ES module
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { getFeedInfo: Jm.getFeedInfo.bind(Jm) };
} else {
  window.getFeedInfo = Jm.getFeedInfo.bind(Jm);
}