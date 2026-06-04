/* global Img, computed */
import { api_client$ } from "@/store/index.js";

const PROXY_IMAGE_HOSTS = ["mmbiz.qpic.cn"];
const OFFICIAL_ACCOUNT_PLATFORMS = ["wx_official_account", "officialaccount"];

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
  const parsed = parsedURL(url);
  return parsed?.pathname === "/mp/proxy";
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

export function shouldProxyImage(rawURL, options = {}) {
  const url = String(rawURL || "").trim();
  if (!url || isProxiedImageURL(url) || !isRemoteHTTPURL(url)) {
    return false;
  }
  const parsed = parsedURL(url);
  if (parsed && hostMatches(parsed.hostname, PROXY_IMAGE_HOSTS)) {
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

export function resolveProxyImgSrc(rawURL, options = {}) {
  const url = String(rawURL || "").trim();
  const proxy = readOptionValue(options.proxy);
  if (!url) return url;
  if (proxy === false) return url;
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
  };
  const resolvedSrc = isReactiveValue(src)
    ? computed(src, (value) => resolveProxyImgSrc(value, options))
    : resolveProxyImgSrc(src, options);
  return Img({ ...imgProps, src: resolvedSrc });
}
