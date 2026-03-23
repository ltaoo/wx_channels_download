/** 视频号搜索页 */
export default function HomePageView(props) {
  const keyword$ = new Timeless.ui.InputCore({
    placeholder: "输入视频号名称搜索...",
  });
  const contacts = refarr([]);
  const loading = ref(false);
  const errMsg = ref("");
  const hasSearched = ref(false);

  async function doSearch() {
    const kw = keyword$.value.trim();
    if (!kw) return;
    loading.as(true);
    errMsg.as("");
    contacts.as([]);
    hasSearched.as(true);
    try {
      const resp = await fetch(
        "/api/channels/contact/search?keyword=" + encodeURIComponent(kw),
      );
      const result = await resp.json();
      if (result.code === 0 && result.data) {
        const data = result.data;
        if (data.data && data.data.infoList) {
          contacts.as(data.data.infoList);
        }
        if (
          (!data.data ||
            !data.data.infoList ||
            data.data.infoList.length === 0) &&
          data.errMsg
        ) {
          errMsg.as(data.errMsg);
        }
      } else {
        errMsg.as(result.msg || "搜索失败");
      }
    } catch (e) {
      console.error("搜索失败:", e);
      errMsg.as("网络错误，请确认服务是否可用");
    }
    loading.as(false);
  }

  keyword$.onEnter(() => {
    doSearch();
  });

  // WebSocket 连接，接收实时事件
  let ws = null;
  function connectWS() {
    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    ws = new WebSocket(protocol + "//" + location.host + "/ws/channels");
    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data);
        if (
          msg.type === "api_call" &&
          msg.data &&
          msg.data.key === "key:channels:contact_list"
        ) {
          // 服务端通过 ws 请求前端搜索，这里转发给注入页面处理
          console.log("[WS] api_call", msg.data.key);
        }
      } catch (e) {
        // ignore
      }
    };
    ws.onclose = () => {
      setTimeout(connectWS, 3000);
    };
    ws.onerror = () => {
      ws.close();
    };
  }
  // connectWS();

  function renderContact(item) {
    const contact = item.contact || {};
    return View(
      {
        class:
          "flex items-center gap-4 p-4 rounded-lg bg-[var(--weui-BG-2)] border border-[var(--weui-FG-5)] hover:border-[var(--weui-GREEN)] cursor-pointer transition-colors",
        onClick() {
          // 跳转到博主视频列表（使用 username 查询）
          props.history.push("root.home_layout.index", {
            username: contact.username,
            nickname: contact.nickname,
          });
        },
      },
      [
        // Avatar
        contact.headUrl
          ? Img({
              src: contact.headUrl,
              class: "w-12 h-12 rounded-full object-cover flex-shrink-0",
            })
          : View(
              {
                class:
                  "w-12 h-12 rounded-full bg-[var(--weui-GREEN)] text-white flex items-center justify-center font-bold text-lg flex-shrink-0",
              },
              [Txt((contact.nickname || "?")[0])],
            ),
        // Info
        View(
          { class: "flex-1 min-w-0 space-y-1" },
          [
            View(
              { class: "text-sm font-medium text-[var(--weui-FG-0)] truncate" },
              [
                DangerouslyInnerHTML(
                  item.highlightNickname || contact.nickname || "",
                ),
              ],
            ),
            contact.signature
              ? View(
                  {
                    class: "text-xs text-[var(--weui-FG-1)] truncate",
                  },
                  [Txt(contact.signature)],
                )
              : null,
            item.highlightProfession
              ? View({ class: "text-xs text-[var(--weui-FG-2)] truncate" }, [
                  DangerouslyInnerHTML(item.highlightProfession),
                ])
              : null,
          ].filter(Boolean),
        ),
        // Arrow
        View({ class: "text-[var(--weui-FG-2)] flex-shrink-0" }, [Txt("›")]),
      ],
    );
  }

  return View({ class: "p-6 h-full overflow-y-auto" }, [
    View({ class: "max-w-3xl mx-auto space-y-6" }, [
      // Header
      View({ class: "space-y-2" }, [
        View({ class: "text-2xl font-bold text-[var(--weui-FG-0)]" }, [
          Txt("视频号"),
        ]),
        View({ class: "text-sm text-[var(--weui-FG-1)]" }, [
          Txt("搜索视频号博主，浏览和下载视频"),
        ]),
      ]),
      // Search
      View({ class: "flex gap-2" }, [
        View(
          {
            class:
              "flex-1 h-10 px-3 rounded-lg border border-[var(--weui-FG-3)] bg-[var(--weui-BG-2)] text-[var(--weui-FG-0)] text-sm flex items-center",
          },
          [
            Input({
              store: keyword$,
              class: "w-full bg-transparent outline-none",
            }),
          ],
        ),
        // Button(
        //   {
        //     store: new Timeless.ui.ButtonCore({
        //       onClick() {
        //         doSearch();
        //       },
        //     }),
        //     class: computed(loading, (l) =>
        //       l
        //         ? "px-4 h-10 rounded-lg bg-[var(--weui-GREEN)] text-white text-sm font-medium opacity-50 cursor-not-allowed"
        //         : "px-4 h-10 rounded-lg bg-[var(--weui-GREEN)] text-white text-sm font-medium hover:opacity-90 transition-opacity",
        //     ),
        //   },
        //   [
        //     Show({ when: loading }, [Txt("搜索中...")]),
        //     Show({ when: computed(loading, (l) => !l) }, [Txt("搜索")]),
        //   ],
        // ),
      ]),
      // Error message
      Show({ when: computed(errMsg, (m) => !!m) }, [
        View(
          {
            class:
              "p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-500 text-sm",
          },
          [
            // DynamicContent({ $content: computed(errMsg, (m) => Txt(m)) })
            computed(errMsg, (m) => Txt(m)),
          ],
        ),
      ]),
      // Results
      Show(
        {
          when: computed(contacts, (c) => c.length > 0),
        },
        [
          View({ class: "space-y-2" }, [
            View({ class: "text-sm text-[var(--weui-FG-1)] mb-2" }, [
              // DynamicContent({
              //   $content: computed(contacts, (c) =>
              //     Txt("找到 " + c.length + " 个结果"),
              //   ),
              // }),
              computed(contacts, (c) => Txt("找到 " + c.length + " 个结果")),
            ]),
            For({
              each: contacts,
              render: renderContact,
            }),
          ]),
        ],
      ),
      // Empty state
      Show(
        {
          when: Timeless.reactive.combine(
            { hasSearched, contacts, loading, errMsg },
            (t) =>
              !t.hasSearched ||
              (t.hasSearched &&
                t.contacts.length === 0 &&
                !t.loading &&
                !t.errMsg),
          ),
        },
        [
          View(
            {
              class:
                "flex flex-col items-center justify-center py-20 text-[var(--weui-FG-2)]",
            },
            [
              View({ class: "text-4xl mb-4 opacity-30" }, [Txt("🔍")]),
              View({ class: "text-sm" }, [
                Show({ when: computed(hasSearched, (s) => !s) }, [
                  Txt("搜索视频号博主开始下载"),
                ]),
                Show({ when: hasSearched }, [Txt("未找到相关视频号博主")]),
              ]),
            ],
          ),
        ],
      ),
    ]),
  ]);
}
