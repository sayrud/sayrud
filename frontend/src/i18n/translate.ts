// The translation entry without vue-i18n and path aliases, so the utils loaded by the node tests can use it.
// The implementation is injected by src/i18n/index.ts, it returns the key before that.

type Translate = (key: string, ...args: unknown[]) => string

let impl: Translate = (key) => key

export function setTranslator(fn: Translate) {
  impl = fn
}

/** Follows the language when called in render functions or computed. */
export function t(key: string, ...args: unknown[]): string {
  return impl(key, ...args)
}

/** Returns the labels translated when read, so they follow the language. */
export function lazyLabels<K extends string>(getters: Record<K, () => string>): Record<K, string> {
  const labels = {} as Record<K, string>
  for (const key of Object.keys(getters) as K[]) {
    Object.defineProperty(labels, key, { get: getters[key], enumerable: true })
  }
  return labels
}
