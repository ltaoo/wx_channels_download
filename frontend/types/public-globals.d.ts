type VirtualListReactive<T> =
  | T
  | import("@timeless/timeless").Ref<T>
  | import("@timeless/timeless").DerivedRef<T>;

type VirtualListItems<T> =
  | T[]
  | import("@timeless/timeless").Ref<T[]>
  | import("@timeless/timeless").DerivedRef<T[]>
  | import("@timeless/timeless").TimelessRefArray<T>;

interface VirtualListScrollEvent {
  target: HTMLElement;
  scrollTop: number;
  clientHeight: number;
  scrollHeight: number;
}

interface VirtualListItemResizeEvent<T> {
  target: HTMLElement;
  index: number;
  item: T;
  key: PropertyKey;
  height: number;
  previousHeight: number;
}

type VirtualListViewProps<T = any> = Omit<
  import("@timeless/timeless").ViewProps,
  "key"
> & {
    each: VirtualListItems<T>;
    render: (
      item: import("@timeless/timeless").Ref<T>,
      index: import("@timeless/timeless").Ref<number>,
    ) => unknown;
    key?: keyof T | ((item: T, index: number) => PropertyKey);
    size?: VirtualListReactive<number>;
    buffer?: VirtualListReactive<number>;
    gutter?: VirtualListReactive<number>;
    itemHeight?:
      | VirtualListReactive<number | string>
      | ((item: T, index: number) => number);
    externalScroll?: VirtualListReactive<boolean>;
    scrollTop?: VirtualListReactive<number | string>;
    viewportHeight?: VirtualListReactive<number | string>;
    paddingBottom?: VirtualListReactive<number | string>;
    onScrollTopAdjust?: (delta: number) => void;
    onItemResize?: (event: VirtualListItemResizeEvent<T>) => void;
    onScroll?: (event: VirtualListScrollEvent) => void;
    onReachBottom?: (event: VirtualListScrollEvent) => void;
  };

declare function VirtualListView<T = any>(
  props: VirtualListViewProps<T>,
): ReturnType<typeof View>;

interface DLUtilsNotificationOptions {
  msg?: string;
  message?: string;
  source?: string;
  duration?: number;
  [key: string]: unknown;
}

interface DLUtilsGlobal {
  error(options: string | Error | DLUtilsNotificationOptions): unknown;
  warning(options: string | Error | DLUtilsNotificationOptions): unknown;
  toast(message: string | DLUtilsNotificationOptions): unknown;
  parseJSON<T = unknown>(value: string): [Error | null, T | null];
  log: any;
}

declare const DLUtils: DLUtilsGlobal;

interface DLRef<T> {
  readonly value: T;
  as(value: T | ((current: T) => T)): void;
  subscribe(listener: { onChange(value: T): void }): () => void;
}

interface DLRefArray<T> extends DLRef<T[]> {
  readonly length: number;
}

interface DownloadProgress {
  percent: number;
  downloaded: number;
  total: number;
  speed: number;
}

interface DownloadTaskFile extends Record<string, unknown> {
  output_path: string;
}

interface DownloadTaskSnapshot {
  id: string | number | null;
  status: string;
  title: string;
  name: string;
  filepath: string;
  progress: DownloadProgress;
  error: Error | null;
  files: DownloadTaskFile[];
  raw: Record<string, unknown>;
}

interface DownloadTaskState {
  id: DLRef<string | number | null>;
  status: DLRef<string>;
  title: DLRef<string>;
  name: DLRef<string>;
  filepath: DLRef<string>;
  progress: DLRef<DownloadProgress>;
  error: DLRef<Error | null>;
  raw: DLRef<Record<string, unknown>>;
}

interface DownloadTaskMethods {
  onSuccess(listener: (task: DownloadTaskModelInstance) => void): () => void;
  onFail(
    listener: (event: {
      error: Error;
      task: DownloadTaskModelInstance;
    }) => void,
  ): () => void;
  onFailed(listener: (error: Error) => void): () => void;
  onProgress(
    listener: (event: {
      task: DownloadTaskModelInstance;
      progress: DownloadProgress;
      previous: DownloadProgress;
    }) => void,
  ): () => void;
  onChange(listener: (event: Record<string, unknown>) => void): () => void;
  start(): Promise<DownloadTaskModelInstance>;
  resume(): Promise<DownloadTaskModelInstance>;
  pause(): Promise<DownloadTaskModelInstance>;
  retry(): Promise<DownloadTaskModelInstance>;
  open(): Promise<unknown>;
  delete(options?: {
    delete_files?: boolean;
    deleteFiles?: boolean;
  }): Promise<DownloadTaskModelInstance | null>;
  snapshot(): DownloadTaskSnapshot;
}

