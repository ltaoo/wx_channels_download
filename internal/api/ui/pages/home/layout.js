/** 首页布局 */
import { defaultRouteName } from "@/store/index.js";
import NotFoundPageView from "@/pages/notfound/index.js";

export default function HomeLayoutView(props) {
  const view = props.view;
  const subViews = refarr([]);
  const curSubView = refobj(view.curView);
  view.onCurViewChange((view) => {
    curSubView.as(view);
  });
  view.onSubViewAppended((v) => {
    subViews.push(v);
  });
  const sidemenu$ = Timeless.RouteMenusModel({
    route: props.view.curView ? props.view.curView.name : defaultRouteName,
    history: props.history,
    menus: [
      { title: "视频号", url: "root.home_layout.index" },
      { title: "下载列表", url: "root.home_layout.download" },
      { title: "设置", url: "root.home_layout.settings" },
    ],
  });
  const curMenu = ref(sidemenu$.curMenu);
  sidemenu$.onStateChange(() => {
    curMenu.as(sidemenu$.curMenu);
  });

  return Flex({ class: "layout_home w-full h-full" }, [
    View(
      {
        class:
          "sidebar-wrapper w-[72px] h-full flex flex-col items-center py-6 border-r border-[var(--weui-FG-3)] bg-[var(--weui-BG-0)]",
      },
      [
        // Logo
        View(
          {
            class:
              "w-10 h-10 rounded-xl bg-[var(--weui-GREEN)] text-white flex items-center justify-center font-bold text-lg mb-8 shadow-sm cursor-pointer hover:opacity-90 transition-opacity",
            onClick() {
              props.history.push("root.home_layout.index");
            },
          },
          [Txt("号")],
        ),

        // Navigation
        View({ class: "flex flex-col gap-2 items-center" }, [
          // 视频号
          View(
            {
              class: computed(curMenu, (c) => {
                return c === "root.home_layout.index"
                  ? "w-10 h-10 rounded-lg bg-[var(--weui-BG-COLOR-ACTIVE)] flex items-center justify-center text-[var(--weui-FG-0)] cursor-pointer transition-colors"
                  : "w-10 h-10 rounded-lg hover:bg-[var(--weui-BG-COLOR-ACTIVE)] flex items-center justify-center text-[var(--weui-FG-1)] cursor-pointer transition-colors";
              }),
              onClick() {
                props.history.push("root.home_layout.index");
              },
            },
            [
              Timeless.icons.SearchOutlined({
                style: "font-size: 22px",
              }),
            ],
          ),
          // 下载列表
          View(
            {
              class: computed(curMenu, (c) => {
                return c === "root.home_layout.download"
                  ? "w-10 h-10 rounded-lg bg-[var(--weui-BG-COLOR-ACTIVE)] flex items-center justify-center text-[var(--weui-FG-0)] cursor-pointer transition-colors"
                  : "w-10 h-10 rounded-lg hover:bg-[var(--weui-BG-COLOR-ACTIVE)] flex items-center justify-center text-[var(--weui-FG-1)] cursor-pointer transition-colors";
              }),
              onClick() {
                props.history.push("root.home_layout.download");
              },
            },
            [
              Timeless.icons.DownloadOutlined({
                style: "font-size: 22px",
              }),
            ],
          ),
        ]),

        // Spacer
        View({ class: "flex-1" }, []),

        // Status indicator
        (() => {
          const statusAvailable = ref(null);
          const statusMsg = ref("");
          const fetchStatus = () => {
            fetch("/api/status")
              .then((r) => r.json())
              .then((res) => {
                statusAvailable.as(res.data?.available ?? false);
                statusMsg.as(res.code === 0 ? "已连接" : res.msg || "未连接");
              })
              .catch(() => {
                statusAvailable.as(false);
                statusMsg.as("请求失败");
              });
          };
          fetchStatus();
          const timer = setInterval(fetchStatus, 30000);
          return View(
            {
              class: "flex flex-col items-center gap-1 cursor-pointer",
              title: computed(statusMsg, (m) => m),
              onClick: fetchStatus,
            },
            [
              View({
                class: computed(statusAvailable, (v) =>
                  v === null
                    ? "w-2.5 h-2.5 rounded-full bg-[var(--weui-FG-3)]"
                    : v
                      ? "w-2.5 h-2.5 rounded-full bg-[var(--weui-GREEN)]"
                      : "w-2.5 h-2.5 rounded-full bg-[var(--weui-RED)]",
                ),
              }),
              Txt({
                class: "text-[10px] text-[var(--weui-FG-2)]",
                text: computed(statusMsg, (m) => m || "检测中"),
              }),
            ],
          );
        })(),

        // Bottom: Settings
        View({ class: "flex flex-col gap-4 items-center mb-4" }, [
          View(
            {
              class: computed(curMenu, (c) => {
                return c === "root.home_layout.settings"
                  ? "w-10 h-10 rounded-lg bg-[var(--weui-BG-COLOR-ACTIVE)] flex items-center justify-center text-[var(--weui-FG-0)] cursor-pointer transition-colors"
                  : "w-10 h-10 rounded-lg hover:bg-[var(--weui-BG-COLOR-ACTIVE)] flex items-center justify-center text-[var(--weui-FG-1)] cursor-pointer transition-colors";
              }),
              onClick() {
                props.history.push("root.home_layout.settings");
              },
            },
            [
              Timeless.icons.BoltOutlined({
                style: "font-size: 22px",
              }),
            ],
          ),
        ]),
      ],
    ),
    View({ class: "relative overflow-y-auto flex-1 w-0 h-full" }, [
      KeepAliveSubViews({
        ...props,
        NotFound: NotFoundPageView,
      }),
    ]),
  ]);
}
