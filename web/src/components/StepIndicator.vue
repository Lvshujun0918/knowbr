<script setup lang="ts">
import type { StepDef } from '../types'

const props = defineProps<{
  steps: StepDef[]
  /** 当前查看的步骤（高亮）。 */
  current: number
  /** 流程已推进到的步骤（含）之前均标记为已完成。 */
  reached: number
  /** 不可点击的步骤索引（例如尚未生成学习反馈）。 */
  disabledFrom?: number
}>()

const emit = defineEmits<{
  select: [index: number]
}>()

function state(i: number): 'done' | 'current' | 'todo' {
  if (i === props.current) return 'current'
  if (i <= props.reached) return 'done'
  return 'todo'
}

function clickable(i: number): boolean {
  return props.disabledFrom === undefined || i < props.disabledFrom
}
</script>

<template>
  <ol class="step-indicator" aria-label="学习流程步骤">
    <li
      v-for="(step, i) in steps"
      :key="step.key"
      class="step-indicator__item"
      :class="`is-${state(i)}`"
    >
      <button
        type="button"
        class="step-indicator__btn"
        :disabled="!clickable(i)"
        :aria-current="i === current ? 'step' : undefined"
        @click="emit('select', i)"
      >
        <span class="step-indicator__num" aria-hidden="true">{{ i + 1 }}</span>
        <span class="step-indicator__title">{{ step.title }}</span>
      </button>
      <span v-if="i < steps.length - 1" class="step-indicator__sep" aria-hidden="true"></span>
    </li>
  </ol>
</template>
