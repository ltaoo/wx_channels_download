import { fetchAccountList, synchronizeAccount } from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";

function normalizeAccount(account) {
  const medias = (account.video_accounts || [])
    .map((row) => row.video || row.Video || null)
    .filter(Boolean)
    .map((video) => ({
      ...video,
      cover_url: video.cover_url || video.CoverURL || video.coverUrl || "",
      title: video.title || video.Title || "",
    }));
  return {
    ...account,
    nickname:
      account.nickname ||
      account.username ||
      account.external_id ||
      "未命名帐号",
    avatar_url: account.avatar_url || "",
    medias,
  };
}

export function AccountsPageModel(props) {
  const reqs = {
    accounts: new Timeless.RequestCore(fetchAccountList, {
      client: api_client$,
    }),
    synchronize: new Timeless.RequestCore(synchronizeAccount, {
      client: api_client$,
    }),
  };
  const accounts_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const keyword_ = ref("");
  const selected_ = ref(null);
  const playing_url_ = ref("");

  async function load() {
    loading_.as(true);
    error_.as("");
    const r = await reqs.accounts.run({});
    loading_.as(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      return;
    }
    accounts_.as((r.data.list || []).map(normalizeAccount));
  }

  const filtered_ = combine(
    { accounts: accounts_, keyword: keyword_ },
    (d) => {
      const k = String(d.keyword || "")
        .trim()
        .toLowerCase();
      if (!k) return d.accounts;
      return d.accounts.filter((it) => {
        return [it.nickname, it.username, it.external_id].some((v) =>
          String(v || "")
            .toLowerCase()
            .includes(k),
        );
      });
    },
  );

  return {
    state: {
      accounts: accounts_,
      filtered: filtered_,
      loading: loading_,
      error: error_,
      keyword: keyword_,
      selected: selected_,
      playingUrl: playing_url_,
    },
    ui: {
      view: new Timeless.ui.ScrollViewCore({}),
      keyword: new Timeless.ui.InputCore({
        placeholder: "搜索昵称、用户名",
        onChange(value) {
          keyword_.as(value);
        },
      }),
    },
    methods: {
      init: load,
      refresh: load,
      select(account) {
        selected_.as(account);
      },
      async synchronize(account) {
        const r = await reqs.synchronize.run({ account_id: account.id });
        if (r.error) {
          props.app.tip?.({
            type: "error",
            text: [r.error.message || String(r.error)],
          });
          return;
        }
        props.app.tip?.({ type: "success", text: ["同步完成"] });
        await load();
      },
      play(video) {
        const url = video.url || video.URL || "";
        if (!url) {
          props.app.tip?.({ type: "warning", text: ["该视频没有可播放地址"] });
          return;
        }
        playing_url_.as(url);
        window.open(url, "_blank");
      },
    },
  };
}
