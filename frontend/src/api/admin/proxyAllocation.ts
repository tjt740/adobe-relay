import { apiClient } from '../client'

export interface ProxyAllocationView {
  enabled: boolean
  revision: number
  accounts_per_proxy: number
  accounts: number
  assigned: number
  waiting: number
  healthy_nodes: number
  available_slots: number
  checked_at: string | null
  nodes: Array<{ id: number; name: string; accounts: number; status: string; latency_ms: number }>
}

export async function getProxyAllocation(): Promise<ProxyAllocationView> {
  return (await apiClient.get<ProxyAllocationView>('/admin/accounts/proxy-allocation')).data
}
export async function saveProxyAllocation(enabled: boolean, revision: number): Promise<ProxyAllocationView> {
  return (await apiClient.put<ProxyAllocationView>('/admin/accounts/proxy-allocation', { enabled, revision })).data
}
