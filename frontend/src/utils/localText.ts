export type LocalText = string | Record<string, string>

/** Uses the same language fallback as workflow plugin manifests. */
export function localText(value: LocalText | undefined, language: string): string {
  if (typeof value === 'string') return value
  if (!value) return ''
  if (value[language] !== undefined) return value[language]
  const keys = Object.keys(value).sort()
  const base = language.split('-', 1)[0]?.toLowerCase()
  const match = base && keys.find((key) => key.split('-', 1)[0]?.toLowerCase() === base)
  if (match) return value[match]!
  return value.en ?? value[''] ?? value[keys[0] ?? ''] ?? ''
}

export function normalizeLocalText(value: unknown): LocalText {
  if (typeof value === 'string') return value.trim()
  if (!value || typeof value !== 'object' || Array.isArray(value)) return ''
  const entries = Object.entries(value)
  if (entries.some(([, text]) => typeof text !== 'string')) return ''
  return Object.fromEntries(entries.map(([language, text]) => [language, (text as string).trim()]).filter(([, text]) => text !== ''))
}

export function validLocalText(value: LocalText, maxLength: number, required: boolean): boolean {
  if (required && !localText(value, 'en').trim()) return false
  const entries: [string, string][] = typeof value === 'string' ? [['', value]] : Object.entries(value)
  return entries.length <= 20 && entries.every(([language, text]) =>
    (language === '' || /^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$/.test(language)) && [...text].length <= maxLength)
}
