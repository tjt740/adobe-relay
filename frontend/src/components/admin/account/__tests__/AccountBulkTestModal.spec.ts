import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountBulkTestModal from '../AccountBulkTestModal.vue'
import { AccountTestSessionExpired } from '@/api/admin/accountTest'

const { testConnection } = vi.hoisted(() => ({ testConnection: vi.fn() }))
vi.mock('@/api/admin/accountTest', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/admin/accountTest')>(),
  testAccountConnection: testConnection
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const mountModal = (ids = [1, 2, 3, 4, 5]) => mount(AccountBulkTestModal, {
  props: { show: false, targets: ids.map(id => ({ id, name: `account-${id}` })) },
  global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } }
})
beforeEach(() => { testConnection.mockReset() })

describe('bulk account tests', () => {
  it('tests selected IDs once, limits concurrency to three, and continues after a failure', async () => {
    const finish = new Map<number, (value: { success: boolean; statusCode?: number }) => void>()
    testConnection.mockImplementation((id: number) => new Promise(resolve => finish.set(id, resolve)))
    const wrapper = mountModal([3, 5, 7, 9, 11, 3])
    await wrapper.setProps({ show: true })
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([3, 5, 7])
    // Changes to the live selection must not retarget an in-progress run.
    await wrapper.setProps({ targets: [{ id: 99, name: 'unselected' }] })
    finish.get(3)!({ success: false, statusCode: 401 })
    await flushPromises()
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([3, 5, 7, 9])
    finish.get(5)!({ success: true }); finish.get(7)!({ success: true })
    await flushPromises()
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([3, 5, 7, 9, 11])
    finish.get(9)!({ success: true }); finish.get(11)!({ success: true })
    await flushPromises()
    expect(wrapper.get('[data-testid="bulk-test-row-3"]').text()).toContain('401')
    expect(wrapper.get('[data-testid="bulk-test-row-11"]').text()).toContain('states.success')
    expect(wrapper.emitted('completed')).toHaveLength(1)
    wrapper.unmount()
  })
  it('stops pending and in-flight work without starting more accounts', async () => {
    testConnection.mockImplementation((_id, signal: AbortSignal) => new Promise((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    }))
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await wrapper.get('[data-testid="bulk-test-stop"]').trigger('click')
    await flushPromises()
    expect(testConnection).toHaveBeenCalledTimes(3)
    expect(wrapper.get('[data-testid="bulk-test-row-5"]').text()).toContain('states.cancelled')
    expect(wrapper.get('[data-testid="bulk-test-row-1"]').text()).toContain('states.cancelled')
    expect(wrapper.emitted('completed')).toHaveLength(1)
    wrapper.unmount()
  })
  it('stops the batch if the admin session expires', async () => {
    testConnection.mockRejectedValue(new AccountTestSessionExpired())
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(testConnection).toHaveBeenCalledTimes(3)
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="bulk-test-row-5"]').text()).toContain('states.cancelled')
    wrapper.unmount()
  })
})
