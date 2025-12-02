---
title: 长视频下载
---

# 长视频下载

由于网页限制（下载超过一定时间会中断），超过30分钟的视频建议使用命令行 `download` 进行下载。

## 本地中转下载

参考 [本地下载中转服务](../config/download.md#本地下载中转服务)

## 命令行下载

可以多线程下载，速度更快

在「视频详情页」的「更多」菜单，如下所示，点击「打印下载命令」

![长视频下载建议](../assets/faq1.png)

在终端会打印下载命令

![长视频下载建议](../assets/faq2.png)

其中 `[FRONTEND]download --url "` 即下载命令，从 `download` 开始复制，直到结尾

```bash
download --url "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eez3Y79SxtvVL0L7CkPM6dFibFeI6caGYwFEC4864roibGczGjuApTjyib3umVSgzI8sLibE4EUwkVwbDKEymHBRUG34yjM6SGcv44EtQoD1EUKQ89KzkUojUNb7Mick3Rb0GC5mMxko1oaX1Sg&hy=SH&idx=1&m=3bd9398bcd242a67bf1efe7969fd9512&uzid=7a1b6&token=AxricY7RBHdVsH2yJZpBVrjHLkJZLfcm8ueibLvFWicoHL5UuJvfRVmj1iccsf8QqpaDsZdbNlPkdCcutAQHDVyFUcqEdSkyxBOfZ6MCZJkicWGyLia2tKbB3tccbUnmbAAdBTzicgdQ1dCz7yO7fYWUBIbnMKh5DblO5Z7PeMiarYRsZP8&basedata=CAESABoDeFYwIgAqBwiLKRAAGAI&sign=UfS5lzr9DtNQPWgtdPllIp8py4n7mS1iGm-x05-PRo9277wxdW-rKfXynPal2RYNHAT3UX0wIKZmpbwPTOJEJQ&ctsc=146&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAAD9m17FKaAt3BlsfrYqaRAAAADnaHZTnGbFfAj9RgZXfw6VOZcy4UmLLbWjFXTyPelvE6lGZXAEivENDjacRqyPSpY9djoTRJDFxNM%3D&svrnonce=1764406910&X-snsvideoflag=xWT111" --key 1233185028 --filename "#cosplay #走路摇 #永劫无间-xWT111.mp4"
```

打开新的终端，将「下载器」拖动到终端，空格，粘贴，然后终端会类似这样

```bash
wx_video_download download --url "https://xxxx
```

回车即可开始下载

下载成功后，在终端会显示下载好的视频地址
