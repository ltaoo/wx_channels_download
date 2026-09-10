const categories = [
  { id: "recommended", label: "推荐", query: "news" },
  { id: "technology", label: "科技", query: "technology" },
  { id: "ai", label: "AI", query: "artificial intelligence machine learning" },
  { id: "programming", label: "开发", query: "software engineering programming" },
  { id: "business", label: "商业", query: "business finance" },
  { id: "science", label: "科学", query: "science research" },
  { id: "design", label: "设计", query: "design creative" },
  { id: "life", label: "生活", query: "lifestyle culture" },
];

const initial_category = categories[0];

function safe_http_url(value) {
  if (typeof value !== "string" || value.length === 0 || value.length > 2048) return "";
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:" ? url.toString() : "";
  } catch {
    return "";
  }
}

function plain_text(value, limit = 420) {
  const source = String(value || "").replace(/<[^>]*>/g, " ");
  const element = document.createElement("template");
  element.innerHTML = source;
  return element.content.textContent.replace(/\s+/g, " ").trim().slice(0, limit);
}

function format_count(value) {
  const number = Number(value || 0);
  if (number >= 10000) return (number / 10000).toFixed(number >= 100000 ? 0 : 1) + " 万订阅";
  return number + " 订阅";
}

function format_time(value) {
  const timestamp = Number(value || 0);
  if (!timestamp) return "";
  const date = new Date(timestamp > 100000000000 ? timestamp : timestamp * 1000);
  return Number.isNaN(date.getTime()) ? "" : date.toLocaleString();
}

function element(tag_name, semantic_name, properties = {}) {
  const node = document.createElement(tag_name);
  node.setAttribute("data-n", semantic_name);
  Object.assign(node, properties);
  return node;
}

