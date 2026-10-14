<script setup lang="ts">
import { computed } from 'vue'
import type { Session } from '../../types'
import { claimStatusKind, formatDateTime, isOpenStatus } from '../../lib/utils'
import AlertBox from '../AlertBox.vue'
import MetricsTable from '../MetricsTable.vue'

const props = defineProps<{ session: Session }>()

const diagnosis = computed(() => props.session.diagnosis ?? null)

/** 未解决的知识点（存在错误 / 部分掌握 / 证据不足）。 */
const openKnowledge = computed(() => (diagnosis.value?.knowledge ?? []).filter((k) => isOpenStatus(k.status)))

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
              <dt>任务类型</dt>
              <dd>
                <span class="tag tag--neutral">{{ diagnosis.taskTypeLabel }}</span>
                <span class="side-note">诊断、分层提示与迁移题都限定在这一任务范围内</span>
              </dd>
            </div>
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

        <div v-if="diagnosis.claims && diagnosis.claims.length > 0" class="card">
          <h2 class="card__title">逐条主张诊断</h2>
          <p class="card__desc">
            系统把你的解释拆成独立主张分别判断，并给出各自的证据、理由、证据边界与追问；没有讨论到的知识点不会被当作未掌握。
          </p>
          <ol class="claim-list">
            <li v-for="c in diagnosis.claims" :key="c.id" class="claim-list__item">
              <div class="claim-list__head">
                <span class="tag" :class="`tag--${claimStatusKind(c.status)}`">{{ c.statusLabel }}</span>
                <span class="claim-list__point">{{ c.knowledgePoint }}</span>
              </div>
              <p class="claim-list__quote">{{ c.evidence }}</p>
              <p class="claim-list__block"><strong>判断：</strong>{{ c.judgment }}</p>
              <p class="claim-list__block"><strong>原因：</strong>{{ c.reason }}</p>
              <p class="claim-list__block"><strong>证据边界：</strong>{{ c.evidenceBoundary }}</p>
              <p class="claim-list__block"><strong>下一步追问：</strong>{{ c.nextQuestion }}</p>
            </li>
          </ol>
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
          <h2 class="card__title">材料必要性</h2>
          <ul v-if="diagnosis.evidenceNeeds && diagnosis.evidenceNeeds.length > 0" class="check-list">
            <li
              v-for="n in diagnosis.evidenceNeeds"
              :key="n.name"
              :class="n.present ? 'is-yes' : n.required ? 'is-no' : 'is-optional'"
            >
              {{ n.name }}
              <span class="side-note">{{ n.required ? '（必需）' : '（可选，不影响本次诊断成立）' }}</span>
              <span v-if="!n.present" class="side-note">{{ n.impact }}</span>
            </li>
          </ul>
          <ul v-else class="check-list">
            <li :class="diagnosis.completeness.haveExplanation ? 'is-yes' : 'is-no'">学生自己的解释</li>
            <li :class="diagnosis.completeness.haveMatrix ? 'is-yes' : 'is-optional'">混淆矩阵</li>
            <li :class="diagnosis.completeness.havePerClassMetrics ? 'is-yes' : 'is-optional'">逐类指标可计算</li>
            <li :class="diagnosis.completeness.haveDistribution ? 'is-yes' : 'is-optional'">类别分布</li>
            <li :class="diagnosis.completeness.haveTaskGoal ? 'is-yes' : 'is-optional'">实验目标</li>
            <li :class="diagnosis.completeness.haveRunEvidence ? 'is-yes' : 'is-optional'">运行证据</li>
          </ul>
        </div>
        <div v-if="diagnosis.knowledge && diagnosis.knowledge.length > 0" class="card">
          <h2 class="card__title">知识点状态</h2>
          <p class="side-note" v-if="openKnowledge.length === 0">本次涉及的知识点均已解决。</p>
          <ul class="knowledge-list">
            <li v-for="k in diagnosis.knowledge" :key="k.code">
              <div class="knowledge-list__head">
                <span class="knowledge-list__name">{{ k.name }}</span>
                <span class="tag" :class="`tag--${claimStatusKind(k.status)}`">{{ k.statusLabel }}</span>
              </div>
              <p class="knowledge-list__note">{{ k.note }}</p>
            </li>
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
