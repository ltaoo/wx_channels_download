/**
 * @file 事件总线
 */
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
    OfficialAccountLoaded: "OfficialAccountLoaded",
    APILoaded: "APILoaded",
    UtilsLoaded: "UtilsLoaded",
    Init: "Init",
    /** 首页推荐获取到视频列表 */
    PCFlowLoaded: "PCFlowLoaded",
    RecommendFeedsLoaded: "RecommendFeedsLoaded",
    UserFeedsLoaded: "UserFeedsLoaded",
    GotoNextFeed: "GotoNextFeed",
    GotoPrevFeed: "GotoPrevFeed",
    /** 获取到视频详情 */
    FeedProfileLoaded: "OnFeedProfileLoaded",
    /** 获取到直播详情 */
    LiveProfileLoaded: "OnLiveProfileLoaded",
    JoinLive: "JoinLive",
    /** 视频下载之前 */
    BeforeDownloadMedia: "BeforeDownloadMedia",
    /** 封面下载之前 */
    BeforeDownloadCover: "BeforeDownloadCover",
    /** 视频下载完成 */
    MediaDownloaded: "MediaDownloaded",
    /** MP3下载完成 */
    MP3Downloaded: "MP3Downloaded",
    /** 加载了 feed */
    Feed: "Feed",
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
    onAPILoaded(handler) {
      eventbus.on(ChannelsEvents.APILoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.APILoaded, handler);
      };
    },
    onUtilsLoaded(handler) {
      eventbus.on(ChannelsEvents.UtilsLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.UtilsLoaded, handler);
      };
    },
    onInit(handler) {
      eventbus.on(ChannelsEvents.Init, handler);
      return () => {
        eventbus.off(ChannelsEvents.Init, handler);
      };
    },
    /**
     * 首页获取到视频列表
     * @param {(feeds: ChannelsFeed[]) => void} handler
     */
    onPCFlowLoaded(handler) {
      eventbus.on(ChannelsEvents.PCFlowLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.PCFlowLoaded, handler);
      };
    },
    /**
     * 首页推荐 切换到下一个视频
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onGotoNextFeed(handler) {
      eventbus.on(ChannelsEvents.GotoNextFeed, handler);
      return () => {
        eventbus.off(ChannelsEvents.GotoNextFeed, handler);
      };
    },
    /**
     * 首页推荐 切换到上一个视频
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onGotoPrevFeed(handler) {
      eventbus.on(ChannelsEvents.GotoPrevFeed, handler);
      return () => {
        eventbus.off(ChannelsEvents.GotoPrevFeed, handler);
      };
    },
    /**
     * 获取到推荐列表
     * @param {(feeds: ChannelsFeed[]) => void} handler
     */
    onRecommendFeedsLoaded(handler) {
      eventbus.on(ChannelsEvents.RecommendFeedsLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.RecommendFeedsLoaded, handler);
      };
    },
    /**
     * 获取到指定用户的部分视频列表
     * @param {(feeds: ChannelsFeed[]) => void} handler
     */
    onUserFeedsLoaded(handler) {
      eventbus.on(ChannelsEvents.UserFeedsLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.UserFeedsLoaded, handler);
      };
    },
    /**
     * 获取到视频详情
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onFetchFeedProfile(handler) {
      eventbus.on(ChannelsEvents.FeedProfileLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.FeedProfileLoaded, handler);
      };
    },
    /**
     * 获取到直播详情
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onFetchLiveProfile(handler) {
      eventbus.on(ChannelsEvents.LiveProfileLoaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.LiveProfileLoaded, handler);
      };
    },
    /**
     * 加入直播
     * @param {(data: JoinLivePayload) => void} handler
     */
    onJoinLive(handler) {
      eventbus.on(ChannelsEvents.JoinLive, handler);
      return () => {
        eventbus.off(ChannelsEvents.JoinLive, handler);
      };
    },
    /**
     * 下载视频前
     * @param {(media: FeedProfilePayload) => void} handler
     */
    beforeDownloadMedia(handler) {
      eventbus.on(ChannelsEvents.BeforeDownloadMedia, handler);
      return () => {
        eventbus.off(ChannelsEvents.BeforeDownloadMedia, handler);
      };
    },
    /**
     * 下载封面前
     * @param {(media: FeedProfilePayload) => void} handler
     */
    beforeDownloadCover(handler) {
      eventbus.on(ChannelsEvents.BeforeDownloadCover, handler);
      return () => {
        eventbus.off(ChannelsEvents.BeforeDownloadCover, handler);
      };
    },
    /**
     * 视频下载完成
     * @param {(media: FeedProfilePayload) => void} handler
     */
    onMediaDownloaded(handler) {
      eventbus.on(ChannelsEvents.MediaDownloaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.MediaDownloaded, handler);
      };
    },
    /**
     * mp3 下载完成
     * @param {(media: FeedProfilePayload) => void} handler
     */
    onMP3Downloaded(handler) {
      eventbus.on(ChannelsEvents.MP3Downloaded, handler);
      return () => {
        eventbus.off(ChannelsEvents.MP3Downloaded, handler);
      };
    },
    /**
     * 加载了 feed。包括首页推荐、上一个下一个；视频详情页；直播详情页 都会触发该事件
     * 可用于记录访问过的视频
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onFeed(handler) {
      eventbus.on(ChannelsEvents.Feed, handler);
      return () => {
        eventbus.off(ChannelsEvents.Feed, handler);
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

WXE.onGotoNextFeed((feed) => {
  console.log("[eventbus.js]onGotoNextFeed", feed);
  WXE.emit(WXE.Events.Feed, feed);
});
WXE.onGotoPrevFeed((feed) => {
  console.log("[eventbus.js]onGotoPrevFeed", feed);
  WXE.emit(WXE.Events.Feed, feed);
});
WXE.onFetchFeedProfile((feed) => {
  console.log("[eventbus.js]onFetchFeedProfile", feed);
  WXE.emit(WXE.Events.Feed, feed);
});
var home_recommend_mounted = false;
WXE.onPCFlowLoaded((feeds) => {
  if (home_recommend_mounted) {
    return;
  }
  home_recommend_mounted = true;
  console.log("[eventbus.js]onPCFlowLoaded", feeds);
  WXE.emit(WXE.Events.Feed, feeds[0]);
});
