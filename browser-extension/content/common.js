(function () {
  if (window.__wx_browser_extension__) {
    return;
  }

  var LOG_PREFIX = "[WX-BROWSER-EXT]";
  var REPORT_ENDPOINT = "/__wx_channels_api/platform/browser";
  var sites = [];
  var defaultActionDelay = 80;

  function debugEnabled() {
    try {
      return localStorage.getItem("__wx_browser_extension_debug__") === "1";
    } catch (e) {
      return false;
    }
  }

  function log() {
    if (!debugEnabled()) {
      return;
    }
    try {
      console.log.apply(console, [LOG_PREFIX].concat(Array.prototype.slice.call(arguments)));
    } catch (e) {}
  }

  function createLogger(scope) {
    return function () {
      if (!debugEnabled()) {
        return;
      }
      try {
        console.log.apply(
          console,
          [LOG_PREFIX + "[" + scope + "]"].concat(Array.prototype.slice.call(arguments)),
        );
      } catch (e) {}
    };
  }

  function text(value) {
    return value == null ? "" : String(value).trim();
  }

  function first() {
    for (var i = 0; i < arguments.length; i += 1) {
      var value = text(arguments[i]);
      if (value) {
        return value;
      }
    }
    return "";
  }

  function absoluteURL(value, base) {
    var url = text(value);
    if (!url) {
      return "";
    }
    try {
      var parsed = new URL(url, base || location.href);
      parsed.hash = "";
      return parsed.href;
    } catch (e) {
      return url;
    }
  }

  function parseJSON(value) {
    var raw = text(value);
    if (!raw) {
      return {};
    }
    try {
      return JSON.parse(raw);
    } catch (e) {
      return {};
    }
  }

  function attr(el, name) {
    return el ? text(el.getAttribute(name)) : "";
  }

  function sleep(ms) {
    return new Promise(function (resolve) {
      setTimeout(resolve, Math.max(0, Number(ms) || 0));
    });
  }

  function nextFrame() {
    return new Promise(function (resolve) {
      requestAnimationFrame(function () {
        resolve();
      });
    });
  }

  function waitUntil(check, options) {
    var opt = options || {};
    var timeout = Number(opt.timeout) || 5000;
    var interval = Number(opt.interval) || 100;
    var started = Date.now();
    return new Promise(function (resolve, reject) {
      function tick() {
        var result = false;
        try {
          result = check();
        } catch (err) {
          reject(err);
          return;
        }
        if (result) {
          resolve(result);
          return;
        }
        if (Date.now() - started >= timeout) {
          reject(new Error(opt.message || "waitUntil timeout"));
          return;
        }
        setTimeout(tick, interval);
      }
      tick();
    });
  }

  function query(selector, root) {
    if (!selector) {
      return null;
    }
    var scope = root && root.querySelector ? root : document;
    return scope.querySelector(selector);
  }

  function queryAll(selector, root) {
    if (!selector) {
      return [];
    }
    var scope = root && root.querySelectorAll ? root : document;
    return Array.prototype.slice.call(scope.querySelectorAll(selector));
  }

  function metaContent(root, selector) {
    var el = root && root.querySelector ? root.querySelector(selector) : null;
    return attr(el, "content");
  }

  function metaContents(root, selector) {
    if (!root || !root.querySelectorAll) {
      return [];
    }
    return Array.prototype.slice.call(root.querySelectorAll(selector)).map(function (el) {
      return attr(el, "content");
    });
  }

  function isVisible(el) {
    if (!el || el.nodeType !== 1) {
      return false;
    }
    var style = window.getComputedStyle(el);
    if (!style || style.display === "none" || style.visibility === "hidden" || Number(style.opacity) === 0) {
      return false;
    }
    var rect = el.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
  }

  function elementText(el) {
    return text(el && (el.innerText || el.textContent));
  }

  function findByText(selector, pattern, root) {
    var matcher =
      pattern instanceof RegExp
        ? function (value) {
            return pattern.test(value);
          }
        : function (value) {
            return value.indexOf(text(pattern)) >= 0;
          };
    return (
      queryAll(selector, root).find(function (el) {
        return matcher(elementText(el));
      }) || null
    );
  }

  function dispatchEvent(el, type, options) {
    if (!el) {
      return false;
    }
    var eventOptions = Object.assign({ bubbles: true, cancelable: true, composed: true }, options || {});
    el.dispatchEvent(new Event(type, eventOptions));
    return true;
  }

  function dispatchMouseEvent(el, type, options) {
    if (!el) {
      return false;
    }
    var eventOptions = Object.assign(
      { bubbles: true, cancelable: true, composed: true, view: window },
      options || {},
    );
    el.dispatchEvent(new MouseEvent(type, eventOptions));
    return true;
  }

  function ensureElement(target, root) {
    if (!target) {
      return null;
    }
    if (typeof target === "string") {
      return query(target, root);
    }
    return target.nodeType === 1 ? target : null;
  }

  function scrollIntoView(target, options) {
    var el = ensureElement(target, options && options.root);
    if (!el || !el.scrollIntoView) {
      return Promise.resolve(null);
    }
    el.scrollIntoView(
      Object.assign(
        {
          block: "center",
          inline: "nearest",
          behavior: "auto",
        },
        (options && options.scrollIntoView) || {},
      ),
    );
    return sleep((options && options.delay) || defaultActionDelay).then(function () {
      return el;
    });
  }

  function click(target, options) {
    var opt = options || {};
    var el = ensureElement(target, opt.root);
    if (!el) {
      return Promise.reject(new Error("click target not found"));
    }
    return scrollIntoView(el, opt).then(function () {
      if (opt.mouseEvents) {
        dispatchMouseEvent(el, "mouseover", opt.event);
        dispatchMouseEvent(el, "mousedown", opt.event);
        dispatchMouseEvent(el, "mouseup", opt.event);
      }
      el.click();
      return sleep(opt.delay || defaultActionDelay).then(function () {
        return el;
      });
    });
  }

  function clickByText(selector, pattern, options) {
    var opt = options || {};
    var el = findByText(selector, pattern, opt.root);
    if (!el) {
      return Promise.reject(new Error("clickByText target not found"));
    }
    return click(el, opt);
  }

  function setNativeValue(el, value) {
    var proto = Object.getPrototypeOf(el);
    var descriptor = Object.getOwnPropertyDescriptor(proto, "value");
    if (descriptor && descriptor.set) {
      descriptor.set.call(el, value);
      return;
    }
    el.value = value;
  }

  function fill(target, value, options) {
    var opt = options || {};
    var el = ensureElement(target, opt.root);
    if (!el) {
      return Promise.reject(new Error("fill target not found"));
    }
    return scrollIntoView(el, opt).then(function () {
      if (typeof el.focus === "function") {
        el.focus();
      }
      if (el.isContentEditable) {
        el.textContent = value == null ? "" : String(value);
      } else if ("value" in el) {
        setNativeValue(el, value == null ? "" : String(value));
      } else {
        el.textContent = value == null ? "" : String(value);
      }
      dispatchEvent(el, "input");
      dispatchEvent(el, "change");
      return sleep(opt.delay || defaultActionDelay).then(function () {
        return el;
      });
    });
  }

  function selectValue(target, value, options) {
    var opt = options || {};
    var el = ensureElement(target, opt.root);
    if (!el) {
      return Promise.reject(new Error("select target not found"));
    }
    return scrollIntoView(el, opt).then(function () {
      el.value = value == null ? "" : String(value);
      dispatchEvent(el, "input");
      dispatchEvent(el, "change");
      return sleep(opt.delay || defaultActionDelay).then(function () {
        return el;
      });
    });
  }

  function submitForm(target, options) {
    var opt = options || {};
    var el = ensureElement(target, opt.root);
    var form = el && (el.matches("form") ? el : el.closest("form"));
    if (!form) {
      return Promise.reject(new Error("form not found"));
    }
    if (typeof form.requestSubmit === "function") {
      form.requestSubmit();
    } else {
      dispatchEvent(form, "submit");
      form.submit();
    }
    return sleep(opt.delay || defaultActionDelay).then(function () {
      return form;
    });
  }

  function scrollContainer(target, options) {
    var opt = options || {};
    var el = target ? ensureElement(target, opt.root) : document.scrollingElement || document.documentElement;
    if (!el) {
      return Promise.reject(new Error("scroll container not found"));
    }
    var step = Number(opt.step) || Math.max(200, Math.floor(window.innerHeight * 0.8));
    var delay = Number(opt.delay) || 250;
    var maxTimes = Number(opt.maxTimes) || 20;
    var until = typeof opt.until === "function" ? opt.until : null;
    var lastTop = -1;
    var count = 0;

    return new Promise(function (resolve) {
      function currentTop() {
        return el === window || el === document || el === document.body ? window.scrollY : el.scrollTop;
      }
      function currentMax() {
        if (el === window || el === document || el === document.body) {
          return Math.max(document.documentElement.scrollHeight, document.body.scrollHeight) - window.innerHeight;
        }
        return el.scrollHeight - el.clientHeight;
      }
      function doScroll() {
        if (until && until(el)) {
          resolve({ stoppedBy: "until", count: count, scrollTop: currentTop() });
          return;
        }
        if (count >= maxTimes) {
          resolve({ stoppedBy: "maxTimes", count: count, scrollTop: currentTop() });
          return;
        }
        var top = currentTop();
        if (top === lastTop && top >= currentMax()) {
          resolve({ stoppedBy: "end", count: count, scrollTop: top });
          return;
        }
        lastTop = top;
        count += 1;
        if (el === window || el === document || el === document.body) {
          window.scrollBy({ top: step, left: 0, behavior: opt.behavior || "auto" });
        } else {
          el.scrollBy({ top: step, left: 0, behavior: opt.behavior || "auto" });
        }
        setTimeout(doScroll, delay);
      }
      doScroll();
    });
  }

  function collectText(selector, options) {
    var opt = options || {};
    return queryAll(selector, opt.root)
      .filter(function (el) {
        return opt.visibleOnly ? isVisible(el) : true;
      })
      .map(elementText)
      .filter(Boolean);
  }

  function collectLinks(selector, options) {
    var opt = options || {};
    return queryAll(selector || "a[href]", opt.root)
      .filter(function (el) {
        return opt.visibleOnly ? isVisible(el) : true;
      })
      .map(function (el) {
        return {
          text: elementText(el),
          href: absoluteURL(el.href || attr(el, "href")),
          title: attr(el, "title"),
        };
      })
      .filter(function (item) {
        return !!item.href;
      });
  }

  function collectImages(selector, options) {
    var opt = options || {};
    return queryAll(selector || "img", opt.root)
      .filter(function (el) {
        return opt.visibleOnly ? isVisible(el) : true;
      })
      .map(function (el) {
        return {
          src: absoluteURL(el.currentSrc || el.src || attr(el, "data-src") || attr(el, "data-original")),
          alt: attr(el, "alt"),
          width: el.naturalWidth || el.width || 0,
          height: el.naturalHeight || el.height || 0,
        };
      })
      .filter(function (item) {
        return !!item.src;
      });
  }

  function collectMeta(root) {
    var scope = root && root.querySelectorAll ? root : document;
    var result = {};
    queryAll("meta", scope).forEach(function (el) {
      var key = attr(el, "property") || attr(el, "name") || attr(el, "itemprop");
      var value = attr(el, "content");
      if (key && value && !result[key]) {
        result[key] = value;
      }
    });
    return result;
  }

  function collectJSONLD(root) {
    return queryAll('script[type="application/ld+json"]', root)
      .map(function (el) {
        return parseJSON(el.textContent);
      })
      .filter(function (item) {
        return !!item && Object.keys(item).length > 0;
      });
  }

  function collectTable(table) {
    var el = ensureElement(table);
    if (!el) {
      return [];
    }
    var headers = queryAll("thead th, tr:first-child th, tr:first-child td", el).map(elementText);
    var headerRow = headers.length ? query("thead tr, tr:first-child", el) : null;
    var rows = queryAll("tbody tr, tr", el);
    rows = rows.filter(function (row) {
      return row !== headerRow;
    });
    return rows
      .map(function (row) {
        var cells = queryAll("td, th", row).map(elementText);
        if (!cells.length) {
          return null;
        }
        if (!headers.length) {
          return cells;
        }
        return headers.reduce(function (acc, key, index) {
          acc[key || String(index)] = cells[index] || "";
          return acc;
        }, {});
      })
      .filter(Boolean);
  }

  function extractField(root, spec) {
    if (typeof spec === "function") {
      return spec(root, api);
    }
    if (typeof spec === "string") {
      return elementText(query(spec, root));
    }
    if (!spec || typeof spec !== "object") {
      return "";
    }
    var el = spec.selector ? query(spec.selector, root) : root;
    if (!el) {
      return spec.defaultValue || "";
    }
    if (spec.all) {
      return queryAll(spec.selector, root).map(function (item) {
        return extractField(item, Object.assign({}, spec, { selector: null, all: false }));
      });
    }
    if (spec.attr) {
      return spec.url ? absoluteURL(attr(el, spec.attr)) : attr(el, spec.attr);
    }
    if (spec.html) {
      return el.innerHTML || "";
    }
    if (spec.value) {
      return "value" in el ? text(el.value) : "";
    }
    return elementText(el);
  }

  function scrapeElements(selector, schema, options) {
    var opt = options || {};
    return queryAll(selector, opt.root)
      .filter(function (el) {
        return opt.visibleOnly ? isVisible(el) : true;
      })
      .map(function (el) {
        var item = {};
        Object.keys(schema || {}).forEach(function (key) {
          item[key] = extractField(el, schema[key]);
        });
        return item;
      });
  }

  function getRuntime() {
    if (typeof chrome !== "undefined" && chrome.runtime && chrome.runtime.id) {
      return chrome.runtime;
    }
    if (typeof browser !== "undefined" && browser.runtime && browser.runtime.id) {
      return browser.runtime;
    }
    return null;
  }

  function sendRuntimeMessage(message) {
    var runtime = getRuntime();
    if (!runtime || !runtime.sendMessage) {
      return Promise.reject(new Error("runtime messaging is unavailable"));
    }
    return new Promise(function (resolve, reject) {
      try {
        var result = runtime.sendMessage(message, function (response) {
          var lastError = runtime.lastError;
          if (lastError) {
            reject(new Error(lastError.message));
            return;
          }
          resolve(response);
        });
        if (result && typeof result.then === "function") {
          result.then(resolve).catch(reject);
        }
      } catch (err) {
        reject(err);
      }
    });
  }

  function reportProfile(payload, options) {
    var opt = options || {};
    var endpoint = absoluteURL(opt.endpoint || REPORT_ENDPOINT);
    var body = Object.assign(
      {
        page_url: location.href,
        page_title: document.title,
      },
      payload || {},
    );
    if (opt.siteId && !body.platform_id) {
      body.platform_id = opt.siteId;
    }

    return fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(body),
    })
      .then(function (resp) {
        if (!resp.ok) {
          throw new Error("report status " + resp.status);
        }
        return { ok: resp.ok, status: resp.status, via: "content" };
      })
      .catch(function () {
        return sendRuntimeMessage({
          type: "WX_BROWSER_EXTENSION_REPORT",
          endpoint: endpoint,
          payload: body,
        }).then(function (response) {
          return Object.assign({ via: "background" }, response || {});
        });
      });
  }

  function normalizeCookieOptions(options) {
    var opt = Object.assign({}, options || {});
    if (typeof opt.domain === "string" && !opt.domains) {
      opt.domains = [opt.domain];
    }
    if (Array.isArray(opt.domains)) {
      opt.domains = opt.domains
        .map(function (domain) {
          return text(domain).replace(/^\./, "").toLowerCase();
        })
        .filter(Boolean);
    }
    if (Array.isArray(opt.blacklist)) {
      opt.blacklist = opt.blacklist
        .map(function (domain) {
          return text(domain).replace(/^\./, "").toLowerCase();
        })
        .filter(Boolean);
    }
    return opt;
  }

  function getCookies(options) {
    return sendRuntimeMessage({
      type: "WX_BROWSER_EXTENSION_GET_COOKIES",
      options: normalizeCookieOptions(options),
    }).then(function (response) {
      if (!response || !response.ok) {
        throw new Error((response && response.error) || "get cookies failed");
      }
      return response;
    });
  }

  function getCookieHeader(options) {
    return getCookies(options).then(function (response) {
      return response.header || "";
    });
  }

  function getCookiesByDomain(options) {
    return getCookies(options).then(function (response) {
      return response.by_domain || {};
    });
  }

  function nodeMatches(node, selector) {
    return !!(node && node.nodeType === 1 && node.matches && node.matches(selector));
  }

  function findInNode(node, selector) {
    if (nodeMatches(node, selector)) {
      return node;
    }
    if (node && node.nodeType === 1 && node.querySelector) {
      return node.querySelector(selector);
    }
    return null;
  }

  function observerRoot(root) {
    if (root && root.nodeType) {
      return root;
    }
    return document.documentElement || document.body;
  }

  function observeNode(selector, cb, options) {
    var opt = options || {};
    var root = observerRoot(opt.root);
    var existing = (root && root.querySelector && root.querySelector(selector)) || document.querySelector(selector);
    var done = false;
    var timer = null;
    var observer = null;

    function finish(el) {
      if (done) {
        return;
      }
      done = true;
      if (timer) {
        clearTimeout(timer);
      }
      if (observer) {
        observer.disconnect();
      }
      cb(el);
    }

    if (existing) {
      finish(existing);
      return function () {};
    }

    observer = new MutationObserver(function (mutations) {
      for (var i = 0; i < mutations.length; i += 1) {
        var nodes = mutations[i].addedNodes || [];
        for (var j = 0; j < nodes.length; j += 1) {
          var found = findInNode(nodes[j], selector);
          if (found) {
            finish(found);
            return;
          }
        }
      }
    });

    if (root) {
      observer.observe(root, { childList: true, subtree: true });
    }
    if (opt.timeout && opt.timeout > 0) {
      timer = setTimeout(function () {
        if (done) {
          return;
        }
        done = true;
        if (observer) {
          observer.disconnect();
        }
        if (typeof opt.error === "function") {
          opt.error();
        }
      }, opt.timeout);
    }

    return function () {
      done = true;
      if (timer) {
        clearTimeout(timer);
      }
      if (observer) {
        observer.disconnect();
      }
    };
  }

  function observeElements(selector, cb, options) {
    var opt = options || {};
    var root = observerRoot(opt.root);
    var seen = opt.seen || new WeakSet();
    var observer = null;

    function emit(el) {
      if (!el || seen.has(el)) {
        return;
      }
      seen.add(el);
      cb(el);
    }

    function scan(scope) {
      if (!scope) {
        return;
      }
      if (nodeMatches(scope, selector)) {
        emit(scope);
      }
      var queryRoot = scope.querySelectorAll ? scope : document;
      Array.prototype.slice.call(queryRoot.querySelectorAll(selector)).forEach(emit);
    }

    scan(root || document);
    observer = new MutationObserver(function (mutations) {
      mutations.forEach(function (mutation) {
        Array.prototype.slice.call(mutation.addedNodes || []).forEach(function (node) {
          if (node && node.nodeType === 1) {
            scan(node);
          }
        });
      });
    });
    if (root) {
      observer.observe(root, { childList: true, subtree: true });
    }
    return function () {
      if (observer) {
        observer.disconnect();
      }
    };
  }

  function onReady(cb) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", cb, { once: true });
      return;
    }
    cb();
  }

  function hostnameMatches(hostname, domains) {
    var host = text(hostname).toLowerCase();
    return (domains || []).some(function (domain) {
      var normalized = text(domain).toLowerCase();
      return host === normalized || host.endsWith("." + normalized);
    });
  }

  function registerSite(site) {
    if (!site || !site.id || typeof site.run !== "function") {
      return;
    }
    sites.push(site);
  }

  function runSites() {
    onReady(function () {
      sites.forEach(function (site) {
        var matched = false;
        try {
          matched = typeof site.matches === "function" ? site.matches(location) : false;
        } catch (err) {
          console.warn(LOG_PREFIX, "site matcher failed", site.id, err);
        }
        if (!matched) {
          return;
        }
        var key = "__wx_browser_extension_site_" + site.id + "__";
        if (window[key]) {
          return;
        }
        window[key] = true;
        try {
          site.run(api);
          log("site activated", site.id, location.href);
        } catch (err) {
          console.warn(LOG_PREFIX, "site run failed", site.id, err);
        }
      });
    });
  }

  var api = {
    absoluteURL: absoluteURL,
    attr: attr,
    click: click,
    clickByText: clickByText,
    collectImages: collectImages,
    collectJSONLD: collectJSONLD,
    collectLinks: collectLinks,
    collectMeta: collectMeta,
    collectTable: collectTable,
    collectText: collectText,
    createLogger: createLogger,
    dispatchEvent: dispatchEvent,
    dispatchMouseEvent: dispatchMouseEvent,
    elementText: elementText,
    fill: fill,
    findByText: findByText,
    first: first,
    hostnameMatches: hostnameMatches,
    getCookieHeader: getCookieHeader,
    getCookies: getCookies,
    getCookiesByDomain: getCookiesByDomain,
    isVisible: isVisible,
    log: log,
    metaContent: metaContent,
    metaContents: metaContents,
    nextFrame: nextFrame,
    observeElements: observeElements,
    observeNode: observeNode,
    onReady: onReady,
    parseJSON: parseJSON,
    query: query,
    queryAll: queryAll,
    registerSite: registerSite,
    reportProfile: reportProfile,
    runSites: runSites,
    scrapeElements: scrapeElements,
    scrollContainer: scrollContainer,
    scrollIntoView: scrollIntoView,
    selectValue: selectValue,
    sleep: sleep,
    submitForm: submitForm,
    text: text,
    waitUntil: waitUntil,
  };

  window.__wx_browser_extension__ = api;
})();
