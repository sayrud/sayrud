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

/** Generates a random password with at least one uppercase letter, one lowercase letter and one digit. */
export function generatePassword(length = 16): string {
  const chars = [pick(UPPER), pick(LOWER), pick(DIGITS)]
  while (chars.length < length) chars.push(pick(ALL))
  for (let i = chars.length - 1; i > 0; i--) {
    const j = randomIndex(i + 1)
    ;[chars[i], chars[j]] = [chars[j]!, chars[i]!]
  }
  return chars.join('')
}
