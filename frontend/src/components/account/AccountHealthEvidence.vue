<template>
  <div v-if="account.platform === 'adobe'" class="mt-1 max-w-[290px] space-y-1 text-[11px] leading-4" data-testid="account-health-evidence">
    <span v-if="jobState === 'pending' || jobState === 'running'" class="block text-blue-600 dark:text-blue-300">{{ jobState === 'pending' ? 'Cookie 已收到，等待接口检测' : '正在检测接口，请等待结果' }}</span>
    <template v-if="latest">
      <div :class="latest.success && !health?.pending ? 'text-emerald-700 dark:text-emerald-300' : 'text-amber-700 dark:text-amber-300'">{{ latest.success ? (health?.pending ? '本次成功，仍有其他失败待验证' : '最近一次请求成功（所测模型）') : reasonLabel }}</div>
      <div v-if="!latest.success" class="text-gray-600 dark:text-gray-300">{{ advice }}</div>
      <div class="text-gray-500">{{ sourceLabel }} · {{ time(latest.checked_at) }}<span v-if="latest.stage"> · {{ stageLabel }}</span></div>
      <div v-if="health?.last_success" class="text-gray-500">最近成功 {{ time(health.last_success.checked_at) }}</div>
    </template>
    <span v-else class="block text-gray-500">尚无完整成功记录 · 请结合最近检测时间判断</span>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { Account } from '@/types'
import { healthReason, healthSource, healthStage, type HealthObservation } from '@/utils/adobeHealth'
const props = defineProps<{ account: Account }>()
const health = computed(() => props.account.extra?.adobe_health as { latest?: HealthObservation; last_success?: HealthObservation; pending?: boolean } | undefined)
const latest = computed(() => health.value?.latest)
const jobState = computed(() => (props.account.extra?.okad_connection_test as { state?: string } | undefined)?.state)
const reasonLabel = computed(() => healthReason(latest.value?.reason || '')[0])
const advice = computed(() => healthReason(latest.value?.reason || '')[1])
const sourceLabel = computed(() => healthSource(latest.value?.source || ''))
const stageLabel = computed(() => healthStage(latest.value?.stage || ''))
function time(value: string) { return new Date(value).toLocaleString('zh-CN', { hour12: false }) }
</script>
