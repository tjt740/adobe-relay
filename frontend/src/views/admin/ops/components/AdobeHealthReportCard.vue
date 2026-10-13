<script setup lang="ts">
import { computed, ref } from 'vue'
import apiClient from '@/api/client'
import { useAccountLiveUpdates } from '@/composables/useAccountLiveUpdates'
import { healthReason, healthSource, healthStage, type AdobeHealthReport } from '@/utils/adobeHealth'
const props = defineProps<{ snapshot?: AdobeHealthReport }>()
const report = ref<AdobeHealthReport | null>(props.snapshot || null)
const error = ref('')
const sending = ref(false)
const message = ref('')
const filter = ref('pending')
const search = ref('')
async function load() {
  if (props.snapshot) return
  try { report.value = (await apiClient.get<AdobeHealthReport>('/admin/ops/adobe-health')).data; error.value = '' }
  catch { error.value = '当前报表暂未更新，以下保留上次数据。'; throw new Error('Health report unavailable') }
}
const { connected } = useAccountLiveUpdates(async () => { await load() }, !props.snapshot)
const rows = computed(() => (report.value?.accounts || []).filter(a =>
  (filter.value === 'all' || (filter.value === 'pending' ? a.pending : !a.pending && a.failed_at && a.success_at)) &&
  `${a.id} ${a.name} ${a.scope}`.toLowerCase().includes(search.value.trim().toLowerCase())))
