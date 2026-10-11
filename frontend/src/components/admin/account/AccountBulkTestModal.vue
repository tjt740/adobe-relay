<template>
  <BaseDialog :show="show" :title="t('admin.accounts.bulkTest.title')" width="wide" @close="close">
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.bulkTest.hint') }}</p>
      <p v-if="sessionExpired" class="text-sm text-red-600" role="alert">{{ t('admin.accounts.bulkTest.sessionExpired') }}</p>
      <div class="flex flex-wrap items-center justify-between gap-2 text-sm" aria-live="polite">
        <span class="inline-flex items-center gap-2 font-medium" data-testid="bulk-test-progress">
          <Icon v-if="running" name="refresh" size="sm" class="animate-spin text-pink-500 motion-reduce:animate-none" aria-hidden="true" />
          <Icon v-else name="checkCircle" size="sm" class="text-pink-500" aria-hidden="true" />
          {{ t('admin.accounts.bulkTest.progress', { done: completed, total: rows.length }) }}
          <span class="text-pink-600 dark:text-pink-400">{{ percentage }}%</span>
        </span>
        <span>{{ t('admin.accounts.bulkTest.summary', { success: succeeded, failed, cancelled }) }}</span>
      </div>
      <AccountTestProgress :completed="completed" :total="rows.length" :running="running" :label="t('admin.accounts.bulkTest.title')" />
      <p v-if="running" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.bulkTest.backgroundHint') }}</p>
      <div class="max-h-96 overflow-auto rounded-lg border border-gray-200 dark:border-dark-500">
        <table class="w-full text-left text-sm">
          <thead class="sticky top-0 bg-gray-50 dark:bg-dark-700">
            <tr><th class="px-3 py-2">{{ t('admin.accounts.account') }}</th><th class="px-3 py-2">{{ t('admin.accounts.bulkTest.result') }}</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" :data-testid="`bulk-test-row-${row.id}`" class="border-t border-gray-100 dark:border-dark-600">
              <td class="max-w-xs break-all px-3 py-2">{{ row.name }} <span class="text-xs text-gray-400">#{{ row.id }}</span></td>
              <td class="px-3 py-2">
                <span class="inline-flex items-center gap-1.5" :class="row.state === 'success' ? 'text-green-600' : row.state === 'failed' ? 'text-red-600' : row.state === 'running' ? 'text-pink-600 dark:text-pink-400' : 'text-gray-500'">
                  <Icon v-if="row.state === 'running'" name="refresh" size="sm" class="animate-spin motion-reduce:animate-none" aria-hidden="true" />
                  {{ t(`admin.accounts.bulkTest.states.${row.state}`) }}
                </span>
                <span v-if="row.statusCode" class="ml-2 font-mono text-xs">{{ row.statusCode }}</span>
                <p v-if="row.error" class="mt-1 max-w-lg break-words text-xs text-gray-500">{{ row.error }}</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <template #footer>
      <button v-if="running" type="button" data-testid="bulk-test-stop" class="btn btn-warning" @click="stop">{{ t('admin.accounts.bulkTest.stop') }}</button>
      <button type="button" data-testid="bulk-test-close" :class="['btn', running ? 'btn-secondary' : 'bg-pink-600 text-white hover:bg-pink-700 focus:ring-pink-500/50']" @click="close">
        <Icon v-if="!running" name="check" size="sm" class="mr-1.5" aria-hidden="true" />
        {{ t(running ? 'admin.accounts.bulkTest.runInBackground' : 'admin.accounts.bulkTest.complete') }}
      </button>
    </template>
  </BaseDialog>
  <Teleport to="body">
    <aside
      v-if="!show && rows.length && !backgroundDismissed"
      data-testid="bulk-test-background"
      :aria-label="t('admin.accounts.bulkTest.title')"
      class="fixed bottom-5 right-5 z-40 w-80 max-w-[calc(100vw-2.5rem)] space-y-3 rounded-2xl border border-pink-200 bg-white p-4 shadow-lg dark:border-pink-800 dark:bg-dark-800"
    >
      <div class="flex items-center justify-between gap-3">
        <span class="inline-flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white" role="status">
          <Icon :name="running ? 'refresh' : 'checkCircle'" size="sm" :class="['text-pink-500', running && 'animate-spin motion-reduce:animate-none']" aria-hidden="true" />
          {{ t(running ? 'admin.accounts.bulkTest.backgroundRunning' : 'admin.accounts.bulkTest.complete') }}
        </span>
        <button v-if="!running" type="button" :aria-label="t('common.close')" class="rounded p-1 text-gray-400 hover:text-gray-600" @click="backgroundDismissed = true">
          <Icon name="x" size="sm" />
        </button>
      </div>
      <AccountTestProgress :completed="completed" :total="rows.length" :running="running" :label="t('admin.accounts.bulkTest.title')" />
      <div class="flex items-center justify-between gap-2 text-xs">
        <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.bulkTest.progress', { done: completed, total: rows.length }) }} · {{ percentage }}%</span>
        <button type="button" data-testid="bulk-test-reopen" class="font-medium text-pink-600 hover:text-pink-700 dark:text-pink-400" @click="emit('open')">
          {{ t(running ? 'admin.accounts.bulkTest.viewProgress' : 'admin.accounts.bulkTest.viewResults') }}
        </button>
      </div>
      <p v-if="!running" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.bulkTest.summary', { success: succeeded, failed, cancelled }) }}</p>
    </aside>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountTestProgress from './AccountTestProgress.vue'
