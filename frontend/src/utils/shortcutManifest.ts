import { javascriptLanguage, typescriptLanguage } from '@codemirror/lang-javascript'

import type { PluginLanguage } from './workflowPlugin'

type SyntaxNode = ReturnType<typeof javascriptLanguage.parser.parse>['topNode']

function children(node: SyntaxNode): SyntaxNode[] {
  const nodes: SyntaxNode[] = []
  for (let child = node.firstChild; child; child = child.nextSibling) nodes.push(child)

  return nodes.filter((child) => child.name !== 'LineComment' && child.name !== 'BlockComment')
}

/** Locates the exported manifest without evaluating code in the browser. */
function manifestValue(source: string, language: PluginLanguage): SyntaxNode | null {
  const tree = (language === 'typescript' ? typescriptLanguage : javascriptLanguage).parser.parse(source)
  let invalid = false
  tree.iterate({ enter: (node) => { if (node.type.isError) invalid = true } })
  if (invalid) throw new Error('syntax')

  const statements = children(tree.topNode)
  let binding = 'manifest'
  let exported = false

  for (const statement of statements) {
    if (statement.name !== 'ExportDeclaration') continue

    const declaration = statement.getChild('VariableDeclaration')
    if (declaration && children(declaration).some((node) => node.name === 'VariableDefinition' && source.slice(node.from, node.to) === 'manifest')) exported = true

    const group = statement.getChild('ExportGroup')
    if (!group) continue

    const nodes = children(group)
    for (let i = 0; i < nodes.length; i++) {
      const node = nodes[i]!
      if (node.name !== 'VariableName') continue

      const alias = nodes[i + 1]?.name === 'as' ? nodes[i + 2] : node
      if (alias && source.slice(alias.from, alias.to) === 'manifest') {
        binding = source.slice(node.from, node.to)
        exported = true
      }
      if (alias !== node) i += 2
    }
  }

  if (!exported) return null

  for (const statement of statements) {
    const declaration = statement.name === 'VariableDeclaration' ? statement : statement.getChild('VariableDeclaration')
    if (!declaration) continue

    const nodes = children(declaration)
    for (let i = 0; i < nodes.length; i++) {
      const node = nodes[i]!
      const equals = nodes[i + 1]?.name === 'TypeAnnotation' ? i + 2 : i + 1
      if (node.name === 'VariableDefinition' && source.slice(node.from, node.to) === binding && nodes[equals]?.name === 'Equals') {
        return nodes[equals + 1] ? configurationValue(nodes[equals + 1]!) : null
      }
    }
  }

  throw new Error('manifest')
}

function configurationValue(node: SyntaxNode): SyntaxNode {
  const nodes = children(node)
  if (node.name === 'ParenthesizedExpression' && nodes[1]) return configurationValue(nodes[1])
  if (node.name === 'BinaryExpression' && (nodes[1]?.name === 'as' || nodes[1]?.name === 'satisfies')) return configurationValue(nodes[0]!)

  return node
}

/** Classic shortcuts keep their existing execution semantics and separate settings. */
export function hasShortcutManifest(source: string, language: PluginLanguage = 'javascript'): boolean {
  return manifestValue(source, language) !== null
}

