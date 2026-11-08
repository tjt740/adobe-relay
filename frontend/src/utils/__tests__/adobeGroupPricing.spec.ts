import { describe, expect, it } from 'vitest'
import { adobeQualityModel, adobeGroupQualityModel, adobeQualityPricePayload } from '../adobeGroupPricing'
import { adobeFallbackModelOptions } from '../adobeModelOptions'

const options = adobeFallbackModelOptions
const flare = adobeQualityModel('gpt-image-2.6-flare', options)!
const sunburst = adobeQualityModel('GPT Image 2.6 Sunburst', options)!

describe('model-bound Adobe pricing', () => {
  it('resolves the selected 2.6 variant and keeps the API display name', () => {
    expect(flare.id).toBe('gpt-image-2.6-flare')
    expect(sunburst.id).toBe('gpt-image-2.6-sunburst')
    expect(adobeQualityModel('gpt-image-2.6-flare', [{ id: flare.id, display_name: 'Latest Adobe Flare' }])?.display_name).toBe('Latest Adobe Flare')
    expect(adobeGroupQualityModel('Adobe Latest Adobe Flare - preview', [{ id: flare.id, display_name: 'Latest Adobe Flare' }])?.id).toBe(flare.id)
  })
  it.each(['custom', 'GPT Image 2.6 Flare/Sunburst'])('does not substitute a 2.6 model for %s', value => {
    expect(adobeQualityModel(value, options)).toBeUndefined()
  })
  it.each(options)('resolves every supported model $id', model => {
    expect(adobeQualityModel(model.id, options)?.id).toBe(model.id)
    expect(adobeGroupQualityModel(`Adobe ${model.display_name} - preview`, options)?.id).toBe(model.id)
  })
  it('identifies clear edit names and refuses ambiguous variants', () => {
    expect(adobeGroupQualityModel('Adobe GPT Image 2.6 Sunburst - preview', options)?.id).toBe(sunburst.id)
    expect(adobeGroupQualityModel('Adobe 2.6 Flare - preview', options)?.id).toBe(flare.id)
    for (const name of ['Adobe 2.6 - Flare/Sunburst - preview', 'Adobe 2.6 - preview', 'Adobe 2.5 - Flare/Sunburst - preview', 'Adobe Custom - GPT Image 2.6 Flare']) {
      expect(adobeGroupQualityModel(name, options)).toBeUndefined()
    }
  })
  it('creates only the current model price and preserves zero', () => {
    const draft = { [flare.id]: { '4K': { max: 0.5 } }, [sunburst.id]: { '1K': { low: 0 } } }
    expect(adobeQualityPricePayload(draft, sunburst, true)).toEqual({ [sunburst.id]: { '1K': { low: 0 } } })
    expect(adobeQualityPricePayload(draft, undefined, true)).toBeUndefined()
    expect(adobeQualityPricePayload(draft, sunburst, false)).toBeUndefined()
  })
  it('retains other saved models but excludes their unsaved changes, including on clear', () => {
    const saved = { [flare.id]: { '4K': { max: 0.5 } }, [sunburst.id]: { '1K': { low: 0.1 } } }
    const draft = { [flare.id]: { '4K': { max: 9 } }, [sunburst.id]: { '1K': { low: 0 } } }
    expect(adobeQualityPricePayload(draft, sunburst, true, saved)).toEqual({
      [flare.id]: { '4K': { max: 0.5 } }, [sunburst.id]: { '1K': { low: 0 } },
    })
    expect(adobeQualityPricePayload({}, sunburst, true, saved)).toEqual({ [flare.id]: { '4K': { max: 0.5 } } })
    expect(saved[sunburst.id]['1K'].low).toBe(0.1)
  })
})
