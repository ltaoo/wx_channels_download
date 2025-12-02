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

export default defineConfig({
  lang: "zh-CN",
  title: "wx_channels_download",
  description: "微信视频号下载工具文档",
  base: "/wx_channels_download/",
  lastUpdated: true,
  themeConfig: {
    nav: [
      { text: "首页", link: "/" },
      { text: "Releases", link: `/releases/${getLatestRelease()}` },
      { text: "FAQ", link: "/faq/button_inject_failed" },
    ],
    sidebar: [
      {
        text: "开始使用",
        items: [
          { text: "下载并启用", link: "/guide/start" },
          { text: "macOS 启用", link: "/guide/macos" },
          { text: "使用步骤", link: "/guide/step" },
        ],
      },
      {
        text: "功能",
        items: [
          { text: "长视频下载", link: "/feature/long_video" },
          { text: "指定文件名", link: "/feature/filename" },
          { text: "mp3下载", link: "/feature/mp3" },
          { text: "直播下载", link: "/feature/live" },
        ],
      },
      {
        text: "命令行",
        items: [
          { text: "代理服务", link: "/cli/proxy" },
          { text: "下载", link: "/cli/download" },
          { text: "解密", link: "/cli/decrypt" },
          { text: "删除证书", link: "/cli/uninstall" },
          { text: "查看版本", link: "/cli/version" },
        ],
      },
      {
        text: "配置",
        items: [
          { text: "下载", link: "/config/download" },
          { text: "代理", link: "/config/proxy" },
          { text: "脚本", link: "/config/script" },
          { text: "视频号", link: "/config/channel" },
        ],
      },
      {
        text: "常见问题",
        items: [
          { text: "注入下载按钮失败", link: "/faq/button_inject_failed" },
          { text: "下载卡住", link: "/faq/download_stuck" },
          { text: "解密失败", link: "/faq/decrypt_fail" },
          { text: "网络无法访问", link: "/faq/network_failed" },
          { text: "PowerShell", link: "/faq/powershell" },
        ],
      },
      {
        text: "发布日志",
        items: getReleaseItems(),
      },
    ],
    socialLinks: [
      { icon: "github", link: "https://github.com/ltaoo/wx_channels_download" },
    ],
    outline: "deep",
  },
});
