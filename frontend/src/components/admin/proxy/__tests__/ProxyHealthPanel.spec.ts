import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ProxyHealthPanel from '../ProxyHealthPanel.vue'
import { getProxyHealth, type ProxyHealth } from '@/api/admin/clash'
import proxyHealth from '@/i18n/locales/zh/proxyHealth'
import { getProxyRelatedAccounts } from '@/api/admin/proxyRelatedAccounts'
vi.mock('@/api/admin/clash', () => ({ getProxyHealth: vi.fn() }))
vi.mock('@/api/admin/proxyRelatedAccounts', () => ({ getProxyRelatedAccounts: vi.fn() }))
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
  it('opens account details even when the current association count is zero', async () => {
    const data = fixture()
    data.sources[0].blocked_nodes = [{ proxy_id: 1290, name: '节点', server: '', server_port: 443, enabled: true, bound_accounts: 0, manual: true, excluded_at: data.checked_at, failed_at: null, retry_after: null }]
    vi.mocked(getProxyHealth).mockResolvedValue(data)
    vi.mocked(getProxyRelatedAccounts).mockResolvedValue({ proxy: { id: 1290, name: '节点' }, checked_at: data.checked_at, history_since: data.checked_at, current_count: 0, departed_count: 0, total: 0, page: 1, page_size: 20, items: [] })
    const w = mount(ProxyHealthPanel, { global: { stubs: { teleport: true } } }); await flushPromises()
    await w.get('[data-testid="blocked-details"]').trigger('click')
    expect(w.get('[data-testid="related-accounts"]').text()).toContain('关联账号数')
    await w.get('[data-testid="related-accounts"]').trigger('click'); await flushPromises()
    expect(w.findAll('[role="dialog"]')).toHaveLength(2)
    expect(getProxyRelatedAccounts).toHaveBeenCalledWith(1290, expect.any(Object), expect.any(AbortSignal))
    expect(w.text()).toContain('关联账号明细')
  })
  it.each([
    [0, 100, '0', 'bg-red-500'],
    [7, 100, '7', 'bg-red-500'],
    [24, 100, '24', 'bg-amber-500'],
    [82, 100, '82', 'bg-emerald-500'],
    [120, 100, '100', 'bg-emerald-500'],
  ])('renders remaining capacity %s / %s with a bounded percentage', async (remaining, total, percent, color) => {
    const data = fixture()
    Object.assign(data.sources[0], { remaining_bytes: remaining, total_bytes: total })
    vi.mocked(getProxyHealth).mockResolvedValue(data)
    const w = mount(ProxyHealthPanel); await flushPromises()
    const bar = w.get('[data-testid="source-card"] [role="progressbar"]')
    expect(bar.attributes('aria-valuenow')).toBe(percent)
    expect(bar.get('div').classes()).toContain(color)
    expect(bar.get('div').attributes('style')).toContain(`width: ${percent}%`)
  })
  it.each([null, 0])('does not invent a percentage for missing or zero quota %s', async total => {
    const data = fixture(); data.sources[0].total_bytes = total
    vi.mocked(getProxyHealth).mockResolvedValue(data)
    const w = mount(ProxyHealthPanel); await flushPromises()
    expect(w.find('[data-testid="source-card"] [role="progressbar"]').exists()).toBe(false)
    expect(w.text()).toContain('剩余流量未知')
    expect(w.html()).not.toContain('NaN')
  })
  it('shows all groups with candidate totals and can filter to limited groups', async () => {
    const data = fixture()
    data.groups.push({ id: 8, name: '可用分组', total: 10, enabled: 8, available: 6, waiting_proxy: 2 })
    vi.mocked(getProxyHealth).mockResolvedValue(data)
    const w = mount(ProxyHealthPanel); await flushPromises()
    expect(w.findAll('[data-testid="group-capacity"]')).toHaveLength(2)
    expect(w.findAll('[data-testid="group-capacity"] [role="progressbar"]')[1].attributes('aria-valuenow')).toBe('60')
    await w.get('button[aria-pressed]').trigger('click')
    expect(w.findAll('[data-testid="group-capacity"]')).toHaveLength(1)
    expect(w.text()).toContain('保留人工状态')
  })
  it('opens 61 real detail rows with search, reasons and pagination without another request', async () => {
    const data = fixture()
    data.sources[0].generation_blocked = 61
    data.sources[0].blocked_nodes = Array.from({ length: 61 }, (_, i) => ({
      proxy_id: i + 1, name: `测试节点-${i + 1}`, server: `192.0.2.${i + 1}`, server_port: 443,
      enabled: true, bound_accounts: 2, manual: i === 0, excluded_at: i === 0 ? data.checked_at : null,
      failed_at: i === 0 ? null : data.checked_at, retry_after: i === 0 ? null : '2026-10-17T02:19:00+08:00',
    }))
    vi.mocked(getProxyHealth).mockResolvedValue(data)
    const w = mount(ProxyHealthPanel, { global: { stubs: { teleport: true } } }); await flushPromises()
    await w.get('[data-testid="blocked-details"]').trigger('click')
    expect(w.get('[role="dialog"]').text()).toContain('61 个节点暂停')
    expect(w.findAll('[data-testid="blocked-node"]')).toHaveLength(20)
    expect(w.get('[role="dialog"]').text()).toContain('人工隔离')
    const next = w.findAll('[role="dialog"] button').find(b => b.text() === '下一页')!
    await next.trigger('click')
    expect(w.get('[data-testid="blocked-node"]').text()).toContain('测试节点-21')
    await w.get('input[type="search"]').setValue('192.0.2.61')
    expect(w.findAll('[data-testid="blocked-node"]')).toHaveLength(1)
    expect(w.get('[data-testid="blocked-node"]').text()).toContain('冷却截止')
    await w.get('input[type="search"]').setValue('')
    await w.get('select').setValue('manual')
    expect(w.findAll('[data-testid="blocked-node"]')).toHaveLength(1)
    expect(w.get('[data-testid="blocked-node"]').text()).toContain('不会自动解除人工隔离')
    expect(getProxyHealth).toHaveBeenCalledTimes(1)
  })
  it('refreshes open details and shows stale evidence when refreshing fails', async () => {
    const data = fixture()
    vi.mocked(getProxyHealth).mockResolvedValue(data)
    const w = mount(ProxyHealthPanel, { global: { stubs: { teleport: true } } }); await flushPromises()
    await w.get('[data-testid="blocked-details"]').trigger('click')
    expect(w.get('[role="dialog"]').text()).toContain('#1290')
    expect(w.get('[role="dialog"]').text()).toContain('暂未取得具体原因')
    vi.mocked(getProxyHealth).mockRejectedValueOnce(new Error('offline'))
    await w.findAll('button')[1].trigger('click'); await flushPromises()
    expect(w.get('[role="dialog"] [role="status"]').text()).toContain('已有内容可能过期')
    const recovered = fixture()
    Object.assign(recovered.sources[0], { generation_blocked: 0, blocked_proxy_ids: [], blocked_nodes: [] })
    vi.mocked(getProxyHealth).mockResolvedValueOnce(recovered)
    await w.findAll('button')[1].trigger('click'); await flushPromises()
    expect(w.get('[role="dialog"]').text()).toContain('当前没有暂停生成的节点')
    expect(w.find('[data-testid="blocked-details"]').exists()).toBe(false)
  })
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
