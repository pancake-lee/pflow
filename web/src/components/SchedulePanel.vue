<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { NAlert, NButton, NInput, useMessage } from 'naive-ui'
import { highlightToMarquee } from '../composables/useReminderScores'

type Item = { id: string; start: string; day_offset?: number; title: string; focus_minutes: number; break_minutes: number; status?: string }
type Event = { id: string; task_id?: string; time: string; action: string; duration_seconds?: number; note?: string }
type ReviewSummary = { score: number; completed_count: number; deviation_count: number; total_duration_seconds: number; incomplete_legacy_items: boolean }
type ParkingItem = { id: string; content: string; created_at: string; processed_at?: string }

const day = ref('')
const current = ref<Item | null>(null)
const next = ref<Item | null>(null)
const phase = ref('free')
const fog = ref(1)
const reminder = ref(0)
const breakRemainingMinutes = ref(0)
const loadError = ref('')
const acting = ref(false)
const items = ref<Item[]>([])
const events = ref<Event[]>([])
const runtimeOrder = ref<string[]>([])
const showMore = ref(false)
const note = ref('')
const now = ref(Date.now())
const review = ref<ReviewSummary | null>(null)
const reviewNote = ref('')
const showReview = ref(false)
const savingReview = ref(false)
const parking = ref<ParkingItem[]>([])
const parkingText = ref('')
const parkingOpen = ref(false)
const parkingBusy = ref(false)
const activeLeave = ref<Event | null>(null)
const leaveOpen = ref(false)
const leaveIntent = ref('')
const leaveRemaining = ref(10)
const emit = defineEmits<{ edit: [] }>()
const message = useMessage()
let refreshTimer: ReturnType<typeof setInterval> | null = null
let clockTimer: ReturnType<typeof setInterval> | null = null
let leaveTimer: ReturnType<typeof setInterval> | null = null

async function load() {
  try {
    const response = await fetch('/api/v1/schedules')
    if (!response.ok) throw new Error()
    const payload = await response.json()
    day.value = payload.day.date
    current.value = payload.current
    next.value = payload.next
    phase.value = payload.phase
    fog.value = payload.fog
    reminder.value = payload.reminder_score
    breakRemainingMinutes.value = payload.break_remaining_minutes ?? 0
    items.value = payload.day.items ?? []
    events.value = payload.day.events ?? []
    runtimeOrder.value = payload.day.runtime_order ?? []
    review.value = payload.review ?? null
    parking.value = payload.day.parking ?? []
    activeLeave.value = payload.active_leave ?? null
    if (!showReview.value) reviewNote.value = payload.day.review_note ?? ''
    loadError.value = ''
  } catch {
    loadError.value = '读取今日日程失败，操作暂不可用。'
  }
}

async function addParking() {
  const content = parkingText.value.trim()
  if (!day.value || !content || parkingBusy.value) return
  parkingBusy.value = true
  try {
    const response = await fetch('/api/v1/schedules/parking', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date: day.value, content }),
    })
    if (!response.ok) throw new Error('收纳失败')
    parkingText.value = ''
    await load()
  } catch {
    message.error('收纳失败，请重试')
  } finally {
    parkingBusy.value = false
  }
}

async function processParking(id: string) {
  if (!day.value || parkingBusy.value) return
  parkingBusy.value = true
  try {
    const response = await fetch('/api/v1/schedules/parking', {
      method: 'PATCH', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date: day.value, id }),
    })
    if (!response.ok) throw new Error('更新失败')
    await load()
  } catch {
    message.error('更新停车场失败，请重试')
  } finally {
    parkingBusy.value = false
  }
}

async function saveReview() {
  if (!day.value || savingReview.value) return
  savingReview.value = true
  try {
    const response = await fetch('/api/v1/schedules/review', {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date: day.value, note: reviewNote.value }),
    })
    if (!response.ok) throw new Error('保存对账失败')
    message.success('已保存明日调整点')
    await load()
  } catch {
    message.error('保存对账失败，请重试')
  } finally {
    savingReview.value = false
  }
}

async function act(action: string, id = current.value?.id, actionNote = '') {
  if (!id || acting.value || loadError.value) return
  acting.value = true
  try {
    const options = {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date: day.value, id, action, note: actionNote }),
    }
    const response = await fetch('/api/v1/schedules/action', options)
    if (!response.ok) throw new Error('更新日程状态失败，请重试')
    await load()
    if (action === 'note') note.value = ''
    if (action === 'leave') {
      leaveOpen.value = false
      leaveIntent.value = ''
      stopLeaveTimer()
    }
  } catch {
    message.error('更新日程状态失败，请重试')
  } finally {
    acting.value = false
  }
}

function stopLeaveTimer() {
  if (leaveTimer) clearInterval(leaveTimer)
  leaveTimer = null
}

