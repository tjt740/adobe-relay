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
  props: { show: false, runId: 0, targets: ids.map(id => ({ id, name: `account-${id}` })) },
  global: { stubs: {
    Teleport: true,
    BaseDialog: { name: 'BaseDialog', props: ['show'], emits: ['close'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }
  } }
})
beforeEach(() => { testConnection.mockReset() })

describe('bulk account tests', () => {
  it('tests selected IDs once, limits concurrency to three, and continues after a failure', async () => {
    const finish = new Map<number, (value: { success: boolean; statusCode?: number }) => void>()
    testConnection.mockImplementation((id: number) => new Promise(resolve => finish.set(id, resolve)))
    const wrapper = mountModal([3, 5, 7, 9, 11, 3])
    await wrapper.setProps({ show: true, runId: 1 })
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([3, 5, 7])
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('0')
    expect(wrapper.find('[data-testid="bulk-test-progress-loading"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="bulk-test-row-3"]').find('.animate-spin').exists()).toBe(true)
    // Changes to the live selection must not retarget an in-progress run.
    await wrapper.setProps({ targets: [{ id: 99, name: 'unselected' }] })
    finish.get(3)!({ success: false, statusCode: 401 })
    await flushPromises()
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([3, 5, 7, 9])
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('1')
    expect(wrapper.emitted('account-tested')).toEqual([[3]])
    expect(wrapper.emitted('completed')).toBeUndefined()
    expect(wrapper.get('[data-testid="bulk-test-progress-fill"]').attributes('style')).toContain('width: 20%')
    finish.get(5)!({ success: true }); finish.get(7)!({ success: true })
    await flushPromises()
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([3, 5, 7, 9, 11])
    finish.get(9)!({ success: true }); finish.get(11)!({ success: true })
    await flushPromises()
    expect(wrapper.get('[data-testid="bulk-test-row-3"]').text()).toContain('401')
    expect(wrapper.get('[data-testid="bulk-test-row-11"]').text()).toContain('states.success')
    expect(wrapper.emitted('completed')).toHaveLength(1)
    expect(wrapper.get('[data-testid="bulk-test-progress-fill"]').attributes('style')).toContain('width: 100%')
    expect(wrapper.find('[data-testid="bulk-test-progress-loading"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="bulk-test-close"]').text()).toBe('admin.accounts.bulkTest.complete')
    wrapper.unmount()
  })

  it('continues after closing, resumes the same results, and completes while hidden without duplicate requests', async () => {
    const finish = new Map<number, (value: { success: boolean }) => void>()
    testConnection.mockImplementation((id: number) => new Promise(resolve => finish.set(id, resolve)))
    const wrapper = mountModal([1, 2, 3, 4])
    await wrapper.setProps({ show: true, runId: 1 })
    expect(wrapper.get('[data-testid="bulk-test-close"]').text()).toBe('admin.accounts.bulkTest.runInBackground')
    await wrapper.get('[data-testid="bulk-test-close"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    await wrapper.setProps({ show: false })
    expect(testConnection.mock.calls.every(call => !call[1].aborted)).toBe(true)
    expect(wrapper.find('[data-testid="bulk-test-background"]').exists()).toBe(true)

    finish.get(1)!({ success: true })
    await flushPromises()
    expect(wrapper.emitted('account-tested')).toEqual([[1]])
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([1, 2, 3, 4])
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('1')
    await wrapper.get('[data-testid="bulk-test-reopen"]').trigger('click')
    expect(wrapper.emitted('open')).toHaveLength(1)
    await wrapper.setProps({ show: true })
    expect(wrapper.get('[data-testid="bulk-test-row-1"]').text()).toContain('states.success')
    expect(testConnection).toHaveBeenCalledTimes(4)

    // Header X / Escape use the same close event as the footer button.
    wrapper.findComponent({ name: 'BaseDialog' }).vm.$emit('close')
    await wrapper.setProps({ show: false })
    finish.get(2)!({ success: true }); finish.get(3)!({ success: true }); finish.get(4)!({ success: true })
    await flushPromises()
    expect(wrapper.emitted('completed')).toHaveLength(1)
    expect(wrapper.get('[data-testid="bulk-test-background"]').text()).toContain('bulkTest.complete')
    expect(wrapper.get('[data-testid="bulk-test-reopen"]').text()).toContain('bulkTest.viewResults')
    expect(wrapper.find('.animate-spin').exists()).toBe(false)
    expect(wrapper.emitted('running-change')?.at(-1)).toEqual([false])

    await wrapper.setProps({ show: true })
    expect(testConnection).toHaveBeenCalledTimes(4)
    await wrapper.get('[data-testid="bulk-test-close"]').trigger('click')
    await wrapper.setProps({ show: false })
    expect(wrapper.find('[data-testid="bulk-test-background"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not start a second batch while one is running, and accepts an explicit new batch after completion', async () => {
    let finish!: (value: { success: boolean }) => void
    testConnection.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const wrapper = mountModal([1])
    await wrapper.setProps({ show: true, runId: 1 })
    await wrapper.setProps({ runId: 2, targets: [{ id: 99, name: 'new-account' }] })
    expect(testConnection).toHaveBeenCalledTimes(1)
    finish({ success: true })
    await flushPromises()
    await wrapper.setProps({ runId: 3 })
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([1, 99])
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('0')
    finish({ success: true })
    await flushPromises()
    wrapper.unmount()
  })

  it('stops pending and in-flight work without starting more accounts', async () => {
    testConnection.mockImplementation((_id, signal: AbortSignal) => new Promise((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    }))
    const wrapper = mountModal()
    await wrapper.setProps({ show: true, runId: 1 })
    await wrapper.get('[data-testid="bulk-test-stop"]').trigger('click')
    await flushPromises()
    expect(testConnection).toHaveBeenCalledTimes(3)
    expect(wrapper.get('[data-testid="bulk-test-row-5"]').text()).toContain('states.cancelled')
    expect(wrapper.get('[data-testid="bulk-test-row-1"]').text()).toContain('states.cancelled')
    expect(wrapper.emitted('account-tested')).toBeUndefined()
    expect(wrapper.emitted('completed')).toHaveLength(1)
    expect(wrapper.find('.animate-spin').exists()).toBe(false)
    wrapper.unmount()
  })

  it('cancels requests when leaving the page without announcing a finished batch', async () => {
    testConnection.mockImplementation((_id, signal: AbortSignal) => new Promise((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    }))
    const wrapper = mountModal()
    await wrapper.setProps({ show: true, runId: 1 })
    wrapper.unmount()
    await flushPromises()
    expect(testConnection).toHaveBeenCalledTimes(3)
    expect(testConnection.mock.calls.every(call => call[1].aborted)).toBe(true)
    expect(wrapper.emitted('completed')).toBeUndefined()
  })
  it('stops the batch if the admin session expires', async () => {
    testConnection.mockRejectedValue(new AccountTestSessionExpired())
    const wrapper = mountModal()
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    expect(testConnection).toHaveBeenCalledTimes(3)
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.emitted('account-tested')).toBeUndefined()
    expect(wrapper.get('[data-testid="bulk-test-row-5"]').text()).toContain('states.cancelled')
    wrapper.unmount()
  })
})
