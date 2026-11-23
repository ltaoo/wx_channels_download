// Minimal ECMAScript host for the official AWSC UAB module, not a page runner.
// Unsupported Canvas capabilities return null; the SDK selects its own fallback.
var window = globalThis, self = globalThis, top = globalThis, parent = globalThis;
var location = {
  href: __youku_input.page, protocol: "https:", host: "v.youku.com",
  hostname: "v.youku.com", pathname: __youku_input.page.slice("https://v.youku.com".length),
  search: "", hash: ""
};
var navigator = {
  userAgent: __youku_input.user_agent, appName: "Netscape", appVersion: "5.0",
  platform: "MacIntel", language: "zh-CN", languages: ["zh-CN", "zh"], plugins: [],
  mimeTypes: [], cookieEnabled: true, onLine: true, hardwareConcurrency: 8, maxTouchPoints: 0
};
var screen = { width: 1440, height: 900, availWidth: 1440, availHeight: 875, colorDepth: 24, pixelDepth: 24 };
var innerWidth = 1440, innerHeight = 800, outerWidth = 1440, outerHeight = 900, devicePixelRatio = 2;
var performance = { now: function() { return 0; }, timing: { navigationStart: Date.now() }, getEntriesByType: function() { return []; } };
var console = { log: function() {}, warn: function() {}, error: function() {} };
function setTimeout() { return 1; }
function clearTimeout() {}
function setInterval() { return 1; }
function clearInterval() {}
function addEventListener() {}
function removeEventListener() {}
function attachEvent() {}
function detachEvent() {}
function Image() { this.style = {}; }
function storage() {
  var data = Object.create(null);
  return {
    getItem: function(key) { return Object.prototype.hasOwnProperty.call(data, key) ? data[key] : null; },
    setItem: function(key, value) { data[key] = String(value); },
    removeItem: function(key) { delete data[key]; }
  };
}
var localStorage = storage(), sessionStorage = storage();
function element(name) {
  return {
    tagName: name.toUpperCase(), nodeName: name.toUpperCase(), style: {}, children: [], childNodes: [],
    setAttribute: function(key, value) { this[key] = String(value); },
    getAttribute: function(key) { return this[key] === undefined ? null : this[key]; },
    hasAttribute: function(key) { return this[key] !== undefined; },
    appendChild: function(child) { this.children.push(child); child.parentNode = this; return child; },
    removeChild: function(child) { return child; },
    insertBefore: function(child) { child.parentNode = this; return child; },
    addEventListener: addEventListener, removeEventListener: removeEventListener,
    attachEvent: attachEvent, detachEvent: detachEvent,
    getElementsByTagName: function() { return []; },
    getBoundingClientRect: function() { return { width: 0, height: 0, top: 0, left: 0 }; },
    getContext: function() { return null; }
  };
}
var document = {
  cookie: __youku_input.cookie, referrer: "https://www.youku.com/", URL: location.href,
  location: location, domain: "youku.com", readyState: "complete", defaultView: window,
  documentElement: element("html"), head: element("head"), body: element("body"),
  createElement: element, createEvent: function() { return { initEvent: function() {} }; },
  getElementById: function() { return null; }, querySelector: function() { return null; },
  getElementsByTagName: function(name) { return name === "head" ? [this.head] : name === "body" ? [this.body] : []; },
  addEventListener: addEventListener, removeEventListener: removeEventListener,
  attachEvent: attachEvent, detachEvent: detachEvent
};
function Document() {}
function HTMLDocument() {}
function Element() {}
function HTMLElement() {}
function HTMLCanvasElement() {}
function Navigator() {}
Document.prototype = document;
HTMLDocument.prototype = document;
document.constructor = HTMLDocument;
Element.prototype = {};
HTMLElement.prototype = Element.prototype;
HTMLCanvasElement.prototype = Element.prototype;
Navigator.prototype = navigator;
var __youku_uab;
var AWSC = {}, AWSCInner = {
  register: function(name, version, module) { if (name === "uabModule") __youku_uab = module; }
};
