<template>
  <span
    v-if="okadAccountType(account) === 'company'"
    class="inline-flex shrink-0 items-center gap-0.5 whitespace-nowrap rounded border border-emerald-200 bg-emerald-50 px-1.5 py-px text-xs font-normal text-emerald-600 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-400"
    :title="t('admin.accounts.okadCompanyHint')"
    data-test="okad-company-tag"
  >
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M20 13 13 20a2 2 0 0 1-3 0l-8-8V2h10l8 8a2 2 0 0 1 0 3Z"/><circle cx="7" cy="7" r="1"/></svg>
    {{ t('admin.accounts.okadCompanyTag') }}
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import { okadAccountType } from '@/utils/okadAccountType'
defineProps<{ account: Account }>()
const { t } = useI18n()
</script>
