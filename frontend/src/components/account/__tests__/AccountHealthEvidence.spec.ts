import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AccountHealthEvidence from '../AccountHealthEvidence.vue'
import type { Account } from '@/types'

describe('Account health evidence', () => {
  it('does not call a received Cookie healthy before its test succeeds', () => {
    const wrapper = mount(AccountHealthEvidence, { props: { account: { platform: 'adobe', extra: { okad_connection_test: { state: 'pending' } } } as Account } })
    expect(wrapper.text()).toContain('已排队，尚未开始')
    expect(wrapper.text()).toContain('时间未记录')
    expect(wrapper.text()).not.toContain('请求成功')
  })
  it('shows the recorded local timeout and its source without inventing a Cookie failure', () => {
    const wrapper = mount(AccountHealthEvidence, { props: { account: { platform: 'adobe', extra: { adobe_health: { pending: true, latest: { success: false, code: 408, reason: 'timeout', source: 'okad', stage: 'poll', checked_at: '2026-10-13T09:00:00Z' } } } } as Account } })
    expect(wrapper.text()).toContain('本地等待响应超时')
    expect(wrapper.text()).not.toContain('Cookie 失效')
    expect(wrapper.text()).toContain('Cookie 推送后的检测')
    expect(wrapper.text()).toContain('等待生成')
  })
  it('keeps other model failures visible after a successful observation', () => {
    const wrapper = mount(AccountHealthEvidence, { props: { account: { platform: 'adobe', extra: { adobe_health: { pending: true, latest: { success: true, checked_at: '2026-10-13T09:00:00Z' } } } } as Account } })
    expect(wrapper.text()).toContain('失败后还没有对应的成功记录')
    expect(wrapper.text()).toContain('请求成功')
  })
  it('shows old detection evidence instead of incorrectly claiming no success history', () => {
    const wrapper = mount(AccountHealthEvidence, { props: { account: { platform: 'adobe', extra: { last_check_success: false, last_check_status_code: 408, last_check_at: '2026-10-13T08:38:54Z' } } as Account } })
    expect(wrapper.text()).toContain('最近账号检测')
    expect(wrapper.text()).toContain('检测失败')
    expect(wrapper.text()).toContain('408')
    expect(wrapper.text()).toContain('2026/10/13')
    expect(wrapper.text()).toContain('没有完整的请求过程记录')
    expect(wrapper.text()).not.toContain('尚无完整成功记录')
    expect(wrapper.text()).not.toContain('真实业务请求')
  })
})
