import dayjs from 'dayjs'

import type { SLField } from '@/types/bitable'
import { t } from '../i18n/translate.ts'

export type FormulaValue = number | string | boolean | Date | null

export class FormulaError extends Error {
  constructor(
    message: string,
    /** Error code shown in the cell. */
    public code = '#ERROR!',
  ) {
    super(message)
  }
}

// ---- Lexer ----

type Token =
  | { t: 'num'; v: number }
  | { t: 'str'; v: string }
  | { t: 'ref'; v: string }
  | { t: 'ident'; v: string }
  | { t: 'op'; v: string }
  | { t: 'lp' }
  | { t: 'rp' }
  | { t: 'comma' }

const OPERATORS = ['<=', '>=', '!=', '<>', '+', '-', '*', '/', '%', '^', '&', '=', '<', '>']

function tokenize(src: string): Token[] {
  const tokens: Token[] = []
  let i = 0
  while (i < src.length) {
    const c = src[i]!
    if (/\s/.test(c)) {
      i++
      continue
    }
    if (/[0-9.]/.test(c)) {
      const m = /^(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?/.exec(src.slice(i))
      if (!m) throw new FormulaError(t('formula.error.invalidNumber', { text: src.slice(i, i + 8) }))
      tokens.push({ t: 'num', v: Number(m[0]) })
      i += m[0].length
      continue
    }
    if (c === '"' || c === "'") {
      let j = i + 1
      let s = ''
      while (j < src.length && src[j] !== c) {
        if (src[j] === '\\' && j + 1 < src.length) {
          s += src[j + 1]
          j += 2
        } else {
          s += src[j]
          j++
        }
      }
      if (j >= src.length) throw new FormulaError(t('formula.error.unclosedString'))
      tokens.push({ t: 'str', v: s })
      i = j + 1
      continue
    }
    if (c === '{') {
      const j = src.indexOf('}', i)
      if (j < 0) throw new FormulaError(t('formula.error.unclosedField'))
      tokens.push({ t: 'ref', v: src.slice(i + 1, j) })
      i = j + 1
      continue
    }
    if (/[A-Za-z_]/.test(c)) {
      const m = /^[A-Za-z_][A-Za-z0-9_]*/.exec(src.slice(i))!
      tokens.push({ t: 'ident', v: m[0].toUpperCase() })
      i += m[0].length
      continue
    }
    if (c === '(') {
      tokens.push({ t: 'lp' })
      i++
      continue
    }
    if (c === ')') {
      tokens.push({ t: 'rp' })
      i++
      continue
    }
    if (c === ',' || c === '，') {
      tokens.push({ t: 'comma' })
      i++
      continue
    }
    const op = OPERATORS.find((o) => src.startsWith(o, i))
    if (op) {
      tokens.push({ t: 'op', v: op })
      i += op.length
      continue
    }
    throw new FormulaError(t('formula.error.invalidChar', { char: c }))
  }
  return tokens
}

// ---- Parser ----

export type FormulaNode =
  | { k: 'lit'; v: FormulaValue }
  | { k: 'ref'; uid: string }
  | { k: 'unary'; op: string; arg: FormulaNode }
  | { k: 'bin'; op: string; l: FormulaNode; r: FormulaNode }
  | { k: 'call'; name: string; args: FormulaNode[] }

const PRECEDENCE: Record<string, number> = {
  '=': 1,
  '!=': 1,
  '<>': 1,
  '<': 1,
  '>': 1,
  '<=': 1,
  '>=': 1,
  '&': 2,
  '+': 3,
  '-': 3,
  '*': 4,
  '/': 4,
  '%': 4,
  '^': 5,
}

function parse(tokens: Token[]): FormulaNode {
  let pos = 0
  const peek = () => tokens[pos]
  const next = () => tokens[pos++]

  function parseExpr(minPrec: number): FormulaNode {
    let left = parseUnary()
    for (;;) {
      const tok = peek()
      if (!tok || tok.t !== 'op') break
      const prec = PRECEDENCE[tok.v]!
      if (prec < minPrec) break
      next()
      // ^ is right associative, the others are left associative.
      const right = parseExpr(tok.v === '^' ? prec : prec + 1)
      left = { k: 'bin', op: tok.v, l: left, r: right }
    }
    return left
  }

  function parseUnary(): FormulaNode {
    const tok = peek()
    if (tok?.t === 'op' && (tok.v === '-' || tok.v === '+')) {
      next()
      return { k: 'unary', op: tok.v, arg: parseUnary() }
    }
    return parsePrimary()
  }

  function parsePrimary(): FormulaNode {
    const tok = next()
    if (!tok) throw new FormulaError(t('formula.error.incomplete'))
    switch (tok.t) {
      case 'num':
      case 'str':
        return { k: 'lit', v: tok.v }
      case 'ref':
        return { k: 'ref', uid: tok.v }
      case 'lp': {
        const e = parseExpr(1)
        if (next()?.t !== 'rp') throw new FormulaError(t('formula.error.missingParen'))
        return e
      }
      case 'ident': {
        if (tok.v === 'TRUE') return { k: 'lit', v: true }
        if (tok.v === 'FALSE') return { k: 'lit', v: false }
        if (peek()?.t !== 'lp') throw new FormulaError(t('formula.error.unknownIdentifier', { name: tok.v }))
        next()
        const args: FormulaNode[] = []
        if (peek()?.t !== 'rp') {
          for (;;) {
            args.push(parseExpr(1))
            const sep = next()
            if (sep?.t === 'rp') break
            if (sep?.t !== 'comma') throw new FormulaError(t('formula.error.invalidArgs', { name: tok.v }))
          }
        } else {
          next()
        }
        if (!FUNCTIONS[tok.v]) throw new FormulaError(t('formula.error.unknownFunction', { name: tok.v }))
        return { k: 'call', name: tok.v, args }
      }
      default:
        throw new FormulaError(t('formula.error.syntax'))
    }
  }

  if (tokens.length === 0) throw new FormulaError(t('formula.error.empty'))
  const node = parseExpr(1)
  if (pos < tokens.length) throw new FormulaError(t('formula.error.syntax'))
  return node
}

// ---- Evaluation ----

function isBlank(v: FormulaValue): boolean {
  return v === null || v === ''
}

function toNumber(v: FormulaValue): number {
  if (v === null || v === '') return 0
  if (typeof v === 'number') return v
  if (typeof v === 'boolean') return v ? 1 : 0
  if (v instanceof Date) return v.getTime()
  const n = Number(v.replace(/,/g, ''))
  if (Number.isNaN(n)) throw new FormulaError(t('formula.error.notNumber', { value: String(v) }), '#VALUE!')
  return n
}

export function formulaToText(v: FormulaValue): string {
  if (v === null) return ''
  if (typeof v === 'boolean') return v ? 'TRUE' : 'FALSE'
  if (v instanceof Date) {
    const d = dayjs(v)
    return d.hour() === 0 && d.minute() === 0 ? d.format('YYYY/MM/DD') : d.format('YYYY/MM/DD HH:mm')
  }
  if (typeof v === 'number') {
    if (!Number.isFinite(v)) return '#NUM!'
    return String(Math.round(v * 100) / 100)
  }
  return v
}

function toDate(v: FormulaValue): Date | null {
  if (v === null || v === '') return null
  if (v instanceof Date) return v
  if (typeof v === 'number') return new Date(v)
  const d = dayjs(String(v))
  if (!d.isValid()) throw new FormulaError(t('formula.error.notDate', { value: String(v) }), '#VALUE!')
  return d.toDate()
}

function toBool(v: FormulaValue): boolean {
  if (typeof v === 'boolean') return v
  if (v === null || v === '') return false
  if (typeof v === 'number') return v !== 0
  if (v instanceof Date) return true
  return v.toUpperCase() !== 'FALSE' && v !== '0'
}

function compare(a: FormulaValue, b: FormulaValue): number {
  if (typeof a === 'string' || typeof b === 'string') {
    if (!(a instanceof Date) && !(b instanceof Date)) {
      const na = Number(a)
      const nb = Number(b)
      if (typeof a === 'string' && typeof b === 'string') return a.localeCompare(b)
      if (!Number.isNaN(na) && !Number.isNaN(nb)) return na - nb
      return formulaToText(a).localeCompare(formulaToText(b))
    }
  }
  return toNumber(a) - toNumber(b)
}

const DAY = 86400000

type Fn = (args: FormulaValue[]) => FormulaValue

function nums(args: FormulaValue[]): number[] {
  return args.filter((a) => !isBlank(a)).map(toNumber)
}

function arity(name: string, args: FormulaValue[], min: number, max = min) {
  if (args.length < min || args.length > max) {
    throw new FormulaError(t('formula.error.argCount', { name }))
  }
}

export type FormulaCategory = 'math' | 'logic' | 'text' | 'date'

const FUNCTION_DEFS: Record<string, { fn: Fn; lazy?: boolean; category: FormulaCategory }> = {
  SUM: { category: 'math', fn: (a) => nums(a).reduce((s, n) => s + n, 0) },
  AVERAGE: {
    category: 'math',
    fn: (a) => {
      const n = nums(a)
      return n.length ? n.reduce((s, x) => s + x, 0) / n.length : 0
    },
  },
  MAX: { category: 'math', fn: (a) => (nums(a).length ? Math.max(...nums(a)) : 0) },
  MIN: { category: 'math', fn: (a) => (nums(a).length ? Math.min(...nums(a)) : 0) },
  ROUND: {
    category: 'math',
    fn: (a) => {
      arity('ROUND', a, 1, 2)
      const p = 10 ** toNumber(a[1] ?? 0)
      return Math.round(toNumber(a[0]!) * p) / p
    },
  },
  ABS: { category: 'math', fn: (a) => (arity('ABS', a, 1), Math.abs(toNumber(a[0]!))) },
  INT: { category: 'math', fn: (a) => (arity('INT', a, 1), Math.floor(toNumber(a[0]!))) },
  MOD: {
    category: 'math',
    fn: (a) => {
      arity('MOD', a, 2)
      const d = toNumber(a[1]!)
      if (d === 0) throw new FormulaError(t('formula.error.divByZero'), '#DIV/0!')
      return toNumber(a[0]!) % d
    },
  },
  POWER: { category: 'math', fn: (a) => (arity('POWER', a, 2), toNumber(a[0]!) ** toNumber(a[1]!)) },
  SQRT: { category: 'math', fn: (a) => (arity('SQRT', a, 1), Math.sqrt(toNumber(a[0]!))) },
  IF: {
    category: 'logic',
    lazy: true,
    fn: () => null,
  },
  AND: { category: 'logic', fn: (a) => a.every(toBool) },
  OR: { category: 'logic', fn: (a) => a.some(toBool) },
  NOT: { category: 'logic', fn: (a) => (arity('NOT', a, 1), !toBool(a[0]!)) },
  ISBLANK: { category: 'logic', fn: (a) => (arity('ISBLANK', a, 1), isBlank(a[0]!)) },
  CONCATENATE: { category: 'text', fn: (a) => a.map(formulaToText).join('') },
  LEN: { category: 'text', fn: (a) => (arity('LEN', a, 1), formulaToText(a[0]!).length) },
  LOWER: { category: 'text', fn: (a) => (arity('LOWER', a, 1), formulaToText(a[0]!).toLowerCase()) },
  UPPER: { category: 'text', fn: (a) => (arity('UPPER', a, 1), formulaToText(a[0]!).toUpperCase()) },
  TRIM: { category: 'text', fn: (a) => (arity('TRIM', a, 1), formulaToText(a[0]!).trim()) },
  LEFT: {
    category: 'text',
    fn: (a) => (arity('LEFT', a, 1, 2), formulaToText(a[0]!).slice(0, toNumber(a[1] ?? 1))),
  },
  RIGHT: {
    category: 'text',
    fn: (a) => {
      arity('RIGHT', a, 1, 2)
      const s = formulaToText(a[0]!)
      const n = toNumber(a[1] ?? 1)
      return n <= 0 ? '' : s.slice(-n)
    },
  },
  MID: {
    category: 'text',
    fn: (a) => {
      arity('MID', a, 3)
      const start = toNumber(a[1]!) - 1
      return formulaToText(a[0]!).slice(start, start + toNumber(a[2]!))
    },
  },
  CONTAINS: {
    category: 'text',
    fn: (a) => (arity('CONTAINS', a, 2), formulaToText(a[0]!).includes(formulaToText(a[1]!))),
  },
  SUBSTITUTE: {
    category: 'text',
    fn: (a) => (arity('SUBSTITUTE', a, 3), formulaToText(a[0]!).split(formulaToText(a[1]!)).join(formulaToText(a[2]!))),
  },
  TODAY: {
    category: 'date',
    fn: () => dayjs().startOf('day').toDate(),
  },
  NOW: { category: 'date', fn: () => new Date() },
  YEAR: { category: 'date', fn: (a) => (arity('YEAR', a, 1), a[0] ? dayjs(toDate(a[0])).year() : null) },
  MONTH: { category: 'date', fn: (a) => (arity('MONTH', a, 1), a[0] ? dayjs(toDate(a[0])).month() + 1 : null) },
  DAY: { category: 'date', fn: (a) => (arity('DAY', a, 1), a[0] ? dayjs(toDate(a[0])).date() : null) },
  WEEKDAY: {
    category: 'date',
    fn: (a) => {
      arity('WEEKDAY', a, 1)
      const d = toDate(a[0]!)
      return d ? ((dayjs(d).day() + 6) % 7) + 1 : null
    },
  },
  DATEDIF: {
    category: 'date',
    fn: (a) => {
      arity('DATEDIF', a, 2, 3)
      const s = toDate(a[0]!)
      const e = toDate(a[1]!)
      if (!s || !e) return null
      const unit = formulaToText(a[2] ?? 'D').toUpperCase()
      const map: Record<string, 'day' | 'month' | 'year'> = { D: 'day', M: 'month', Y: 'year' }
      if (!map[unit]) throw new FormulaError(t('formula.error.dateUnit', { name: 'DATEDIF' }))
      return dayjs(e).startOf('day').diff(dayjs(s).startOf('day'), map[unit])
    },
  },
  DATEADD: {
    category: 'date',
    fn: (a) => {
      arity('DATEADD', a, 2, 3)
      const d = toDate(a[0]!)
      if (!d) return null
      const unit = formulaToText(a[2] ?? 'D').toUpperCase()
      const map: Record<string, 'day' | 'month' | 'year'> = { D: 'day', M: 'month', Y: 'year' }
      if (!map[unit]) throw new FormulaError(t('formula.error.dateUnit', { name: 'DATEADD' }))
      return dayjs(d).add(toNumber(a[1]!), map[unit]).toDate()
    },
  },
}

export interface FormulaFunction {
  fn: Fn
  lazy?: boolean
  category: FormulaCategory
  /** The usage, e.g. SUM(number1, number2, ...). */
  readonly usage: string
  readonly desc: string
}

export const FUNCTIONS: Record<string, FormulaFunction> = Object.fromEntries(
  Object.entries(FUNCTION_DEFS).map(([name, f]) => [
    name,
    {
      ...f,
      get usage() {
        return t(`formula.fn.${name}.usage`)
      },
      get desc() {
        return t(`formula.fn.${name}.desc`)
      },
    },
  ]),
)

export function formulaCategoryLabel(category: FormulaCategory): string {
  return t(`formula.category.${category}`)
}

export type RefResolver = (fieldUID: string) => FormulaValue

function evaluate(node: FormulaNode, resolve: RefResolver): FormulaValue {
  switch (node.k) {
    case 'lit':
      return node.v
    case 'ref':
      return resolve(node.uid)
    case 'unary': {
      const v = toNumber(evaluate(node.arg, resolve))
      return node.op === '-' ? -v : v
    }
    case 'call': {
      if (node.name === 'IF') {
        if (node.args.length < 2 || node.args.length > 3) throw new FormulaError(t('formula.error.argCount', { name: 'IF' }))
        const cond = toBool(evaluate(node.args[0]!, resolve))
        if (cond) return evaluate(node.args[1]!, resolve)
        return node.args[2] ? evaluate(node.args[2], resolve) : null
      }
      return FUNCTIONS[node.name]!.fn(node.args.map((a) => evaluate(a, resolve)))
    }
    case 'bin': {
      const l = evaluate(node.l, resolve)
      const r = evaluate(node.r, resolve)
      switch (node.op) {
        case '&':
          return formulaToText(l) + formulaToText(r)
        case '+':
          if (l instanceof Date && !isBlank(r)) return new Date(l.getTime() + toNumber(r) * DAY)
          return toNumber(l) + toNumber(r)
        case '-':
          if (l instanceof Date && r instanceof Date) return Math.round((l.getTime() - r.getTime()) / DAY)
          if (l instanceof Date && !isBlank(r)) return new Date(l.getTime() - toNumber(r) * DAY)
          return toNumber(l) - toNumber(r)
        case '*':
          return toNumber(l) * toNumber(r)
        case '/': {
          const d = toNumber(r)
          if (d === 0) throw new FormulaError(t('formula.error.divByZero'), '#DIV/0!')
          return toNumber(l) / d
        }
        case '%':
          return toNumber(l) % toNumber(r)
        case '^':
          return toNumber(l) ** toNumber(r)
        case '=':
          return compare(l, r) === 0
        case '!=':
        case '<>':
          return compare(l, r) !== 0
        case '<':
          return compare(l, r) < 0
        case '>':
          return compare(l, r) > 0
        case '<=':
          return compare(l, r) <= 0
        case '>=':
          return compare(l, r) >= 0
      }
    }
  }
  throw new FormulaError(t('formula.error.syntax'))
}

// ---- Compilation ----

export interface CompiledFormula {
  error?: string
  refs: string[]
  run: (resolve: RefResolver) => FormulaValue
}

const cache = new Map<string, CompiledFormula>()

function collectRefs(node: FormulaNode, out: Set<string>) {
  if (node.k === 'ref') out.add(node.uid)
  else if (node.k === 'unary') collectRefs(node.arg, out)
  else if (node.k === 'bin') {
    collectRefs(node.l, out)
    collectRefs(node.r, out)
  } else if (node.k === 'call') node.args.forEach((a) => collectRefs(a, out))
}

export function compileFormula(exp: string): CompiledFormula {
  const hit = cache.get(exp)
  if (hit) return hit
  let compiled: CompiledFormula
  try {
    const ast = parse(tokenize(exp))
    const refs = new Set<string>()
    collectRefs(ast, refs)
    compiled = { refs: [...refs], run: (resolve) => evaluate(ast, resolve) }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    compiled = {
      error: msg,
      refs: [],
      run: () => {
        throw new FormulaError(msg)
      },
    }
  }
  cache.set(exp, compiled)
  return compiled
}

// ---- Editor display: {fldXXX} <-> [field label] ----

export function expToDisplay(exp: string, fields: SLField[]): string {
  const byUID = new Map(fields.map((f) => [f.uid, f.label]))
  return exp.replace(/\{([A-Za-z0-9_]+)\}/g, (m, uid: string) => {
    const label = byUID.get(uid)
    return label !== undefined ? `[${label}]` : m
  })
}

export function expFromDisplay(text: string, fields: SLField[]): { exp: string; unknown: string[] } {
  const byLabel = new Map(fields.map((f) => [f.label, f.uid]))
  const unknown: string[] = []
  const exp = text.replace(/\[([^\]]+)\]/g, (_m, label: string) => {
    const uid = byLabel.get(label)
    if (!uid) {
      unknown.push(label)
      return `{${label}}`
    }
    return `{${uid}}`
  })
  return { exp, unknown }
}