function literalValue(node: SyntaxNode, source: string): unknown {
  const text = source.slice(node.from, node.to)
  if (node.name === 'String') {
    return JSON.parse(text.startsWith("'") ? '"' + text.slice(1, -1).replace(/\\'|"/g, (match) => match === '"' ? '\\"' : "'") + '"' : text)
  }
  if (node.name === 'Number' || node.name === 'BooleanLiteral' || node.name === 'null') return JSON.parse(text)
  if (node.name === 'ArrayExpression') {
    return children(node).filter((child) => child.name !== '[' && child.name !== ']' && child.name !== ',').map((child) => literalValue(child, source))
  }
  if (node.name === 'ObjectExpression') {
    return Object.fromEntries(children(node).filter((child) => child.name === 'Property').map((property) => {
      const nodes = children(property)
      const name = nodes[0]!
      if (nodes[1]?.name !== ':' || !nodes[2]) throw new Error('expression')

      return [name.name === 'String' ? literalValue(name, source) : source.slice(name.from, name.to), literalValue(nodes[2], source)]
    }))
  }

  throw new Error('expression')
}

function sameValue(a: unknown, b: unknown): boolean {
  if (a === b) return true
  if (Array.isArray(a) && Array.isArray(b)) return a.length === b.length && a.every((value, index) => sameValue(value, b[index]))
  if (!a || !b || typeof a !== 'object' || typeof b !== 'object' || Array.isArray(a) || Array.isArray(b)) return false

  const left = Object.entries(a)
  const right = b as Record<string, unknown>
  return left.length === Object.keys(b).length && left.every(([key, value]) => Object.hasOwn(right, key) && sameValue(value, right[key]))
}

/** Updates only changed manifest properties, leaving the rest of the source and its comments intact. */
export function updateShortcutManifest(source: string, values: Record<string, unknown>, language: PluginLanguage = 'javascript'): string {
  const value = manifestValue(source, language)
  if (!value || Object.keys(values).length === 0) return source

  const indent = source.slice(source.lastIndexOf('\n', value.from - 1) + 1, value.from).match(/^\s*/)?.[0] ?? ''
  const propertyIndent = indent + '  '
  const serialize = (data: unknown, prefix: string) => JSON.stringify(data, null, 2).replace(/\n/g, '\n' + prefix)

  if (value.name !== 'ObjectExpression') {
    const original = source.slice(value.from, value.to)
    const properties = Object.entries(values).map(([key, data]) => `${propertyIndent}${key}: ${serialize(data, propertyIndent)},`)
    const replacement = `{\n${propertyIndent}...(${original}),\n${properties.join('\n')}\n${indent}}`

    return source.slice(0, value.from) + replacement + source.slice(value.to)
  }

  const properties = children(value).filter((node) => node.name === 'Property')
  const edits: { from: number; to: number; text: string }[] = []
  const additions: string[] = []

  for (const [key, data] of Object.entries(values)) {
    const property = [...properties].reverse().find((node) => {
      const name = node.firstChild
      const text = name && source.slice(name.from, name.to)
      return text === key || text === JSON.stringify(key) || text === `'${key}'`
    })
    // Append after spreads so that the form's value wins over inherited configuration.
    const spreadAfter = property && properties.some((node) => node.from > property.from && node.firstChild?.name === 'Spread')
    if (!property || spreadAfter) {
      additions.push(`${propertyIndent}${key}: ${serialize(data, propertyIndent)},`)
      continue
    }

    const nodes = children(property)
    const colon = nodes.findIndex((node) => node.name === ':')
    const current = colon >= 0 ? nodes[colon + 1] : null
    if (current) {
      // JSON-compatible values need no rewrite when unchanged, preserving inline comments.
      try {
        if (sameValue(literalValue(current, source), data)) continue
      } catch {
        // JavaScript expressions and single-quoted strings are replaced by the configured value.
      }

      const prefix = source.slice(source.lastIndexOf('\n', property.from - 1) + 1, property.from).match(/^\s*/)?.[0] ?? propertyIndent
      edits.push({ from: current.from, to: current.to, text: serialize(data, prefix) })
    } else {
      edits.push({ from: property.from, to: property.to, text: `${key}: ${serialize(data, propertyIndent)}` })
    }
  }

  if (additions.length) {
    const closing = value.lastChild!
    const last = children(value).at(-2)
    const lineStart = source.lastIndexOf('\n', closing.from - 1) + 1
    const from = source.slice(lineStart, closing.from).trim() === '' ? lineStart : closing.from
    edits.push({ from, to: closing.from, text: `${from === closing.from ? '\n' : ''}${additions.join('\n')}\n${indent}` })

    if (last && last.name !== '{' && last.name !== ',') edits.push({ from: last.to, to: last.to, text: ',' })
  }

  for (const edit of edits.sort((a, b) => b.from - a.from)) source = source.slice(0, edit.from) + edit.text + source.slice(edit.to)

  return source
}
