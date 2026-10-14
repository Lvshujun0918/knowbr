import { reactive } from 'vue'
import { api } from './api/client'
import type { DemoCase, DemoSubmission, Meta } from './types'

/** 跨页面共享的轻量状态：元信息、演示案例缓存、首页带入提交页的草稿。 */
export interface SubmitDraft {
  caseId: string
  caseName: string
  dataNotice: string
  submission: DemoSubmission
}

export const appState = reactive({
  meta: null as Meta | null,
  metaError: '',
  demoCases: null as DemoCase[] | null,
  demoNotice: '',
  draft: null as SubmitDraft | null,
})

let metaPromise: Promise<void> | null = null

/** 拉取并缓存 /api/meta；失败时记录错误，不阻断页面使用。 */
export function ensureMeta(): Promise<void> {
  if (appState.meta) return Promise.resolve()
  if (!metaPromise) {
    metaPromise = api
      .getMeta()
      .then((res) => {
        const { ok: _ok, ...meta } = res
        appState.meta = meta
        appState.metaError = ''
      })
      .catch((err: unknown) => {
        appState.metaError = err instanceof Error ? err.message : '元信息加载失败'
      })
      .finally(() => {
        metaPromise = null
      })
  }
  return metaPromise ?? Promise.resolve()
}

let demoPromise: Promise<void> | null = null

/** 拉取并缓存 /api/demo-cases。 */
export function ensureDemoCases(): Promise<void> {
  if (appState.demoCases) return Promise.resolve()
  if (!demoPromise) {
    demoPromise = api
      .getDemoCases()
      .then((res) => {
        appState.demoCases = res.cases
        appState.demoNotice = res.notice
      })
      .catch(() => {
        appState.demoCases = []
      })
      .finally(() => {
        demoPromise = null
      })
  }
  return demoPromise ?? Promise.resolve()
}

/** 首页“体验演示案例”：写入草稿并跳转到提交页。 */
export function setDraft(draft: SubmitDraft): void {
  appState.draft = draft
}

export function takeDraft(): SubmitDraft | null {
  const d = appState.draft
  appState.draft = null
  return d
}
