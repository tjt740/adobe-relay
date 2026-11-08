// Development-only preview: group data stays in memory.
// adobe-groups.vite.ts fetches only the public Adobe model directory.
import { createApp, h, ref } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { apiClient } from '../src/api/client'
import GroupsView from '../src/views/admin/GroupsView.vue'
import Toast from '../src/components/common/Toast.vue'
import { useAuthStore } from '../src/stores/auth'
import { useAppStore } from '../src/stores/app'
import { i18n, loadLocaleMessages } from '../src/i18n'
import { adobeGroupNameModels, adobeGroupNameModelIds, adobeModels } from '../src/utils/adobeGroupName'
import { makeDemoGroups, demoNameChanges } from './adobe-groups-data'
import '../src/style.css'

function matchesModel(name: string, selection: string): boolean {
  const known = adobeModels.find(m => m.id === selection || m.label === selection)
  if (known) return adobeGroupNameModelIds(name).includes(known.id)
  if (['2', '2.5', '2.6'].includes(selection)) return adobeGroupNameModels(name).includes(selection as '2')
  return name.toLowerCase().startsWith(('Adobe ' + selection + ' - ').toLowerCase())
}
const groups = makeDemoGroups()
let nextId = 1000
apiClient.defaults.adapter = async config => {
  const path = config.url || ''
  const method = config.method || 'get'
  const params = config.params || {}
  const body = typeof config.data === 'string' ? JSON.parse(config.data) : config.data || {}
  let data: unknown = {}
  if (path === '/admin/groups' && method === 'get') {
    let items = groups.filter(g =>
      (!params.platform || g.platform === params.platform) &&
      (!params.status || g.status === params.status) &&
      (params.is_exclusive == null || g.is_exclusive === params.is_exclusive) &&
      (!params.adobe_model || (g.platform === 'adobe' && matchesModel(g.name, params.adobe_model))) &&
      (!params.search || (g.name + (g.description || '')).toLowerCase().includes(String(params.search).toLowerCase())))
    const key = params.sort_by || 'sort_order'
    items = [...items].sort((a, b) => {
      const av = (a as unknown as Record<string, unknown>)[key] ?? ''
      const bv = (b as unknown as Record<string, unknown>)[key] ?? ''
      const value = typeof av === 'number' && typeof bv === 'number' ? av - bv : String(av).localeCompare(String(bv))
      return value * (params.sort_order === 'desc' ? -1 : 1) || a.id - b.id
    })
    const total = items.length
    const page = Number(params.page || 1)
    const pageSize = Number(params.page_size || 20)
    data = { items: items.slice((page - 1) * pageSize, page * pageSize), total, page, page_size: pageSize, pages: Math.ceil(total / pageSize) }
  } else if (path === '/admin/groups' && method === 'post') {
    const created = { ...groups[0], ...body, id: nextId++, account_count: 0, active_account_count: 0, sort_order: groups.length, status: 'active' }
    groups.push(created)
    data = created
  } else if (path === '/admin/groups/all') {
    data = groups.filter(g => (!params.platform || g.platform === params.platform) && (params.include_inactive || g.status === 'active'))
  } else if (path.endsWith('/live-capability')) {
    data = { supported: false }
  } else if (path.endsWith('/usage-summary')) {
    data = groups.map(g => ({ group_id: g.id, today_cost: 0, yesterday_cost: 0, total_cost: 0 }))
  } else if (path.endsWith('/capacity-summary')) {
    data = groups.map(g => ({ group_id: g.id, concurrency_used: 0, concurrency_max: (g.active_account_count || 0) * 3, sessions_used: 0, sessions_max: 0, rpm_used: 0, rpm_max: 0 }))
  } else if (path === '/admin/groups/adobe-models') {
    const mode = new URLSearchParams(location.search).get('models')
    if (mode === 'error') throw new Error('Simulated Adobe discovery failure')
    if (mode === 'empty') {
      data = { source: 'adobe', models: [] }
    } else {
      const response = await fetch('/__preview/adobe-models', { signal: AbortSignal.timeout(10000), credentials: 'omit' })
      if (!response.ok) throw new Error('Adobe discovery unavailable')
      data = await response.json()
    }
  } else if (/\/(model-allowlist-candidates|effective-models)$/.test(path)) {
    data = { models: adobeModels.map(m => m.id) }
  } else if (path === '/admin/groups/sort-order') {
    for (const update of body.updates || []) {
      const group = groups.find(g => g.id === update.id)
      if (group) group.sort_order = update.sort_order
    }
  } else if (/^\/admin\/groups\/\d+/.test(path)) {
    const id = Number(path.split('/')[3])
    const group = groups.find(g => g.id === id)
    if (path.endsWith('/duplicate') && group) {
      const copy = { ...structuredClone(group), id: nextId++, name: group.name + ' (副本)', status: 'inactive' as const, sort_order: groups.length }
      groups.push(copy)
      data = copy
    } else if (path.endsWith('/rate-multipliers') || path.endsWith('/composite-routes')) {
      data = []
    } else if (method === 'put' && group) {
      Object.assign(group, body)
      data = group
    } else if (method === 'delete') {
      const index = groups.findIndex(g => g.id === id)
      if (index >= 0) groups.splice(index, 1)
    } else {
      data = group || {}
    }
  } else if (path === '/admin/settings') {
    data = { custom_menu_items: [], ops_monitoring_enabled: false }
  } else if (path.includes('announcements')) {
    data = { items: [], total: 0 }
  } else if (path.includes('batch-image')) {
    data = { enabled: false, available: false }
  } else if (path.includes('/accounts')) {
    data = { items: [], total: 0, pages: 0 }
  }
  return { config, status: 200, statusText: 'OK', headers: {}, data: { code: 0, data } }
}

