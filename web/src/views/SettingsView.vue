<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NCard, NInputNumber, NSelect, NSwitch, NTabPane, NTabs, useMessage } from 'naive-ui'

type Settings = {
  version: number
  dashboard: { window: string; max_inactive: number; refresh_seconds: number; daily_boot_enabled: boolean }
  attention: { protect_minutes: number; focus_add_minutes: number; mask_strength: number }
  time_estimate: { minutes_per_message: number; fallback_ratio: number }
}
type ProjectRoot = { path: string; priority: string; slot?: string }

const emit = defineEmits<{ close: [] }>()
const message = useMessage()
const value = ref<Settings | null>(null)
const roots = ref<ProjectRoot[]>([])
const loading = ref(true)
const saving = ref(false)
const windowOptions = ['1h', '3h', '6h', '1d', '3d', '7d'].map(value => ({ label: value, value }))
const refreshOptions = [0, 10, 30, 60].map(value => ({ label: value === 0 ? '关闭' : `${value}s`, value }))

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

onMounted(load)
</script>

<template>
  <main class="settings-page">
    <header><NButton quaternary @click="emit('close')">← 返回 Dashboard</NButton><h1>设置</h1></header>
    <NCard v-if="loading">正在加载设置…</NCard>
    <NTabs v-else-if="value" type="line" animated>
      <NTabPane name="dashboard" tab="通用与显示">
        <p>控制 Dashboard 的默认扫描范围、展示密度和自动刷新。</p>
        <label>扫描范围 <NSelect v-model:value="value.dashboard.window" :options="windowOptions" /></label>
        <label>Inactive 会话上限（0 为不限）<NInputNumber v-model:value="value.dashboard.max_inactive" :min="0" :max="10" /></label>
        <label>自动刷新 <NSelect v-model:value="value.dashboard.refresh_seconds" :options="refreshOptions" /></label>
        <label class="switch">每日引导 <NSwitch v-model:value="value.dashboard.daily_boot_enabled" /></label>
        <NButton @click="reset('dashboard')">恢复本类默认值</NButton>
      </NTabPane>
      <NTabPane name="attention" tab="注意力与专注">
        <p>这些高级数值影响后续专注和提醒的感知方式。</p>
        <label>保护时长（分钟）<NInputNumber v-model:value="value.attention.protect_minutes" :min="1" :max="120" /></label>
        <label>每次专注增加（分钟）<NInputNumber v-model:value="value.attention.focus_add_minutes" :min="1" :max="120" /></label>
        <label>遮罩强度 <NInputNumber v-model:value="value.attention.mask_strength" :min="0.25" :max="1.5" :step="0.05" /></label>
        <NButton @click="reset('attention')">恢复本类默认值</NButton>
      </NTabPane>
      <NTabPane name="time" tab="时间估算">
        <p>只影响估算展示与行动建议依据，不改写原始会话记录。</p>
        <label>每条消息折算分钟 <NInputNumber v-model:value="value.time_estimate.minutes_per_message" :min="0.5" :max="30" :step="0.5" /></label>
        <label>无消息保守估算比例 <NInputNumber v-model:value="value.time_estimate.fallback_ratio" :min="0.05" :max="1" :step="0.05" /></label>
        <NButton @click="reset('time_estimate')">恢复本类默认值</NButton>
      </NTabPane>
      <NTabPane name="projects" tab="项目管理">
        <p v-if="roots.length === 0">尚未标记项目，可在 Dashboard 卡片中标记后在此管理。</p>
        <div v-for="root in roots" :key="root.path" class="project"><code>{{ root.path }}</code><span>{{ root.priority }}{{ root.slot ? ` · ${root.slot}` : '' }}</span><NButton size="small" type="error" secondary @click="removeRoot(root.path)">移除</NButton></div>
      </NTabPane>
    </NTabs>
    <footer v-if="value"><NButton type="primary" :loading="saving" @click="save">保存设置</NButton></footer>
  </main>
</template>

<style scoped>
.settings-page { max-width: 900px; margin: 0 auto; min-height: 100vh; padding: 28px; color: #e8e8ec; background: #1a1a1c; }
header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; } h1 { margin: 0; }
label { display: grid; gap: 6px; max-width: 420px; margin: 18px 0; } .switch { display: flex; align-items: center; gap: 12px; }
.project { display: flex; align-items: center; gap: 12px; padding: 12px 0; border-bottom: 1px solid #34343a; } .project code { flex: 1; } footer { margin-top: 24px; }
</style>
