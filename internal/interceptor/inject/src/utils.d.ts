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
  downloadLocalServerEnabled: boolean;
  downloadLocalServerAddr: string;
};
type DropdownMenuItemPayload = {
  label: string;
  onClick: (profile: ChannelsMediaProfile) => void;
};

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
type ChannelsMediaProfile = {
  type: "media" | "picture" | "live";
  id: number;
  nonce_id: string;
  /** 标题 */
  title: string;
  /** 下载地址 */
  url: string;
  key: number;
  /** 图片列表，类型为 pictures 才有 */
  files: { url: string }[];
  cover_url: string;
  createtime: number;
  /** 规格列表，类型为 media 才有 */
  spec: ChannelsMediaSpec[];
};
