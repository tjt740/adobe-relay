import type { Account } from '@/types'

type StatusAccount = Pick<Account, 'platform' | 'error_message' | 'extra' | 'temp_unschedulable_until' | 'temp_unschedulable_reason'>
const metadataKeys = ['last_status_code', 'status_code', 'upstream_status_code', 'adobe_last_status_code']
const currentError = (account: StatusAccount) => account.error_message || (
  account.temp_unschedulable_until && new Date(account.temp_unschedulable_until).getTime() > Date.now()
    ? account.temp_unschedulable_reason || '' : ''
)
const parseStatusCode = (value: unknown): number | null => {
  if (typeof value === 'number' && Number.isInteger(value)) return value
  if (typeof value !== 'string') return null
  return /\b408\b/.test(value) ? 408 : null
}
export const isAdobeAccountCookieError = (account: StatusAccount, message: string) =>
  account.platform === 'adobe' && /adobe cookie is no longer valid|\binvalid_credentials\b/i.test(message)

export function getAccountAuthenticationError(account: StatusAccount, fallback = 'HTTP 401'): string {
  const message = currentError(account)
  if (message) {
    if (isAdobeAccountCookieError(account, message)) return message
    try {
      const reason = JSON.parse(message)
      if (reason && typeof reason === 'object' && Number(reason.status_code) === 401) return message
    } catch { /* Plain-text errors are common. */ }
    return /(?:^\s*|\(|\b(?:http|oauth|status(?:\s+code)?|returned|rejected|upstream|failed)\s*[:=]?\s*\(?)401\b/i.test(message) ? message : ''
  }
  return metadataKeys.some(key => Number(account.extra?.[key]) === 401) ? fallback : ''
}

export function getAccountStatusCode(account: StatusAccount): number | null {
  if (parseStatusCode(currentError(account)) === 408) return 408
  if (typeof account.extra?.last_check_success === 'boolean') return Number(account.extra.last_check_status_code) || null
  for (const key of ['last_check_status_code', ...metadataKeys]) {
    if (parseStatusCode(account.extra?.[key]) === 408) return 408
  }
  return null
}

export function matchesAccountRuntimeStatus(account: StatusAccount, status: string): boolean {
  const auth = Boolean(getAccountAuthenticationError(account))
  const timeout = !auth && getAccountStatusCode(account) === 408
  const success = account.extra?.last_check_success
  const checked = typeof success === 'boolean'
  switch (status) {
    case 'active': return !auth && !timeout
    case 'authentication_failed': return auth || (success === false && Number(account.extra?.last_check_status_code) === 401)
    case 'request_timeout': return timeout
    case 'test_passed': return success === true && !auth && !timeout
    case 'test_failed': return success === false
    case 'untested': return !checked
    default: return true
  }
}
export const runtimeAccountStatuses = ['authentication_failed', 'request_timeout', 'test_passed', 'test_failed', 'untested']
