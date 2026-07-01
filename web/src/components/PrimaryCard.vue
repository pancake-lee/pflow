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
import { formatSince, formatMinutes, truncate, escapeNewlines } from '../composables/format'
import { highlightToMarquee, fogPctToOpacity, FOG_CONFIG, FOCUS_CONFIG } from '../composables/useReminderScores'
import { STAR_BONUS_MINUTES } from '../config/attention'

const starTooltip = `星标后只要闲置时间比其他对话不长于 ${STAR_BONUS_MINUTES}min 即可保持主要对话`

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
  starredSessionId: string | null
  disabled: boolean
  highlight?: number
  fogPct?: number
  projectOptions?: Array<{ label: string; value: string }>
  focusActive?: boolean
  focusFocusedProject?: string
  focusLoading?: boolean
  focusCountdown?: string
}>()

const emit = defineEmits<{
  starSession: [sessionId: string]
  rowClick: [row: DashboardEntry]
  openTerminal: [row: DashboardEntry]
  selectProject: [path: string]
  focusExtend: [projectKey: string]
  focusStop: []
}>()

const isMainStarred = computed(() =>
  !!props.mainSession && props.mainSession.session_id === props.starredSessionId,
)

/** Cumulative focus minutes across all sessions in this project group. */
const projectTotalMinutes = computed(() => {
  if (!props.group) return 0
  return props.group.sessions.reduce((sum, s) => sum + (s.today_minutes ?? 0), 0)
})

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
  first_active: new Date().toISOString(),
  last_active: new Date().toISOString(),
  last_req: '-',
  last_resp: '-',
  today_minutes: 0,
  has_terminal: false,
}

const tableData = computed(() => {
  if (!props.group) return [PLACEHOLDER_ROW]
  if (otherSessions.value.length > 0) return otherSessions.value
  return [PLACEHOLDER_ROW]
})

const isEmpty = computed(() => !props.group || props.group.sessions.length === 0)

const selectedProject = computed(() => {
  if (!props.group) return null
  return props.group.isRoot ? props.group.fullPath : null
})

const isFocusedProject = computed(() =>
  props.focusActive && props.focusFocusedProject !== '' && props.group?.fullPath === props.focusFocusedProject,
)

/** Whether this card should be dimmed during focus mode. */
const focusDimmed = computed(() => props.focusActive && !isFocusedProject.value)
const focusDimOpacity = computed(() => FOCUS_CONFIG.dimOpacity)

function onProjectSelect(path: string) {
  emit('selectProject', path)
}

const marquee = computed(() => highlightToMarquee(props.highlight ?? 0))
const fogOpacity = computed(() => fogPctToOpacity(props.fogPct ?? 0))

const hlStyle = computed(() => {
  const m = marquee.value
  if (!m.visible) return {} as Record<string, string | number>
  return {
    '--hl-speed': m.speed + 's',
    '--hl-width': m.width + 'px',
    '--hl-opacity': m.opacity,
  } as Record<string, string | number>
})

const fogStyle = computed(() => {
  const opacity = fogOpacity.value
  if (opacity <= 0 && !FOG_CONFIG.maskImage) return {} as Record<string, string | number>
  return {
    '--fog-opacity': opacity,
    '--fog-image': FOG_CONFIG.maskImage ? `url(${FOG_CONFIG.maskImage})` : 'none',
  } as Record<string, string | number>
})

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
    title: 'Sum',
    key: 'today_minutes',
    width: 70,
    render(row) {
      if (row.session_id === '-') return h('span', '—')
      const mins = formatMinutes(row.today_minutes)
      const tooltip = `${row.today_minutes?.toFixed(1) ?? '0'} min estimated active today`
      return h(NTooltip, {}, { trigger: () => h('span', { style: { fontVariantNumeric: 'tabular-nums' } }, mins), default: () => tooltip })
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
          title: row.session_id === props.starredSessionId ? '取消星标' : starTooltip,
          onClick: (e: MouseEvent) => { e.stopPropagation(); emit('starSession', row.session_id) },
        }, { default: () => row.session_id === props.starredSessionId ? '🌟' : '⭐' }),
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
  <div class="primary-card" :class="{ 'primary-card--empty': isEmpty }" :style="{ ...hlStyle, ...fogStyle }">
    <!-- Focus mode dimming overlay (when focus is on a different project) -->
    <div v-if="focusDimmed" class="focus-overlay" :style="{ opacity: focusDimOpacity }"></div>
    <!-- Header: zone title + dropdown + main session metadata + focus controls -->
    <div class="card-header">
      <div class="card-header-content">
        <span class="zone-title">⭐ 主线项目</span>
        <NSelect
          size="tiny"
          :value="selectedProject"
          :options="projectOptions ?? []"
          :disabled="disabled"
          placeholder="Assign..."
          clearable
          style="width: 220px"
          @update:value="onProjectSelect"
        />
        <span class="h-sep">|</span>
        <NTooltip>
          <template #trigger>
            <span class="h-project-total">⏱ {{ formatMinutes(projectTotalMinutes) }}</span>
          </template>
          {{ projectTotalMinutes.toFixed(1) }} min today
        </NTooltip>
        <template v-if="mainSession">
          <span class="h-sep">|</span>
          <NIcon :size="16" :component="agentIcon(mainSession.agent_type)" />
          <span class="h-agent">{{ mainSession.agent_type === 'claude' ? 'Claude' : 'Hermes' }}</span>
          <code class="h-sid">{{ mainSession.session_id }}</code>
          <NButton
            size="tiny"
            quaternary
            :title="isMainStarred ? '取消星标' : starTooltip"
            @click.stop="emit('starSession', mainSession.session_id)"
          >
            {{ isMainStarred ? '🌟' : '⭐' }}
          </NButton>
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
          <span class="h-time">last {{ formatSince(mainSession.last_active) }} | sum {{ formatMinutes(mainSession.today_minutes) }}</span>
        </template>
        <!-- Focus controls -->
        <span class="h-sep">|</span>
        <NButton size="tiny" quaternary @click.stop="emit('focusExtend', group?.fullPath ?? '')" :loading="focusLoading">🎯 专注 +15min</NButton>
        <template v-if="isFocusedProject">
          <NButton size="tiny" quaternary @click.stop="emit('focusStop')" :loading="focusLoading">退出专注</NButton>
          <span class="h-focus-countdown">⏱ {{ focusCountdown }}</span>
        </template>
      </div>
    </div>

    <!-- Body -->
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

    <!-- No project assigned to this slot -->
    <div v-else-if="!group" class="card-empty">
      <span>Assign a project to this slot using the dropdown above</span>
    </div>

    <!-- Project assigned but no active sessions -->
    <template v-else>
      <div class="ms-name-row">
        <div class="ms-name ms-name--empty">没有活动session</div>
      </div>
      <div class="ms-req-row">
        <div class="ms-req-label">Last Request</div>
        <div class="ms-req-text">—</div>
      </div>
      <div class="other-sessions">
        <div class="other-header">Other sessions (0)</div>
        <NDataTable
          :columns="tableColumns"
          :data="[PLACEHOLDER_ROW]"
          :bordered="false"
          :single-line="false"
          size="small"
        />
      </div>
    </template>
  </div>
