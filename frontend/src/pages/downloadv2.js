import { format_time } from "@/utils.js";
import {
  BatchOverwriteConfirmDialog,
  ClearTasksConfirmDialog,
  CreatePlatformTaskDialog,
  CreatePlatformTaskPreviewDialog,
  CreateTaskDialog,
  CreateTaskPreviewDialog,
  DownloadV2SelectionBar,
  DownloadV2StatusBar,
  DownloadV2TaskActions,
  DownloadV2TaskMain,
  DownloadV2TaskSkeletonRow,
  OverwriteConfirmDialog,
  SingleOverwriteConfirmDialog,
  TaskDeleteConfirmDialog,
} from "./downloadv2.components.js";
import { DownloadV2Model } from "./downloadv2.model.js";
import PreviewPageView from "./preview.js";

function DownloadV2TaskTable(props) {
  const vm$ = props.store;

  return Table({
    name: "download-task-table",
    containerClass: "min-w-0 flex-auto p-4 group-content-main ",
    containerAttributes: { n: "download-page-main" },
    panelClass: "w-full",
    panelAttributes: { n: "download-task-list-panel" },
    headerClass: "max-760:grid-cols-32px-main-136px-92px max-520:grid-cols-32px-main-124px-78px",
    headerCellClass: "overflow-hidden whitespace-nowrap text-ellipsis",
    listClass: "flex relative min-h-0 flex-1 flex-col virtual-list-stretch ",
    columns: props.columns,
    rows: vm$.state.tasks,
    pagination: {
      class: " dm-px-4",
      summary: vm$.state.range_text,
      page: vm$.state.page,
      pageCount: vm$.state.page_count,
      pageSize: vm$.state.page_size,
      loading: vm$.state.loading,
      onChange(page) {
        return vm$.methods.changePage(page);
      },
    },
    rowKey(task$) {
      return task$.state.id.value;
    },
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    rowClass: "w-full max-w-none max-760:grid-cols-32px-main-136px-92px max-520:grid-cols-32px-main-124px-78px hover:bg-color-bg-subtle",
    skeletonCount: 8,
    renderSkeletonRow: DownloadV2TaskSkeletonRow,
    rowSelection: {
      width: 48,
      headerState: vm$.state.loaded_task_selection,
      allAriaLabel: "全选下载任务",
      itemAriaLabel: "选择下载任务",
      itemState(task) {
        return vm$.methods.taskSelectionState(task);
      },
      onSelectAll() {
        vm$.methods.toggleLoadedTasksSelected();
      },
      onSelect(task, event) {
        vm$.methods.toggleTaskSelected(task, {
          shiftKey: Boolean(event && event.shiftKey),
        });
      },
    },
    errorTitle: "下载任务加载失败",
    retry: {
      store: vm$.ui.btn_refresh_tasks$,
    },
    emptyTitle: "暂无下载任务",
  });
}

function DownloadV2TaskPreviewDrawer(props) {
  const vm$ = props.store;
  return Drawer(
    {
      store: vm$.ui.taskPreviewDrawer$,
      class: "dm-drawer--wide",
      attributes: { n: "download-task-preview-drawer" },
    },
    () => [
      PreviewPageView({
        tip: vm$.methods.tip,
        app: props.app,
        client: props.client,
        embedded: true,
        fileView: "gallery",
        hlsPlayer: props.hlsPlayer,
        taskId: vm$.state.preview_task_id,
      }),
    ],
  );
}

function DownloadV2Page(props) {
  const page_props = props || {};
  const vm$ = DownloadV2Model({
    ...page_props,
    downloader: window.dl$,
  });
  const task_columns = [
    {
      name: "task",
      title: "下载任务",
      cellClass: "flex min-w-0 items-center",
      render(task$) {
        return DownloadV2TaskMain({
          store: vm$,
          task: task$,
        });
      },
    },
    {
      name: "created-at",
      title: "下载时间",
      width: 160,
      cellClass: "overflow-hidden text-sm font-variant-numeric-tabular-nums whitespace-nowrap text-ellipsis text-color-text-secondary",
      cellAttributes(task$) {
        return {
          title: computed(task$.state.raw, (raw) =>
            format_time(raw && raw.created_at),
          ),
        };
      },
      render(task$) {
        return computed(task$.state.raw, (raw) =>
          format_time(raw && raw.created_at),
        );
      },
    },
    {
      name: "actions",
      title: "操作",
      width: 132,
      cellClass: "flex items-center justify-end gap-1.5 max-760:gap-1",
      render(task$) {
        return DownloadV2TaskActions({
          store: vm$,
          task: task$,
        });
      },
    },
  ];

  return View(
    {
      class: "flex relative h-100vh min-h-0 min-w-0 flex-col overflow-hidden font-sans bg-color-bg-canvas text-color-text-primary group-content-page paginated-page",
      attributes: { n: "download-page" },
      onMounted() {
        vm$.methods.ready();
      },
      onUnmounted() {
        vm$.methods.clean();
      },
    },
    [
      DownloadV2StatusBar({ store: vm$ }),
      DownloadV2TaskTable({ store: vm$, columns: task_columns }),
      DownloadV2SelectionBar({ store: vm$ }),
      DownloadV2TaskPreviewDrawer({
        store: vm$,
        app: page_props.app,
        client: page_props.client,
        hlsPlayer: page_props.hlsPlayer,
      }),
      CreateTaskDialog({ store: vm$ }),
      CreatePlatformTaskDialog({ store: vm$ }),
      CreateTaskPreviewDialog({ store: vm$ }),
      CreatePlatformTaskPreviewDialog({ store: vm$ }),
      TaskDeleteConfirmDialog({ store: vm$ }),
      ClearTasksConfirmDialog({ store: vm$ }),
      OverwriteConfirmDialog({ store: vm$ }),
      SingleOverwriteConfirmDialog({ store: vm$ }),
      BatchOverwriteConfirmDialog({ store: vm$ }),
    ],
  );
}

export default DownloadV2Page;
