import { describe, expect, it, vi, afterEach } from 'vitest'
import {
  claimStatusKind,
  copyText,
  errorMessage,
  formatDateTime,
  hintLevelLabel,
  isOpenStatus,
  pct,
  progressLabel,
  progressTagKind,
  sessionStepIndex,
  transferScoreText,
} from './utils'

/**
 * 这些函数决定界面上的状态映射与文案，全部是纯逻辑（除 copyText 需要 DOM）。
 * 断言集中在“哪个状态映射到哪种展示”这类机器消费的值上，而不是复述整段文案。
 */

describe('formatDateTime', () => {
  it('空值与非法值不会抛错', () => {
    expect(formatDateTime(undefined)).toBe('—')
    expect(formatDateTime('')).toBe('—')
    // 无法解析时按原样回显，避免把用户的输入吞掉
    expect(formatDateTime('not-a-date')).toBe('not-a-date')
  })

  it('合法 ISO 时间格式化为本地时间文本', () => {
    const out = formatDateTime('2026-10-15T12:30:00Z')
    expect(out).toMatch(/2026/)
    expect(out).not.toBe('—')
  })
})

describe('pct', () => {
  it('比例转百分比并保留一位小数', () => {
    expect(pct(0.9)).toBe('90.0%')
    expect(pct(1 / 3, 2)).toBe('33.33%')
  })

  it('非有限值显示占位符而不是 NaN%', () => {
    expect(pct(Number.NaN)).toBe('—')
    expect(pct(Number.POSITIVE_INFINITY)).toBe('—')
  })
})

describe('transferScoreText', () => {
  it('-1 表示尚未评分', () => {
    expect(transferScoreText(-1, 6)).toBe('未评分')
  })

  it('已评分时显示 得分 / 满分', () => {
    expect(transferScoreText(4, 6)).toBe('4 / 6')
    expect(transferScoreText(0, 6)).toBe('0 / 6')
  })
})

describe('状态映射', () => {
  it('修正进度映射到文案与配色', () => {
    expect(progressTagKind('improved')).toBe('success')
    expect(progressTagKind('partial')).toBe('warning')
    expect(progressTagKind('none')).toBe('neutral')
    expect(progressLabel('improved')).toBe('有所修正')
    expect(progressLabel('unknown-state')).toBe('unknown-state')
  })

  it('主张的六种状态映射到配色，未涉及与不适用不算缺陷', () => {
    expect(claimStatusKind('mastered')).toBe('success')
    expect(claimStatusKind('error')).toBe('danger')
    expect(claimStatusKind('partial')).toBe('warning')
    expect(claimStatusKind('insufficient_evidence')).toBe('warning')
    expect(claimStatusKind('not_mentioned')).toBe('neutral')
    expect(claimStatusKind('not_applicable')).toBe('neutral')

    expect(isOpenStatus('error')).toBe(true)
    expect(isOpenStatus('partial')).toBe(true)
    expect(isOpenStatus('mastered')).toBe(false)
    expect(isOpenStatus('not_mentioned')).toBe(false)
    expect(isOpenStatus('not_applicable')).toBe(false)
  })

  it('会话状态映射到步骤序号', () => {
    expect(sessionStepIndex('diagnosed')).toBe(1)
    expect(sessionStepIndex('tutoring')).toBe(2)
    expect(sessionStepIndex('transfer')).toBe(3)
    expect(sessionStepIndex('completed')).toBe(4)
    expect(sessionStepIndex('unknown')).toBe(1)
  })

  it('提示等级映射到名称', () => {
    expect(hintLevelLabel(1)).toBe('一级提示')
    expect(hintLevelLabel(3)).toContain('局部示例')
    expect(hintLevelLabel(9)).toBe('9 级提示')
  })
})

describe('errorMessage', () => {
  it('优先使用错误自带的中文提示', () => {
    expect(errorMessage(new Error('请先填写学生解释'))).toBe('请先填写学生解释')
  })

  it('非错误对象给出兜底文案，不显示 [object Object]', () => {
    expect(errorMessage(undefined)).toBe('操作失败，请稍后重试。')
    expect(errorMessage({ code: 'X' })).toBe('操作失败，请稍后重试。')
  })
})

describe('copyText', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('优先走 clipboard API', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    await expect(copyText('内容')).resolves.toBe(true)
    expect(writeText).toHaveBeenCalledWith('内容')
  })

  it('clipboard 不可用时退回隐藏 textarea，成功返回 true', async () => {
    const select = vi.fn()
    const removeChild = vi.fn()
    vi.stubGlobal('navigator', {})
    vi.stubGlobal('document', {
      createElement: () => ({ style: {}, setAttribute: vi.fn(), select }),
      body: { appendChild: vi.fn(), removeChild },
      execCommand: () => true,
    })
    await expect(copyText('内容')).resolves.toBe(true)
    expect(select).toHaveBeenCalled()
    expect(removeChild).toHaveBeenCalled()
  })

  it('两条路径都失败时返回 false，供界面提示失败', async () => {
    vi.stubGlobal('navigator', {})
    vi.stubGlobal('document', {
      createElement: () => ({ style: {}, setAttribute: vi.fn(), select: () => { throw new Error('blocked') } }),
      body: { appendChild: vi.fn(), removeChild: vi.fn() },
      execCommand: () => false,
    })
    await expect(copyText('内容')).resolves.toBe(false)
  })
})
