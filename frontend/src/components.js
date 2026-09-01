const Runtime = window.Timeless;

if (!Runtime) {
  throw new Error("组件库无法启动：Timeless 运行时未加载");
}

const { Show, View } = Runtime;

export function PlatformIcon(props = {}) {
  const favicon = String(props.favicon || "");
  if (!favicon.includes("#")) return null;
  const semantic_name = props.name || "platform-icon";
  return Runtime.SVG.SVG(
    {
      class: props.class,
      attributes: {
        n: semantic_name,
        viewBox: "0 0 32 32",
        "aria-hidden": "true",
        focusable: "false",
        ...(props.attributes || {}),
      },
    },
    [
      Runtime.SVG.Use({
        attributes: { href: favicon },
      }),
    ],
  );
}

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
          return PlatformIcon({
            class: "dm-platform-badge__icon",
            favicon: props.favicon,
            name: `${semantic_name}-icon`,
          });
        },
      }),
      ...label,
    ],
  );
}
