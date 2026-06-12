<script setup lang="ts">
import { ref, computed, onMounted, h, type Component } from 'vue'
import {
  NLayout,
  NLayoutHeader,
  NLayoutContent,
  NLayoutFooter,
  NDataTable,
  NTag,
  NSelect,
  NInputNumber,
  NButton,
  NDrawer,
  NDrawerContent,
  NDescriptions,
  NDescriptionsItem,
  NSpace,
  NStatistic,
  NGrid,
  NGi,
  NIcon,
  NSpin,
  NAlert,
  NTooltip,
  NModal,
} from 'naive-ui'
import {
  DesktopOutline,
  RefreshOutline,
  HardwareChipOutline,
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
import { formatSince, truncate, escapeNewlines } from '../composables/format'

// ── State ────────────────────────────────────────────────────────

const { data, loading, error, fetchDashboard } = useDashboard()

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

// Resizable drawer: min 1/4 screen, max 3/4 screen
const drawerWidth = ref(Math.max(480, Math.floor(window.innerWidth / 4)))
const minDrawerWidth = computed(() => Math.floor(window.innerWidth / 4))
const maxDrawerWidth = computed(() => Math.floor(window.innerWidth * 3 / 4))

function startResize(e: MouseEvent) {
  e.preventDefault()
  const startX = e.clientX
  const startWidth = drawerWidth.value

  function onMove(ev: MouseEvent) {
    const delta = startX - ev.clientX // moving left = wider drawer
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

// Terminal modal
const showTerminalModal = ref(false)

function openTerminalModal() {
  if (terminalUrl.value) {
    showTerminalModal.value = true
    return
  }
  // Will start ttyd first, then open modal
  startTerminal()
}

// Open terminal directly from the table column button
async function openTerminalFromTable(row: DashboardEntry) {
  selectedSession.value = row
  showDetail.value = false // don't open drawer

  // Reset terminal state
  terminalUrl.value = null
  terminalError.value = null
  terminalLookupHint.value = null
  terminalLookupWarning.value = null

  // If dashboard API already found a mapping, use it directly
  if (row.has_terminal && row.terminal_tmux_name) {
    terminalFound.value = true
    terminalVerified.value = false
    terminalName.value = row.terminal_tmux_name
    terminalLookupDone.value = true
    await startTerminal() // this auto-opens modal on success
    return
  }

  // Fallback: do a lookup (should not normally happen if backend populated mapping)
  if (row.agent_type === 'claude' && row.session_id) {
    await lookupTerminal(row.session_id)
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

const stats = computed(() => {
  const sessions = data.value?.sessions ?? []
  return {
    total: sessions.length,
    active: sessions.filter((s) => s.is_active).length,
    waiting: sessions.filter((s) => s.status === 'waiting').length,
    idle: sessions.filter((s) => s.status === 'idle').length,
  }
})

// ── Actions ──────────────────────────────────────────────────────

function refresh() {
  fetchDashboard(scanOpts.value)
}

function openDetail(row: DashboardEntry) {
  // Reset terminal state when opening a different session
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

  // Look up matching tmux session for Claude sessions
  if (row.agent_type === 'claude' && row.session_id) {
    lookupTerminal(row.session_id)
  } else {
    terminalLookupDone.value = true
    terminalFound.value = false
  }
}

async function lookupTerminal(sessionId: string) {
  terminalLookupDone.value = false
  terminalFound.value = false
  terminalVerified.value = false
  terminalLookupHint.value = null
  terminalLookupWarning.value = null

  try {
    const resp = await fetch(`/api/v1/terminal/lookup?session_id=${encodeURIComponent(sessionId)}`)
    if (!resp.ok) {
      terminalLookupHint.value = `Lookup failed (HTTP ${resp.status})`
      terminalLookupDone.value = true
      return
    }
    const data: TerminalLookupResponse = await resp.json()
    terminalLookupDone.value = true

    if (data.found) {
      terminalFound.value = true
      terminalVerified.value = data.verified
      terminalName.value = data.tmux_name ?? null
      if (data.warning) {
        terminalLookupWarning.value = data.warning
      }
      // If ttyd is already running, use it
      if (data.ttyd_url) {
        terminalUrl.value = data.ttyd_url
      }
    } else {
      terminalLookupHint.value = data.hint ?? 'No tmux session found for this Claude session'
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
    // If we already found a tmux session via lookup, use its name
    const body: Record<string, string> = { work_dir: workDir }
    if (terminalFound.value && terminalName.value) {
      body['tmux_name'] = terminalName.value
    }

    const resp = await fetch('/api/v1/terminal/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    const data: TerminalResponse = await resp.json()
    if (!resp.ok || data.error) {
      throw new Error(data.error || `HTTP ${resp.status}`)
    }
    terminalUrl.value = data.ttyd_url ?? null
    terminalName.value = data.name ?? null
    terminalFound.value = true
    // Auto-open the terminal modal
    showTerminalModal.value = true
  } catch (e) {
    terminalError.value = e instanceof Error ? e.message : 'Failed to start terminal'
    terminalUrl.value = null
    terminalName.value = null
  } finally {
    terminalLoading.value = false
  }
}

// ── Lifecycle ────────────────────────────────────────────────────

onMounted(() => {
  refresh()
})

usePolling(refresh, refreshInterval)

// ── Table columns ────────────────────────────────────────────────

function agentIcon(agentType: string): Component {
  return agentType === 'claude' ? DesktopOutline : HardwareChipOutline
}

type TagType = 'default' | 'success' | 'warning' | 'error' | 'primary' | 'info'

function trafficColor(light: string): TagType {
  switch (light) {
    case '🟢': return 'success'
    case '🟡': return 'warning'
    case '⚪': return 'default'
    default: return 'default' // ⚫ — no special color
  }
}

const columns: DataTableColumns<DashboardEntry> = [
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
    title: 'Project',
    key: 'project',
    width: 140,
    ellipsis: { tooltip: true },
    sorter: (a, b) => {
      const pa = (a.project || a.platform || '?').toLowerCase()
      const pb = (b.project || b.platform || '?').toLowerCase()
      if (pa < pb) return -1
      if (pa > pb) return 1
      return 0
    },
    render(row) {
      const p = row.project || (row.platform ? row.platform : '?')
      return h('span', p)
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
    width: 160,
    ellipsis: { tooltip: true },
    render(row) {
      const name = escapeNewlines(truncate(row.name, 20))
      return h('span', name || '—')
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
    title: 'Last Req',
    key: 'last_req',
    width: 180,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', escapeNewlines(truncate(row.last_req, 15)) || '—')
    },
  },
  {
    title: 'Last Resp',
    key: 'last_resp',
    width: 180,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', escapeNewlines(truncate(row.last_resp, 15)) || '—')
    },
  },
  {
    title: 'TTY',
    key: 'terminal',
    width: 50,
    render(row) {
      if (row.agent_type !== 'claude' || !row.has_terminal) return null
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

// ── Row props for click ──────────────────────────────────────────

function rowProps(row: DashboardEntry) {
  return {
    style: 'cursor: pointer',
    onClick: () => openDetail(row),
  }
}
</script>

<template>
  <NLayout class="layout">
    <!-- Header -->
    <NLayoutHeader bordered>
      <div class="header">
        <div class="header-left">
          <h1 class="title">⚔️ pflow</h1>
          <span class="subtitle">Agent Activity Dashboard</span>
        </div>
        <div class="header-right">
          <NSpace>
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
          </NSpace>
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

        <!-- Stats cards -->
        <NGrid cols="4" x-gap="12" style="margin-bottom: 16px">
          <NGi>
            <div class="stat-card">
              <NStatistic label="Total" :value="stats.total" />
            </div>
          </NGi>
          <NGi>
            <div class="stat-card stat-active">
              <NStatistic label="🟢 Active" :value="stats.active" />
            </div>
          </NGi>
          <NGi>
            <div class="stat-card stat-waiting">
              <NStatistic label="🟡 Waiting" :value="stats.waiting" />
            </div>
          </NGi>
          <NGi>
            <div class="stat-card stat-idle">
              <NStatistic label="⚪ Idle" :value="stats.idle" />
            </div>
          </NGi>
        </NGrid>

        <!-- Filter bar -->
        <div class="filter-bar">
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

        <!-- Sessions table -->
        <NSpin :show="loading && !data">
          <NDataTable
            :columns="columns"
            :data="filteredSessions"
            :row-props="rowProps"
            :bordered="false"
            :single-line="false"
            size="small"
            :max-height="600"
            virtual-scroll
          />
        </NSpin>

        <!-- Empty state -->
        <div v-if="!loading && !error && filteredSessions.length === 0" class="empty-state">
          <p>No active sessions found in the selected time window.</p>
          <p class="hint">Try increasing the window or checking if agents are running.</p>
        </div>
      </div>
    </NLayoutContent>

    <!-- Footer -->
    <NLayoutFooter bordered>
      <div class="footer">
        <span>{{ stats.total }} sessions</span>
        <span>🟢 busy/running &nbsp; 🟡 waiting &nbsp; ⚪ idle &nbsp; ⚫ inactive</span>
        <span v-if="maxInactive > 0">(inactive limited to {{ maxInactive }} per project)</span>
      </div>
    </NLayoutFooter>

    <!-- Session Detail Drawer -->
    <NDrawer v-model:show="showDetail" :width="drawerWidth" placement="right">
      <NDrawerContent v-if="selectedSession" title="Session Detail" closable>
        <!-- Resize handle on left edge -->
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
              <span>{{ selectedSession.agent_type === 'claude' ? 'Claude Code' : 'Hermes' }}</span>
            </NSpace>
          </NDescriptionsItem>
          <NDescriptionsItem label="Project">
            {{ selectedSession.project || selectedSession.platform || '?' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="Status">
            <NTag
              :type="trafficColor(selectedSession.traffic_light)"
              size="small"
              :bordered="false"
            >
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

        <!-- Terminal Section (Claude only) -->
        <div class="terminal-section" v-if="selectedSession?.agent_type === 'claude'">
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

          <NAlert
            v-if="terminalError"
            type="error"
            size="tiny"
            style="margin-top: 8px"
          >
            {{ terminalError }}
          </NAlert>

          <NAlert
            v-if="terminalFound && !terminalVerified && !terminalError"
            type="warning"
            size="tiny"
            style="margin-top: 8px"
          >
            {{ terminalLookupWarning || 'Unable to verify this tmux session matches the current Claude session — it may have changed.' }}
          </NAlert>

          <div v-if="terminalLookupDone && !terminalFound && !terminalError" class="terminal-placeholder">
            <p>{{ terminalLookupHint || 'No tmux session found' }}</p>
            <p class="terminal-placeholder-hint">
              Start with: <code>pflow claude</code> in the project directory.
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
        <iframe
          :src="terminalUrl"
          class="terminal-modal-iframe"
          frameborder="0"
          title="Web Terminal"
        />
      </div>
      <div v-else-if="terminalLoading" class="terminal-modal-loading">
        <NSpin />
        <p>Starting terminal...</p>
      </div>
      <div v-else class="terminal-modal-loading">
        <p>Terminal not available. Click "Open Terminal" to start.</p>
      </div>
    </NModal>
  </NLayout>
</template>

<style scoped>
.layout {
  min-height: 100vh;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  height: 56px;
}

.header-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
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

.content {
  padding: 16px 24px;
  max-width: 1400px;
  margin: 0 auto;
}

/* Stats */
.stat-card {
  background: var(--n-color-target);
  border-radius: 8px;
  padding: 14px 18px;
}

.stat-active {
  border-left: 3px solid #18a058;
}

.stat-waiting {
  border-left: 3px solid #f0a020;
}

.stat-idle {
  border-left: 3px solid #999;
}

/* Filters */
.filter-bar {
  margin-bottom: 12px;
  padding: 10px 14px;
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

/* Detail drawer text */
.detail-text {
  max-height: 200px;
  overflow-y: auto;
  white-space: pre-wrap;
  font-size: 13px;
  line-height: 1.5;
  color: var(--n-text-color-2);
  word-break: break-all;
}

/* Resizable drawer handle */
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

/* Terminal section in sidebar */
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
</style>
