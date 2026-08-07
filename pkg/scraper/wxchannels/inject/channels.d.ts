type ChannelsConfig = {
  /** Download button defaults to original video */
  defaultHighest: boolean;
  /** Download filename template, without extension */
  downloadFilenameTemplate: string;
  /** Pause playback when downloading */
  downloadPauseWhenDownload: boolean;
  /** Download in frontend */
  downloadInFrontend: boolean;
  /** API server address */
  apiServerAddr: string;
  channelsHostname: string;
  downloadHostname: string;
  MaxRunning: number;
  downloadForceCheckAllFeeds: boolean;
};

type DropdownMenuItemPayload = {
  label: string;
  onClick: (event: { feed: FeedProfile; href: string }) => void;
};

/** Raw video metadata from Channels */
type ChannelsFeed = {
  id: string;
  description?: string;
  objectDesc: {
    /** 2=picture 4=video 9=live */
    mediaType: number;
    description: string;
    media: ChannelsMedia[];
    flowCardDesc?: {
      description: string;
    };
    finderNewlifeDesc?: {
      richTextTitle: string;
    };
    followPostInfo?: {
      musicInfo?: {
        docId?: string;
        docType?: number;
        name?: string;
        artist?: string;
        mediaStreamingUrl?: string;
      };
    };
  };
  objectNonceId: string;
  objectStatus: number;
  createtime: number;
  /** forward count */
  forwardCount: number;
  /** like count */
  likeCount: number;
  /** comment count */
  commentCount: number;
  favCount: number;
  /** publisher */
  contact: {
    username: string;
    headUrl: string;
    nickname: string;
    signature: string;
  };
  liveCover?: {
    imgUrl: string;
    imgUrlToken: string;
  };
  liveInfo?: {
    streamUrl: string;
  };
  anchorContact?: {
    username: string;
    nickname: string;
    headUrl: string;
    signature: string;
    liveCoverImgUrl: string;
  };
};

/** Raw media data from Channels */
type ChannelsMedia = {
  url: string;
  urlToken: string;
  coverUrl: string;
  thumbUrl?: string;
  fullThumbUrl?: string;
  fullUrl?: string;
  fileSize: number;
  decodeKey: number;
  /** duration */
  videoPlayLen: number;
  width: number;
  height: number;
  spec: ChannelsMediaSpec[];
};

type ChannelsMediaSpec = {
  /** spec value */
  fileFormat: string;
};

type ChannelsBgm = {
  url: string;
  filename: string;
  name: string;
  artist: string;
  doc_id: string;
  doc_type: number;
};

/**
 * Extracted and normalized feed data.
 * This is the type returned by WXU.check_profile_existing.
 */
type FeedProfile = {
  type: "media" | "picture" | "live";
  id: string;
  nonce_id: string;
  /** title */
  title: string;
  /** download URL */
  url: string;
  key: number;
  /** cover image URL */
  cover_url: string;
  /** video publish time */
  createtime: number;
  /** file size */
  size?: number;
  /** video duration */
  duration?: number;
  /** image list, only present for pictures type */
  files?: { url: string; urlToken?: string }[];
  /** background music, may be present for picture posts */
  bgm?: ChannelsBgm | null;
  /** spec list, only present for media type */
  spec?: ChannelsMediaSpec[];
  /** publisher */
  contact: {
    id: string;
    avatar_url: string;
    nickname: string;
  };
};

/**
 * FeedProfile with additional download-related fields.
 */
type FeedProfilePayload = FeedProfile & {
  /** filename */
  filename: string;
  /** original URL */
  original_url: string;
  /** download URL with spec suffix appended */
  url: string;
  /** target spec */
  target_spec?: ChannelsMediaSpec;
  /** source URL */
  source_url: string;
  /** buffered video content (for downloading current video) */
  data?: ArrayBuffer;
  mp3: boolean;
};

type ChannelsAPIResp = {
  errCode: number;
  errMsg: string;
  data: {
    err: {
      base_resp: {
        err_msg: string;
        ret: number;
      };
      err_msg: string;
      jsapi_resp: {
        error_msg: string;
        resp_json: null;
        ret: number;
      };
    };
  };
};

type SharedFeedProfileResp = {
  data: {
    authorInfo: {
      nickname: string;
      headImgUrl: string;
      authIconUrl: string;
    };
    feedInfo: {
      picInfo: unknown[];
      description: string;
      favCountFmt: string;
      likeCountFmt: string;
      forwardCountFmt: string;
      commentCountFmt: string;
      createtime: number;
      isHardAd: boolean;
      coverUrl: string;
    };
    errMsg: {
      type: number;
    };
    sceneInfo: {
      dynamicExportId: string;
      commentScene: number;
      expiredTime: number;
      requestScene: number;
      entryScene: number;
      entryCardType: number;
    };
  };
  errCode: number;
  errMsg: string;
};
