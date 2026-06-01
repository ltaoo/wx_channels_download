var WXRouter = (() => {
  const DEFAULT_ORIGIN = "https://channels.weixin.qq.com";

  function isObject(value) {
    return value !== null && typeof value === "object";
  }

  function pick(...values) {
    for (let i = 0; i < values.length; i += 1) {
      const v = values[i];
      if (v !== undefined && v !== null && String(v).trim() !== "") {
        return v;
      }
    }
    return "";
  }

  function normalizeUint64String(value) {
    if (value === undefined || value === null) {
      return "";
    }
    const s = String(value).trim();
    if (!s) {
      return "";
    }
    const head = s.split("_")[0];
    if (!/^\d+$/.test(head)) {
      return "";
    }
    try {
      const n = BigInt(head);
      if (n < 0n || n > 18446744073709551615n) {
        return "";
      }
      return n.toString(10);
    } catch (e) {
      return "";
    }
  }

  function encodeUint64ToBase64(decimalUint64) {
    try {
      let n = BigInt(decimalUint64);
      if (n < 0n || n > 18446744073709551615n) {
        return "";
      }
      const bytes = new Uint8Array(8);
      for (let i = 7; i >= 0; i -= 1) {
        bytes[i] = Number(n & 255n);
        n >>= 8n;
      }
      let bin = "";
      for (let i = 0; i < bytes.length; i += 1) {
        bin += String.fromCharCode(bytes[i]);
      }
      return btoa(bin)
        .replace(/\+/g, "-")
        .replace(/\//g, "_")
        .replace(/=+$/g, "");
    } catch (e) {
      return "";
    }
  }

  function getOrigin(optOrigin) {
    const origin = pick(optOrigin, isObject(window) ? window.location.origin : "");
    if (origin && origin.startsWith("http")) {
      return origin;
    }
    return DEFAULT_ORIGIN;
  }

  function isLiveFeed(feed) {
    if (!feed) return false;
    if (feed.liveInfo) return true;
    if (feed.type === "live") return true;
    return false;
  }

  function buildJumpUrl(feed, options) {
    const origin = getOrigin(options && options.origin);
    const url = new URL(origin);

    const encryptedObjectId = pick(
      feed && (feed.eid || feed.encrypted_objectid || feed.encryptedObjectId),
    );

    if (isLiveFeed(feed)) {
      url.pathname = "/web/pages/live";
      const username = pick(
        feed &&
          feed.anchorContact &&
          (feed.anchorContact.username || feed.anchorContact.id),
        feed && feed.contact && (feed.contact.username || feed.contact.id),
        feed && feed.username,
      );
      if (username) {
        url.searchParams.set("username", username);
      }
      if (encryptedObjectId) {
        url.searchParams.set("eid", encryptedObjectId);
      } else {
        const oid = normalizeUint64String(pick(feed && feed.id, feed && feed.objectId));
        if (oid) {
          url.searchParams.set("oid", encodeUint64ToBase64(oid));
        }
      }
      return url.toString();
    }

    url.pathname = "/web/pages/feed";

    const rawOid = normalizeUint64String(
      pick(feed && feed.id, feed && feed.objectId, feed && feed.objectid),
    );
    const rawNid = normalizeUint64String(
      pick(feed && feed.objectNonceId, feed && feed.nonce_id, feed && feed.nid),
    );

    const username = pick(
      feed && feed.contact && (feed.contact.username || feed.contact.id),
      feed && feed.username,
    );

    if (username) {
      url.searchParams.set("username", username);
    }

    if (encryptedObjectId) {
      url.searchParams.set("eid", encryptedObjectId);
      return url.toString();
    }

    if (rawOid) {
      url.searchParams.set("oid", encodeUint64ToBase64(rawOid));
    }
    if (rawNid) {
      url.searchParams.set("nid", encodeUint64ToBase64(rawNid));
    }

    const extra = (options && options.extra) || null;
    if (extra && typeof extra === "object") {
      Object.keys(extra).forEach((k) => {
        const v = extra[k];
        if (v === undefined || v === null) return;
        url.searchParams.set(k, String(v));
      });
    }

    return url.toString();
  }

  function pushState(url, state) {
    const nextUrl = pick(url);
    if (!nextUrl) {
      return false;
    }
    try {
      history.pushState(state || {}, "", nextUrl);
      window.dispatchEvent(new PopStateEvent("popstate", { state: history.state }));
      return true;
    } catch (e) {
      return false;
    }
  }

  function replaceState(url, state) {
    const nextUrl = pick(url);
    if (!nextUrl) {
      return false;
    }
    try {
      history.replaceState(state || {}, "", nextUrl);
      window.dispatchEvent(new PopStateEvent("popstate", { state: history.state }));
      return true;
    } catch (e) {
      return false;
    }
  }

  function gotoFeed(feed, options) {
    const url = buildJumpUrl(feed, options);
    return pushState(url, (options && options.state) || {});
  }

  const api = {
    buildJumpUrl,
    gotoFeed,
    pushState,
    replaceState,
    encodeUint64ToBase64,
  };

  try {
    if (typeof window !== "undefined") {
      window.WXRouter = api;
    }
  } catch (e) {
    void e;
  }

  try {
    if (typeof WXU !== "undefined" && WXU) {
      WXU.router = api;
    }
  } catch (e) {
    void e;
  }

  return api;
})();
