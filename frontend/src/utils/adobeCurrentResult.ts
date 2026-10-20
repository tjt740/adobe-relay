import type { Account } from '@/types'

type RecordValue = Record<string, unknown>
const object = (value: unknown): RecordValue => value && typeof value === 'object' ? value as RecordValue : {}
const time = (value: unknown) => typeof value === 'string' ? Date.parse(value) || 0 : 0

/** The latest completed result is shown once; older model failures stay in details. */
export function adobeCurrentResult(account: Account) {
  if (account.platform !== 'adobe') return null
  const extra = account.extra || {}
  const check = object(extra.okad_connection_test)
  if (['pending', 'running', 'proxy_pending'].includes(String(check.state))) return null
  const latest = object(object(extra.adobe_health).latest)
  const checkedAt = time(extra.last_check_at)
  const observedAt = time(latest.checked_at)
  // A later request failure supersedes a successful test.
  if (typeof latest.success === 'boolean' && observedAt > checkedAt) {
    return { success: latest.success, code: Number(latest.code) || 0, source: ['test', 'okad'].includes(String(latest.source)) ? 'test' : 'business', at: String(latest.checked_at || ''), model: String(latest.scope || '') }
  }
  if (extra.last_check_success !== true) return null
  const sameCheck = latest.success === true && ['test', 'okad'].includes(String(latest.source)) && Math.abs(observedAt - checkedAt) <= 1000
  return { success: true, code: 200, source: 'test', at: String(extra.last_check_at || ''), model: sameCheck ? String(latest.scope || '') : '' }
}
