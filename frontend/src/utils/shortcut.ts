import type { FieldShortcutManifest, SaveFieldShortcut, ShortcutFormItem } from '@/api/shortcut'
import type { FieldShortcut, FieldType, SLField } from '@/types/bitable'

/** Field types which can have a shortcut, the formula values are computed by the browser so they can not. */
export const SHORTCUT_HOST_TYPES: FieldType[] = ['text', 'number', 'single_select', 'multi_select', 'checkbox', 'datetime']

/** Starter drafts only become available to members after an admin saves them. */
export function shortcutStarter(textLabel: string):
  Pick<SaveFieldShortcut, 'aiEnabled' | 'resultType' | 'code' | 'formItems' | 'timeoutSeconds'> {
  return {
    aiEnabled: false,
    resultType: 'number',
    timeoutSeconds: 30,
    formItems: [
      { key: 'text', label: textLabel, component: 'field_select', required: true },
    ],
    code: `// params: the form item values keyed by key, a field_select item passes the cell value of the record.
// context: { projectUID, tableUID, fieldUID, recordUID, fetch(url, options, credentialKey), ai.complete({ prompt, system? }), log(...args) }
async function execute(params, context) {
  const text = params.text ?? ''

  return text.length
}
`,
  }
}

/** Sample parameters use the defaults so a starter draft can be tested immediately. */
export function sampleShortcutParams(items: ShortcutFormItem[]): string {
  return JSON.stringify(Object.fromEntries(items.map((item) => [item.key, item.component === 'field_options' ? ['A', 'B'] : item.default ?? (item.component === 'select' ? (item.options?.[0]?.value ?? '') : 'Hello')])), null, 2)
}

/** Returns the initial inputs of a new shortcut by the defaults of the form items. */
export function defaultInputs(manifest: Pick<FieldShortcutManifest, 'formItems'>): Record<string, string> {
  const inputs: Record<string, string> = {}
  for (const item of manifest.formItems) if (item.component !== 'field_options' && item.default) inputs[item.key] = item.default
  return inputs
}

export function newShortcut(manifest: Pick<FieldShortcutManifest, 'id' | 'formItems'>): FieldShortcut {
  return { id: manifest.id, inputs: defaultInputs(manifest), autoUpdate: true }
}

/** Imports the previous line-based category input when upgrading to host options. */
export function legacyOptionNames(manifest: Pick<FieldShortcutManifest, 'formItems'>, inputs: Record<string, string>): string[] {
  return [...new Set(manifest.formItems.filter((item) => item.component === 'field_options')
    .flatMap((item) => (inputs[item.key] ?? '').split(/\r?\n/).map((line) => line.split(/[:：]/, 1)[0]!.trim()))
    .filter(Boolean))]
}

/** Returns the fields which the field_select item of the field can pick. */
export function selectableFields(item: Pick<ShortcutFormItem, 'fieldTypes'>, fields: SLField[], selfUID?: string): SLField[] {
  return fields.filter(
    (f) => f.uid !== selfUID && f.type !== 'formula' && (!item.fieldTypes?.length || item.fieldTypes.includes(f.type)),
  )
}

/** Returns the label of the first required item without a value, or null if complete. */
export function missingInput(manifest: Pick<FieldShortcutManifest, 'formItems'>, inputs: Record<string, string>): string | null {
  const item = manifest.formItems.find((i) => i.component !== 'field_options' && i.required && !inputs[i.key]?.trim())
  return item ? item.label : null
}

const REF = /\{(fld[A-Za-z0-9]{7})\}/g

/** Shows the field references `{fldXXXXXXX}` of a prompt as `[label]`. */
export function promptToDisplay(prompt: string, fields: SLField[]): string {
  const labels = new Map(fields.map((f) => [f.uid, f.label]))
  return prompt.replace(REF, (m, uid: string) => {
    const label = labels.get(uid)
    return label !== undefined ? `[${label}]` : m
  })
}

/** Converts the `[label]` of known fields back to `{fldXXXXXXX}`, the other brackets are kept as typed. */
export function promptFromDisplay(text: string, fields: SLField[]): string {
  const uids = new Map(fields.map((f) => [f.label, f.uid]))
  return text.replace(/\[([^\]]+)\]/g, (m, label: string) => {
    const uid = uids.get(label)
    return uid ? `{${uid}}` : m
  })
}
