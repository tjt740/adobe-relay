import { describe, it, expect } from 'vitest'
import type { Account } from '@/types'
import { okadAccountType, okadAccountGroup } from '../okadAccountType'
const account = () => ({ platform: 'adobe', type: 'oauth', credentials: { email: 'COMPANY@outlook.com' }, extra: {
  adobeteam_external_email: 'company@outlook.com',
  okad_account: { issuer: 'site', source_key: 'external:1', email: 'company@outlook.com', is_company: true }
} }) as unknown as Account

describe('Okad company classification', () => {
  it('uses the source flag, never the email domain', () => {
    const a = account()
    expect(okadAccountType(a)).toBe('company')
    ;(a.extra!.okad_account as Record<string, unknown>).is_company = false
    expect(okadAccountType(a)).toBe('non_company')
    delete a.extra!.okad_account
    expect(okadAccountType(a)).toBe('unknown')
    delete a.extra!.adobeteam_external_email
    expect(okadAccountType(a)).toBe('')
  })
  it('rejects stale identity, malformed flags and other platforms', () => {
    const a = account()
    a.credentials.email = 'different@outlook.com'
    expect(okadAccountType(a)).toBe('unknown')
    const b = account()
    b.extra!.okad = { issuer: 'other', source_key: 'external:1' }
    expect(okadAccountType(b)).toBe('unknown')
    const c = account()
    ;(c.extra!.okad_account as Record<string, unknown>).is_company = 'true'
    expect(okadAccountType(c)).toBe('unknown')
    c.platform = 'openai'
    expect(okadAccountType(c)).toBe('')
  })
})

describe('Okad Vietnam and custom groups', () => {
  it('requires verified metadata and namespaces group IDs by issuer', () => {
    const a = account()
    Object.assign(a.extra!.okad_account!, {is_company: false, account_type: 'vietnam', group_id: 3, group_name: '客户 A'})
    expect(okadAccountType(a)).toBe('vietnam')
    expect(okadAccountGroup(a)).toBe('site:3')
    Object.assign(a.extra!.okad_account!, {group_id: null, group_name: ''})
    expect(okadAccountGroup(a)).toBe('ungrouped')
    a.extra!.okad = {issuer: 'foreign', source_key: 'external:1'}
    expect(okadAccountGroup(a)).toBe('')
  })
})
