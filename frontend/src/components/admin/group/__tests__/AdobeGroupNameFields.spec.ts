import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import { adobeFallbackModelOptions } from '@/utils/adobeModelOptions'
import { nextTick } from 'vue'
import AdobeGroupNameFields from '../AdobeGroupNameFields.vue'

const mountFields = (props = {}) => mount(AdobeGroupNameFields, {
  props: { modelValue: '', ...props },
  global: { plugins: [createI18n({ legacy: false, locale: 'zh', missingWarn: false, fallbackWarn: false, messages: { zh: {} } })] }
})
const value = (w: ReturnType<typeof mountFields>) => w.emitted('update:modelValue')?.at(-1)?.[0]
describe('Adobe group name creation', () => {
  it('uses API labels and lets the user enter a custom model without losing the customer', async () => {
    const w = mountFields({ modelOptions: [{ id: 'api-new', display_name: 'API New Model' }], modelSource: 'adobe' })
    await w.get('#adobe-name-customer').setValue('blue')
    expect(value(w)).toBe('Adobe API New Model - blue')
    expect(w.findComponent(Select).props('creatable')).toBe(true)
    w.findComponent(Select).vm.$emit('change', 'My Custom Image')
    await nextTick()
    expect(value(w)).toBe('Adobe API New Model + My Custom Image - blue')
    await w.setProps({ modelOptions: [], modelSource: 'fallback' })
    expect(value(w)).toBe('Adobe API New Model + My Custom Image - blue')
  })

  it('retains the selected Adobe display name if a later lookup fails', async () => {
    const w = mountFields({ modelOptions: [{ id: 'family@next', display_name: 'Future Adobe Image' }], modelSource: 'adobe' })
    await w.get('#adobe-name-customer').setValue('preview')
    expect(value(w)).toBe('Adobe Future Adobe Image - preview')
    await w.setProps({ modelOptions: [], modelSource: 'fallback' })
    expect(value(w)).toBe('Adobe Future Adobe Image - preview')
  })

  it('treats object-property names as ordinary custom input', async () => {
    const w = mountFields()
    w.findComponent(Select).vm.$emit('change', 'constructor')
    await w.get('#adobe-name-customer').setValue('preview')
    expect(value(w)).toBe('Adobe GPT Image 2 + constructor - preview')
  })

  it('requires a customer and composes a trimmed default name', async () => {
    const w = mountFields()
    expect(value(w)).toBe('')
    await w.get('#adobe-name-customer').setValue(' 杂乱无章 ')
    expect(value(w)).toBe('Adobe GPT Image 2 - 杂乱无章')
    await nextTick()
    await w.get('#adobe-name-customer').setValue('低中质量 124K 全参分组 - 杂乱无章')
    expect(value(w)).toBe('Adobe GPT Image 2 - 低中质量 124K 全参分组 - 杂乱无章')
    await w.get('#adobe-name-customer').setValue('   ')
    expect(value(w)).toBe('')
  })
  it('supports a group label or customer in the same name field', async () => {
    const w = mountFields({ initialModel: 'gpt-image-2.5-sunburst' })
    await w.get('#adobe-name-customer').setValue('6分分组')
    expect(value(w)).toBe('Adobe GPT Image 2.5 Sunburst - 6分分组')
    await w.get('#adobe-name-customer').setValue('')
    expect(value(w)).toBe('')
    await w.get('#adobe-name-customer').setValue('Aapeli')
    expect(value(w)).toBe('Adobe GPT Image 2.5 Sunburst - Aapeli')
  })
  it('preserves the full name suffix when restoring an existing draft', async () => {
    const w = mountFields()
    await w.get('#adobe-name-customer').setValue(' 高质量 4K - blue ')
    expect(value(w)).toBe('Adobe GPT Image 2 - 高质量 4K - blue')
    const draft = String(value(w))
    w.unmount()
    const restored = mountFields({ modelValue: draft })
    expect(value(restored)).toBe(draft)
    expect((restored.get('#adobe-name-customer').element as HTMLInputElement).value).toBe('高质量 4K - blue')
  })
  it('toggles models without losing the customer and restores a multi-model draft', async () => {
    const w = mountFields()
    await w.get('#adobe-name-customer').setValue('blue')
    w.findComponent(Select).vm.$emit('change', 'gpt-image-2.5-flare')
    await nextTick()
    expect(value(w)).toBe('Adobe GPT Image 2 + GPT Image 2.5 Flare - blue')
    expect(w.findComponent(Select).props('closeOnSelect')).toBe(false)
    const restored = mountFields({ modelValue: String(value(w)) })
    expect(restored.findComponent(Select).props('selectedValues')).toEqual(['gpt-image-2', 'gpt-image-2.5-flare'])
    restored.findComponent(Select).vm.$emit('change', 'gpt-image-2')
    await nextTick()
    expect(value(restored)).toBe('Adobe GPT Image 2.5 Flare - blue')
    restored.findComponent(Select).vm.$emit('change', 'gpt-image-2.5-flare')
    await nextTick()
    expect(value(restored)).toBe('')
    expect(restored.get<HTMLInputElement>('#adobe-name-customer').element.validationMessage).toBeTruthy()
  })

  it('keeps all five model names and prevents an oversized combined name', async () => {
    const w = mountFields()
    const select = w.findComponent(Select)
    for (const model of ['gpt-image-2.5-flare', 'gpt-image-2.5-sunburst', 'gpt-image-2.6-flare', 'gpt-image-2.6-sunburst']) select.vm.$emit('change', model)
    await w.get('#adobe-name-customer').setValue('preview')
    expect(String(value(w)).length).toBeGreaterThan(100)
    expect(select.props('selectedValues')).toHaveLength(5)
    select.vm.$emit('change', '模型'.repeat(100))
    await nextTick()
    expect(value(w)).toBe('')
    expect(w.get<HTMLInputElement>('#adobe-name-customer').element.validationMessage).toBeTruthy()
  })

  it('passes capability metadata to the pricing tabs and retains it across selection changes', async () => {
    const model = adobeFallbackModelOptions[1]
    const w = mountFields({ modelOptions: [model], modelSource: 'fallback' })
    expect(w.emitted('update:selectedModels')?.at(-1)?.[0]).toEqual([model])
    w.findComponent(Select).vm.$emit('change', 'My Custom Model')
    await nextTick()
    expect(w.emitted('update:selectedModels')?.at(-1)?.[0]).toEqual([model, { id: 'My Custom Model', display_name: 'My Custom Model' }])
  })

})
