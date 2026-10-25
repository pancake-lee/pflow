<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NAlert, NButton, NInput, NInputNumber, NSelect, NSwitch, NTabPane, NTabs, useMessage } from 'naive-ui'

import type { KnowledgeTip } from '../types/dashboard'

type Settings = {
  custom_knowledge_tips: KnowledgeTip[]
  version: number
  dashboard: { window: string; max_active: number; max_inactive: number; agent_filter: string; refresh_seconds: number; daily_boot_enabled: boolean }
  attention: { protect_minutes: number; focus_add_minutes: number; mask_strength: number }
  time_estimate: { minutes_per_message: number; fallback_ratio: number }
}
type ProjectRoot = { path: string; priority: string; slot?: string }

const emit = defineEmits<{ saved: []; close: [] }>()
const message = useMessage()
const value = ref<Settings | null>(null)
const loadedValue = ref<Settings | null>(null)
const defaults = ref<Settings | null>(null)
const roots = ref<ProjectRoot[]>([])
const loading = ref(true)
const saving = ref(false)
const projectBusy = ref(false)
const error = ref('')
const busy = computed(() => loading.value || saving.value || projectBusy.value)
const dirty = computed(() => JSON.stringify(value.value) !== JSON.stringify(loadedValue.value))
const windowOptions = ['1h', '3h', '6h', '1d', '3d', '7d'].map(value => ({ label: value, value }))
const refreshOptions = [0, 10, 30, 60].map(value => ({ label: value === 0 ? '关闭' : `${value}s`, value }))
const agentFilterOptions = [
  { label: '全部 Agent', value: 'all' },
  { label: 'Claude Code', value: 'claude' },
  { label: 'Hermes', value: 'hermes' },
  { label: 'Codex', value: 'codex' },
]
const newPath = ref('')
const validationError = computed(() => {
  const current = value.value
  if (!current) return ''
  if (!windowOptions.some(option => option.value === current.dashboard.window)) return '请选择有效扫描范围'
  if (!refreshOptions.some(option => option.value === current.dashboard.refresh_seconds)) return '请选择有效刷新间隔'
  if (!agentFilterOptions.some(option => option.value === current.dashboard.agent_filter)) return '请选择有效 Agent 类型'
  const seen = new Set<string>()
  for (const [index, tip] of current.custom_knowledge_tips.entries()) {
    if (!tip.title.trim() || !tip.theory.trim()) return `自定义知识点 ${index + 1}：标题和知识正文不能为空`
    if (!tip.id.startsWith('custom_') || !tip.id.slice(7).trim() || tip.id.trim() !== tip.id || seen.has(tip.id)) return `自定义知识点 ${index + 1}：标识无效或重复`
    seen.add(tip.id)
  }
  const ranges = [
    ['Active 会话上限', current.dashboard.max_active, 1, 10, true],
    ['Inactive 会话上限', current.dashboard.max_inactive, 0, 10, true],
    ['保护时长', current.attention.protect_minutes, 1, 120, false],
    ['每次专注增加', current.attention.focus_add_minutes, 1, 120, false],
    ['遮罩强度', current.attention.mask_strength, 0.25, 1.5, false],
    ['每条消息分钟', current.time_estimate.minutes_per_message, 0.5, 30, false],
    ['保守估算比例', current.time_estimate.fallback_ratio, 0.05, 1, false],
  ] as const
  for (const [name, number, min, max, integer] of ranges) {
    if (!Number.isFinite(number) || number < min || number > max || (integer && !Number.isInteger(number))) return `${name}需为 ${min} 至 ${max} 的有效${integer ? '整数' : '数值'}`
  }
  return ''
})
function clone<T>(current: T): T { return JSON.parse(JSON.stringify(current)) as T }
async function readResponse(response: Response, fallback: string) {
  const body = await response.json()
  if (!response.ok) throw new Error(body.error || fallback)
  return body
}
async function loadRoots() {
  const response = await fetch('/api/v1/project-roots')
  roots.value = await readResponse(response, '项目读取失败') as ProjectRoot[]
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const response = await fetch('/api/v1/settings')
    const body = await readResponse(response, '设置加载失败') as Settings & { defaults: Settings }
    if (!body.defaults) throw new Error('设置接口缺少默认值，请更新服务后重试')
    const { defaults: baseline, ...settings } = body
    await loadRoots()
    defaults.value = baseline
    value.value = { ...settings, custom_knowledge_tips: settings.custom_knowledge_tips ?? [] }
    loadedValue.value = clone(value.value)
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '设置加载失败'
  } finally { loading.value = false }
}
async function save() {
  if (!value.value || busy.value || validationError.value) return
  saving.value = true
  error.value = ''
  try {
    const options = { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(value.value) }
    const response = await fetch('/api/v1/settings', options)
    value.value = await readResponse(response, '保存失败') as Settings
    value.value.custom_knowledge_tips ??= []
    loadedValue.value = clone(value.value)
    message.success('已保存并立即生效')
    emit('saved')
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' } finally { saving.value = false }
}
let knowledgeSequence = 0
function addKnowledgeTip() {
  if (!value.value || busy.value) return
  const tips = value.value.custom_knowledge_tips
  let id: string
  do { id = `custom_${Date.now()}_${knowledgeSequence++}` } while (tips.some((tip) => tip.id === id))
  tips.push({ id, title: '', theory: '', design: '' })
}
function removeKnowledgeTip(id: string) {
  if (!value.value || busy.value) return
  value.value.custom_knowledge_tips = value.value.custom_knowledge_tips.filter((tip) => tip.id !== id)
}
type Section = 'dashboard' | 'attention' | 'time_estimate'
function reset(section: Section) {
  if (!value.value || !defaults.value || busy.value) return
  if (!window.confirm('恢复本类默认值将替换此分类的编辑，点击保存设置后生效，继续吗？')) return
  Object.assign(value.value[section], clone(defaults.value[section]))
}
function resetField(section: Section, field: string) {
  if (!value.value || !defaults.value || busy.value) return
  const current = value.value[section] as unknown as Record<string, unknown>
  current[field] = (defaults.value[section] as unknown as Record<string, unknown>)[field]
}
function requestClose() {
  if (busy.value) return false
  if (dirty.value && !window.confirm('关闭会放弃未保存的设置，确定关闭吗？')) return false
  emit('close')
  return true
}
defineExpose({ requestClose })
async function changeProject(method: string, url: string, body: object | null) {
  if (busy.value) return
  projectBusy.value = true
  error.value = ''
  try {
    const options = { method, headers: { 'Content-Type': 'application/json' }, ...(body ? { body: JSON.stringify(body) } : {}) }
    const response = await fetch(url, options)
    await readResponse(response, '项目修改失败')
    await loadRoots()
    emit('saved')
    message.success('项目已更新，其他未保存设置保持不变')
    return true
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '项目修改失败'; return false } finally { projectBusy.value = false }
}
async function removeRoot(path: string) {
  if (!window.confirm(`移除项目“${path}”，确定继续吗？`)) return
  await changeProject('DELETE', `/api/v1/project-roots?path=${encodeURIComponent(path)}`, null)
}
async function addRoot() {
  const path = newPath.value.trim()
  if (!path) return
  const saved = await changeProject('PUT', '/api/v1/project-roots', { path, priority: 'normal' })
  if (saved) newPath.value = ''
}
async function setSlot(path: string, slot: string) {
  await changeProject('PUT', '/api/v1/project-roots/slot', { path, slot })
}

onMounted(load)
</script>

<template>
  <main class="settings">
    <p v-if="loading">正在加载设置…</p>
    <NAlert v-if="error" type="error">{{ error }}<NButton v-if="!value" size="small" @click="load">重试</NButton></NAlert>
    <NAlert v-if="validationError" type="warning">{{ validationError }}</NAlert>
    <fieldset class="settings-controls" :disabled="busy">
    <NTabs v-if="value" type="line" animated>
      <NTabPane name="dashboard" tab="通用与显示">
        <p>控制 Dashboard 的默认扫描范围、展示密度和自动刷新。保存后旧筛选失效，随后仍可调整本浏览器筛选。</p>
        <div class="setting-field"><label>扫描范围 <NSelect v-model:value="value.dashboard.window" :options="windowOptions" /></label><small v-if="defaults" class="default-value">默认：{{ defaults.dashboard.window }}<NButton size="tiny" quaternary @click="resetField('dashboard', 'window')">恢复此项</NButton></small></div>
        <div class="setting-field"><label>Active 会话上限<NInputNumber v-model:value="value.dashboard.max_active" :min="1" :max="10" /><small class="hint">每个项目最多展示的活跃会话数，至少保留 1 个。</small></label><small v-if="defaults" class="default-value">默认：{{ defaults.dashboard.max_active }}<NButton size="tiny" quaternary @click="resetField('dashboard', 'max_active')">恢复此项</NButton></small></div>
        <div class="setting-field"><label>Inactive 会话上限（0 为不展示）<NInputNumber v-model:value="value.dashboard.max_inactive" :min="0" :max="10" /></label><small v-if="defaults" class="default-value">默认：{{ defaults.dashboard.max_inactive }}<NButton size="tiny" quaternary @click="resetField('dashboard', 'max_inactive')">恢复此项</NButton></small></div>
        <div class="setting-field"><label>Agent 类型 <NSelect v-model:value="value.dashboard.agent_filter" :options="agentFilterOptions" /></label><small v-if="defaults" class="default-value">默认：{{ agentFilterOptions.find(option => option.value === defaults?.dashboard.agent_filter)?.label }}<NButton size="tiny" quaternary @click="resetField('dashboard', 'agent_filter')">恢复此项</NButton></small></div>
        <div class="setting-field"><label>自动刷新 <NSelect v-model:value="value.dashboard.refresh_seconds" :options="refreshOptions" /></label><small v-if="defaults" class="default-value">默认：{{ defaults.dashboard.refresh_seconds }}<NButton size="tiny" quaternary @click="resetField('dashboard', 'refresh_seconds')">恢复此项</NButton></small></div>
        <div class="setting-field"><label class="switch">每日引导 <NSwitch v-model:value="value.dashboard.daily_boot_enabled" /></label><small v-if="defaults" class="default-value">默认：{{ defaults.dashboard.daily_boot_enabled ? '开启' : '关闭' }}<NButton size="tiny" quaternary @click="resetField('dashboard', 'daily_boot_enabled')">恢复此项</NButton></small></div>
        <NButton @click="reset('dashboard')">恢复本类默认值</NButton>
      </NTabPane>
      <NTabPane name="attention" tab="注意力与专注">
        <p>这些高级数值影响后续专注和提醒的感知方式，点击保存设置后生效。</p>
        <div class="setting-field"><label>保护时长（分钟）<NInputNumber v-model:value="value.attention.protect_minutes" :min="1" :max="120" /><small class="hint">未点击「专注」但仍在密集操作时，按该时长为当前项目提供保护，期间其他项目的提醒被压制。</small></label><small v-if="defaults" class="default-value">默认：{{ defaults.attention.protect_minutes }}<NButton size="tiny" quaternary @click="resetField('attention', 'protect_minutes')">恢复此项</NButton></small></div>
        <div class="setting-field"><label>每次专注增加（分钟）<NInputNumber v-model:value="value.attention.focus_add_minutes" :min="1" :max="120" /><small class="hint">在 Dashboard 每点一次「专注 +」，该项目的专注时长就延长这么多分钟。</small></label><small v-if="defaults" class="default-value">默认：{{ defaults.attention.focus_add_minutes }}<NButton size="tiny" quaternary @click="resetField('attention', 'focus_add_minutes')">恢复此项</NButton></small></div>
        <div class="setting-field"><label>遮罩强度 <NInputNumber v-model:value="value.attention.mask_strength" :min="0.25" :max="1.5" :step="0.05" /><small class="hint">迷雾遮罩的整体强度倍率，大于 1 更浓、小于 1 更淡，1 为算法的原始效果。</small></label><small v-if="defaults" class="default-value">默认：{{ defaults.attention.mask_strength }}<NButton size="tiny" quaternary @click="resetField('attention', 'mask_strength')">恢复此项</NButton></small></div>
        <NButton @click="reset('attention')">恢复本类默认值</NButton>
      </NTabPane>
      <NTabPane name="time" tab="时间估算">
        <p>这两项是多层时间计算的兜底策略：优先使用 tmux 专注记录，取不到时才逐层降级到这里。只影响估算展示与行动建议依据，不改写原始会话记录。</p>
        <div class="setting-field"><label>每条消息折算分钟 <NInputNumber v-model:value="value.time_estimate.minutes_per_message" :min="0.5" :max="30" :step="0.5" /><small class="hint">第二层兜底：项目没有专注记录时，按消息条数 × 该值估算活跃时间。</small></label><small v-if="defaults" class="default-value">默认：{{ defaults.time_estimate.minutes_per_message }}<NButton size="tiny" quaternary @click="resetField('time_estimate', 'minutes_per_message')">恢复此项</NButton></small></div>
        <div class="setting-field"><label>无消息保守估算比例 <NInputNumber v-model:value="value.time_estimate.fallback_ratio" :min="0.05" :max="1" :step="0.05" /><small class="hint">最后一层兜底：连消息条数都取不到时，按会话在线时长 × 该比例估算，避免把整段挂机算成工作时间。</small></label><small v-if="defaults" class="default-value">默认：{{ defaults.time_estimate.fallback_ratio }}<NButton size="tiny" quaternary @click="resetField('time_estimate', 'fallback_ratio')">恢复此项</NButton></small></div>
        <NButton @click="reset('time_estimate')">恢复本类默认值</NButton>
      </NTabPane>
      <NTabPane name="knowledge" tab="自定义知识点">
        <p>候选列表包含：全部行动建议关联的知识（去重）、随机 2 条未重复的内置知识、随机 2 条自定义知识，总数不限。每类不足 2 条时取全部可用内容。选好候选后统一打乱一次，后续按该顺序循环播放。</p>
        <p>修改后点击「保存设置」生效。每条知识点的应用说明可留空。</p>
        <NButton :disabled="busy" @click="addKnowledgeTip">添加知识点</NButton>
        <p v-if="value.custom_knowledge_tips.length === 0" class="knowledge-empty">尚未添加自定义知识点，当前使用内置知识池。</p>
        <div v-for="(tip, index) in value.custom_knowledge_tips" :key="tip.id" class="knowledge-editor">
          <div class="knowledge-heading"><strong>知识点 {{ index + 1 }}</strong><NButton size="small" type="error" secondary :disabled="busy" @click="removeKnowledgeTip(tip.id)">删除</NButton></div>
          <label>标题（必填）<NInput v-model:value="tip.title" :disabled="busy" placeholder="例如：先完成最重要的一步" /></label>
          <label>知识正文（必填）<NInput v-model:value="tip.theory" :disabled="busy" type="textarea" :autosize="{ minRows: 3 }" placeholder="填写希望轮播展示的知识内容" /></label>
          <label>应用说明（选填）<NInput v-model:value="tip.design" :disabled="busy" type="textarea" :autosize="{ minRows: 2 }" placeholder="填写如何在工作中应用" /></label>
        </div>
      </NTabPane>
      <NTabPane name="projects" tab="项目管理">
        <p>项目调整即时保存，不影响其他未保存设置。</p>
        <p v-if="roots.length === 0">尚未标记项目，可在 Dashboard 卡片中标记后在此管理。</p>
        <div class="add-project"><NInput v-model:value="newPath" placeholder="/path/to/project" /><NButton @click="addRoot">添加项目</NButton></div>
        <div v-for="root in roots" :key="root.path" class="project"><code>{{ root.path }}</code><span>{{ root.priority }}{{ root.slot ? ` · ${root.slot}` : '' }}</span><NButton size="tiny" @click="setSlot(root.path, 'primary')">主线</NButton><NButton size="tiny" @click="setSlot(root.path, 'secondary_1')">支线 1</NButton><NButton size="tiny" @click="setSlot(root.path, 'secondary_2')">支线 2</NButton><NButton size="small" type="error" secondary @click="removeRoot(root.path)">移除</NButton></div>
      </NTabPane>
    </NTabs>
    <footer v-if="value"><span>{{ dirty ? '有未保存修改' : '已保存' }}</span><NButton @click="requestClose">关闭</NButton><NButton type="primary" :loading="saving" :disabled="!!validationError" @click="save">保存设置</NButton></footer>
    </fieldset>
  </main>
</template>

<style scoped>
.settings { display: flex; flex-direction: column; gap: 16px; }
.knowledge-editor { margin-top: 16px; padding: 16px; border: 1px solid var(--n-divider-color); border-radius: 8px; }
.knowledge-heading { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.knowledge-editor label { max-width: none; margin: 12px 0; }
.knowledge-empty { padding-top: 16px; }
.settings-controls { border: 0; margin: 0; padding: 0; min-width: 0; }
.setting-field { max-width: 420px; margin: 18px 0 24px; }
.setting-field label { margin: 0; }
.default-value { margin-top: 6px; display: flex; gap: 8px; align-items: center; color: var(--n-text-color-3); }
.settings > p { margin: 0; }
.settings :deep(.n-tab-pane) p { margin: 0 0 4px; opacity: 0.7; }
label { display: grid; gap: 6px; max-width: 420px; margin: 18px 0 24px; } .switch { display: flex; align-items: center; gap: 12px; }
.hint { font-size: 12px; line-height: 1.6; opacity: 0.65; }
.project, .add-project { display: flex; align-items: center; gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--n-divider-color); } .project code { flex: 1; } .add-project :deep(.n-input) { max-width: 460px; }
footer { display: flex; gap: 12px; align-items: center; justify-content: flex-end; }
@media (max-width: 760px) { .project { flex-wrap: wrap; } .project code { flex-basis: 100%; overflow-wrap: anywhere; } }
</style>
