import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountBulkTestModal from '../AccountBulkTestModal.vue'
import { AccountTestSessionExpired } from '@/api/admin/accountTest'

const { testConnection, getModels } = vi.hoisted(() => ({ testConnection: vi.fn(), getModels: vi.fn() }))
vi.mock('@/api/admin/accounts', () => ({ getAvailableModels: getModels }))
vi.mock('@/api/admin/accountTest', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/admin/accountTest')>(),
  testAccountConnection: testConnection
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const mountModal = (ids = [1, 2, 3, 4, 5]) => mount(AccountBulkTestModal, {
  props: { show: false, runId: 0, targets: ids.map(id => ({ id, name: `account-${id}` })) },
  global: { stubs: {
    Teleport: true,
    Select: { name: 'Select', props: ['modelValue', 'options', 'disabled'], emits: ['update:modelValue'], template: `<select :value="modelValue" :disabled="disabled" @change="$emit('update:modelValue', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>` },
    BaseDialog: { name: 'BaseDialog', props: ['show'], emits: ['close'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }
  } }
})
beforeEach(() => {
  testConnection.mockReset()
  getModels.mockReset().mockResolvedValue([
    { id: 'gpt-image-2', display_name: 'GPT Image 2' },
    { id: 'gpt-image-2.5-flare', display_name: 'GPT Image 2.5 Flare' },
    { id: 'gpt-image-2.5-sunburst', display_name: 'GPT Image 2.5 Sunburst' }
  ])
})

describe('bulk account tests', () => {
  it.each([1, 3, 15])('uses %i workers and fills each freed slot without exceeding the selection', async concurrency => {
    const finish = new Map<number, (result: { success: boolean }) => void>()
    testConnection.mockImplementation(id => new Promise(resolve => finish.set(id, resolve)))
    const ids = Array.from({ length: 18 }, (_, i) => i + 1)
    const wrapper = mountModal([...ids, 1])
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    const select = wrapper.get('#bulk-test-concurrency')
    expect((select.element as HTMLSelectElement).value).toBe('3')
    expect(select.findAll('option').map(option => option.attributes('value'))).toEqual(Array.from({ length: 15 }, (_, i) => String(i + 1)))
    await select.setValue(String(concurrency))
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
    expect(testConnection).toHaveBeenCalledTimes(concurrency)
    expect(select.attributes('disabled')).toBeDefined()
    wrapper.findAllComponents({ name: 'Select' })[1].vm.$emit('update:modelValue', 1)
    await wrapper.setProps({ show: false })
    expect(wrapper.get('[data-testid="bulk-test-background-concurrency"]').exists()).toBe(true)
    for (const id of ids) {
      expect(finish.has(id)).toBe(true)
      finish.get(id)!({ success: true })
      await flushPromises()
      expect(testConnection).toHaveBeenCalledTimes(Math.min(concurrency + id, ids.length))
    }
    expect(new Set(testConnection.mock.calls.map(call => call[0])).size).toBe(ids.length)
    await wrapper.setProps({ show: true })
    expect((wrapper.get('#bulk-test-concurrency').element as HTMLSelectElement).value).toBe(String(concurrency))
    expect(wrapper.emitted('completed')).toHaveLength(1)
    wrapper.unmount()
  })

  it.each([0, 16, 2.5, '', 'bad', NaN, Infinity, null, true])('rejects an invalid concurrency value %s', async value => {
    testConnection.mockResolvedValue({ success: true })
    const wrapper = mountModal()
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    wrapper.findAllComponents({ name: 'Select' })[1].vm.$emit('update:modelValue', value)
    await flushPromises()
    expect((wrapper.get('#bulk-test-concurrency').element as HTMLSelectElement).value).toBe('3')
    wrapper.unmount()
  })

  it('limits fifteen workers to the unique targets and stops every in-flight request', async () => {
    testConnection.mockImplementation((_id, signal: AbortSignal) => new Promise((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    }))
    const wrapper = mountModal([1, 2, 1])
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    await wrapper.get('#bulk-test-concurrency').setValue('15')
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
    expect(testConnection).toHaveBeenCalledTimes(2)
    await wrapper.get('[data-testid="bulk-test-stop"]').trigger('click')
    await flushPromises()
    expect(testConnection.mock.calls.every(call => call[1].aborted)).toBe(true)
    expect(wrapper.emitted('account-tested')).toBeUndefined()
    await wrapper.setProps({ runId: 2 })
    await flushPromises()
    expect((wrapper.get('#bulk-test-concurrency').element as HTMLSelectElement).value).toBe('3')
    wrapper.unmount()
  })

  it.each(['stop', 'session'])('stops a fifteen-worker batch with queued targets after %s', async reason => {
    let failFirst!: (error: Error) => void
    testConnection.mockImplementation((id, signal: AbortSignal) => new Promise((_resolve, reject) => {
      if (id === 1) failFirst = reject
      signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    }))
    const wrapper = mountModal(Array.from({ length: 18 }, (_, i) => i + 1))
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    await wrapper.get('#bulk-test-concurrency').setValue('15')
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
    expect(testConnection).toHaveBeenCalledTimes(15)
    if (reason === 'session') failFirst(new AccountTestSessionExpired())
    else await wrapper.get('[data-testid="bulk-test-stop"]').trigger('click')
    await flushPromises()
    expect(testConnection).toHaveBeenCalledTimes(15)
    expect(testConnection.mock.calls.every(call => call[1].aborted)).toBe(true)
    expect(wrapper.get('[data-testid="bulk-test-row-18"]').text()).toContain('states.cancelled')
    expect(wrapper.emitted('account-tested')).toBeUndefined()
    expect(wrapper.emitted('completed')).toHaveLength(1)
    wrapper.unmount()
  })

  it('prepares the selected accounts without testing and defaults to Flare before starting', async () => {
    testConnection.mockResolvedValue({ success: true })
    const wrapper = mountModal([1, 2])
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    expect(getModels).toHaveBeenCalledTimes(1)
    expect(getModels).toHaveBeenCalledWith(1)
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('gpt-image-2.5-flare')
    expect(testConnection).not.toHaveBeenCalled()
    expect(wrapper.emitted('completed')).toBeUndefined()
    await wrapper.setProps({ targets: [{ id: 99, name: 'changed' }] })
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
    await flushPromises()
    expect(testConnection.mock.calls.map(call => call[2])).toEqual([
      { modelId: 'gpt-image-2.5-flare' }, { modelId: 'gpt-image-2.5-flare' }
    ])
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([1, 2])
    wrapper.unmount()
  })

  it('sends the chosen model to every queued account and retains it in the background', async () => {
    const finish = new Map<number, (value: { success: boolean }) => void>()
    testConnection.mockImplementation((id: number) => new Promise(resolve => finish.set(id, resolve)))
    const wrapper = mountModal([1, 2, 3, 4])
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    await wrapper.get('select').setValue('gpt-image-2.5-sunburst')
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
    expect(wrapper.get('select').attributes('disabled')).toBeDefined()
    // The model and target snapshots also protect later queued requests.
    wrapper.findComponent({ name: 'Select' }).vm.$emit('update:modelValue', 'gpt-image-2')
    await wrapper.setProps({ show: false, targets: [{ id: 99, name: 'changed' }] })
    expect(wrapper.get('[data-testid="bulk-test-background-model"]').text()).toBe('GPT Image 2.5 Sunburst')
    finish.get(1)!({ success: true })
    await flushPromises()
    expect(testConnection.mock.calls.map(call => call[0])).toEqual([1, 2, 3, 4])
    expect(testConnection.mock.calls.every(call => call[2].modelId === 'gpt-image-2.5-sunburst')).toBe(true)
    finish.get(2)!({ success: true }); finish.get(3)!({ success: true }); finish.get(4)!({ success: true })
    await flushPromises()
    await wrapper.setProps({ show: true })
    expect(testConnection).toHaveBeenCalledTimes(4)
    wrapper.unmount()
  })

  it('allows each account’s default model and preserves that fallback for other catalogs', async () => {
    getModels.mockResolvedValue([{ id: 'claude-sonnet', display_name: 'Sonnet' }])
    testConnection.mockResolvedValue({ success: true })
    const wrapper = mountModal([1])
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('')
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
    await flushPromises()
    expect(testConnection).toHaveBeenCalledWith(1, expect.any(AbortSignal), { modelId: '' })
    wrapper.unmount()
  })

  it('blocks testing while models load or fail and supports reloading without upstream tests', async () => {
    let reject!: (error: Error) => void
    getModels.mockImplementationOnce(() => new Promise((_resolve, fail) => { reject = fail }))
    const wrapper = mountModal([1])
    await wrapper.setProps({ show: true, runId: 1 })
    expect(wrapper.get('[data-testid="bulk-test-start"]').attributes('disabled')).toBeDefined()
    reject(new Error('offline'))
    await flushPromises()
    expect(wrapper.get('[data-testid="bulk-test-start"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="bulk-test-retry-models"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="bulk-test-start"]').attributes('disabled')).toBeUndefined()
    expect(testConnection).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('discards a stale model list when preparing another batch', async () => {
    let finish!: (models: { id: string }[]) => void
    getModels.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const wrapper = mountModal([1])
    await wrapper.setProps({ show: true, runId: 1 })
    await wrapper.setProps({ runId: 2, targets: [{ id: 2, name: 'second' }] })
    await flushPromises()
    finish([{ id: 'stale-model' }])
    await flushPromises()
    expect(wrapper.find('option[value="stale-model"]').exists()).toBe(false)
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('gpt-image-2.5-flare')
    wrapper.unmount()
  })

  it('closes an unstarted batch without a test or background notification', async () => {
    const wrapper = mountModal([1])
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    await wrapper.get('[data-testid="bulk-test-close"]').trigger('click')
    await wrapper.setProps({ show: false })
    expect(wrapper.find('[data-testid="bulk-test-background"]').exists()).toBe(false)
    expect(testConnection).not.toHaveBeenCalled()
    expect(wrapper.emitted('completed')).toBeUndefined()
    wrapper.unmount()
  })

  it('tests selected IDs once, limits concurrency to three, and continues after a failure', async () => {
    const finish = new Map<number, (value: { success: boolean; statusCode?: number }) => void>()
    testConnection.mockImplementation((id: number) => new Promise(resolve => finish.set(id, resolve)))
    const wrapper = mountModal([3, 5, 7, 9, 11, 3])
    await wrapper.setProps({ show: true, runId: 1 })
    await flushPromises()
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
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
    await flushPromises()
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
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
    await flushPromises()
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
    await wrapper.setProps({ runId: 2, targets: [{ id: 99, name: 'new-account' }] })
    expect(testConnection).toHaveBeenCalledTimes(1)
    finish({ success: true })
    await flushPromises()
    await wrapper.setProps({ runId: 3 })
    await flushPromises()
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
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
    await flushPromises()
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
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
    await flushPromises()
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
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
    await wrapper.get('[data-testid="bulk-test-start"]').trigger('click')
    await flushPromises()
    expect(testConnection).toHaveBeenCalledTimes(3)
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.emitted('account-tested')).toBeUndefined()
    expect(wrapper.get('[data-testid="bulk-test-row-5"]').text()).toContain('states.cancelled')
    wrapper.unmount()
  })
})
