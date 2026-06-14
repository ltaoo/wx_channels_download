import { defineConfig } from "vitepress";
import { readdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

// 动态读取 releases 目录生成发布日志项
function getReleaseItems() {
  const __dirname = dirname(fileURLToPath(import.meta.url));
  const releasesDir = join(__dirname, "../releases");
  const files = readdirSync(releasesDir);

  return files
    .filter((file: string) => file.endsWith(".md"))
    .map((file: string) => file.replace(".md", ""))
    .sort((a: string, b: string) => b.localeCompare(a)) // 按日期倒序排列
    .map((date: string) => ({
      text: `v${date}`,
      link: `/releases/${date}`,
    }));
}

// 获取最新的 release 日期
function getLatestRelease() {
  const __dirname = dirname(fileURLToPath(import.meta.url));
  const releasesDir = join(__dirname, "../releases");
  const files = readdirSync(releasesDir);

  const dates = files
    .filter((file: string) => file.endsWith(".md"))
    .map((file: string) => file.replace(".md", ""))
    .sort((a: string, b: string) => b.localeCompare(a));

  return dates[0] || "251201"; // 如果没有文件，返回默认值
}

// 构建时从 GitHub Release API 获取最新版本下载信息
async function fetchReleaseData() {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10000);
  try {
    const res = await fetch(
      "https://api.github.com/repos/ltaoo/wx_channels_download/releases/latest",
      {
        headers: { Accept: "application/vnd.github+json" },
        signal: controller.signal,
      }
    );
    clearTimeout(timeout);
    if (!res.ok) return null;
    const release = await res.json();
    return {
      tag: release.tag_name,
      url: release.html_url,
      assets: (release.assets || []).map((a: any) => ({
        name: a.name,
        url: a.browser_download_url,
        size: a.size,
      })),
    };
  } catch {
    clearTimeout(timeout);
    return null;
  }
}

export default defineConfig(async () => {
  const latestReleaseDate = getLatestRelease();
  const releaseItems = getReleaseItems();
  // 使用模拟数据验证组件（取消注释以使用模拟数据）
  const releaseData = await fetchReleaseData();
  /* const releaseData = {
    tag: "v260531",
    url: "https://github.com/litaoo/wx_channels_download/releases/tag/v260531",
    assets: [
      { name: "wx_video_download_v260531_darwin_arm64.zip", url: "https://example.com/darwin_arm64.zip", size: 8_388_608 },
      { name: "wx_video_download_v260531_darwin_x86_64.zip", url: "https://example.com/darwin_x86_64.zip", size: 8_500_000 },
      { name: "wx_video_download_v260531_windows_x86_64.zip", url: "https://example.com/windows_x86_64.zip", size: 7_200_000 },
      { name: "wx_video_download_safe_v260531_windows_x86_64.zip", url: "https://example.com/safe_windows_x86_64.zip", size: 14_000_000 },
      { name: "wx_video_download_v260531_windows_arm64.zip", url: "https://example.com/windows_arm64.zip", size: 6_800_000 },
      { name: "wx_video_download_v260531_linux_x86_64.tar.gz", url: "https://example.com/linux_x86_64.tar.gz", size: 12_000_000 },
      { name: "wx_video_download_v260531_linux_arm64.tar.gz", url: "https://example.com/linux_arm64.tar.gz", size: 11_500_000 },
    ],
  }; */

  return {
  lang: "zh-CN",
  title: "wx_channels_download",
  description: "微信视频号下载工具文档",
  base: "/wx_channels_download/",
  lastUpdated: true,
  head: [
    ["link", { rel: "shortcut icon", href: "/wx_channels_download/favicon.png" }],
    ["link", { rel: "icon", type: "image/png", href: "/wx_channels_download/favicon.png" }],
  ],
  themeConfig: {
    nav: [
      { text: "首页", link: "/" },
      { text: "Releases", link: `/releases/${latestReleaseDate}` },
      { text: "API Playground", link: "/feature/playground" },
      { text: "FAQ", link: "/faq/button_inject_failed" },
    ],
    sidebar: [
      {
        text: "开始使用",
        items: [
          { text: "下载并运行", link: "/guide/start" },
          { text: "使用步骤", link: "/guide/step" },
          { text: "使用 Docker 运行", link: "/guide/docker" },
          { text: "手动安装根证书", link: "/guide/certificate" },
        ],
      },
      {
        text: "功能",
        items: [
          { text: "API", link: "/feature/api" },
          { text: "公众号", link: "/feature/mp-rss" },
          { text: "长视频下载", link: "/feature/long-video" },
          { text: "mp3下载", link: "/feature/mp3" },
          { text: "直播下载", link: "/feature/live" },
          { text: "批量下载", link: "/feature/batch" },
          { text: "指定文件名", link: "/feature/filename" },
          { text: "自定义菜单", link: "/feature/custom-menu" },
          { text: "监听事件", link: "/feature/event" },
        ],
      },
      {
        text: "命令行",
        items: [
          { text: "代理服务", link: "/cli/proxy" },
          { text: "下载", link: "/cli/download" },
          { text: "解密", link: "/cli/decrypt" },
          { text: "视频号解析", link: "/cli/sph" },
          { text: "删除证书", link: "/cli/uninstall" },
          { text: "查看版本", link: "/cli/version" },
          { text: "更新", link: "/cli/update" },
        ],
      },
      {
        text: "配置",
        items: [
          { text: "代理", link: "/config/proxy" },
          { text: "根证书", link: "/config/cert" },
          { text: "API 服务", link: "/config/api" },
          { text: "公众号服务", link: "/config/officialacount" },
          { text: "下载", link: "/config/download" },
          { text: "脚本", link: "/config/script" },
          { text: "视频号", link: "/config/channels" },
          { text: "Cloudflare", link: "/config/cloudflare" },
          { text: "调试与 PageSpy", link: "/config/debug" },
        ],
      },
      {
        text: "常见问题",
        items: [
          { text: "没有下载按钮", link: "/faq/button_inject_failed" },
          { text: "下载卡住", link: "/faq/download_stuck" },
          { text: "解密失败", link: "/faq/decrypt_fail" },
          { text: "网络无法访问", link: "/faq/network_failed" },
          { text: "PowerShell", link: "/faq/powershell" },
        ],
      },
      {
        text: "发布日志",
        items: releaseItems,
      },
    ],
    socialLinks: [
      { icon: "github", link: "https://github.com/ltaoo/wx_channels_download" },
    ],
    outline: "deep",
    search: {
      provider: "local",
    },
  },
  vite: {
    define: {
      __RELEASE_DATA__: JSON.stringify(releaseData),
    },
  },
};
});
