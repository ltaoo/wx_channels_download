/**
 * Download task REST and WebSocket protocol declarations.
 *
 * All int/int64/float64 fields from Go are JSON numbers in the browser.
 */

/** Download task lifecycle status. */
type DownloadTaskStatus =
  | 0 // waiting
  | 1 // preparing / starting / resuming
  | 2 // downloading
  | 3 // paused
  | 4 // merging
  | 5 // finished
  | 6 // failed
  | 7; // cancelled

/** JSON object keys used by the list endpoint's status statistics. */
type DownloadTaskStatusKey = "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7";

type DownloadTaskRelationType =
  | "discovered"
  | "derived"
  | "dependency";

type DownloadTaskFileStatus =
  | "waiting"
  | "downloading"
  | "paused"
  | "finished"
  | "error"
  | "cancelled";

/** File data embedded in a list item or a full WebSocket task record. */
type DownloadTaskFileRecord = {
  id: number;
  /** Directory containing the file; this is not the complete file path. */
  download_dir: string;
  name: string;
  kind: string;
  type: string;
  status: DownloadTaskFileStatus;
  size: number;
  downloaded: number;
  speed: number;
  /** Percentage in the range 0-100. */
  progress: number;
  url: string;
  error: string;
};

/**
 * Complete task record returned by the list endpoint and full WS events.
 * Fields marked optional are omitted by Go when their value is empty/nil.
 */
type DownloadTaskRecord = {
  id: number;
  content_id?: string;
  content_type?: string;
  parent_task_id?: number;
  root_task_id: number;
  relation_type?: DownloadTaskRelationType;
  child_count: number;
  name: string;
  platform_id: string;
  status: DownloadTaskStatus;
  source_url: string;
  cover_url: string;
  /** The backend currently serializes cover dimensions as strings. */
  cover_width: string;
  cover_height: string;
  /** JSON encoded as a string, not an already parsed object. */
  config_json: string;
  /** JSON encoded as a string, not an already parsed object. */
  metadata_json: string;
  url: string;
  size: number;
  downloaded: number;
  speed: number;
  /** Percentage in the range 0-100. */
  progress: number;
  error: string;
  files: DownloadTaskFileRecord[];
  file_count: number;
  /** Unix timestamp in milliseconds. */
  created_at: number;
  /** Unix timestamp in milliseconds. */
  updated_at: number;
};

/** Status counts returned by the paginated list endpoint. */
type DownloadTaskListStatusStats = Partial<
  Record<DownloadTaskStatusKey, number>
>;

/** GET /api/v1/download_task/list without task_id. */
type DownloadTaskListPageData = {
  list: DownloadTaskRecord[];
  total: number;
  page: number;
  page_size: number;
  /** Counts grouped by the original numeric task status. */
  stats: DownloadTaskListStatusStats;
};

/**
 * Success data of GET /api/v1/download_task/list.
 * With task_id it is one record; otherwise it is a paginated result.
 */
type DownloadTaskListData = DownloadTaskRecord | DownloadTaskListPageData;

type DownloadTaskDetailAccount = {
  id: string;
  nickname: string;
  avatar_url: string;
  external_id: string;
};

type DownloadTaskDetailContent = {
  id: string;
  platform_id: string;
  type: string;
  title: string;
  description: string;
  cover_url: string;
  url: string;
  source_url: string;
  /** Unix timestamp; 0 means no publish time is stored. */
  publish_time: number;
  /** Go serializes the empty, nil account slice as null. */
  accounts: DownloadTaskDetailAccount[] | null;
};

type DownloadTaskDetailFileType =
  | "image"
  | "video"
  | "audio"
  | "html"
  | "zip"
  | "pdf"
  | "other";

/** File data enriched by the detail endpoint. */
type DownloadTaskDetailFile = DownloadTaskFileRecord & {
  local_path: string;
  file_type: DownloadTaskDetailFileType;
  file_url: string;
  exists: boolean;
};

