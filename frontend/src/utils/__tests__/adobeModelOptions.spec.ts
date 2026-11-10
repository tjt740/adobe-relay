import { describe, expect, it } from 'vitest'
import { normalizeAdobeModelOptions, adobeFallbackModelOptions, parseAdobeWebsiteModelOptions } from '../adobeModelOptions'
import { adobeGroupNameProposal, adobeGroupNameModelIds } from '../adobeGroupName'
import { demoNameChanges } from '../../../dev/adobe-groups-data'

describe('Adobe model options', () => {
  it('prefers API names and keeps new/custom models', () => {
    expect(normalizeAdobeModelOptions([
      { id: 'gpt-image-2.6-flare', display_name: 'API display name' },
      { id: 'future-image', display_name: 'Future Image Pro' },
      { id: 'future-image', display_name: 'Duplicate' },
      null, { id: '' },
    ])).toEqual([
      { id: 'gpt-image-2.6-flare', display_name: 'API display name' },
      { id: 'future-image', display_name: 'Future Image Pro' },
    ])
  })
  it('maps ID-only older APIs without losing unknown IDs', () => {
    expect(normalizeAdobeModelOptions([{ id: 'gpt-image-2.6-flare' }, { id: 'my-model' }])).toEqual([
      { id: 'gpt-image-2.6-flare', display_name: 'GPT Image 2.6 Flare' },
      { id: 'my-model', display_name: 'my-model' },
    ])
    expect(normalizeAdobeModelOptions(null)).toEqual([])
    expect(adobeFallbackModelOptions.slice(0, 5).map(m => m.display_name)).toEqual([
      'GPT Image 2', 'GPT Image 2.5 Flare', 'GPT Image 2.5 Sunburst', 'GPT Image 2.6 Flare', 'GPT Image 2.6 Sunburst',
    ])
  })
  it('keeps both explicit legacy variants but does not guess an unspecified variant', () => {
    expect(adobeGroupNameModelIds('Adobe 2.6 - Flare/Sunburst - blue')).toEqual(['gpt-image-2.6-flare', 'gpt-image-2.6-sunburst'])
    expect(adobeGroupNameModelIds('adobe 2.6 张昊')).toEqual([])
    expect(adobeGroupNameModelIds('Adobe GPT Image 2.6 Flare - blue')).toEqual(['gpt-image-2.6-flare'])
    expect(adobeGroupNameModelIds('Adobe 2.6 - customer Flare')).toEqual([])
  })
  it('proposes only names whose model is unambiguous', () => {
    expect(demoNameChanges.filter(c => c.reason === 'clear')).toHaveLength(6)
    expect(adobeGroupNameProposal('Adobe 2 - Aapeli')).toEqual({ name: 'Adobe GPT Image 2 - Aapeli', reason: 'clear' })
    for (const name of ['Adobe 2.5 and 2 ----南小镇', 'adobe 2.6 张昊', 'Adobe 2.6 - Flare/Sunburst - blue']) {
      expect(adobeGroupNameProposal(name)).toEqual({ name, reason: 'ambiguous' })
    }
  })
})



describe('Adobe public discovery parsing', () => {
  const image = (name: string) => ({ enabled: true, outputModality: ['image'], modelDisplayName: name })
  it('uses official labels, maps existing GPT variant IDs, and accepts future image models', () => {
    const options = parseAdobeWebsiteModelOptions([
      { modelId: 'gpt-image', modelVersions: {
        '2': image('GPT Image 2'),
        'gpt-image-2.5-flare': image('GPT Image 2.5 Flare'),
        'gpt-image-2.5-prism': image('GPT Image 2.5 Sunburst'),
        'gpt-image-2.6-aurora': image('GPT Image 2.6 Sunburst'),
        'gpt-image-2.6-star-swirl': image('Adobe Latest Flare Name'),
        future: image('GPT Image Future'),
      } },
      { modelId: 'gemini-flash', modelVersions: { next: image('Gemini Latest') } },
    ])
    expect(options).toHaveLength(7)
    expect(options.map(({ id, display_name }) => ({ id, display_name }))).toContainEqual({ id: 'gpt-image-2.6-flare', display_name: 'Adobe Latest Flare Name' })
    expect(options.map(({ id, display_name }) => ({ id, display_name }))).toContainEqual({ id: 'gpt-image-2.5-sunburst', display_name: 'GPT Image 2.5 Sunburst' })
    expect(options.map(({ id, display_name }) => ({ id, display_name }))).toContainEqual({ id: 'gpt-image@future', display_name: 'GPT Image Future' })
    expect(options.map(({ id, display_name }) => ({ id, display_name }))).toContainEqual({ id: 'gemini-flash@next', display_name: 'Gemini Latest' })
  })
  it('excludes disabled, deprecated, non-image and malformed entries, and deduplicates labels', () => {
    expect(parseAdobeWebsiteModelOptions([
      null, {}, { modelId: '', modelVersions: {} }, { modelId: 'bad', modelVersions: [] },
      { modelId: 'family', modelVersions: {
        current: image('Current Image'), duplicate: image('Current Image'),
        disabled: { ...image('Disabled'), enabled: false },
        deprecated: { ...image('Deprecated'), isDeprecated: true },
        video: { ...image('Video'), outputModality: ['video'] },
        missing: { modelDisplayName: 'Missing flags' }, invalid: null,
        blank: image(' '),
      } },
    ] ).map(({ id, display_name }) => ({ id, display_name }))).toEqual([{ id: 'family@current', display_name: 'Current Image' }])
  })
})
