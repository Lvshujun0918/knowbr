<script setup lang="ts">
import { computed, ref } from 'vue'
import { api } from '../../api/client'
import type { Hint, HintState, Revision, Session, TransferState } from '../../types'
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
                  <time class="chat__time">{{ formatDateTime(item.hint.createdAt) }}</time>
                </div>
                <div class="chat__bubble chat__bubble--hint">
                  <p v-for="(line, j) in item.hint.content.split('\n')" :key="j">{{ line }}</p>
                </div>
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
        <div class="card">
          <h2 class="card__title">获取提示</h2>
          <p class="side-note">提示逐级加深：先观察方向，再概念解释，最后局部示例。</p>
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
