<script setup lang="ts">
import type { MatrixMetrics } from '../types'
import { pct } from '../lib/utils'

defineProps<{
  metrics: MatrixMetrics
  /** 是否同时展示原始混淆矩阵。 */
  showMatrix?: boolean
}>()

function ratio(x: number, undefined_: boolean): string {
  return undefined_ ? '0（约定）' : pct(x)
}
</script>

<template>
  <div class="metrics">
    <div class="metrics__summary">
      <div class="metrics__stat">
        <span class="metrics__stat-value">{{ metrics.total }}</span>
        <span class="metrics__stat-label">总样本数</span>
      </div>
      <div class="metrics__stat">
        <span class="metrics__stat-value">{{ metrics.correct }}</span>
        <span class="metrics__stat-label">正确分类数</span>
      </div>
      <div class="metrics__stat">
        <span class="metrics__stat-value">{{ pct(metrics.accuracy) }}</span>
        <span class="metrics__stat-label">整体准确率</span>
      </div>
      <div class="metrics__stat">
        <span class="metrics__stat-value">{{ metrics.macroF1.toFixed(3) }}</span>
        <span class="metrics__stat-label">宏平均 F1</span>
      </div>
      <div class="metrics__stat">
        <span class="metrics__stat-value">{{ metrics.weightedF1.toFixed(3) }}</span>
        <span class="metrics__stat-label">加权平均 F1</span>
      </div>
      <div class="metrics__stat">
        <span class="metrics__stat-value">{{ metrics.imbalanced ? '是' : '否' }}</span>
        <span class="metrics__stat-label">类别不平衡（最大类占比 {{ pct(metrics.maxClassRatio) }}）</span>
      </div>
    </div>

    <div v-if="showMatrix" class="table-scroll">
      <table class="table table--matrix" aria-label="混淆矩阵">
        <caption class="table-caption">混淆矩阵（行 = 真实类别，列 = 预测类别）</caption>
        <thead>
          <tr>
            <th scope="col">真实 \ 预测</th>
            <th v-for="label in metrics.labels" :key="label" scope="col">{{ label }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, i) in metrics.values" :key="i">
            <th scope="row">{{ metrics.labels[i] ?? `类别${i + 1}` }}</th>
            <td v-for="(v, j) in row" :key="j" :class="{ 'table__diag': i === j }">{{ v }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="table-scroll">
      <table class="table" aria-label="按类别的评价指标">
        <thead>
          <tr>
            <th scope="col">类别</th>
            <th scope="col">真实样本数</th>
            <th scope="col">预测样本数</th>
            <th scope="col">召回率</th>
            <th scope="col">精确率</th>
            <th scope="col">F1</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in metrics.classes" :key="c.index">
            <th scope="row">{{ c.label }}</th>
            <td>{{ c.support }}</td>
            <td>{{ c.predicted }}</td>
            <td>{{ ratio(c.recall, c.recallUndefined) }}</td>
            <td>{{ ratio(c.precision, c.precisionUndefined) }}</td>
            <td>{{ c.f1.toFixed(3) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <p v-if="metrics.zeroRecallClasses.length > 0" class="metrics__warn">
      召回率为 0 的类别：{{ metrics.zeroRecallClasses.join('、') }}
    </p>

    <div class="metrics__notes">
      <p class="metrics__computed">以上指标由程序计算，未经过大模型。</p>
      <ul>
        <li v-for="(note, i) in metrics.notes" :key="i">{{ note }}</li>
      </ul>
    </div>
  </div>
</template>
