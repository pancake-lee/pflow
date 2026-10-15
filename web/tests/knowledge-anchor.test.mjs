import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref, computed, watch, reactive, nextTick, effectScope, createSSRApp } from 'vue'
import { renderToString } from 'vue/server-renderer'

const source = readFileSync(new URL('../src/components/KnowledgeAnchor.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/^import .*$/gm, '')
const knowledgeSource = readFileSync(new URL('../src/config/knowledge-tips.ts', import.meta.url), 'utf8')
  .replace(/^import .*$/gm, '').replace(/export /g, '')
const knowledge = vm.createContext({})
vm.runInContext(ts.transpileModule(knowledgeSource, {
  compilerOptions: { target: ts.ScriptTarget.ES2022 },
}).outputText, knowledge)
const selectKnowledgeTips = vm.runInContext('selectKnowledgeTips', knowledge)
const getAssociatedTips = vm.runInContext('getAssociatedTips', knowledge)
const tip = (id) => ({ id, title: id, theory: `Theory ${id}`, design: '' })
function anchor(suggestions = []) {
  const intervals = new Map(), timeouts = new Map()
  let serial = 0, mount, unmount
  const props = reactive({ suggestions })
  const scope = effectScope()
  const context = vm.createContext({
    ref, computed, watch, defineProps: () => props,
    ALL_TIPS: [tip('a'), tip('b')], getAssociatedTips,
    selectKnowledgeTips: (suggestions, pool) => selectKnowledgeTips(suggestions, pool, () => 0),
    onMounted: (fn) => { mount = fn }, onUnmounted: (fn) => { unmount = fn },
    setInterval: (fn, delay) => { intervals.set(++serial, { fn, delay }); return serial },
    clearInterval: (id) => intervals.delete(id),
    setTimeout: (fn, delay) => { timeouts.set(++serial, { fn, delay }); return serial },
    clearTimeout: (id) => timeouts.delete(id),
  })
  scope.run(() => vm.runInContext(ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText, context))
  mount()
  return {
    props, intervals, timeouts,
    run: (code) => vm.runInContext(code, context),
    dispose() { unmount(); scope.stop() },
    tick() { for (const { fn } of [...intervals.values()]) fn() },
    resume() { const pending = [...timeouts.values()]; timeouts.clear(); for (const { fn } of pending) fn() },
  }
}

test('rotation, wraparound navigation, pause timing and cleanup use one list', () => {
  const api = anchor()
  assert.equal(api.intervals.values().next().value.delay, 15000)
  assert.ok(['a', 'b'].includes(api.run('visibleTip.value.id')))
  api.tick()
  assert.equal(api.run('visibleTip.value.id'), 'b')
  api.tick()
  assert.equal(api.run('visibleTip.value.id'), 'a')
  api.run('goPrev()')
  assert.equal(api.run('visibleTip.value.id'), 'b')
  assert.equal(api.run('currentPage.value'), 2)
  assert.equal(api.run('manualPause.value'), true)
  assert.equal(api.intervals.size, 0)
  assert.equal(api.timeouts.values().next().value.delay, 30000)
  api.run('goNext()')
  assert.equal(api.timeouts.size, 1)
  api.resume()
  assert.equal(api.run('manualPause.value'), false)
  assert.equal(api.intervals.size, 1)
  api.run('goNext()')
  api.dispose()
  assert.equal(api.intervals.size, 0)
  assert.equal(api.timeouts.size, 0)
})

test('associated content and changing lists keep navigation and page counts consistent', async () => {
  const api = anchor([{ knowledge_tip: tip('associated') }])
  assert.equal(api.run('visibleTip.value.id'), 'associated')
  assert.equal(api.run('totalPages.value'), 3)
  api.run('goNext()')
  assert.equal(api.run('currentPage.value'), 2)
  api.props.suggestions = []
  await nextTick()
  assert.equal(api.run('totalPages.value'), 2)
  assert.equal(api.intervals.size, 0, 'list changes preserve the manual pause')
  api.resume()
  assert.equal(api.intervals.size, 1)
  api.run('goNext()')
  api.props.suggestions = [{ knowledge_tip: tip('new') }]
  await nextTick()
  assert.equal(api.run('visibleTip.value.id'), 'new')
  assert.equal(api.run('currentPage.value'), 1)
  api.resume()
  assert.equal(api.intervals.size, 1)
  api.dispose()
})

const plain = (value) => JSON.parse(JSON.stringify(value))
test('all suggestions are considered, deduplicated, preferred and capped at five', () => {
  const pool = Array.from({ length: 12 }, (_, index) => tip(String(index)))
  const suggestions = [{}, { knowledge_tip: pool[3] }, { knowledge_tip: pool[3] }, { knowledge_tip: pool[8] }]
  const result = plain(selectKnowledgeTips(suggestions, pool, () => 0))
  assert.deepEqual(result.map((t) => t.id), ['3', '8', '0', '1', '2'])
  assert.deepEqual(plain(selectKnowledgeTips(pool.map((knowledge_tip) => ({ knowledge_tip })), pool)), pool.slice(0, 5))
  assert.deepEqual(plain(selectKnowledgeTips([], [pool[0], pool[0]])), [pool[0]])
  assert.deepEqual(plain(selectKnowledgeTips([], [])), [])
})

test('every built-in tip can be sampled without suggestions, including scenario tips', () => {
  const pool = vm.runInContext('ALL_TIPS', knowledge)
  for (let index = 0; index < pool.length; index++) {
    const result = selectKnowledgeTips([], pool, () => (index + 0.1) / pool.length)
    assert.equal(result.length, 5)
    assert.equal(new Set(result.map((t) => t.id)).size, 5)
    assert.equal(result[0].id, pool[index].id)
  }
})

test('equivalent polling does not reshuffle or reset playback; associated changes do', async () => {
  const api = anchor([{ knowledge_tip: tip('associated') }])
  api.tick()
  const before = api.run('rotationTips.value')
  api.props.suggestions = [{ text: 'changed wording', knowledge_tip: tip('associated') }]
  await nextTick()
  assert.equal(api.run('rotationTips.value'), before)
  assert.equal(api.run('rotationIndex.value'), 1)
  api.props.suggestions = [{}, { knowledge_tip: tip('new') }]
  await nextTick()
  assert.equal(api.run('visibleTip.value.id'), 'new')
  assert.equal(api.run('rotationIndex.value'), 0)
  api.dispose()
})

test('custom knowledge participates in selection, reacts to edits and disappears after deletion', async () => {
  const api = anchor()
  const custom = tip('custom_mine')
  custom.theory = '<img src=x onerror=alert(1)>'
  api.props.customTips = [custom]
  await nextTick()
  assert.equal(api.run('totalPages.value'), 3)
  api.run('goPrev()')
  assert.equal(api.run('visibleTip.value.id'), 'custom_mine')
  assert.equal(api.run('visibleTip.value.theory'), custom.theory)
  api.props.customTips[0].theory = 'edited'
  await nextTick()
  assert.equal(api.run('rotationTips.value.find(t => t.id === "custom_mine").theory'), 'edited')
  api.props.customTips = []
  await nextTick()
  assert.equal(api.run('totalPages.value'), 2)
  assert.equal(api.run('rotationTips.value.some(t => t.id === "custom_mine")'), false)
  assert.ok(api.run('visibleTip.value'))
  api.dispose()
})

test('the actual card template escapes custom text and omits an empty application section', async () => {
  const api = anchor()
  api.props.customTips = [{ ...tip('custom_html'), title: '<b>title</b>', theory: '<img src=x onerror=alert(1)>' }]
  await nextTick()
  api.run('goPrev()')
  const template = readFileSync(new URL('../src/components/KnowledgeAnchor.vue', import.meta.url), 'utf8')
    .match(/<template>([\s\S]*?)<\/template>/)[1]
  const app = createSSRApp({
    template,
    setup: () => api.run('({ isHovered, manualPause, modeLabel, totalPages, currentPage, visibleTip, goPrev, goNext })'),
  })
  const html = await renderToString(app)
  assert.match(html, /&lt;b&gt;title&lt;\/b&gt;/)
  assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/)
  assert.equal(html.includes('<img'), false)
  assert.equal(html.includes('ka-design'), false)
  api.dispose()
})
