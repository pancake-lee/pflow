<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NInput, NInputNumber, NSelect, NSwitch, NTabPane, NTabs, useMessage } from 'naive-ui'

type Settings = {
  version: number
  dashboard: { window: string; max_active: number; max_inactive: number; refresh_seconds: number; daily_boot_enabled: boolean }
  attention: { protect_minutes: number; focus_add_minutes: number; mask_strength: number }
  time_estimate: { minutes_per_message: number; fallback_ratio: number }
}
type ProjectRoot = { path: string; priority: string; slot?: string }

const emit = defineEmits<{ saved: [] }>()
const message = useMessage()
const value = ref<Settings | null>(null)
const roots = ref<ProjectRoot[]>([])
const loading = ref(true)
const saving = ref(false)
const windowOptions = ['1h', '3h', '6h', '1d', '3d', '7d'].map(value => ({ label: value, value }))
const refreshOptions = [0, 10, 30, 60].map(value => ({ label: value === 0 ? '关闭' : `${value}s`, value }))
const newPath = ref('')

async function load() {
  loading.value = true
  try {
    const [settingsResp, rootsResp] = await Promise.all([fetch('/api/v1/settings'), fetch('/api/v1/project-roots')])
    if (!settingsResp.ok) throw new Error(`设置加载失败：${settingsResp.status}`)
    value.value = await settingsResp.json() as Settings
    if (rootsResp.ok) roots.value = await rootsResp.json() as ProjectRoot[]
  } catch (error) {
    message.error(error instanceof Error ? error.message : '设置加载失败')
  } finally { loading.value = false }
}

async function save() {
  if (!value.value) return
  saving.value = true
  try {
    const resp = await fetch('/api/v1/settings', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(value.value) })
    const body = await resp.json()
    if (!resp.ok) throw new Error(body.error || '保存失败')
    value.value = body as Settings
    message.success('已保存并立即生效')
    emit('saved')
  } catch (error) { message.error(error instanceof Error ? error.message : '保存失败') } finally { saving.value = false }
}

async function reset(section: string) {
  try {
    const resp = await fetch('/api/v1/settings/reset', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ section }) })
    const body = await resp.json()
    if (!resp.ok) throw new Error(body.error || '恢复默认失败')
    value.value = body as Settings
    message.success('已恢复默认值')
  } catch (error) { message.error(error instanceof Error ? error.message : '恢复默认失败') }
}

async function removeRoot(path: string) {
  const resp = await fetch(`/api/v1/project-roots?path=${encodeURIComponent(path)}`, { method: 'DELETE' })
  if (!resp.ok) { message.error('移除项目失败'); return }
  roots.value = roots.value.filter(root => root.path !== path)
  message.success('已移除项目')
}

async function addRoot() {
  const path = newPath.value.trim()
  if (!path) return
  const resp = await fetch('/api/v1/project-roots', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path, priority: 'normal' }) })
  if (!resp.ok) { message.error('添加项目失败'); return }
  newPath.value = ''
  await load()
}

async function setSlot(path: string, slot: string) {
  const resp = await fetch('/api/v1/project-roots/slot', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path, slot }) })
  if (!resp.ok) { message.error('调整项目策略失败'); return }
  await load()
  message.success('项目策略已更新')
}

onMounted(load)
</script>

