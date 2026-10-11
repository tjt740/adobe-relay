import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import * as XLSX from 'xlsx'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  batchRefresh,
  exportData,
  getBatchTodayStats,
  getUpstreamBillingProbeSettings,
  getAllProxies,
  getAllGroups,
  showError
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  batchRefresh: vi.fn(),
  exportData: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
      batchDelete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh,
      exportData,
      bulkUpdate: vi.fn()
    },
    proxies: {
      getAllWithCount: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const makeAccounts = (count: number) => Array.from({ length: count }, (_, index) => ({
  id: index + 1,
  name: `account-${index + 1}`,
  platform: 'grok',
  type: 'oauth',
  status: 'active',
  schedulable: true,
  created_at: '2026-07-23T00:00:00Z',
  updated_at: '2026-07-23T00:00:00Z'
}))

const AccountBulkActionsBarStub = {
  props: ['selectedIds', 'totalResults', 'selectingAll', 'allResultsSelected'],
  emits: ['select-all-results', 'select-page', 'clear', 'refresh-token', 'export'],
  template: `
    <div>
      <span data-test="selected-count">{{ selectedIds.length }}</span>
      <span data-test="total-results">{{ totalResults }}</span>
      <span data-test="all-results-selected">{{ String(allResultsSelected) }}</span>
      <button data-test="select-page" @click="$emit('select-page')">select page</button>
      <button data-test="select-all-results" @click="$emit('select-all-results')">select all</button>
      <button data-test="clear" @click="$emit('clear')">clear</button>
      <button data-test="refresh-token" @click="$emit('refresh-token')">refresh token</button>
      <button data-test="export" @click="$emit('export')">export accounts</button>
    </div>
  `
}

const AccountTableFiltersStub = {
  emits: ['change'],
  template: '<button data-test="change-filter" @click="$emit(\'change\')">change filter</button>'
}

const ConfirmDialogStub = {
  name: 'ConfirmDialog',
  props: ['show', 'title', 'message'],
  emits: ['confirm', 'cancel'],
  template: '<button v-if="show" data-test="confirm-dialog" @click="$emit(\'confirm\')">{{ title }}</button>'
}

const mountView = () => mount(AccountsView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: {
        template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
      },
      DataTable: {
        props: ['data'],
        template: '<div data-test="data-table"><div v-for="row in data" :key="row.id"><slot name="cell-select" :row="row" /></div></div>'
      },
      Pagination: true,
      ConfirmDialog: ConfirmDialogStub,
      AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
      AccountTableFilters: AccountTableFiltersStub,
      AccountBulkActionsBar: AccountBulkActionsBarStub,
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
      Icon: true,
      Teleport: true
    }
  }
})

