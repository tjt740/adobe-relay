<template>
  <section data-testid="account-recovery-demo" :aria-label="t(`${prefix}.title`)" class="space-y-5">
    <div class="rounded-xl bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900 dark:bg-amber-900/20 dark:text-amber-200" role="note">
      {{ t(`${prefix}.previewNotice`) }}
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <label class="min-w-0 space-y-2 text-sm font-medium text-gray-800 dark:text-gray-200">
        <span>{{ t(`${prefix}.scenarioLabel`) }}</span>
        <select v-model="scenario" data-testid="recovery-scenario" class="input w-full min-w-0 text-sm">
          <option v-for="item in recoveryScenarios" :key="item" :value="item">{{ t(`${prefix}.scenarios.${item}.label`) }}</option>
        </select>
      </label>
      <label class="min-w-0 space-y-2 text-sm font-medium text-gray-800 dark:text-gray-200">
        <span>{{ t(`${prefix}.budgetLabel`) }}</span>
        <select v-model.number="retryLimit" :disabled="!demo.retryable" data-testid="recovery-budget" :aria-describedby="`${instanceId}-budget-hint`" class="input w-full min-w-0 text-sm disabled:cursor-not-allowed disabled:opacity-60">
          <option v-for="count in recoveryRetryOptions" :key="count" :value="count">{{ t(`${prefix}.retryCount`, { count }) }}</option>
        </select>
      </label>
    </div>
    <p :id="`${instanceId}-budget-hint`" class="text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t(`${prefix}.budgetHint`) }}</p>
    <div v-for="lane in lanes" :key="lane.id" class="space-y-3">
      <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t(`${prefix}.${lane.id}Title`) }} <span class="ml-2 font-normal text-gray-500 dark:text-gray-400">{{ t(`${prefix}.${lane.id}Hint`) }}</span></h4>
      <ol :data-testid="`recovery-${lane.id}`" :aria-label="t(`${prefix}.${lane.id}Title`)" class="recovery-flow">
        <li v-for="(step, index) in lane.steps" :key="`${index}-${step}`" class="recovery-step text-center" :class="toneStyles[demo.tone(step)]" :data-step="step">
          <span class="text-sm font-semibold leading-6 text-gray-900 dark:text-gray-100">{{ t(`${prefix}.steps.${step}.title`) }}</span>
          <span class="text-xs leading-6 text-gray-600 dark:text-gray-300">{{ t(`${prefix}.steps.${step}.body`, stepValues) }}</span>
          <span v-if="index < lane.steps.length - 1" aria-hidden="true" class="recovery-arrow text-gray-400 dark:text-gray-500">→</span>
        </li>
      </ol>
    </div>
    <div aria-live="polite" aria-atomic="true" class="space-y-4">
      <p data-testid="recovery-assumption" class="text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t(`${prefix}.scenarios.${scenario}.detail`) }}</p>
      <dl class="grid gap-x-5 gap-y-2 text-sm leading-7 sm:grid-cols-[auto_1fr]">
        <dt class="font-medium text-gray-900 dark:text-gray-100">{{ t(`${prefix}.gainLabel`) }}</dt>
        <dd data-testid="recovery-gain" class="text-gray-700 dark:text-gray-300">{{ t(gainKey) }}</dd>
        <dt class="mt-2 font-medium text-gray-900 dark:text-gray-100 sm:mt-0">{{ t(`${prefix}.costLabel`) }}</dt>
        <dd data-testid="recovery-cost" class="text-gray-700 dark:text-gray-300">{{ t(costKey, { count: demo.extraChecks }) }}</dd>
        <dt class="mt-2 font-medium text-gray-900 dark:text-gray-100 sm:mt-0">{{ t(`${prefix}.limitLabel`) }}</dt>
        <dd data-testid="recovery-limit" class="text-gray-700 dark:text-gray-300">{{ t(limitKey) }}</dd>
      </dl>
    </div>
    <details class="border-t border-gray-200 pt-4 dark:border-dark-700" data-testid="recovery-review">
      <summary class="cursor-pointer text-sm font-medium text-gray-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-gray-100">{{ t(`${prefix}.reviewTitle`) }}</summary>
      <dl class="mt-4 space-y-4">
        <div v-for="point in recoveryReviewPoints" :key="point">
          <dt class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ t(`${prefix}.review.${point}.title`) }}</dt>
          <dd class="mt-1 text-sm leading-7 text-gray-600 dark:text-gray-400">{{ t(`${prefix}.review.${point}.body`) }}</dd>
        </div>
      </dl>
    </details>
    <ul class="flex list-none flex-wrap gap-x-5 gap-y-2 border-t border-gray-100 pt-4 text-xs leading-6 text-gray-500 dark:border-dark-700 dark:text-gray-400" :aria-label="t(`${prefix}.boundariesLabel`)">
      <li v-for="boundary in boundaries" :key="boundary">{{ t(`${prefix}.boundaries.${boundary}`) }}</li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getRecoveryDemo, recoveryRetryOptions, recoveryScenarios, recoveryReviewPoints, recoveryDemoStorageKey, type RecoveryScenario, type RecoveryRetryLimit } from '@/content/changelog/accountRecoveryDemo'

