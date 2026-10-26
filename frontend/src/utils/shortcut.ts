import type { AdminFieldShortcut, FieldShortcutManifest, SaveFieldShortcut, ShortcutField, ShortcutSection } from '@/api/shortcut'
import type { FieldShortcut, FieldType, SLField } from '@/types/bitable'
import { localText, type LocalText } from './localText.ts'

/** Field types which can have a shortcut, the formula values are computed by the browser so they can not. */
export const SHORTCUT_HOST_TYPES: FieldType[] = ['text', 'number', 'single_select', 'multi_select', 'checkbox', 'datetime']

/** Returns the form fields of all the sections. */
export function shortcutFields(manifest: { sections: ShortcutSection[] }): ShortcutField[] {
  return manifest.sections.flatMap((section) => section.fields)
}

/** Shows the label of a form field, or its key if the label is empty, the same as the backend. */
export function fieldLabel(field: Pick<ShortcutField, 'key' | 'label'>, language: string): string {
  return localText(field.label, language).trim() || field.key
}

/** Shows the label of a select option, or its value if the label is empty, the same as the backend. */
export function optionLabel(option: { value: string; label?: LocalText }, language: string): string {
  return localText(option.label, language).trim() || option.value
}

/** Saves the shortcut as is, the manifest properties of the admin view and of the save request have the same names. */
export function saveBody(s: AdminFieldShortcut, changes: Partial<SaveFieldShortcut> = {}): SaveFieldShortcut {
  return {
    id: s.id,
    homepage: s.homepage,
    icon: s.icon,
    name: s.name,
    version: s.version,
    description: s.description,
    resultType: s.resultType as SaveFieldShortcut['resultType'],
    timeout: s.timeout,
    permissions: s.permissions,
    credentials: s.credentials.map(({ key, label, type, name }) => ({ key, label, type, name })),
    sections: s.sections,
    code: s.code,
    language: s.language,
    enabled: s.enabled,
    ...changes,
  }
}

/** Sample parameters use the defaults declared by the form fields. */
export function sampleShortcutParams(fields: ShortcutField[]): string {
  return JSON.stringify(Object.fromEntries(fields.map((field) => [field.key, field.widget === 'field_options' ? ['A', 'B'] : field.default ?? (field.widget === 'select' ? (field.options?.[0]?.value ?? '') : 'Hello')])), null, 2)
}

/** Returns the initial inputs of a new shortcut by the defaults of the form fields. */
export function defaultInputs(manifest: Pick<FieldShortcutManifest, 'sections'>): Record<string, string> {
  const inputs: Record<string, string> = {}
  for (const field of shortcutFields(manifest)) if (field.widget !== 'field_options' && field.default) inputs[field.key] = field.default
  return inputs
}

export function newShortcut(manifest: Pick<FieldShortcutManifest, 'id' | 'sections'>): FieldShortcut {
  return { id: manifest.id, inputs: defaultInputs(manifest), autoUpdate: true }
}

/** Imports the previous line-based category input when upgrading to host options. */
export function legacyOptionNames(manifest: Pick<FieldShortcutManifest, 'sections'>, inputs: Record<string, string>): string[] {
  return [...new Set(shortcutFields(manifest).filter((item) => item.widget === 'field_options')
    .flatMap((item) => (inputs[item.key] ?? '').split(/\r?\n/).map((line) => line.split(/[:：]/, 1)[0]!.trim()))
    .filter(Boolean))]
}

/** Returns the fields which the field_select item of the field can pick. */
export function selectableFields(item: Pick<ShortcutField, 'fieldTypes'>, fields: SLField[], selfUID?: string): SLField[] {
  return fields.filter(
    (f) => f.uid !== selfUID && f.type !== 'formula' && (!item.fieldTypes?.length || item.fieldTypes.includes(f.type)),
  )
}

/** Returns the label of the first required field without a value, or null if complete. */
export function missingInput(manifest: Pick<FieldShortcutManifest, 'sections'>, inputs: Record<string, string>, language = 'en'): string | null {
  const field = shortcutFields(manifest).find((f) => f.widget !== 'field_options' && f.required && !inputs[f.key]?.trim())
  return field ? fieldLabel(field, language) : null
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
