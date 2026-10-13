export interface HealthObservation {
  success: boolean; code: number; reason: string; stage: string; source: string; scope: string; checked_at: string
}
export interface AdobeHealthAccount {
  id: number; name: string; scope: string; code: number; reason: string; stage: string; source: string
  failed_at?: string; success_at?: string; pending: boolean
}
export interface AdobeHealthReport {
  generated_at: string; window_start: string; total: number; pending: number; recent_pending: number
  recovered: number; unknown: number; historical: number; ratio: number; threshold: number; minimum: number
  accounts: AdobeHealthAccount[]
}
const reasons: Record<string, [string, string]> = {
  auth: ['登录凭据未通过验证', '查看 Cookie 联动和验证结果，避免反复登录。'],
  quota: ['账号额度不足', '检查剩余额度和重置时间。'],
  entitlement: ['模型权限不足', '检查账号套餐与模型权限。'],
  timeout: ['本地等待超时', '检查代理和网络，不代表 Cookie 失效。'],
  upstream: ['上游暂时无法完成请求', '检查是否批量超时或上游拥堵。'],
  proxy: ['代理连接异常', '检查账号绑定的代理。'],
  network: ['网络连接失败', '检查代理、DNS 和上游连接。'],
  content: ['内容被上游拒绝', '调整请求内容，账号不一定有问题。'],
  legacy: ['历史失败待验证', '下一次实际请求或检测成功后确认恢复。'],
  unknown: ['失败原因待确认', '未获得完整响应，请查看检测阶段和日志。']
}
export const healthReason = (reason: string): [string, string] => reasons[reason] || reasons.unknown!
export const healthSource = (source: string) => ({ business: '真实业务请求', okad: 'Cookie 推送后检测', test: '手动 / 批量检测', legacy: '升级前记录' })[source] || '未知来源'
export const healthStage = (stage: string) => ({ token: '验证 Cookie / 换取 Token', prepare: '准备请求', upload: '上传素材', submit: '提交生成', poll: '等待生成', download: '下载结果' })[stage] || '阶段未记录'
