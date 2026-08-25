/**
 * @file Error capture
 */
class ErrorModel {
  constructor() {
    this.errors = [];
    this.visible = false;
    this.view = null;
  }

  setView(view) {
    this.view = view;
  }

  show(error) {
    this.errors.push(normalize_error(error));
    this.visible = true;
    this.render();
  }

  hide() {
    this.visible = false;
    this.render();
  }

  render() {
    if (this.view) {
      this.view.render({ errors: this.errors, visible: this.visible });
    }
  }
}

class ErrorModalView {
  constructor(model) {
    this.model = model;
    this.mounted = false;
    this.renderScheduled = false;
  }

  insertElements() {
    // Create styles
    var style = document.createElement("style");
    style.setAttribute("data-n", "error-modal-style");
    style.textContent = `
    .error-modal {
        --error-modal-overlay: rgba(0, 0, 0, 0.5);
        --error-modal-surface: #fff;
        --error-modal-border: #eee;
        --error-modal-text: #333;
        --error-modal-muted: #666;
        --error-modal-muted-hover: #333;
        --error-modal-danger: #f44336;
        --error-modal-danger-hover: #d32f2f;
        --error-modal-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: var(--error-modal-overlay);
        display: flex;
        justify-content: center;
        align-items: center;
        z-index: 1000;
        opacity: 0;
        visibility: hidden;
        transition: opacity 0.3s ease, visibility 0.3s ease;
    }
    .error-modal.active {
        opacity: 1;
        visibility: visible;
    }
    .error-modal-content {
        background-color: var(--error-modal-surface);
        border-radius: 8px;
        width: 90%;
        max-width: 400px;
        box-shadow: var(--error-modal-shadow);
        transform: translateY(-50px);
        transition: transform 0.3s ease;
    }

    .error-modal.active .error-modal-content {
        transform: translateY(0);
    }

    .error-modal-header {
        padding: 8px 12px;
        border-bottom: 1px solid var(--error-modal-border);
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .error-modal-title {
        margin: 0;
        font-size: 1.25rem;
        color: var(--error-modal-danger);
    }

    .error-modal-close {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: var(--error-modal-muted);
        padding: 0;
        line-height: 1;
    }

    .error-modal-close:hover {
        color: var(--error-modal-muted-hover);
    }

    .error-modal-body {
        overflow-y: auto;
        padding: 12px;
        color: var(--error-modal-text);
        line-height: 1.5;
        max-height: 400px;
    }

    .error-modal-footer {
        padding: 8px 12px;
        border-top: 1px solid var(--error-modal-border);
        display: flex;
        justify-content: flex-end;
    }

    .error-modal-confirm {
        background-color: var(--error-modal-danger);
        color: white;
        border: none;
        padding: 8px 8px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.875rem;
        transition: background-color 0.2s ease;
    }

    .error-modal-confirm:hover {
        background-color: var(--error-modal-danger-hover);
    }

    .dark .error-modal,
    body[data-weui-theme="dark"] .error-modal {
        --error-modal-overlay: rgba(0, 0, 0, 0.68);
        --error-modal-surface: #191919;
        --error-modal-border: rgba(255, 255, 255, 0.12);
        --error-modal-text: rgba(255, 255, 255, 0.88);
        --error-modal-muted: rgba(255, 255, 255, 0.56);
        --error-modal-muted-hover: rgba(255, 255, 255, 0.9);
        --error-modal-danger: #fa5151;
        --error-modal-danger-hover: #c84040;
        --error-modal-shadow: 0 8px 28px rgba(0, 0, 0, 0.48);
        color-scheme: dark;
    }

    @media (prefers-color-scheme: dark) {
        body:not([data-weui-theme="light"]) .error-modal {
            --error-modal-overlay: rgba(0, 0, 0, 0.68);
            --error-modal-surface: #191919;
            --error-modal-border: rgba(255, 255, 255, 0.12);
            --error-modal-text: rgba(255, 255, 255, 0.88);
            --error-modal-muted: rgba(255, 255, 255, 0.56);
            --error-modal-muted-hover: rgba(255, 255, 255, 0.9);
            --error-modal-danger: #fa5151;
            --error-modal-danger-hover: #c84040;
            --error-modal-shadow: 0 8px 28px rgba(0, 0, 0, 0.48);
            color-scheme: dark;
        }
    }

    @media (max-width: 480px) {
        .error-modal-content {
            width: 95%;
        }
        
        .error-modal-header, .error-modal-body, .error-modal-footer {
            padding: 12px 16px;
        }
    }
    `;
    document.head.appendChild(style);

    // Create DOM structure
    var modal = document.createElement("div");
    modal.id = "error-modal";
    modal.className = "error-modal";
    modal.setAttribute("data-n", "error-modal");
    modal.setAttribute("role", "alertdialog");
    modal.setAttribute("aria-modal", "true");
    modal.setAttribute("aria-labelledby", "error-modal-title");

    modal.innerHTML = `
    <div class="error-modal-content" data-n="error-modal-content">
        <div class="error-modal-header" data-n="error-modal-header">
            <h3 id="error-modal-title" class="error-modal-title" data-n="error-modal-title">错误提示</h3>
            <button class="error-modal-close" data-n="error-modal-close" type="button" aria-label="关闭">&times;</button>
        </div>
        <div class="error-modal-body" data-n="error-modal-body">
            <div class="error-message" data-n="error-modal-errors"></div>
        </div>
        <div class="error-modal-footer" data-n="error-modal-footer">
            <button class="error-modal-confirm" data-n="error-modal-confirm" type="button">确定</button>
        </div>
    </div>
    `;
    document.body.appendChild(modal);
    this.modal = modal;
    this.errorMessage = modal.querySelector('[data-n="error-modal-errors"]');
    this.closeBtn = modal.querySelector('[data-n="error-modal-close"]');
    this.confirmBtn = modal.querySelector('[data-n="error-modal-confirm"]');
    this.closeBtn.addEventListener("click", () => this.model.hide());
    this.confirmBtn.addEventListener("click", () => this.model.hide());
    this.modal.addEventListener("click", (event) => {
      if (event.target === this.modal) {
        this.model.hide();
      }
    });
    this.mounted = true;
  }

