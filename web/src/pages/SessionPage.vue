<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { api } from '../api/client'
import { appState, ensureMeta } from '../state'
import type { Hint, HintState, Revision, Session, StepDef, Summary, TransferState } from '../types'
import { errorMessage, sessionStepIndex } from '../lib/utils'
import StepIndicator from '../components/StepIndicator.vue'
import StateBlock from '../components/StateBlock.vue'
import DiagnosisPanel from '../components/session/DiagnosisPanel.vue'
import TutoringPanel from '../components/session/TutoringPanel.vue'
import TransferPanel from '../components/session/TransferPanel.vue'
import ReportPanel from '../components/session/ReportPanel.vue'

const props = defineProps<{ id: string }>()
const router = useRouter()

const FALLBACK_STEPS: StepDef[] = [
  { key: 'submit', title: '提交材料' },
  { key: 'diagnosis', title: '学习诊断' },
  { key: 'tutoring', title: '分层辅导' },
  { key: 'transfer', title: '迁移测试' },
  { key: 'report', title: '学习反馈' },
]

const session = ref<Session | null>(null)
const loading = ref(true)
const loadError = ref('')
const activePanel = ref(1)

const steps = computed(() => appState.meta?.steps ?? FALLBACK_STEPS)
const reached = computed(() => (session.value ? sessionStepIndex(session.value.status) : 1))
const reportReady = computed(() => session.value?.status === 'completed')

onMounted(async () => {
  void ensureMeta()
  try {
    const res = await api.getSession(props.id)
    session.value = res.session
    activePanel.value = sessionStepIndex(res.session.status)
  } catch (err) {
    loadError.value = errorMessage(err)
  } finally {
    loading.value = false
  }
})

function selectPanel(i: number): void {
  if (i === 0) {
    void router.push('/submit')
    return
  }
  activePanel.value = i
}

function onTutoringUpdate(payload: { hints: Hint[]; hintState: HintState; revisions: Revision[] }): void {
  if (!session.value) return
  session.value.hints = payload.hints
  session.value.hintState = payload.hintState
  session.value.revisions = payload.revisions
  if (session.value.status === 'diagnosed') session.value.status = 'tutoring'
}

function onTransferUpdate(t: TransferState): void {
  if (!session.value) return
  session.value.transfer = t
  if (session.value.status === 'diagnosed' || session.value.status === 'tutoring') {
    session.value.status = 'transfer'
  }
}

function onCompleted(payload: { transfer: TransferState; summary: Summary; session: Session }): void {
  session.value = payload.session
  session.value.transfer = payload.transfer
  session.value.summary = payload.summary
  activePanel.value = 4
}
</script>

<template>
  <div class="page">
    <StateBlock v-if="loading" variant="loading" title="正在载入学习记录…" />
    <StateBlock v-else-if="loadError" variant="error" title="无法载入学习记录" :message="loadError">
      <RouterLink to="/records" class="btn btn--secondary">返回学习记录</RouterLink>
    </StateBlock>

    <template v-else-if="session">
      <header class="page-head page-head--row">
        <div>
          <h1 class="page-title">
            {{ session.experiment.experimentTitle || '未命名实验' }}
            <span v-if="session.isDemo" class="tag tag--demo">模拟案例</span>
          </h1>
          <p class="page-lead">{{ session.modeLabel }} · 创建于 {{ session.createdAt.slice(0, 10) }}</p>
        </div>
        <RouterLink to="/records" class="btn btn--ghost">返回记录列表</RouterLink>
      </header>

      <div v-if="session.dataNotice" class="alert alert--warning" role="note">
        <strong class="alert__title">模拟案例</strong>
        <div class="alert__body">{{ session.dataNotice }}</div>
      </div>

      <div class="step-wrap">
        <StepIndicator
          :steps="steps"
          :current="activePanel"
          :reached="reached"
          :disabled-from="reportReady ? undefined : 4"
          @select="selectPanel"
        />
      </div>

      <DiagnosisPanel v-if="activePanel === 1" :session="session" />
      <div v-if="activePanel === 1" class="form-actions">
        <button type="button" class="btn btn--primary" @click="activePanel = 2">进入分层辅导</button>
      </div>
      <TutoringPanel
        v-else-if="activePanel === 2"
        :session="session"
        @update="onTutoringUpdate"
        @enter-transfer="
          (t) => {
            onTransferUpdate(t)
            activePanel = 3
          }
        "
      />
      <TransferPanel
        v-else-if="activePanel === 3"
        :session="session"
        @update="onTransferUpdate"
        @completed="onCompleted"
      />
      <ReportPanel v-else-if="activePanel === 4" :session="session" />
    </template>
  </div>
</template>
