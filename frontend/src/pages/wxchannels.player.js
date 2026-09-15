import { WxChannelsPlayerViewModel } from "./wxchannels.model.js";

function WxChannelsPlayerView(props) {
  console.log("[wxch-player-view] mount", { url: props.url, decodeKey: props.decodeKey, autoplay: props.autoplay, hasStore: Boolean(props.store) });
  const owns_model = !props.store;
  const vm$ = props.store || WxChannelsPlayerViewModel();
  if (props.autoplay) {
    queueMicrotask(() => {
      console.log("[wxch-player-view] autoplay → calling play()");
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
        if (owns_model) vm$.methods.destroy();
      },
    },
    [
      Show({
        when: vm$.state.status,
        ok() {
          return View({
            class: "wxchannels-status",
            attributes: { n: "wxchannels-player-status", role: "status" },
          }, [vm$.state.status]);
        },
      }),
      Show({
        when: vm$.state.error,
        ok() {
          return View({
            class: "wxchannels-error",
            attributes: { n: "wxchannels-player-error", role: "alert" },
          }, [vm$.state.error]);
        },
      }),
      Show({
        when: vm$.state.stream_playback,
        ok() {
          console.log("[wxch-player-view] rendering stream video element");
          return Timeless.Video({
            class: "wxchannels-video",
            controls: true,
            autoplay: true,
            playsInline: true,
            attributes: { n: "wxchannels-player-media" },
            onMounted(event) {
              console.log("[wxch-player-view] stream video onMounted, calling mount_stream_player");
              vm$.methods.mount_stream_player(event);
            },
            onError(event) {
              vm$.methods.media_error(event);
            },
          });
        },
      }),
      Show({
        when: computed(
          { stream: vm$.state.stream_playback, url: vm$.state.playback_url },
          (state) => !state.stream && Boolean(state.url),
        ),
        ok() {
          console.log("[wxch-player-view] rendering blob video, src:", vm$.state.playback_url.value);
          return Timeless.Video({
            class: "wxchannels-video",
            src: vm$.state.playback_url.value,
            controls: true,
            autoplay: Boolean(props.autoplay),
            playsInline: true,
            preload: "metadata",
            attributes: { n: "wxchannels-player-media" },
            onError(event) {
              vm$.methods.media_error(event);
            },
          });
        },
      }),
    ],
  );
}

export default WxChannelsPlayerView;
