import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountTableFilters from '../AccountTableFilters.vue'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const Select = { props: ['modelValue', 'options'], emits: ['update:modelValue', 'change'], template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value); $emit(\'change\')"><option v-for="option in options" :value="option.value">{{ option.label }}</option></select>' }
describe('account table filters', () => {
  it('keeps status filters while moving capacity controls out of the toolbar', async () => {
    const filters = { platform: 'adobe', status: 'request_timeout', capacity: 'full', concurrency_min: '', concurrency_max: '' }
    const wrapper = mount(AccountTableFilters, { props: { filters, searchQuery: '' }, global: { stubs: { Select, SearchInput: true } } })
    const options = wrapper.findAll('option').map(option => option.attributes('value'))
    expect(options).toEqual(expect.arrayContaining(['authentication_failed', 'request_timeout', 'test_passed', 'test_failed', 'untested']))
    expect(wrapper.find('input[type=number]').exists()).toBe(false)
    wrapper.unmount()
  })
})
