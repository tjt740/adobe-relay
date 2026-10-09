<template>
  <BaseDialog :show="true" :title="t('admin.proxies.allocation.title')" width="wide" @close="emit('close')">
    <div class="space-y-5" :aria-busy="loading || saving">
      <p class="text-sm text-gray-500">{{ t('admin.proxies.allocation.description') }}</p>
      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-600 dark:bg-red-900/20">{{ error }}</p>
      <p v-if="saved" role="status" class="text-sm text-teal-600">{{ t('admin.proxies.allocation.saved') }}</p>
      <template v-if="view">
        <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <label class="flex items-center gap-3 font-medium">
            <input v-model="enabled" type="checkbox" :disabled="saving" class="h-4 w-4 rounded text-primary-600" @change="dirty = true; saved = false" />
            {{ t('admin.proxies.allocation.enable') }}
          </label>
          <p class="mt-2 text-xs leading-relaxed text-gray-500">{{ t('admin.proxies.allocation.behavior') }}</p>
          <p class="mt-2 text-xs leading-relaxed text-gray-500">{{ t('admin.proxies.allocation.disableHint') }}</p>
        </div>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div v-for="stat in stats" :key="stat.label" class="rounded-xl border border-gray-200 p-3 dark:border-dark-600">
            <p class="text-xs text-gray-500">{{ stat.label }}</p>
            <p class="mt-1 text-2xl font-semibold tabular-nums">{{ stat.value }}</p>
          </div>
        </div>
        <p v-if="view.binding_only_accounts" class="text-sm text-gray-500" data-testid="binding-only-hint">
          {{ t('admin.proxies.allocation.bindingOnlyHint', { count: view.binding_only_accounts }) }}
        </p>
        <p v-if="view.enabled && view.waiting" role="status" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-400">
          {{ t('admin.proxies.allocation.waitingHint', { count: view.waiting }) }}
        </p>
        <p class="text-xs text-gray-500">{{ t('admin.proxies.allocation.checkedAt') }}：{{ view.checked_at ? new Date(view.checked_at).toLocaleString() : t('admin.proxies.allocation.pending') }}</p>
        <div v-if="view.nodes.length" class="max-h-72 overflow-auto rounded-xl border border-gray-200 dark:border-dark-600">
          <table class="w-full text-left text-sm">
            <thead class="sticky top-0 bg-gray-50 text-xs text-gray-500 dark:bg-dark-700">
              <tr><th class="p-3">{{ t('admin.proxies.columns.name') }}</th><th class="p-3">{{ t('admin.proxies.allocation.bindings') }}</th><th class="p-3">{{ t('admin.proxies.allocation.health') }}</th></tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
              <tr v-for="node in view.nodes" :key="node.id">
                <td class="break-words p-3">{{ node.name }}</td>
                <td class="whitespace-nowrap p-3 tabular-nums">{{ node.accounts }} / {{ view.accounts_per_proxy }}</td>
                <td class="whitespace-nowrap p-3" :class="node.status === 'healthy' ? 'text-teal-600' : 'text-gray-500'">
                  {{ t(`admin.proxies.allocation.states.${node.status}`) }}<span v-if="node.status === 'healthy'"> · {{ node.latency_ms }} ms</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="text-sm text-gray-500">{{ t('admin.proxies.allocation.noNodes') }}</p>
      </template>
      <p v-else-if="loading" role="status" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" :disabled="loading || saving" @click="refresh(true)">{{ t('common.refresh') }}</button>
        <button class="btn btn-primary" :disabled="!view || !dirty || saving || loading" @click="save">{{ t('common.save') }}</button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { getProxyAllocation, saveProxyAllocation, type ProxyAllocationView } from '@/api/admin/proxyAllocation'

const emit = defineEmits<{ close: []; changed: [] }>()
const { t } = useI18n()
const view = ref<ProxyAllocationView | null>(null)
const enabled = ref(false)
const revision = ref(0)
const dirty = ref(false)
const loading = ref(false)
const saving = ref(false)
const saved = ref(false)
const error = ref('')
let alive = true
let timer: ReturnType<typeof setInterval> | undefined
const stats = computed(() => [
  { label: t('admin.proxies.allocation.assigned'), value: view.value?.assigned ?? 0 },
  { label: t('admin.proxies.allocation.waiting'), value: view.value?.waiting ?? 0 },
  { label: t('admin.proxies.allocation.healthyNodes'), value: view.value?.healthy_nodes ?? 0 },
  { label: t('admin.proxies.allocation.availableSlots'), value: view.value?.available_slots ?? 0 }
])
function apply(v: ProxyAllocationView, reset: boolean) {
  view.value = v
  if (reset) { enabled.value = v.enabled; revision.value = v.revision; dirty.value = false }
}
async function refresh(reportError = false) {
  if (loading.value || saving.value) return
  loading.value = true
  try {
    const v = await getProxyAllocation()
    if (alive) { apply(v, !dirty.value); error.value = '' }
  } catch {
    if (alive && (reportError || !view.value)) error.value = t('admin.proxies.allocation.loadFailed')
  } finally { loading.value = false }
}
async function save() {
  if (!view.value || !dirty.value || saving.value || loading.value) return
  saving.value = true; saved.value = false; error.value = ''
  try {
    const v = await saveProxyAllocation(enabled.value, revision.value)
    if (alive) { apply(v, true); saved.value = true; emit('changed') }
  } catch (err: unknown) {
    if (alive) {
      const conflict = (err as { response?: { status?: number } })?.response?.status === 409
      error.value = t(conflict ? 'admin.proxies.allocation.conflict' : 'admin.proxies.allocation.saveFailed')
      if (conflict) { view.value = null; dirty.value = false }
    }
  } finally { saving.value = false }
}
onMounted(() => { void refresh(); timer = setInterval(() => { if (!document.hidden) void refresh() }, 10_000) })
onUnmounted(() => { alive = false; clearInterval(timer) })
</script>
