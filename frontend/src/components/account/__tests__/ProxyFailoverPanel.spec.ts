import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ProxyFailoverPanel from '../ProxyFailoverPanel.vue'
import { getProxyFailover, saveProxyFailover, type ProxyFailoverView } from '@/api/admin/proxyFailover'

vi.mock('@/api/admin/proxyFailover', () => ({ getProxyFailover: vi.fn(), saveProxyFailover: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { proxies: { testProxy: vi.fn() } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
const initial = (): ProxyFailoverView => ({ enabled: true, primary_proxy_id: 1, backup_proxy_ids: [2, 3], current_proxy_id: 1, revision: 4, state: 'healthy', health: [], events: [] })
beforeEach(() => { vi.clearAllMocks(); vi.mocked(getProxyFailover).mockResolvedValue(initial()) })
async function panel(selectedProxyId = 1) {
  const wrapper = mount(ProxyFailoverPanel, { props: { accountId: 9, selectedProxyId, proxies: [] }, global: { stubs: { ProxySelector: true } } })
  await flushPromises()
  return wrapper
}
describe('Proxy failover policy', () => {
  it('saves backup priority with the loaded revision and current proxy', async () => {
    const wrapper = await panel()
    await wrapper.findAll('[aria-label="admin.accounts.proxyFailover.moveUp"]')[1].trigger('click')
    vi.mocked(saveProxyFailover).mockResolvedValue({ ...initial(), backup_proxy_ids: [3, 2], revision: 5 })
    await wrapper.get('.btn-primary').trigger('click'); await flushPromises()
    expect(saveProxyFailover).toHaveBeenCalledWith(9, { enabled: true, primary_proxy_id: 1, backup_proxy_ids: [3, 2], revision: 4, current_proxy_id: 1 })
    expect(wrapper.text()).toContain('admin.accounts.proxyFailover.saved')
  })
  it('blocks policy saves while the account proxy has unsaved changes', async () => {
    const wrapper = await panel(4)
    await wrapper.get('input[type="checkbox"]').setValue(false)
    expect(wrapper.get('.btn-primary').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('admin.accounts.proxyFailover.pendingProxy')
    expect(saveProxyFailover).not.toHaveBeenCalled()
  })
  it('keeps unsaved ordering across status refresh and rejects stale saves', async () => {
    const wrapper = await panel()
    await wrapper.findAll('[aria-label="admin.accounts.proxyFailover.moveUp"]')[1].trigger('click')
    vi.mocked(getProxyFailover).mockResolvedValue({ ...initial(), revision: 5 })
    await wrapper.findAll('button').find(b => b.text() === 'common.refresh')!.trigger('click'); await flushPromises()
    vi.mocked(saveProxyFailover).mockRejectedValue({ response: { status: 409 } })
    await wrapper.get('.btn-primary').trigger('click'); await flushPromises()
    expect(saveProxyFailover).toHaveBeenCalledWith(9, expect.objectContaining({ revision: 4, backup_proxy_ids: [3, 2] }))
    expect(wrapper.get('.btn-primary').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('saveFailed')
  })
  it('emits fresh routing and permits disabling an existing policy', async () => {
    vi.mocked(getProxyFailover).mockResolvedValue({ ...initial(), current_proxy_id: 2 })
    const wrapper = await panel(2)
    expect(wrapper.emitted('current')?.[0]).toEqual([2])
    await wrapper.get('input[type="checkbox"]').setValue(false)
    vi.mocked(saveProxyFailover).mockResolvedValue({ ...initial(), enabled: false, current_proxy_id: 2 })
    await wrapper.get('.btn-primary').trigger('click'); await flushPromises()
    expect(saveProxyFailover).toHaveBeenCalledWith(9, expect.objectContaining({ enabled: false, current_proxy_id: 2 }))
  })
})
