(function () {
  var api = window.__wx_browser_extension__;
  if (!api) {
    return;
  }

  api.registerSite({
    id: "zhihu-topstory",
    matches: function (loc) {
      return loc.protocol === "https:" && loc.hostname === "www.zhihu.com" && loc.pathname === "/";
    },
    run: function (WXExt) {
      var log = WXExt.createLogger("ZHIHU");
      var reported = new Set();
      var observed = new WeakSet();
      var cardSelector = ".Topstory-recommend .Card.TopstoryItem.TopstoryItem-isRecommend";
      var observer = null;

      if ("IntersectionObserver" in window) {
        observer = new IntersectionObserver(
          function (entries) {
            entries.forEach(function (entry) {
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
      }

      function normalizeContentType(value, fallback) {
        var kind = WXExt.text(value || fallback).toLowerCase();
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
        var unique = WXExt.first(token, fallback);
        return unique ? "zhihu:" + (kind || "other") + ":" + unique : "";
      }

      function findContentURL(card, contentItem, link, contentType) {
        var root = contentItem || card;
        var urls = WXExt.metaContents(root, 'meta[itemprop="url"]')
          .map(function (url) {
            return WXExt.absoluteURL(url);
          })
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
        return WXExt.absoluteURL(WXExt.first(urls[0], link && link.href));
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
        return WXExt.first(
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
        return WXExt.first(
          img && (img.currentSrc || img.src),
          img && img.getAttribute("data-original"),
          img && img.getAttribute("data-actualsrc"),
        );
      }

      function findAvatar(card) {
        var img = card.querySelector(
          ".AuthorInfo-avatar img, .AuthorInfo .Avatar, .UserLink .Avatar, img.Avatar",
        );
        return WXExt.first(
          img && (img.currentSrc || img.src),
          img && img.getAttribute("data-original"),
          img && img.getAttribute("data-actualsrc"),
        );
      }

      function isAdCard(card) {
        return !!(card && card.querySelector(".Pc-feedAd-new, .Pc-feedAd-new-title"));
      }

      function reportCard(card) {
        if (!card || isAdCard(card)) {
          return;
        }

        var feed = card.querySelector(".Feed");
        var contentItem = card.querySelector(".ContentItem");
        var feedExtra = WXExt.parseJSON(WXExt.attr(feed, "data-za-extra-module"));
        var zop = WXExt.parseJSON(WXExt.attr(contentItem, "data-zop"));
        var contentExtra = WXExt.parseJSON(WXExt.attr(contentItem, "data-za-extra-module"));
        var feedContent = (feedExtra.card && feedExtra.card.content) || {};
        var itemContent = (contentExtra.card && contentExtra.card.content) || {};
        var link = findContentLink(card);
        var contentType = normalizeContentType(
          WXExt.first(zop.type, feedContent.type, itemContent.type, WXExt.attr(contentItem, "itemprop")),
          link && link.href && link.href.indexOf("/zvideo/") >= 0 ? "video" : "",
        );
        var contentURL = findContentURL(card, contentItem, link, contentType);
        var questionURL = WXExt.absoluteURL(
          WXExt.metaContent(card, '[itemprop="zhihu:question"] meta[itemprop="url"]'),
        );
        var contentToken = WXExt.first(
          itemContent.token,
          feedContent.token,
          zop.itemId,
          WXExt.attr(contentItem, "name"),
          WXExt.metaContent(contentItem || card, 'meta[itemprop="url"]'),
        );
        var contentExternalID = zhihuUnique(contentType, contentToken, contentURL);
        if (!contentExternalID || reported.has(contentExternalID)) {
          return;
        }
        reported.add(contentExternalID);

        var authorLink = findAuthorLink(card);
        var authorURL = WXExt.absoluteURL(authorLink && authorLink.href);
        var authorMemberHashID = WXExt.first(
          itemContent.author_member_hash_id,
          feedContent.author_member_hash_id,
        );
        var authorName = WXExt.first(
          zop.authorName,
          authorLink && authorLink.textContent,
          card.querySelector(".AuthorInfo-name") && card.querySelector(".AuthorInfo-name").textContent,
          card.querySelector(".UserLink") && card.querySelector(".UserLink").textContent,
        );
        var payload = {
          platform_id: "zhihu",
          platform_name: "知乎",
          content_type: contentType,
          content_external_id: contentExternalID,
          content_title: WXExt.first(
            zop.title,
            WXExt.metaContent(card, 'meta[itemprop="name"]'),
            findTitle(card, link),
          ),
          content_url: contentURL,
          content_source_url: contentURL,
          content_cover_url: WXExt.absoluteURL(findImage(card)),
          account_external_id: WXExt.first(authorMemberHashID, authorURL, authorName),
          account_username: authorURL,
          account_nickname: authorName,
          account_avatar_url: WXExt.absoluteURL(findAvatar(card)),
          zhihu_content_kind: contentType,
          zhihu_content_token: contentToken,
          zhihu_question_token: WXExt.first(itemContent.parent_token, feedContent.parent_token),
          zhihu_question_url: questionURL,
          zhihu_author_member_hash_id: authorMemberHashID,
          zhihu_date_created: WXExt.metaContent(contentItem || card, 'meta[itemprop="dateCreated"]'),
          zhihu_date_modified: WXExt.metaContent(contentItem || card, 'meta[itemprop="dateModified"]'),
          zhihu_upvote_num:
            Number(
              WXExt.first(
                itemContent.upvote_num,
                WXExt.metaContent(card, 'meta[itemprop="upvoteCount"]'),
                0,
              ),
            ) || 0,
          zhihu_comment_num:
            Number(
              WXExt.first(
                itemContent.comment_num,
                WXExt.metaContent(card, 'meta[itemprop="commentCount"]'),
                0,
              ),
            ) || 0,
          zhihu_feed_id: feedExtra.card && feedExtra.card.feed_id ? String(feedExtra.card.feed_id) : "",
        };

        WXExt.reportProfile(payload, { siteId: "zhihu" })
          .then(function (result) {
            log("reported", contentExternalID, result);
          })
          .catch(function (err) {
            log("report failed", contentExternalID, err && (err.message || err));
          });
      }

      function observeCard(card) {
        if (!card || isAdCard(card) || observed.has(card)) {
          return;
        }
        observed.add(card);
        if (observer) {
          observer.observe(card);
          return;
        }
        reportCard(card);
      }

      function start() {
        WXExt.observeNode(
          ".Topstory-recommend",
          function () {
            WXExt.observeElements(cardSelector, observeCard, { root: document });
          },
          {
            timeout: 10000,
            error: function () {
              log("topstory container not found");
            },
          },
        );
      }

      start();
    },
  });
})();
