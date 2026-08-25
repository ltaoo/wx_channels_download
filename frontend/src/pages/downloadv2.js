import {
  BatchOverwriteConfirmDialog,
  ClearTasksConfirmDialog,
  CreatePlatformTaskDialog,
  CreatePlatformTaskPreviewDialog,
  CreateTaskDialog,
  CreateTaskPreviewDialog,
  DownloadV2SelectionBar,
  DownloadV2StatusBar,
  DownloadV2TaskColumns,
  DownloadV2TaskSkeletonRow,
  OverwriteConfirmDialog,
  SingleOverwriteConfirmDialog,
  TaskDeleteConfirmDialog,
} from "./downloadv2.components.js";
import { DownloadV2Model } from "./downloadv2.model.js";
import PreviewPageView from "./preview.js";
import { Table } from "./table.js";

function DownloadV2TaskTable(props) {
  const vm$ = props.store;

  return Table({
    name: "download-task-table",
    containerClass: "wx-content-main dm-container",
    containerAttributes: { n: "download-page-main" },
    panelClass:
      "wx-content-rows wx-content-history-rows wx-dl-page-task-table dm-panel",
    panelAttributes: { n: "download-task-list-panel" },
    headerClass: "wx-dl-page-table-head",
    headerCellClass: "wx-dl-page-table-head-cell",
    listClass: "wx-content-history-list wx-dl-page-list wx-dl-dark-scroll",
    columns: DownloadV2TaskColumns({ store: vm$ }),
    rows: vm$.state.tasks,
    rowKey(task$) {
      return task$.state.id.value;
    },
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    rowClass: "wx-dl-page-task-row",
    skeletonCount: 8,
    renderSkeletonRow: DownloadV2TaskSkeletonRow,
    rowSelection: {
      headerState: vm$.state.loaded_task_selection,
      allAriaLabel: "全选下载任务",
      itemAriaLabel: "选择下载任务",
      size: 18,
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
      class: "wx-dl-preview-drawer",
      style: { width: "min(max(560px, 80vw), 100vw)" },
      attributes: { n: "download-task-preview-drawer" },
    },
    () => [
      PreviewPageView({
        app: props.app,
        client: props.client,
        embedded: true,
        fileView: "gallery",
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

  return View(
    {
      class:
        "wx-content-page wx-content-library-page wx-dl-page-root dm-page",
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
      DownloadV2TaskTable({ store: vm$ }),
      Show({
        when: computed(vm$.state.tasks, (tasks) => tasks.length > 0),
        ok() {
          return Pagination({
            summary: vm$.state.range_text,
            page: vm$.state.page,
            pageCount: vm$.state.page_count,
            loading: vm$.state.loading,
            onPrevious() {
              vm$.methods.previousPage();
            },
            onNext() {
              vm$.methods.nextPage();
            },
          });
        },
      }),
      DownloadV2SelectionBar({ store: vm$ }),
      DownloadV2TaskPreviewDrawer({
        store: vm$,
        app: page_props.app,
        client: page_props.client,
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
