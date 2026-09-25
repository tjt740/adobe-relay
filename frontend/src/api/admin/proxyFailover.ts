import { apiClient } from '../client'

export interface ProxyFailoverPolicy {
  enabled: boolean
  primary_proxy_id: number
  backup_proxy_ids: number[]
  revision: number
  current_proxy_id: number
}
export interface ProxyFailoverView extends ProxyFailoverPolicy {
  state: string
  health: Array<{ proxy_id: number; status: string; failures: number; latency_ms: number; checked_at: string; message: string }>
  events: Array<{ from_proxy_id: number; to_proxy_id: number; created_at: string }>
}
export async function getProxyFailover(id: number): Promise<ProxyFailoverView> {
  return (await apiClient.get<ProxyFailoverView>(`/admin/accounts/${id}/proxy-failover`)).data
}
export async function saveProxyFailover(id: number, policy: ProxyFailoverPolicy): Promise<ProxyFailoverView> {
  return (await apiClient.put<ProxyFailoverView>(`/admin/accounts/${id}/proxy-failover`, policy)).data
}
