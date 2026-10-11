<template>
  <BaseDialog :show="show" :title="t('admin.accounts.bulkTest.title')" width="wide" @close="close">
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.bulkTest.hint') }}</p>
      <p v-if="sessionExpired" class="text-sm text-red-600" role="alert">{{ t('admin.accounts.bulkTest.sessionExpired') }}</p>
      <div class="flex flex-wrap items-center justify-between gap-2 text-sm" aria-live="polite">
        <span data-testid="bulk-test-progress">{{ t('admin.accounts.bulkTest.progress', { done: completed, total: rows.length }) }}</span>
        <span>{{ t('admin.accounts.bulkTest.summary', { success: succeeded, failed, cancelled }) }}</span>
      </div>
      <progress class="h-2 w-full accent-primary-600" :value="completed" :max="rows.length || 1" :aria-label="t('admin.accounts.bulkTest.title')" />
      <div class="max-h-96 overflow-auto rounded-lg border border-gray-200 dark:border-dark-500">
        <table class="w-full text-left text-sm">
          <thead class="sticky top-0 bg-gray-50 dark:bg-dark-700">
            <tr><th class="px-3 py-2">{{ t('admin.accounts.account') }}</th><th class="px-3 py-2">{{ t('admin.accounts.bulkTest.result') }}</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" :data-testid="`bulk-test-row-${row.id}`" class="border-t border-gray-100 dark:border-dark-600">
              <td class="max-w-xs break-all px-3 py-2">{{ row.name }} <span class="text-xs text-gray-400">#{{ row.id }}</span></td>
              <td class="px-3 py-2">
                <span :class="row.state === 'success' ? 'text-green-600' : row.state === 'failed' ? 'text-red-600' : 'text-gray-500'">{{ t(`admin.accounts.bulkTest.states.${row.state}`) }}</span>
                <span v-if="row.statusCode" class="ml-2 font-mono text-xs">{{ row.statusCode }}</span>
                <p v-if="row.error" class="mt-1 max-w-lg break-words text-xs text-gray-500">{{ row.error }}</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <template #footer>
      <button v-if="running" data-testid="bulk-test-stop" class="btn btn-warning" @click="stop">{{ t('admin.accounts.bulkTest.stop') }}</button>
      <button class="btn btn-secondary" @click="close">{{ running ? t('admin.accounts.bulkTest.stopAndClose') : t('common.close') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { AccountTestSessionExpired, testAccountConnection } from '@/api/admin/accountTest'

interface Target { id: number; name: string }
interface TestRow extends Target {
  state: 'pending' | 'running' | 'success' | 'failed' | 'cancelled'
  statusCode?: number
  error?: string
}
const props = defineProps<{ show: boolean; targets: Target[] }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'completed'): void }>()
const { t } = useI18n()
const rows = ref<TestRow[]>([])
const running = ref(false)
const sessionExpired = ref(false)
const succeeded = computed(() => rows.value.filter(row => row.state === 'success').length)
const failed = computed(() => rows.value.filter(row => row.state === 'failed').length)
const cancelled = computed(() => rows.value.filter(row => row.state === 'cancelled').length)
const completed = computed(() => succeeded.value + failed.value + cancelled.value)
let controller: AbortController | undefined
let generation = 0

const stop = () => {
  controller?.abort()
  rows.value.forEach(row => { if (row.state === 'pending') row.state = 'cancelled' })
}
const close = () => { stop(); emit('close') }
const start = async () => {
  stop()
  const run = ++generation
  const batch = new AbortController()
  controller = batch
  // Snapshot selection: later page/filter/checkbox changes cannot alter this run.
  rows.value = [...new Map(props.targets.map(target => [target.id, target])).values()].map(target => ({ ...target, state: 'pending' }))
  sessionExpired.value = false
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
    if (run === generation) { running.value = false; emit('completed') }
  }
}
watch(() => props.show, show => { if (show) void start(); else stop() }, { immediate: true })
onBeforeUnmount(() => { stop(); generation++ })
</script>
