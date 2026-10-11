import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpsAlertRulesCard from '../OpsAlertRulesCard.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), update: vi.fn(), success: vi.fn(), error: vi.fn() }))
vi.mock('@/api', () => ({ adminAPI: { groups: { getAll: async () => [] } } }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { listAlertRules: mocks.list, updateAlertRule: mocks.update } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.error, showSuccess: mocks.success }) }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...(await importOriginal<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string, values?: unknown) => key + (values ? JSON.stringify(values) : '') }) }))
vi.mock('@vueuse/core', async (importOriginal) => { const { ref } = await import('vue'); return { ...(await importOriginal<typeof import('@vueuse/core')>()), useMediaQuery: () => ref(true) } })

const rule = () => ({
 id: 9, name: 'Adobe alerts', enabled: true, metric_type: 'adobe_401_408_accounts', operator: '>=', threshold: 5,
 window_minutes: 10, sustained_minutes: 1, severity: 'P1', cooldown_minutes: 5, notify_email: true,
 filters: { adobe_error_ratio_percent: 20, email_recipient: 'owner@example.test', platform: 'adobe' }
})
const setup = async () => {
 const wrapper = mount(OpsAlertRulesCard, { global: { stubs: {
  BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' },
  ConfirmDialog: true, Select: true
 } } })
 await flushPromises()
 return wrapper
}

describe('Adobe alert ratio rule', () => {
 beforeEach(() => { vi.clearAllMocks(); mocks.list.mockResolvedValue([rule()]); mocks.update.mockResolvedValue({}) })
 it('shows the percentage alongside the count and preserves recipient and cooldown on edit', async () => {
  const w = await setup()
  expect(w.text()).toContain('adobeRatioSummary{"percent":20}')
  await w.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
  expect((w.get('#adobe-alert-ratio').element as HTMLInputElement).value).toBe('20')
  await w.get('#adobe-alert-ratio').setValue('25')
  await w.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
  await flushPromises()
  expect(mocks.update).toHaveBeenCalledWith(9, expect.objectContaining({ threshold: 5, cooldown_minutes: 5,
   filters: { adobe_error_ratio_percent: 25, email_recipient: 'owner@example.test', platform: 'adobe' }
  }))
  w.unmount()
 })
 it('rejects zero rather than silently disabling the percentage rule', async () => {
  const w = await setup()
  await w.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
  await w.get('#adobe-alert-ratio').setValue('0')
  expect(w.text()).toContain('admin.ops.alertRules.adobeRatioInvalid')
  await w.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
  await flushPromises()
  expect(mocks.error).toHaveBeenCalledWith('admin.ops.alertRules.adobeRatioInvalid')
  expect(mocks.update).not.toHaveBeenCalled()
  w.unmount()
 })
 it('keeps legacy count-only rules count-only when saved', async () => {
  const legacy = rule(); delete (legacy.filters as Record<string, unknown>).adobe_error_ratio_percent
  mocks.list.mockResolvedValue([legacy])
  const w = await setup()
  expect(w.text()).not.toContain('adobeRatioSummary')
  await w.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
  expect((w.get('#adobe-alert-ratio').element as HTMLInputElement).value).toBe('')
  await w.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
  await flushPromises()
  expect(mocks.update.mock.calls[0][1].filters).not.toHaveProperty('adobe_error_ratio_percent')
  w.unmount()
 })
})
