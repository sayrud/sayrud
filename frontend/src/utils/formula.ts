import dayjs from 'dayjs'

import type { SLField } from '@/types/bitable'

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
      if (!m) throw new FormulaError(`无法识别的数字：${src.slice(i, i + 8)}`)
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
      if (j >= src.length) throw new FormulaError('字符串缺少结束引号')
      tokens.push({ t: 'str', v: s })
      i = j + 1
      continue
    }
    if (c === '{') {
      const j = src.indexOf('}', i)
      if (j < 0) throw new FormulaError('字段引用缺少 }')
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
    throw new FormulaError(`无法识别的字符：${c}`)
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
    if (!tok) throw new FormulaError('公式不完整')
    switch (tok.t) {
      case 'num':
      case 'str':
        return { k: 'lit', v: tok.v }
      case 'ref':
        return { k: 'ref', uid: tok.v }
      case 'lp': {
        const e = parseExpr(1)
        if (next()?.t !== 'rp') throw new FormulaError('缺少右括号')
        return e
      }
      case 'ident': {
        if (tok.v === 'TRUE') return { k: 'lit', v: true }
        if (tok.v === 'FALSE') return { k: 'lit', v: false }
        if (peek()?.t !== 'lp') throw new FormulaError(`未知标识符：${tok.v}`)
        next()
        const args: FormulaNode[] = []
        if (peek()?.t !== 'rp') {
          for (;;) {
            args.push(parseExpr(1))
            const sep = next()
            if (sep?.t === 'rp') break
            if (sep?.t !== 'comma') throw new FormulaError(`函数 ${tok.v} 参数格式错误`)
          }
        } else {
          next()
        }
        if (!FUNCTIONS[tok.v]) throw new FormulaError(`未知函数：${tok.v}`)
        return { k: 'call', name: tok.v, args }
      }
      default:
        throw new FormulaError('公式语法错误')
    }
  }

  if (tokens.length === 0) throw new FormulaError('公式为空')
  const node = parseExpr(1)
  if (pos < tokens.length) throw new FormulaError('公式语法错误')
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
  if (Number.isNaN(n)) throw new FormulaError(`"${v}" 不是数字`, '#VALUE!')
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
  if (!d.isValid()) throw new FormulaError(`"${v}" 不是日期`, '#VALUE!')
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
    throw new FormulaError(`函数 ${name} 参数数量错误`)
  }
}

