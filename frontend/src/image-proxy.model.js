const image_proxy_paths = {
  weibo: "/api/weibo/imgproxy",
};

export function proxy_image_url(platform_id, raw_url) {
  const image_url = String(raw_url || "").trim();
  const proxy_path =
    image_proxy_paths[
      String(platform_id || "")
        .trim()
        .toLowerCase()
    ];
  if (!proxy_path || !/^https:\/\//i.test(image_url)) return image_url;
  return `${proxy_path}?url=${encodeURIComponent(image_url)}`;
}
