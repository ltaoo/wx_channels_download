import { ContentViewModel } from "./content.model.js";

function ContentPageView(props) {
  const vm$ = ContentViewModel(props);
  return View(
    {
      class:
        "wx-content-page wx-content-library-page wx-browse-history-page dm-page",
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      View({ class: "wx-content-toolbar-wrap" }, [
        ContentPageToolbar({ store: vm$ }),
      ]),
      ContentPageBody({ store: vm$ }),
      Show({
        when: computed(vm$.state.contents, (contents) => contents.length > 0),
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
    ],
  );
}

function ContentPageActionButton(props) {
  return Button(
    {
      store: props.store,
      class: [
        "wx-content-page-button",
        props.compact ? "wx-content-action-compact" : "",
        props.class,
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: (props.attributes && props.attributes.type) || "button",
        title: props.title || "",
        ...(props.attributes || {}),
      },
      onClick: props.onClick,
      prefix: props.icon
        ? Timeless.Icon({ name: props.icon, size: props.iconSize || 16 })
        : null,
    },
    props.label
      ? [View({ class: "wx-content-action-label" }, [props.label])]
      : [],
  );
}

function ContentPageToolbar(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "wx-content-toolbar wx-content-filter-form",
      attributes: { role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View({ class: "wx-content-filter-fields" }, [
        View({ class: "wx-content-search wx-content-filter-search" }, [
          Timeless.Icon({ name: "search", size: 16 }),
          Input({
            store: vm$.ui.input_keyword$,
            class: "wx-content-search-input",
            attributes: {
              name: "keyword",
              type: "search",
              autocomplete: "off",
              "aria-label": "搜索内容标题或描述",
            },
          }),
        ]),
        // Select({
        //   store: vm$.ui.select_scope$,
        //   class: "wx-content-scope-select wx-content-filter-select",
        //   attributes: { "aria-label": "筛选内容范围" },
        // }),
        // Select({
        //   store: vm$.ui.select_content_type$,
        //   class: "wx-content-type-select wx-content-filter-select",
        //   attributes: { "aria-label": "筛选内容类型" },
        // }),
      ]),
      View({ class: "wx-content-filter-actions" }, [
        ContentPageActionButton({
          store: vm$.ui.btn_search$,
          icon: "search",
          label: "搜索",
          variant: "primary",
          attributes: { type: "submit" },
          onClick(event) {
            event.preventDefault();
            vm$.methods.search();
          },
        }),
        ContentPageActionButton({
          store: vm$.ui.btn_refresh$,
          icon: "rotate-ccw",
          label: "重置",
        }),
      ]),
    ],
  );
}

function ContentRowCover(props) {
  const content = props.content;
  const fallback = View(
    { class: "wx-content-row-cover wx-content-row-cover-fallback" },
    [Timeless.Icon({ name: "file", size: 18 })],
  );
  if (!content.cover_url) {
    return fallback;
  }
  return View({ class: "wx-content-row-cover-wrap" }, [
    fallback,
    LazyImg({
      class: "wx-content-row-cover",
      src: content.cover_url,
      alt: content.title,
      attributes: {
        referrerpolicy: "no-referrer",
      },
    }),
  ]);
}

function ContentRowAccounts(props) {
  const accounts = props.content.accounts || [];
  if (accounts.length === 0) {
    return View({ class: "wx-content-row-author" }, ["暂无关联账号"]);
  }
  return View({ class: "wx-content-row-author" }, [
    For({
      each: accounts,
      render(account_) {
        const account =
          account_ && account_.value !== undefined
            ? account_.value
            : account_;
        const name =
          account.nickname || account.alias || account.external_id || "未知";
        return View({ class: "wx-content-row-author-account" }, [
          Show({
            when: account.avatar_url,
            ok() {
              return Img({
                class: "wx-content-row-author-avatar",
                src: account.avatar_url,
                alt: name,
                attributes: {
                  loading: "lazy",
                  referrerpolicy: "no-referrer",
                },
                onError(event) {
                  event.target.style.display = "none";
                },
              });
            },
          }),
          View(
            {
              class: "wx-content-row-author-name",
              attributes: { title: name },
            },
            [name],
          ),
        ]);
      },
    }),
  ]);
}

function ContentRow(props) {
  const vm$ = props.store;
  const content = props.content;
  const favicon = window.PLATFORM_FAVICONS[content.platform_id] || "";
  const detail_href = vm$.methods.detailHref(content);
  return View(
    {
      class: ["wx-content-row", detail_href ? "wx-content-row-clickable" : ""]
        .filter(Boolean)
        .join(" "),
      attributes: detail_href ? { title: "查看内容详情" } : {},
      onClick() {
        vm$.methods.openDetail(content);
      },
    },
    [
      ContentRowCover({ content }),
      View({ class: "wx-content-row-main" }, [
        View(
          {
            class: "wx-content-row-title",
            attributes: { title: content.title },
          },
          [content.title],
        ),
        View({ class: "wx-content-row-badges" }, [
          View({ class: "wx-content-row-platform" }, [
            Show({
              when: favicon,
              ok() {
                return Img({
                  class: "wx-content-row-platform-icon",
                  src: favicon,
                  alt: "",
                  attributes: {
                    loading: "lazy",
                    referrerpolicy: "no-referrer",
                  },
                  onError(event) {
                    event.target.style.display = "none";
                  },
                });
              },
            }),
            vm$.methods.platformName(content),
          ]),
          View({ class: "wx-content-row-type" }, [
            vm$.methods.typeLabel(
              content.content_type,
              content.content_subtype,
            ),
          ]),
        ]),
      ]),
      ContentRowAccounts({ content }),
      View({ class: "wx-content-row-meta" }, [
        Timeless.Icon({ name: "clock3", size: 12 }),
        vm$.methods.formatTime(content.publish_time),
      ]),
      View({ class: "wx-content-row-visits" }, [
        vm$.methods.downloadStatus(content.download_tasks),
      ]),
    ],
  );
}

function ContentTableHead() {
  return View({ class: "wx-content-row wx-content-row-head" }, [
    View({ class: "wx-content-row-head-cell" }, ["封面"]),
    View({ class: "wx-content-row-head-cell" }, ["标题"]),
    View({ class: "wx-content-row-head-cell" }, ["账号"]),
    View({ class: "wx-content-row-head-cell" }, ["发布时间"]),
    View({ class: "wx-content-row-head-cell" }, ["下载状态"]),
  ]);
}

function ContentSkeletonRow() {
  return View({ class: "wx-content-row wx-content-skeleton-row" }, [
    View({ class: "wx-content-row-cover wx-content-skeleton" }),
    View({}, [
      View({ class: "wx-content-skeleton wx-content-skeleton-title" }),
      View({ class: "wx-content-skeleton wx-content-skeleton-tag" }),
    ]),
    View({ class: "wx-content-skeleton wx-content-skeleton-line" }),
    View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
    View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
  ]);
}

function ContentListView(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-history-list" }, [
    VirtualListView({
      style: {
        height: "100%",
        "max-height": "100%",
        overflow: "auto",
        position: "relative",
        "box-sizing": "border-box",
        "background-color": "transparent",
      },
      key: "id",
      size: 10,
      buffer: 6,
      gutter: 0,
      itemHeight: 72,
      each: vm$.state.contents,
      render(content_) {
        const content =
          content_ && content_.value !== undefined
            ? content_.value
            : content_;
        return ContentRow({ store: vm$, content });
      },
    }),
  ]);
}

