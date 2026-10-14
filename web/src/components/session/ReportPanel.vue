<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api, exportUrl } from '../../api/client'
import type { Session, Summary } from '../../types'
import { copyText, errorMessage, formatDateTime, hintLevelLabel } from '../../lib/utils'
import AlertBox from '../AlertBox.vue'
import StateBlock from '../StateBlock.vue'

const props = defineProps<{ session: Session }>()

const summary = ref<Summary | null>(null)
const markdown = ref('')
const loading = ref(true)
const loadError = ref('')
const copyState = ref<'' | 'ok' | 'fail'>('')

onMounted(async () => {
  try {
    const res = await api.getSummary(props.session.id)
    summary.value = res.summary
    markdown.value = res.markdown
  } catch (err) {
    loadError.value = errorMessage(err)
  } finally {
    loading.value = false
  }
})

async function copyFeedback(): Promise<void> {
  const text = markdown.value || summary.value?.teacherSummary || ''
  if (!text) return
  copyState.value = (await copyText(text)) ? 'ok' : 'fail'
  setTimeout(() => {
    copyState.value = ''
  }, 2500)
}
</script>

<template>
  <section class="panel" aria-label="学习反馈">
    <StateBlock v-if="loading" variant="loading" title="正在生成学习反馈…" />
    <StateBlock v-else-if="loadError" variant="error" title="无法生成学习反馈" :message="loadError" />

    <template v-else-if="summary">
      <AlertBox v-if="summary.dataNotice" kind="warning" title="模拟案例">{{ summary.dataNotice }}</AlertBox>

      <div class="card report-actions">
        <div class="report-actions__buttons">
          <button type="button" class="btn btn--secondary" @click="copyFeedback">复制反馈</button>
          <a class="btn btn--secondary" :href="exportUrl(session.id, 'md')" download>导出 Markdown</a>
          <a class="btn btn--secondary" :href="exportUrl(session.id, 'txt')" download>导出 TXT</a>
          <RouterLink to="/" class="btn btn--ghost">返回首页</RouterLink>
          <RouterLink to="/teacher" class="btn btn--ghost">查看教师视图</RouterLink>
        </div>
        <p v-if="copyState === 'ok'" class="form-ok" role="status">已复制到剪贴板。</p>
        <p v-else-if="copyState === 'fail'" class="form-error" role="alert">复制失败，请使用导出功能。</p>
      </div>

      <div class="panel-grid">
        <div class="panel-main">
          <div class="card">
            <h2 class="card__title">本次学习目标</h2>
            <p>{{ summary.learningGoal }}</p>
          </div>

          <div class="card">
            <h2 class="card__title">关键证据与初始解释</h2>
            <h3 class="card__subtitle">初始解释</h3>
            <p class="report-quote">{{ summary.initialExplanation }}</p>
            <h3 class="card__subtitle">关键证据</h3>
            <ul class="list list--tight">
              <li v-for="(e, i) in summary.keyEvidence ?? []" :key="i">{{ e }}</li>
            </ul>
          </div>

          <div class="card">
            <h2 class="card__title">主要误区与诊断</h2>
            <p class="diag-main-issue">{{ summary.mainIssue }}</p>
            <div class="tag-row">
              <span v-for="c in summary.categories" :key="c.code" class="tag tag--category">{{ c.label }}</span>
              <span class="tag tag--neutral">掌握情况：{{ summary.mastery.label }}</span>
            </div>
            <h3 class="card__subtitle">诊断理由</h3>
            <p>{{ summary.diagnosisReasoning }}</p>
            <template v-if="summary.issueEvidence && summary.issueEvidence.length > 0">
              <h3 class="card__subtitle">相关证据</h3>
              <ul class="list list--tight">
                <li v-for="(e, i) in summary.issueEvidence" :key="i">{{ e }}</li>
              </ul>
            </template>
          </div>

          <div class="card">
            <h2 class="card__title">提示轨迹与修正情况</h2>
            <p v-if="!summary.hintTrail || summary.hintTrail.length === 0" class="state-block__message">本次没有使用分层提示。</p>
            <ol v-else class="hint-trail">
              <li v-for="(h, i) in summary.hintTrail" :key="i">
                <span class="tag tag--hint">{{ hintLevelLabel(h.level) }}</span>
                <span class="hint-trail__time">{{ formatDateTime(h.createdAt) }}</span>
                <p class="hint-trail__content">{{ h.content }}</p>
              </li>
            </ol>
            <p class="report-revision">{{ summary.revisionSummary }}</p>
          </div>

          <div v-if="summary.transfer" class="card">
            <h2 class="card__title">迁移测试表现</h2>
            <p class="report-transfer-q">{{ summary.transfer.question }}</p>
            <p>
              得分：<strong>{{ summary.transfer.scored ? `${summary.transfer.score} / ${summary.transfer.maxScore}` : '未评分' }}</strong>
            </p>
            <div class="table-scroll">
              <table class="table" aria-label="迁移测试各维度评分">
                <thead>
                  <tr>
                    <th scope="col">维度</th>
                    <th scope="col">得分</th>
                    <th scope="col">评分理由</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="fb in summary.transfer.rubricFeedback ?? []" :key="fb.code">
                    <th scope="row">{{ fb.title }}</th>
                    <td>{{ fb.score }} / {{ fb.maxScore }}</td>
                    <td>{{ fb.reason }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="summary.transfer.matchedKeywords && summary.transfer.matchedKeywords.length > 0" class="tag-row">
              <span class="tag-row__label">命中要点：</span>
              <span v-for="k in summary.transfer.matchedKeywords" :key="k" class="tag tag--success">{{ k }}</span>
            </div>
            <template v-if="summary.transfer.missingPoints && summary.transfer.missingPoints.length > 0">
              <h3 class="card__subtitle">尚未覆盖的要点</h3>
              <ul class="list list--tight">
                <li v-for="(m, i) in summary.transfer.missingPoints" :key="i">{{ m }}</li>
              </ul>
            </template>
            <details v-if="summary.transfer.referenceAnswer" class="transfer-my-answer">
              <summary>参考答案</summary>
              <p>{{ summary.transfer.referenceAnswer }}</p>
            </details>
          </div>
        </div>

        <aside class="panel-side">
          <div class="card">
            <h2 class="card__title">建议学生继续学习</h2>
            <ul class="list list--tight">
              <li v-for="(s, i) in summary.studentNextSteps ?? []" :key="i">{{ s }}</li>
            </ul>
          </div>
          <div class="card">
            <h2 class="card__title">建议教师关注</h2>
            <ul class="list list--tight">
              <li v-for="(t, i) in summary.teacherNotes ?? []" :key="i">{{ t }}</li>
            </ul>
            <AlertBox v-if="summary.teacherIntervention" kind="danger" title="建议教师介入">本次学习触发了教师介入条件。</AlertBox>
          </div>
          <div class="card">
            <h2 class="card__title">教师摘要</h2>
            <p class="side-note side-note--pre">{{ summary.teacherSummary }}</p>
          </div>
          <div class="card">
            <h2 class="card__title">诚实说明</h2>
            <p class="side-note">运行模式：{{ summary.modeLabel }}；大模型实际参与：{{ summary.llmUsed ? '是（仅组织语言，不改动计算结果）' : '否' }}</p>
            <p class="side-note">{{ summary.disclaimer }}</p>
            <p class="side-note">生成时间：{{ formatDateTime(summary.generatedAt) }}</p>
          </div>
        </aside>
      </div>
    </template>
  </section>
</template>
