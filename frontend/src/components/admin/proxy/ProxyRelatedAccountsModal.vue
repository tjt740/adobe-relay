<template>
  <BaseDialog :show="open" :title="t('proxyHealth.related.title')" width="extra-wide" :z-index="60" @close="close">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="break-words text-sm font-medium">{{ view?.proxy.name || node.name }} <span class="font-normal text-gray-500 dark:text-gray-400">#{{ node.proxy_id }}</span></p>
        <p v-if="view" class="mt-2 text-xs tabular-nums text-gray-500 dark:text-gray-400">{{ t('proxyHealth.updated') }} {{ formatDateTime(view.checked_at) }}</p>
      </div>
      <button class="btn btn-secondary" :disabled="loading" @click="refresh">{{ t('common.refresh') }}</button>
    </div>
    <p class="mt-3 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('proxyHealth.related.scopeHint') }}</p>
    <div v-if="view" class="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-sm tabular-nums">
      <span>{{ t('proxyHealth.related.currentCount', { count: view.current_count }) }}</span>
      <span>{{ t('proxyHealth.related.departedCount', { count: view.departed_count }) }}</span>
    </div>
    <div class="my-4 flex flex-col gap-2 sm:flex-row">
      <input v-model="search" type="search" class="input min-w-0 flex-1" :aria-label="t('proxyHealth.related.search')" :placeholder="t('proxyHealth.related.search')" />
      <select v-model="relation" class="input sm:w-64" :aria-label="t('proxyHealth.related.relationFilter')">
        <option value="all">{{ t('proxyHealth.related.all') }}</option>
        <option value="current">{{ t('proxyHealth.related.current') }}</option>
        <option value="departed">{{ t('proxyHealth.related.departed') }}</option>
      </select>
    </div>
    <p v-if="failed" role="status" class="mb-3 text-sm text-amber-700 dark:text-amber-300">{{ t('proxyHealth.related.readFailed') }}</p>
    <p v-if="loading" role="status" class="mb-3 text-xs text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</p>
    <p v-if="view && !view.items.length && !loading" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('proxyHealth.related.empty') }}</p>
    <ul v-if="view?.items.length" class="divide-y divide-gray-200 rounded-xl border border-gray-200 dark:divide-dark-600 dark:border-dark-600" :aria-busy="loading">
      <li v-for="account in view.items" :key="account.id" class="grid min-w-0 gap-4 p-4 md:grid-cols-3" data-testid="related-account">
        <div class="min-w-0">
          <p class="break-words text-sm font-medium">{{ account.name }} <span class="font-normal text-gray-500 dark:text-gray-400">#{{ account.id }}</span></p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ account.platform }}</p>
          <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ t('proxyHealth.related.mode') }}</p>
          <p class="mt-1 text-sm">{{ t(`proxyHealth.related.modes.${account.mode}`) }}</p>
        </div>
        <div class="min-w-0 text-sm">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('proxyHealth.related.currentNode') }}</p>
          <p class="mt-1 break-words">
            <template v-if="account.current_proxy">{{ proxyLabel(account.current_proxy) }}</template>
            <template v-else>{{ t(account.route_state === 'direct' ? 'proxyHealth.related.direct' : 'proxyHealth.related.noProxy') }}</template>
          </p>
          <p class="mt-1 text-xs" :class="account.using_this_node ? 'text-amber-700 dark:text-amber-300' : 'text-gray-500 dark:text-gray-400'">{{ t(account.using_this_node ? 'proxyHealth.related.stillHere' : 'proxyHealth.related.leftHere') }}</p>
          <div v-if="account.last_departure" class="mt-3 space-y-1 text-xs text-gray-500 dark:text-gray-400">
            <p>{{ t(account.using_this_node ? 'proxyHealth.related.returned' : 'proxyHealth.related.lastDeparture') }}</p>
            <p class="tabular-nums">{{ formatDateTime(account.last_departure.at) }}</p>
            <p class="break-words">{{ t('proxyHealth.related.destination') }} {{ account.last_departure.destination ? proxyLabel(account.last_departure.destination) : t('proxyHealth.related.unassigned') }}</p>
          </div>
          <p v-else class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ t('proxyHealth.related.noDeparture') }}</p>
        </div>
        <div class="min-w-0 space-y-2 text-xs">
          <p class="text-sm font-medium" :class="impactColor(account.impact)">{{ t(`proxyHealth.related.impacts.${account.impact}`) }}</p>
          <p><span class="mr-1 text-gray-500 dark:text-gray-400">{{ t('proxyHealth.related.accountState') }}</span>{{ t(`proxyHealth.related.accountStates.${account.account_state}`) }}</p>
          <p><span class="mr-1 text-gray-500 dark:text-gray-400">{{ t('proxyHealth.related.routeState') }}</span>{{ t(`proxyHealth.related.routeStates.${account.route_state}`) }}</p>
        </div>
      </li>
    </ul>
    <div v-if="view" class="mt-4 flex flex-wrap items-center justify-between gap-3 text-xs text-gray-500 dark:text-gray-400">
      <span>{{ t('proxyHealth.details.results', { count: view.total, page: view.page, pages }) }}</span>
      <div v-if="pages > 1" class="flex gap-2">
        <button class="btn btn-secondary" :disabled="loading || page <= 1" @click="changePage(page - 1)">{{ t('proxyHealth.details.previous') }}</button>
        <button class="btn btn-secondary" :disabled="loading || page >= pages" @click="changePage(page + 1)">{{ t('proxyHealth.details.next') }}</button>
      </div>
    </div>
    <p class="mt-4 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('proxyHealth.related.evidenceHint') }}</p>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { BlockedProxyNode } from '@/api/admin/clash'
