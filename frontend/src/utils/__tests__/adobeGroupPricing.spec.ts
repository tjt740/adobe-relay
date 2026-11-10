import { describe, expect, it } from 'vitest'
import { adobeQualityPricePayload } from '../adobeGroupPricing'

describe('independent Adobe pricing', () => {
  const prices = {
    'gpt-image-2.6-flare': { '4K': { max: 0.5 } },
    'gpt-image-2.6-sunburst': { '1K': { low: 0 } },
  }
  it('saves both drafts, including zero, without sharing mutable values', () => {
    const result = adobeQualityPricePayload(prices, true)!
    expect(result).toEqual(prices)
    result['gpt-image-2.6-flare']['4K'].max = 9
    expect(prices['gpt-image-2.6-flare']['4K'].max).toBe(0.5)
  })
  it('omits the field when closed and explicitly clears when open and empty', () => {
    expect(adobeQualityPricePayload(prices, false)).toBeUndefined()
    expect(adobeQualityPricePayload({}, true)).toEqual({})
  })
})
