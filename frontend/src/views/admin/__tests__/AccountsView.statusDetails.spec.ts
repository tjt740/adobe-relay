import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const { listAccounts } = vi.hoisted(() => ({
  listAccounts: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag: vi.fn(),
      getBatchTodayStats: vi.fn().mockResolvedValue({ stats: {} }),
      getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({ enabled: true, interval_minutes: 30 }),
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: { getAllWithCount: vi.fn().mockResolvedValue([]) },
    groups: { getAll: vi.fn().mockResolvedValue([]) }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div data-test="data-table"><div v-for="row in data" :key="row.id"><slot name="cell-status" :row="row" /></div>
      <span v-for="column in columns" :key="column.key" :data-column="column.key">
        {{ column.sortable ? 'sortable' : 'fixed' }}
      </span>
      <button data-test="sort-priority" @click="$emit('sort', 'priority', 'desc')" />
    </div>
  `
}

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        AccountTableActions: { template: '<div><slot name="after" /></div>' },
        AccountTableFilters: true,
        AccountBulkActionsBar: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        HelpTooltip: true,
        Icon: true,
        Teleport: true
      }
    }
  })
}

describe('account status record details', () => {
  beforeEach(() => { localStorage.clear(); listAccounts.mockReset() })
  it('keeps records collapsed until clicked and displays a stable read-only snapshot', async () => {
    const account = { id: 8, name: 'record@example.test', platform: 'adobe', type: 'oauth', status: 'active', schedulable: false, extra: { last_check_success: false, last_check_status_code: 408, last_check_at: '2026-10-13T08:38:54Z' } }
    listAccounts.mockResolvedValue({ items: [account], total: 1, page: 1, page_size: 300, pages: 1 })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="account-health-evidence"]').exists()).toBe(false)
    const requestsBeforeOpen = listAccounts.mock.calls.length
    await wrapper.get('button[aria-label="查看 record@example.test 的状态详情"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="account-health-evidence"]').text()).toContain('检测失败')
    expect(wrapper.get('[data-testid="account-status-details"]').text()).toContain('关闭')
    account.extra.last_check_success = true
    account.extra.last_check_status_code = 200
    await flushPromises()
    expect(wrapper.get('[data-testid="account-health-evidence"]').text()).toContain('检测失败')
    expect(listAccounts.mock.calls.length).toBe(requestsBeforeOpen)
    await wrapper.get('button[aria-label="Close modal"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="account-health-evidence"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
