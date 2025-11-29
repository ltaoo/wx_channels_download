<template>
  <div class="dbox">
    <div class="row">
      <select v-model="selectedOS" class="select">
        <option value="Darwin">macOS</option>
        <option value="Windows">Windows</option>
        <option value="Linux">Linux</option>
      </select>
      <select v-model="selectedArch" class="select">
        <option v-for="a in archOptions" :key="a" :value="a">{{ a }}</option>
      </select>
      <a
        :href="downloadUrl || releasePageUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="btn"
      >
        {{ buttonText }}
      </a>
    </div>
    <div v-if="statusText" class="status">{{ statusText }}</div>
  </div>
  </template>

<script setup>
import { onMounted, ref, watch } from 'vue'

const releasePageUrl = 'https://github.com/ltaoo/wx_channels_download/releases'
const buttonText = ref('下载最新版本')
const statusText = ref('')
const downloadUrl = ref('')
const selectedOS = ref('Darwin')
const selectedArch = ref('x86_64')
const archOptions = ref(['x86_64', 'arm64'])
let latestAssets = []
let latestTag = ''

function detectOS() {
  const ua = navigator.userAgent.toLowerCase()
  const platform = navigator.platform.toLowerCase()
  const isMac = platform.includes('mac') || ua.includes('mac os')
  const isWin = platform.includes('win') || ua.includes('windows')
  const isLinux = platform.includes('linux') || ua.includes('linux')
  if (isMac) return 'Darwin'
  if (isWin) return 'Windows'
  if (isLinux) return 'Linux'
  return 'Darwin'
}

function preferredArch(os) {
  const ua = navigator.userAgent.toLowerCase()
  const isAppleSiliconHint = ua.includes('arm') || ua.includes('apple')
  if (os === 'Darwin') return isAppleSiliconHint ? ['arm64', 'x86_64'] : ['x86_64', 'arm64']
  if (os === 'Windows') return ['x86_64', 'arm64']
  return ['x86_64', 'arm64']
}

function preferredExt(os) {
  if (os === 'Windows') return ['.zip']
  return ['.tar.gz', '.zip']
}

function selectAsset(assets, os) {
  const archOrder = preferredArch(os)
  const extOrder = preferredExt(os)
  const candidates = assets.filter(a => {
    const n = a.name || ''
    return n.includes(os)
  })
  const tryPick = (arch, ext) => candidates.find(a => (a.name || '').includes(arch) && (a.name || '').endsWith(ext))
  for (const ext of extOrder) {
    const byManual = tryPick(selectedArch.value, ext)
    if (byManual) return byManual
    for (const arch of archOrder) {
      const hit = tryPick(arch, ext)
      if (hit) return hit
    }
    const any = candidates.find(a => (a.name || '').endsWith(ext))
    if (any) return any
  }
  return candidates[0] || null
}

async function fetchLatest() {
  try {
    const os = detectOS()
    selectedOS.value = os
    buttonText.value = `下载 (${os})`
    statusText.value = '正在获取最新版本…'
    const res = await fetch('https://api.github.com/repos/ltaoo/wx_channels_download/releases/latest', {
      headers: { Accept: 'application/vnd.github+json' }
    })
    const data = await res.json()
    latestAssets = Array.isArray(data.assets) ? data.assets : []
    latestTag = data.tag_name || ''
    refreshSelection()
  } catch (e) {
    statusText.value = '获取失败，请前往 Releases 页面下载'
  }
}

function refreshSelection() {
  const os = selectedOS.value
  const asset = selectAsset(latestAssets, os)
  if (asset && asset.browser_download_url) {
    downloadUrl.value = asset.browser_download_url
    buttonText.value = `下载 (${os}, ${selectedArch.value})`
    statusText.value = `版本 ${latestTag} · ${asset.name}`
  } else {
    downloadUrl.value = ''
    statusText.value = '未找到适配构建，请尝试切换架构或前往 Releases 页面下载'
  }
}

watch([selectedOS, selectedArch], refreshSelection)
onMounted(fetchLatest)
</script>

<style scoped>
.dbox { margin-top: 16px; }
.row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.select { padding: 8px 10px; border-radius: 6px; border: 1px solid var(--vp-c-border); background: var(--vp-c-bg-soft); color: var(--vp-c-text-1); }
.btn { display:inline-block; padding:8px 16px; border-radius:6px; background: var(--vp-c-brand); color: var(--vp-c-white); text-decoration:none; }
.btn:hover { opacity: 0.9; }
.status { margin-top:8px; font-size:12px; color: var(--vp-c-text-2); }
</style>
