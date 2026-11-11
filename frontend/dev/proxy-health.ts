// Local layout fixture only. This adapter never contacts production.
import { createApp, h } from 'vue'
import { createI18n } from 'vue-i18n'
import { apiClient } from '../src/api/client'
import type { ProxyHealth, ProxySourceHealth } from '../src/api/admin/clash'
import ProxyHealthPanel from '../src/components/admin/proxy/ProxyHealthPanel.vue'
import zh from '../src/i18n/locales/zh/proxyHealth'
import en from '../src/i18n/locales/en/proxyHealth'
import '../src/style.css'

const params = new URLSearchParams(location.search)
const locale = params.get('lang') === 'en' ? 'en' : 'zh'
if (params.has('dark')) document.documentElement.classList.add('dark')
document.documentElement.lang = locale
const at = new Date().toISOString()
const statuses: ProxySourceHealth['status'][] = ['exhausted', 'healthy', 'degraded', 'healthy', 'fetch_failed', 'unavailable', 'degraded']
const remaining = [0, null, 4942.58, 31.74, null, null, 148]
const totals = [480, null, 5120, 500, null, null, 600]
const blocked = [61, 12, 23, 7, 39, 0, 3]
const data: ProxyHealth = {
  checked_at: at,
  sources: statuses.map((status, i) => {
    const nodes = 96 + i * 7
    const failed = status === 'unavailable' ? nodes : status === 'degraded' ? 23 : 0
    const unknown = status === 'fetch_failed' ? 21 : 0
    const blocked_nodes = Array.from({ length: blocked[i] }, (_, n) => ({
      proxy_id: 2000 + i * 100 + n, name: `订阅 ${i + 1} · ${['香港', '日本', '新加坡'][n % 3]} ${String(n + 1).padStart(2, '0')}`,
      server: `192.0.2.${n + 1}`, server_port: 443, enabled: true, bound_accounts: n % 4,
      manual: n % 5 === 0, excluded_at: n % 5 === 0 ? at : null,
      failed_at: n % 5 === 0 ? null : at, retry_after: n % 5 === 0 ? null : new Date(Date.now() + 240000).toISOString(),
    }))
    return {
      id: `preview-source-${i + 1}`, index: i + 1, url_hint: `https://provider-${i + 1}.example/••••`, status,
      proxy_ids: Array.from({ length: nodes }, (_, n) => 2000 + i * 100 + n),
      blocked_proxy_ids: blocked_nodes.map(n => n.proxy_id), generation_blocked: blocked[i], blocked_nodes,
      traffic_known: remaining[i] !== null, remaining_bytes: remaining[i] === null ? null : remaining[i]! * 1024 ** 3,
      total_bytes: totals[i] === null ? null : totals[i]! * 1024 ** 3,
      checked_at: at, last_success_at: at, traffic_observed_at: at, expires_at: '2026-12-04T12:55:45Z',
      fetch_error: status === 'fetch_failed' ? 'http_error' : undefined, http_status: status === 'fetch_failed' ? 503 : undefined,
      nodes, healthy: nodes - failed - unknown, failed, unknown, bound_accounts: 72,
    }
  }),
  groups: [
    { id: 2, name: 'GPT Image 2.6 Flare', total: 191, enabled: 174, available: 148, waiting_proxy: 8 },
    { id: 7, name: '客户测试分组', total: 18, enabled: 15, available: 1, waiting_proxy: 4 },
    { id: 15, name: 'GPT Image 2.6 Sunburst', total: 155, enabled: 146, available: 125, waiting_proxy: 2 },
    { id: 18, name: '手动暂停分组', total: 24, enabled: 0, available: 0, waiting_proxy: 0 },
  ],
}
apiClient.defaults.adapter = async config => {
  if (config.method !== 'get' || config.url !== '/admin/proxies/clash/health') throw new Error('Preview only supports health reads')
  return { config, status: 200, statusText: 'OK', headers: {}, data: { code: 0, data } }
}
const i18n = createI18n({ legacy: false, locale, messages: {
  zh: { proxyHealth: zh, common: { refresh: '刷新', loading: '读取中' } },
  en: { proxyHealth: en, common: { refresh: 'Refresh', loading: 'Loading' } },
} })
createApp({ render: () => h('main', { class: 'mx-auto max-w-[1800px] p-4 sm:p-6' }, [
  h('h1', { class: 'text-2xl font-semibold' }, locale === 'zh' ? 'IP 管理' : 'IP management'),
  h('p', { class: 'mt-2 text-sm text-gray-500' }, locale === 'zh' ? '管理代理服务器配置 · 本地示例数据' : 'Manage proxy configuration · Local example data'),
  h(ProxyHealthPanel),
]) }).use(i18n).mount('#app')
