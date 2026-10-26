import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref, computed } from 'vue'

const source = readFileSync(new URL('../src/components/SchedulePanel.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '')
function panel(fetch) {
  const errors = []
  const context = vm.createContext({
    ref, computed, fetch, document: { visibilityState: 'visible' }, setInterval, clearInterval, onMounted: () => {}, onUnmounted: () => {},
    defineEmits: () => () => {}, useMessage: () => ({ error: message => errors.push(message), success: () => {} }),
    highlightToMarquee: () => ({ visible: false }),
  })
  vm.runInContext(ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context)
  vm.runInContext("current.value = {id: 'a', title: 'A'}; day.value = '2026-10-13'", context)
  return { run: code => vm.runInContext(code, context), errors }
}
test('a failed action gives feedback and retains the current item for retry', async () => {
  const api = panel(async () => { throw new Error('offline') })
  await api.run("act('completed')")
  assert.equal(api.run('current.value.id'), 'a')
  assert.equal(api.run('acting.value'), false)
  assert.equal(api.errors.length, 1)
})
test('an in-flight action prevents duplicate completion and reloads after success', async () => {
  let release, posts = 0
  const api = panel(async (_, options) => {
    if (options) {
      posts++
      if (JSON.parse(options.body).action === 'completed') await new Promise(resolve => { release = resolve })
      return { ok: true, json: async () => ({ events: [] }) }
    }
    return { ok: true, json: async () => ({ day: {date:'2026-10-13'}, current:{id:'b',title:'B'}, phase:'focus' }) }
  })
  const first = api.run("act('completed')")
  assert.equal(api.run('acting.value'), true)
  await api.run("act('skipped')")
  assert.equal(posts, 1)
  release()
  await first
  assert.equal(api.run('current.value.id'), 'b')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(posts, 2)
  assert.equal(api.run('acting.value'), false)
})

test('automatic start retries after failure and restarts after a pause', async () => {
  let posts = 0
  const api = panel(async (_, options) => {
    if (options) {
      posts++
      return { ok: posts !== 1, json: async () => ({ events: [] }) }
    }
    return { ok: true, json: async () => ({ day: { date: '2026-10-13' }, current: { id: 'a', title: 'A' }, phase: 'focus' }) }
  })
  api.run('maybeAutoStart()')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(posts, 1)
  api.run('maybeAutoStart()')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(posts, 2)
  api.run("events.value = [{ id: 'pause-1', task_id: 'a', action: 'pause', time: '2026-10-13T09:10:00Z', duration_seconds: 600 }]")
  api.run('maybeAutoStart()')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(posts, 3)
})

test('review date change saves the previous day note before loading the next day', async () => {
  const notes = { '2026-10-13': '原备注', '2026-10-14': '次日备注' }
  const calls = []
  const api = panel(async (url, options) => {
    if (options?.method === 'PUT') {
      const body = JSON.parse(options.body)
      calls.push(`save:${body.date}`)
      notes[body.date] = body.note
      return { ok: true, json: async () => ({ note: body.note }) }
    }
    const date = new URL(url, 'http://localhost').searchParams.get('date')
    calls.push(`load:${date}`)
    return { ok: true, json: async () => ({ review: { score: 0 }, note: notes[date] ?? '' }) }
  })
  api.run("reviewDate.value = '2026-10-13'; review.value = {score: 0}; reviewNote.value = '原备注'; storedReviewNote.value = '原备注'")
  api.run("reviewNote.value = '改后备注'")
  await api.run("changeReviewDate('2026-10-14')")
  assert.deepEqual(calls, ['save:2026-10-13', 'load:2026-10-14'])
  assert.equal(notes['2026-10-13'], '改后备注')
  assert.equal(api.run('reviewNote.value'), '次日备注')
  assert.equal(api.run('reviewDate.value'), '2026-10-14')
})

test('review stays on the original day when automatic save fails', async () => {
  let reads = 0
  const api = panel(async (_, options) => {
    if (options?.method === 'PUT') return { ok: false }
    reads++
    return { ok: true, json: async () => ({ review: { score: 0 }, note: '' }) }
  })
  api.run("reviewDate.value = '2026-10-13'; reviewNote.value = '未保存'; storedReviewNote.value = ''; showReview.value = true")
  await api.run("changeReviewDate('2026-10-14')")
  await api.run('setReviewVisible(false)')
  assert.equal(api.run('reviewDate.value'), '2026-10-13')
  assert.equal(api.run('showReview.value'), true)
  assert.equal(reads, 0)
  assert.equal(api.errors.length, 2)
})

test('parking note stays yellow until processed and disappears only when deleted', async () => {
  const entries = []
  const actions = []
  const api = panel(async (url, options) => {
    if (url.endsWith('/parking')) {
      const body = JSON.parse(options.body)
      actions.push(options.method)
      if (options.method === 'POST') entries.push({ id: 'note-1', content: body.content, created_at: '2026-10-13T09:00:00Z' })
      if (options.method === 'PATCH') entries[0].processed_at = '2026-10-13T09:10:00Z'
      if (options.method === 'DELETE') entries.splice(0, 1)
      return { ok: true }
    }
    return { ok: true, json: async () => ({ day: { date: '2026-10-13', parking: [...entries] }, current: { id: 'a', title: 'A' }, phase: 'focus' }) }
  })
  api.run("parkingText.value = '待处理便签'")
  await api.run('addParking()')
  assert.equal(api.run('parkingProcessed(parking.value[0])'), false)
  assert.equal(api.run('current.value.id'), 'a')
  await api.run("processParking('note-1')")
  assert.equal(api.run('parkingProcessed(parking.value[0])'), true)
  await api.run("deleteParking('note-1')")
  assert.equal(api.run('parking.value.length'), 0)
  assert.deepEqual(actions, ['POST', 'PATCH', 'DELETE'])
})

test('leave remains active after reload until the return action succeeds', async () => {
  let away = false
  const leave = { id: 'leave-1', task_id: 'a', action: 'leave', time: '2026-10-13T09:00:00Z', note: '接电话' }
  const api = panel(async (url, options) => {
    if (url.endsWith('/action')) {
      const action = JSON.parse(options.body).action
      if (action === 'leave') away = true
      if (action === 'return') away = false
      return { ok: true, json: async () => ({ events: action === 'leave' ? [leave] : [] }) }
    }
    return { ok: true, json: async () => ({ day: { date: '2026-10-13', items: [{ id: 'a', title: 'A' }] }, current: { id: 'a', title: 'A' }, active_leave: away ? leave : null, phase: 'focus' }) }
  })
  api.run('openLeave()')
  await api.run("act('leave', 'a', '接电话')")
  assert.equal(api.run('activeLeave.value.note'), '接电话')
  assert.equal(api.run('leaveOpen.value'), false)
  await api.run('load()')
  assert.equal(api.run('activeLeave.value.id'), 'leave-1')
  await api.run("act('return', 'a')")
  assert.equal(api.run('activeLeave.value'), null)
})

test('jump choices include the next item and display their live point cost', () => {
  const api = panel(async () => ({ ok: true }))
  api.run("items.value = [{id:'a',title:'A'}, {id:'b',title:'B'}, {id:'c',title:'C'}]; current.value = {id:'c',title:'C'}; next.value = {id:'a',title:'A'}; jumpPoints.value = {a:0,b:-3}")
  assert.deepEqual(Array.from(api.run('reorderOptions.value'), option => option.label), ['A · 0 分', 'B · −3 分'])
  assert.equal(api.run('formatStandardScore(35)'), '35/100')
  assert.equal(api.run('formatStandardScore(null)'), '—')
  assert.equal(api.run("eventPoints({action:'reorder',points:0})"), 0)
  assert.equal(api.run("eventPoints({action:'reorder'})"), -3)
})

test('completion preview changes at the plan boundary and the next card has no completion action', () => {
  const api = panel(async () => ({ ok: true }))
  api.run("completionWindow.value = {start:'2026-10-25T09:00:00Z',end:'2026-10-25T09:30:00Z'}")
  for (const [clock, points] of [
    ['2026-10-25T08:59:59Z', -3],
    ['2026-10-25T09:00:00Z', 10],
    ['2026-10-25T09:20:00Z', 10],
    ['2026-10-25T09:29:59Z', 10],
    ['2026-10-25T09:30:00Z', -3],
  ]) {
    api.run(`now.value = Date.parse('${clock}')`)
    assert.equal(api.run('completionPoints.value'), points)
  }
  assert.equal(readFileSync(new URL('../src/components/SchedulePanel.vue', import.meta.url), 'utf8').includes('提前完成'), false)
})
