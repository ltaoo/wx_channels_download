/// <reference path="../utils.js" />
/**
 * @file Scraper fetch page Model and View.
 */
var HomePageModel = (() => {
  const home_api_origin = WXEnv.get("apiOrigin");
  const home_http_client = new Timeless.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: home_api_origin,
  });
  Timeless.web.provide_http_client(home_http_client);

  const home_request = Timeless.request_factory({
    headers: { "Content-Type": "application/json" },
    process(response) {
      if (response.error) {
        return Timeless.Result.Err(response.error);
      }
      const payload = response.data || {};
      if (payload.code !== 0) {
        return Timeless.Result.Err(
          payload.msg || "获取失败",
          payload.code,
          payload.data,
        );
      }
      return Timeless.Result.Ok(payload.data || {});
    },
  });

  function create_model() {
    const url_ = ref("");
    const loading_ = ref(false);
    const error_ = ref("");
    const result_ = ref(null);
    const download_loading_ = ref(false);
    const download_error_ = ref("");
    const download_success_ = ref("");
    let request_sequence = 0;

    const fetch_request = new Timeless.RequestCore(
      (params) => home_request.get("/api/scraper/fetch", params),
      {
        client: home_http_client,
      },
    );
    const download_request = new Timeless.RequestCore(
      (body) => home_request.post("/api/v1/download_task/create", body),
      {
        client: home_http_client,
      },
    );

    const submit_disabled_ = combine(
      {
        url: url_,
        loading: loading_,
        downloadLoading: download_loading_,
      },
      (state) =>
        state.loading ||
        state.downloadLoading ||
        !String(state.url || "").trim(),
    );
    const status_text_ = combine(
      {
        loading: loading_,
        error: error_,
        downloadLoading: download_loading_,
        downloadError: download_error_,
        downloadSuccess: download_success_,
      },
      (state) => {
        if (state.loading) {
          return "正在获取...";
        }
        if (state.downloadLoading) {
          return "正在创建下载任务...";
        }
        return (
          state.error || state.downloadError || state.downloadSuccess || ""
        );
      },
    );
    const busy_ = combine(
      { loading: loading_, downloadLoading: download_loading_ },
      (state) => state.loading || state.downloadLoading,
    );
    const has_error_ = combine(
      { error: error_, downloadError: download_error_ },
      (state) => Boolean(state.error || state.downloadError),
    );
    const has_result_ = computed(result_, (data) => Boolean(data));
    const result_platform_ = computed(
      result_,
      (data) => (data && data.platform) || "result",
    );
    const result_text_ = computed(result_, (data) =>
      data ? JSON.stringify(data, null, 2) : "",
    );
    const download_disabled_ = combine(
      {
        result: result_,
        loading: download_loading_,
        success: download_success_,
      },
      (state) => !state.result || state.loading || Boolean(state.success),
    );
    const download_button_text_ = combine(
      { loading: download_loading_, success: download_success_ },
      (state) => {
        if (state.loading) {
          return "创建中";
        }
        return state.success ? "已创建" : "下载";
      },
    );

    async function submit() {
      if (loading_.value || download_loading_.value) {
        return null;
      }
      const raw_url = String(url_.value || "").trim();
      if (!raw_url) {
        error_.as("请输入 URL");
        return null;
      }

      const sequence = ++request_sequence;
      loading_.as(true);
      error_.as("");
      result_.as(null);
      download_error_.as("");
      download_success_.as("");

      const result = await fetch_request.run({ url: raw_url });
      if (sequence !== request_sequence) {
        return result;
      }
      loading_.as(false);
      if (result.error) {
        error_.as(result.error.message || String(result.error));
        return result;
      }

      result_.as(result.data || {});
      return result;
    }

    async function create_download_task() {
      if (download_loading_.value || download_success_.value) {
        return null;
      }

      const fetch_result = result_.value;
      const platform = String(
        (fetch_result && fetch_result.platform) || "",
      ).trim();
      const content = fetch_result && fetch_result.result;
      if (!platform || content === undefined || content === null) {
        download_error_.as("解析结果缺少 platform 或 result");
        return null;
      }

      download_loading_.as(true);
      download_error_.as("");
      download_success_.as("");
      const body = {
        objects: [
          {
            platform,
            content,
            config: { platform },
          },
        ],
      };
      const result = await download_request.run(body);
      download_loading_.as(false);
      if (result.error) {
        download_error_.as(result.error.message || String(result.error));
        return result;
      }

      const tasks =
        result.data && Array.isArray(result.data.tasks)
          ? result.data.tasks
          : [];
      const failed_task = tasks.find(
        (task) => task && Number(task.code || 0) !== 0,
      );
      if (failed_task) {
        download_error_.as(failed_task.msg || "创建下载任务失败");
        return result;
      }

      download_success_.as("下载任务创建成功");
      return result;
    }

    const methods = {
      setURL(value) {
        url_.as(String(value || ""));
        if (error_.value) {
          error_.as("");
        }
      },
      submit,
      createDownloadTask: create_download_task,
    };

    return {
      state: {
        url: url_,
        loading: loading_,
        error: error_,
        result: result_,
        download_loading: download_loading_,
        download_error: download_error_,
        download_success: download_success_,
        submit_disabled: submit_disabled_,
        status_text: status_text_,
        busy: busy_,
        has_error: has_error_,
        has_result: has_result_,
        result_platform: result_platform_,
        result_text: result_text_,
        download_disabled: download_disabled_,
        download_button_text: download_button_text_,
      },
      methods,
    };
  }

  return create_model;
})();

