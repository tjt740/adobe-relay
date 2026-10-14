import { beforeEach, describe, expect, it, vi } from 'vitest'
import { bulkUpdate, exportData, list, listWithEtag, getUpstreamBillingRatesWithEtag } from '../accounts'
const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('../../client', () => ({ apiClient: { get, post } }))
const filters = { okad_account_type: 'company', status: 'request_timeout', capacity: 'full', concurrency_min: '2', concurrency_max: '8' }
beforeEach(() => { get.mockReset(); post.mockReset(); get.mockResolvedValue({ data: {}, headers: {} }); post.mockResolvedValue({ data: {} }) })
describe('account runtime filter transport', () => {
  it('preserves filters for list, refresh, billing snapshots and export', async () => {
    await list(2, 20, filters)
    await listWithEtag(2, 20, filters)
    await getUpstreamBillingRatesWithEtag(2, 20, filters)
    await exportData({ filters })
    for (const call of get.mock.calls) expect(call[1].params).toMatchObject(filters)
  })
  it('sends numeric JSON bounds for filtered bulk edits without modifying the snapshot', async () => {
    await bulkUpdate({ filters, priority: 10 })
    expect(post).toHaveBeenCalledWith('/admin/accounts/bulk-update', {
      filters: { ...filters, concurrency_min: 2, concurrency_max: 8 }, priority: 10
    })
    expect(filters.concurrency_min).toBe('2')
    await bulkUpdate({ filters: { capacity: 'idle', concurrency_min: '', concurrency_max: '' }, priority: 10 })
    expect(post).toHaveBeenLastCalledWith('/admin/accounts/bulk-update', { filters: { capacity: 'idle' }, priority: 10 })
  })
})
