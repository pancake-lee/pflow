<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NInput, NInputNumber, NSelect } from 'naive-ui'
type Item={id:string;start:string;title:string;focus_minutes:number;break_minutes:number;until_end?:boolean}
type Template={id:string;name:string;items:Item[]}
const emit=defineEmits<{ close: [] }>(); const date=ref(new Date().toISOString().slice(0,10));const items=ref<Item[]>([]);const templates=ref<Template[]>([]);const name=ref('')
function newItem(){items.value.push({id:`item-${Date.now()}`,start:'09:00',title:'新日程',focus_minutes:50,break_minutes:10})}
async function load(){const r=await fetch(`/api/v1/schedules?date=${date.value}`);if(!r.ok)return;const b=await r.json();items.value=b.day.items;templates.value=b.templates}
async function save(){await fetch('/api/v1/schedules',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({date:date.value,items:items.value})})}
async function saveTemplate(){if(!name.value)return;await fetch('/api/v1/schedule-templates',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:name.value,items:items.value})});name.value='';load()}
function applyTemplate(id:string){const template=templates.value.find(t=>t.id===id);if(template)items.value=structuredClone(template.items)}
async function delTemplate(id:string){await fetch(`/api/v1/schedule-templates?id=${encodeURIComponent(id)}`,{method:'DELETE'});load()}
onMounted(load)
</script>
<template><main class="editor"><header><NButton quaternary @click="emit('close')">← 返回 Dashboard</NButton><h2>日程编辑</h2></header><label>日期 <input v-model="date" type="date" @change="load" /></label><section v-for="(item,index) in items" :key="item.id" class="item"><NInput v-model:value="item.start" style="width:90px"/><NInput v-model:value="item.title"/><NInputNumber v-model:value="item.focus_minutes" :min="1"/> <span>分钟</span><NInputNumber v-model:value="item.break_minutes" :min="0"/> <span>break</span><NButton size="small" @click="items.splice(index,1)">删除</NButton></section><NButton @click="newItem">添加日程项</NButton><NButton type="primary" @click="save">保存当日日程</NButton><hr/><div><NSelect :options="templates.map(t=>({label:t.name,value:t.id}))" placeholder="应用模板" @update:value="applyTemplate"/><NInput v-model:value="name" placeholder="模板名称"/><NButton @click="saveTemplate">另存为模板</NButton></div><div v-for="template in templates" :key="template.id"><span>{{template.name}}</span><NButton size="tiny" @click="delTemplate(template.id)">删除模板</NButton></div></main></template>
<style scoped>.editor{max-width:960px;margin:auto;padding:28px}.editor header,.item,.editor>div{display:flex;gap:10px;align-items:center;margin:12px 0}.item :deep(.n-input){flex:1}.editor label{display:flex;gap:8px;align-items:center}</style>
