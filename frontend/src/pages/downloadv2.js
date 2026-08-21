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
  const list_height_style = vm$.state.fixed_list_height
    ? {
        height: `${vm$.state.list_height}px`,
        "max-height": `${vm$.state.list_height}px`,
      }
    : { "max-height": "100%" };

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
    virtualListStyle: {
      ...list_height_style,
      padding: "0",
    },
    columns: DownloadV2TaskColumns({ store: vm$ }),
    rows: vm$.state.tasks,
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    showHeaderWhenEmpty: true,
    rowClass: "wx-dl-page-task-row",
    rowKey: "id",
    size: 12,
    buffer: vm$.state.list_buffer,
    gutter: 0,
    itemHeight: vm$.state.list_item_height,
    paddingBottom: 0,
    renderEnabled: vm$.state.list_render_enabled,
    skeletonCount: 8,
    renderSkeletonRow: DownloadV2TaskSkeletonRow,
    rowSelection: {
      headerState: vm$.state.loaded_task_selection,
      allAriaLabel: "全选下载任务",
      itemAriaLabel: "选择下载任务",
      size: 18,
      enabled(task) {
        return !vm$.methods.isPlaceholderTask(task);
      },
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
    onListMounted(element) {
      vm$.methods.setListViewElement(element);
    },
    onListScroll(position) {
      vm$.methods.handleListViewScroll(position);
    },
    isPlaceholder(task) {
      return vm$.methods.isPlaceholderTask(task);
    },
    onPlaceholder(task) {
      vm$.methods.ensureTaskPageForIndex(task.__index);
    },
    renderPlaceholderRow() {
      return DownloadV2TaskSkeletonRow();
    },
    renderError(error) {
      return View(
        {
          class: "wx-content-state",
          attributes: { n: "download-task-table-error", role: "alert" },
        },
        [
          Timeless.Icon({ name: "circle-alert", size: 32 }),
          View(
            {
              class: "wx-content-state-title",
              attributes: { n: "download-task-table-error-title" },
            },
            ["下载任务加载失败"],
          ),
          View(
            {
              class: "wx-content-state-text",
              attributes: { n: "download-task-table-error-message" },
            },
            [error],
          ),
          View(
            {
              type: "button",
              class: "wx-dl-v2-action dm-focus-ring",
              attributes: {
                n: "download-task-table-retry",
                type: "button",
              },
              onClick() {
                vm$.methods.refreshTasks();
              },
            },
            ["重试"],
          ),
        ],
      );
    },
    renderEmpty() {
      return View(
        {
          class: "wx-dl-page-empty",
          attributes: { n: "download-task-table-empty" },
        },
        ["暂无下载任务"],
      );
    },
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
