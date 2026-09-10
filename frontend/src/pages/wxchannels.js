import { WxChannelsPlayerViewModel } from "./wxchannels.model.js";

function WxChannelsPlayerPageView(props) {
  const vm$ = WxChannelsPlayerViewModel(props);

  return View(
    {
      class: "content-page wxchannels-page page",
      attributes: { n: "wxchannels-page" },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      View({
        class: "content-main container wxchannels-main",
        attributes: { n: "wxchannels-main" },
      }, [
        View({ class: "wxchannels-panel", attributes: { n: "wxchannels-panel" } }, [
          View({
            class: "wxchannels-heading",
            attributes: { n: "wxchannels-heading" },
          }, [
            View({
              as: "h1",
              class: "wxchannels-title",
              attributes: { n: "wxchannels-title" },
            }, ["视频号播放"]),
            View({
              class: "wxchannels-description",
              attributes: { n: "wxchannels-description" },
            }, ["输入视频 URL 和 decodeKey，前端默认流式解密并播放。"]),
          ]),
          View({
            type: "form",
            class: "wxchannels-form",
            attributes: { n: "wxchannels-form" },
            onSubmit(event) {
              event.preventDefault();
              vm$.methods.submit();
            },
          }, [
            View({
              class: "wxchannels-field",
              attributes: { n: "wxchannels-url-field" },
            }, [
              View({
                as: "label",
                class: "wxchannels-label",
                attributes: {
                  n: "wxchannels-url-label",
                  for: "wxchannels-url-input",
                },
              }, ["视频 URL"]),
              Input({
                store: vm$.ui.input_url$,
                rootAttributes: { n: "wxchannels-url-control" },
                attributes: {
                  n: "wxchannels-url-input",
                  id: "wxchannels-url-input",
                  name: "url",
                  type: "url",
                  autocomplete: "off",
                  spellcheck: "false",
                },
              }),
            ]),
            View({
              class: "wxchannels-field",
              attributes: { n: "wxchannels-key-field" },
            }, [
              View({
                as: "label",
                class: "wxchannels-label",
                attributes: {
                  n: "wxchannels-key-label",
                  for: "wxchannels-key-input",
                },
              }, ["decodeKey"]),
              Input({
                store: vm$.ui.input_decode_key$,
                rootAttributes: { n: "wxchannels-key-control" },
                attributes: {
                  n: "wxchannels-key-input",
                  id: "wxchannels-key-input",
                  name: "decode_key",
                  type: "text",
                  autocomplete: "off",
                  spellcheck: "false",
                },
              }),
            ]),
            Button(
              {
                store: vm$.ui.btn_submit$,
                class: "wxchannels-submit",
                attributes: { n: "wxchannels-submit-action" },
              },
              [vm$.state.submit_text],
            ),
          ]),
          Show({
            when: vm$.state.status,
            ok() {
              return View({
                class: "wxchannels-status",
                attributes: { n: "wxchannels-status", role: "status" },
              }, [vm$.state.status]);
            },
          }),
          Show({
            when: vm$.state.error,
            ok() {
              return View({
                class: "wxchannels-error",
                attributes: { n: "wxchannels-error", role: "alert" },
              }, [vm$.state.error]);
            },
          }),
        ]),
        Show({
          when: vm$.state.playback_url,
          ok() {
            return [
              Show({
                when: vm$.state.stream_playback,
                ok() {
                  return View({
                    class: "wxchannels-player",
                    attributes: { n: "wxchannels-player" },
                  }, [
                    Timeless.Video({
                      class: "wxchannels-video",
                      controls: true,
                      playsInline: true,
                      preload: "auto",
                      attributes: { n: "wxchannels-video-media" },
                      onMounted(event) {
                        vm$.methods.mount_stream_player(event);
                      },
                      onUnmounted() {
                        vm$.methods.unmount_stream_player();
                      },
                      onError(event) {
                        vm$.methods.media_error(event);
                      },
                    }),
                  ]);
                },
              }),
              Show({
                when: Timeless.combine(
                  {
                    playback_url: vm$.state.playback_url,
                    stream_playback: vm$.state.stream_playback,
                  },
                  (state) => Boolean(state.playback_url) && !state.stream_playback,
                ),
                ok() {
                  return View({
                    class: "wxchannels-player",
                    attributes: { n: "wxchannels-player" },
                  }, [
                    Timeless.Video({
                      class: "wxchannels-video",
                      src: vm$.state.playback_url.value,
                      controls: true,
                      playsInline: true,
                      preload: "metadata",
                      attributes: { n: "wxchannels-video-media" },
                      onError(event) {
                        vm$.methods.media_error(event);
                      },
                    }),
                  ]);
                },
              }),
            ];
          },
        }),
      ]),
    ],
  );
}

export default WxChannelsPlayerPageView;
