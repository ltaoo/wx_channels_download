import { WxChannelsPlayerViewModel } from "./wxchannels.model.js";

export function WxChannelsPlayerView(props) {
  const vm$ = props.store || WxChannelsPlayerViewModel(props);
  if (props.autoplay) {
    queueMicrotask(() => {
      vm$.methods.play({ url: props.url, decode_key: props.decodeKey });
    });
  }

  return View(
    {
      class: ["wxchannels-player", props.class].filter(Boolean).join(" "),
      attributes: {
        n: props.nodeName || "wxchannels-player",
        role: "region",
        "aria-label": props.ariaLabel || "视频号视频播放器",
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      Show({
        when: vm$.state.status,
        ok() {
          return View(
            {
              class: "wxchannels-status",
              attributes: { n: "wxchannels-player-status", role: "status" },
            },
            [vm$.state.status],
          );
        },
      }),

      Show({
        when: vm$.state.playback_url,
        ok() {
          return Timeless.Video({
            class: "group-wxchannels-video",
            src: vm$.state.playback_url.value,
            controls: true,
            autoplay: Boolean(props.autoplay),
            playsInline: true,
            preload: "metadata",
            attributes: { n: "wxchannels-player-media" },
            onMounted(event) {
              vm$.methods.media_mounted(event);
            },
            onError(event) {
              vm$.methods.media_error(event);
            },
          });
        },
      }),
    ],
  );
}
