import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DailyChangelog from '@/components/admin/changelog/DailyChangelog.vue'
import { dailyUpdates, groupDailyUpdates, changelogPreviews, dailyOverviews, pendingDailyUpdates } from '@/content/changelog/daily'
import zh from '@/i18n/locales/zh/changelog'
import en from '@/i18n/locales/en/changelog'

vi.mock('vue-i18n', () => ({ useI18n: () => ({
  t: (key: string, values: Record<string, unknown> = {}) => {
    let value: unknown = { changelog: zh }
    for (const part of key.split('.')) value = (value as Record<string, unknown>)?.[part]
    return String(value ?? key).replace(/\{(\w+)\}/g, (_, name: string) => String(values[name] ?? ''))
  },
}) }))

describe('Daily changelog', () => {
  it('includes the proxy fix on its Beijing release date with complete bilingual notes', () => {
    const item = dailyUpdates.find(update => update.id === 'proxyVerificationPriority')!
    expect(item).toBeDefined()
    expect(item.date).toBe('2026-10-22')
    expect(pendingDailyUpdates.some(update => update.id === item.id)).toBe(false)
    for (const messages of [zh, en]) {
      const note = messages.daily.updates.proxyVerificationPriority
      expect(note.title.trim()).toBeTruthy()
      for (const key of item.sections ?? []) {
        const section = note.sections[key as keyof typeof note.sections]
        expect(section.title.trim()).toBeTruthy()
        expect(section.body.trim()).toBeTruthy()
      }
    }
  })

  it('combines releases on the same date and orders days newest first', () => {
    const grouped = groupDailyUpdates([
      { date: '2026-10-17', id: 'earlier', type: 'fixed' },
      { date: '2026-10-18', id: 'morning', type: 'fixed' },
      { date: '2026-10-16', id: 'oldest', type: 'added' },
      { date: '2026-10-18', id: 'evening', type: 'improved' },
    ])
    expect(grouped.map(day => day.date)).toEqual(['2026-10-18', '2026-10-17', '2026-10-16'])
    expect(grouped[0].items.map(item => item.id)).toEqual(['morning', 'evening'])
  })

  it('keeps a valid release date and complete bilingual text for every update', () => {
    expect(new Set(dailyUpdates.map(item => item.id)).size).toBe(dailyUpdates.length)
    for (const item of dailyUpdates) {
      expect(item.date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
      expect(new Date(item.date).toISOString().slice(0, 10)).toBe(item.date)
    }
    for (const item of pendingDailyUpdates) {
      expect('date' in item).toBe(false)
      expect(dailyUpdates.some(released => released.id === item.id)).toBe(false)
    }
    for (const item of [...dailyUpdates, ...pendingDailyUpdates]) {
      for (const messages of [zh, en]) {
        const note = messages.daily.updates[item.id as keyof typeof messages.daily.updates]
        expect(note?.title.trim()).toBeTruthy()
        expect(note?.body.trim()).toBeTruthy()
        const detailed = note as unknown as Record<string, Record<string, Record<string, string>>>
        for (const section of item.sections ?? []) {
          expect(detailed.sections[section].title.trim()).toBeTruthy()
          expect(detailed.sections[section].body.trim()).toBeTruthy()
        }
        for (const metric of item.metrics ?? []) {
          expect(detailed.metrics[metric].label.trim()).toBeTruthy()
          expect(detailed.metrics[metric].value.trim()).toBeTruthy()
          expect(detailed.metrics[metric].note.trim()).toBeTruthy()
        }
      }
    }
    for (const item of changelogPreviews) {
      expect(dailyUpdates.some(update => update.id === item.id)).toBe(false)
      expect('date' in item).toBe(false)
      for (const messages of [zh, en]) {
        expect(messages.daily.updates[item.id]?.title.trim()).toBeTruthy()
        expect(messages.daily.updates[item.id]?.body.trim()).toBeTruthy()
      }
    }
  })

  it('records the subscription release on its Beijing deployment date', () => {
    const item = dailyUpdates.find(update => update.id === 'clashSubscriptions')!
    expect(item.date).toBe('2026-10-22')
    expect(item.sections).toHaveLength(7)
    expect(dailyOverviews[item.date]).toBe('proxyVerificationPriority')
    for (const messages of [zh, en]) {
      expect(messages.daily.updates.clashSubscriptions.title).not.toMatch(/待发布|pending release/)
      expect(messages.daily.updates.clashSubscriptions.body).not.toMatch(/尚未部署|has not been deployed/)
    }
  })

  it('shows one section per day, expands the latest day and preserves older entries', () => {
    const wrapper = mount(DailyChangelog)
    const sections = wrapper.findAll('.daily-release')
    expect(sections).toHaveLength(new Set(dailyUpdates.map(item => item.date)).size)
    expect(sections[0].attributes('open')).toBeDefined()
    expect(sections.slice(1).every(section => section.attributes('open') === undefined)).toBe(true)
    expect(sections[0].findAll('li')).toHaveLength(groupDailyUpdates(dailyUpdates)[0].items.length)
    expect(wrapper.text()).toContain('订阅输入与状态卡片')
    expect(wrapper.text()).toContain('2026-10-12')
    expect(wrapper.findAll('.daily-release [data-update-id]')).toHaveLength(dailyUpdates.length)
    const previews = wrapper.get('[data-testid="changelog-previews"]')
    expect(previews.text()).toContain('调度方案预览 · 规则尚未实施')
    expect(previews.find('[data-testid="account-recovery-demo"]').exists()).toBe(true)
    expect(previews.findAll('[data-release-date]')).toHaveLength(0)
    const release = wrapper.get('[data-update-id="releaseSpeed"]')
    expect(release.findAll('[data-testid="release-metrics"] > div')).toHaveLength(4)
    expect(release.findAll('[data-testid="release-details"] > div')).toHaveLength(7)
    expect(release.text()).toContain('不是每次发布或完整上线的固定时限')
    expect(release.text()).toContain('生产部署步骤均跳过')
    expect(wrapper.get('[data-release-date="2026-10-21"] [data-testid="daily-overview"]').text()).toContain('缩短发布等待')
    const latest = groupDailyUpdates(dailyUpdates)[0]
    const overview = dailyOverviews[latest.date] as keyof typeof zh.daily.overviews
    expect(sections[0].get('[data-testid="daily-overview"]').text()).toContain(zh.daily.overviews[overview].title)
    expect(wrapper.get('[data-release-date="2026-10-22"] [data-testid="daily-overview"]').text()).toContain('代理名额优先保障业务')
    expect(wrapper.text()).not.toContain('changelog.daily.')
    for (const id of Object.values(dailyOverviews)) {
      for (const messages of [zh, en]) {
        const overview = messages.daily.overviews[id as keyof typeof messages.daily.overviews]
        expect(overview.title.trim()).toBeTruthy()
        expect(overview.body.trim()).toBeTruthy()
      }
    }
    wrapper.unmount()
  })
})
