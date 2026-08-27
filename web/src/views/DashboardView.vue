<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, h, nextTick, type Component } from 'vue'
import {
  NLayout,
  NLayoutHeader,
  NLayoutContent,
  NLayoutFooter,
  NTag,
  NSelect,
  NInputNumber,
  NButton,
  NDrawer,
  NDrawerContent,
  NDescriptions,
  NDescriptionsItem,
  NSpace,
  NIcon,
  NSpin,
  NAlert,
  NTooltip,
  NModal,
  NCollapse,
  NCollapseItem,
  useMessage,
} from 'naive-ui'
import {
  DesktopOutline,
  RefreshOutline,
  HardwareChipOutline,
  SettingsOutline,
  TrashOutline,
} from '@vicons/ionicons5'
import type { DataTableColumns } from 'naive-ui'
import type {
  DashboardEntry,
  ScanOptions,
  AgentFilter,
  RefreshInterval,
  TerminalResponse,
  TerminalLookupResponse,
} from '../types/dashboard'
import { useDashboard } from '../composables/useDashboard'
import { usePolling } from '../composables/usePolling'
import { formatSince, formatMinutes, truncate, escapeNewlines } from '../composables/format'
import GroupCard from '../components/GroupCard.vue'
import PrimaryCard from '../components/PrimaryCard.vue'
import SecondaryCard from '../components/SecondaryCard.vue'
import SuggestCard from '../components/SuggestCard.vue'
import KnowledgeAnchor from '../components/KnowledgeAnchor.vue'
import type { SessionGroup } from '../components/GroupCard.vue'
import type { ReminderScoreInfo } from '../types/dashboard'
import { FOCUS_CONFIG } from '../composables/useReminderScores'
import { STAR_BONUS_MINUTES } from '../config/attention'

// ── Props ─────────────────────────────────────────────────────────

const props = defineProps<{
  initialGoal?: string
}>()

// ── Daily goal ────────────────────────────────────────────────────

const todayGoal = ref(props.initialGoal ?? '')

async function updateGoal(newGoal: string) {
  todayGoal.value = newGoal
  try {
    await fetch('/api/v1/daily-boot', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'update_goal', goal: newGoal }),
    })
  } catch {
    // Silently fail
  }
}

// Inline editing
const editingGoalInline = ref(false)
const editGoalTextInline = ref('')
const goalInlineInputRef = ref<HTMLInputElement | null>(null)

function startEditGoalInline() {
  editGoalTextInline.value = todayGoal.value
  editingGoalInline.value = true
  nextTick(() => {
    goalInlineInputRef.value?.focus()
    goalInlineInputRef.value?.select()
  })
}

function saveGoalInline() {
  editingGoalInline.value = false
  const trimmed = editGoalTextInline.value.trim()
  if (trimmed !== todayGoal.value) {
    updateGoal(trimmed)
  }
}

function cancelEditGoalInline() {
  editingGoalInline.value = false
}

// ── State ────────────────────────────────────────────────────────

const { data, loading, error, fetchDashboard } = useDashboard()
const message = useMessage()

// ── Collapse state persistence ────────────────────────────────────
// Persists which project zones (normal / unmatched / archived) are
// expanded/collapsed so the user's preference survives page refreshes.

const COLLAPSE_KEY = 'pflow:dashboard:collapse'

interface CollapseStore {
  normal: string[]
  unmatched: string[]
}

function loadCollapsed(): CollapseStore {
  try {
    const raw = localStorage.getItem(COLLAPSE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      return {
        normal: Array.isArray(parsed.normal) ? parsed.normal : ['normal'],
        unmatched: Array.isArray(parsed.unmatched) ? parsed.unmatched : [],
      }
    }
  } catch { /* ignore corrupt data */ }
  return { normal: ['normal'], unmatched: [] }
}

function persistCollapsed(normal: string[], unmatched: string[]) {
  try {
    localStorage.setItem(COLLAPSE_KEY, JSON.stringify({ normal, unmatched }))
  } catch { /* ignore quota errors */ }
}

const init = loadCollapsed()
const normalExpanded = ref<string[]>(init.normal)
const unmatchedExpanded = ref<string[]>(init.unmatched)

watch(normalExpanded, (val) => persistCollapsed(val, unmatchedExpanded.value), { deep: true })
watch(unmatchedExpanded, (val) => persistCollapsed(normalExpanded.value, val), { deep: true })

// Reminder scores from API response
const reminderScores = computed(() => data.value?.reminder_scores ?? {})

/** Get reminder score info for a project group by its key. */
function getGroupScore(groupKey: string): ReminderScoreInfo | undefined {
  return reminderScores.value[groupKey]
}

// ── Focus mode ──────────────────────────────────────────────────

const focusActive = computed(() => data.value?.focus?.active ?? false)
const focusFocusedProject = computed(() => data.value?.focus?.focused_project ?? '')
const focusMinutes = computed(() => data.value?.focus?.minutes ?? 0)
const focusSince = computed(() => data.value?.focus?.since ?? '')
const focusLoading = ref(false)

// Reactive "now" updated every second to drive the countdown computed.
const now = ref(Date.now())
let nowTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => { nowTimer = setInterval(() => { now.value = Date.now() }, 1000) })
onUnmounted(() => { if (nowTimer) clearInterval(nowTimer) })

const focusCountdown = computed(() => {
  if (!focusActive.value) return ''
  const since = focusSince.value
  if (!since) return '...'
  const start = new Date(since).getTime()
  const end = start + focusMinutes.value * 60 * 1000
  const remaining = end - now.value
  if (remaining <= 0) return '0:00'
  const mins = Math.floor(remaining / 60000)
  const secs = Math.floor((remaining % 60000) / 1000)
  return `${mins}:${secs.toString().padStart(2, '0')}`
})

/** Focus-mode dimming opacity (from config, used as inline style). */
const focusDimOpacity = computed(() => FOCUS_CONFIG.dimOpacity)

async function focusExtend(projectKey: string) {
  if (!projectKey) {
    message.error('No project selected for focus')
    return
  }
  focusLoading.value = true
  try {
    const resp = await fetch('/api/v1/focus/extend', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ project: projectKey }),
    })
    const result = await resp.json()
    if (!resp.ok) {
      message.error(result.error || 'Failed to extend focus')
      return
    }
    message.success(`Focus extended to ${result.minutes}min`)
    refresh()
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'Failed to extend focus')
  } finally {
    focusLoading.value = false
  }
}

