<template>
  <div v-if="state || recovery || check" class="flex max-w-[230px] flex-col gap-1 text-[11px]" data-testid="okad-integration-status">
    <span class="rounded bg-blue-50 px-2 py-1 text-blue-700 dark:bg-blue-950 dark:text-blue-200" :title="detail">{{ label }}</span>
    <span v-if="state?.version" class="text-gray-500">Cookie v{{ state.version }} · {{ state.source_key }}</span>
    <span v-if="recovery?.next_check_at" class="text-gray-500">下次检查 {{ new Date(String(recovery.next_check_at)).toLocaleTimeString() }}</span>
    <button v-if="state?.version" type="button" class="text-left text-blue-600 hover:underline disabled:opacity-50" :disabled="busy" @click="verify">{{ busy ? '正在验证…' : '重新验证凭据' }}</button>
    <span v-if="message" class="text-gray-500" role="status">{{ message }}</span>
  </div>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import apiClient from '@/api/client'
import type { Account } from '@/types'
const props = defineProps<{ account: Account }>()
const busy = ref(false)
const message = ref('')
const state = computed(() => props.account.extra?.okad as Record<string, unknown> | undefined)
const check = computed(() => props.account.extra?.okad_connection_test as Record<string, unknown> | undefined)
const recovery = computed(() => props.account.extra?.okad_recovery as Record<string, unknown> | undefined)
const reasons: Record<string, string> = {
  pending: 'Cookie 已同步，等待验证', verified: '上次凭据验证通过', verification_retry: '等待重新验证',
  proxy_pending: '等待代理就绪后验证',
  auth_failed: '新凭据未通过验证', quota_blocked: '额度不足或未知', entitlement_blocked: '账号权益不足',
  awaiting_sync: '已有新 Cookie，等待同步', needs_manual: '需要人工验证码', missing_mail: '缺少取信配置',
  not_registered: '尚未登记恢复资料', ambiguous: '同邮箱来源重复', configuration: '联动配置异常',
  service_unavailable: 'Okad 暂不可用', outcome_unknown: '登录结果待确认', identity_mismatch: '关联身份不一致',
  login_failed: '自动登录未完成', rate_limited: '登录服务限流', protocol: '联动版本不兼容'
}
const label = computed(() => {
  const phase = String(check.value?.state || '')
  if (phase === 'pending') return 'Cookie 已同步，等待接口检测'
  if (phase === 'running') return check.value?.resume === false ? '正在检测，保留原暂停状态' : '正在检测，验证通过后恢复调度'
  if (phase === 'proxy_pending') return '等待代理就绪后检测'
  if (phase === 'interrupted') return '检测中断，请重新检测'
  if (phase === 'failed') return props.account.status === 'active' && props.account.schedulable ? '最新接口检测失败，账号仍在调度' : '接口检测失败，未恢复调度'
  if (phase === 'success') {
    if (props.account.extra?.last_check_success === false) return '上次 Cookie 验证通过，最新接口检测失败'
    return props.account.status === 'active' && props.account.schedulable ? '接口检测通过，已恢复调度' : '上次检测通过，当前账号仍暂停或异常'
  }
  const code = String(recovery.value?.code || state.value?.verification || '')
  return reasons[code] || '查看联动详情'
})
const detail = computed(() => String(check.value ? label.value : state.value?.message || label.value))
async function verify() {
  busy.value = true
  try {
    const { data } = await apiClient.post(`/admin/accounts/${props.account.id}/okad-verify`, null, { timeout: 75000 })
    message.value = String(data.integration?.message || '验证完成，请刷新账号状态')
  } catch (err: unknown) {
    const detail = (err as { message?: unknown })?.message
    message.value = typeof detail === 'string' && detail ? detail : '验证暂未完成，或账号已被修改；请刷新后重试'
  } finally { busy.value = false }
}
</script>
