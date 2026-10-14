import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Account } from '@/types'
import AccountCompanyBadge from '../AccountCompanyBadge.vue'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key.endsWith('okadCompanyTag') ? '公司' : key }) }))
describe('company name tag', () => {
  it('shows a tag only after explicit company metadata and updates without changing status', async () => {
    const account = { platform: 'adobe', type: 'oauth', credentials: { email: 'company@outlook.com' }, extra: {
      adobeteam_external_email: 'company@outlook.com',
      okad_account: { issuer: 'site', source_key: 'external:1', email: 'company@outlook.com', is_company: true }
    } } as unknown as Account
    const wrapper = mount(AccountCompanyBadge, { props: { account } })
    expect(wrapper.text()).toBe('公司')
    expect(wrapper.get('[data-test="okad-company-tag"]').classes()).toContain('shrink-0')
    await wrapper.setProps({ account: { ...account, extra: { adobeteam_external_email: 'company@outlook.com' } } })
    expect(wrapper.find('[data-test="okad-company-tag"]').exists()).toBe(false)
  })
})
