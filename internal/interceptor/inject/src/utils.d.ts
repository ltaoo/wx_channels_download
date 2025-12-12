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
type DropdownMenuItemPayload = {
  label: string;
  onClick: (profile: ChannelsMediaProfile) => void;
};
type ChannelsMediaSpec = {
  /** 规格值 */
  fileFormat: string;
};
type ChannelsMediaProfile = {
  id: number;
  /** 标题 */
  title: string;
  /** 下载地址 */
  url: string;
  /** 规格列表 */
  spec: ChannelsMediaSpec[];
  createtime: number;
};
