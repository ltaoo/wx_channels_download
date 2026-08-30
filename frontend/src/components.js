const Runtime = window.Timeless;

if (!Runtime) {
  throw new Error("组件库无法启动：Timeless 运行时未加载");
}

const { Show, View } = Runtime;

export function LoadingView() {
  return View(
    {
      class:
        "route-loading page dm-grid dm-place-center dm-text-muted dm-p-8",
      attributes: {
        n: "route-loading-placeholder",
        role: "status",
        "aria-label": "页面加载中",
      },
    },
    [
      View({
        class: "dm-loading-logo",
        attributes: {
          n: "route-loading-logo",
          "aria-hidden": "true",
        },
      }),
    ],
  );
}

/**
 * @param {Error} error
 * @param {string} view_name
 */
export function ErrorFallbackView(error, view_name) {
  return View(
    {
      class: "route-error page dm-grid dm-place-center dm-p-8",
      attributes: { role: "alert" },
    },
    [
      View({ class: "route-error-card" }, [
        View(
          {
            class: "route-error-card__icon",
            attributes: { "aria-hidden": "true" },
          },
          [Runtime.Icon({ name: "circle-alert", size: 24 })],
        ),
        View({ class: "route-error-card__content" }, [
          View({ as: "strong", class: "route-error-card__title" }, [
            "页面加载失败",
          ]),
          View({ as: "span", class: "route-error-card__context" }, [
            view_name || "未知页面",
          ]),
        ]),
        View({ as: "pre", class: "route-error-card__detail" }, [
          error.message,
        ]),
      ]),
    ],
  );
}

export function TablePlatformBadge(props = {}) {
  const semantic_name = props.name || "table-platform";
  const label =
    props.label === undefined || props.label === null
      ? []
      : Array.isArray(props.label)
        ? props.label
        : [props.label];
  return View(
    {
      class: Runtime.classNames([
        "dm-platform-badge dm-inline-flex dm-items-center dm-gap-1-5",
        props.class,
      ]),
      attributes: {
        n: semantic_name,
        ...(props.attributes || {}),
      },
    },
    [
      Show({
        when: props.favicon,
        ok() {
          return Runtime.Img({
            class: "dm-platform-badge__icon",
            src: props.favicon,
            alt: "",
            attributes: {
              n: `${semantic_name}-icon`,
              loading: "lazy",
              referrerpolicy: "no-referrer",
            },
            onError(event) {
              event.target.style.display = "none";
            },
          });
        },
      }),
      ...label,
    ],
  );
}
