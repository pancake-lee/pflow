<script setup lang="ts">
import { computed, h, type Component } from 'vue'
import {
  NDataTable,
  NTooltip,
  NButton,
  NTag,
  NIcon,
} from 'naive-ui'
import { DesktopOutline, HardwareChipOutline } from '@vicons/ionicons5'
import type { DataTableColumns } from 'naive-ui'
import type { DashboardEntry } from '../types/dashboard'
import type { SessionGroup } from './GroupCard.vue'
import { formatSince, truncate, escapeNewlines } from '../composables/format'

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

const props = defineProps<{
  group: SessionGroup | null
  mainSession: DashboardEntry | null
  disabled: boolean
}>()

const emit = defineEmits<{
  setMainSession: [sessionId: string]
  rowClick: [row: DashboardEntry]
  openTerminal: [row: DashboardEntry]
}>()

const otherSessions = computed(() => {
  if (!props.group) return []
  const main = props.mainSession
  if (!main) return props.group.sessions
  return props.group.sessions.filter(s => s.session_id !== main.session_id)
})

// Empty placeholder row for table stability
const PLACEHOLDER_ROW: DashboardEntry = {
  session_id: '-',
  agent_type: 'claude',
  project: '-',
  status: '-',
  is_active: false,
  traffic_light: '⚪',
  name: '-',
  last_active: new Date().toISOString(),
  last_req: '-',
  last_resp: '-',
  has_terminal: false,
}

const tableData = computed(() => {
  if (!props.group) return [PLACEHOLDER_ROW]
  if (otherSessions.value.length > 0) return otherSessions.value
  return [PLACEHOLDER_ROW]
})

const isEmpty = computed(() => !props.group || props.group.sessions.length === 0)

// ── Table columns with last_req/last_resp ──

const tableColumns: DataTableColumns<DashboardEntry> = [
  {
    title: '',
    key: 'agent',
    width: 40,
    render(row) {
      if (row.session_id === '-') return h('span', '—')
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
      if (row.session_id === '-') return h('span', '—')
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
      if (row.session_id === '-') return h('span', '—')
      const since = formatSince(row.last_active)
      const full = new Date(row.last_active).toLocaleString()
      return h(NTooltip, {}, { trigger: () => h('span', since), default: () => full })
    },
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
    title: 'Ops',
    key: 'ops',
    width: 70,
    render(row) {
      if (row.session_id === '-') return h('span', '—')
      return h('div', { style: { display: 'flex', gap: '2px', alignItems: 'center' } }, [
        h(NButton, {
          size: 'tiny',
          quaternary: true,
          title: 'Set as main session',
          onClick: (e: MouseEvent) => { e.stopPropagation(); emit('setMainSession', row.session_id) },
        }, { default: () => '⭐' }),
        row.has_terminal
          ? h(NButton, {
            size: 'tiny',
            quaternary: true,
            title: 'Open terminal',
            onClick: (e: MouseEvent) => { e.stopPropagation(); emit('openTerminal', row) },
          }, { default: () => '🖥' })
          : null,
      ])
    },
  },
]

function rowProps(row: DashboardEntry) {
  if (row.session_id === '-') return {}
  return { style: 'cursor: pointer', onClick: () => emit('rowClick', row) }
}
</script>

<template>
  <div class="primary-card" :class="{ 'primary-card--empty': isEmpty }">
    <template v-if="!isEmpty && mainSession">
      <!-- Name: full version, clickable -->
      <div class="ms-name-row" @click="emit('rowClick', mainSession)">
        <div class="ms-name">{{ mainSession.name || '—' }}</div>
      </div>

      <!-- Last Req full width -->
      <div class="ms-req-row">
        <div class="ms-req-label">Last Request</div>
        <div class="ms-req-text">{{ mainSession.last_req_full || mainSession.last_req || '—' }}</div>
      </div>

      <!-- Other sessions table -->
      <div class="other-sessions">
        <div class="other-header">
          {{ otherSessions.length > 0 ? `Other sessions (${otherSessions.length})` : 'Other sessions' }}
        </div>
        <NDataTable
          :columns="tableColumns"
          :data="tableData"
          :row-props="rowProps"
          :bordered="false"
          :single-line="false"
          size="small"
        />
      </div>
    </template>

    <!-- Empty placeholder -->
    <div v-else class="card-empty">
      <span>Assign a project to this slot using the dropdown above</span>
    </div>
  </div>
</template>

<style scoped>
.primary-card {
  background: transparent;
  overflow: hidden;
}

.primary-card--empty {
  opacity: 0.6;
}

/* ── Name row ──────────────────────────────── */

.ms-name-row {
  padding: 12px 20px 8px;
  cursor: pointer;
  transition: background 0.15s;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.ms-name-row:hover {
  background: rgba(255, 255, 255, 0.04);
}

.ms-name {
  font-weight: 600;
  font-size: 14px;
  color: var(--n-text-color);
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

/* ── Last Req ──────────────────────────────── */

.ms-req-row {
  padding: 12px 20px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.ms-req-label {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--n-text-color-4);
  margin-bottom: 6px;
}

.ms-req-text {
  font-size: 13px;
  line-height: 1.5;
  color: var(--n-text-color-2);
  white-space: pre-wrap;
  word-break: break-all;
  min-height: 4.5em;        /* fixed 3 lines */
  max-height: 4.5em;
  overflow-y: auto;
}

/* ── Other sessions ─────────────────────────── */

.other-header {
  padding: 8px 20px;
  font-size: 11px;
  font-weight: 600;
  color: var(--n-text-color-4);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* ── Empty ──────────────────────────────────── */

.card-empty {
  padding: 40px 16px;
  text-align: center;
  color: var(--n-text-color-4);
  font-size: 13px;
}
</style>
