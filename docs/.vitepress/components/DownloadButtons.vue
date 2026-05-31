<template>
  <div class="download-section">
    <!-- 数据获取失败时的降级 -->
    <div v-if="!data" class="fallback">
      <p>无法获取最新版本信息，请前往 GitHub Releases 页面下载：</p>
      <a
        href="https://github.com/ltaoo/wx_channels_download/releases"
        target="_blank"
        class="fallback-link"
      >
        前往 GitHub Releases
      </a>
    </div>

    <!-- 正常展示 -->
    <template v-else>
      <!-- 版本信息 -->
      <div class="version-header" style="margin-bottom: 16px; text-align: center;">
        <span class="version-badge">{{ data.tag }}</span>
      </div>

      <!-- 主下载按钮（延迟到客户端水合后，根据检测到的平台展示） -->
      <div v-if="mounted" class="primary-section">
        <div v-if="primaryAsset" class="" style="display: flex; align-items: center; gap: 12px;">
          <a :href="primaryAsset.url" class="primary-btn">
            <svg class="btn-download-icon" viewBox="0 0 24 24" width="18" height="18">
              <path fill="currentColor" d="M12 16l-5-5h3V4h4v7h3l-5 5zm-7 3h14v2H5v-2z"/>
            </svg>
            下载 {{ platformLabel }} 版
          </a>
          <div class="primary-meta">
            <span>{{ primaryAsset.name }}</span>
            <span v-if="primaryAsset.humanSize" class="file-size">{{ primaryAsset.humanSize }}</span>
          </div>
        </div>
        <div v-else class="primary-download">
          <span class="no-asset">当前平台暂无构建，请从下方选择其他平台</span>
        </div>
      </div>

      <!-- 三大平台下载卡片 -->
      <div class="platform-title">{{ mounted ? '其他平台下载' : '选择您的平台下载' }}</div>
      <div class="platform-cards">
          <div
            v-for="p in platforms"
            :key="p.key"
            class="platform-card"
            :class="{ 'is-active': p.key === detectedPlatform }"
          >
            <div class="card-platform">
              <span class="card-icon">
                <svg v-if="p.key === 'macos'" viewBox="0 0 24 24" width="24" height="24">
                  <path fill="currentColor" d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.8-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z"/>
                </svg>
                <svg v-else-if="p.key === 'windows'" viewBox="0 0 24 24" width="24" height="24">
                  <path fill="currentColor" d="M3 12V6.5l8-1.09V12H3zm0 .73V18l8 1.09v-6.36H3zM12 5.15L21 3v9h-9V5.15zM21 12.73V21l-9-1.27v-7H21z"/>
                </svg>
                <svg v-else viewBox="0 0 24 24" width="24" height="24">
                  <path fill="currentColor" d="M12.506 3.023c-.271-.03-.543-.03-.814 0C10.056 3.306 8.59 4.1 7.484 5.38 6.353 6.68 5.64 8.53 5.64 11.06c0 3.207.975 5.807 2.566 7.537 1.564 1.693 3.636 2.302 5.3 1.884.524-.131.776-.214 1.025-.291l.002-.003c.283-.09.518-.163.958-.2.573-.05 1.09.016 1.623.124.395.081.878.25 1.61.585l.02.01c.434.198 1.065.46 1.616.46.355 0 .622-.14.813-.393.206-.271.303-.688.187-1.25-.188-.91-.674-1.67-1.346-2.198-.322-.254-.685-.435-1.068-.546l-.014-.004c-.265-.076-.513-.147-.727-.238-.208-.088-.437-.214-.677-.428l-.003-.004c-.31-.287-.598-.682-.777-1.174-.188-.518-.247-1.074-.114-1.626.118-.492.326-.802.611-1.069.394-.37.93-.622 1.555-.756l.014-.003c.501-.106.988-.102 1.5-.02.165.026.47.096.738.179l.006.001c.283.088.543.158.81.158.35 0 .64-.115.835-.352.22-.27.333-.67.241-1.195-.188-1.074-.733-1.866-1.522-2.416-.738-.513-1.714-.805-2.702-.92l-.023-.002c-1.46-.173-2.82-.1-3.748.471-.288.177-.545.39-.78.64l-.01.01c-.285.298-.525.65-.767 1.025l-.007.01c-.244.382-.476.743-.92.92l-.023.008c-.307.118-.655.15-1.01.083-.419-.08-.84-.314-1.214-.621l-.045-.035C15.031 7.06 13.9 6.67 12.506 3.023z"/>
                </svg>
              </span>
              <span class="card-name">{{ p.name }}</span>
            </div>
            <div class="card-archs">
              <a
                v-for="a in p.archs"
                :key="a.key"
                v-show="a.url"
                :href="a.url"
                class="arch-link"
              >
                {{ a.label }}
              </a>
              <span
                v-for="a in p.archs"
                :key="a.key + '-na'"
                v-show="!a.url"
                class="arch-none"
              >
                {{ a.label }}
              </span>
            </div>
          </div>
        </div>

        <!-- 底部链接 -->
        <div class="footer-links">
          <a :href="data.url" target="_blank" rel="noopener">在 GitHub 查看所有版本</a>
        </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

