<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api/client'
import type { RecordDTO, SessionListStorage } from '../types'
import { errorMessage, formatDateTime, transferScoreText } from '../lib/utils'
import AlertBox from '../components/AlertBox.vue'
import StateBlock from '../components/StateBlock.vue'

const records = ref<RecordDTO[]>([])
const storage = ref<SessionListStorage | null>(null)
const loading = ref(true)
const loadError = ref('')
const actionError = ref('')
const deletingId = ref('')
const clearing = ref(false)

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const res = await api.listSessions()
    records.value = res.records
    storage.value = res.storage
  } catch (err) {
    loadError.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function remove(id: string, title: string): Promise<void> {
  if (!window.confirm(`确定删除学习记录「${title}」吗？此操作不可恢复。`)) return
  actionError.value = ''
  deletingId.value = id
  try {
    await api.deleteSession(id)
    const removed = records.value.find((r) => r.id === id)
    records.value = records.value.filter((r) => r.id !== id)
    if (storage.value) {
      // 说明文字里的三条计数必须同时跟着变，否则删除后会与列表实际条数不一致。
      storage.value = {
        ...storage.value,
        total: Math.max(0, storage.value.total - 1),
        demoRecords: Math.max(0, storage.value.demoRecords - (removed?.isDemo ? 1 : 0)),
        realRecords: Math.max(0, storage.value.realRecords - (removed?.isDemo ? 0 : 1)),
      }
    }
  } catch (err) {
    actionError.value = errorMessage(err)
  } finally {
    deletingId.value = ''
  }
}

async function clearAll(): Promise<void> {
  if (!window.confirm(`确定清空全部 ${records.value.length} 条学习记录吗？此操作不可恢复。`)) return
  actionError.value = ''
  clearing.value = true
  try {
    await api.clearSessions()
    records.value = []
    if (storage.value) storage.value = { ...storage.value, total: 0, demoRecords: 0, realRecords: 0 }
  } catch (err) {
    actionError.value = errorMessage(err)
  } finally {
    clearing.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head page-head--row">
      <div>
        <h1 class="page-title">学习记录</h1>
        <p class="page-lead">保存在本机服务端数据库中的学习过程记录。</p>
      </div>
      <div class="page-head__actions">
        <RouterLink to="/submit" class="btn btn--primary">开始新的学习诊断</RouterLink>
        <button v-if="records.length > 0" type="button" class="btn btn--danger" :disabled="clearing" @click="clearAll">
          {{ clearing ? '正在清空…' : '清空全部' }}
        </button>
      </div>
    </header>

    <AlertBox v-if="storage" kind="info" title="记录保存说明">
      {{ storage.note }}（当前共 {{ storage.total }} 条：演示记录 {{ storage.demoRecords }} 条，真实提交 {{ storage.realRecords }} 条。）
    </AlertBox>

    <StateBlock v-if="loading" variant="loading" title="正在读取学习记录…" />
    <StateBlock v-else-if="loadError" variant="error" title="读取失败" :message="loadError">
      <button type="button" class="btn btn--secondary" @click="load">重试</button>
    </StateBlock>
    <StateBlock v-else-if="records.length === 0" variant="empty" title="还没有学习记录" message="提交一次实验材料并完成诊断后，记录会显示在这里。">
      <RouterLink to="/submit" class="btn btn--primary">开始学习诊断</RouterLink>
    </StateBlock>

    <template v-else>
      <div v-if="actionError" class="alert alert--danger" role="alert">{{ actionError }}</div>
      <div class="table-scroll card card--flush">
        <table class="table table--records" aria-label="学习记录列表">
          <thead>
            <tr>
              <th scope="col">记录</th>
              <th scope="col">创建时间</th>
              <th scope="col">状态</th>
              <th scope="col">主要问题</th>
              <th scope="col">提示等级</th>
              <th scope="col">迁移得分</th>
              <th scope="col">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in records" :key="r.id">
              <td>
                <RouterLink :to="`/session/${r.id}`" class="record-title">{{ r.title }}</RouterLink>
                <div class="tag-row">
                  <span v-if="r.isDemo" class="tag tag--demo">模拟案例</span>
                  <span v-if="r.teacherIntervention" class="tag tag--danger">建议教师介入</span>
                </div>
              </td>
              <td class="table__nowrap">{{ formatDateTime(r.createdAt) }}</td>
              <td><span class="tag tag--status">{{ r.statusLabel }}</span></td>
              <td class="table__ellipsis" :title="r.mainIssue">{{ r.mainIssue || '—' }}</td>
              <td>
                <template v-if="r.hintLevelsUsed && r.hintLevelsUsed.length > 0">
                  <span v-for="l in r.hintLevelsUsed" :key="l" class="tag tag--hint">{{ l }} 级</span>
                </template>
                <span v-else>未使用</span>
              </td>
              <td class="table__nowrap">{{ transferScoreText(r.transferScore, r.transferMaxScore) }}</td>
              <td class="table__nowrap">
                <RouterLink :to="`/session/${r.id}`" class="btn btn--ghost btn--sm">查看详情</RouterLink>
                <button type="button" class="btn btn--danger-ghost btn--sm" :disabled="deletingId === r.id" @click="remove(r.id, r.title)">
                  {{ deletingId === r.id ? '删除中…' : '删除' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>
