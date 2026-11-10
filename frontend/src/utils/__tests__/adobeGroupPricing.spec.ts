import { describe, expect, it } from 'vitest'
import { adobeQualityPricePayload } from '../adobeGroupPricing'

describe('independent Adobe pricing', () => {
  const prices = {
    'gpt-image-2.6-flare': { '4K': { max: 0.5 } },
    'gpt-image-2.6-sunburst': { '1K': { low: 0 } },
  }
  it('saves both drafts, including zero, without sharing mutable values', () => {
    const result = adobeQualityPricePayload(prices)!
    expect(result).toEqual(prices)
    result['gpt-image-2.6-flare']['4K'].max = 9
    expect(prices['gpt-image-2.6-flare']['4K'].max).toBe(0.5)
  })
  it('stores drafts independently of the billing switch and only clears an explicitly empty draft', () => {
    expect(adobeQualityPricePayload(prices)).toEqual(prices)
    expect(adobeQualityPricePayload({})).toEqual({})
  })
  it('saves only selected models on creation while retaining all editing data by default', () => {
    expect(adobeQualityPricePayload(prices, ['gpt-image-2.6-sunburst'])).toEqual({ 'gpt-image-2.6-sunburst': prices['gpt-image-2.6-sunburst'] })
    expect(adobeQualityPricePayload(prices, [])).toEqual({})
    expect(adobeQualityPricePayload(prices)).toEqual(prices)
  })

})