// 构建时由 config.ts vite.define 注入
declare const __RELEASE_DATA__: {
  tag: string
  url: string
  assets: Array<{ name: string; url: string; size: number }>
} | null

const data = __RELEASE_DATA__
const mounted = ref(false)
const detectedPlatform = ref<string | null>(null)
const detectedArch = ref<string | null>(null)

// ---------- 平台检测 ----------

function detectPlatform() {
  const ua = navigator.userAgent.toLowerCase()
  const platform = (navigator.platform || '').toLowerCase()

  if (platform.includes('mac') || ua.includes('mac os')) return 'macos'
  if (platform.includes('win') || ua.includes('windows')) return 'windows'
  if (platform.includes('linux') || ua.includes('linux')) return 'linux'
  return null
}

async function detectArch(): Promise<string | null> {
  // 优先使用 UserAgentData API
  if ('userAgentData' in navigator) {
    try {
      const uaData = navigator.userAgentData as any
      if (uaData.getHighEntropyValues) {
        const high = await uaData.getHighEntropyValues(['architecture'])
        const arch = (high.architecture || '').toLowerCase()
        if (arch.includes('arm') || arch.includes('aarch')) return 'arm64'
        return 'x86_64'
      }
    } catch { /* fall through */ }
  }

  // 降级：从 UA 推断
  const ua = navigator.userAgent.toLowerCase()
  if (ua.includes('arm') || ua.includes('aarch64')) return 'arm64'
  return 'x86_64'
}

// ---------- 资产匹配 ----------

interface Asset {
  name: string
  url: string
  size: number
  humanSize?: string
}

function formatSize(bytes: number): string {
  const mb = bytes / 1024 / 1024
  if (mb >= 1) return `${mb.toFixed(1)} MB`
  return `${(bytes / 1024).toFixed(1)} KB`
}

function findAsset(os: string, arch: string, variant?: string): Asset | null {
  if (!data?.assets) return null

  const osStr = { macos: 'darwin', windows: 'windows', linux: 'linux' }[os] || os

  let list = data.assets.filter(
    (a) => a.name.includes(`_${osStr}_`) && a.name.includes(`_${arch}`)
  )

  if (variant === 'safe') {
    list = list.filter((a) => a.name.includes('_safe_'))
  } else {
    list = list.filter((a) => !a.name.includes('_safe_'))
  }

  if (list.length === 0) return null

  // Darwin 和 Windows 优先 .zip, Linux .tar.gz
  const asset = list[0]
  return { ...asset, humanSize: formatSize(asset.size) }
}

// ---------- 计算属性 ----------

const releaseDate = computed(() => {
  if (!data?.tag) return ''
  // tag_name 如 "v260531" -> "260531"
  return data.tag.replace(/^v/, '')
})

const platformLabel = computed(() => {
  const map: Record<string, string> = { macos: 'macOS', windows: 'Windows', linux: 'Linux' }
  return map[detectedPlatform.value || ''] || detectedPlatform.value || ''
})

const primaryAsset = computed(() => {
  if (!detectedPlatform.value || !detectedArch.value) return null
  return findAsset(detectedPlatform.value, detectedArch.value)
})

// ---------- 平台卡片数据 ----------

