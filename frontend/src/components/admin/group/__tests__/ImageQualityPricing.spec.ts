import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ImageQualityPricing from '../ImageQualityPricing.vue'
import type { ImageQualityPrices } from '@/types'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('Image 2.6 quality pricing', () => {
  it('offers all 30 cells and keeps blank prices unset', () => {
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: {} } })
    expect(wrapper.findAll('input')).toHaveLength(30)
    for (const input of wrapper.findAll('input')) expect(input.element.value).toBe('')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
  it('preserves zero and other cells; clearing restores inheritance', async () => {
    const original = { 'gpt-image-2.6-sunburst': { '2K': { xhigh: 0.25 } } }
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: original } })
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
    const wrapper = mount(ImageQualityPricing, { props: { modelValue: {} as ImageQualityPrices } })
    for (const [i, input] of wrapper.findAll('input').entries()) {
      await input.setValue(String((i + 1) / 10))
      await wrapper.setProps({ modelValue: wrapper.emitted('update:modelValue')!.at(-1)![0] as ImageQualityPrices })
    }
    const stored = JSON.parse(JSON.stringify(wrapper.props('modelValue')))
    const reloaded = mount(ImageQualityPricing, { props: { modelValue: stored } })
    reloaded.findAll('input').forEach((input, i) => expect(Number(input.element.value)).toBe((i + 1) / 10))
  })
})
