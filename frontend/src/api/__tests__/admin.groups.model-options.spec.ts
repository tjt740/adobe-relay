import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))
import { getAdobeWebsiteModelOptions } from '@/api/admin/groups'

describe('Adobe website model directory API', () => {
  beforeEach(() => { get.mockReset() })

  it('reads Adobe discovery through the dedicated relay and keeps Adobe display names', async () => {
    get.mockResolvedValue({ data: { source: 'adobe', models: [{ modelId: 'future', modelVersions: {
      next: { enabled: true, outputModality: ['image'], modelDisplayName: 'Adobe Latest Name' }
    } }] } })
    expect(await getAdobeWebsiteModelOptions()).toEqual([{ id: 'future@next', display_name: 'Adobe Latest Name', capabilities: { source: 'adobe', checkedAt: '', resolutions: [], resolutionMode: 'unknown', qualities: [], qualityMode: 'unknown' } }])
    expect(get).toHaveBeenCalledWith('/admin/groups/adobe-models', { timeout: 10000 })
  })

  it('does not mislabel the Sub2 built-in catalog as Adobe discovery', async () => {
    for (const data of [{ models: ['gpt-image-2'] }, { source: 'sub2', models: [] }]) {
      get.mockResolvedValueOnce({ data })
      await expect(getAdobeWebsiteModelOptions()).rejects.toThrow('Missing Adobe discovery source')
    }
  })

  it('returns empty for unusable Adobe directories so the caller can fall back', async () => {
    for (const models of [undefined, [], 'bad response']) {
      get.mockResolvedValueOnce({ data: { source: 'adobe', models } })
      expect(await getAdobeWebsiteModelOptions()).toEqual([])
    }
  })

  it('propagates request errors for the fallback path', async () => {
    get.mockRejectedValue(new Error('unavailable'))
    await expect(getAdobeWebsiteModelOptions()).rejects.toThrow('unavailable')
  })
})
