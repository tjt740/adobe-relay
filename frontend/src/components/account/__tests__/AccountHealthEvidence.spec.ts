import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AccountHealthEvidence from '../AccountHealthEvidence.vue'
import type { Account } from '@/types'

describe('Account health evidence', () => {
  it('does not call a received Cookie healthy before its test succeeds', () => {
    const wrapper = mount(AccountHealthEvidence, { props: { account: { platform: 'adobe', extra: { okad_connection_test: { state: 'pending' } } } as Account } })
    expect(wrapper.text()).toContain('等待接口检测')
    expect(wrapper.text()).toContain('尚无完整成功记录')
    expect(wrapper.text()).not.toContain('请求成功')
  })
  it('shows local timeout advice and its source without claiming expired cookies', () => {
    const wrapper = mount(AccountHealthEvidence, { props: { account: { platform: 'adobe', extra: { adobe_health: { pending: true, latest: { success: false, code: 408, reason: 'timeout', source: 'okad', stage: 'poll', checked_at: '2026-10-13T09:00:00Z' } } } } as Account } })
    expect(wrapper.text()).toContain('本地等待超时')
    expect(wrapper.text()).toContain('不代表 Cookie 失效')
    expect(wrapper.text()).toContain('Cookie 推送后检测')
    expect(wrapper.text()).toContain('等待生成')
  })
  it('keeps other model failures visible after a successful observation', () => {
    const wrapper = mount(AccountHealthEvidence, { props: { account: { platform: 'adobe', extra: { adobe_health: { pending: true, latest: { success: true, checked_at: '2026-10-13T09:00:00Z' } } } } as Account } })
    expect(wrapper.text()).toContain('仍有其他失败待验证')
  })
})
