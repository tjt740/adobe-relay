<script setup lang="ts">
import { computed, ref } from 'vue'
import apiClient from '@/api/client'
import { useAccountLiveUpdates } from '@/composables/useAccountLiveUpdates'
import { healthReason, healthSource, healthStage, type AdobeHealthReport } from '@/utils/adobeHealth'
const props = defineProps<{ snapshot?: AdobeHealthReport }>()
const report = ref<AdobeHealthReport | null>(props.snapshot || null)
const error = ref('')
const filter = ref('pending')
const search = ref('')
async function load() {
  if (props.snapshot) return
  try { report.value = (await apiClient.get<AdobeHealthReport>('/admin/ops/adobe-health')).data; error.value = '' }
  catch { error.value = '报表暂未更新，当前保留上次数据。'; throw new Error('Health report unavailable') }
}
const { connected } = useAccountLiveUpdates(async () => { await load() }, !props.snapshot)
const rows = computed(() => (report.value?.accounts || []).filter(a =>
  (filter.value === 'all' || (filter.value === 'pending' ? a.pending : !a.pending && a.failed_at && a.success_at)) &&
  `${a.id} ${a.name} ${a.scope}`.toLowerCase().includes(search.value.trim().toLowerCase())))
const windowRows = computed(() => (report.value?.window_accounts || []).filter(a => `${a.account_id} ${a.name}`.toLowerCase().includes(search.value.trim().toLowerCase())))
const stats = computed(() => {
  const all = report.value?.window_accounts || []
  return { authAccounts: all.filter(a => a.count_401 > 0).length, timeoutAccounts: all.filter(a => a.count_408 > 0).length,
    auth: all.reduce((n, a) => n + a.auth_401, 0), quota: all.reduce((n, a) => n + a.quota_401, 0), entitled: all.reduce((n, a) => n + a.entitled_401, 0), unknown: all.reduce((n, a) => n + a.unknown_401, 0),
    errors401: all.reduce((n, a) => n + a.count_401, 0), errors408: all.reduce((n, a) => n + a.count_408, 0),
    business: all.reduce((n, a) => n + a.business_errors, 0), tests: all.reduce((n, a) => n + a.test_errors, 0), timeouts: all.reduce((n, a) => n + a.test_timeouts, 0) }
})
const hasWindowCounts = computed(() => report.value?.window_accounts != null)
const triggerRatio = computed(() => report.value?.total ? report.value.recent_pending * 100 / report.value.total : 0)
function time(value?: string) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false, timeZone: 'Asia/Shanghai' }) : '未记录' }
</script>
<template>
  <section class="adobe-report" data-testid="adobe-health-report">
    <header class="report-header">
      <div class="eyebrow">ADOBE · 账号运行监控</div><h2>账号异常报表</h2>
      <p v-if="report">统计时段 {{ time(report.window_start) }} — {{ time(report.generated_at) }} · 北京时间</p>
      <span v-if="!snapshot" class="connection">{{ connected ? '正在同步最新记录' : '连接恢复中 · 保留上次数据' }}</span>
    </header>
    <div class="report-body">
      <p v-if="error" class="notice" role="status">{{ error }}</p>
      <template v-if="report">
        <div class="metrics">
          <div class="metric"><div>近期异常待验证</div><strong>{{ report.recent_pending }}<small>个账号</small></strong><p>统计窗口内有失败，尚未确认恢复</p></div>
          <div class="metric"><div>本次触发占比</div><strong>{{ triggerRatio.toFixed(2) }}<small>%</small></strong><p>{{ report.recent_pending }} / {{ report.total }} · 阈值 {{ report.threshold }}%</p></div>
          <div class="metric red"><div><b>401</b> 认证或权限异常</div><strong>{{ hasWindowCounts ? stats.authAccounts : '\u2014' }}<small>个账号</small></strong><p v-if="hasWindowCounts">窗口内共 {{ stats.errors401 }} 条错误</p></div>
          <div class="metric yellow"><div><b>408</b> 请求超时</div><strong>{{ hasWindowCounts ? stats.timeoutAccounts : '\u2014' }}<small>个账号</small></strong><p v-if="hasWindowCounts">窗口内共 {{ stats.errors408 }} 条错误</p></div>
        </div>
        <div class="overview"><div class="bar-label"><b>告警触发情况</b><span>阈值 {{ report.threshold }}% · 至少 {{ report.minimum }} 个账号</span></div><div class="bar"><div :style="{ width: `${Math.min(100, triggerRatio)}%` }"></div><i :style="{ left: `${Math.min(100, report.threshold)}%` }"></i></div><p>全部仍待成功验证 <b>{{ report.pending }}</b> 个（{{ report.ratio.toFixed(2) }}%） · 已有恢复证据 <b>{{ report.recovered }}</b> 个 · 尚无完整记录 <b>{{ report.unknown }}</b> 个</p></div>
        <h3>异常分类与处理建议</h3>
        <div v-if="hasWindowCounts" class="causes">
          <div><h4 class="red-text"><span class="badge red">401</span> 认证或权限异常</h4><p>鉴权 {{ stats.auth }} 条 · 额度不足 {{ stats.quota }} 条 · 权益不足 {{ stats.entitled }} 条 · 原因待确认 {{ stats.unknown }} 条。</p><p>按具体原因检查 Cookie、额度或模型权限，避免反复登录。</p></div>
          <div><h4 class="yellow-text"><span class="badge yellow">408</span> 请求超时</h4><p>超时 {{ stats.errors408 }} 条，其中检测超时 {{ stats.timeouts }} 条。</p><p>优先检查上游服务、代理和网络。检测超时不代表 Adobe 明确返回 HTTP 408。</p></div>
        </div>
        <div class="notice">业务请求错误 {{ hasWindowCounts ? stats.business : '\u2014' }} 条 · 账号检测错误 {{ hasWindowCounts ? stats.tests : '\u2014' }} 条 · 窗口内报过错的账号 {{ report.historical }} 个。<br>同一账号可能同时出现两类错误，分类数量不相加。历史报错与当前待验证数量不相加；没有新请求不代表恢复。</div>
        <div class="section-heading"><h3>完整窗口明细</h3><input v-model="search" class="input search" placeholder="搜索账号、ID 或模型" aria-label="搜索健康报告账号" /></div>
        <div class="table-wrap"><table class="window-table"><thead><tr><th>账号 ID / 账号</th><th class="red">401</th><th class="yellow">408</th><th>错误记录分布</th><th>最后报错</th></tr></thead><tbody>
          <tr v-for="row in windowRows" :key="row.account_id"><td>#{{ row.account_id }} {{ row.name }}</td><td><span v-if="row.count_401" class="badge red">{{ row.count_401 }} 次</span><span v-else>—</span></td><td><span v-if="row.count_408" class="badge yellow">{{ row.count_408 }} 次</span><span v-else>—</span></td><td>业务 {{ row.business_errors }} / 检测 {{ row.test_errors }}<small>检测超时 {{ row.test_timeouts }}</small></td><td>{{ time(row.last_seen) }}</td></tr>
          <tr v-if="!windowRows.length"><td colspan="5" class="empty">没有符合条件的窗口错误明细</td></tr>
        </tbody></table></div>
        <div class="section-heading"><h3>状态与成功证据</h3><div class="filters"><button v-for="option in [{value:'pending',label:'仍待验证'},{value:'recovered',label:'已有恢复证据'},{value:'all',label:'全部记录'}]" :key="option.value" :class="{ selected: filter === option.value }" @click="filter = option.value">{{ option.label }}</button></div></div>
        <div class="table-wrap"><table class="evidence-table"><thead><tr><th>账号 / 模型</th><th>失败原因 / 来源</th><th>验证状态</th><th>失败与成功时间</th></tr></thead><tbody>
          <tr v-for="row in rows" :key="`${row.id}:${row.scope}`"><td>#{{ row.id }} {{ row.name }}<small>{{ row.scope || '模型未记录' }}</small></td><td><template v-if="row.failed_at"><span v-if="row.code" class="badge" :class="row.code === 401 ? 'red' : row.code === 408 ? 'yellow' : ''">{{ row.code }}</span> {{ healthReason(row.reason)[0] }}</template><template v-else>成功记录</template><small>{{ healthSource(row.source) }} · {{ healthStage(row.stage) }}</small><small v-if="row.reason === 'timeout'">本地超时（按 408 统计）</small></td><td>{{ row.pending ? '仍待验证' : row.failed_at ? '已有后续成功记录' : '本次请求成功' }}<small v-if="row.alert_pending">401/408 仍待验证</small></td><td>失败 {{ time(row.failed_at) }}<small>成功 {{ time(row.success_at) }}{{ row.pending && row.success_at ? '（不足以确认本次恢复）' : '' }}</small></td></tr>
          <tr v-if="!rows.length"><td colspan="4" class="empty">没有符合条件的状态记录</td></tr>
        </tbody></table></div>
        <div class="notice"><b>通知策略</b><br>近期待验证比例达到 {{ report.threshold }}% 且至少 {{ report.minimum }} 个账号<template v-if="report.quiet_notifications">，持续确认至少 {{ report.confirmation_minutes }} 分钟后通知。同一轮异常成功通知一次，新邮件至少间隔 {{ report.notification_cooldown_minutes }} 分钟。</template><template v-else>时触发。</template><br><b>恢复判定</b><br>全部待验证比例严格低于 {{ report.recovery_threshold ?? report.threshold }}%，并有本轮账号的真实成功记录，告警才自动结束。模型相关故障需对应模型成功。Cookie 更新、重置状态或没有新请求都不能单独证明恢复。告警结束不代表所有账号均已恢复。</div>
      </template><p v-else class="empty">等待账号健康记录…</p>
    </div>
  </section>
