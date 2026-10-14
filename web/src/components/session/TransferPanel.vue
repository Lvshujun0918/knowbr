<script setup lang="ts">
import { computed, ref } from 'vue'
import { api } from '../../api/client'
import type { Session, Summary, TransferState } from '../../types'
import { errorMessage, formatDateTime } from '../../lib/utils'
import AlertBox from '../AlertBox.vue'

const props = defineProps<{ session: Session }>()

const emit = defineEmits<{
  update: [transfer: TransferState]
  completed: [payload: { transfer: TransferState; summary: Summary; session: Session }]
}>()

const transfer = computed(() => props.session.transfer ?? null)

const answerText = ref('')
const pending = ref<'question' | 'answer' | 'reference' | 'complete' | ''>('')
const panelError = ref('')

async function startQuestion(): Promise<void> {
  panelError.value = ''
  pending.value = 'question'
  try {
    const res = await api.transferQuestion(props.session.id)
    emit('update', res.transfer)
  } catch (err) {
    panelError.value = errorMessage(err)
  } finally {
    pending.value = ''
  }
}

async function submitAnswer(): Promise<void> {
  panelError.value = ''
  pending.value = 'answer'
  try {
    const res = await api.transferAnswer(props.session.id, answerText.value)
    emit('update', res.transfer)
  } catch (err) {
    panelError.value = errorMessage(err)
  } finally {
    pending.value = ''
  }
}

async function showReference(): Promise<void> {
  panelError.value = ''
  pending.value = 'reference'
  try {
    const res = await api.transferReference(props.session.id)
    emit('update', res.transfer)
  } catch (err) {
    panelError.value = errorMessage(err)
  } finally {
    pending.value = ''
  }
}

async function complete(): Promise<void> {
  panelError.value = ''
  pending.value = 'complete'
  try {
    const res = await api.transferComplete(props.session.id)
    emit('completed', { transfer: res.transfer, summary: res.summary, session: res.session })
  } catch (err) {
    panelError.value = errorMessage(err)
  } finally {
    pending.value = ''
  }
}

const busy = computed(() => pending.value !== '')
</script>

