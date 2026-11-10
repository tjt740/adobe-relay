import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminGroup } from '@/types'
import GroupsView from '@/views/admin/GroupsView.vue'
import { adminAPI } from '@/api/admin'

const {
  listGroups,
  duplicateGroup,
  updateGroup,
  getModelAllowlistCandidates,
  getUsageSummary,
  getCapacitySummary,
  getLiveCapability,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  listGroups: vi.fn(),
  duplicateGroup: vi.fn(),
  updateGroup: vi.fn(),
  getModelAllowlistCandidates: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getLiveCapability: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

const authState = vi.hoisted(() => ({ isSimpleMode: false }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list: listGroups,
      duplicate: duplicateGroup,
      getModelAllowlistCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
      getAdobeWebsiteModelOptions: vi.fn().mockResolvedValue([]),
      getAll: vi.fn(),
      create: vi.fn(),
      update: updateGroup,
      delete: vi.fn(),
      updateSortOrder: vi.fn()
    },
    accounts: {
      list: vi.fn(),
      getById: vi.fn()
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const sourceGroup: AdminGroup = {
  id: 42,
  name: 'Primary',
  description: null,
  platform: 'openai',
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 0.5,
  batch_image_hold_multiplier: 0.6,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  web_search_price_per_call: null,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  default_mapped_model: '',
  messages_dispatch_model_config: undefined,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-07-16T00:00:00Z',
  updated_at: '2026-07-16T00:00:00Z',
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 1,
  active_account_count: 1,
  rate_limited_account_count: 0,
  model_allowlist: undefined,
  sort_order: 10
}

const AppLayoutStub = defineComponent({
  template: '<main><slot /></main>'
})

const TablePageLayoutStub = defineComponent({
  template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>'
})

const DataTableStub = defineComponent({
  props: {
    data: { type: Array, default: () => [] },
    columns: { type: Array, default: () => [] },
    loading: { type: Boolean, default: false }
  },
  template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>'
})

const BaseDialogStub = defineComponent({
  props: {
    show: { type: Boolean, default: false }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

function mountView() {
  return mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: true,
        PlatformIcon: true,
        Icon: true,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        VueDraggable: true
      }
    }
  })
}

describe('GroupsView duplicate action', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.groups.getAdobeWebsiteModelOptions).mockReset().mockResolvedValue([])
    authState.isSimpleMode = false
    localStorage.clear()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    for (const fn of [
      listGroups,
      duplicateGroup,
      updateGroup,
      getModelAllowlistCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
      showSuccess,
      showError
    ]) {
      fn.mockReset()
    }

    listGroups.mockResolvedValue({
      items: [sourceGroup],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    duplicateGroup.mockResolvedValue({
      ...sourceGroup,
      id: 43,
      name: 'Primary (Copy)',
      status: 'inactive'
    })
    getModelAllowlistCandidates.mockResolvedValue([])
    getUsageSummary.mockResolvedValue([])
    getCapacitySummary.mockResolvedValue([])
    getLiveCapability.mockResolvedValue({ supported: false })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('uses API display names and retains newer models in the filter', async () => {
    vi.mocked(adminAPI.groups.getAdobeWebsiteModelOptions).mockResolvedValue([{ id: 'future', display_name: 'Future Image Pro' }])
    const wrapper = mountView()
    await flushPromises()
    const options = wrapper.findComponent('[data-testid="adobe-model-filter"]').props('options')
    expect(options).toContainEqual({ value: 'Future Image Pro', label: 'Future Image Pro' })
    expect(options).not.toContainEqual({ value: 'GPT Image 2', label: 'GPT Image 2' })
    wrapper.unmount()
  })

  it('falls back to website names on API failure', async () => {
    vi.mocked(adminAPI.groups.getAdobeWebsiteModelOptions).mockRejectedValue(new Error('unavailable'))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findComponent('[data-testid="adobe-model-filter"]').props('options')).toContainEqual({ value: 'GPT Image 2.6 Flare', label: 'GPT Image 2.6 Flare' })
    wrapper.unmount()
  })

  it.each([false, true])('saves the billing mode and retains entered prices even when disabled (%s)', async enabled => {
    const wrapper = mountView()
    await flushPromises()
    const filter = wrapper.findComponent('[data-testid="adobe-model-filter"]')
    filter.vm.$emit('update:modelValue', 'GPT Image 2.6 Flare')
    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await flushPromises()
    await wrapper.get('#adobe-name-customer').setValue('preview')
    expect(wrapper.find('[data-testid="image-quality-pricing"]').exists()).toBe(true)
    const mode = wrapper.findComponent({ name: 'AdobePricingModeSwitch' })
    mode.vm.$emit('update:modelValue', true)
    await flushPromises()
    await wrapper.get('input[aria-label="GPT Image 2.6 Flare 4K Max"]').setValue('0')
    if (!enabled) mode.vm.$emit('update:modelValue', false)
    await flushPromises()
    vi.mocked(adminAPI.groups.create).mockResolvedValue({ ...sourceGroup, platform: 'adobe' })
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()
    const payload = vi.mocked(adminAPI.groups.create).mock.calls.at(-1)![0]
    expect(payload.image_quality_pricing_enabled).toBe(enabled)
    expect(payload.image_quality_prices).toEqual({ 'gpt-image-2.6-flare': { '4K': { max: 0 } } })
    expect(payload.image_price_1k).toBeNull()
    expect(payload.image_price_2k).toBeNull()
    expect(payload.image_price_4k).toBeNull()
    wrapper.unmount()
  })

  it.each([undefined, false])('defaults an unset mode to new billing and respects saved false (%s)', async savedMode => {
    const prices = { 'gpt-image-2.6-flare': { '4K': { max: 0.5 } } }
    listGroups.mockResolvedValue({ items: [{ ...sourceGroup, name: 'Adobe GPT Image 2.6 Flare - preview', platform: 'adobe', image_quality_pricing_enabled: savedMode, image_price_1k: 0.05, image_quality_prices: prices }], total: 1, page: 1, page_size: 20, pages: 1 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
    await flushPromises()
    const expected = savedMode ?? true
    expect(wrapper.find('[data-testid="image-quality-pricing"]').exists()).toBe(expected)
    expect(wrapper.findComponent({ name: 'AdobePricingModeSwitch' }).props('modelValue')).toBe(expected)
    const tooltip = document.body.querySelector('[role="tooltip"]') as HTMLElement
    expect(tooltip.style.display).toBe('none')
    await wrapper.get('button[aria-label="admin.groups.adobePricingMode.help"]').trigger('click')
    expect(tooltip.style.display).not.toBe('none')
    expect(tooltip.textContent).toContain(expected ? 'admin.groups.adobePricingMode.newDescription' : 'admin.groups.adobePricingMode.savedNotice')
    updateGroup.mockResolvedValue({ ...sourceGroup, platform: 'adobe', image_quality_prices: prices })
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    const payload = updateGroup.mock.calls.at(-1)![1]
    expect(payload.image_quality_pricing_enabled).toBe(expected)
    expect(payload.image_quality_prices).toEqual(prices)
    expect(payload.image_price_1k).toBe(0.05)
    wrapper.unmount()
  })

  it('reopens the saved billing mode and preserves prices through off-save-on', async () => {
    const prices = { 'gpt-image-2.6-flare': { '4K': { max: 0.1234 } } }
    const group = { ...sourceGroup, name: 'Adobe GPT Image 2.6 Flare - mode preview', platform: 'adobe', image_quality_pricing_enabled: true, image_quality_prices: prices }
    listGroups.mockImplementation(async () => ({ items: [{ ...group }], total: 1, page: 1, page_size: 20, pages: 1 }))
    updateGroup.mockImplementation(async (_id, payload) => Object.assign(group, payload))
    const wrapper = mountView()
    await flushPromises()
    const open = async () => {
      await wrapper.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
      await flushPromises()
    }
    await open()
    let mode = wrapper.findComponent({ name: 'AdobePricingModeSwitch' })
    expect(mode.props('modelValue')).toBe(true)
    expect(wrapper.get<HTMLInputElement>('input[aria-label="GPT Image 2.6 Flare 4K Max"]').element.value).toBe('0.1234')
    await wrapper.get('input[aria-label="GPT Image 2.6 Flare 4K Max"]').setValue('0.2345')
    mode.vm.$emit('update:modelValue', false)
    await flushPromises()
    expect(wrapper.find('[data-testid="image-quality-pricing"]').exists()).toBe(false)
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(group.image_quality_pricing_enabled).toBe(false)
    expect(group.image_quality_prices['gpt-image-2.6-flare']['4K'].max).toBe(0.2345)
    await open()
    mode = wrapper.findComponent({ name: 'AdobePricingModeSwitch' })
    expect(mode.props('modelValue')).toBe(false)
    mode.vm.$emit('update:modelValue', true)
    await flushPromises()
    expect(wrapper.get<HTMLInputElement>('input[aria-label="GPT Image 2.6 Flare 4K Max"]').element.value).toBe('0.2345')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(group.image_quality_pricing_enabled).toBe(true)
    await open()
    expect(wrapper.findComponent({ name: 'AdobePricingModeSwitch' }).props('modelValue')).toBe(true)
    wrapper.unmount()
  })

  it('creates a multi-model group and saves independent four-decimal tab prices', async () => {
    const wrapper = mountView()
    await flushPromises()
    wrapper.findComponent('[data-testid="adobe-model-filter"]').vm.$emit('update:modelValue', 'GPT Image 2')
    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await flushPromises()
    await wrapper.get('#adobe-name-customer').setValue('preview')
    const models = wrapper.findComponent('#adobe-name-model')
    models.vm.$emit('change', 'gpt-image-2.5-flare')
    models.vm.$emit('change', 'gpt-image-2.6-sunburst')
    await flushPromises()
    const mode = wrapper.findComponent({ name: 'AdobePricingModeSwitch' })
    expect(mode.props('modelValue')).toBe(true)
    mode.vm.$emit('update:modelValue', true)
    await flushPromises()
    expect(wrapper.findAll('[role="tab"]')).toHaveLength(3)
    await wrapper.get('input[aria-label="GPT Image 2 1K Low"]').setValue('0.0001')
    await wrapper.get('[role="tab"][data-model="gpt-image-2.5-flare"]').trigger('click')
    await wrapper.get('input[aria-label="GPT Image 2.5 Flare 4K Max"]').setValue('0')
    await wrapper.get('[role="tab"][data-model="gpt-image-2.6-sunburst"]').trigger('click')
    await wrapper.get('input[aria-label="GPT Image 2.6 Sunburst 4K Max"]').setValue('0.1234')
    models.vm.$emit('change', 'gpt-image-2.5-flare')
    await flushPromises()
    expect(wrapper.findAll('[role="tab"]')).toHaveLength(2)
    models.vm.$emit('change', 'gpt-image-2.5-flare')
    await flushPromises()
    await wrapper.get('[role="tab"][data-model="gpt-image-2.5-flare"]').trigger('click')
    expect(wrapper.get<HTMLInputElement>('input[aria-label="GPT Image 2.5 Flare 4K Max"]').element.value).toBe('0.0000')
    vi.mocked(adminAPI.groups.create).mockResolvedValue({ ...sourceGroup, platform: 'adobe' })
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()
    expect(vi.mocked(adminAPI.groups.create).mock.calls.at(-1)![0]).toMatchObject({
      name: 'Adobe GPT Image 2 + GPT Image 2.6 Sunburst + GPT Image 2.5 Flare - preview',
      image_quality_prices: { 'gpt-image-2': { '1K': { low: 0.0001 } }, 'gpt-image-2.5-flare': { '4K': { max: 0 } }, 'gpt-image-2.6-sunburst': { '4K': { max: 0.1234 } } },
    })
    wrapper.unmount()
  })

  it('allows ambiguous old names to edit both models and clears only the requested cell', async () => {
    const prices = { 'gpt-image-2.6-flare': { '4K': { max: 0.5 } }, 'gpt-image-2.6-sunburst': { '4K': { max: 0.3 } } }
    listGroups.mockResolvedValue({ items: [{ ...sourceGroup, name: 'Adobe 2.6 - Flare/Sunburst - preview', platform: 'adobe', image_quality_prices: prices }], total: 1, page: 1, page_size: 20, pages: 1 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
    await flushPromises()
    const mode = wrapper.findComponent({ name: 'AdobePricingModeSwitch' })
    expect(mode.props('modelValue')).toBe(true)
    mode.vm.$emit('update:modelValue', true)
    await flushPromises()
    await wrapper.get('input[aria-label="GPT Image 2.6 Flare 4K Max"]').setValue('')
    await wrapper.get('[role="tab"][data-model="gpt-image-2.6-sunburst"]').trigger('click')
    expect(wrapper.get<HTMLInputElement>('input[aria-label="GPT Image 2.6 Sunburst 4K Max"]').element.value).toBe('0.3000')
    await wrapper.get('[data-tour="edit-group-form-name"]').setValue('Adobe Custom - renamed')
    await flushPromises()
    expect(wrapper.get('[role="tab"][data-model="gpt-image-2.6-sunburst"]').attributes('aria-selected')).toBe('true')
    updateGroup.mockResolvedValue(sourceGroup)
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(updateGroup.mock.calls.at(-1)![1].image_quality_prices).toEqual({ 'gpt-image-2.6-sunburst': { '4K': { max: 0.3 } } })
    expect(prices['gpt-image-2.6-flare']['4K'].max).toBe(0.5)
    wrapper.unmount()
  })

  it('sends model filtering before pagination and resets the page', async () => {
    const wrapper = mountView()
    await flushPromises()
    wrapper.findComponent({ name: 'Pagination' }).vm.$emit('update:page', 3)
    await flushPromises()
    const filter = wrapper.findComponent('[data-testid="adobe-model-filter"]')
    filter.vm.$emit('update:modelValue', '2.6')
    filter.vm.$emit('change', '2.6')
    await flushPromises()
    expect(listGroups).toHaveBeenLastCalledWith(1, expect.any(Number), expect.objectContaining({ adobe_model: '2.6' }), expect.any(Object))
    const platform = wrapper.findAllComponents({ name: 'Select' })[0]
    platform.vm.$emit('update:modelValue', 'openai')
    platform.vm.$emit('change', 'openai')
    await flushPromises()
    expect(wrapper.find('[data-testid="adobe-model-filter"]').exists()).toBe(false)
    expect(listGroups).toHaveBeenLastCalledWith(1, expect.any(Number), expect.objectContaining({ platform: 'openai', adobe_model: undefined }), expect.any(Object))
    wrapper.unmount()
  })

  it('creates a canonical Adobe name using the selected model filter', async () => {
    const wrapper = mountView()
    await flushPromises()
    const filter = wrapper.findComponent('[data-testid="adobe-model-filter"]')
    filter.vm.$emit('update:modelValue', 'GPT Image 2.5 Flare')
    filter.vm.$emit('change', 'GPT Image 2.5 Flare')
    await flushPromises()
    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await flushPromises()
    expect(wrapper.findComponent('#adobe-name-model').props('selectedValues')).toEqual(['gpt-image-2.5-flare'])
    await wrapper.get('#adobe-name-customer').setValue(' 杂乱无章 ')
    vi.mocked(adminAPI.groups.create).mockResolvedValue({ ...sourceGroup, platform: 'adobe', name: 'Adobe GPT Image 2.5 Flare - 杂乱无章' })
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()
    expect(adminAPI.groups.create).toHaveBeenCalledWith(expect.objectContaining({ platform: 'adobe', name: 'Adobe GPT Image 2.5 Flare - 杂乱无章' }))
    wrapper.unmount()
  })

  it('duplicates the selected group, reports success, and refreshes the list', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="group-duplicate"]').trigger('click')
    await flushPromises()

    expect(duplicateGroup).toHaveBeenCalledTimes(1)
    expect(duplicateGroup).toHaveBeenCalledWith(42)
    expect(showSuccess).toHaveBeenCalledWith('admin.groups.duplicateSuccess')
    expect(listGroups).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('hides advanced group actions in simple mode', async () => {
    authState.isSimpleMode = true
    const compositeGroup = { ...sourceGroup, platform: 'composite' }
    listGroups.mockResolvedValueOnce({ items: [compositeGroup], total: 1, page: 1, page_size: 20, pages: 1 })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="group-duplicate"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="group-composite-routes"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="group-rate-multipliers"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="group-rpm-overrides"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('ignores repeated clicks while the duplicate request is in flight', async () => {
    let resolveDuplicate!: (value: AdminGroup) => void
    duplicateGroup.mockImplementationOnce(
      () => new Promise<AdminGroup>((resolve) => { resolveDuplicate = resolve })
    )
    const wrapper = mountView()
    await flushPromises()

    const button = wrapper.get('[data-testid="group-duplicate"]')
    void button.trigger('click')
    void button.trigger('click')
    await wrapper.vm.$nextTick()

    expect(duplicateGroup).toHaveBeenCalledTimes(1)
    expect(button.attributes('disabled')).toBeDefined()
    expect(button.attributes('title')).toBe('admin.groups.duplicating')

    resolveDuplicate({ ...sourceGroup, id: 43, name: 'Primary (Copy)', status: 'inactive' })
    await flushPromises()
    expect(wrapper.get('[data-testid="group-duplicate"]').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('shows the API error and restores the action when duplication fails', async () => {
    duplicateGroup.mockRejectedValueOnce(new Error('duplicate failed'))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="group-duplicate"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('duplicate failed')
    expect(wrapper.get('[data-testid="group-duplicate"]').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('does not report a successful duplicate as failed when the refresh fails', async () => {
    listGroups
      .mockResolvedValueOnce({
        items: [sourceGroup],
        total: 1,
        page: 1,
        page_size: 20,
        pages: 1
      })
      .mockRejectedValueOnce(new Error('refresh failed'))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="group-duplicate"]').trigger('click')
    await flushPromises()

    expect(showSuccess).toHaveBeenCalledWith('admin.groups.duplicateSuccess')
    expect(showError).toHaveBeenCalledWith('admin.groups.failedToLoad')
    expect(showError).not.toHaveBeenCalledWith('admin.groups.duplicateFailed')
    wrapper.unmount()
  })

  it('shows the standardized API message when updating a group fails', async () => {
    updateGroup.mockRejectedValueOnce({
      status: 409,
      code: 409,
      message: 'group name already exists',
      reason: 'GROUP_EXISTS'
    })
    const wrapper = mountView()
    await flushPromises()

    const editButton = wrapper.findAll('button').find((button) => button.text() === 'common.edit')
    expect(editButton).toBeTruthy()
    await editButton!.trigger('click')
    await flushPromises()
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledTimes(1)
    expect(showError).toHaveBeenCalledWith('group name already exists')
    wrapper.unmount()
  })

  it('loads, edits, and saves custom reasoning multipliers for group pricing', async () => {
    const group = {
      ...sourceGroup,
      model_pricing: [{
        platform: 'openai', models: ['example-model'], billing_mode: 'token',
        input_price: 3e-6, output_price: 15e-6, cache_write_price: null, cache_read_price: null,
        image_input_price: null, image_output_price: null, per_request_price: null,
        reasoning_effort_multipliers: { high: 1.5, max: 3 }, intervals: [], time_pricing: null,
      }],
    }
    listGroups.mockResolvedValue({ items: [group], total: 1, page: 1, page_size: 20, pages: 1 })
    updateGroup.mockResolvedValue(group)
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'common.edit')!.trigger('click')
    await flushPromises()

    expect(wrapper.get<HTMLInputElement>('[data-reasoning-effort="high"]').element.value).toBe('1.5')
    await wrapper.get('[data-reasoning-effort="high"]').setValue('0.5')
    await wrapper.get('[data-reasoning-effort="max"]').setValue('')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({
      model_pricing: [expect.objectContaining({ reasoning_effort_multipliers: { high: 0.5 } })],
    }))
    wrapper.unmount()
  })

  it('blocks saving an invalid group reasoning multiplier and allows clearing it', async () => {
    const group = {
      ...sourceGroup,
      model_pricing: [{
        platform: 'openai', models: ['example-model'], billing_mode: 'token',
        input_price: 3e-6, output_price: 15e-6, cache_write_price: null, cache_read_price: null,
        image_input_price: null, image_output_price: null, per_request_price: null,
        reasoning_effort_multipliers: { high: 1.5 }, intervals: [], time_pricing: null,
      }],
    }
    listGroups.mockResolvedValue({ items: [group], total: 1, page: 1, page_size: 20, pages: 1 })
    updateGroup.mockResolvedValue(group)
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'common.edit')!.trigger('click')
    await flushPromises()

    await wrapper.get('[data-reasoning-effort="high"]').setValue('0')
    await wrapper.get('#edit-group-form').trigger('submit')
    expect(updateGroup).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith(expect.stringContaining('reasoningEffortMultiplierPositive'))

    await wrapper.get('[data-testid="reasoning-effort-multipliers"] button').trigger('click')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({
      model_pricing: [expect.objectContaining({ reasoning_effort_multipliers: null })],
    }))
    wrapper.unmount()
  })

  it('updates manifest controls immediately and submits the displayed selection', async () => {
    vi.useFakeTimers()
    vi.mocked(adminAPI.accounts.list).mockResolvedValue({
      items: [{ id: 5, name: 'Manifest account' }]
    } as never)
    updateGroup.mockResolvedValue(sourceGroup)
    const wrapper = mountView()
    try {
      await flushPromises()
      const editButton = wrapper.findAll('button').find((button) => button.text() === 'common.edit')!
      await editButton.trigger('click')
      await flushPromises()

      const toggle = wrapper.get('[data-testid="codex-manifest-toggle"]')
      await toggle.trigger('click')
      expect(toggle.attributes('aria-checked')).toBe('true')
      const search = wrapper.get('[data-testid="codex-manifest-search"]')
      await search.trigger('focus')
      await vi.advanceTimersByTimeAsync(300)
      await flushPromises()
      expect(adminAPI.accounts.list).toHaveBeenCalledWith(
        1, 20, { search: '', platform: 'openai', group: '42' }, expect.anything()
      )
      await wrapper.get('[data-testid="codex-manifest-dropdown"] button').trigger('click')
      expect(wrapper.get('[data-testid="codex-manifest-selected-tags"]').text()).toContain('Manifest account')

      await wrapper.get('[aria-label="remove account 5"]').trigger('click')
      expect(wrapper.find('[data-testid="codex-manifest-selected-tags"]').exists()).toBe(false)
      await wrapper.get('#edit-group-form').trigger('submit')
      expect(updateGroup).not.toHaveBeenCalled()
      expect(wrapper.find('[data-testid="codex-manifest-validation-error"]').exists()).toBe(true)

      await search.trigger('focus')
      await wrapper.get('[data-testid="codex-manifest-dropdown"] button').trigger('click')
      expect(wrapper.get('[data-testid="codex-manifest-selected-tags"]').text()).toContain('Manifest account')
      const fallback = wrapper.get('[data-testid="codex-manifest-fallback-toggle"]')
      await fallback.trigger('click')
      expect(fallback.attributes('aria-checked')).toBe('true')
      await fallback.trigger('click')
      expect(fallback.attributes('aria-checked')).toBe('false')
      await fallback.trigger('click')

      await toggle.trigger('click')
      expect(wrapper.find('[data-testid="codex-manifest-search"]').exists()).toBe(false)
      await toggle.trigger('click')
      expect(wrapper.get('[data-testid="codex-manifest-selected-tags"]').text()).toContain('Manifest account')
      await wrapper.get('#edit-group-form').trigger('submit')
      await flushPromises()
      expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({
        codex_models_manifest_config: {
          enabled: true, account_ids: [5], fallback_to_scheduler: true
        }
      }))

      // Reopening reads the saved group afresh, without retaining the prior draft.
      await editButton.trigger('click')
      await flushPromises()
      expect(wrapper.get('[data-testid="codex-manifest-toggle"]').attributes('aria-checked')).toBe('false')
      expect(wrapper.find('[data-testid="codex-manifest-search"]').exists()).toBe(false)
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })

})
