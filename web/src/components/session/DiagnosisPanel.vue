<script setup lang="ts">
import { computed } from 'vue'
import type { Session } from '../../types'
import { formatDateTime } from '../../lib/utils'
import AlertBox from '../AlertBox.vue'
import MetricsTable from '../MetricsTable.vue'

const props = defineProps<{ session: Session }>()

const diagnosis = computed(() => props.session.diagnosis ?? null)

/** 证据来源标签（与 evidence 一一对应，越界时兜底为“学生输入”）。 */
function sourceAt(i: number): string {
  return diagnosis.value?.evidenceSource?.[i] ?? '学生输入'
}

function masteryKind(status: string): 'success' | 'warning' | 'danger' | 'neutral' {
  switch (status) {
    case 'demonstrated':
      return 'success'
    case 'partial':
      return 'warning'
    case 'not_demonstrated':
      return 'danger'
    default:
      return 'neutral'
  }
}
</script>

<template>
  <section v-if="diagnosis" class="panel" aria-label="学习诊断">
    <div class="panel-grid">
      <div class="panel-main">
        <div class="card">
          <h2 class="card__title">主要问题</h2>
          <p class="diag-main-issue">{{ diagnosis.mainIssue }}</p>
          <div class="tag-row">
            <span v-for="c in diagnosis.categories" :key="c.code" class="tag tag--category">{{ c.label }}</span>
          </div>
          <dl class="kv">
            <div class="kv__row">
              <dt>掌握情况</dt>
              <dd><span class="tag" :class="`tag--${masteryKind(diagnosis.masteryStatus)}`">{{ diagnosis.masteryLabel }}</span></dd>
            </div>
            <div class="kv__row">
              <dt>建议下一步</dt>
              <dd>{{ diagnosis.nextStep }}</dd>
            </div>
            <div v-if="diagnosis.matchedArchetypes.length > 0" class="kv__row">
              <dt>命中的典型误区</dt>
              <dd>
                <span v-for="a in diagnosis.matchedArchetypes" :key="a.code" class="tag tag--neutral" :title="a.knowledgePoint">
                  {{ a.code }} {{ a.name }}
                </span>
              </dd>
            </div>
          </dl>
        </div>

        <AlertBox v-if="diagnosis.contradiction.detected" kind="warning" title="你的陈述与提交的证据存在矛盾">
          <p v-if="diagnosis.contradiction.claim">{{ diagnosis.contradiction.claim }}</p>
          <p>{{ diagnosis.contradiction.detail }}</p>
        </AlertBox>

        <AlertBox v-if="!diagnosis.completeness.sufficient || (diagnosis.completeness.limitations?.length ?? 0) > 0" kind="info" title="诊断限制与补充材料">
          <ul v-if="diagnosis.completeness.missing && diagnosis.completeness.missing.length > 0" class="list list--tight">
            <li v-for="(m, i) in diagnosis.completeness.missing" :key="'m' + i">缺少：{{ m }}</li>
          </ul>
          <ul v-if="diagnosis.completeness.limitations && diagnosis.completeness.limitations.length > 0" class="list list--tight">
            <li v-for="(l, i) in diagnosis.completeness.limitations" :key="'l' + i">{{ l }}</li>
          </ul>
        </AlertBox>

        <div class="card">
          <h2 class="card__title">诊断依据（引用你提交的证据）</h2>
          <ul class="evidence-list">
            <li v-for="(ev, i) in diagnosis.evidence" :key="i" class="evidence-list__item">
              <span class="tag tag--source">{{ sourceAt(i) }}</span>
              <span class="evidence-list__text">{{ ev }}</span>
            </li>
          </ul>
          <h3 class="card__subtitle">诊断理由</h3>
          <p class="diag-reasoning">{{ diagnosis.reasoning }}</p>
        </div>

        <div v-if="session.metrics" class="card">
          <h2 class="card__title">由程序计算的指标</h2>
          <MetricsTable :metrics="session.metrics" :show-matrix="true" />
        </div>
      </div>

      <aside class="panel-side">
        <div class="card">
          <h2 class="card__title">材料完整性</h2>
          <ul class="check-list">
            <li :class="diagnosis.completeness.haveExplanation ? 'is-yes' : 'is-no'">学生自己的解释</li>
            <li :class="diagnosis.completeness.haveMatrix ? 'is-yes' : 'is-no'">混淆矩阵</li>
            <li :class="diagnosis.completeness.havePerClassMetrics ? 'is-yes' : 'is-no'">逐类指标可计算</li>
            <li :class="diagnosis.completeness.haveDistribution ? 'is-yes' : 'is-no'">类别分布</li>
            <li :class="diagnosis.completeness.haveTaskGoal ? 'is-yes' : 'is-no'">实验目标</li>
            <li :class="diagnosis.completeness.haveRunEvidence ? 'is-yes' : 'is-no'">运行证据</li>
          </ul>
        </div>
        <div class="card">
          <h2 class="card__title">诊断说明</h2>
          <p class="side-note">生成方式：{{ diagnosis.source === 'llm' ? '大模型组织语言（类别与数值仍由规则与程序产生）' : '内置规则引擎' }}</p>
          <p class="side-note">生成时间：{{ formatDateTime(diagnosis.generatedAt) }}</p>
          <p v-if="session.modeNotice" class="side-note">{{ session.modeNotice }}</p>
        </div>
      </aside>
    </div>
  </section>

  <div v-else class="card">
    <p class="state-block__message">这次记录没有可用的诊断结果。</p>
  </div>
</template>
