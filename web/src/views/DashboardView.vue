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
} from '../types/dashboard'
import { useDashboard } from '../composables/useDashboard'
import { usePolling } from '../composables/usePolling'
import { formatSince, truncate, escapeNewlines, shortID } from '../composables/format'

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
  selectedSession.value = row
  showDetail.value = true
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
    width: 140,
    render(row) {
      return h(
        NTooltip,
        {},
        { trigger: () => h('span', { style: { fontFamily: 'monospace', fontSize: '12px' } }, shortID(row.session_id)), default: () => row.session_id },
      )
    },
  },
  {
    title: 'Project',
    key: 'project',
    width: 140,
    ellipsis: { tooltip: true },
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
        <NDescriptions label-placement="left" :column="1" bordered size="small">
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
      </NDrawerContent>
    </NDrawer>
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
</style>