function HomePageForm(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "wx-home-form",
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.submit();
      },
    },
    [
      View({ class: "wx-content-search wx-home-search" }, [
        Timeless.Icon({ name: "search", size: 16 }),
        Input({
          class: "wx-content-search-input",
          value: vm$.state.url,
          placeholder: "输入需要获取的内容 URL",
          attributes: {
            type: "text",
            name: "url",
            autocomplete: "off",
            autofocus: "autofocus",
            "aria-label": "内容 URL",
          },
          onInput(event) {
            const target =
              event &&
              event.target &&
              typeof event.target.get$elm === "function"
                ? event.target.get$elm()
                : event && event.target;
            vm$.methods.setURL(
              target && typeof target.value === "string" ? target.value : "",
            );
          },
        }),
      ]),
      Button(
        {
          class: "wx-content-action wx-home-submit",
          disabled: vm$.state.submit_disabled,
          attributes: { type: "button" },
          onClick() {
            vm$.methods.submit();
          },
        },
        [
          computed(vm$.state.loading, (loading) =>
            loading ? "获取中" : "确认",
          ),
        ],
      ),
    ],
  );
}

function HomePageResult(props) {
  const vm$ = props.store;
  return Show({
    when: vm$.state.has_result,
    ok() {
      return View({ class: "wx-home-result" }, [
        View({ class: "wx-home-result-head" }, [
          View({ class: "wx-content-row-platform" }, [
            vm$.state.result_platform,
          ]),
          Button(
            {
              class: "wx-content-action wx-home-download",
              disabled: vm$.state.download_disabled,
              attributes: {
                type: "button",
                title: "创建下载任务",
              },
              onClick() {
                vm$.methods.createDownloadTask();
              },
            },
            [
              Timeless.Icon({ name: "download", size: 16 }),
              View({ class: "wx-content-action-label" }, [
                vm$.state.download_button_text,
              ]),
            ],
          ),
        ]),
        View({ type: "pre", class: "wx-home-result-body" }, [
          vm$.state.result_text,
        ]),
      ]);
    },
  });
}

function HomePageHeader() {
  return View({ class: "wx-content-header" }, [
    View({ class: "wx-content-header-inner" }, [
      View({ class: "wx-content-brand" }, [
        View({ class: "wx-content-brand-icon" }, [
          Timeless.Icon({ name: "search", size: 28 }),
        ]),
        View({}, [
          View({ class: "wx-content-title" }, ["链接解析"]),
          View({ class: "wx-content-subtitle" }, [
            "输入内容链接，获取平台解析结果",
          ]),
        ]),
      ]),
    ]),
  ]);
}

function HomePageView(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-page wx-home-page" }, [
    HomePageHeader(),
    View({ class: "wx-content-main wx-home-main" }, [
      View({ class: "wx-home-content" }, [
        HomePageForm({ store: vm$ }),
        Show({
          when: computed(vm$.state.status_text, (text) => Boolean(text)),
          ok() {
            return View(
              {
                class: computed(vm$.state.has_error, (hasError) =>
                  hasError ? "wx-home-status error" : "wx-home-status",
                ),
              },
              [
                Show({
                  when: vm$.state.busy,
                  ok() {
                    return View({ class: "weui-loading" });
                  },
                }),
                vm$.state.status_text,
              ],
            );
          },
        }),
        HomePageResult({ store: vm$ }),
      ]),
    ]),
  ]);
}

(() => {
  function mount() {
    let root = document.getElementById("app");
    if (!root) {
      root = document.createElement("div");
      root.id = "app";
      document.body.appendChild(root);
    }
    const vm$ = HomePageModel();
    Timeless.DOM.render(HomePageView({ store: vm$ }), root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount, { once: true });
    return;
  }
  mount();
})();
