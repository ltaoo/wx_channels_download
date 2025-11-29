import { defineConfig } from 'vitepress'

export default defineConfig({
  lang: 'zh-CN',
  title: 'wx_channels_download',
  description: '微信视频号下载工具文档',
  base: '/wx_channels_download/',
  lastUpdated: true,
  themeConfig: {
    nav: [
      { text: '首页', link: '/' },
      { text: 'Releases', link: '/releases' },
      { text: 'FAQ', link: '/guide/faq' },
    ],
    sidebar: [
      {
        text: '开始使用',
        items: [
          { text: '下载并启用', link: '/guide/' },
          { text: 'macOS 启用', link: '/guide/macos' },
          { text: '使用步骤', link: '/guide/step' }
        ]
      },
      {
        text: '命令行',
        items: [
          { text: '代理服务', link: '/guide/cli' },
          { text: '下载', link: '/guide/cli/download' },
          { text: '解密', link: '/guide/cli/decrypt' }
        ]
      },
      {
        text: '配置',
        items: [
          { text: '下载', link: '/guide/config/download' },
          { text: '代理', link: '/guide/config/proxy' }
        ]
      },
      {
        text: '常见问题',
        items: [
          { text: '注入下载按钮失败', link: '/guide/faq/button_inject_failed' },
          { text: '长视频下载', link: '/guide/faq/long_video' },
          { text: '下载卡住', link: '/guide/faq/download_stuck' },
          { text: '网络无法访问', link: '/guide/faq/network_failed' },
          { text: 'PowerShell', link: '/guide/faq/powershell' }
        ]
      },
      {
        text: '发布日志',
        items: [
          { text: '251122', link: '/releases/251122' }
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/ltaoo/wx_channels_download' }
    ],
    outline: 'deep'
  }
})
