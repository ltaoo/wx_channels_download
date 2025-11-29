---
title: macOS 启用
---

# macOS 启用

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