<template>
  <section class="panel" aria-label="迁移测试">
    <div v-if="!transfer" class="card state-card">
      <p class="state-block__message">
        迁移测试会给出一道与本次学习目标相关、但情境不同的新题目。先独立作答，提交后才能看到评分依据与参考答案。
      </p>
      <button type="button" class="btn btn--primary" :disabled="busy" @click="startQuestion">
        {{ pending === 'question' ? '正在生成迁移题…' : '生成迁移题' }}
      </button>
    </div>

    <template v-else>
      <div class="card">
        <div class="transfer-head">
          <h2 class="card__title">{{ transfer.topic }}</h2>
          <span class="tag tag--neutral">满分 {{ transfer.maxScore }} 分</span>
        </div>
        <div class="tag-row">
          <span class="tag tag--category">{{ transfer.taskTypeLabel }}</span>
          <span class="tag tag--source">{{ transfer.sourceLabel }}</span>
          <span v-if="transfer.knowledgePoint" class="tag tag--neutral">考查：{{ transfer.knowledgePoint }}</span>
        </div>
        <div v-if="transfer.scenario" class="transfer-context">情境：{{ transfer.scenario }}</div>
        <p class="transfer-context">{{ transfer.context }}</p>
        <p class="transfer-question">{{ transfer.question }}</p>

        <div class="transfer-rubric">
          <h3 class="card__subtitle">必要背景与作答要求</h3>
          <ul v-if="transfer.requiredData && transfer.requiredData.length > 0" class="list list--tight">
            <li v-for="(d, i) in transfer.requiredData" :key="'d' + i">题干已提供：{{ d }}</li>
          </ul>
          <p v-if="transfer.answerRequirement" class="transfer-context">{{ transfer.answerRequirement }}</p>
          <h3 class="card__subtitle">评分维度（作答前可见）</h3>
          <ul class="list list--tight">
            <li v-for="dim in transfer.rubric" :key="dim.code">{{ dim.title }}</li>
          </ul>
        </div>
        <AlertBox kind="info">{{ transfer.disclaimer }}</AlertBox>
        <details class="transfer-my-answer">
          <summary>这道题是怎么生成的？</summary>
          <p>{{ transfer.generationNote }}</p>
          <p v-if="transfer.selectionReason">选题依据：{{ transfer.selectionReason }}</p>
          <p v-if="transfer.dedupNote">重复检查：{{ transfer.dedupNote }}</p>
          <p v-if="transfer.familyLabel">题目族：{{ transfer.familyLabel }}<span v-if="transfer.variantCode">（{{ transfer.variantCode }}）</span></p>
          <div v-if="transfer.paramsUsed && Object.keys(transfer.paramsUsed).length > 0" class="tag-row">
            <span class="tag-row__label">本次参数：</span>
            <span v-for="(v, k) in transfer.paramsUsed" :key="k" class="tag tag--neutral">{{ k }} = {{ v }}</span>
          </div>
          <div v-if="transfer.comparedItems && transfer.comparedItems.length > 0" class="tag-row">
            <span class="tag-row__label">已比对：</span>
            <span v-for="(c, i) in transfer.comparedItems" :key="i" class="tag tag--neutral">{{ c }}</span>
          </div>
        </details>
      </div>

      <div v-if="!transfer.answered" class="card">
        <div class="field">
          <label class="field__label" for="transfer-answer">我的答案</label>
          <textarea
            id="transfer-answer"
            v-model="answerText"
            class="textarea"
            rows="5"
            placeholder="独立写下你的判断与理由，提交后才能查看评分与参考答案。"
          ></textarea>
        </div>
        <div class="form-actions">
          <button type="button" class="btn btn--primary" :disabled="busy" @click="submitAnswer">
            {{ pending === 'answer' ? '正在提交…' : '提交答案' }}
          </button>
        </div>
      </div>

      <template v-else>
        <div class="card">
          <h2 class="card__title">评分结果</h2>
          <p class="transfer-score">
            <span class="transfer-score__value">{{ transfer.scoreLabel }}</span>
            <span v-if="transfer.scoredAt" class="transfer-score__time">{{ formatDateTime(transfer.scoredAt) }}</span>
          </p>
          <div class="table-scroll">
            <table class="table" aria-label="各评分维度得分">
              <thead>
                <tr>
                  <th scope="col">维度</th>
                  <th scope="col">得分</th>
                  <th scope="col">评分理由</th>
                  <th scope="col">命中要点</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="fb in transfer.rubricFeedback ?? []" :key="fb.code">
                  <th scope="row">{{ fb.title }}</th>
                  <td>{{ fb.score }} / {{ fb.maxScore }}</td>
                  <td>{{ fb.reason }}</td>
                  <td>
                    <span v-for="k in fb.matchedKeywords ?? []" :key="k" class="tag tag--success">{{ k }}</span>
                    <span v-if="!fb.matchedKeywords || fb.matchedKeywords.length === 0">—</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="transfer.matchedKeywords && transfer.matchedKeywords.length > 0" class="tag-row">
            <span class="tag-row__label">全部命中要点：</span>
            <span v-for="k in transfer.matchedKeywords" :key="k" class="tag tag--success">{{ k }}</span>
          </div>
          <div v-if="transfer.missingPoints && transfer.missingPoints.length > 0" class="transfer-missing">
            <h3 class="card__subtitle">尚未覆盖的要点</h3>
            <ul class="list list--tight">
              <li v-for="(m, i) in transfer.missingPoints" :key="i">{{ m }}</li>
            </ul>
          </div>
          <details class="transfer-my-answer">
            <summary>查看我的答案</summary>
            <p>{{ transfer.answer }}</p>
          </details>
        </div>

        <div class="card">
          <h2 class="card__title">参考答案</h2>
          <template v-if="transfer.referenceAnswer">
            <p class="transfer-reference">{{ transfer.referenceAnswer }}</p>
            <ul v-if="transfer.expectedPoints && transfer.expectedPoints.length > 0" class="list">
              <li v-for="(p, i) in transfer.expectedPoints" :key="i">{{ p }}</li>
            </ul>
          </template>
          <p v-else class="state-block__message">参考答案只在你提交自己的答案之后显示。</p>
          <div class="form-actions">
            <button v-if="!transfer.referenceAnswer" type="button" class="btn btn--secondary" :disabled="busy" @click="showReference">
              {{ pending === 'reference' ? '正在获取…' : '查看参考答案' }}
            </button>
            <button type="button" class="btn btn--primary" :disabled="busy" @click="complete">
              {{ pending === 'complete' ? '正在生成学习反馈…' : '完成本次学习' }}
            </button>
          </div>
        </div>
      </template>
    </template>

    <div v-if="panelError" class="alert alert--danger" role="alert">{{ panelError }}</div>
  </section>
</template>