</template>

<style scoped>
.primary-card {
  position: relative;
  background: transparent;
  overflow: hidden;
}

/* ── Fog overlay (::before) ──────────────────── */

.primary-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: var(--fog-image, none) 0 0 / cover no-repeat, var(--pflow-fog-bg, #18181b);
  opacity: var(--fog-opacity, 0);
  pointer-events: none;
  z-index: 5;
  transition: opacity 0.5s ease;
}

.primary-card:hover::before {
  opacity: calc(var(--fog-opacity, 0) * 0.3);
}

/* ── Highlight marquee (::after) ─────────────── */

.primary-card::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: conic-gradient(
    from var(--hl-angle, 0deg),
    transparent 0deg,
    rgba(24, 160, 88, 1) 12deg,
    transparent 24deg,
    transparent 78deg,
    rgba(24, 160, 88, 1) 90deg,
    transparent 102deg,
    transparent 168deg,
    rgba(24, 160, 88, 1) 180deg,
    transparent 192deg,
    transparent 258deg,
    rgba(24, 160, 88, 1) 270deg,
    transparent 282deg,
    transparent 348deg,
    transparent 360deg
  );
  /* Mask: only expose the border strip */
  mask:
    linear-gradient(#fff 0 0) content-box,
    linear-gradient(#fff 0 0);
  mask-composite: exclude;
  -webkit-mask-composite: xor;
  padding: var(--hl-width, 2px);
  animation: hl-marquee var(--hl-speed, 3s) linear infinite;
  pointer-events: none;
  z-index: 10;
  opacity: var(--hl-opacity, 0);
  transition: opacity 0.3s ease;
}

.primary-card--empty {
  opacity: 0.6;
}

/* ── Header ─────────────────────────────────── */

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  gap: 12px;
  border-bottom: 1px solid rgba(24, 160, 88, 0.15);
}

.card-header-content {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  flex: 1;
  flex-wrap: wrap;
}

.zone-title {
  font-weight: 700;
  font-size: 14px;
  color: #18a058;
  white-space: nowrap;
  flex-shrink: 0;
}

.h-sep {
  color: var(--n-text-color-4);
  font-size: 12px;
}

.h-agent {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--n-text-color-3);
}

.h-sid {
  font-size: 10px;
  font-family: monospace;
  color: var(--n-text-color-2);
}

.h-project-total {
  font-size: 12px;
  font-weight: 600;
  color: #18a058;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  cursor: default;
}

.h-time {
  font-size: 11px;
  color: var(--n-text-color-4);
}

.h-focus-countdown {
  font-size: 11px;
  font-weight: 600;
  color: #f0a020;
  font-variant-numeric: tabular-nums;
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

.ms-name--empty {
  color: var(--n-text-color-4);
  font-style: italic;
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

/* ── Focus mode overlay ─────────────────────── */

.focus-overlay {
  position: absolute;
  inset: 0;
  z-index: 60;
  pointer-events: none;
  border-radius: inherit;
  background: var(--n-color-target, #18181b);
}
</style>
