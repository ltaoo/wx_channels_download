---
title: macOS 启用
---

# macOS 启用

自 251213 之后，会对 `wx_video_download` 进行签名和公证，以避免 macOS 提示「文件不能打开」。但由于无法对二进制文件「钉证」，双击打开还会触发 `Gatekeeper` 保护，需要手动确认才能运行。

但是通过命令行运行就完全不会触发任何的校验

**首次打开需要使用 `sudo wx_video_download` 运行一次，后续双击打开即可**

如果有问题，仍按照下面步骤进行

## 赋予执行权限

```sh
chmod +x ./wx_video_download
```

## 以管理员身份运行

```sh
sudo ./wx_video_download
```


## 允许来自未知来源的应用

若系统提示「文件不能打开」，在系统设置中允许来自未签名开发者的应用

![step1](../assets/enable_step1_macos.png)

再次执行 `./wx_video_download`，可能出现下面窗口，选择 `Open Anyway`

![step2](../assets/enable_step2_macos.png)


## 正常使用

只有首次打开需要经历上述步骤，之后可直接双击 `wx_video_download` 运行，无需繁琐步骤