async function focusStop() {
  focusLoading.value = true
  try {
    const resp = await fetch('/api/v1/focus/stop', { method: 'POST' })
    const result = await resp.json()
    if (!resp.ok) {
      message.error(result.error || 'Failed to stop focus')
      return
    }
    message.success('Focus mode exited')
    refresh()
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'Failed to stop focus')
  } finally {
    focusLoading.value = false
  }
}

// Filter params
const windowOptions = [
  { label: '1 hour', value: '1h' },
  { label: '3 hours', value: '3h' },
  { label: '6 hours', value: '6h' },
  { label: '1 day', value: '1d' },
  { label: '3 days', value: '3d' },
  { label: '7 days', value: '7d' },
]
const selectedWindow = ref('1d')
const maxInactive = ref(1)
const agentFilter = ref<AgentFilter>('all')
const refreshInterval = ref<RefreshInterval>(30)

const agentFilterOptions = [
  { label: 'All Agents', value: 'all' },
  { label: 'Claude Code', value: 'claude' },
  { label: 'Hermes', value: 'hermes' },
  { label: 'Codex', value: 'codex' },
]

const refreshOptions = [
  { label: 'Off', value: 0 },
  { label: '10s', value: 10 },
  { label: '30s', value: 30 },
  { label: '60s', value: 60 },
]

// Detail drawer
const showDetail = ref(false)
const selectedSession = ref<DashboardEntry | null>(null)

// Resizable drawer
const drawerWidth = ref(Math.max(480, Math.floor(window.innerWidth / 4)))
const minDrawerWidth = computed(() => Math.floor(window.innerWidth / 4))
const maxDrawerWidth = computed(() => Math.floor(window.innerWidth * 3 / 4))

function startResize(e: MouseEvent) {
  e.preventDefault()
  const startX = e.clientX
  const startWidth = drawerWidth.value
  function onMove(ev: MouseEvent) {
    const delta = startX - ev.clientX
    const newWidth = startWidth + delta
    drawerWidth.value = Math.min(maxDrawerWidth.value, Math.max(minDrawerWidth.value, newWidth))
  }
  function onUp() {
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

// Terminal state
const terminalUrl = ref<string | null>(null)
const terminalName = ref<string | null>(null)
const terminalLoading = ref(false)
const terminalError = ref<string | null>(null)
const terminalFound = ref(false)
const terminalVerified = ref(false)
const terminalLookupHint = ref<string | null>(null)
const terminalLookupWarning = ref<string | null>(null)
const terminalLookupDone = ref(false)
const showTerminalModal = ref(false)

function openTerminalModal() {
  if (terminalUrl.value) {
    showTerminalModal.value = true
    return
  }
  startTerminal()
}

async function openTerminalFromTable(row: DashboardEntry) {
  selectedSession.value = row
  showDetail.value = false
  terminalUrl.value = null
  terminalError.value = null
  terminalLookupHint.value = null
  terminalLookupWarning.value = null
  if (row.has_terminal && row.terminal_tmux_name) {
    terminalFound.value = true
    terminalVerified.value = false
    terminalName.value = row.terminal_tmux_name
    terminalLookupDone.value = true
    await startTerminal()
    return
  }
  if ((row.agent_type === 'claude' || row.agent_type === 'codex') && row.session_id) {
    await lookupTerminal(row.session_id, row.agent_type)
    if (terminalFound.value) {
      await startTerminal()
    }
  }
}

// ── Derived ──────────────────────────────────────────────────────

const scanOpts = computed<ScanOptions>(() => ({
  window: selectedWindow.value,
  max_inactive: maxInactive.value,
}))

const filteredSessions = computed(() => {
  if (!data.value?.sessions) return []
  if (agentFilter.value === 'all') return data.value.sessions
  return data.value.sessions.filter((s) => s.agent_type === agentFilter.value)
})

function projectBasename(path: string): string {
  if (!path || path === '?' || path === '/') return 'Other'
  const cleaned = path.replace(/\/+$/, '')
  const idx = cleaned.lastIndexOf('/')
  return idx >= 0 ? cleaned.slice(idx + 1) : cleaned
}

// Build a lookup map from the slots response (slot_id → path)
const slotsMap = computed(() => data.value?.slots ?? {} as Record<string, string>)

type Priority = 'primary' | 'secondary' | 'normal'

const groupedSessions = computed<SessionGroup[]>(() => {
  const sessions = filteredSessions.value
  const roots = data.value?.project_roots ?? []
  const rootSet = new Set(roots.map(r => r.path))
  const rootPriorityMap = new Map<string, Priority>()
  for (const r of roots) {
    rootPriorityMap.set(r.path, r.priority)
  }
  const groups = new Map<string, DashboardEntry[]>()

  for (const s of sessions) {
    const key = s.matched_root || s.project || 'Other'
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key)!.push(s)
  }

  // Ensure every marked project root appears as a group, even if it has
  // no sessions (e.g., after a reboot). This allows slot cards to show
  // the assigned project with a "暂无活动的Agent" placeholder.
  for (const r of roots) {
    if (!groups.has(r.path)) {
      groups.set(r.path, [])
    }
  }

  const result: SessionGroup[] = Array.from(groups.entries()).map(([key, sess]) => ({
    key,
    basename: projectBasename(key),
    fullPath: key,
    sessions: sess,
    hasActive: sess.some(s => s.is_active),
    hasWaiting: sess.some(s => s.status === 'waiting'),
    lastActive: sess.length > 0 ? Math.max(...sess.map(s => new Date(s.last_active).getTime())) : 0,
    isRoot: rootSet.has(key),
    priority: rootPriorityMap.get(key) ?? null,
  }))

  // Sort: primary first, then secondary, then normal, then unmatched
  const priorityOrder: Record<string, number> = { primary: 0, secondary: 1, normal: 2, unmatched: 3 }
  result.sort((a, b) => {
    const pa = priorityOrder[a.priority ?? 'unmatched']
    const pb = priorityOrder[b.priority ?? 'unmatched']
    if (pa !== pb) return pa - pb
    if (a.hasActive !== b.hasActive) return a.hasActive ? -1 : 1
    if (a.hasWaiting !== b.hasWaiting) return a.hasWaiting ? -1 : 1
    return b.lastActive - a.lastActive
  })

  return result
})

// Suggestions from API
const suggestions = computed(() => data.value?.suggestions ?? [])

// Zone splits — use slots map directly for stable slot positioning
const primaryGroup = computed(() => {
  const path = slotsMap.value['primary'] ?? null
  if (!path) return null
  return groupedSessions.value.find(g => g.fullPath === path) ?? null
})
const secondaryGroups = computed(() => {
  const slot1Path = slotsMap.value['secondary_1'] ?? null
  const slot2Path = slotsMap.value['secondary_2'] ?? null
  const findGroup = (p: string | null): SessionGroup | null =>
    p ? groupedSessions.value.find(g => g.fullPath === p) ?? null : null
  return [findGroup(slot1Path), findGroup(slot2Path)]
})
const normalGroups = computed(() =>
  groupedSessions.value.filter(g => g.priority === 'normal'),
)
const unmatchedGroups = computed(() =>
  groupedSessions.value.filter(g => g.priority === null),
)

// ── Main session state (frontend-only, per-group) ──────────────
// Star = user preference mark, NOT a hard override. The starred session
// gets STAR_BONUS_MINUTES added to its last_active when competing for
// main session — a "tolerance window", not a permanent assignment.

const starredSessionIds = ref<Record<string, string>>({})

function getMainSession(group: SessionGroup | null): DashboardEntry | null {
  if (!group || group.sessions.length === 0) return null

  let best: DashboardEntry | null = null
  let bestTime = -Infinity

  for (const s of group.sessions) {
    const base = new Date(s.last_active).getTime()
    const bonus = s.session_id === starredSessionIds.value[group.key]
      ? STAR_BONUS_MINUTES * 60_000
      : 0
    const effective = base + bonus
    if (effective > bestTime) {
      bestTime = effective
      best = s
    }
  }

  return best
}

const primaryMainSession = computed(() => getMainSession(primaryGroup.value))

// ── Project selector options (all groups that could fill a slot) ──

const projectSelectOptions = computed(() =>
  groupedSessions.value.map(g => ({
    label: g.basename + (g.fullPath !== 'Other' ? ' — ' + g.fullPath : ''),
    value: g.fullPath,
  })),
)

// ── Slot assignment ────────────────────────────────────────────

const projectRootLoading = ref(false)

async function assignSlot(path: string, slot: string) {
  projectRootLoading.value = true
  try {
    const resp = await fetch('/api/v1/project-roots/slot', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, slot }),
    })
    const result = await resp.json()
    if (!resp.ok) {
      message.error(result.error || `Failed to assign to slot ${slot}`)
      return
    }
    message.success(`Project assigned to ${slot}`)
    refresh()
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'Failed to assign project')
  } finally {
    projectRootLoading.value = false
  }
}

