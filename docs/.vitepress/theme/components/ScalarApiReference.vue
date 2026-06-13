<script setup>
import { ApiReference } from '@scalar/api-reference'
import '@scalar/api-reference/style.css'
import { withBase } from 'vitepress'
import { computed, onMounted, ref } from 'vue'

const props = defineProps({
  configuration: {
    type: Object,
    default: () => ({})
  }
})

const resolvedSpec = ref(null)
const loadError = ref('')

const sourceUrl = computed(() => (
  props.configuration.url ??
  props.configuration.spec?.url ??
  withBase('/openapi.json')
))

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

function getJsonPointerValue(document, pointer) {
  if (!pointer || pointer === '/') return document

  return pointer
    .replace(/^\//, '')
    .split('/')
    .reduce((current, segment) => {
      if (current == null) return undefined
      const key = segment.replace(/~1/g, '/').replace(/~0/g, '~')
      return current[key]
    }, document)
}

async function fetchJson(url, cache) {
  if (!cache.has(url)) {
    cache.set(url, fetch(url).then((response) => {
      if (!response.ok) {
        throw new Error(`Failed to load ${url}: ${response.status}`)
      }
      return response.json()
    }))
  }
  return cache.get(url)
}

async function resolvePathRefs(document, baseUrl) {
  const spec = clone(document)
  const cache = new Map()
  const entries = Object.entries(spec.paths ?? {})

  await Promise.all(entries.map(async ([path, pathItem]) => {
    const ref = pathItem?.$ref
    if (typeof ref !== 'string') return

    const [refPath, pointer = ''] = ref.split('#')
    const refUrl = refPath ? new URL(refPath, baseUrl).toString() : baseUrl
    const refDocument = refUrl === baseUrl ? spec : await fetchJson(refUrl, cache)
    const resolved = pointer ? getJsonPointerValue(refDocument, pointer) : refDocument

    if (!resolved || typeof resolved !== 'object') {
      throw new Error(`Invalid OpenAPI path reference: ${ref}`)
    }

    spec.paths[path] = clone(resolved)
  }))

  return spec
}

async function loadSpec() {
  const url = new URL(sourceUrl.value, window.location.href).toString()
  const document = await fetchJson(url, new Map())
  resolvedSpec.value = await resolvePathRefs(document, url)
}

onMounted(async () => {
  if (props.configuration.content) {
    resolvedSpec.value = props.configuration.content
    return
  }

  try {
    await loadSpec()
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : String(error)
  }
})

const config = computed(() => ({
  ...props.configuration,
  url: undefined,
  spec: undefined,
  content: resolvedSpec.value,
}))
</script>

<template>
  <div class="scalar-api-page">
    <ApiReference v-if="resolvedSpec" :configuration="config" />
    <div v-else-if="loadError" class="scalar-api-error">{{ loadError }}</div>
  </div>
</template>

<style>
.api-page .VPPage {
  padding-left: 0 !important;
  padding-right: 0 !important;
  padding-bottom: 0 !important;
  max-width: 100% !important;
}

.api-page .VPContent {
  padding: 0 !important;
}

.api-page .VPFooter {
  display: none !important;
}

.scalar-api-page {
  
}
.scalar-api-error {
  color: var(--vp-c-danger-1);
  padding: 24px;
}
.api-page .references-layout {
  position: fixed;
  top: 65px;
  left: 0;
  bottom: 0;
  overflow-y: auto;
  height: calc(100vh - 65px) !important;
  min-height: calc(100vh - 65px) !important;
}
.api-page .scalar-app .h-dvh {
  height: calc(100vh - 65px) !important;
  min-height: calc(100vh - 65px) !important;
}
.scalar-api-reference {
  width: 100%;
  overflow-y: auto;
}
</style>