/** Success data of GET /api/v1/download_task/detail?id={id}. */
type DownloadTaskDetailData = {
  id: number;
  content: DownloadTaskDetailContent | null;
  content_id: string | null;
  content_type: string;
  parent_task_id: number | null;
  root_task_id: number;
  relation_type: DownloadTaskRelationType | "";
  child_count: number;
  name: string;
  platform_id: string;
  status: DownloadTaskStatus;
  source_url: string;
  cover_url: string;
  cover_width: string;
  cover_height: string;
  config_json: string;
  metadata_json: string;
  url: string;
  size: number;
  downloaded: number;
  speed: number;
  /** Percentage in the range 0-100. */
  progress: number;
  error: string;
  files: DownloadTaskDetailFile[];
  file_count: number;
  /** Unix timestamp in milliseconds. */
  created_at: number;
  /** Unix timestamp in milliseconds. */
  updated_at: number;
};

type DownloadTaskAPISuccessResponse<T> = {
  code: 0;
  msg: string;
  data: T;
};

type DownloadTaskAPIErrorResponse = {
  code: number;
  msg: string;
  /** Only error responses produced with ErrWithData contain this field. */
  data?: unknown;
};

type DownloadTaskAPIResponse<T> =
  | DownloadTaskAPISuccessResponse<T>
  | DownloadTaskAPIErrorResponse;

type DownloadTaskListResponse = DownloadTaskAPIResponse<DownloadTaskListData>;
type DownloadTaskListPageResponse =
  DownloadTaskAPIResponse<DownloadTaskListPageData>;
type DownloadTaskListItemResponse = DownloadTaskAPIResponse<DownloadTaskRecord>;
type DownloadTaskDetailResponse = DownloadTaskAPIResponse<DownloadTaskDetailData>;

/** Mutable file fields sent by task_update. */
type DownloadTaskWSFileUpdate = {
  id: number;
  status: DownloadTaskFileStatus;
  size: number;
  downloaded: number;
  speed: number;
  /** Percentage in the range 0-100. */
  progress: number;
  error: string;
};

/**
 * Lightweight task patch used for progress and state changes such as start,
 * pause, resume and retry. It only applies to a record the frontend already has.
 */
type DownloadTaskWSUpdate = {
  id: number;
  status: DownloadTaskStatus;
  size: number;
  downloaded: number;
  speed: number;
  /** Percentage in the range 0-100. */
  progress: number;
  error: string;
  files?: DownloadTaskWSFileUpdate[];
  /** Omitted by in-memory progress updates. */
  updated_at?: number;
};

/** Named aggregate counts used by task_stats WS messages. */
type DownloadTaskWSStats = {
  total: number;
  downloading: number;
  paused: number;
  waiting: number;
  finished: number;
  error: number;
};

/** New records; tasks contain complete list-compatible data. */
type DownloadTaskWSCreateMessage = {
  type: "task_create";
  tasks: DownloadTaskRecord[];
};

/** Existing records replaced with complete list-compatible data. */
type DownloadTaskWSUpsertMessage = {
  type: "task_upsert";
  tasks: DownloadTaskRecord[];
};

/** Existing records patched with mutable progress/state data. */
type DownloadTaskWSUpdateMessage = {
  type: "task_update";
  updates: DownloadTaskWSUpdate[];
};

/** IDs of deleted records; the frontend removes them from its local list. */
type DownloadTaskWSDeleteMessage = {
  type: "task_delete";
  task_ids: number[];
};

type DownloadTaskWSStatsMessage = {
  type: "task_stats";
  stats: DownloadTaskWSStats;
};

/** Every server-to-client message emitted by /ws/v1/download_task. */
type DownloadTaskWSMessage =
  | DownloadTaskWSCreateMessage
  | DownloadTaskWSUpsertMessage
  | DownloadTaskWSUpdateMessage
  | DownloadTaskWSDeleteMessage
  | DownloadTaskWSStatsMessage;

/** Optional task filter in /ws/v1/download_task?task_id={task_id}. */
type DownloadTaskWSQuery = {
  task_id?: number;
};

/** Client-to-server message for changing the connection's task filter. */
type DownloadTaskWSSubscribeMessage = {
  type: "subscribe";
  /** Must be a positive integer. */
  task_id: number;
};

type DownloadTaskWSClientMessage = DownloadTaskWSSubscribeMessage;
