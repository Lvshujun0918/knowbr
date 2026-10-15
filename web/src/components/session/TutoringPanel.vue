<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../../api/client'
import type { AgentStep, Hint, HintState, Revision, Session, TransferState } from '../../types'
import { errorMessage, formatDateTime, hintLevelLabel, progressLabel, progressTagKind } from '../../lib/utils'
import AlertBox from '../AlertBox.vue'

const props = defineProps<{ session: Session }>()

const emit = defineEmits<{
  update: [payload: { hints: Hint[]; hintState: HintState; revisions: Revision[] }]
  'enter-transfer': [transfer: TransferState]
}>()

type HistoryItem =
  | { kind: 'hint'; time: string; hint: Hint }
  | { kind: 'revision'; time: string; revision: Revision }

const history = computed<HistoryItem[]>(() => {
  const items: HistoryItem[] = []
  for (const hint of props.session.hints) items.push({ kind: 'hint', time: hint.createdAt, hint })
  for (const revision of props.session.revisions) items.push({ kind: 'revision', time: revision.createdAt, revision })
  return items.sort((a, b) => a.time.localeCompare(b.time))
})

const hintPending = ref(0)
const revisionPending = ref(false)
const transferPending = ref(false)
const panelError = ref('')
const hintNotice = ref('')
const revisionText = ref('')

async function requestHint(level: number): Promise<void> {
  panelError.value = ''
  hintNotice.value = ''
  hintPending.value = level
  try {
    const res = await api.addHint(props.session.id, level)
    emit('update', { hints: res.hints, hintState: res.hintState, revisions: props.session.revisions })
    if (res.alreadyProvided) {
      hintNotice.value = `${hintLevelLabel(level)}此前已提供过，已为你再次展示，不会重复记录。`
    }
  } catch (err) {
    panelError.value = errorMessage(err)
  } finally {
    hintPending.value = 0
  }
}

async function submitRevision(): Promise<void> {
  panelError.value = ''
  hintNotice.value = ''
  revisionPending.value = true
  try {
    const res = await api.addRevision(props.session.id, revisionText.value)
    emit('update', { hints: props.session.hints, hintState: res.hintState, revisions: res.revisions })
    revisionText.value = ''
  } catch (err) {
    panelError.value = errorMessage(err)
  } finally {
    revisionPending.value = false
  }
}

async function enterTransfer(): Promise<void> {
  panelError.value = ''
  transferPending.value = true
  try {
    const res = await api.transferQuestion(props.session.id)
    emit('enter-transfer', res.transfer)
  } catch (err) {
    panelError.value = errorMessage(err)
  } finally {
    transferPending.value = false
  }
}

const busy = computed(() => hintPending.value !== 0 || revisionPending.value || transferPending.value)

/* ---------------- 智能体决策 ---------------- */

const agent = ref<AgentStep | null>(null)
const agentPending = ref(false)
const agentNote = ref('')
const agentError = ref('')

async function loadAgent(): Promise<void> {
  agentError.value = ''
  try {
    const res = await api.getAgent(props.session.id)
    agent.value = res.agent
  } catch (err) {
    agentError.value = errorMessage(err)
  }
}

async function runAgentStep(): Promise<void> {
  agentError.value = ''
  agentNote.value = ''
  agentPending.value = true
  try {
    const res = await api.agentStep(props.session.id)
    agent.value = res.agent
    agentNote.value = res.executedNote
    emit('update', { hints: res.hints ?? props.session.hints, hintState: res.hintState, revisions: props.session.revisions })
  } catch (err) {
    agentError.value = errorMessage(err)
  } finally {
    agentPending.value = false
  }
}

onMounted(() => {
  void loadAgent()
})

const planStatusLabel: Record<string, string> = { done: '已完成', current: '进行中', pending: '待进行' }
</script>

