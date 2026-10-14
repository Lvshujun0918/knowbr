/** 展示辅助：时间、百分比、得分与状态文案。 */

export function formatDateTime(iso: string | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

/** 比例 → 百分比文本（0.9 → "90.0%"）。 */
export function pct(x: number, digits = 1): string {
  if (!Number.isFinite(x)) return '—'
  return `${(x * 100).toFixed(digits)}%`
}

/** 记录列表中的迁移得分：-1 表示未评分。 */
export function transferScoreText(score: number, max: number): string {
  if (score < 0) return '未评分'
  return `${score} / ${max}`
}

export function progressLabel(progress: string): string {
  switch (progress) {
    case 'improved':
      return '有所修正'
    case 'partial':
      return '部分推进'
    case 'none':
      return '暂无明显进展'
    default:
      return progress
  }
}

export function progressTagKind(progress: string): 'success' | 'warning' | 'neutral' {
  switch (progress) {
    case 'improved':
      return 'success'
    case 'partial':
      return 'warning'
    default:
      return 'neutral'
  }
}

/** 知识点状态 → 标签配色，与后端六种状态一一对应。 */
export function claimStatusKind(status: string): 'success' | 'warning' | 'danger' | 'neutral' {
  switch (status) {
    case 'mastered':
      return 'success'
    case 'partial':
    case 'insufficient_evidence':
      return 'warning'
    case 'error':
      return 'danger'
    default:
      return 'neutral'
  }
}

/** 一知识点是否仍需学生处理：未涉及与不适用都不算缺陷。 */
export function isOpenStatus(status: string): boolean {
  return status === 'error' || status === 'partial' || status === 'insufficient_evidence'
}

export function hintLevelLabel(level: number): string {
  switch (level) {
    case 1:
      return '一级提示'
    case 2:
      return '二级提示'
    case 3:
      return '三级提示 · 局部示例'
    default:
      return `${level} 级提示`
  }
}

export function sessionStepIndex(status: string): number {
  switch (status) {
    case 'diagnosed':
      return 1
    case 'tutoring':
      return 2
    case 'transfer':
      return 3
    case 'completed':
      return 4
    default:
      return 1
  }
}

/** 复制文本，优先 navigator.clipboard，失败时退回隐藏 textarea。 */
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    try {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.setAttribute('readonly', '')
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(ta)
      return ok
    } catch {
      return false
    }
  }
}

/** 错误对象 → 可展示文案。 */
export function errorMessage(err: unknown): string {
  if (err instanceof Error && err.message) return err.message
  return '操作失败，请稍后重试。'
}
