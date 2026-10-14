import type { Account } from '@/types'

type AccountIdentity = Pick<Account, 'platform' | 'type' | 'credentials' | 'extra'>
const text = (value: unknown) => typeof value === 'string' ? value : ''
const email = (value: unknown) => text(value).trim().toLowerCase()
const record = (value: unknown): Record<string, unknown> =>
  value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}

// Same identity guards as service.OkadAccountKind. An absent flag is unknown.
export function okadAccountType(account: AccountIdentity): '' | 'company' | 'non_company' | 'unknown' {
  if (account.platform !== 'adobe' || account.type !== 'oauth') return ''
  const owner = record(account.extra?.okad)
  const marker = email(account.extra?.adobeteam_external_email)
  const issuer = text(owner.issuer)
  if (!issuer && !marker) return ''
  const metadata = record(account.extra?.okad_account)
  const credentialEmail = email(account.credentials?.email)
  const currentEmail = credentialEmail || marker
  if (!text(metadata.issuer) || text(metadata.issuer).length > 100 ||
      !/^(external|pool):[1-9][0-9]*$/.test(text(metadata.source_key)) || text(metadata.source_key).length > 100 ||
      typeof metadata.is_company !== 'boolean' || !text(metadata.email) || text(metadata.email).length > 320 ||
      text(metadata.email).toLowerCase() !== currentEmail || (credentialEmail && marker && credentialEmail !== marker) ||
      (metadata.is_company && !text(metadata.source_key).startsWith('external:')) ||
      (issuer && (issuer !== metadata.issuer || owner.source_key !== metadata.source_key))) return 'unknown'
  return metadata.is_company ? 'company' : 'non_company'
}
