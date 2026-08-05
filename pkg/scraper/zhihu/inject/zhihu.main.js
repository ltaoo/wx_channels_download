(function () {
  var LOG_PREFIX = "[WX-ZHIHU]";
  function log() {
    try {
      console.log.apply(console, [LOG_PREFIX].concat(Array.prototype.slice.call(arguments)));
    } catch (e) {}
  }

  log("script loaded", {
    href: location.href,
    protocol: location.protocol,
    hostname: location.hostname,
    pathname: location.pathname,
    readyState: document.readyState,
  });

  if (location.protocol !== "https:" || location.hostname !== "www.zhihu.com" || location.pathname !== "/") {
    log("skip by location");
    return;
  }
  if (window.__wx_platform_zhihu_topstory_recommend__) {
    log("skip duplicate script");
    return;
  }
  window.__wx_platform_zhihu_topstory_recommend__ = true;
  log("script activated");

  var reported = new Set();
  var observed = new WeakSet();
  var observer = new IntersectionObserver(
    function (entries) {
      log("intersection entries", entries.length);
      entries.forEach(function (entry) {
        log("intersection entry", {
          isIntersecting: entry.isIntersecting,
          ratio: entry.intersectionRatio,
          className: entry.target && entry.target.className,
        });
        if (!entry.isIntersecting || entry.intersectionRatio < 0.35) {
          return;
        }
        reportCard(entry.target);
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
    return Array.prototype.slice.call(root.querySelectorAll(selector)).map(function (el) {
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
        if (urls[j].indexOf("/p/") >= 0 || urls[j].indexOf("zhuanlan.zhihu.com") >= 0) {
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
      card.querySelector(".ContentItem-title") && card.querySelector(".ContentItem-title").textContent,
      card.querySelector(".RichContent-inner") && card.querySelector(".RichContent-inner").textContent,
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
    return !!(card && card.querySelector(".Pc-feedAd-new, .Pc-feedAd-new-title"));
  }

  function reportCard(card) {
    log("reportCard called", {
      className: card && card.className,
      text: card && text(card.textContent).slice(0, 100),
    });
    if (isAdCard(card)) {
      log("reportCard skipped", { reason: "ad card" });
      return;
    }
    var feed = card.querySelector(".Feed");
    var contentItem = card.querySelector(".ContentItem");
    var feedExtra = parseJSON(attr(feed, "data-za-extra-module"));
    var zop = parseJSON(attr(contentItem, "data-zop"));
    var contentExtra = parseJSON(attr(contentItem, "data-za-extra-module"));
    var feedContent = (feedExtra.card && feedExtra.card.content) || {};
    var itemContent = (contentExtra.card && contentExtra.card.content) || {};
    var link = findContentLink(card);
    var contentType = normalizeContentType(
      first(zop.type, feedContent.type, itemContent.type, attr(contentItem, "itemprop")),
      link && link.href && link.href.indexOf("/zvideo/") >= 0 ? "video" : "",
    );
    var contentURL = findContentURL(card, contentItem, link, contentType);
    var questionURL = absoluteURL(metaContent(card, '[itemprop="zhihu:question"] meta[itemprop="url"]'));
    var contentToken = first(
      itemContent.token,
      feedContent.token,
      zop.itemId,
      attr(contentItem, "name"),
      metaContent(contentItem || card, 'meta[itemprop="url"]'),
    );
    var contentExternalID = zhihuUnique(contentType, contentToken, contentURL);
    if (!contentExternalID || reported.has(contentExternalID)) {
      log("reportCard skipped", {
        reason: !contentExternalID ? "missing contentExternalID" : "already reported",
        contentType: contentType,
        contentToken: contentToken,
        contentURL: contentURL,
        contentExternalID: contentExternalID,
      });
      return;
    }
    reported.add(contentExternalID);

    var authorLink = findAuthorLink(card);
    var authorURL = absoluteURL(authorLink && authorLink.href);
    var authorMemberHashID = first(itemContent.author_member_hash_id, feedContent.author_member_hash_id);
    var authorName = first(
      zop.authorName,
      authorLink && authorLink.textContent,
      card.querySelector(".AuthorInfo-name") && card.querySelector(".AuthorInfo-name").textContent,
      card.querySelector(".UserLink") && card.querySelector(".UserLink").textContent,
    );
    // Build a scraper.AnswerPage object and submit via WXU.downloader.browse
    var questionToken = first(itemContent.parent_token, feedContent.parent_token, "0");
    var dateCreated = metaContent(contentItem || card, 'meta[itemprop="dateCreated"]');
    var dateModified = metaContent(contentItem || card, 'meta[itemprop="dateModified"]');
    var createdTime = dateCreated ? Math.floor(new Date(dateCreated).getTime() / 1000) : 0;
    var updatedTime = dateModified ? Math.floor(new Date(dateModified).getTime() / 1000) : 0;
    var authorAvatarURL = absoluteURL(findAvatar(card));
    var answerCoverURL = absoluteURL(findImage(card));
    var title = first(zop.title, metaContent(card, 'meta[itemprop="name"]'), findTitle(card, link));
    var urlToken = "";
    if (authorURL) {
      var match = authorURL.match(/\/people\/([^/?&#]+)/);
      urlToken = match ? match[1] : (authorMemberHashID || "");
    }
    var upvoteNum = Number(first(itemContent.upvote_num, metaContent(card, 'meta[itemprop="upvoteCount"]'), 0)) || 0;
    var commentNum = Number(first(itemContent.comment_num, metaContent(card, 'meta[itemprop="commentCount"]'), 0)) || 0;
    var answerPage = {
      URL: {
        QuestionID: questionToken,
        AnswerID: contentToken,
        Canonical: contentURL,
      },
      Source: contentURL,
      Question: {
        id: questionToken,
        title: title,
      },
      Answer: {
        id: contentToken,
        type: contentType,
        author: {
          id: authorMemberHashID || "",
          name: authorName || "",
          url: authorURL || "",
          urlToken: urlToken,
          url_token: urlToken,
          avatarUrl: authorAvatarURL || "",
          avatar_url: authorAvatarURL || "",
        },
        content: answerCoverURL ? '<img src="' + answerCoverURL + '" />' : "",
        createdTime: createdTime,
        updatedTime: updatedTime,
        voteupCount: upvoteNum,
        commentCount: commentNum,
      },
    };
    log("reportCard answerPage", answerPage);
    if (typeof WXU !== "undefined" && WXU.downloader && typeof WXU.downloader.browse === "function") {
      WXU.downloader.browse([answerPage], { platform: "zhihu" });
      log("reportCard browse called", { contentExternalID: contentExternalID });
    } else {
      log("reportCard skipped", { reason: "WXU.downloader.browse not available" });
    }
  }

  function observeCards(root) {
    var scope = root || document;
    var selector = ".Topstory-recommend .Card.TopstoryItem.TopstoryItem-isRecommend";
    var cards = Array.prototype.slice.call(scope.querySelectorAll(selector));
    if (
      scope !== document &&
      scope.matches &&
      scope.matches(".Card.TopstoryItem.TopstoryItem-isRecommend")
    ) {
      cards.unshift(scope);
    }
    log("observeCards scan", {
      rootTag: scope.tagName || "document",
      rootClass: scope.className || "",
      count: cards.length,
    });
    cards.forEach(function (card) {
      if (isAdCard(card)) {
        log("observeCards skip ad", {
          className: card.className,
          text: text(card.textContent).slice(0, 100),
        });
        return;
      }
      if (observed.has(card)) {
        log("observeCards skip observed", card.className);
        return;
      }
      observed.add(card);
      observer.observe(card);
      log("observeCards observing", {
        className: card.className,
        text: text(card.textContent).slice(0, 100),
      });
    });
  }

  function start() {
    log("start", {
      readyState: document.readyState,
      topstoryExists: !!document.querySelector(".Topstory-recommend"),
      initialCards: document.querySelectorAll(".Card.TopstoryItem.TopstoryItem-isRecommend").length,
      recommendCards: document.querySelectorAll(".Topstory-recommend .Card.TopstoryItem.TopstoryItem-isRecommend").length,
    });
    observeCards(document);
    var container = document.querySelector(".Topstory-recommend") || document.body;
    if (!container) {
      log("start skipped: no container");
      return;
    }
    log("mutation observer attached", {
      tagName: container.tagName,
      className: container.className,
    });
    new MutationObserver(function (mutations) {
      log("mutations", mutations.length);
      mutations.forEach(function (mutation) {
        log("mutation nodes", mutation.addedNodes.length);
        mutation.addedNodes.forEach(function (node) {
          if (node && node.nodeType === 1) {
            log("mutation added element", {
              tagName: node.tagName,
              className: node.className,
            });
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