function openLeave() {
  leaveIntent.value = ''
  leaveRemaining.value = 10
  leaveOpen.value = true
  stopLeaveTimer()
  leaveTimer = setInterval(() => {
    leaveRemaining.value = Math.max(0, leaveRemaining.value - 1)
    if (leaveRemaining.value === 0) stopLeaveTimer()
  }, 1_000)
}

function closeLeave() {
  leaveOpen.value = false
  stopLeaveTimer()
}

const pending = computed(() => {
  const byID = new Map(items.value.map(item => [item.id, item]))
  const ordered = runtimeOrder.value.length
    ? [...runtimeOrder.value.map(id => byID.get(id)).filter((item): item is Item => !!item), ...items.value.filter(item => !runtimeOrder.value.includes(item.id))]
    : items.value
  return ordered.filter(item => !item.status && item.id !== current.value?.id && item.id !== next.value?.id)
})

const elapsed = computed(() => {
  const id = current.value?.id
  if (!id) return ''
  for (let i = events.value.length - 1; i >= 0; i--) {
    const event = events.value[i]
    if (event.task_id !== id) continue
    if (event.action === 'completed' || event.action === 'skipped') return ''
    if (event.action === 'start') {
      const seconds = Math.max(0, Math.floor((now.value - Date.parse(event.time)) / 1000))
      return `已用 ${Math.floor(seconds / 60)} 分钟`
    }
  }
  return ''
})

