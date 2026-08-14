/**
 * 微信公众号页面工具。
 *
 * 该文件只提供无状态的数据读取、解析与转换工具，必须在 mp.components.js
 * 和 mp.main.js 之前加载。
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
      bizuin: ids.biz,
      mid: ids.mid,
      idx: ids.idx,
      sn: ids.sn,
      title: parsed_article.title || content.title || summary.title || "",
      desc: content.description || summary.digest || "",
      content_noencode: parsed_article.content || summary.content || "",
      cdn_url: content.cover_url || summary.cover || "",
      link: content.url || entry.url,
      source_url: content.source_url || summary.source_url || entry.url,
      user_name: parsed_article.author_id || account.external_id || "",
      nick_name:
        parsed_article.author_nickname ||
        account.nickname ||
        summary.author ||
        "",
      round_head_img: parsed_article.author_avatar || account.avatar_url || "",
      author: parsed_article.creator || summary.author || "",
      ori_create_time:
        publish_time > 1000000000000
          ? Math.floor(publish_time / 1000)
          : publish_time,
      page_type: Number(parsed_article.type) || 0,
      item_show_type: Number(summary.item_show_type) || 0,
      picture_page_info_list: parsed_article.picture_page_info_list || [],
      video_page_infos: parsed_article.videos || [],
      copyright_info: {
        copyright_stat: Number(summary.copyright_stat) || 0,
      },
    };
  }

  window.WXMPUtils = Object.freeze({
    article_ids_from_url,
    build_download_article,
    collect_push_article_entries,
    decode_html_text,
    decode_official_account_url,
    first_non_empty,
    get_page_data_value,
    get_url_param,
    parse_official_account_msg_list,
  });
})();