withDefaults(defineProps<{ instanceId?: string }>(), { instanceId: 'account-recovery-demo' })
const { t } = useI18n()
const prefix = 'changelog.daily.recoveryDemo'
const scenario = ref<RecoveryScenario>('temporary')
const retryLimit = ref<RecoveryRetryLimit>(2)
try {
  const saved = JSON.parse(localStorage.getItem(recoveryDemoStorageKey) || 'null')
  if (saved && recoveryScenarios.includes(saved.scenario) && recoveryRetryOptions.includes(saved.retryLimit)) {
    scenario.value = saved.scenario
    retryLimit.value = saved.retryLimit
  }
} catch { /* The explainer also works when browser storage is unavailable. */ }
watch([scenario, retryLimit], () => {
  try { localStorage.setItem(recoveryDemoStorageKey, JSON.stringify({ scenario: scenario.value, retryLimit: retryLimit.value })) } catch { /* Browser preferences are optional. */ }
})
const demo = computed(() => getRecoveryDemo(scenario.value, retryLimit.value))
const lanes = computed(() => [{ id: 'current', steps: demo.value.current }, { id: 'proposed', steps: demo.value.proposed }])
const stepValues = computed(() => ({ count: retryLimit.value, reason: t(`${prefix}.${scenario.value === 'rate' ? 'rateReason' : 'temporaryReason'}`), delays: demo.value.delays.map(count => t(`${prefix}.seconds`, { count })).join(' / ') }))
const gainKey = computed(() => demo.value.retryable ? `${prefix}.${demo.value.recovered ? 'retryGain' : 'noRecoveryGain'}` : `${prefix}.scenarios.${scenario.value}.gain`)
const costKey = computed(() => demo.value.retryable ? `${prefix}.extraCost` : `${prefix}.scenarios.${scenario.value}.cost`)
const limitKey = computed(() => demo.value.retryable ? `${prefix}.${demo.value.recovered ? 'retryLimit' : 'exhaustedLimit'}` : `${prefix}.scenarios.${scenario.value}.limit`)
const boundaries = ['manual', 'login', 'dedup', 'concurrency', 'originalTask'] as const
const toneStyles = {
  neutral: 'bg-gray-100 dark:bg-dark-800',
  waiting: 'bg-amber-50 dark:bg-amber-900/20',
  retry: 'bg-blue-50 dark:bg-blue-900/20',
  ready: 'bg-emerald-50 dark:bg-emerald-900/20',
}
</script>

<style scoped>
.recovery-flow { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 1.5rem; list-style: none; padding: 0; }
.recovery-step { position: relative; display: flex; min-width: 0; flex-direction: column; justify-content: center; gap: 0.375rem; padding: 1rem 0.75rem; border-radius: 0.75rem; overflow-wrap: anywhere; }
.recovery-arrow { position: absolute; right: -1.125rem; top: 50%; transform: translateY(-50%); }
@media (max-width: 639px) {
  .recovery-flow { grid-template-columns: 1fr; }
  .recovery-step { padding: 0.75rem; }
  .recovery-arrow { right: 50%; top: auto; bottom: -1.375rem; transform: translateX(50%) rotate(90deg); }
}
</style>
