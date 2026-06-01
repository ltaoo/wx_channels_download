/**
 * SplitLayout - Resizable split layout component
 *
 * @param {Object} options
 * @param {"vertical"|"horizontal"} options.direction
 * @param {string} [options.class]
 * @param {Array<{defaultSize: number, minSize?: number, maxSize?: number, class?: string, scroll?: boolean, children: Array}>} options.items
 */
export function SplitLayout(options) {
  const { direction, items } = options;
  const isVertical = direction === "vertical";
  const panelSizeClass = isVertical ? "min-h-0" : "min-w-0";

  const group$ = new Timeless.ui.ResizablePanelsCore({ direction });

  const panels = items.map((item) => {
    const useScroll = item.scroll !== false;
    return {
      panel$: new Timeless.ui.ResizablePanelCore({
        defaultSize: item.defaultSize,
        minSize: item.minSize,
        maxSize: item.maxSize,
      }),
      scroll$: useScroll ? new Timeless.ui.ScrollViewCore({}) : null,
      useScroll,
      class: item.class,
      children: item.children,
    };
  });

  const elements = [];
  for (let i = 0; i < panels.length; i++) {
    const p = panels[i];
    const content = p.useScroll
      ? [ScrollView({ class: p.class || "", store: p.scroll$ }, p.children)]
      : p.class
        ? [View({ class: p.class }, p.children)]
        : p.children;
    elements.push(
      ResizablePanel(
        {
          store: p.panel$,
          group: group$,
          class: classNames([panelSizeClass, "overflow-clip"]),
        },
        content,
      ),
    );
    if (i < panels.length - 1) {
      elements.push(
        ResizableHandle({
          store: group$,
          panelBefore: panels[i].panel$,
          panelAfter: panels[i + 1].panel$,
          withHandle: true,
        }),
      );
    }
  }

  return ResizablePanels(
    {
      store: group$,
      direction,
      class: "w-full",
      style: { height: "100vh" },
    },
    elements,
  );
}

/**
 * SidebarLayout - Fixed sidebar + flexible content
 *
 * @param {Object} options
 * @param {Array} options.sidebar
 * @param {string} [options.sidebarWidth]
 * @param {string} [options.sidebarClass]
 * @param {string} [options.class]
 * @param {Array} children
 */
export function SidebarLayout(options, children) {
  const { sidebar, sidebarWidth = "240px", sidebarClass } = options;
  return View({ class: classNames(["w-full h-full", options.class]) }, [
    Flex({ class: "h-full" }, [
      View(
        {
          class: classNames([`w-[${sidebarWidth}]`, "h-full shrink-0", sidebarClass]),
        },
        sidebar,
      ),
      View({ class: "relative flex-1 w-0 h-full min-w-0" }, children),
    ]),
  ]);
}

/**
 * StackLayout - Vertical stack with fixed header/footer + flexible content
 *
 * @param {Object} options
 * @param {Array} [options.header]
 * @param {string} [options.headerClass]
 * @param {Array} [options.footer]
 * @param {string} [options.footerClass]
 * @param {string} [options.class]
 * @param {Array} children
 */
export function StackLayout(options, children) {
  const { header, headerClass, footer, footerClass } = options;
  const elements = [];
  if (header) {
    elements.push(View({ class: classNames(["shrink-0", headerClass]) }, header));
  }
  elements.push(View({ class: "relative flex-1 min-h-0" }, children));
  if (footer) {
    elements.push(View({ class: classNames(["shrink-0", footerClass]) }, footer));
  }
  return View(
    { class: classNames(["w-full h-full flex flex-col", options.class]) },
    elements,
  );
}

/**
 * PageContent - Simple scrollable page wrapper
 *
 * @param {Object} props
 * @param {string} [props.class]
 * @param {Array} children
 */
export function PageContent(props, children) {
  return ScrollView(
    {
      class: classNames(["h-full", props.class]),
      store: new Timeless.ui.ScrollViewCore({}),
    },
    children,
  );
}
