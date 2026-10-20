import { Schema, EditorState, TextSelection, history, closeHistory, undo, redo } from "../../public/prosemirror.js";

const template_schema = new Schema({ nodes: { doc: { content: "text*", whitespace: "pre" }, text: { inline: true } } });

function TemplateEditorModel({ value = "", get_variables, multiline = true, onChange, onRender }) {
  let editor_state = create_state(value);
  let focused = false, composing = false, dismissed = false, selected = 0;

  function normalize(text) {
    const normalized = String(text ?? "").replace(/\r\n?/g, "\n");
    return multiline ? normalized : normalized.replace(/\n/g, "");
  }

  function create_state(text) {
    const normalized = normalize(text);
    const doc = template_schema.node("doc", null, normalized ? template_schema.text(normalized) : null);
    return EditorState.create({ doc, selection: TextSelection.create(doc, doc.content.size), plugins: [history()] });
  }

  function snapshot() {
    const text = editor_state.doc.textContent;
    const cursor = editor_state.selection.from;
    const match = focused && !composing && !dismissed && editor_state.selection.empty
      ? /\{\{([^{}\r\n]*)$/.exec(text.slice(0, cursor)) : null;
    const keyword = match ? match[1].trim() : "";
    const suggestions = match ? get_variables().filter(({ namespace, key }) => `${namespace}.${key}`.includes(keyword)) : [];
    // 编辑已有模板时一并替换剩余路径和闭括号，其他后缀原样保留。
    const suffix = match ? /^[\w.\-\t ]*\}\}/.exec(text.slice(cursor)) : null;
    return { editor_state, text, suggestions, selected: selected < suggestions.length ? selected : 0,
      from: match ? match.index : cursor, to: cursor + (suffix ? suffix[0].length : 0) };
  }

  function render() { onRender?.(snapshot()); }

  function dispatch(transaction) {
    const before = editor_state.doc.textContent;
    editor_state = editor_state.apply(transaction);
    if (transaction.docChanged || transaction.selectionSet) { dismissed = false; selected = 0; }
    render();
    if (before !== editor_state.doc.textContent) onChange?.(editor_state.doc.textContent);
  }

  function accept(index = snapshot().selected) {
    const { suggestions, from, to } = snapshot();
    const entry = suggestions[index];
    if (!entry) return false;
    dispatch(closeHistory(editor_state.tr).insertText(`{{${entry.namespace}.${entry.key}}}`, from, to).scrollIntoView());
    dispatch(closeHistory(editor_state.tr));
    return true;
  }

  function insert_text(text) { dispatch(editor_state.tr.insertText(normalize(text))); }

  return {
    snapshot, dispatch, accept, insert_text,
    set_value(value) {
      if (normalize(value) === editor_state.doc.textContent) return;
      editor_state = create_state(value);
      dismissed = false; selected = 0;
      render();
    },
    set_focus(value) { focused = value; dismissed = false; render(); },
    set_composing(value) { composing = value; render(); },
    keydown(event) {
      if (composing || event.isComposing) return false;
      if (event.ctrlKey || event.metaKey) {
        if (event.altKey) return false;
        const key = event.key.toLowerCase();
        if (key === "z" || key === "y") return (key === "y" || event.shiftKey ? redo : undo)(editor_state, dispatch);
        return false;
      }
      if (event.altKey) return false;
      const { suggestions, selected: active } = snapshot();
      if (suggestions.length) {
        if (event.key === "Enter" || (event.key === "Tab" && !event.shiftKey)) return accept();
        if (event.key === "ArrowDown" || event.key === "ArrowUp") {
          selected = (active + (event.key === "ArrowDown" ? 1 : -1) + suggestions.length) % suggestions.length;
          render(); return true;
        }
        if (event.key === "Escape") { dismissed = true; render(); return true; }
      }
      if (event.key === "Enter") { if (multiline) insert_text("\n"); return true; }
      return false;
    },
  };
}

export { TemplateEditorModel };
