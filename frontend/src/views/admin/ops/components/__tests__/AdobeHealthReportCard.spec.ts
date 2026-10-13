import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AdobeHealthReportCard from '../AdobeHealthReportCard.vue'
import type { AdobeHealthReport } from '@/utils/adobeHealth'

vi.mock('@/composables/useAccountLiveUpdates', () => ({ useAccountLiveUpdates: () => ({ connected: false }) }))
const snapshot: AdobeHealthReport = { generated_at: '2026-10-13T09:00:00Z', window_start: '2026-10-13T08:50:00Z', total: 108, pending: 39, recent_pending: 35, recovered: 4, unknown: 10, historical: 39, ratio: 36.11, threshold: 20, minimum: 5, accounts: [
  { id: 1, name: '<script>account</script>', scope: 'model-a', code: 408, reason: 'timeout', source: 'okad', stage: 'poll', pending: true, failed_at: '2026-10-13T08:55:00Z' },
  { id: 2, name: 'recovered', scope: 'model-a', code: 401, reason: 'auth', source: 'test', stage: '', pending: false, failed_at: '2026-10-13T08:51:00Z', success_at: '2026-10-13T08:56:00Z' }
] }
describe('Adobe readable report', () => {
  it('renders every window account with separate error counts and preserves evidence filters', async () => {
    const window_accounts = Array.from({ length: 39 }, (_, index) => ({ account_id: index + 1, name: `account-${index + 1}`, status: 'active', count_401: index === 0 ? 1 : 0, count_408: 2, auth_401: index === 0 ? 1 : 0, quota_401: 0, entitled_401: 0, unknown_401: 0, business_errors: index === 0 ? 1 : 0, test_errors: 2, test_timeouts: 0, first_seen: snapshot.window_start, last_seen: snapshot.generated_at }))
    const wrapper = mount(AdobeHealthReportCard, { props: { snapshot: { ...snapshot, window_accounts, quiet_notifications: true, recovery_threshold: 10, confirmation_minutes: 4, notification_cooldown_minutes: 60 } } })
    expect(wrapper.findAll('.window-table tbody tr')).toHaveLength(39)
    expect(wrapper.find('.window-table tbody').text()).toContain('account-39')
    expect(wrapper.find('.window-table tbody .red').text()).toBe('1 次')
    expect(wrapper.findAll('.window-table tbody .yellow')).toHaveLength(39)
    expect(wrapper.text()).toContain('持续确认至少 4 分钟')
    expect(wrapper.text()).toContain('新邮件至少间隔 60 分钟')
    expect(wrapper.text()).toContain('严格低于 10%')
    await wrapper.find('input').setValue('account-39')
    expect(wrapper.findAll('.window-table tbody tr')).toHaveLength(1)
    expect(wrapper.find('.window-table tbody').text()).toContain('account-39')
  })
  it('separates historical errors, pending recovery and successful evidence', async () => {
    const wrapper = mount(AdobeHealthReportCard, { props: { snapshot } })
    expect(wrapper.text()).toContain('36.11%')
    expect(wrapper.text()).toContain('阈值 20%')
    expect(wrapper.text()).toContain('历史报错与当前待验证数量不相加')
    expect(wrapper.text()).toContain('本地超时（按 408 统计）')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.find('.evidence-table tbody').text()).not.toContain('recovered')
    await wrapper.findAll('button').find(b => b.text() === '已有恢复证据')!.trigger('click')
    expect(wrapper.find('.evidence-table tbody').text()).toContain('recovered')
    expect(wrapper.find('.evidence-table tbody').text()).not.toContain('#1')
    expect(wrapper.text()).not.toContain('发送报表到告警邮箱')
    expect(wrapper.text()).not.toMatch(/历史快照|你提供|？/)
    expect(wrapper.find('.metric.red').text()).toContain('401')
    expect(wrapper.find('.metric.yellow').text()).toContain('408')
  })
})
