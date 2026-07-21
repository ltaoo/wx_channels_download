/**
 * 视频号专用工具。
 *
 * 这里集中放置视频号页面的下载入口，通用 DOM、请求和文件工具仍由
 * utils.js 提供。该文件必须在 channels.ws.js 之前加载。
 */
if (typeof WXU === "undefined") {
  throw new Error("utils.js must be loaded before channels.utils.js");
}

var WXChannelsUtils = (() => {
  function currentFeed(options) {
    return WXU.check_feed_existing(options);
  }

  async function download(spec, mp3) {
    const [err, feed] = currentFeed();
    if (err) return;
    const payload = { ...feed, mp3: !!mp3, original_url: feed.url,
      target_spec: spec, source_url: location.href };
    WXU.emit(WXU.Events.BeforeDownloadMedia, payload);
    __wx_channels_download4(payload, {
      spec,
      suffix: mp3 ? ".mp3" : payload.type === "picture" ? ".zip" : ".mp4",
    });
  }

  function downloadCurrent() {
    return __wx_channels_download_cur__();
  }

  function copyPageURL() {
    return __wx_channels_handle_copy__();
  }

  function downloadCover() {
    return __wx_channels_handle_download_cover();
  }

  return { currentFeed, download, downloadCurrent, copyPageURL, downloadCover };
})();
