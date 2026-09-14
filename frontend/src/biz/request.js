export const request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(r) {
    if (r.error) {
      return Timeless.Result.Err(r.error);
    }
    const data = r.data || {};
    if (typeof data.code === "undefined") {
      return Timeless.Result.Ok(data);
    }
    if (data.code !== 0) {
      return Timeless.Result.Err(data.msg || "请求失败", data.code, data.data);
    }
    return Timeless.Result.Ok(data.data || {});
  },
});
