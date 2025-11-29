<template>
  <div class="env">
    <span>当前系统：{{ osLabel }} · 架构：{{ archLabel }}</span>
  </div>
  </template>

<script setup>
import { onMounted, ref } from 'vue'

const osLabel = ref('检测中')
const archLabel = ref('检测中')

async function detect() {
  let os = 'Darwin'
  let arch = 'x86_64'
  try {
    if (navigator.userAgentData && navigator.userAgentData.getHighEntropyValues) {
      const high = await navigator.userAgentData.getHighEntropyValues(['platform', 'architecture'])
      const p = (high.platform || '').toLowerCase()
      const a = (high.architecture || '').toLowerCase()
      if (p.includes('win')) os = 'Windows'
      else if (p.includes('mac')) os = 'Darwin'
      else if (p.includes('linux')) os = 'Linux'
      arch = a.includes('arm') ? 'arm64' : 'x86_64'
    } else {
      const ua = navigator.userAgent.toLowerCase()
      const platform = navigator.platform.toLowerCase()
      if (platform.includes('win') || ua.includes('windows')) os = 'Windows'
      else if (platform.includes('mac') || ua.includes('mac os')) os = 'Darwin'
      else if (platform.includes('linux') || ua.includes('linux')) os = 'Linux'
      arch = ua.includes('arm') || ua.includes('apple') ? 'arm64' : 'x86_64'
    }
  } catch {}
  osLabel.value = os === 'Darwin' ? 'macOS' : os
  archLabel.value = arch
}

onMounted(detect)
</script>

<style scoped>
.env { padding: 10px; border: 1px solid var(--vp-c-border); background: var(--vp-c-bg-soft); border-radius: 6px; font-size: 14px; color: var(--vp-c-text-1); }
</style>
