<script setup lang="ts">
import type { Suggestion } from '../types/dashboard'

defineProps<{
  suggestions: Suggestion[]
}>()
</script>

<template>
  <div class="suggest-card">
    <div class="suggest-header">
      <span class="suggest-icon">🔔</span>
      <span class="suggest-title">军情哨</span>
    </div>
    <div class="suggest-body">
      <template v-if="suggestions.length > 0">
        <div
          v-for="(s, idx) in suggestions"
          :key="idx"
          class="suggest-item"
        >
          <span class="suggest-text">{{ s.text }}</span>
        </div>
      </template>
      <div v-else class="suggest-empty">
        ⚪ 暂无更多建议
      </div>
    </div>
  </div>
</template>

<style scoped>
.suggest-card {
  background: rgba(32, 128, 240, 0.08);
  border: 1px solid rgba(32, 128, 240, 0.25);
  border-radius: 12px;
  padding: 14px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.suggest-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 700;
  font-size: 15px;
  color: #2080f0;
}

.suggest-icon {
  font-size: 16px;
}

.suggest-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.suggest-item {
  font-size: 13px;
  line-height: 1.6;
  color: var(--n-text-color-2);
  padding: 6px 10px;
  background: rgba(32, 128, 240, 0.05);
  border-radius: 6px;
  border-left: 3px solid rgba(32, 128, 240, 0.4);
  white-space: pre-line;
}

.suggest-empty {
  font-size: 13px;
  color: var(--n-text-color-4);
  padding: 4px 0;
}
</style>
