import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountStatusIndicator from '../AccountStatusIndicator.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/i18n', () => ({
  i18n: {
    global: {
      t: (key: string) => key
    }
  },
  getLocale: () => 'en'
}))

vi.mock('@/utils/format', async () => {
  const actual = await vi.importActual<typeof import('@/utils/format')>('@/utils/format')
  return {
    ...actual,
    formatCountdown: () => '1h'
  }
})

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'account',
    platform: 'antigravity',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-03-15T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    kiro_quota_state: null,
    kiro_quota_reason: null,
    kiro_quota_reset_at: null,
    kiro_runtime_state: null,
    kiro_runtime_reason: null,
    kiro_runtime_reset_at: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides,
  }
}

describe('AccountStatusIndicator', () => {
  it('keeps the 408 quarantine visible alongside a successful result on another model', () => {
    const wrapper = mount(AccountStatusIndicator, { props: { account: makeAccount({ platform: 'adobe', extra: {
      adobe_proxy_408: { managed: true, state: 'quarantined', models: ['gpt-image-2.5-flare'] },
      adobe_health: { latest: { success: true, code: 200, source: 'test', scope: 'gpt-image-2', checked_at: '2026-10-22T08:00:00Z' } }
    } }) }, global: { stubs: { Icon: true } } })
    expect(wrapper.get('[data-testid="account-408-quarantine"]').text()).toContain('account408Waiting')
    expect(wrapper.get('[data-testid="account-last-check"]').text()).toContain('bulkTest.passed')
  })
  it('shows one successful Adobe check without stale error or historical pending badges', () => {
    const wrapper = mount(AccountStatusIndicator, { props: { account: makeAccount({
      platform: 'adobe', status: 'error', error_message: 'Clash 订阅节点已移除，请重新选择代理并恢复账号调度',
      extra: { last_check_success: true, last_check_status_code: 200, last_check_at: '2026-10-20T02:00:00Z', adobe_health: {
        pending: true, latest: { success: true, code: 200, source: 'test', scope: 'gpt-image-2', checked_at: '2026-10-20T02:00:00Z' }
      } }
    }) }, global: { stubs: { Icon: true } } })
    expect(wrapper.findAll('.badge')).toHaveLength(1)
    expect(wrapper.get('[data-testid="account-last-check"]').text()).toContain('bulkTest.passed')
    expect(wrapper.text()).toContain('gpt-image-2')
    expect(wrapper.text()).not.toContain('异常待成功验证')
    expect(wrapper.text()).not.toContain('status.error')
    expect(wrapper.find('[data-testid="account-error-details"]').exists()).toBe(false)
  })

  it.each([[401, 'badge-danger'], [408, 'badge-warning']])('preserves the severity color of a newer Adobe %i failure', (code, color) => {
    const wrapper = mount(AccountStatusIndicator, { props: { account: makeAccount({ platform: 'adobe', extra: {
      last_check_success: true, last_check_at: '2026-10-20T02:00:00Z', adobe_health: {
        latest: { success: false, code, source: 'business', checked_at: '2026-10-20T02:01:00Z' }
      }
    } }) }, global: { stubs: { Icon: true } } })
    expect(wrapper.get('[data-testid="account-current-result"]').classes()).toContain(color)
  })

  it('replaces a passed Adobe check when a newer business request fails', () => {
    const wrapper = mount(AccountStatusIndicator, { props: { account: makeAccount({ platform: 'adobe', extra: {
      last_check_success: true, last_check_at: '2026-10-20T02:00:00Z', adobe_health: {
        latest: { success: false, code: 408, source: 'business', scope: 'gpt-image-2.5-sunburst', checked_at: '2026-10-20T02:00:00.050Z' }
      }
    } }) }, global: { stubs: { Icon: true } } })
    expect(wrapper.findAll('.badge')).toHaveLength(1)
    expect(wrapper.text()).toContain('currentResult.failed')
    expect(wrapper.text()).toContain('408')
    expect(wrapper.text()).not.toContain('bulkTest.passed')
  })

  it.each([
    { status: 'error', error_message: 'Authentication failed (401): invalid_credentials' },
    { extra: { adobe_health: { pending: true }, last_check_success: false, last_check_status_code: 408 } },
    { extra: { last_check_success: false, last_check_status_code: 408 } },
    { status: 'error', extra: { okad_connection_test: { state: 'pending' } } },
    { rate_limit_reset_at: '2099-01-01T00:00:00Z' },
    { overload_until: '2099-01-01T00:00:00Z' },
    { temp_unschedulable_until: '2099-01-01T00:00:00Z' },
    { schedulable: false },
    { extra: { last_check_success: true, last_check_status_code: 200 } }
  ] as Partial<Account>[])('keeps every status tag unchanged when descriptions move into details: %j', async (overrides) => {
    const wrapper = mount(AccountStatusIndicator, { props: { account: makeAccount({ platform: 'adobe', ...overrides }) }, global: { stubs: { Icon: true, teleport: true } } })
    const tags = () => wrapper.findAll('span.badge, span.inline-flex, button.badge').map(node => ({ text: node.text(), classes: node.classes() }))
    const before = tags()
    await wrapper.setProps({ detailsInDialog: true })
    expect(tags()).toEqual(before)
    expect(wrapper.find('[data-testid="account-error-details"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('replaces a previous timeout with each new completed test result', async () => {
    const account = makeAccount({
      platform: 'adobe',
      temp_unschedulable_until: '2020-01-01T00:00:00Z',
      temp_unschedulable_reason: '{"status_code":408}',
      extra: { last_check_success: false, last_check_status_code: 408, adobe_last_status_code: 408 }
    })
    const wrapper = mount(AccountStatusIndicator, { props: { account }, global: { stubs: { Icon: true } } })
    expect(wrapper.text()).toContain('requestTimeout')
    await wrapper.setProps({ account: { ...account, extra: { ...account.extra, last_check_success: true, last_check_status_code: 200 } } })
    expect(wrapper.text()).not.toContain('408')
    expect(wrapper.get('[data-testid="account-last-check"]').text()).toContain('bulkTest.passed')
    await wrapper.setProps({ account: { ...account, extra: { ...account.extra, last_check_success: false, last_check_status_code: 401 } } })
    expect(wrapper.text()).not.toContain('408')
    expect(wrapper.get('[data-testid="account-last-check"]').text()).toContain('401')
    wrapper.unmount()
  })
  it('shows persisted test results independently of scheduling status', async () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: { account: makeAccount({ extra: { last_check_success: false, last_check_status_code: 401, last_check_at: '2026-10-11T03:00:00Z' } }) },
      global: { stubs: { Icon: true } }
    })
    expect(wrapper.get('[data-testid="account-last-check"]').text()).toContain('401')
    expect(wrapper.text()).toContain('admin.accounts.status.active')
    expect(wrapper.find('[data-testid="account-auth-status"]').exists()).toBe(false)
    await wrapper.setProps({ account: makeAccount({ extra: { last_check_success: true, last_check_status_code: 200 } }) })
    expect(wrapper.get('[data-testid="account-last-check"]').text()).toContain('bulkTest.passed')
    expect(wrapper.text()).not.toContain('401')
    wrapper.unmount()
  })

  it('shows Adobe invalid_credentials as a red 401 and reveals the full error on hover', async () => {
    const error = 'Token refresh failed (non-retryable): adobe cookie is no longer valid, re-export it from the browser: refresh response missing access_token (keys=error) error=invalid_credentials'
    const wrapper = mount(AccountStatusIndicator, {
      props: { account: makeAccount({ platform: 'adobe', status: 'error', error_message: error }) },
      global: { stubs: { Icon: true, teleport: true } }
    })

    expect(wrapper.get('.badge-danger').text()).toBe('admin.accounts.status.cookieInvalid')
    expect(wrapper.get('[data-testid="account-auth-status"]').classes()).toContain('bg-red-100')
    expect(wrapper.get('[role="tooltip"]').isVisible()).toBe(false)
    await wrapper.get('[data-testid="account-error-details"]').trigger('mouseenter')
    expect(wrapper.get('[role="tooltip"]').isVisible()).toBe(true)
    expect(wrapper.get('[role="tooltip"]').text()).toBe(error)
    await wrapper.get('[data-testid="account-error-details"]').trigger('mouseleave')
    expect(wrapper.get('[role="tooltip"]').isVisible()).toBe(false)

    await wrapper.setProps({ account: makeAccount({ platform: 'adobe', extra: { last_check_success: false, last_check_status_code: 401 } }) })
    expect(wrapper.find('[data-testid="account-auth-status"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.status.active')
    expect(wrapper.get('[data-testid="account-last-check"]').text()).toContain('401')
    expect(wrapper.get('[role="tooltip"]').text()).toBe('admin.accounts.status.lastCheckAuthenticationFailedHint')
    await wrapper.setProps({ account: makeAccount({ platform: 'adobe', extra: { last_check_success: true, last_check_status_code: 200 } }) })
    expect(wrapper.find('[data-testid="account-error-details"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it.each([
    'API returned 401 while checking the account',
    'Authentication failed (401): Okad automatic login limit reached (2/2); manual reauthorization required',
    'OAuth 401 (no refresh_token): credentials rejected'
  ])('shows current authentication errors ahead of an old 408: %s', (error) => {
    const wrapper = mount(AccountStatusIndicator, {
      props: { account: makeAccount({ status: 'error', error_message: error, extra: { last_check_status_code: 408, last_check_success: false } }) },
      global: { stubs: { Icon: true, teleport: true } }
    })
    expect(wrapper.get('[data-testid="account-auth-status"]').text()).toBe('401')
    expect(wrapper.text()).not.toContain('requestTimeout')
    expect(wrapper.get('[role="tooltip"]').text()).toBe(error)
    wrapper.unmount()
  })

  it('keeps the full temporary authentication error and ignores expired temporary failures', async () => {
    const account = makeAccount({
      temp_unschedulable_until: '2099-03-15T00:00:00Z',
      temp_unschedulable_reason: JSON.stringify({ status_code: 401, error_message: 'upstream session expired', matched_keyword: 'session' })
    })
    const wrapper = mount(AccountStatusIndicator, {
      props: { account },
      global: { stubs: { Icon: true, teleport: true } }
    })
    expect(wrapper.get('[data-testid="account-auth-status"]').text()).toBe('401')
    expect(wrapper.get('[role="tooltip"]').text()).toBe(account.temp_unschedulable_reason)
    await wrapper.setProps({ account: { ...account, temp_unschedulable_until: '2020-03-15T00:00:00Z' } })
    expect(wrapper.find('[data-testid="account-auth-status"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it.each([
    'refresh response missing access_token (keys=error) error=invalid_client',
    'refresh response missing access_token (keys=error) error=invalid_scope',
    'account 401 failed to connect: network unavailable',
    'API returned 403: permission denied'
  ])('does not label unrelated errors as 401: %s', (error) => {
    const wrapper = mount(AccountStatusIndicator, {
      props: { account: makeAccount({ platform: 'adobe', status: 'error', error_message: error, extra: { last_check_status_code: 401, last_check_success: false } }) },
      global: { stubs: { Icon: true, teleport: true } }
    })
    expect(wrapper.find('[data-testid="account-auth-status"]').exists()).toBe(false)
    expect(wrapper.get('[role="tooltip"]').text()).toBe(error)
    wrapper.unmount()
  })
  it('账号检查返回 408 时显示请求超时状态和状态码', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          error_message: 'API returned 408 while checking the account'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.requestTimeout')
    expect(wrapper.text()).toContain('408')
  })

  it('临时不可调度原因中的 408 也显示请求超时状态', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          temp_unschedulable_until: '2099-03-15T00:00:00Z',
          temp_unschedulable_reason: JSON.stringify({ status_code: 408 })
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.requestTimeout')
    expect(wrapper.text()).toContain('408')
    expect(wrapper.text()).not.toContain('admin.accounts.status.tempUnschedulable')
  })

  it('Claude 5 系列模型限流时显示 Opus 和 Sonnet 的短别名', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          extra: {
            model_rate_limits: {
              'claude-opus-5': {
                rate_limited_at: '2026-07-28T00:00:00Z',
                rate_limit_reset_at: '2099-07-28T00:00:00Z'
              },
              'claude-sonnet-5': {
                rate_limited_at: '2026-07-28T00:00:00Z',
                rate_limit_reset_at: '2099-07-28T00:00:00Z'
              },
              'claude-sonnet-5-5': {
                rate_limited_at: '2026-09-28T00:00:00Z',
                rate_limit_reset_at: '2099-09-28T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('COpus5')
    expect(wrapper.text()).toContain('CSon5')
    expect(wrapper.text()).toContain('CSon55')
    expect(wrapper.text()).not.toContain('claude-sonnet-5')
  })

  it('Grok 账号额度限流时显示自动恢复时间而非临时不可调度', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 5,
          name: 'grok-free-1',
          platform: 'grok',
          rate_limited_at: '2026-07-11T12:00:00Z',
          rate_limit_reset_at: '2099-07-11T13:00:00Z',
          temp_unschedulable_until: '2099-07-11T12:30:00Z',
          temp_unschedulable_reason: 'legacy grok rate limited'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.find('.badge-warning').text()).toBe('admin.accounts.status.rateLimited')
    expect(wrapper.text()).toContain('admin.accounts.status.rateLimitedAutoResume')
    expect(wrapper.text()).not.toContain('admin.accounts.status.tempUnschedulable')
  })

  it('模型限流 + overages 启用 + 无 AICredits key → 显示 ⚡ (credits_active)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 1,
          name: 'ag-1',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('⚡')
    expect(wrapper.text()).toContain('CSon45')
  })

  it('模型限流 + overages 未启用 → 普通限流样式（无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 2,
          name: 'ag-2',
          extra: {
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
  })

  it('AICredits key 生效 → 显示积分已用尽 (credits_exhausted)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 3,
          name: 'ag-3',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })

  it('模型限流 + overages 启用 + AICredits key 生效 → 普通限流样式（积分耗尽，无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 4,
          name: 'ag-4',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              },
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    // 模型限流 + 积分耗尽 → 不应显示 ⚡
    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
    // AICredits 积分耗尽状态应显示
    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })

  it('Kiro 运行时冷却在状态列复用限流展示', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 5,
          name: 'kiro-cooldown',
          platform: 'kiro',
          kiro_runtime_state: 'cooldown',
          kiro_runtime_reason: 'rate_limit_exceeded',
          kiro_runtime_reset_at: '2099-03-15T00:00:00Z'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.rateLimited')
    expect(wrapper.text()).toContain('admin.accounts.status.rateLimitedAutoResume')
    expect(wrapper.text()).toContain('429')
  })

  it('Kiro suspended 在状态列显示为 forbidden', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 6,
          name: 'kiro-suspended',
          platform: 'kiro',
          kiro_runtime_state: 'suspended',
          kiro_runtime_reason: 'account_suspended',
          kiro_runtime_reset_at: '2099-03-15T00:00:00Z'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.forbidden')
  })

})
