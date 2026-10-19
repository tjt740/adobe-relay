import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DailyChangelog from '@/components/admin/changelog/DailyChangelog.vue'
import { dailyUpdates, groupDailyUpdates } from '@/content/changelog/daily'
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
      for (const messages of [zh, en]) {
        const note = messages.daily.updates[item.id as keyof typeof messages.daily.updates]
        expect(note?.title.trim()).toBeTruthy()
        expect(note?.body.trim()).toBeTruthy()
      }
    }
  })

  it('shows one section per day, expands the latest day and preserves older entries', () => {
    const wrapper = mount(DailyChangelog)
    const sections = wrapper.findAll('details')
    expect(sections).toHaveLength(new Set(dailyUpdates.map(item => item.date)).size)
    expect(sections[0].attributes('open')).toBeDefined()
    expect(sections.slice(1).every(section => section.attributes('open') === undefined)).toBe(true)
    expect(sections[0].findAll('li')).toHaveLength(groupDailyUpdates(dailyUpdates)[0].items.length)
    expect(wrapper.text()).toContain('订阅输入与状态卡片')
    expect(wrapper.text()).toContain('2026-10-12')
    expect(wrapper.findAll('li')).toHaveLength(dailyUpdates.length)
    wrapper.unmount()
  })
})
