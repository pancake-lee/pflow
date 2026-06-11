// TypeScript types matching the Go DashboardEntry and DashboardResponse.
// Keep in sync with internal/api/server.go.

export interface DashboardEntry {
  session_id: string
  agent_type: 'claude' | 'hermes'
  project: string
  status: string
  is_active: boolean
  traffic_light: string
  name: string
  last_active: string // ISO 8601
  last_req: string
  last_resp: string
  platform?: string // Hermes only
}

export interface DashboardResponse {
  now: string // ISO 8601
  window: string
  sessions: DashboardEntry[]
  errors?: string[]
}

export interface ScanOptions {
  window: string // e.g. "1h", "3h", "1d"
  max_inactive: number
}

export type AgentFilter = 'all' | 'claude' | 'hermes'

export type RefreshInterval = 0 | 10 | 30 | 60 // seconds, 0 = off
