import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
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
    w.findComponent(Select).vm.$emit('update:modelValue', 'My Custom Image')
    w.findComponent(Select).vm.$emit('change', 'My Custom Image')
    await nextTick()
    expect(value(w)).toBe('Adobe My Custom Image - blue')
    await w.setProps({ modelOptions: [], modelSource: 'fallback' })
    expect(value(w)).toBe('Adobe My Custom Image - blue')
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
    w.findComponent(Select).vm.$emit('update:modelValue', 'constructor')
    w.findComponent(Select).vm.$emit('change', 'constructor')
    await w.get('#adobe-name-customer').setValue('preview')
    expect(value(w)).toBe('Adobe constructor - preview')
  })

  it('requires a customer and composes a trimmed default name', async () => {
    const w = mountFields()
    expect(value(w)).toBe('')
    await w.get('#adobe-name-customer').setValue(' 杂乱无章 ')
    expect(value(w)).toBe('Adobe GPT Image 2 - 杂乱无章')
    w.findComponent(Select).vm.$emit('update:modelValue', 'gpt-image-2')
    await nextTick()
    await w.get('#adobe-name-category').setValue('低中质量 124K 全参分组')
    expect(value(w)).toBe('Adobe GPT Image 2 - 低中质量 124K 全参分组 - 杂乱无章')
    await w.get('#adobe-name-customer').setValue('   ')
    expect(value(w)).toBe('')
  })
  it('supports the six-cent group with an optional customer and ordinary groups', async () => {
    const w = mountFields({ initialModel: 'gpt-image-2.5-sunburst' })
    await w.get('#adobe-name-category').setValue('6分分组')
    expect(value(w)).toBe('Adobe GPT Image 2.5 Sunburst - 6分分组')
    await w.get('#adobe-name-category').setValue('')
    expect(value(w)).toBe('')
    await w.get('#adobe-name-customer').setValue('Aapeli')
    expect(value(w)).toBe('Adobe GPT Image 2.5 Sunburst - Aapeli')
  })
  it('requires the custom type and restores an existing draft after remount', async () => {
    const w = mountFields()
    await w.get('#adobe-name-category').setValue('custom')
    await w.get('#adobe-name-customer').setValue('blue')
    expect(value(w)).toBe('')
    await w.get('#adobe-name-custom').setValue(' 高质量 4K ')
    expect(value(w)).toBe('Adobe GPT Image 2 - 高质量 4K - blue')
    const draft = String(value(w))
    w.unmount()
    const restored = mountFields({ modelValue: draft })
    expect(value(restored)).toBe(draft)
    expect((restored.get('#adobe-name-custom').element as HTMLInputElement).value).toBe('高质量 4K')
  })
})
