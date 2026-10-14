<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, ApiError } from '../api/client'
import { appState, ensureDemoCases, takeDraft } from '../state'
import type { DemoCase, MatrixMetrics } from '../types'
import { errorMessage, pct } from '../lib/utils'
import AlertBox from '../components/AlertBox.vue'
import MatrixEditor from '../components/MatrixEditor.vue'

const router = useRouter()

const DEFAULT_TITLE = '分类模型评价实验'

const form = reactive({
  experimentTitle: DEFAULT_TITLE,
  experimentGoal: '',
  datasetDescription: '',
  modelName: '',
  codeSnippet: '',
  executionLog: '',
  confusionMatrixText: '',
  metricsText: '',
  studentInitialExplanation: '',
  question: '',
})

/* 表格编辑器状态：单元格保留用户原始输入（字符串），空白按 0 处理。 */
const matrixLabels = ref<string[]>(['类别A', '类别B', '类别C'])
const matrixCells = ref<string[][]>([
  ['', '', ''],
  ['', '', ''],
  ['', '', ''],
])

const demoCaseId = ref('')
const demoCaseName = ref('')
const demoNotice = ref('')

const showCasePanel = ref(false)
const casesLoading = ref(false)
const caseListError = ref('')

const matrixError = ref('')
const matrixChecking = ref(false)
const matrixResult = ref<MatrixMetrics | null>(null)

const submitError = ref('')
const submitting = ref(false)

let validateTimer: ReturnType<typeof setTimeout> | null = null
let validateSeq = 0

const tableHasContent = computed(() => matrixCells.value.some((row) => row.some((c) => c.trim() !== '')))

/** 本地解析表格：空白 → 0；非数字 → 报错并指出位置。 */
function parseTable(): number[][] | string {
  const out: number[][] = []
  for (let i = 0; i < matrixCells.value.length; i++) {
    const row: number[] = []
    for (let j = 0; j < matrixCells.value[i].length; j++) {
      const raw = matrixCells.value[i][j].trim()
      if (raw === '') {
        row.push(0)
        continue
      }
      const n = Number(raw)
      if (!Number.isFinite(n)) {
        return `表格第 ${i + 1} 行第 ${j + 1} 列的 “${raw}” 不是有效数字，请只填写数字。`
      }
      row.push(n)
    }
    out.push(row)
  }
  return out
}

/** 实时校验：表格优先，其次粘贴文本；调用 POST /api/metrics/compute。 */
async function validateMatrix(): Promise<void> {
  const seq = ++validateSeq
  matrixError.value = ''
  matrixResult.value = null
  matrixChecking.value = false

  if (tableHasContent.value) {
    const parsed = parseTable()
    if (typeof parsed === 'string') {
      matrixError.value = parsed
      return
    }
    matrixChecking.value = true
    try {
      const res = await api.computeMetrics({
        confusionMatrix: parsed,
        classLabels: matrixLabels.value.map((l) => l.trim()),
      })
      if (seq !== validateSeq) return
      matrixResult.value = res.metrics
    } catch (err) {
      if (seq !== validateSeq) return
      matrixError.value = errorMessage(err)
    } finally {
      if (seq === validateSeq) matrixChecking.value = false
    }
    return
  }

  if (form.confusionMatrixText.trim() !== '') {
    matrixChecking.value = true
    try {
      const res = await api.computeMetrics({ confusionMatrixText: form.confusionMatrixText })
      if (seq !== validateSeq) return
      matrixResult.value = res.metrics
    } catch (err) {
      if (seq !== validateSeq) return
      matrixError.value = errorMessage(err)
    } finally {
      if (seq === validateSeq) matrixChecking.value = false
    }
  }
}

function scheduleValidate(): void {
  if (validateTimer) clearTimeout(validateTimer)
  validateTimer = setTimeout(() => {
    void validateMatrix()
  }, 600)
}

watch([matrixCells, matrixLabels, () => form.confusionMatrixText], scheduleValidate, { deep: true })

