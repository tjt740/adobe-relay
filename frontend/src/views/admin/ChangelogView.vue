<template>
  <AppLayout>
    <div class="changelog mx-auto max-w-6xl space-y-7">
      <header class="release-hero">
        <p class="text-xs font-medium text-primary-600 dark:text-primary-300">{{ t('changelog.daily.timezone') }}</p>
        <h2 class="mt-3 text-2xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-3xl">{{ t('changelog.daily.title') }}</h2>
        <p class="mt-3 text-sm leading-7 text-gray-600 dark:text-gray-300">{{ t('changelog.daily.intro') }}</p>
      </header>
      <DailyChangelog />

      <details class="release-archive space-y-7">
        <summary class="flex cursor-pointer items-center justify-between gap-3 rounded-xl border border-gray-200 bg-white p-5 text-sm font-medium text-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200">
          <span>{{ t('changelog.daily.archiveTitle') }}</span>
          <Icon name="chevronDown" size="sm" class="archive-chevron shrink-0 transition-transform motion-reduce:transition-none" aria-hidden="true" />
        </summary>
      <header class="release-hero">
        <div class="flex items-center gap-2 text-xs font-medium text-primary-600 dark:text-primary-300">
          <Icon name="clock" size="sm" aria-hidden="true" />
          <span>{{ t('changelog.edition') }}</span>
        </div>
        <h2 class="mt-4 text-balance text-2xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-3xl">{{ t('changelog.headline') }}</h2>
        <p class="mt-3 max-w-3xl text-sm leading-7 text-gray-600 dark:text-gray-300">{{ t('changelog.intro') }}</p>
        <p class="mt-4 text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t('changelog.archive') }}</p>
      </header>

      <nav class="flex flex-wrap gap-2" :aria-label="t('nav.changelog')">
        <a v-for="section in sections" :key="section.id" :href="`#${section.id}`" class="section-link" @click.prevent="scrollToSection(section.id)">{{ t(section.label) }}</a>
      </nav>

      <section id="release-overview" class="scroll-mt-24" :aria-label="t('changelog.overview')">
        <div class="grid gap-4 md:grid-cols-3">
          <article v-for="entry in entries" :key="entry.id" class="release-card flex flex-col" :data-release-status="entry.id">
            <div class="flex items-center justify-between gap-2">
              <span class="release-badge" :class="entry.badgeClass">{{ t(entry.status) }}</span>
              <Icon :name="entry.icon" size="md" class="text-gray-400" aria-hidden="true" />
            </div>
            <h3 class="mt-4 font-semibold text-gray-900 dark:text-white">{{ t(entry.title) }}</h3>
            <p class="mt-2 flex-1 text-sm leading-7 text-gray-600 dark:text-gray-300">{{ t(entry.summary) }}</p>
            <p class="mt-4 border-t border-gray-100 pt-3 text-xs leading-6 text-gray-500 dark:border-dark-700 dark:text-gray-400">{{ t(entry.time) }}</p>
          </article>
        </div>
        <article class="release-card mt-4">
          <h3 class="section-heading">{{ t('changelog.resourceHeading') }}</h3>
          <p class="section-hint">{{ t('changelog.resourceHint') }}</p>
          <dl class="mt-6 grid grid-cols-2 gap-5 sm:grid-cols-4">
            <div v-for="metric in resources" :key="metric.label">
              <dt class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t(metric.label) }}</dt>
              <dd class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ metric.value }} <span class="text-xs font-normal text-gray-500">{{ metric.unit }}</span></dd>
            </div>
          </dl>
          <div class="mt-5 rounded-xl bg-gray-100 p-2 dark:bg-dark-700" role="img" :aria-label="t('changelog.memoryDiagram')">
            <div class="w-[77.78%] rounded-lg bg-primary-200 p-1.5 dark:bg-primary-900/70">
              <div class="w-[90%] rounded-md bg-primary-500 px-3 py-2 text-xs font-medium text-white">37.8 GiB</div>
            </div>
          </div>
          <p class="mt-4 text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t('changelog.resourceFootnote') }}</p>
          <div class="mt-5 flex gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-900/60">
            <Icon name="server" size="md" class="mt-0.5 shrink-0 text-primary-500" aria-hidden="true" />
            <div><h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('changelog.ingress') }}</h4><p class="mt-1 text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t('changelog.ingressHint') }}</p></div>
          </div>
        </article>
      </section>

      <section id="release-roadmap" class="scroll-mt-24 space-y-4" aria-labelledby="roadmap-heading">
        <div class="flex flex-wrap items-center gap-3">
          <h3 id="roadmap-heading" class="section-heading">{{ t('changelog.routingHeading') }}</h3>
          <span class="release-badge bg-amber-50 text-amber-800 dark:bg-amber-900/30 dark:text-amber-200">{{ t('changelog.pending') }}</span>
        </div>
        <p class="section-hint">{{ t('changelog.routingHint') }}</p>
        <div class="release-card">
          <ol class="grid gap-5 sm:grid-cols-2 xl:grid-cols-5" :aria-label="t('changelog.roadmap')">
            <li v-for="step in 5" :key="step" class="relative min-w-0">
              <div class="flex items-center gap-3">
                <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary-50 text-sm font-semibold text-primary-600 dark:bg-primary-900/40 dark:text-primary-300">{{ String(step).padStart(2, '0') }}</span>
                <div v-if="step < 5" class="h-px flex-1 bg-gray-200 dark:bg-dark-600" aria-hidden="true"></div>
                <Icon v-if="step < 5" name="arrowRight" size="sm" class="shrink-0 text-gray-300 dark:text-gray-500" aria-hidden="true" />
              </div>
              <h4 class="mt-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t(`changelog.step${step}Title`) }}</h4>
              <p class="mt-2 text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t(`changelog.step${step}Body`) }}</p>
            </li>
          </ol>
          <div class="mt-6 border-t border-dashed border-gray-200 pt-5 dark:border-dark-600">
            <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('changelog.fallbackTitle') }}</h4>
            <div class="mt-3 grid gap-3 sm:grid-cols-2">
              <div class="rounded-xl bg-amber-50/70 p-4 dark:bg-amber-900/15"><p class="text-sm font-semibold text-amber-800 dark:text-amber-200">429 · {{ t('changelog.busy') }}</p><p class="mt-2 text-xs leading-6 text-gray-600 dark:text-gray-300">{{ t('changelog.busyBody') }}</p></div>
              <div class="rounded-xl bg-rose-50/70 p-4 dark:bg-rose-900/15"><p class="text-sm font-semibold text-rose-800 dark:text-rose-200">503 · {{ t('changelog.unavailable') }}</p><p class="mt-2 text-xs leading-6 text-gray-600 dark:text-gray-300">{{ t('changelog.unavailableBody') }}</p></div>
            </div>
            <p class="mt-4 text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t('changelog.fallbackBound') }}</p>
          </div>
        </div>
        <h4 class="pt-2 text-sm font-semibold text-gray-900 dark:text-white">{{ t('changelog.scenarios') }}</h4>
        <div class="grid gap-3 md:grid-cols-2">
          <details v-for="scene in 7" :key="scene" class="scenario-card">
            <summary class="flex cursor-pointer items-center gap-3 p-4 text-sm font-medium text-gray-900 dark:text-white">
              <span class="text-xs tabular-nums text-gray-400">{{ String(scene).padStart(2, '0') }}</span>
              <span class="min-w-0 flex-1">{{ t(`changelog.scene${scene}Title`) }}</span>
              <Icon name="chevronDown" size="sm" class="scenario-chevron shrink-0 text-gray-400" aria-hidden="true" />
            </summary>
            <dl class="space-y-3 px-4 pb-4 text-xs leading-6">
              <div><dt class="text-gray-400">{{ t('changelog.before') }}</dt><dd class="mt-1 text-gray-500 dark:text-gray-400">{{ t(`changelog.scene${scene}Before`) }}</dd></div>
              <div><dt class="font-medium text-primary-600 dark:text-primary-300">{{ t('changelog.after') }}</dt><dd class="mt-1 text-gray-700 dark:text-gray-300">{{ t(`changelog.scene${scene}After`) }}</dd></div>
            </dl>
          </details>
        </div>
        <aside class="rounded-xl border border-gray-200 p-4 text-xs leading-6 dark:border-dark-700"><h4 class="font-medium text-gray-800 dark:text-gray-200">{{ t('changelog.limitsTitle') }}</h4><p class="mt-1 text-gray-500 dark:text-gray-400">{{ t('changelog.limitsBody') }}</p></aside>
      </section>

      <section id="release-stress" class="scroll-mt-24 space-y-4" aria-labelledby="stress-heading">
        <div><h3 id="stress-heading" class="section-heading">{{ t('changelog.stressHeading') }}</h3><p class="section-hint">{{ t('changelog.stressHint') }}</p></div>
        <dl class="grid grid-cols-3 gap-3">
          <div v-for="metric in testMetrics" :key="metric.label" class="release-card !p-4"><dt class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t(metric.label) }}</dt><dd class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ metric.value }}</dd></div>
        </dl>
        <article class="release-card">
          <h4 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('changelog.comparison') }}</h4>
          <p class="section-hint">{{ t('changelog.comparisonHint') }}</p>
          <div class="mt-6 space-y-5">
            <div v-for="(row, index) in comparison" :key="row.profile" :data-test="`comparison-${index}`">
              <div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-sm">
                <p class="font-medium text-gray-800 dark:text-gray-200">{{ row.hard_limit_GiB }} / {{ row.go_target_GiB }} GiB <span class="ml-2 text-xs" :class="index === 1 ? 'text-emerald-600 dark:text-emerald-300' : 'text-gray-400'">{{ t(index === 1 ? 'changelog.selected' : index === 0 ? 'changelog.previous' : 'changelog.candidate') }}</span></p>
                <p class="text-xs text-gray-500 dark:text-gray-400"><strong class="text-base font-semibold tabular-nums text-gray-900 dark:text-white">{{ row.accepted }} / {{ row.requested }}</strong> · {{ t('changelog.completed') }}</p>
              </div>
              <div class="mt-2 flex h-2.5 overflow-hidden rounded-full bg-amber-200 dark:bg-amber-900" aria-hidden="true"><div :style="{ width: `${row.accepted / row.requested * 100}%` }" class="rounded-full" :class="index === 1 ? 'bg-emerald-500' : 'bg-primary-400 dark:bg-primary-500'"></div></div>
              <p class="mt-2 text-xs tabular-nums text-gray-500 dark:text-gray-400">{{ t('changelog.rejected') }} {{ row.rejected }} · {{ t('changelog.eventMax') }} {{ row.memory_events.max }}</p>
            </div>
          </div>
          <p class="mt-5 text-xs leading-6 text-gray-600 dark:text-gray-300">{{ t('changelog.selectedReason') }}</p>
          <p class="mt-4 rounded-xl bg-amber-50 p-4 text-xs leading-6 text-amber-900 dark:bg-amber-900/20 dark:text-amber-200">{{ t('changelog.boundary') }}</p>
          <h4 class="mt-6 text-sm font-semibold text-gray-900 dark:text-white">{{ t('changelog.extraChecks') }}</h4>
          <dl class="mt-3 grid gap-3 sm:grid-cols-2"><div v-for="(row, index) in additionalChecks" :key="row.case" class="rounded-xl border border-gray-100 p-4 dark:border-dark-700"><dt class="text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t(index === 0 ? 'changelog.mixedCheck' : 'changelog.fullCheck') }}</dt><dd class="mt-2 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ row.accepted }} / {{ row.requested }}</dd></div></dl>
          <details class="mt-5 rounded-xl border border-gray-200 dark:border-dark-700" data-test="all-results">
            <summary class="cursor-pointer p-4 text-sm font-medium text-primary-700 dark:text-primary-300">{{ t('changelog.allResults') }}</summary>
            <div class="overflow-x-auto px-4 pb-4" role="region" tabindex="0" :aria-label="t('changelog.allResults')">
              <table class="w-full min-w-[720px] text-left text-xs tabular-nums">
                <caption class="sr-only">{{ t('changelog.stressHint') }}</caption>
                <thead class="border-b border-gray-200 text-gray-500 dark:border-dark-600 dark:text-gray-400"><tr><th v-for="key in resultColumns" :key="key" scope="col" class="px-2 py-3 font-medium">{{ t(key) }}</th></tr></thead>
                <tbody class="divide-y divide-gray-100 text-gray-700 dark:divide-dark-700 dark:text-gray-300"><tr v-for="row in replayResults" :key="`${row.case}-${row.profile}`"><td class="px-2 py-3">{{ row.cpu_limit }}</td><th scope="row" class="px-2 py-3 font-normal">{{ scenarioLabel(row.case) }} · {{ row.case.includes('16m') ? 16 : 8 }} MiB</th><td class="px-2 py-3">{{ row.hard_limit_GiB }} / {{ row.go_target_GiB }} / {{ row.gomaxprocs }}</td><td class="whitespace-nowrap px-2 py-3">{{ row.accepted }} / {{ row.requested }}</td><td class="px-2 py-3">{{ row.rejected }}</td><td class="px-2 py-3">{{ row.memory_events.max }}</td><td class="px-2 py-3">{{ row.elapsed_seconds.toFixed(2) }}</td></tr></tbody>
              </table>
            </div>
          </details>
          <div class="mt-5"><h4 class="text-xs font-medium text-gray-800 dark:text-gray-200">{{ t('changelog.method') }}</h4><p class="mt-1 text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t('changelog.methodBody') }}</p></div>
        </article>
        <aside class="rounded-xl border border-amber-200 bg-amber-50/50 p-5 dark:border-amber-900/50 dark:bg-amber-900/10"><h4 class="text-sm font-semibold text-amber-900 dark:text-amber-200">{{ t('changelog.onlineTitle') }}</h4><p class="mt-2 text-xs leading-6 text-amber-900/80 dark:text-amber-100/80">{{ t('changelog.onlineBody') }}</p></aside>
      </section>

      <section id="release-documents" class="scroll-mt-24 space-y-4" aria-labelledby="documents-heading">
        <div><h3 id="documents-heading" class="section-heading">{{ t('changelog.docsHeading') }}</h3><p class="section-hint">{{ t('changelog.docsHint') }}</p></div>
        <div class="grid gap-3 md:grid-cols-3">
          <article v-for="doc in documents" :key="doc.id" class="release-card flex flex-col !p-5">
            <Icon name="document" size="md" class="text-primary-500" aria-hidden="true" />
            <h4 class="mt-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t(doc.title) }}</h4>
            <p class="mt-2 flex-1 text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t(doc.hint) }}</p>
            <div class="mt-4 flex flex-wrap items-center gap-4 text-xs font-medium text-primary-700 dark:text-primary-300">
              <button type="button" :aria-label="`${t('changelog.read')} · ${t(doc.title)}`" :aria-expanded="selectedDocument?.id === doc.id" aria-controls="release-document-reader" class="rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500" @click="readDocument(doc.id)">{{ t('changelog.read') }}</button>
              <a :href="doc.downloadUrl" :download="doc.filename" :aria-label="`${t('changelog.download')} · ${t(doc.title)}`" class="inline-flex items-center gap-1 rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500"><Icon name="download" size="xs" aria-hidden="true" />{{ t('changelog.download') }}</a>
            </div>
          </article>
        </div>
        <article v-if="selectedDocument" id="release-document-reader" ref="documentReader" class="release-card scroll-mt-24" tabindex="-1" :aria-label="`${t('changelog.docReading')} · ${t(selectedDocument.title)}`">
          <div class="mb-5 flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 pb-4 dark:border-dark-700"><span class="text-xs text-gray-500 dark:text-gray-400">{{ t('changelog.original') }} · {{ selectedDocument.filename }}</span><button type="button" class="btn btn-secondary btn-sm" @click="selectedDocumentId = null">{{ t('changelog.closeDoc') }}</button></div>
          <div class="markdown-body release-document" lang="zh" v-html="documentHtml"></div>
        </article>
      </section>
      </details>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import DailyChangelog from '@/components/admin/changelog/DailyChangelog.vue'
