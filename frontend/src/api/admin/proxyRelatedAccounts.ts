import { apiClient } from '../client'

export interface RelatedProxyRef { id: number; name: string }
export interface ProxyRelatedAccount {
  id: number
  name: string
  platform: string
  using_this_node: boolean
  mode: 'manual' | 'manual_direct' | 'automatic' | 'failover' | 'unmanaged'
  account_state: 'waiting_proxy' | 'paused' | 'disabled' | 'error' | 'expired' | 'cooldown' | 'eligible'
  route_state: 'waiting_proxy' | 'direct' | 'unavailable' | 'subscription_blocked' | 'generation_paused' | 'unconfirmed' | 'reachable'
  impact: 'left_node' | 'affected' | 'unconfirmed' | 'not_blocked'
  current_proxy: RelatedProxyRef | null
  last_departure: { at: string; destination: RelatedProxyRef | null } | null
}
export interface ProxyRelatedAccountsView {
  proxy: RelatedProxyRef
  checked_at: string
  history_since: string
  current_count: number
  departed_count: number
  total: number
  page: number
  page_size: number
  items: ProxyRelatedAccount[]
}
export interface ProxyRelatedAccountsQuery {
  page: number
  page_size: number
  relation: 'all' | 'current' | 'departed'
  search: string
}
export async function getProxyRelatedAccounts(id: number, params: ProxyRelatedAccountsQuery, signal?: AbortSignal): Promise<ProxyRelatedAccountsView> {
  return (await apiClient.get<ProxyRelatedAccountsView>(`/admin/proxies/${id}/related-accounts`, { params, signal, timeout: 8000 })).data
}
