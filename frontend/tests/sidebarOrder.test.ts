import assert from 'node:assert/strict'
import test from 'node:test'

import { moveSidebarItem, orderSidebarItems } from '../src/utils/sidebarOrder.ts'

test('sidebar order merges table and workflow positions and temporarily applies optimistic moves', () => {
  const items = [{ uid: 'table-1', position: 2 }, { uid: 'table-2', position: 0 }, { uid: 'workflow-1', position: 1 }]

  assert.deepEqual(orderSidebarItems(items), [items[1], items[2], items[0]])
  assert.deepEqual(orderSidebarItems(items, ['workflow-1', 'deleted', 'workflow-1', 'table-2']), [items[2], items[1], items[0]])
  assert.deepEqual(items.map((item) => item.uid), ['table-1', 'table-2', 'workflow-1'])
})

test('sidebar moves insert before or after any entry without losing filtered items', () => {
  const order = ['first', 'hidden', 'middle', 'last']

  assert.deepEqual(moveSidebarItem(order, 'last', 'first', false), ['last', 'first', 'hidden', 'middle'])
  assert.deepEqual(moveSidebarItem(order, 'first', 'last', true), ['hidden', 'middle', 'last', 'first'])
  assert.deepEqual(moveSidebarItem(order, 'last', 'middle', false), ['first', 'hidden', 'last', 'middle'])
  assert.deepEqual(moveSidebarItem(order, 'first', 'middle', true), ['hidden', 'middle', 'first', 'last'])

  for (const [uid, target] of [['middle', 'middle'], ['missing', 'first'], ['first', 'missing']]) {
    assert.deepEqual(moveSidebarItem(order, uid!, target!, true), order)
  }

  assert.deepEqual(order, ['first', 'hidden', 'middle', 'last'])
})
