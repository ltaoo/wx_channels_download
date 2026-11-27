import { track_error } from "@/utils.js";

// ListCore is the pagination domain exported by the bundled timeless runtime.
export function create_list_request(load) {
  return new Timeless.kit.RequestCore(
    (params) => ({ method: "POST", url: "list", body: params }),
    { client: { async post(_url, params) {
      try {
        return Timeless.Result.Ok(await load(params));
      } catch (error) {
        return Timeless.Result.Err(error);
      }
    } } },
  );
}

export function create_list_model({ request, normalize = (data) => data, page_size = 50, tip = () => {} }) {
  const run = request.run.bind(request);
  request.run = async ({ pageSize, ...params }) => {
    params.page_size = pageSize;
    if (params.next_marker === "") delete params.next_marker;
    try {
      const result = await run(params);
      if (result.error) return result;
      const data = normalize(result.data, params);
      return Timeless.Result.Ok({ ...data, pageSize: data.page_size ?? pageSize });
    } catch (error) {
      return Timeless.Result.Err(error);
    }
  };
  const list = new Timeless.kit.ListCore(request, {
    pageSize: page_size,
    processor: (response, original) => ({ ...response, next_marker: original.next_marker }),
  });
  const default_params = { ...list.params };
  const rows = refarr([]);
  const total = ref(0);
  const page = ref(1);
  const size = ref(page_size);
  const initial = ref(true);
  const loading = ref(false);
  const error = track_error(ref(""), tip);
  const completed = ref(0);
  const status = combine({ initial, loading, error, rows }, (state) => {
    if (state.initial) return "initial";
    if (state.loading) return state.rows.length ? "normal" : "loading";
    if (state.error) return "error";
    return state.rows.length ? "normal" : "empty";
  });
  let request_completed = false;
  let append = false;
  let destroyed = false;
  let pending = Promise.resolve();
  list.onComplete(() => { request_completed = true; });
  list.onStateChange((state) => {
    if (destroyed) return;
    if (rows.value !== state.dataSource) rows.as(state.dataSource, { reset: true });
    total.as(state.total);
    page.as(state.page);
    size.as(state.pageSize);
    initial.as(state.initial);
    loading.as(state.loading);
    error.as(state.loading ? "" : state.error?.message || String(state.error || ""));
    // timeless emits Complete before committing the new response.
    if (!state.loading && request_completed) {
      request_completed = false;
      if (!append) completed.as(completed.value + 1);
    }
  });
  // ListCore mutates shared request/response state; serialize operations to keep
  // fast filter changes from committing responses out of order.
  for (const name of ["init", "search", "goto", "next", "prev", "refresh", "loadMore"]) {
    const action = list[name].bind(list);
    list[name] = (...args) => {
      const result = pending.then(async () => {
        if (destroyed) return Timeless.Result.Err("列表已销毁");
        append = name === "loadMore";
        const previous_params = { ...list.params };
        if (name === "search") list.setParams(() => ({ ...default_params, ...args[0] }));
        const response = await action(...args);
        list.setParams(() => response?.error ? previous_params : {
          ...list.params, page: list.response.page, pageSize: list.response.pageSize,
          next_marker: list.response.next_marker || "",
        });
        return response;
      });
      pending = result.catch(() => {});
      return result;
    };
  }
  const destroy = list.destroy.bind(list);
  list.destroy = () => {
    destroyed = true;
    request.destroy();
    destroy();
  };
  list.state = { rows, total, page, page_size: size, initial, loading, error, status, completed };
  return list;
}
