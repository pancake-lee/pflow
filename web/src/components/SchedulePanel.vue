<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton } from 'naive-ui'
type Item={id:string;start:string;title:string;focus_minutes:number;break_minutes:number;status?:string}
const day=ref(''); const current=ref<Item|null>(null); const next=ref<Item|null>(null)
async function load(){const r=await fetch('/api/v1/schedules');if(!r.ok)return;const b=await r.json();day.value=b.day.date;current.value=b.current;next.value=b.next}
async function act(action:string){if(!current.value||!current.value.id)return;await fetch('/api/v1/schedules/action',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({date:day.value,id:current.value.id,action})});load()}
onMounted(load)
</script>
<template><section v-if="current" class="schedule"><div><strong>今日日程</strong><span class="current">{{ current.title }}</span><small>{{ current.start }} · {{ current.focus_minutes }} 分钟</small></div><div v-if="next" class="next">下一项：{{ next.start }} {{ next.title }}</div><div v-if="current.id"><NButton size="small" @click="act('completed')">完成</NButton><NButton size="small" @click="act('skipped')">跳过</NButton></div></section></template>
<style scoped>.schedule{display:flex;gap:16px;align-items:center;margin:16px 0;padding:12px 16px;border:1px solid rgba(94,163,240,.35);border-radius:10px;background:rgba(32,128,240,.08)}.schedule>div{display:flex;gap:8px;align-items:center}.current{color:#8bbdff}.next,small{color:#aaa}@media(max-width:760px){.schedule{align-items:flex-start;flex-direction:column}}</style>