  render(state) {
    if (!document.body) {
      if (!this.renderScheduled) {
        this.renderScheduled = true;
        document.addEventListener(
          "DOMContentLoaded",
          () => {
            this.renderScheduled = false;
            this.model.render();
          },
          { once: true },
        );
      }
      return;
    }

    if (!this.mounted) {
      this.insertElements();
    }
    this.renderErrors(state.errors);
    this.modal.classList.toggle("active", state.visible);
    document.body.style.overflow = state.visible ? "hidden" : "";
  }

  renderErrors(errors) {
    var fragment = document.createDocumentFragment();
    for (let i = 0; i < errors.length; i += 1) {
      var error = errors[i];
      var type = document.createElement("div");
      type.setAttribute("data-n", "error-type");
      type.style.cssText = "font-size: 18px";
      type.textContent = error.type;

      var message = document.createElement("div");
      message.setAttribute("data-n", "error-message");
      message.textContent = error.msg;

      var source = document.createElement("div");
      source.setAttribute("data-n", "error-source");
      source.style.cssText = "margin-left: 12px; white-space: pre-wrap;";
      source.textContent = "at " + error.source;

      var container = document.createElement("div");
      container.setAttribute("data-n", "error-item");
      container.appendChild(type);
      container.appendChild(message);
      container.appendChild(source);
      fragment.appendChild(container);
    }
    this.errorMessage.replaceChildren(fragment);
  }
}

var errorModel = new ErrorModel();
var errorModalView = new ErrorModalView(errorModel);
errorModel.setView(errorModalView);
window.errorModal = errorModel;

window.addEventListener("error", function (event) {
  // Print all error events to the console for easy DevTools debugging
  console.error("[ERROR.js]", {
    message: event.message,
    filename: event.filename,
    lineno: event.lineno,
    colno: event.colno,
    error: event.error,
    targetSrc: event.target && event.target.src,
    targetTag: event.target && event.target.tagName,
  });
  errorModel.show(normalize_window_error(event));
}, true);

window.addEventListener("unhandledrejection", function (event) {
  errorModel.show(event.reason);
});

function normalize_window_error(event) {
  if (event.error) {
    return normalize_error(event.error, error_event_source(event));
  }

  var target = event.target;
  var target_source =
    target && (target.src || target.href || target.tagName);
  return {
    type: target_source ? "Resource error" : "Script error",
    msg:
      event.message ||
      (target_source
        ? "Failed to load: " + target_source
        : "No message (cross-origin sanitized)"),
    source: event.filename || target_source || "Unknown",
  };
}

function normalize_error(error, fallback_source) {
  if (error && error.type && error.msg) {
    return {
      type: String(error.type),
      msg: String(error.msg),
      source: String(error.source || fallback_source || "Unknown"),
    };
  }

  if (error && typeof error === "object") {
    return {
      type: String(error.name || "Error"),
      msg: String(error.message || "发生未知错误"),
      source: String(error.stack || fallback_source || "Unknown"),
    };
  }

  return {
    type: "Error",
    msg: error == null ? "发生未知错误" : String(error),
    source: String(fallback_source || "Unknown"),
  };
}

function error_event_source(event) {
  var source = event.filename || "Unknown";
  if (event.lineno) {
    source += ":" + event.lineno;
  }
  if (event.colno) {
    source += ":" + event.colno;
  }
  return source;
}
