<template>
  <section class="mb-4 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800" data-testid="adobe-pricing-mode">
    <div class="flex items-center justify-between gap-4">
      <div class="flex items-center gap-1">
        <label :id="labelId" class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ t('admin.groups.adobePricingMode.title') }}</label>
        <HelpTooltip trigger="click" width-class="w-72 max-w-[calc(100vw-2rem)]">
          <template #trigger>
            <button type="button" class="inline-flex h-7 w-7 items-center justify-center rounded-full text-gray-500 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500" :aria-label="t('admin.groups.adobePricingMode.help')">
              <Icon name="exclamationCircle" size="sm" />
            </button>
          </template>
          <div class="space-y-2 pr-5">
            <p>{{ t(modelValue ? 'admin.groups.adobePricingMode.newDescription' : 'admin.groups.adobePricingMode.oldDescription') }}</p>
            <p>{{ t(hasSavedPrices && !modelValue ? 'admin.groups.adobePricingMode.savedNotice' : 'admin.groups.adobePricingMode.safeNotice') }}</p>
          </div>
        </HelpTooltip>
      </div>
      <Toggle :model-value="modelValue" :aria-labelledby="labelId" @update:model-value="$emit('update:modelValue', $event)" />
    </div>
  </section>
</template>
<script setup lang="ts">
import { useId } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
defineProps<{ modelValue: boolean; hasSavedPrices?: boolean }>()
defineEmits<{ 'update:modelValue': [value: boolean] }>()
const labelId = useId()
const { t } = useI18n()
</script>
