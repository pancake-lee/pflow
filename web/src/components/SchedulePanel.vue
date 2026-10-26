<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { NAlert, NButton, NInput, NModal, NSelect, useMessage } from 'naive-ui'
import { highlightToMarquee } from '../composables/useReminderScores'

type Item = { id: string; start: string; day_offset?: number; title: string; focus_minutes: number; break_minutes: number; status?: string }
type Event = { id: string; task_id?: string; time: string; action: string; points?: number; duration_seconds?: number; note?: string }
type ReviewSummary = { score: number; max_score: number; standard_score: number | null; completed_count: number; deviation_count: number; total_duration_seconds: number; incomplete_legacy_items: boolean }
type ParkingItem = { id: string; content: string; created_at: string; processed_at?: string | null }

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
const jumpPoints = ref<Record<string, number>>({})
const now = ref(Date.now())
const review = ref<ReviewSummary | null>(null)
const todayReview = ref<ReviewSummary | null>(null)
const reviewDate = ref('')
const reviewItems = ref<Item[]>([])
const reviewEvents = ref<Event[]>([])
const reviewNote = ref('')
const storedReviewNote = ref('')
const showReview = ref(false)
const savingReview = ref(false)
const loadingReview = ref(false)
const reviewError = ref('')
const parking = ref<ParkingItem[]>([])
const parkingText = ref('')
const parkingOpen = ref(false)
const parkingBusy = ref(false)
const activeLeave = ref<Event | null>(null)
const leaveOpen = ref(false)
const leaveIntent = ref('')
const leaveRemaining = ref(10)
const mobileIntent = ref('')
const emit = defineEmits<{ edit: [] }>()
const message = useMessage()
let refreshTimer: ReturnType<typeof setInterval> | null = null
let clockTimer: ReturnType<typeof setInterval> | null = null
let leaveTimer: ReturnType<typeof setInterval> | null = null
let autoStartAttempted = ''
let reviewRequestID = 0

function parkingProcessed(entry: ParkingItem) {
  return !!entry.processed_at && !entry.processed_at.startsWith('0001-01-01T00:00:00')
}

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
    jumpPoints.value = payload.jump_points ?? {}
    todayReview.value = payload.review ?? null
    parking.value = payload.day.parking ?? []
    activeLeave.value = payload.active_leave ?? null
    loadError.value = ''
    maybeAutoStart()
  } catch {
    loadError.value = '读取今日日程失败，操作暂不可用。'
  }
}

function openReview() {
  reviewDate.value = day.value
  showReview.value = true
  void loadReview(reviewDate.value)
}

async function loadReview(date: string) {
  const requestID = ++reviewRequestID
  loadingReview.value = true
  reviewError.value = ''
  review.value = null
  try {
    const response = await fetch(`/api/v1/schedules/review?date=${encodeURIComponent(date)}`)
    if (!response.ok) throw new Error('读取日对账失败')
    const payload = await response.json()
    if (requestID !== reviewRequestID) return
    review.value = payload.review
    reviewItems.value = payload.items ?? []
    reviewEvents.value = payload.events ?? []
    reviewNote.value = payload.note ?? ''
    storedReviewNote.value = reviewNote.value
  } catch {
    if (requestID === reviewRequestID) reviewError.value = '读取日对账失败，请重试。'
  } finally {
    if (requestID === reviewRequestID) loadingReview.value = false
  }
}

async function changeReviewDate(date: string) {
  if (!date || date === reviewDate.value || savingReview.value) return
  if (reviewNote.value.trim() !== storedReviewNote.value && !await saveReview(false)) return
  reviewDate.value = date
  await loadReview(date)
}

function shiftReviewDate(days: number) {
  const [year, month, date] = reviewDate.value.split('-').map(Number)
  const shifted = new Date(year, month - 1, date + days)
  const target = `${shifted.getFullYear()}-${String(shifted.getMonth() + 1).padStart(2, '0')}-${String(shifted.getDate()).padStart(2, '0')}`
  void changeReviewDate(target)
}

function chooseReviewDate(event: globalThis.Event) {
  const input = event.target as HTMLInputElement
  const selected = input.value
  void changeReviewDate(selected).then(() => { input.value = reviewDate.value })
}

