const decrypt_player_module_url = new URL(
  "../public/player/decrypt-video-core.js",
  import.meta.url,
);
const decrypt_core_assets = {
  workerUrl: new URL(
    "public/decrypt-video-core/1.3.0/worker_release.js",
    document.baseURI,
  ),
  vtsWasmUrl: new URL(
    "public/decrypt-video-core/1.3.0/wasm_video_decode.wasm",
    document.baseURI,
  ),
  vtsJsUrl: new URL(
    "public/decrypt-video-core/1.3.0/wasm_video_decode.js",
    document.baseURI,
  ),
};

let decrypt_player_module = null;
let decrypt_player_pool = null;

/** Estimate LRU cache size based on video duration (in seconds). */
function lru_cache_size(duration) {
  const seconds = Number(duration) || 0;
  if (seconds <= 0) return 50 * 1024 * 1024;
  // ~1 MB per 10 seconds, clamped to 20-200 MB
  const estimated = Math.ceil(seconds / 10) * 1024 * 1024;
  return Math.max(20 * 1024 * 1024, Math.min(estimated, 200 * 1024 * 1024));
}

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
    seed: Number(input.key),
    mediaElement: video,
    minBufferedTime: 45,
    maxBufferedTime: 60,
    maxBackwardBufferedTime: 60,
    minBackwardBufferedTime: 30,
    cacheConfig: {
      useLruCache: true,
      lruCacheSize: lru_cache_size(input.duration),
    },
    ffmpegConfig: {
      segmentDuration: 1,
    },
    networkConfig: {
      firstBufSize: 256 * 1024,
      seekFirstBufSize: 512 * 1024,
      concurrentNum: 1,
      winBufSize: 256 * 1024,
      firstBufTimeout: 10000,
      timeout: 6000,
      retryTimeout: 6000,
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