<template>
  <section class="panel" aria-label="分层辅导">
    <AlertBox v-if="session.hintState.teacherIntervention" kind="danger" title="建议教师介入">
      {{ session.hintState.teacherReason || '多轮提示后仍没有明显进展，建议教师进一步辅导。' }}
    </AlertBox>

    <div class="panel-grid">
      <div class="panel-main">
        <div class="card">
          <h2 class="card__title">辅导对话</h2>
          <p v-if="history.length === 0" class="state-block__message">
            还没有辅导记录。可以先要一个一级提示（只给观察方向），也可以直接写下你现在的解释。
          </p>
          <ol v-else class="chat">
            <li v-for="(item, i) in history" :key="i" class="chat__item" :class="`chat__item--${item.kind}`">
              <template v-if="item.kind === 'hint'">
                <div class="chat__meta">
                  <span class="tag tag--hint">{{ hintLevelLabel(item.hint.level) }}</span>
                  <span v-if="!item.hint.socratic" class="tag tag--warning">直接排障步骤</span>
                  <span v-if="item.hint.requestedByStudent" class="tag tag--neutral">学生主动请求</span>
                  <span v-if="item.hint.targetKnowledge" class="tag tag--category">本轮目标：{{ item.hint.targetKnowledge }}</span>
                  <time class="chat__time">{{ formatDateTime(item.hint.createdAt) }}</time>
                </div>
                <div class="chat__bubble chat__bubble--hint">
                  <p v-for="(line, j) in item.hint.content.split('\n')" :key="j">{{ line }}</p>
                </div>
                <p v-if="item.hint.why" class="side-note">为什么给这一级：{{ item.hint.why }}</p>
              </template>
              <template v-else>
                <div class="chat__meta">
                  <span class="tag tag--student">我的修正解释</span>
                  <span class="tag" :class="`tag--${progressTagKind(item.revision.progress)}`">{{ progressLabel(item.revision.progress) }}</span>
                  <time class="chat__time">{{ formatDateTime(item.revision.createdAt) }}</time>
                </div>
                <div class="chat__bubble chat__bubble--student">
                  <p>{{ item.revision.content }}</p>
                </div>
                <div class="chat__feedback">
                  <p>{{ item.revision.feedback }}</p>
                  <ul v-if="item.revision.points && item.revision.points.length > 0" class="list list--tight">
                    <li v-for="(p, j) in item.revision.points" :key="j">{{ p }}</li>
                  </ul>
                  <div v-if="item.revision.addressedKeywords && item.revision.addressedKeywords.length > 0" class="tag-row">
                    <span class="tag-row__label">已覆盖：</span>
                    <span v-for="k in item.revision.addressedKeywords" :key="'a' + k" class="tag tag--success">{{ k }}</span>
                  </div>
                  <div v-if="item.revision.missingKeywords && item.revision.missingKeywords.length > 0" class="tag-row">
                    <span class="tag-row__label">仍缺少：</span>
                    <span v-for="k in item.revision.missingKeywords" :key="'m' + k" class="tag tag--warning">{{ k }}</span>
                  </div>
                </div>
              </template>
            </li>
          </ol>
        </div>

        <div class="card">
          <h2 class="card__title">提交我的解释</h2>
          <div class="field">
            <label class="field__label" for="revision-input">我现在的理解</label>
            <textarea
              id="revision-input"
              v-model="revisionText"
              class="textarea"
              rows="4"
              placeholder="根据提示重新解释你的实验结果，系统会判断是否有所修正并给出简短反馈。"
            ></textarea>
          </div>
          <div class="form-actions">
            <button type="button" class="btn btn--primary" :disabled="busy" @click="submitRevision">
              {{ revisionPending ? '正在提交…' : '我再想一想 / 提交我的解释' }}
            </button>
          </div>
        </div>
      </div>

      <aside class="panel-side">
        <div class="card agent-card">
          <h2 class="card__title">智能体决策</h2>
          <template v-if="agent">
            <div class="tag-row">
              <span class="tag tag--agent">{{ agent.actionLabel }}</span>
              <span v-if="agent.blocked" class="tag tag--warning">已阻止直接给示例</span>
            </div>
            <p class="agent-card__title">{{ agent.title }}</p>
            <p class="side-note">决策依据：{{ agent.reason }}</p>

            <div v-if="agent.requires && agent.requires.length > 0" class="agent-card__requires">
              <h3 class="card__subtitle">需要你补充的材料</h3>
              <ul class="list list--tight">
                <li v-for="(r, i) in agent.requires" :key="'req' + i">{{ r }}</li>
              </ul>
              <p v-if="agent.blockedReason" class="side-note">{{ agent.blockedReason }}</p>
            </div>

            <button
              type="button"
              class="btn btn--primary btn--block"
              :disabled="agentPending || !agent.canAct"
              @click="runAgentStep"
            >
              {{ agentPending ? '智能体正在执行…' : '让智能体决定下一步' }}
            </button>
            <p v-if="!agent.canAct" class="side-note">当前状态下没有需要系统自动执行的动作，按下面的建议继续即可。</p>
            <p v-if="agentNote" class="side-note" aria-live="polite">{{ agentNote }}</p>

            <div v-if="agent.plan && agent.plan.length > 0" class="agent-card__plan">
              <h3 class="card__subtitle">分步计划</h3>
              <ol class="agent-plan">
                <li v-for="item in agent.plan" :key="item.step" class="agent-plan__item" :class="`agent-plan__item--${item.status}`">
                  <span class="agent-plan__status">{{ planStatusLabel[item.status] ?? item.status }}</span>
                  <span class="agent-plan__title">{{ item.title }}</span>
                  <span v-if="item.note" class="agent-plan__note">备注：{{ item.note }}</span>
                </li>
              </ol>
            </div>

            <details v-if="agent.tools && agent.tools.length > 0" class="agent-card__tools">
              <summary>智能体运行了哪些检查？</summary>
              <ul class="list list--tight">
                <li v-for="(tool, i) in agent.tools" :key="'tool' + i">
                  <strong>{{ tool.purpose }}</strong>
                  <div class="side-note">{{ tool.result }}</div>
                </li>
              </ul>
            </details>

            <details v-if="agent.memory && agent.memory.lookedAt > 0" class="agent-card__memory">
              <summary>长期记忆（本机历史记录 {{ agent.memory.lookedAt }} 条）</summary>
              <div v-if="agent.memory.repeated && agent.memory.repeated.length > 0">
                <h3 class="card__subtitle">重复出现的误区</h3>
                <ul class="list list--tight">
                  <li v-for="r in agent.memory.repeated" :key="r.code">
                    <strong>{{ r.label }}</strong>（{{ r.count }} 次）
                    <div class="side-note">{{ r.advice }}</div>
                  </li>
                </ul>
              </div>
              <div v-if="agent.memory.mastered && agent.memory.mastered.length > 0">
                <h3 class="card__subtitle">已体现掌握的知识点</h3>
                <div class="tag-row">
                  <span v-for="m in agent.memory.mastered" :key="m" class="tag tag--success">{{ m }}</span>
                </div>
              </div>
              <div v-if="agent.memory.milestones && agent.memory.milestones.length > 0">
                <h3 class="card__subtitle">进展标记</h3>
                <ul class="list list--tight">
                  <li v-for="(m, i) in agent.memory.milestones" :key="'ms' + i">{{ m }}</li>
                </ul>
              </div>
              <p class="side-note">{{ agent.memory.note }}</p>
            </details>

            <p class="side-note agent-card__policy">{{ agent.policyNote }}</p>
          </template>
          <p v-else-if="agentError" class="form-error" role="alert">{{ agentError }}</p>
          <p v-else class="side-note">正在读取智能体的下一步计划…</p>
        </div>

        <div class="card">
          <h2 class="card__title">获取提示</h2>
          <p class="side-note">提示逐级加深：先观察方向，再概念解释，最后局部示例。每级提示都会说明针对哪个知识点，并避开你已经掌握的内容。</p>
          <div class="stack">
            <button type="button" class="btn btn--secondary btn--block" :disabled="busy" @click="requestHint(1)">
              {{ hintPending === 1 ? '正在获取…' : '给我一级提示' }}
            </button>
            <button type="button" class="btn btn--secondary btn--block" :disabled="busy" @click="requestHint(2)">
              {{ hintPending === 2 ? '正在获取…' : '给我二级提示' }}
            </button>
            <button type="button" class="btn btn--secondary btn--block" :disabled="busy" @click="requestHint(3)">
              {{ hintPending === 3 ? '正在获取…' : '查看局部示例' }}
            </button>
          </div>
          <p v-if="session.hintState.usedLevels && session.hintState.usedLevels.length > 0" class="side-note">
            已使用：{{ session.hintState.usedLevels.map(hintLevelLabel).join('、') }}
          </p>
        </div>

        <div class="card">
          <h2 class="card__title">下一步</h2>
          <p class="side-note">觉得自己已经想清楚后，进入迁移测试：一道新的情境题，先独立作答。</p>
          <button type="button" class="btn btn--primary btn--block" :disabled="busy" @click="enterTransfer">
            {{ transferPending ? '正在生成迁移题…' : '进入迁移测试' }}
          </button>
        </div>

        <p v-if="hintNotice" class="side-note" aria-live="polite">{{ hintNotice }}</p>
        <div v-if="panelError" class="alert alert--danger" role="alert">{{ panelError }}</div>
      </aside>
    </div>
  </section>
</template>