async function setReviewVisible(visible: boolean) {
  if (visible) return
  if (reviewNote.value.trim() !== storedReviewNote.value && !await saveReview(false)) return
  showReview.value = false
  reviewRequestID++
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
    if (!response.ok) throw new Error('记下失败')
    parkingText.value = ''
    await load()
  } catch {
    message.error('记下失败，请重试')
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

async function deleteParking(id: string) {
  if (!day.value || parkingBusy.value) return
  parkingBusy.value = true
  try {
    const response = await fetch('/api/v1/schedules/parking', {
      method: 'DELETE', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date: day.value, id }),
    })
    if (!response.ok) throw new Error('删除失败')
    await load()
  } catch {
    message.error('删除便签失败，请重试')
  } finally {
    parkingBusy.value = false
  }
}

async function saveReview(showSuccess = true): Promise<boolean> {
  if (!reviewDate.value || savingReview.value || loadingReview.value || reviewError.value) return false
  savingReview.value = true
  try {
    const response = await fetch('/api/v1/schedules/review', {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date: reviewDate.value, note: reviewNote.value }),
    })
    if (!response.ok) throw new Error('保存对账失败')
    const payload = await response.json()
    reviewNote.value = payload.note
    storedReviewNote.value = payload.note
    if (showSuccess) message.success('已保存日对账备注')
    return true
  } catch {
    message.error('保存对账失败，请重试')
    return false
  } finally {
    savingReview.value = false
  }
}

async function act(action: string, id = current.value?.id, actionNote = '') {
  if (!id || acting.value || loadError.value) return
  acting.value = true
  let succeeded = false
  try {
    const options = {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date: day.value, id, action, note: actionNote }),
    }
    const response = await fetch('/api/v1/schedules/action', options)
    if (!response.ok) throw new Error('更新日程状态失败，请重试')
    const updatedDay = await response.json()
    if (action === 'leave') {
      const latestEvent: Event | undefined = updatedDay.events?.at(-1)
      if (latestEvent?.action === 'leave') activeLeave.value = latestEvent
    }
    if (action === 'return') activeLeave.value = null
    await load()
    succeeded = true
    if (action === 'leave' && activeLeave.value) {
      leaveOpen.value = false
      leaveIntent.value = ''
      mobileIntent.value = ''
      stopLeaveTimer()
    }
  } catch {
    if (action === 'start') autoStartAttempted = ''
    message.error('更新日程状态失败，请重试')
  } finally {
    acting.value = false
    if (succeeded && action !== 'start') maybeAutoStart()
  }
}

function maybeAutoStart() {
  const id = current.value?.id
  if (!id || acting.value || activeLeave.value ||
      document.visibilityState === 'hidden' || hasActiveStart(id)) return
  const lastPause = [...events.value].reverse().find(event => event.task_id === id && event.action === 'pause')
  const key = `${day.value}:${id}:${lastPause?.id ?? ''}`
  if (autoStartAttempted === key) return
  autoStartAttempted = key
  void act('start', id)
}

function hasActiveStart(id: string) {
  for (let i = events.value.length - 1; i >= 0; i--) {
    const event = events.value[i]
    if (event.task_id !== id) continue
    if (event.action === 'start') return true
    if (['pause', 'completed', 'skipped'].includes(event.action)) return false
  }
  return false
}

function onVisibilityChange() {
  if (document.visibilityState === 'visible') void load()
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
  return ordered.filter(item => !item.status && item.id !== current.value?.id)
})
const reorderOptions = computed(() => pending.value.map(item => ({
  label: `${item.title} · ${formatPoints(jumpPoints.value[item.id] ?? -3)} 分`, value: item.id,
})))

function formatPoints(points: number) {
  return points > 0 ? `+${points}` : points < 0 ? `−${Math.abs(points)}` : '0'
}

function formatStandardScore(score: number | null | undefined) {
  return score == null ? '—' : `${Number(score.toFixed(1))}/100`
}

function eventPoints(event: Event) {
  if (event.points != null) return event.points
  if (event.action === 'completed') return 10
  if (event.action === 'skipped' || event.action === 'reorder') return -3
  return 0
}

