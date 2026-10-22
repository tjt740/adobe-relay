// Synthetic development fixture: no production accounts, credentials or network calls.
// Run with dev/account-recovery-demo.vite.ts to avoid app initialization requests.
import { createApp, h, ref } from 'vue'
import { createI18n } from 'vue-i18n'
import { apiClient } from '../src/api/client'
import ClashSubscriptionModal from '../src/components/admin/proxy/ClashSubscriptionModal.vue'
import ProxyHealthPanel from '../src/components/admin/proxy/ProxyHealthPanel.vue'
import TablePageLayout from '../src/components/layout/TablePageLayout.vue'
import DataTable from '../src/components/common/DataTable.vue'
import zh from '../src/i18n/locales/zh'
import en from '../src/i18n/locales/en'
import '../src/style.css'

const params = new URLSearchParams(location.search)
document.documentElement.classList.toggle('dark', params.get('dark') === '1')
const locale = params.get('locale') === 'en' ? 'en' : 'zh'
const at = '2026-10-22T12:00:00+08:00'
const urls = Array.from({ length: 12 }, (_, i) => `https://provider-${i + 1}.example/subscription?token=demo-${i + 1}`)
const nodes = Array.from({ length: 103 }, (_, i) => ({ name: `订阅${i % 12 + 1} · Node ${i + 1}`, type: 'http', server: `node-${i + 1}.example`, server_port: 443, source_id: `sample${i % 12}`, proxy_id: i + 1, proxy_host: 'mihomo', proxy_port: 20000 + i, enabled: true }))
const sources = urls.map((_, i) => ({ id: `sample${i}`, index: i + 1, url_hint: `https://provider-${i + 1}.example/••••`, node_count: 9, checked_at: at, fetch_error: i === 1 ? 'http_error' : '', http_status: i === 1 ? 503 : 200 }))
const subscription = { configured: true, ready: true, revision: 3, updated_at: at, url: urls.join('\n'), url_hint: 'https://provider-1.example/••••', nodes, sources: sources.map(s => ({ ...s, fetch_error: '' })) }
const requests: Array<{ url?: string; method?: string; data?: string }> = []
Object.assign(window, { __clashQaRequests: requests })
apiClient.defaults.adapter = async config => {
  requests.push({ url: config.url, method: config.method, data: config.data })
  const path = config.url ?? ''
  let data: unknown = subscription
  if (path.endsWith('/health')) data = { checked_at: at, sources: sources.map((s, i) => ({ ...s, status: i === 1 ? 'fetch_failed' : i === 3 ? 'degraded' : 'healthy', traffic_known: true, remaining_bytes: 12 * 1024 ** 3, total_bytes: 100 * 1024 ** 3, last_success_at: at, traffic_observed_at: at, expires_at: null, nodes: 103, healthy: 88, failed: 7, unknown: 8, bound_accounts: 12 })), groups: [] }
  if (path.endsWith('/preview')) data = { ...subscription, sources, nodes: nodes.map(n => ({ ...n, retained: n.source_id === 'sample1' })) }
  if (path.endsWith('/check')) {
    const url = String(JSON.parse(config.data).url)
    const index = urls.indexOf(url)
    data = sources[index] ?? { id: 'new', index: 1, url_hint: 'https://new.example/••••', node_count: 16 }
  }
  if (path.endsWith('/import')) data = { ...subscription, sources, revision: 4 }
  return { config, status: 200, statusText: 'OK', headers: {}, data: { code: 0, data } }
}
createApp({
  setup() {
    const show = ref(false)
    return () => h('main', { class: 'mx-auto max-w-7xl p-4 text-gray-900 dark:text-gray-100' }, [
      h('p', { class: 'mb-4 text-xs text-gray-500' }, 'Local synthetic fixture · 12 subscriptions · no production requests'),
      h(TablePageLayout, { scrollMode: 'page' }, {
        filters: () => h(ProxyHealthPanel, { onManage: () => { show.value = true } }),
        table: () => h(DataTable, { columns: [{ key: 'id', label: 'ID' }, { key: 'name', label: 'Proxy' }], data: nodes.map(n => ({ id: n.proxy_id, name: n.name })) }),
        pagination: () => h('footer', { 'data-testid': 'page-bottom', class: 'rounded-xl border p-4' }, 'Pagination · end of page'),
      }),
      show.value ? h(ClashSubscriptionModal, { onClose: () => { show.value = false } }) : null,
    ])
  },
}).use(createI18n({ legacy: false, locale, messages: { zh, en } })).mount('#app')
