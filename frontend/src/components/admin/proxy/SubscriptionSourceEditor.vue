<template>
  <div class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800" :aria-label="t('admin.proxies.clash.inputMode')">
        <button v-for="option in modes" :key="option.id" type="button" class="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium"
          :class="mode === option.id ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-600 dark:text-primary-300' : 'text-gray-500 dark:text-gray-400'"
          :aria-pressed="mode === option.id" @click="mode = option.id">
          <Icon :name="option.icon" size="sm" aria-hidden="true" />{{ t(option.label) }}
        </button>
      </div>
      <button type="button" class="inline-flex items-center gap-1.5 text-xs text-gray-600 dark:text-gray-300"
        :aria-pressed="showURL" :aria-label="t(showURL ? 'admin.proxies.clash.hideURL' : 'admin.proxies.clash.showURL')" @click="showURL = !showURL">
        <Icon :name="showURL ? 'eyeOff' : 'eye'" size="sm" aria-hidden="true" />{{ t(showURL ? 'admin.proxies.clash.hideURL' : 'admin.proxies.clash.showURL') }}
      </button>
    </div>
    <template v-if="mode === 'list'">
      <div v-for="(row, index) in rows" :key="row.id" class="rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800" data-testid="subscription-row">
        <div class="mb-2 flex items-center justify-between gap-2">
          <label :for="index === 0 ? 'clash-subscription-url' : `clash-source-${row.id}`" class="text-xs font-medium text-gray-500">{{ t('proxyHealth.source', { index: index + 1 }) }}</label>
          <div class="flex shrink-0 items-center gap-3">
            <button type="button" class="inline-flex items-center gap-1 text-xs text-primary-600 disabled:opacity-50 dark:text-primary-400" :disabled="disabled || !row.url.trim() || checking.has(row.id)" @click="check(row)">
              <Icon :name="checking.has(row.id) ? 'refresh' : 'play'" size="xs" :class="{ 'motion-safe:animate-spin': checking.has(row.id) }" aria-hidden="true" />
              {{ t(checking.has(row.id) ? 'admin.proxies.clash.sourceChecking' : 'admin.proxies.clash.checkSource') }}
            </button>
            <button type="button" class="rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-50 dark:hover:bg-red-900/20" :disabled="disabled || checking.size > 0 || rows.length === 1" :title="rows.length === 1 ? t('admin.proxies.clash.keepOneSource') : undefined" :aria-label="t('admin.proxies.clash.removeSource', { index: index + 1 })" @click="remove(index)">
              <Icon name="trash" size="sm" aria-hidden="true" />
            </button>
          </div>
        </div>
        <input :id="index === 0 ? 'clash-subscription-url' : `clash-source-${row.id}`" v-model="row.url" :type="showURL ? 'text' : 'password'" autocomplete="off" autocapitalize="off" spellcheck="false"
          class="input min-w-0 font-mono text-xs sm:text-sm" :disabled="disabled || checking.size > 0" :placeholder="t('admin.proxies.clash.singleURLPlaceholder')" @input="update" @paste="pasteRows($event, index)" />
        <p v-if="resultFor(row)" role="status" class="mt-2 flex items-start gap-1.5 break-words text-xs" :class="resultFor(row)?.fetch_error ? 'text-red-600 dark:text-red-400' : 'text-emerald-700 dark:text-emerald-400'">
          <Icon :name="resultFor(row)?.fetch_error ? 'exclamationCircle' : 'checkCircle'" size="xs" class="shrink-0" aria-hidden="true" />
          <span>{{ resultText(resultFor(row)!) }}<span v-if="resultFor(row)?.checked_at && !resultFor(row)!.checked_at!.startsWith('0001')" class="ml-2 text-gray-400">{{ formatDateTime(resultFor(row)!.checked_at!) }}</span></span>
        </p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="disabled || checking.size > 0" @click="add">
        <Icon name="plus" size="sm" aria-hidden="true" />{{ t('admin.proxies.clash.addSource') }}
      </button>
    </template>
    <div v-else class="space-y-2">
      <label for="clash-bulk-urls" class="sr-only">{{ t('admin.proxies.clash.url') }}</label>
      <textarea id="clash-bulk-urls" :value="modelValue" rows="6" autocomplete="off" autocapitalize="off" spellcheck="false" class="input min-h-40 resize-y font-mono text-sm leading-6"
        :class="{ 'clash-subscription-hidden': !showURL }" :disabled="disabled || checking.size > 0" :placeholder="t('admin.proxies.clash.urlPlaceholder')" @input="bulkUpdate" />
      <p class="text-xs text-gray-500">{{ t('admin.proxies.clash.bulkHint') }}</p>
    </div>
    <p class="text-xs leading-relaxed text-gray-500">{{ t('admin.proxies.clash.checkSourceHint') }}</p>
  </div>
