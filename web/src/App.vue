<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  darkTheme,
  dateZhCN,
  zhCN,
  NConfigProvider,
  NMessageProvider,
  NModal,
} from 'naive-ui'
import DashboardView from './views/DashboardView.vue'
import DailyBootView from './views/DailyBootView.vue'
import SettingsView from './views/SettingsView.vue'
import ScheduleEditor from './components/ScheduleEditor.vue'
import DesktopConnection from './components/DesktopConnection.vue'
import type { DailyBootResponse } from './types/dashboard'

// ── Daily Boot routing ──────────────────────────────────────────

const showBoot = ref(false)
const bootChecked = ref(false) // true once API check completes
const todayGoal = ref('')
const showSettings = ref(false)
const settingsView = ref<{ requestClose: () => boolean } | null>(null)
const showScheduleEditor = ref(false)
const scheduleEditor = ref<{ requestClose: () => boolean } | null>(null)
const dashboardKey = ref(0) // bumped when settings are saved, so the dashboard re-reads them
const isDesktop = import.meta.env.VITE_PFLOW_DESKTOP === '1'

function updateScheduleVisibility(show: boolean) {
  if (show) showScheduleEditor.value = true
  else scheduleEditor.value?.requestClose()
}

function updateSettingsVisibility(show: boolean) {
  if (show) showSettings.value = true
  else settingsView.value?.requestClose()
}

async function checkBoot() {
  try {
    const settingsResp = await fetch('/api/v1/settings')
    const settings = settingsResp.ok ? await settingsResp.json() as { dashboard?: { daily_boot_enabled?: boolean } } : null
    if (settings?.dashboard?.daily_boot_enabled === false) {
      return
    }
    const resp = await fetch('/api/v1/daily-boot')
    if (resp.ok) {
      const data: DailyBootResponse = await resp.json()
      if (!data.completed) {
        showBoot.value = true
      }
      todayGoal.value = data.goal
    }
  } catch {
    // API not available — skip boot, show dashboard directly
  } finally {
    bootChecked.value = true
  }
}

onMounted(() => {
  if (!isDesktop) void checkBoot()
})

async function onBootComplete(goal: string) {
  try {
    await fetch('/api/v1/daily-boot', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ goal }),
    })
  } catch {
    // Silently fail — don't block the user
  }
  todayGoal.value = goal
  showBoot.value = false
}

async function onBootSkip() {
  try {
    await fetch('/api/v1/daily-boot', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ goal: '' }),
    })
  } catch {
    // Silently fail
  }
  showBoot.value = false
}
</script>

<template>
  <NConfigProvider :theme="darkTheme" :locale="zhCN" :date-locale="dateZhCN">
    <NMessageProvider>
      <DesktopConnection v-if="isDesktop" @ready="checkBoot" />
      <!-- Daily Boot: shown when today's boot hasn't been completed -->
      <DailyBootView
        v-if="showBoot"
        @complete="onBootComplete"
        @skip="onBootSkip"
      />
      <!-- Dashboard: show after boot check completes and boot is not needed -->
      <template v-else-if="bootChecked">
        <DashboardView :key="dashboardKey" :initial-goal="todayGoal" @open-settings="showSettings = true" @open-schedule="showScheduleEditor = true" />
        <NModal
          :show="showSettings"
          @update:show="updateSettingsVisibility"
          preset="card"
          title="设置"
          :style="{ width: 'min(760px, calc(100vw - 32px))' }"
          :mask-closable="false"
          closable
        >
          <SettingsView ref="settingsView" @close="showSettings = false" @saved="dashboardKey++" />
        </NModal>
        <NModal
          :show="showScheduleEditor"
          @update:show="updateScheduleVisibility"
          preset="card"
          title="今日日程编辑"
          :style="{ width: 'min(960px, calc(100vw - 32px))', maxHeight: 'calc(100vh - 48px)', overflowY: 'auto' }"
          :mask-closable="false"
          closable
        >
          <ScheduleEditor ref="scheduleEditor" @close="showScheduleEditor = false" @saved="dashboardKey++" />
        </NModal>
      </template>
      <!-- Loading state while checking boot status -->
      <div v-else class="boot-loading" />
    </NMessageProvider>
  </NConfigProvider>
</template>

<style>
/* ── Shared highlight marquee animation ──────── */

@property --hl-angle {
  syntax: '<angle>';
  initial-value: 0deg;
  inherits: false;
}

@keyframes hl-marquee {
  to { --hl-angle: 360deg; }
}

/* ── Boot loading placeholder ────────────────── */

.boot-loading {
  min-height: 100vh;
  background: #1a1a1c;
}
</style>
