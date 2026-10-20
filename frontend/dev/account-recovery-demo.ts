// Development-only preview of the actual changelog component. No backend calls.
import { createApp, h, ref, watchEffect } from 'vue'
import { createI18n } from 'vue-i18n'
import DailyChangelog from '../src/components/admin/changelog/DailyChangelog.vue'
import zh from '../src/i18n/locales/zh/changelog'
import en from '../src/i18n/locales/en/changelog'
import '../src/style.css'

const params = new URLSearchParams(window.location.search)
const i18n = createI18n({ legacy: false, locale: params.get('lang') === 'en' ? 'en' : 'zh', messages: { zh: { changelog: zh }, en: { changelog: en } } })
createApp({
  setup() {
    const dark = ref(params.get('dark') === '1')
    watchEffect(() => {
      document.documentElement.classList.toggle('dark', dark.value)
      document.documentElement.lang = i18n.global.locale.value
    })
    return () => h('main', { class: 'mx-auto max-w-6xl space-y-6 p-4 sm:p-8' }, [
      h('header', { class: 'flex flex-wrap items-center justify-between gap-3' }, [
        h('h1', { class: 'text-xl font-semibold text-gray-900 dark:text-white' }, i18n.global.t('changelog.daily.title')),
        h('div', { class: 'flex gap-3' }, [
          h('button', { class: 'btn btn-secondary', 'data-preview-locale': true, onClick: () => { i18n.global.locale.value = i18n.global.locale.value === 'zh' ? 'en' : 'zh' } }, i18n.global.locale.value === 'zh' ? 'English' : '中文'),
          h('button', { class: 'btn btn-secondary', 'data-preview-theme': true, onClick: () => { dark.value = !dark.value } }, dark.value ? 'Light' : 'Dark'),
        ]),
      ]),
      h(DailyChangelog),
    ])
  },
}).use(i18n).mount('#app')
