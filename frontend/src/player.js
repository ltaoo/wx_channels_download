const decrypt_player_module_url = new URL(
  "../public/player/decrypt-video-core.js",
  import.meta.url,
);
const decrypt_core_assets = {
  workerUrl: new URL(
    "decrypt-video-core/1.3.0/worker_release.js",
    document.baseURI,
  ),
  vtsWasmUrl: new URL(
    "decrypt-video-core/1.3.0/wasm_video_decode.wasm",
    document.baseURI,
  ),
  vtsJsUrl: new URL(
    "decrypt-video-core/1.3.0/wasm_video_decode.js",
    document.baseURI,
  ),
};

let decrypt_player_module = null;
let decrypt_player_pool = null;

function version_asset_url(url) {
  const version = String(
    (window.__d_config && window.__d_config.version) || "",
  ).trim();
  if (version) url.searchParams.set("v", version);
  return url.href;
}

async function load_decrypt_player_module() {
  if (!window.MediaSource || !window.WebAssembly) {
    throw new Error("当前浏览器不支持 MSE 流式播放");
  }
  if (decrypt_player_module) return decrypt_player_module;

  const module = await import(version_asset_url(decrypt_player_module_url));
  const core = module.d;
  if (!core || !core.DecryptVideoPool || !core.setDefaultUrl) {
    throw new Error("流式解密播放器加载失败");
  }

  core.setDefaultUrl({
    workerUrl: version_asset_url(decrypt_core_assets.workerUrl),
    vtsWasmUrl: version_asset_url(decrypt_core_assets.vtsWasmUrl),
    vtsJsUrl: version_asset_url(decrypt_core_assets.vtsJsUrl),
  });
  decrypt_player_module = core;
  return core;
}

export async function prepare_wxchannels_player() {
  const core = await load_decrypt_player_module();
  decrypt_player_pool ??= new core.DecryptVideoPool({
    workerLimitNum: 1,
  });
}

export async function mount_wxchannels_player(video, input, callbacks) {
  await prepare_wxchannels_player();
  const player = await decrypt_player_pool.getDecryptCore({
    url: input.url,
    seed: input.key,
    mediaElement: video,
    minBufferedTime: 15,
    maxBufferedTime: 60,
    maxBackwardBufferedTime: 60,
    minBackwardBufferedTime: 30,
    ffmpegConfig: {
      segmentDuration: 1,
    },
    networkConfig: {
      firstBufSize: 1024 * 1024,
      seekFirstBufSize: 512 * 1024,
      concurrentNum: 2,
      winBufSize: 1024 * 1024,
      firstBufTimeout: 10000,
      timeout: 10000,
      retryTimeout: 10000,
    },
    errorCallback: callbacks.onError,
    firstSegmentDownloadCallback: callbacks.onFirstSegmentDownload,
    firstSegmentRemuxCallback: callbacks.onFirstSegmentRemux,
    onDispose: callbacks.onDispose,
  });

  let disposed = false;
  const session = {
    async dispose() {
      if (disposed) return;
      disposed = true;
      await player.dispose();
    },
  };

  await player.load();
  return session;
}

export async function dispose_wxchannels_player_pool() {
  const pool = decrypt_player_pool;
  decrypt_player_pool = null;
  await pool?.terminate();
}