const actionLabel: Record<string, string> = {
  start: '开始', completed: '完成', skipped: '跳过',
  early: '提前', reorder: '调序', delay: '拖延', note: '备注',
  park: '收纳', parking_processed: '处理停车场',
  leave: '暂离', return: '返回',
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

function itemTime(item: Item) { return `${item.day_offset === 1 ? '次日 ' : ''}${item.start}` }

onMounted(() => {
  void load()
  refreshTimer = setInterval(() => void load(), 30_000)
  clockTimer = setInterval(() => { now.value = Date.now() }, 1_000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (clockTimer) clearInterval(clockTimer)
  stopLeaveTimer()
})
</script>

<template>
  <section v-if="current || loadError" class="schedule">
    <NAlert v-if="loadError" type="error">{{ loadError }}</NAlert>
    <div
      v-if="current"
      class="s-card s-card--current"
      :class="{ 's-card--break': phase === 'break' }"
      :style="currentStyle"
    >
      <div class="s-head">
        <span class="s-label">今日日程 · 当前</span>
        <div class="s-actions">
          <NButton size="small" quaternary @click="emit('edit')">编辑</NButton>
          <NButton v-if="items.length" size="small" quaternary @click="parkingOpen = !parkingOpen">停车场</NButton>
          <template v-if="current.id">
            <NButton size="small" :disabled="acting || !!elapsed || !!loadError || !!activeLeave" @click="act('start')">开始</NButton>
            <NButton size="small" :loading="acting" :disabled="!!loadError || !!activeLeave" @click="act('completed')">完成</NButton>
            <NButton size="small" :disabled="acting || !!loadError || !!activeLeave" @click="act('skipped')">跳过</NButton>
            <NButton size="small" quaternary :disabled="acting || !!loadError || !!activeLeave" @click="act('delay')">拖延</NButton>
            <NButton size="small" quaternary :disabled="acting || !!loadError || !!activeLeave" @click="openLeave">暂离</NButton>
          </template>
        </div>
      </div>
      <div class="s-body">
        <span class="s-title">{{ phase === 'break' ? `☕ 休息中（约 ${breakRemainingMinutes} 分钟）：` : '' }}{{ current.title }}</span>
        <small class="s-meta">{{ current.id ? `${itemTime(current)} · ${current.focus_minutes} 分钟` : '自由安排时间' }}</small>
        <small v-if="elapsed" class="s-meta">{{ elapsed }}</small>
      </div>
    </div>
    <div v-if="leaveOpen" class="s-leave">
      <div>现在切去做什么？写一句即可暂离；也可以等待 {{ leaveRemaining }} 秒。</div>
      <NInput v-model:value="leaveIntent" size="small" placeholder="切换意图或返回线索，可留空" maxlength="200" />
      <div class="s-extra-row">
        <NButton size="small" :disabled="(!leaveIntent.trim() && leaveRemaining > 0) || acting" @click="act('leave', current?.id, leaveIntent)">确认暂离</NButton>
        <NButton size="small" quaternary @click="closeLeave">取消</NButton>
      </div>
    </div>
    <div v-if="activeLeave" class="s-leave">
      <div>暂离中 · 回来继续：{{ items.find(item => item.id === activeLeave?.task_id)?.title ?? '当前日程' }}</div>
      <div v-if="activeLeave.note">切换意图：{{ activeLeave.note }}</div>
      <NButton size="small" :disabled="acting" @click="act('return', activeLeave.task_id)">我回来了</NButton>
    </div>
    <div v-if="parkingOpen && items.length" class="s-parking">
      <div class="s-extra-row">
        <NInput v-model:value="parkingText" size="small" placeholder="先记下这件事，继续当前项" maxlength="300" @keyup.enter="addParking" />
        <NButton size="small" :disabled="!parkingText.trim() || parkingBusy" @click="addParking">收纳</NButton>
      </div>
      <div v-for="entry in parking" :key="entry.id" class="s-parking-item">
        <span :class="{ 's-parking-done': !!entry.processed_at }">{{ entry.content }}</span>
        <NButton v-if="!entry.processed_at" size="tiny" quaternary :disabled="parkingBusy" @click="processParking(entry.id)">已处理</NButton>
      </div>
    </div>
    <div v-if="next" class="s-card s-card--next" :style="nextStyle">
      <div class="s-head">
        <span class="s-label">下一项</span>
        <NButton v-if="current?.id" size="small" quaternary :disabled="acting || !!loadError || !!activeLeave" @click="act('completed', next.id)">提前完成</NButton>
      </div>
      <div class="s-body">
        <span class="s-title">{{ next.title }}</span>
        <small class="s-meta">{{ itemTime(next) }} · {{ next.focus_minutes }} 分钟</small>
      </div>
    </div>
    <div v-if="current?.id || events.length" class="s-extra">
      <NButton size="tiny" quaternary @click="showMore = !showMore">{{ showMore ? '收起' : '更多操作与日志' }}</NButton>
      <div v-if="showMore" class="s-extra-content">
        <div v-if="pending.length" class="s-extra-row">
          <span>调序到：</span>
          <NButton v-for="item in pending" :key="item.id" size="tiny" :disabled="acting || !!activeLeave" @click="act('reorder', item.id)">{{ item.title }}</NButton>
        </div>
        <div v-if="current?.id" class="s-extra-row">
          <NInput v-model:value="note" size="small" placeholder="可选备注" maxlength="200" />
          <NButton size="tiny" :disabled="!note.trim() || acting || !!activeLeave" @click="act('note', current?.id, note)">追加</NButton>
        </div>
        <div v-if="events.length" class="s-events">
          <div v-for="event in [...events].reverse()" :key="event.id">
            {{ new Date(event.time).toLocaleTimeString() }} · {{ items.find(item => item.id === event.task_id)?.title ?? '日程' }} · {{ actionLabel[event.action] ?? event.action }}<span v-if="event.duration_seconds != null"> · {{ Math.floor(event.duration_seconds / 60) }} 分钟</span><span v-if="event.note"> · {{ event.note }}</span>
          </div>
        </div>
      </div>
    </div>
    <div v-if="items.length" class="s-review">
      <NButton size="tiny" quaternary @click="showReview = !showReview">{{ showReview ? '收起对账' : '日对账' }}</NButton>
      <div v-if="showReview && review" class="s-extra-content">
        <div>总分 {{ review.score }} · 完成 {{ review.completed_count }} 项 · 偏离 {{ review.deviation_count }} 次 · 可计算耗时 {{ Math.floor(review.total_duration_seconds / 60) }} 分钟</div>
        <NAlert v-if="review.incomplete_legacy_items" type="warning">旧日程缺少动作记录，历史积分和耗时未补算。</NAlert>
        <div v-if="events.length" class="s-events">
          <div v-for="event in events" :key="event.id">
            {{ new Date(event.time).toLocaleTimeString() }} · {{ items.find(item => item.id === event.task_id)?.title ?? '日程' }} · {{ actionLabel[event.action] ?? event.action }}<span v-if="event.duration_seconds != null"> · {{ Math.floor(event.duration_seconds / 60) }} 分钟</span><span v-if="event.note"> · {{ event.note }}</span>
          </div>
        </div>
        <div class="s-extra-row">
          <NInput v-model:value="reviewNote" size="small" placeholder="明天只改一个变量，可留空" maxlength="300" />
          <NButton size="tiny" :loading="savingReview" @click="saveReview">保存</NButton>
        </div>
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
.s-extra { padding: 0 4px; }
.s-review { padding: 0 4px; font-size: 13px; color: #ccc; }
.s-parking { display: grid; gap: 8px; padding: 8px 4px; }
.s-parking-item { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.s-parking-done { color: #888; text-decoration: line-through; }
.s-leave { display: grid; gap: 8px; padding: 10px; border: 1px solid rgba(240, 160, 32, .4); border-radius: 8px; font-size: 13px; }
.s-extra-content { display: grid; gap: 8px; margin-top: 6px; }
.s-extra-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.s-extra-row .n-input { max-width: 300px; }
.s-events { max-height: 150px; overflow: auto; font-size: 12px; color: #aaa; }

/* ── Next card fog: blur + haze from 迷雾分, hover to peek ── */
.s-card--next .s-body {
  filter: blur(var(--fog-blur, 0px));
  opacity: var(--next-opacity, 1);
  transition: filter 0.5s ease, opacity 0.5s ease;
}
.s-card--next:hover .s-body,
.s-card--next:focus-within .s-body {
  filter: blur(0);
  opacity: 1;
}

@media (max-width: 760px) { .s-head { flex-direction: column; align-items: flex-start; } }
</style>
