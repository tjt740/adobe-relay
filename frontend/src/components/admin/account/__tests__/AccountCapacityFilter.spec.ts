import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AccountCapacityFilter from '../AccountCapacityFilter.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const initial = { capacity: 'full', concurrency_min: '2', concurrency_max: '8' }
const wrappers: ReturnType<typeof mount>[] = []
const mountFilter = (filters = initial) => {
  const wrapper = mount(AccountCapacityFilter, {
    props: { id: 'capacity-test', filters },
    attachTo: document.body,
    global: { stubs: { Teleport: true, Icon: true } }
  })
  wrappers.push(wrapper)
  return wrapper
}
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

describe('capacity header filter', () => {
  it('edits a draft, applies the state and bounds together, and restores trigger focus', async () => {
    const wrapper = mountFilter()
    const trigger = wrapper.get('[data-test="capacity-filter-trigger"]')
    expect(wrapper.get('[data-test="capacity-filter-count"]').text()).toBe('2')
    await trigger.trigger('click')
    await flushPromises()
    expect(wrapper.get('input[value="full"]').element).toHaveProperty('checked', true)
    await wrapper.get('input[value="in_use"]').setValue()
    await wrapper.get('input[aria-label="admin.accounts.capacityFilter.min"]').setValue('3')
    expect(wrapper.emitted('apply')).toBeUndefined()
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('apply')).toEqual([[{ capacity: 'in_use', concurrency_min: '3', concurrency_max: '8' }]])
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
  })

  it('discards edits on cancel and Escape and reopens with the applied values', async () => {
    const wrapper = mountFilter()
    const trigger = wrapper.get('[data-test="capacity-filter-trigger"]')
    await trigger.trigger('click')
    await wrapper.get('input[value="idle"]').setValue()
    await wrapper.get('[data-test="capacity-filter-cancel"]').trigger('click')
    expect(wrapper.emitted('apply')).toBeUndefined()
    await trigger.trigger('click')
    expect(wrapper.get('input[value="full"]').element).toHaveProperty('checked', true)
    await wrapper.get('[role="dialog"]').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.emitted('apply')).toBeUndefined()
  })

  it('rejects invalid ranges and clears both applied conditions in one action', async () => {
    const wrapper = mountFilter()
    await wrapper.get('[data-test="capacity-filter-trigger"]').trigger('click')
    const minimum = wrapper.get('input[aria-label="admin.accounts.capacityFilter.min"]')
    await minimum.setValue('9')
    expect(wrapper.get('[role="alert"]').text()).toContain('invalidRange')
    expect(wrapper.get('[data-test="capacity-filter-apply"]').attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('apply')).toBeUndefined()
    await minimum.setValue('1.5')
    expect(wrapper.get('[role="alert"]').text()).toContain('positiveInteger')
    await wrapper.get('[data-test="capacity-filter-clear"]').trigger('click')
    expect(wrapper.emitted('apply')).toEqual([[{ capacity: '', concurrency_min: '', concurrency_max: '' }]])
    await wrapper.setProps({ filters: { capacity: '', concurrency_min: '', concurrency_max: '' } })
    expect(wrapper.find('[data-test="capacity-filter-count"]').exists()).toBe(false)
  })
})
