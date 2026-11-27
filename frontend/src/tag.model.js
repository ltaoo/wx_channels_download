import { setContentTags } from "@/biz/request.js";

const tag_writes = new WeakMap();

// A content's selection is shared by the picker and the list's remove buttons.
export function persist_content_tags({ client, content_id, tags_ref, next, tip, on_change }) {
  let state = tag_writes.get(tags_ref);
  if (!state) {
    state = { pending: Promise.resolve(), revision: 0, confirmed: (tags_ref.value || []).slice() };
    tag_writes.set(tags_ref, state);
  }
  const revision = ++state.revision;
  const selection = (next || []).slice();
  tags_ref.as(selection);
  const pending = state.pending.then(async () => {
    const request = new Timeless.kit.RequestCore(setContentTags, { client });
    try {
      const result = await request.run({ content_id, tag_ids: selection.map((tag) => tag.id) });
      if (result.error) throw result.error;
      state.confirmed = selection;
      if (revision === state.revision && on_change) on_change(selection);
      return result;
    } finally {
      request.destroy();
    }
  }).catch((error) => {
    if (revision === state.revision) tags_ref.as(state.confirmed);
    if (tip) tip({ msg: [error.message || String(error)], type: "error" });
    return Timeless.Result.Err(error);
  });
  state.pending = pending;
  return pending;
}