onBeforeUnmount(() => {
  if (validateTimer) clearTimeout(validateTimer)
})

/** 把演示案例的矩阵文本同步填入表格编辑器（可解析且 ≤6 阶时）。 */
function fillTableFromText(text: string, labels: string[]): void {
  const rows = text
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '')
    .map((line) => line.split(/[\s,;，、]+/).filter((f) => f !== ''))
  if (rows.length >= 2 && rows.length <= 6 && rows.every((r) => r.length === rows.length)) {
    if (rows.every((r) => r.every((f) => Number.isFinite(Number(f))))) {
      matrixCells.value = rows.map((r) => r.slice())
      if (labels.length === rows.length) {
        matrixLabels.value = labels.slice()
      } else {
        matrixLabels.value = rows.map((_, i) => `类别${'ABCDEFGHIJKLMNOPQRSTUVWXYZ'[i] ?? i + 1}`)
      }
    }
  }
}

function applyDemo(dc: DemoCase): void {
  const s = dc.submission
  matrixCells.value = [['', '', ''], ['', '', ''], ['', '', '']]
  matrixLabels.value = ['类别A', '类别B', '类别C']
  form.experimentTitle = s.experimentTitle || DEFAULT_TITLE
  form.experimentGoal = s.experimentGoal
  form.datasetDescription = s.datasetDescription
  form.modelName = s.modelName
  form.codeSnippet = s.codeSnippet
  form.executionLog = s.executionLog
  form.confusionMatrixText = s.confusionMatrixText
  form.metricsText = s.metricsText
  form.studentInitialExplanation = s.studentInitialExplanation
  form.question = s.question
  demoCaseId.value = dc.id
  demoCaseName.value = dc.name
  demoNotice.value = dc.dataNotice
  if (s.confusionMatrixText.trim() !== '') {
    fillTableFromText(s.confusionMatrixText, s.classLabels ?? [])
  }
  showCasePanel.value = false
  void validateMatrix()
}

async function openCasePanel(): Promise<void> {
  showCasePanel.value = true
  caseListError.value = ''
  if (appState.demoCases) return
  casesLoading.value = true
  try {
    await ensureDemoCases()
    const loadedCases = appState.demoCases as DemoCase[] | null
    if (!loadedCases || loadedCases.length === 0) {
      caseListError.value = '演示案例加载失败，请确认服务端已启动后重试。'
    }
  } finally {
    casesLoading.value = false
  }
}

function clearAll(): void {
  form.experimentTitle = DEFAULT_TITLE
  form.experimentGoal = ''
  form.datasetDescription = ''
  form.modelName = ''
  form.codeSnippet = ''
  form.executionLog = ''
  form.confusionMatrixText = ''
  form.metricsText = ''
  form.studentInitialExplanation = ''
  form.question = ''
  matrixLabels.value = ['类别A', '类别B', '类别C']
  matrixCells.value = [
    ['', '', ''],
    ['', '', ''],
    ['', '', ''],
  ]
  demoCaseId.value = ''
  demoCaseName.value = ''
  demoNotice.value = ''
  submitError.value = ''
  matrixError.value = ''
  matrixChecking.value = false
  matrixResult.value = null
  validateSeq++
}

