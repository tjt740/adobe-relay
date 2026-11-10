import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ImageQualityPricing from '../ImageQualityPricing.vue'
import type { ImageQualityPrices } from '@/types'
import { adobeFallbackModelOptions } from '@/utils/adobeModelOptions'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const models = adobeFallbackModelOptions.slice(0, 5)
const tab = (id: string) => `[role="tab"][data-model="${id}"]`

describe('multi-model quality pricing tabs', () => {
  it('roundtrips all 63 website cells with four decimals and keeps the active table independent', async () => {
    const w = mount(ImageQualityPricing, { props: { modelValue: {} as ImageQualityPrices, models } })
    expect(w.findAll('[role="tab"]')).toHaveLength(5)
    for (const [m, model] of models.entries()) {
      await w.get(tab(model.id)).trigger('click')
      expect(w.findAll('input')).toHaveLength(m === 0 ? 3 : 15)
      expect(w.findAll('input').every(i => i.element.value === '')).toBe(true)
      for (const [i, input] of w.findAll('input').entries()) {
        expect(input.attributes('step')).toBe('0.0001')
        await input.setValue(String((m * 15 + i) / 10000))
        await w.setProps({ modelValue: w.emitted('update:modelValue')!.at(-1)![0] as ImageQualityPrices })
      }
    }
    const restored = mount(ImageQualityPricing, { props: { models, modelValue: JSON.parse(JSON.stringify(w.props('modelValue'))) } })
    for (const [m, model] of models.entries()) {
      await restored.get(tab(model.id)).trigger('click')
      restored.findAll('input').forEach((input, i) => expect(input.element.value).toBe(((m * 15 + i) / 10000).toFixed(4)))
    }
    await restored.get('input[aria-label="GPT Image 2.6 Sunburst 4K Max"]').setValue('')
    const cleared = restored.emitted('update:modelValue')!.at(-1)![0] as ImageQualityPrices
    expect(cleared['gpt-image-2.6-sunburst']['4K'].max).toBeUndefined()
    expect(cleared['gpt-image-2']['1K'].low).toBe(0)
  })
  it('supports keyboard tabs and returns to a remaining tab without clearing draft prices', async () => {
    const prices = { 'gpt-image-2.5-flare': { '1K': { low: 0.1234 } } }
    const w = mount(ImageQualityPricing, { props: { models: models.slice(0, 2), modelValue: prices } })
    await w.get(tab('gpt-image-2')).trigger('keydown', { key: 'ArrowRight' })
    expect(w.get(tab('gpt-image-2.5-flare')).attributes('aria-selected')).toBe('true')
    await w.setProps({ models: models.slice(0, 1) })
    expect(w.get(tab('gpt-image-2')).attributes('aria-selected')).toBe('true')
    await w.setProps({ models: models.slice(0, 2) })
    await w.get(tab('gpt-image-2.5-flare')).trigger('click')
    expect(w.get<HTMLInputElement>('input[aria-label="GPT Image 2.5 Flare 1K Low"]').element.value).toBe('0.1234')
  })
  it('does not invent quality pricing for custom or unsupported models', () => {
    const w = mount(ImageQualityPricing, { props: { models: [{ id: 'Custom', display_name: 'Custom' }], modelValue: {} } })
    expect(w.findAll('[role="tab"]')).toHaveLength(1)
    expect(w.findAll('input')).toHaveLength(0)
    expect(w.text()).toContain('admin.groups.imageQualityPricing.unsupported')
  })
  it('uses live tiers, preserves hidden saved prices and does not offer unsupported new cells', async () => {
    const model = { ...models[1], capabilities: { ...models[1].capabilities!, source: 'adobe' as const, resolutions: ['2K', '8K'], qualities: [{ id: 'medium', label: 'Medium' }] } }
    const prices = { [model.id]: { '1K': { max: 0.5678 } } }
    const w = mount(ImageQualityPricing, { props: { models: [model], modelValue: prices } })
    expect(w.findAll('input')).toHaveLength(1)
    expect(w.text()).toContain('8K')
    await w.get('input').setValue('0.1234')
    expect(w.emitted('update:modelValue')!.at(-1)![0]).toEqual({ [model.id]: { '1K': { max: 0.5678 }, '2K': { medium: 0.1234 } } })
  })
  it('shows real resolution options without inventing quality columns for Firefly', () => {
    const model = adobeFallbackModelOptions.find(m => m.display_name === 'Firefly Image 5')!
    const w = mount(ImageQualityPricing, { props: { models: [model], modelValue: {} } })
    expect(w.text()).toContain('1K / 1.5K / 2K')
    expect(w.text()).toContain('admin.groups.imageQualityPricing.noQuality')
    expect(w.findAll('input')).toHaveLength(0)
    expect(w.text()).not.toContain('Extra High')
  })

})
