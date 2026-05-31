<template>
  <ClientOnly>
    <div class="VPHero VPHomeHero" v-bind="$attrs">
      <div class="container">
        <div class="main">
          <!-- 名称 -->
          <h1 v-if="name" class="name">
            <span class="clip">{{ name }}</span>
          </h1>

          <!-- 标语 -->
          <p v-if="tagline" class="tagline">{{ tagline }}</p>

          <!-- 操作按钮 -->
          <div class="actions">
            <!-- 快速开始 -->
            <a class="VPButton brand" :href="withBase(startLink)">
              {{ startText }}
            </a>

            <!-- 下载按钮 -->
            <a
              v-if="data && primaryAsset"
              class="VPButton alt download-btn"
              :href="primaryAsset.url"
            >
              <svg class="download-icon" viewBox="0 0 24 24" width="16" height="16">
                <path fill="currentColor" d="M12 16l-5-5h3V4h4v7h3l-5 5zm-7 3h14v2H5v-2z"/>
              </svg>
              下载 {{ platformLabel }} 版
            </a>
            <a v-else class="VPButton alt" :href="withBase(startLink)">
              下载最新版
            </a>

            <!-- 版本标记 -->
            <span v-if="data" class="hero-version">最新 {{ data.tag }}</span>
          </div>
        </div>
      </div>

      <!-- 特色说明 -->
      <div v-if="features.length" class="VPFeatures">
        <div class="features-container">
          <div class="items">
            <div v-for="f in features" :key="f.title" class="item" :class="gridClass">
              <article class="box">
                <h2 class="feature-title">{{ f.title }}</h2>
                <p class="feature-detail">{{ f.details }}</p>
              </article>
            </div>
          </div>
        </div>
      </div>
    </div>
  </ClientOnly>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { withBase } from 'vitepress'

defineOptions({ inheritAttrs: false })

interface Feature {
  title: string
  details: string
}

const props = withDefaults(defineProps<{
  name?: string
  tagline?: string
  startText?: string
  startLink?: string
  features?: Feature[]
}>(), {
  name: '',
  tagline: '',
  startText: '快速开始',
  startLink: '/guide/start',
  features: () => [],
})

// 构建时由 config.ts vite.define 注入
declare const __RELEASE_DATA__: {
  tag: string
  url: string
  assets: Array<{ name: string; url: string; size: number }>
} | null

const data = __RELEASE_DATA__
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
  if ('userAgentData' in navigator) {
    try {
      const uaData = (navigator as any).userAgentData
      if (uaData.getHighEntropyValues) {
        const high = await uaData.getHighEntropyValues(['architecture'])
        const arch = (high.architecture || '').toLowerCase()
        if (arch.includes('arm') || arch.includes('aarch')) return 'arm64'
        return 'x86_64'
      }
    } catch { /* fall through */ }
  }
  const ua = navigator.userAgent.toLowerCase()
  if (ua.includes('arm') || ua.includes('aarch64')) return 'arm64'
  return 'x86_64'
}

// ---------- 网格布局 ----------

const gridClass = computed(() => {
  const len = props.features.length
  if (len === 2) return 'grid-2'
  if (len === 3) return 'grid-3'
  if (len % 3 === 0) return 'grid-6'
  if (len > 3) return 'grid-4'
  return ''
})

// ---------- 资产匹配 ----------

interface Asset {
  name: string
  url: string
  size: number
}

function findAsset(os: string, arch: string): Asset | null {
  if (!data?.assets) return null
  const osStr = { macos: 'darwin', windows: 'windows', linux: 'linux' }[os] || os
  const list = data.assets.filter(
    (a) => a.name.includes(`_${osStr}_`) && a.name.includes(`_${arch}`) && !a.name.includes('_safe_')
  )
  return list[0] || null
}

// ---------- 计算属性 ----------

const platformLabel = computed(() => {
  const map: Record<string, string> = { macos: 'macOS', windows: 'Windows', linux: 'Linux' }
  return map[detectedPlatform.value || ''] || detectedPlatform.value || ''
})