async function clearSlot(slotName: string) {
  projectRootLoading.value = true
  try {
    const resp = await fetch(`/api/v1/project-roots/slot?slot=${encodeURIComponent(slotName)}`, {
      method: 'DELETE',
    })
    const result = await resp.json()
    if (!resp.ok) {
      message.error(result.error || `Failed to clear slot`)
      return
    }
    message.success('Slot cleared (project demoted to normal)')
    refresh()
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'Failed to clear slot')
  } finally {
    projectRootLoading.value = false
  }
}

function handleSelectPrimary(path: string | null) {
  if (!path) {
    if (slotsMap.value['primary']) {
      clearSlot('primary')
    }
    return
  }
  assignSlot(path, 'primary')
}

function handleSelectSecondary(path: string | null, slotName: string) {
  if (!path) {
    clearSlot(slotName)
    return
  }
  assignSlot(path, slotName)
}

// ── Checkbox (normal/unmatched groups) ─────────────────────────

async function markAsRoot(path: string) {
  projectRootLoading.value = true
  try {
    const resp = await fetch('/api/v1/project-roots', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, priority: 'normal' }),
    })
    const result = await resp.json()
    if (!resp.ok) {
      message.error(result.error || `Failed to mark as project root`)
      return
    }
    message.success(`Marked as project root`)
    refresh()
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'Failed to mark as project root')
  } finally {
    projectRootLoading.value = false
  }
}

async function unmarkRoot(path: string) {
  projectRootLoading.value = true
  try {
    const resp = await fetch(`/api/v1/project-roots?path=${encodeURIComponent(path)}`, {
      method: 'DELETE',
    })
    const result = await resp.json()
    if (!resp.ok) {
      message.error(result.error || `Failed to unmark project root`)
      return
    }
    message.success('Removed project root')
    refresh()
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'Failed to unmark project root')
  } finally {
    projectRootLoading.value = false
  }
}

function handleCheckChange(group: SessionGroup, checked: boolean) {
  if (checked) {
    markAsRoot(group.fullPath)
  } else {
    unmarkRoot(group.fullPath)
  }
}

// ── Star session control ────────────────────────────────────────

function handleStarSession(groupKey: string, sessionId: string) {
  if (starredSessionIds.value[groupKey] === sessionId) {
    // Unstar: clicking the already-starred session
    const { [groupKey]: _, ...rest } = starredSessionIds.value
    starredSessionIds.value = rest
  } else {
    starredSessionIds.value = { ...starredSessionIds.value, [groupKey]: sessionId }
  }
}

function getStarredSessionId(groupKey: string): string | null {
  return starredSessionIds.value[groupKey] ?? null
}

// ── Row click → detail drawer ───────────────────────────────────

function openDetail(row: DashboardEntry) {
  if (selectedSession.value?.session_id !== row.session_id) {
    terminalUrl.value = null
    terminalError.value = null
    terminalFound.value = false
    terminalVerified.value = false
    terminalLookupHint.value = null
    terminalLookupWarning.value = null
    terminalLookupDone.value = false
    showTerminalModal.value = false
  }
  selectedSession.value = row
  showDetail.value = true
  if ((row.agent_type === 'claude' || row.agent_type === 'codex') && row.session_id) {
    lookupTerminal(row.session_id, row.agent_type)
  } else {
    terminalLookupDone.value = true
    terminalFound.value = false
  }
}

function handleOpenTerminalFromCard(row: DashboardEntry) {
  openTerminalFromTable(row)
}

// ── Terminal logic ──────────────────────────────────────────────

