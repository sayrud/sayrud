import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import type { FieldType, QueryFilter, QuerySort, RecordWriter, SLField, SLRecord, UserBrief } from '../src/types/bitable.ts'
import { addResolved, resetDirectory, setMembers, unresolvedIds, userOf } from '../src/utils/directory.ts'
import { queryRecords, TableContext } from '../src/utils/engine.ts'
import { cellValue, textToValue, valueToText } from '../src/utils/format.ts'

// The fixtures are shared with the backend tests, so both implementations of the filters and formulas are checked against the same cases.
const testdata = (name: string) => JSON.parse(readFileSync(new URL(`../../internal/db/testdata/${name}`, import.meta.url), 'utf8'))

interface FixtureField {
  uid: string
  type: FieldType
  metadata: Record<string, unknown>
}

const toField = (f: FixtureField, position = 0): SLField =>
  ({ uid: f.uid, tableUID: 'tbl', label: f.uid, type: f.type, metadata: f.metadata, position, createdAt: '', updatedAt: '' }) as SLField

const user = (id: number, userName: string, email: string, isBot = false): UserBrief => ({ id, userName, email, emailMd5: '', color: '#3370ff', isBot })

test('member and writer filters and sorting match the backend', () => {
  const fixture = testdata('member_filters.json') as {
    currentUserId: number
    fields: FixtureField[]
    records: { uid: string; data: SLRecord['data']; createdBy?: RecordWriter; updatedBy?: RecordWriter }[]
    cases: { name: string; currentUserId?: number; matchAny?: boolean; filter?: QueryFilter[]; sort?: QuerySort[]; expect: string[] }[]
  }
  const fields = fixture.fields.map(toField)
  const records: SLRecord[] = fixture.records.map((r) => ({
    uid: r.uid,
    tableUID: 'tbl',
    data: r.data,
    createdAt: '2026-11-03T00:00:00Z',
    updatedAt: '2026-11-03T00:00:00Z',
    createdBy: r.createdBy,
    // The backend keeps the creator as the modifier when no other writer is given.
    updatedBy: r.updatedBy ?? r.createdBy,
  }))
  for (const c of fixture.cases) {
    const ctx = new TableContext(fields, c.currentUserId ?? fixture.currentUserId)
    const got = queryRecords(ctx, records, { filter: c.filter, conjunction: c.matchAny ? 'or' : 'and', sort: c.sort }).map((r) => r.uid)
    if (c.sort) assert.deepEqual(got, c.expect, c.name)
    else assert.deepEqual([...got].sort(), [...c.expect].sort(), c.name)
  }
})

test('member and system formulas match the backend', () => {
  const fixture = testdata('member_formulas.json') as {
    users: { id: number; userName: string; email: string; isBot: boolean }[]
    memberIds: number[]
    workflows: Record<string, string>
    fields: FixtureField[]
    record: { data: SLRecord['data']; createdAt: string; createdBy: RecordWriter; updatedBy: RecordWriter }
    cases: { exp: string; expect: unknown }[]
  }
  resetDirectory()
  const users = fixture.users.map((u) => user(u.id, u.userName, u.email, u.isBot))
  setMembers(users.filter((u) => fixture.memberIds.includes(u.id)))
  addResolved(
    users.filter((u) => !fixture.memberIds.includes(u.id)).map((u) => ({ user: u, left: true })),
    Object.entries(fixture.workflows).map(([id, name]) => ({ id: Number(id), name })),
  )
  const fields = [
    ...fixture.fields.map(toField),
    ...fixture.cases.map((c, i) => toField({ uid: `fldFormula${i}`, type: 'formula', metadata: { exp: c.exp } })),
  ]
  const record: SLRecord = { uid: 'rec1', tableUID: 'tbl', ...fixture.record, updatedAt: fixture.record.createdAt }
  const ctx = new TableContext(fields)
  fixture.cases.forEach((c, i) => {
    const result = ctx.formula(record, fields.find((f) => f.uid === `fldFormula${i}`)!)
    assert.equal(result.error, undefined, c.exp)
    assert.deepEqual(result.value, c.expect, c.exp)
  })
})

test('members are shown, pasted and resolved by the directory', () => {
  resetDirectory(1)
  const ann = user(1, 'Ann', 'ann@example.com')
  const bob = user(2, 'Bob', 'bob@example.com')
  setMembers([ann, bob])
  addResolved([{ user: user(3, 'Left', 'left@example.com'), left: true }], [{ id: 7, name: 'Nightly sync' }], [3, 99], [7])

  const multiple = toField({ uid: 'fldTeam0001', type: 'member', metadata: { multiple: true } })
  const single = toField({ uid: 'fldOwner001', type: 'member', metadata: { multiple: false } })
  const editor = toField({ uid: 'fldEditor01', type: 'updated_by', metadata: {} })
  const created = toField({ uid: 'fldCreated1', type: 'created_at', metadata: { format: 'YYYY-MM-DD', with_time: false } })

  assert.equal(valueToText(multiple, [1, 3, 99]), 'Ann, Left', 'the members who left keep their names')
  assert.deepEqual(textToValue(multiple, 'BOB@example.com, Ann, Nobody').value, [2, 1])
  assert.deepEqual(textToValue(single, 'Ann, Bob').value, [1])
  assert.equal(textToValue(multiple, 'Left').value, null, 'the users who left are not matched')

  const record: SLRecord = { uid: 'rec1', tableUID: 'tbl', data: {}, createdAt: '2026-11-03T08:30:00Z', updatedAt: '2026-11-03T09:00:00Z', updatedBy: { workflowId: 7 } }
  assert.deepEqual(cellValue(record, editor), { workflowId: 7 })
  assert.equal(valueToText(editor, cellValue(record, editor)), 'Nightly sync')
  assert.equal(valueToText(created, cellValue(record, created)).length, 10)
  assert.equal(cellValue({ ...record, updatedBy: {} }, editor), null, 'unknown writers are empty')

  const pending = unresolvedIds([multiple, editor], [
    { ...record, data: { fldTeam0001: [1, 4, 99] }, createdBy: { userId: 5 }, updatedBy: { workflowId: 8 } },
  ])
  assert.deepEqual(pending, { userIds: [4, 5], workflowIds: [8] }, 'the known and missing IDs are not requested again')
})

test('only the removed collaborators are left, the other writers such as admins are not', () => {
  resetDirectory(1)
  const ann = user(1, 'Ann', 'ann@example.com')
  const bob = user(2, 'Bob', 'bob@example.com')
  setMembers([ann, bob])
  addResolved([{ user: user(5, 'Admin', 'admin@example.com'), left: false }], [], [5])
  setMembers([ann])
  assert.equal(userOf(2)?.left, true, 'a member missing from the refreshed list has left')
  assert.equal(userOf(5)?.left, false, 'a writer who never joined is not shown as left')
  addResolved([{ user: bob, left: true }], [], [2])
  setMembers([ann, bob])
  assert.equal(userOf(2)?.left, false, 'a member added back is current again')
})
