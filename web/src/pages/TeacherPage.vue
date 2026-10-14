<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api, exportUrl } from '../api/client'
import type { CommonIssue, TeacherCounts, TeacherRecord } from '../types'
import { copyText, errorMessage, formatDateTime } from '../lib/utils'
import AlertBox from '../components/AlertBox.vue'
import StateBlock from '../components/StateBlock.vue'

const records = ref<TeacherRecord[]>([])
const commonIssues = ref<CommonIssue[]>([])
const counts = ref<TeacherCounts | null>(null)
const note = ref('')
const loading = ref(true)
const loadError = ref('')
const copiedId = ref('')

const titleById = computed(() => {
  const m = new Map<string, string>()
  for (const r of records.value) m.set(r.record.id, r.record.title)
  return m
})

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const res = await api.getTeacherOverview()
    records.value = res.records
    commonIssues.value = res.commonIssues
    counts.value = res.counts
    note.value = res.note
  } catch (err) {
    loadError.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function copySummary(item: TeacherRecord): Promise<void> {
  if (!item.summary) return
  const ok = await copyText(item.summary)
  if (ok) {
    copiedId.value = item.record.id
    setTimeout(() => {
      copiedId.value = ''
    }, 2500)
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <h1 class="page-title">教师视图</h1>
      <p class="page-lead">查看已保存学习记录的结构化摘要与共性问题。</p>
    </header>

    <StateBlock v-if="loading" variant="loading" title="正在读取教师视图…" />
    <StateBlock v-else-if="loadError" variant="error" title="读取失败" :message="loadError">
      <button type="button" class="btn btn--secondary" @click="load">重试</button>
    </StateBlock>

    <template v-else>
      <AlertBox kind="info" title="统计口径说明">{{ note }}</AlertBox>

      <div v-if="counts" class="stat-strip">
        <div class="stat-strip__item">
          <span class="stat-strip__value">{{ counts.total }}</span>
          <span class="stat-strip__label">记录总数</span>
        </div>
        <div class="stat-strip__item">
          <span class="stat-strip__value">{{ counts.real }}</span>
          <span class="stat-strip__label">真实提交</span>
        </div>
        <div class="stat-strip__item">
          <span class="stat-strip__value">{{ counts.demo }}</span>
          <span class="stat-strip__label">模拟案例</span>
        </div>
      </div>

      <section class="card" aria-labelledby="common-title">
        <h2 id="common-title" class="card__title">共性问题（基于已保存记录）</h2>
        <p v-if="commonIssues.length === 0" class="state-block__message">暂无足够记录统计共性问题。</p>
        <ul v-else class="common-list">
          <li v-for="ci in commonIssues" :key="ci.code" class="common-list__item">
            <span class="tag tag--category">{{ ci.label }}</span>
            <span class="common-list__count">出现 {{ ci.count }} 次</span>
            <span class="common-list__records">
              涉及记录：
              <RouterLink v-for="rid in ci.recordIds" :key="rid" :to="`/session/${rid}`" class="common-list__link">
                {{ titleById.get(rid) ?? rid }}
              </RouterLink>
            </span>
          </li>
        </ul>
      </section>

      <StateBlock
        v-if="records.length === 0"
        variant="empty"
        title="还没有学习记录"
        message="学生完成学习诊断后，这里会显示每条记录的摘要与共性问题。"
      />

      <div v-else class="teacher-list">
        <article v-for="item in records" :key="item.record.id" class="card teacher-card">
          <header class="teacher-card__head">
            <h2 class="teacher-card__title">
              <RouterLink :to="`/session/${item.record.id}`">{{ item.record.title }}</RouterLink>
            </h2>
            <div class="tag-row">
              <span v-if="item.record.isDemo" class="tag tag--demo">模拟案例</span>
              <span class="tag tag--status">{{ item.record.statusLabel }}</span>
              <span v-if="item.intervention" class="tag tag--danger">建议教师介入</span>
            </div>
          </header>
          <dl class="kv">
            <div class="kv__row">
              <dt>创建时间</dt>
              <dd>{{ formatDateTime(item.record.createdAt) }}</dd>
            </div>
            <div class="kv__row">
              <dt>主要问题</dt>
              <dd>{{ item.mainIssue || '—' }}</dd>
            </div>
            <div class="kv__row">
              <dt>提示等级</dt>
              <dd>
                <template v-if="item.hintLevels && item.hintLevels.length > 0">
                  <span v-for="l in item.hintLevels" :key="l" class="tag tag--hint">{{ l }} 级</span>
                </template>
                <span v-else>未使用</span>
              </dd>
            </div>
          </dl>
          <div class="teacher-card__summary">
            <h3 class="card__subtitle">教师摘要</h3>
            <p v-if="item.summary" class="side-note side-note--pre">{{ item.summary }}</p>
            <p v-else class="state-block__message">尚未生成学习反馈摘要（该记录可能还未完成迁移测试）。</p>
          </div>
          <footer class="teacher-card__foot">
            <RouterLink :to="`/session/${item.record.id}`" class="btn btn--secondary btn--sm">查看详情</RouterLink>
            <button v-if="item.summary" type="button" class="btn btn--ghost btn--sm" @click="copySummary(item)">
              {{ copiedId === item.record.id ? '已复制' : '复制摘要' }}
            </button>
            <a v-if="item.summary" class="btn btn--ghost btn--sm" :href="exportUrl(item.record.id, 'md')" download>导出 Markdown</a>
          </footer>
        </article>
      </div>
    </template>
  </div>
</template>
