<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NAlert, NButton, NInput, NModal } from 'naive-ui'

const emit = defineEmits<{ ready: [] }>()
const serverURL = ref('')
const savedURL = ref('')
const showEditor = ref(false)
const loading = ref(false)
const error = ref('')

async function load() {
  try {
    const response = await fetch('/desktop/connection')
    if (!response.ok) throw new Error('无法读取连接配置')
    const value = await response.json() as { server_url: string }
    savedURL.value = value.server_url
    serverURL.value = value.server_url || 'http://127.0.0.1:10002'
    showEditor.value = !value.server_url
    if (value.server_url) emit('ready')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '无法读取连接配置'
    showEditor.value = true
  }
}

async function save() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const response = await fetch('/desktop/connection', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ server_url: serverURL.value }),
    })
    const value = await response.json() as { error?: string }
    if (!response.ok) throw new Error(value.error || '保存失败')
    window.location.reload()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '保存失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="connection-bar">
    <span>服务：{{ savedURL || '尚未配置' }}</span>
    <NButton size="small" @click="showEditor = true">连接设置</NButton>
  </div>
  <NModal
    v-model:show="showEditor"
    preset="card"
    title="连接 pflow 服务"
    :style="{ width: 'min(540px, calc(100vw - 32px))' }"
    :mask-closable="false"
    :closable="!!savedURL"
  >
    <p>填写 Linux、Docker 或 WSL 上的 pflow 服务地址。项目、日程和设置保存到该服务。</p>
    <label for="server-url">服务地址</label>
    <NInput
      id="server-url"
      v-model:value="serverURL"
      placeholder="http://127.0.0.1:10002"
      @keyup.enter="save"
    />
    <p class="hint">使用 SSH 隧道时填写本地转发地址。桌面版暂不支持 Web 终端，请通过已有终端操作 Agent。</p>
    <NAlert v-if="error" type="error" class="connection-error">{{ error }}</NAlert>
    <NButton type="primary" :loading="loading" :disabled="!serverURL.trim()" @click="save">
      保存并连接
    </NButton>
  </NModal>
</template>

<style scoped>
.connection-bar { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 8px 20px; background: #202024; color: #ddd; }
.connection-bar span { overflow-wrap: anywhere; }
label { display: block; margin-bottom: 8px; }
.hint { opacity: .7; font-size: 12px; }
.connection-error { margin-bottom: 16px; }
</style>
