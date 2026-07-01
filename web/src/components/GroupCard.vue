<script setup lang="ts">
import { computed } from 'vue'
import {
  NDataTable,
  NTooltip,
  NCheckbox,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import type { DashboardEntry } from '../types/dashboard'
import { highlightToMarquee, fogPctToOpacity, FOG_CONFIG } from '../composables/useReminderScores'

type Priority = 'primary' | 'secondary' | 'normal'

export interface SessionGroup {
  key: string
  basename: string
  fullPath: string
  sessions: DashboardEntry[]
  hasActive: boolean
  hasWaiting: boolean
  lastActive: number
  isRoot: boolean
  priority: Priority | null
}

const props = defineProps<{
  group: SessionGroup
  columns: DataTableColumns<DashboardEntry>
  rowProps: (row: DashboardEntry) => Record<string, unknown>
  disabled: boolean
  highlight?: number
  fogPct?: number
}>()

const emit = defineEmits<{
  check: [group: SessionGroup, checked: boolean]
}>()

function onCheckChange(checked: boolean) {
  emit('check', props.group, checked)
}

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

const tableData = computed(() =>
  props.group.sessions.length > 0 ? props.group.sessions : [PLACEHOLDER_ROW],
)

function wrappedRowProps(row: DashboardEntry) {
  if (row.session_id === '-') return {}
  return props.rowProps(row)
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
</script>

<template>
  <div class="group-card" :style="{ ...hlStyle, ...fogStyle }">
    <!-- Group header -->
    <div class="group-header">
      <div class="group-header-left">
        <span
          class="group-icon"
        >
          {{ group.hasActive ? '🟢' : group.hasWaiting ? '🟡' : '📁' }}
        </span>
        <NTooltip placement="bottom">
          <template #trigger>
            <span class="group-name">{{ group.basename }}</span>
          </template>
          {{ group.fullPath }}
        </NTooltip>
        <span class="group-path-hint">{{ group.fullPath }}</span>
      </div>
      <div class="group-header-right">
        <span class="group-count">{{ group.sessions.length }} session{{ group.sessions.length > 1 ? 's' : '' }}</span>

        <!-- Checkbox: "识别为项目" -->
        <NTooltip placement="top">
          <template #trigger>
            <NCheckbox
              :checked="group.isRoot"
              :disabled="disabled"
              size="small"
              @update:checked="onCheckChange"
            >
              <span class="checkbox-label">识别为项目</span>
            </NCheckbox>
          </template>
          将该目录标记为项目根，其子目录下的所有 session 都将归类到此项目下
        </NTooltip>
      </div>
    </div>

    <!-- Group body: session table or empty placeholder -->
    <div v-if="group.sessions.length === 0" class="card-empty">
      <span>没有活动session</span>
    </div>
    <NDataTable
      v-else
      :columns="columns"
      :data="tableData"
      :row-props="wrappedRowProps"
      :bordered="false"
      :single-line="false"
      size="small"
    />
  </div>
</template>

<style scoped>
.group-card {
  position: relative;
  background: var(--n-color-target);
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid var(--n-border-color);
  transition: border-color 0.2s;
}

/* ── Fog overlay (::before) ──────────────────── */

.group-card::before {
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

.group-card:hover::before {
  opacity: calc(var(--fog-opacity, 0) * 0.3);
}

/* ── Highlight marquee (::after) ─────────────── */

.group-card::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: conic-gradient(
    from var(--hl-angle, 0deg),
    transparent 0deg,
    rgba(64, 128, 255, 1) 12deg,
    transparent 24deg,
    transparent 78deg,
    rgba(64, 128, 255, 1) 90deg,
    transparent 102deg,
    transparent 168deg,
    rgba(64, 128, 255, 1) 180deg,
    transparent 192deg,
    transparent 258deg,
    rgba(64, 128, 255, 1) 270deg,
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

.group-card:hover {
  border-color: var(--n-color-target-3);
}

.group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: var(--n-color-embedded);
  border-bottom: 1px solid var(--n-border-color);
  gap: 12px;
}

.group-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.group-header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.group-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.group-name {
  font-weight: 600;
  font-size: 15px;
  color: var(--n-text-color);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: default;
}

.group-path-hint {
  font-size: 12px;
  color: var(--n-text-color-4);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.group-count {
  font-size: 12px;
  color: var(--n-text-color-4);
  white-space: nowrap;
}

.checkbox-label {
  font-size: 12px;
  color: var(--n-text-color-3);
}

.card-empty {
  padding: 40px 16px;
  text-align: center;
  color: var(--n-text-color-4);
  font-size: 13px;
}
</style>
