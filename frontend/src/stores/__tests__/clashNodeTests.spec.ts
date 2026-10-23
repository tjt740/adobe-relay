import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import { useClashNodeTestStore } from '../clashNodeTests'
import { useAuthStore } from '../auth'
import { testProxy } from '@/api/admin/proxies'
import type { ClashNode } from '@/api/admin/clash'
import type { User } from '@/types'

vi.mock('@/api/admin/proxies', () => ({ testProxy: vi.fn(), default: {} }))
const node = (id: number, enabled = true): ClashNode => ({ name: `Node ${id}`, type: 'ss', proxy_id: id, enabled })
beforeEach(() => { setActivePinia(createPinia()); vi.resetAllMocks() })

describe('application-owned Clash test queue', () => {
  it('keeps four workers, isolates failures and ignores duplicate or disabled nodes and duplicate starts', async () => {
    const pending = new Map<number, { resolve: (result: { success: boolean; latency_ms: number }) => void; reject: (error: Error) => void }>()
    vi.mocked(testProxy).mockImplementation(id => new Promise((resolve, reject) => { pending.set(id, { resolve, reject }) }))
    const task = useClashNodeTestStore()
    const run = task.start([node(1), node(1), node(0), node(10, false), ...[2, 3, 4, 5, 6].map(id => node(id))], String)
    expect(testProxy).toHaveBeenCalledTimes(4)
    expect(task.total).toBe(6)
    await task.start([node(8)], String)
    expect(task.total).toBe(6)
    task.clearResults()
    expect(task.total).toBe(6)
    pending.get(1)!.reject(new Error('timed out'))
    await flushPromises()
    expect(task.testingIDs.size).toBe(4)
    expect(task.results[1]).toMatchObject({ success: false, message: 'Error: timed out' })
    pending.get(2)!.resolve({ success: false, latency_ms: 0 })
    await flushPromises()
    expect(testProxy).toHaveBeenCalledTimes(6)
    for (const id of [3, 4, 5, 6]) pending.get(id)!.resolve({ success: true, latency_ms: 42 })
    await run
    expect(task.done).toBe(6)
    expect(task.healthyCount).toBe(4)
    expect(task.completed).toBe(true)
    expect(task.testingIDs.size).toBe(0)
  })

  it('discards old results and stops dispatching queued tests when the signed-in user changes', async () => {
    const pending: ((result: { success: boolean; message: string }) => void)[] = []
    vi.mocked(testProxy).mockImplementation(() => new Promise(done => { pending.push(done) }))
    const task = useClashNodeTestStore()
    const run = task.start([1, 2, 3, 4, 5].map(id => node(id)), String)
    const auth = useAuthStore()
    auth.user = { id: 20 } as User
    expect(task.testing).toBe(false)
    expect(task.total).toBe(0)
    for (const resolve of pending) resolve({ success: true, message: 'old session result' })
    await flushPromises()
    expect(testProxy).toHaveBeenCalledTimes(4)
    expect(task.results).toEqual({})
    await run
  })
})
