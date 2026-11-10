import { describe, expect, it } from 'vitest'
import { adobeFallbackModelOptions, parseAdobeWebsiteModelOptions, normalizeAdobeModelOptions } from '../adobeModelOptions'
import { adobeWebsiteSnapshot, readAdobeModelCapabilities } from '../adobeModelCapabilities'
import { adobePricingModelsForName } from '../adobePricingModels'

const options = parseAdobeWebsiteModelOptions(adobeWebsiteSnapshot.models, '2026-11-11T01:00:00Z')
const byName = (name: string) => options.find(m => m.display_name === name)!.capabilities!
describe('actual Adobe website capability metadata', () => {
  it('distinguishes Image 2 default sizing from the 2.5 / 2.6 resolution selectors', () => {
    expect(byName('GPT Image 2')).toMatchObject({ resolutionMode: 'default', resolutions: ['1K'], qualities: [
      { id: 'low', label: 'Low' }, { id: 'medium', label: 'Medium' }, { id: 'high', label: 'High' },
    ] })
    for (const name of ['GPT Image 2.5 Flare', 'GPT Image 2.5 Sunburst', 'GPT Image 2.6 Flare', 'GPT Image 2.6 Sunburst']) {
      expect(byName(name).resolutions).toEqual(['1K', '2K', '4K'])
      expect(byName(name).qualities.map(q => q.label)).toEqual(['Low', 'Medium', 'High', 'Extra High', 'Max'])
    }
  })
  it('keeps explicit non-GPT tiers and does not invent quality choices', () => {
    for (const [name, sizes] of [
      ['Firefly Image 4', ['1K', '2K']], ['Firefly Image 5', ['1K', '1.5K', '2K']],
      ['HUMAIN Image 1', ['2K']], ['Gemini 3.1 (with Nano Banana 2)', ['512', '1K', '2K', '4K']],
    ] as const) {
      expect(byName(name)).toMatchObject({ resolutions: sizes, qualities: [], qualityMode: 'none' })
    }
    expect(byName('Flux 2').resolutionMode).toBe('unknown')
  })
  it('uses new API enums and bounds instead of the snapshot matrix', () => {
    const schema = { properties: {
      resolution: { anyOf: [{ enum: ['2K', '8K'] }, { type: 'null' }] },
      generationSettings: { properties: { detailLevel: { minimum: 3, maximum: 6 } } },
    } }
    const result = readAdobeModelCapabilities(schema, 'gpt-image', 'gpt-image-2.6-star-swirl')
    expect(result.resolutions).toEqual(['2K', '8K'])
    expect(result.qualities.map(q => q.id)).toEqual(['medium', 'high', 'xhigh'])
  })
  it('does not mistake missing or unresolved metadata for no quality control', () => {
    expect(readAdobeModelCapabilities(undefined, 'gpt-image', '2').qualityMode).toBe('unknown')
    const schema = { properties: { generationSettings: { $ref: '#/$defs/settings' }, resolution: { $ref: '#/$defs/res' } } }
    expect(readAdobeModelCapabilities(schema, 'custom', 'next').qualityMode).toBe('unknown')
    const result = readAdobeModelCapabilities(schema, 'custom', 'next', { $defs: {
      settings: { properties: { detailLevel: { anyOf: [{ minimum: 1, maximum: 5 }, { type: 'null' }] } } }, res: { enum: ['1.5K'] },
    } })
    expect(result.resolutions).toEqual(['1.5K'])
    expect(result.qualities).toHaveLength(3)
  })
  it('preserves capabilities through normalization and editing, and marks fallback honestly', () => {
    expect(normalizeAdobeModelOptions(options)).toEqual(options)
    const selected = adobePricingModelsForName('Adobe GPT Image 2 + Firefly Image 5 - preview', options)
    expect(selected.map(m => m.capabilities)).toEqual([byName('GPT Image 2'), byName('Firefly Image 5')])
    expect(adobeFallbackModelOptions.every(m => m.capabilities?.source === 'verified-fallback' && m.capabilities.checkedAt === '2026-11-11')).toBe(true)
  })
  it('honors local reference overrides and stops cyclic references', () => {
    const schema = { properties: { resolution: { $ref: '#/$defs/res', enum: ['2K'] } } }
    const common = { $defs: { res: { enum: ['1K', '4K'] }, cycle: { $ref: '#/$defs/cycle' } } }
    expect(readAdobeModelCapabilities(schema, 'custom', 'next', common).resolutions).toEqual(['2K'])
    expect(readAdobeModelCapabilities({ $ref: '#/$defs/cycle' }, 'custom', 'next', common).qualityMode).toBe('unknown')
  })

})
