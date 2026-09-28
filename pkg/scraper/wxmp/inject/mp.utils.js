/**
 * 微信公众号页面工具。
 *
 * 该文件提供无状态的数据读取、解析与转换工具，并向 WXU 注册文章页视频号
 * 的事件入口，必须在 mp.components.js 和 mp.main.js 之前加载。
 */
(() => {
  function first_non_empty() {
    for (let index = 0; index < arguments.length; index += 1) {
      let value = arguments[index];
      if (value === undefined || value === null) {
        continue;
      }
      value = String(value).trim();
      if (value) {
        return value;
      }
    }
    return "";
  }

  function get_page_data_value(key) {
    if (window.cgiDataNew && window.cgiDataNew[key] !== undefined) {
      return window.cgiDataNew[key];
    }
    if (window.cgiData && window.cgiData[key] !== undefined) {
      return window.cgiData[key];
    }
    return "";
  }

  function get_url_param(raw_url, key) {
    if (!raw_url) {
      return "";
    }
    try {
      return new URL(raw_url, window.location.href).searchParams.get(key) || "";
    } catch {
      return "";
    }
  }

  function decode_html_text(value) {
    const decoder = document.createElement("textarea");
    decoder.innerHTML = String(value || "");
    return decoder.value;
  }

  function parse_official_account_msg_list(data) {
    if (data && Array.isArray(data.list)) {
      return data.list;
    }
    const raw_list = (data && data.general_msg_list) || "";
    if (!raw_list) {
      return [];
    }
    try {
      const parsed =
        typeof raw_list === "string" ? JSON.parse(raw_list) : raw_list;
      return Array.isArray(parsed.list) ? parsed.list : [];
    } catch {
      return [];
    }
  }

  function decode_official_account_url(raw_url) {
    if (!raw_url) {
      return "";
    }
    try {
      const parsed_url = new URL(
        decode_html_text(raw_url),
        "https://mp.weixin.qq.com",
      );
      if (parsed_url.hostname !== "mp.weixin.qq.com") {
        return "";
      }
      return parsed_url.href;
    } catch {
      return "";
    }
  }

  function collect_push_article_entries(items) {
    const entries = [];
    const urls = new Set();

    function append(article, publish_time) {
      const url = decode_official_account_url(article && article.content_url);
      if (!url || urls.has(url)) {
        return;
      }
      urls.add(url);
      entries.push({ article, publish_time, url });
    }

    (items || []).forEach((item) => {
      const article = item.app_msg_ext_info || {};
      const publish_time = item.comm_msg_info?.datetime || 0;
      append(article, publish_time);
      (article.multi_app_msg_item_list || []).forEach((child) => {
        append(child, publish_time);
      });
    });
    return entries;
  }

  function article_ids_from_url(article_url, biz, external_id) {
    const parsed_url = new URL(article_url);
    let mid = Number(parsed_url.searchParams.get("mid")) || 0;
    let idx = Number(parsed_url.searchParams.get("idx")) || 0;
    const external_prefix = biz ? biz + "_" : "";
    if (
      (!mid || !idx) &&
      external_prefix &&
      external_id?.startsWith(external_prefix)
    ) {
      const external_parts = external_id
        .slice(external_prefix.length)
        .split("_");
      mid = mid || Number(external_parts[0]) || 0;
      idx = idx || Number(external_parts[1]) || 0;
    }
    return {
      biz: parsed_url.searchParams.get("__biz") || biz,
      idx: idx || 1,
      mid,
      sn: parsed_url.searchParams.get("sn") || "",
    };
  }

  function build_download_article(fetch_data, entry, credentials) {
    const parsed_article = (fetch_data && fetch_data.result) || {};
    const content = (fetch_data && fetch_data.content) || {};
    const account = (fetch_data && fetch_data.account) || {};
    const summary = entry.article || {};
    const ids = article_ids_from_url(
      entry.url,
      credentials.biz,
      content.external_id || "",
    );
    const publish_time =
      Number(content.publish_time || entry.publish_time) || 0;
    return {
      bizuin: parsed_article.bizuin || ids.biz,
      mid: Number(parsed_article.mid) || ids.mid,
      idx: Number(parsed_article.idx) || ids.idx,
      sn: parsed_article.sn || ids.sn,
      title: parsed_article.title || content.title || summary.title || "",
      desc:
        parsed_article.desc || content.description || summary.digest || "",
      content_noencode:
        parsed_article.content_noencode || summary.content || "",
      cdn_url:
        parsed_article.cdn_url || content.cover_url || summary.cover || "",
      link: parsed_article.link || content.url || entry.url,
      source_url:
        parsed_article.source_url ||
        content.source_url ||
        summary.source_url ||
        entry.url,
      user_name:
        parsed_article.user_name || account.external_id || "",
      nick_name:
        parsed_article.nick_name ||
        account.nickname ||
        summary.author ||
        "",
      round_head_img:
        parsed_article.round_head_img || account.avatar_url || "",
      author: parsed_article.author || summary.author || "",
      ori_create_time:
        Number(parsed_article.ori_create_time) ||
        (publish_time > 1000000000000
          ? Math.floor(publish_time / 1000)
          : publish_time),
      page_type: Number(parsed_article.page_type) || 0,
      item_show_type:
        Number(parsed_article.item_show_type) ||
        Number(summary.item_show_type) ||
        0,
      picture_page_info_list: parsed_article.picture_page_info_list || [],
      video_page_infos: parsed_article.video_page_infos || [],
      copyright_info:
        parsed_article.copyright_info || {
          copyright_stat: Number(summary.copyright_stat) || 0,
        },
    };
  }

  // 文章页内嵌的视频号 feed 只能通过 WeixinJSBridge 调用 getCommentDetails
  // 拿到，服务端无法代取，所以由 interceptor 改写后的 getCommentDetail 触发
  // channels:OnFeedProfileLoaded，这里把它缓存下来供「下载」提交。
  const captured_finder_feeds = [];

  // 与 common_share_video 的 getUrl 保持一致：换成公网 CDN 主机、补上 token，
  // 否则离开微信客户端后这个地址不可用。
  function normalize_finder_media_url(media) {
    if (!media) {
      return "";
    }
    let url = String(media.url || "");
    if (!url) {
      return "";
    }
    const url_token = String(media.urlToken || "");
    url = url.split("wxapp.tc.qq.com").join("finder.video.qq.com");
    if (url_token && !url.includes("&token=")) {
      url = url + url_token;
    }
    if (/^http:\/\//i.test(url)) {
      url = url.replace(/^http:\/\//i, "https://");
    }
    if (!url.includes("web=1")) {
      url = url + "&web=1";
    }
    if (!url.includes("&fexam=1")) {
      url = url + "&fexam=1";
    }
    return url;
  }

  // 与 getMediaObject 一致：附件视频优先，其次 objectDesc.media。
  function finder_media_from_feed(feed) {
    const object = feed && feed.object ? feed.object : feed;
    if (!object) {
      return null;
    }
    const attachment = object.attachmentList?.attachments?.[0];
    if (attachment && attachment.type === 1) {
      const attachment_media = attachment.video?.video?.desc?.media?.[0];
      if (attachment_media) {
        return attachment_media;
      }
    }
    const media_list = object.objectDesc?.media;
    return Array.isArray(media_list) && media_list.length ? media_list[0] : null;
  }

  // 规整成 pkg/scraper/wxchannels/types.go 的 ChannelsObject，下载端据此解密
  // （objectDesc.media[0].decodeKey）并下载。
  function normalize_finder_feed(feed) {
    const object = feed && feed.object ? feed.object : feed;
    const media = finder_media_from_feed(object);
    if (!media) {
      return null;
    }
    const desc = (object && object.objectDesc) || {};
    // 只处理视频形态；图片/直播形态不能按视频分支下载。
    const media_type =
      Number(media.mediaType || 0) || Number(desc.mediaType || 0) || 4;
    if (media_type !== 4) {
      return null;
    }
    const url = normalize_finder_media_url(media);
    if (!url) {
      return null;
    }
    const contact = (object && object.contact) || {};
    const decode_key =
      media.decodeKey === undefined || media.decodeKey === null
        ? ""
        : String(media.decodeKey);
    return {
      id: String((object && object.id) || decode_key || url),
      objectNonceId: String((object && object.objectNonceId) || ""),
      createtime: Number((object && object.createtime) || 0),
      type: "video",
      source_url: String((object && object.source_url) || window.location.href),
      contact: {
        username: String(contact.username || ""),
        nickname: String(contact.nickname || ""),
        headUrl: String(contact.headUrl || ""),
        signature: String(contact.signature || ""),
        coverImgUrl: String(contact.coverImgUrl || ""),
        liveCoverImgUrl: String(contact.liveCoverImgUrl || ""),
      },
      objectDesc: {
        description: String(desc.description || ""),
        mediaType: 4,
        media: [
          {
            url,
            urlToken: "",
            mediaType: 4,
            thumbUrl: String(media.thumbUrl || ""),
            coverUrl: String(media.coverUrl || ""),
            videoPlayLen: Number(media.videoPlayLen || 0),
            width: Number(media.width || 0),
            height: Number(media.height || 0),
            fileSize: Number(media.fileSize || 0),
            decodeKey: decode_key,
            spec: Array.isArray(media.spec) ? media.spec : [],
          },
        ],
      },
    };
  }

  function capture_finder_feed(feed) {
    // 事件在文章页自己的 promise 链里派发，抛错会打断页面渲染，必须全吞。
    try {
      const object = normalize_finder_feed(feed);
      if (!object) {
        return null;
      }
      const duplicated = captured_finder_feeds.some(
        (item) => item.id && item.id === object.id,
      );
      if (!duplicated) {
        captured_finder_feeds.push(object);
      }
      return object;
    } catch (error) {
      try {
        WXU.log.Error(error).Msg("[mp.utils.js]capture_finder_feed");
      } catch (_) {
        // 日志不可用时忽略，绝不影响文章页。
      }
      return null;
    }
  }

  function collect_finder_feeds() {
    return captured_finder_feeds.slice();
  }

  // 文章是否含 wxmp 适配器能直接下载的内容（正文/图片/内嵌视频）。
  function wxmp_article_downloadable(article) {
    if (!article) {
      return false;
    }
    if (String(article.content_noencode || "").trim()) {
      return true;
    }
    if (
      Array.isArray(article.picture_page_info_list) &&
      article.picture_page_info_list.length
    ) {
      return true;
    }
    return (
      Array.isArray(article.video_page_infos) &&
      article.video_page_infos.length > 0
    );
  }

  // 文章页内嵌的视频号 feed 由 interceptor 改写后的 getCommentDetail 通过
  // WXU.emit("channels:OnFeedProfileLoaded", feed) 派发，公众号页面不加载
  // channels.events.js，所以在这里补上同名的注册入口。
  Object.assign(WXE.Events, {
    FeedProfileLoaded: "channels:OnFeedProfileLoaded",
  });
  Object.assign(WXU, {
    /**
     * 获取到视频详情（文章页内嵌的视频号视频）
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onFetchFeedProfile: function (handler) {
      WXE.on(WXE.Events.FeedProfileLoaded, handler);
      return function () {
        WXE.off(WXE.Events.FeedProfileLoaded, handler);
      };
    },
  });
  WXU.onFetchFeedProfile(capture_finder_feed);

  window.WXMPUtils = Object.freeze({
    article_ids_from_url,
    build_download_article,
    capture_finder_feed,
    collect_finder_feeds,
    collect_push_article_entries,
    decode_html_text,
    decode_official_account_url,
    finder_media_from_feed,
    first_non_empty,
    get_page_data_value,
    get_url_param,
    normalize_finder_feed,
    normalize_finder_media_url,
    parse_official_account_msg_list,
    wxmp_article_downloadable,
  });
})();