const primaryAsset = computed(() => {
  if (!detectedPlatform.value || !detectedArch.value) return null
  return findAsset(detectedPlatform.value, detectedArch.value)
})

// ---------- 挂载 ----------

onMounted(async () => {
  detectedPlatform.value = detectPlatform()
  detectedArch.value = await detectArch()
})
</script>

<style scoped>
/* 复用 VitePress 的 hero 类名以继承主题样式 */
.VPHero :deep(.container) {
  display: flex;
  justify-content: center;
}
.VPHero :deep(.main) {
  text-align: center;
  max-width: 640px;
}
.VPHero :deep(.name) {
  font-size: 56px;
  font-weight: 800;
  line-height: 64px;
  color: var(--vp-home-hero-name-color);
  margin-bottom: 16px;
}
.VPHero :deep(.tagline) {
  font-size: 24px;
  line-height: 32px;
  color: var(--vp-c-text-2);
  margin-bottom: 32px;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: 12px;
}

/* 复用 VitePress VPButton 样式 */
.VPButton {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0 22px;
  line-height: 38px;
  font-size: 14px;
  font-weight: 500;
  border-radius: 20px;
  border: 1px solid transparent;
  text-decoration: none;
  transition: background-color 0.25s, border-color 0.25s;
  white-space: nowrap;
}
.VPButton.brand {
  background-color: var(--vp-button-brand-bg);
  color: var(--vp-button-brand-text);
}
.VPButton.brand:hover {
  background-color: var(--vp-button-brand-hover-bg);
}
.VPButton.alt {
  background-color: var(--vp-button-alt-bg);
  color: var(--vp-button-alt-text);
}
.VPButton.alt:hover {
  background-color: var(--vp-button-alt-hover-bg);
  color: var(--vp-button-alt-hover-text);
}

.download-btn {
  font-weight: 600;
}
.download-icon {
  flex-shrink: 0;
}

.hero-version {
  font-size: 12px;
  color: var(--vp-c-text-3);
  line-height: 38px;
}

/* ---------- 特色说明（匹配 VitePress VPFeature 样式） ---------- */

.VPFeatures {
  margin-top: 40px;
  padding: 0 24px;
}
.features-container {
  margin: 0 auto;
  max-width: 1152px;
}

@media (min-width: 640px) {
  .VPFeatures {
    padding: 0 48px;
  }
}

@media (min-width: 960px) {
  .VPFeatures {
    padding: 0 64px;
  }
}
.items {
  display: flex;
  flex-wrap: wrap;
  margin: -8px;
}
.item {
  padding: 8px;
  width: 100%;
}

@media (min-width: 640px) {
  .item.grid-2,
  .item.grid-4,
  .item.grid-6 {
    width: calc(100% / 2);
  }
}

@media (min-width: 768px) {
  .item.grid-2,
  .item.grid-4 {
    width: calc(100% / 2);
  }
  .item.grid-3,
  .item.grid-6 {
    width: calc(100% / 3);
  }
}

@media (min-width: 960px) {
  .item.grid-4 {
    width: calc(100% / 4);
  }
}

.box {
  display: flex;
  flex-direction: column;
  padding: 24px;
  height: 100%;
  border: 1px solid var(--vp-c-bg-soft);
  border-radius: 12px;
  background-color: var(--vp-c-bg-soft);
  transition: border-color 0.25s, background-color 0.25s;
}

.feature-title {
  margin: 0;
  border: 0;
  line-height: 24px;
  font-size: 16px;
  font-weight: 600;
  color: var(--vp-c-text-1);
}

.feature-detail {
  flex-grow: 1;
  padding-top: 8px;
  line-height: 24px;
  font-size: 14px;
  font-weight: 500;
  color: var(--vp-c-text-2);
}

/* 移动端适配 */
@media (max-width: 640px) {
  .VPHero :deep(.name) {
    font-size: 36px;
    line-height: 44px;
  }
  .VPHero :deep(.tagline) {
    font-size: 18px;
    line-height: 26px;
  }
}
</style>
