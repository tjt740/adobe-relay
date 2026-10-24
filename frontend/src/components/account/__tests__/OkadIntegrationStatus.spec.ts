import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import Status from '../OkadIntegrationStatus.vue'
import type { Account } from '@/types'
vi.mock('@/api/client', () => ({ default: { post: vi.fn() } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('Okad credential validation status', () => {
  it('ignores legacy quarantine when showing the actual credential state', () => {
    const account = { id: 1, platform: 'adobe', extra: { okad: { verification: 'proxy_pending' }, adobe_proxy_408: { managed: true, state: 'quarantined' } } } as unknown as Account
    const wrapper = mount(Status, { props: { account } })
    expect(wrapper.text()).not.toContain('account408Waiting')
    expect(wrapper.text()).toContain('等待代理就绪')
  })
  it.each([
    ['pending','error',false,undefined,'等待接口检测'],
    ['running','error',false,undefined,'正在检测'],
    ['failed','error',false,false,'检测失败'],
    ['failed','active',true,false,'账号仍在调度'],
    ['interrupted','error',false,undefined,'检测中断'],
    ['proxy_pending','error',false,undefined,'等待代理'],
    ['success','active',true,true,'已恢复调度'],
    ['success','active',false,true,'当前账号仍暂停'],
    ['success','error',false,true,'当前账号仍暂停'],
    ['success','active',true,false,'最新接口检测失败'],
  ])('%s reflects current availability', (phase,status,schedulable,last,text) => {
    const account = {id:1, platform:'adobe', status, schedulable, extra:{okad_connection_test:{state:phase}, last_check_success:last}} as unknown as Account
    const wrapper=mount(Status,{props:{account}})
    expect(wrapper.text()).toContain(text)
    if(phase !== 'success') expect(wrapper.text()).not.toContain('已恢复调度')
  })
})
