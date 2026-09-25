<template>
  <section class="mt-3 rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-800">
    <div class="flex items-center justify-between gap-3">
      <label class="flex items-center gap-2 font-medium">
        <input v-model="enabled" type="checkbox" :disabled="!view || saving" @change="dirty = true" />
        {{ t('admin.accounts.proxyFailover.title') }}
      </label>
      <span v-if="view" class="text-sm" :class="view.state === 'healthy' ? 'text-teal-600' : 'text-gray-500'" aria-live="polite">
        {{ t(`admin.accounts.proxyFailover.states.${view.state}`) }}
      </span>
    </div>
    <p class="mt-2 text-xs text-gray-500">{{ t('admin.accounts.proxyFailover.hint') }}</p>
    <p v-if="error" role="alert" class="mt-2 text-sm text-red-600">{{ error }}</p>
    <template v-if="view && (enabled || view.enabled || view.events.length)">
      <div class="mt-3 space-y-1 text-sm">
        <p>{{ t('admin.accounts.proxyFailover.current') }}：{{ name(view.current_proxy_id) }}</p>
        <p>{{ t('admin.accounts.proxyFailover.primary') }}：{{ name(primary) }}</p>
      </div>
      <button v-if="view.current_proxy_id && primary !== view.current_proxy_id" type="button" class="mt-2 text-xs text-primary-600" @click="resetPrimary">
        {{ t('admin.accounts.proxyFailover.resetPrimary') }}
      </button>
      <div v-if="enabled" class="mt-3 space-y-2">
        <p class="text-sm font-medium">{{ t('admin.accounts.proxyFailover.backups') }}</p>
        <div v-for="(_, index) in backups" :key="index" class="flex items-center gap-2">
          <span class="text-xs text-gray-500">{{ index + 1 }}</span>
          <div class="min-w-0 flex-1">
            <ProxySelector :model-value="backups[index]" :proxies="choices(index)" :disabled="saving" @update:model-value="setBackup(index, $event)" />
          </div>
          <button type="button" class="px-1 disabled:opacity-30" :disabled="index === 0 || saving" :aria-label="t('admin.accounts.proxyFailover.moveUp')" @click="moveUp(index)">↑</button>
          <button type="button" class="px-1 text-red-500" :disabled="saving" :aria-label="t('common.delete')" @click="backups.splice(index, 1); dirty = true">×</button>
        </div>
        <button v-if="backups.length < 8" type="button" class="text-sm text-primary-600" :disabled="saving" @click="backups.push(null); dirty = true">+ {{ t('admin.accounts.proxyFailover.add') }}</button>
        <div class="space-y-1 text-xs text-gray-500">
          <p v-for="health in view.health" :key="health.proxy_id">
            {{ name(health.proxy_id) }} · {{ t(`admin.accounts.proxyFailover.health.${health.status}`) }}
            <span v-if="health.status === 'healthy'"> · {{ health.latency_ms }} ms</span>
            <span v-if="health.failures"> · {{ t('admin.accounts.proxyFailover.failures', { count: health.failures }) }}</span>
            <span v-if="!health.checked_at.startsWith('0001')"> · {{ formatTime(health.checked_at) }}</span>
          </p>
        </div>
      </div>
      <p v-if="pendingProxyChange" class="mt-3 text-xs text-amber-600">{{ t('admin.accounts.proxyFailover.pendingProxy') }}</p>
      <p v-if="view.events[0]" class="mt-3 text-xs text-gray-500">
        {{ t('admin.accounts.proxyFailover.lastSwitch') }}：{{ name(view.events[0].from_proxy_id) }} → {{ name(view.events[0].to_proxy_id) }} · {{ formatTime(view.events[0].created_at) }}
      </p>
    </template>
    <div class="mt-3 flex items-center gap-3">
      <button type="button" class="btn btn-primary btn-sm" :disabled="!canSave" @click="save">{{ t('admin.accounts.proxyFailover.save') }}</button>
      <button type="button" class="text-sm text-gray-500" :disabled="loading || saving" @click="refresh(true)">{{ t('common.refresh') }}</button>
      <span v-if="saved && !dirty" class="text-xs text-teal-600" role="status">{{ t('admin.accounts.proxyFailover.saved') }}</span>
    </div>
    <p class="mt-2 text-xs text-gray-500">{{ t('admin.accounts.proxyFailover.saveHint') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Proxy } from '@/types'
import ProxySelector from '@/components/common/ProxySelector.vue'
import { getProxyFailover, saveProxyFailover, type ProxyFailoverView } from '@/api/admin/proxyFailover'

const props = defineProps<{ accountId: number; selectedProxyId: number | null; proxies: Proxy[] }>()
const emit = defineEmits<{ current: [id: number | null] }>()
const { t } = useI18n()
const view = ref<ProxyFailoverView | null>(null)
const enabled = ref(false)
const primary = ref(0)
const backups = ref<Array<number | null>>([])
const revision = ref(0)
const dirty = ref(false)
const saved = ref(false)
const saving = ref(false)
const loading = ref(false)
const error = ref('')
let alive = true
let timer: ReturnType<typeof setInterval> | undefined
const pendingProxyChange = computed(() => (props.selectedProxyId || 0) !== view.value?.current_proxy_id)
const canSave = computed(() => view.value && dirty.value && !saving.value && !pendingProxyChange.value &&
  (!enabled.value || (primary.value > 0 && backups.value.length > 0 && backups.value.every(id => id && id !== primary.value) && new Set(backups.value).size === backups.value.length)))
const name = (id: number) => props.proxies.find(p => p.id === id)?.name || (id ? `#${id}` : t('admin.accounts.proxyFailover.noProxy'))
const formatTime = (value: string) => new Date(value).toLocaleString()
function choices(index: number) {
  return props.proxies.filter(p => p.id !== primary.value && !backups.value.some((id, i) => i !== index && id === p.id))
}
function setBackup(index: number, id: number | null) { backups.value[index] = id; dirty.value = true }
function moveUp(index: number) {
  const previous = backups.value[index - 1]
  backups.value[index - 1] = backups.value[index] ?? null
  backups.value[index] = previous ?? null
  dirty.value = true
}
function resetPrimary() {
  primary.value = view.value?.current_proxy_id || 0
  backups.value = backups.value.filter(id => id !== primary.value)
  dirty.value = true
}
function apply(v: ProxyFailoverView, reset: boolean) {
  view.value = v
  emit('current', v.current_proxy_id || null)
  if (reset) {
    enabled.value = v.enabled; primary.value = v.primary_proxy_id
    backups.value = [...v.backup_proxy_ids]; revision.value = v.revision; dirty.value = false
  }
}
async function refresh(reportError = false) {
  if (loading.value || saving.value) return
  loading.value = true
  try {
    const v = await getProxyFailover(props.accountId)
    if (alive) { apply(v, !dirty.value); error.value = '' }
  } catch {
    if (alive && (reportError || !view.value)) error.value = t('admin.accounts.proxyFailover.loadFailed')
  } finally { loading.value = false }
}
async function save() {
  if (!canSave.value || !view.value) return
  saving.value = true; error.value = ''; saved.value = false
  try {
    const v = await saveProxyFailover(props.accountId, {
      enabled: enabled.value, primary_proxy_id: primary.value, backup_proxy_ids: backups.value.filter((id): id is number => id !== null),
      revision: revision.value, current_proxy_id: view.value.current_proxy_id
    })
    if (alive) { apply(v, true); saved.value = true }
  } catch (err: unknown) {
    if (alive) {
      error.value = t('admin.accounts.proxyFailover.saveFailed')
      // A conflict must not overwrite another editor's policy on the next click.
      if ((err as { response?: { status?: number } })?.response?.status === 409) {
        view.value = null; dirty.value = false
      }
    }
  } finally { saving.value = false }
}
onMounted(() => { void refresh(); timer = setInterval(() => { if (!document.hidden) void refresh() }, 10_000) })
onUnmounted(() => { alive = false; clearInterval(timer) })
</script>
