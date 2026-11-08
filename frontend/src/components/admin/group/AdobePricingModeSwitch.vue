<template>
  <section class="mb-4 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800" data-testid="adobe-pricing-mode">
    <div class="flex items-center justify-between gap-4">
      <div>
        <label :id="labelId" class="block text-sm font-medium text-gray-900 dark:text-gray-100">{{ t('admin.groups.adobePricingMode.title') }}</label>
        <p class="mt-1 text-xs text-gray-500">{{ t(modelValue ? 'admin.groups.adobePricingMode.newDescription' : 'admin.groups.adobePricingMode.oldDescription') }}</p>
      </div>
      <Toggle :model-value="modelValue" :aria-labelledby="labelId" @update:model-value="$emit('update:modelValue', $event)" />
    </div>
    <p v-if="modelValue || hasSavedPrices" class="mt-2 text-xs leading-5 text-gray-500">
      {{ t(hasSavedPrices && !modelValue ? 'admin.groups.adobePricingMode.savedNotice' : 'admin.groups.adobePricingMode.safeNotice') }}
    </p>
  </section>
</template>
<script setup lang="ts">
import { useId } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
defineProps<{ modelValue: boolean; hasSavedPrices?: boolean }>()
defineEmits<{ 'update:modelValue': [value: boolean] }>()
const labelId = useId()
const { t } = useI18n()
</script>
