---
title: 网络无法访问
---

# 网络无法访问

## 常见原因

- 启动后设置了系统代理，影响其他代理软件或网络策略。

## 解决方案

取消系统代理即可


## 和 Clash 一起使用

如果使用了类似 `Clash`、`Sing-box` 等翻墙软件，可以在 `config.yaml` 设置 `proxy.system=false`，通过 Clash 将流量转发到下载器端口。

参考 [代理配置](../config/proxy.md#与-clash-协同)
