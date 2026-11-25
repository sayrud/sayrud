import assert from 'node:assert/strict'
import test from 'node:test'

import type { RecordComment } from '../src/api/comments.ts'
import { applyCommentChange, mentionedIds, plainText, toBody, toEditorText } from '../src/utils/comments.ts'

const names: Record<number, string> = { 1: 'Ann', 2: 'Ann Lee', 3: 'Bot' }
const nameOf = (id: number) => names[id] ?? ''

const comment = (id: number, extra: Partial<RecordComment> = {}): RecordComment => ({
  id, recordUID: 'rec', author: { userId: 1 }, body: `c${id}`, kind: 'comment', mentions: [], createdAt: '', ...extra,
})

test('mentions round-trip between the body and the editor text', () => {
  const { text, mentions } = toEditorText('hi <@2> and <@1>, <@9> <@2>', nameOf)
  assert.equal(text, 'hi @Ann Lee and @Ann, <@9> @Ann Lee', 'unknown users keep their tokens')
  assert.deepEqual(mentions, [{ id: 2, name: 'Ann Lee' }, { id: 1, name: 'Ann' }])
  assert.equal(toBody(text, mentions), 'hi <@2> and <@1>, <@9> <@2>', 'the longer names are replaced first')
  assert.equal(toBody('@Ann typed without choosing', []), '@Ann typed without choosing', 'only the chosen members become mentions')
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
