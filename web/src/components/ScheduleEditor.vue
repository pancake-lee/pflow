<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { NAlert, NButton, NInput, NInputNumber, NTag, useMessage } from 'naive-ui'

type Item = { id: string; start: string; day_offset?: 0 | 1; title: string; focus_minutes: number; break_minutes: number; status?: 'completed' | 'skipped'; completed_at?: string }
type Template = { id: string; name: string; items: Item[] }
type SchedulePayload = { day: { date: string; items: Item[] }; templates: Template[] }

const emit = defineEmits<{ close: []; saved: [] }>()
const message = useMessage()
const date = ref(today())
const loadedDate = ref('')
const items = ref<Item[]>([])
const loadedItems = ref<Item[]>([])
const templates = ref<Template[]>([])
const templateName = ref('')
const loading = ref(false)
const saving = ref(false)
const savingTemplate = ref(false)
const deletingTemplateID = ref('')
const saveConflict = ref(false)
const error = ref('')
type SortMode = 'item' | 'content'
const drag = ref<{ id: string; mode: SortMode } | null>(null)
const dropTarget = ref<number | null>(null)
const busy = computed(() => loading.value || saving.value || savingTemplate.value || !!deletingTemplateID.value)
const timelineError = computed(() => validateTimeline(items.value))
const recordedChanges = computed(() => loadedItems.value.some((previous) => {
  if (!previous.status) return false
  const current = items.value.find((item) => item.id === previous.id)
  return !current || previous.title !== current.title || previous.start !== current.start ||
    (previous.day_offset ?? 0) !== (current.day_offset ?? 0) ||
    previous.focus_minutes !== current.focus_minutes || previous.break_minutes !== current.break_minutes
}))
watch(items, () => { saveConflict.value = false }, { deep: true })
let uidSeq = 0

function validateTimeline(itemList: Item[]): string {
  let previousEnd = -1
  for (const [index, item] of itemList.entries()) {
    const prefix = `第 ${index + 1} 项：`
    if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(item.start)) return prefix + '请输入有效开始时间'
    if (![0, 1].includes(item.day_offset ?? 0)) return prefix + '只支持当天或次日'
    if (!Number.isInteger(item.focus_minutes) || item.focus_minutes < 1 ||
      !Number.isInteger(item.break_minutes) || item.break_minutes < 0) return prefix + '请填写有效的专注和休息分钟'
    const start = minuteOf(item)
    if (index > 0 && start <= minuteOf(itemList[index - 1]!)) return prefix + '时间轴乱序，请按新顺序修改开始时间，或按时间排序'
    if (start < previousEnd) return prefix + '与上一项重叠，请修改开始时间或时长'
    previousEnd = start + item.focus_minutes + item.break_minutes
    if (previousEnd > 2880) return prefix + '结束时间不能超过次日午夜'
    if (!item.title.trim()) return prefix + '请输入内容'
  }
  return ''
}

// boundary is an insertion gap in the original list (0 through length).
function moveItems(itemList: Item[], from: number, boundary: number, mode: SortMode): Item[] {
  if (from < 0 || from >= itemList.length || boundary < 0 || boundary > itemList.length) return itemList
  const to = boundary > from ? boundary - 1 : boundary
  if (to === from) return itemList
  const reordered = [...itemList]
  const [moved] = reordered.splice(from, 1)
  reordered.splice(to, 0, moved!)
  return mode === 'item' ? reordered : itemList.map((item, index) => ({ ...item, title: reordered[index]!.title }))
}
function endDrag() { drag.value = null; dropTarget.value = null }
function startDrag(event: DragEvent, item: Item, mode: SortMode) {
  if (busy.value || !event.dataTransfer) { event.preventDefault(); return }
  drag.value = { id: item.id, mode }
  event.dataTransfer.effectAllowed = 'move'
  event.dataTransfer.setData('text/plain', item.id)
}
function dragOver(event: DragEvent, index: number) {
  if (busy.value || !drag.value) return
  event.preventDefault()
  event.stopPropagation()
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  dropTarget.value = index + Number(event.clientY >= rect.top + rect.height / 2)
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}
function dragOverGap(event: DragEvent, boundary: number) {
  if (busy.value || !drag.value) return
  event.preventDefault()
  event.stopPropagation()
  dropTarget.value = boundary
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}
function drop(event: DragEvent) {
  if (busy.value || !drag.value || dropTarget.value === null) return
  const mode = drag.value.mode
  event.preventDefault()
  event.stopPropagation()
  const from = items.value.findIndex((item) => item.id === drag.value!.id)
  items.value = moveItems(items.value, from, dropTarget.value, mode)
  endDrag()
}

