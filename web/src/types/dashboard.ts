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
  first_active: string // ISO 8601
  last_active: string // ISO 8601
  last_req: string        // truncated ~15 chars for table
  last_resp: string       // truncated ~15 chars for table
  last_req_full?: string  // full text for detail view
  last_resp_full?: string // full text for detail view
  platform?: string // Hermes only
  has_terminal: boolean
  terminal_tmux_name?: string
  matched_root?: string // project root path if matched, empty = unmatched
}

export interface ReminderScoreInfo {
  score: number
  fog_score: number
  highlight: number   // 0-100, log-mapped from score
  fog_pct: number     // 0-100, fog_score * 100
  level: 'none' | 'low' | 'medium' | 'high'
  waiting_min: number
  streak_min: number
  is_current: boolean
}

export interface ProjectRoot {
  path: string
  priority: 'primary' | 'secondary' | 'normal'
}

export interface DashboardResponse {
  now: string // ISO 8601
  window: string
  project_roots: ProjectRoot[]
  sessions: DashboardEntry[]
  reminder_scores: Record<string, ReminderScoreInfo>
  focus?: FocusState
  errors?: string[]
}

export interface FocusState {
  active: boolean
  focused_project: string
  minutes: number
  since: string  // ISO 8601 timestamp when the current focus period started
}

export interface ScanOptions {
  window: string // e.g. "1h", "3h", "1d"
  max_inactive: number
}

export type AgentFilter = 'all' | 'claude' | 'hermes'

export type RefreshInterval = 0 | 10 | 30 | 60 // seconds, 0 = off

// ── Terminal types ────────────────────────────────────────────────

export interface TerminalSession {
  name: string
  work_dir: string
  ttyd_port: number
  ttyd_url: string
}

export interface TerminalResponse {
  ok?: boolean
  error?: string
  name?: string
  work_dir?: string
  ttyd_port?: number
  ttyd_url?: string
}

export interface TerminalLookupResponse {
  found: boolean
  verified: boolean
  tmux_name?: string
  work_dir?: string
  ttyd_port?: number
  ttyd_url?: string
  hint?: string
  warning?: string
}