async function lookupTerminal(sessionId: string, agentType = 'claude') {
  terminalLookupDone.value = false
  terminalFound.value = false
  terminalVerified.value = false
  terminalLookupHint.value = null
  terminalLookupWarning.value = null
  try {
    const resp = await fetch(`/api/v1/terminal/lookup?session_id=${encodeURIComponent(sessionId)}&agent_type=${encodeURIComponent(agentType)}`)
    if (!resp.ok) {
      terminalLookupHint.value = `Lookup failed (HTTP ${resp.status})`
      terminalLookupDone.value = true
      return
    }
    const data2: TerminalLookupResponse = await resp.json()
    terminalLookupDone.value = true
    if (data2.found) {
      terminalFound.value = true
      terminalVerified.value = data2.verified
      terminalName.value = data2.tmux_name ?? null
      if (data2.warning) terminalLookupWarning.value = data2.warning
      if (data2.ttyd_url) terminalUrl.value = data2.ttyd_url
    } else {
      terminalLookupHint.value = data2.hint ?? 'No tmux session found for this agent session'
    }
  } catch (e) {
    terminalLookupHint.value = e instanceof Error ? e.message : 'Lookup error'
    terminalLookupDone.value = true
  }
}

async function startTerminal() {
  if (!selectedSession.value) return
  const workDir = selectedSession.value.project
  if (!workDir || workDir === '?' || workDir === '/') {
    terminalError.value = 'No valid working directory for this session'
    return
  }
  terminalLoading.value = true
  terminalError.value = null
  try {
    const body: Record<string, string> = { work_dir: workDir }
    if (terminalFound.value && terminalName.value) {
      body['tmux_name'] = terminalName.value
    }
    const resp = await fetch('/api/v1/terminal/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    const data2: TerminalResponse = await resp.json()
    if (!resp.ok || data2.error) {
      throw new Error(data2.error || `HTTP ${resp.status}`)
    }
    terminalUrl.value = data2.ttyd_url ?? null
    terminalName.value = data2.name ?? null
    terminalFound.value = true
    showTerminalModal.value = true
  } catch (e) {
    terminalError.value = e instanceof Error ? e.message : 'Failed to start terminal'
    terminalUrl.value = null
    terminalName.value = null
  } finally {
    terminalLoading.value = false
  }
}

// ── Project management modal ────────────────────────────────────

const showProjectMgmt = ref(false)
const forgetLoading = ref<Record<string, boolean>>({})

function openProjectMgmt() {
  showProjectMgmt.value = true
}

async function forgetProject(path: string) {
  forgetLoading.value = { ...forgetLoading.value, [path]: true }
  try {
    const resp = await fetch(`/api/v1/project-roots?path=${encodeURIComponent(path)}`, {
      method: 'DELETE',
    })
    const result = await resp.json()
    if (!resp.ok) {
      message.error(result.error || `Failed to forget project`)
      return
    }
    message.success('Project forgotten')
    refresh()
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'Failed to forget project')
  } finally {
    const { [path]: _, ...rest } = forgetLoading.value
    forgetLoading.value = rest
  }
}

/** Look up which slot (if any) a project path occupies. */
function getSlotForPath(path: string): string | null {
  const slots = slotsMap.value
  for (const [slotId, slotPath] of Object.entries(slots)) {
    if (slotPath === path) return slotId
  }
  return null
}

/** Get last activity time for a project root from sessions. */
function getLastActiveForRoot(path: string): string {
  const sessions = data.value?.sessions ?? []
  let last = 0
  for (const s of sessions) {
    if (s.matched_root === path || s.project === path) {
      const t = new Date(s.last_active).getTime()
      if (t > last) last = t
    }
  }
  if (last > 0) {
    return new Date(last).toLocaleString()
  }
  return '从未活动'
}

// ── Lifecycle ────────────────────────────────────────────────────

onMounted(() => { refresh() })
usePolling(refresh, refreshInterval)

function refresh() {
  fetchDashboard(scanOpts.value)
}

// ── Stats ────────────────────────────────────────────────────────

const stats = computed(() => {
  const sessions = data.value?.sessions ?? []
  return {
    total: sessions.length,
    active: sessions.filter((s) => s.is_active).length,
    waiting: sessions.filter((s) => s.status === 'waiting').length,
    idle: sessions.filter((s) => s.status === 'idle').length,
    groups: groupedSessions.value.length,
  }
})

// ── Agent icon helper ────────────────────────────────────────────

function agentIcon(agentType: string): Component {
  return agentType === 'claude' ? DesktopOutline : HardwareChipOutline
}

type TagType = 'default' | 'success' | 'warning' | 'error' | 'primary' | 'info'

function trafficColor(light: string): TagType {
  switch (light) {
    case '🟢': return 'success'
    case '🟡': return 'warning'
    case '⚪': return 'default'
    default: return 'default'
  }
}

// ── Group columns (for normal/unmatched GroupCard) ──────────────

const groupColumns: DataTableColumns<DashboardEntry> = [
  {
    title: '',
    key: 'agent',
    width: 40,
    render(row) {
      return h(NIcon, { size: 18 }, { default: () => h(agentIcon(row.agent_type)) })
    },
  },
  {
    title: 'Session ID',
    key: 'session_id',
    width: 100,
    render(row) {
      return h('span', { style: { fontFamily: 'monospace', fontSize: '12px' } }, row.session_id || '—')
    },
  },
  {
    title: 'Status',
    key: 'status',
    width: 110,
    render(row) {
      const color = trafficColor(row.traffic_light)
      const label = row.traffic_light + ' ' + row.status
      return h(NTag, { size: 'small', type: color, bordered: false }, { default: () => label })
    },
  },
  {
    title: 'Name',
    key: 'name',
    width: 180,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', escapeNewlines(truncate(row.name, 24)) || '—')
    },
  },
  {
    title: 'Last Active',
    key: 'last_active',
    width: 100,
    render(row) {
      const since = formatSince(row.last_active)
      const full = new Date(row.last_active).toLocaleString()
      return h(NTooltip, {}, { trigger: () => h('span', since), default: () => full })
    },
    sorter: (a, b) => new Date(a.last_active).getTime() - new Date(b.last_active).getTime(),
    defaultSortOrder: 'descend',
  },
  {
    title: 'Sum Active',
    key: 'today_minutes',
    width: 90,
    render(row) {
      const mins = formatMinutes(row.today_minutes)
      const tooltip = `${row.today_minutes?.toFixed(1) ?? '0'} min estimated active today`
      return h(NTooltip, {}, { trigger: () => h('span', { style: { fontVariantNumeric: 'tabular-nums' } }, mins), default: () => tooltip })
    },
    sorter: (a, b) => (a.today_minutes ?? 0) - (b.today_minutes ?? 0),
  },
  {
    title: 'Last Req',
    key: 'last_req',
    width: 250,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', escapeNewlines(truncate(row.last_req, 30)) || '—')
    },
  },
  {
    title: 'TTY',
    key: 'terminal',
    width: 50,
    render(row) {
      if ((row.agent_type !== 'claude' && row.agent_type !== 'codex') || !row.has_terminal) return null
      return h(
        NButton,
        {
          size: 'tiny',
          quaternary: true,
          onClick: (e: MouseEvent) => {
            e.stopPropagation()
            openTerminalFromTable(row)
          },
        },
        { default: () => '🖥' },
      )
    },
  },
]

