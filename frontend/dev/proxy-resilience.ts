// Development-only visual fixture. No production credentials, requests, or state.
import { createApp, h } from 'vue'
import { createI18n } from 'vue-i18n'
import { apiClient } from '../src/api/client'
import ProxyHealthPanel from '../src/components/admin/proxy/ProxyHealthPanel.vue'
import proxyHealth from '../src/i18n/locales/zh/proxyHealth'
import '../src/style.css'
const at = new Date().toISOString()
apiClient.defaults.adapter = async config => ({ config, status: 200, statusText: 'OK', headers: {}, data: { code: 0, data: { checked_at: at, sources: ['exhausted', 'fetch_failed', 'degraded', 'healthy'].map((status, i) => ({ id: `sample${i}`, index: i + 1, url_hint: `https://provider-${i + 1}.example/••••`, status, traffic_known: status !== 'fetch_failed', remaining_bytes: status === 'exhausted' ? 0 : 12 * 1024 ** 3, total_bytes: 100 * 1024 ** 3, checked_at: at, last_success_at: at, traffic_observed_at: at, expires_at: null, fetch_error: status === 'fetch_failed' ? 'http_error' : '', http_status: status === 'fetch_failed' ? 503 : 200, nodes: 10, healthy: status === 'exhausted' ? 0 : 7, failed: status === 'degraded' ? 3 : 0, unknown: status === 'fetch_failed' ? 3 : 0, bound_accounts: 12 })), groups: [{ id: 2, name: '示例客户分组', total: 20, enabled: 3, available: 1, waiting_proxy: 2 }, { id: 7, name: '人工暂停示例', total: 4, enabled: 0, available: 0, waiting_proxy: 0 }] } } })
const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh: { proxyHealth, common: { refresh: '刷新', loading: '读取中' } } } })
createApp({ render: () => h('main', { class: 'mx-auto max-w-6xl p-4' }, [h('h1', { class: 'text-xl font-semibold' }, '代理自动恢复'), h('p', { class: 'mt-2 text-sm text-gray-500' }, '布局示例数据 · 不连接生产 · 不修改账号或代理'), h(ProxyHealthPanel)]) }).use(i18n).mount('#app')
