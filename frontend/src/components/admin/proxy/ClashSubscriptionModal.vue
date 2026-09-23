<template>
  <BaseDialog :show="true" :title="t('admin.proxies.clash.title')" width="extra-wide"
    :show-close-button="!busy" :close-on-escape="!busy" @close="close">
    <div class="space-y-5" :aria-busy="busy">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.proxies.clash.description') }}</p>
        <span v-if="state" class="inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
          :class="state.ready ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400' : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'">
          <Icon :name="state.ready ? 'checkCircle' : 'exclamationCircle'" size="sm" />
          {{ t(state.ready ? 'admin.proxies.clash.ready' : 'admin.proxies.clash.unavailable') }}
        </span>
      </div>
      <p v-if="state && !state.configured" class="text-sm text-amber-600">{{ t('admin.proxies.clash.setupHint') }}</p>

      <form class="space-y-3 rounded-xl border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-600 dark:bg-dark-700/40" @submit.prevent="preview">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <label for="clash-subscription-url" class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.proxies.clash.url') }}</label>
          <span v-if="state?.revision" class="text-xs text-gray-500">{{ t('admin.proxies.clash.updated') }} {{ formatDateTime(state.updated_at) }}</span>
        </div>
        <div class="flex flex-col gap-2 sm:flex-row">
          <div class="relative min-w-0 flex-1">
            <Icon name="link" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input id="clash-subscription-url" v-model="url" :type="showURL ? 'text' : 'password'" autocomplete="off" autocapitalize="off" spellcheck="false"
              class="input h-11 pl-9 pr-11 font-mono text-sm" :disabled="busy"
              :placeholder="t(state?.url_hint ? 'admin.proxies.clash.keepURL' : 'admin.proxies.clash.urlPlaceholder')" @input="invalidatePreview" />
            <button type="button" class="absolute right-1 top-1/2 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 hover:text-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-600 dark:hover:text-gray-200"
              :aria-label="t(showURL ? 'admin.proxies.clash.hideURL' : 'admin.proxies.clash.showURL')" :title="t(showURL ? 'admin.proxies.clash.hideURL' : 'admin.proxies.clash.showURL')"
              :aria-pressed="showURL" @click="showURL = !showURL">
              <Icon :name="showURL ? 'eyeOff' : 'eye'" size="md" />
            </button>
          </div>
          <button type="submit" class="btn btn-secondary h-11 shrink-0" :disabled="busy || (!url.trim() && !state?.url_hint)">
            <Icon name="refresh" size="sm" :class="previewing ? 'motion-safe:animate-spin' : ''" />
            {{ t(previewing ? 'admin.proxies.clash.previewing' : 'admin.proxies.clash.preview') }}
          </button>
        </div>
        <p class="text-xs leading-relaxed text-gray-500">{{ t('admin.proxies.clash.replaceHint') }}</p>
      </form>

      <p v-if="errorMessage" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-600 dark:bg-red-900/20">{{ errorMessage }}</p>
      <p v-if="successMessage" role="status" class="flex items-center gap-2 text-sm text-emerald-600 dark:text-emerald-400"><Icon name="checkCircle" size="sm" />{{ successMessage }}</p>

      <div v-if="loading || previewing" role="status" class="rounded-xl border border-gray-200 p-5 dark:border-dark-600" data-testid="clash-loading">
        <div class="mb-5 flex items-center gap-3">
          <Icon name="refresh" size="md" class="text-primary-500 motion-safe:animate-spin" />
          <div>
            <p class="text-sm font-medium">{{ t(previewing ? 'admin.proxies.clash.previewing' : 'common.loading') }}</p>
            <p class="mt-1 text-xs text-gray-500">{{ t(previewing ? 'admin.proxies.clash.previewHint' : 'admin.proxies.clash.loadingHint') }}</p>
          </div>
        </div>
        <div aria-hidden="true" class="space-y-3 motion-safe:animate-pulse">
          <div v-for="i in 3" :key="i" class="flex items-center gap-4 rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
            <div class="h-9 w-9 rounded-lg bg-gray-200 dark:bg-dark-600"></div>
            <div class="flex-1 space-y-2"><div class="h-3 w-2/5 rounded bg-gray-200 dark:bg-dark-600"></div><div class="h-2 w-3/5 rounded bg-gray-100 dark:bg-dark-600"></div></div>
            <div class="h-5 w-14 rounded-full bg-gray-200 dark:bg-dark-600"></div>
          </div>
        </div>
      </div>
      <template v-else>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div v-for="stat in stats" :key="stat.label" class="min-w-0 rounded-xl border border-gray-200 p-3 dark:border-dark-600">
            <p class="text-xs text-gray-500">{{ stat.label }}</p>
            <p class="mt-1 text-2xl font-semibold tabular-nums" :class="stat.color">{{ stat.value }}</p>
            <p class="mt-1 text-xs text-gray-400">{{ stat.detail }}</p>
          </div>
        </div>

        <div v-if="saving || testing" class="rounded-xl bg-primary-50 p-3 dark:bg-primary-900/20" role="status">
          <div class="flex items-center gap-2 text-sm font-medium text-primary-700 dark:text-primary-300">
            <Icon name="refresh" size="sm" class="motion-safe:animate-spin" />
            {{ saving ? t('admin.proxies.clash.importing') : t('admin.proxies.clash.testing', { done: testDone, total: testTotal }) }}
          </div>
          <p v-if="saving" class="mt-1 text-xs text-primary-600 dark:text-primary-400">{{ t('admin.proxies.clash.savingHint') }}</p>
          <progress v-if="testing" :value="testDone" :max="testTotal || 1" :aria-label="t('admin.proxies.clash.testProgress')" class="clash-progress mt-3 h-1.5 w-full overflow-hidden rounded-full"></progress>
        </div>

        <div class="space-y-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-center gap-2 text-sm">
              <h4 class="font-semibold">{{ t('admin.proxies.clash.nodeList') }}</h4>
              <span v-if="hasPreview" class="rounded-full bg-primary-50 px-2 py-0.5 text-xs text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">{{ t('admin.proxies.clash.selectedCount', { count: selected.size }) }}</span>
            </div>
            <div class="flex flex-wrap gap-2">
              <button v-if="hasPreview" class="btn btn-secondary btn-sm" :disabled="busy" @click="selectAll">
                {{ t(selected.size === nodes.length ? 'admin.proxies.clash.selectNone' : 'admin.proxies.clash.selectAll') }}
              </button>
              <button class="btn btn-secondary btn-sm" :disabled="busy || !testableNodes.length" @click="testNodes">
                <Icon :name="testing ? 'refresh' : 'play'" size="sm" :class="testing ? 'motion-safe:animate-spin' : ''" />
                {{ testing ? t('admin.proxies.clash.testing', { done: testDone, total: testTotal }) : t('admin.proxies.clash.test') }}
              </button>
            </div>
          </div>
          <div v-if="nodes.length" class="flex flex-wrap items-center gap-2">
            <div class="relative min-w-0 flex-1 sm:max-w-xs">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="search" class="input h-9 py-1.5 pl-9 text-sm" :aria-label="t('admin.proxies.clash.searchNodes')" :placeholder="t('admin.proxies.clash.searchNodes')" />
            </div>
            <span v-for="protocol in protocols" :key="protocol.name" class="rounded-md bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ protocol.name }} · {{ protocol.count }}</span>
          </div>
          <div v-if="filteredNodes.length" class="max-h-80 overflow-auto rounded-xl border border-gray-200 dark:border-dark-600">
            <table class="w-full min-w-[640px] text-left text-sm">
              <thead class="sticky top-0 z-10 bg-gray-50 text-xs text-gray-500 dark:bg-dark-700">
                <tr>
                  <th v-if="hasPreview" class="w-10 p-3"><span class="sr-only">{{ t('admin.proxies.clash.selectAll') }}</span></th>
                  <th class="p-3">{{ t('admin.proxies.clash.nodeInfo') }}</th>
                  <th class="p-3">{{ t('admin.proxies.columns.protocol') }}</th>
                  <th class="p-3">{{ t('admin.proxies.clash.assignment') }}</th>
                  <th class="p-3">{{ t('admin.proxies.clash.health') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
                <tr v-for="node in filteredNodes" :key="node.name" class="hover:bg-gray-50/70 dark:hover:bg-dark-700/40">
                  <td v-if="hasPreview" class="p-3"><input type="checkbox" :aria-label="node.name" :checked="selected.has(node.name)" :disabled="busy" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" @change="toggle(node.name)" /></td>
                  <td class="max-w-xs p-3">
                    <div class="break-words font-medium text-gray-800 dark:text-gray-100">{{ node.name }}</div>
                    <div class="mt-1 break-all font-mono text-xs text-gray-400">{{ endpoint(node.server, node.server_port) }}</div>
                  </td>
                  <td class="p-3"><span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium uppercase text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ node.type }}</span></td>
                  <td class="p-3">
                    <div class="flex items-center gap-1.5 whitespace-nowrap text-xs" :class="node.proxy_id && node.enabled ? 'text-primary-600 dark:text-primary-400' : 'text-gray-500'">
                      <span class="h-1.5 w-1.5 rounded-full" :class="node.proxy_id && node.enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-500'"></span>
                      {{ t(!node.proxy_id ? 'admin.proxies.clash.pending' : node.enabled ? 'admin.proxies.clash.assigned' : 'admin.proxies.clash.disabled') }}
                      <span v-if="node.proxy_id" class="text-gray-400">#{{ node.proxy_id }}</span>
                    </div>
                    <div v-if="node.proxy_host" class="mt-1 break-all font-mono text-xs text-gray-400">{{ endpoint(node.proxy_host, node.proxy_port) }}</div>
                  </td>
                  <td class="max-w-[200px] p-3">
                    <span class="inline-flex items-center gap-1.5 rounded-full px-2 py-1 text-xs" :class="healthClass(node)">
                      <Icon v-if="testingIDs.has(node.proxy_id)" name="refresh" size="xs" class="shrink-0 motion-safe:animate-spin" />
                      <Icon v-else-if="results[node.proxy_id]" :name="results[node.proxy_id].success ? 'checkCircle' : 'xCircle'" size="xs" class="shrink-0" />
                      {{ healthLabel(node) }}
                    </span>
                    <p v-if="results[node.proxy_id] && !results[node.proxy_id].success" class="mt-1 break-words text-xs text-red-500">{{ results[node.proxy_id].message }}</p>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-else class="rounded-xl border border-dashed border-gray-200 px-4 py-8 text-center dark:border-dark-600">
            <Icon :name="nodes.length ? 'search' : 'server'" size="xl" class="mx-auto mb-3 text-gray-300 dark:text-dark-500" />
            <p class="text-sm font-medium text-gray-600 dark:text-gray-300">{{ t(nodes.length ? 'admin.proxies.clash.noMatches' : 'admin.proxies.clash.emptyTitle') }}</p>
            <p v-if="!nodes.length" class="mt-1 text-xs text-gray-400">{{ t('admin.proxies.clash.empty') }}</p>
          </div>
        </div>
        <p v-if="hasPreview" class="rounded-lg bg-amber-50 p-3 text-xs leading-relaxed text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">{{ t('admin.proxies.clash.importHint') }}</p>
        <p class="text-xs leading-relaxed text-gray-500">{{ t('admin.proxies.clash.useHint') }}</p>
      </template>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="busy" @click="close">{{ t('common.close') }}</button>
      <button type="button" class="btn btn-primary" :disabled="busy || !hasPreview || !state?.configured" @click="save">
        <Icon :name="saving ? 'refresh' : 'download'" size="sm" :class="saving ? 'motion-safe:animate-spin' : ''" />
        {{ saving ? t('admin.proxies.clash.importing') : t('admin.proxies.clash.import') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { getClashSubscription, previewClashSubscription, importClashSubscription, type ClashNode, type ClashSubscription } from '@/api/admin/clash'
import { testProxy } from '@/api/admin/proxies'
import { formatDateTime } from '@/utils/format'

const emit = defineEmits<{ close: []; changed: [] }>()
const { t } = useI18n()
const state = ref<ClashSubscription | null>(null)
const nodes = ref<ClashNode[]>([])
const url = ref('')
const showURL = ref(true)
const search = ref('')
const selected = ref(new Set<string>())
const hasPreview = ref(false)
const previewRevision = ref(0)
const loading = ref(true)
const previewing = ref(false)
const saving = ref(false)
const testing = ref(false)
const testingIDs = ref(new Set<number>())
const testDone = ref(0)
const testTotal = ref(0)
const errorMessage = ref('')
const successMessage = ref('')
const results = ref<Record<number, { success: boolean; latency_ms?: number; message?: string }>>({})
const busy = computed(() => loading.value || previewing.value || saving.value || testing.value)
const testableNodes = computed(() => (state.value?.nodes ?? []).filter(n => n.enabled && n.proxy_id))
const healthyCount = computed(() => Object.values(results.value).filter(r => r.success).length)
const protocols = computed(() => {
  const counts = new Map<string, number>()
  for (const node of nodes.value) counts.set(node.type.toUpperCase(), (counts.get(node.type.toUpperCase()) ?? 0) + 1)
  return [...counts].map(([name, count]) => ({ name, count }))
})
const filteredNodes = computed(() => {
  const query = search.value.trim().toLowerCase()
  return nodes.value.filter(n => `${n.name} ${n.type} ${n.server ?? ''} ${n.server_port ?? ''} ${n.proxy_id || ''}`.toLowerCase().includes(query))
})
const stats = computed(() => [
  { label: t('admin.proxies.clash.totalNodes'), value: nodes.value.length, detail: t(hasPreview.value ? 'admin.proxies.clash.previewData' : 'admin.proxies.clash.savedData'), color: 'text-gray-900 dark:text-white' },
  { label: t('admin.proxies.clash.protocolCount'), value: protocols.value.length, detail: t('admin.proxies.clash.protocolHint'), color: 'text-gray-900 dark:text-white' },
  { label: t('admin.proxies.clash.assignedNodes'), value: testableNodes.value.length, detail: t('admin.proxies.clash.assignedHint'), color: 'text-primary-600 dark:text-primary-400' },
  { label: t('admin.proxies.clash.healthyNodes'), value: Object.keys(results.value).length ? healthyCount.value : '—', detail: t('admin.proxies.clash.testedCount', { count: Object.keys(results.value).length }), color: 'text-emerald-600 dark:text-emerald-400' }
])
let disposed = false
onUnmounted(() => { disposed = true })

function errorText(error: unknown): string {
  const e = error as { message?: string; response?: { data?: { message?: string; detail?: string } } }
  return e.response?.data?.detail || e.response?.data?.message || e.message || t('admin.proxies.clash.failed')
}
function close() { if (!busy.value) emit('close') }
function requestURL() { return url.value.trim() === state.value?.url ? '' : url.value.trim() }
function invalidatePreview() { hasPreview.value = false; nodes.value = state.value?.nodes ?? []; search.value = ''; successMessage.value = ''; errorMessage.value = '' }
function toggle(name: string) { const next = new Set(selected.value); if (next.has(name)) next.delete(name); else next.add(name); selected.value = next }
function selectAll() { selected.value = selected.value.size === nodes.value.length ? new Set() : new Set(nodes.value.map(n => n.name)) }
function endpoint(host?: string, port?: number) { return host ? `${host.includes(':') && !host.startsWith('[') ? `[${host}]` : host}${port ? `:${port}` : ''}` : '—' }
function healthLabel(node: ClashNode) {
  if (testingIDs.value.has(node.proxy_id)) return t('admin.proxies.clash.nodeTesting')
  const result = results.value[node.proxy_id]
  if (result) return result.success ? `${result.latency_ms ?? 0} ms` : t('admin.proxies.clash.unhealthy')
  if (!node.proxy_id) return t('admin.proxies.clash.testAfterImport')
  if (!node.enabled) return t('admin.proxies.clash.disabled')
  return t(testing.value ? 'admin.proxies.clash.queued' : 'admin.proxies.clash.notTested')
}
function healthClass(node: ClashNode) {
  if (testingIDs.value.has(node.proxy_id)) return 'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300'
  const result = results.value[node.proxy_id]
  if (result) return result.success ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400' : 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
}
async function load() {
  loading.value = true
  try { state.value = await getClashSubscription(); nodes.value = state.value.nodes; url.value = state.value.url ?? '' }
  catch (e) { errorMessage.value = errorText(e) }
  finally { loading.value = false }
}
async function preview() {
  if (busy.value) return
  previewing.value = true; errorMessage.value = ''; successMessage.value = ''; hasPreview.value = false; search.value = ''
  try {
    const data = await previewClashSubscription(requestURL())
    nodes.value = data.nodes; previewRevision.value = data.revision
    selected.value = new Set(data.nodes.filter(n => n.enabled).map(n => n.name)); hasPreview.value = true
  } catch (e) { nodes.value = state.value?.nodes ?? []; errorMessage.value = errorText(e) }
  finally { previewing.value = false }
}
async function save() {
  if (busy.value || !hasPreview.value || !state.value?.configured) return
  saving.value = true; errorMessage.value = ''; successMessage.value = ''
  try {
    state.value = await importClashSubscription(requestURL(), [...selected.value], previewRevision.value)
    nodes.value = state.value.nodes; hasPreview.value = false; url.value = state.value.url ?? url.value; results.value = {}; search.value = ''
    successMessage.value = t('admin.proxies.clash.imported', { count: state.value.nodes.filter(n => n.enabled).length })
    emit('changed')
  } catch (e) { errorMessage.value = errorText(e) }
  finally { saving.value = false }
}
async function testNodes() {
  if (busy.value || !testableNodes.value.length) return
  testing.value = true; testDone.value = 0; errorMessage.value = ''; successMessage.value = ''; results.value = {}
  const queue = [...testableNodes.value]; testTotal.value = queue.length
  async function worker() {
    while (queue.length && !disposed) {
      const node = queue.shift()!
      testingIDs.value.add(node.proxy_id)
      try { results.value[node.proxy_id] = await testProxy(node.proxy_id) }
      catch (e) { results.value[node.proxy_id] = { success: false, message: errorText(e) } }
      finally { testingIDs.value.delete(node.proxy_id); testDone.value++ }
    }
  }
  try {
    await Promise.all(Array.from({ length: Math.min(4, queue.length) }, worker))
    successMessage.value = t('admin.proxies.clash.testComplete', { success: healthyCount.value, failed: testTotal.value - healthyCount.value })
    emit('changed')
  } finally { testing.value = false }
}
onMounted(load)
</script>

<style scoped>
.clash-progress {
  appearance: none;
}
.clash-progress::-webkit-progress-bar {
  @apply rounded-full bg-primary-100 dark:bg-primary-900/40;
}
.clash-progress::-webkit-progress-value {
  @apply rounded-full bg-primary-500 transition-all;
}
.clash-progress::-moz-progress-bar {
  @apply rounded-full bg-primary-500;
}
</style>