export const FUNCTIONS: Record<string, { fn: Fn; lazy?: boolean; desc: string; usage: string; category: string }> = {
  SUM: { category: '数学', usage: 'SUM(数值1, 数值2, ...)', desc: '求和', fn: (a) => nums(a).reduce((s, n) => s + n, 0) },
  AVERAGE: {
    category: '数学',
    usage: 'AVERAGE(数值1, 数值2, ...)',
    desc: '平均值',
    fn: (a) => {
      const n = nums(a)
      return n.length ? n.reduce((s, x) => s + x, 0) / n.length : 0
    },
  },
  MAX: { category: '数学', usage: 'MAX(数值1, 数值2, ...)', desc: '最大值', fn: (a) => (nums(a).length ? Math.max(...nums(a)) : 0) },
  MIN: { category: '数学', usage: 'MIN(数值1, 数值2, ...)', desc: '最小值', fn: (a) => (nums(a).length ? Math.min(...nums(a)) : 0) },
  ROUND: {
    category: '数学',
    usage: 'ROUND(数值, 小数位数)',
    desc: '四舍五入到指定小数位',
    fn: (a) => {
      arity('ROUND', a, 1, 2)
      const p = 10 ** toNumber(a[1] ?? 0)
      return Math.round(toNumber(a[0]!) * p) / p
    },
  },
  ABS: { category: '数学', usage: 'ABS(数值)', desc: '绝对值', fn: (a) => (arity('ABS', a, 1), Math.abs(toNumber(a[0]!))) },
  INT: { category: '数学', usage: 'INT(数值)', desc: '向下取整', fn: (a) => (arity('INT', a, 1), Math.floor(toNumber(a[0]!))) },
  MOD: {
    category: '数学',
    usage: 'MOD(被除数, 除数)',
    desc: '取余',
    fn: (a) => {
      arity('MOD', a, 2)
      const d = toNumber(a[1]!)
      if (d === 0) throw new FormulaError('除数不能为 0', '#DIV/0!')
      return toNumber(a[0]!) % d
    },
  },
  POWER: { category: '数学', usage: 'POWER(底数, 指数)', desc: '乘方', fn: (a) => (arity('POWER', a, 2), toNumber(a[0]!) ** toNumber(a[1]!)) },
  SQRT: { category: '数学', usage: 'SQRT(数值)', desc: '平方根', fn: (a) => (arity('SQRT', a, 1), Math.sqrt(toNumber(a[0]!))) },
  IF: {
    category: '逻辑',
    usage: 'IF(条件, 条件为真时的值, 条件为假时的值)',
    desc: '条件判断',
    lazy: true,
    fn: () => null,
  },
  AND: { category: '逻辑', usage: 'AND(条件1, 条件2, ...)', desc: '全部为真时返回 TRUE', fn: (a) => a.every(toBool) },
  OR: { category: '逻辑', usage: 'OR(条件1, 条件2, ...)', desc: '任一为真时返回 TRUE', fn: (a) => a.some(toBool) },
  NOT: { category: '逻辑', usage: 'NOT(条件)', desc: '取反', fn: (a) => (arity('NOT', a, 1), !toBool(a[0]!)) },
  ISBLANK: { category: '逻辑', usage: 'ISBLANK(值)', desc: '是否为空', fn: (a) => (arity('ISBLANK', a, 1), isBlank(a[0]!)) },
  CONCATENATE: { category: '文本', usage: 'CONCATENATE(文本1, 文本2, ...)', desc: '拼接文本', fn: (a) => a.map(formulaToText).join('') },
  LEN: { category: '文本', usage: 'LEN(文本)', desc: '文本长度', fn: (a) => (arity('LEN', a, 1), formulaToText(a[0]!).length) },
  LOWER: { category: '文本', usage: 'LOWER(文本)', desc: '转小写', fn: (a) => (arity('LOWER', a, 1), formulaToText(a[0]!).toLowerCase()) },
  UPPER: { category: '文本', usage: 'UPPER(文本)', desc: '转大写', fn: (a) => (arity('UPPER', a, 1), formulaToText(a[0]!).toUpperCase()) },
  TRIM: { category: '文本', usage: 'TRIM(文本)', desc: '去除首尾空白', fn: (a) => (arity('TRIM', a, 1), formulaToText(a[0]!).trim()) },
  LEFT: {
    category: '文本',
    usage: 'LEFT(文本, 字符数)',
    desc: '从左侧截取',
    fn: (a) => (arity('LEFT', a, 1, 2), formulaToText(a[0]!).slice(0, toNumber(a[1] ?? 1))),
  },
  RIGHT: {
    category: '文本',
    usage: 'RIGHT(文本, 字符数)',
    desc: '从右侧截取',
    fn: (a) => {
      arity('RIGHT', a, 1, 2)
      const s = formulaToText(a[0]!)
      const n = toNumber(a[1] ?? 1)
      return n <= 0 ? '' : s.slice(-n)
    },
  },
  MID: {
    category: '文本',
    usage: 'MID(文本, 起始位置, 字符数)',
    desc: '从中间截取（位置从 1 开始）',
    fn: (a) => {
      arity('MID', a, 3)
      const start = toNumber(a[1]!) - 1
      return formulaToText(a[0]!).slice(start, start + toNumber(a[2]!))
    },
  },
  CONTAINS: {
    category: '文本',
    usage: 'CONTAINS(文本, 查找内容)',
    desc: '是否包含',
    fn: (a) => (arity('CONTAINS', a, 2), formulaToText(a[0]!).includes(formulaToText(a[1]!))),
  },
  SUBSTITUTE: {
    category: '文本',
    usage: 'SUBSTITUTE(文本, 旧文本, 新文本)',
    desc: '替换文本',
    fn: (a) => (arity('SUBSTITUTE', a, 3), formulaToText(a[0]!).split(formulaToText(a[1]!)).join(formulaToText(a[2]!))),
  },
  TODAY: {
    category: '日期',
    usage: 'TODAY()',
    desc: '今天的日期',
    fn: () => dayjs().startOf('day').toDate(),
  },
  NOW: { category: '日期', usage: 'NOW()', desc: '当前日期时间', fn: () => new Date() },
  YEAR: { category: '日期', usage: 'YEAR(日期)', desc: '年份', fn: (a) => (arity('YEAR', a, 1), a[0] ? dayjs(toDate(a[0])).year() : null) },
  MONTH: { category: '日期', usage: 'MONTH(日期)', desc: '月份', fn: (a) => (arity('MONTH', a, 1), a[0] ? dayjs(toDate(a[0])).month() + 1 : null) },
  DAY: { category: '日期', usage: 'DAY(日期)', desc: '日', fn: (a) => (arity('DAY', a, 1), a[0] ? dayjs(toDate(a[0])).date() : null) },
  WEEKDAY: {
    category: '日期',
    usage: 'WEEKDAY(日期)',
    desc: '星期几（周一为 1）',
    fn: (a) => {
      arity('WEEKDAY', a, 1)
      const d = toDate(a[0]!)
      return d ? ((dayjs(d).day() + 6) % 7) + 1 : null
    },
  },
  DATEDIF: {
    category: '日期',
    usage: 'DATEDIF(开始日期, 结束日期, "D" | "M" | "Y")',
    desc: '两个日期的间隔',
    fn: (a) => {
      arity('DATEDIF', a, 2, 3)
      const s = toDate(a[0]!)
      const e = toDate(a[1]!)
      if (!s || !e) return null
      const unit = formulaToText(a[2] ?? 'D').toUpperCase()
      const map: Record<string, 'day' | 'month' | 'year'> = { D: 'day', M: 'month', Y: 'year' }
      if (!map[unit]) throw new FormulaError('DATEDIF 单位应为 D、M 或 Y')
      return dayjs(e).startOf('day').diff(dayjs(s).startOf('day'), map[unit])
    },
  },
  DATEADD: {
    category: '日期',
    usage: 'DATEADD(日期, 数量, "D" | "M" | "Y")',
    desc: '日期加减',
    fn: (a) => {
      arity('DATEADD', a, 2, 3)
      const d = toDate(a[0]!)
      if (!d) return null
      const unit = formulaToText(a[2] ?? 'D').toUpperCase()
      const map: Record<string, 'day' | 'month' | 'year'> = { D: 'day', M: 'month', Y: 'year' }
      if (!map[unit]) throw new FormulaError('DATEADD 单位应为 D、M 或 Y')
      return dayjs(d).add(toNumber(a[1]!), map[unit]).toDate()
    },
  },
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
        if (node.args.length < 2 || node.args.length > 3) throw new FormulaError('函数 IF 参数数量错误')
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
          if (d === 0) throw new FormulaError('除数不能为 0', '#DIV/0!')
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
  throw new FormulaError('公式语法错误')
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
