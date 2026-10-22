<template>
  <section class="mt-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800" :aria-label="t('proxyHealth.title')">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h3 class="text-sm font-semibold">{{ t('proxyHealth.title') }}</h3>
      <div class="flex w-full flex-wrap items-center gap-x-3 gap-y-2 text-xs sm:w-auto">
        <span v-if="view" class="w-full whitespace-nowrap text-gray-500 sm:w-auto">{{ t('proxyHealth.updated') }} {{ formatDateTime(view.checked_at) }}</span>
        <button class="inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap text-primary-600 hover:underline" @click="emit('manage')"><Icon name="link" size="sm" aria-hidden="true" />{{ t('proxyHealth.manage') }}</button>
        <button class="inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap text-primary-600 hover:underline" :disabled="loading" @click="refresh"><Icon name="refresh" size="sm" :class="{ 'motion-safe:animate-spin': loading }" aria-hidden="true" />{{ t('common.refresh') }}</button>
        <button class="inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap text-primary-600 hover:underline" :aria-expanded="expanded" @click="expanded = !expanded"><Icon :name="expanded ? 'chevronUp' : 'chevronDown'" size="sm" aria-hidden="true" />{{ t(expanded ? 'proxyHealth.collapse' : 'proxyHealth.expand') }}</button>
      </div>
    </div>
    <p v-if="failed" role="status" class="mt-2 text-xs text-amber-600">{{ t('proxyHealth.readFailed') }}</p>
    <p v-else-if="loading && !view" class="mt-2 text-xs text-gray-500">{{ t('common.loading') }}</p>
    <p v-if="view?.sources.length" class="mt-2 text-xs text-gray-500">{{ t('proxyHealth.sourceSummary', { total: view.sources.length, abnormal: problemSources.length }) }}</p>
    <div v-if="!expanded && problemSources.length" class="mt-2 space-y-1">
      <p v-for="source in problemSources" :key="source.id" class="break-all rounded-md px-2 py-1 text-xs" :class="severity(source.status)">
        {{ t('proxyHealth.source', { index: source.index }) }} · {{ t(`proxyHealth.states.${source.status}`) }} · {{ source.url_hint }} · {{ source.id.slice(-6) }}
      </p>
    </div>
    <div v-if="expanded && view?.sources.length" class="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <article v-for="source in view.sources" :key="source.id" class="subscription-card relative min-w-0 overflow-hidden rounded-xl border bg-white p-4 text-gray-700 dark:bg-dark-800 dark:text-gray-300" :class="`subscription-card--${tone(source.status)}`">
        <span class="subscription-watermark" aria-hidden="true">{{ t(`proxyHealth.watermarks.${source.status}`) }}</span>
        <div class="relative pr-32">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('proxyHealth.source', { index: source.index }) }}</h4>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t(`proxyHealth.states.${source.status}`) }}</p>
        </div>
        <p class="relative mt-4 break-all font-mono text-xs text-gray-500 dark:text-gray-400">{{ source.url_hint }} · {{ source.id.slice(-6) }}</p>
        <p class="mt-2 text-xs">{{ source.traffic_known ? t('proxyHealth.remaining', { remaining: bytes(source.remaining_bytes), total: bytes(source.total_bytes) }) : t('proxyHealth.trafficUnknown') }}</p>
        <p v-if="source.expires_at" class="mt-1 text-xs">{{ t('proxyHealth.expires') }} {{ formatDateTime(source.expires_at) }}</p>
        <div class="relative mt-3 flex flex-wrap items-center gap-2 text-xs tabular-nums" :aria-label="t('proxyHealth.nodes', { healthy: source.healthy, failed: source.failed, unknown: source.unknown, total: source.nodes })">
          <span class="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"><Icon name="checkCircle" size="xs" aria-hidden="true" />{{ t('proxyHealth.reachableCount', { count: source.healthy }) }}</span>
          <span class="inline-flex items-center gap-1 rounded-md bg-red-50 px-2 py-1 text-red-700 dark:bg-red-900/30 dark:text-red-300"><Icon name="xCircle" size="xs" aria-hidden="true" />{{ t('proxyHealth.failedCount', { count: source.failed }) }}</span>
          <span class="inline-flex items-center gap-1 rounded-md bg-amber-50 px-2 py-1 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300"><Icon name="questionCircle" size="xs" aria-hidden="true" />{{ t('proxyHealth.unknownCount', { count: source.unknown }) }}</span>
          <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400"><Icon name="server" size="xs" aria-hidden="true" />{{ t('proxyHealth.totalCount', { count: source.nodes }) }}</span>
        </div>
        <p v-if="source.status === 'exhausted' || source.status === 'expired'" class="mt-2 text-xs font-medium">{{ t('proxyHealth.excluded') }}</p>
        <p v-if="source.fetch_error" class="mt-2 text-xs">{{ t('proxyHealth.fetchFailed', { reason: fetchReason(source.fetch_error, source.http_status) }) }}</p>
        <p v-if="source.traffic_observed_at" class="mt-1 text-xs opacity-75">{{ t('proxyHealth.observed') }} {{ formatDateTime(source.traffic_observed_at) }}</p>
      </article>
    </div>
    <p v-else-if="view && !view.sources.length" class="mt-2 text-xs text-gray-500">{{ t('proxyHealth.noSources') }}</p>
    <details v-if="limitedGroups.length" class="mt-3 text-sm" :open="expanded">
      <summary class="cursor-pointer font-medium">{{ t('proxyHealth.capacity', { count: limitedGroups.length }) }}</summary>
      <div class="mt-2 space-y-1">
        <p v-for="group in limitedGroups" :key="group.id" class="break-words rounded-md px-2 py-1 text-xs" :class="group.enabled || group.waiting_proxy ? 'bg-amber-50 text-amber-800 dark:bg-amber-900/20 dark:text-amber-300' : 'bg-gray-50 text-gray-500 dark:bg-dark-700'">
          {{ group.name }} #{{ group.id }} · {{ t('proxyHealth.groupCapacity', { available: group.available, enabled: group.enabled, waiting: group.waiting_proxy }) }}
          <span v-if="!group.enabled && !group.waiting_proxy"> · {{ t('proxyHealth.paused') }}</span>
        </p>
      </div>
    </details>
    <p v-if="view" class="mt-3 text-xs leading-relaxed text-gray-500">{{ t('proxyHealth.evidenceHint') }}</p>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getProxyHealth, type ProxyHealth } from '@/api/admin/clash'
import { formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
const emit = defineEmits<{ manage: [] }>()
const { t } = useI18n()
const view = ref<ProxyHealth | null>(null)
const loading = ref(false)
const failed = ref(false)
const expanded = ref(false)
const problemSources = computed(() => (view.value?.sources ?? []).filter(s => ['exhausted', 'expired', 'unavailable', 'degraded', 'fetch_failed', 'stale'].includes(s.status)))
const limitedGroups = computed(() => (view.value?.groups ?? []).filter(g => g.available <= 1))
let alive = true
let timer: ReturnType<typeof setInterval> | undefined
function bytes(n: number | null) {
  if (n === null) return '—'
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MiB`
  return `${(n / 1024 ** 3).toFixed(2)} GiB`
}
function severity(status: string) {
  if (['exhausted', 'expired', 'unavailable'].includes(status)) return 'border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
  if (['degraded', 'fetch_failed', 'stale'].includes(status)) return 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300'
  return 'border-gray-200 text-gray-700 dark:border-dark-600 dark:text-gray-300'
}
function tone(status: string) {
  if (status === 'healthy') return 'healthy'
  if (['exhausted', 'expired', 'unavailable'].includes(status)) return 'danger'
  if (['degraded', 'fetch_failed', 'stale'].includes(status)) return 'warning'
  return 'neutral'
}
function fetchReason(reason: string, status?: number) {
  if (reason === 'http_error') return `HTTP ${status || '—'}`
  if (['invalid_url', 'connection_failed', 'read_failed', 'invalid_subscription', 'not_checked'].includes(reason)) return t(`proxyHealth.fetchReasons.${reason}`)
  return t('proxyHealth.fetchReasons.unknown')
}
async function refresh() {
  if (loading.value) return
  loading.value = true
  try { const data = await getProxyHealth(); if (alive) { view.value = data; failed.value = false } }
  catch { if (alive) failed.value = true }
  finally { loading.value = false }
}
onMounted(() => { void refresh(); timer = setInterval(() => { if (!document.hidden) void refresh() }, 30_000) })
onUnmounted(() => { alive = false; clearInterval(timer) })
</script>

<style scoped>
.subscription-card--healthy {
  @apply border-emerald-200 dark:border-emerald-900;
}
.subscription-card--warning {
  @apply border-amber-200 dark:border-amber-900;
}
.subscription-card--danger {
  @apply border-red-200 dark:border-red-900;
}
.subscription-card--neutral {
  @apply border-gray-200 dark:border-dark-600;
}
.subscription-watermark {
  position: absolute;
  top: 1.1rem;
  right: 1rem;
  max-width: 8rem;
  padding: 0.35rem 0.65rem;
  border: 3px double currentColor;
  border-radius: 0.4rem;
  font-size: 1.25rem;
  font-weight: 800;
  line-height: 1.15;
  text-align: center;
  letter-spacing: 0.08em;
  transform: rotate(-10deg);
  opacity: 0.3;
  pointer-events: none;
  user-select: none;
}
.subscription-card--healthy .subscription-watermark {
  @apply text-emerald-600 dark:text-emerald-400;
}
.subscription-card--warning .subscription-watermark {
  @apply text-amber-600 dark:text-amber-400;
}
.subscription-card--danger .subscription-watermark {
  @apply text-red-600 dark:text-red-400;
}
.subscription-card--neutral .subscription-watermark {
  @apply text-gray-500 dark:text-gray-400;
}
</style>
