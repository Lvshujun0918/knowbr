<script setup lang="ts">
withDefaults(
  defineProps<{
    variant: 'loading' | 'empty' | 'error'
    title?: string
    message?: string
  }>(),
  { title: '', message: '' },
)
</script>

<template>
  <div class="state-block" :class="`state-block--${variant}`">
    <span v-if="variant === 'loading'" class="spinner" aria-hidden="true"></span>
    <p class="state-block__title">{{ title || (variant === 'loading' ? '正在加载…' : variant === 'error' ? '加载失败' : '暂无内容') }}</p>
    <p v-if="message" class="state-block__message" :role="variant === 'error' ? 'alert' : undefined">{{ message }}</p>
    <div v-if="$slots.default" class="state-block__actions">
      <slot />
    </div>
  </div>
</template>
