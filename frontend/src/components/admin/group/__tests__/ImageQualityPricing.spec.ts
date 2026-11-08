import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ImageQualityPricing from '../ImageQualityPricing.vue'
import type { ImageQualityPrices } from '@/types'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('Adobe model quality pricing', () => {
  it('follows the parent model without an independent selector and preserves other drafts', async () => {
    const original = { 'gpt-image-2.6-sunburst': { '2K': { max: 0.25 } } }
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: original, model: 'gpt-image-2.6-flare' } })
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.findAll('input')).toHaveLength(15)
    await wrapper.get('input[aria-label="GPT Image 2.6 Flare 4K Max"]').setValue('0')
    const saved = wrapper.emitted('update:modelValue')!.at(-1)![0] as ImageQualityPrices
    await wrapper.setProps({ modelValue: saved })
    await wrapper.setProps({ model: 'gpt-image-2.6-sunburst' })
    expect(wrapper.findAll('input')).toHaveLength(15)
    expect((wrapper.get('input[aria-label="GPT Image 2.6 Sunburst 2K Max"]').element as HTMLInputElement).value).toBe('0.25')
    expect(saved['gpt-image-2.6-flare']['4K'].max).toBe(0)
  })

  it.each([
    ['gpt-image-2', 9], ['gpt-image-2.5-flare', 15], ['gpt-image-2.5-sunburst', 15],
    ['gpt-image-2.6-flare', 15], ['gpt-image-2.6-sunburst', 15],
  ])('shows the website-supported cells for %s', (model, cells) => {
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: {}, model: String(model) } })
    expect(wrapper.findAll('input')).toHaveLength(Number(cells))
    expect(wrapper.text().includes('Extra High')).toBe(model !== 'gpt-image-2')
    expect(wrapper.text().includes('Max')).toBe(model !== 'gpt-image-2')
    expect(wrapper.findAll('thead th').map(th => th.text()).slice(1)).toEqual(['1K ($)', '2K ($)', '4K ($)'])
  })
  it('offers only the selected model and keeps blank prices unset', () => {
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: {}, model: 'gpt-image-2.6-sunburst' } })
    expect(wrapper.findAll('input')).toHaveLength(15)
    for (const input of wrapper.findAll('input')) expect(input.element.value).toBe('')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
  it('preserves zero and other cells; clearing restores inheritance', async () => {
    const original = { 'gpt-image-2.6-sunburst': { '2K': { xhigh: 0.25 } } }
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: original, model: 'gpt-image-2.6-flare' } })
    const input = wrapper.get('input[aria-label="GPT Image 2.6 Flare 4K Max"]')
    await input.setValue('0')
    const saved = wrapper.emitted('update:modelValue')!.at(-1)![0] as ImageQualityPrices
    expect(saved).toEqual({ ...original, 'gpt-image-2.6-flare': { '4K': { max: 0 } } })
    expect(original).not.toHaveProperty('gpt-image-2.6-flare')
    await wrapper.setProps({ modelValue: saved })
    expect((input.element as HTMLInputElement).value).toBe('0')
    await input.setValue('')
    expect(wrapper.emitted('update:modelValue')!.at(-1)![0]).toEqual(original)
  })
  it('roundtrips each model/resolution/quality without leaking into another cell', async () => {
    const models = ['gpt-image-2', 'gpt-image-2.5-flare', 'gpt-image-2.5-sunburst', 'gpt-image-2.6-flare', 'gpt-image-2.6-sunburst']
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: {} as ImageQualityPrices, model: models[0] } })
    for (const [m, model] of models.entries()) {
      await wrapper.setProps({ model })
      for (const [i, input] of wrapper.findAll('input').entries()) {
        await input.setValue(String((m * 15 + i + 1) / 10))
        await wrapper.setProps({ modelValue: wrapper.emitted('update:modelValue')!.at(-1)![0] as ImageQualityPrices })
      }
    }
    const stored = JSON.parse(JSON.stringify(wrapper.props('modelValue')))
    for (const [m, model] of models.entries()) {
      const reloaded = mount(ImageQualityPricing, { props: { modelValue: stored, model } })
      reloaded.findAll('input').forEach((input, i) => expect(Number(input.element.value)).toBe((m * 15 + i + 1) / 10))
    }
  })
  it.each(['', 'My Custom Model'])('never falls back to an unrelated model table for %s', model => {
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: {}, model } })
    expect(wrapper.find('[data-testid="image-quality-pricing"]').exists()).toBe(false)
    expect(wrapper.findAll('input')).toHaveLength(0)
  })
})
