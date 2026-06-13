<script setup lang="ts">
import { computed } from 'vue'
import {
  NDataTable,
  NTooltip,
  NCheckbox,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import type { DashboardEntry } from '../types/dashboard'

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
  last_active: new Date().toISOString(),
  last_req: '-',
  last_resp: '-',
  has_terminal: false,
}

const tableData = computed(() =>
  props.group.sessions.length > 0 ? props.group.sessions : [PLACEHOLDER_ROW],
)

function wrappedRowProps(row: DashboardEntry) {
  if (row.session_id === '-') return {}
  return props.rowProps(row)
}
</script>

<template>
  <div class="group-card">
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

    <!-- Group body: session table -->
    <NDataTable
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
  background: var(--n-color-target);
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid var(--n-border-color);
  transition: border-color 0.2s;
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
</style>
