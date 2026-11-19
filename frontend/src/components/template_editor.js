import { EditorView, Decoration, DecorationSet } from "../../public/prosemirror.js";
import { TemplateEditorModel } from "./template_editor.model.js";

function TemplateEditor({ store, value, get_variables, multiline = true, attributes, min_height }) {
  let editor, model, list;
  const unlistens = [];
  const semantic = attributes.n;
  const list_id = `${attributes.id || semantic}-suggestions`;

  function render(snapshot) {
    if (!editor) return;
    editor.updateState(snapshot.editor_state);
    editor.dom.setAttribute("aria-expanded", String(snapshot.suggestions.length > 0));
    editor.dom.dataset.empty = String(!snapshot.text);
    list.hidden = snapshot.suggestions.length === 0;
    list.replaceChildren(...snapshot.suggestions.map((entry, index) => {
      const option = document.createElement("div");
      option.dataset.n = `${semantic}-suggestion-${entry.namespace}.${entry.key}`;
      option.id = `${list_id}-${index}`;
      option.setAttribute("role", "option");
      option.setAttribute("aria-selected", String(index === snapshot.selected));
      option.textContent = `{{${entry.namespace}.${entry.key}}}`;
      option.addEventListener("mousedown", (event) => event.preventDefault());
      option.addEventListener("click", () => { model.accept(index); editor.focus(); });
      return option;
    }));
    if (snapshot.suggestions.length) {
      const active = list.children[snapshot.selected];
      editor.dom.setAttribute("aria-activedescendant", active.id);
      active.scrollIntoView({ block: "nearest" });
    } else editor.dom.removeAttribute("aria-activedescendant");
  }

  return View({
    class: "dm-template-editor",
    attributes: { n: `${semantic}-root` },
    onMounted(event) {
      const root = event.target.get$elm ? event.target.get$elm() : event.target;
      list = document.createElement("div");
      list.dataset.n = `${semantic}-suggestions`;
      list.className = "dm-template-suggestions";
      list.id = list_id;
      list.setAttribute("role", "listbox");
      list.setAttribute("aria-label", "可用变量（方向键选择，Tab 或 Enter 补全）");
      list.hidden = true;
      model = TemplateEditorModel({ value: value ? value.value : store.state.value, get_variables, multiline,
        onChange: (text) => store.setValue(text), onRender: render });
      const { n, ...dom_attributes } = attributes;
      editor = new EditorView(root, {
        state: model.snapshot().editor_state,
        attributes: { ...dom_attributes, "data-n": n, role: "combobox", tabindex: "0", spellcheck: "false",
          "aria-multiline": String(multiline), "aria-autocomplete": "list", "aria-haspopup": "listbox",
          "aria-controls": list_id, "data-placeholder": store.state.placeholder || "输入 {{ 选择变量",
          class: "dm-field dm-template-input", style: `min-height: ${min_height || (multiline ? "100px" : "var(--dm-control-height-md)")}` },
        dispatchTransaction: (transaction) => model.dispatch(transaction),
        decorations(state) {
          return DecorationSet.create(state.doc, [...state.doc.textContent.matchAll(/\{\{[^{}\r\n]+\}\}/g)].map((match) =>
            Decoration.inline(match.index, match.index + match[0].length, { class: "dm-template-token", "data-n": `${semantic}-variable` })));
        },
        handleKeyDown(view, event) {
          if (!model.keydown(event)) return false;
          event.stopPropagation(); return true;
        },
        handlePaste(view, event) {
          model.insert_text(event.clipboardData?.getData("text/plain") || ""); return true;
        },
        handleDOMEvents: {
          focus() { store.handleFocus(); model.set_focus(true); return false; },
          blur() { store.handleBlur(); model.set_focus(false); return false; },
          compositionstart() { model.set_composing(true); return false; },
          compositionend() { model.set_composing(false); return false; },
          drop(view, event) { event.preventDefault(); model.insert_text(event.dataTransfer?.getData("text/plain") || ""); return true; },
        },
      });
      root.append(list);
      unlistens.push(store.onStateChange((state) => model.set_value(state.value)));
      if (value) unlistens.push(value.subscribe({ onChange: (next) => model.set_value(next) }));
      render(model.snapshot());
    },
    onUnmounted() { unlistens.forEach((unlisten) => unlisten()); editor?.destroy(); editor = null; },
  });
}

export { TemplateEditor };