function rowProps(row: DashboardEntry) {
  return {
    style: 'cursor: pointer',
    onClick: () => openDetail(row),
  }
}
</script>

<template>
  <div class="app-shell" :class="{ 'focus-mode': focusActive }">
    <NLayout class="layout">
    <!-- Header with inline stats -->
    <NLayoutHeader bordered>
      <div class="header">
        <div class="header-left">
          <h1 class="title">⚔️ pflow</h1>
          <span class="subtitle">Agent Activity Dashboard</span>
        </div>

        <!-- Stats in header -->
        <div class="header-stats">
          <div v-if="focusActive" class="focus-overlay" :style="{ opacity: focusDimOpacity }"></div>
          <div class="header-stat">
            <span class="header-stat-value">{{ stats.total }}</span>
            <span class="header-stat-label">Total</span>
          </div>
          <div class="header-stat header-stat--active">
            <span class="header-stat-value">{{ stats.active }}</span>
            <span class="header-stat-label">Active</span>
          </div>
          <div class="header-stat header-stat--waiting">
            <span class="header-stat-value">{{ stats.waiting }}</span>
            <span class="header-stat-label">Waiting</span>
          </div>
          <div class="header-stat header-stat--idle">
            <span class="header-stat-value">{{ stats.idle }}</span>
            <span class="header-stat-label">Idle</span>
          </div>
        </div>

        <div class="header-right">
          <NButton
            size="small"
            quaternary
            @click="refresh"
            :loading="loading"
          >
            <template #icon>
              <NIcon><RefreshOutline /></NIcon>
            </template>
            Refresh
          </NButton>
          <NButton
            size="small"
            quaternary
            @click="openProjectMgmt"
          >
            <template #icon>
              <NIcon><SettingsOutline /></NIcon>
            </template>
            项目
          </NButton>
        </div>
      </div>
    </NLayoutHeader>

    <!-- Content -->
    <NLayoutContent>
      <div class="content">
        <!-- Error banner -->
        <NAlert
          v-if="error"
          type="error"
          :title="error"
          closable
          style="margin-bottom: 16px"
        />

        <!-- Filter bar -->
        <div class="filter-bar">
          <div v-if="focusActive" class="focus-overlay" :style="{ opacity: focusDimOpacity }"></div>
          <NSpace align="center" wrap>
            <span class="filter-label">Window:</span>
            <NSelect
              v-model:value="selectedWindow"
              :options="windowOptions"
              size="small"
              style="width: 120px"
              @update:value="refresh"
            />
            <span class="filter-label">Inactive:</span>
            <NInputNumber
              v-model:value="maxInactive"
              size="small"
              :min="0"
              :max="10"
              style="width: 80px"
              @update:value="refresh"
            />
            <span class="filter-label">Agent:</span>
            <NSelect
              v-model:value="agentFilter"
              :options="agentFilterOptions"
              size="small"
              style="width: 130px"
            />
            <span class="filter-label">Refresh:</span>
            <NSelect
              v-model:value="refreshInterval"
              :options="refreshOptions"
              size="small"
              style="width: 80px"
            />
            <span v-if="data" class="filter-note">
              Last updated: {{ new Date(data.now).toLocaleTimeString() }}
            </span>
          </NSpace>
        </div>

        <!-- Main content -->
        <NSpin :show="loading && !data">
          <div v-if="groupedSessions.length > 0" class="groups-container">

            <!-- ⭐ 主线项目 — full-width card, always visible (header now inside PrimaryCard) -->
            <div class="zone-section zone-section--primary">
              <PrimaryCard
                :group="primaryGroup"
                :main-session="primaryMainSession"
                :starred-session-id="primaryGroup ? getStarredSessionId(primaryGroup.key) : null"
                :disabled="projectRootLoading"
                :highlight="primaryGroup ? (getGroupScore(primaryGroup.key)?.highlight ?? 0) : 0"
                :fog-pct="primaryGroup ? (getGroupScore(primaryGroup.key)?.fog_pct ?? 0) : 0"
                :project-options="projectSelectOptions"
                :focus-active="focusActive"
                :focus-focused-project="focusFocusedProject"
                :focus-loading="focusLoading"
                :focus-countdown="focusCountdown"
                @star-session="(sid: string) => primaryGroup && handleStarSession(primaryGroup.key, sid)"
                @row-click="openDetail"
                @open-terminal="handleOpenTerminalFromCard"
                @select-project="handleSelectPrimary"
                @focus-extend="(key: string) => focusExtend(key)"
                @focus-stop="focusStop"
              />
            </div>

            <!-- 🎯 今日目标 — standalone goal sentence between primary and suggest -->
            <div v-if="todayGoal" class="zone-section">
              <div class="goal-card">
                <div class="goal-card-content">
                  <span class="goal-card-star">🎯</span>
                  <span v-if="!editingGoalInline" class="goal-card-text" @click="startEditGoalInline">{{ todayGoal }}</span>
                  <input
                    v-else
                    ref="goalInlineInputRef"
                    v-model="editGoalTextInline"
                    class="goal-card-input"
                    @keydown.enter="saveGoalInline"
                    @keydown.escape="cancelEditGoalInline"
                    @blur="saveGoalInline"
                  />
                  <span v-if="!editingGoalInline" class="goal-card-edit-icon" title="编辑今日目标" @click="startEditGoalInline">✎</span>
                </div>
              </div>
            </div>

            <!-- 🔔 军情哨 — suggest analysis, full width between primary and secondary -->
            <div class="zone-section">
              <SuggestCard :suggestions="suggestions" />
            </div>

            <!-- 🚩 支线项目 — 2 cards side by side, each with own title -->
            <div class="zone-section">
              <div class="secondary-grid">
                <SecondaryCard
                  v-for="(group, idx) in secondaryGroups"
                  :key="group ? group.key : 'empty-secondary-' + idx"
                  :group="group"
                  :project-options="projectSelectOptions"
                  :main-session="group ? getMainSession(group) : null"
                  :starred-session-id="group ? getStarredSessionId(group.key) : null"
                  :disabled="projectRootLoading"
                  :index="idx"
                  :highlight="group ? (getGroupScore(group.key)?.highlight ?? 0) : 0"
                  :fog-pct="group ? (getGroupScore(group.key)?.fog_pct ?? 0) : 0"
                  :focus-active="focusActive"
                  :focus-focused-project="focusFocusedProject"
                  :focus-minutes="focusMinutes"
                  :focus-loading="focusLoading"
                  :focus-countdown="focusCountdown"
                  @select-project="(path: string) => handleSelectSecondary(path, idx === 0 ? 'secondary_1' : 'secondary_2')"
                  @star-session="(sid: string) => group && handleStarSession(group.key, sid)"
                  @row-click="openDetail"
                  @open-terminal="handleOpenTerminalFromCard"
                  @focus-extend="(key: string) => focusExtend(key)"
                  @focus-stop="focusStop"
                />
              </div>
            </div>

            <!-- 📁 普通项目 — collapsible, current GroupCard style -->
            <div v-if="normalGroups.length > 0" class="zone-collapse-wrap">
              <div v-if="focusActive" class="focus-overlay" :style="{ opacity: focusDimOpacity }"></div>
            <NCollapse class="zone-collapse" v-model:expanded-names="normalExpanded">
              <NCollapseItem name="normal">
                <template #header>
                  <span class="zone-title">📁 普通项目</span>
                  <span class="zone-count">{{ normalGroups.length }}</span>
                </template>
                <GroupCard
                  v-for="group in normalGroups"
                  :key="group.key"
                  :group="group"
                  :columns="groupColumns"
                  :row-props="rowProps"
                  :disabled="projectRootLoading"
                  :highlight="getGroupScore(group.key)?.highlight ?? 0"
                  :fog-pct="getGroupScore(group.key)?.fog_pct ?? 0"
                  @check="handleCheckChange"
                />
              </NCollapseItem>
            </NCollapse>
            </div>

            <!-- 📂 未归类 — collapsible, collapsed by default -->
            <div v-if="unmatchedGroups.length > 0" class="zone-collapse-wrap">
              <div v-if="focusActive" class="focus-overlay" :style="{ opacity: focusDimOpacity }"></div>
            <NCollapse class="zone-collapse" v-model:expanded-names="unmatchedExpanded">
              <NCollapseItem name="unmatched">
                <template #header>
                  <span class="zone-title">📂 未归类</span>
                  <span class="zone-count">{{ unmatchedGroups.length }}</span>
                </template>
                <GroupCard
                  v-for="group in unmatchedGroups"
                  :key="group.key"
                  :group="group"
                  :columns="groupColumns"
                  :row-props="rowProps"
                  :disabled="projectRootLoading"
                  :highlight="getGroupScore(group.key)?.highlight ?? 0"
                  :fog-pct="getGroupScore(group.key)?.fog_pct ?? 0"
                  @check="handleCheckChange"
                />
              </NCollapseItem>
            </NCollapse>
            </div>

            <!-- All-unmatched hint -->
            <div
              v-if="!primaryGroup && secondaryGroups.every(g => !g) && normalGroups.length === 0 && unmatchedGroups.length > 0"
              class="zone-hint"
            >
              <p>💡 Use the dropdowns in the ⭐ primary / 🚩 secondary cards above to assign projects to priority slots. Or check the ☐ box next to a directory below to mark it as a project root first.</p>
            </div>

          </div>

          <!-- Empty state -->
          <div v-if="!loading && !error && filteredSessions.length === 0" class="empty-state">
            <p>No active sessions found in the selected time window.</p>
            <p class="hint">Try increasing the window or checking if agents are running.</p>
          </div>
        </NSpin>
      </div>
    </NLayoutContent>

    <!-- Footer -->
    <NLayoutFooter bordered>
      <div class="footer">
        <span>🟢 active &nbsp; 🟡 waiting &nbsp; ⚪ idle &nbsp; ⚫ inactive</span>
      </div>
    </NLayoutFooter>

    <!-- Session Detail Drawer -->
    <NDrawer v-model:show="showDetail" :width="drawerWidth" placement="right">
      <NDrawerContent v-if="selectedSession" title="Session Detail" closable>
        <div class="resize-handle" @mousedown="startResize"></div>
        <NDescriptions label-placement="left" :column="1" bordered size="small" label-style="width: 100px; min-width: 100px; white-space: nowrap">
          <NDescriptionsItem label="Session ID">
            <code>{{ selectedSession.session_id }}</code>
          </NDescriptionsItem>
          <NDescriptionsItem label="Agent">
            <NSpace :size="4" align="center">
              <NIcon :size="16">
                <component :is="agentIcon(selectedSession.agent_type)" />
              </NIcon>
              <span>{{ selectedSession.agent_type === 'claude' ? 'Claude Code' : selectedSession.agent_type === 'codex' ? 'Codex' : 'Hermes' }}</span>
            </NSpace>
          </NDescriptionsItem>
          <NDescriptionsItem label="Project">
            {{ selectedSession.project || selectedSession.platform || '?' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Status">
            <NTag :type="trafficColor(selectedSession.traffic_light)" size="small" :bordered="false">
              {{ selectedSession.traffic_light }} {{ selectedSession.status }}
            </NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Active">
            <NTag :type="selectedSession.is_active ? 'success' : 'default'" size="small" bordered>
              {{ selectedSession.is_active ? 'Yes' : 'No' }}
            </NTag>
          </NDescriptionsItem>
          <NDescriptionsItem label="Name">
            {{ selectedSession.name || '—' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Last Active">
            {{ new Date(selectedSession.last_active).toLocaleString() }}
            &nbsp;({{ formatSince(selectedSession.last_active) }})
          </NDescriptionsItem>
          <NDescriptionsItem label="Sum Active">
            {{ formatMinutes(selectedSession.today_minutes) }}
            &nbsp;(est. active today)
          </NDescriptionsItem>
          <NDescriptionsItem label="Last Req">
            <div class="detail-text">{{ selectedSession.last_req_full || selectedSession.last_req || '—' }}</div>
          </NDescriptionsItem>
          <NDescriptionsItem label="Last Resp">
            <div class="detail-text">{{ selectedSession.last_resp_full || selectedSession.last_resp || '—' }}</div>
          </NDescriptionsItem>
          <NDescriptionsItem v-if="selectedSession.platform" label="Platform">
            {{ selectedSession.platform }}
          </NDescriptionsItem>
        </NDescriptions>

        <!-- Terminal Section -->
        <div class="terminal-section" v-if="selectedSession?.agent_type === 'claude' || selectedSession?.agent_type === 'codex'">
          <NSpace>
            <NButton
              size="small"
              type="primary"
              @click="openTerminalModal"
              :loading="terminalLoading || !terminalLookupDone"
              :disabled="terminalLookupDone && !terminalFound"
            >
              🖥 Terminal
            </NButton>
          </NSpace>
          <NAlert v-if="terminalError" type="error" size="tiny" style="margin-top: 8px">
            {{ terminalError }}
          </NAlert>
          <NAlert v-if="terminalFound && !terminalVerified && !terminalError" type="warning" size="tiny" style="margin-top: 8px">
            {{ terminalLookupWarning || 'Unable to verify this tmux session matches the current Claude session — it may have changed.' }}
          </NAlert>
          <div v-if="terminalLookupDone && !terminalFound && !terminalError" class="terminal-placeholder">
            <p>{{ terminalLookupHint || 'No tmux session found' }}</p>
            <p class="terminal-placeholder-hint">
              Start with: <code>pflow {{ selectedSession.agent_type }}</code> in the project directory.
            </p>
          </div>
        </div>
      </NDrawerContent>
    </NDrawer>

    <!-- Terminal Modal -->
    <NModal
      v-model:show="showTerminalModal"
      :mask-closable="false"
      preset="card"
      :style="{ width: '90vw', maxWidth: '1200px' }"
      :title="'🖥 Terminal: ' + (terminalName || '')"
      closable
    >
      <div v-if="terminalUrl" class="terminal-modal-body">
        <iframe :src="terminalUrl" class="terminal-modal-iframe" frameborder="0" title="Web Terminal" />
      </div>
      <div v-else-if="terminalLoading" class="terminal-modal-loading">
        <NSpin /><p>Starting terminal...</p>
      </div>
      <div v-else class="terminal-modal-loading">
        <p>Terminal not available. Click "Open Terminal" to start.</p>
      </div>
    </NModal>

    <!-- 📋 项目管理 — Project management modal -->
    <NModal
      v-model:show="showProjectMgmt"
      preset="card"
      :style="{ width: '620px', maxWidth: '90vw' }"
      title="📋 项目管理"
      closable
    >
      <div class="project-mgmt-list">
        <div v-if="!data || data.project_roots.length === 0" class="project-mgmt-empty">
          没有标记的项目。在下方"未归类"区域勾选 ☐ 识别为项目，或通过下拉框将项目分配到主线/支线 slot 即可自动标记。
        </div>
        <div
          v-for="root in data?.project_roots ?? []"
          :key="root.path"
          class="project-mgmt-item"
        >
          <div class="project-mgmt-info">
            <div class="project-mgmt-name">{{ projectBasename(root.path) }}</div>
            <div class="project-mgmt-path">{{ root.path }}</div>
            <div class="project-mgmt-meta">
              <NTag :type="root.priority === 'primary' ? 'success' : root.priority === 'secondary' ? 'warning' : 'default'" size="tiny" :bordered="false">
                {{ root.priority === 'primary' ? '⭐ 主线' : root.priority === 'secondary' ? '🚩 支线' : '📁 普通' }}
              </NTag>
              <span v-if="getSlotForPath(root.path)" class="project-mgmt-slot">
                slot: {{ getSlotForPath(root.path) }}
              </span>
              <span class="project-mgmt-last">
                最后活跃: {{ getLastActiveForRoot(root.path) }}
              </span>
            </div>
          </div>
          <NButton
            size="tiny"
            type="error"
            quaternary
            :loading="forgetLoading[root.path]"
            @click="forgetProject(root.path)"
          >
            <template #icon>
              <NIcon><TrashOutline /></NIcon>
            </template>
            忘记
          </NButton>
        </div>
      </div>
    </NModal>

    <!-- 🧠 知识锚点 — Knowledge Anchor, fixed bottom-right corner -->
    <KnowledgeAnchor :suggestions="suggestions" />

    </NLayout>
  </div>
</template>

<style scoped>
.layout {
  min-height: 100vh;
}

/* ── Header ─────────────────────────────────── */

:deep(.n-layout-header) {
  position: sticky;
  top: 0;
  z-index: 100;
  background: var(--n-color-target);
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  height: 56px;
  gap: 16px;
}

.header-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
  flex-shrink: 0;
}

.title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.subtitle {
  color: var(--n-text-color-3);
  font-size: 14px;
}

/* Stats in header */
.header-stats {
  display: flex;
  align-items: center;
  gap: 0;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid var(--n-border-color);
}

.header-stat {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: var(--n-color-target);
  border-right: 1px solid var(--n-border-color);
}

.header-stat:last-child {
  border-right: none;
}

.header-stat-value {
  font-size: 16px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.header-stat-label {
  font-size: 11px;
  color: var(--n-text-color-4);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.header-stat--active .header-stat-value {
  color: #18a058;
}

.header-stat--waiting .header-stat-value {
  color: #f0a020;
}

.header-stat--idle .header-stat-value {
  color: #999;
}

.header-right {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

/* ── Content ────────────────────────────────── */

.content {
  padding: 6px 24px;
  max-width: 1400px;
  margin: 0 auto;
}

/* Filters */
.filter-bar {
  margin-bottom: 6px;
  padding: 0px 0px;
  background: var(--n-color-target);
  border-radius: 8px;
}

.filter-label {
  font-size: 13px;
  color: var(--n-text-color-3);
  margin-right: -8px;
}

.filter-note {
  font-size: 12px;
  color: var(--n-text-color-3);
  margin-left: 12px;
}

/* Zone layout */
.groups-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.zone-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.zone-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  border-radius: 8px;
  font-weight: 600;
  font-size: 14px;
}

.zone-primary {
  background: rgba(24, 160, 88, 0.12);
  color: #18a058;
  border: 1px solid rgba(24, 160, 88, 0.3);
}

.zone-secondary {
  background: rgba(240, 160, 32, 0.12);
  color: #f0a020;
  border: 1px solid rgba(240, 160, 32, 0.3);
}

/* Zone section backgrounds */
.zone-section--primary {
  background: rgba(24, 160, 88, 0.06);
  border-radius: 12px;
  padding: 4px;
  border: 1px solid rgba(24, 160, 88, 0.2);
}

/* ── Goal card ──────────────────────────────── */

.goal-card {
  background: rgba(74, 144, 217, 0.08);
  border: 1px solid rgba(74, 144, 217, 0.2);
  border-radius: 10px;
  padding: 14px 20px;
}

.goal-card-content {
  display: flex;
  align-items: center;
  gap: 10px;
}

.goal-card-star {
  color: #4A90D9;
  font-size: 20px;
  flex-shrink: 0;
  line-height: 1;
}

.goal-card-text {
  font-size: 18px;
  color: #c8d6e5;
  cursor: text;
  line-height: 1.5;
  flex: 1;
  min-width: 0;
}

.goal-card-edit-icon {
  color: #555;
  cursor: pointer;
  font-size: 14px;
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.15s;
}

.goal-card-content:hover .goal-card-edit-icon {
  opacity: 1;
}

.goal-card-input {
  flex: 1;
  background: transparent;
  border: none;
  border-bottom: 2px solid #4A90D9;
  color: #c8d6e5;
  font-size: 18px;
  padding: 2px 4px;
  outline: none;
  font-family: inherit;
  line-height: 1.5;
  min-width: 0;
}

.zone-title {
  display: flex;
  align-items: center;
  gap: 6px;
}

.zone-count {
  font-size: 12px;
  opacity: 0.7;
}

.zone-collapse {
  border-radius: 8px;
  overflow: hidden;
}

.zone-collapse .zone-title {
  font-weight: 600;
  font-size: 14px;
}

.zone-collapse .zone-count {
  margin-left: 8px;
}

.zone-hint {
  text-align: center;
  padding: 20px;
  color: var(--n-text-color-4);
  font-size: 13px;
  background: var(--n-color-embedded);
  border-radius: 8px;
  border: 1px dashed var(--n-border-color);
}

/* Secondary grid */
.secondary-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

/* Empty */
.empty-state {
  text-align: center;
  padding: 60px 0;
  color: var(--n-text-color-3);
}

.empty-state .hint {
  font-size: 13px;
  color: var(--n-text-color-4);
}

/* Footer */
:deep(.n-layout-footer) {
  position: sticky;
  bottom: 0;
  z-index: 100;
  background: var(--n-color-target);
}

.footer {
  display: flex;
  justify-content: center;
  gap: 24px;
  padding: 0 24px;
  height: 36px;
  align-items: center;
  font-size: 12px;
  color: var(--n-text-color-4);
}

/* Detail drawer */
.detail-text {
  max-height: 200px;
  overflow-y: auto;
  white-space: pre-wrap;
  font-size: 13px;
  line-height: 1.5;
  color: var(--n-text-color-2);
  word-break: break-all;
}

.resize-handle {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 6px;
  cursor: col-resize;
  z-index: 10;
  transition: background-color 0.15s;
}

.resize-handle:hover {
  background: var(--n-color-target);
}

/* Terminal section */
.terminal-section {
  margin-top: 16px;
  border-top: 1px solid var(--n-border-color);
  padding-top: 12px;
}

.terminal-placeholder {
  padding: 12px;
  background: var(--n-color-embedded);
  border-radius: 6px;
  border: 1px dashed var(--n-border-color);
  color: var(--n-text-color-3);
  font-size: 13px;
  margin-top: 8px;
}

.terminal-placeholder-hint {
  font-size: 11px;
  color: var(--n-text-color-4);
  margin-top: 4px;
}

.terminal-placeholder-hint code {
  background: var(--n-color-embedded);
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 11px;
}

/* Terminal modal */
.terminal-modal-body {
  height: 75vh;
  min-height: 400px;
  background: #1e1e1e;
  border-radius: 4px;
  overflow: hidden;
}

.terminal-modal-iframe {
  width: 100%;
  height: 100%;
  display: block;
  border: none;
}

.terminal-modal-loading {
  height: 200px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--n-text-color-3);
}

/* ── Focus mode dimming ──────────────────────── */

.focus-overlay {
  position: absolute;
  inset: 0;
  z-index: 50;
  pointer-events: none;
  border-radius: inherit;
  background: var(--n-color-target, #18181b);
}

/* Containers that hold focus overlays need positioning */
.header-stats,
.filter-bar,
.zone-collapse-wrap {
  position: relative;
}

.zone-collapse-wrap {
  border-radius: 8px;
  overflow: hidden;
}

/* ── Project management modal ──────────────────── */

.project-mgmt-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 60vh;
  overflow-y: auto;
}

.project-mgmt-empty {
  text-align: center;
  padding: 32px 16px;
  color: var(--n-text-color-4);
  font-size: 13px;
  line-height: 1.6;
}

.project-mgmt-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 6px;
  gap: 12px;
  transition: background 0.15s;
}

.project-mgmt-item:hover {
  background: var(--n-color-embedded);
}

.project-mgmt-info {
  min-width: 0;
  flex: 1;
}

.project-mgmt-name {
  font-weight: 600;
  font-size: 14px;
  color: var(--n-text-color);
}

.project-mgmt-path {
  font-size: 12px;
  color: var(--n-text-color-4);
  font-family: monospace;
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.project-mgmt-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  flex-wrap: wrap;
}

.project-mgmt-slot {
  font-size: 11px;
  color: var(--n-text-color-3);
  font-family: monospace;
}

.project-mgmt-last {
  font-size: 11px;
  color: var(--n-text-color-4);
}
</style>
