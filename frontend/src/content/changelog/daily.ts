export interface DailyUpdate {
  date: string
  id: string
  type: 'added' | 'improved' | 'fixed'
}

// Use the actual release date in Asia/Shanghai; multiple releases share one day.
export const dailyUpdates: readonly DailyUpdate[] = [
  { date: '2026-10-20', id: 'okadGroups', type: 'added' },
  { date: '2026-10-18', id: 'subscriptionUI', type: 'improved' },
  { date: '2026-10-18', id: 'dailyHistory', type: 'added' },
  { date: '2026-10-18', id: 'adobeModels', type: 'fixed' },
  { date: '2026-10-18', id: 'qualityLabel', type: 'fixed' },
  { date: '2026-10-17', id: 'proxyRouting', type: 'improved' },
  { date: '2026-10-17', id: 'capacity', type: 'improved' },
  { date: '2026-10-16', id: 'groupIDs', type: 'improved' },
  { date: '2026-10-16', id: 'imageAdmission', type: 'fixed' },
  { date: '2026-10-15', id: 'imageSafety', type: 'fixed' },
  { date: '2026-10-15', id: 'imageDelivery', type: 'improved' },
  { date: '2026-10-15', id: 'imageMemory', type: 'improved' },
  { date: '2026-10-14', id: 'quietAlerts', type: 'improved' },
  { date: '2026-10-14', id: 'imageURL', type: 'added' },
  { date: '2026-10-14', id: 'accountDetails', type: 'improved' },
  { date: '2026-10-13', id: 'accountEvidence', type: 'added' },
  { date: '2026-10-13', id: 'reliableLink', type: 'improved' },
  { date: '2026-10-12', id: 'recoveryProtection', type: 'improved' },
  { date: '2026-10-12', id: 'adobeAlerts', type: 'added' },
]

export function groupDailyUpdates(updates: readonly DailyUpdate[]) {
  const days = new Map<string, DailyUpdate[]>()
  for (const update of updates) {
    const items = days.get(update.date) ?? []
    items.push(update)
    days.set(update.date, items)
  }
  return [...days].sort(([a], [b]) => b.localeCompare(a)).map(([date, items]) => ({ date, items }))
}
