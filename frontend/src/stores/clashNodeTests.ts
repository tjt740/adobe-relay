import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { testProxy } from '@/api/admin/proxies'
import { useAuthStore } from '@/stores/auth'
import type { ClashNode } from '@/api/admin/clash'

type TestResult = { success: boolean; latency_ms?: number; message?: string }

// Browser-session state: the queue belongs to the application, not the dialog.
// Closing a dialog or changing routes must not discard pending nodes.
export const useClashNodeTestStore = defineStore('clashNodeTests', () => {
  const auth = useAuthStore()
  let generation = 0
  const testing = ref(false)
  const testingIDs = ref(new Set<number>())
  const done = ref(0)
  const total = ref(0)
  const results = ref<Record<number, TestResult>>({})
  const healthyCount = computed(() => Object.values(results.value).filter(result => result.success).length)
  const completed = computed(() => total.value > 0 && !testing.value)

  function clearResults() {
    if (testing.value) return
    results.value = {}
    done.value = total.value = 0
  }

  // A detached queue must never continue under a different signed-in user.
  watch(() => auth.user?.id, () => {
    generation++
    testing.value = false
    testingIDs.value.clear()
    clearResults()
  }, { flush: 'sync' })

  async function start(nodes: readonly ClashNode[], errorText: (error: unknown) => string) {
    if (testing.value) return
    const queue = [...new Set(nodes.filter(node => node.enabled && node.proxy_id > 0).map(node => node.proxy_id))]
    if (!queue.length) return
    clearResults()
    total.value = queue.length
    testing.value = true
    const run = ++generation
    async function worker() {
      while (queue.length && run === generation) {
        const id = queue.shift()!
        testingIDs.value.add(id)
        try {
          const result = await testProxy(id)
          if (run === generation) results.value[id] = result
        }
        catch (error) { if (run === generation) results.value[id] = { success: false, message: errorText(error) } }
        finally { if (run === generation) { testingIDs.value.delete(id); done.value++ } }
      }
    }
    try { await Promise.all(Array.from({ length: Math.min(4, queue.length) }, worker)) }
    finally { if (run === generation) testing.value = false }
  }

  return { testing, testingIDs, done, total, results, healthyCount, completed, clearResults, start }
})
