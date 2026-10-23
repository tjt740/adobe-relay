import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ProxiesView from '../ProxiesView.vue'
import { useClashNodeTestStore } from '@/stores/clashNodeTests'

const { listProxies, getAllWithCount } = vi.hoisted(() => ({
  listProxies: vi.fn(),
  getAllWithCount: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { proxies: { list: listProxies, getAllWithCount } }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const mountView = () => shallowMount(ProxiesView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: {
        template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
      },
      DataTable: {
        props: ['data'],
        template: '<div data-test="rows">{{ data.map(row => row.name).join(",") }}</div>'
      },
      Pagination: {
        props: ['page'],
        emits: ['update:page'],
        template: '<button data-test="page" @click="$emit(\'update:page\', 2)">{{ page }}</button>'
      },
      Select: {
        props: ['modelValue', 'options', 'placeholder'],
        emits: ['update:modelValue', 'change'],
        template: `<select :data-filter="placeholder" :value="modelValue"
          @change="$emit('update:modelValue', $event.target.value); $emit('change', $event.target.value)">
          <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>`
      }
    }
  }
})

let wrapper: ReturnType<typeof mountView>

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  getAllWithCount.mockResolvedValue([])
  listProxies.mockResolvedValue({ items: [], total: 100, pages: 5 })
})

afterEach(() => wrapper?.unmount())

describe('proxy list filter pagination', () => {
  it.each([
    ['protocol', 'admin.proxies.allProtocols', 'socks5', ''],
    ['protocol', 'admin.proxies.allProtocols', '', 'http'],
    ['status', 'admin.proxies.allStatus', 'expired', ''],
    ['status', 'admin.proxies.allStatus', '', 'active']
  ])('starts at page one when changing %s through %s to "%s"', async (field, placeholder, value, initial) => {
    wrapper = mountView()
    await flushPromises()
    const filter = wrapper.get(`select[data-filter="${placeholder}"]`)
    if (initial) {
      await filter.setValue(initial)
      await flushPromises()
    }
    await wrapper.get('[data-test="page"]').trigger('click')
    await flushPromises()
    expect(listProxies.mock.lastCall?.[0]).toBe(2)
    const [, pageSize, previousFilters] = listProxies.mock.lastCall!
    listProxies.mockImplementation(async (page) => ({
      items: page === 1 ? [{ id: 1, name: 'matching-proxy' }] : [],
      total: 1,
      pages: 1
    }))

    await filter.setValue(value)
    await flushPromises()

    expect(listProxies).toHaveBeenLastCalledWith(
      1, pageSize, { ...previousFilters, [field]: value || undefined },
      { signal: expect.any(AbortSignal) }
    )
    expect(wrapper.get('[data-test="page"]').text()).toBe('1')
    expect(wrapper.get('[data-test="rows"]').text()).toBe('matching-proxy')
  })

  it('keeps the current page when refreshing the same filter', async () => {
    wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="page"]').trigger('click')
    await flushPromises()

    await wrapper.get('button[title="common.refresh"]').trigger('click')
    await flushPromises()

    expect(listProxies.mock.lastCall?.[0]).toBe(2)
    expect(wrapper.get('[data-test="page"]').text()).toBe('2')
  })

  it('shows background progress, reopens the dialog, and refreshes on completion while closed', async () => {
    wrapper = mountView()
    await flushPromises()
    const task = useClashNodeTestStore()
    task.total = 7; task.done = 2; task.testing = true
    await flushPromises()
    expect(wrapper.get('[data-testid="clash-background-test-status"] progress').attributes('value')).toBe('2')
    await wrapper.findAll('button').find(b => b.text() === 'admin.proxies.clash.viewTestProgress')!.trigger('click')
    expect(wrapper.findComponent({ name: 'ClashSubscriptionModal' }).exists()).toBe(true)
    wrapper.getComponent({ name: 'ClashSubscriptionModal' }).vm.$emit('close')
    await flushPromises()
    const calls = listProxies.mock.calls.length
    task.done = 7; task.testing = false
    await flushPromises()
    expect(listProxies).toHaveBeenCalledTimes(calls + 1)
    expect(wrapper.get('[data-testid="clash-background-test-status"]').text()).toContain('admin.proxies.clash.testComplete')
    expect(wrapper.findComponent({ name: 'ClashSubscriptionModal' }).exists()).toBe(false)
    wrapper.unmount()
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="clash-background-test-status"]').text()).toContain('admin.proxies.clash.testComplete')
  })

  it('returns to page one after a subscription change removes old proxies', async () => {
    wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="page"]').trigger('click')
    await flushPromises()
    const subscriptionButton = wrapper.findAll('button').find(button => button.text().includes('admin.proxies.clash.title'))!
    await subscriptionButton.trigger('click')
    listProxies.mockResolvedValue({ items: [{ id: 1, name: 'current-node' }], total: 1, pages: 1 })

    wrapper.getComponent({ name: 'ClashSubscriptionModal' }).vm.$emit('changed', true)
    await flushPromises()

    expect(listProxies.mock.lastCall?.[0]).toBe(1)
    expect(wrapper.get('[data-test="page"]').text()).toBe('1')
    expect(wrapper.get('[data-test="rows"]').text()).toBe('current-node')
  })
})
