// A live page owns its working copy. MCP requests must be answered by that page,
// and writes are acknowledged only after the reactive graph has been updated.
function PipelineEditorBridge({ channel, read, apply, on_error = () => {} }) {
  let revision = 0;
  let fingerprint = "";
  let destroyed = false;
  let connected = false;
  function snapshot() {
    const value = read();
    if (!value) throw new Error("Pipeline 编辑器尚未就绪，请等待加载完成后重试");
    const next = JSON.stringify(value);
    if (next !== fingerprint) { fingerprint = next; revision += 1; }
    return { ...value, revision };
  }
  function send(message) {
    return channel.send(JSON.stringify(message));
  }
  function register() {
    if (destroyed) return;
    const current = read();
    if (!current) return;
    send({ type: "pipeline_editor_register", flow_id: current.flow_id }).catch(on_error);
  }
  async function receive(message) {
    if (destroyed || !["pipeline_editor_read", "pipeline_editor_apply"].includes(message?.type)) return;
    const reply = { type: "pipeline_editor_reply", request_id: message.request_id };
    try {
      const current = snapshot();
      if (message.flow_id !== current.flow_id) throw new Error("编辑页面已经切换 Pipeline，请重新获取编辑器");
      if (!Number.isFinite(message.expires_at) || Date.now() >= message.expires_at) throw new Error("操作请求已过期，请重新读取草稿");
      if (message.type === "pipeline_editor_apply") {
        if (message.revision !== current.revision) throw new Error("草稿已发生变化，请重新读取最新 revision 后合并修改");
        if (current.saving) throw new Error("Pipeline 正在保存，请等待完成后重新读取");
        if (current.pending_node_config) throw new Error("节点配置文本尚未应用，请先应用或还原配置文本后重试");
        apply(message.draft);
        // Let the reactive renderer process its pending graph updates.
        await Promise.resolve();
      }
      reply.snapshot = snapshot();
    } catch (error) { reply.error = error.message || "编辑器操作失败"; }
    if (!destroyed) await send(reply);
  }
  const unsubscribe_message = channel.onMessage((message) => { receive(message).catch(on_error); });
  const unsubscribe_state = channel.onStateChange((state) => {
    const was_connected = connected;
    connected = Boolean(state.connected);
    // Sending also emits state; register only when the connection becomes ready.
    if (connected && !was_connected) register();
  });
  return {
    register,
    destroy() {
      destroyed = true;
      if (typeof unsubscribe_message === "function") unsubscribe_message();
      if (typeof unsubscribe_state === "function") unsubscribe_state();
    },
  };
}

export { PipelineEditorBridge };
