<template>
  <BaseDialog :show="show" title="状态详情" width="normal" @close="$emit('close')">
    <div v-if="account" class="space-y-5" data-testid="account-status-details">
      <div>
        <p class="break-all font-medium text-gray-900 dark:text-gray-100">{{ account.name }}</p>
        <p class="mt-1 text-xs text-gray-500">账号 #{{ account.id }} · 以下为打开详情时已有的记录，时间按当前浏览器时区显示</p>
      </div>
      <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-800">
        <template v-for="item in facts" :key="item.label">
          <dt class="text-gray-500 dark:text-dark-400">{{ item.label }}</dt>
          <dd class="break-words text-gray-800 dark:text-gray-200">{{ item.value }}</dd>
        </template>
      </dl>
      <AccountHealthEvidence :account="account" />
      <section v-if="message || temporaryMessage" class="space-y-3 text-sm">
        <div v-if="message">
          <h4 class="font-medium text-gray-900 dark:text-gray-100">账号保存的错误说明</h4>
          <p class="mt-2 text-gray-600 dark:text-gray-300">{{ statusErrorExplanation(account.error_message) }}</p>
          <details class="mt-2 text-xs text-gray-500"><summary class="cursor-pointer">原始记录（凭据已隐藏）</summary><p class="mt-2 whitespace-pre-wrap break-words">{{ message }}</p></details>
          <p class="mt-1 text-xs text-gray-500">该字段没有单独的发生时间，请结合上面的请求和检测记录判断。</p>
        </div>
        <div v-if="temporaryMessage">
          <h4 class="font-medium text-gray-900 dark:text-gray-100">临时停止调度的记录</h4>
          <p class="mt-2 text-gray-600 dark:text-gray-300">{{ statusErrorExplanation(account.temp_unschedulable_reason) }}</p>
          <details class="mt-2 text-xs text-gray-500"><summary class="cursor-pointer">原始记录（凭据已隐藏）</summary><p class="mt-2 whitespace-pre-wrap break-words">{{ temporaryMessage }}</p></details>
        </div>
      </section>
    </div>
    <template #footer><button type="button" class="btn btn-secondary" @click="$emit('close')">关闭</button></template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { Account } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import AccountHealthEvidence from './AccountHealthEvidence.vue'
import { accountStatusFacts, safeStatusMessage, statusErrorExplanation } from '@/utils/accountStatusEvidence'
const props = defineProps<{ show: boolean; account: Account | null }>()
defineEmits<{ (event: 'close'): void }>()
const facts = computed(() => props.account ? accountStatusFacts(props.account) : [])
const message = computed(() => safeStatusMessage(props.account?.error_message))
const temporaryMessage = computed(() => safeStatusMessage(props.account?.temp_unschedulable_reason))
</script>
