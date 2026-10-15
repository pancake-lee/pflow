import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref, computed, watch, reactive, nextTick, effectScope } from 'vue'

const source = readFileSync(new URL('../src/components/KnowledgeAnchor.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/^import .*$/gm, '')
const tip = (id) => ({ id, title: id, theory: `Theory ${id}`, design: '' })
function anchor(suggestions = []) {
  const intervals = new Map(), timeouts = new Map()
  let serial = 0, mount, unmount
  const props = reactive({ suggestions })
  const scope = effectScope()
  const context = vm.createContext({
    ref, computed, watch, defineProps: () => props,
    GENERIC_TIPS: [tip('a'), tip('b')],
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
  assert.equal(api.run('visibleTip.value.id'), 'a')
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
  assert.equal(api.run('totalPages.value'), 1)
  api.run('goNext()')
  assert.equal(api.run('currentPage.value'), 1)
  api.props.suggestions = []
  await nextTick()
  assert.equal(api.run('totalPages.value'), 2)
  assert.equal(api.intervals.size, 1)
  api.run('goNext()')
  api.props.suggestions = [{ knowledge_tip: tip('new') }]
  await nextTick()
  assert.equal(api.run('visibleTip.value.id'), 'new')
  assert.equal(api.run('currentPage.value'), 1)
  api.resume()
  assert.equal(api.intervals.size, 0)
  api.dispose()
})
