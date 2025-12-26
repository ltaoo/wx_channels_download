type LogMsg = {
  /** 消息内容 */
  msg: string;
  /** 日志前缀，默认是 [FRONTEND] */
  prefix?: string;
  ignore_prefix?: 1;
  replace?: 1;
  end?: 1;
};
type ErrorMsg = {
  /** 是否同时调用 alert */
  alert?: 1;
  /** 错误消息内容 */
  msg: string;
};
type ChannelsConfig = {
  /** 下载按钮默认下载原始视频 */
  defaultHighest: boolean;
  /** 下载文件名的模板，不带后缀 */
  downloadFilenameTemplate: string;
  /** 下载时暂停播放 */
  downloadPauseWhenDownload: boolean;
  /** 在前端下载 */
  downloadInFrontend: boolean;
  /** API 服务地址 */
  apiServerAddr: string;
};
type DropdownMenuItemPayload = {
  label: string;
  onClick: (event: { feed: FeedProfile; href: string }) => void;
};

/** 视频号原始的视频数据 */
type ChannelsFeed = {
  id: string;
  objectDesc: {
    /** 4视频 9直播 */
    mediaType: number;
    description: string;
    media: ChannelsMedia[];
  };
  objectNonceId: string;
  objectStatus: number;
  createtime: number;
  /** 转发数 */
  forwardCount: number;
  /** 点赞数 */
  likeCount: number;
  /** 评论数 */
  commentCount: number;
  favCount: number;
  /** 发布者 */
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
/** 视频号原始的 media */
type ChannelsMedia = {
  url: string;
  coverUrl: string;
  fileSize: number;
  decodeKey: string;
  /** 时长 */
  videoPlayLen: number;
  width: number;
  height: number;
  spec: ChannelsMediaSpec[];
};
type ChannelsMediaSpec = {
  /** 规格值 */
  fileFormat: string;
};
/**
 * 对原始 feed 做了一些提取后的
 * 调用 WXU.check_profile_existing 获取到的就是这个类型的数据
 */
type FeedProfile = {
  type: "media" | "picture" | "live";
  id: number;
  nonce_id: string;
  /** 标题 */
  title: string;
  /** 下载地址 */
  url: string;
  key: number;
  /** 封面地址 */
  cover_url: string;
  /** 视频发布时间 */
  createtime: number;
  /** 文件大小 */
  size?: number;
  /** 视频时长 */
  duration?: number;
  /** 图片列表，类型为 pictures 才有 */
  files?: { url: string }[];
  /** 规格列表，类型为 media 才有 */
  spec?: ChannelsMediaSpec[];
  /** 发布者 */
  contact: {
    id: string;
    avatar_url: string;
    nickname: string;
  };
};

/**
 * 对 FeedProfile 又增加了用于下载的一些字段
 */
type FeedProfilePayload = FeedProfile & {
  /** 文件名 */
  filename: string;
  /** 原始 URL */
  original_url: string;
  /** 添加了 规格 后缀的视频下载地址 */
  url: string;
  /** 目标规格 */
  target_spec?: ChannelsMediaSpec;
  /** 源 URL */
  source_url: string;
  /** 已播放的视频内容（用于下载当前视频） */
  data?: ArrayBuffer;
  mp3: boolean;
};
