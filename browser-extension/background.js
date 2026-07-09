(function () {
  var runtime = typeof chrome !== "undefined" && chrome.runtime ? chrome.runtime : null;
  if (!runtime || !runtime.onMessage) {
    return;
  }

  var REPORT_MESSAGE = "WX_BROWSER_EXTENSION_REPORT";
  var GET_COOKIES_MESSAGE = "WX_BROWSER_EXTENSION_GET_COOKIES";

  function text(value) {
    return value == null ? "" : String(value).trim();
  }

  function unique(values) {
    var seen = {};
    var out = [];
    (values || []).forEach(function (value) {
      var item = text(value).toLowerCase();
      if (!item || seen[item]) {
        return;
      }
      seen[item] = true;
      out.push(item);
    });
    return out;
  }

  function cookieDomain(cookie) {
    return text(cookie && cookie.domain).replace(/^\./, "").toLowerCase();
  }

  function domainMatches(host, domain) {
    var normalizedHost = text(host).replace(/^\./, "").toLowerCase();
    var normalizedDomain = text(domain).replace(/^\./, "").toLowerCase();
    return normalizedHost === normalizedDomain || normalizedHost.endsWith("." + normalizedDomain);
  }

  function senderURL(sender) {
    return text(sender && sender.tab && sender.tab.url);
  }

  function senderHostname(sender) {
    var url = senderURL(sender);
    if (!url) {
      return "";
    }
    try {
      return new URL(url).hostname;
    } catch (e) {
      return "";
    }
  }

  function urlHostname(url) {
    url = text(url);
    if (!url) {
      return "";
    }
    try {
      return new URL(url).hostname;
    } catch (e) {
      return "";
    }
  }

  function normalizeCookie(cookie) {
    var out = {};
    [
      "name",
      "value",
      "domain",
      "hostOnly",
      "path",
      "secure",
      "httpOnly",
      "sameSite",
      "session",
      "expirationDate",
      "storeId",
      "partitionKey",
    ].forEach(function (key) {
      if (cookie && cookie[key] !== undefined) {
        out[key] = cookie[key];
      }
    });
    return out;
  }

  function cookiesAPI() {
    if (typeof chrome !== "undefined" && chrome.cookies) {
      return chrome.cookies;
    }
    if (typeof browser !== "undefined" && browser.cookies) {
      return browser.cookies;
    }
    return null;
  }

  function getAllCookies(details) {
    var api = cookiesAPI();
    if (!api || !api.getAll) {
      return Promise.reject(new Error("cookies api is unavailable"));
    }
    return new Promise(function (resolve, reject) {
      try {
        var result = api.getAll(details || {}, function (cookies) {
          var lastError = runtime.lastError || (typeof chrome !== "undefined" && chrome.runtime && chrome.runtime.lastError);
          if (lastError) {
            reject(new Error(lastError.message));
            return;
          }
          resolve(cookies || []);
        });
        if (result && typeof result.then === "function") {
          result.then(resolve).catch(reject);
        }
      } catch (err) {
        reject(err);
      }
    });
  }

  function getAllCookiesWithPartitionFallback(details) {
    return getAllCookies(details).catch(function (err) {
      if (details && details.partitionKey !== undefined) {
        var fallback = Object.assign({}, details);
        delete fallback.partitionKey;
        return getAllCookies(fallback);
      }
      throw err;
    });
  }

  function cookieHeader(cookies) {
    return (cookies || [])
      .slice()
      .sort(function (a, b) {
        return text(b.path).length - text(a.path).length || text(a.name).localeCompare(text(b.name));
      })
      .map(function (cookie) {
        return cookie.name + "=" + cookie.value;
      })
      .join("; ");
  }

  function cookiesByDomain(cookies) {
    return (cookies || []).reduce(function (acc, cookie) {
      var domain = cookie.domain || "";
      if (!acc[domain]) {
        acc[domain] = [];
      }
      acc[domain].push(cookie);
      return acc;
    }, {});
  }

  function cookieMatches(cookie, options, sender) {
    var domain = cookieDomain(cookie);
    var domains = unique(options.domains || (options.domain ? [options.domain] : []));
    var blacklist = unique(options.blacklist || []);
    var host = urlHostname(options.url) || senderHostname(sender);

    if (!domains.length && !options.all && host) {
      domains = [host];
    }
    if (domains.length) {
      var matched = domains.some(function (item) {
        return domainMatches(domain, item) || domainMatches(item, domain);
      });
      if (!matched) {
        return false;
      }
    }
    if (blacklist.some(function (item) {
      return domainMatches(domain, item) || domain.indexOf(item) >= 0;
    })) {
      return false;
    }
    if (options.name && cookie.name !== options.name) {
      return false;
    }
    return true;
  }

  function cookieQueryDetails(options, sender) {
    var details = {};
    var url = text(options.url) || (!options.domain && !options.domains && !options.all ? senderURL(sender) : "");
    if (url) {
      details.url = url;
    }
    if (options.name) {
      details.name = String(options.name);
    }
    if (options.storeId) {
      details.storeId = String(options.storeId);
    }
    if (options.partitionKey) {
      details.partitionKey = options.partitionKey;
    } else if (options.includePartitioned) {
      details.partitionKey = {};
    }
    return details;
  }

  function handleGetCookies(message, sender, sendResponse) {
    var options = message.options || {};
    getAllCookiesWithPartitionFallback(cookieQueryDetails(options, sender))
      .then(function (cookies) {
        var filtered = cookies.map(normalizeCookie).filter(function (cookie) {
          return cookieMatches(cookie, options, sender);
        });
        sendResponse({
          ok: true,
          cookies: filtered,
          header: cookieHeader(filtered),
          by_domain: cookiesByDomain(filtered),
          count: filtered.length,
        });
      })
      .catch(function (err) {
        sendResponse({ ok: false, error: err && (err.message || String(err)) });
      });
    return true;
  }

  runtime.onMessage.addListener(function (message, sender, sendResponse) {
    if (!message || !message.type) {
      return false;
    }

    if (message.type === GET_COOKIES_MESSAGE) {
      return handleGetCookies(message, sender, sendResponse);
    }

    if (message.type !== REPORT_MESSAGE) {
      return false;
    }

    var endpoint = message.endpoint;
    if (!endpoint && sender && sender.tab && sender.tab.url) {
      try {
        endpoint = new URL("/__wx_channels_api/platform/browser", sender.tab.url).href;
      } catch (e) {}
    }
    if (!endpoint) {
      sendResponse({ ok: false, error: "missing endpoint" });
      return false;
    }

    fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(message.payload || {}),
    })
      .then(function (resp) {
        sendResponse({ ok: resp.ok, status: resp.status });
      })
      .catch(function (err) {
        sendResponse({ ok: false, error: err && (err.message || String(err)) });
      });
    return true;
  });
})();