function today() {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
}
function cloneItems(itemList: Item[]) { return itemList.map((item) => ({ ...item })) }
function uid() { return `item-${Date.now()}-${uidSeq++}` }
const dirty = computed(() => JSON.stringify(items.value) !== JSON.stringify(loadedItems.value))

function minuteOf(item: Item) {
  const [hour, minute] = item.start.split(':').map(Number)
  return (item.day_offset ?? 0) * 24 * 60 + hour * 60 + minute
}
function startAfterLastItem(): Pick<Item, 'start' | 'day_offset'> {
  const last = items.value.at(-1)
  if (!last) return { start: '09:00' }
  const total = Math.min(minuteOf(last) + last.focus_minutes + last.break_minutes, 2 * 24 * 60 - 10)
  return { start: `${String(Math.floor((total % 1440) / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`, day_offset: (total >= 1440 ? 1 : 0) as 0 | 1 }
}
function newItem() { items.value.push({ id: uid(), ...startAfterLastItem(), title: '', focus_minutes: 50, break_minutes: 10 }) }
function sortItems() { items.value.sort((left, right) => minuteOf(left) - minuteOf(right)) }
function isNextDayCandidate(index: number) {
  const item = items.value[index]
  const previous = items.value[index - 1]
  if (!previous || item.day_offset === 1 || previous.day_offset === 1) return false
  return minuteOf({ ...previous, day_offset: 0 }) >= 18 * 60 && minuteOf({ ...item, day_offset: 0 }) < minuteOf(previous)
}
function toggleDayOffset(item: Item) { item.day_offset = item.day_offset === 1 ? 0 : 1 }
async function readError(response: Response, fallback: string) {
  try { return ((await response.json()) as { error?: string }).error || fallback } catch { return fallback }
}
async function load(targetDate = date.value) {
  loading.value = true
  error.value = ''
  try {
    const response = await fetch(`/api/v1/schedules?date=${encodeURIComponent(targetDate)}`)
    if (!response.ok) throw new Error(await readError(response, '读取日程失败'))
    const payload = await response.json() as SchedulePayload
    date.value = targetDate
    loadedDate.value = targetDate
    items.value = cloneItems(payload.day.items)
    loadedItems.value = cloneItems(payload.day.items)
    templates.value = payload.templates
    saveConflict.value = false
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '读取日程失败'
    date.value = loadedDate.value || date.value
  } finally { loading.value = false }
}
async function changeDate() {
  const targetDate = date.value
  if (targetDate === loadedDate.value) return
  if (dirty.value && !window.confirm('切换日期会放弃当前未保存的编辑，确定切换吗？')) { date.value = loadedDate.value; return }
  await load(targetDate)
}
async function save(replaceRecorded = false) {
  if (busy.value || timelineError.value) return
  saving.value = true
  error.value = ''
  try {
    const response = await fetch('/api/v1/schedules', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ date: date.value, items: items.value, replace_recorded: replaceRecorded }) })
    if (response.status === 409) { saveConflict.value = true; return }
    if (!response.ok) throw new Error(await readError(response, '保存日程失败'))
    const day = await response.json() as { items: Item[] }
    items.value = cloneItems(day.items)
    loadedItems.value = cloneItems(day.items)
    saveConflict.value = false
    message.success(`${date.value} 日程已保存`)
    emit('saved')
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存日程失败' } finally { saving.value = false }
}
function applyTemplate(template: Template) {
  if (dirty.value && !window.confirm('应用模板会替换当前未保存的编辑，确定继续吗？')) return
  items.value = cloneItems(template.items).map((item) => ({ ...item, id: uid(), status: undefined, completed_at: undefined }))
  message.success(`已将“${template.name}”填入编辑器，保存后才会写入 ${date.value}`)
}
async function saveTemplate(replaceExisting = false) {
  if (timelineError.value || loading.value || saving.value) return
  const name = templateName.value.trim()
  if (!name) { error.value = '请输入模板名称'; return }
  savingTemplate.value = true
  error.value = ''
  try {
    const response = await fetch('/api/v1/schedule-templates', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, items: items.value, replace_existing: replaceExisting }) })
    if (response.status === 409 && !replaceExisting) {
      if (window.confirm(`模板“${name}”已存在。确定用当前编辑覆盖它吗？`)) await saveTemplate(true)
      return
    }
    if (!response.ok) throw new Error(await readError(response, '另存模板失败'))
    const template = await response.json() as Template
    templates.value = templates.value.some((item) => item.id === template.id)
      ? templates.value.map((item) => item.id === template.id ? template : item)
      : [...templates.value, template]
    templateName.value = ''
    message.success(replaceExisting ? '模板已覆盖，当前编辑不会被重置' : '模板已保存，当前编辑不会被重置')
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '另存模板失败' } finally { savingTemplate.value = false }
}
async function delTemplate(template: Template) {
  if (!window.confirm(`删除模板“${template.name}”不会影响已保存日程，确定删除吗？`)) return
  deletingTemplateID.value = template.id
  error.value = ''
  try {
    const response = await fetch(`/api/v1/schedule-templates?id=${encodeURIComponent(template.id)}`, { method: 'DELETE' })
    if (!response.ok) throw new Error(await readError(response, '删除模板失败'))
    templates.value = templates.value.filter((item) => item.id !== template.id)
    message.success('模板已删除')
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '删除模板失败' } finally { deletingTemplateID.value = '' }
}
function close() {
  if (dirty.value && !window.confirm('关闭会放弃当前未保存的编辑，确定关闭吗？')) return
  emit('close')
}
onMounted(() => void load())
</script>

