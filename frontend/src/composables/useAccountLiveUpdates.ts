import { onMounted, onUnmounted, ref } from 'vue'
import { buildApiUrl } from '@/api/url'
import { ADMIN_UI_REQUEST_HEADER } from '@/api/adminUIRequest'

// The stream only invalidates a local snapshot. No Adobe requests are made.
export function useAccountLiveUpdates(refresh: () => Promise<void | boolean>, enabled = true) {
  const connected = ref(false)
  const lastUpdated = ref<Date | null>(null)
  let stopped = false
  let controller: AbortController | null = null
  let timer: ReturnType<typeof setTimeout> | undefined
  let refreshing = false
  let pending = false
  let attempted = false
  async function update() {
    pending = true
    if (refreshing) return
    refreshing = true
    try {
      while (pending && !stopped && !document.hidden) {
        pending = false
        const applied = await refresh()
        if (applied === false) { pending = true; break }
        lastUpdated.value = new Date()
      }
    } finally { refreshing = false }
  }
  async function connect() {
    if (!enabled || stopped || document.hidden || controller) return
    const current = new AbortController()
    controller = current
    let retry = 1000
    let reader: ReadableStreamDefaultReader<Uint8Array> | undefined
    const watchdog = setTimeout(() => current.abort(), 40_000)
    try {
      const response = await fetch(buildApiUrl('/admin/accounts/runtime-events'), {
        headers: { Authorization: `Bearer ${localStorage.getItem('auth_token')}`, [ADMIN_UI_REQUEST_HEADER]: '1' },
        signal: current.signal
      })
      if (response.status === 401 || response.status === 403) { stopped = true; return }
      if (!response.ok || !response.body) throw new Error('Status stream unavailable')
      connected.value = true
      reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (!stopped) {
        const chunk = await reader.read()
        buffer += decoder.decode(chunk.value, { stream: !chunk.done })
        const lines = buffer.split('\n'); buffer = lines.pop() || ''
        for (const line of lines) {
          if (!line.startsWith('data:')) continue
          const event = JSON.parse(line.slice(5))
          if (event.type === 'unavailable') throw new Error('Status source unavailable')
          if (event.type === 'change' || (event.type === 'heartbeat' && pending)) await update()
        }
        if (chunk.done) break
      }
    } catch {
      retry = 10_000
      if (attempted && !stopped && !document.hidden && !current.signal.aborted) {
        try { await update() } catch { /* Keep the stale timestamp visible. */ }
      }
    } finally {
      attempted = true
      clearTimeout(watchdog)
      reader?.releaseLock()
      if (controller === current) controller = null
      connected.value = false
      if (!stopped && !document.hidden) timer = setTimeout(connect, retry)
    }
  }
  function visibility() {
    clearTimeout(timer)
    if (document.hidden) { controller?.abort(); connected.value = false }
    else { void connect() }
  }
  onMounted(() => { document.addEventListener('visibilitychange', visibility); void connect() })
  onUnmounted(() => { stopped = true; clearTimeout(timer); controller?.abort(); document.removeEventListener('visibilitychange', visibility) })
  return { connected, lastUpdated }
}
