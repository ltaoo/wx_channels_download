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
    containerClass: "group-content-main flex-[1_1_auto] min-w-0 p-4 ",
    containerAttributes: { n: "download-page-main" },
    panelClass: "w-full",
    panelAttributes: { n: "download-task-list-panel" },
    headerClass: "max-760:grid-cols-[32px_minmax(0,_1fr)_136px_92px] max-520:grid-cols-[32px_minmax(0,_1fr)_124px_78px]",
    headerCellClass: "overflow-hidden text-ellipsis whitespace-nowrap",
    listClass: "flex flex-1 flex-col min-h-0 relative [&_[data-list-view-root]]:h-full [&_[data-list-view-root]]:max-h-full [&_[data-list-view-root]]:flex-1 [&_[data-list-view-root]]:min-h-0 [&_[data-list-view-root]]:overflow-auto [&_[data-list-view-content]]:w-full [&_[data-list-view-content]]:box-sizing-border-box [&_[data-list-view-viewport]]:w-full [&_[data-list-view-viewport]]:box-sizing-border-box [&_[data-list-view-item]]:w-full [&_[data-list-view-item]]:box-sizing-border-box [&_[data-list-view-viewport]]:flex [&_[data-list-view-viewport]]:flex-col [&_[data-list-view-viewport]]:gap-0 [&_[data-list-view-item]]:flex [&_[data-list-view-item]]:justify-stretch [&_[data-list-view-item]]:bg-color-bg-surface",
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
    rowClass: "w-full max-w-none hover:bg-color-bg-subtle max-760:grid-cols-[32px_minmax(0,_1fr)_136px_92px] max-520:grid-cols-[32px_minmax(0,_1fr)_124px_78px]",
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
      cellClass: "min-w-0 flex items-center",
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
      cellClass: "overflow-hidden text-color-text-secondary text-sm font-variant-numeric-tabular-nums text-ellipsis whitespace-nowrap",
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
      cellClass: "flex items-center justify-end gap-1-5 max-760:gap-1",
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
      class: "group-content-page flex flex-col [.group-content-page&]:h-full [.group-content-page&]:min-h-0 [.group-content-page&]:overflow-hidden [&_>_.group-content-main]:min-h-0 [&_>_.group-content-main]:flex [&_>_.group-content-main]:flex-col [&_>_.dm-pagination]:flex-[0_0_auto] relative h-100vh min-h-0 flex flex-col overflow-hidden bg-color-bg-canvas text-color-text-primary min-w-0 text-color-text-primary font-sans bg-color-bg-canvas",
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
