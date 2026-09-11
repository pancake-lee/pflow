<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { NButton } from 'naive-ui'
import { highlightToMarquee } from '../composables/useReminderScores'

type Item = { id: string; start: string; title: string; focus_minutes: number; break_minutes: number; status?: string }

const day = ref('')
const current = ref<Item | null>(null)
const next = ref<Item | null>(null)
const phase = ref('free')
const fog = ref(1)
const reminder = ref(0)
const emit = defineEmits<{ edit: [] }>()
let refreshTimer: ReturnType<typeof setInterval> | null = null

async function load() {
  const response = await fetch('/api/v1/schedules')
  if (!response.ok) return
  const payload = await response.json()
  day.value = payload.day.date
  current.value = payload.current
  next.value = payload.next
  phase.value = payload.phase
  fog.value = payload.fog
  reminder.value = payload.reminder_score
}

async function act(action: 'completed' | 'skipped') {
  if (!current.value?.id) return
  const response = await fetch('/api/v1/schedules/action', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ date: day.value, id: current.value.id, action }),
  })
  if (response.ok) await load()
}

// Break drives the current card's orange marquee at full intensity.
const currentStyle = computed(() => {
  const m = highlightToMarquee(phase.value === 'break' ? 100 : 0)
  if (!m.visible) return {} as Record<string, string | number>
  return {
    '--hl-speed': m.speed + 's',
    '--hl-width': m.width + 'px',
    '--hl-opacity': m.opacity,
  } as Record<string, string | number>
})

// Reminder score drives the next card's marquee; fog score drives its blur
// and haze (sharp and clear as the start time approaches).
const nextStyle = computed(() => {
  const style: Record<string, string | number> = {
    '--fog-blur': (fog.value * 3.5).toFixed(2) + 'px',
    '--next-opacity': 0.6 + (1 - fog.value) * 0.4,
  }
  const m = highlightToMarquee(reminder.value)
  if (m.visible) {
    style['--hl-speed'] = m.speed + 's'
    style['--hl-width'] = m.width + 'px'
    style['--hl-opacity'] = m.opacity
  }
  return style
})

onMounted(() => {
  void load()
  refreshTimer = setInterval(() => void load(), 30_000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <section v-if="current" class="schedule">
    <div
      class="s-card s-card--current"
      :class="{ 's-card--break': phase === 'break' }"
      :style="currentStyle"
    >
      <div class="s-head">
        <span class="s-label">今日日程 · 当前</span>
        <div class="s-actions">
          <NButton size="small" quaternary @click="emit('edit')">编辑</NButton>
          <template v-if="current.id">
            <NButton size="small" @click="act('completed')">完成</NButton>
            <NButton size="small" @click="act('skipped')">跳过</NButton>
          </template>
        </div>
      </div>
      <div class="s-body">
        <span class="s-title">{{ phase === 'break' ? '☕ 休息中：' : '' }}{{ current.title }}</span>
        <small class="s-meta">{{ current.id ? `${current.start} · ${current.focus_minutes} 分钟` : '自由安排时间' }}</small>
      </div>
    </div>
    <div v-if="next" class="s-card s-card--next" :style="nextStyle">
      <div class="s-head">
        <span class="s-label">下一项</span>
      </div>
      <div class="s-body">
        <span class="s-title">{{ next.title }}</span>
        <small class="s-meta">{{ next.start }} · {{ next.focus_minutes }} 分钟</small>
      </div>
    </div>
  </section>
</template>

<style scoped>
.schedule { display: flex; flex-direction: column; gap: 8px; margin: 16px 0; }
.s-card { position: relative; display: flex; flex-direction: column; gap: 4px; padding: 10px 16px; border: 1px solid rgba(94, 163, 240, .35); border-radius: 10px; background: rgba(32, 128, 240, .08); }
.s-card--next { background: rgba(32, 128, 240, .04); }
.s-card--break { --hl-stroke: rgba(240, 160, 32, 1); border-color: rgba(240, 160, 32, .5); }

/* ── Highlight marquee (::after), same language as project cards ── */
.s-card::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: conic-gradient(
    from var(--hl-angle, 0deg),
    transparent 0deg,
    var(--hl-stroke, rgba(24, 160, 88, 1)) 12deg,
    transparent 24deg,
    transparent 78deg,
    var(--hl-stroke, rgba(24, 160, 88, 1)) 90deg,
    transparent 102deg,
    transparent 168deg,
    var(--hl-stroke, rgba(24, 160, 88, 1)) 180deg,
    transparent 192deg,
    transparent 258deg,
    var(--hl-stroke, rgba(24, 160, 88, 1)) 270deg,
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

/* ── Card content ─────────────────────────────── */
.s-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.s-label { font-size: 11px; color: #8bbdff; letter-spacing: .5px; }
.s-actions { display: flex; gap: 8px; align-items: center; }
.s-body { display: flex; align-items: baseline; gap: 10px; flex-wrap: wrap; }
.s-title { color: #8bbdff; font-weight: 600; }
.s-card--next .s-title { color: #ccc; }
.s-meta { color: #aaa; }

/* ── Next card fog: blur + haze from 迷雾分, hover to peek ── */
.s-card--next .s-body {
  filter: blur(var(--fog-blur, 0px));
  opacity: var(--next-opacity, 1);
  transition: filter 0.5s ease, opacity 0.5s ease;
}
.s-card--next:hover .s-body {
  filter: blur(0);
  opacity: 1;
}

@media (max-width: 760px) { .s-head { flex-direction: column; align-items: flex-start; } }
</style>
