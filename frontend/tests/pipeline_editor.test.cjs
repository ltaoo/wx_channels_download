const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('node:test');

const context = vm.createContext({ Date, JSON, Promise });
const source = fs.readFileSync(path.join(__dirname, '../src/pages/pipeline_editor.model.js'), 'utf8');
vm.runInContext(source.replace(/export \{[^}]+\};\s*$/, ''), context);

function setup({ emit_on_send = false } = {}) {
  let value = { flow_id: 'flow-1', dirty: true, saving: false, pending_node_config: false, draft: { start_node_id: 'start', nodes: [{ id: 'start', name: '未保存节点' }] } };
  let message, state;
  const sent = [];
  let applied = 0;
  const bridge = context.PipelineEditorBridge({
    channel: {
      onMessage(fn) { message = fn; return () => { message = null; }; },
      onStateChange(fn) { state = fn; return () => { state = null; }; },
      async send(raw) {
        sent.push(JSON.parse(raw));
        if (emit_on_send && sent.length <= 10) state({ connected: true });
      },
    },
    read: () => value,
    apply(draft) { applied += 1; value = { ...value, draft, dirty: true }; },
  });
  return {
    bridge, sent, value: () => value, applied: () => applied,
    change(patch) { value = { ...value, ...patch }; },
    connected() { state({ connected: true }); },
    disconnected() { state({ connected: false }); },
    async receive(patch) {
      message({ type: 'pipeline_editor_read', flow_id: 'flow-1', request_id: 'request', expires_at: Date.now() + 4000, ...patch });
      await new Promise(setImmediate);
      return sent[sent.length - 1];
    },
  };
}

test('reads the live unsaved working copy and acknowledges a visible revision-checked update', async () => {
  const h = setup();
  h.connected();
  assert.equal(h.sent[0].type, 'pipeline_editor_register');
  const read = await h.receive();
  assert.equal(read.snapshot.draft.nodes[0].name, '未保存节点');
  const next = { start_node_id: 'start', nodes: [{ id: 'start', name: 'MCP 修改' }] };
  const applied = await h.receive({ type: 'pipeline_editor_apply', revision: read.snapshot.revision, draft: next });
  assert.equal(h.applied(), 1);
  assert.equal(h.value().draft.nodes[0].name, 'MCP 修改');
  assert.equal(applied.snapshot.dirty, true);
  assert.ok(applied.snapshot.revision > read.snapshot.revision);
  h.bridge.destroy();
});

test('rejects old revisions after manual edits and stale repeated writes', async () => {
  const h = setup();
  const { snapshot } = await h.receive();
  h.change({ draft: { start_node_id: 'start', nodes: [{ id: 'start', name: '手动修改' }] } });
  const failed = await h.receive({ type: 'pipeline_editor_apply', revision: snapshot.revision, draft: snapshot.draft });
  assert.match(failed.error, /草稿已发生变化/);
  assert.equal(h.applied(), 0);
  assert.equal(h.value().draft.nodes[0].name, '手动修改');
  h.bridge.destroy();
});

test('refuses writes during save, with pending inspector text, on another pipeline or after expiry', async () => {
  for (const [patch, message, pattern] of [
    [{ saving: true }, {}, /正在保存/],
    [{ pending_node_config: true }, {}, /配置文本尚未应用/],
    [{}, { flow_id: 'another' }, /已经切换/],
    [{}, { expires_at: Date.now() - 100 }, /已过期/],
  ]) {
    const h = setup();
    h.change(patch);
    const { snapshot } = await h.receive();
    const failed = await h.receive({ type: 'pipeline_editor_apply', revision: snapshot.revision, draft: snapshot.draft, ...message });
    assert.match(failed.error, pattern);
    assert.equal(h.applied(), 0);
    h.bridge.destroy();
  }
});

test('reconnect registers the current pipeline and closing releases its subscriptions', () => {
  const h = setup();
  h.connected();
  h.change({ flow_id: 'flow-2' });
  h.disconnected();
  h.connected();
  assert.equal(h.sent[1].flow_id, 'flow-2');
  h.bridge.destroy();
  h.bridge.register();
  assert.equal(h.sent.length, 2);
});

test('sending registration or replies cannot recursively register the connected channel', async () => {
  const h = setup({ emit_on_send: true });
  h.connected();
  assert.equal(h.sent.length, 1);
  h.connected();
  assert.equal(h.sent.length, 1);
  const read = await h.receive();
  assert.equal(read.type, 'pipeline_editor_reply');
  assert.equal(h.sent.length, 2);
  h.disconnected();
  h.connected();
  assert.equal(h.sent.length, 3);
  assert.equal(h.sent[2].type, 'pipeline_editor_register');
  h.bridge.destroy();
});
