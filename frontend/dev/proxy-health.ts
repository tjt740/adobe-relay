// Local layout fixture only. This adapter never contacts production.
import { createApp, h } from 'vue'
import { createI18n } from 'vue-i18n'
import { apiClient } from '../src/api/client'
import type { ProxyHealth, ProxySourceHealth } from '../src/api/admin/clash'
import type { ProxyRelatedAccount, ProxyRelatedAccountsView } from '../src/api/admin/proxyRelatedAccounts'
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
  if (config.method !== 'get') throw new Error('Preview only supports reads')
  let payload: ProxyHealth | ProxyRelatedAccountsView = data
  const match = config.url?.match(/^\/admin\/proxies\/(\d+)\/related-accounts$/)
  if (match) {
    const id = Number(match[1])
    const node = data.sources.flatMap(s => s.blocked_nodes ?? []).find(n => n.proxy_id === id)!
    const items: ProxyRelatedAccount[] = Array.from({ length: node.bound_accounts }, (_, i) => ({
      id: 3100 + i, name: ['图片业务 A', '固定出口 B', '已暂停账号 C'][i], platform: 'adobe',
      using_this_node: true, mode: i === 1 ? 'manual' : 'automatic',
      account_state: i === 2 ? 'paused' : 'eligible', route_state: 'generation_paused', impact: 'affected',
      current_proxy: { id, name: node.name }, last_departure: null,
    }))
    items.push(
      { id: 3201, name: '已切换账号 D', platform: 'adobe', using_this_node: false, mode: 'automatic', account_state: 'eligible', route_state: 'reachable', impact: 'left_node', current_proxy: { id: 2901, name: '日本备用 01' }, last_departure: { at, destination: { id: 2901, name: '日本备用 01' } } },
      { id: 3202, name: '等待代理 E', platform: 'adobe', using_this_node: false, mode: 'automatic', account_state: 'waiting_proxy', route_state: 'waiting_proxy', impact: 'left_node', current_proxy: null, last_departure: { at, destination: null } },
      { id: 3203, name: '再次切换账号 F', platform: 'adobe', using_this_node: false, mode: 'automatic', account_state: 'eligible', route_state: 'unavailable', impact: 'left_node', current_proxy: { id: 2902, name: '新加坡备用 02' }, last_departure: { at, destination: { id: 2901, name: '日本备用 01' } } },
    )
    const { search = '', relation = 'all', page = 1, page_size = 20 } = config.params
    const matching = items.filter(a => (relation === 'all' || (relation === 'current' ? a.using_this_node : !a.using_this_node)) && `${a.name} ${a.id}`.toLowerCase().includes(String(search).toLowerCase()))
    payload = { proxy: { id, name: node.name }, checked_at: new Date().toISOString(), history_since: new Date(Date.now() - 86400000).toISOString(), current_count: node.bound_accounts, departed_count: 3, total: matching.length, page, page_size, items: matching.slice((page - 1) * page_size, page * page_size) }
  } else if (config.url !== '/admin/proxies/clash/health') throw new Error('Unsupported preview request')
  return { config, status: 200, statusText: 'OK', headers: {}, data: { code: 0, data: payload } }
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
