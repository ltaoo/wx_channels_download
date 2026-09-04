globalThis.__setupBrowser = function (config) {
  let writtenCookie = "";
  const cookieValues = new Map();
  const nativeFunctions = new WeakMap();
  const htmlDDAValues = new WeakSet();
  const markNative = (value, name, kind = "") => {
    if (typeof value === "function") {
      nativeFunctions.set(value, `function ${kind ? kind + " " : ""}${String(name || value.name || "")}() { [native code] }`);
    }
    return value;
  };
  const markMethods = (value) => {
    for (const key of Reflect.ownKeys(value)) {
      const descriptor = Object.getOwnPropertyDescriptor(value, key);
      if (!descriptor) continue;
      markNative(descriptor.value, key);
      markNative(descriptor.get, key, "get");
      markNative(descriptor.set, key, "set");
    }
    return value;
  };
  const tagged = (value, tag) => {
    Object.defineProperty(value, Symbol.toStringTag, { configurable: true, value: tag });
    return value;
  };
  const collection = (values, tag = "HTMLCollection") => {
    const result = {
      item(index) { return this[index] ?? null; },
      namedItem(name) { return values.find(value => value && (value.id === name || value.name === name)) || null; },
    };
    values.forEach((value, index) => Object.defineProperty(result, index, { enumerable: true, value }));
    Object.defineProperty(result, "length", { value: values.length });
    Object.defineProperty(result, Symbol.iterator, { value: function* () { yield* values; } });
    return markMethods(tagged(result, tag));
  };

  const location = markMethods(tagged({
    ...config.location,
    reload() {},
    toString() { return this.href; },
  }, "Location"));
  function Location() { throw new TypeError("Illegal constructor"); }
  markNative(Location, "Location");
  tagged(Location.prototype, "Location");
  Object.setPrototypeOf(location, Location.prototype);
  const currentScript = tagged({
    tagName: "SCRIPT",
    nodeName: "SCRIPT",
    src: config.scriptUrl,
    crossOrigin: "",
    style: tagged({}, "CSSStyleDeclaration"),
    getAttribute(name) {
      name = String(name).toLowerCase();
      if (name === "data-assets-tracker-config") return '{"appName":"zse_ck","trackJSRuntimeError":true}';
      if (name === "crossorigin") return "";
      if (name === "src") return this.src;
      return null;
    },
  }, "HTMLScriptElement");
  const challengeMeta = tagged({
    id: "zh-zse-ck",
    content: config.meta,
    charset: "UTF-8",
    getAttribute(name) {
      name = String(name).toLowerCase();
      return name === "id" ? this.id : name === "content" ? this.content : name === "charset" ? this.charset : null;
    },
  }, "HTMLMetaElement");
  const body = tagged({ tagName: "BODY", nodeName: "BODY", parentNode: null, style: tagged({}, "CSSStyleDeclaration") }, "HTMLBodyElement");
  const head = tagged({ tagName: "HEAD", nodeName: "HEAD", parentNode: null }, "HTMLHeadElement");
  const challengeDiv = tagged({
    tagName: "DIV", nodeName: "DIV", parentNode: body,
    style: tagged({}, "CSSStyleDeclaration"),
  }, "HTMLDivElement");
  const htmlElement = tagged({
    tagName: "HTML",
    nodeName: "HTML",
    parentNode: null,
    getElementsByTagName(name) {
      name = String(name).toLowerCase();
      if (name === "script") return collection([currentScript]);
      if (name === "meta") return collection([challengeMeta]);
      if (name === "head") return collection([head]);
      if (name === "body") return collection([body]);
      if (name === "div") return collection([challengeDiv]);
      if (name === "*") return collection([head, challengeMeta, body, challengeDiv, currentScript]);
      return collection([]);
    },
  }, "HTMLHtmlElement");
  head.parentNode = htmlElement;
  body.parentNode = htmlElement;
  challengeMeta.parentNode = head;
  currentScript.parentNode = body;
  const allNodes = [htmlElement, head, challengeMeta, body, challengeDiv, currentScript];
  function documentAll(nameOrIndex) {
    if (nameOrIndex == null) return null;
    if (nameOrIndex === "zh-zse-ck") return challengeMeta;
    return allNodes[Number(nameOrIndex)] || null;
  }
  allNodes.forEach((node, index) => { documentAll[index] = node; });
  Object.defineProperty(documentAll, "length", { value: allNodes.length });
  tagged(documentAll, "HTMLAllCollection");
  markNative(documentAll, "all");
  htmlDDAValues.add(documentAll);
  function blankDocumentAll() { return null; }
  Object.defineProperty(blankDocumentAll, "length", { value: 0 });
  tagged(blankDocumentAll, "HTMLAllCollection");
  markNative(blankDocumentAll, "all");
  htmlDDAValues.add(blankDocumentAll);

  const canvasContext = tagged({
    canvas: null,
    fillStyle: "#000000",
    font: "10px sans-serif",
    textBaseline: "alphabetic",
    fillRect() {}, clearRect() {}, fillText() {}, strokeText() {},
    beginPath() {}, closePath() {}, moveTo() {}, lineTo() {}, arc() {}, stroke() {}, fill() {},
    measureText(text) { return { width: String(text).length * 5 }; },
    getImageData() { return { data: new Uint8ClampedArray(4), width: 1, height: 1 }; },
  }, "CanvasRenderingContext2D");
  const webglDebugRendererInfo = {
    UNMASKED_VENDOR_WEBGL: 0x9245,
    UNMASKED_RENDERER_WEBGL: 0x9246,
  };
  const webglContext = tagged(markMethods({
    canvas: null,
    getSupportedExtensions() { return []; },
    getExtension(name) { return name === "WEBGL_debug_renderer_info" ? webglDebugRendererInfo : null; },
    getParameter(name) {
      if (name === webglDebugRendererInfo.UNMASKED_VENDOR_WEBGL) return "Google Inc. (Apple)";
      if (name === webglDebugRendererInfo.UNMASKED_RENDERER_WEBGL) {
        return "ANGLE (Apple, ANGLE Metal Renderer: Apple M1 Pro, Unspecified Version)";
      }
      return null;
    },
  }), "WebGLRenderingContext");
  const document = {
    currentScript,
    URL: config.targetUrl,
    baseURI: config.targetUrl,
    all: documentAll,
    body,
    referrer: "",
    title: "",
    charset: "UTF-8",
    readyState: "loading",
    getElementById(id) { return id === "zh-zse-ck" ? challengeMeta : null; },
    getElementsByTagName(name) {
      name = String(name).toLowerCase();
      if (name === "meta") return collection([challengeMeta]);
      if (name === "script") return collection([currentScript]);
      if (name === "html") return collection([htmlElement]);
      if (name === "head") return collection([head]);
      if (name === "body") return collection([body]);
      if (name === "div") return collection([challengeDiv]);
      if (name === "*") return collection(allNodes);
      return collection([]);
    },
    querySelector(selector) { return selector === "script[data-assets-tracker-config]" ? currentScript : null; },
    querySelectorAll(selector) { return collection(selector === ":defined" ? allNodes : [], "NodeList"); },
    createNodeIterator(root) {
      let consumed = false;
      return tagged(markMethods({
        root,
        referenceNode: root,
        pointerBeforeReferenceNode: true,
        whatToShow: 0xffffffff,
        filter: null,
        nextNode() { if (consumed) return null; consumed = true; return root; },
        previousNode() { return null; },
        detach() {},
      }), "NodeIterator");
    },
    createRange() {
      return tagged(markMethods({
        collapsed: true,
        commonAncestorContainer: document,
        startContainer: document,
        startOffset: 0,
        endContainer: document,
        endOffset: 0,
        cloneContents() { return tagged({}, "DocumentFragment"); },
        cloneRange() { return document.createRange(); },
        collapse() {}, compareBoundaryPoints() { return 0; }, comparePoint() { return 0; },
        intersectsNode() { return false; }, isPointInRange() { return false; }, deleteContents() {},
        detach() {}, extractContents() { return {}; }, getBoundingClientRect() { return { x: 0, y: 0, width: 0, height: 0 }; },
        getClientRects() { return []; }, insertNode() {}, selectNode() {}, selectNodeContents() {},
        setEnd() {}, setEndAfter() {}, setEndBefore() {}, setStart() {}, setStartAfter() {}, setStartBefore() {},
        surroundContents() {}, toString() { return ""; },
      }), "Range");
    },
    createElement(name = "div") {
      name = String(name).toLowerCase();
      const element = {
        tagName: name.toUpperCase(),
        nodeName: name.toUpperCase(),
        width: name === "canvas" ? 300 : undefined,
        height: name === "canvas" ? 150 : undefined,
        outerHTML: `<${name}></${name}>`,
        baseURI: config.targetUrl,
        ownerDocument: document,
        parentNode: null,
        style: tagged({}, "CSSStyleDeclaration"),
        getContext(kind) {
          if (name !== "canvas") return null;
          if (kind === "2d") { canvasContext.canvas = element; return canvasContext; }
          if (kind === "webgl" || kind === "experimental-webgl") { webglContext.canvas = element; return webglContext; }
          return null;
        },
        toDataURL() { return config.canvasDataUrls[`${this.width}x${this.height}`] || "data:,"; },
        getAttribute() { return null; }, setAttribute() {},
        appendChild(child) {
          if (!child || typeof child !== "object") throw new TypeError("Failed to execute 'appendChild' on 'Node'");
          child.parentNode = this;
          return child;
        },
        addEventListener() {}, removeEventListener() {},
      };
      return tagged(markMethods(element), name === "canvas" ? "HTMLCanvasElement" : `HTML${name[0].toUpperCase()}${name.slice(1)}Element`);
    },
    addEventListener() {},
    removeEventListener() {},
  };
  tagged(document, "HTMLDocument");
  htmlElement.parentNode = document;
  Object.defineProperty(document, "cookie", {
    get() { return Array.from(cookieValues, ([name, value]) => `${name}=${value}`).join("; "); },
    set(value) {
      writtenCookie = String(value);
      const [pair, ...attributes] = writtenCookie.split(";");
      const separator = pair.indexOf("=");
      if (separator < 0) return;
      const name = pair.slice(0, separator).trim();
      const cookieValue = pair.slice(separator + 1).trim();
      const expired = attributes.some(attribute => /^\s*expires\s*=.*(?:1970|1969)/i.test(attribute));
      if (expired || !cookieValue) cookieValues.delete(name); else cookieValues.set(name, cookieValue);
      globalThis.__writtenCookie = writtenCookie;
    },
  });

  function Document() { this.all = blankDocumentAll; this.URL = "about:blank"; this.baseURI = "about:blank"; }
  tagged(Document.prototype, "Document");
  function HTMLDocument() { throw new TypeError("Illegal constructor"); }
  HTMLDocument.prototype = Object.create(Document.prototype, { constructor: { value: HTMLDocument } });
  function XMLDocument() { throw new TypeError("Illegal constructor"); }
  tagged(XMLDocument.prototype, "XMLDocument");
  Object.setPrototypeOf(document, HTMLDocument.prototype);

  function XMLHttpRequest() {
    this.readyState = 0; this.response = null; this.responseText = ""; this.responseType = "";
    this.responseURL = ""; this.responseXML = null; this.status = 0; this.statusText = "";
    this.timeout = 0; this.withCredentials = false; this.upload = tagged({}, "XMLHttpRequestUpload");
    this.onreadystatechange = null; this.onabort = null; this.onerror = null; this.onload = null;
    this.onloadend = null; this.onloadstart = null; this.onprogress = null; this.ontimeout = null;
  }
  Object.assign(XMLHttpRequest.prototype, {
    abort() {}, addEventListener() {}, getAllResponseHeaders() { return ""; }, getResponseHeader() { return null; },
    open() { this.readyState = 1; }, overrideMimeType() {}, removeEventListener() {}, send() {}, setRequestHeader() {},
  });
  tagged(XMLHttpRequest.prototype, "XMLHttpRequest");
  for (const [name, value] of Object.entries({ UNSENT: 0, OPENED: 1, HEADERS_RECEIVED: 2, LOADING: 3, DONE: 4 })) {
    Object.defineProperty(XMLHttpRequest, name, { value });
    Object.defineProperty(XMLHttpRequest.prototype, name, { value });
  }

  function MediaSource() { throw new TypeError("Illegal constructor"); }
  MediaSource.isTypeSupported = mime => String(mime).toLowerCase() === 'video/mp4; codecs="avc1.42e01e"';
  tagged(MediaSource.prototype, "MediaSource");
  const makeStorage = () => {
    const values = new Map();
    return tagged(markMethods({
      get length() { return values.size; },
      clear() { values.clear(); },
      getItem(key) { key = String(key); return values.has(key) ? values.get(key) : null; },
      key(index) { return Array.from(values.keys())[Number(index)] ?? null; },
      removeItem(key) { values.delete(String(key)); },
      setItem(key, value) { values.set(String(key), String(value)); },
    }), "Storage");
  };
  const navigator = tagged({
    userAgent: config.profile.userAgent,
    language: config.profile.language,
    languages: config.profile.languages,
    platform: config.profile.platform,
    hardwareConcurrency: config.profile.hardwareConcurrency,
    deviceMemory: config.profile.deviceMemory,
    webdriver: false,
    plugins: collection([
      "Chrome PDF Viewer", "Chromium PDF Viewer", "Microsoft Edge PDF Viewer",
      "PDF Viewer", "WebKit built-in PDF",
    ].map(name => tagged({ name, filename: "internal-pdf-viewer", description: "Portable Document Format" }, "Plugin")), "PluginArray"),
  }, "Navigator");
  function Navigator() { throw new TypeError("Illegal constructor"); }
  markNative(Navigator, "Navigator");
  tagged(Navigator.prototype, "Navigator");
  Object.setPrototypeOf(navigator, Navigator.prototype);
  const screen = tagged({ availLeft: 0, availTop: 0, ...config.profile.screen }, "Screen");
  function Screen() { throw new TypeError("Illegal constructor"); }
  markNative(Screen, "Screen");
  tagged(Screen.prototype, "Screen");
  Object.setPrototypeOf(screen, Screen.prototype);
  const history = tagged(markMethods({
    length: 1, scrollRestoration: "auto", state: null,
    back() {}, forward() {}, go() {}, pushState() {}, replaceState() {},
  }), "History");
  function History() { throw new TypeError("Illegal constructor"); }
  markNative(History, "History");
  tagged(History.prototype, "History");
  Object.setPrototypeOf(history, History.prototype);
  const customElements = tagged(markMethods({
    define() {}, get() {}, getName() { return null; }, upgrade() {}, whenDefined() { return Promise.resolve(); },
  }), "CustomElementRegistry");

  const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=";
  const atob = input => {
    input = String(input).replace(/[^A-Za-z0-9+/=]/g, "");
    let output = "";
    for (let i = 0; i < input.length;) {
      const e1 = base64Chars.indexOf(input[i++]); const e2 = base64Chars.indexOf(input[i++]);
      const e3 = base64Chars.indexOf(input[i++]); const e4 = base64Chars.indexOf(input[i++]);
      const c1 = (e1 << 2) | (e2 >> 4); const c2 = ((e2 & 15) << 4) | (e3 >> 2); const c3 = ((e3 & 3) << 6) | e4;
      output += String.fromCharCode(c1); if (e3 !== 64) output += String.fromCharCode(c2); if (e4 !== 64) output += String.fromCharCode(c3);
    }
    return output;
  };
  const btoa = input => {
    input = String(input); let output = "";
    for (let i = 0; i < input.length;) {
      const c1 = input.charCodeAt(i++); const c2 = input.charCodeAt(i++); const c3 = input.charCodeAt(i++);
      const e1 = c1 >> 2; const e2 = ((c1 & 3) << 4) | (c2 >> 4);
      let e3 = ((c2 & 15) << 2) | (c3 >> 6); let e4 = c3 & 63;
      if (Number.isNaN(c2)) e3 = e4 = 64; else if (Number.isNaN(c3)) e4 = 64;
      output += base64Chars[e1] + base64Chars[e2] + base64Chars[e3] + base64Chars[e4];
    }
    return output;
  };
  class TextEncoder {
    encode(text = "") {
      const encoded = unescape(encodeURIComponent(String(text)));
      return Array.from(encoded, char => char.charCodeAt(0));
    }
  }
  class TextDecoder {
    decode(bytes = new Uint8Array()) {
      let binary = ""; for (const byte of new Uint8Array(bytes.buffer || bytes, bytes.byteOffset || 0, bytes.byteLength ?? bytes.length)) binary += String.fromCharCode(byte);
      try { return decodeURIComponent(escape(binary)); } catch { return binary; }
    }
  }
  function URLSearchParams(input = "") {
    this._entries = [];
    const source = typeof input === "string" ? input.replace(/^\?/, "") : "";
    for (const part of source.split("&")) {
      if (!part) continue;
      const [key, ...rest] = part.split("=");
      this._entries.push([decodeURIComponent(key), decodeURIComponent(rest.join("="))]);
    }
  }
  Object.assign(URLSearchParams.prototype, {
    append(key, value) { this._entries.push([String(key), String(value)]); },
    get(key) { return this._entries.find(entry => entry[0] === String(key))?.[1] ?? null; },
    getAll(key) { return this._entries.filter(entry => entry[0] === String(key)).map(entry => entry[1]); },
    has(key) { return this._entries.some(entry => entry[0] === String(key)); },
    set(key, value) { this.delete(key); this.append(key, value); },
    delete(key) { this._entries = this._entries.filter(entry => entry[0] !== String(key)); },
    toString() { return this._entries.map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`).join("&"); },
  });
  tagged(URLSearchParams.prototype, "URLSearchParams");
  function URL(input, base = config.targetUrl) {
    input = String(input);
    base = String(base);
    let href = input;
    if (!/^[a-z][a-z0-9+.-]*:/i.test(href)) {
      const origin = base.match(/^([a-z][a-z0-9+.-]*:\/\/[^/]+)/i)?.[1] || "";
      if (href.startsWith("//")) href = base.match(/^[a-z][a-z0-9+.-]*:/i)?.[0] + href;
      else if (href.startsWith("/")) href = origin + href;
      else href = base.replace(/[?#].*$/, "").replace(/[^/]*$/, "") + href;
    }
    const match = href.match(/^([a-z][a-z0-9+.-]*:)(?:\/\/([^/?#]*))?([^?#]*)(\?[^#]*)?(#.*)?$/i);
    if (!match) throw new TypeError("Invalid URL");
    this.protocol = match[1]; this.host = match[2] || ""; this.hostname = this.host.replace(/:\d+$/, "");
    this.port = this.host.match(/:(\d+)$/)?.[1] || ""; this.pathname = match[3] || "/";
    this.search = match[4] || ""; this.hash = match[5] || ""; this.origin = `${this.protocol}//${this.host}`;
    this.href = `${this.origin}${this.pathname}${this.search}${this.hash}`;
    this.searchParams = new URLSearchParams(this.search);
  }
  URL.prototype.toString = function toString() { return this.href; };
  URL.prototype.toJSON = function toJSON() { return this.href; };
  tagged(URL.prototype, "URL");
  const blobData = new WeakMap();
  const blobUrls = new Map();
  let blobCounter = 0;
  function Blob(parts = [], options = {}) {
    const text = Array.from(parts, part => String(part)).join("");
    blobData.set(this, { text, type: String(options.type || "").toLowerCase() });
  }
  Object.defineProperties(Blob.prototype, {
    size: { configurable: true, get() { return unescape(encodeURIComponent(blobData.get(this).text)).length; } },
    type: { configurable: true, get() { return blobData.get(this).type; } },
  });
  tagged(Blob.prototype, "Blob");
  URL.createObjectURL = blob => {
    if (!blobData.has(blob)) throw new TypeError("Overload resolution failed");
    const value = `blob:${config.location.protocol}//${config.location.host}/${++blobCounter}`;
    blobUrls.set(value, blob);
    return value;
  };
  URL.revokeObjectURL = value => { blobUrls.delete(String(value)); };
  function Worker(url) {
    url = String(url);
    if (!blobUrls.has(url)) throw new TypeError("Failed to construct 'Worker'");
    this.onmessage = null;
    this.onerror = null;
    this._url = url;
  }
  Object.assign(Worker.prototype, {
    postMessage() {
      const worker = this;
      globalThis.setTimeout(() => {
        if (typeof worker.onmessage === "function") {
          worker.onmessage(tagged({ data: `Error\n    at self.onmessage (${worker._url}:1:52)` }, "MessageEvent"));
        }
      }, 0);
    },
    terminate() {}, addEventListener() {}, removeEventListener() {}, dispatchEvent() { return true; },
  });
  tagged(Worker.prototype, "Worker");
  // ponytail: this is the zse-ck one-shot worker path; add worker isolation only if another scraper executes worker code.
  markMethods(URL); markMethods(URL.prototype); markMethods(URLSearchParams.prototype);
  markMethods(Blob.prototype); markMethods(Worker.prototype);
  let randomIndex = 0;
  const crypto = {
    getRandomValues(array) {
      for (let i = 0; i < array.length; i++) {
        array[i] = config.randomValues[randomIndex++ % config.randomValues.length];
      }
      return array;
    },
  };
  const performance = { timeOrigin: Date.now(), now: () => Date.now() - performance.timeOrigin };
  const nativeError = globalThis.Error;
  const formatErrorStack = message => {
    const stack = String(config.errorStack || `Error\n    at ${config.targetUrl}:1:1`);
    return message ? stack.replace(/^Error(?=\r?\n)/, `Error: ${String(message)}`) : stack;
  };
  const browserError = new Proxy(nativeError, {
    apply(target, thisArg, args) {
      const error = Reflect.apply(target, thisArg, args);
      error.stack = formatErrorStack(error.message);
      return error;
    },
    construct(target, args, newTarget) {
      const error = Reflect.construct(target, args, newTarget);
      error.stack = formatErrorStack(error.message);
      return error;
    },
  });
  const setInterval = markNative(function setInterval(callback, delay, ...args) {
    return setTimeout(callback, delay, ...args);
  }, "setInterval");
  const clearInterval = markNative(function clearInterval(id) { clearTimeout(id); }, "clearInterval");
  markNative(globalThis.setTimeout, "setTimeout");
  markNative(globalThis.clearTimeout, "clearTimeout");
  const console = tagged({}, "console");
  for (const name of ["debug", "error", "info", "log", "table", "trace", "warn"]) {
    Object.defineProperty(console, name, { configurable: true, enumerable: true, value: markNative(function () {}, name) });
  }
  function Window() { throw new TypeError("Illegal constructor"); }
  Object.defineProperties(Window.prototype, {
    devicePixelRatio: { configurable: true, get: markNative(function () { return config.profile.devicePixelRatio; }, "devicePixelRatio", "get") },
    blur: { configurable: true, value: markNative(function blur() {}, "blur") },
  });
  tagged(Window.prototype, "Window");
  const addEventListener = markNative(function addEventListener() {}, "addEventListener");
  const removeEventListener = markNative(function removeEventListener() {}, "removeEventListener");
  for (const value of [currentScript, challengeMeta, htmlElement, document, canvasContext]) markMethods(value);
  for (const value of [XMLHttpRequest.prototype, TextEncoder.prototype, TextDecoder.prototype]) markMethods(value);
  for (const value of [Document, HTMLDocument, XMLDocument, Location, Navigator, Screen, History,
    XMLHttpRequest, MediaSource, TextEncoder, TextDecoder, URL, URLSearchParams, Blob, Worker, Window]) {
    markNative(value, value.name);
  }
  markNative(MediaSource.isTypeSupported, "isTypeSupported");
  markNative(crypto.getRandomValues, "getRandomValues");
  markNative(performance.now, "now");
  markNative(atob, "atob");
  markNative(btoa, "btoa");
  const globals = {
    window: globalThis, self: globalThis, top: globalThis, parent: globalThis, frames: globalThis,
    document, location, navigator, screen, history, customElements,
    sessionStorage: makeStorage(), localStorage: makeStorage(),
    Window, Document, HTMLDocument, XMLDocument, Location, Navigator, Screen, History,
    XMLHttpRequest, MediaSource,
    devicePixelRatio: config.profile.devicePixelRatio,
    crypto, performance, Error: browserError, TextEncoder, TextDecoder, URL, URLSearchParams, Blob, Worker,
    atob, btoa, setInterval, clearInterval, console,
    addEventListener, removeEventListener, blur: Window.prototype.blur,
    find: markNative(() => false, "find"), stop: markNative(() => {}, "stop"),
    open: markNative(() => null, "open"), print: markNative(() => {}, "print"), name: "", length: 0,
  };
  for (const [name, value] of Object.entries(globals)) {
    const descriptor = Object.getOwnPropertyDescriptor(globalThis, name);
    if (!descriptor || descriptor.configurable) {
      Object.defineProperty(globalThis, name, { configurable: true, enumerable: true, writable: true, value });
    } else if (descriptor.writable) {
      globalThis[name] = value;
    }
  }
  for (const [name, value] of Object.entries({
    window: globalThis, self: globalThis, top: globalThis, parent: globalThis, frames: globalThis,
    document, location, navigator, screen, history,
  })) {
    const descriptor = Object.getOwnPropertyDescriptor(globalThis, name);
    if (!descriptor || descriptor.configurable) {
      Object.defineProperty(globalThis, name, { configurable: false, enumerable: true, get: () => value });
    } else if (descriptor.writable) {
      globalThis[name] = value;
    }
  }
  Object.setPrototypeOf(globalThis, Window.prototype);
  Object.defineProperty(globalThis, Symbol.toStringTag, { configurable: true, value: "Window" });

  const browserTypeof = value => htmlDDAValues.has(value) ? "undefined" : typeof value;
  const browserBoolean = value => htmlDDAValues.has(value) ? false : Boolean(value);
  const browserNot = value => htmlDDAValues.has(value) ? true : !value;
  const browserLooseEqual = (left, right) => {
    if (htmlDDAValues.has(left) && (right === null || right === undefined)) return true;
    if (htmlDDAValues.has(right) && (left === null || left === undefined)) return true;
    return left == right;
  };
  const browserAnd = (left, right) => htmlDDAValues.has(left) ? left : left && right;
  const browserOr = (left, right) => htmlDDAValues.has(left) ? right : left || right;
  Object.defineProperties(globalThis, {
    __T: { configurable: true, value: browserTypeof },
    __B: { configurable: true, value: browserBoolean },
    __N: { configurable: true, value: browserNot },
    __E: { configurable: true, value: browserLooseEqual },
    __A: { configurable: true, value: browserAnd },
    __O: { configurable: true, value: browserOr },
  });
  const nativeEval = globalThis.eval;
  globalThis.eval = new Proxy(nativeEval, {
    apply(target, thisArg, args) {
      if (typeof args[0] === "string" && args[0].length > 1000) {
        const replaceSameLength = (source, before, after) => source
          .split(before).join(after.padEnd(before.length, " "));
        args[0] = replaceSameLength(args[0], "typeof this['A'][this['k']]", "__T(this.A[this.k])");
        args[0] = replaceSameLength(args[0], "!this['A'][this['k']]", "__N(this.A[this.k])");
        args[0] = replaceSameLength(args[0], "this['A'][this['k']]==this['A'][this['o']]", "__E(this.A[this.k],this.A[this.o])");
        args[0] = replaceSameLength(args[0], "this['A'][this['k']]&&this['A'][this['o']]", "__A(this.A[this.k],this.A[this.o])");
        args[0] = replaceSameLength(args[0], "this['A'][this['k']]||this['A'][this['o']]", "__O(this.A[this.k],this.A[this.o])");
        args[0] = replaceSameLength(args[0], "this['D']=this['A'][this['v']]", "this.D=__B(this.A[this.v])");
      }
      return Reflect.apply(target, globalThis, args);
    },
  });
  const nativeToString = Function.prototype.toString;
  Function.prototype.toString = new Proxy(nativeToString, {
    apply(target, thisArg, args) {
      if (nativeFunctions.has(thisArg)) return nativeFunctions.get(thisArg);
      if (thisArg === globalThis.eval) return "function eval() { [native code] }";
      if (thisArg === browserError) return "function Error() { [native code] }";
      if (thisArg === Document) return "function Document() { [native code] }";
      if (thisArg === HTMLDocument) return "function HTMLDocument() { [native code] }";
      if (thisArg === XMLDocument) return "function XMLDocument() { [native code] }";
      if (thisArg === XMLHttpRequest) return "function XMLHttpRequest() { [native code] }";
      return Reflect.apply(target, thisArg, args);
    },
  });
  const nativeInstantiate = WebAssembly.instantiate;
  WebAssembly.instantiate = async (bytes, imports) => {
    if (!imports || !imports.env || typeof imports.env["syscall/js.copyBytesToGo"] !== "function") {
      return nativeInstantiate(bytes, imports);
    }
    const originalCopy = imports.env["syscall/js.copyBytesToGo"];
    imports.env["syscall/js.copyBytesToGo"] = (...wasmArgs) => {
      const bridgeCopy = globalThis.__g.copyBytesToGo;
      globalThis.__g.copyBytesToGo = value => {
        const args = Array.from(value[0]);
        const operation = Number(args[1]);
        if (operation === 1030) {
          const result = bridgeCopy(value);
          // goja does not copy this obfuscated TinyGo bridge's plain byte array.
          const raw_reference = BigInt(args[2]);
          const source_reference = (raw_reference & ~0xffffffffn) | ((raw_reference + 1030n) & 0xffffffffn);
          const source = Array.from(value[1](source_reference) || []);
          const destination = new Uint8Array(value[3], Number(args[10]), Number(args[11]));
          source.forEach((byte, index) => { destination[index] = byte; });
          return result;
        }
        if (operation !== 1022 && operation !== 1024 && operation !== 1026 && operation !== 1034) {
          return bridgeCopy(value);
        }
        const method = value[2](Number(args[4]), Number(args[5]));
        const decodeReference = encoded => {
          const raw = BigInt(encoded);
          return (raw & ~0xffffffffn) | ((raw + BigInt(operation)) & 0xffffffffn);
        };
        if (operation === 1022) {
          const receiver = value[1](decodeReference(args[2]));
          receiver[method] = value[1](args[3]);
          return;
        }
        if (operation === 1024) {
          const receiver = value[1](decodeReference(args[2]));
          let result = receiver == null ? undefined : receiver[method];
          if (result === undefined && method in document) result = document[method];
          value[4](Number(args[0]), result);
          return;
        }
        const callArgs = value[6](Number(args[7]), Number(args[8]), Number(args[9]));
        if (operation === 1026) {
          let receiver = value[1](decodeReference(args[2]));
          let fn = receiver == null ? undefined : receiver[method];
          if (typeof fn !== "function" && typeof document[method] === "function") {
            receiver = document;
            fn = document[method];
          }
          const result = method === "eval"
            ? Reflect.apply(globalThis.eval, globalThis, callArgs)
            : Reflect.apply(fn, receiver, callArgs);
          value[4](Number(args[0]), result);
          new DataView(value[3]).setUint8(Number(args[0]) + 8, 1);
          return;
        }
        if (operation === 1034) {
          const fn = value[1](decodeReference(args[2]));
          const result = Reflect.apply(fn, undefined, callArgs);
          value[4](Number(args[0]), result);
          new DataView(value[3]).setUint8(Number(args[0]) + 8, 1);
          return;
        }
        return bridgeCopy(value);
      };
      try {
        return originalCopy(...wasmArgs);
      } finally {
        globalThis.__g.copyBytesToGo = bridgeCopy;
      }
    };
    if (imports.gojs) imports.gojs["syscall/js.copyBytesToGo"] = imports.env["syscall/js.copyBytesToGo"];
    const instantiated = await nativeInstantiate(bytes, imports);
    return instantiated;
  };
  globalThis.__writtenCookie = "";
  return true;
};
