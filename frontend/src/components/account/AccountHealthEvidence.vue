<template>
  <div class="space-y-3" data-testid="account-health-evidence">
    <p v-if="pending" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
      失败后还没有对应的成功记录，暂不能确认恢复。
    </p>
    <p v-if="!records.length" class="rounded-lg bg-gray-50 p-3 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-300">
      没有可展示的请求或检测记录，无法据此判断当前请求能否成功。
    </p>
    <section v-for="record in records" :key="record.title" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
      <div class="flex flex-wrap items-baseline justify-between gap-2">
        <h4 class="font-medium text-gray-900 dark:text-gray-100">{{ record.title }}</h4>
        <span class="text-sm text-gray-700 dark:text-gray-200">{{ record.result }}</span>
      </div>
      <p v-if="record.time !== undefined" class="mt-1 text-xs text-gray-500">{{ statusRecordTime(record.time) }}</p>
      <dl class="mt-3 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm">
        <template v-for="item in record.fields" :key="item.label">
          <dt class="text-gray-500 dark:text-dark-400">{{ item.label }}</dt>
          <dd class="break-words text-gray-800 dark:text-gray-200">{{ item.value }}</dd>
        </template>
      </dl>
      <p v-if="record.note" class="mt-3 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ record.note }}</p>
    </section>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { Account } from '@/types'
import { accountStatusRecords, statusRecordTime } from '@/utils/accountStatusEvidence'
const props = defineProps<{ account: Account }>()
const records = computed(() => accountStatusRecords(props.account))
const pending = computed(() => (props.account.extra?.adobe_health as { pending?: boolean } | undefined)?.pending === true)
</script>