function selectReorder(value: string | null) {
  if (value) void act('reorder', value)
}

const elapsed = computed(() => {
  const id = current.value?.id
  if (!id) return ''
  let seconds = 0
  let started = ''
  for (const event of events.value) {
    if (event.task_id !== id) continue
    if (event.action === 'start') started = event.time
    if (event.action === 'pause') {
      seconds += event.duration_seconds ?? (started ? Math.max(0, Math.floor((Date.parse(event.time) - Date.parse(started)) / 1000)) : 0)
      started = ''
    }
  }
  if (started) seconds += Math.max(0, Math.floor((now.value - Date.parse(started)) / 1000))
  return seconds || started ? `已计时 ${Math.floor(seconds / 60)} 分钟` : ''
})

const actionLabel: Record<string, string> = {
  start: '开始', completed: '完成', skipped: '跳过',
  pause: '暂停计时',
  early: '提前', reorder: '跳转日程', delay: '拖延', note: '备注',
  park: '记下', parking_processed: '处理停车场',
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
  document.addEventListener('visibilitychange', onVisibilityChange)
  refreshTimer = setInterval(() => void load(), 30_000)
  clockTimer = setInterval(() => { now.value = Date.now() }, 1_000)
})

onUnmounted(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
  if (refreshTimer) clearInterval(refreshTimer)
  if (clockTimer) clearInterval(clockTimer)
  stopLeaveTimer()
})
</script>