import replayResults from '@/content/changelog/2026-10-17/replay-results.json'
import overview from '@/content/changelog/2026-10-17/night-overview.md?raw'
import overviewUrl from '@/content/changelog/2026-10-17/night-overview.md?url'
import roadmap from '@/content/changelog/2026-10-17/proxy-roadmap.md?raw'
import roadmapUrl from '@/content/changelog/2026-10-17/proxy-roadmap.md?url'
import stressReport from '@/content/changelog/2026-10-17/stress-report.md?raw'
import stressReportUrl from '@/content/changelog/2026-10-17/stress-report.md?url'
import '@/styles/announcement-markdown.css'

const { t } = useI18n()
const sections = [
  { id: 'release-overview', label: 'changelog.overview' },
  { id: 'release-roadmap', label: 'changelog.roadmap' },
  { id: 'release-stress', label: 'changelog.stress' },
  { id: 'release-documents', label: 'changelog.documents' },
]
const entries = [
  { id: 'live', status: 'changelog.live', title: 'changelog.resourceTitle', summary: 'changelog.resourceSummary', time: 'changelog.resourceTime', icon: 'server', badgeClass: 'bg-emerald-50 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200' },
  { id: 'pending', status: 'changelog.pending', title: 'changelog.proxyTitle', summary: 'changelog.proxySummary', time: 'changelog.proxyTime', icon: 'sync', badgeClass: 'bg-amber-50 text-amber-800 dark:bg-amber-900/30 dark:text-amber-200' },
  { id: 'verified', status: 'changelog.verified', title: 'changelog.testTitle', summary: 'changelog.testSummary', time: 'changelog.testTime', icon: 'chart', badgeClass: 'bg-blue-50 text-blue-800 dark:bg-blue-900/30 dark:text-blue-200' },
] as const
const resources = [
  { label: 'changelog.container', value: '54', unit: 'GiB' },
  { label: 'changelog.go', value: '42', unit: 'GiB' },
  { label: 'changelog.image', value: '37.8', unit: 'GiB' },
  { label: 'changelog.parallel', value: '16', unit: '' },
]
const testMetrics = [
  { label: 'changelog.groups', value: replayResults.length },
  { label: 'changelog.oom', value: `${replayResults.reduce((sum, row) => sum + row.memory_events.oom, 0)} / ${replayResults.reduce((sum, row) => sum + row.memory_events.oom_kill, 0)}` },
  { label: 'changelog.leftovers', value: replayResults.reduce((sum, row) => sum + row.temporary_files, 0) },
]
const comparison = replayResults.filter(row => row.case === 'json-1600-16m-cpu16')
const additionalChecks = replayResults.filter(row => row.profile === 'selected54-go42')
const resultColumns = ['changelog.cores', 'changelog.case', 'changelog.config', 'changelog.completed', 'changelog.rejected', 'changelog.eventMax', 'changelog.elapsed']
function scenarioLabel(name: string) {
  return t(name.startsWith('full') ? 'changelog.fullCase' : name.startsWith('mixed') ? 'changelog.mixedCase' : 'changelog.jsonCase')
}
const documents = [
  { id: 'overview', title: 'changelog.summaryDoc', hint: 'changelog.summaryDocHint', content: overview, downloadUrl: overviewUrl, filename: '2026-10-17-更新总览.md' },
  { id: 'roadmap', title: 'changelog.roadmapDoc', hint: 'changelog.roadmapDocHint', content: roadmap, downloadUrl: roadmapUrl, filename: '2026-10-17-代理与账号路线图.md' },
  { id: 'stress', title: 'changelog.stressDoc', hint: 'changelog.stressDocHint', content: stressReport, downloadUrl: stressReportUrl, filename: '2026-10-17-压测报告.md' },
]
const selectedDocumentId = ref<string | null>(null)
const selectedDocument = computed(() => documents.find(doc => doc.id === selectedDocumentId.value))
const documentHtml = computed(() => DOMPurify.sanitize(marked.parse(selectedDocument.value?.content || '', { async: false, gfm: true })))
const documentReader = ref<HTMLElement | null>(null)
function scrollToSection(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
async function readDocument(id: string) {
  selectedDocumentId.value = id
  await nextTick()
  documentReader.value?.focus({ preventScroll: true })
  documentReader.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<style scoped>
.release-hero { @apply rounded-2xl border border-primary-100 bg-gradient-to-br from-primary-50 via-white to-white p-6 dark:border-primary-900/50 dark:from-primary-900/20 dark:via-dark-900 dark:to-dark-900 sm:p-8; }
.release-card { @apply min-w-0 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-6; }
.release-badge { @apply inline-flex rounded-full px-2.5 py-1 text-[11px] font-medium leading-5; }
.section-heading { @apply text-lg font-semibold tracking-tight text-gray-900 dark:text-white; }
.section-hint { @apply mt-2 text-xs leading-6 text-gray-500 dark:text-gray-400; }
.section-link { @apply rounded-full border border-gray-200 bg-white px-4 py-2 text-xs font-medium text-gray-600 transition-colors hover:border-primary-300 hover:text-primary-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:hover:text-primary-300; }
.scenario-card { @apply self-start rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900; }
summary { list-style: none; }
summary::-webkit-details-marker { display: none; }
.scenario-card[open] .scenario-chevron { transform: rotate(180deg); }
.release-archive[open] > summary .archive-chevron { transform: rotate(180deg); }
.release-document { overflow-wrap: anywhere; }
.release-document :deep(table) { display: block; overflow-x: auto; }
.release-document :deep(h1) { @apply mt-0 text-2xl; }
.release-document :deep(h2) { @apply text-xl; }
.release-document :deep(h3) { @apply text-lg; }
</style>
