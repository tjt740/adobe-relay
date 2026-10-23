import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ProxyHealthPanel from '../ProxyHealthPanel.vue'
import { getProxyHealth, type ProxyHealth } from '@/api/admin/clash'
import proxyHealth from '@/i18n/locales/zh/proxyHealth'
vi.mock('@/api/admin/clash', () => ({ getProxyHealth: vi.fn() }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({
 t: (key: string, values: Record<string, unknown> = {}) => {
   const messages: Record<string, unknown> = { proxyHealth, common: { refresh: '刷新', loading: '读取中' } }
   let value: unknown = messages
   for (const part of key.split('.')) value = (value as Record<string, unknown>)?.[part]
   return String(value ?? key).replace(/\{(\w+)\}/g, (_, name: string) => String(values[name] ?? ''))
 }
}) }))
enableAutoUnmount(afterEach)
const fixture = (): ProxyHealth => ({ checked_at: '2026-10-17T02:14:00+08:00', sources: [{ proxy_ids: [1290], blocked_proxy_ids: [1290], generation_blocked: 1, id: 'abcdef123456', index: 2, url_hint: 'https://provider.example/••••', status: 'exhausted', checked_at: '2026-10-17T02:14:00+08:00', last_success_at: '2026-10-17T02:14:00+08:00', traffic_known: true, remaining_bytes: 0, total_bytes: 1024 ** 3, traffic_observed_at: '2026-10-17T02:14:00+08:00', expires_at: null, nodes: 8, healthy: 0, failed: 8, unknown: 0, bound_accounts: 3 }], groups: [{ id: 7, name: '客户测试', total: 4, enabled: 0, available: 0, waiting_proxy: 0 }] })
async function panel() {
  const wrapper = mount(ProxyHealthPanel)
  await flushPromises(); await wrapper.get('button[aria-expanded]').trigger('click'); return wrapper
}
beforeEach(() => { vi.clearAllMocks(); vi.mocked(getProxyHealth).mockResolvedValue(fixture()) })
describe('Proxy subscription health', () => {
  it('keeps actionable warnings visible while details start collapsed', async () => {
    const w = mount(ProxyHealthPanel); await flushPromises()
    expect(w.get('button[aria-expanded]').attributes('aria-expanded')).toBe('false')
    expect(w.text()).toContain('订阅流量耗尽'); expect(w.text()).toContain('provider.example')
    expect(w.text()).toContain('剩余 0.0 MiB')
    expect(w.text()).toContain('1 个节点因 Adobe')
    expect(w.emitted('loaded')).toHaveLength(1)
  })
  it('locates the exhausted subscription without revealing its token', async () => {
    const w = await panel()
    expect(w.text()).toContain('订阅 2'); expect(w.text()).toContain('订阅流量耗尽')
    expect(w.text()).toContain('https://provider.example/••••'); expect(w.text()).toContain('剩余 0.0 MiB')
    expect(w.text()).toContain('已排除本订阅的新分配')
    expect(w.text()).toContain('保留人工状态')
    await w.get('button').trigger('click'); expect(w.emitted('manage')).toHaveLength(1)
  })
  it('never invents remaining traffic when the provider does not expose it', async () => {
    const data = fixture(); data.sources[0].traffic_known = false; data.sources[0].remaining_bytes = null; data.sources[0].status = 'healthy'
    vi.mocked(getProxyHealth).mockResolvedValue(data)
    const w = await panel(); expect(w.text()).toContain('剩余流量未知'); expect(w.text()).toContain('连接可达'); expect(w.text()).not.toContain('已排除本订阅的新分配')
  })
  it('keeps previous evidence visible with a stale warning after a refresh failure', async () => {
    const w = await panel(); vi.mocked(getProxyHealth).mockRejectedValue(new Error('offline'))
    await w.findAll('button')[1].trigger('click'); await flushPromises()
    expect(w.text()).toContain('已有内容可能过期'); expect(w.text()).toContain('订阅流量耗尽')
  })
})