const pendingReasons = computed(() => {
  const counts = new Map<string, Set<number>>()
  for (const a of report.value?.accounts || []) { if (a.pending) { const ids = counts.get(a.reason) || new Set<number>(); ids.add(a.id); counts.set(a.reason, ids) } }
  return [...counts].map(([reason, ids]) => ({ reason, count: ids.size, label: healthReason(reason)[0], advice: healthReason(reason)[1] }))
})
function time(value?: string) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '尚无记录' }
async function send() {
  sending.value = true; message.value = ''
  try { const { data } = await apiClient.post('/admin/ops/adobe-health/email'); message.value = data.message }
  catch { message.value = '邮件尚未确认发送成功，请稍后重试。' }
  finally { sending.value = false }
}
</script>
<template>
  <section class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800" data-testid="adobe-health-report">
    <div class="flex flex-wrap items-start justify-between gap-4 border-b border-slate-100 px-5 py-5 dark:border-dark-700">
      <div><div class="mb-1 text-[11px] font-semibold tracking-widest text-teal-600">ADOBE · 账号运行报告</div><h2 class="text-lg font-semibold text-slate-900 dark:text-white">哪些账号需要关注</h2><p class="mt-1 text-xs text-slate-500">{{ snapshot ? '告警记录快照' : connected ? '正在同步最新记录' : '连接恢复中 · 保留上次数据' }}<span v-if="report"> · {{ time(report.generated_at) }}</span></p></div>
      <button v-if="!snapshot" class="btn btn-secondary btn-sm" :disabled="sending || !report" @click="send">{{ sending ? '正在发送…' : '发送报表到告警邮箱' }}</button>
    </div>
    <div class="space-y-5 p-5">
      <p v-if="error" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800" role="status">{{ error }}</p>
      <p v-if="message" class="text-sm text-teal-700" role="status">{{ message }}</p>
      <template v-if="report">
        <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <div class="rounded-xl bg-amber-50 p-4 dark:bg-amber-950/30"><div class="text-xs text-amber-800 dark:text-amber-300">仍待成功验证</div><div class="mt-2 text-3xl font-semibold text-amber-800 dark:text-amber-200">{{ report.pending }}<span class="ml-1 text-sm font-normal">个账号</span></div><p class="mt-2 text-xs text-amber-700 dark:text-amber-300">401 / 408 有失败，尚未确认恢复</p></div>
          <div class="rounded-xl bg-emerald-50 p-4 dark:bg-emerald-950/30"><div class="text-xs text-emerald-800 dark:text-emerald-300">已有恢复证据</div><div class="mt-2 text-3xl font-semibold text-emerald-800 dark:text-emerald-200">{{ report.recovered }}<span class="ml-1 text-sm font-normal">个账号</span></div><p class="mt-2 text-xs text-emerald-700 dark:text-emerald-300">失败之后，取得实际成功记录</p></div>
          <div class="rounded-xl bg-slate-50 p-4 dark:bg-dark-900"><div class="text-xs text-slate-500">尚无完整记录</div><div class="mt-2 text-3xl font-semibold">{{ report.unknown }}<span class="ml-1 text-sm font-normal">个账号</span></div><p class="mt-2 text-xs text-slate-500">没有请求，不代表账号正常</p></div>
          <div class="rounded-xl bg-slate-50 p-4 dark:bg-dark-900"><div class="text-xs text-slate-500">待验证比例 / 告警阈值</div><div class="mt-2 text-3xl font-semibold">{{ report.ratio.toFixed(1) }}<span class="text-lg">%</span></div><p class="mt-2 text-xs text-slate-500">{{ report.pending }} / {{ report.total }} · 阈值 {{ report.threshold }}%</p></div>
        </div>
        <p class="text-sm leading-6 text-slate-600 dark:text-slate-300">最近统计窗口有 <strong>{{ report.historical }}</strong> 个账号报过错，其中 <strong>{{ report.recent_pending }}</strong> 个仍待成功验证。触发仍需达到 {{ report.threshold }}% 且至少 {{ report.minimum }} 个账号；分母包含全部未删除 Adobe 账号（包括暂停账号）。历史报错与当前待验证数量不相加。</p>
        <div v-if="pendingReasons.length" class="grid gap-3 sm:grid-cols-2">
          <div v-for="item in pendingReasons" :key="item.reason" class="rounded-lg border border-slate-200 p-3 dark:border-dark-600"><div class="flex justify-between text-sm font-medium"><span>{{ item.label }}</span><span class="text-amber-700">{{ item.count }} 个</span></div><p class="mt-1 text-xs leading-5 text-slate-500">{{ item.advice }}</p></div>
        </div>
        <p class="text-xs text-slate-500">下方同时列出其他失败原因；只有 401 / 408 计入 20% 告警阈值。同一账号可能涉及多个模型，分类数量不相加。</p>
        <div class="flex flex-wrap justify-between gap-3"><div class="flex gap-1 rounded-lg bg-slate-100 p-1 dark:bg-dark-900"><button v-for="option in [{value:'pending',label:'仍待验证'},{value:'recovered',label:'已有恢复证据'},{value:'all',label:'全部记录'}]" :key="option.value" class="rounded-md px-3 py-1.5 text-xs" :class="filter === option.value ? 'bg-white text-teal-700 shadow-sm dark:bg-dark-700' : 'text-slate-500'" @click="filter = option.value">{{ option.label }}</button></div><input v-model="search" class="input w-full sm:w-60" placeholder="搜索账号、ID 或模型" aria-label="搜索健康报告账号" /></div>
        <div class="max-h-[480px] overflow-auto rounded-xl border border-slate-200 dark:border-dark-700"><table class="w-full text-left text-xs"><thead class="sticky top-0 bg-slate-50 text-slate-500 dark:bg-dark-900"><tr><th class="p-3">账号 / 模型</th><th class="p-3">发生了什么</th><th class="p-3">记录来源</th><th class="p-3">失败与成功时间</th></tr></thead><tbody class="divide-y divide-slate-100 dark:divide-dark-700"><tr v-for="row in rows" :key="`${row.id}:${row.scope}`"><td class="max-w-[240px] break-words p-3"><span class="font-medium">#{{ row.id }} {{ row.name }}</span><div class="mt-1 text-slate-500">{{ row.scope || '历史记录（模型未记录）' }}</div></td><td class="p-3"><span :class="row.pending ? 'text-amber-700' : 'text-emerald-700'">{{ row.pending ? healthReason(row.reason)[0] : row.failed_at ? '已有后续成功记录' : '本次请求成功' }}</span><div v-if="row.code" class="mt-1 text-slate-400">{{ row.reason === 'timeout' ? '本地超时（按 408 统计）' : `状态码 ${row.code}` }}</div></td><td class="p-3">{{ healthSource(row.source) }}<div v-if="row.stage" class="mt-1 text-slate-400">{{ healthStage(row.stage) }}</div></td><td class="whitespace-nowrap p-3"><div>失败 {{ time(row.failed_at) }}</div><div class="mt-1 text-slate-500">成功 {{ time(row.success_at) }}</div></td></tr><tr v-if="!rows.length"><td colspan="4" class="p-8 text-center text-slate-500">没有符合条件的记录</td></tr></tbody></table></div>
        <p class="rounded-lg bg-teal-50 p-3 text-xs leading-6 text-teal-900 dark:bg-teal-950/30 dark:text-teal-200">恢复依据：失败之后开始的真实业务请求或检测成功。Cookie 更新、重置状态和长时间没有新请求都不算恢复。模型相关故障需对应模型成功；比例回到阈值以下且有实际恢复证据，告警才自动结束。</p>
      </template><p v-else class="py-6 text-center text-sm text-slate-500">等待账号健康记录…</p>
    </div>
  </section>
</template>
