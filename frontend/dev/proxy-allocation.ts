// Synthetic preview. The adapter never contacts production or external nodes.
import { createApp, h, ref } from 'vue'
import { createI18n } from 'vue-i18n'
import { apiClient } from '../src/api/client'
import ProxyAllocationModal from '../src/components/admin/proxy/ProxyAllocationModal.vue'
import zh from '../src/i18n/locales/zh'
import en from '../src/i18n/locales/en'
import '../src/style.css'

const params = new URLSearchParams(location.search)
document.documentElement.classList.toggle('dark', params.get('dark') === '1')
const locale = params.get('locale') === 'en' ? 'en' : 'zh'
const requests: Array<{ url?: string; method?: string }> = []
Object.assign(window, { __allocationQaRequests: requests })
apiClient.defaults.adapter = async config => {
  requests.push({ url: config.url, method: config.method })
  const enabled = config.method === 'put' ? JSON.parse(config.data).enabled : true
  const data = {
    enabled, revision: 1, accounts_per_proxy: 3, accounts: 978, assigned: 951, waiting: 27,
    binding_only_accounts: 963, verification_priority_accounts: 6, business_waiting: 0,
    verification_waiting: 6, healthy_nodes: 308, available_slots: 0, reclaimable_slots: 903,
    checked_at: '2026-10-22T15:12:50+08:00',
    nodes: Array.from({ length: 40 }, (_, i) => ({ id: i + 1, name: `Clash · 订阅 1 · 香港中转 ${String(i + 1).padStart(2, '0')}`, accounts: 3, status: i === 4 ? 'unknown' : 'healthy', latency_ms: 1000 + i * 10 })),
  }
  return { config, status: 200, statusText: 'OK', headers: {}, data: { code: 0, data } }
}
createApp({ setup() {
  const show = ref(true)
  return () => h('main', { class: 'p-4 text-gray-900 dark:text-gray-100' }, [
    h('button', { class: 'btn btn-primary', onClick: () => { show.value = true } }, locale === 'zh' ? '查看代理分配' : 'Open proxy allocation'),
    show.value ? h(ProxyAllocationModal, { onClose: () => { show.value = false } }) : null,
  ])
} }).use(createI18n({ legacy: false, locale, messages: { zh, en } })).mount('#app')
