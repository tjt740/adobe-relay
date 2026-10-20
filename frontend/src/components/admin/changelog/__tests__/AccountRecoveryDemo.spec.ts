import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { baseCompile } from '@intlify/message-compiler'
import AccountRecoveryDemo from '../AccountRecoveryDemo.vue'
import { recoveryRetryOptions, recoveryScenarios, getRecoveryDemo, recoveryReviewPoints, recoveryDemoStorageKey } from '@/content/changelog/accountRecoveryDemo'
import zh from '@/i18n/locales/zh/changelog'
import en from '@/i18n/locales/en/changelog'

function render(locale = 'zh') {
  const i18n = createI18n({
    legacy: false, locale, messages: { zh: { changelog: zh }, en: { changelog: en } },
    // Vitest uses the runtime-only i18n alias without Vite's JIT definition.
    // Use the real message compiler here so interpolation is tested as well.
    messageCompiler: message => {
      if (typeof message !== 'string') throw new Error('Expected a locale message')
      return new Function(`return ${baseCompile(message, { mode: 'arrow' }).code}`)()
    },
  })
  return mount(AccountRecoveryDemo, { global: { plugins: [i18n] } })
}

describe('Account recovery proposal demo', () => {
  beforeEach(() => localStorage.removeItem(recoveryDemoStorageKey))

  it('remembers valid browser-only choices and ignores invalid saved values', async () => {
    let wrapper = render()
    await wrapper.get('[data-testid="recovery-budget"]').setValue('3')
    await wrapper.get('[data-testid="recovery-scenario"]').setValue('ownership')
    wrapper.unmount()
    wrapper = render()
    expect((wrapper.get('[data-testid="recovery-scenario"]').element as HTMLSelectElement).value).toBe('ownership')
    expect((wrapper.get('[data-testid="recovery-budget"]').element as HTMLSelectElement).value).toBe('3')
    wrapper.unmount()
    localStorage.setItem(recoveryDemoStorageKey, '{"scenario":"enable-all","retryLimit":999}')
    wrapper = render()
    expect((wrapper.get('[data-testid="recovery-scenario"]').element as HTMLSelectElement).value).toBe('temporary')
    expect((wrapper.get('[data-testid="recovery-budget"]').element as HTMLSelectElement).value).toBe('2')
    wrapper.unmount()
  })
  it('changes both the proposed outcome and extra-check cost when the retry budget changes', async () => {
    const wrapper = render()
    expect(wrapper.get('[data-testid="recovery-proposed"]').text()).toContain('第 2 次重试成功')
    await wrapper.get('[data-testid="recovery-budget"]').setValue('1')
    expect(wrapper.get('[data-testid="recovery-proposed"]').text()).toContain('达到重试上限')
    expect(wrapper.get('[data-testid="recovery-cost"]').text()).toContain('最多增加 1 次')
    expect(wrapper.get('[data-testid="recovery-current"]').text()).toContain('失败任务不自动重新领取')
    await wrapper.get('[data-testid="recovery-scenario"]').setValue('persistent')
    await wrapper.get('[data-testid="recovery-budget"]').setValue('3')
    expect(wrapper.get('[data-testid="recovery-proposed"]').text()).toContain('达到重试上限')
    expect(wrapper.get('[data-testid="recovery-proposed"]').text()).toContain('240 秒')
    wrapper.unmount()
  })

  it('does not suggest new generation retries for accepted or uncertain jobs, or enable paused accounts', async () => {
    const wrapper = render()
    for (const [scenario, text] of [
      ['accepted', '仅在原任务内重试'],
      ['unknown', '无法确认则待复核'],
      ['manual', '保留人工暂停'],
      ['ownership', '核对暂停来源'],
      ['later', '旧成功不覆盖新失败'],
      ['auth', '不走临时故障重试'],
      ['quota', '不额外出图重试'],
    ]) {
      await wrapper.get('[data-testid="recovery-scenario"]').setValue(scenario)
      expect(wrapper.get('[data-testid="recovery-budget"]').attributes('disabled')).toBeDefined()
      expect(wrapper.get('[data-testid="recovery-proposed"]').text()).toContain(text)
    }
    wrapper.unmount()
  })

  it('has complete bilingual content for every scenario and every reachable budget', async () => {
    for (const locale of ['zh', 'en']) {
      const wrapper = render(locale)
      for (const scenario of recoveryScenarios) {
        await wrapper.get('[data-testid="recovery-scenario"]').setValue(scenario)
        for (const limit of recoveryRetryOptions) {
          const flow = getRecoveryDemo(scenario, limit)
          if (flow.retryable) await wrapper.get('[data-testid="recovery-budget"]').setValue(String(limit))
          expect(wrapper.get('[data-testid="recovery-proposed"]').findAll('li')).toHaveLength(4)
          expect(wrapper.text()).not.toContain('changelog.daily.')
          expect(wrapper.get('[data-testid="recovery-cost"]').text()).toBeTruthy()
        }
      }
      const messages = locale === 'zh' ? zh : en
      for (const point of recoveryReviewPoints) {
        expect(wrapper.text()).toContain(messages.daily.recoveryDemo.review[point].title)
      }
      wrapper.unmount()
    }
  })
})
