/* global Img, computed */
import { api_client$ } from "@/store/index.js";

const OFFICIAL_ACCOUNT_IMAGE_HOSTS = ["mmbiz.qpic.cn"];
const XIAOHONGSHU_IMAGE_HOSTS = ["xhscdn.com", "xhscdn.net"];
const BILIBILI_IMAGE_HOSTS = ["hdslb.com", "biliimg.com"];
const DOUBAN_IMAGE_HOSTS = ["doubanio.com"];
const INSTAGRAM_IMAGE_HOSTS = ["cdninstagram.com", "fbcdn.net"];
const QIDIAN_IMAGE_HOSTS = ["ccportrait.yuewen.com"];
const WEIBO_IMAGE_HOSTS = ["sinaimg.cn", "sinaimg.com"];
const OFFICIAL_ACCOUNT_PLATFORMS = ["wxmp", "officialaccount"];

function getConfig() {
  if (typeof WXU !== "undefined" && WXU.config) return WXU.config;
  if (typeof window !== "undefined" && window.__wx_channels_config__) {
    return window.__wx_channels_config__;
  }
  return {};
}

function getAPIClientOrigin() {
  const hostname = String(api_client$?.hostname || "").trim();
  const base = typeof window !== "undefined" ? window.location.origin : "";
  if (!hostname) {
    return base;
  }
  try {
    return new URL(hostname, base).origin;
  } catch {
    return hostname.replace(/\/+$/, "");
  }
}

function isReactiveValue(value) {
  return !!value && typeof value === "object" && "value" in value;
}

function readOptionValue(value) {
  return isReactiveValue(value) ? value.value : value;
}

function hostMatches(host, matches) {
  const normalized = String(host || "").toLowerCase();
  return matches.some(
    (item) => normalized === item || normalized.endsWith(`.${item}`),
  );
}

function parsedURL(rawURL) {
  const url = String(rawURL || "").trim();
  if (!url) return null;
  try {
    const base =
      typeof window !== "undefined" ? window.location.origin : "http://localhost";
    return new URL(url, base);
  } catch {
    return null;
  }
}

export function isProxiedImageURL(rawURL) {
  const url = String(rawURL || "").trim();
  if (!url) return false;
  if (url.includes("/mp/proxy?")) return true;
  if (url.includes("/xiaohongshu/proxy?")) return true;
  if (url.includes("/bilibili/proxy?")) return true;
  if (url.includes("/douban/proxy?")) return true;
  if (url.includes("/instagram/proxy?")) return true;
  if (url.includes("/qidian/proxy?")) return true;
  if (url.includes("/weibo/proxy?")) return true;
  const parsed = parsedURL(url);
  return (
    parsed?.pathname === "/mp/proxy" ||
    parsed?.pathname === "/xiaohongshu/proxy" ||
    parsed?.pathname === "/bilibili/proxy" ||
    parsed?.pathname === "/douban/proxy" ||
    parsed?.pathname === "/instagram/proxy" ||
    parsed?.pathname === "/qidian/proxy" ||
    parsed?.pathname === "/weibo/proxy"
  );
}

function isRemoteHTTPURL(rawURL) {
  const url = String(rawURL || "").trim();
  return /^https?:\/\//i.test(url) || /^\/\//.test(url);
}

function contextPlatform(options = {}) {
  return String(
    readOptionValue(options.platformId) ||
      readOptionValue(options.platform_id) ||
      readOptionValue(options.platform) ||
      readOptionValue(options.sourcePlatform) ||
      "",
  )
    .trim()
    .toLowerCase();
}

function contextContentType(options = {}) {
  return String(
    readOptionValue(options.contentType) ||
      readOptionValue(options.content_type) ||
      readOptionValue(options.type) ||
      "",
  )
    .trim()
    .toLowerCase();
}

function isXiaohongshuImageURL(rawURL) {
  const parsed = parsedURL(rawURL);
  return parsed && hostMatches(parsed.hostname, XIAOHONGSHU_IMAGE_HOSTS);
}

function isBilibiliImageURL(rawURL) {
  const parsed = parsedURL(rawURL);
  return parsed && hostMatches(parsed.hostname, BILIBILI_IMAGE_HOSTS);
}

function isDoubanImageURL(rawURL) {
  const parsed = parsedURL(rawURL);
  return parsed && hostMatches(parsed.hostname, DOUBAN_IMAGE_HOSTS);
}

function isInstagramImageURL(rawURL) {
  const parsed = parsedURL(rawURL);
  return parsed && hostMatches(parsed.hostname, INSTAGRAM_IMAGE_HOSTS);
}

function isQidianImageURL(rawURL) {
  const parsed = parsedURL(rawURL);
  return parsed && hostMatches(parsed.hostname, QIDIAN_IMAGE_HOSTS);
}

function isWeiboImageURL(rawURL) {
  const parsed = parsedURL(rawURL);
  return parsed && hostMatches(parsed.hostname, WEIBO_IMAGE_HOSTS);
}

export function shouldProxyImage(rawURL, options = {}) {
  const url = String(rawURL || "").trim();
  if (!url || isProxiedImageURL(url) || !isRemoteHTTPURL(url)) {
    return false;
  }
  const parsed = parsedURL(url);
  if (parsed && hostMatches(parsed.hostname, OFFICIAL_ACCOUNT_IMAGE_HOSTS)) {
    return true;
  }
  if (isXiaohongshuImageURL(url)) {
    return true;
  }
  if (isBilibiliImageURL(url)) {
    return true;
  }
  if (isDoubanImageURL(url)) {
    return true;
  }
  if (isInstagramImageURL(url)) {
    return true;
  }
  if (isQidianImageURL(url)) {
    return true;
  }
  if (isWeiboImageURL(url)) {
    return true;
  }
  if (OFFICIAL_ACCOUNT_PLATFORMS.includes(contextPlatform(options))) {
    return true;
  }
  return contextContentType(options) === "article";
}

