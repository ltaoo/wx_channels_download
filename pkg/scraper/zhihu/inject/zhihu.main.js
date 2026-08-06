(function () {
  WXU.log
    .Info()
    .Str("file", "zhihu.main.js")
    .Str("href", location.href)
    .Str("protocol", location.protocol)
    .Str("hostname", location.hostname)
    .Str("pathname", location.pathname)
    .Str("ready_state", document.readyState)
    .Msg("script loaded");

  if (
    location.protocol !== "https:" ||
    location.hostname !== "www.zhihu.com" ||
    location.pathname !== "/"
  ) {
    WXU.log.Info().Str("file", "zhihu.main.js").Msg("skip by location");
    return;
  }
  if (window.__wx_platform_zhihu_topstory_recommend__) {
    WXU.log.Info().Str("file", "zhihu.main.js").Msg("skip duplicate script");
    return;
  }
  window.__wx_platform_zhihu_topstory_recommend__ = true;
  WXU.log.Info().Str("file", "zhihu.main.js").Msg("script activated");

  var reported = new Set();
  var observed = new WeakSet();
  var observer = new IntersectionObserver(
    function (entries) {
      WXU.log
        .Info()
        .Str("file", "zhihu.main.js")
        .Int("count", entries.length)
        .Msg("intersection entries");
      entries.forEach(function (entry) {
        WXU.log
          .Info()
          .Str("file", "zhihu.main.js")
          .Bool("is_intersecting", entry.isIntersecting)
          .Float("ratio", entry.intersectionRatio)
          .Str("class_name", entry.target && entry.target.className)
          .Msg("intersection entry");
        if (!entry.isIntersecting || entry.intersectionRatio < 0.35) {
          return;
        }
        reportCard(entry.target).catch(function (error) {
          WXU.log
            .Error()
            .Str("file", "zhihu.main.js")
            .Err(error)
            .Msg("reportCard failed");
        });
      });
    },
    {
      threshold: [0.35],
      rootMargin: "0px 0px -10% 0px",
    },
  );

  function text(value) {
    return value == null ? "" : String(value).trim();
  }

  function absoluteURL(value) {
    var url = text(value);
    if (!url) {
      return "";
    }
    try {
      var u = new URL(url, location.href);
      u.hash = "";
      return u.href;
    } catch (e) {
      return url;
    }
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

  function metaContent(root, selector) {
    var el = root.querySelector(selector);
    return attr(el, "content");
  }

  function metaContents(root, selector) {
    return Array.prototype.slice
      .call(root.querySelectorAll(selector))
      .map(function (el) {
        return attr(el, "content");
      });
  }

  function normalizeContentType(value, fallback) {
    var kind = text(value || fallback).toLowerCase();
    if (kind === "answer" || kind === "answers") {
      return "answer";
    }
    if (kind === "article" || kind === "articles" || kind === "post") {
      return "article";
    }
    if (kind === "zvideo" || kind === "video") {
      return "video";
    }
    return "other";
  }

  function zhihuUnique(kind, token, fallback) {
    var unique = first(token, fallback);
    if (!unique) {
      return "";
    }
    return "zhihu:" + (kind || "other") + ":" + unique;
  }

  function findContentURL(card, contentItem, link, contentType) {
    var root = contentItem || card;
    var urls = metaContents(root, 'meta[itemprop="url"]')
      .map(absoluteURL)
      .filter(Boolean);
    if (contentType === "answer") {
      for (var i = 0; i < urls.length; i += 1) {
        if (urls[i].indexOf("/answer/") >= 0) {
          return urls[i];
        }
      }
    }
    if (contentType === "article") {
      for (var j = 0; j < urls.length; j += 1) {
        if (
          urls[j].indexOf("/p/") >= 0 ||
          urls[j].indexOf("zhuanlan.zhihu.com") >= 0
        ) {
          return urls[j];
        }
      }
    }
    return absoluteURL(first(urls[0], link && link.href));
  }

  function findContentLink(card) {
    var selectors = [
      "h2 a[href]",
      ".ContentItem-title a[href]",
      ".AnswerItem-title a[href]",
      "a[data-za-detail-view-element_name][href]",
      "a[href*='/question/']",
      "a[href*='/answer/']",
      "a[href*='/zvideo/']",
    ];
    for (var i = 0; i < selectors.length; i += 1) {
      var link = card.querySelector(selectors[i]);
      if (link && link.href) {
        return link;
      }
    }
    return null;
  }

  function findTitle(card, link) {
    return first(
      link && (link.getAttribute("title") || link.textContent),
      card.querySelector("h2") && card.querySelector("h2").textContent,
      card.querySelector(".ContentItem-title") &&
        card.querySelector(".ContentItem-title").textContent,
      card.querySelector(".RichContent-inner") &&
        card.querySelector(".RichContent-inner").textContent,
    );
  }

  function findAuthorLink(card) {
    var selectors = [
      ".AuthorInfo-name a[href]",
      ".UserLink-link[href]",
      "a[href*='/people/']",
      "a[href*='/org/']",
    ];
    for (var i = 0; i < selectors.length; i += 1) {
      var link = card.querySelector(selectors[i]);
      if (link && link.href) {
        return link;
      }
    }
    return null;
  }

  function findImage(card) {
    var img = card.querySelector(".RichContent-cover img, img");
    return first(
      img && (img.currentSrc || img.src),
      img && img.getAttribute("data-original"),
      img && img.getAttribute("data-actualsrc"),
    );
  }

  function findAvatar(card) {
    var img = card.querySelector(
      ".AuthorInfo-avatar img, .AuthorInfo .Avatar, .UserLink .Avatar, img.Avatar",
    );
    return first(
      img && (img.currentSrc || img.src),
      img && img.getAttribute("data-original"),
      img && img.getAttribute("data-actualsrc"),
    );
  }

  function isAdCard(card) {
    return !!(
      card && card.querySelector(".Pc-feedAd-new, .Pc-feedAd-new-title")
    );
  }

  async function findRecommendFeed(itemId) {
    var normalizedItemId = text(itemId);
    if (!normalizedItemId || typeof wx_fetch === "undefined") {
      return null;
    }
    try {
      var response = await wx_fetch.get_response(
        "/api/v3/feed/topstory/recommend",
      );
      if (!response) {
        return null;
      }
      var payload =
        typeof response.json === "function" ? await response.json() : response;
      var data = payload && Array.isArray(payload.data) ? payload.data : [];
      return (
        data.find(function (item) {
          return (
            item && item.target && text(item.target.id) === normalizedItemId
          );
        }) || null
      );
    } catch (error) {
      WXU.log
        .Error()
        .Str("file", "zhihu.main.js")
        .Str("item_id", normalizedItemId)
        .Err(error)
        .Msg("read recommend feed response failed");
      return null;
    }
  }

  async function reportCard(card) {
    WXU.log
      .Info()
      .Str("file", "zhihu.main.js")
      .Str("class_name", card && card.className)
      .Str("text", card && text(card.textContent).slice(0, 100))
      .Msg("reportCard called");
    if (isAdCard(card)) {
      WXU.log
        .Info()
        .Str("file", "zhihu.main.js")
        .Str("reason", "ad card")
        .Msg("reportCard skipped");
      return;
    }
    var feed = card.querySelector(".Feed");
    var contentItem = card.querySelector(".ContentItem");
    var feedExtra = parseJSON(attr(feed, "data-za-extra-module"));
    var zop = parseJSON(attr(contentItem, "data-zop"));
    var itemId = text(zop.itemId);
    var recommendFeed = await findRecommendFeed(itemId);
    WXU.log
      .Info()
      .Str("file", "zhihu.main.js")
      .Str("item_id", itemId)
      .Bool("matched", !!recommendFeed)
      .JSON("recommend_feed", recommendFeed)
      .Msg("recommend feed lookup");
    if (!recommendFeed) {
      return;
    }
    downloadermodel$.browse([recommendFeed], { platform: "zhihu" });
  }

  function observeCards(root) {
    var scope = root || document;
    var selector =
      ".Topstory-recommend .Card.TopstoryItem.TopstoryItem-isRecommend";
    var cards = Array.prototype.slice.call(scope.querySelectorAll(selector));
    if (
      scope !== document &&
      scope.matches &&
      scope.matches(".Card.TopstoryItem.TopstoryItem-isRecommend")
    ) {
      cards.unshift(scope);
    }
    WXU.log
      .Info()
      .Str("file", "zhihu.main.js")
      .Str("root_tag", scope.tagName || "document")
      .Str("root_class", scope.className || "")
      .Int("count", cards.length)
      .Msg("observeCards scan");
    cards.forEach(function (card) {
      if (isAdCard(card)) {
        WXU.log
          .Info()
          .Str("file", "zhihu.main.js")
          .Str("class_name", card.className)
          .Str("text", text(card.textContent).slice(0, 100))
          .Msg("observeCards skip ad");
        return;
      }
      if (observed.has(card)) {
        WXU.log
          .Info()
          .Str("file", "zhihu.main.js")
          .Str("class_name", card.className)
          .Msg("observeCards skip observed");
        return;
      }
      observed.add(card);
      observer.observe(card);
      WXU.log
        .Info()
        .Str("file", "zhihu.main.js")
        .Str("class_name", card.className)
        .Str("text", text(card.textContent).slice(0, 100))
        .Msg("observeCards observing");
    });
  }

  function start() {
    WXU.log
      .Info()
      .Str("file", "zhihu.main.js")
      .Str("ready_state", document.readyState)
      .Bool("topstory_exists", !!document.querySelector(".Topstory-recommend"))
      .Int(
        "initial_cards",
        document.querySelectorAll(".Card.TopstoryItem.TopstoryItem-isRecommend")
          .length,
      )
      .Int(
        "recommend_cards",
        document.querySelectorAll(
          ".Topstory-recommend .Card.TopstoryItem.TopstoryItem-isRecommend",
        ).length,
      )
      .Msg("start");
    observeCards(document);
    var container =
      document.querySelector(".Topstory-recommend") || document.body;
    if (!container) {
      WXU.log
        .Info()
        .Str("file", "zhihu.main.js")
        .Msg("start skipped: no container");
      return;
    }
    WXU.log
      .Info()
      .Str("file", "zhihu.main.js")
      .Str("tag_name", container.tagName)
      .Str("class_name", container.className)
      .Msg("mutation observer attached");
    new MutationObserver(function (mutations) {
      WXU.log
        .Info()
        .Str("file", "zhihu.main.js")
        .Int("count", mutations.length)
        .Msg("mutations");
      mutations.forEach(function (mutation) {
        WXU.log
          .Info()
          .Str("file", "zhihu.main.js")
          .Int("count", mutation.addedNodes.length)
          .Msg("mutation nodes");
        mutation.addedNodes.forEach(function (node) {
          if (node && node.nodeType === 1) {
            WXU.log
              .Info()
              .Str("file", "zhihu.main.js")
              .Str("tag_name", node.tagName)
              .Str("class_name", node.className)
              .Msg("mutation added element");
            observeCards(node);
          }
        });
      });
    }).observe(container, { childList: true, subtree: true });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start, { once: true });
  } else {
    start();
  }
})();