</template>
<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import { checkClashSource, type ClashSource } from '@/api/admin/clash'

const props = defineProps<{ modelValue: string; disabled?: boolean; sources?: ClashSource[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; checking: [value: boolean] }>()
const { t } = useI18n()
const mode = ref<'list' | 'bulk'>('list')
const modes = [
  { id: 'list', icon: 'server', label: 'admin.proxies.clash.listMode' },
  { id: 'bulk', icon: 'clipboard', label: 'admin.proxies.clash.bulkMode' }
] as const
const showURL = ref(true)
let nextID = 0
type Row = { id: number; url: string }
const rows = ref<Row[]>([])
const checking = ref(new Set<number>())
const results = ref<Record<string, ClashSource>>({})
const sourceIndexByURL = computed(() => new Map([...new Set(rows.value.map(row => row.url.trim()).filter(Boolean))].map((url, index) => [url, index + 1])))
const sourcesByIndex = computed(() => new Map((props.sources ?? []).map(source => [source.index, source])))
let disposed = false
onUnmounted(() => { disposed = true })
watch(() => props.modelValue, value => {
  if (rows.value.length && rows.value.map(row => row.url).join('\n') === value) return
  rows.value = (value.trim() ? value.trim().split(/\s+/) : ['']).map(url => ({ id: ++nextID, url }))
}, { immediate: true })
function update() { emit('update:modelValue', rows.value.map(row => row.url).join('\n')) }
function add() { rows.value.push({ id: ++nextID, url: '' }); update() }
function remove(index: number) { rows.value.splice(index, 1); if (!rows.value.length) add(); else update() }
function bulkUpdate(event: Event) { emit('update:modelValue', (event.target as HTMLTextAreaElement).value) }
function pasteRows(event: ClipboardEvent, index: number) {
  const urls = event.clipboardData?.getData('text').trim().split(/\s+/) ?? []
  if (urls.length < 2) return
  event.preventDefault()
  rows.value.splice(index, 1, ...urls.map(url => ({ id: ++nextID, url }))); update()
}
function resultFor(row: Row) {
  const own = results.value[row.url.trim()]
  const source = sourcesByIndex.value.get(sourceIndexByURL.value.get(row.url.trim()) ?? 0)
  const observed = source && (source.fetch_error || (source.checked_at && !source.checked_at.startsWith('0001'))) ? source : undefined
  if (!own || !observed) return own ?? observed
  return (Date.parse(own.checked_at ?? '') || 0) > (Date.parse(observed.checked_at ?? '') || 0) ? own : observed
}
function resultText(result: ClashSource) {
  if (!result.fetch_error) return result.node_count > 0 ? t('admin.proxies.clash.sourceReadable', { count: result.node_count }) : t('admin.proxies.clash.sourceReadableUnknown')
  const reason = result.fetch_error === 'http_error' ? `HTTP ${result.http_status}` : t(`proxyHealth.fetchReasons.${result.fetch_error}`)
  return t('admin.proxies.clash.sourceFailed', { reason })
}
async function check(row: Row) {
  if (props.disabled || checking.value.has(row.id) || !row.url.trim()) return
  const url = row.url.trim()
  checking.value = new Set(checking.value).add(row.id); emit('checking', true)
  try { const result = await checkClashSource(url); if (!disposed) results.value[url] = { ...result, checked_at: result.checked_at || new Date().toISOString() } }
  catch { if (!disposed) results.value[url] = { id: '', index: 0, url_hint: '', node_count: 0, fetch_error: 'connection_failed', checked_at: new Date().toISOString() } }
  finally {
    checking.value = new Set([...checking.value].filter(id => id !== row.id))
    if (!disposed) emit('checking', checking.value.size > 0)
  }
}
</script>
<style scoped>
.clash-subscription-hidden { -webkit-text-security: disc; }
</style>
