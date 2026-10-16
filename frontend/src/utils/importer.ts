// Turns the sheet preview returned by the server into the columns of the import dialog.

import type { ImportSheet } from '../api/api.ts'
import type { FieldType } from '../types/bitable.ts'
import { t } from '../i18n/translate.ts'

/** Maximum size of an imported file, the same as the server. */
export const MAX_IMPORT_FILE_MB = 20

/** Field types the imported values can be converted to, formulas and attachments are not importable. */
export const IMPORTABLE_TYPES: FieldType[] = ['text', 'number', 'single_select', 'multi_select', 'datetime', 'checkbox']

/** Value of the accept attribute of the file input. */
export const IMPORT_ACCEPT = '.xlsx,.xlsm,.csv,.tsv,.txt'

/** Legacy .xls files are uploaded too, so the server explains how to convert them. */
const UPLOADABLE_EXTENSIONS = ['xlsx', 'xlsm', 'xls', 'csv', 'tsv', 'txt']

/** Whether the file can be uploaded by its extension. */
export function isImportFile(name: string): boolean {
  return UPLOADABLE_EXTENSIONS.includes(name.slice(name.lastIndexOf('.') + 1).toLowerCase())
}

/** A column of the sheet shown in the mapping step. */
export interface ImportColumn {
  /** Index of the column in the sheet. */
  index: number
  /** Title from the header row, or a generated one; unique among the columns. */
  name: string
  /** First distinct values of the preview rows, shown as a hint. */
  samples: string[]
  /** Type inferred by the server. */
  type: FieldType
  /** All the data cells of the column are empty. */
  empty: boolean
}

/** Preview rows excluding the header row. */
export function dataRows(sheet: ImportSheet, headerRow: boolean): string[][] {
  return headerRow ? sheet.rows.slice(1) : sheet.rows
}

/** Number of the data rows to import. */
export function rowCount(sheet: ImportSheet, headerRow: boolean): number {
  return Math.max(0, sheet.rowCount - (headerRow ? 1 : 0))
}

/** Columns of the sheet, titled by the header row if headerRow is true. */
export function buildColumns(sheet: ImportSheet, headerRow: boolean): ImportColumn[] {
  const head = headerRow ? (sheet.rows[0] ?? []) : []
  const rows = dataRows(sheet, headerRow)
  const used = new Set<string>()
  return sheet.columns.map((schema, i) => {
    let name = head[i]?.replace(/\s+/g, ' ').trim() || t('import.columnName', { n: i + 1 })
    if (used.has(name)) {
      let n = 2
      while (used.has(`${name} ${n}`)) n++
      name = `${name} ${n}`
    }
    used.add(name)

    const samples: string[] = []
    for (const row of rows) {
      if (samples.length >= 3) break
      const v = row[i]
      if (v && !samples.includes(v)) samples.push(v)
    }
    return {
      index: i,
      name,
      samples,
      type: headerRow ? schema.type : schema.typeWithoutHeader,
      empty: headerRow ? schema.empty : schema.emptyWithoutHeader,
    }
  })
}

/** File name without the extension, used as the name of the new table. */
export function baseName(fileName: string): string {
  const i = fileName.lastIndexOf('.')
  return (i > 0 ? fileName.slice(0, i) : fileName).trim()
}
