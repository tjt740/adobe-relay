<template>
  <section :aria-label="t('changelog.daily.title')" class="space-y-4">
    <section v-if="changelogPreviews.length" :aria-label="t('changelog.daily.previewTitle')" class="space-y-4" data-testid="changelog-previews">
      <details v-for="preview in changelogPreviews" :key="preview.id" open class="daily-preview overflow-hidden rounded-2xl border border-amber-200 bg-white dark:border-amber-900/50 dark:bg-dark-900">
        <summary class="flex cursor-pointer flex-wrap items-center gap-3 px-5 py-5 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 sm:px-6">
          <span class="rounded-full bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-800 dark:bg-amber-900/30 dark:text-amber-200">{{ t('changelog.daily.previewBadge') }}</span>
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t(`changelog.daily.updates.${preview.id}.title`) }}</h3>
          <Icon name="chevronDown" size="sm" aria-hidden="true" class="daily-chevron ml-auto shrink-0 text-gray-400 transition-transform motion-reduce:transition-none" />
        </summary>
        <div class="space-y-5 border-t border-amber-100 px-5 py-5 dark:border-amber-900/40 sm:px-6">
          <p class="text-sm leading-7 text-gray-600 dark:text-gray-400">{{ t(`changelog.daily.updates.${preview.id}.body`) }}</p>
          <AccountRecoveryDemo v-if="preview.demo === 'account-recovery'" />
        </div>
      </details>
    </section>
    <details v-for="(day, index) in days" :key="day.date" :open="index === 0" class="daily-release overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900" :data-release-date="day.date">
      <summary class="flex cursor-pointer flex-wrap items-center gap-3 px-5 py-5 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 sm:px-6">
        <span class="h-2 w-2 shrink-0 rounded-full" :class="index === 0 ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'"></span>
        <h3 class="text-base font-semibold tabular-nums text-gray-900 dark:text-white"><time :datetime="day.date">{{ day.date }}</time></h3>
        <span v-if="index === 0" class="rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">{{ t('changelog.daily.latest') }}</span>
        <span class="ml-auto text-xs text-gray-500 dark:text-gray-400">{{ t('changelog.daily.count', { count: day.items.length }) }}</span>
        <Icon name="chevronDown" size="sm" aria-hidden="true" class="daily-chevron shrink-0 text-gray-400 transition-transform motion-reduce:transition-none" />
      </summary>
      <ul class="divide-y divide-gray-100 border-t border-gray-100 px-5 dark:divide-dark-700 dark:border-dark-700 sm:px-6">
        <li v-for="item in day.items" :key="item.id" class="py-5">
          <div class="flex flex-wrap items-center gap-2.5">
            <span class="rounded-md px-2 py-0.5 text-[11px] font-medium" :class="typeStyles[item.type]">{{ t(`changelog.daily.types.${item.type}`) }}</span>
            <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ t(`changelog.daily.updates.${item.id}.title`) }}</h4>
          </div>
          <p class="mt-2 text-sm leading-7 text-gray-600 dark:text-gray-400">{{ t(`changelog.daily.updates.${item.id}.body`) }}</p>
        </li>
      </ul>
    </details>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import AccountRecoveryDemo from './AccountRecoveryDemo.vue'
import { dailyUpdates, groupDailyUpdates, changelogPreviews } from '@/content/changelog/daily'

const { t } = useI18n()
const days = groupDailyUpdates(dailyUpdates)
const typeStyles = {
  added: 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300',
  improved: 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
  fixed: 'bg-amber-50 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300',
}
</script>

<style scoped>
summary { list-style: none; }
summary::-webkit-details-marker { display: none; }
.daily-release[open] .daily-chevron { transform: rotate(180deg); }
.daily-preview[open] > summary .daily-chevron { transform: rotate(180deg); }
</style>
