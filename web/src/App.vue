<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  darkTheme,
  dateZhCN,
  zhCN,
  NConfigProvider,
  NMessageProvider,
} from 'naive-ui'
import DashboardView from './views/DashboardView.vue'
import DailyBootView from './views/DailyBootView.vue'
import type { DailyBootResponse } from './types/dashboard'

// ── Daily Boot routing ──────────────────────────────────────────

const showBoot = ref(false)
const bootChecked = ref(false) // true once API check completes
const todayGoal = ref('')

onMounted(async () => {
  try {
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
  }
  bootChecked.value = true
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
      <!-- Daily Boot: shown when today's boot hasn't been completed -->
      <DailyBootView
        v-if="showBoot"
        @complete="onBootComplete"
        @skip="onBootSkip"
      />
      <!-- Dashboard: show after boot check completes and boot is not needed -->
      <DashboardView v-else-if="bootChecked" :initial-goal="todayGoal" />
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
