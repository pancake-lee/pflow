<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'

const emit = defineEmits<{
  complete: [goal: string]
  skip: []
}>()

// ── Phase state ──────────────────────────────────────────────────

type Phase = 'boot' | 'ritual'
const phase = ref<Phase>('boot')
const transitioning = ref(false)

// ── Act 1: Boot state ────────────────────────────────────────────

const goal = ref('')
const bootStartTime = Date.now()
const bootDuration = 30 * 60 * 1000 // 30 minutes
const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  timer = setInterval(() => { now.value = Date.now() }, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const progressPct = computed(() => {
  const elapsed = now.value - bootStartTime
  return Math.min(100, Math.round((elapsed / bootDuration) * 100))
})

const goToRitual = () => {
  transitioning.value = true
  setTimeout(() => {
    phase.value = 'ritual'
    transitioning.value = false
  }, 300)
}

// ── Act 2: Ritual state ─────────────────────────────────────────

const checks = ref([false, false, false])

const checkItems = [
  '我已倒好一杯水/咖啡，放在手边。',
  '我已打开主线任务所需的全部窗口。',
  '我已关闭微信、企微及其他无关应用。',
]

const allChecked = computed(() => checks.value.every(c => c))

const confirmRitual = () => {
  if (!allChecked.value) return
  transitioning.value = true
  setTimeout(() => {
    emit('complete', goal.value.trim())
  }, 400)
}

const skipBoot = () => {
  emit('skip')
}
</script>

<template>
  <div class="daily-boot" :class="{ 'fade-out': transitioning }">
    <!-- ── Act 1: Boot ───────────────────────────── -->
    <div v-if="phase === 'boot'" class="boot-act boot-act--1">
      <div class="act-content">
        <h1 class="greeting">早安。</h1>
        <p class="subtitle">先花些时间清理杂务，为今日定调。</p>

        <div class="goal-input-wrap">
          <input
            v-model="goal"
            type="text"
            class="goal-input"
            placeholder="今天只有一件事："
            @keydown.enter="goToRitual"
          />
        </div>

        <a class="next-link" @click="goToRitual">
          我已处理完杂务，确定今日目标 →
        </a>
      </div>

      <!-- Progress bar -->
      <div class="progress-section">
        <div class="progress-track">
          <div class="progress-fill" :style="{ width: progressPct + '%' }"></div>
        </div>
        <div class="progress-info">
          <span class="progress-pct">{{ progressPct }}%</span>
        </div>
        <a class="skip-link" @click="skipBoot">跳过引导 →</a>
      </div>
    </div>

    <!-- ── Act 2: Ritual ─────────────────────────── -->
    <div v-if="phase === 'ritual'" class="boot-act boot-act--2">
      <div class="act-content">
        <p class="ritual-subtitle">亲赴前线前，检查行装。</p>

        <div class="checklist">
          <label
            v-for="(item, idx) in checkItems"
            :key="idx"
            class="check-item"
            :class="{ checked: checks[idx] }"
          >
            <div class="check-box" @click="checks[idx] = !checks[idx]">
              <span v-if="checks[idx]" class="check-mark">✓</span>
            </div>
            <span class="check-label">{{ item }}</span>
          </label>
        </div>

        <button
          class="confirm-btn"
          :class="{ active: allChecked }"
          :disabled="!allChecked"
          :title="allChecked ? '' : '请先完成三项准备'"
          @click="confirmRitual"
        >
          确认，进入战局 →
        </button>

        <a class="skip-link skip-link--ritual" @click="skipBoot">直接进入 Dashboard</a>
      </div>
    </div>
  </div>
</template>

<style scoped>
.daily-boot {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #1a1a1c;
  background: linear-gradient(180deg, #1c1c1e 0%, #1a1a1c 100%);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  color: #e0e0e0;
  transition: opacity 0.3s ease;
}

.daily-boot.fade-out {
  opacity: 0;
}

.boot-act {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
  max-width: 560px;
  padding: 0 32px;
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}

.act-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
}

/* ── Act 1 ─────────────────────────────────────── */

.greeting {
  font-size: 32px;
  font-weight: 300;
  margin: 0 0 12px;
  color: #e8e8e8;
  letter-spacing: 1px;
}

.subtitle {
  font-size: 15px;
  color: #888;
  margin: 0 0 40px;
  line-height: 1.6;
}

.goal-input-wrap {
  width: 100%;
  margin-bottom: 24px;
}

.goal-input {
  width: 100%;
  background: transparent;
  border: none;
  border-bottom: 2px solid #333;
  color: #ccc;
  font-size: 18px;
  padding: 12px 4px;
  outline: none;
  transition: border-color 0.2s;
  font-family: inherit;
}

.goal-input::placeholder {
  color: #555;
  font-style: italic;
}

.goal-input:focus {
  border-bottom-color: #4A90D9;
}

.next-link {
  color: #666;
  font-size: 14px;
  cursor: pointer;
  transition: color 0.2s;
  text-decoration: none;
}

.next-link:hover {
  color: #aaa;
}

/* ── Progress bar ──────────────────────────────── */

.progress-section {
  margin-top: 48px;
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.progress-track {
  flex: 1;
  height: 4px;
  background: #2a2a2c;
  border-radius: 2px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: #8BB8D6;
  border-radius: 2px;
  transition: width 1s linear;
}

.progress-info {
  flex-shrink: 0;
}

.progress-pct {
  font-size: 12px;
  color: #555;
  font-variant-numeric: tabular-nums;
  min-width: 36px;
  display: inline-block;
}

.skip-link {
  font-size: 13px;
  color: #444;
  cursor: pointer;
  text-decoration: none;
  white-space: nowrap;
  transition: color 0.2s;
}

.skip-link:hover {
  color: #888;
}

/* ── Act 2 ─────────────────────────────────────── */

.boot-act--2 .act-content {
  gap: 0;
}

.ritual-subtitle {
  font-size: 18px;
  color: #bbb;
  margin: 0 0 40px;
  font-weight: 300;
}

.checklist {
  display: flex;
  flex-direction: column;
  gap: 18px;
  width: 100%;
  margin-bottom: 40px;
}

.check-item {
  display: flex;
  align-items: center;
  gap: 14px;
  cursor: pointer;
  padding: 6px 0;
  user-select: none;
}

.check-box {
  width: 22px;
  height: 22px;
  border: 2px solid #555;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.2s ease;
}

.check-item.checked .check-box {
  border-color: #4A90D9;
  background: #4A90D9;
}

.check-mark {
  color: #1a1a1c;
  font-size: 14px;
  font-weight: 700;
  animation: checkPop 0.2s ease;
}

@keyframes checkPop {
  0% { transform: scale(0); }
  60% { transform: scale(1.2); }
  100% { transform: scale(1); }
}

.check-label {
  font-size: 15px;
  color: #aaa;
  line-height: 1.4;
  transition: color 0.2s;
}

.check-item.checked .check-label {
  color: #ddd;
}

.confirm-btn {
  padding: 10px 32px;
  border: 1px solid #444;
  border-radius: 6px;
  background: transparent;
  color: #555;
  font-size: 15px;
  cursor: default;
  transition: all 0.25s ease;
  font-family: inherit;
}

.confirm-btn.active {
  border-color: #4A90D9;
  color: #4A90D9;
  cursor: pointer;
}

.confirm-btn.active:hover {
  background: rgba(74, 144, 217, 0.1);
}

.skip-link--ritual {
  margin-top: 20px;
}
</style>
