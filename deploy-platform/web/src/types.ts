export type App = {
  id: number
  name: string
  image_name: string
  healthcheck_path: string
  healthcheck_timeout_sec: number
  current_version_id?: number
  created_at?: string
  updated_at?: string
}

export type Version = {
  id: number
  app_id: number
  tag: string
  description: string
  created_at?: string
}

export const deploymentStatusLabels = {
  pending: '等待入队',
  queued: '已入队',
  running: '发布中',
  success: '已上线',
  failed: '发布失败',
  rolled_back: '已回滚',
  failed_rollback: '回滚失败',
} as const

export type DeploymentStatus = keyof typeof deploymentStatusLabels

export type Deployment = {
  id: number
  app_id: number
  version_id: number
  from_version_id?: number
  status: DeploymentStatus
  message_id: string
  lease_owner?: string
  lease_expires_at?: string
  attempt: number
  fail_reason: string
  created_at?: string
  updated_at?: string
}

export type DeploymentEvent = {
  id: number
  deployment_id: number
  type: string
  from_status?: string
  to_status?: string
  actor: string
  detail?: unknown
  created_at?: string
}

export type Page<T> = {
  items: T[]
  total: number
  page: number
  page_size: number
}