<template>
  <main class="editor" :aria-busy="loading || saving">
    <div class="editor-toolbar"><div><span class="eyebrow">日程编辑</span><h2>{{ date }}</h2></div><NTag v-if="dirty" type="warning" size="small">有未保存编辑</NTag><NTag v-else type="success" size="small">已保存</NTag></div>
    <fieldset class="editor-controls" :disabled="busy">
    <div class="date-row"><label for="schedule-date">编辑日期</label><input id="schedule-date" v-model="date" type="date" @change="changeDate" /><small>切换日期前会保护未保存编辑</small></div>
    <NAlert v-if="error" type="error" closable @close="error = ''">{{ error }}</NAlert>
    <NAlert v-if="timelineError" type="warning" aria-live="polite">时间轴未通过校验，修正后才能保存。{{ timelineError }}</NAlert>
    <NAlert v-if="recordedChanges" type="warning">修改涉及已完成或跳过的日程；保存时需确认替换，变更项的执行记录将被清除。</NAlert>
    <NAlert v-if="saveConflict" type="warning">此次修改会替换或删除已完成/跳过的日程及其记录。<NButton size="small" type="warning" :loading="saving" @click="save(true)">确认替换记录并保存</NButton></NAlert>

    <section class="timeline" aria-label="日程时间轴"><div class="section-head"><div><h3>时间轴</h3><p>左侧拖整项，内容标题前拖内容。拖到任意一行的上半部插在该行前，下半部插在该行后；行间高亮线表示插入位置。</p></div><div class="timeline-actions"><NButton @click="sortItems">按时间排序</NButton><NButton @click="newItem">添加日程项</NButton></div></div>
      <div class="drag-status" aria-live="polite"><NTag v-if="drag" type="info" size="small">{{ drag.mode === 'content' ? '正在拖内容，时间和时长保持固定' : '正在拖整项，全部字段随项移动' }}</NTag></div>
      <div v-if="items.length === 0" class="empty-state">还没有日程项。可添加一项，或选择模板后应用。</div>
      <div v-if="items.length" class="timeline-items">
      <template v-for="(item, index) in items" :key="item.id">
        <div class="insert-gap" :class="{ 'insert-gap--active': drag && dropTarget === index }" :data-boundary="index" @dragover="dragOverGap($event, index)" @drop="drop($event)" aria-hidden="true"></div>
      <article class="item-card" @dragover.capture="dragOver($event, index)" @drop.capture="drop($event)"><div class="item-order"><button type="button" class="drag-handle item-handle" :draggable="!busy" :aria-label="`拖整项，第 ${index + 1} 项`" title="拖整项，包含时间、内容和时长" @dragstart.stop="startDrag($event, item, 'item')" @dragend="endDrag">⠿</button><strong>{{ index + 1 }}</strong></div>
        <div class="item-fields"><label>开始时间<input v-model="item.start" type="time" /></label><label class="item-title"><span class="content-label"><button type="button" class="drag-handle content-handle" :draggable="!busy" :aria-label="`拖内容，第 ${index + 1} 项`" title="只拖内容，时间和时长固定" @dragstart.stop="startDrag($event, item, 'content')" @dragend="endDrag">⠿</button>内容</span><NInput v-model:value="item.title" placeholder="例如：撰写方案" /></label><label>专注分钟<NInputNumber v-model:value="item.focus_minutes" :min="1" /></label><label>休息分钟<NInputNumber v-model:value="item.break_minutes" :min="0" /></label></div>
        <div class="item-actions"><NTag v-if="item.status" :type="item.status === 'completed' ? 'success' : 'warning'" size="small">{{ item.status === 'completed' ? '已完成记录' : '已跳过记录' }}</NTag><NButton v-if="item.day_offset === 1 || isNextDayCandidate(index)" size="small" tertiary class="date-adjust" :class="{ 'date-adjust--active': item.day_offset === 1 }" @click="toggleDayOffset(item)">次日</NButton><NButton size="small" tertiary type="error" @click="items.splice(index, 1)">删除</NButton></div></article>
      </template>
      <div class="insert-gap" :class="{ 'insert-gap--active': drag && dropTarget === items.length }" :data-boundary="items.length" @dragover="dragOverGap($event, items.length)" @drop="drop($event)" aria-hidden="true"></div>
      </div>
    </section>

    <section class="templates"><div class="section-head"><div><h3>模板</h3><p>模板只保存日程内容，不保存完成或跳过记录。</p></div></div><div class="template-save"><NInput v-model:value="templateName" placeholder="将当前编辑另存为新模板" @keyup.enter="saveTemplate()" /><NButton :loading="savingTemplate" :disabled="!!timelineError" @click="saveTemplate()">另存为模板</NButton></div><div v-if="templates.length" class="template-list"><div v-for="template in templates" :key="template.id" class="template-row"><span>{{ template.name }} · {{ template.items.length }} 项</span><div class="template-actions"><NButton size="tiny" tertiary @click="applyTemplate(template)">应用</NButton><NButton size="tiny" tertiary type="error" :loading="deletingTemplateID === template.id" @click="delTemplate(template)">删除</NButton></div></div></div></section>
    <footer class="editor-footer"><NButton @click="close">关闭</NButton><NButton type="primary" :loading="saving" :disabled="!!timelineError || loading" @click="save()">保存 {{ date }} 日程</NButton></footer>
    </fieldset>
  </main>
