var WXE = (() => {
  var eventbus = mitt();
  var ChannelsEvents = {
    /** DOM 完全加载和解析 */
    DOMContentLoaded: "DOMContentLoaded",
    /** DOM 加载完成前 */
    DOMContentBeforeUnLoaded: "DOMContentBeforeUnLoaded",
    /** 所有资源加载完成 */
    WindowLoaded: "WindowLoaded",
    /** 页面卸载完成 */
    WindowUnLoaded: "WindowUnLoaded",
    /** 首页推荐获取到视频列表 */
    FeedListLoaded: "OnFeedListLoaded",
    /** 获取到视频详情 */
    FeedProfileLoaded: "OnFeedProfileLoaded",
    /** 获取到直播详情 */
    LiveProfileLoaded: "OnLiveProfileLoaded",
    /** 视频下载之前 */
    BeforeDownloadMedia: "BeforeDownloadMedia",
    /** 封面下载之前 */
    BeforeDownloadCover: "BeforeDownloadCover",
    /** 视频下载完成 */
    MediaDownloaded: "MediaDownloaded",
    /** MP3下载完成 */
    MP3Downloaded: "MP3Downloaded",
    GotoNextFeed: "GotoNextFeed",
    GotoPrevFeed: "GotoPrevFeed",
  };
  return {
    Events: ChannelsEvents,
    emit: eventbus.emit,
    /** DOM 完全加载和解析 */
    onDOMContentLoaded(handler) {
      eventbus.on(ChannelsEvents.DOMContentLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.DOMContentLoaded, handler);
      };
    },
    /** DOM 加载完成前 */
    onDOMContentBeforeUnLoaded(handler) {
      eventbus.on(ChannelsEvents.DOMContentBeforeUnLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.DOMContentBeforeUnLoaded, handler);
      };
    },
    /** 所有资源加载完成 */
    onWindowLoaded(handler) {
      eventbus.on(ChannelsEvents.WindowLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.WindowLoaded, handler);
      };
    },
    /** 页面卸载完成 */
    onWindowUnLoaded(handler) {
      eventbus.on(ChannelsEvents.WindowUnLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.WindowUnLoaded, handler);
      };
    },
    /**
     *
     * @param {(feeds: ChannelsFeed[]) => void} handler
     */
    onFeedListLoaded(handler) {
      eventbus.on(ChannelsEvents.FeedListLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.FeedListLoaded, handler);
      };
    },
    /**
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onFetchFeedProfile(handler) {
      eventbus.on(ChannelsEvents.FeedProfileLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.FeedProfileLoaded, handler);
      };
    },
    /**
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onFetchLiveProfile(handler) {
      eventbus.on(ChannelsEvents.LiveProfileLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.LiveProfileLoaded, handler);
      };
    },
    /**
     */
    beforeDownloadMedia(handler) {
      eventbus.on(ChannelsEvents.BeforeDownloadMedia, handler);
      return () => {
        eventbus.off(ChannelsEvents.BeforeDownloadMedia, handler);
      };
    },
    beforeDownloadCover(handler) {
      eventbus.on(ChannelsEvents.BeforeDownloadCover, handler);
      return () => {
        eventbus.off(ChannelsEvents.BeforeDownloadCover, handler);
      };
    },
    onMediaDownloaded(handler) {
      eventbus.on(ChannelsEvents.MediaDownloaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.MediaDownloaded, handler);
      };
    },
    onMP3Downloaded(handler) {
      eventbus.on(ChannelsEvents.MP3Downloaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.MP3Downloaded, handler);
      };
    },
    /**
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onGotoNextFeed(handler) {
      eventbus.on(ChannelsEvents.GotoNextFeed, handler);
      return () => {
        eventbus.off(ChannelsEvents.GotoNextFeed, handler);
      };
    },
    /**
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onGotoPrevFeed(handler) {
      eventbus.on(ChannelsEvents.GotoPrevFeed, handler);
      return () => {
        eventbus.off(ChannelsEvents.GotoPrevFeed, handler);
      };
    },
  };
})();

document.addEventListener("DOMContentLoaded", function () {
  WXE.emit(WXE.Events.DOMContentLoaded, {
    href: window.location.href,
  });
});
window.addEventListener("beforeunload", function () {
  // 用户即将离开页面时触发（DOM 还存在）
  WXE.emit(WXE.Events.DOMContentBeforeUnLoaded, {
    href: window.location.href,
  });
});
window.addEventListener("load", function () {
  WXE.emit(WXE.Events.WindowLoaded, {
    href: window.location.href,
  });
});
window.addEventListener("unload", function () {
  // 页面即将卸载时触发（DOM 即将被销毁）
  WXE.emit(WXE.Events.WindowUnLoaded, {
    href: window.location.href,
  });
});
