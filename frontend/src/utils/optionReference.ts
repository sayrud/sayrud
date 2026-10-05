import type { OptionReference, SelectOption, SLField } from '../types/bitable.ts'

export function optionsReferenceOf(field: SLField): OptionReference | undefined {
  return (field.metadata as { optionsReference?: OptionReference }).optionsReference
}

export function referencedOptions(current: SelectOption[], source: SelectOption[], newUID: () => string): SelectOption[] {
  const bySource = new Map(current.filter((o) => o.sourceUID).map((o) => [o.sourceUID, o]))
  const byName = new Map(current.filter((o) => !o.sourceUID).map((o) => [o.name, o]))

  // Preserve selected values when source options are renamed or reordered.
  return source.map((s) => ({ uid: (bySource.get(s.uid) ?? byName.get(s.name))?.uid ?? newUID(), name: s.name, color: s.color, sourceUID: s.uid }))
}

export function referenceComplete(ref: OptionReference): boolean {
  return !!ref.tableUID && !!ref.fieldUID && ref.conditions.length <= 20 && ref.conditions.every((c) =>
    !!c.fieldUID && (c.operation === 'empty' || c.operation === 'not_empty' || !!c.valueFieldUID),
  )
}
