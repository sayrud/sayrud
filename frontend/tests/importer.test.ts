import assert from 'node:assert/strict'
import test from 'node:test'

import type { ImportSheet } from '../src/api/api.ts'
import { baseName, buildColumns, dataRows, isImportFile, rowCount } from '../src/utils/importer.ts'

const sheet: ImportSheet = {
  name: 'Customers',
  rowCount: 4,
  rows: [
    ['Name', 'Name', '', 'Status'],
    ['Alice', 'a', '', 'Open'],
    ['Bob', 'a', '', 'Closed'],
    ['Carol', 'b', '', 'Open'],
  ],
  columns: [
    {
      type: 'text',
      empty: false,
      typeWithoutHeader: 'text',
      emptyWithoutHeader: false,
    },
    {
      type: 'single_select',
      empty: false,
      typeWithoutHeader: 'text',
      emptyWithoutHeader: false,
    },
    {
      type: 'text',
      empty: true,
      typeWithoutHeader: 'text',
      emptyWithoutHeader: true,
    },
    {
      type: 'single_select',
      empty: false,
      typeWithoutHeader: 'text',
      emptyWithoutHeader: false,
    },
  ],
}

test('columns take unique titles from the header row and the inferred types', () => {
  const columns = buildColumns(sheet, true)
  assert.deepEqual(columns.map((c) => c.name), ['Name', 'Name 2', 'import.columnName', 'Status'])
  assert.deepEqual(columns.map((c) => c.type), ['text', 'single_select', 'text', 'single_select'])
  assert.deepEqual(columns.map((c) => c.empty), [false, false, true, false])
  assert.deepEqual(columns[1]!.samples, ['a', 'b'])
  assert.deepEqual(columns[0]!.samples, ['Alice', 'Bob', 'Carol'])
  assert.equal(rowCount(sheet, true), 3)
  assert.equal(dataRows(sheet, true).length, 3)
})

test('without the header row all the rows are data', () => {
  const columns = buildColumns(sheet, false)
  assert.ok(columns.every((c) => c.name.startsWith('import.columnName')))
  assert.deepEqual(columns.map((c) => c.type), ['text', 'text', 'text', 'text'])
  assert.deepEqual(columns[0]!.samples, ['Name', 'Alice', 'Bob'])
  assert.equal(rowCount(sheet, false), 4)
})

test('file names are checked by extension and used as table names', () => {
  assert.ok(isImportFile('data.XLSX'))
  assert.ok(isImportFile('legacy.xls'))
  assert.ok(isImportFile('export.csv'))
  assert.ok(!isImportFile('report.pdf'))
  assert.equal(baseName('客户名单.xlsx'), '客户名单')
  assert.equal(baseName('.hidden'), '.hidden')
})
