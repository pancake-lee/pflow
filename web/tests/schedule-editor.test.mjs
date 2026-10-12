import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref, computed, watch, nextTick } from 'vue'

// Execute the actual editor script with lightweight Vue and API doubles.
// DOM layout and browser drag behavior remain a separate acceptance check.
const source = readFileSync(new URL('../src/components/ScheduleEditor.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/^import .*$/gm, '')
function editor() {
  const requests = []
  const context = vm.createContext({
    ref, computed, watch, onMounted: () => {}, defineEmits: () => () => {},
    useMessage: () => ({ success() {} }), window: { confirm: () => true },
    fetch: async (...args) => { requests.push(args); return { ok: true, json: async () => ({ items: [] }) } },
  })
  vm.runInContext(ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context)
  return { run: (code) => vm.runInContext(code, context), requests }
}
const fixture = [
  { id: 'a', title: 'A', start: '22:00', focus_minutes: 50, break_minutes: 10, status: 'completed', completed_at: '2026-10-13T22:50:00Z' },
  { id: 'b', title: 'B', start: '23:00', focus_minutes: 40, break_minutes: 20 },
  { id: 'c', title: 'C', start: '00:00', day_offset: 1, focus_minutes: 30, break_minutes: 10 },
]
function setup() {
  const api = editor()
  api.run(`items.value = ${JSON.stringify(fixture)}; loadedItems.value = cloneItems(items.value)`)
  return api
}
function plain(value) { return JSON.parse(JSON.stringify(value)) }

for (const mode of ['content', 'item']) {
  for (const [from, gap, order] of [[2, 0, 'CAB'], [0, 3, 'BCA'], [1, 0, 'BAC'], [1, 3, 'ACB'], [0, 0, 'ABC'], [0, 1, 'ABC'], [2, 3, 'ABC'], [-1, 0, 'ABC'], [0, 4, 'ABC']]) {
    test(`${mode}: source ${from}, insertion gap ${gap}`, () => {
      const api = setup()
      const result = plain(api.run(`moveItems(items.value, ${from}, ${gap}, '${mode}')`))
      assert.equal(result.map((item) => item.title).join(''), order)
      assert.deepEqual(plain(api.run('items.value')), fixture, 'original list must not be mutated')
      if (mode === 'content') {
        assert.deepEqual(result.map(({ title, ...rest }) => rest), fixture.map(({ title, ...rest }) => rest))
      } else {
        for (const item of result) assert.deepEqual(item, fixture.find((original) => original.id === item.id))
      }
    })
  }
}
test('empty list and cross-midnight timeline', () => {
  const api = setup()
  assert.equal(api.run('validateTimeline([])'), '')
  assert.deepEqual(plain(api.run("moveItems([], 0, 0, 'item')")), [])
  assert.equal(api.run('timelineError.value'), '')
  api.run("items.value = moveItems(items.value, 2, 0, 'item')")
  assert.match(api.run('timelineError.value'), /乱序/)
  api.run('sortItems()')
  assert.equal(api.run('timelineError.value'), '')
})
for (const [edit, message] of [
  ["items.value[1].start = '22:30'", '重叠'],
  ["items.value[1].start = '22:00'", '乱序'],
  ["items.value[1].start = '25:00'", '有效开始时间'],
  ["items.value[1].start = ''", '有效开始时间'],
  ["items.value[1].focus_minutes = null", '有效的专注'],
  ["items.value[1].focus_minutes = 1.5", '有效的专注'],
  ["items.value[1].break_minutes = -1", '有效的专注'],
  ["items.value[2].day_offset = 2", '当天或次日'],
  ["items.value[2].start = '23:50'; items.value[2].focus_minutes = 20", '次日午夜'],
  ["items.value[0].title = ' '", '请输入内容'],
]) {
  test(`reject invalid input: ${message}`, async () => {
    const api = setup()
    api.run(edit)
    assert.match(api.run('timelineError.value'), new RegExp(message))
    await api.run('save()')
    await api.run("templateName.value = 'test'; saveTemplate()")
    assert.equal(api.requests.length, 0, 'invalid draft must never be sent for persistence')
  })
}
test('content move detects affected execution records without moving statuses', () => {
  const api = setup()
  assert.equal(api.run('recordedChanges.value'), false)
  api.run("items.value = moveItems(items.value, 2, 0, 'content')")
  assert.equal(api.run('recordedChanges.value'), true)
  assert.equal(api.run('items.value[0].status'), 'completed')
  assert.equal(api.run('timelineError.value'), '')
})
test('content drag accepts the full row and uses its height for insertion', () => {
  const api = setup()
  api.run(`drag.value = { id: 'a', mode: 'content' };
    var event = { clientY: 75, currentTarget: { getBoundingClientRect: () => ({ top: 0, height: 100 }) }, dataTransfer: {}, preventDefault() {}, stopPropagation() {} };
    dragOver(event, 2)`)
  assert.equal(api.run('dropTarget.value'), 3)
  api.run("drop(event)")
  assert.equal(api.run("items.value.map(item => item.title).join('')"), 'BCA')
  assert.equal(api.run('drag.value'), null)

})
test('cancelled drag clears insertion feedback', () => {
  const api = setup()
  api.run("drag.value = { id: 'a', mode: 'item' }; dropTarget.value = 1; endDrag()")
  assert.equal(api.run('drag.value'), null)
  assert.equal(api.run('dropTarget.value'), null)
})

test('editing after a conflict invalidates the old replacement confirmation', async () => {
  const api = setup()
  await nextTick()
  api.run("saveConflict.value = true; items.value[0].title = 'changed'")
  await nextTick()
  assert.equal(api.run('saveConflict.value'), false)
})

test('native drag startup sets the payload and mode, and rejects busy startup', () => {
  const api = setup()
  api.run(`var event = { dataTransfer: { setData(type, value) { this.payload = [type, value] } }, preventDefault() { this.prevented = true } };
    startDrag(event, items.value[0], 'item')`)
  assert.deepEqual(plain(api.run('drag.value')), { id: 'a', mode: 'item' })
  assert.deepEqual(plain(api.run('event.dataTransfer.payload')), ['text/plain', 'a'])
  api.run("endDrag(); loading.value = true; startDrag(event, items.value[0], 'content')")
  assert.equal(api.run('drag.value'), null)
  assert.equal(api.run('event.prevented'), true)
})

test('adjacent row edges resolve to the same insertion gap', () => {
  const api = setup()
  api.run(`drag.value = { id: 'c', mode: 'item' };
    var event = { clientY: 75, currentTarget: { getBoundingClientRect: () => ({ top: 0, height: 100 }) }, dataTransfer: {}, preventDefault() {}, stopPropagation() {} };
    dragOver(event, 0)`)
  assert.equal(api.run('dropTarget.value'), 1)
  api.run("event.clientY = 25; dragOver(event, 1)")
  assert.equal(api.run('dropTarget.value'), 1)
})
for (const mode of ['item', 'content']) {
  test(`${mode}: drop directly into the first gap`, () => {
    const api = setup()
    api.run(`drag.value = { id: 'c', mode: '${mode}' };
      var event = { dataTransfer: {}, preventDefault() {}, stopPropagation() {} };
      dragOverGap(event, 0); drop(event)`)
    assert.equal(api.run("items.value.map(item => item.title).join('')"), 'CAB')
    if (mode === 'content') assert.equal(api.run("items.value.map(item => item.id).join('')"), 'abc')
  })
}

for (const mode of ['item', 'content']) {
  for (const [clientY, boundary] of [[125, 1], [175, 2]]) {
    test(`${mode}: full row ${clientY} maps to gap ${boundary}`, () => {
      const api = setup()
      api.run(`drag.value = { id: 'c', mode: '${mode}' };
        var event = { clientY: ${clientY}, currentTarget: { getBoundingClientRect: () => ({ top: 100, height: 100 }) }, dataTransfer: {}, preventDefault() { this.prevented = true }, stopPropagation() {} };
        dragOver(event, 1)`)
      assert.equal(api.run('dropTarget.value'), boundary)
      assert.equal(api.run('event.prevented'), true)
    })
  }
}
