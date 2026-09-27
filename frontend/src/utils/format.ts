import dayjs from 'dayjs'

import type { CellValue, SelectOption, SLField } from '@/types/bitable'

export function isEmptyValue(v: unknown): boolean {
  return v === null || v === undefined || v === '' || (Array.isArray(v) && v.length === 0)
}

export function formatNumber(value: number | null | undefined, format: string): string {
  if (value === null || value === undefined || Number.isNaN(value)) return ''
  const m = /^([^0-9]*)(0(?:,000)?)(?:\.(0+))?(%?)$/.exec(format || '0')
  if (!m) return String(value)
  const [, prefix = '', intPart = '0', decimals = '', percent] = m
  const n = percent ? value * 100 : value
  const text = n.toLocaleString('en-US', {
    minimumFractionDigits: decimals.length,
    maximumFractionDigits: decimals.length,
    useGrouping: intPart.includes(','),
  })
  if (text.startsWith('-')) return '-' + prefix + text.slice(1) + percent
  return prefix + text + percent
}

/** Parses the number text typed by the user, which may contain the thousands separator, currency symbol and percent sign. */
export function parseNumber(text: string, format = '0'): number | null {
  const t = text.trim().replace(/[,¥$\s]/g, '')
  if (t === '') return null
  const isPercent = t.endsWith('%')
  const n = Number(isPercent ? t.slice(0, -1) : t)
  if (Number.isNaN(n)) return null
  // "10" is treated as 10% in the percent format.
  if (isPercent || format.endsWith('%')) return n / 100
  return n
}

export function formatDate(value: string | null | undefined, format: string, withTime: boolean): string {
  if (!value) return ''
  const d = dayjs(value)
  if (!d.isValid()) return ''
  return d.format(withTime ? `${format} HH:mm` : format)
}

export function parseDate(text: string): string | null {
  const t = text.trim()
  if (!t) return null
  const normalized = t.replace(/[年月]/g, '/').replace(/日/g, '').replace(/\./g, '/')
  let d = dayjs(normalized)
  if (!d.isValid()) d = dayjs(t)
  return d.isValid() ? d.toISOString() : null
}

export function optionsOf(field: SLField): SelectOption[] {
  if (field.type === 'single_select' || field.type === 'multi_select') {
    return (field.metadata as { options: SelectOption[] }).options ?? []
  }
  return []
}

export function findOption(field: SLField, uid: string): SelectOption | undefined {
  return optionsOf(field).find((o) => o.uid === uid)
}

/** Text of a non-formula cell, used for copying, searching and sorting. */
export function valueToText(field: SLField, value: CellValue): string {
  if (isEmptyValue(value)) return ''
  switch (field.type) {
    case 'text':
      return String(value)
    case 'number':
      return formatNumber(Number(value), (field.metadata as { format: string }).format)
    case 'single_select':
      return findOption(field, String(value))?.name ?? ''
    case 'multi_select':
      return (Array.isArray(value) ? value : [])
        .map((uid) => findOption(field, uid)?.name)
        .filter(Boolean)
        .join(', ')
    case 'datetime': {
      const md = field.metadata as { format: string; with_time: boolean }
      return formatDate(String(value), md.format, md.with_time)
    }
    case 'checkbox':
      return value ? '是' : '否'
    default:
      return String(value)
  }
}

export interface ParsedValue {
  value: CellValue
  /** Option names not found when pasting, which need to be created in the field first. */
  newOptions: string[]
}

/** Parses the text as the value of the field type, used for pasting and type conversion. */
export function textToValue(field: SLField, text: string): ParsedValue {
  const t = text.trim()
  switch (field.type) {
    case 'text':
      return { value: text === '' ? null : text, newOptions: [] }
    case 'number':
      return { value: parseNumber(t, (field.metadata as { format: string }).format), newOptions: [] }
    case 'checkbox':
      return { value: ['true', '1', '是', 'yes', 'y', '✓', '√', 'checked'].includes(t.toLowerCase()), newOptions: [] }
    case 'datetime':
      return { value: parseDate(t), newOptions: [] }
    case 'single_select': {
      if (!t) return { value: null, newOptions: [] }
      const opt = optionsOf(field).find((o) => o.name === t)
      return opt ? { value: opt.uid, newOptions: [] } : { value: null, newOptions: [t] }
    }
    case 'multi_select': {
      const names = t
        .split(/[,，、;；\n]/)
        .map((s) => s.trim())
        .filter(Boolean)
      const opts = optionsOf(field)
      const uids: string[] = []
      const missing: string[] = []
      for (const name of names) {
        const opt = opts.find((o) => o.name === name)
        if (opt) uids.push(opt.uid)
        else missing.push(name)
      }
      return { value: uids, newOptions: [...new Set(missing)] }
    }
    default:
      return { value: null, newOptions: [] }
  }
}

/** Maps the new option names to UIDs, called after the options are added to the field. */
export function resolveOptionNames(field: SLField, text: string): CellValue {
  return textToValue(field, text).value
}

export function defaultValueOf(field: SLField): CellValue {
  const md = field.metadata as Record<string, unknown>
  switch (field.type) {
    case 'text':
      return (md.default as string) || null
    case 'number':
      return (md.default as number | null) ?? null
    case 'single_select':
      return (md.default as string) || null
    case 'multi_select':
      return (md.default as string[])?.length ? [...(md.default as string[])] : null
    case 'datetime':
      return md.default === 'now' ? new Date().toISOString() : null
    default:
      return null
  }
}

/** Parses TSV, supporting the quoted multi-line cells copied from spreadsheets. */
export function parseTSV(text: string): string[][] {
  const rows: string[][] = []
  let row: string[] = []
  let cell = ''
  let i = 0
  let quoted = false
  const src = text.replace(/\r\n?/g, '\n')
  while (i < src.length) {
    const c = src[i]!
    if (quoted) {
      if (c === '"' && src[i + 1] === '"') {
        cell += '"'
        i += 2
        continue
      }
      if (c === '"') {
        quoted = false
        i++
        continue
      }
      cell += c
      i++
      continue
    }
    if (c === '"' && cell === '') {
      quoted = true
      i++
      continue
    }
    if (c === '\t') {
      row.push(cell)
      cell = ''
    } else if (c === '\n') {
      row.push(cell)
      rows.push(row)
      row = []
      cell = ''
    } else {
      cell += c
    }
    i++
  }
  if (cell !== '' || row.length) {
    row.push(cell)
    rows.push(row)
  }
  return rows
}

export function toTSV(rows: string[][]): string {
  return rows
    .map((r) => r.map((c) => (/[\t\n"]/.test(c) ? `"${c.replace(/"/g, '""')}"` : c)).join('\t'))
    .join('\n')
}
