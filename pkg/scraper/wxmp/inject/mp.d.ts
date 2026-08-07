/** Official account message list response */
type OfficialAccountMsgListResp = {
  ret: number;
  errmsg: string;
  msg_count: number;
  can_msg_continue: number;
  general_msg_list: string;
  next_offset: number;
  video_count: number;
  use_video_tab: number;
  real_type: number;
  home_page_list: unknown[];
};
