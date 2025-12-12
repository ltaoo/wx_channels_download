var __wx_eventbus = mitt();
var ChannelsEvents = {
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
};
var ChannelsEventBus = {
  emit: __wx_eventbus.emit,
  onFetchFeedProfile(handler) {
    __wx_eventbus.on(ChannelsEvents.FeedProfileLoaded, handler);
    return () => {
      __wx_eventbus.off(ChannelsEvents.FeedProfileLoaded, handler);
    };
  },
  onFetchLiveProfile(handler) {
    __wx_eventbus.on(ChannelsEvents.LiveProfileLoaded, handler);
    return () => {
      __wx_eventbus.off(ChannelsEvents.LiveProfileLoaded, handler);
    };
  },
  beforeDownloadMedia(handler) {
    __wx_eventbus.on(ChannelsEvents.BeforeDownloadMedia, handler);
    return () => {
      __wx_eventbus.off(ChannelsEvents.BeforeDownloadMedia, handler);
    };
  },
  beforeDownloadCover(handler) {
    __wx_eventbus.on(ChannelsEvents.BeforeDownloadCover, handler);
    return () => {
      __wx_eventbus.off(ChannelsEvents.BeforeDownloadCover, handler);
    };
  },
  onMediaDownloaded(handler) {
    __wx_eventbus.on(ChannelsEvents.MediaDownloaded, handler);
    return () => {
      __wx_eventbus.off(ChannelsEvents.MediaDownloaded, handler);
    };
  },
  onMP3Downloaded(handler) {
    __wx_eventbus.on(ChannelsEvents.MP3Downloaded, handler);
    return () => {
      __wx_eventbus.off(ChannelsEvents.MP3Downloaded, handler);
    };
  },
};
