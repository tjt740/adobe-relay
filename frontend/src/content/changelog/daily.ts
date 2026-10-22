export interface DailyUpdate {
  date: string
  id: string
  type: 'added' | 'improved' | 'fixed'
  sections?: readonly string[]
  metrics?: readonly string[]
}

// Proposals have no release date and must not be counted as shipped updates.
// Move a delivered feature into dailyUpdates only on its actual Beijing release date.
export const changelogPreviews = [
  { id: 'accountRecoveryDemo', demo: 'account-recovery' },
] as const

// Prepared release notes have no deployment date and are not rendered as shipped.
// Move the entry into dailyUpdates on its actual Asia/Shanghai deployment date.
export const pendingDailyUpdates: readonly Omit<DailyUpdate, 'date'>[] = [
  { id: 'proxyVerificationPriority', type: 'fixed', sections: ['reason', 'priority', 'quarantine', 'leases', 'protection', 'wording', 'validation', 'boundary'] },
]

export const dailyOverviews: Readonly<Record<string, string>> = {
  '2026-10-22': 'clashSubscriptions',
  '2026-10-21': 'releaseSpeed',
}

// Use the actual release date in Asia/Shanghai; multiple releases share one day.
export const dailyUpdates: readonly DailyUpdate[] = [
  { date: '2026-10-22', id: 'upstreamV0217', type: 'improved', sections: ['reason', 'models', 'gateway', 'payments', 'compatibility', 'validation', 'boundary'] },
  { date: '2026-10-22', id: 'clashSubscriptions', type: 'improved', sections: ['reason', 'fetching', 'editing', 'status', 'scrolling', 'validation', 'boundary'] },
  {
    date: '2026-10-21', id: 'releaseSpeed', type: 'improved',
    metrics: ['baseline', 'cold', 'warm', 'transfer'],
    sections: ['reason', 'parallel', 'cache', 'transfer', 'scenarios', 'validation', 'boundary'],
  },
  { date: '2026-10-20', id: 'adobeManualRevalidation', type: 'fixed' },
  { date: '2026-10-20', id: 'adobeQuotaDiagnostic', type: 'fixed' },
  { date: '2026-10-20', id: 'adobeSessionRenewal', type: 'fixed' },
  { date: '2026-10-20', id: 'recoveryDemoPublished', type: 'added' },
  { date: '2026-10-20', id: 'accountCheckState', type: 'fixed' },
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
