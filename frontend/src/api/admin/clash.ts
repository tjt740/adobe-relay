import { apiClient } from '../client'

export interface ClashNode {
  name: string
  type: string
  server?: string
  server_port?: number
  proxy_id: number
  proxy_host?: string
  proxy_port?: number
  enabled: boolean
}

export interface ClashSubscription {
  configured: boolean
  ready: boolean
  url?: string
  url_hint: string
  revision: number
  updated_at: string
  nodes: ClashNode[]
}

export async function getClashSubscription(): Promise<ClashSubscription> {
  const { data } = await apiClient.get<ClashSubscription>('/admin/proxies/clash')
  return data
}

export async function previewClashSubscription(url: string): Promise<ClashSubscription> {
  const { data } = await apiClient.post<ClashSubscription>('/admin/proxies/clash/preview', { url }, { timeout: 45000 })
  return data
}

export async function importClashSubscription(url: string, names: string[], revision: number): Promise<ClashSubscription> {
  const { data } = await apiClient.post<ClashSubscription>('/admin/proxies/clash/import', { url, names, revision }, { timeout: 120000 })
  return data
}

export interface ProxySourceHealth {
  id: string
  index: number
  url_hint: string
  status: 'checking' | 'healthy' | 'degraded' | 'unavailable' | 'fetch_failed' | 'stale' | 'exhausted' | 'expired'
  fetch_error?: string
  http_status?: number
  checked_at: string
  last_success_at: string
  traffic_known: boolean
  remaining_bytes: number | null
  total_bytes: number | null
  traffic_observed_at: string | null
  expires_at: string | null
  nodes: number
  healthy: number
  failed: number
  unknown: number
  bound_accounts: number
}
export interface ProxyHealth {
  checked_at: string
  sources: ProxySourceHealth[]
  groups: Array<{ id: number; name: string; total: number; enabled: number; available: number; waiting_proxy: number }>
}
export async function getProxyHealth(): Promise<ProxyHealth> {
  return (await apiClient.get<ProxyHealth>('/admin/proxies/clash/health', { timeout: 8000 })).data
}
