<template>
  <article class="min-w-0 rounded-xl border border-gray-200 p-4 dark:border-dark-600" data-testid="source-card">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ t('proxyHealth.source', { index: source.index }) }}</h4>
      <span class="inline-flex items-center gap-1.5 text-xs" :class="statusColor">
        <span class="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true" />{{ t(`proxyHealth.states.${source.status}`) }}
      </span>
    </div>
    <p class="mt-2 truncate text-xs text-gray-500 dark:text-gray-400" :title="source.url_hint + ' · ' + source.id.slice(-6)">{{ source.url_hint }} · {{ source.id.slice(-6) }}</p>
    <div class="mt-4 space-y-2">
      <div class="flex items-baseline justify-between gap-2 text-xs">
        <span class="text-gray-600 dark:text-gray-400">{{ t('proxyHealth.trafficCapacity') }}</span>
        <span class="font-medium tabular-nums" :class="trafficColor">{{ percentage === null ? '—' : t('proxyHealth.remainingPercent', { percent: percentage }) }}</span>
      </div>
      <HealthCapacityBar :value="percentage" :total="100" :label="t('proxyHealth.trafficCapacity')" :summary="trafficText" :tone="trafficTone" />
      <p class="text-xs tabular-nums text-gray-600 dark:text-gray-400">{{ trafficText }}</p>
    </div>
    <div class="mt-4 space-y-2">
      <div class="flex items-baseline justify-between gap-2 text-xs">
        <span class="text-gray-600 dark:text-gray-400">{{ t('proxyHealth.nodeCapacity') }}</span>
        <span class="tabular-nums text-gray-700 dark:text-gray-300">{{ t('proxyHealth.totalCount', { count: source.nodes }) }}</span>
      </div>
      <div class="flex h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-600" role="img" :aria-label="t('proxyHealth.nodes', { healthy: source.healthy, failed: source.failed, unknown: source.unknown, total: source.nodes })" data-testid="node-capacity">
        <span class="h-full bg-emerald-500" :style="{ width: nodePercent(source.healthy) }" />
        <span class="h-full bg-red-500" :style="{ width: nodePercent(source.failed) }" />
        <span class="h-full bg-gray-400" :style="{ width: nodePercent(source.unknown) }" />
      </div>
      <div class="flex flex-wrap gap-x-3 gap-y-1 text-xs tabular-nums">
        <span class="inline-flex items-center gap-1.5"><i class="h-1.5 w-1.5 rounded-full bg-emerald-500" aria-hidden="true" />{{ t('proxyHealth.reachableCount', { count: source.healthy }) }}</span>
        <span class="inline-flex items-center gap-1.5"><i class="h-1.5 w-1.5 rounded-full bg-red-500" aria-hidden="true" />{{ t('proxyHealth.failedCount', { count: source.failed }) }}</span>
        <span class="inline-flex items-center gap-1.5 text-gray-500 dark:text-gray-400"><i class="h-1.5 w-1.5 rounded-full bg-gray-400" aria-hidden="true" />{{ t('proxyHealth.unknownCount', { count: source.unknown }) }}</span>
      </div>
    </div>
    <button v-if="source.generation_blocked" class="mt-4 flex w-full items-center justify-between gap-2 rounded-lg bg-amber-50 px-3 py-2 text-left text-xs leading-5 text-amber-800 transition-colors hover:bg-amber-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-amber-600 dark:bg-amber-900/20 dark:text-amber-300 dark:hover:bg-amber-900/30" data-testid="blocked-details" @click="emit('details')">
      <span>{{ t('proxyHealth.generationBlocked', { count: source.generation_blocked }) }} <span class="whitespace-nowrap underline underline-offset-2">{{ t('proxyHealth.viewDetails') }}</span></span>
      <Icon name="chevronRight" size="sm" class="shrink-0" aria-hidden="true" />
    </button>
    <div v-if="expanded" class="mt-4 space-y-2 border-t border-gray-100 pt-3 text-xs leading-5 text-gray-500 dark:text-gray-400 dark:border-dark-600">
      <p v-if="source.expires_at">{{ t('proxyHealth.expires') }} <span class="tabular-nums">{{ formatDateTime(source.expires_at) }}</span></p>
      <p v-if="source.traffic_observed_at">{{ t('proxyHealth.observed') }} <span class="tabular-nums">{{ formatDateTime(source.traffic_observed_at) }}</span></p>
      <p v-if="source.status === 'exhausted' || source.status === 'expired'" class="text-red-700 dark:text-red-300">{{ t('proxyHealth.excluded') }}</p>
      <p v-if="source.fetch_error" class="text-amber-700 dark:text-amber-300">{{ t('proxyHealth.fetchFailed', { reason: fetchReason(source.fetch_error, source.http_status) }) }}</p>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProxySourceHealth } from '@/api/admin/clash'
import { formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import HealthCapacityBar from './HealthCapacityBar.vue'
const props = defineProps<{ source: ProxySourceHealth; expanded: boolean }>()
const emit = defineEmits<{ details: [] }>()
const { t } = useI18n()
const percentage = computed(() => {
  const s = props.source
  if (!s.traffic_known || s.remaining_bytes === null || s.total_bytes === null || !Number.isFinite(s.remaining_bytes) || !Number.isFinite(s.total_bytes) || s.total_bytes <= 0) return null
  return Math.round(Math.min(100, Math.max(0, s.remaining_bytes / s.total_bytes * 100)) * 10) / 10
})
const trafficText = computed(() => percentage.value === null ? t('proxyHealth.trafficUnknown') : t('proxyHealth.remaining', { remaining: bytes(props.source.remaining_bytes), total: bytes(props.source.total_bytes) }))
const trafficTone = computed(() => percentage.value === null ? 'gray' : percentage.value <= 10 ? 'red' : percentage.value <= 25 ? 'amber' : 'green')
const trafficColor = computed(() => ({ gray: 'text-gray-500 dark:text-gray-400', red: 'text-red-700 dark:text-red-300', amber: 'text-amber-700 dark:text-amber-300', green: 'text-emerald-700 dark:text-emerald-300' })[trafficTone.value])
const statusColor = computed(() => {
  const status = props.source.status
  if (['exhausted', 'expired', 'unavailable'].includes(status)) return 'text-red-700 dark:text-red-300'
  if (['degraded', 'fetch_failed', 'stale'].includes(status)) return 'text-amber-700 dark:text-amber-300'
  return status === 'healthy' ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-500 dark:text-gray-400'
})
function bytes(n: number | null) {
  if (n === null || !Number.isFinite(n)) return '—'
  if (n < 1024 ** 3) return `${(Math.max(0, n) / 1024 ** 2).toFixed(1)} MiB`
  return `${(n / 1024 ** 3).toFixed(2)} GiB`
}
function nodePercent(count: number) { return `${props.source.nodes > 0 ? Math.min(100, Math.max(0, count / props.source.nodes * 100)) : 0}%` }
function fetchReason(reason: string, status?: number) {
  if (reason === 'http_error') return `HTTP ${status || '—'}`
  if (['invalid_url', 'connection_failed', 'read_failed', 'invalid_subscription', 'not_checked'].includes(reason)) return t(`proxyHealth.fetchReasons.${reason}`)
  return t('proxyHealth.fetchReasons.unknown')
}
</script>