async function submit(): Promise<void> {
  submitError.value = ''
  if (form.studentInitialExplanation.trim() === '') {
    submitError.value = '请先填写“学生自己的实验解释”（必填）。系统需要基于你自己的判断来诊断，而不是替你判断。'
    return
  }
  if (validateTimer) clearTimeout(validateTimer)
  await validateMatrix()
  if (matrixError.value) {
    submitError.value = `混淆矩阵存在问题：${matrixError.value}`
    return
  }

  const payload: Parameters<typeof api.createSession>[0] = {
    experimentTitle: form.experimentTitle.trim(),
    experimentGoal: form.experimentGoal,
    datasetDescription: form.datasetDescription,
    modelName: form.modelName,
    codeSnippet: form.codeSnippet,
    executionLog: form.executionLog,
    metricsText: form.metricsText,
    studentInitialExplanation: form.studentInitialExplanation,
    question: form.question,
  }
  if (demoCaseId.value) payload.demoCaseId = demoCaseId.value

  if (tableHasContent.value) {
    const parsed = parseTable()
    if (typeof parsed === 'string') {
      submitError.value = parsed
      return
    }
    payload.confusionMatrix = parsed
    payload.classLabels = matrixLabels.value.map((l) => l.trim())
  } else if (form.confusionMatrixText.trim() !== '') {
    payload.confusionMatrixText = form.confusionMatrixText
  }

  submitting.value = true
  try {
    const res = await api.createSession(payload)
    await router.push(`/session/${res.session.id}`)
  } catch (err) {
    submitError.value = err instanceof ApiError ? err.message : errorMessage(err)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  const draft = takeDraft()
  if (draft) {
    applyDemo({ id: draft.caseId, name: draft.caseName, dataNotice: draft.dataNotice, description: '', expectedFocus: [], submission: draft.submission })
  }
})

const matrixStateText = computed(() => {
  if (matrixChecking.value) return '正在校验混淆矩阵…'
  if (matrixResult.value) {
    const m = matrixResult.value
    return `矩阵有效：总样本 ${m.total}，整体准确率 ${pct(m.accuracy)}，宏平均 F1 ${m.macroF1.toFixed(3)}`
  }
  return ''
})
</script>

