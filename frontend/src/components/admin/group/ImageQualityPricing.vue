<template>
  <section v-if="visibleModels.length" class="mt-4 border-t border-gray-200 pt-4 dark:border-dark-700" data-testid="image-quality-pricing">
    <h4 class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('admin.groups.imageQualityPricing.title') }}</h4>
    <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.groups.imageQualityPricing.hint') }}</p>
    <div v-for="model in visibleModels" :key="model.id" class="mt-4">
      <h5 class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-200">{{ model.label }}</h5>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[360px] text-left text-sm">
          <thead><tr><th class="py-2 font-medium">{{ t('admin.groups.imageQualityPricing.quality') }}</th><th v-for="size in model.sizes" :key="size" class="px-1 py-2 font-medium">{{ size }} ($)</th></tr></thead>
          <tbody>
            <tr v-for="quality in model.qualities" :key="quality.id">
              <th class="py-1 pr-2 font-normal">{{ quality.label }}</th>
              <td v-for="size in model.sizes" :key="size" class="px-1 py-1">
                <input type="number" min="0" max="100000000" step="any" class="input w-full min-w-0 text-sm"
                  :value="modelValue[model.id]?.[size]?.[quality.id] ?? ''"
                  :aria-label="`${model.label} ${size} ${quality.label}`"
                  :placeholder="t('admin.groups.imageQualityPricing.inherit')"
                  @input="setPrice(model.id, size, quality.id, ($event.target as HTMLInputElement).value)" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import type { ImageQualityPrices } from '@/types'
import { adobeModels } from '@/utils/adobeGroupName'
const props = defineProps<{ modelValue: ImageQualityPrices; model: string; modelLabel?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: ImageQualityPrices] }>()
const { t } = useI18n()
// Verified against Firefly discovery and its detailLevel converter on 2026-11-09.
// Unknown/custom models deliberately have no guessed quality table.
const qualities = [
  { id: 'low', label: 'Low' }, { id: 'medium', label: 'Medium' },
  { id: 'high', label: 'High' }, { id: 'xhigh', label: 'Extra High' }, { id: 'max', label: 'Max' },
]
const visibleModels = computed(() => adobeModels.filter(model => model.id === props.model)
  .map(model => ({ ...model, label: props.modelLabel || model.label,
    sizes: ['1K', '2K', '4K'], qualities: model.version === '2' ? qualities.slice(0, 3) : qualities })))
function setPrice(model: string, size: string, quality: string, raw: string) {
  const prices = { ...props.modelValue }
  prices[model] = { ...prices[model] }
  prices[model][size] = { ...prices[model][size] }
  if (raw.trim() === '') delete prices[model][size][quality]
  else prices[model][size][quality] = Number(raw)
  if (!Object.keys(prices[model][size]).length) delete prices[model][size]
  if (!Object.keys(prices[model]).length) delete prices[model]
  emit('update:modelValue', prices)
}
</script>