<template>
  <main class="settings">
    <p v-if="loading">正在加载设置…</p>
    <NTabs v-else-if="value" type="line" animated>
      <NTabPane name="dashboard" tab="通用与显示">
        <p>控制 Dashboard 的默认扫描范围、展示密度和自动刷新。</p>
        <label>扫描范围 <NSelect v-model:value="value.dashboard.window" :options="windowOptions" /></label>
        <label>Active 会话上限<NInputNumber v-model:value="value.dashboard.max_active" :min="1" :max="10" /><small class="hint">每个项目最多展示的活跃会话数，至少保留 1 个。</small></label>
        <label>Inactive 会话上限（0 为不展示）<NInputNumber v-model:value="value.dashboard.max_inactive" :min="0" :max="10" /></label>
        <label>自动刷新 <NSelect v-model:value="value.dashboard.refresh_seconds" :options="refreshOptions" /></label>
        <label class="switch">每日引导 <NSwitch v-model:value="value.dashboard.daily_boot_enabled" /></label>
        <NButton @click="reset('dashboard')">恢复本类默认值</NButton>
      </NTabPane>
      <NTabPane name="attention" tab="注意力与专注">
        <p>这些高级数值影响后续专注和提醒的感知方式。</p>
        <label>保护时长（分钟）<NInputNumber v-model:value="value.attention.protect_minutes" :min="1" :max="120" /><small class="hint">未点击「专注」但仍在密集操作时，按该时长为当前项目提供保护，期间其他项目的提醒被压制。</small></label>
        <label>每次专注增加（分钟）<NInputNumber v-model:value="value.attention.focus_add_minutes" :min="1" :max="120" /><small class="hint">在 Dashboard 每点一次「专注 +」，该项目的专注时长就延长这么多分钟。</small></label>
        <label>遮罩强度 <NInputNumber v-model:value="value.attention.mask_strength" :min="0.25" :max="1.5" :step="0.05" /><small class="hint">迷雾遮罩的整体强度倍率，大于 1 更浓、小于 1 更淡，1 为算法的原始效果。</small></label>
        <NButton @click="reset('attention')">恢复本类默认值</NButton>
      </NTabPane>
      <NTabPane name="time" tab="时间估算">
        <p>这两项是多层时间计算的兜底策略：优先使用 tmux 专注记录，取不到时才逐层降级到这里。只影响估算展示与行动建议依据，不改写原始会话记录。</p>
        <label>每条消息折算分钟 <NInputNumber v-model:value="value.time_estimate.minutes_per_message" :min="0.5" :max="30" :step="0.5" /><small class="hint">第二层兜底：项目没有专注记录时，按消息条数 × 该值估算活跃时间。</small></label>
        <label>无消息保守估算比例 <NInputNumber v-model:value="value.time_estimate.fallback_ratio" :min="0.05" :max="1" :step="0.05" /><small class="hint">最后一层兜底：连消息条数都取不到时，按会话在线时长 × 该比例估算，避免把整段挂机算成工作时间。</small></label>
        <NButton @click="reset('time_estimate')">恢复本类默认值</NButton>
      </NTabPane>
      <NTabPane name="projects" tab="项目管理">
        <p v-if="roots.length === 0">尚未标记项目，可在 Dashboard 卡片中标记后在此管理。</p>
        <div class="add-project"><NInput v-model:value="newPath" placeholder="/path/to/project" /><NButton @click="addRoot">添加项目</NButton></div>
        <div v-for="root in roots" :key="root.path" class="project"><code>{{ root.path }}</code><span>{{ root.priority }}{{ root.slot ? ` · ${root.slot}` : '' }}</span><NButton size="tiny" @click="setSlot(root.path, 'primary')">主线</NButton><NButton size="tiny" @click="setSlot(root.path, 'secondary_1')">支线 1</NButton><NButton size="tiny" @click="setSlot(root.path, 'secondary_2')">支线 2</NButton><NButton size="small" type="error" secondary @click="removeRoot(root.path)">移除</NButton></div>
      </NTabPane>
    </NTabs>
    <footer v-if="value"><NButton type="primary" :loading="saving" @click="save">保存设置</NButton></footer>
  </main>
</template>

<style scoped>
.settings { display: flex; flex-direction: column; gap: 16px; }
.settings > p { margin: 0; }
.settings :deep(.n-tab-pane) p { margin: 0 0 4px; opacity: 0.7; }
label { display: grid; gap: 6px; max-width: 420px; margin: 18px 0 24px; } .switch { display: flex; align-items: center; gap: 12px; }
.hint { font-size: 12px; line-height: 1.6; opacity: 0.65; }
.project, .add-project { display: flex; align-items: center; gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--n-divider-color); } .project code { flex: 1; } .add-project :deep(.n-input) { max-width: 460px; }
footer { display: flex; justify-content: flex-end; }
</style>
