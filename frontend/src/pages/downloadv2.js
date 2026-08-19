import {
  BatchOverwriteConfirmDialog,
  ClearTasksConfirmDialog,
  CreatePlatformTaskDialog,
  CreatePlatformTaskPreviewDialog,
  CreateTaskDialog,
  CreateTaskPreviewDialog,
  DownloadV2SelectionBar,
  DownloadV2StatusBar,
  DownloadV2TaskTable,
  OverwriteConfirmDialog,
  SingleOverwriteConfirmDialog,
  TaskDeleteConfirmDialog,
} from "./downloadv2.components.js";
import { DownloadV2Model } from "./downloadv2.model.js";
import PreviewPageView from "./preview.js";

function DownloadV2TaskPreviewDrawer(props) {
  const vm$ = props.store;
  return Drawer(
    {
      store: vm$.ui.taskPreviewDrawer$,
      class: "wx-dl-preview-drawer",
      style: { width: "min(max(560px, 80vw), 100vw)" },
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
      class: "wx-dl-page-root dm-page dm-flex dm-flex-col dm-min-h-0",
      onMounted() {
        vm$.methods.ready();
      },
      onUnmounted() {
        vm$.methods.clean();
      },
    },
    [
      DownloadV2StatusBar({ store: vm$ }),
      View({ class: "wx-dl-page-main dm-container" }, [
        View({ class: "wx-dl-page-list-wrap dm-panel" }, [
          DownloadV2TaskTable({ store: vm$ }),
        ]),
      ]),
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
