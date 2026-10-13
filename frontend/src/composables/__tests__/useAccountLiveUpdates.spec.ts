import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAccountLiveUpdates } from '../useAccountLiveUpdates'

describe('Account live status stream', () => {
  beforeEach(() => { vi.useFakeTimers(); Object.defineProperty(document, 'hidden', { configurable: true, value: false }) })
  afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers() })
  it('refreshes on split change frames and stops reconnecting after unmount', async () => {
    const refresh = vi.fn().mockResolvedValue(undefined)
    const read = vi.fn().mockResolvedValueOnce({ value: new TextEncoder().encode('data: {"type":"cha'), done: false }).mockResolvedValueOnce({ value: new TextEncoder().encode('nge"}\n\ndata: {"type":"heartbeat"}\n\n'), done: false }).mockResolvedValue({ done: true })
    const fetcher = vi.fn().mockResolvedValue({ ok: true, status: 200, body: { getReader: () => ({ read, releaseLock: vi.fn() }) } })
    vi.stubGlobal('fetch', fetcher)
    const wrapper = mount(defineComponent({ setup() { useAccountLiveUpdates(refresh); return () => null } }))
    await flushPromises(); expect(refresh).toHaveBeenCalledTimes(1)
    wrapper.unmount(); await vi.advanceTimersByTimeAsync(60_000); expect(fetcher).toHaveBeenCalledTimes(1)
  })
  it('stops on an expired admin session without falling back to data requests', async () => {
    const refresh = vi.fn()
    const fetcher = vi.fn().mockResolvedValue({ status: 401 })
    vi.stubGlobal('fetch', fetcher)
    const wrapper = mount(defineComponent({ setup() { useAccountLiveUpdates(refresh); return () => null } }))
    await flushPromises(); await vi.advanceTimersByTimeAsync(60_000)
    expect(refresh).not.toHaveBeenCalled(); expect(fetcher).toHaveBeenCalledTimes(1); wrapper.unmount()
  })
})
