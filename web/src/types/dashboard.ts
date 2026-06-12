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
  last_req: string        // truncated ~15 chars for table
  last_resp: string       // truncated ~15 chars for table
  last_req_full?: string  // full text for detail view
  last_resp_full?: string // full text for detail view
  platform?: string // Hermes only
  // Managed session fields
  is_managed?: boolean
  pending_permission?: PermissionInfo
}

export interface PermissionInfo {
  request_id: string
  tool_name: string
  tool_input: string
  tool_input_raw: Record<string, unknown>
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

export interface StartSessionRequest {
  project: string
  prompt?: string
}

export interface StartSessionResponse {
  session_id: string
  project: string
  started_at: string
}

export type AgentFilter = 'all' | 'claude' | 'hermes'

export type RefreshInterval = 0 | 10 | 30 | 60 // seconds, 0 = off
