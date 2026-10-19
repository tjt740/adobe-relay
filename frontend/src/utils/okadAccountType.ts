import type { Account } from '@/types'

type AccountIdentity = Pick<Account, 'platform' | 'type' | 'credentials' | 'extra'>
const text = (value: unknown) => typeof value === 'string' ? value : ''
const email = (value: unknown) => text(value).trim().toLowerCase()
const record = (value: unknown): Record<string, unknown> =>
  value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}

// Same identity guards as service.OkadAccountKind. An absent flag is unknown.
export function okadAccountType(account: AccountIdentity): '' | 'company' | 'vietnam' | 'non_company' | 'unknown' {
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
  if ((metadata.account_type != null && typeof metadata.account_type !== 'string') || (metadata.group_name != null && typeof metadata.group_name !== 'string')) return 'unknown'
  const kind = text(metadata.account_type)
  if (kind && (!['company','vietnam','non_company'].includes(kind) || (kind === 'company') !== metadata.is_company || (kind === 'vietnam' && !text(metadata.source_key).startsWith('external:')))) return 'unknown'
  const groupID = metadata.group_id
  const groupName = text(metadata.group_name)
  if (groupID != null ? (!Number.isSafeInteger(groupID) || Number(groupID) <= 0 || !groupName.trim() || [...groupName].length > 64 || !text(metadata.source_key).startsWith('external:')) : groupName !== '') return 'unknown'
  return kind === 'vietnam' ? 'vietnam' : metadata.is_company ? 'company' : 'non_company'
}

export function okadAccountGroup(account: AccountIdentity): string {
  const kind = okadAccountType(account)
  if (!kind || kind === 'unknown') return ''
  const metadata = record(account.extra?.okad_account)
  return metadata.group_id == null ? 'ungrouped' : `${text(metadata.issuer)}:${metadata.group_id}`
}
