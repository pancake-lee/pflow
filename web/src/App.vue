<script setup lang="ts">
import {
  darkTheme,
  dateZhCN,
  zhCN,
  NConfigProvider,
  NMessageProvider,
} from 'naive-ui'
import DashboardView from './views/DashboardView.vue'
</script>

<template>
  <NConfigProvider :theme="darkTheme" :locale="zhCN" :date-locale="dateZhCN">
    <NMessageProvider>
      <DashboardView />
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
</style>
