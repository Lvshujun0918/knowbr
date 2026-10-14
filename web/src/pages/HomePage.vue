<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { appState, ensureDemoCases, ensureMeta, setDraft } from '../state'
import AlertBox from '../components/AlertBox.vue'
import ModeBadge from '../components/ModeBadge.vue'

const router = useRouter()
const loadingDemo = ref(false)
const demoError = ref('')

const meta = computed(() => appState.meta)

onMounted(() => {
  void ensureMeta()
})

/** 体验演示案例：加载核心演示案例（case-imbalance），填入草稿后进入提交页。 */
async function tryDemo(): Promise<void> {
  loadingDemo.value = true
  demoError.value = ''
  try {
    await ensureDemoCases()
    const core = appState.demoCases?.find((c) => c.id === 'case-imbalance') ?? appState.demoCases?.[0]
    if (!core) {
      demoError.value = '演示案例加载失败，请确认服务端已启动后重试。'
      return
    }
    setDraft({ caseId: core.id, caseName: core.name, dataNotice: core.dataNotice, submission: core.submission })
    await router.push('/submit')
  } finally {
    loadingDemo.value = false
  }
}
</script>

<template>
  <div class="page">
    <section class="hero">
      <div class="hero__main">
        <p class="hero__course">{{ meta?.course ?? '《人工智能基础（A）》' }}</p>
        <h1 class="hero__title">{{ meta?.productName ?? '知行桥——人工智能基础课程教学衔接智能体' }}</h1>
        <p class="hero__tagline">
          {{ meta?.tagline ?? '以实验结果为证据，通过诊断、分层提示和迁移检验，帮助学生真正理解人工智能实验。' }}
        </p>
        <div class="hero__actions">
          <RouterLink to="/submit" class="btn btn--primary btn--lg">开始学习诊断</RouterLink>
          <button type="button" class="btn btn--secondary btn--lg" :disabled="loadingDemo" @click="tryDemo">
            {{ loadingDemo ? '正在加载演示案例…' : '体验演示案例' }}
          </button>
          <RouterLink to="/records" class="btn btn--ghost btn--lg">查看学习记录 / 教师反馈</RouterLink>
          <RouterLink to="/help" class="btn btn--ghost btn--lg">使用说明</RouterLink>
        </div>
        <p v-if="demoError" class="form-error" role="alert">{{ demoError }}</p>
      </div>
      <aside class="hero__side">
        <div class="card hero__mode-card">
          <h2 class="card__title">当前运行模式</h2>
          <p v-if="meta" class="hero__mode-line">
            <ModeBadge :mode="meta.mode" />
          </p>
          <p v-if="meta" class="hero__mode-notice">{{ meta.mode.notice }}</p>
          <p v-else-if="appState.metaError" class="hero__mode-notice">无法获取运行模式：{{ appState.metaError }}</p>
          <p v-else class="hero__mode-notice">正在获取运行模式…</p>
        </div>
        <div v-if="meta" class="card hero__focus-card">
          <h2 class="card__title">本期聚焦</h2>
          <p class="hero__focus">{{ meta.focus }}</p>
        </div>
      </aside>
    </section>

    <section class="card flow-card" aria-labelledby="flow-title">
      <h2 id="flow-title" class="card__title">一轮完整的学习流程</h2>
      <ol class="flow">
        <li v-for="(step, i) in meta?.steps ?? []" :key="step.key" class="flow__item">
          <span class="flow__num" aria-hidden="true">{{ i + 1 }}</span>
          <span class="flow__title">{{ step.title }}</span>
          <span v-if="i < (meta?.steps.length ?? 0) - 1" class="flow__arrow" aria-hidden="true">→</span>
        </li>
      </ol>
    </section>

    <section class="home-grid">
      <div class="card">
        <h2 class="card__title">它如何工作</h2>
        <ul class="list">
          <li><strong>以证据诊断</strong>：诊断引用你提交的混淆矩阵、指标与解释，不凭空猜测。</li>
          <li><strong>以提示促学</strong>：先给观察方向，再给概念解释，最后才给局部示例，而不是直接给答案。</li>
          <li><strong>以迁移验证</strong>：用一道新情境题检验是否真正理解，先作答，再看参考答案。</li>
          <li><strong>以反馈衔接教学</strong>：把误区、提示轨迹与迁移表现整理为教师可用的学习反馈。</li>
        </ul>
      </div>
      <div class="card">
        <h2 class="card__title">诚实声明</h2>
        <ul class="list">
          <li v-for="(line, i) in meta?.honesty ?? []" :key="i">{{ line }}</li>
        </ul>
        <AlertBox v-if="meta" kind="info" title="数据保存">
          {{ meta.storage.note }}
        </AlertBox>
      </div>
    </section>
  </div>
</template>
