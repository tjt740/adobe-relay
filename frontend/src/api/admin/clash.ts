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