describe('admin AccountsView select all filtered results', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset()
    listWithEtag.mockReset()
    batchRefresh.mockReset()
    exportData.mockReset()
    getBatchTodayStats.mockReset()
    getUpstreamBillingProbeSettings.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    showError.mockReset()

    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockResolvedValue({ enabled: true, interval_minutes: 30 })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it.each([
    { name: 'keeps only failed accounts selected', result: { total: 3, success: 2, failed: 1, errors: [{ account_id: 2, error: 'no refresh token available' }] }, expectedIds: [2] },
    { name: 'clears the selection after every account succeeds', result: { total: 3, success: 3, failed: 0 }, expectedIds: [] },
    { name: 'keeps the original selection when failure details are missing', result: { total: 3, success: 2, failed: 1 }, expectedIds: [1, 2, 3] },
  ])('$name after a batch token refresh and table reload', async ({ result, expectedIds }) => {
    listAccounts.mockResolvedValue({ items: makeAccounts(3), total: 3, page: 1, page_size: 20, pages: 1 })
    batchRefresh.mockResolvedValue(result)
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="select-page"]').trigger('click')
    await wrapper.get('[data-test="refresh-token"]').trigger('click')
    expect(batchRefresh).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="confirm-dialog"]').text()).toBe('admin.accounts.bulkRefreshTokenTitle')
    await wrapper.get('[data-test="confirm-dialog"]').trigger('click')
    await flushPromises()

    expect(batchRefresh).toHaveBeenCalledWith([1, 2, 3])
    expect(listAccounts).toHaveBeenCalledTimes(2)
    expect(wrapper.getComponent(AccountBulkActionsBarStub).props('selectedIds')).toEqual(expectedIds)
    expect(wrapper.findAll<HTMLInputElement>('[data-test="data-table"] input').map(input => input.element.checked))
      .toEqual([1, 2, 3].map(id => expectedIds.includes(id)))
    if (result.failed > 0) {
      expect(showError).toHaveBeenCalledWith('admin.accounts.bulkActions.partialSuccess')
      await wrapper.get('[data-test="refresh-token"]').trigger('click')
      await wrapper.get('[data-test="confirm-dialog"]').trigger('click')
      await flushPromises()
      expect(batchRefresh).toHaveBeenLastCalledWith(expectedIds)
    }
    wrapper.unmount()
  })

  it('exports all selected results to a single account column in Excel and clears selection when filters change', async () => {
    const allAccounts = makeAccounts(45)
    const expectedAccounts = allAccounts.map(account => account.name)
    expectedAccounts.splice(0, 6, 'preferred@example.com', 'extra@example.com', 'credential@example.com', '中文账号', '000123', '=1+1')
    const exportedAccounts = allAccounts.map((account, index) => ({
      ...account,
      name: expectedAccounts[index],
      credentials: { cookie: 'private-cookie', password: 'private-password' },
      notes: 'private-note',
      proxy_key: 'private-proxy'
    }))
    Object.assign(exportedAccounts[0], {
      name: 'display name',
      extra: { email_address: expectedAccounts[0], email: 'lower-priority@example.com' },
      credentials: { email: 'credentials-lower-priority@example.com', cookie: 'private-cookie' }
    })
    Object.assign(exportedAccounts[1], { name: 'display name', extra: { email: expectedAccounts[1] } })
    Object.assign(exportedAccounts[2], { name: 'display name', credentials: { email: expectedAccounts[2] } })
    exportData.mockResolvedValue({ accounts: exportedAccounts, proxies: [], exported_at: '2026-10-11T00:00:00Z' })
    const download = mockDownload()
    listAccounts.mockImplementation(async (_page: number, pageSize: number) => {
      if (pageSize === 1000) {
        return {
          items: allAccounts,
          total: 45,
          page: 1,
          page_size: 1000,
          pages: 1
        }
      }
      return {
        items: allAccounts.slice(0, 20),
        total: 45,
        page: 1,
        page_size: 20,
        pages: 3
      }
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('45')
    expect(wrapper.get('[data-test="total-results"]').text()).toBe('45')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('true')
    expect(listAccounts).toHaveBeenCalledWith(1, 1000, expect.objectContaining({
      lite: '1',
      include_scheduler_score: '0'
    }))

    await wrapper.get('[data-test="export"]').trigger('click')
    await vi.waitFor(() => expect(download.click).toHaveBeenCalledTimes(1))
    expect(exportData).toHaveBeenCalledWith({ ids: allAccounts.map(account => account.id), includeProxies: false })
    expect(download.filename()).toMatch(/^sub2api-accounts-\d{14}\.xlsx$/)
    const blob = download.blob()
    expect(blob.type).toBe('application/vnd.openxmlformats-officedocument.spreadsheetml.sheet')
    const workbook = XLSX.read(await readBlob(blob), { type: 'array' })
    expect(workbook.SheetNames).toEqual(['Accounts'])
    const sheet = workbook.Sheets.Accounts
    expect(XLSX.utils.sheet_to_json(sheet, { header: 1 })).toEqual([
      ['admin.accounts.bulkActions.accountColumn'],
      ...expectedAccounts.map(account => [account])
    ])
    expect(sheet.A6).toMatchObject({ t: 's', v: '000123' })
    expect(sheet.A7).toMatchObject({ t: 's', v: '=1+1' })
    expect(sheet.A7.f).toBeUndefined()
    expect(download.revokeObjectURL).toHaveBeenCalledWith('blob:account-export')

    await wrapper.get('[data-test="change-filter"]').trigger('click')

    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('0')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('false')
    await wrapper.get('[data-test="export"]').trigger('click')
    expect(exportData).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('keeps the More Actions export as JSON with its proxy option', async () => {
    listAccounts.mockResolvedValue({ items: makeAccounts(1), total: 1, page: 1, page_size: 20, pages: 1 })
    const payload = { accounts: [{ name: 'account-1', credentials: { cookie: 'backup-cookie' } }], proxies: [{ name: 'backup-proxy' }], exported_at: '2026-10-11T00:00:00Z' }
    exportData.mockResolvedValue(payload)
    const download = mockDownload()
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="select-page"]').trigger('click')
    await wrapper.get('[title="admin.accounts.moreActions"]').trigger('click')
    await wrapper.findAll('.account-tools-menu-item').find(button => button.text().includes('admin.accounts.dataExportSelected'))!.trigger('click')
    await wrapper.get('[data-test="confirm-dialog"]').trigger('click')
    await flushPromises()

    expect(exportData).toHaveBeenCalledWith({ ids: [1], includeProxies: true })
    expect(download.filename()).toMatch(/^sub2api-account-\d{14}\.json$/)
    expect(download.blob().type).toBe('application/json')
    expect(JSON.parse(await readBlob(download.blob(), true) as string)).toEqual(payload)
    wrapper.unmount()
  })

  it('keeps the original page selection when loading all results fails', async () => {
    const currentPage = makeAccounts(20)
    listAccounts.mockImplementation(async (_page: number, pageSize: number) => {
      if (pageSize === 1000) {
        throw new Error('load all failed')
      }
      return {
        items: currentPage,
        total: 45,
        page: 1,
        page_size: 20,
        pages: 3
      }
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-page"]').trigger('click')
    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('20')

    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('20')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('false')
    expect(showError).toHaveBeenCalledWith('admin.accounts.bulkActions.selectAllFailed')
  })
})

function mockDownload() {
  const createObjectURL = vi.fn((_blob: Blob) => 'blob:account-export')
  const revokeObjectURL = vi.fn()
  vi.stubGlobal('URL', class extends URL {
    static createObjectURL = createObjectURL
    static revokeObjectURL = revokeObjectURL
  })
  let filename = ''
  const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    filename = this.download
  })
  return { click, revokeObjectURL, filename: () => filename, blob: () => createObjectURL.mock.calls[0][0] as Blob }
}

function readBlob(blob: Blob, asText = false): Promise<string | ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string | ArrayBuffer)
    reader.onerror = () => reject(reader.error)
    if (asText) reader.readAsText(blob)
    else reader.readAsArrayBuffer(blob)
  })
}
