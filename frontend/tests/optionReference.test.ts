import assert from 'node:assert/strict'
import test from 'node:test'

import { referenceComplete, referencedOptions } from '../src/utils/optionReference.ts'

test('referenced options retain cell IDs by initial name and then source identity', () => {
  const original = [{ uid: 'localA', name: 'Alpha', color: 0 }]
  const source = [{ uid: 'sourceA', name: 'Alpha', color: 2 }, { uid: 'sourceB', name: 'Beta', color: 3 }]

  const linked = referencedOptions(original, source, () => 'newB')
  assert.equal(linked[0]!.uid, 'localA')
  assert.equal(linked[0]!.sourceUID, 'sourceA')

  const renamed = referencedOptions(linked, [{ ...source[0]!, name: 'Renamed', color: 8 }], () => 'unused')
  assert.deepEqual(renamed, [{ uid: 'localA', name: 'Renamed', color: 8, sourceUID: 'sourceA' }])
  assert.deepEqual(original, [{ uid: 'localA', name: 'Alpha', color: 0 }])
})

test('reference configuration requires a source and complete current table field conditions', () => {
  const ref = { tableUID: 'table', fieldUID: 'field', conditions: [] }

  assert.ok(referenceComplete(ref))
  assert.equal(referenceComplete({ ...ref, fieldUID: '' }), false)
  assert.equal(referenceComplete({ ...ref, conditions: [{ fieldUID: 'parent', operation: 'eq', value: '' }] }), false)
  assert.equal(referenceComplete({ ...ref, conditions: [{ fieldUID: 'parent', operation: 'eq', value: 'fixed' }] }), false)
  assert.ok(referenceComplete({ ...ref, conditions: [{ fieldUID: 'parent', operation: 'eq', value: '', valueFieldUID: 'local' }] }))
  assert.ok(referenceComplete({ ...ref, conditions: [{ fieldUID: 'parent', operation: 'empty', value: '' }] }))
})
