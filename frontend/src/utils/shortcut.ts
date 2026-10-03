import type { FieldShortcutManifest, ShortcutFormItem } from '@/api/shortcut'
import type { FieldShortcut, FieldType, SLField } from '@/types/bitable'

/** Field types which can have a shortcut, the formula values are computed by the browser so they can not. */
export const SHORTCUT_HOST_TYPES: FieldType[] = ['text', 'number', 'single_select', 'multi_select', 'checkbox', 'datetime']

/** Returns the initial inputs of a new shortcut by the defaults of the form items. */
export function defaultInputs(manifest: Pick<FieldShortcutManifest, 'formItems'>): Record<string, string> {
  const inputs: Record<string, string> = {}
  for (const item of manifest.formItems) if (item.default) inputs[item.key] = item.default
  return inputs
}

export function newShortcut(manifest: Pick<FieldShortcutManifest, 'id' | 'formItems'>): FieldShortcut {
  return { id: manifest.id, inputs: defaultInputs(manifest), autoUpdate: true }
}

/** Returns the fields which the field_select item of the field can pick. */
export function selectableFields(item: Pick<ShortcutFormItem, 'fieldTypes'>, fields: SLField[], selfUID?: string): SLField[] {
  return fields.filter(
    (f) => f.uid !== selfUID && f.type !== 'formula' && (!item.fieldTypes?.length || item.fieldTypes.includes(f.type)),
  )
}

/** Returns the label of the first required item without a value, or null if complete. */
export function missingInput(manifest: Pick<FieldShortcutManifest, 'formItems'>, inputs: Record<string, string>): string | null {
  const item = manifest.formItems.find((i) => i.required && !inputs[i.key]?.trim())
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
