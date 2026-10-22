import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ClashSubscriptionModal from '@/components/admin/proxy/ClashSubscriptionModal.vue'
import { getClashSubscription, previewClashSubscription, importClashSubscription, checkClashSource, type ClashSubscription } from '@/api/admin/clash'
import { testProxy } from '@/api/admin/proxies'

vi.mock('@/api/admin/clash', () => ({ getClashSubscription: vi.fn(), previewClashSubscription: vi.fn(), importClashSubscription: vi.fn(), checkClashSource: vi.fn() }))
vi.mock('@/api/admin/proxies', () => ({ testProxy: vi.fn() }))
vi.mock('@/utils/format', () => ({ formatDateTime: (s: string) => s }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const saved: ClashSubscription = {
  configured: true, ready: true, revision: 2, url_hint: 'https://example.com/••••', updated_at: '2026-09-24T00:00:00Z',
  nodes: [{ name: 'Japan', type: 'ss', server: 'jp.example.com', server_port: 443, proxy_id: 7, proxy_host: 'mihomo', proxy_port: 20000, enabled: true }]
}
async function setup() {
  const wrapper = mount(ClashSubscriptionModal, { global: { stubs: { BaseDialog: { template: '<div><slot /><footer><slot name="footer" /></footer></div>' } } } })
  await flushPromises()
  return wrapper
}
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(getClashSubscription).mockResolvedValue(saved)
  vi.mocked(previewClashSubscription).mockResolvedValue({ ...saved, nodes: [...saved.nodes, { name: 'US', type: 'vmess', proxy_id: 0, enabled: true }] })
  vi.mocked(importClashSubscription).mockResolvedValue({ ...saved, revision: 3 })
  vi.mocked(testProxy).mockResolvedValue({ success: true, latency_ms: 42, message: 'OK' })
})
describe('Clash subscription workflow', () => {
  it('reuses the saved URL, previews nodes, and saves the exact selection with revision', async () => {
    const w = await setup()
    expect(w.get('#clash-subscription-url').element).toHaveProperty('value', '')
    await w.get('form').trigger('submit'); await flushPromises()
    await w.get('input[aria-label="US"]').setValue(false)
    await w.get('footer .btn-primary').trigger('click'); await flushPromises()
    expect(previewClashSubscription).toHaveBeenCalledWith('')
    expect(importClashSubscription).toHaveBeenCalledWith('', ['Japan'], 2)
    expect(w.emitted('changed')).toHaveLength(1)
  })
  it('invalidates the preview when the subscription link changes', async () => {
    const w = await setup()
    await w.get('form').trigger('submit'); await flushPromises()
    await w.get('#clash-subscription-url').setValue('https://replacement.example/private')
    expect(w.get('footer .btn-primary').attributes('disabled')).toBeDefined()
    expect(w.find('input[type="checkbox"]').exists()).toBe(false)
  })
  it('preserves the preview and shows a failed import without claiming success', async () => {
    vi.mocked(importClashSubscription).mockRejectedValue(new Error('Runtime unavailable'))
    const w = await setup()
    await w.get('form').trigger('submit'); await flushPromises()
    await w.get('footer .btn-primary').trigger('click'); await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('Runtime unavailable')
    expect(w.emitted('changed')).toBeUndefined()
    expect(w.findAll('input[type="checkbox"]')).toHaveLength(2)
  })
  it('allows an empty selection to delete all imported nodes and refresh the pool', async () => {
    vi.mocked(importClashSubscription).mockResolvedValue({ ...saved, revision: 3, nodes: [] })
    const w = await setup()
    await w.get('form').trigger('submit'); await flushPromises()
    for (const checkbox of w.findAll('input[type="checkbox"]')) await checkbox.setValue(false)
    await w.get('footer .btn-primary').trigger('click'); await flushPromises()
    expect(importClashSubscription).toHaveBeenCalledWith('', [], 2)
    expect(w.emitted('changed')).toHaveLength(1)
    expect(w.text()).not.toContain('jp.example.com:443')
    expect(w.text()).toContain('admin.proxies.clash.emptyTitle')
  })
  it('tests only enabled imported nodes and renders latency', async () => {
    const w = await setup()
    await w.get('form').trigger('submit'); await flushPromises()
    const button = w.findAll('button').find(b => b.text() === 'admin.proxies.clash.test')!
    await button.trigger('click'); await flushPromises()
    expect(testProxy).toHaveBeenCalledTimes(1)
    expect(testProxy).toHaveBeenCalledWith(7)
    expect(w.text()).toContain('42 ms')
    expect(w.emitted('changed')).toHaveLength(1)
  })
  it('disables importing until the runtime has been configured', async () => {
    vi.mocked(getClashSubscription).mockResolvedValue({ ...saved, configured: false, ready: false })
    const w = await setup()
    await w.get('form').trigger('submit'); await flushPromises()
    expect(w.text()).toContain('admin.proxies.clash.setupHint')
    expect(w.get('footer .btn-primary').attributes('disabled')).toBeDefined()
  })
  it('shows the saved URL and node endpoints, with an explicit visibility toggle', async () => {
    vi.mocked(getClashSubscription).mockResolvedValue({ ...saved, url: 'https://subscription.example/private' })
    const w = await setup()
    const input = w.get('#clash-subscription-url')
    expect(input.element.tagName).toBe('INPUT')
    expect(input.attributes('type')).toBe('text')
    expect(input.element).toHaveProperty('value', 'https://subscription.example/private')
    await w.get('button[aria-label="admin.proxies.clash.hideURL"]').trigger('click')
    expect(input.attributes('type')).toBe('password')
    await w.get('button[aria-label="admin.proxies.clash.showURL"]').trigger('click')
    expect(input.attributes('type')).toBe('text')
    expect(w.text()).toContain('jp.example.com:443')
    expect(w.text()).toContain('mihomo:20000')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(previewClashSubscription).toHaveBeenCalledWith('')
  })
  it('keeps the preview loading state until the request resolves and recovers on failure', async () => {
    let reject!: (reason: Error) => void
    vi.mocked(previewClashSubscription).mockReturnValue(new Promise((_, fail) => { reject = fail }))
    const w = await setup()
    await w.get('form').trigger('submit')
    expect(w.get('[data-testid="clash-loading"]').text()).toContain('admin.proxies.clash.previewing')
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await w.get('form').trigger('submit')
    expect(previewClashSubscription).toHaveBeenCalledTimes(1)
    reject(new Error('Subscription timed out')); await flushPromises()
    expect(w.find('[data-testid="clash-loading"]').exists()).toBe(false)
    expect(w.get('[role="alert"]').text()).toContain('Subscription timed out')
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    expect(w.get('footer .btn-primary').attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('jp.example.com:443')
  })
  it('shows real test progress and each failed result', async () => {
    let resolve!: (result: { success: boolean; message: string }) => void
    vi.mocked(testProxy).mockReturnValue(new Promise(done => { resolve = done }))
    const w = await setup()
    await w.findAll('button').find(b => b.text() === 'admin.proxies.clash.test')!.trigger('click')
    expect(w.get('progress').attributes('value')).toBe('0')
    expect(w.get('progress').attributes('max')).toBe('1')
    expect(w.text()).toContain('admin.proxies.clash.nodeTesting')
    resolve({ success: false, message: 'Connection timed out' }); await flushPromises()
    expect(w.find('progress').exists()).toBe(false)
    expect(w.text()).toContain('Connection timed out')
    expect(w.text()).toContain('admin.proxies.clash.testComplete')
    expect(w.text()).not.toContain('admin.proxies.clash.nodeTesting')
  })
  it('shows import feedback while saving and retains the saved URL', async () => {
    let resolve!: (result: ClashSubscription) => void
    vi.mocked(importClashSubscription).mockReturnValue(new Promise(done => { resolve = done }))
    const w = await setup()
    await w.get('#clash-subscription-url').setValue('https://replacement.example/private')
    await w.get('form').trigger('submit'); await flushPromises()
    await w.get('footer .btn-primary').trigger('click')
    expect(w.text()).toContain('admin.proxies.clash.savingHint')
    expect(w.get('footer .btn-secondary').attributes('disabled')).toBeDefined()
    resolve({ ...saved, url: 'https://replacement.example/private' }); await flushPromises()
    expect(w.text()).not.toContain('admin.proxies.clash.savingHint')
    expect(w.get('#clash-subscription-url').element).toHaveProperty('value', 'https://replacement.example/private')
    expect(w.emitted('changed')).toHaveLength(1)
  })
  it('filters node information without changing the selected nodes', async () => {
    const w = await setup()
    await w.get('form').trigger('submit'); await flushPromises()
    await w.get('input[aria-label="admin.proxies.clash.searchNodes"]').setValue('jp.example')
    expect(w.find('input[aria-label="Japan"]').exists()).toBe(true)
    expect(w.find('input[aria-label="US"]').exists()).toBe(false)
    await w.get('footer .btn-primary').trigger('click'); await flushPromises()
    expect(importClashSubscription).toHaveBeenCalledWith('', ['Japan', 'US'], 2)
  })

  it('preserves more than eight URLs when switching between bulk and list editors', async () => {
    const w = await setup()
    const findButton = (key: string) => w.findAll('button').find(button => button.text() === key)!
    await findButton('admin.proxies.clash.bulkMode').trigger('click')
    const urls = Array.from({ length: 12 }, (_, i) => `https://feed${i}.example/sub`).join('\n')
    await w.get('#clash-bulk-urls').setValue(urls)
    await findButton('admin.proxies.clash.listMode').trigger('click')
    expect(w.findAll('[data-testid="subscription-row"]')).toHaveLength(12)
    await w.get('#clash-subscription-url').setValue('https://changed.example/sub')
    await findButton('admin.proxies.clash.bulkMode').trigger('click')
    expect((w.get('#clash-bulk-urls').element as HTMLTextAreaElement).value).toContain('https://changed.example/sub')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(previewClashSubscription).toHaveBeenCalledWith(urls.replace('https://feed0.example/sub', 'https://changed.example/sub'))
  })
  it('checks only the requested row without importing or testing existing proxies', async () => {
    vi.mocked(checkClashSource).mockResolvedValue({ id: 'one', index: 1, url_hint: '', node_count: 9 })
    vi.mocked(getClashSubscription).mockResolvedValue({ ...saved, url: 'https://one.example/sub\nhttps://two.example/sub' })
    const w = await setup()
    await w.findAll('[data-testid="subscription-row"]')[1].findAll('button')[0].trigger('click'); await flushPromises()
    expect(checkClashSource).toHaveBeenCalledWith('https://two.example/sub')
    expect(previewClashSubscription).not.toHaveBeenCalled()
    expect(importClashSubscription).not.toHaveBeenCalled()
    expect(testProxy).not.toHaveBeenCalled()
    expect(w.text()).toContain('admin.proxies.clash.sourceReadable')
  })
  it('keeps failed-source nodes selected and allows other subscriptions to save', async () => {
    vi.mocked(previewClashSubscription).mockResolvedValue({ ...saved, sources: [
      { id: 'one', index: 1, url_hint: '', node_count: 1 },
      { id: 'two', index: 2, url_hint: '', node_count: 0, fetch_error: 'http_error', http_status: 503 }
    ], nodes: [{ ...saved.nodes[0], source_id: 'one' }, { name: 'Retained', type: 'ss', proxy_id: 8, source_id: 'two', enabled: true, retained: true }] })
    const w = await setup()
    await w.get('form').trigger('submit'); await flushPromises()
    expect(w.get('input[aria-label="Retained"]').attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('HTTP 503')
    await w.findAll('button').find(button => button.text() === 'admin.proxies.clash.selectNone')!.trigger('click')
    expect((w.get('input[aria-label="Retained"]').element as HTMLInputElement).checked).toBe(true)
    await w.get('footer .btn-primary').trigger('click'); await flushPromises()
    expect(importClashSubscription).toHaveBeenCalledWith('', ['Retained'], 2, { Retained: 'two' })
  })
  it('does not allow an all-failed preview to overwrite the saved configuration', async () => {
    vi.mocked(previewClashSubscription).mockResolvedValue({ ...saved, sources: [{ id: 'one', index: 1, url_hint: '', node_count: 0, fetch_error: 'connection_failed' }], nodes: [{ ...saved.nodes[0], retained: true }] })
    const w = await setup()
    await w.get('form').trigger('submit'); await flushPromises()
    expect(w.text()).toContain('admin.proxies.clash.partialFailure')
    expect(w.get('footer .btn-primary').attributes('disabled')).toBeDefined()
  })

  it('does not claim a new or migrated subscription was successfully checked with zero nodes', async () => {
    vi.mocked(getClashSubscription).mockResolvedValue({ ...saved, url: 'https://one.example/sub', sources: [{ id: 'one', index: 1, url_hint: '', node_count: 0, checked_at: '0001-01-01T00:00:00Z' }] })
    const w = await setup()
    expect(w.text()).not.toContain('admin.proxies.clash.sourceReadable')
    expect(w.get('footer .btn-primary').attributes('disabled')).toBeDefined()
  })

})
