<script setup lang="ts">
withDefaults(
  defineProps<{
    kind?: 'info' | 'success' | 'warning' | 'danger'
    title?: string
    live?: boolean
  }>(),
  { kind: 'info', title: '', live: false },
)
</script>

<template>
  <div class="alert" :class="`alert--${kind}`" :role="kind === 'danger' ? 'alert' : undefined" :aria-live="live ? 'polite' : undefined">
    <strong v-if="title" class="alert__title">{{ title }}</strong>
    <div class="alert__body">
      <slot />
    </div>
  </div>
</template>
