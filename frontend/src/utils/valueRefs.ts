import type { WorkflowValue } from '@/types/workflow'

/** {{variable}} in templates, matching the backend TemplatePattern. */
export const TEMPLATE_REF = /\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}/g
/** {variable} in formulas. */
export const FORMULA_REF = /\{([^{}]+)\}/g

export type RefStyle = 'template' | 'formula'
export type TextToken = { text: string } | { ref: string }

/** Split a template or formula into text and variable references. */
export function splitRefs(text: string, style: RefStyle): TextToken[] {
  const pattern = new RegExp(style === 'template' ? TEMPLATE_REF.source : FORMULA_REF.source, 'g')
  const out: TextToken[] = []
  let last = 0
  for (const m of text.matchAll(pattern)) {
    if (m.index > last) out.push({ text: text.slice(last, m.index) })
    out.push({ ref: m[1]!.trim() })
    last = m.index + m[0].length
  }
  if (last < text.length) out.push({ text: text.slice(last) })
  return out
}

export function refText(path: string, style: RefStyle) {
  return style === 'template' ? `{{${path}}}` : `{${path}}`
}

/** A text constant in a formula. */
function formulaString(s: string) {
  return `"${s.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`
}

/** Convert a value into an equivalent formula, used when switching from input to formula editing. */
export function valueToFormula(v: WorkflowValue | undefined): string {
  if (!v) return ''
  switch (v.kind) {
    case 'formula':
      return v.formula ?? ''
    case 'var':
      return v.var ? `{${v.var}}` : ''
    case 'template':
      return splitRefs(v.template ?? '', 'template')
        .map((t) => ('ref' in t ? `{${t.ref}}` : formulaString(t.text)))
        .join(' & ')
    default:
      if (typeof v.const === 'number') return String(v.const)
      if (typeof v.const === 'boolean') return v.const ? 'TRUE' : 'FALSE'
      if (typeof v.const === 'string') return v.const ? formulaString(v.const) : ''
      return ''
  }
}
