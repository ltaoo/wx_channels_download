<template>
  <div>
    <div v-if="items.length === 0 && loaded" class="empt">暂无数据，请前往 Issues 查看更多</div>
    <ul v-else class="list">
      <li v-for="it in items" :key="it.id">
        <a :href="it.html_url" target="_blank" rel="noopener noreferrer">#{{ it.number }} · {{ it.title }}</a>
      </li>
    </ul>
    <div class="more"><a :href="issuesUrl" target="_blank" rel="noopener noreferrer">查看全部 Issues</a></div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'

const issuesUrl = 'https://github.com/ltaoo/wx_channels_download/issues'
const items = ref([])
const loaded = ref(false)

async function load() {
  try {
    const res = await fetch('https://api.github.com/repos/ltaoo/wx_channels_download/issues?state=open&per_page=5', {
      headers: { Accept: 'application/vnd.github+json' }
    })
    const data = await res.json()
    items.value = Array.isArray(data) ? data : []
  } catch (e) {
  } finally {
    loaded.value = true
  }
}

onMounted(load)
</script>

<style scoped>
.list { list-style: none; padding: 0; margin: 8px 0; }
.list li { margin: 6px 0; }
.empt { color: var(--vp-c-text-2); font-size: 13px; }
.more { margin-top: 8px; font-size: 13px; }
</style>
