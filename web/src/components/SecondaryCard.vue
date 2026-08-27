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
  projectOptions: Array<{ label: string; value: string }>
  mainSession: DashboardEntry | null
  starredSessionId: string | null
  disabled: boolean
  index: number
  highlight?: number
  fogPct?: number
  focusActive?: boolean
  focusFocusedProject?: string
  focusMinutes?: number
  focusLoading?: boolean
  focusCountdown?: string
}>()

const emit = defineEmits<{
  selectProject: [path: string]
  starSession: [sessionId: string]
  rowClick: [row: DashboardEntry]
  openTerminal: [row: DashboardEntry]
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

const isFocusedProject = computed(() =>
  props.focusActive && props.focusFocusedProject !== '' && props.group?.fullPath === props.focusFocusedProject,
)

/** Whether this card should be dimmed during focus mode. */
const focusDimmed = computed(() => props.focusActive && !isFocusedProject.value)
const focusDimOpacity = computed(() => FOCUS_CONFIG.dimOpacity)

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
    title: 'Sum',
    key: 'today_minutes',
    width: 60,
    render(row) {
      if (row.session_id === '-') return h('span', '—')
      const mins = formatMinutes(row.today_minutes)
      const tooltip = `${row.today_minutes?.toFixed(1) ?? '0'} min estimated active today`
      return h(NTooltip, {}, { trigger: () => h('span', { style: { fontSize: '11px', fontVariantNumeric: 'tabular-nums' } }, mins), default: () => tooltip })
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
          size: 'tiny',
          quaternary: true,
          title: row.session_id === props.starredSessionId ? '取消星标' : starTooltip,
          onClick: (e: MouseEvent) => { e.stopPropagation(); emit('starSession', row.session_id) },
        }, { default: () => row.session_id === props.starredSessionId ? '🌟' : '⭐' }),
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
  <div class="secondary-card" :class="{ 'secondary-card--empty': isEmpty }" :style="{ ...hlStyle, ...fogStyle }">
    <!-- Focus mode dimming overlay (when focus is on a different project) -->
    <div v-if="focusDimmed" class="focus-overlay" :style="{ opacity: focusDimOpacity }"></div>
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

        <span class="h-sep">|</span>
        <NTooltip>
          <template #trigger>
            <span class="h-project-total">⏱ {{ formatMinutes(projectTotalMinutes) }}</span>
          </template>
          {{ projectTotalMinutes.toFixed(1) }} min today
        </NTooltip>

        <!-- Main session metadata (if available) -->
        <template v-if="mainSession">
          <span class="h-sep">|</span>
          <NIcon :size="14" :component="agentIcon(mainSession.agent_type)" />
          <span class="h-agent">{{ mainSession.agent_type === 'claude' ? 'Claude' : mainSession.agent_type === 'codex' ? 'Codex' : 'Hermes' }}</span>
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

    <!-- No project assigned to this slot -->
    <div v-else-if="!group" class="card-empty">
      <span>Assign a project</span>
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
.secondary-card {
  position: relative;
  background: rgba(240, 160, 32, 0.05);
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid rgba(240, 160, 32, 0.25);
  transition: border-color 0.2s;
  display: flex;
  flex-direction: column;
}

/* ── Fog overlay (::before) ──────────────────── */

.secondary-card::before {
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

.secondary-card:hover::before {
  opacity: calc(var(--fog-opacity, 0) * 0.3);
}

/* ── Highlight marquee (::after) ─────────────── */

.secondary-card::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: conic-gradient(
    from var(--hl-angle, 0deg),
    transparent 0deg,
    rgba(240, 160, 32, 1) 12deg,
    transparent 24deg,
    transparent 78deg,
    rgba(240, 160, 32, 1) 90deg,
    transparent 102deg,
    transparent 168deg,
    rgba(240, 160, 32, 1) 180deg,
    transparent 192deg,
    transparent 258deg,
    rgba(240, 160, 32, 1) 270deg,
    transparent 282deg,
    transparent 348deg,
    transparent 360deg
  );
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

.h-project-total {
  font-size: 11px;
  font-weight: 600;
  color: #f0a020;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  cursor: default;
}

.h-time {
  font-size: 10px;
  color: var(--n-text-color-4);
}

.h-focus-label {
  font-size: 10px;
  font-weight: 600;
  color: #f0a020;
}

.h-focus-countdown {
  font-size: 10px;
  font-weight: 600;
  color: #f0a020;
  font-variant-numeric: tabular-nums;
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

.ms-name--empty {
  color: var(--n-text-color-4);
  font-style: italic;
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
