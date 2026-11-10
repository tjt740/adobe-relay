import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ImageQualityPricing from '../ImageQualityPricing.vue'
import type { ImageQualityPrices } from '@/types'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('independent Image 2.6 pricing', () => {
  it('switches between both models, preserves drafts and never pre-fills a price', async () => {
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: {} } })
    const select = wrapper.get('[data-testid="quality-pricing-model"]')
    expect(wrapper.findAll('option').map(o => o.element.value)).toEqual(['gpt-image-2.6-flare', 'gpt-image-2.6-sunburst'])
    expect(wrapper.findAll('input')).toHaveLength(15)
    expect(wrapper.findAll('input').every(input => input.element.value === '')).toBe(true)
    await wrapper.get('input[aria-label="GPT Image 2.6 Flare 4K Max"]').setValue('0')
    await wrapper.setProps({ modelValue: wrapper.emitted('update:modelValue')!.at(-1)![0] as ImageQualityPrices })
    await select.setValue('gpt-image-2.6-sunburst')
    await wrapper.get('input[aria-label="GPT Image 2.6 Sunburst 2K Extra High"]').setValue('0.25')
    await wrapper.setProps({ modelValue: wrapper.emitted('update:modelValue')!.at(-1)![0] as ImageQualityPrices })
    await select.setValue('gpt-image-2.6-flare')
    expect(wrapper.get<HTMLInputElement>('input[aria-label="GPT Image 2.6 Flare 4K Max"]').element.value).toBe('0')
    await wrapper.get('input[aria-label="GPT Image 2.6 Flare 4K Max"]').setValue('')
    expect(wrapper.emitted('update:modelValue')!.at(-1)![0]).toEqual({ 'gpt-image-2.6-sunburst': { '2K': { xhigh: 0.25 } } })
  })
  it('roundtrips all 30 cells without overwriting either model', async () => {
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: {} as ImageQualityPrices } })
    const models = ['gpt-image-2.6-flare', 'gpt-image-2.6-sunburst']
    for (const [m, model] of models.entries()) {
      await wrapper.get('select').setValue(model)
      for (const [i, input] of wrapper.findAll('input').entries()) {
        await input.setValue(String((m * 15 + i) / 10))
        await wrapper.setProps({ modelValue: wrapper.emitted('update:modelValue')!.at(-1)![0] as ImageQualityPrices })
      }
    }
    const reloaded = mount(ImageQualityPricing, { props: { modelValue: JSON.parse(JSON.stringify(wrapper.props('modelValue'))) } })
    for (const [m, model] of models.entries()) {
      await reloaded.get('select').setValue(model)
      expect(reloaded.findAll('input')).toHaveLength(15)
      reloaded.findAll('input').forEach((input, i) => expect(Number(input.element.value)).toBe((m * 15 + i) / 10))
    }
  })
})
