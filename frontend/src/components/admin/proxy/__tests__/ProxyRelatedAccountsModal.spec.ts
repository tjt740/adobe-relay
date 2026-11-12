import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ProxyRelatedAccountsModal from '../ProxyRelatedAccountsModal.vue'
import { getProxyRelatedAccounts, type ProxyRelatedAccountsView } from '@/api/admin/proxyRelatedAccounts'
import proxyHealth from '@/i18n/locales/zh/proxyHealth'

vi.mock('@/api/admin/proxyRelatedAccounts', () => ({ getProxyRelatedAccounts: vi.fn() }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({
  t: (key: string, values: Record<string, unknown> = {}) => {
    let value: unknown = { proxyHealth, common: { refresh: '刷新', loading: '读取中' } }
    for (const part of key.split('.')) value = (value as Record<string, unknown>)?.[part]
    return String(value ?? key).replace(/\{(\w+)\}/g, (_, name: string) => String(values[name] ?? ''))
  },
}) }))
const at = '2026-11-12T04:35:00+08:00'
const fixture = (): ProxyRelatedAccountsView => ({
  proxy: { id: 3, name: '香港 04' }, checked_at: at, history_since: '2026-11-11T04:35:00+08:00',
  current_count: 2, departed_count: 2, total: 4, page: 1, page_size: 20,
  items: [
    { id: 1, name: '自动账号', platform: 'adobe', using_this_node: true, mode: 'automatic', account_state: 'eligible', route_state: 'generation_paused', impact: 'affected', current_proxy: { id: 3, name: '香港 04' }, last_departure: null },
    { id: 2, name: '固定账号', platform: 'adobe', using_this_node: true, mode: 'manual', account_state: 'error', route_state: 'generation_paused', impact: 'affected', current_proxy: { id: 3, name: '香港 04' }, last_departure: { at, destination: { id: 8, name: '历史节点' } } },
    { id: 4, name: '多次切换账号', platform: 'adobe', using_this_node: false, mode: 'automatic', account_state: 'eligible', route_state: 'unavailable', impact: 'left_node', current_proxy: { id: 9, name: '当前新节点' }, last_departure: { at, destination: { id: 8, name: '历史节点' } } },
    { id: 5, name: '等待账号', platform: 'adobe', using_this_node: false, mode: 'automatic', account_state: 'waiting_proxy', route_state: 'waiting_proxy', impact: 'left_node', current_proxy: null, last_departure: { at, destination: null } },
  ],
})
enableAutoUnmount(afterEach)
beforeEach(() => { vi.useFakeTimers(); vi.clearAllMocks(); vi.mocked(getProxyRelatedAccounts).mockResolvedValue(fixture()) })
afterEach(() => { vi.useRealTimers() })
async function modal() {
  const wrapper = mount(ProxyRelatedAccountsModal, { props: { node: { proxy_id: 3, name: '香港 04' } }, global: { stubs: { teleport: true } } })
  await flushPromises()
  return wrapper
}
describe('Proxy associated account details', () => {
  it('separates current association, mode, route restrictions and account state', async () => {
    const w = await modal()
    expect(w.text()).toContain('当前关联 2 个')
    const rows = w.findAll('[data-testid="related-account"]')
    expect(rows[0].text()).toContain('自动分配')
    expect(rows[0].text()).toContain('仍受此节点限制')
    expect(rows[1].text()).toContain('手动固定 IP')
    expect(rows[1].text()).toContain('账号异常')
    expect(rows[1].text()).toContain('曾从此节点切出，当前已返回')
    expect(rows[2].text()).toContain('当前新节点 #9')
    expect(rows[2].text()).toContain('历史节点 #8')
    expect(rows[2].text()).toContain('已离开此节点')
    expect(rows[2].text()).toContain('节点停用、到期或连接故障')
    expect(rows[3].text()).toContain('尚未分配代理')
    expect(w.text()).not.toContain('proxyHealth.related.')
    expect(getProxyRelatedAccounts).toHaveBeenCalledWith(3, { page: 1, page_size: 20, relation: 'all', search: '' }, expect.any(AbortSignal))
  })
  it('still shows departed accounts when there are zero current associations', async () => {
    const data = fixture(); data.current_count = 0; data.total = 2; data.items = data.items.slice(2)
    vi.mocked(getProxyRelatedAccounts).mockResolvedValue(data)
    const w = await modal()
    expect(w.text()).toContain('当前关联 0 个')
    expect(w.findAll('[data-testid="related-account"]')).toHaveLength(2)
  })
  it('debounces search and filters and resets to the first page', async () => {
    const w = await modal()
    await w.get('input').setValue('固定')
    await w.get('select').setValue('current')
    expect(getProxyRelatedAccounts).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(250); await flushPromises()
    expect(getProxyRelatedAccounts).toHaveBeenLastCalledWith(3, { page: 1, page_size: 20, relation: 'current', search: '固定' }, expect.any(AbortSignal))
  })
  it('fetches the requested page without showing stale rows under the new page number', async () => {
    const first = fixture(); first.total = 21
    vi.mocked(getProxyRelatedAccounts).mockResolvedValueOnce(first)
    const w = await modal()
    const second = fixture(); second.total = 21; second.page = 2; second.items = [second.items[3]]
    vi.mocked(getProxyRelatedAccounts).mockResolvedValueOnce(second)
    await w.findAll('button').find(b => b.text() === '下一页')!.trigger('click')
    await flushPromises()
    expect(getProxyRelatedAccounts).toHaveBeenLastCalledWith(3, expect.objectContaining({ page: 2 }), expect.any(AbortSignal))
    expect(w.findAll('[data-testid="related-account"]')).toHaveLength(1)
    expect(w.text()).toContain('第 2 / 2 页')
  })
  it('keeps previous evidence with a warning on refresh failure and allows retry', async () => {
    const w = await modal()
    vi.mocked(getProxyRelatedAccounts).mockRejectedValueOnce(new Error('offline'))
    await w.findAll('button').find(b => b.text() === '刷新')!.trigger('click'); await flushPromises()
    expect(w.get('[role="status"]').text()).toContain('已有明细可能过期')
    expect(w.text()).toContain('自动账号')
    await w.findAll('button').find(b => b.text() === '刷新')!.trigger('click'); await flushPromises()
    expect(w.find('[role="status"]').exists()).toBe(false)
  })
  it('returns to page one if all matching accounts disappear during pagination', async () => {
    const first = fixture(); first.total = 21
    vi.mocked(getProxyRelatedAccounts).mockResolvedValueOnce(first)
    const w = await modal()
    const empty = fixture(); empty.total = 0; empty.items = []; empty.current_count = 0; empty.departed_count = 0
    vi.mocked(getProxyRelatedAccounts).mockResolvedValueOnce({ ...empty, page: 2 }).mockResolvedValueOnce(empty)
    await w.findAll('button').find(b => b.text() === '下一页')!.trigger('click'); await flushPromises()
    expect(getProxyRelatedAccounts).toHaveBeenLastCalledWith(3, expect.objectContaining({ page: 1 }), expect.any(AbortSignal))
    expect(w.text()).toContain('第 1 / 1 页')
    expect(w.text()).toContain('没有符合条件的账号')
  })
  it('does not turn an initial read error into a zero-account claim', async () => {
    vi.mocked(getProxyRelatedAccounts).mockRejectedValueOnce(new Error('offline'))
    const w = await modal()
    expect(w.text()).toContain('账号明细读取失败')
    expect(w.text()).not.toContain('当前关联 0 个')
    expect(w.text()).not.toContain('没有符合条件的账号')
  })
  it('ignores a late response after changing nodes and cancels requests on close', async () => {
    let resolveOld!: (value: ProxyRelatedAccountsView) => void
    vi.mocked(getProxyRelatedAccounts).mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const w = await modal()
    const oldSignal = vi.mocked(getProxyRelatedAccounts).mock.calls[0][2]!
    const next = fixture(); next.proxy = { id: 8, name: '新选中节点' }; next.items = []; next.total = 0
    vi.mocked(getProxyRelatedAccounts).mockResolvedValueOnce(next)
    await w.setProps({ node: { proxy_id: 8, name: '新选中节点' } })
    await vi.advanceTimersByTimeAsync(250); await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    resolveOld(fixture()); await flushPromises()
    expect(w.text()).toContain('新选中节点')
    expect(w.text()).not.toContain('自动账号')
    const latestSignal = vi.mocked(getProxyRelatedAccounts).mock.calls.at(-1)![2]!
    w.unmount()
    expect(latestSignal.aborted).toBe(true)
  })
})