<template>
  <section v-if="current || loadError" class="schedule">
    <NAlert v-if="loadError" type="error">{{ loadError }}</NAlert>
    <div class="s-toolbar">
      <div class="s-heading">
        <strong>今日日程</strong>
        <span v-if="todayReview" class="s-score">今日积分 {{ formatPoints(todayReview.score) }}<template v-if="todayReview.max_score > 0"> / {{ todayReview.max_score }}</template> · 标准分 {{ formatStandardScore(todayReview.standard_score) }}<template v-if="todayReview.incomplete_legacy_items"> · 旧记录未计入</template></span>
      </div>
      <div class="s-toolbar-actions">
        <NSelect
          v-if="items.length"
          :value="null"
          :options="reorderOptions"
          size="small"
          placeholder="跳转日程"
          class="s-reorder"
          :disabled="!current?.id || !reorderOptions.length || acting || !!activeLeave || !!loadError"
          @update:value="selectReorder"
        />
        <NButton size="small" quaternary @click="emit('edit')">编辑</NButton>
        <NButton v-if="items.length" size="small" quaternary @click="parkingOpen = true">停车场</NButton>
        <NButton size="small" quaternary :disabled="!!loadError" @click="openReview">日对账</NButton>
      </div>
    </div>
    <div
      v-if="current"
      class="s-card s-card--current"
      :class="{ 's-card--break': phase === 'break' }"
      :style="currentStyle"
    >
      <div class="s-head">
        <span class="s-label">当前日程</span>
        <div class="s-actions">
          <template v-if="current.id">
            <NButton size="small" :loading="acting" :disabled="!!loadError || !!activeLeave" @click="act('completed')">完成 +10</NButton>
            <NButton size="small" :disabled="acting || !!loadError || !!activeLeave" @click="act('skipped')">跳过 −3</NButton>
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
    <div v-if="next" class="s-card s-card--next" :style="nextStyle">
      <div class="s-head">
        <span class="s-label">下一项</span>
        <NButton v-if="current?.id" size="small" quaternary :disabled="acting || !!loadError || !!activeLeave" @click="act('completed', next.id)">提前完成 +10</NButton>
      </div>
      <div class="s-body">
        <span class="s-title">{{ next.title }}</span>
        <small class="s-meta">{{ itemTime(next) }} · {{ next.focus_minutes }} 分钟</small>
      </div>
    </div>
    <NModal
      :show="leaveOpen || !!activeLeave"
      preset="card"
      title="暂离"
      :style="{ width: 'min(520px, calc(100vw - 32px))' }"
      :mask-closable="false"
      :close-on-esc="false"
      :closable="false"
    >
      <div class="s-leave">
        <NAlert v-if="loadError" type="error">{{ loadError }}</NAlert>
        <template v-if="activeLeave">
          <div>暂离中 · 回来继续：{{ items.find(item => item.id === activeLeave?.task_id)?.title ?? '当前日程' }}</div>
          <div v-if="activeLeave.note">切换意图：{{ activeLeave.note }}</div>
          <NButton size="small" :loading="acting" :disabled="!!loadError" @click="act('return', activeLeave.task_id)">我回来了</NButton>
        </template>
        <template v-else>
          <div>现在切去做什么？写一句即可暂离；也可以等待 {{ leaveRemaining }} 秒。</div>
          <NInput v-model:value="leaveIntent" size="small" placeholder="切换意图或返回线索，可留空" maxlength="200" />
          <div class="s-extra-row">
            <NButton size="small" :loading="acting" :disabled="(!leaveIntent.trim() && leaveRemaining > 0) || !!loadError" @click="act('leave', current?.id, leaveIntent)">确认暂离</NButton>
            <NButton size="small" quaternary :disabled="acting" @click="closeLeave">我回来了</NButton>
          </div>
        </template>
      </div>
    </NModal>
    <div v-if="current?.id && !activeLeave" class="s-mobile-cue">
      <NInput v-model:value="mobileIntent" size="small" placeholder="记下当前意图或返回线索" maxlength="200" />
      <NButton size="small" :disabled="!mobileIntent.trim() || acting" @click="act('leave', current?.id, mobileIntent)">记录并暂离</NButton>
    </div>
    <NModal
      v-model:show="parkingOpen"
      preset="card"
      title="停车场"
      :style="{ width: 'min(680px, calc(100vw - 32px))' }"
      closable
    >
      <div class="s-parking">
        <div class="s-extra-row">
          <NInput v-model:value="parkingText" size="small" placeholder="先记下这件事，继续当前项" maxlength="300" @keyup.enter="addParking" />
          <NButton size="small" :disabled="!parkingText.trim() || parkingBusy" @click="addParking">记下</NButton>
        </div>
        <div v-if="parking.length" class="s-parking-list">
          <div v-for="entry in parking" :key="entry.id" class="s-parking-item" :class="{ 's-parking-item--done': parkingProcessed(entry) }">
            <span>{{ entry.content }}</span>
            <div class="s-parking-actions">
              <button v-if="!parkingProcessed(entry)" type="button" title="标记完成" aria-label="标记完成" :disabled="parkingBusy" @click="processParking(entry.id)">✔</button>
              <button type="button" title="删除便签" aria-label="删除便签" :disabled="parkingBusy" @click="deleteParking(entry.id)">×</button>
            </div>
          </div>
        </div>
        <div v-else class="s-meta">暂无收纳事项。</div>
      </div>
    </NModal>
    <NModal
      :show="showReview"
      preset="card"
      title="日对账"
      :style="{ width: 'min(680px, calc(100vw - 32px))' }"
      closable
      @update:show="setReviewVisible"
    >
      <template #header-extra>
        <div class="s-review-nav">
          <NButton size="small" quaternary circle title="前一天" aria-label="前一天" :disabled="savingReview || loadingReview" @click="shiftReviewDate(-1)">‹</NButton>
          <input type="date" :value="reviewDate" :disabled="savingReview || loadingReview" aria-label="日对账日期" @change="chooseReviewDate" />
          <NButton size="small" quaternary circle title="后一天" aria-label="后一天" :disabled="savingReview || loadingReview" @click="shiftReviewDate(1)">›</NButton>
        </div>
      </template>
      <NAlert v-if="reviewError" type="error">{{ reviewError }}</NAlert>
      <div v-else-if="loadingReview">正在读取日对账…</div>
      <div v-else-if="review" class="s-review-content">
        <div>原始积分 {{ formatPoints(review.score) }}<template v-if="review.max_score > 0"> / {{ review.max_score }}</template> · 标准分 {{ formatStandardScore(review.standard_score) }} · 完成 {{ review.completed_count }} 项 · 偏离 {{ review.deviation_count }} 次 · 累计计时 {{ Math.floor(review.total_duration_seconds / 60) }} 分钟</div>
        <NAlert v-if="review.incomplete_legacy_items" type="warning">旧日程缺少动作记录，历史积分和耗时未补算。</NAlert>
        <div v-if="reviewEvents.length" class="s-events">
          <div v-for="event in reviewEvents" :key="event.id">
            {{ new Date(event.time).toLocaleTimeString() }} · {{ reviewItems.find(item => item.id === event.task_id)?.title ?? '日程' }} · {{ event.action === 'reorder' && eventPoints(event) === 0 ? '跳回计划' : (actionLabel[event.action] ?? event.action) }} · {{ formatPoints(eventPoints(event)) }} 分<span v-if="event.duration_seconds != null"> · {{ Math.floor(event.duration_seconds / 60) }} 分钟</span><span v-if="event.note"> · {{ event.note }}</span>
          </div>
        </div>
        <div v-else class="s-meta">这一天没有日程动作记录。</div>
        <div class="s-extra-row">
          <NInput v-model:value="reviewNote" size="small" placeholder="日对账备注" maxlength="300" :disabled="savingReview" />
          <NButton size="small" :loading="savingReview" :disabled="reviewNote.trim() === storedReviewNote" @click="saveReview()">保存备注</NButton>
        </div>
      </div>
    </NModal>
  </section>