interface DownloadTaskModelInstance
  extends DownloadTaskState,
    DownloadTaskMethods {
  state: DownloadTaskState;
  ui: Record<string, unknown>;
  reqs: Record<string, unknown>;
  methods: DownloadTaskMethods;
  handler: Record<string, (...args: any[]) => any>;
  readonly ready: Promise<DownloadTaskModelInstance>;
  readonly finished: Promise<DownloadTaskModelInstance>;
  readonly files: DownloadTaskFile[];
}

type DownloadTaskTarget =
  | string
  | number
  | DownloadTaskModelInstance
  | { id?: string | number; task_id?: string | number };

interface DownloaderState {
  task_list: DLRefArray<DownloadTaskModelInstance>;
  tasks: DLRefArray<DownloadTaskModelInstance>;
  websocket_connected: DLRef<boolean>;
  websocket_connecting: DLRef<boolean>;
  last_error: DLRef<Error | null>;
  websocket_url: string;
}

interface DownloaderMethods {
  create(
    input: unknown,
    options?: {
      platform?: string;
      skip?: boolean;
      existing_action?: "skip" | "duplicate" | "overwrite";
      build_from_fetch?: boolean;
      resource_indexes?: number[];
      download_dir?: string;
      filename?: string;
      auto_start?: boolean;
      parent_task_id?: number;
      relation_type?: string;
      config?: Record<string, unknown>;
    },
  ): Promise<DownloadTaskModelInstance>;
  prepare(input: string | Record<string, unknown>): Promise<Record<string, unknown>>;
  list(options?: Record<string, unknown>): Promise<
    DLRefArray<DownloadTaskModelInstance>
  >;
  refresh(options?: Record<string, unknown>): Promise<
    DLRefArray<DownloadTaskModelInstance>
  >;
  get(target: DownloadTaskTarget): DownloadTaskModelInstance | null;
  delete(
    target: DownloadTaskTarget,
    options?: { delete_files?: boolean; deleteFiles?: boolean },
  ): Promise<DownloadTaskModelInstance | null>;
  start(target: DownloadTaskTarget): Promise<DownloadTaskModelInstance>;
  resume(target: DownloadTaskTarget): Promise<DownloadTaskModelInstance>;
  continue(target: DownloadTaskTarget): Promise<DownloadTaskModelInstance>;
  pause(target: DownloadTaskTarget): Promise<DownloadTaskModelInstance>;
  retry(target: DownloadTaskTarget): Promise<DownloadTaskModelInstance>;
  startAll(options?: Record<string, unknown>): Promise<
    DLRefArray<DownloadTaskModelInstance>
  >;
  pauseAll(options?: Record<string, unknown>): Promise<
    DLRefArray<DownloadTaskModelInstance>
  >;
  clear(options?: {
    delete_files?: boolean;
    deleteFiles?: boolean;
  }): Promise<DLRefArray<DownloadTaskModelInstance>>;
  open(target: DownloadTaskTarget): Promise<unknown>;
  connect(): Promise<boolean>;
  reconnect(): Promise<boolean>;
  disconnect(): Promise<boolean>;
  ready(): Promise<{
    tasks: DLRefArray<DownloadTaskModelInstance>;
    connected: boolean;
    results: PromiseSettledResult<unknown>[];
  }>;
  destroy(): void;
}

interface DownloaderModelInstance extends DownloaderState, DownloaderMethods {
  state: DownloaderState;
  ui: Record<string, unknown>;
  reqs: Record<string, unknown>;
  methods: DownloaderMethods;
  handler: Record<string, (...args: any[]) => any>;
  channel: unknown;
  socket_client: unknown;
  requests: Record<string, unknown>;
}

interface DLProps {
  client: InstanceType<typeof Timeless.kit.HttpClientCore>;
  socket_client: InstanceType<typeof Timeless.kit.SocketClientCore>;
  debug?: boolean;
  reconnect?: boolean;
  reconnect_interval?: number;
  auto_start?: boolean;
}

interface DLFactory {
  (props: DLProps): DownloaderModelInstance;
  DownloadTaskModel: (
    props: Record<string, unknown>,
  ) => DownloadTaskModelInstance;
  DownloaderModel: (props: DLProps) => DownloaderModelInstance;
  TaskStatus: Readonly<Record<number, string>>;
}

declare const DL: DLFactory;
declare const DownloadTaskModel: DLFactory["DownloadTaskModel"];
declare const DownloaderModel: DLFactory["DownloaderModel"];
declare const dl$: DownloaderModelInstance;

interface Window {
  Timeless: typeof Timeless;
  VirtualListView: typeof VirtualListView;
  DLUtils: DLUtilsGlobal;
  DL: DLFactory;
  DownloadTaskModel: DLFactory["DownloadTaskModel"];
  DownloaderModel: DLFactory["DownloaderModel"];
  dl$: DownloaderModelInstance;
}
