<template>
  <div
    role="progressbar"
    :aria-label="label"
    :aria-valuenow="completed"
    :aria-valuemin="0"
    :aria-valuemax="total || 1"
    :aria-busy="running"
    class="relative h-2.5 overflow-hidden rounded-full bg-pink-100 dark:bg-pink-950/50"
  >
    <div
      data-testid="bulk-test-progress-fill"
      class="absolute inset-y-0 left-0 rounded-full bg-pink-500 transition-[width] duration-500 ease-out motion-reduce:transition-none"
      :style="{ width: `${percentage}%` }"
    />
    <!-- Motion means a request is still running; the solid fill is real progress. -->
    <div v-if="running" class="absolute inset-y-0 overflow-hidden" :style="{ left: `${percentage}%`, right: 0 }">
      <div data-testid="bulk-test-progress-loading" class="progress-sweep absolute inset-y-0 w-1/3 rounded-full bg-gradient-to-r from-transparent via-pink-400 to-transparent" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ completed: number; total: number; running: boolean; label: string }>()
const percentage = computed(() => props.total ? Math.min(100, props.completed / props.total * 100) : 0)
</script>

<style scoped>
.progress-sweep {
  animation: progress-sweep 1.5s ease-in-out infinite;
}
@keyframes progress-sweep {
  from { transform: translateX(-100%); }
  to { transform: translateX(400%); }
}
@media (prefers-reduced-motion: reduce) {
  .progress-sweep { animation: none; left: 33%; }
}
</style>
