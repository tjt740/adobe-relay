<template>
  <button
    ref="triggerRef"
    type="button"
    class="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
    :class="activeCount ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : 'text-gray-500 hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white'"
    :aria-label="t('admin.accounts.capacityFilter.title')"
    :aria-expanded="open"
    :aria-controls="id"
    aria-haspopup="dialog"
    :title="summary || t('admin.accounts.capacityFilter.title')"
    data-test="capacity-filter-trigger"
    @click.stop="toggle"
  >
    {{ t('admin.accounts.columns.capacity') }}
    <Icon name="filter" size="xs" />
    <span v-if="activeCount" class="rounded bg-primary-100 px-1 text-[10px] tabular-nums dark:bg-primary-800" :aria-label="summary" data-test="capacity-filter-count">{{ activeCount }}</span>
  </button>

  <Teleport to="body">
    <div
      v-if="open"
      :id="id"
      ref="panelRef"
      role="dialog"
      :aria-labelledby="`${id}-title`"
      class="fixed z-[100000010] w-80 overflow-y-auto rounded-xl border border-gray-200 bg-white text-left text-sm font-normal normal-case tracking-normal shadow-xl dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
      :style="panelStyle"
      data-test="capacity-filter-panel"
      @keydown="handleKeydown"
    >
      <form novalidate @submit.prevent="apply">
        <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <h3 :id="`${id}-title`" class="font-semibold text-gray-900 dark:text-white">{{ t('admin.accounts.capacityFilter.title') }}</h3>
          <button type="button" class="rounded p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-dark-700" :aria-label="t('common.close')" @click="close()">
            <Icon name="x" size="sm" />
          </button>
        </div>
        <div class="space-y-4 p-4">
          <fieldset>
            <legend class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accounts.capacityFilter.label') }}</legend>
            <div class="grid grid-cols-2 gap-2">
              <label
                v-for="option in capacityOptions"
                :key="option.value"
                class="flex cursor-pointer items-center gap-2 rounded-lg border px-2.5 py-2.5 text-xs transition-colors focus-within:ring-2 focus-within:ring-primary-500"
                :class="draft.capacity === option.value ? 'border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-600 dark:bg-primary-900/30 dark:text-primary-300' : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'"
              >
                <input v-model="draft.capacity" type="radio" :name="`${id}-state`" :value="option.value" class="h-3.5 w-3.5 text-primary-600 focus:ring-0" />
                {{ option.label }}
              </label>
            </div>
          </fieldset>
          <fieldset>
            <legend class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accounts.capacityFilter.limitRange') }}</legend>
            <div class="flex items-center gap-2">
              <input v-model="draft.concurrency_min" type="number" min="1" step="1" class="input min-w-0 flex-1" :aria-label="t('admin.accounts.capacityFilter.min')" :aria-invalid="!!validationError" :aria-describedby="validationError ? `${id}-error` : undefined" :placeholder="t('admin.accounts.capacityFilter.min')" />
              <span class="text-gray-400">–</span>
              <input v-model="draft.concurrency_max" type="number" min="1" step="1" class="input min-w-0 flex-1" :aria-label="t('admin.accounts.capacityFilter.max')" :aria-invalid="!!validationError" :aria-describedby="validationError ? `${id}-error` : undefined" :placeholder="t('admin.accounts.capacityFilter.max')" />
            </div>
            <p class="mt-2 text-xs text-gray-400">{{ t('admin.accounts.capacityFilter.rangeHint') }}</p>
            <p v-if="validationError" :id="`${id}-error`" role="alert" class="mt-2 text-xs text-red-600 dark:text-red-400">{{ validationError }}</p>
          </fieldset>
        </div>
        <div class="flex items-center justify-between border-t border-gray-100 px-4 py-3 dark:border-dark-700">
          <button type="button" class="text-xs text-gray-500 hover:text-primary-600 dark:text-gray-400" data-test="capacity-filter-clear" @click="clear">{{ t('admin.accounts.capacityFilter.clear') }}</button>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary px-3 py-1.5 text-xs" data-test="capacity-filter-cancel" @click="close()">{{ t('common.cancel') }}</button>
            <button type="submit" class="btn btn-primary px-3 py-1.5 text-xs" :disabled="!!validationError" data-test="capacity-filter-apply">{{ t('common.apply') }}</button>
          </div>
        </div>
      </form>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref } from 'vue'