export function mpProxyURL(rawURL) {
  const url = String(rawURL || "").trim();
  if (!url || isProxiedImageURL(url) || !isRemoteHTTPURL(url)) {
    return url;
  }
  const cfg = getConfig();
  const token = cfg.officialServerRefreshToken || "";
  const params = new URLSearchParams();
  if (token) {
    params.set("token", token);
  }
  params.set("url", parsedURL(url)?.href || url);
  return `${getAPIClientOrigin()}/mp/proxy?${params.toString()}`;
}

export function xiaohongshuProxyURL(rawURL) {
  const url = String(rawURL || "").trim();
  if (!url || isProxiedImageURL(url) || !isRemoteHTTPURL(url)) {
    return url;
  }
  const params = new URLSearchParams();
  params.set("url", parsedURL(url)?.href || url);
  return `${getAPIClientOrigin()}/xiaohongshu/proxy?${params.toString()}`;
}

export function bilibiliProxyURL(rawURL) {
  const url = String(rawURL || "").trim();
  if (!url || isProxiedImageURL(url) || !isRemoteHTTPURL(url)) {
    return url;
  }
  const params = new URLSearchParams();
  params.set("url", parsedURL(url)?.href || url);
  return `${getAPIClientOrigin()}/bilibili/proxy?${params.toString()}`;
}

function doubanReferer(options = {}) {
  return String(
    readOptionValue(options.referer) ||
      readOptionValue(options.sourceURL) ||
      readOptionValue(options.source_url) ||
      readOptionValue(options.canonicalURL) ||
      readOptionValue(options.canonical_url) ||
      readOptionValue(options.url) ||
      "",
  ).trim();
}

export function doubanProxyURL(rawURL, options = {}) {
  const url = String(rawURL || "").trim();
  if (!url || isProxiedImageURL(url) || !isRemoteHTTPURL(url)) {
    return url;
  }
  const params = new URLSearchParams();
  params.set("url", parsedURL(url)?.href || url);
  const referer = doubanReferer(options);
  if (referer) {
    params.set("referer", referer);
  }
  return `${getAPIClientOrigin()}/douban/proxy?${params.toString()}`;
}

export function instagramProxyURL(rawURL) {
  const url = String(rawURL || "").trim();
  if (!url || isProxiedImageURL(url) || !isRemoteHTTPURL(url)) {
    return url;
  }
  const params = new URLSearchParams();
  params.set("url", parsedURL(url)?.href || url);
  return `${getAPIClientOrigin()}/instagram/proxy?${params.toString()}`;
}

export function qidianProxyURL(rawURL) {
  const url = String(rawURL || "").trim();
  if (!url || isProxiedImageURL(url) || !isRemoteHTTPURL(url)) {
    return url;
  }
  const params = new URLSearchParams();
  params.set("url", parsedURL(url)?.href || url);
  return `${getAPIClientOrigin()}/qidian/proxy?${params.toString()}`;
}

export function weiboProxyURL(rawURL) {
  const url = String(rawURL || "").trim();
  if (!url || isProxiedImageURL(url) || !isRemoteHTTPURL(url)) {
    return url;
  }
  const params = new URLSearchParams();
  params.set("url", parsedURL(url)?.href || url);
  return `${getAPIClientOrigin()}/weibo/proxy?${params.toString()}`;
}

export function resolveProxyImgSrc(rawURL, options = {}) {
  const url = String(rawURL || "").trim();
  const proxy = readOptionValue(options.proxy);
  if (!url) return url;
  if (proxy === false) return url;
  if (isXiaohongshuImageURL(url) && !isProxiedImageURL(url)) {
    return xiaohongshuProxyURL(url);
  }
  if (isBilibiliImageURL(url) && !isProxiedImageURL(url)) {
    return bilibiliProxyURL(url);
  }
  if (isDoubanImageURL(url) && !isProxiedImageURL(url)) {
    return doubanProxyURL(url, options);
  }
  if (isInstagramImageURL(url) && !isProxiedImageURL(url)) {
    return instagramProxyURL(url);
  }
  if (isQidianImageURL(url) && !isProxiedImageURL(url)) {
    return qidianProxyURL(url);
  }
  if (isWeiboImageURL(url) && !isProxiedImageURL(url)) {
    return weiboProxyURL(url);
  }
  if (
    (proxy === true && isRemoteHTTPURL(url)) ||
    shouldProxyImage(url, options)
  ) {
    return mpProxyURL(url);
  }
  return url;
}

export function ProxyImg(props = {}) {
  const {
    src,
    proxy,
    platformId,
    platform_id,
    platform,
    sourcePlatform,
    contentType,
    content_type,
    type,
    referer,
    sourceURL,
    source_url,
    canonicalURL,
    canonical_url,
    url,
    ...imgProps
  } = props;
  const options = {
    proxy,
    platformId,
    platform_id,
    platform,
    sourcePlatform,
    contentType,
    content_type,
    type,
    referer,
    sourceURL,
    source_url,
    canonicalURL,
    canonical_url,
    url,
  };
  const resolvedSrc = isReactiveValue(src)
    ? computed(src, (value) => resolveProxyImgSrc(value, options))
    : resolveProxyImgSrc(src, options);
  return Img({ ...imgProps, src: resolvedSrc });
}
