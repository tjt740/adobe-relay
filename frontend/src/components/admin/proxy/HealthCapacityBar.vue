<template>
  <div
    class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-600"
    :role="percentage === null ? 'img' : 'progressbar'"
    :aria-label="percentage === null ? label + ': ' + summary : label"
    :aria-valuemin="percentage === null ? undefined : 0"
    :aria-valuemax="percentage === null ? undefined : 100"
    :aria-valuenow="percentage ?? undefined"
    :aria-valuetext="percentage === null ? undefined : summary"
  >
    <div v-if="percentage !== null" class="h-full rounded-full" :class="colors[tone]" :style="{ width: percentage + '%' }" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
const props = defineProps<{
  value: number | null
  total: number | null
  label: string
  summary: string
  tone: 'green' | 'amber' | 'red' | 'gray'
}>()
const colors = { green: 'bg-emerald-500', amber: 'bg-amber-500', red: 'bg-red-500', gray: 'bg-gray-400' }
const percentage = computed(() => {
  if (props.value === null || props.total === null || !Number.isFinite(props.value) || !Number.isFinite(props.total) || props.total <= 0) return null
  return Math.round(Math.min(100, Math.max(0, props.value / props.total * 100)) * 10) / 10
})
</script>
