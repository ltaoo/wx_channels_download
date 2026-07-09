/**
 * @file 错误捕获
 */
class ErrorModal {
  constructor() {
    this.mounted = false;
  }
  insertElements() {
    // 创建样式
    var style = document.createElement("style");
    style.textContent = `
    .error-modal {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.5);
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
        background-color: var(--BG-0);
        color: var(--FG-0);
        border-radius: 8px;
        width: 90%;
        max-width: 680px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        transform: translateY(-50px);
        transition: transform 0.3s ease;
        border: 1px solid var(--FG-3);
    }

    .error-modal.active .error-modal-content {
        transform: translateY(0);
    }

    .error-modal-header {
        padding: 8px 12px;
        border-bottom: 1px solid var(--FG-3);
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .error-modal-title {
        margin: 0;
        font-size: 1.25rem;
        color: var(--RED);
    }

    .error-modal-close {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: var(--FG-1);
        padding: 0;
        line-height: 1;
    }

    .error-modal-close:hover {
        color: var(--FG-0);
    }

    .error-modal-body {
	    overflow-y: auto;
        padding: 12px;
        color: var(--FG-0);
        line-height: 1.5;
	    max-height:400px;
    }

    .error-modal-footer {
        padding: 8px 12px;
        border-top: 1px solid var(--FG-3);
        display: flex;
        justify-content: flex-end;
    }

    .error-modal-confirm {
        background-color: var(--RED);
        color: white;
        border: none;
        padding: 8px 8px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.875rem;
        transition: background-color 0.2s ease;
    }

    .error-modal-confirm:hover {
        opacity: 0.9;
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

    // 创建 DOM 结构
    var modal = document.createElement("div");
    modal.id = "error-modal";
    modal.className = "error-modal";

    modal.innerHTML = `
    <div class="error-modal-content">
        <div class="error-modal-header">
            <h3 class="error-modal-title">错误提示</h3>
            <button class="error-modal-close">&times;</button>
        </div>
        <div class="error-modal-body">
            <p class="error-message">这里显示错误信息</p>
        </div>
        <div class="error-modal-footer">
            <button class="error-modal-confirm">确定</button>
        </div>
    </div>
    `;
    document.body.appendChild(modal);
  }

  show(error) {
    if (this.mounted === false) {
      this.insertElements();
      this.modal = document.getElementById("error-modal");
      this.errorMessage = this.modal.querySelector(".error-message");
      this.closeBtn = this.modal.querySelector(".error-modal-close");
      this.confirmBtn = this.modal.querySelector(".error-modal-confirm");
      this.closeBtn.addEventListener("click", () => this.hide());
      this.confirmBtn.addEventListener("click", () => this.hide());
      this.modal.addEventListener("click", (e) => {
        if (e.target === this.modal) {
          this.hide();
        }
      });
      this.mounted = true;
    }
    var text =
      typeof error === "string" ? error : error.message || "发生未知错误";
    this.errorMessage.innerHTML = text;
    this.modal.classList.add("active");
    document.body.style.overflow = "hidden";
  }
  hide() {
    this.modal.classList.remove("active");
    document.body.style.overflow = "";
  }
}

const errorModal = new ErrorModal();
var errors = [];
window.addEventListener("error", function (event) {
  event.preventDefault();
  var r = parse_error_stack(event.error.stack);
  if (r) {
    errors.push(r);
  }
  if (errors.length) {
    var text = render_errors(errors);
    errorModal.show(text);
  }
});
window.addEventListener("unhandledrejection", function (event) {
  event.preventDefault();
  var r = parse_error_stack(event.reason.stack);
  if (r) {
    errors.push(r);
  }
  if (errors.length) {
    var text = render_errors(errors);
    errorModal.show(text);
  }
});

function render_errors(errors) {
  var result = [];
  for (let i = 0; i < errors.length; i += 1) {
    const e = errors[i];
    var $type = document.createElement("div");
    $type.style.cssText = "font-size: 18px";
    $type.innerHTML = e.type;
    var $msg = document.createElement("div");
    $msg.innerHTML = e.msg;
    /** @type {HTMLDivElement} */
    var $source = document.createElement("div");
    $source.style.cssText = "margin-left: 12px;";
    $source.innerHTML = "at " + e.source;
    var $container = document.createElement("div");
    $container.appendChild($type);
    $container.appendChild($msg);
    $container.appendChild($source);
    result.push($container.innerHTML);
  }
  return result.join("");
}
function parse_error_stack(error_stack) {
  if (!error_stack) {
    return null;
  }
  var regexp = /^([a-zA-Z]{1,}):([\s\S]{1,})[\r\n ]{1,}at([\s\S]{1,})$/;
  var matched = error_stack.match(regexp);
  if (!matched) {
    return null;
  }
  return {
    type: matched[1],
    msg: matched[2],
    source: matched[3],
  };
}
