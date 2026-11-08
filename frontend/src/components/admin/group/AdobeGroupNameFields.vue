<template>
  <div class="space-y-3" data-testid="adobe-group-name-fields">
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <div>
        <label for="adobe-name-model" class="input-label">{{ t('admin.groups.adobeNaming.model') }}</label>
        <Select id="adobe-name-model" v-model="model" :options="options" :searchable="true" creatable
          :creatable-prefix="t('admin.groups.adobeNaming.useCustom')" :aria-label="t('admin.groups.adobeNaming.model')"
          :search-placeholder="t('admin.groups.adobeNaming.modelSearch')" @change="modelTouched = true" />
        <p class="input-hint">{{ t('admin.groups.adobeNaming.source.' + (modelSource || 'fallback')) }}</p>
      </div>
      <div>
        <label for="adobe-name-category" class="input-label">{{ t('admin.groups.adobeNaming.category') }}</label>
        <select id="adobe-name-category" v-model="category" class="input">
          <option value="低中质量 124K 全参分组">{{ t('admin.groups.adobeNaming.lowMedium') }}</option>
          <option value="低中质量 124K 全参分组（7分）">{{ t('admin.groups.adobeNaming.lowMediumSeven') }}</option>
          <option value="6分分组">{{ t('admin.groups.adobeNaming.sixCent') }}</option>
          <option value="">{{ t('admin.groups.adobeNaming.noCategory') }}</option>
          <option value="custom">{{ t('admin.groups.adobeNaming.custom') }}</option>
        </select>
      </div>
    </div>
    <div v-if="category === 'custom'">
      <label for="adobe-name-custom" class="input-label">{{ t('admin.groups.adobeNaming.customCategory') }}</label>
      <input id="adobe-name-custom" v-model="customCategory" class="input" required maxlength="60"
        :placeholder="t('admin.groups.adobeNaming.customPlaceholder')" />
    </div>
    <div>
      <label for="adobe-name-customer" class="input-label">{{ t('admin.groups.adobeNaming.customer') }}
        <span v-if="category === '6分分组'" class="font-normal text-gray-400">({{ t('admin.groups.adobeNaming.optional') }})</span>
      </label>
      <input id="adobe-name-customer" v-model="customer" class="input" :required="category !== '6分分组'" maxlength="100"
        :placeholder="t('admin.groups.adobeNaming.customerPlaceholder')" />
    </div>
    <div class="rounded-xl border border-primary-100 bg-primary-50/60 px-4 py-3 dark:border-primary-800 dark:bg-primary-900/15">
      <p class="mb-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.adobeNaming.preview') }}</p>
      <p class="break-words text-sm font-medium text-gray-900 dark:text-gray-100">{{ preview }}</p>
    </div>
    <p class="input-hint">{{ t('admin.groups.adobeNaming.hint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adobeModels, formatAdobeGroupName, parseAdobeGroupName } from '@/utils/adobeGroupName'
import Select from '@/components/common/Select.vue'
import type { GroupModelOption } from '@/api/admin/groups'

const props = defineProps<{ modelValue: string; initialModel?: string; modelOptions?: GroupModelOption[]; modelSource?: 'loading' | 'adobe' | 'fallback' }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; 'update:selectedModel': [value: string] }>()
const { t } = useI18n()
const parsed = parseAdobeGroupName(props.modelValue)
const availableOptions = computed(() => (props.modelOptions?.length ? props.modelOptions : adobeModels.map(m => ({ id: m.id, display_name: m.label }))).map(m => ({ value: m.id, label: m.display_name })))
const rememberedLabels = ref(new Map<string, string>())
watch(availableOptions, values => {
  rememberedLabels.value = new Map([...rememberedLabels.value, ...values.map(m => [m.value, m.label] as const)])
}, { immediate: true })
const initialOption = availableOptions.value.find(m => m.value === props.initialModel || m.label === props.initialModel)
const draftParts = props.modelValue.startsWith('Adobe ') ? props.modelValue.slice(6).split(' - ') : []
const model = ref(parsed?.model || initialOption?.value || props.initialModel || draftParts[0] || availableOptions.value[0]?.value || '')
const options = computed(() => {
  const values = availableOptions.value
  const label = rememberedLabels.value.get(model.value)
  return label && !values.some(m => m.value === model.value) ? [...values, { value: model.value, label }] : values
})
const modelTouched = ref(false)
watch(availableOptions, values => {
  if (!customer.value.trim() && !modelTouched.value && !props.modelValue && !props.initialModel && !values.some(m => m.value === model.value)) model.value = values[0]?.value || ''
})
const presets = ['低中质量 124K 全参分组', '低中质量 124K 全参分组（7分）', '6分分组']
const initialCategory = parsed?.category || (draftParts.length > 2 ? draftParts[1] : '')
const category = ref(presets.includes(initialCategory) || !initialCategory ? initialCategory : 'custom')
const customCategory = ref(category.value === 'custom' ? initialCategory : '')
const customer = ref(parsed?.customer || (draftParts.length ? draftParts.slice(initialCategory ? 2 : 1).join(' - ') : ''))
const selectedCategory = computed(() => category.value === 'custom' ? customCategory.value.trim() : category.value)
const modelLabel = computed(() => options.value.find(m => m.value === model.value)?.label || rememberedLabels.value.get(model.value) || model.value.trim())
const preview = computed(() => formatAdobeGroupName(modelLabel.value, selectedCategory.value,
  customer.value.trim() || (category.value === '6分分组' ? '' : t('admin.groups.adobeNaming.customerPreview'))))
watch(model, value => emit('update:selectedModel', value), { immediate: true })
watch([modelLabel, category, customCategory, customer], () => {
  const valid = !!modelLabel.value.trim() && (category.value !== 'custom' || !!customCategory.value.trim()) &&
    (category.value === '6分分组' || !!customer.value.trim())
  emit('update:modelValue', valid ? formatAdobeGroupName(modelLabel.value, selectedCategory.value, customer.value) : '')
}, { immediate: true })
</script>
