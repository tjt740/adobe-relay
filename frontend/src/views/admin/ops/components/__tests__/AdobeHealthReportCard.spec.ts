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
  it('separates historical errors, pending recovery and successful evidence', async () => {
    const wrapper = mount(AdobeHealthReportCard, { props: { snapshot } })
    expect(wrapper.text()).toContain('36.1%')
    expect(wrapper.text()).toContain('阈值 20%')
    expect(wrapper.text()).toContain('历史报错与当前待验证数量不相加')
    expect(wrapper.text()).toContain('本地超时（按 408 统计）')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.find('tbody').text()).not.toContain('recovered')
    await wrapper.findAll('button').find(b => b.text() === '已有恢复证据')!.trigger('click')
    expect(wrapper.find('tbody').text()).toContain('recovered')
    expect(wrapper.find('tbody').text()).not.toContain('#1')
    expect(wrapper.text()).not.toContain('发送报表到告警邮箱')
  })
})
