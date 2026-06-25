<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import type { Suggestion, KnowledgeTip } from '../types/dashboard'
import { GENERIC_TIPS } from '../config/knowledge-tips'

const props = defineProps<{
  suggestions: Suggestion[]
}>()

// ── State ──────────────────────────────────────────────────────────

const isHovered = ref(false)
const rotationIndex = ref(0)
const manualPause = ref(false)
const currentTipId = ref<string>('default') // for transition key
let rotationTimer: ReturnType<typeof setInterval> | null = null
let manualPauseTimer: ReturnType<typeof setTimeout> | null = null

// ── Derived ────────────────────────────────────────────────────────

// Highest-priority suggestion (array is sorted by priority ascending)
const topSuggestion = computed<Suggestion | null>(() => {
  if (props.suggestions.length === 0) return null
  return props.suggestions[0]
})

// Whether we are in "associated" mode (following a suggestion's tip)
const associatedTip = computed<KnowledgeTip | null>(() => {
  const top = topSuggestion.value
  if (top && top.knowledge_tip) {
    return top.knowledge_tip
  }
  return null
})

// The list of tips to rotate through when not associated
// Uses generic tips (those without scenario association)
const rotationTips = computed(() => {
  return GENERIC_TIPS.map((t) => ({
    id: t.id,
    title: t.title,
    theory: t.theory,
    design: t.design,
  }))
})

// The currently visible tip
const visibleTip = computed(() => {
  if (associatedTip.value) {
    return associatedTip.value
  }
  if (rotationTips.value.length === 0) return null
  return rotationTips.value[rotationIndex.value % rotationTips.value.length]
})

// Display mode label
const modeLabel = computed(() => {
  if (associatedTip.value) return '📡 关联军情'
  return '📖 知识库'
})

// Total pages for rotation
const totalPages = computed(() => Math.max(rotationTips.value.length, 1))
const currentPage = computed(() => (rotationIndex.value % totalPages.value) + 1)

// ── Rotation logic ─────────────────────────────────────────────────

function startRotation() {
  stopRotation()
  rotationTimer = setInterval(() => {
    rotationIndex.value++
  }, 15000)
}

function stopRotation() {
  if (rotationTimer) {
    clearInterval(rotationTimer)
    rotationTimer = null
  }
}

function pauseRotation() {
  stopRotation()
  // Resume after 30s of no manual interaction
  if (manualPauseTimer) clearTimeout(manualPauseTimer)
  manualPauseTimer = setTimeout(() => {
    manualPause.value = false
    if (!associatedTip.value) {
      startRotation()
    }
  }, 30000)
}

function goNext() {
  rotationIndex.value++
  pauseRotation()
}

function goPrev() {
  rotationIndex.value = Math.max(0, rotationIndex.value - 1)
  pauseRotation()
}

// ── Lifecycle ──────────────────────────────────────────────────────

onMounted(() => {
  if (!associatedTip.value) {
    startRotation()
  }
})

onUnmounted(() => {
  stopRotation()
  if (manualPauseTimer) clearTimeout(manualPauseTimer)
})

// Watch for changes in associated tip
watch(
  () => associatedTip.value?.id ?? null,
  (newId, oldId) => {
    if (newId !== oldId) {
      currentTipId.value = newId ?? 'rotation'
    }
    if (newId) {
      // Associated mode: stop rotation
      stopRotation()
    } else if (!manualPause.value) {
      // No associated tip, resume rotation
      startRotation()
    }
  }
)

// Watch top suggestion scenario change to trigger animation
watch(
  () => topSuggestion.value?.scenario_id,
  (newSid, oldSid) => {
    if (newSid !== oldSid) {
      currentTipId.value = newSid ?? 'rotation'
    }
  }
)
</script>

<template>
  <div
    class="knowledge-anchor"
    :class="{ 'is-hovered': isHovered }"
    @mouseenter="isHovered = true"
    @mouseleave="isHovered = false"
  >
    <!-- Header -->
    <div class="ka-header">
      <span class="ka-mode-label">{{ modeLabel }}</span>
      <span class="ka-controls" v-if="isHovered || manualPause">
        <button
          class="ka-nav-btn"
          @click="goPrev"
          :disabled="rotationIndex === 0"
          title="上一条"
        >
          ◀
        </button>
        <button
          class="ka-nav-btn"
          @click="goNext"
          title="下一条"
        >
          ▶
        </button>
        <span class="ka-page">{{ currentPage }}/{{ totalPages }}</span>
      </span>
    </div>

    <!-- Content -->
    <Transition name="ka-fade" mode="out-in">
      <div class="ka-body" :key="currentTipId" v-if="visibleTip">
        <div class="ka-title">🧠 {{ visibleTip.title }}</div>
        <div class="ka-theory">{{ visibleTip.theory }}</div>
        <div class="ka-design">
          <span class="ka-design-icon">⚙️</span>
          <span>{{ visibleTip.design }}</span>
        </div>
      </div>
      <div class="ka-body ka-empty" v-else :key="'empty'">
        <div class="ka-title">知识锚点</div>
        <div class="ka-theory">暂无相关知识内容。</div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.knowledge-anchor {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 100;
  width: 320px;
  min-height: 80px;
  max-height: 160px;
  overflow-y: auto;
  background: rgba(15, 23, 42, 0.75);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: background 0.3s, border-color 0.3s;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.3);
}

.knowledge-anchor.is-hovered {
  background: rgba(15, 23, 42, 0.88);
  border-color: rgba(255, 255, 255, 0.18);
}

/* ── Header ─────────────────────────────────────────────────────── */

.ka-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}

.ka-mode-label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.5);
  font-weight: 500;
}

.ka-controls {
  display: flex;
  align-items: center;
  gap: 4px;
}

.ka-nav-btn {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: rgba(255, 255, 255, 0.7);
  border-radius: 4px;
  padding: 2px 6px;
  font-size: 11px;
  cursor: pointer;
  transition: background 0.2s, color 0.2s;
}

.ka-nav-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.15);
  color: rgba(255, 255, 255, 0.9);
}

.ka-nav-btn:disabled {
  opacity: 0.3;
  cursor: default;
}

.ka-page {
  font-size: 10px;
  color: rgba(255, 255, 255, 0.35);
  margin-left: 4px;
  min-width: 28px;
  text-align: right;
}

/* ── Body ───────────────────────────────────────────────────────── */

.ka-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.ka-title {
  font-size: 14px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.9);
  line-height: 1.3;
}

.ka-theory {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.55);
  line-height: 1.5;
}

.ka-design {
  font-size: 13px;
  color: #5ea3f0;
  line-height: 1.5;
  display: flex;
  gap: 4px;
}

.ka-design-icon {
  flex-shrink: 0;
  font-size: 12px;
  margin-top: 1px;
}

.ka-empty {
  opacity: 0.5;
}

/* ── Transition ─────────────────────────────────────────────────── */

.ka-fade-enter-active,
.ka-fade-leave-active {
  transition: opacity 0.4s ease;
}

.ka-fade-enter-from,
.ka-fade-leave-to {
  opacity: 0;
}
</style>
