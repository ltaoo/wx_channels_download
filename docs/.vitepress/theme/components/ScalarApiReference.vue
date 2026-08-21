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
  withBase('/openapi/index.json')
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

function splitReference(reference) {
  const hashIndex = reference.indexOf('#')
  if (hashIndex === -1) {
    return { path: reference, pointer: '' }
  }

  return {
    path: reference.slice(0, hashIndex),
    pointer: reference.slice(hashIndex + 1)
  }
}

async function resolveOpenAPIValue(value, document, documentUrl, cache, ancestors = new Set()) {
  if (Array.isArray(value)) {
    return Promise.all(value.map((item) => (
      resolveOpenAPIValue(item, document, documentUrl, cache, ancestors)
    )))
  }

  if (!value || typeof value !== 'object') {
    return value
  }

  if (typeof value.$ref === 'string') {
    const reference = value.$ref
    const { path, pointer } = splitReference(reference)
    const targetUrl = path ? new URL(path, documentUrl).toString() : documentUrl
    const targetDocument = targetUrl === documentUrl
      ? document
      : await fetchJson(targetUrl, cache)
    const target = pointer
      ? getJsonPointerValue(targetDocument, pointer)
      : targetDocument

    if (target === undefined) {
      throw new Error(`Invalid OpenAPI reference: ${reference} (from ${documentUrl})`)
    }

    const referenceKey = `${targetUrl}#${pointer}`
    if (ancestors.has(referenceKey)) {
      throw new Error(`Circular OpenAPI reference: ${referenceKey}`)
    }

    const nextAncestors = new Set(ancestors)
    nextAncestors.add(referenceKey)
    const resolved = await resolveOpenAPIValue(
      clone(target),
      targetDocument,
      targetUrl,
      cache,
      nextAncestors
    )
    const siblingEntries = Object.entries(value).filter(([key]) => key !== '$ref')

    if (siblingEntries.length === 0) {
      return resolved
    }

    const siblings = await resolveOpenAPIValue(
      Object.fromEntries(siblingEntries),
      document,
      documentUrl,
      cache,
      ancestors
    )

    return resolved && typeof resolved === 'object' && !Array.isArray(resolved)
      ? { ...resolved, ...siblings }
      : resolved
  }

  const entries = await Promise.all(Object.entries(value).map(async ([key, item]) => ([
    key,
    await resolveOpenAPIValue(item, document, documentUrl, cache, ancestors)
  ])))

  return Object.fromEntries(entries)
}

async function resolveOpenAPIRefs(document, baseUrl, cache) {
  return resolveOpenAPIValue(clone(document), document, baseUrl, cache)
}

async function loadSpec() {
  const url = new URL(sourceUrl.value, window.location.href).toString()
  const cache = new Map()
  const document = await fetchJson(url, cache)
  resolvedSpec.value = await resolveOpenAPIRefs(document, url, cache)
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
  <div class="scalar-api-page" data-n="scalar-api-page">
    <ApiReference
      v-if="resolvedSpec"
      :configuration="config"
      data-n="scalar-api-reference"
    />
    <div v-else-if="loadError" class="scalar-api-error" data-n="scalar-api-load-error">
      {{ loadError }}
    </div>
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
