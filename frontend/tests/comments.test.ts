import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { RecordComment } from '../src/api/comments.ts'
import * as comments from '../src/utils/comments.ts'

const { applyCommentChange, mentionedIds, plainText, toBody, toEditorText, trackMentions } = comments

const names: Record<number, string> = { 1: 'Ann', 2: 'Ann Lee', 3: 'Bot', 10: 'Sam', 20: 'Sam' }
const nameOf = (id: number) => names[id] ?? ''

const comment = (id: number, extra: Partial<RecordComment> = {}): RecordComment => ({
  id, recordUID: 'rec', author: { userId: 1 }, body: `c${id}`, kind: 'comment', mentions: [], createdAt: '', ...extra,
})

test('comment saves respect the current editor, draft version and account', async (t) => {
  const props = vue.reactive({ recordUid: 'rec' })
  const auth = vue.reactive({ user: { id: 1 } })
  const source = readFileSync(new URL('../src/components/record/CommentPanel.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'comment-panel-test' })
  const code = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  let finish!: (comment: RecordComment) => void
  let fail!: (err: Error) => void
  const pending = () => new Promise<RecordComment>((resolve, reject) => { finish = resolve; fail = reject })
  const messages: string[] = []
  const modules: Record<string, unknown> = {
    vue: { ...vue, onBeforeUnmount: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@arco-design/web-vue': { Message: { error: (message: string) => messages.push(message) } },
    '@/api/comments': { commentsApi: { create: pending, update: pending, list: async () => ({ comments: [], nextCursor: 0 }) } },
    '@/stores/auth': { useAuthStore: () => auth },
    '@/stores/base': { useBaseStore: () => ({ project: { uid: 'project' }, activeTableUID: 'table', resolveWriters: () => {}, onCommentChange: () => () => {} }) },
    '@/utils/directory': { userName: nameOf },
    '@/utils/comments': comments,
  }
  type Panel = {
    draft: vue.WritableComputedRef<comments.CommentDraft>
    editing: vue.Ref<{ id: number } | undefined>
    startEdit: (c: RecordComment) => void
    cancel: () => void
    submit: () => Promise<void>
  }
  const module = { exports: {} as { default: { setup: (props: object, context: object) => Panel } } }
  new Function('require', 'module', 'exports', code)((id: string) => modules[id] ?? {}, module, module.exports)
  const scope = vue.effectScope()
  t.after(() => { scope.stop(); comments.commentDrafts.clear() })
  const panel = scope.run(() => module.exports.default.setup(props, { expose: () => {} }))!
  const type = (text: string) => { panel.draft.value = { text, mentions: [] } }

  type('sent draft')
  const sent = panel.submit()
  props.recordUid = 'other'
  await vue.nextTick()
  type('other record draft')
  finish(comment(1))
  await sent
  assert.equal(panel.draft.value.text, 'other record draft')
  props.recordUid = 'rec'
  await vue.nextTick()
  assert.equal(panel.draft.value.text, '')

  type('first account draft')
  const failed = panel.submit()
  fail(new Error('Failed to send'))
  await failed
  assert.deepEqual(messages, ['Failed to send'])
  assert.equal(panel.draft.value.text, 'first account draft')
  auth.user.id = 2
  await vue.nextTick()
  assert.equal(panel.draft.value.text, '')
  type('second account draft')
  auth.user.id = 1
  await vue.nextTick()
  assert.equal(panel.draft.value.text, 'first account draft')

  panel.startEdit(comment(1))
  const savedEdit = panel.submit()
  panel.startEdit(comment(2))
  type('new edit')
  finish(comment(1))
  await savedEdit
  assert.equal(panel.editing.value?.id, 2)
  assert.equal(panel.draft.value.text, 'new edit')
  panel.cancel()

  const sentEarlier = panel.submit()
  type('newer unsent draft')
  finish(comment(3))
  await sentEarlier
  assert.equal(panel.draft.value.text, 'newer unsent draft')
})

test('mentions round-trip between the body and the editor text', () => {
  const { text, mentions } = toEditorText('hi <@2> and <@1>, <@9> <@2>', nameOf)
  assert.equal(text, 'hi @Ann Lee and @Ann, <@9> @Ann Lee', 'unknown users keep their tokens')
  assert.deepEqual(mentions, [{ id: 2, name: 'Ann Lee', start: 3 }, { id: 1, name: 'Ann', start: 16 }, { id: 2, name: 'Ann Lee', start: 27 }])
  assert.equal(toBody(text, mentions), 'hi <@2> and <@1>, <@9> <@2>')
  assert.equal(toBody('@Ann typed without choosing', []), '@Ann typed without choosing', 'only the chosen members become mentions')

  const same = toEditorText('<@10> <@20>', nameOf)
  assert.equal(same.text, '@Sam @Sam')
  assert.equal(toBody(same.text, same.mentions), '<@10> <@20>', 'the members of the same name stay apart')
})

test('mentions follow the edits of the editor text', () => {
  // The editor inserts the option value, i.e. the user ID, which is replaced with the name.
  let edit = trackMentions('hi @Sa', 'hi @20 ', [], { id: 20, name: 'Sam', inserted: '20' })
  assert.equal(edit.text, 'hi @Sam ')
  edit = trackMentions(edit.text, 'cc @Sa hi @Sam ', edit.mentions)
  edit = trackMentions(edit.text, 'cc @10 hi @Sam ', edit.mentions, { id: 10, name: 'Sam', inserted: '10' })
  assert.equal(edit.text, 'cc @Sam hi @Sam ')
  assert.equal(toBody(edit.text, edit.mentions), 'cc <@10> hi <@20> ', 'a mention inserted before another moves it')

  const typedInside = trackMentions(edit.text, 'cc @Sxam hi @Sam ', edit.mentions)
  assert.equal(toBody(typedInside.text, typedInside.mentions), 'cc @Sxam hi <@20> ', 'a mention edited inside becomes text')
  let typedAround = trackMentions(edit.text, 'ok cc @Sam hi @Sam ', edit.mentions)
  typedAround = trackMentions(typedAround.text, 'ok cc @Sam! hi @Sam ', typedAround.mentions)
  assert.equal(toBody(typedAround.text, typedAround.mentions), 'ok cc <@10>! hi <@20> ', 'the text typed around keeps the mentions')
  const removed = trackMentions(edit.text, 'cc hi @Sam ', edit.mentions)
  assert.deepEqual(removed.mentions, [{ id: 20, name: 'Sam', start: 6 }])
})

test('the text after the cursor tells which of the same mentions is edited', () => {
  const both = toEditorText('<@10> <@20>', nameOf)
  // Deleting the selected `@Sam ` at the start leaves the cursor before the remaining `@Sam`.
  const first = trackMentions(both.text, '@Sam', both.mentions, undefined, 4)
  assert.equal(toBody(first.text, first.mentions), '<@20>')
  const second = trackMentions(both.text, '@Sam', both.mentions, undefined, 0)
  assert.equal(toBody(second.text, second.mentions), '<@10>')
  const backspace = trackMentions(both.text, '@Sam@Sam', both.mentions, undefined, 4)
  assert.equal(toBody(backspace.text, backspace.mentions), '<@10><@20>', 'deleting the space keeps both')
  const stale = trackMentions(both.text, '@Sam', both.mentions, undefined, 9)
  assert.equal(toBody(stale.text, stale.mentions), '<@10>', 'a cursor not matching the texts is ignored')
})

test('mentioned users and plain texts', () => {
  assert.deepEqual(mentionedIds('<@3> <@1> <@3> <@x>'), [3, 1])
  assert.equal(plainText('ping <@3> <@9>', nameOf), 'ping @Bot <@9>')
})

test('comment changes apply by ID to the roots and their replies', () => {
  let roots = applyCommentChange([], 'created', comment(1))
  roots = applyCommentChange(roots, 'created', comment(1))
  assert.equal(roots.length, 1, 'a repeated change is applied once')
  roots = applyCommentChange(roots, 'created', comment(3, { rootId: 1, replyToId: 1 }))
  roots = applyCommentChange(roots, 'created', comment(2, { rootId: 1, replyToId: 1 }))
  assert.deepEqual(roots[0]!.replies!.map((r) => r.id), [2, 3], 'the replies are in creation order')
  roots = applyCommentChange(roots, 'updated', comment(1, { body: 'edited' }))
  assert.equal(roots[0]!.body, 'edited')
  assert.equal(roots[0]!.replies!.length, 2, 'editing the root keeps its replies')

  roots = applyCommentChange(roots, 'deleted', comment(1, { body: '', deleted: true }))
  assert.equal(roots[0]!.deleted, true, 'a deleted root with replies stays as a placeholder')
  roots = applyCommentChange(roots, 'deleted', comment(2, { rootId: 1 }))
  roots = applyCommentChange(roots, 'deleted', comment(3, { rootId: 1 }))
  assert.deepEqual(roots, [], 'the placeholder goes with its last reply')

  roots = applyCommentChange([comment(5)], 'updated', comment(6))
  assert.deepEqual(roots.map((r) => r.id), [5], 'an update of an unknown comment is ignored')
  roots = applyCommentChange([comment(7)], 'deleted', comment(7, { deleted: true }))
  assert.deepEqual(roots, [], 'a deleted root without replies disappears')
})
