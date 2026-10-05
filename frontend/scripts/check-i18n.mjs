import { readdirSync, readFileSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { ESLint } from 'eslint'
import vue from 'eslint-plugin-vue'
import ts from 'typescript-eslint'

import zhCN from '../src/locales/zh-CN.ts'
import { AI_PROVIDERS } from '../src/utils/aiProviders.ts'
import { FUNCTIONS } from '../src/utils/formula.ts'
import { SSO_ERROR_CODES } from '../src/utils/ssoError.ts'
import { TEMPLATES } from '../src/utils/ssoTemplates.ts'

const FRONTEND = join(dirname(fileURLToPath(import.meta.url)), '..')
const ROOT = join(FRONTEND, '..')

function leaves(messages, prefix = '') {
  return Object.entries(messages).flatMap(([name, value]) => {
    const key = prefix ? `${prefix}.${name}` : name
    return typeof value === 'string' ? [key] : leaves(value, key)
  })
}

// Read icon/color names without loading their browser component imports in Node.
function appearanceKeys(name) {
  const { ast } = ts.parser.parseForESLint(readFileSync(join(FRONTEND, 'src/utils/appearance.ts'), 'utf8'))
  for (const statement of ast.body) {
    if (statement.type !== 'ExportNamedDeclaration' || statement.declaration?.type !== 'VariableDeclaration') continue
    for (const declaration of statement.declaration.declarations) {
      if (declaration.id.type !== 'Identifier' || declaration.id.name !== name) continue
      const init = declaration.init?.type === 'TSAsExpression' ? declaration.init.expression : declaration.init
      if (init?.type === 'ObjectExpression') {
        return init.properties.flatMap((property) => {
          if (property.type !== 'Property') return []
          return property.key.type === 'Identifier' ? [property.key.name] : [String(property.key.value)]
        })
      }
    }
  }
  throw new Error(`Cannot read ${name}`)
}

// Finite domains keep dynamic lookups accurate: a removed provider/function is still reported as unused.
const DYNAMIC_KEYS = new Map([
  ['formula.fn.${}.usage', Object.keys(FUNCTIONS).map((name) => `formula.fn.${name}.usage`)],
  ['formula.fn.${}.desc', Object.keys(FUNCTIONS).map((name) => `formula.fn.${name}.desc`)],
  ['formula.category.${}', [...new Set(Object.values(FUNCTIONS).map((fn) => `formula.category.${fn.category}`))]],
  ['sso.errors.${}', SSO_ERROR_CODES.map((code) => `sso.errors.${code}`)],
  ['sso.admin.templateDesc.${}', TEMPLATES.map((template) => `sso.admin.templateDesc.${template.key}`)],
  ['admin.ai.providerNames.${}', AI_PROVIDERS.map((provider) => `admin.ai.providerNames.${provider.id}`)],
  ['appearance.colors.${}', appearanceKeys('APPEARANCE_COLORS').map((key) => `appearance.colors.${key}`)],
  ['appearance.icons.${}', appearanceKeys('APPEARANCE_ICONS').map((key) => `appearance.icons.${key}`)],
])

const UI_PROPERTIES = new Set([
  'label', 'title', 'description', 'placeholder', 'message', 'content', 'tooltip',
  'okText', 'cancelText', 'emptyText', 'help', 'alt', 'aria-label', 'aria-placeholder',
  'aria-description', 'aria-roledescription', 'aria-valuetext',
])
const hasWords = (text) => /\p{L}/u.test(text)
const hasHan = (text) => /\p{Script=Han}/u.test(text)

// Exact, file-scoped exceptions for technical examples and names that keep their original spelling.
const LITERAL_ALLOWLIST = {
  'i18n/index.ts': ['English', '简体中文', '繁體中文', '日本語', '한국어', 'Español', 'Português', 'Français', 'Deutsch', 'Русский'], // Native language names.
  'utils/fieldTypes.ts': ['YYYY年MM月DD日', '2026年01月30日'], // Date format and its example.
  'utils/format.ts': ['是'], // Accepted input when importing checkbox values.
  'pages/HomePage.vue': ['Sayrud'], // Brand.
  'pages/admin/SitePage.vue': ['Sayrud'], // Default site name.
  'pages/admin/OverviewPage.vue': ['Go'], // Runtime name.
  'pages/admin/FieldShortcutEditPage.vue': ['Bearer', 'Authorization'], // HTTP syntax.
  'pages/AuthPage.vue': ['name@example.com'], // Example email address.
  'components/admin/AuthProviderDrawer.vue': [ // Protocol terms and example configuration values.
    '{username}', 'https://', 'Issuer', 'https://accounts.google.com', 'Client ID', 'Client Secret', 'Scopes',
    'id', 'email', 'name', 'groups', 'ldaps://ldap.example.com', 'StartTLS', 'Bind DN', 'Base DN',
    'cn=sayrud,ou=services,dc=example,dc=com', 'dc=example,dc=com', '-----BEGIN CERTIFICATE-----',
    'uid', 'mail', 'displayName', 'memberOf',
  ],
}

export async function audit(
  sources,
  keys = leaves(zhCN),
  dynamicKeys = DYNAMIC_KEYS,
) {
  const defined = new Set(keys)
  const roots = new Set(keys.map((key) => key.split('.')[0]))
  const used = new Set()
  const checker = new ESLint({
    cwd: FRONTEND,
    overrideConfigFile: true,
    overrideConfig: [
      ...vue.configs['flat/base'],
      { files: ['**/*.ts'], languageOptions: { parser: ts.parser } },
      {
        files: ['**/*.{ts,vue}'],
        linterOptions: { reportUnusedDisableDirectives: false },
        languageOptions: { parserOptions: { parser: ts.parser } },
        plugins: {
          '@typescript-eslint': ts.plugin,
          'i18n-audit': {
            rules: {
              source: {
                meta: { schema: [], messages: {
                  hardcoded: 'Hardcoded UI text: {{text}}',
                  missing: 'Undefined translation key: {{key}}',
                  dynamic: 'Unresolved translation key; add its finite domain to DYNAMIC_KEYS: {{key}}',
                } },
                create(context) {
                  const services = context.sourceCode.parserServices
                  const report = (node, text) => {
                    if (LITERAL_ALLOWLIST[relative(join(FRONTEND, 'src'), context.filename)]?.includes(text)) return
                    if (hasWords(text)) context.report({ node, messageId: 'hardcoded', data: { text } })
                  }
                  const use = (node, key) => {
                    used.add(key)
                    if (!defined.has(key)) context.report({ node, messageId: 'missing', data: { key } })
                  }
                  const templateKey = (node) => {
                    const pattern = node.quasis.map((part) => part.value.cooked ?? part.value.raw).join('${}')
                    const resolved = dynamicKeys.get(pattern)
                    if (resolved) resolved.forEach((key) => use(node, key))
                    else context.report({ node, messageId: 'dynamic', data: { key: pattern } })
                  }
                  const translation = (node) => {
                    if (node.type === 'Literal' && typeof node.value === 'string') use(node, node.value)
                    else if (node.type === 'TemplateLiteral') {
                      if (node.expressions.length) templateKey(node)
                      else use(node, node.quasis[0].value.cooked)
                    } else if (node.type === 'ConditionalExpression') {
                      translation(node.consequent)
                      translation(node.alternate)
                    } else if (node.type === 'CallExpression' && ['ssoErrorKey', 'formItemIssueKey'].includes(node.callee.name)) {
                      // Their return values are collected from the helper's literal keys / finite template domain.
                    } else context.report({ node, messageId: 'dynamic', data: { key: context.sourceCode.getText(node) } })
                  }
                  const rawExpression = (node) => {
                    if (!node) return
                    if (node.type === 'Literal' && typeof node.value === 'string') report(node, node.value)
                    else if (node.type === 'TemplateLiteral') node.quasis.forEach((part) => report(part, part.value.cooked ?? part.value.raw))
                    else if (node.type === 'ConditionalExpression') {
                      rawExpression(node.consequent)
                      rawExpression(node.alternate)
                    } else if (node.type === 'LogicalExpression' || node.type === 'BinaryExpression') {
                      rawExpression(node.left)
                      rawExpression(node.right)
                    } else if (node.type === 'CallExpression' && node.callee.name === 'ref') {
                      rawExpression(node.arguments[0])
                    }
                  }
                  const visitor = {
                    CallExpression(node) {
                      const callee = node.callee
                      const name = callee.type === 'Identifier' ? callee.name : callee.property?.name
                      if (['t', '$t', 'te', '$te', 'tm', '$tm'].includes(name) && node.arguments[0]) translation(node.arguments[0])
                      if (callee.type === 'Identifier' && ['alert', 'confirm', 'prompt'].includes(name)) rawExpression(node.arguments[0])
                      if (callee.type === 'MemberExpression' && callee.object.name === 'Message') rawExpression(node.arguments[0])
                    },
                    Literal(node) {
                      if (typeof node.value !== 'string') return
                      const parent = node.parent
                      if (parent.type === 'TSLiteralType' || (parent.type === 'Property' && parent.key === node)) return
                      // Includes keys returned by helpers and held in lookup tables; comments and tests never count as usage.
                      if (defined.has(node.value)) used.add(node.value)
                      if (hasHan(node.value)) report(node, node.value)
                    },
                    TemplateLiteral(node) {
                      const head = node.quasis[0].value.cooked ?? ''
                      if (node.expressions.length && roots.has(head.split('.')[0])) templateKey(node)
                      for (const part of node.quasis) if (hasHan(part.value.cooked ?? '')) report(part, part.value.cooked)
                    },
                    Property(node) {
                      const name = node.key.name ?? node.key.value
                      if (UI_PROPERTIES.has(name) && node.value.type !== 'FunctionExpression' && !defined.has(node.value.value)) rawExpression(node.value)
                    },
                    VariableDeclarator(node) {
                      if (UI_PROPERTIES.has(node.id.name)) rawExpression(node.init)
                    },
                    AssignmentExpression(node) {
                      const left = node.left
                      const name = left.property?.name === 'value' ? left.object.name : left.property?.name ?? left.name
                      if (UI_PROPERTIES.has(name)) rawExpression(node.right)
                    },
                  }
                  return services.defineTemplateBodyVisitor?.({
                    ...visitor,
                    VText(node) { report(node, node.value.trim()) },
                    VAttribute(node) {
                      if (!node.directive && node.key.name === 'keypath' && node.value) use(node.value, node.value.value)
                      if (node.directive && node.key.name.name === 'bind' && node.key.argument?.name === 'keypath' && node.value?.expression) translation(node.value.expression)
                      if (!node.directive && UI_PROPERTIES.has(node.key.name) && node.value) report(node.value, node.value.value)
                      if (node.directive && node.key.name.name === 'bind' && UI_PROPERTIES.has(node.key.argument?.name)) rawExpression(node.value?.expression)
                      if (node.directive && ['text', 'html'].includes(node.key.name.name)) rawExpression(node.value?.expression)
                    },
                    VExpressionContainer(node) {
                      if (node.parent.type !== 'VAttribute') rawExpression(node.expression)
                    },
                  }, visitor) ?? visitor
                },
              },
            },
          },
        },
        rules: { 'i18n-audit/source': 'error' },
      },
    ],
  })
  const diagnostics = []
  const seen = new Set()
  for (const { filename, code } of sources) {
    const [result] = await checker.lintText(code, { filePath: filename })
    for (const message of result.messages) {
      const id = `${filename}:${message.line}:${message.column}:${message.message}`
      if (seen.has(id)) continue
      seen.add(id)
      diagnostics.push({
        file: relative(ROOT, filename), line: message.line ?? 1, column: message.column ?? 1, message: message.message,
      })
    }
  }
  return {
    diagnostics,
    unused: keys.filter((key) => !used.has(key)).sort(),
  }
}

function sourceFiles(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) return entry.name === 'locales' || entry.name === 'scripts' ? [] : sourceFiles(path)
    return /\.(ts|vue)$/.test(path) && path !== join(FRONTEND, 'src/api/api.ts') ? [path] : []
  })
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const { diagnostics, unused } = await audit(sourceFiles(join(FRONTEND, 'src')).map((filename) => ({
    filename, code: readFileSync(filename, 'utf8'),
  })))
  for (const { file, line, column, message } of diagnostics) console.error(`${file}:${line}:${column}: ${message}`)
  for (const key of unused) console.error(`frontend/src/locales/zh-CN.ts: Unused translation key: ${key}`)
  console.log(`Frontend i18n: ${diagnostics.length} source issue(s), ${unused.length} unused key(s).`)
  if (diagnostics.length || unused.length) process.exitCode = 1
}
