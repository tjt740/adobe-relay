<template>
  <section class="mt-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800" :aria-label="t('proxyHealth.title')">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('proxyHealth.title') }}</h3>
        <p v-if="view?.sources.length" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('proxyHealth.sourceSummary', { total: view.sources.length, abnormal: problemSources.length }) }}</p>
      </div>
      <div class="flex w-full flex-wrap items-center gap-x-4 gap-y-2 text-xs sm:w-auto">
        <span v-if="view" class="w-full text-gray-500 dark:text-gray-400 sm:w-auto">{{ t('proxyHealth.updated') }} {{ formatDateTime(view.checked_at) }}</span>
        <button class="health-action" @click="emit('manage')"><Icon name="link" size="sm" aria-hidden="true" />{{ t('proxyHealth.manage') }}</button>
        <button class="health-action" :disabled="loading" @click="refresh"><Icon name="refresh" size="sm" :class="{ 'motion-safe:animate-spin': loading }" aria-hidden="true" />{{ t('common.refresh') }}</button>
        <button class="health-action" :aria-expanded="expanded" @click="expanded = !expanded"><Icon :name="expanded ? 'chevronUp' : 'chevronDown'" size="sm" aria-hidden="true" />{{ t(expanded ? 'proxyHealth.collapse' : 'proxyHealth.expand') }}</button>
      </div>
    </div>
    <p v-if="failed" role="status" class="mt-3 text-sm text-amber-700 dark:text-amber-300">{{ t('proxyHealth.readFailed') }}</p>
    <p v-else-if="loading && !view" class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</p>
    <div v-if="view?.sources.length" class="mt-4 grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
      <ProxySourceHealthCard v-for="source in view.sources" :key="source.id" :source="source" :expanded="expanded" @details="selectedSourceID = source.id" />
    </div>
    <p v-else-if="view && !view.sources.length" class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t('proxyHealth.noSources') }}</p>
    <div v-if="view?.groups.length" class="mt-5 border-t border-gray-200 pt-4 dark:border-dark-600">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h4 class="text-sm font-medium">{{ t('proxyHealth.groupTitle') }} <span class="ml-2 text-xs font-normal text-gray-500 dark:text-gray-400">{{ t('proxyHealth.capacity', { count: limitedGroups.length }) }}</span></h4>
        <button class="health-action text-xs" :aria-pressed="onlyLimited" @click="onlyLimited = !onlyLimited">{{ t(onlyLimited ? 'proxyHealth.showAllGroups' : 'proxyHealth.onlyLimitedGroups') }}</button>
      </div>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('proxyHealth.groupRatio') }}</p>
      <div class="mt-3 grid gap-x-6 gap-y-4 md:grid-cols-2 2xl:grid-cols-3">
        <div v-for="group in shownGroups" :key="group.id" class="min-w-0 space-y-2" data-testid="group-capacity">
          <div class="flex items-baseline justify-between gap-3 text-sm">
            <p class="min-w-0 truncate" :title="group.name">{{ group.name }} <span class="text-xs text-gray-500 dark:text-gray-400">#{{ group.id }}</span></p>
            <span class="shrink-0 tabular-nums" :class="group.available >= 2 ? 'text-emerald-700 dark:text-emerald-300' : 'text-amber-700 dark:text-amber-300'">{{ group.available }} <span class="text-xs text-gray-500 dark:text-gray-400">/ {{ group.total }}</span></span>
          </div>
          <HealthCapacityBar :value="group.available" :total="group.total" :label="t('proxyHealth.groupProgress', { name: group.name })" :summary="t('proxyHealth.groupCapacity', { available: group.available, enabled: group.enabled, waiting: group.waiting_proxy })" :tone="group.available >= 2 ? 'green' : group.available ? 'amber' : 'gray'" />
          <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('proxyHealth.groupCapacity', { available: group.available, enabled: group.enabled, waiting: group.waiting_proxy }) }}</p>
          <p v-if="!group.enabled && !group.waiting_proxy" class="text-xs text-gray-500 dark:text-gray-400">{{ t('proxyHealth.paused') }}</p>
        </div>
      </div>
      <p v-if="!shownGroups.length" class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ t('proxyHealth.noLimitedGroups') }}</p>
    </div>
    <p v-if="view" class="mt-4 text-xs leading-relaxed text-gray-500 dark:text-gray-400">{{ t('proxyHealth.evidenceHint') }}</p>
    <BlockedProxyNodesModal :source="selectedSource" :checked-at="view?.checked_at ?? ''" :stale="failed" @close="selectedSourceID = null" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getProxyHealth, type ProxyHealth } from '@/api/admin/clash'
import { formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import HealthCapacityBar from './HealthCapacityBar.vue'
import ProxySourceHealthCard from './ProxySourceHealthCard.vue'
import BlockedProxyNodesModal from './BlockedProxyNodesModal.vue'
const emit = defineEmits<{ manage: []; loaded: [view: ProxyHealth] }>()
const { t } = useI18n()
const view = ref<ProxyHealth | null>(null)
const loading = ref(false)
const failed = ref(false)
const expanded = ref(false)
const onlyLimited = ref(false)
const selectedSourceID = ref<string | null>(null)
const selectedSource = computed(() => view.value?.sources.find(s => s.id === selectedSourceID.value) ?? null)
const problemSources = computed(() => (view.value?.sources ?? []).filter(s => s.generation_blocked > 0 || ['exhausted', 'expired', 'unavailable', 'degraded', 'fetch_failed', 'stale'].includes(s.status)))
const limitedGroups = computed(() => (view.value?.groups ?? []).filter(g => g.available <= 1))
const shownGroups = computed(() => onlyLimited.value ? limitedGroups.value : view.value?.groups ?? [])
let alive = true
let timer: ReturnType<typeof setInterval> | undefined
async function refresh() {
  if (loading.value) return
  loading.value = true
  try { const data = await getProxyHealth(); if (alive) { view.value = data; emit('loaded', data); failed.value = false } }
  catch { if (alive) failed.value = true }
  finally { loading.value = false }
}
onMounted(() => { void refresh(); timer = setInterval(() => { if (!document.hidden) void refresh() }, 30_000) })
onUnmounted(() => { alive = false; clearInterval(timer) })
</script>

<style scoped>
.health-action {
  @apply inline-flex min-h-8 shrink-0 items-center gap-2 rounded text-primary-600 hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-primary-400;
}
</style>
