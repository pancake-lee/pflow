<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import type { Suggestion, KnowledgeTip } from '../types/dashboard'
import { ALL_TIPS, getAssociatedTips, selectKnowledgeTips, shuffleKnowledgeTips } from '../config/knowledge-tips'

const props = defineProps<{
  suggestions: Suggestion[]
  customTips?: KnowledgeTip[]
}>()

const isHovered = ref(false)
const rotationIndex = ref(0)
const manualPause = ref(false)
let rotationTimer: ReturnType<typeof setInterval> | null = null
let manualPauseTimer: ReturnType<typeof setTimeout> | null = null

// Stable across polling responses that contain the same associated knowledge.
const candidateKey = computed(() => JSON.stringify({
  associated: getAssociatedTips(props.suggestions),
  builtIn: ALL_TIPS,
  custom: props.customTips ?? [],
}))
const rotationTips = ref<KnowledgeTip[]>([])
const visibleTip = computed(() => rotationTips.value[rotationIndex.value] ?? null)
const visibleTipKind = computed(() => {
  const id = visibleTip.value?.id
  if (id && props.suggestions.some((suggestion) => suggestion.knowledge_tip?.id === id)) {
    return { emoji: '🔗', label: '行动建议关联知识' }
  }
  if (id && props.customTips?.some((tip) => tip.id === id)) {
    return { emoji: '✍️', label: '自定义知识' }
  }
  return { emoji: '📚', label: '内置知识' }
})
const modeLabel = computed(() => '📖 知识库')
const totalPages = computed(() => rotationTips.value.length)
const currentPage = computed(() => totalPages.value ? rotationIndex.value + 1 : 0)

function stopRotation() {
  if (rotationTimer !== null) clearInterval(rotationTimer)
  rotationTimer = null
}

function startRotation() {
  stopRotation()
  if (rotationTips.value.length < 2) return
  rotationTimer = setInterval(() => {
    rotationIndex.value = (rotationIndex.value + 1) % rotationTips.value.length
  }, 15000)
}

function pauseRotation() {
  stopRotation()
  manualPause.value = true
  if (manualPauseTimer !== null) clearTimeout(manualPauseTimer)
  manualPauseTimer = setTimeout(() => {
    manualPauseTimer = null
    manualPause.value = false
    startRotation()
  }, 30000)
}

function goNext() {
  if (rotationTips.value.length < 2) return
  rotationIndex.value = (rotationIndex.value + 1) % rotationTips.value.length
  pauseRotation()
}

function goPrev() {
  if (rotationTips.value.length < 2) return
  rotationIndex.value = (rotationIndex.value - 1 + rotationTips.value.length) % rotationTips.value.length
  pauseRotation()
}

watch(candidateKey, () => {
  const selected = selectKnowledgeTips(props.suggestions, ALL_TIPS, props.customTips ?? [])
  rotationTips.value = shuffleKnowledgeTips(selected)
  rotationIndex.value = 0
  if (!manualPause.value) startRotation()
}, { immediate: true })
onMounted(startRotation)
onUnmounted(() => {
  stopRotation()
  if (manualPauseTimer !== null) clearTimeout(manualPauseTimer)
})
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
      <span class="ka-controls" :class="{ 'is-hidden': !isHovered && !manualPause }">
        <button
          class="ka-nav-btn"
          @click="goPrev"
          :disabled="totalPages < 2"
          title="上一条"
        >
          ◀
        </button>
        <button
          class="ka-nav-btn"
          :disabled="totalPages < 2"
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
      <div class="ka-body" :key="visibleTip.id" v-if="visibleTip">
        <div class="ka-title"><span :title="visibleTipKind.label" role="img" :aria-label="visibleTipKind.label">{{ visibleTipKind.emoji }}</span> {{ visibleTip.title }}</div>
        <div class="ka-theory">{{ visibleTip.theory }}</div>
        <div class="ka-design" v-if="visibleTip.design">
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
  min-width: 0;
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
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.25);
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
  flex-shrink: 0;
}

/* Reserve the controls' natural height even while they are hidden. */
.ka-controls.is-hidden {
  visibility: hidden;
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
