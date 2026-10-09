import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ProxyAllocationModal from '../ProxyAllocationModal.vue'
import { getProxyAllocation, saveProxyAllocation, type ProxyAllocationView } from '@/api/admin/proxyAllocation'

vi.mock('@/api/admin/proxyAllocation', () => ({ getProxyAllocation: vi.fn(), saveProxyAllocation: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
const initial = (): ProxyAllocationView => ({ enabled: false, revision: 4, accounts_per_proxy: 3, accounts: 7, assigned: 6, waiting: 1, binding_only_accounts: 0, healthy_nodes: 2, available_slots: 0, checked_at: null, nodes: [{ id: 1, name: 'Tokyo', accounts: 3, status: 'healthy', latency_ms: 200 }] })
beforeEach(() => { vi.clearAllMocks(); vi.mocked(getProxyAllocation).mockResolvedValue(initial()) })
async function panel() {
  const w = mount(ProxyAllocationModal, { global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } } })
  await flushPromises()
  return w
}
describe('Continuous proxy allocation', () => {
  it('saves the enable setting with its revision and shows waiting capacity', async () => {
    const w = await panel()
    expect(w.text()).toContain('3 / 3')
    await w.get('input[type="checkbox"]').setValue(true)
    vi.mocked(saveProxyAllocation).mockResolvedValue({ ...initial(), enabled: true, revision: 5 })
    await w.get('.btn-primary').trigger('click'); await flushPromises()
    expect(saveProxyAllocation).toHaveBeenCalledWith(true, 4)
    expect(w.text()).toContain('admin.proxies.allocation.waitingHint')
    expect(w.emitted('changed')).toHaveLength(1)
  })
  it('preserves an unsaved toggle across refresh and blocks stale retries', async () => {
    const w = await panel()
    await w.get('input').setValue(true)
    vi.mocked(getProxyAllocation).mockResolvedValue({ ...initial(), revision: 5 })
    await w.get('.btn-secondary').trigger('click'); await flushPromises()
    expect((w.get('input').element as HTMLInputElement).checked).toBe(true)
    vi.mocked(saveProxyAllocation).mockRejectedValue({ response: { status: 409 } })
    await w.get('.btn-primary').trigger('click'); await flushPromises()
    expect(saveProxyAllocation).toHaveBeenCalledWith(true, 4)
    expect(w.get('[role="alert"]').text()).toContain('conflict')
    expect(w.get('.btn-primary').attributes('disabled')).toBeDefined()
    await w.get('.btn-secondary').trigger('click'); await flushPromises()
    expect((w.get('input').element as HTMLInputElement).checked).toBe(false)
  })
  it('distinguishes IP bindings from resuming existing stopped accounts', async () => {
    vi.mocked(getProxyAllocation).mockResolvedValue({ ...initial(), enabled: true, binding_only_accounts: 3 })
    const w = await panel()
    expect(w.get('[data-testid="binding-only-hint"]').text()).toContain('admin.proxies.allocation.bindingOnlyHint')
    expect(w.text()).toContain('admin.proxies.allocation.assigned')
    expect(w.text()).toContain('admin.proxies.allocation.waitingHint')
  })
  it('does not allow changing settings before a successful load', async () => {
    vi.mocked(getProxyAllocation).mockRejectedValue(new Error('offline'))
    const w = await panel()
    expect(w.get('[role="alert"]').text()).toContain('loadFailed')
    expect(w.get('.btn-primary').attributes('disabled')).toBeDefined()
  })
})