</template>

<style scoped>
.schedule { display: flex; flex-direction: column; gap: 8px; margin: 16px 0; padding: 14px; border: 1px solid rgba(94, 163, 240, .28); border-radius: 12px; background: rgba(32, 128, 240, .025); }
.s-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.s-toolbar strong { color: #8bbdff; font-size: 14px; }
.s-heading { display: flex; align-items: baseline; gap: 12px; flex-wrap: wrap; }
.s-score { color: var(--n-text-color-2); font-size: 12px; }
.s-toolbar-actions { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.s-reorder { width: 150px; }
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
.s-review-content { display: grid; gap: 12px; font-size: 13px; color: #ccc; }
.s-review-nav { display: flex; align-items: center; gap: 4px; }
.s-review-nav input { width: 122px; min-width: 0; padding: 4px; border: 1px solid rgba(94, 163, 240, .35); border-radius: 6px; color: #ddd; background: #202833; font-size: 12px; }
@media (max-width: 480px) {
  .s-review-nav { gap: 0; }
  .s-review-nav input { width: 100px; font-size: 11px; }
  .s-review-nav :deep(.n-button) { width: 24px; height: 24px; }
}
.s-parking { display: grid; gap: 8px; }
.s-parking .n-input { flex: 1; max-width: none; }
.s-parking-list { display: flex; flex-wrap: wrap; gap: 8px; }
.s-parking-item { display: flex; flex: 1 1 160px; flex-direction: column; justify-content: space-between; gap: 10px; max-width: 220px; min-height: 76px; padding: 10px; border-radius: 8px; background: #ffe29a; color: #34280e; font-size: 13px; }
.s-parking-item--done { background: #a7dfb2; color: #123522; }
.s-parking-item span { overflow-wrap: anywhere; }
.s-parking-actions { display: flex; justify-content: flex-end; gap: 6px; }
.s-parking-actions button { width: 24px; height: 24px; border: 0; border-radius: 5px; background: rgba(0, 0, 0, .1); color: inherit; font-size: 17px; line-height: 1; cursor: pointer; }
.s-parking-actions button:hover:not(:disabled) { background: rgba(0, 0, 0, .2); }
.s-parking-actions button:disabled { opacity: .5; cursor: default; }
.s-leave { display: grid; gap: 8px; font-size: 13px; }
.s-mobile-cue { display: none; }
.s-extra-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.s-extra-row .n-input { max-width: 300px; }
.s-events { max-height: 320px; overflow: auto; font-size: 12px; color: #aaa; }

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

@media (max-width: 760px) {
  .s-head { flex-direction: column; align-items: flex-start; }
  .s-actions { flex-wrap: wrap; }
  .s-mobile-cue { display: flex; gap: 8px; align-items: center; }
  .s-mobile-cue .n-input { flex: 1; min-width: 0; }
}
</style>
