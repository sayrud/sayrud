// shortcutFormItems converts form items between the visual editor and JSON text, and validates them the same way as normalizeFormItems.

export const MAX_FORM_ITEMS = 20

export const FORM_COMPONENTS = ['field_select', 'input', 'textarea', 'select', 'prompt'] as const
export type FormComponent = (typeof FORM_COMPONENTS)[number]

export const FORM_FIELD_TYPES = ['text', 'single_select', 'multi_select', 'datetime', 'number', 'checkbox'] as const
export type FormFieldType = (typeof FORM_FIELD_TYPES)[number]

const KEY = /^[A-Za-z][A-Za-z0-9_]{0,31}$/
const ISSUE_KEYS = {
  too_many: 'shortcutAdmin.issueTooMany',
  key: 'shortcutAdmin.issueKey',
  duplicate_key: 'shortcutAdmin.issueDuplicateKey',
  label: 'shortcutAdmin.issueLabel',
  component: 'shortcutAdmin.issueComponent',
  field_types: 'shortcutAdmin.issueFieldTypes',
  options: 'shortcutAdmin.issueOptions',
  option_value: 'shortcutAdmin.issueOptionValue',
  duplicate_option: 'shortcutAdmin.issueDuplicateOption',
  default: 'shortcutAdmin.issueDefault',
} as const

export interface ShortcutFormOptionDraft {
  value: string
  label: string
}

export interface ShortcutFormItemDraft {
  key: string
  label: string
  component: string
  required: boolean
  placeholder: string
  fieldTypes: string[]
  options: ShortcutFormOptionDraft[]
  /** JSON field `default`. A field_select item has no default. */
  defaultValue: string
}

export interface FormItemIssue {
  /** -1 for a list-level issue, otherwise the 0-based item index. */
  index: number
  /** 0-based option index when the issue is about one option. */
  option?: number
  field: 'list' | 'key' | 'label' | 'component' | 'fieldTypes' | 'options' | 'default'
  code: keyof typeof ISSUE_KEYS
}

export function isFormComponent(value: string): value is FormComponent {
  return (FORM_COMPONENTS as readonly string[]).includes(value)
}

export function isFormFieldType(value: string): value is FormFieldType {
  return (FORM_FIELD_TYPES as readonly string[]).includes(value)
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function stringList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return [...new Set(value.filter((entry): entry is string => typeof entry === 'string'))]
}

function optionValue(value: unknown): string {
  if (typeof value === 'string') return value
  if (value == null) return ''
  return String(value)
}

function optionsOf(value: unknown): ShortcutFormOptionDraft[] {
  if (!Array.isArray(value)) return []
  const out: ShortcutFormOptionDraft[] = []
  for (const entry of value) {
    if (!isPlainObject(entry)) continue
    const option = optionValue(entry.value)
    const label = typeof entry.label === 'string' ? entry.label.trim() : ''
    out.push({ value: option, label: label === '' ? option : label })
  }
  return out
}

function coerce(raw: Record<string, unknown>): ShortcutFormItemDraft {
  const component = typeof raw.component === 'string' ? raw.component : ''
  return {
    key: typeof raw.key === 'string' ? raw.key.trim() : '',
    label: typeof raw.label === 'string' ? raw.label.trim() : '',
    component,
    required: raw.required === true,
    placeholder: typeof raw.placeholder === 'string' ? raw.placeholder.trim() : '',
    fieldTypes: component === 'field_select' ? stringList(raw.fieldTypes) : [],
    options: component === 'select' ? optionsOf(raw.options) : [],
    defaultValue: component === 'field_select' || typeof raw.default !== 'string' ? '' : raw.default,
  }
}

export function loadFormItems(text: string): { ok: true; items: ShortcutFormItemDraft[] } | { ok: false } {
  try {
    const parsed: unknown = JSON.parse(text)
    if (!Array.isArray(parsed) || !parsed.every(isPlainObject)) return { ok: false }
    return { ok: true, items: parsed.map(coerce) }
  } catch {
    return { ok: false }
  }
}

function serializeItem(item: ShortcutFormItemDraft): Record<string, unknown> {
  const out: Record<string, unknown> = { key: item.key, label: item.label, component: item.component }
  if (item.required) out.required = true
  if (item.placeholder !== '') out.placeholder = item.placeholder
  if (item.component === 'field_select' && item.fieldTypes.length > 0) out.fieldTypes = item.fieldTypes
  if (item.component === 'select' && item.options.length > 0) {
    out.options = item.options.map((option) => ({
      value: option.value,
      label: option.label === '' ? option.value : option.label,
    }))
  }
  if (item.component !== 'field_select' && item.defaultValue !== '') out.default = item.defaultValue
  return out
}

export function serializeFormItems(items: readonly ShortcutFormItemDraft[]): string {
  return JSON.stringify(items.map(serializeItem), null, 2)
}

export function validateFormItems(items: readonly ShortcutFormItemDraft[]): FormItemIssue[] {
  const issues: FormItemIssue[] = []
  if (items.length > MAX_FORM_ITEMS) issues.push({ index: -1, field: 'list', code: 'too_many' })
  const counts = new Map<string, number>()
  for (const entry of items) counts.set(entry.key, (counts.get(entry.key) ?? 0) + 1)
  items.forEach((entry, index) => {
    if (!KEY.test(entry.key)) issues.push({ index, field: 'key', code: 'key' })
    else if ((counts.get(entry.key) ?? 0) > 1) issues.push({ index, field: 'key', code: 'duplicate_key' })
    const label = entry.label.trim()
    if (label.length === 0 || [...label].length > 64) issues.push({ index, field: 'label', code: 'label' })
    if (!isFormComponent(entry.component)) issues.push({ index, field: 'component', code: 'component' })
    if (entry.component === 'field_select' && entry.fieldTypes.some((type) => !isFormFieldType(type))) {
      issues.push({ index, field: 'fieldTypes', code: 'field_types' })
    }
    if (entry.component !== 'select') return
    if (entry.options.length === 0) {
      issues.push({ index, field: 'options', code: 'options' })
    } else {
      const values = new Map<string, number>()
      for (const option of entry.options) values.set(option.value, (values.get(option.value) ?? 0) + 1)
      entry.options.forEach((option, optionIndex) => {
        if (option.value === '') issues.push({ index, option: optionIndex, field: 'options', code: 'option_value' })
        else if ((values.get(option.value) ?? 0) > 1) issues.push({ index, option: optionIndex, field: 'options', code: 'duplicate_option' })
      })
    }
    if (entry.defaultValue !== '' && !entry.options.some((option) => option.value === entry.defaultValue)) {
      issues.push({ index, field: 'default', code: 'default' })
    }
  })
  return issues
}

export function nextFormItemKey(keys: readonly string[]): string {
  const used = new Set(keys)
  if (!used.has('field')) return 'field'
  let n = 2
  while (used.has(`field${n}`)) n += 1
  return `field${n}`
}

export function formItemIssueName(items: readonly ShortcutFormItemDraft[], issue: FormItemIssue): string {
  if (issue.index < 0) return ''
  const key = items[issue.index]?.key ?? ''
  return key !== '' ? key : String(issue.index + 1)
}

export function formItemIssueKey(issue: FormItemIssue): string {
  return ISSUE_KEYS[issue.code]
}
