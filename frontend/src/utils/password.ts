// Excludes the confusable 0 O 1 l I.
const UPPER = 'ABCDEFGHJKLMNPQRSTUVWXYZ'
const LOWER = 'abcdefghijkmnopqrstuvwxyz'
const DIGITS = '23456789'
const ALL = UPPER + LOWER + DIGITS

function randomIndex(n: number): number {
  const buf = new Uint32Array(1)
  crypto.getRandomValues(buf)
  return buf[0]! % n
}

const pick = (chars: string) => chars[randomIndex(chars.length)]!

/** Requires 8–18 printable ASCII characters from at least two character groups. */
export function isValidSharePassword(password: string): boolean {
  if (password.length < 8 || password.length > 18 || /[^!-~]/.test(password)) return false
  return [/[0-9]/, /[A-Za-z]/, /[^A-Za-z0-9]/].filter((group) => group.test(password)).length >= 2
}

/** Includes both letter cases, a digit, and a symbol when a symbol alphabet is supplied. */
export function generatePassword(length = 16, symbols = ''): string {
  const chars = [pick(UPPER), pick(LOWER), pick(DIGITS)]
  if (symbols) chars.push(pick(symbols))
  while (chars.length < length) chars.push(pick(ALL + symbols))
  for (let i = chars.length - 1; i > 0; i--) {
    const j = randomIndex(i + 1)
    ;[chars[i], chars[j]] = [chars[j]!, chars[i]!]
  }
  return chars.join('')
}