</template>
<style scoped>
.adobe-report{overflow:hidden;border:1px solid #e1e8f0;border-radius:16px;background:#fff;color:#20324a}.report-header{padding:26px 28px;background:#14283f;color:#fff}.eyebrow{font-size:11px;letter-spacing:2px;color:#b8cbdc}.report-header h2{font-size:26px;font-weight:650;margin:12px 0}.report-header p,.connection{font-size:12px;color:#c1cfde;line-height:1.8}.connection{display:block;margin-top:8px}.report-body{padding:24px}.metrics{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}.metric{padding:20px;border:1px solid #e5ebf1;border-radius:12px;font-size:13px}.metric strong{display:block;font-size:34px;font-weight:700;margin:12px 0}.metric strong small{display:inline;font-size:12px;font-weight:400;margin-left:5px}.metric p{font-size:11px;color:#687a8f;line-height:1.7}.red{background:#ffe6e9!important;color:#c52334!important}.yellow{background:#fff0ac!important;color:#856000!important}.red-text{color:#c52334}.yellow-text{color:#856000}.overview,.notice{padding:16px 18px;background:#f4f7fa;border-radius:10px;margin-top:18px;font-size:12px;line-height:1.9;color:#61748b}.bar-label,.section-heading{display:flex;justify-content:space-between;gap:12px;align-items:center;flex-wrap:wrap}.bar{position:relative;height:10px;background:#e1e8f0;border-radius:5px;margin:16px 0}.bar>div{height:100%;background:#425e7b;border-radius:5px}.bar i{position:absolute;top:-4px;height:18px;width:2px;background:#927789}h3{font-size:17px;font-weight:650;margin:24px 0 14px}.causes{display:grid;grid-template-columns:1fr 1fr;gap:14px}.causes>div{padding:18px;border:1px solid #e5ebf1;border-radius:12px}.causes h4{font-size:14px;font-weight:600;margin-bottom:10px}.causes p{font-size:12px;color:#61748b;line-height:1.9}.badge{display:inline-block;padding:4px 8px;border-radius:5px;font-size:12px;font-weight:600;white-space:nowrap}.table-wrap{overflow-x:auto;border:1px solid #e5ebf1;border-radius:10px}table{width:100%;text-align:left;font-size:12px;border-collapse:collapse;min-width:660px}th{background:#f3f6fa;color:#64758b;font-weight:550}th,td{padding:13px 12px;border-bottom:1px solid #e5ebf1;vertical-align:top}tbody tr:last-child td{border-bottom:0}tbody tr:nth-child(even){background:#fbfcfe}td:first-child{max-width:270px;overflow-wrap:anywhere}td small{display:block;margin-top:5px;color:#8291a4;line-height:1.7}.empty{text-align:center;padding:24px;color:#8291a4}.filters{display:flex;gap:4px;background:#f1f5f9;padding:4px;border-radius:7px;margin:12px 0}.filters button{font-size:12px;padding:6px 10px;color:#64758b;border-radius:5px}.filters .selected{background:#fff;color:#245674;box-shadow:0 1px 3px #0001}.search{width:240px;font-size:12px}.section-heading h3{margin:24px 0 16px}
@media(max-width:1000px){.metrics{grid-template-columns:repeat(2,minmax(0,1fr))}.causes{grid-template-columns:1fr}.report-body{padding:16px}.report-header{padding:20px}}
</style>
