(function () {
  let virtualListIdSeed = 0;

  function createVirtualListId() {
    virtualListIdSeed += 1;
    return `wx-virtual-list-${virtualListIdSeed}`;
  }

  function readValue(value) {
    if (
      value &&
      (value.__is_ref ||
        value.__is_ref_array ||
        value.__is_ref_object ||
        (typeof value === "object" &&
          "value" in value &&
          typeof value.subscribe === "function"))
    ) {
      return value.value;
    }
    return value;
  }

  function readNumber(value, fallback) {
    const n = Number(readValue(value));
    return Number.isFinite(n) && n >= 0 ? n : fallback;
  }

  function readPixelValue(value, fallback) {
    const raw = readValue(value);
    const n =
      typeof raw === "number"
        ? raw
        : typeof raw === "string"
          ? Number(raw.trim().replace(/px$/, ""))
          : NaN;
    return Number.isFinite(n) && n >= 0 ? n : fallback;
  }

  function requestFrame(callback) {
    if (typeof requestAnimationFrame === "function") {
      const id = requestAnimationFrame(callback);
      return () => {
        if (typeof cancelAnimationFrame === "function") {
          cancelAnimationFrame(id);
        }
      };
    }
    const id = setTimeout(callback, 0);
    return () => clearTimeout(id);
  }

  function isDOMElement(value) {
    return !!(
      value &&
      typeof value.appendChild === "function" &&
      typeof value.setAttribute === "function"
    );
  }

  function getMountedElement(event) {
    let target = event && event.target ? event.target : event;
    for (let depth = 0; depth < 4; depth += 1) {
      if (isDOMElement(target)) {
        return target;
      }
      if (target && typeof target.get$elm === "function") {
        target = target.get$elm();
        continue;
      }
      if (target && target.$elm) {
        target = target.$elm;
        continue;
      }
      break;
    }
    return null;
  }

  function asArray(value) {
    if (!value) {
      return [];
    }
    const list = readValue(value);
    return Array.isArray(list) ? list : [];
  }

  function isRefLike(value) {
    return !!(
      value &&
      (value.__is_ref ||
        value.__is_ref_array ||
        value.__is_ref_object ||
        (typeof value === "object" &&
          "value" in value &&
          typeof value.subscribe === "function"))
    );
  }

  function isElementLike(value) {
    if (typeof isElement === "function") {
      return isElement(value);
    }
    return !!(Timeless && Timeless.isElement && Timeless.isElement(value));
  }

  function disposeElement(element) {
    if (!element) {
      return;
    }
    if (typeof element.beforeUnmounted === "function") {
      element.beforeUnmounted();
    }
    if (typeof element.onUnmounted === "function") {
      element.onUnmounted();
    }
  }

  function createIndexRef(index) {
    if (typeof ref === "function") {
      return ref(index);
    }
    return { value: index };
  }

  function createItemRef(item) {
    if (typeof ref === "function") {
      return ref(item);
    }
    return { value: item };
  }

  function getItemKey(item, index, key) {
    if (typeof key === "function") {
      const value = key(item, index);
      return value === null || typeof value === "undefined" ? index : value;
    }
    if (key && item && typeof item === "object") {
      const value = item[key];
      return value === null || typeof value === "undefined" ? index : value;
    }
    return index;
  }

  function createVirtualListInstance(root, props) {
    const content = document.createElement("div");
    const viewport = document.createElement("div");
    const rows = new Map();
    const measuredHeights = new Map();
    const cleanups = [];

    let items = [];
    let destroyed = false;
    let cancelScheduledRender = null;
    let scheduledForce = false;
    let reachedBottom = false;
    let resizeObserver = null;
    let offsets = [0];
    let offsetsDirty = true;
    let lastRange = { start: 0, end: 0 };

    function updateRenderAttributes(range) {
      if (!isDOMElement(root)) {
        return;
      }
      root.setAttribute("data-virtual-list-items", String(items.length));
      root.setAttribute("data-virtual-list-range-start", String(range.start));
      root.setAttribute("data-virtual-list-range-end", String(range.end));
      root.setAttribute("data-virtual-list-rendered", String(rows.size));
      root.setAttribute(
        "data-virtual-list-padding-bottom",
        String(paddingBottom()),
      );
    }

    function estimatedItemHeight() {
      return Math.max(1, readNumber(props.itemHeight, 40));
    }

    function gutter() {
      return readNumber(props.gutter, 0);
    }

    function buffer() {
      return Math.max(0, Math.floor(readNumber(props.buffer, 0)));
    }

    function size() {
      return Math.max(1, Math.floor(readNumber(props.size, 4)));
    }

    function paddingBottom() {
      return readPixelValue(props.paddingBottom, 0);
    }

    function externalScrollEnabled() {
      return (
        readValue(props.externalScroll) === true ||
        typeof props.scrollTop !== "undefined" ||
        typeof props.viewportHeight !== "undefined"
      );
    }

    function currentScrollTop() {
      if (externalScrollEnabled()) {
        return Math.max(0, readPixelValue(props.scrollTop, 0));
      }
      return Math.max(0, root.scrollTop || 0);
    }

    function currentViewportHeight() {
      const fallback = root.clientHeight || estimatedViewportHeight();
      if (externalScrollEnabled()) {
        return Math.max(1, readPixelValue(props.viewportHeight, fallback));
      }
      return Math.max(1, fallback);
    }

    function getKeyAt(index) {
      return getItemKey(items[index], index, props.key);
    }

    function resolvedItemHeight(index) {
      if (typeof props.itemHeight === "function") {
        try {
          const n = Number(props.itemHeight(items[index], index));
          if (Number.isFinite(n) && n > 0) {
            return n;
          }
        } catch (error) {
          console.error("[VirtualListView] itemHeight failed", error);
        }
      }
      return 0;
    }

    function getItemHeight(index) {
      const resolved = resolvedItemHeight(index);
      if (resolved > 0) {
        return resolved;
      }
      const measured = measuredHeights.get(getKeyAt(index));
      return Number.isFinite(measured) && measured > 0
        ? measured
        : estimatedItemHeight();
    }

    function markOffsetsDirty() {
      offsetsDirty = true;
    }

    function ensureOffsets() {
      if (!offsetsDirty && offsets.length === items.length + 1) {
        return offsets;
      }
      const nextOffsets = new Array(items.length + 1);
      let top = 0;
      nextOffsets[0] = 0;
      for (let index = 0; index < items.length; index += 1) {
        top += getItemHeight(index);
        if (index < items.length - 1) {
          top += gutter();
        }
        nextOffsets[index + 1] = top;
      }
      offsets = nextOffsets;
      offsetsDirty = false;
      return offsets;
    }

    function totalHeight() {
      const listOffsets = ensureOffsets();
      return listOffsets[listOffsets.length - 1] || 0;
    }

    function scrollableHeight() {
      return totalHeight() + paddingBottom();
    }

    function getOffset(index) {
      const listOffsets = ensureOffsets();
      return listOffsets[Math.max(0, Math.min(index, items.length))] || 0;
    }

    function findIndexAtOffset(offset) {
      if (!items.length) {
        return 0;
      }
      const listOffsets = ensureOffsets();
      const maxOffset = listOffsets[listOffsets.length - 1] || 0;
      const value = Math.max(0, Number(offset) || 0);
      if (value <= 0) {
        return 0;
      }
      if (value >= maxOffset) {
        return items.length - 1;
      }
      let low = 0;
      let high = items.length - 1;
      let result = 0;
      while (low <= high) {
        const mid = Math.floor((low + high) / 2);
        if (listOffsets[mid + 1] <= value) {
          low = mid + 1;
        } else {
          result = mid;
          high = mid - 1;
        }
      }
      return Math.max(0, Math.min(result, items.length - 1));
    }

    function estimatedViewportHeight() {
      return estimatedItemHeight() * size() + gutter() * Math.max(0, size() - 1);
    }

    function emitScroll() {
      if (typeof props.onScroll === "function") {
        props.onScroll({
          target: root,
          scrollTop: currentScrollTop(),
          clientHeight: currentViewportHeight(),
          scrollHeight: scrollableHeight(),
        });
      }
    }

    function maybeReachBottom() {
      if (typeof props.onReachBottom !== "function") {
        return;
      }
      const height = scrollableHeight();
      const scrollTop = currentScrollTop();
      const viewportHeight = currentViewportHeight();
      const nearBottom =
        height > 0 &&
        scrollTop + viewportHeight >=
          height - Math.max(estimatedItemHeight() * 2, gutter());
      if (nearBottom && !reachedBottom) {
        reachedBottom = true;
        props.onReachBottom({
          target: root,
          scrollTop,
          clientHeight: viewportHeight,
          scrollHeight: height,
        });
      } else if (!nearBottom) {
        reachedBottom = false;
      }
    }

    function updateContentHeight() {
      const height = scrollableHeight();
      content.style.height = `${height}px`;
      content.style.minHeight = `${height}px`;
      const spacer = content.querySelector("[data-wx-download-list-spacer]");
      if (spacer) {
        spacer.style.height = `${height}px`;
      }
    }

    function getVisibleRange() {
      if (!items.length) {
        return { start: 0, end: 0 };
      }
      const scrollTop = currentScrollTop();
      const viewportHeight = currentViewportHeight();
      const startIndex = findIndexAtOffset(scrollTop);
      const endIndex = findIndexAtOffset(scrollTop + viewportHeight) + 1;
      const rangeBuffer = buffer();
      const start = Math.max(0, startIndex - rangeBuffer);
      const end = Math.min(
        items.length,
        Math.max(start + 1, endIndex + rangeBuffer),
      );
      return { start, end };
    }

    function positionRow(entry) {
      entry.row.style.transform = `translateY(${getOffset(entry.index)}px)`;
    }

    function positionRows() {
      rows.forEach(positionRow);
    }

    function measureRow(entry) {
      if (!entry || entry.destroyed || destroyed) {
        return;
      }
      const rect = entry.row.getBoundingClientRect();
      const measured = Math.ceil(rect.height);
      if (!Number.isFinite(measured) || measured <= 0) {
        return;
      }
      const previous = getItemHeight(entry.index);
      if (Math.abs(previous - measured) < 1) {
        return;
      }
      measuredHeights.set(entry.key, measured);
      markOffsetsDirty();
      if (entry.index < lastRange.start && currentScrollTop() > 0) {
        const delta = measured - previous;
        if (externalScrollEnabled()) {
          if (typeof props.onScrollTopAdjust === "function") {
            props.onScrollTopAdjust(delta);
          }
        } else {
          root.scrollTop = Math.max(0, root.scrollTop + delta);
        }
      }
      updateContentHeight();
      positionRows();
      scheduleRender(false);
      if (typeof props.onItemResize === "function") {
        props.onItemResize({
          target: entry.row,
          index: entry.index,
          item: items[entry.index],
          key: entry.key,
          height: measured,
          previousHeight: previous,
        });
      }
    }

    function scheduleMeasure(entry) {
      if (!entry || entry.measureCleanup) {
        return;
      }
      entry.measureCleanup = requestFrame(() => {
        entry.measureCleanup = null;
        measureRow(entry);
      });
    }

    function mountChild(value, parent, entry) {
      if (value === null || typeof value === "undefined" || value === false) {
        return;
      }
      if (Array.isArray(value)) {
        value.forEach((child) => mountChild(child, parent, entry));
        return;
      }
      if (isElementLike(value)) {
        const rendered = Timeless.DOM.buildAndRender(value);
        if (rendered && rendered.dom) {
          parent.appendChild(rendered.dom);
        }
        entry.elements.push(value);
        return;
      }
      if (isRefLike(value)) {
        const node = document.createTextNode(
          value.value === null || typeof value.value === "undefined"
            ? ""
            : String(value.value),
        );
        parent.appendChild(node);
        if (typeof value.subscribe === "function") {
          entry.cleanups.push(
            value.subscribe({
              onChange(next) {
                node.textContent =
                  next === null || typeof next === "undefined"
                    ? ""
                    : String(next);
              },
            }),
          );
        }
        return;
      }
      parent.appendChild(document.createTextNode(String(value)));
    }

    function mountRow(index) {
      const item = items[index];
      const row = document.createElement("div");
      const indexRef = createIndexRef(index);
      const itemRef = createItemRef(item);
      const key = getKeyAt(index);
      const entry = {
        row,
        index,
        key,
        indexRef,
        itemRef,
        elements: [],
        cleanups: [],
        measureCleanup: null,
        destroyed: false,
      };

      row.setAttribute("data-list-view-item", "");
      row.setAttribute("data-list-view-index", String(index));
      row.setAttribute("data-list-view-key", String(key));
      row.style.position = "absolute";
      row.style.top = "0";
      row.style.left = "0";
      row.style.right = "0";
      row.style.width = "100%";
      row.style.boxSizing = "border-box";
      row.style.transform = `translateY(${getOffset(index)}px)`;

      if (typeof props.render === "function") {
        try {
          mountChild(props.render(itemRef, indexRef), row, entry);
        } catch (error) {
          console.error("[VirtualListView] render failed", error);
        }
      }

      rows.set(index, entry);
      viewport.appendChild(row);

      if (typeof ResizeObserver !== "undefined") {
        const rowResizeObserver = new ResizeObserver(() => scheduleMeasure(entry));
        rowResizeObserver.observe(row);
        entry.cleanups.push(() => rowResizeObserver.disconnect());
      }
      scheduleMeasure(entry);

      setTimeout(() => {
        if (entry.destroyed || destroyed) {
          return;
        }
        entry.elements.forEach((element) => {
          if (element && typeof element.onMounted === "function") {
            element.onMounted({ target: element.$elm || row });
          }
        });
        scheduleMeasure(entry);
      }, 0);
    }

    function removeRow(index) {
      const entry = rows.get(index);
      if (!entry) {
        return;
      }
      entry.destroyed = true;
      if (entry.measureCleanup) {
        entry.measureCleanup();
        entry.measureCleanup = null;
      }
      entry.elements.forEach(disposeElement);
      entry.cleanups.forEach((cleanup) => {
        if (typeof cleanup === "function") {
          cleanup();
        }
      });
      if (entry.indexRef && typeof entry.indexRef.destroy === "function") {
        entry.indexRef.destroy();
      }
      if (entry.itemRef && typeof entry.itemRef.destroy === "function") {
        entry.itemRef.destroy();
      }
      if (entry.row.parentNode) {
        entry.row.parentNode.removeChild(entry.row);
      }
      rows.delete(index);
    }

    function clearRows() {
      Array.from(rows.keys()).forEach(removeRow);
    }

    function renderVisible(force) {
      if (destroyed) {
        return;
      }
      const nextItems = asArray(props.each);
      const prevItems = items;
      const itemsChanged = nextItems !== prevItems;
      if (force || itemsChanged) {
        markOffsetsDirty();
      }
      items = nextItems;
      updateContentHeight();

      const range = getVisibleRange();
      lastRange = range;
      updateRenderAttributes(range);

      // Remove out-of-range rows, and update itemRef for changed items
      Array.from(rows.keys()).forEach((index) => {
        if (index < range.start || index >= range.end) {
          removeRow(index);
          return;
        }
        const entry = rows.get(index);
        const newKey = getKeyAt(index);
        if (entry.key !== newKey) {
          // Key at this index changed — must remount
          removeRow(index);
        } else if (itemsChanged && items[index] !== prevItems[index]) {
          // Same key but item object replaced — update the ref reactively
          if (entry.itemRef && typeof entry.itemRef.as === "function") {
            entry.itemRef.as(items[index]);
          } else if (entry.itemRef) {
            entry.itemRef.value = items[index];
          }
        }
      });

      for (let index = range.start; index < range.end; index += 1) {
        if (!rows.has(index)) {
          mountRow(index);
        } else {
          const entry = rows.get(index);
          entry.index = index;
          entry.key = getKeyAt(index);
          entry.row.setAttribute("data-list-view-index", String(index));
          entry.row.setAttribute("data-list-view-key", String(entry.key));
          if (entry.indexRef && typeof entry.indexRef.as === "function") {
            entry.indexRef.as(index);
          } else if (entry.indexRef) {
            entry.indexRef.value = index;
          }
          positionRow(entry);
        }
      }

      updateRenderAttributes(range);
      maybeReachBottom();
    }

    function scheduleRender(force) {
      scheduledForce = scheduledForce || !!force;
      if (cancelScheduledRender) {
        return;
      }
      cancelScheduledRender = requestFrame(() => {
        cancelScheduledRender = null;
        const forceNext = scheduledForce;
        scheduledForce = false;
        renderVisible(forceNext);
      });
    }

    function handleScroll() {
      emitScroll();
      scheduleRender(false);
      maybeReachBottom();
    }

    function subscribeItems() {
      const source = props.each;
      if (!source || typeof source.subscribe !== "function") {
        return;
      }
      cleanups.push(
        source.subscribe({
          onPatch() {
            markOffsetsDirty();
            scheduleRender(false);
          },
          onChange() {
            markOffsetsDirty();
            scheduleRender(false);
          },
        }),
      );
    }

    function subscribeValue(source, callback) {
      if (!source || source === props.each || typeof source.subscribe !== "function") {
        return;
      }
      cleanups.push(
        source.subscribe({
          onPatch() {
            callback();
          },
          onChange() {
            callback();
          },
        }),
      );
    }

    function subscribeLayoutProps() {
      subscribeValue(props.paddingBottom, () => {
        markOffsetsDirty();
        scheduleRender(false);
      });
      subscribeValue(props.scrollTop, () => scheduleRender(false));
      subscribeValue(props.viewportHeight, () => scheduleRender(false));
      subscribeValue(props.itemHeight, () => {
        markOffsetsDirty();
        scheduleRender(false);
      });
      subscribeValue(props.gutter, () => {
        markOffsetsDirty();
        scheduleRender(false);
      });
    }

    function mount() {
      if (!isDOMElement(root)) {
        throw new Error("VirtualListView root is not a DOM element");
      }

      root.setAttribute("data-list-view-root", "");
      root.setAttribute("data-virtual-list-view", "dynamic");
      if (!root.style.position) {
        root.style.position = "relative";
      }
      if (!externalScrollEnabled() && !root.style.overflowY) {
        root.style.overflowY = "auto";
      }

      content.setAttribute("data-list-view-content", "");
      content.style.position = "relative";
      content.style.width = "100%";
      content.style.boxSizing = "border-box";

      viewport.setAttribute("data-list-view-viewport", "");
      viewport.style.position = "absolute";
      viewport.style.top = "0";
      viewport.style.left = "0";
      viewport.style.right = "0";
      viewport.style.width = "100%";
      viewport.style.boxSizing = "border-box";

      content.appendChild(viewport);
      root.appendChild(content);
      if (!externalScrollEnabled()) {
        root.addEventListener("scroll", handleScroll, { passive: true });
        cleanups.push(() => root.removeEventListener("scroll", handleScroll));
      }

      if (typeof ResizeObserver !== "undefined") {
        resizeObserver = new ResizeObserver(() => scheduleRender(false));
        resizeObserver.observe(root);
        cleanups.push(() => resizeObserver.disconnect());
      } else {
        const handleResize = () => scheduleRender(false);
        window.addEventListener("resize", handleResize);
        cleanups.push(() => window.removeEventListener("resize", handleResize));
      }

      subscribeItems();
      subscribeLayoutProps();
      markOffsetsDirty();
      renderVisible(true);
      setTimeout(() => {
        if (!destroyed) {
          scheduleRender(true);
        }
      }, 0);
      emitScroll();
    }

    function destroy() {
      if (destroyed) {
        return;
      }
      destroyed = true;
      if (cancelScheduledRender) {
        cancelScheduledRender();
        cancelScheduledRender = null;
      }
      clearRows();
      cleanups.forEach((cleanup) => {
        if (typeof cleanup === "function") {
          cleanup();
        }
      });
      cleanups.length = 0;
      if (content.parentNode) {
        content.parentNode.removeChild(content);
      }
    }

    return { mount, destroy, refresh: () => scheduleRender(true) };
  }

  function VirtualListView(props) {
    const {
      each,
      render,
      key,
      size,
      buffer,
      gutter,
      itemHeight,
      externalScroll,
      scrollTop,
      viewportHeight,
      onScrollTopAdjust,
      onItemResize,
      onScroll,
      onReachBottom,
      paddingBottom,
      onMounted,
      beforeUnmounted,
      onUnmounted,
      ...viewProps
    } = props || {};

    const instanceId = createVirtualListId();
    let instance = null;
    let mountedCleanup = null;
    let retryMountTimer = null;
    let fallbackMountTimer = null;
    let fallbackMountAttempts = 0;
    let mountedRoot = null;

    function mountInstance(event) {
      const root = getMountedElement(event);
      if (!root) {
        return false;
      }
      if (instance && mountedRoot === root) {
        instance.refresh();
        return true;
      }
      if (instance) {
        instance.destroy();
      }
      mountedRoot = root;
      instance = createVirtualListInstance(root, {
        each,
        render,
        key,
        size,
        buffer,
        gutter,
        itemHeight,
        externalScroll,
        scrollTop,
        viewportHeight,
        onScrollTopAdjust,
        onItemResize,
        onScroll,
        onReachBottom,
        paddingBottom,
      });
      instance.mount();
      return true;
    }

    function scheduleFallbackMount() {
      if (
        fallbackMountTimer ||
        instance ||
        typeof document === "undefined" ||
        fallbackMountAttempts >= 10
      ) {
        return;
      }
      fallbackMountAttempts += 1;
      fallbackMountTimer = setTimeout(() => {
        fallbackMountTimer = null;
        if (instance) {
          return;
        }
        const root = document.querySelector(
          `[data-virtual-list-id="${instanceId}"]`,
        );
        if (root) {
          try {
            mountInstance(root);
          } catch (error) {
            console.error("[VirtualListView] fallback mount failed", error);
          }
          return;
        }
        scheduleFallbackMount();
      }, 0);
    }

    scheduleFallbackMount();

    return View(
      {
        ...viewProps,
        attributes: {
          ...(viewProps.attributes || {}),
          "data-list-view-root": "",
          "data-virtual-list-id": instanceId,
        },
        onMounted(event) {
          try {
            if (!mountInstance(event)) {
              retryMountTimer = setTimeout(() => {
                retryMountTimer = null;
                try {
                  if (!mountInstance(event)) {
                    console.error(
                      "[VirtualListView] mounted without a DOM root",
                      event,
                    );
                  }
                } catch (error) {
                  console.error("[VirtualListView] mount failed", error);
                }
              }, 0);
            }
          } catch (error) {
            console.error("[VirtualListView] mount failed", error);
          }
          if (typeof onMounted === "function") {
            mountedCleanup = onMounted(event);
          }
        },
        beforeUnmounted() {
          if (typeof beforeUnmounted === "function") {
            beforeUnmounted();
          }
        },
        onUnmounted() {
          if (retryMountTimer) {
            clearTimeout(retryMountTimer);
            retryMountTimer = null;
          }
          if (fallbackMountTimer) {
            clearTimeout(fallbackMountTimer);
            fallbackMountTimer = null;
          }
          if (instance) {
            instance.destroy();
            instance = null;
          }
          mountedRoot = null;
          if (typeof mountedCleanup === "function") {
            mountedCleanup();
            mountedCleanup = null;
          }
          if (typeof onUnmounted === "function") {
            onUnmounted();
          }
        },
      },
      [],
    );
  }

  window.VirtualListView = VirtualListView;
})();
