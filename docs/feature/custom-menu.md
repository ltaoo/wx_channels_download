---
title: 自定义菜单
---

# 自定义菜单

可以在现有「下载按钮」的悬浮菜单中，增加自己的功能


## 增加「下载视频和封面」

在和 `wx_video_download.exe` 相同目录下增加一个 `global.js` 文件，内容如下

```js
// global.js
WXU.unshiftMenuItems([
  {
    label: "下载视频和封面",
    async onClick() {
      var [err, feed] = WXU.check_feed_existing({
        silence: true,
      });
      if (err) return;
      var filename = WXU.build_filename(
        feed,
        feed.spec[0],
        WXU.config.downloadFilenameTemplate
      );
      if (WXU.config.downloadPauseWhenDownload) {
        WXU.pause_cur_video();
      }
      var ins = WXU.loading();
      var [err, response] = await WXU.fetch(feed.url);
      if (err) {
        WXU.error({
          msg: err.message,
        });
        return;
      }
      var media_blob = await WXU.download_with_progress(response, {
        onStart({ total_size }) {
          WXU.log({
            msg: `总大小 ${WXU.bytes_to_size(total_size)}`,
          });
        },
        onProgress({ loaded_size, progress }) {
          WXU.log({
            replace: 1,
            msg:
              progress === null
                ? `${WXU.bytes_to_size(loaded_size)}`
                : `${progress}%`,
          });
        },
      });
      var media_buf = new Uint8Array(await media_blob.arrayBuffer());
      if (feed.key) {
        WXU.log({
          msg: "下载完成，开始解密",
        });
        var [err, data] = await WXU.decrypt_video(media_buf, feed.key);
        if (err) {
          WXU.error({ msg: "解密失败，" + err.message, alert: 0 });
          WXU.error({ msg: "尝试使用 decrypt 命令解密", alert: 0 });
        } else {
          WXU.log({ msg: "解密成功" });
          media_buf = data;
        }
      }
      var decrypted_media_blob = new Blob([media_buf], { type: "video/mp4" });
      var zip = await WXU.Zip();
      zip.file(filename + ".mp4", decrypted_media_blob);
      var cover_url = feed.cover_url.replace(/^http/, "https");
      var [err, cover_response] = await WXU.fetch(cover_url);
      if (err) {
        WXU.error({
          msg: err.message,
        });
        return;
      }
      var cover_blob = await cover_response.blob();
      zip.file(filename + ".jpg", cover_blob);
      var zip_blob = await zip.generateAsync({ type: "blob" });
      WXU.save(zip_blob, filename + ".zip");
      ins.hide();
      if (WXU.config.downloadPauseWhenDownload) {
        WXU.play_cur_video();
      }
    },
  },
]);
```
