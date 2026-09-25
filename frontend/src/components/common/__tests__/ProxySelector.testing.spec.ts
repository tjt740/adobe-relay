import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ProxySelector from '../ProxySelector.vue'
import type { Proxy } from '@/types'

const testProxy = vi.hoisted(() => vi.fn())
vi.mock('@/api/admin', () => ({ adminAPI: { proxies: { testProxy } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks() })

async function openSelector() {
  const wrapper = mount(ProxySelector, {
    props: { modelValue: null, proxies: [1, 2].map(id => ({
      id, name: `Proxy ${id}`, host: 'localhost', port: 8080, protocol: 'http'
    } as Proxy)) },
    global: { stubs: { Icon: true } }
  })
  await wrapper.get('.select-trigger').trigger('click')
  return wrapper
}

describe('proxy connection tests', () => {
  it('shows saved latency on the selected proxy and every option without retesting', async () => {
    const wrapper = mount(ProxySelector, {
      props: { modelValue: 1, proxies: [
        { id: 1, name: 'Clash Singapore', protocol: 'http', host: 'mihomo', port: 20000,
          latency_status: 'success', latency_ms: 128 },
        { id: 2, name: 'Clash offline', protocol: 'http', host: 'mihomo', port: 20001,
          latency_status: 'failed', latency_ms: 99, latency_message: 'REALITY authentication failed' },
        { id: 3, name: 'Clash new', protocol: 'http', host: 'mihomo', port: 20002 },
        { id: 4, name: 'Clash local', protocol: 'http', host: 'mihomo', port: 20003,
          latency_status: 'success', latency_ms: 0 }
      ] as Proxy[] },
      global: { stubs: { Icon: true } }
    })
    expect(wrapper.get('.select-trigger').text()).toContain('128 ms')
    await wrapper.get('.select-trigger').trigger('click')
    const badges = wrapper.findAll('.select-options .latency-badge')
    expect(badges.map(badge => badge.text())).toEqual([
      '128 ms', 'admin.proxies.testFailed', 'admin.proxies.clash.notTested', '0 ms'
    ])
    expect(badges[1].attributes('title')).toBe('REALITY authentication failed')
    expect(testProxy).not.toHaveBeenCalled()
    await wrapper.findAll('.select-option')[2].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([2])
    await wrapper.setProps({ modelValue: 2 })
    expect(wrapper.get('.select-trigger').text()).toContain('admin.proxies.testFailed')
    expect(wrapper.get('.select-trigger').text()).not.toContain('99 ms')
  })

  it('updates the selected latency after a fresh test, including a failed retest', async () => {
    const wrapper = await openSelector()
    await wrapper.setProps({ modelValue: 1 })
    let finish!: (result: object) => void
    testProxy.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await wrapper.findAll('.test-btn')[0].trigger('click')
    expect(wrapper.get('.select-trigger').text()).toContain('admin.proxies.testing')
    finish({ success: true, latency_ms: 46 })
    await flushPromises()
    expect(wrapper.get('.select-trigger').text()).toContain('46 ms')
    testProxy.mockResolvedValueOnce({ success: false, message: 'Connection closed' })
    await wrapper.findAll('.test-btn')[0].trigger('click')
    await flushPromises()
    expect(wrapper.get('.select-trigger').text()).toContain('admin.proxies.testFailed')
    expect(wrapper.get('.select-trigger').text()).not.toContain('46 ms')
  })

  it('does not restart an individual test when a batch is started', async () => {
    let finish!: (result: object) => void
    testProxy.mockImplementation((id: number) => id === 1
      ? new Promise(resolve => { finish = resolve })
      : Promise.resolve({ success: true, country: 'GB' }))
    const wrapper = await openSelector()
    await wrapper.findAll('.test-btn')[0].trigger('click')
    await wrapper.get('.batch-test-btn').trigger('click')
    await flushPromises()
    expect(testProxy.mock.calls.map(([id]) => id)).toEqual([1, 2])
    expect(wrapper.findAll('.test-btn')[0].attributes('disabled')).toBeDefined()
    finish({ success: true, country: 'US' })
    await flushPromises()
    expect(wrapper.text()).toContain('US')
    expect(wrapper.findAll('.test-btn')[0].attributes('disabled')).toBeUndefined()
  })

  it('shows per-proxy outcomes and allows another batch after a failure', async () => {
    testProxy.mockImplementation((id: number) => id === 1
      ? Promise.reject(new Error('offline'))
      : Promise.resolve({ success: true, country: 'GB' }))
    const wrapper = await openSelector()
    await wrapper.get('.batch-test-btn').trigger('click')
    await flushPromises()
    expect(testProxy).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('admin.proxies.testFailed')
    expect(wrapper.text()).toContain('GB')
    expect(wrapper.get('.batch-test-btn').attributes('disabled')).toBeUndefined()
    await wrapper.get('.batch-test-btn').trigger('click')
    await flushPromises()
    expect(testProxy).toHaveBeenCalledTimes(4)
  })
})