import { onClickOutside, useEventListener, useResizeObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

type CapacityFilters = { capacity: string; concurrency_min: string; concurrency_max: string }
const props = defineProps<{ id: string; filters: Partial<CapacityFilters> }>()
const emit = defineEmits<{ apply: [filters: CapacityFilters] }>()
const { t } = useI18n()
const open = ref(false)
const triggerRef = ref<HTMLButtonElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const draft = reactive({ capacity: '', concurrency_min: '' as string | number, concurrency_max: '' as string | number })
const position = ref({ top: 0, left: 0 })
const panelStyle = computed(() => ({ top: `${position.value.top}px`, left: `${position.value.left}px`, maxWidth: 'calc(100vw - 16px)', maxHeight: 'calc(100dvh - 16px)' }))
const capacityOptions = computed(() => [
  { value: '', label: t('admin.accounts.capacityFilter.all') },
  { value: 'idle', label: t('admin.accounts.capacityFilter.idle') },
  { value: 'in_use', label: t('admin.accounts.capacityFilter.inUse') },
  { value: 'full', label: t('admin.accounts.capacityFilter.full') }
])
const activeCount = computed(() => Number(!!props.filters.capacity) + Number(!!(props.filters.concurrency_min || props.filters.concurrency_max)))
const summary = computed(() => {
  const parts: string[] = []
  if (props.filters.capacity) parts.push(capacityOptions.value.find(option => option.value === props.filters.capacity)?.label || '')
  if (props.filters.concurrency_min || props.filters.concurrency_max) {
    parts.push(`${t('admin.accounts.capacityFilter.limitRange')}: ${props.filters.concurrency_min || '1'}–${props.filters.concurrency_max || '∞'}`)
  }
  return parts.join(' · ')
})
const validationError = computed(() => {
  for (const raw of [draft.concurrency_min, draft.concurrency_max]) {
    if (raw !== '' && (!Number.isSafeInteger(Number(raw)) || Number(raw) < 1)) return t('admin.accounts.capacityFilter.positiveInteger')
  }
  if (draft.concurrency_min !== '' && draft.concurrency_max !== '' && Number(draft.concurrency_min) > Number(draft.concurrency_max)) return t('admin.accounts.capacityFilter.invalidRange')
  return ''
})

const updatePosition = () => {
  if (!open.value || !triggerRef.value || !panelRef.value) return
  const anchor = triggerRef.value.getBoundingClientRect()
  const panel = panelRef.value.getBoundingClientRect()
  const maxTop = window.innerHeight - panel.height - 8
  const top = anchor.bottom + 8 <= maxTop ? anchor.bottom + 8 : anchor.top - panel.height - 8
  position.value = {
    top: Math.max(8, Math.min(top, maxTop)),
    left: Math.max(8, Math.min(anchor.left, window.innerWidth - panel.width - 8))
  }
}
const close = (restoreFocus = true) => {
  open.value = false
  if (restoreFocus) triggerRef.value?.focus({ preventScroll: true })
}
const toggle = async () => {
  if (open.value) { close(); return }
  Object.assign(draft, {
    capacity: props.filters.capacity || '',
    concurrency_min: props.filters.concurrency_min || '',
    concurrency_max: props.filters.concurrency_max || ''
  })
  open.value = true
  await nextTick()
  updatePosition()
  panelRef.value?.querySelector<HTMLInputElement>('input[type="radio"]:checked')?.focus({ preventScroll: true })
}
const apply = () => {
  if (validationError.value) return
  emit('apply', {
    capacity: draft.capacity,
    concurrency_min: draft.concurrency_min === '' ? '' : String(Number(draft.concurrency_min)),
    concurrency_max: draft.concurrency_max === '' ? '' : String(Number(draft.concurrency_max))
  })
  close()
}
const clear = () => {
  emit('apply', { capacity: '', concurrency_min: '', concurrency_max: '' })
  close()
}
const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(); return }
  if (event.key !== 'Tab') return
  const focusable = panelRef.value?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not([type="radio"]), input[type="radio"]:checked')
  if (!focusable?.length) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
onClickOutside(panelRef, () => close(false), { ignore: [triggerRef] })
useEventListener(window, 'resize', updatePosition)
useEventListener(window, 'scroll', (event) => {
  if (open.value && !(event.target instanceof Node && panelRef.value?.contains(event.target))) close(false)
}, { capture: true })
useResizeObserver(panelRef, updatePosition)
</script>
