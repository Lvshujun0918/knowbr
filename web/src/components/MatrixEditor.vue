<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  labels: string[]
  cells: string[][]
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:labels': [value: string[]]
  'update:cells': [value: string[][]]
}>()

const size = computed(() => props.cells.length)

const SIZE_OPTIONS = [2, 3, 4, 5, 6]

function resize(next: number): void {
  const n = Math.max(2, Math.min(6, next))
  const cells: string[][] = []
  const labels: string[] = []
  for (let i = 0; i < n; i++) {
    const row: string[] = []
    for (let j = 0; j < n; j++) {
      row.push(props.cells[i]?.[j] ?? '')
    }
    cells.push(row)
    labels.push(props.labels[i] ?? defaultLabel(i))
  }
  emit('update:cells', cells)
  emit('update:labels', labels)
}

function defaultLabel(i: number): string {
  const letters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'
  return i < letters.length ? `类别${letters[i]}` : `类别${i + 1}`
}

function setCell(i: number, j: number, value: string): void {
  const cells = props.cells.map((row) => row.slice())
  cells[i][j] = value
  emit('update:cells', cells)
}

function setLabel(i: number, value: string): void {
  const labels = props.labels.slice()
  labels[i] = value
  emit('update:labels', labels)
}

function clear(): void {
  emit(
    'update:cells',
    props.cells.map((row) => row.map(() => '')),
  )
}

defineExpose({ clear })
</script>

<template>
  <div class="matrix-editor">
    <div class="matrix-editor__bar">
      <label class="matrix-editor__size">
        矩阵规模
        <select
          class="input input--sm"
          :value="size"
          :disabled="disabled"
          aria-label="选择混淆矩阵规模"
          @change="resize(Number(($event.target as HTMLSelectElement).value))"
        >
          <option v-for="n in SIZE_OPTIONS" :key="n" :value="n">{{ n }} × {{ n }}</option>
        </select>
      </label>
      <button type="button" class="btn btn--ghost btn--sm" :disabled="disabled" @click="clear">清空表格</button>
      <span class="matrix-editor__hint">行 = 真实类别，列 = 预测类别；空白单元格按 0 处理</span>
    </div>
    <div class="table-scroll">
      <table class="table table--edit">
        <thead>
          <tr>
            <th scope="col">真实 \ 预测</th>
            <th v-for="(_, j) in size" :key="j" scope="col">
              <input
                class="input input--label"
                :value="labels[j] ?? ''"
                :disabled="disabled"
                :aria-label="`第 ${j + 1} 个类别名称`"
                @input="setLabel(j, ($event.target as HTMLInputElement).value)"
              />
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(_, i) in size" :key="i">
            <th scope="row" class="table--edit__rowhead">{{ labels[i] || defaultLabel(i) }}</th>
            <td v-for="(__, j) in size" :key="j">
              <input
                class="input input--cell"
                inputmode="decimal"
                :value="cells[i]?.[j] ?? ''"
                :disabled="disabled"
                :aria-label="`真实 ${labels[i] || defaultLabel(i)}，预测 ${labels[j] || defaultLabel(j)} 的样本数`"
                placeholder="0"
                @input="setCell(i, j, ($event.target as HTMLInputElement).value)"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
