(function installPcwebBrowserRuntime() {
// Host primitives are installed by pcweb_goja.go. This file deliberately has
// no CommonJS or Node runtime dependency.
const hostProcess = {
  env: { ZSE_STRICT_BROWSER: "1" },
  stderr: { write() {} },
  stdout: { write() {} },
};
const process = hostProcess;
const fs = { writeFileSync() {} };
const strictBrowserMode = true;
const profile = __goja_profile;
const meta = __goja_meta;
const targetUrl = __goja_target_url;
const parsedUrl = __goja_parse_url(targetUrl, "");
let writtenCookie = "";
const fingerprintProperties = [];
const nativeLikeFunctions = new WeakMap();
const htmlDDAValues = new WeakSet();

function markNativeFunction(value, name, kind = "") {
  if (typeof value !== "function") return value;
  const normalizedName = typeof name === "symbol" ? `[${name.description || ""}]` : String(name || value.name || "");
  const prefix = kind ? `${kind} ` : "";
  nativeLikeFunctions.set(value, `function ${prefix}${normalizedName}() { [native code] }`);
  return value;
}

function markNativeMethods(value) {
  if (value == null) return value;
  for (const key of Reflect.ownKeys(value)) {
    let descriptor;
    try { descriptor = Object.getOwnPropertyDescriptor(value, key); } catch { continue; }
    if (!descriptor) continue;
    if (typeof descriptor.value === "function") markNativeFunction(descriptor.value, key);
    if (typeof descriptor.get === "function") markNativeFunction(descriptor.get, key, "get");
    if (typeof descriptor.set === "function") markNativeFunction(descriptor.set, key, "set");
  }
  return value;
}

const browserOnlyHiddenGlobals = new Set(["global", "process", "Buffer", "setImmediate", "clearImmediate"]);
const browserWindow = new Proxy(globalThis, {
  get(target, property, receiver) {
    if (strictBrowserMode && browserOnlyHiddenGlobals.has(property)) return undefined;
    if (property === "globalThis") return browserWindow;
    return Reflect.get(target, property, receiver);
  },
  has(target, property) {
    if (strictBrowserMode && browserOnlyHiddenGlobals.has(property)) return false;
    return Reflect.has(target, property);
  },
  getOwnPropertyDescriptor(target, property) {
    if (strictBrowserMode && browserOnlyHiddenGlobals.has(property)) {
      const descriptor = Reflect.getOwnPropertyDescriptor(target, property);
      if (!descriptor || descriptor.configurable) return undefined;
    }
    return Reflect.getOwnPropertyDescriptor(target, property);
  },
});

const webcrypto = __goja_webcrypto;
const performance = __goja_performance;
const blankCanvasDataURL = __goja_blank_canvas_data_url;

const locationValue = {
  href: parsedUrl.href,
  protocol: parsedUrl.protocol,
  host: parsedUrl.host,
  hostname: parsedUrl.hostname,
  pathname: parsedUrl.pathname,
  search: parsedUrl.search,
  reload() {},
  toString() { return this.href; },
};
Object.defineProperty(locationValue, Symbol.toStringTag, { value: "Location" });
const currentScript = {
  src: __goja_script_url,
  getAttribute(name) {
    return name === "data-assets-tracker-config"
      ? '{"appName":"zse_ck","trackJSRuntimeError":true}'
      : null;
  },
};
Object.defineProperty(currentScript, Symbol.toStringTag, { value: "HTMLScriptElement" });
const challengeMetaElement = {
  id: "zh-zse-ck",
  content: meta,
  getAttribute(name) {
    if (name === "id") return this.id;
    if (name === "content") return this.content;
    return null;
  },
};
Object.defineProperty(challengeMetaElement, Symbol.toStringTag, { value: "HTMLMetaElement" });
const htmlElementValue = {
  tagName: "HTML",
  nodeName: "HTML",
  getElementsByTagName(name) {
    const normalizedName = String(name).toLowerCase();
    if (normalizedName === "script") return makeCollection([currentScript]);
    if (normalizedName === "meta") return makeCollection([challengeMetaElement]);
    return makeCollection([]);
  },
};
Object.defineProperty(htmlElementValue, Symbol.toStringTag, { value: "HTMLHtmlElement" });
function makeCollection(values, tag = "HTMLCollection") {
  const collection = {
    item(index) { return this[index] ?? null; },
    namedItem(name) {
      return values.find((value) => value && (value.id === name || value.name === name)) || null;
    },
  };
  values.forEach((value, index) => {
    Object.defineProperty(collection, index, { configurable: true, enumerable: true, value });
  });
  Object.defineProperty(collection, "length", { configurable: true, value: values.length });
  Object.defineProperty(collection, Symbol.iterator, {
    configurable: true,
    value: function* iterator() { yield* values; },
  });
  Object.defineProperty(collection, Symbol.toStringTag, { value: tag });
  return markNativeMethods(collection);
}
function makeDocumentAll(firstElement, length) {
  function all(nameOrIndex) {
    return firstElement && (nameOrIndex === 0 || nameOrIndex === "0" || nameOrIndex == null)
      ? firstElement
      : null;
  }
  if (firstElement) all[0] = firstElement;
  Object.defineProperty(all, "length", { configurable: true, value: length });
  Object.defineProperty(all, Symbol.toStringTag, { value: "HTMLAllCollection" });
  if (strictBrowserMode) {
    Object.defineProperty(all, "toString", {
      configurable: true,
      value: function toString() { return Object.prototype.toString.call(this); },
    });
    markNativeMethods(all);
  }
  htmlDDAValues.add(all);
  return all;
}
const documentAllValue = makeDocumentAll(
  htmlElementValue,
  strictBrowserMode ? 6 : 1,
);
const documentBodyValue = { tagName: "BODY", nodeName: "BODY", style: {} };
Object.defineProperty(documentBodyValue.style, Symbol.toStringTag, { value: "CSSStyleDeclaration" });
Object.defineProperty(documentBodyValue, Symbol.toStringTag, { value: "HTMLBodyElement" });
const documentValue = {
  currentScript,
  URL: targetUrl,
  baseURI: targetUrl,
  all: documentAllValue,
  body: documentBodyValue,
  referrer: "",
  title: "",
  charset: "UTF-8",
  readyState: "complete",
  getElementById(id) {
    return id === "zh-zse-ck" ? { getAttribute: (name) => name === "content" ? meta : null } : null;
  },
  getElementsByTagName(name) {
    const normalizedName = String(name).toLowerCase();
    if (normalizedName === "meta") return makeCollection([challengeMetaElement]);
    if (normalizedName === "script") return makeCollection([currentScript]);
    if (normalizedName === "html" || normalizedName === "*") return makeCollection([documentAllValue[0]]);
    return makeCollection([]);
  },
  querySelector(selector) { return selector === "script[data-assets-tracker-config]" ? currentScript : null; },
  createNodeIterator(root) {
    let consumed = false;
    const iterator = {
      root,
      referenceNode: root,
      pointerBeforeReferenceNode: true,
      whatToShow: 0xffffffff,
      filter: null,
      nextNode() {
        if (consumed) return null;
        consumed = true;
        return root;
      },
      previousNode() { return null; },
      detach() {},
    };
    Object.defineProperty(iterator, Symbol.toStringTag, { value: "NodeIterator" });
    return markNativeMethods(iterator);
  },
  createRange() {
    const range = {
      collapsed: true,
      commonAncestorContainer: documentValue,
      startContainer: documentValue,
      startOffset: 0,
      endContainer: documentValue,
      endOffset: 0,
      cloneContents() {
        const fragment = {};
        Object.defineProperty(fragment, Symbol.toStringTag, { value: "DocumentFragment" });
        return fragment;
      },
      cloneRange() { return documentValue.createRange(); },
      collapse() {}, compareBoundaryPoints() { return 0; },
      comparePoint() { return 0; }, intersectsNode() { return false; }, isPointInRange() { return false; },
      deleteContents() {}, detach() {}, extractContents() { return {}; },
      getBoundingClientRect() { return { x: 0, y: 0, width: 0, height: 0 }; },
      getClientRects() { return []; }, insertNode() {}, selectNode() {},
      selectNodeContents() {}, setEnd() {}, setEndAfter() {}, setEndBefore() {},
      setStart() {}, setStartAfter() {}, setStartBefore() {}, surroundContents() {},
      toString() { return ""; },
    };
    Object.defineProperty(range, Symbol.toStringTag, { value: "Range" });
    return markNativeMethods(range);
  },
  createElement(name = "div") {
    const normalizedName = String(name).toLowerCase();
    const element = {
      tagName: normalizedName.toUpperCase(),
      nodeName: normalizedName.toUpperCase(),
      width: normalizedName === "canvas" ? 300 : undefined,
      height: normalizedName === "canvas" ? 150 : undefined,
      outerHTML: `<${normalizedName}></${normalizedName}>`,
      baseURI: targetUrl,
      ownerDocument: documentValue,
      style: {},
      getContext(kind) {
        if (normalizedName !== "canvas") return null;
        if (kind === "2d") return canvas2dValue;
        return null;
      },
      toDataURL() {
        return blankCanvasDataURL;
      },
      setAttribute() {}, appendChild() {}, addEventListener() {}, removeEventListener() {},
    };
    const tag = normalizedName === "canvas"
      ? "HTMLCanvasElement"
      : `HTML${normalizedName.charAt(0).toUpperCase()}${normalizedName.slice(1)}Element`;
    Object.defineProperty(element, Symbol.toStringTag, { value: tag });
    Object.defineProperty(element.style, Symbol.toStringTag, { value: "CSSStyleDeclaration" });
    return markNativeMethods(element);
  },
  addEventListener() {},
  removeEventListener() {},
};
const canvas2dValue = {
  canvas: null,
  fillStyle: "#000000",
  font: "10px sans-serif",
  textBaseline: "alphabetic",
  fillRect() {}, clearRect() {}, fillText() {}, strokeText() {},
  beginPath() {}, closePath() {}, moveTo() {}, lineTo() {}, arc() {}, stroke() {}, fill() {},
  measureText(text) { return { width: String(text).length * 5 }; },
  getImageData() { return { data: new Uint8ClampedArray(4), width: 1, height: 1 }; },
};
Object.defineProperty(canvas2dValue, Symbol.toStringTag, { value: "CanvasRenderingContext2D" });
Object.defineProperty(documentValue, Symbol.toStringTag, { value: "HTMLDocument" });
Object.defineProperty(documentValue, "cookie", {
  get() { return writtenCookie; },
  set(value) { writtenCookie = value; },
});

const navigatorValue = {
  userAgent: profile.userAgent,
  language: profile.language,
  languages: profile.languages,
  platform: profile.platform,
  hardwareConcurrency: profile.hardwareConcurrency,
  deviceMemory: profile.deviceMemory,
  webdriver: profile.webdriver,
};
Object.defineProperty(navigatorValue, Symbol.toStringTag, { value: "Navigator" });
const screenValue = { availLeft: 0, availTop: 0, ...profile.screen };
Object.defineProperty(screenValue, Symbol.toStringTag, { value: "Screen" });
const historyValue = {
  length: 1,
  scrollRestoration: "auto",
  state: null,
  back() {},
  forward() {},
  go() {},
  pushState() {},
  replaceState() {},
};
Object.defineProperty(historyValue, Symbol.toStringTag, { value: "History" });
function Document() {
  this.all = makeDocumentAll(null, 0);
  this.URL = "about:blank";
  this.baseURI = "about:blank";
}
Object.defineProperty(Document.prototype, Symbol.toStringTag, { value: "Document" });
function HTMLDocument() {
  throw new TypeError("Illegal constructor");
}
HTMLDocument.prototype = Object.create(Document.prototype, {
  constructor: { value: HTMLDocument, writable: true, configurable: true },
});
Object.setPrototypeOf(documentValue, HTMLDocument.prototype);
function XMLDocument() {
  throw new TypeError("Illegal constructor");
}
Object.defineProperty(XMLDocument.prototype, Symbol.toStringTag, { value: "XMLDocument" });
function XMLHttpRequest() {
  this.readyState = 0;
  this.response = null;
  this.responseText = "";
  this.responseType = "";
  this.responseURL = "";
  this.responseXML = null;
  this.status = 0;
  this.statusText = "";
  this.timeout = 0;
  this.upload = {};
  Object.defineProperty(this.upload, Symbol.toStringTag, { value: "XMLHttpRequestUpload" });
  this.withCredentials = false;
  this.onreadystatechange = null;
  this.onabort = null;
  this.onerror = null;
  this.onload = null;
  this.onloadend = null;
  this.onloadstart = null;
  this.onprogress = null;
  this.ontimeout = null;
}
Object.defineProperty(XMLHttpRequest.prototype, "upload", {
  configurable: true,
  get() {
    if (!this.__upload) {
      this.__upload = {};
      Object.defineProperty(this.__upload, Symbol.toStringTag, { value: "XMLHttpRequestUpload" });
    }
    return this.__upload;
  },
});
for (const [name, value] of Object.entries({
  UNSENT: 0, OPENED: 1, HEADERS_RECEIVED: 2, LOADING: 3, DONE: 4,
})) {
  Object.defineProperty(XMLHttpRequest, name, { enumerable: true, value });
  Object.defineProperty(XMLHttpRequest.prototype, name, { enumerable: true, value });
}
Object.assign(XMLHttpRequest.prototype, {
  abort() {},
  addEventListener() {},
  getAllResponseHeaders() { return ""; },
  getResponseHeader() { return null; },
  open() { this.readyState = 1; },
  overrideMimeType() {},
  removeEventListener() {},
  send() {},
  setRequestHeader() {},
});
Object.defineProperty(XMLHttpRequest.prototype, Symbol.toStringTag, { value: "XMLHttpRequest" });
function find() { return false; }
function stop() {}
function open() { return null; }
function print() {}
function MediaSource() {
  throw new TypeError("Illegal constructor");
}
MediaSource.isTypeSupported = function isTypeSupported(mimeType) {
  return String(mimeType).toLowerCase() === 'video/mp4; codecs="avc1.42e01e"';
};
Object.defineProperty(MediaSource.prototype, Symbol.toStringTag, { value: "MediaSource" });
function makeStorage() {
  const values = new Map();
  const storage = {
    get length() { return values.size; },
    clear() { values.clear(); },
    getItem(key) {
      const normalizedKey = String(key);
      return values.has(normalizedKey) ? values.get(normalizedKey) : null;
    },
    key(index) { return Array.from(values.keys())[Number(index)] ?? null; },
    removeItem(key) { values.delete(String(key)); },
    setItem(key, value) { values.set(String(key), String(value)); },
  };
  Object.defineProperty(storage, Symbol.toStringTag, { value: "Storage" });
  return storage;
}
const sessionStorageValue = makeStorage();
const localStorageValue = makeStorage();
const customElementsValue = {
  define() {}, get() { return undefined; }, getName() { return null; },
  upgrade() {}, whenDefined() { return Promise.resolve(); },
};
Object.defineProperty(customElementsValue, Symbol.toStringTag, { value: "CustomElementRegistry" });
const browserError = new Proxy(Error, {
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

function formatErrorStack(message) {
  const normalizedMessage = String(message || "");
  const stack = profile.errorStack.replace("{message}", normalizedMessage);
  return normalizedMessage ? stack : stack.replace(/^Error:\s*(?=\r?\n)/, "Error");
}

const browserGlobals = {
  window: browserWindow, self: browserWindow, document: documentValue, location: locationValue,
  navigator: navigatorValue, screen: screenValue, devicePixelRatio: profile.devicePixelRatio,
  history: historyValue,
  Document,
  HTMLDocument,
  XMLDocument,
  XMLHttpRequest,
  MediaSource,
  name: "",
  crypto: webcrypto, performance, Error: browserError,
  sessionStorage: sessionStorageValue,
  localStorage: localStorageValue,
  customElements: customElementsValue,
  addEventListener() {}, removeEventListener() {}, blur() {}, find, stop, open, print,
};
for (const [name, value] of Object.entries(browserGlobals)) {
  const existing = Object.getOwnPropertyDescriptor(globalThis, name);
  if (existing && !existing.configurable) globalThis[name] = value;
  else {
    Object.defineProperty(globalThis, name, {
      configurable: true,
      enumerable: true,
      writable: true,
      value,
    });
  }
}
for (const [name, getter] of [
  ["window", () => browserWindow],
  ["self", () => browserWindow],
  ["document", () => documentValue],
  ["location", () => locationValue],
  ["navigator", () => navigatorValue],
  ["screen", () => screenValue],
  ["history", () => historyValue],
  ["top", () => browserWindow],
  ["parent", () => browserWindow],
  ["frames", () => browserWindow],
]) {
  Object.defineProperty(globalThis, name, {
    configurable: false,
    enumerable: true,
    get: getter,
  });
}
Object.defineProperty(globalThis, "length", { configurable: true, value: 0 });
if (!strictBrowserMode) {
  Object.defineProperty(globalThis, "global", {
    configurable: true,
    enumerable: false,
    get() { return globalThis; },
  });
} else {
  // browserWindow hides Node-only globals while the host runtime keeps them.
}
Object.defineProperty(globalThis, Symbol.toStringTag, { configurable: true, value: "Window" });

const nativeToString = Function.prototype.toString;
const nativeBigInt = globalThis.BigInt;
const blurFunction = globalThis.blur;
const locationToString = locationValue.toString;
const nativeEval = globalThis.eval;
const browserEval = new Proxy(nativeEval, {
  apply(target, thisArg, args) {
    try {
      if (strictBrowserMode && typeof args[0] === "string" && args[0].length > 1000) {
        args[0] = args[0]
          .split("typeof this['A'][this['k']]").join("__browserTypeof(this['A'][this['k']])")
          .split("!this['A'][this['k']]").join("__browserNot(this['A'][this['k']])")
          .split("this['A'][this['k']]==this['A'][this['o']]").join("__browserLooseEqual(this['A'][this['k']],this['A'][this['o']])")
          .split("this['A'][this['k']]&&this['A'][this['o']]").join("__browserAnd(this['A'][this['k']],this['A'][this['o']])")
          .split("this['A'][this['k']]||this['A'][this['o']]").join("__browserOr(this['A'][this['k']],this['A'][this['o']])")
          .split("this['D']=this['A'][this['v']]").join("this['D']=__browserBoolean(this['A'][this['v']])");
      }
      if (hostProcess.env.ZSE_NESTED_OUTPUT && typeof args[0] === "string" && args[0].length > 1000) {
        fs.writeFileSync(hostProcess.env.ZSE_NESTED_OUTPUT, args[0], "utf8");
      }
      if (process.env.ZSE_DEBUG === "1" && globalThis.__b && globalThis.__b.A === 21 && typeof args[0] === "string" && args[0].length > 1000) {
        if (hostProcess.env.ZSE_NESTED_OUTPUT) {
          fs.writeFileSync(hostProcess.env.ZSE_NESTED_OUTPUT, args[0], "utf8");
        }
        globalThis.q3 = (object, property) => {
          let label = typeof object;
          try { label = Object.prototype.toString.call(object); } catch {}
          if (object === globalThis) label = "window";
          else if (object === documentValue) label = "document";
          else if (object === navigatorValue) label = "navigator";
          else if (object === screenValue) label = "screen";
          else if (object === locationValue) label = "location";
          let keys = "";
          try { keys = Reflect.ownKeys(object).slice(0, 40).map(String).join(","); } catch {}
          const event = { label, property: String(property), keys, J: globalThis.__c && globalThis.__c.J };
          try {
            let result = Reflect.get(Object(object), property);
            if (result === undefined && label === "[object Object]" && keys === "" && property in documentValue) {
              result = documentValue[property];
              event.compat = "document";
            }
            if (property === "URL" && result === undefined && label === "[object Object]") {
              result = targetUrl;
              event.compat = "document-url";
            }
            if (property === "all" && result === undefined && label === "[object Object]") {
              result = documentAllValue;
              event.compat = "document-all";
            }
            event.result = typeof result;
            if (result == null || ["string", "number", "boolean"].includes(typeof result)) {
              event.value = result;
            }
            fingerprintProperties.push(event);
            process.stderr.write(`cvm_property=${JSON.stringify(event)}\n`);
            return result;
          } catch (error) {
            event.result = `throws:${error}`;
            process.stderr.write(`cvm_property=${JSON.stringify(event)}\n`);
            throw error;
          }
        };
        globalThis.q3_set = (object, property, value) => {
          let label = typeof object;
          try { label = Object.prototype.toString.call(object); } catch {}
          process.stderr.write(`cvm_property_set=${JSON.stringify({ label, property: String(property), type: typeof value, J: globalThis.__c && globalThis.__c.J })}\n`);
          return Reflect.set(Object(object), property, value);
        };
        globalThis.q3_call = (object, property, callArgs) => {
          let label = typeof object;
          try { label = Object.prototype.toString.call(object); } catch {}
          let fn = object == null ? undefined : object[property];
          let receiver = object;
          let compat = "";
          if (typeof fn !== "function" && label === "[object Object]" && typeof documentValue[property] === "function") {
            fn = documentValue[property];
            receiver = documentValue;
            compat = "document";
          }
          const callResult = Reflect.apply(fn, receiver, callArgs);
          const resultPreview = callResult == null || ["number", "boolean"].includes(typeof callResult)
            ? callResult
            : typeof callResult === "string"
              ? callResult.slice(0, 500)
            : typeof callResult;
          process.stderr.write(`cvm_call=${JSON.stringify({ label, property: String(property), fn: typeof fn, argc: callArgs.length, args: callArgs.map((item) => typeof item === "string" ? item.slice(0, 500) : typeof item), result: resultPreview, compat })}\n`);
          return callResult;
        };
        args[0] = args[0]
          .split("this['A'][this['k']][_0x2f480b]=_0x580678").join("q3_set(this.A[this.k],_0x2f480b,_0x580678)")
          .split("this['A'][this['k']][_0x2f480b]").join("q3(this.A[this.k],_0x2f480b)")
          .split("this['A'][this['N']][_0x2cdf8f](..._0x29649d)").join("q3_call(this.A[this.N],_0x2cdf8f,_0x29649d)")
          .replace("catch(_0x306b0a){return;}", "catch(_0x306b0a){process.stderr.write('cvm_failure='+String(_0x306b0a)+'\\n');throw _0x306b0a;}");
      }
      const result = Reflect.apply(target, thisArg, args);
      if (process.env.ZSE_DEBUG === "1") {
        const source = typeof args[0] === "string" ? args[0].slice(0, 240) : String(args[0]);
        const bvmState = globalThis.__b ? `@${globalThis.__b.A}/${globalThis.__b.k}` : "";
        if (process.env.ZSE_NESTED_OUTPUT && globalThis.__b && globalThis.__b.A === 21 && typeof args[0] === "string" && args[0].length > 1000) {
          fs.writeFileSync(process.env.ZSE_NESTED_OUTPUT, args[0], "utf8");
        }
        process.stderr.write(`dynamic_eval${bvmState}=${JSON.stringify(source)}=>${typeof result}:${String(result).slice(0, 160)}\n`);
      }
      return result;
    }
    catch (error) {
      if (process.env.ZSE_DEBUG === "1") process.stderr.write(`eval_error=${error.stack || error}\n`);
      throw error;
    }
  },
});
Object.defineProperty(globalThis, "__browserTypeof", {
  configurable: true,
  value(value) { return htmlDDAValues.has(value) ? "undefined" : typeof value; },
});
Object.defineProperties(globalThis, {
  __browserBoolean: {
    configurable: true,
    value(value) { return htmlDDAValues.has(value) ? false : Boolean(value); },
  },
  __browserNot: {
    configurable: true,
    value(value) { return htmlDDAValues.has(value) ? true : !value; },
  },
  __browserLooseEqual: {
    configurable: true,
    value(left, right) {
      if ((htmlDDAValues.has(left) && right == null) || (htmlDDAValues.has(right) && left == null)) return true;
      return left == right;
    },
  },
  __browserAnd: {
    configurable: true,
    value(left, right) { return htmlDDAValues.has(left) ? left : left && right; },
  },
  __browserOr: {
    configurable: true,
    value(left, right) { return htmlDDAValues.has(left) ? right : left || right; },
  },
});
globalThis.eval = browserEval;
if (strictBrowserMode) {
  for (const value of [
    locationValue,
    currentScript,
    challengeMetaElement,
    htmlElementValue,
    documentAllValue,
    documentValue,
    canvas2dValue,
    historyValue,
    XMLHttpRequest.prototype,
    sessionStorageValue,
    localStorageValue,
    customElementsValue,
    browserGlobals,
    globalThis,
  ]) markNativeMethods(value);
  for (const constructor of [Document, HTMLDocument, XMLDocument, XMLHttpRequest, MediaSource]) {
    markNativeFunction(constructor, constructor.name);
  }
  markNativeMethods(MediaSource);
  markNativeFunction(browserEval, "eval");
}
let browserToString;
browserToString = new Proxy(nativeToString, {
  apply(target, thisArg, args) {
    if (strictBrowserMode && nativeLikeFunctions.has(thisArg)) return nativeLikeFunctions.get(thisArg);
    if (thisArg === blurFunction) return profile.nativeFunctions.blur;
    if (thisArg === browserEval) return "function eval() { [native code] }";
    if (thisArg === XMLDocument) return "function XMLDocument() { [native code] }";
    if (thisArg === Document) return "function Document() { [native code] }";
    if (thisArg === HTMLDocument) return "function HTMLDocument() { [native code] }";
    if (thisArg === XMLHttpRequest) return "function XMLHttpRequest() { [native code] }";
    if (thisArg === find) return "function find() { [native code] }";
    if (thisArg === stop) return "function stop() { [native code] }";
    if (thisArg === open) return "function open() { [native code] }";
    if (thisArg === print) return "function print() { [native code] }";
    if (thisArg === locationToString || thisArg === browserToString) return profile.nativeFunctions.toString;
    return Reflect.apply(target, thisArg, args);
  },
});
Function.prototype.toString = browserToString;

{
  const nativeInstantiate = WebAssembly.instantiate;
  WebAssembly.instantiate = async (bytes, imports) => {
    if (!imports || !imports.env || typeof imports.env["syscall/js.copyBytesToGo"] !== "function") {
      return nativeInstantiate(bytes, imports);
    }
    if (process.env.ZSE_BRIDGE_OUTPUT) {
      fs.writeFileSync(process.env.ZSE_BRIDGE_OUTPUT, String(globalThis.__g.copyBytesToGo), "utf8");
    }
    const originalCopy = imports.env["syscall/js.copyBytesToGo"];
    let callCount = 0;
    imports.env["syscall/js.copyBytesToGo"] = (...wasmArgs) => {
      callCount += 1;
      let payload;
      let payloadBefore;
      const bridgeCopy = globalThis.__g.copyBytesToGo;
      globalThis.__g.copyBytesToGo = (value) => {
        payload = value;
        payloadBefore = Array.from(value[0]);
        const bridgeArgs = payloadBefore;
        const operation = Number(bridgeArgs[1]);
        const method = value[2](Number(bridgeArgs[4]), Number(bridgeArgs[5]));
        const decodeReference = (encoded) => {
          const raw = BigInt(encoded);
          return (raw & ~0xffffffffn) | ((raw + BigInt(operation)) & 0xffffffffn);
        };
        if (operation === 1022) {
          const receiver = value[1](decodeReference(bridgeArgs[2]));
          const propertyValue = value[1](bridgeArgs[3]);
          if (receiver == null) throw new TypeError(`cannot set ${method} on ${receiver}`);
          receiver[method] = propertyValue;
          if (process.env.ZSE_DEBUG === "1") {
            process.stderr.write(`bridge_set=${JSON.stringify({ method, receiver: Object.prototype.toString.call(receiver), value: typeof propertyValue })}\n`);
          }
          return undefined;
        }
        if (operation === 1024) {
          const receiver = value[1](decodeReference(bridgeArgs[2]));
          let propertyValue = receiver == null ? undefined : receiver[method];
          if (propertyValue === undefined && method in documentValue) {
            propertyValue = documentValue[method];
          }
          if (process.env.ZSE_DEBUG === "1") {
            process.stderr.write(`bridge_get=${JSON.stringify({ method, receiver: Object.prototype.toString.call(receiver), result: typeof propertyValue })}\n`);
            if (method === "qh" || method === "zh") {
              process.stderr.write(`bridge_value=${JSON.stringify({ method, value: String(propertyValue) })}\n`);
            }
          }
          value[4](Number(bridgeArgs[0]), propertyValue);
          return undefined;
        }
        if (operation === 1026 && method !== "eval") {
          let receiver = value[1](decodeReference(bridgeArgs[2]));
          const callArgs = value[6](
            Number(bridgeArgs[7]),
            Number(bridgeArgs[8]),
            Number(bridgeArgs[9]),
          );
          let fn = receiver == null ? undefined : receiver[method];
          if (typeof fn !== "function" && typeof documentValue[method] === "function") {
            receiver = documentValue;
            fn = documentValue[method];
          }
          if (process.env.ZSE_DEBUG === "1") {
            process.stderr.write(`bridge_invoke=${JSON.stringify({ method, receiver: Object.prototype.toString.call(receiver), fn: typeof fn, argc: callArgs.length })}\n`);
          }
          const callResult = Reflect.apply(fn, receiver, callArgs);
          value[4](Number(bridgeArgs[0]), callResult);
          new DataView(value[3]).setUint8(Number(bridgeArgs[0]) + 8, 1);
          return undefined;
        }
        if (operation === 1034) {
          const fn = value[1](decodeReference(bridgeArgs[2]));
          const callArgs = value[6](
            Number(bridgeArgs[7]),
            Number(bridgeArgs[8]),
            Number(bridgeArgs[9]),
          );
          if (process.env.ZSE_DEBUG === "1") {
            const raw = BigInt(bridgeArgs[2]);
            const decoded = decodeReference(raw);
            const refs = globalThis._g && globalThis._g.i;
            const functionRefs = refs
              ? Reflect.ownKeys(refs).filter((key) => typeof refs[key] === "function").slice(-30).map(String)
              : [];
            process.stderr.write(`bridge_invoke_value=${JSON.stringify({ raw: raw.toString(16), decoded: decoded.toString(16), fn: typeof fn, preview: String(fn).slice(0, 80), argc: callArgs.length, functionRefs })}\n`);
          }
          const callResult = Reflect.apply(fn, undefined, callArgs);
          value[4](Number(bridgeArgs[0]), callResult);
          new DataView(value[3]).setUint8(Number(bridgeArgs[0]) + 8, 1);
          return undefined;
        }
        if (operation === 1026 && method === "eval") {
          const callArgs = value[6](
            Number(bridgeArgs[7]),
            Number(bridgeArgs[8]),
            Number(bridgeArgs[9]),
          );
          if (process.env.ZSE_DEBUG === "1" && typeof callArgs[0] === "string") {
            const trace = [];
            const changes = [];
            const propertyEvents = [];
            const previousRegisters = new WeakMap();
            const instanceIds = new WeakMap();
            let nextInstanceId = 1;
            let previousWindow = globalThis.window;
            const windowTransitions = [];
            globalThis.__zseTraceStep = (instance, pc) => {
              if (!instanceIds.has(instance)) instanceIds.set(instance, nextInstanceId++);
              const registerValues = Array.from(instance.F || []).slice(0, 5);
              const preview = Array.from(instance.F || []).slice(0, 5).map((entry) => {
                if (entry === null) return "null";
                if (entry === undefined) return "undefined";
                if (entry === globalThis) return "object:window";
                if (entry === documentValue) return "object:document";
                if (entry === navigatorValue) return "object:navigator";
                if (entry === screenValue) return "object:screen";
                if (entry === locationValue) return "object:location";
                const type = typeof entry;
                if (type === "string" || type === "number" || type === "boolean") {
                  return `${type}:${String(entry).slice(0, 80)}`;
                }
                let keys = "";
                try { keys = Reflect.ownKeys(entry).slice(0, 12).map(String).join("|"); } catch {}
                return `${type}:${Object.prototype.toString.call(entry)}:${keys}`;
              });
              const state = { id: instanceIds.get(instance), pc, k: instance.k, A: instance.A, Z: instance.Z, P: instance.P, T: instance.T, O: instance.O, N: instance.N, K: instance.K, Bn: typeof (instance.B || [])[instance.N], Bk: typeof (instance.B || [])[instance.K], F: preview };
              trace.push(state);
              if (trace.length > 80) trace.shift();
              if (globalThis.window !== previousWindow) {
                let keys = "";
                try { keys = Reflect.ownKeys(globalThis.window).slice(0, 20).map(String).join("|"); } catch {}
                windowTransitions.push({ id: instanceIds.get(instance), pc, k: instance.k, A: instance.A, type: typeof globalThis.window, keys, recent: trace.slice(-12) });
                previousWindow = globalThis.window;
              }
              const previous = previousRegisters.get(instance);
              const poolValue = (instance.g || [])[instance.N];
              state.Gn = poolValue === null ? "null" : poolValue === undefined ? "undefined" : `${typeof poolValue}:${String(poolValue).slice(0, 100)}`;
              const bindingValue = (instance.B || [])[instance.N];
              state.Bnv = bindingValue === null ? "null" : bindingValue === undefined ? "undefined" : `${typeof bindingValue}:${String(bindingValue).slice(0, 100)}`;
              if (!previous || registerValues.some((entry, index) => entry !== previous[index])) {
                changes.push(state);
                if (changes.length > 300) changes.shift();
                if (instanceIds.get(instance) === 1) {
                  if (instance.A >= 380 && instance.A <= 700) {
                    process.stderr.write(`bvm_probe_step=${JSON.stringify(state)}\n`);
                  }
                  registerValues.forEach((entry, index) => {
                    if (typeof entry === "string" && (!previous || entry !== previous[index]) && entry.length <= 240) {
                      process.stderr.write(`bvm_string_change=${JSON.stringify({ pc, k: instance.k, A: instance.A, index, value: entry })}\n`);
                    }
                  });
                }
                previousRegisters.set(instance, registerValues);
              }
            };
            globalThis.__zseDebugLog = (state) => {
              process.stderr.write(`bvm_failure=${JSON.stringify(state)}\n`);
              process.stderr.write(`bvm_trace=${JSON.stringify(trace)}\n`);
              process.stderr.write(`bvm_changes=${JSON.stringify(changes)}\n`);
              process.stderr.write(`bvm_focus=${JSON.stringify(changes.filter((entry) => entry.id === 1 && entry.A >= 155 && entry.A <= 230))}\n`);
              process.stderr.write(`bvm_properties=${JSON.stringify(propertyEvents)}\n`);
              process.stderr.write(`bvm_window_transitions=${JSON.stringify(windowTransitions)}\n`);
            };
            globalThis.__zseDebugSummary = () => ({
              trace: trace.slice(-40),
              changes: changes.slice(-80),
              propertyEvents,
              windowTransitions,
            });
            globalThis.q = (object, property) => {
              let label = typeof object;
              if (object === globalThis) label = "window";
              else if (object === globalThis.window) label = "window-property";
              else if (object === globalThis.self) label = "self-property";
              else if (object === documentValue) label = "document";
              else if (object === navigatorValue) label = "navigator";
              else if (object === screenValue) label = "screen";
              else if (object === locationValue) label = "location";
              else if (object !== null && object !== undefined) {
                try { label = Object.prototype.toString.call(object); } catch {}
              }
              const event = { label, property: String(property), k: globalThis.__b && globalThis.__b.k, A: globalThis.__b && globalThis.__b.A };
              try {
                const result = Reflect.get(Object(object), property);
                event.result = typeof result;
                propertyEvents.push(event);
                fingerprintProperties.push(event);
                process.stderr.write(`bvm_property=${JSON.stringify(event)}\n`);
                return result;
              } catch (error) {
                event.result = `throws:${error}`;
                propertyEvents.push(event);
                throw error;
              }
            };
            globalThis.q_set = (object, property, propertyValue) => {
              let label = object === globalThis ? "window" : typeof object;
              if (object !== null && object !== undefined && object !== globalThis) {
                try { label = Object.prototype.toString.call(object); } catch {}
              }
              propertyEvents.push({ label, property: String(property), set: typeof propertyValue, k: globalThis.__b && globalThis.__b.k, A: globalThis.__b && globalThis.__b.A });
              process.stderr.write(`bvm_property_set=${JSON.stringify({ label, property: String(property), set: typeof propertyValue, k: globalThis.__b && globalThis.__b.k, A: globalThis.__b && globalThis.__b.A })}\n`);
              return Reflect.set(Object(object), property, propertyValue);
            };
            const oldSetter = "this['F'][this['T']][_0x1b9b89]=_0x404520";
            const newSetter = "q_set(this.F[this.T],_0x1b9b89,_0x404520)";
            const oldGetter = "this['F'][this['T']][_0x1b9b89]";
            const newGetter = "q(this.F[this.T],_0x1b9b89)";
            callArgs[0] = callArgs[0].split(oldSetter).join(newSetter).split(oldGetter).join(newGetter);
            callArgs[0] = callArgs[0].replace(
              /(while\(!!\[\]\)\{this\['k'\]=this\['S'\]\[_0x57e9c4\+\+\]\|\|-\([^;]+;)/,
              "$1globalThis.__zseTraceStep(this,_0x57e9c4);",
            );
            callArgs[0] = callArgs[0].replace(
              "catch(_0x19a6d4){return;}",
              "catch(_0x19a6d4){globalThis.__zseDebugLog({message:String(_0x19a6d4),k:this.k,Z:this.Z,P:this.P,T:this.T,O:this.O,FZType:typeof this.F[this.Z],F:Object.prototype.toString.call(this.F),key:typeof _0x2179d2==='undefined'?null:_0x2179d2});throw _0x19a6d4;}",
            );
          }
          const evalResult = Reflect.apply(nativeEval, globalThis, callArgs);
          if (process.env.ZSE_DEBUG === "1") {
            const bvm = globalThis.__b;
            let bvmValues = {};
            if (bvm) {
              for (const key of Reflect.ownKeys(bvm)) {
                const item = bvm[key];
                bvmValues[String(key)] = typeof item === "string"
                  ? `${typeof item}:${item.slice(0, 120)}`
                  : `${typeof item}:${String(item).slice(0, 120)}`;
              }
            }
            process.stderr.write(`bvm_keys=${bvm ? Reflect.ownKeys(bvm).map(String).join(",") : ""}\n`);
            process.stderr.write(`bvm_zh=${bvm ? typeof bvm.zh : "missing"}:${bvm ? String(bvm.zh).slice(0, 200) : ""}\n`);
            if (bvm && hostProcess.env.ZSE_FINGERPRINT_OUTPUT) {
              fs.writeFileSync(hostProcess.env.ZSE_FINGERPRINT_OUTPUT, String(bvm.zh || ""), "utf8");
            }
            process.stderr.write(`bvm_end=${JSON.stringify(bvmValues)}\n`);
            if (globalThis.__zseDebugSummary) {
              const summary = globalThis.__zseDebugSummary();
              for (const state of summary.trace.slice(-20)) {
                process.stderr.write(`bvm_tail=${JSON.stringify(state)}\n`);
              }
              process.stderr.write(`bvm_summary=${JSON.stringify(summary)}\n`);
            }
          }
          globalThis.window = globalThis;
          globalThis.self = globalThis;
          globalThis.BigInt = nativeBigInt;
          value[4](Number(bridgeArgs[0]), evalResult);
          new DataView(value[3]).setUint8(Number(bridgeArgs[0]) + 8, 1);
          return undefined;
        }
        return bridgeCopy(value);
      };
      let result;
      if (process.env.ZSE_DEBUG === "1" && callCount === 22 && globalThis._g && globalThis._g.i) {
        const keys = Reflect.ownKeys(globalThis._g.i);
        process.stderr.write(`  refs_keys=${keys.slice(0, 80).map(String).join(",")}\n`);
        for (const key of keys.slice(0, 80)) {
          const value = globalThis._g.i[key];
          process.stderr.write(`  refs[${String(key)}]=${typeof value}:${String(value).slice(0, 120)}\n`);
        }
      }
      try { result = originalCopy(...wasmArgs); }
      finally { globalThis.__g.copyBytesToGo = bridgeCopy; }
      if (process.env.ZSE_DEBUG === "1" && payload && callCount >= 1) {
        const [args, loadValue, loadString, , , , , , loadSlice] = payload;
        const values = Array.from(args);
        if (callCount === 20) {
          process.stderr.write(`  args_type=${args && args.constructor ? args.constructor.name : typeof args}\n`);
          process.stderr.write(`  loadValue=${String(loadValue).slice(0, 500)}\n`);
        }
        process.stderr.write(`bridge[${callCount}]_before=${payloadBefore.map(String).join(",")}\n`);
        process.stderr.write(`bridge[${callCount}]_after=${values.map(String).join(",")}\n`);
        for (const start of [1, 4, 7, 10]) {
          const [pointer, length, capacity] = values.slice(start, start + 3).map(Number);
          if (!Number.isSafeInteger(pointer) || !Number.isSafeInteger(length) || length < 1 || length > 200000) continue;
          try {
            loadSlice(pointer, length, capacity);
            process.stderr.write(`  string[${start}]=${JSON.stringify(loadString(pointer, Math.min(length, 200)))}\n`);
          } catch {}
        }
        for (const index of [2, 3]) {
          try {
            const loaded = loadValue(values[index]);
            process.stderr.write(`  ref[${index}]=${typeof loaded}:${String(loaded).slice(0, 160)} raw_type=${typeof values[index]}\n`);
          } catch {}
        }
        process.stderr.write(`  result=${String(result)}\n`);
      }
      return result;
    };
    imports.gojs["syscall/js.copyBytesToGo"] = imports.env["syscall/js.copyBytesToGo"];
    const instantiated = await nativeInstantiate(bytes, imports);
    globalThis.__zseWasmInstance = instantiated.instance || instantiated;
    return instantiated;
  };
}

globalThis.__pcwebReadResult = function __pcwebReadResult() {
  const match = writtenCookie.match(/__zse_ck=([^;]+)/);
  if (match) {
    const cookie = match[1];
    const suffix = `-${meta}`;
    const ck = cookie.endsWith(suffix) ? cookie.slice(0, -suffix.length) : "";
    return { ck, cookie };
  }
  return null;
};
})();
