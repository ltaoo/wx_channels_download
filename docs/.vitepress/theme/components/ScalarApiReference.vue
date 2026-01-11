<script setup>
import { ApiReference } from '@scalar/api-reference'
import '@scalar/api-reference/style.css'
import { withBase } from 'vitepress'
import { computed } from 'vue'

const props = defineProps({
  configuration: {
    type: Object,
    default: () => ({})
  }
})

const config = computed(() => ({
  ...props.configuration,
  spec: {
    ...props.configuration.spec,
    url: props.configuration.spec?.url ?? withBase('/openapi.json'),
  },
}))
</script>

<template>
  <div class="scalar-api-page">
    <ApiReference :configuration="config" />
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