</template>

<style scoped>
/* Keep rows stationary when native dragging starts. */
.drag-status { display: flex; align-items: center; height: 24px; flex: none; }
.editor-controls { display: flex; flex-direction: column; gap: 20px; min-width: 0; margin: 0; padding: 0; border: 0; }
.drag-handle { display: inline-flex; align-items: center; justify-content: center; width: 26px; height: 28px; padding: 0; border: 1px solid var(--n-border-color); border-radius: 4px; background: transparent; color: var(--n-text-color-2); cursor: grab; font-size: 20px; }
.drag-handle:active { cursor: grabbing; }
.drag-handle:focus-visible { outline: 2px solid var(--n-color-target, #63e2b7); outline-offset: 2px; }
.content-label { display: flex; align-items: center; gap: 5px; }
.content-handle { width: 22px; height: 22px; color: var(--n-color-target, #63e2b7); }
.timeline-items { display: flex; flex-direction: column; }
.insert-gap { position: relative; height: 12px; flex: none; }
.insert-gap::after { content: ''; position: absolute; left: 0; right: 0; top: 50%; height: 3px; transform: translateY(-50%); background: var(--n-color-target, #63e2b7); border-radius: 2px; opacity: 0; pointer-events: none; }
.insert-gap--active::after { opacity: 1; }

.editor { display: flex; flex-direction: column; gap: 20px; padding-bottom: 4px; }.editor-toolbar,.date-row,.section-head,.item-card,.item-fields,.item-actions,.template-save,.template-actions,.editor-footer,.timeline-actions { display: flex; gap: 12px; align-items: center; }.editor-toolbar,.section-head,.editor-footer { justify-content: space-between; }.eyebrow { color: var(--n-text-color-3); font-size: 12px; }.editor-toolbar h2,.section-head h3 { margin: 2px 0 0; }.date-row { flex-wrap: wrap; }.date-row input,.item-fields input { color-scheme: dark; }.item-fields input { box-sizing: border-box; width: 100%; min-height: 34px; padding: 0 10px; color: var(--n-text-color-base); border: 1px solid var(--n-border-color); border-radius: var(--n-border-radius); background: var(--n-input-color); }.date-row small,.section-head p { margin: 0; color: var(--n-text-color-3); font-size: 12px; }.timeline,.templates { display: flex; flex-direction: column; gap: 12px; padding-top: 16px; border-top: 1px solid var(--n-divider-color); }.item-card { align-items: stretch; padding: 12px; border: 1px solid var(--n-border-color); border-radius: 8px; }.item-order { display: flex; flex-direction: column; align-items: center; gap: 6px; min-width: 26px; color: var(--n-text-color-3); }.item-fields { flex: 1; align-items: end; flex-wrap: wrap; }.item-fields label { display: flex; flex: 0 1 112px; flex-direction: column; gap: 5px; color: var(--n-text-color-3); font-size: 12px; }.item-fields .item-title { flex: 1 1 190px; }.item-actions { flex-direction: column; justify-content: center; align-items: flex-end; }.date-adjust { opacity: 0; transition: opacity .15s ease; }.item-card:hover .date-adjust,.item-card:focus-within .date-adjust { opacity: .45; }.date-adjust--active,.item-card:hover .date-adjust--active,.item-card:focus-within .date-adjust--active { opacity: 1; }.empty-state { padding: 18px; border: 1px dashed var(--n-border-color); border-radius: 8px; color: var(--n-text-color-3); text-align: center; }.template-save > :deep(.n-input) { flex: 1; min-width: 0; }.template-list { display: flex; flex-direction: column; border-top: 1px solid var(--n-divider-color); }.template-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; column-gap: 12px; align-items: center; padding: 8px 0; border-bottom: 1px solid var(--n-divider-color); }.template-row > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.editor-footer { position: sticky; bottom: 0; padding-top: 4px; background: var(--n-color-modal); }
@media (max-width: 760px) { .item-card { flex-direction: column; }.item-order { flex-direction: row; justify-content: space-between; }.reorder-actions { flex-direction: row; }.item-fields { display: grid; grid-template-columns: 1fr 1fr; align-items: end; }.item-fields label,.item-fields .item-title { min-width: 0; }.item-title,.item-fields :deep(.n-checkbox) { grid-column: 1 / -1; }.item-actions { flex-direction: row; justify-content: space-between; align-items: center; }.template-apply,.template-save { align-items: stretch; flex-direction: column; } }
</style>
