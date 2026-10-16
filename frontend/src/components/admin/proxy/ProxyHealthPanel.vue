<template>
  <section class="mt-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800" :aria-label="t('proxyHealth.title')">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h3 class="text-sm font-semibold">{{ t('proxyHealth.title') }}</h3>
      <div class="flex w-full flex-wrap items-center gap-x-3 gap-y-2 text-xs sm:w-auto">
        <span v-if="view" class="w-full whitespace-nowrap text-gray-500 sm:w-auto">{{ t('proxyHealth.updated') }} {{ formatDateTime(view.checked_at) }}</span>
        <button class="shrink-0 whitespace-nowrap text-primary-600 hover:underline" @click="emit('manage')">{{ t('proxyHealth.manage') }}</button>
        <button class="shrink-0 whitespace-nowrap text-primary-600 hover:underline" :disabled="loading" @click="refresh">{{ t('common.refresh') }}</button>
        <button class="shrink-0 whitespace-nowrap text-primary-600 hover:underline" :aria-expanded="expanded" @click="expanded = !expanded">{{ t(expanded ? 'proxyHealth.collapse' : 'proxyHealth.expand') }}</button>
      </div>
    </div>
    <p v-if="failed" role="status" class="mt-2 text-xs text-amber-600">{{ t('proxyHealth.readFailed') }}</p>
    <p v-else-if="loading && !view" class="mt-2 text-xs text-gray-500">{{ t('common.loading') }}</p>
    <p v-if="view?.sources.length" class="mt-2 text-xs text-gray-500">{{ t('proxyHealth.sourceSummary', { total: view.sources.length, abnormal: problemSources.length }) }}</p>
    <div v-if="!expanded && problemSources.length" class="mt-2 max-h-20 space-y-1 overflow-auto">
      <p v-for="source in problemSources" :key="source.id" class="break-all rounded-md px-2 py-1 text-xs" :class="severity(source.status)">
        {{ t('proxyHealth.source', { index: source.index }) }} · {{ t(`proxyHealth.states.${source.status}`) }} · {{ source.url_hint }} · {{ source.id.slice(-6) }}
      </p>
    </div>
    <div v-if="expanded && view?.sources.length" class="mt-3 grid max-h-[42vh] gap-3 overflow-y-auto sm:grid-cols-2 xl:grid-cols-3">
      <article v-for="source in view.sources" :key="source.id" class="min-w-0 rounded-lg border p-3" :class="severity(source.status)">
        <div class="flex items-start justify-between gap-2">
          <span class="text-sm font-medium">{{ t('proxyHealth.source', { index: source.index }) }}</span>
          <span class="shrink-0 text-xs font-medium">{{ t(`proxyHealth.states.${source.status}`) }}</span>
        </div>
        <p class="mt-1 break-all font-mono text-xs">{{ source.url_hint }} · {{ source.id.slice(-6) }}</p>
        <p class="mt-2 text-xs">{{ source.traffic_known ? t('proxyHealth.remaining', { remaining: bytes(source.remaining_bytes), total: bytes(source.total_bytes) }) : t('proxyHealth.trafficUnknown') }}</p>
        <p v-if="source.expires_at" class="mt-1 text-xs">{{ t('proxyHealth.expires') }} {{ formatDateTime(source.expires_at) }}</p>
        <p class="mt-1 text-xs">{{ t('proxyHealth.nodes', { healthy: source.healthy, failed: source.failed, unknown: source.unknown, total: source.nodes }) }}</p>
        <p v-if="source.status === 'exhausted' || source.status === 'expired'" class="mt-2 text-xs font-medium">{{ t('proxyHealth.excluded') }}</p>
        <p v-if="source.fetch_error" class="mt-2 text-xs">{{ t('proxyHealth.fetchFailed', { reason: fetchReason(source.fetch_error, source.http_status) }) }}</p>
        <p v-if="source.traffic_observed_at" class="mt-1 text-xs opacity-75">{{ t('proxyHealth.observed') }} {{ formatDateTime(source.traffic_observed_at) }}</p>
      </article>
    </div>
    <p v-else-if="view && !view.sources.length" class="mt-2 text-xs text-gray-500">{{ t('proxyHealth.noSources') }}</p>
    <details v-if="limitedGroups.length" class="mt-3 text-sm" :open="expanded">
      <summary class="cursor-pointer font-medium">{{ t('proxyHealth.capacity', { count: limitedGroups.length }) }}</summary>
      <div class="mt-2 max-h-44 space-y-1 overflow-auto">
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
function fetchReason(reason: string, status?: number) {
  if (reason === 'http_error') return `HTTP ${status || '—'}`
  if (['invalid_url', 'connection_failed', 'read_failed', 'invalid_subscription'].includes(reason)) return t(`proxyHealth.fetchReasons.${reason}`)
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