const platforms = computed(() => {
  return [
    {
      key: 'macos',
      name: 'macOS',
      archs: [
        {
          key: 'macos-arm64',
          label: 'Apple Silicon',
          url: findAsset('macos', 'arm64')?.url || null,
        },
        {
          key: 'macos-x86_64',
          label: 'Intel',
          url: findAsset('macos', 'x86_64')?.url || null,
        },
      ],
    },
    {
      key: 'windows',
      name: 'Windows',
      archs: [
        {
          key: 'windows-x86_64',
          label: '64 位',
          url: findAsset('windows', 'x86_64')?.url || null,
        },
        {
          key: 'windows-x86_64-safe',
          label: '64 位 (免压缩)',
          url: findAsset('windows', 'x86_64', 'safe')?.url || null,
        },
        {
          key: 'windows-arm64',
          label: 'ARM64',
          url: findAsset('windows', 'arm64')?.url || null,
        },
      ],
    },
    {
      key: 'linux',
      name: 'Linux',
      archs: [
        {
          key: 'linux-x86_64',
          label: '64 位',
          url: findAsset('linux', 'x86_64')?.url || null,
        },
        {
          key: 'linux-arm64',
          label: 'ARM64',
          url: findAsset('linux', 'arm64')?.url || null,
        },
      ],
    },
  ]
})

// ---------- 挂载 ----------

onMounted(async () => {
  detectedPlatform.value = detectPlatform()
  detectedArch.value = await detectArch()
  mounted.value = true
})
</script>

<style scoped>
.download-section {
  margin-top: 24px;
  padding: 28px;
  border-radius: 12px;
  background: var(--vp-c-bg-soft);
}

/* ---------- 降级 ---------- */

.fallback {
  text-align: center;
  color: var(--vp-c-text-2);
  padding: 16px 0;
}
.fallback p {
  margin-bottom: 12px;
}
.fallback-link {
  display: inline-block;
  padding: 8px 20px;
  border-radius: 6px;
  background: var(--vp-c-brand);
  color: var(--vp-c-white);
  text-decoration: none;
  font-weight: 500;
}
.fallback-link:hover {
  opacity: 0.9;
}

/* ---------- 版本头 ---------- */

.version-header {
  display: flex;
  align-items: center;
  gap: 12px;
}
.version-badge {
  display: inline-block;
  padding: 3px 10px;
  border-radius: 4px;
  background: var(--vp-c-brand-soft);
  color: var(--vp-c-brand);
  font-size: 13px;
  font-weight: 600;
}
.release-note-link {
  font-size: 13px;
}
.release-note-link a {
  color: var(--vp-c-text-2);
  text-decoration: none;
}
.release-note-link a:hover {
  color: var(--vp-c-brand);
}

/* ---------- 主下载区域 ---------- */
.vp-doc a:hover {
  color: unset;
}

.primary-section {
  margin-bottom: 24px;
}
.system-info {
  font-size: 14px;
  color: var(--vp-c-text-2);
  margin-bottom: 12px;
}
.system-info strong {
  color: var(--vp-c-text-1);
}

.primary-download {
  display: flex;
  align-items: center;
  gap: 8px;
}
.primary-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0 24px;
  line-height: 44px;
  border-radius: 20px;
  border: 1px solid transparent;
  background-color: var(--vp-button-brand-bg);
  color: var(--vp-button-brand-text);
  text-decoration: none;
  font-size: 14px;
  font-weight: 500;
  transition: background-color 0.25s, border-color 0.25s;
  width: fit-content;
}

.btn-download-icon {
  flex-shrink: 0;
}
.primary-meta {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--vp-c-text-3);
}
.no-asset {
  font-size: 14px;
  color: var(--vp-c-text-2);
}

/* ---------- 平台卡片 ---------- */

.platform-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--vp-c-text-2);
  margin-bottom: 12px;
}
.platform-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-bottom: 20px;
}
@media (max-width: 640px) {
  .platform-cards {
    grid-template-columns: 1fr;
  }
}
.platform-card {
  padding: 16px;
  border: 1px solid var(--vp-c-border);
  border-radius: 8px;
  background: var(--vp-c-bg);
  transition: border-color 0.2s;
}
.platform-card.is-active {
  border-color: var(--vp-c-brand);
}
.card-platform {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 10px;
}
.card-icon {
  color: var(--vp-c-text-2);
  display: flex;
}
.card-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--vp-c-text-1);
}
.card-archs {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.arch-link {
  display: block;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 13px;
  color: var(--vp-c-brand);
  text-decoration: none;
  transition: background 0.2s;
}
.arch-link:hover {
  background: var(--vp-c-brand-soft);
}
.arch-none {
  display: block;
  padding: 4px 8px;
  font-size: 13px;
  color: var(--vp-c-text-3);
}

/* ---------- 底部链接 ---------- */

.footer-links {
  text-align: center;
  padding-top: 8px;
  border-top: 1px solid var(--vp-c-border);
}
.footer-links a {
  font-size: 13px;
  color: var(--vp-c-text-2);
  text-decoration: none;
}
.footer-links a:hover {
  color: var(--vp-c-brand);
}
</style>
