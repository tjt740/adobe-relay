import { describe, expect, it } from 'vitest'
import { accountStatusFacts, accountStatusRecords, safeStatusMessage, statusErrorExplanation, statusRecordTime } from '../accountStatusEvidence'
import type { Account } from '@/types'

describe('actual account status evidence', () => {
  it.each([
    ['auth', 401, '凭据验证失败'], ['quota', 401, '额度不足'], ['entitlement', 401, '权限不足'],
    ['upstream', 408, '上游返回失败响应'], ['timeout', 408, '本地等待响应超时'], ['network', 0, '网络连接失败']
  ])('uses the saved reason for %s, rather than guessing from HTTP %s', (reason, code, expected) => {
    const records = accountStatusRecords({ extra: { adobe_health: { latest: { success: false, code, reason, source: 'business', stage: 'submit', checked_at: '2026-10-13T12:50:22Z' } } } } as Account)
    expect(JSON.stringify(records)).toContain(expected)
    expect(records[0]!.fields.find(x => x.label === '状态码')!.value).toBe(code ? String(code) : '未记录')
  })
  it('does not treat Cookie verification as successful image generation', () => {
    const records = accountStatusRecords({ extra: { okad: { verification: 'verified', verified_at: '2026-10-13T08:37:41Z' }, okad_connection_test: { state: 'failed' }, last_check_success: false, last_check_status_code: 408 } } as Account)
    expect(records.map(x => x.result)).toEqual(['检测失败', '检测失败', '凭据与额度验证通过'])
    expect(JSON.stringify(records)).not.toContain('请求成功')
  })
  it('keeps a pending failure visible after a different request succeeds', () => {
    const fail = { success: false, code: 401, reason: 'entitlement', scope: 'model-a', checked_at: '2026-10-13T09:00:00Z' }
    const latest = { success: true, code: 200, scope: 'model-b', checked_at: '2026-10-13T10:00:00Z' }
    const records = accountStatusRecords({ extra: { adobe_health: { latest, last_failure: fail, last_success: latest, pending: true } } } as Account)
    expect(records).toHaveLength(2)
    expect(records[1]!.title).toBe('尚未确认恢复的失败记录')
    expect(JSON.stringify(records[1])).toContain('model-a')
  })
  it('reports missing data honestly and keeps manual pause separate from an enabled account', () => {
    const account = { status: 'active', schedulable: false, extra: { okad_manual_pause: true }, rate_limit_reset_at: '2020-01-01T00:00:00Z' } as Account
    expect(accountStatusRecords(account)).toEqual([])
    expect(accountStatusFacts(account)).toContainEqual({ label: '账号状态', value: '已启用' })
    expect(accountStatusFacts(account)).toContainEqual({ label: '调度开关', value: '关闭' })
    expect(JSON.stringify(accountStatusFacts(account))).toContain('已过期')
    expect(statusRecordTime('broken')).toBe('时间记录无效')
    expect(statusRecordTime(null)).toBe('时间未记录')
  })
  it('redacts credentials and explains a saved login limit without claiming the current Cookie is invalid', () => {
    expect(safeStatusMessage('failure Cookie: private-cookie\nAuthorization: Bearer private-token\naccess_token=secret')).not.toMatch(/private-cookie|private-token|secret/)
    expect(statusErrorExplanation('Authentication failed (401): Okad automatic login limit reached (2/2)')).toContain('不能说明后来更新的 Cookie 是否有效')
    expect(statusErrorExplanation('Authentication failed (401): taste_exhausted')).toBe('系统记录：账号额度不足。')
    expect(statusErrorExplanation('account 401 failed to connect')).not.toContain('401 错误')
    expect(statusErrorExplanation('{"status_code":408}')).toContain('系统保存了 408 错误')
  })
})
