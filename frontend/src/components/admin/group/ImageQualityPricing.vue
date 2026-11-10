<template>
  <section class="mt-4 border-t border-gray-200 pt-4 dark:border-dark-700" data-testid="image-quality-pricing">
    <h4 class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('admin.groups.imageQualityPricing.title') }}</h4>
    <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.groups.imageQualityPricing.hint') }}</p>
    <div class="mt-3 flex gap-5 overflow-x-auto border-b border-gray-200 dark:border-dark-700" role="tablist" :aria-label="t('admin.groups.imageQualityPricing.model')">
      <button v-for="(model, index) in models" :id="`${tabId}-${index}`" :key="model.id" type="button" role="tab"
        :aria-selected="selectedModel === model.id" :aria-controls="`${tabId}-panel`" :tabindex="selectedModel === model.id ? 0 : -1"
        :data-model="model.id" :class="['shrink-0 border-b-2 py-3 text-sm font-medium transition-colors', selectedModel === model.id ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-800 dark:hover:text-gray-200']"
        @click="selectedModel = model.id" @keydown="onTabKey($event, index)">{{ model.display_name }}</button>
    </div>
    <div v-if="activeModel" :id="`${tabId}-panel`" role="tabpanel" :aria-labelledby="`${tabId}-${activeIndex}`" class="mt-4">
      <p v-if="capabilities" class="mb-3 text-xs text-gray-500">{{ t(capabilities.source === 'adobe' ? 'admin.groups.imageQualityPricing.officialSource' : 'admin.groups.imageQualityPricing.fallbackSource', { date: capabilities.checkedAt.slice(0, 10) }) }}</p>
      <p v-if="capabilities?.resolutionMode === 'default'" class="mb-3 text-sm text-gray-600">{{ t('admin.groups.imageQualityPricing.defaultResolution') }}</p>
      <p v-else-if="!sizes.length" class="mb-3 text-sm text-gray-500">{{ t('admin.groups.imageQualityPricing.unknownResolution') }}</p>
      <p v-if="capabilities?.qualityMode === 'none'" class="mb-3 text-sm text-gray-600">{{ t('admin.groups.imageQualityPricing.noQuality') }}</p>
      <p v-else-if="!qualities.length" class="mb-3 text-sm text-gray-500">{{ t('admin.groups.imageQualityPricing.unknownQuality') }}</p>
      <p v-if="sizes.length && !qualities.length" class="mb-3 text-sm text-gray-600">{{ t('admin.groups.imageQualityPricing.resolution') }}：{{ sizes.join(' / ') }}</p>
      <p v-if="qualities.length && !sizes.length" class="mb-3 text-sm text-gray-600">{{ t('admin.groups.imageQualityPricing.quality') }}：{{ qualities.map(q => q.label).join(' / ') }}</p>
      <div v-if="qualities.length && sizes.length" class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
        <table class="w-full min-w-[480px] text-left text-sm">
          <thead class="bg-gray-50 text-gray-600 dark:bg-dark-800 dark:text-gray-300"><tr>
            <th class="px-3 py-3 font-medium">{{ t('admin.groups.imageQualityPricing.resolution') }}</th>
            <th v-for="quality in qualities" :key="quality.id" class="px-2 py-3 font-medium">{{ quality.label }}</th>
          </tr></thead>
          <tbody><tr v-for="size in sizes" :key="size" class="border-t border-gray-100 dark:border-dark-700">
            <th class="px-3 py-2 font-medium">{{ size }}</th>
            <td v-for="quality in qualities" :key="quality.id" class="px-2 py-2">
              <input v-if="canPrice(size, quality.id)" type="number" inputmode="decimal" min="0" max="100000000" step="0.0001" class="input w-full min-w-[90px] text-sm"
                :value="displayPrice(modelValue[activeModel.id]?.[size]?.[quality.id])"
                :aria-label="`${activeModel.display_name} ${size} ${quality.label}`"
                :placeholder="t('admin.groups.imageQualityPricing.inherit')"
                @input="setPrice(activeModel.id, size, quality.id, ($event.target as HTMLInputElement).value)"
                @blur="formatInput($event)" />
              <span v-else class="text-xs text-gray-400">{{ t('admin.groups.imageQualityPricing.unavailableCell') }}</span>
            </td>
          </tr></tbody>
        </table>
      </div>
      <p v-if="!billingSupported" class="mt-3 rounded-lg bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-800">{{ t('admin.groups.imageQualityPricing.unsupported') }}</p>
    </div>
    <p v-else class="mt-4 text-sm text-gray-500">{{ t('admin.groups.imageQualityPricing.chooseModels') }}</p>
  </section>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, nextTick, ref, useId, watch } from 'vue'
import type { ImageQualityPrices } from '@/types'
import type { GroupModelOption } from '@/api/admin/groups'
import { adobeModels } from '@/utils/adobeGroupName'
import { adobeFallbackModelOptions } from '@/utils/adobeModelOptions'
const props = defineProps<{ modelValue: ImageQualityPrices; models?: GroupModelOption[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: ImageQualityPrices] }>()
const { t } = useI18n()
const tabId = useId()
const models = computed(() => props.models ?? adobeFallbackModelOptions.filter(m => ['gpt-image-2.6-flare', 'gpt-image-2.6-sunburst'].includes(m.id)))
const selectedModel = ref('')
watch(models, value => {
  if (!value.some(m => m.id === selectedModel.value)) selectedModel.value = value[0]?.id || ''
}, { immediate: true })
const activeIndex = computed(() => models.value.findIndex(m => m.id === selectedModel.value))
const activeModel = computed(() => models.value[activeIndex.value])
const capabilities = computed(() => activeModel.value?.capabilities ?? adobeFallbackModelOptions.find(m => m.id === activeModel.value?.id)?.capabilities)
const sizes = computed(() => capabilities.value?.resolutions || [])
const qualities = computed(() => capabilities.value?.qualities || [])
const billingSupported = computed(() => adobeModels.some(m => m.id === activeModel.value?.id))
function canPrice(size: string, quality: string) {
  const allowedQuality = activeModel.value?.id === 'gpt-image-2' ? ['low', 'medium', 'high'] : ['low', 'medium', 'high', 'xhigh', 'max']
  return billingSupported.value && ['1K', '2K', '4K'].includes(size) && allowedQuality.includes(quality)
}
const displayPrice = (value: number | undefined) => value == null ? '' : value.toFixed(4)
function formatInput(event: FocusEvent) {
  const input = event.target as HTMLInputElement
  if (input.value.trim() && input.validity.valid) input.value = Number(input.value).toFixed(4)
}
async function onTabKey(event: KeyboardEvent, index: number) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const count = models.value.length
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? count - 1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + count) % count
  selectedModel.value = models.value[next].id
  await nextTick()
  document.getElementById(`${tabId}-${next}`)?.focus()
}
function setPrice(model: string, size: string, quality: string, raw: string) {
  const value = Number(raw)
  if (raw.trim() && (!Number.isFinite(value) || value < 0 || value > 1e8)) return
  const prices = { ...props.modelValue, [model]: { ...props.modelValue[model], [size]: { ...props.modelValue[model]?.[size] } } }
  if (raw.trim() === '') delete prices[model][size][quality]
  else prices[model][size][quality] = Number(value.toFixed(4))
  if (!Object.keys(prices[model][size]).length) delete prices[model][size]
  if (!Object.keys(prices[model]).length) delete prices[model]
  emit('update:modelValue', prices)
}
</script>