import { getProxyRelatedAccounts, type ProxyRelatedAccountsView, type ProxyRelatedAccountsQuery, type RelatedProxyRef } from '@/api/admin/proxyRelatedAccounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { formatDateTime } from '@/utils/format'
const props = defineProps<{ node: Pick<BlockedProxyNode, 'proxy_id' | 'name'> }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const open = ref(true)
const view = ref<ProxyRelatedAccountsView | null>(null)
const loading = ref(false)
const failed = ref(false)
const search = ref('')
const relation = ref<ProxyRelatedAccountsQuery['relation']>('all')
const page = ref(1)
const pages = computed(() => Math.max(1, Math.ceil((view.value?.total ?? 0) / (view.value?.page_size ?? 20))))
let request = 0
let controller: AbortController | undefined
let debounce: ReturnType<typeof setTimeout> | undefined
let timer: ReturnType<typeof setInterval> | undefined
async function close() {
  open.value = false
  // Let BaseDialog restore focus to the association button before unmounting.
  await nextTick()
  emit('close')
}
function proxyLabel(proxy: RelatedProxyRef) { return `${proxy.name || t('proxyHealth.details.unnamed')} #${proxy.id}` }
function impactColor(impact: string) {
  return impact === 'affected' ? 'text-red-700 dark:text-red-300' : impact === 'unconfirmed' ? 'text-amber-700 dark:text-amber-300' : 'text-gray-700 dark:text-gray-200'
}
async function refresh() {
  clearTimeout(debounce)
  const seq = ++request
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  try {
    const data = await getProxyRelatedAccounts(props.node.proxy_id, { page: page.value, page_size: 20, relation: relation.value, search: search.value.trim() }, controller.signal)
    if (seq !== request) return
    const lastPage = Math.max(1, Math.ceil(data.total / data.page_size))
    if (data.page > lastPage) {
      page.value = lastPage
      void refresh()
      return
    }
    view.value = data
    failed.value = false
  } catch {
    if (seq === request) failed.value = true
  } finally {
    if (seq === request) loading.value = false
  }
}
function resetAndRefresh() {
  ++request
  controller?.abort()
  view.value = null
  failed.value = false
  page.value = 1
  clearTimeout(debounce)
  loading.value = true
  debounce = setTimeout(refresh, 250)
}
function changePage(next: number) { page.value = next; view.value = null; void refresh() }
watch([search, relation], resetAndRefresh)
watch(() => props.node.proxy_id, () => {
  search.value = ''; relation.value = 'all'; resetAndRefresh()
})
onMounted(() => {
  void refresh()
  timer = setInterval(() => { if (!document.hidden && !loading.value) void refresh() }, 30_000)
})
onUnmounted(() => { ++request; controller?.abort(); clearTimeout(debounce); clearInterval(timer) })
</script>