function ContentPageBody(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-main dm-container" }, [
    Show({
      when: vm$.state.loading,
      ok() {
        return View(
          { class: "wx-content-rows dm-panel" },
          Array.from({ length: 8 }, () => ContentSkeletonRow()),
        );
      },
      else() {
        return Show({
          when: computed(vm$.state.error, (error) => Boolean(error)),
          ok() {
            return View({ class: "wx-content-state" }, [
              Timeless.Icon({ name: "circle-alert", size: 32 }),
              View({ class: "wx-content-state-title" }, ["内容加载失败"]),
              View({ class: "wx-content-state-text" }, [vm$.state.error]),
              ContentPageActionButton({
                store: vm$.ui.btn_retry$,
                icon: "refresh-cw",
                label: "重试",
                variant: "primary",
              }),
            ]);
          },
          else() {
            return Show({
              when: computed(
                vm$.state.contents,
                (contents) => contents.length > 0,
              ),
              ok() {
                return View(
                  {
                    class:
                      "wx-content-rows wx-content-history-rows dm-panel",
                  },
                  [ContentTableHead(), ContentListView({ store: vm$ })],
                );
              },
              else() {
                return View({ class: "wx-content-state" }, [
                  Timeless.Icon({ name: "inbox", size: 36 }),
                  View({ class: "wx-content-state-title" }, ["暂无内容"]),
                  View({ class: "wx-content-state-text" }, [
                    "当前筛选条件下没有内容",
                  ]),
                ]);
              },
            });
          },
        });
      },
    }),
  ]);
}

export default ContentPageView;
