<template>
  <div class="space-y-3" data-testid="adobe-group-name-fields">
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <div>
        <label for="adobe-name-model" class="input-label">{{ t('admin.groups.adobeNaming.model') }}</label>
        <Select id="adobe-name-model" :model-value="null" :selected-values="models" :close-on-select="false" :options="options" :searchable="true" creatable
          :creatable-prefix="t('admin.groups.adobeNaming.useCustom')" :aria-label="t('admin.groups.adobeNaming.model')"
          :search-placeholder="t('admin.groups.adobeNaming.modelSearch')" @change="toggleModel">
          <template #selected>
            <span class="flex flex-wrap gap-1 whitespace-normal">
              <span v-for="item in selectedModels" :key="item.id" class="rounded bg-primary-50 px-1.5 py-0.5 text-xs text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">{{ item.display_name }}</span>
              <span v-if="!selectedModels.length">{{ t('admin.groups.adobeNaming.chooseModels') }}</span>
            </span>
          </template>
        </Select>
        <p class="input-hint">{{ t('admin.groups.adobeNaming.source.' + (modelSource || 'fallback')) }}</p>
      </div>
      <div>
        <label for="adobe-name-customer" class="input-label">{{ t('admin.groups.adobeNaming.customer') }}</label>
        <input ref="customerInput" id="adobe-name-customer" v-model="customer" class="input" required maxlength="100"
          :placeholder="t('admin.groups.adobeNaming.customerPlaceholder')" />
      </div>
    </div>
    <div class="rounded-xl border border-primary-100 bg-primary-50/60 px-4 py-3 dark:border-primary-800 dark:bg-primary-900/15">
      <p class="mb-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.adobeNaming.preview') }}</p>
      <p class="break-words text-sm font-medium text-gray-900 dark:text-gray-100">{{ preview }}</p>
    </div>
    <p v-if="nameTooLong" role="alert" class="text-sm text-red-600">{{ t('admin.groups.adobeNaming.tooLong') }}</p>
    <p class="input-hint">{{ t('admin.groups.adobeNaming.hint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adobeGroupModelNames, formatAdobeGroupName, parseAdobeGroupName } from '@/utils/adobeGroupName'
import { adobeFallbackModelOptions } from '@/utils/adobeModelOptions'
import Select from '@/components/common/Select.vue'
import type { GroupModelOption } from '@/api/admin/groups'

const props = defineProps<{ modelValue: string; initialModel?: string; modelOptions?: GroupModelOption[]; modelSource?: 'loading' | 'adobe' | 'fallback' }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; 'update:selectedModels': [value: GroupModelOption[]] }>()
const { t } = useI18n()
const parsed = parseAdobeGroupName(props.modelValue)
const availableModels = computed(() => props.modelOptions?.length ? props.modelOptions : adobeFallbackModelOptions)
const rememberedModels = ref(new Map<string, GroupModelOption>())
watch(availableModels, values => {
  rememberedModels.value = new Map([...rememberedModels.value, ...values.map(m => [m.id, m] as const)])
}, { immediate: true })
const availableOptions = computed(() => availableModels.value.map(m => ({ value: m.id, label: m.display_name })))
const rememberedLabels = ref(new Map<string, string>())
watch(availableOptions, values => {
  rememberedLabels.value = new Map([...rememberedLabels.value, ...values.map(m => [m.value, m.label] as const)])
}, { immediate: true })
const initialOption = availableOptions.value.find(m => m.value === props.initialModel || m.label === props.initialModel)
const draftParts = props.modelValue.startsWith('Adobe ') ? props.modelValue.slice(6).split(' - ') : []
const draftModels = adobeGroupModelNames(props.modelValue)
const initialModels = draftModels.length > 1 ? draftModels.map(label => availableOptions.value.find(m => m.label === label)?.value || label) :
  [parsed?.model || initialOption?.value || props.initialModel || draftParts[0] || availableOptions.value[0]?.value || ''].filter(Boolean)
const models = ref<string[]>(initialModels)
const options = computed(() => {
  const values = availableOptions.value
  return [...values, ...models.value.filter(id => !values.some(m => m.value === id)).map(id => ({ value: id, label: rememberedLabels.value.get(id) || id }))]
})
const modelTouched = ref(false)
watch(availableOptions, values => {
  if (!customer.value.trim() && !modelTouched.value && !props.modelValue && !props.initialModel && !models.value.some(id => values.some(m => m.value === id))) models.value = values[0] ? [values[0].value] : []
})
// Keep the complete existing suffix editable when reopening a draft with a type.
const customer = ref(parsed
  ? [parsed.category, parsed.customer].filter(Boolean).join(' - ')
  : draftParts.slice(1).join(' - '))
const selectedModels = computed(() => models.value.map(id => rememberedModels.value.get(id) || { id, display_name: options.value.find(m => m.value === id)?.label || id }))
const modelLabel = computed(() => selectedModels.value.map(m => m.display_name).join(' + '))
function toggleModel(value: string | number | boolean | null) {
  if (typeof value !== 'string' || !value.trim()) return
  modelTouched.value = true
  const id = options.value.find(m => m.label.toLowerCase() === value.trim().toLowerCase())?.value || value.trim()
  models.value = models.value.includes(id) ? models.value.filter(m => m !== id) : [...models.value, id]
}
const preview = computed(() => modelLabel.value ? formatAdobeGroupName(modelLabel.value, '',
  customer.value.trim() || t('admin.groups.adobeNaming.customerPreview')) : t('admin.groups.adobeNaming.chooseModels'))
const customerInput = ref<HTMLInputElement | null>(null)
const nameTooLong = computed(() => new TextEncoder().encode(formatAdobeGroupName(modelLabel.value, '', customer.value)).length > 512)
watch([models, nameTooLong, customerInput], () => customerInput.value?.setCustomValidity(
  !models.value.length ? t('admin.groups.adobeNaming.chooseModels') : nameTooLong.value ? t('admin.groups.adobeNaming.tooLong') : ''
), { immediate: true })
watch(selectedModels, value => emit('update:selectedModels', value), { immediate: true })
watch([modelLabel, customer, nameTooLong], () => {
  const valid = !!modelLabel.value.trim() && !!customer.value.trim() && !nameTooLong.value
  emit('update:modelValue', valid ? formatAdobeGroupName(modelLabel.value, '', customer.value) : '')
}, { immediate: true })
</script>