import { AccountTestSessionExpired, testAccountConnection } from '@/api/admin/accountTest'

interface Target { id: number; name: string }
interface TestRow extends Target {
  state: 'pending' | 'running' | 'success' | 'failed' | 'cancelled'
  statusCode?: number
  error?: string
}
const props = defineProps<{ show: boolean; targets: Target[]; runId: number }>()
const emit = defineEmits<{
  (e: 'open'): void
  (e: 'close'): void
  (e: 'completed'): void
  (e: 'running-change', running: boolean): void
}>()
const { t } = useI18n()
const rows = ref<TestRow[]>([])
const running = ref(false)
const sessionExpired = ref(false)
const backgroundDismissed = ref(false)
const succeeded = computed(() => rows.value.filter(row => row.state === 'success').length)
const failed = computed(() => rows.value.filter(row => row.state === 'failed').length)
const cancelled = computed(() => rows.value.filter(row => row.state === 'cancelled').length)
const completed = computed(() => succeeded.value + failed.value + cancelled.value)
const percentage = computed(() => rows.value.length ? Math.floor(completed.value / rows.value.length * 100) : 0)
let controller: AbortController | undefined
let generation = 0

const stop = () => {
  controller?.abort()
  rows.value.forEach(row => { if (row.state === 'pending') row.state = 'cancelled' })
}
const close = () => {
  if (!running.value) backgroundDismissed.value = true
  emit('close')
}
const start = async () => {
  if (running.value) return
  const run = ++generation
  const batch = new AbortController()
  controller = batch
  // Snapshot selection: later page/filter/checkbox changes cannot alter this run.
  rows.value = [...new Map(props.targets.map(target => [target.id, target])).values()].map(target => ({ ...target, state: 'pending' }))
  sessionExpired.value = false
  backgroundDismissed.value = false
  running.value = true
  let next = 0
  const worker = async () => {
    while (!batch.signal.aborted && run === generation) {
      const row = rows.value[next++]
      if (!row) return
      row.state = 'running'
      try {
        const result = await testAccountConnection(row.id, batch.signal)
        if (run !== generation) return
        if (batch.signal.aborted) { row.state = 'cancelled'; return }
        row.state = result.success ? 'success' : 'failed'
        row.statusCode = result.statusCode
        row.error = result.error || (result.success ? undefined : t('admin.accounts.bulkTest.failedHint'))
      } catch (error) {
        if (run !== generation) return
        if (batch.signal.aborted) { row.state = 'cancelled'; return }
        row.state = 'failed'
        if (error instanceof AccountTestSessionExpired) {
          sessionExpired.value = true
          row.error = t('admin.accounts.bulkTest.sessionExpired')
          stop()
        } else {
          row.error = error instanceof Error ? error.message : t('common.unknownError')
        }
      }
    }
  }
  try {
    await Promise.all(Array.from({ length: Math.min(3, rows.value.length) }, worker))
  } finally {
    if (run === generation) { controller = undefined; running.value = false; emit('completed') }
  }
}
// Visibility only controls the dialog. Starting a batch is an explicit action,
// so hiding/reopening it never cancels or duplicates upstream requests.
watch(() => props.runId, id => { if (id > 0) void start() }, { immediate: true })
watch(running, value => emit('running-change', value), { flush: 'sync', immediate: true })
onBeforeUnmount(() => { stop(); generation++ })
</script>
