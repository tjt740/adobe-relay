import type { Account } from '@/types'

export interface StatusRecord {
  title: string
  result: string
  time?: string
  fields: { label: string; value: string }[]
  note?: string
}

type Data = Record<string, unknown>
const object = (value: unknown): Data => value && typeof value === 'object' && !Array.isArray(value) ? value as Data : {}
const text = (value: unknown) => typeof value === 'string' ? value : ''
const result = (value: unknown) => value === true ? '成功' : value === false ? '失败' : '结果未记录'
const code = (value: unknown) => Number.isInteger(Number(value)) && Number(value) >= 100 && Number(value) <= 599 ? String(value) : '未记录'
const label = (labels: Record<string, string>, value: unknown) => labels[text(value)] || '未记录或无法识别'
const field = (name: string, value: string) => ({ label: name, value })
const sources = { business: '真实业务请求', okad: 'Cookie 推送后的检测', test: '手动或批量检测', legacy: '旧版历史记录' }
const stages = { token: '使用 Cookie 换取 Token', identity: '核对账号身份', credits: '查询账号额度', prepare: '准备请求', upload: '上传素材', submit: '提交生成请求', poll: '等待生成结果', download: '下载生成结果' }
const reasons = {
  auth: '凭据验证失败', quota: '额度不足', entitlement: '所用模型的权限不足', timeout: '本地等待响应超时',
  upstream: '上游返回失败响应', proxy: '代理连接失败', network: '网络连接失败', content: '上游拒绝了请求内容',
  legacy: '旧记录没有保存具体原因', unknown: '记录中没有明确原因'
}
const checkStates = { pending: '已排队，尚未开始', running: '检测已开始，尚未记录结束结果', proxy_pending: '等待代理就绪', interrupted: '检测已中断', failed: '检测失败', success: '检测成功' }
const verificationStates = {
  pending: '尚未完成凭据验证', verified: '凭据与额度验证通过', verification_retry: '上次验证未完成，等待重试',
  proxy_pending: '等待代理就绪后验证', auth_failed: '凭据验证失败', quota_blocked: '额度未达到验证要求',
  entitlement_blocked: '模型权限未通过验证', identity_mismatch: 'Cookie 对应邮箱与关联账号不一致'
}

export function statusRecordTime(value: unknown): string {
  if (!text(value)) return '时间未记录'
  const date = new Date(text(value))
  return Number.isFinite(date.getTime()) ? date.toLocaleString('zh-CN', { hour12: false }) : '时间记录无效'
}