<template>
  <div class="page page--narrow">
    <header class="page-head">
      <h1 class="page-title">提交实验材料</h1>
      <p class="page-lead">
        填写你的实验信息与自己的判断。带 <span class="required-mark">必填</span> 的字段必须填写；
        其余字段越完整，诊断越有依据。
      </p>
    </header>

    <AlertBox v-if="demoNotice" kind="warning" title="模拟案例">
      <p>{{ demoNotice }}</p>
      <p class="alert__extra">已载入：{{ demoCaseName }}。你可以在此基础上修改任何字段。</p>
    </AlertBox>

    <div class="submit-actions-bar">
      <button type="button" class="btn btn--secondary" @click="openCasePanel">加载演示案例</button>
      <button type="button" class="btn btn--ghost" @click="clearAll">清空</button>
    </div>

    <div v-if="showCasePanel" class="card case-panel">
      <h2 class="card__title">选择演示案例</h2>
      <p v-if="appState.demoNotice" class="case-panel__notice">{{ appState.demoNotice }}</p>
      <p v-if="casesLoading" class="state-inline"><span class="spinner spinner--sm" aria-hidden="true"></span> 正在加载案例…</p>
      <p v-else-if="caseListError" class="form-error" role="alert">{{ caseListError }}</p>
      <div v-else class="case-panel__list">
        <article v-for="dc in appState.demoCases ?? []" :key="dc.id" class="case-card">
          <h3 class="case-card__name">{{ dc.name }}</h3>
          <p class="case-card__desc">{{ dc.description }}</p>
          <ul v-if="dc.expectedFocus.length > 0" class="case-card__focus">
            <li v-for="(f, i) in dc.expectedFocus" :key="i">{{ f }}</li>
          </ul>
          <div class="case-card__foot">
            <span class="tag tag--demo">模拟案例</span>
            <button type="button" class="btn btn--primary btn--sm" @click="applyDemo(dc)">填入此案例</button>
          </div>
        </article>
      </div>
    </div>

    <form class="form" novalidate @submit.prevent="submit">
      <div class="card">
        <h2 class="card__title">基本信息</h2>
        <div class="field">
          <label class="field__label" for="f-title">实验名称 <span class="required-mark">必填</span></label>
          <input id="f-title" v-model="form.experimentTitle" class="input" type="text" :placeholder="DEFAULT_TITLE" />
        </div>
        <div class="field">
          <label class="field__label" for="f-goal">实验目标</label>
          <textarea id="f-goal" v-model="form.experimentGoal" class="textarea" rows="2" placeholder="例如：完成一个三分类任务，并判断该模型是否可以直接交付使用。"></textarea>
        </div>
        <div class="field">
          <label class="field__label" for="f-dataset">数据集或类别分布</label>
          <textarea id="f-dataset" v-model="form.datasetDescription" class="textarea" rows="2" placeholder="例如：类别 A 900 个、类别 B 90 个、类别 C 10 个，总计 1000 个样本。"></textarea>
        </div>
        <div class="field">
          <label class="field__label" for="f-model">模型或算法名称</label>
          <input id="f-model" v-model="form.modelName" class="input" type="text" placeholder="例如：默认参数的 KNN 分类器" />
        </div>
      </div>

      <div class="card">
        <h2 class="card__title">运行材料</h2>
        <div class="field">
          <label class="field__label" for="f-code">实验代码或关键代码片段</label>
          <textarea id="f-code" v-model="form.codeSnippet" class="textarea textarea--mono" rows="5" placeholder="粘贴关键代码即可；系统不会运行你的代码。"></textarea>
        </div>
        <div class="field">
          <label class="field__label" for="f-log">运行日志 / 运行结果</label>
          <textarea id="f-log" v-model="form.executionLog" class="textarea textarea--mono" rows="4" placeholder="例如：accuracy: 0.90 或报错信息。"></textarea>
        </div>
      </div>

      <div class="card">
        <h2 class="card__title">混淆矩阵（选填）</h2>
        <p class="card__desc">可按表格填写，或直接粘贴文本。<strong>两者同时填写时，以表格内容为准。</strong></p>
        <MatrixEditor v-model:labels="matrixLabels" v-model:cells="matrixCells" />
        <div class="field">
          <label class="field__label" for="f-matrixtext">或粘贴矩阵文本</label>
          <textarea
            id="f-matrixtext"
            v-model="form.confusionMatrixText"
            class="textarea textarea--mono"
            rows="4"
            placeholder="每行一个真实类别，数字之间用空格或逗号分隔，例如：&#10;900 0 0&#10;90 0 0&#10;10 0 0"
          ></textarea>
        </div>
        <div class="matrix-status" aria-live="polite">
          <p v-if="matrixChecking" class="state-inline"><span class="spinner spinner--sm" aria-hidden="true"></span> {{ matrixStateText }}</p>
          <p v-else-if="matrixError" class="form-error" role="alert">{{ matrixError }}</p>
          <p v-else-if="matrixResult" class="form-ok">{{ matrixStateText }}</p>
        </div>
      </div>

      <div class="card">
        <h2 class="card__title">你的判断</h2>
        <div class="field">
          <label class="field__label" for="f-metrics">评价指标</label>
          <textarea id="f-metrics" v-model="form.metricsText" class="textarea" rows="2" placeholder="例如：准确率 accuracy = 0.90，其他指标没有计算。"></textarea>
        </div>
        <div class="field">
          <label class="field__label" for="f-explanation">学生自己的实验解释 <span class="required-mark">必填</span></label>
          <textarea
            id="f-explanation"
            v-model="form.studentInitialExplanation"
            class="textarea"
            rows="3"
            placeholder="用你自己的话说明：你认为实验结果怎么样？依据是什么？"
            aria-required="true"
          ></textarea>
          <p class="field__hint">系统基于你自己的判断来诊断，而不是替你判断。</p>
        </div>
        <div class="field">
          <label class="field__label" for="f-question">当前最困惑的问题</label>
          <textarea id="f-question" v-model="form.question" class="textarea" rows="2" placeholder="例如：为什么准确率这么高，我还是不确定这个模型能不能用？"></textarea>
        </div>
      </div>

      <div v-if="submitError" class="alert alert--danger" role="alert">{{ submitError }}</div>

      <div class="form-actions">
        <button type="submit" class="btn btn--primary btn--lg" :disabled="submitting">
          {{ submitting ? '正在诊断…' : '开始诊断' }}
        </button>
        <p class="form-actions__note">提交后系统会先检查材料完整性，再基于你提交的证据生成诊断。</p>
      </div>
    </form>
  </div>
</template>
