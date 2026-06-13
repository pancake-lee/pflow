<script setup lang="ts">
import { computed, h, type Component } from 'vue'
import {
  NDataTable,
  NSelect,
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
  projectOptions: Array<{ label: string; value: string }>
  mainSession: DashboardEntry | null
  disabled: boolean
  index: number
}>()

const emit = defineEmits<{
  selectProject: [path: string]
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

const selectedProject = computed(() => {
  if (!props.group) return null
  return props.group.isRoot ? props.group.fullPath : null
})

function onProjectSelect(path: string) {
  emit('selectProject', path)
}

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

// ── Table columns (no req/resp, narrower) ──────────────────────

const tableColumns: DataTableColumns<DashboardEntry> = [
  {
    title: '',
    key: 'agent',
    width: 36,
    render(row) {
      if (row.session_id === '-') return h('span', '—')
      return h(NIcon, { size: 15 }, { default: () => h(agentIcon(row.agent_type)) })
    },
  },
  {
    title: 'ID',
    key: 'session_id',
    width: 80,
    render(row) {
      return h('span', { style: { fontFamily: 'monospace', fontSize: '10px' } }, row.session_id || '—')
    },
  },
  {
    title: 'Status',
    key: 'status',
    width: 100,
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
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', { style: { fontSize: '12px' } }, escapeNewlines(truncate(row.name, 28)) || '—')
    },
  },
  {
    title: 'Active',
    key: 'last_active',
    width: 70,
    render(row) {
      if (row.session_id === '-') return h('span', '—')
      const since = formatSince(row.last_active)
      const full = new Date(row.last_active).toLocaleString()
      return h(NTooltip, {}, { trigger: () => h('span', { style: { fontSize: '11px' } }, since), default: () => full })
    },
  },
  {
    title: 'Ops',
    key: 'ops',
    width: 60,
    render(row) {
      if (row.session_id === '-') return h('span', '—')
      return h('div', { style: { display: 'flex', gap: '2px', alignItems: 'center' } }, [
        h(NButton, {
          size: 'tiny', quaternary: true, title: 'Set as main session',
          onClick: (e: MouseEvent) => { e.stopPropagation(); emit('setMainSession', row.session_id) },
        }, { default: () => '⭐' }),
        row.has_terminal
          ? h(NButton, {
            size: 'tiny', quaternary: true, title: 'Open terminal',
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
  <div class="secondary-card" :class="{ 'secondary-card--empty': isEmpty }">
    <!-- Header: slot title + dropdown + main session metadata inline -->
    <div class="card-header">
      <div class="card-header-content">
        <!-- Slot title: 🚩 支线项目1 / 🚩 支线项目2 -->
        <span class="slot-title">🚩 支线项目{{ index + 1 }}</span>

        <!-- Project dropdown (replaces text, shows selected project) -->
        <NSelect
          size="tiny"
          :value="selectedProject"
          :options="projectOptions"
          :disabled="disabled"
          placeholder="Assign..."
          clearable
          style="width: 200px"
          @update:value="onProjectSelect"
        />

        <!-- Main session metadata (if available) -->
        <template v-if="mainSession">
          <span class="h-sep">|</span>
          <NIcon :size="14" :component="agentIcon(mainSession.agent_type)" />
          <span class="h-agent">{{ mainSession.agent_type === 'claude' ? 'Claude' : 'Hermes' }}</span>
          <code class="h-sid">{{ mainSession.session_id }}</code>
          <NButton
            v-if="mainSession.has_terminal"
            size="tiny"
            quaternary
            title="Open terminal"
            @click.stop="emit('openTerminal', mainSession)"
          >
            🖥
          </NButton>
          <NTag :type="trafficColor(mainSession.traffic_light)" size="small" :bordered="false">
            {{ mainSession.traffic_light }} {{ mainSession.status }}
          </NTag>
          <span class="h-time">{{ formatSince(mainSession.last_active) }}</span>
        </template>
      </div>
    </div>

    <!-- Body -->
    <template v-if="!isEmpty && mainSession">
      <!-- Name: full version, clickable -->
      <div class="ms-name-row" @click="emit('rowClick', mainSession)">
        <div class="ms-name">{{ mainSession.name || '—' }}</div>
      </div>

      <!-- Last Req -->
      <div class="ms-req-row">
        <div class="ms-req-label">Last Request</div>
        <div class="ms-req-text">{{ mainSession.last_req_full || mainSession.last_req || '—' }}</div>
      </div>

      <!-- Other sessions table -->
      <div class="other-sessions">
        <div class="other-header">{{ otherSessions.length > 0 ? `${otherSessions.length} more` : 'Other sessions' }}</div>
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
      <span>Assign a project</span>
    </div>
  </div>
</template>

<style scoped>
.secondary-card {
  background: rgba(240, 160, 32, 0.05);
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid rgba(240, 160, 32, 0.25);
  transition: border-color 0.2s;
  display: flex;
  flex-direction: column;
}

.secondary-card:hover {
  border-color: rgba(240, 160, 32, 0.45);
}

.secondary-card--empty {
  opacity: 0.7;
  border-style: dashed;
}

/* ── Header ─────────────────────────────────── */

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  gap: 6px;
  border-bottom: 1px solid rgba(240, 160, 32, 0.15);
}

.card-header-content {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
  flex: 1;
  flex-wrap: wrap;
}

.slot-title {
  font-weight: 700;
  font-size: 13px;
  color: #f0a020;
  white-space: nowrap;
  flex-shrink: 0;
}

.h-sep {
  color: var(--n-text-color-4);
  font-size: 11px;
}

.h-agent {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--n-text-color-3);
}

.h-sid {
  font-size: 9px;
  font-family: monospace;
  color: var(--n-text-color-2);
}

.h-time {
  font-size: 10px;
  color: var(--n-text-color-4);
}

/* ── Name row ──────────────────────────────── */

.ms-name-row {
  padding: 8px 12px 6px;
  cursor: pointer;
  transition: background 0.15s;
}

.ms-name-row:hover {
  background: var(--n-color-embedded);
}

.ms-name {
  font-weight: 600;
  font-size: 13px;
  color: var(--n-text-color);
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

/* ── Last Req ──────────────────────────────── */

.ms-req-row {
  padding: 8px 12px;
  border-top: 1px solid rgba(240, 160, 32, 0.12);
}

.ms-req-label {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--n-text-color-4);
  margin-bottom: 4px;
}

.ms-req-text {
  font-size: 12px;
  line-height: 1.5;
  color: var(--n-text-color-2);
  white-space: pre-wrap;
  word-break: break-all;
  min-height: 4.5em;        /* fixed 3 lines */
  max-height: 4.5em;
  overflow-y: auto;
}

/* ── Other sessions ─────────────────────────── */

.other-sessions {
  border-top: 1px solid rgba(240, 160, 32, 0.12);
}

.other-header {
  padding: 6px 12px;
  font-size: 11px;
  font-weight: 600;
  color: var(--n-text-color-4);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* ── Empty ──────────────────────────────────── */

.card-empty {
  padding: 36px 12px;
  text-align: center;
  color: var(--n-text-color-4);
  font-size: 12px;
}
</style>
