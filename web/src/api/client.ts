import type {
  AgentDecisionResponse,
  AgentStepResponse,
  ApiErrorBody,
  DeleteResponse,
  DemoCasesResponse,
  HintResponse,
  MetaResponse,
  MetricsComputeRequest,
  MetricsComputeResponse,
  RevisionResponse,
  SessionListResponse,
  SessionResponse,
  Submission,
  SummaryResponse,
  TeacherOverviewResponse,
  TransferCompleteResponse,
  TransferQuestionResponse,
  TransferResponse,
} from '../types'

/** 统一 API 错误：message 始终为可直接展示的中文提示。 */
export class ApiError extends Error {
  readonly code: string
  readonly field?: string
  readonly detail?: string
  readonly status: number

  constructor(code: string, message: string, status: number, field?: string, detail?: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.field = field
    this.detail = detail
  }
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
      ...init,
    })
  } catch {
    throw new ApiError(
      'NETWORK_ERROR',
      '无法连接服务端。请确认后端已启动（默认 http://127.0.0.1:8080），你已填写的内容不会丢失。',
      0,
    )
  }

  let data: unknown
  try {
    data = await res.json()
  } catch {
    throw new ApiError('BAD_RESPONSE', `服务端返回了无法解析的内容（HTTP ${res.status}）。`, res.status)
  }

  const body = isRecord(data) ? data : {}
  if (!res.ok || body.ok === false) {
    const err = (body.error ?? {}) as Partial<ApiErrorBody>
    throw new ApiError(
      typeof err.code === 'string' ? err.code : `HTTP_${res.status}`,
      typeof err.message === 'string' && err.message !== '' ? err.message : '请求失败，请稍后重试。',
      res.status,
      typeof err.field === 'string' ? err.field : undefined,
      typeof err.detail === 'string' ? err.detail : undefined,
    )
  }
  return data as T
}

export const api = {
  getMeta: () => request<MetaResponse>('/api/meta'),

  getDemoCases: () => request<DemoCasesResponse>('/api/demo-cases'),

  computeMetrics: (body: MetricsComputeRequest) =>
    request<MetricsComputeResponse>('/api/metrics/compute', { method: 'POST', body: JSON.stringify(body) }),

  createSession: (submission: Submission) =>
    request<SessionResponse>('/api/sessions', { method: 'POST', body: JSON.stringify(submission) }),

  listSessions: () => request<SessionListResponse>('/api/sessions'),

  getSession: (id: string) => request<SessionResponse>(`/api/sessions/${encodeURIComponent(id)}`),

  deleteSession: (id: string) =>
    request<DeleteResponse>(`/api/sessions/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  clearSessions: () => request<DeleteResponse>('/api/sessions', { method: 'DELETE' }),

  addHint: (id: string, level: number) =>
    request<HintResponse>(`/api/sessions/${encodeURIComponent(id)}/hints`, {
      method: 'POST',
      body: JSON.stringify({ level, requestedByStudent: true }),
    }),

  addRevision: (id: string, content: string) =>
    request<RevisionResponse>(`/api/sessions/${encodeURIComponent(id)}/revisions`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    }),

  // 智能体：先看决策（无副作用），再执行一跳（可能自动给出提示或标记教师介入）。
  getAgent: (id: string) =>
    request<AgentDecisionResponse>(`/api/sessions/${encodeURIComponent(id)}/agent`),

  agentStep: (id: string) =>
    request<AgentStepResponse>(`/api/sessions/${encodeURIComponent(id)}/agent/step`, {
      method: 'POST',
      body: '{}',
    }),

  transferQuestion: (id: string) =>
    request<TransferQuestionResponse>(`/api/sessions/${encodeURIComponent(id)}/transfer/question`, {
      method: 'POST',
      body: '{}',
    }),

  transferAnswer: (id: string, answer: string) =>
    request<TransferResponse>(`/api/sessions/${encodeURIComponent(id)}/transfer/answer`, {
      method: 'POST',
      body: JSON.stringify({ answer }),
    }),

  transferReference: (id: string) =>
    request<TransferResponse>(`/api/sessions/${encodeURIComponent(id)}/transfer/reference`, {
      method: 'POST',
      body: '{}',
    }),

  transferComplete: (id: string) =>
    request<TransferCompleteResponse>(`/api/sessions/${encodeURIComponent(id)}/transfer/complete`, {
      method: 'POST',
      body: '{}',
    }),

  getSummary: (id: string) => request<SummaryResponse>(`/api/sessions/${encodeURIComponent(id)}/summary`),

  getTeacherOverview: () => request<TeacherOverviewResponse>('/api/teacher/overview'),
}

/** 导出下载地址（浏览器直接触发下载）。 */
export function exportUrl(id: string, format: 'md' | 'txt'): string {
  return `/api/sessions/${encodeURIComponent(id)}/export?format=${format}&download=1`
}
