/**
 * @file Vanilla Toaster - bottom-right stacked toasts with Tailwind dark: support
 */

const TOAST_LIFETIME = 4000;
const TOAST_WIDTH = 356;
const GAP = 14;
const VISIBLE_TOASTS_AMOUNT = 3;
const VIEWPORT_OFFSET = 24;
const TIME_BEFORE_UNMOUNT = 200;

const typeIcons = {
  success: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></svg>`,
  error: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/></svg>`,
  info: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>`,
  warning: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>`,
  loading: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="animate-spin"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>`,
};

const typeBorderClasses = {
  success: "border-green-300 dark:border-green-700",
  error: "border-red-300 dark:border-red-700",
  info: "border-blue-300 dark:border-blue-700",
  warning: "border-amber-300 dark:border-amber-700",
};

const typeBgClasses = {
  success: "bg-green-50 dark:bg-green-950",
  error: "bg-red-50 dark:bg-red-950",
  info: "bg-blue-50 dark:bg-blue-950",
  warning: "bg-amber-50 dark:bg-amber-950",
};

const typeTextClasses = {
  success: "text-green-900 dark:text-green-200",
  error: "text-red-900 dark:text-red-200",
  info: "text-blue-900 dark:text-blue-200",
  warning: "text-amber-900 dark:text-amber-200",
};

function buildToastHTML(toast) {
  const icon = toast.type && typeIcons[toast.type];
  const title = typeof toast.title === "function" ? toast.title() : toast.title;
  const description =
    typeof toast.description === "function"
      ? toast.description()
      : toast.description;

  let html = "";
  if (icon) {
    html += `<div data-icon="">${icon}</div>`;
  }
  html += `<div data-content="">`;
  html += `<div data-title="">${title || ""}</div>`;
  if (description) {
    html += `<div data-description="">${description}</div>`;
  }
  html += `</div>`;
  html += `<button data-close-button="" aria-label="Close toast">✕</button>`;
  return html;
}

function recalcPositions(container) {
  const all = [...container.querySelectorAll("[data-sonner-toast]")].filter(
    (el) => !el.dataset.removed,
  );
  const count = all.length;

  let offsetAccum = 0;
  all.forEach((el, i) => {
    const height = el.offsetHeight || 60;
    el.style.setProperty("--index", String(i));
    el.style.setProperty("--toasts-before", String(i));
    el.style.setProperty("--z-index", String(count - i));
    el.style.setProperty("--offset", `${offsetAccum}px`);
    el.style.setProperty("--initial-height", `${height}px`);
    el.dataset.front = i === 0 ? "true" : "false";
    el.dataset.visible = i < VISIBLE_TOASTS_AMOUNT ? "true" : "false";

    if (i === 0) {
      offsetAccum += height + GAP;
    } else {
      offsetAccum += GAP;
    }
  });
}

export function initToaster() {
  const sonner = Timeless.ui.SonnerCore.getInstance();
  const timers = new Map();
  const heights = new Map();

  // Inject layout styles (non-theme)
  if (!document.getElementById("sonner-styles")) {
    const style = document.createElement("style");
    style.id = "sonner-styles";
    style.textContent = SONNER_CSS;
    document.head.appendChild(style);
  }

  // Create container
  const container = document.createElement("ol");
  container.dir = "ltr";
  container.tabIndex = -1;
  container.setAttribute("data-sonner-toaster", "");
  container.setAttribute("data-y-position", "bottom");
  container.setAttribute("data-x-position", "right");
  container.style.setProperty("--width", `${TOAST_WIDTH}px`);
  container.style.setProperty("--gap", `${GAP}px`);
  container.style.setProperty("--front-toast-height", "0px");
  document.body.appendChild(container);

  function showToast(toast) {
    if (toast.dismiss) return;

    // Update existing toast (e.g. promise loading → success)
    const existing = container.querySelector(
      `[data-sonner-toast][data-id="${toast.id}"]`,
    );
    if (existing) {
      existing.innerHTML = buildToastHTML(toast);
      // Update type classes
      const type = toast.type || "normal";
      existing.dataset.type = type;
      existing.className = toastClass(type);
      recalcPositions(container);
      return;
    }

    const type = toast.type || "normal";
    const el = document.createElement("li");
    el.tabIndex = 0;
    el.setAttribute("data-sonner-toast", "");
    el.setAttribute("data-styled", "true");
    el.setAttribute("data-mounted", "false");
    el.setAttribute("data-type", type);
    el.setAttribute("data-expanded", "false");
    el.setAttribute("data-removed", "false");
    el.setAttribute("data-id", String(toast.id));
    el.className = toastClass(type);
    el.innerHTML = buildToastHTML(toast);

    container.prepend(el);

    // Measure height
    const h = el.getBoundingClientRect().height;
    heights.set(toast.id, h);
    container.style.setProperty("--front-toast-height", `${h}px`);

    recalcPositions(container);

    // Mount animation
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        el.setAttribute("data-mounted", "true");
        if (expanded) {
          el.setAttribute("data-expanded", "true");
          recalcPositions(container);
        }
      });
    });

    // Close button
    el.querySelector("[data-close-button]")?.addEventListener("click", () => {
      deleteToast(el, toast);
    });

    // Auto dismiss
    const duration =
      toast.duration !== undefined ? toast.duration : TOAST_LIFETIME;
    if (duration !== Infinity && duration > 0) {
      const timer = setTimeout(() => {
        deleteToast(el, toast);
        if (toast.onAutoClose) toast.onAutoClose(toast);
        timers.delete(toast.id);
      }, duration);
      timers.set(toast.id, timer);
    }
  }

  function toastClass(type) {
    const base =
      "pointer-events-auto absolute bottom-0 right-0 rounded-lg border shadow-lg " +
      "text-[13px] leading-normal flex items-center gap-1.5 p-4 overflow-hidden";
    const bg = typeBgClasses[type] || "bg-white dark:bg-black";
    const border =
      typeBorderClasses[type] || "border-zinc-200 dark:border-zinc-800";
    const text = typeTextClasses[type] || "text-zinc-900 dark:text-zinc-50";
    return `${base} ${bg} ${border} ${text}`;
  }

  function deleteToast(el, toast) {
    if (el.getAttribute("data-removed") === "true") return;
    el.setAttribute("data-removed", "true");
    el.setAttribute("data-front", "true");

    if (toast.onDismiss) toast.onDismiss(toast);

    heights.delete(toast.id);

    // Update front height
    const remaining = [
      ...container.querySelectorAll("[data-sonner-toast]"),
    ].filter((e) => !e.dataset.removed || e === el);
    const nextFront = remaining.find((e) => e !== el && !e.dataset.removed);
    if (nextFront) {
      container.style.setProperty(
        "--front-toast-height",
        `${heights.get(nextFront.dataset.id) || nextFront.offsetHeight}px`,
      );
    } else {
      container.style.setProperty("--front-toast-height", "0px");
    }

    recalcPositions(container);

    setTimeout(() => {
      el.remove();
      recalcPositions(container);
    }, TIME_BEFORE_UNMOUNT);
  }

  function dismissToastById(id) {
    const el = container.querySelector(`[data-sonner-toast][data-id="${id}"]`);
    if (el) deleteToast(el, {});
    if (timers.has(id)) {
      clearTimeout(timers.get(id));
      timers.delete(id);
    }
  }

  // Expand on hover
  let expanded = false;
  container.addEventListener("mouseenter", () => {
    expanded = true;
    container
      .querySelectorAll("[data-sonner-toast]")
      .forEach((el) => el.setAttribute("data-expanded", "true"));
    recalcPositions(container);
  });
  container.addEventListener("mouseleave", () => {
    expanded = false;
    container
      .querySelectorAll("[data-sonner-toast]")
      .forEach((el) => el.setAttribute("data-expanded", "false"));
    recalcPositions(container);
  });

  const unsubscribe = sonner.onSubscribe((data) => {
    if (data.dismiss) {
      dismissToastById(data.id);
    } else {
      showToast(data);
    }
  });

  return {
    container,
    destroy() {
      unsubscribe();
      timers.forEach((t) => clearTimeout(t));
      timers.clear();
      container.remove();
    },
  };
}

const SONNER_CSS = `
/* ========= Toaster container ========= */
[data-sonner-toaster] {
  position: fixed;
  width: var(--width, 356px);
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont,
    "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  box-sizing: border-box;
  padding: 0;
  margin: 0;
  list-style: none;
  outline: none;
  z-index: 99999;
}

[data-sonner-toaster][data-x-position="right"] { right: 24px; }
[data-sonner-toaster][data-x-position="left"]  { left: 24px; }
[data-sonner-toaster][data-y-position="bottom"] { bottom: 24px; }
[data-sonner-toaster][data-y-position="top"]    { top: 24px; }

/* ========= Toast layout (non-theme) ========= */
[data-sonner-toast] {
  z-index: var(--z-index);
  transform: translateY(100%);
  opacity: 0;
  touch-action: none;
  transition: transform 400ms cubic-bezier(0.22, 1, 0.36, 1),
    opacity 400ms, height 400ms;
  box-sizing: border-box;
  outline: none;
}

/* ========= Mounted ========= */
[data-sonner-toast][data-mounted="true"] {
  opacity: 1;
}

/* ========= Front toast ========= */
[data-sonner-toast][data-front="true"][data-mounted="true"] {
  transform: translateY(0);
}

/* ========= Collapsed (non-front, not expanded) ========= */
[data-sonner-toast][data-mounted="true"][data-front="false"][data-expanded="false"] {
  transform: translateY(calc(-1 * var(--gap, 14px) * var(--toasts-before, 0)))
    scale(calc(1 - var(--toasts-before, 0) * 0.05));
  height: var(--front-toast-height);
}

[data-sonner-toast][data-front="false"][data-expanded="false"] [data-icon],
[data-sonner-toast][data-front="false"][data-expanded="false"] [data-content],
[data-sonner-toast][data-front="false"][data-expanded="false"] [data-close-button] {
  opacity: 0;
  transition: opacity 200ms;
}

/* ========= Expanded (non-front) ========= */
[data-sonner-toast][data-mounted="true"][data-front="false"][data-expanded="true"] {
  transform: translateY(calc(-1 * var(--offset, 0px)));
  height: var(--initial-height);
}

[data-sonner-toast][data-front="false"][data-expanded="true"] [data-icon],
[data-sonner-toast][data-front="false"][data-expanded="true"] [data-content],
[data-sonner-toast][data-front="false"][data-expanded="true"] [data-close-button] {
  opacity: 1;
  transition: opacity 200ms;
}

/* ========= Removed (exit) ========= */
[data-sonner-toast][data-removed="true"] {
  transform: translateY(-100%) !important;
  opacity: 0 !important;
  transition: transform 300ms cubic-bezier(0.22, 1, 0.36, 1),
    opacity 300ms cubic-bezier(0.22, 1, 0.36, 1);
}

/* ========= Visibility ========= */
[data-sonner-toast][data-visible="false"] {
  opacity: 0;
  pointer-events: none;
}

/* ========= Inner layout (non-theme) ========= */
[data-sonner-toast][data-styled="true"] [data-icon] {
  display: flex;
  height: 16px;
  width: 16px;
  align-items: center;
  justify-content: flex-start;
  flex-shrink: 0;
  margin-right: 4px;
}

[data-sonner-toast][data-styled="true"] [data-content] {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

[data-sonner-toast][data-styled="true"] [data-title] {
  font-weight: 500;
  line-height: 1.5;
}

[data-sonner-toast][data-styled="true"] [data-description] {
  font-weight: 400;
  line-height: 1.4;
  color: #71717a;
}

.dark [data-sonner-toast][data-styled="true"] [data-description] {
  color: #a1a1aa;
}

/* ========= Close button (non-theme) ========= */
[data-sonner-toast][data-styled="true"] [data-close-button] {
  position: absolute;
  right: 0;
  top: 0;
  height: 20px;
  width: 20px;
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 0;
  background: inherit;
  border: 1px solid currentColor;
  border-radius: 50%;
  cursor: pointer;
  z-index: 1;
  font-size: 10px;
  transform: translate(35%, -35%);
  opacity: 0.5;
  transition: opacity 100ms;
}
[data-sonner-toast][data-styled="true"]:hover [data-close-button]:hover {
  opacity: 1;
}

/* ========= Focus ========= */
[data-sonner-toast]:focus-visible {
  box-shadow: 0 0 0 2px rgba(0, 0, 0, 0.2);
}

/* ========= Mobile ========= */
@media (max-width: 600px) {
  [data-sonner-toaster] { right: 16px; left: 16px; width: calc(100% - 32px); }
  [data-sonner-toaster][data-y-position="bottom"] { bottom: 16px; }
  [data-sonner-toast] { left: 0; right: 0; width: 100%; }
}

/* ========= Reduced motion ========= */
@media (prefers-reduced-motion) {
  [data-sonner-toast] {
    transition: none !important;
    animation: none !important;
  }
}
`;
