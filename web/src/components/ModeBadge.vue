<script setup lang="ts">
import type { MetaMode } from '../types'

defineProps<{
  mode: MetaMode
}>()
</script>

<template>
  <span class="mode-badge" :class="mode.llmConfigured ? 'mode-badge--llm' : 'mode-badge--rule'" :title="mode.notice">
    <span class="mode-badge__dot" aria-hidden="true"></span>
    {{ mode.label }}
  </span>
</template>
