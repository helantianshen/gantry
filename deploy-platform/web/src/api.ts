import type { App, Deployment, DeploymentEvent, Page, Version } from './types'

type ErrorBody = { msg?: string }

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })
  const body = (await response.json().catch(() => ({}))) as T & ErrorBody
  if (!response.ok) throw new Error(body.msg || `请求失败（${response.status}）`)
  return body
}

function query(params: Record<string, string | number | undefined>) {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') search.set(key, String(value))
  })
  return search.toString()
}

export const api = {
  health: () => request<{ status: string }>('/healthz'),
  apps: (page = 1, pageSize = 20) => request<Page<App>>(`/api/v1/apps?${query({ page, page_size: pageSize })}`),
  app: (id: number) => request<App>(`/api/v1/apps/${id}`),
  createApp: (body: Pick<App, 'name' | 'image_name' | 'healthcheck_path' | 'healthcheck_timeout_sec'>) =>
    request<{ id: number }>('/api/v1/apps', { method: 'POST', body: JSON.stringify(body) }),
  versions: (appID: number) => request<Version[]>(`/api/v1/apps/${appID}/versions`),
  createVersion: (appID: number, body: Pick<Version, 'tag' | 'description'>) =>
    request<{ id: number }>(`/api/v1/apps/${appID}/versions`, { method: 'POST', body: JSON.stringify(body) }),
  deployments: (page = 1, pageSize = 20, appID?: number, status?: string) =>
    request<Page<Deployment>>(`/api/v1/deployments?${query({ page, page_size: pageSize, app_id: appID, status })}`),
  deployment: (id: number) => request<Deployment>(`/api/v1/deployments/${id}`),
  events: (id: number) => request<DeploymentEvent[]>(`/api/v1/deployments/${id}/events`),
  deploy: (appID: number, versionID: number) =>
    request<{ id: number; status: string }>('/api/v1/deployments', {
      method: 'POST',
      body: JSON.stringify({ app_id: appID, version_id: versionID }),
    }),
}

export function formatTime(value?: string) {
  return value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value)) : '—'
}