function create_model() {
  const state = {
    query: initial_category.query,
    category_id: initial_category.id,
    feeds: [],
    loading: false,
    error: "",
    copy_message: "",
    preview: {
      open: false,
      loading: false,
      error: "",
      feed: null,
      items: [],
      copy_message: "",
    },
  };
  const listeners = new Set();
  let search_timer = 0;
  let search_sequence = 0;
  let copy_timer = 0;

  function subscribe(listener) {
    listeners.add(listener);
    return () => listeners.delete(listener);
  }

  function notify() {
    for (const listener of listeners) listener(state);
  }

  function normalize_feed(item) {
    const feed_url = safe_http_url(item.feed_url);
    if (!feed_url) return null;
    const website = safe_http_url(item.website);
    return {
      title: String(item.title || "未命名订阅源").slice(0, 160),
      description: plain_text(item.description, 260),
      website,
      feed_url,
      language: String(item.language || "").slice(0, 12).toUpperCase(),
      subscribers: Number(item.subscribers || 0),
      velocity: Number(item.velocity || 0),
    };
  }

  function manual_feed(url) {
    const parsed = new URL(url);
    return {
      title: parsed.hostname + (parsed.pathname === "/" ? "" : parsed.pathname),
      description: "手动输入的 RSS/Atom 地址",
      website: parsed.origin,
      feed_url: url,
      language: "",
      subscribers: 0,
      velocity: 0,
    };
  }

  async function run_search(query) {
    const trimmed = String(query || "").trim().slice(0, 100);
    const sequence = ++search_sequence;
    if (!trimmed) {
      state.error = "请输入搜索关键词";
      state.feeds = [];
      notify();
      return;
    }
    state.query = trimmed;
    state.loading = true;
    state.error = "";
    state.feeds = [];
    notify();
    try {
      if (safe_http_url(trimmed)) {
        const feed = manual_feed(safe_http_url(trimmed));
        if (sequence !== search_sequence) return;
        state.feeds = [feed];
        state.loading = false;
        notify();
        open_preview(feed);
        return;
      }
      const response = await fetch("/api/search?" + new URLSearchParams({ q: trimmed }), {
        headers: { Accept: "application/json" },
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload.error || "搜索失败");
      if (sequence !== search_sequence) return;
      state.feeds = (Array.isArray(payload.results) ? payload.results : [])
        .map(normalize_feed)
        .filter(Boolean);
    } catch (error) {
      if (sequence !== search_sequence) return;
      state.error = error instanceof Error ? error.message : "搜索失败";
      state.feeds = [];
    } finally {
      if (sequence === search_sequence) {
        state.loading = false;
        notify();
      }
    }
  }

  function queue_search(query) {
    state.query = String(query || "").slice(0, 100);
    clearTimeout(search_timer);
    search_timer = setTimeout(() => run_search(state.query), 350);
  }

  function submit_search(query) {
    clearTimeout(search_timer);
    run_search(query);
  }

  function select_category(category_id) {
    const category = categories.find((item) => item.id === category_id);
    if (!category) return;
    state.category_id = category.id;
    clearTimeout(search_timer);
    run_search(category.query);
  }

  function xml_text(item, tag_name) {
    return item.getElementsByTagName(tag_name)[0]?.textContent || "";
  }

  function item_link(item, root_name) {
    if (root_name === "feed") {
      const links = Array.from(item.getElementsByTagName("link"));
      const alternate = links.find((link) => link.getAttribute("rel") === "alternate") || links[0];
      return safe_http_url(alternate?.getAttribute("href") || alternate?.textContent || "");
    }
    return safe_http_url(xml_text(item, "link"));
  }

  function parse_preview(xml_text_source) {
    const document_node = new DOMParser().parseFromString(xml_text_source, "application/xml");
    if (document_node.querySelector("parsererror")) throw new Error("订阅源 XML 解析失败");
    const root = document_node.documentElement;
    const root_name = root.nodeName.toLowerCase();
    const source_items = root_name === "feed"
      ? Array.from(document_node.getElementsByTagName("entry"))
      : Array.from(document_node.getElementsByTagName("item"));
    if (source_items.length === 0) throw new Error("订阅源中没有可预览的条目");
    return source_items.slice(0, 20).map((item) => ({
      title: plain_text(xml_text(item, "title"), 180) || "无标题",
      link: item_link(item, root_name),
      published_at: format_time(Date.parse(xml_text(item, "pubDate") || xml_text(item, "updated") || xml_text(item, "published"))),
      description: plain_text(xml_text(item, "description") || xml_text(item, "summary") || xml_text(item, "encoded"), 360),
    }));
  }

  async function open_preview(feed) {
    state.preview = {
      open: true,
      loading: true,
      error: "",
      feed,
      items: [],
      copy_message: "",
    };
    notify();
    try {
      const response = await fetch("/api/preview?" + new URLSearchParams({ url: feed.feed_url }), {
        headers: { Accept: "application/xml" },
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => ({}));
        throw new Error(payload.error || "订阅源读取失败");
      }
      state.preview.items = parse_preview(await response.text());
    } catch (error) {
      state.preview.error = error instanceof Error ? error.message : "订阅源读取失败";
    } finally {
      state.preview.loading = false;
      notify();
    }
  }

  function close_preview() {
    state.preview.open = false;
    notify();
  }

  async function copy_feed_url(feed_url) {
    try {
      await navigator.clipboard.writeText(feed_url);
      state.copy_message = "已复制";
    } catch {
      state.copy_message = "复制失败，请手动复制";
    }
    notify();
    clearTimeout(copy_timer);
    copy_timer = setTimeout(() => {
      state.copy_message = "";
      notify();
    }, 1800);
  }

  function start() {
    run_search(initial_category.query);
  }

  return {
    state,
    categories,
    subscribe,
    queue_search,
    submit_search,
    select_category,
    open_preview,
    close_preview,
    copy_feed_url,
    start,
  };
}

function create_view(model) {
  const nodes = Object.fromEntries(
    Array.from(document.querySelectorAll("[data-n]")).map((node) => [node.dataset.n, node]),
  );
  const card_template = nodes["discovery-feed-card-template"];
  const search_input = nodes["discovery-search-input"];

  function render_categories() {
    const container = nodes["discovery-category-list"];
    container.replaceChildren(...model.categories.map((category) => element("button", "discovery-category-button", {
      textContent: category.label,
      type: "button",
      ariaPressed: String(category.id === model.state.category_id),
      onclick() {
        model.select_category(category.id);
        search_input.value = category.query;
      },
    })));
  }

  function render_feed(feed) {
    const card = card_template.content.cloneNode(true);
    const card_node = card.querySelector('[data-n="discovery-feed-card"]');
    const query = (name) => card.querySelector(`[data-n="${name}"]`);
    const hostname = safe_http_url(feed.website) || feed.feed_url;
    query("discovery-feed-initial").textContent = String(feed.title || "?").trim().charAt(0).toUpperCase() || "?";
    query("discovery-feed-title").textContent = feed.title;
    query("discovery-feed-website").textContent = hostname ? new URL(hostname).hostname : "";
    query("discovery-feed-description").textContent = feed.description || "暂无介绍";
    query("discovery-feed-subscribers").textContent = feed.subscribers ? format_count(feed.subscribers) : "RSS";
    query("discovery-feed-language").textContent = feed.language || "FEED";
    query("discovery-feed-velocity").textContent = feed.velocity ? `${feed.velocity.toFixed(1)} 篇/周` : "可预览";
    const open_link = query("discovery-feed-open-link");
    if (feed.website) {
      open_link.href = feed.website;
    } else {
      open_link.hidden = true;
    }
    card_node.dataset.feedUrl = feed.feed_url;
    return card_node;
  }

  function render_feeds() {
    const container = nodes["discovery-feed-list"];
    if (model.state.loading) {
      const loading = element("p", "discovery-empty", { textContent: "正在搜索订阅源…" });
      container.replaceChildren(loading);
      return;
    }
    if (model.state.feeds.length === 0) {
      const empty = element("p", "discovery-empty", { textContent: model.state.error || "没有找到匹配的订阅源" });
      container.replaceChildren(empty);
      return;
    }
    container.replaceChildren(...model.state.feeds.map(render_feed));
  }

  function render_preview_item(item) {
    const article = element("article", "discovery-preview-item");
    const title = element("h3", "discovery-preview-item-title");
    if (item.link) {
      const link = element("a", "discovery-preview-item-link", {
        textContent: item.title,
        href: item.link,
        target: "_blank",
        rel: "noopener noreferrer",
      });
      title.append(link);
    } else {
      title.textContent = item.title;
    }
    const metadata = element("p", "discovery-preview-item-meta", { textContent: item.published_at });
    const description = element("p", "discovery-preview-item-description", {
      textContent: item.description || "暂无摘要",
    });
    article.append(title, metadata, description);
    return article;
  }

  function render_preview() {
    const panel = nodes["discovery-preview-panel"];
    const preview = model.state.preview;
    panel.hidden = !preview.open;
    if (!preview.open) return;
    nodes["discovery-preview-title"].textContent = preview.feed?.title || "订阅源预览";
    const site_link = nodes["discovery-preview-site-link"];
    if (preview.feed?.website) {
      site_link.textContent = preview.feed.website;
      site_link.href = preview.feed.website;
      site_link.hidden = false;
    } else {
      site_link.hidden = true;
    }
    const content = nodes["discovery-preview-content"];
    if (preview.loading) {
      content.replaceChildren(element("p", "discovery-preview-state", { textContent: "正在读取最新内容…" }));
    } else if (preview.error) {
      content.replaceChildren(element("p", "discovery-preview-state", { textContent: preview.error }));
    } else {
      content.replaceChildren(...preview.items.map(render_preview_item));
    }
    nodes["discovery-preview-copy-status"].textContent = preview.copy_message;
  }

  function render() {
    if (document.activeElement !== search_input) {
      search_input.value = model.state.query;
    }
    render_categories();
    render_feeds();
    nodes["discovery-results-status"].textContent = model.state.loading
      ? "搜索中…"
      : `${model.state.feeds.length} 个订阅源 · ${model.state.query}${model.state.copy_message ? " · " + model.state.copy_message : ""}`;
    nodes["discovery-results-error"].textContent = model.state.feeds.length === 0 ? model.state.error : "";
    render_preview();
  }

  nodes["discovery-search-form"].addEventListener("submit", (event) => {
    event.preventDefault();
    model.submit_search(search_input.value);
  });
  search_input.addEventListener("input", () => model.queue_search(search_input.value));
  nodes["discovery-feed-list"].addEventListener("click", (event) => {
    const button = event.target.closest("button");
    if (!button) return;
    const feed = model.state.feeds.find((item) => item.feed_url === button.closest('[data-n="discovery-feed-card"]')?.dataset.feedUrl);
    if (!feed) return;
    if (button.dataset.n === "discovery-feed-preview-button") model.open_preview(feed);
    if (button.dataset.n === "discovery-feed-copy-button") model.copy_feed_url(feed.feed_url);
  });
  nodes["discovery-preview-close-button"].onclick = () => model.close_preview();
  nodes["discovery-preview-copy-button"].onclick = () => {
    if (model.state.preview.feed) model.copy_feed_url(model.state.preview.feed.feed_url);
  };
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && model.state.preview.open) model.close_preview();
  });

  model.subscribe(render);
  render();
  return { focus_search: () => search_input.focus() };
}

const discovery_model = create_model();
const discovery_view = create_view(discovery_model);
discovery_model.start();
discovery_view.focus_search();
