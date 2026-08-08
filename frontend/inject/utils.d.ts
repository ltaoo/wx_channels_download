type LogMsg = {
  /** Message content */
  msg: string;
  /** Log prefix, defaults to [FRONTEND] */
  prefix?: string;
  ignore_prefix?: 1;
  replace?: 1;
  end?: 1;
};
type ErrorMsg = {
  /** Whether to also call alert */
  alert?: number;
  /** Error message content */
  msg: string;
  /** Source file and line of the error call */
  source?: string;
};