async function start() {
  const locale = new URLSearchParams(location.search).get('locale') === 'en' ? 'en' : 'zh'
  await loadLocaleMessages(locale)
  i18n.global.locale.value = locale
  const pinia = createPinia()
  const auth = useAuthStore(pinia)
  auth.user = { id: 900000, email: 'preview@example.test', username: '本地预览', role: 'admin', status: 'active', balance: 0 } as typeof auth.user
  localStorage.setItem('auth_user', JSON.stringify(auth.user))
  localStorage.setItem('admin_guide_900000_admin_v4_interactive', 'true')
  const appStore = useAppStore(pinia)
  appStore.siteName = 'Adobe · 本地预览'
  appStore.publicSettingsLoaded = true
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: GroupsView, meta: { title: '分组管理', titleKey: 'admin.groups.title' } }]
  })
  await router.push('/admin/groups')
  const showNames = ref(false)
  createApp({
    render: () => h('div', [
      h('div', { class: 'fixed bottom-3 left-3 z-40 max-w-[240px] rounded-lg border border-emerald-200 bg-white px-3 py-2 text-xs text-emerald-800 shadow-sm' }, [
        h('strong', { class: 'block' }, '本地预览 · 44 个模拟分组'),
        h('span', '示例名称与账号数均为模拟；操作刷新后重置。'),
        h('button', { class: 'mt-2 block font-medium underline', onClick: () => { showNames.value = !showNames.value } }, '查看命名整理 · 6 项'),
      ]),
      showNames.value ? h('div', { class: 'fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 p-4' }, [
        h('section', { class: 'max-h-[80vh] w-full max-w-4xl overflow-auto rounded-2xl bg-white p-6' }, [
          h('div', { class: 'mb-4 flex items-center justify-between' }, [h('h2', { class: 'text-xl font-semibold' }, '命名整理预览'), h('button', { onClick: () => { showNames.value = false } }, '关闭')]),
          h('p', { class: 'mb-4 text-sm text-gray-600' }, '6 个模型明确的分组已在本地规范化。36 个无法唯一确定模型的分组和 2 个工具分组保留；线上未改名。'),
          ...demoNameChanges.filter(c => c.reason === 'clear').map(c => h('div', { class: 'mb-3 rounded-lg border p-3' }, [h('p', { class: 'text-sm text-gray-500' }, '#' + c.id + ' · ' + c.before), h('p', { class: 'mt-1 text-sm font-medium text-emerald-800' }, c.name)])),
          h('details', [h('summary', { class: 'cursor-pointer text-sm' }, '查看保留的分组'), ...demoNameChanges.filter(c => c.reason !== 'clear').map(c => h('p', { class: 'mt-2 text-xs text-gray-500' }, '#' + c.id + ' · ' + c.before + (c.reason === 'utility' ? '（工具分组）' : '（保留原模型含义）')))]),
        ]),
      ]) : null,
      h(GroupsView), h(Toast)
    ])
  }).use(pinia).use(router).use(i18n).mount('#app')
}
start().catch(error => console.error('Preview failed to start', error))
