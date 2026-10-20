// Explanatory scenarios only. No account data, API calls, or scheduling settings.
export const recoveryScenarios = [
  'temporary', 'rate', 'persistent', 'auth', 'quota', 'manual',
  'later', 'stale', 'ownership', 'accepted', 'unknown',
] as const
export type RecoveryScenario = typeof recoveryScenarios[number]
export const recoveryRetryOptions = [1, 2, 3] as const
export type RecoveryRetryLimit = typeof recoveryRetryOptions[number]
export const recoveryDemoStorageKey = 'changelog-account-recovery-demo-v1'

const stepTones = {
  cookie: 'waiting', temporaryFailure: 'waiting', noRetry: 'waiting', awaitingCheck: 'waiting',
  classifyTemporary: 'waiting', backoff: 'retry', recovered: 'ready', budgetEnded: 'waiting',
  request: 'neutral', invalidCredentials: 'waiting', authRecovery: 'waiting', noAuthRetry: 'waiting',
  quota: 'waiting', quotaCooldown: 'waiting', quotaWait: 'waiting', noQuotaRetry: 'waiting',
  manualPause: 'waiting', success: 'ready', keepManualPause: 'neutral', awaitOperator: 'waiting',
  new401: 'waiting', oldProof: 'neutral', waitCredentials: 'waiting',
  versionA: 'neutral', versionB: 'neutral', oldSuccess: 'ready', rejectOldProof: 'waiting', verifyVersionB: 'retry',
  historicalPause: 'waiting', noResumePermission: 'neutral', inspectPauseOwner: 'neutral', leavePaused: 'waiting',
  acceptedJob: 'neutral', pollTimeout: 'waiting', sameJob: 'retry', sameJobResult: 'neutral',
  unknownResult: 'waiting', interrupted: 'waiting', verifyReceipt: 'neutral', holdForEvidence: 'waiting',
} as const
export type RecoveryStep = keyof typeof stepTones
export type RecoveryTone = typeof stepTones[RecoveryStep]

const flows: Record<Exclude<RecoveryScenario, 'temporary' | 'rate' | 'persistent'>, {
  current: RecoveryStep[]
  proposed: RecoveryStep[]
}> = {
  auth: { current: ['request', 'invalidCredentials', 'authRecovery', 'waitCredentials'], proposed: ['request', 'invalidCredentials', 'noAuthRetry', 'waitCredentials'] },
  quota: { current: ['cookie', 'quota', 'quotaCooldown', 'quotaWait'], proposed: ['cookie', 'quota', 'noQuotaRetry', 'quotaWait'] },
  manual: { current: ['manualPause', 'success', 'keepManualPause', 'awaitOperator'], proposed: ['manualPause', 'success', 'keepManualPause', 'awaitOperator'] },
  later: { current: ['success', 'new401', 'authRecovery', 'oldProof'], proposed: ['success', 'new401', 'authRecovery', 'oldProof'] },
  stale: { current: ['versionA', 'versionB', 'oldSuccess', 'rejectOldProof'], proposed: ['versionA', 'versionB', 'rejectOldProof', 'verifyVersionB'] },
  ownership: { current: ['historicalPause', 'success', 'noResumePermission', 'leavePaused'], proposed: ['historicalPause', 'success', 'inspectPauseOwner', 'leavePaused'] },
  accepted: { current: ['acceptedJob', 'pollTimeout', 'sameJob', 'sameJobResult'], proposed: ['acceptedJob', 'pollTimeout', 'sameJob', 'sameJobResult'] },
  unknown: { current: ['request', 'unknownResult', 'interrupted', 'holdForEvidence'], proposed: ['request', 'unknownResult', 'verifyReceipt', 'holdForEvidence'] },
}

export function getRecoveryDemo(scenario: RecoveryScenario, retryLimit: RecoveryRetryLimit) {
  const retryable = scenario === 'temporary' || scenario === 'rate' || scenario === 'persistent'
  const recovered = retryable && scenario !== 'persistent' && retryLimit >= 2
  const flow = retryable
    ? { current: ['cookie', 'temporaryFailure', 'noRetry', 'awaitingCheck'] as RecoveryStep[],
      proposed: ['cookie', 'classifyTemporary', 'backoff', recovered ? 'recovered' : 'budgetEnded'] as RecoveryStep[] }
    : flows[scenario]
  return {
    ...flow,
    retryable,
    recovered,
    extraChecks: retryable ? retryLimit : 0,
    delays: [60, 120, 240].slice(0, retryLimit),
    tone: (step: RecoveryStep): RecoveryTone => stepTones[step],
    result: recovered ? 'recovered' : 'held',
  }
}

export const recoveryReviewPoints = [
  'classification', 'acceptedTask', 'budget', 'exhaustion', 'eligibility', 'ownership', 'modelScope', 'display',
] as const
