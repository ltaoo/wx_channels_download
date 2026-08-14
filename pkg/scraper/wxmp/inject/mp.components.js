/// <reference path="./mp.utils.js" />
/// <reference path="../../../../frontend/inject/virtual-list-view.js" />

(() => {
  if (!window.WXMPUtils) {
    throw new Error("mp.utils.js must be loaded before mp.components.js");
  }
  const { decode_html_text, decode_official_account_url } = window.WXMPUtils;

  function MsgListItem(props) {
    const item = props.item || {};
    const msg_info = item.app_msg_ext_info || {};
    const article_url = decode_official_account_url(msg_info.content_url);
    const title = decode_html_text(msg_info.title) || "无标题";
    const digest = decode_html_text(msg_info.digest);
    const publish_time = Number(item.comm_msg_info?.datetime) || 0;

    return View(
      {
        style: {
          padding: "10px 12px",
          "border-radius": "6px",
          background:
            "var(--popup-content-bg-color, var(--weui-BG-2, #f7f7f7))",
        },
      },
      [
        View(
          {
            style: {
              "font-size": "14px",
              "font-weight": "500",
              "margin-bottom": "4px",
              color: "var(--weui-FG-0)",
            },
          },
          [
            article_url
              ? View(
                  {
                    type: "a",
                    attributes: {
                      href: article_url,
                      target: "_blank",
                      rel: "noopener noreferrer",
                    },
                    style: {
                      color: "inherit",
                      "text-decoration": "none",
                    },
                  },
                  [title],
                )
              : title,
          ],
        ),
        Show({
          when: Boolean(digest),
          ok() {
            return View(
              {
                style: {
                  "font-size": "12px",
                  color: "var(--weui-FG-1, #888)",
                  "margin-bottom": "4px",
                },
              },
              [digest],
            );
          },
        }),
        Show({
          when: publish_time > 0,
          ok() {
            return View(
              {
                style: {
                  "font-size": "11px",
                  color: "var(--weui-FG-1, #aaa)",
                },
              },
              [
                Timeless.utils
                  .dayjs(publish_time * 1000)
                  .format("YYYY-MM-DD HH:mm"),
              ],
            );
          },
        }),
      ],
    );
  }

  function MsgListPanel(props) {
    const model = props.store;
    const load_more_disabled_ = combine(
      [model.state.loading, model.state.can_load_more],
      (loading, can_load_more) => loading || !can_load_more,
    );
    const load_more_text_ = combine(
      {
        can_load_more: model.state.can_load_more,
        error: model.state.error,
        loading: model.state.loading,
      },
      (state) => {
        if (state.loading) {
          return "加载中...";
        }
        if (state.error) {
          return "重试";
        }
        return state.can_load_more ? "加载更多" : "没有更多了";
      },
    );
    const initial_loading_ = combine(
      [model.state.loading, model.state.items],
      (loading, items) => loading && items.length === 0,
    );
    const initial_error_ = combine(
      [model.state.error, model.state.items],
      (error, items) => Boolean(error) && items.length === 0,
    );
    const has_items_ = computed(model.state.items, (items) => items.length > 0);
    const empty_ = combine(
      {
        error: model.state.error,
        items: model.state.items,
        loading: model.state.loading,
      },
      (state) => !state.loading && !state.error && state.items.length === 0,
    );

    function load_more_on_reach_bottom() {
      Promise.resolve(model.methods.loadMore()).catch((error) => {
        WXU.error({
          msg:
            "获取推送列表失败: " +
            (error && error.message ? error.message : String(error)),
          source: "mp.components.js:MsgListPanel.onReachBottom",
        });
      });
    }

    return View(
      {
        class: "wx-dl-panel-container",
        onMounted() {
          return model.state.error.subscribe({
            onChange(error) {
              if (error) {
                WXU.error({
                  msg: "获取推送列表失败: " + error,
                  source: "mp.components.js:MsgListPanel",
                });
              }
            },
          });
        },
      },
      [
        View({ class: "wx-dl-header" }, [
          View({ class: "wx-dl-heading" }, [
            View({ class: "wx-dl-title" }, ["推送列表"]),
          ]),
          View(
            {
              type: "button",
              class: "wx-dl-more-btn",
              attributes: {
                type: "button",
                "aria-label": "关闭推送列表",
              },
              style: {
                border: "none",
                background: "transparent",
                "line-height": "1",
              },
              onClick() {
                props.dialog$.hide();
              },
            },
            [Timeless.Icon({ name: "x", size: 18 })],
          ),
        ]),
        View(
          {
            style: {
              height: "380px",
              "max-height": "380px",
              "min-height": "0",
              overflow: "hidden",
            },
          },
          [
            Show({
              when: initial_loading_,
              ok() {
                return View(
                  {
                    style: {
                      display: "flex",
                      "align-items": "center",
                      "justify-content": "center",
                      gap: "8px",
                      height: "100%",
                      padding: "20px 12px",
                      color: "var(--weui-FG-1, #888)",
                      "font-size": "13px",
                    },
                  },
                  [View({ class: "weui-loading" }), "正在加载推送列表..."],
                );
              },
            }),
            Show({
              when: initial_error_,
              ok() {
                return View(
                  {
                    style: {
                      display: "flex",
                      "align-items": "center",
                      "justify-content": "center",
                      height: "100%",
                      padding: "20px 12px",
                      color: "var(--weui-RED, #fa5151)",
                      "font-size": "13px",
                    },
                  },
                  ["获取推送列表失败：", model.state.error],
                );
              },
            }),
            Show({
              when: has_items_,
              ok() {
                return VirtualListView({
                  class: "wx-dl-dark-scroll",
                  style: {
                    height: "100%",
                    "max-height": "100%",
                    overflow: "auto",
                    position: "relative",
                    padding: "0 12px",
                    "box-sizing": "border-box",
                    "background-color": "transparent",
                  },
                  key(item, index) {
                    const msg_info = item.app_msg_ext_info || {};
                    return (
                      msg_info.content_url ||
                      `${item.comm_msg_info?.datetime || 0}-${msg_info.title || ""}-${index}`
                    );
                  },
                  size: 6,
                  buffer: 4,
                  gutter: 8,
                  itemHeight: 82,
                  paddingBottom: 0,
                  each: model.state.items,
                  onReachBottom: load_more_on_reach_bottom,
                  render(item_) {
                    const item =
                      item_ && item_.value !== undefined ? item_.value : item_;
                    return MsgListItem({
                      item,
                    });
                  },
                });
              },
            }),
            Show({
              when: empty_,
              ok() {
                return View(
                  {
                    style: {
                      display: "flex",
                      "align-items": "center",
                      "justify-content": "center",
                      height: "100%",
                      padding: "20px 12px",
                      color: "var(--weui-FG-1, #888)",
                      "text-align": "center",
                      "font-size": "13px",
                    },
                  },
                  ["暂无推送"],
                );
              },
            }),
          ],
        ),
        Button(
          {
            disabled: load_more_disabled_,
            attributes: { type: "button" },
            style: {
              margin: "12px 12px 0",
              padding: "8px 16px",
              border: "1px solid var(--weui-FG-6, #eee)",
              "border-radius": "4px",
              background:
                "var(--popup-content-bg-color, var(--weui-BG-2, #f7f7f7))",
              color: "var(--weui-FG-0)",
              cursor: "pointer",
              width: "calc(100% - 24px)",
              "font-size": "13px",
              "flex-shrink": "0",
            },
            onClick() {
              model.methods.loadMore();
            },
          },
          [load_more_text_],
        ),
      ],
    );
  }

  window.MsgListPanel = MsgListPanel;
})();
