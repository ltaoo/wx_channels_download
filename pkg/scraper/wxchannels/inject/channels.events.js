/**
 * @file 视频号专用事件总线扩展。
 *
 * 该文件在 eventbus.js 之后加载，向 WXE 注册微信视频号专用的
 * 事件名和 on* 便捷监听方法。所有通过 WXU (utils.js) 派发的事件
 * 也会经过同一个 mitt 实例。
 *
 * 所有事件名均以 "channels:" 为前缀，避免与其他 scraper 重名。
 */
(function () {
  if (typeof WXE === "undefined") {
    throw new Error("eventbus.js must be loaded before channels.events.js");
  }

  // NOTE: DOMContentLoaded / DOMContentBeforeUnLoaded / WindowLoaded /
  // WindowUnLoaded — the four common DOM lifecycle events — are defined in eventbus.js,
  // and are not repeated here.
  // OfficialAccountRefresh is defined in eventbus.js for the wxmp scraper.

  // Register event names on WXE.Events for backward compatibility (existing scripts reference via WXE.Events.xxx)
  Object.assign(WXE.Events, {
    APILoaded: "channels:APILoaded",
    UtilsLoaded: "channels:UtilsLoaded",
    Init: "channels:Init",
    PCFlowLoaded: "channels:PCFlowLoaded",
    RecommendFeedsLoaded: "channels:RecommendFeedsLoaded",
    InteractionedFeedsLoaded: "channels:InteractionedFeedsLoaded",
    LiveUserFeedsLoaded: "channels:LiveUserFeedsLoaded",
    UserFeedsLoaded: "channels:UserFeedsLoaded",
    GotoNextFeed: "channels:GotoNextFeed",
    GotoPrevFeed: "channels:GotoPrevFeed",
    HomeFeedChanged: "channels:HomeFeedChanged",
    GetFeedH5Url: "channels:GetFeedH5Url",
    FeedProfileLoaded: "channels:OnFeedProfileLoaded",
    FeedCommentListLoaded: "channels:FeedCommentListLoaded",
    LiveProfileLoaded: "channels:OnLiveProfileLoaded",
    JoinLive: "channels:JoinLive",
    BeforeDownloadMedia: "channels:BeforeDownloadMedia",
    BeforeDownloadCover: "channels:BeforeDownloadCover",
    MediaDownloaded: "channels:MediaDownloaded",
    MP3Downloaded: "channels:MP3Downloaded",
    Feed: "channels:Feed",
  });

  Object.assign(WXU, {
    onAPILoaded: function (handler) {
      WXE.on(WXE.Events.APILoaded, handler);
      return function () {
        WXE.off(WXE.Events.APILoaded, handler);
      };
    },
    onUtilsLoaded: function (handler) {
      WXE.on(WXE.Events.UtilsLoaded, handler);
      return function () {
        WXE.off(WXE.Events.UtilsLoaded, handler);
      };
    },
    onInit: function (handler) {
      WXE.on(WXE.Events.Init, handler);
      return function () {
        WXE.off(WXE.Events.Init, handler);
      };
    },
    /**
     * 首页获取到视频列表
     * @param {(feeds: ChannelsFeed[]) => void} handler
     */
    onPCFlowLoaded: function (handler) {
      WXE.on(WXE.Events.PCFlowLoaded, handler);
      return function () {
        WXE.off(WXE.Events.PCFlowLoaded, handler);
      };
    },
    /**
     * 首页推荐 切换到下一个视频
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onGotoNextFeed: function (handler) {
      WXE.on(WXE.Events.GotoNextFeed, handler);
      return function () {
        WXE.off(WXE.Events.GotoNextFeed, handler);
      };
    },
    /**
     * 首页推荐 切换到上一个视频
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onGotoPrevFeed: function (handler) {
      WXE.on(WXE.Events.GotoPrevFeed, handler);
      return function () {
        WXE.off(WXE.Events.GotoPrevFeed, handler);
      };
    },
    /**
     * 首页推荐 切换到指定视频
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onHomeFeedChanged: function (handler) {
      WXE.on(WXE.Events.HomeFeedChanged, handler);
      return function () {
        WXE.off(WXE.Events.HomeFeedChanged, handler);
      };
    },
    /**
     * 获取到推荐列表
     * @param {(feeds: ChannelsFeed[]) => void} handler
     */
    onRecommendFeedsLoaded: function (handler) {
      WXE.on(WXE.Events.RecommendFeedsLoaded, handler);
      return function () {
        WXE.off(WXE.Events.RecommendFeedsLoaded, handler);
      };
    },
    onInteractionedFeedsLoaded: function (handler) {
      WXE.on(WXE.Events.InteractionedFeedsLoaded, handler);
      return function () {
        WXE.off(WXE.Events.InteractionedFeedsLoaded, handler);
      };
    },
    onGetFeedH5Url: function (handler) {
      WXE.on(WXE.Events.GetFeedH5Url, handler);
      return function () {
        WXE.off(WXE.Events.GetFeedH5Url, handler);
      };
    },
    onLiveUserFeedsLoaded: function (handler) {
      WXE.on(WXE.Events.LiveUserFeedsLoaded, handler);
      return function () {
        WXE.off(WXE.Events.LiveUserFeedsLoaded, handler);
      };
    },
    /**
     * 获取到指定用户的部分视频列表
     * @param {(feeds: ChannelsFeed[]) => void} handler
     */
    onUserFeedsLoaded: function (handler) {
      WXE.on(WXE.Events.UserFeedsLoaded, handler);
      return function () {
        WXE.off(WXE.Events.UserFeedsLoaded, handler);
      };
    },
    /**
     * 获取到视频详情
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onFetchFeedProfile: function (handler) {
      WXE.on(WXE.Events.FeedProfileLoaded, handler);
      return function () {
        WXE.off(WXE.Events.FeedProfileLoaded, handler);
      };
    },
    /**
     * 获取到视频评论列表
     * @param {(data: unknown) => void} handler
     */
    onFetchFeedCommentList: function (handler) {
      WXE.on(WXE.Events.FeedCommentListLoaded, handler);
      return function () {
        WXE.off(WXE.Events.FeedCommentListLoaded, handler);
      };
    },
    /**
     * 获取到直播详情
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onFetchLiveProfile: function (handler) {
      WXE.on(WXE.Events.LiveProfileLoaded, handler);
      return function () {
        WXE.off(WXE.Events.LiveProfileLoaded, handler);
      };
    },
    /**
     * 加入直播
     * @param {(data: JoinLivePayload) => void} handler
     */
    onJoinLive: function (handler) {
      WXE.on(WXE.Events.JoinLive, handler);
      return function () {
        WXE.off(WXE.Events.JoinLive, handler);
      };
    },
    /**
     * 下载视频前
     * @param {(media: FeedProfilePayload) => void} handler
     */
    beforeDownloadMedia: function (handler) {
      WXE.on(WXE.Events.BeforeDownloadMedia, handler);
      return function () {
        WXE.off(WXE.Events.BeforeDownloadMedia, handler);
      };
    },
    /**
     * 下载封面前
     * @param {(media: FeedProfilePayload) => void} handler
     */
    beforeDownloadCover: function (handler) {
      WXE.on(WXE.Events.BeforeDownloadCover, handler);
      return function () {
        WXE.off(WXE.Events.BeforeDownloadCover, handler);
      };
    },
    /**
     * 视频下载完成
     * @param {(media: FeedProfilePayload) => void} handler
     */
    onMediaDownloaded: function (handler) {
      WXE.on(WXE.Events.MediaDownloaded, handler);
      return function () {
        WXE.off(WXE.Events.MediaDownloaded, handler);
      };
    },
    /**
     * mp3 下载完成
     * @param {(media: FeedProfilePayload) => void} handler
     */
    onMP3Downloaded: function (handler) {
      WXE.on(WXE.Events.MP3Downloaded, handler);
      return function () {
        WXE.off(WXE.Events.MP3Downloaded, handler);
      };
    },
    /**
     * 加载了 feed。包括首页推荐、上一个下一个；视频详情页；直播详情页 都会触发该事件
     * 可用于记录访问过的视频
     * @param {(feed: ChannelsFeed) => void} handler
     */
    onFeed: function (handler) {
      WXE.on(WXE.Events.Feed, handler);
      return function () {
        WXE.off(WXE.Events.Feed, handler);
      };
    },
  });
})();