// Error fields can contain upstream text. Never display credential values in the details dialog.
export function safeStatusMessage(value: unknown): string {
  return text(value)
    .replace(/\b(?:Bearer\s+)[A-Za-z0-9._~+\/-]+=*/gi, 'Bearer [已隐藏]')
    .replace(/\beyJ[A-Za-z0-9_.-]{20,}/g, '[已隐藏 Token]')
    .replace(/((?:cookie|authorization|access_token|refresh_token|password)\s*["']?\s*[:=]\s*)[^\n]+/gi, '$1[已隐藏]')
    .slice(0, 4000)
}

export function statusErrorExplanation(value: unknown): string {
  const message = text(value).toLowerCase()
  if (!message) return ''
  if (message.includes('automatic login limit reached')) return '系统记录：自动登录次数已达到上限。此记录本身不能说明后来更新的 Cookie 是否有效。'
  if (message.includes('okad credentials pending connection test')) return '系统记录：等待出图检测通过。具体检测进度和结果见上方记录。'
  if (message.includes('okad credentials pending verification')) return '系统记录：等待凭据验证结果。'
  if (/taste_exhausted|quota exhausted|credits exhausted/.test(message)) return '系统记录：账号额度不足。'
  if (/model_not_entitled|user_not_entitled|not entitled/.test(message)) return '系统记录：所用模型的权限不足。'
  if (/invalid_client|invalid_scope/.test(message)) return '系统记录：换取 Token 的客户端或授权范围未通过校验。'
  if (/invalid_credentials|adobe cookie is no longer valid|cookie during token exchange/.test(message)) return '系统记录：使用 Cookie 换取 Token 时，凭据被上游拒绝。'
  const status = message.match(/(?:^|\(|\b(?:http|returned|status(?:_code|\s+code)?)["']?\s*[:=]?\s*)(401|408|429)\b/)?.[1]
  if (status === '401') return '系统保存了 401 错误；具体原因和发生阶段以对应请求记录为准。'
  if (status === '408') return '系统保存了 408 错误；仅凭状态码无法区分上游超时和本地等待超时。'
  if (status === '429') return '系统保存了请求限流错误。'
  return '系统保存了一条错误说明；没有结构化的原因分类，请查看原始记录。'
}

function observation(title: string, value: Data): StatusRecord {
  return {
    title, result: `请求${result(value.success)}`, time: text(value.checked_at),
    fields: [field('状态码', code(value.code)), field('来源', label(sources, value.source)),
      field('发生阶段', label(stages, value.stage)), field('模型', text(value.scope) || '未记录'),
      ...(!value.success ? [field('记录的原因', label(reasons, value.reason))] : [])]
  }
}

export function accountStatusRecords(account: Account): StatusRecord[] {
  const extra = object(account.extra)
  const health = object(extra.adobe_health)
  const latest = object(health.latest)
  const records: StatusRecord[] = []
  const proxy408 = object(extra.adobe_proxy_408)
  if (proxy408.managed === true) {
    records.push({ title: '408 账号代理安排', result: proxy408.state === 'testing' ? '临时代理检测中' : '待恢复，不占常规代理',
      fields: [field('待恢复模型', Array.isArray(proxy408.models) && proxy408.models.length ? proxy408.models.map(model => text(model) || '未记录模型').join('、') : '等待完整检测确认')],
      note: '请检测出现 408 的对应模型。实际出图成功且所有 408 恢复后才保留正式代理；失败或取消释放临时代理。更新 Cookie 或验证额度不能清除模型的 408。' })
  }
  if (Object.keys(latest).length) records.push(observation('最近请求记录', latest))
  const failure = object(health.last_failure)
  if (health.pending === true && Object.keys(failure).length && (latest.success === true || failure.checked_at !== latest.checked_at)) {
    records.push(observation('尚未确认恢复的失败记录', failure))
  }
  const success = object(health.last_success)
  if (Object.keys(success).length && (latest.success !== true || success.checked_at !== latest.checked_at)) {
    records.push(observation('最近成功记录', success))
  }
  if (typeof extra.last_check_success === 'boolean') {
    records.push({ title: '最近账号检测', result: extra.last_check_success ? '检测成功' : '检测失败', time: text(extra.last_check_at),
      fields: [field('状态码', code(extra.last_check_status_code)), field('来源 / 阶段', '此条检测摘要未单独记录')],
      note: !Object.keys(latest).length ? '目前只有检测摘要，没有完整的请求过程记录。' : undefined })
  }
  const check = object(extra.okad_connection_test)
  if (Object.keys(check).length) {
    records.push({ title: 'Cookie 推送后的检测任务', result: label(checkStates, check.state),
      fields: [field('排队时间', statusRecordTime(check.queued_at)), field('开始时间', statusRecordTime(check.started_at)),
        field('结束时间', statusRecordTime(check.finished_at))],
      note: '此处是该检测任务的结果；之后的业务请求或检测可能有不同结果。' })
  }
  const okad = object(extra.okad)
  if (Object.keys(okad).length) {
    records.push({ title: 'Cookie 接收与凭据验证', result: label(verificationStates, okad.verification), time: text(okad.verified_at),
      fields: [field('接收时间', statusRecordTime(okad.received_at)), field('Cookie 版本', okad.version ? String(okad.version) : '未记录')],
      note: '这里记录的是凭据验证结果，出图结果以请求或检测记录为准。' })
  }
  const recovery = object(extra.okad_recovery)
  if (Object.keys(recovery).length) {
    const recoveryStates = { awaiting_sync: '等待新 Cookie 同步', needs_manual: '需要人工完成验证码', missing_mail: '没有取信配置',
      not_registered: '没有登记恢复资料', ambiguous: '同一邮箱存在多个来源', configuration: '联动配置检查未通过',
      service_unavailable: 'Okad 服务请求失败', outcome_unknown: '登录结果尚未确认', login_failed: '自动登录未完成',
      rate_limited: '登录服务返回限流', protocol: '联动协议不兼容' }
    records.push({ title: '自动恢复记录', result: label(recoveryStates, recovery.code),
      fields: [field('下次检查时间', statusRecordTime(recovery.next_check_at))] })
  }
  return records
}

export function accountStatusFacts(account: Account, now = Date.now()): { label: string; value: string }[] {
  const facts = [field('账号状态', { active: '已启用', inactive: '已停用', error: '存在错误记录' }[account.status] || '未记录'),
    field('调度开关', account.schedulable ? '开启' : '关闭'),
    ...(account.extra?.okad_manual_pause === true ? [field('人工暂停记录', '已设置')] : [])]
  if (account.expires_at) {
    const date = new Date(account.expires_at * 1000)
    facts.push(field('账号到期时间', Number.isFinite(date.getTime()) ? statusRecordTime(date.toISOString()) : '时间记录无效'))
  }
  for (const [name, value] of [['限流截止时间', account.rate_limit_reset_at], ['过载截止时间', account.overload_until], ['临时停止调度截止时间', account.temp_unschedulable_until]]) {
    if (value) facts.push(field(name!, `${statusRecordTime(value)}${new Date(value).getTime() <= now ? '（已过期）' : ''}`))
  }
  const limits = object(account.extra?.model_rate_limits)
  for (const [model, value] of Object.entries(limits)) {
    const until = text(object(value).rate_limit_reset_at)
    if (until) facts.push(field(`模型 ${model} 限流截止`, `${statusRecordTime(until)}${new Date(until).getTime() <= now ? '（已过期）' : ''}`))
  }
  for (const [name, used, limit] of [['总额度', account.quota_used, account.quota_limit], ['每日额度', account.quota_daily_used, account.quota_daily_limit], ['每周额度', account.quota_weekly_used, account.quota_weekly_limit]] as const) {
    if (typeof limit === 'number' && limit > 0) facts.push(field(name, `已用 ${typeof used === 'number' ? used : '未记录'} / 上限 ${limit}`))
  }
  if (account.kiro_quota_reset_at) facts.push(field('额度重置时间', statusRecordTime(account.kiro_quota_reset_at)))
  if (account.kiro_runtime_reset_at) facts.push(field('运行限制截止时间', statusRecordTime(account.kiro_runtime_reset_at)))
  return facts
}
