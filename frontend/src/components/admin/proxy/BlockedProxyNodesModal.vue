<template>
  <BaseDialog :show="!!source" :title="t('proxyHealth.details.title')" width="wide" @close="emit('close')">
    <template v-if="source">
      <div class="space-y-2">
        <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
          {{ t('proxyHealth.source', { index: source.index }) }}
          <span class="ml-2 font-normal text-gray-500 dark:text-gray-400">{{ t('proxyHealth.details.count', { count: source.generation_blocked }) }}</span>
        </p>
        <p class="break-all text-xs text-gray-500 dark:text-gray-400">{{ source.url_hint }} · {{ source.id.slice(-6) }}</p>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('proxyHealth.updated') }} {{ formatDateTime(checkedAt) }}</p>
        <p v-if="stale" role="status" class="text-sm text-amber-700 dark:text-amber-300">{{ t('proxyHealth.readFailed') }}</p>
        <p class="text-sm leading-relaxed text-gray-600 dark:text-gray-400">{{ t('proxyHealth.details.hint') }}</p>
      </div>
      <div class="my-4 flex flex-col gap-2 sm:flex-row">
        <input v-model="search" type="search" class="input min-w-0 flex-1" :aria-label="t('proxyHealth.details.search')" :placeholder="t('proxyHealth.details.search')" />
        <select v-model="reason" class="input sm:w-44" :aria-label="t('proxyHealth.details.reasonFilter')">
          <option value="all">{{ t('proxyHealth.details.allReasons') }}</option>
          <option value="manual">{{ t('proxyHealth.details.manual') }}</option>
          <option value="cooldown">{{ t('proxyHealth.details.cooldown') }}</option>
        </select>
      </div>
      <p v-if="!source.blocked_nodes && source.generation_blocked" role="status" class="mb-3 text-sm text-amber-700 dark:text-amber-300">{{ t('proxyHealth.details.unavailable') }}</p>
      <p v-if="!filtered.length" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t(nodes.length ? 'proxyHealth.details.noMatch' : 'proxyHealth.details.none') }}</p>
      <ul v-else class="divide-y divide-gray-200 rounded-xl border border-gray-200 dark:divide-dark-600 dark:border-dark-600">
        <li v-for="node in pageNodes" :key="node.proxy_id" class="space-y-3 p-4" data-testid="blocked-node">
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="break-words text-sm font-medium text-gray-900 dark:text-gray-100">{{ node.name || t('proxyHealth.details.unnamed') }} <span class="font-normal text-gray-500 dark:text-gray-400">#{{ node.proxy_id }}</span></p>
              <p v-if="node.server" class="mt-1 break-all font-mono text-xs text-gray-500 dark:text-gray-400">{{ node.server }}:{{ node.server_port }}</p>
            </div>
            <div class="flex flex-wrap gap-2 text-xs">
              <span v-if="node.manual" class="rounded-md bg-red-50 px-2 py-1 text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ t('proxyHealth.details.manual') }}</span>
              <span v-if="node.retry_after" class="rounded-md bg-amber-50 px-2 py-1 text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">{{ t('proxyHealth.details.cooldown') }}</span>
            </div>
          </div>
          <p class="text-sm text-gray-600 dark:text-gray-300">
            {{ t(node.manual ? 'proxyHealth.details.manualHint' : node.retry_after ? 'proxyHealth.details.cooldownHint' : 'proxyHealth.details.unavailable') }}
          </p>
          <dl class="grid gap-x-6 gap-y-2 text-xs sm:grid-cols-2">
            <div v-if="node.excluded_at"><dt class="inline text-gray-500 dark:text-gray-400">{{ t('proxyHealth.details.excludedAt') }} </dt><dd class="inline tabular-nums">{{ formatDateTime(node.excluded_at) }}</dd></div>
            <div v-if="node.failed_at"><dt class="inline text-gray-500 dark:text-gray-400">{{ t('proxyHealth.details.failedAt') }} </dt><dd class="inline tabular-nums">{{ formatDateTime(node.failed_at) }}</dd></div>
            <div v-if="node.retry_after"><dt class="inline text-gray-500 dark:text-gray-400">{{ t('proxyHealth.details.retryAfter') }} </dt><dd class="inline tabular-nums">{{ formatDateTime(node.retry_after) }}</dd></div>
            <div v-if="source.blocked_nodes"><dt class="inline text-gray-500 dark:text-gray-400">{{ t('proxyHealth.details.accounts') }} </dt><dd class="inline tabular-nums">{{ node.bound_accounts }}</dd></div>
          </dl>
          <p v-if="source.blocked_nodes && !node.enabled" class="text-xs text-gray-500 dark:text-gray-400">{{ t('proxyHealth.details.disabled') }}</p>
        </li>
      </ul>
      <div v-if="filtered.length" class="mt-4 flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ t('proxyHealth.details.results', { count: filtered.length, page, pages }) }}</span>
        <div v-if="pages > 1" class="flex gap-2">
          <button class="btn btn-secondary" :disabled="page === 1" @click="page--">{{ t('proxyHealth.details.previous') }}</button>
          <button class="btn btn-secondary" :disabled="page === pages" @click="page++">{{ t('proxyHealth.details.next') }}</button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { BlockedProxyNode, ProxySourceHealth } from '@/api/admin/clash'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { formatDateTime } from '@/utils/format'
const props = defineProps<{ source: ProxySourceHealth | null; checkedAt: string; stale: boolean }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const search = ref('')
const reason = ref('all')
const page = ref(1)
const nodes = computed<BlockedProxyNode[]>(() => props.source?.blocked_nodes ?? (props.source?.blocked_proxy_ids ?? []).map(proxy_id => ({
  proxy_id, name: '', server: '', server_port: 0, enabled: false, bound_accounts: 0,
  manual: false, excluded_at: null, failed_at: null, retry_after: null,
})))
const filtered = computed(() => nodes.value.filter(node =>
  (reason.value === 'all' || (reason.value === 'manual' ? node.manual : !!node.retry_after)) &&
  [node.name, node.proxy_id, node.server].join(' ').toLowerCase().includes(search.value.trim().toLowerCase())
))
const pages = computed(() => Math.max(1, Math.ceil(filtered.value.length / 20)))
const pageNodes = computed(() => filtered.value.slice((page.value - 1) * 20, page.value * 20))
watch(() => props.source?.id, () => { search.value = ''; reason.value = 'all'; page.value = 1 })
watch([search, reason], () => { page.value = 1 })
watch(pages, count => { page.value = Math.min(page.value, count) })
</script>
