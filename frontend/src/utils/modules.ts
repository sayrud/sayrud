import type { Component } from 'vue'
import { defineComponent, h, shallowRef } from 'vue'
import {
  Bell,
  Blocks,
  Bot,
  Braces,
  Calculator,
  Calendar,
  ChartBar,
  ChartLine,
  Clock,
  Cloud,
  Code,
  Cpu,
  CreditCard,
  Database,
  FilePlus2,
  FileText,
  Filter,
  GitBranch,
  Globe,
  Hand,
  Hash,
  Image,
  KeyRound,
  Languages,
  Link,
  Mail,
  MapPin,
  Megaphone,
  MessageCircle,
  MessageSquare,
  Package,
  PencilLine,
  Phone,
  Puzzle,
  QrCode,
  Reply,
  Rss,
  Send,
  Server,
  ShoppingCart,
  Shuffle,
  Smartphone,
  Sparkles,
  Table2,
  Tag,
  Terminal,
  Timer,
  Truck,
  User,
  Users,
  Wallet,
  Webhook,
  Workflow,
  Wrench,
  Zap,
} from '@lucide/vue'

import { i18n, t } from '@/i18n'
import type { CredentialTypeDef, FlowKind, ModuleCategory, ModuleDef, StepType, WorkflowModules, WorkflowStep } from '@/types/workflow'
import { isImageIcon } from '@/utils/workflowPlugin'

/** Maps the icon names in the DSL to icon components; plugins pick from the same set, unknown names fall back to a generic icon. */
const ICONS: Record<string, Component> = {
  database: Database,
  'file-plus': FilePlus2,
  'pencil-line': PencilLine,
  'git-branch': GitBranch,
  reply: Reply,
  mail: Mail,
  globe: Globe,
  key: KeyRound,
  bell: Bell,
  blocks: Blocks,
  bot: Bot,
  braces: Braces,
  calculator: Calculator,
  calendar: Calendar,
  'chart-bar': ChartBar,
  'chart-line': ChartLine,
  clock: Clock,
  cloud: Cloud,
  code: Code,
  cpu: Cpu,
  'credit-card': CreditCard,
  'file-text': FileText,
  filter: Filter,
  hand: Hand,
  hash: Hash,
  image: Image,
  'key-round': KeyRound,
  languages: Languages,
  link: Link,
  'map-pin': MapPin,
  megaphone: Megaphone,
  'message-circle': MessageCircle,
  'message-square': MessageSquare,
  package: Package,
  phone: Phone,
  puzzle: Puzzle,
  'qr-code': QrCode,
  rss: Rss,
  send: Send,
  server: Server,
  'shopping-cart': ShoppingCart,
  shuffle: Shuffle,
  smartphone: Smartphone,
  sparkles: Sparkles,
  table: Table2,
  tag: Tag,
  terminal: Terminal,
  timer: Timer,
  truck: Truck,
  user: User,
  users: Users,
  wallet: Wallet,
  webhook: Webhook,
  workflow: Workflow,
  wrench: Wrench,
  zap: Zap,
}

const imageIcons = new Map<string, Component>()

/** An icon component that renders an embedded plugin image at the size of the Lucide icons. */
function imageIcon(src: string): Component {
  let icon = imageIcons.get(src)
  if (!icon) {
    icon = defineComponent({
      props: { size: { type: [Number, String], default: 16 } },
      setup: (props) => () => h('img', { src, alt: '', width: props.size, height: props.size, draggable: false, style: 'display:block;object-fit:contain' }),
    })
    imageIcons.set(src, icon)
  }
  return icon
}

// DSL text keys come from the backend definition files, which scripts/check-i18n.mjs scans directly, so they must not be passed to a literal t(key) call here.
const translate = t
const hasKey = i18n.global.te

/** Translates an i18n key from the DSL or from backend issue args; text that is not a key (e.g. the placeholder smtp.example.com) is returned as is, and empty values return an empty string. */
export function dslText(key: string | undefined, args?: Record<string, unknown>): string {
  if (!key) return ''
  return hasKey(key) || hasKey(key, 'zh-CN') ? translate(key, args ?? {}) : key
}

export function moduleIcon(name: string | undefined): Component {
  if (isImageIcon(name)) return imageIcon(name)
  return (name && ICONS[name]) || Blocks
}

const defs = shallowRef<WorkflowModules>({ modules: [], credentialTypes: [], secretsReady: false })
let byType = new Map<string, ModuleDef>()
let credentialByType = new Map<string, CredentialTypeDef>()

/** Sets the module definitions from the backend; called before the editor loads a workflow. */
export function setModuleDefs(data: WorkflowModules) {
  const modules = [...data.modules].sort((a, b) => a.order - b.order)
  byType = new Map(modules.map((m) => [m.type, m]))
  credentialByType = new Map(data.credentialTypes.map((c) => [c.type, c]))
  defs.value = { ...data, modules }
}

export function moduleDef(type: StepType): ModuleDef | undefined {
  return byType.get(type)
}

export function credentialTypeDef(type: string | undefined): CredentialTypeDef | undefined {
  return type ? credentialByType.get(type) : undefined
}

export function secretsReady() {
  return defs.value.secretsReady
}

/** Step types that can be added to a kind of flow, sorted by the DSL order; disabled plugins are left out. */
export function stepTypesFor(flow: FlowKind): StepType[] {
  return defs.value.modules.filter((m) => m.flows.includes(flow) && m.plugin?.enabled !== false).map((m) => m.type)
}

export interface StepMeta {
  icon: Component
  /** Accent color of the node icon. */
  color: string
  label: string
  description: string
}

/** Icon, color and name of a node; unknown types show the raw type name. */
export function stepMeta(type: StepType): StepMeta {
  const def = byType.get(type)
  return {
    icon: moduleIcon(def?.icon),
    color: def?.color ?? '#8f959e',
    label: def ? dslText(def.label) : type,
    description: dslText(def?.description),
  }
}

/** Categories of the node picker in display order. */
export const MODULE_CATEGORIES: ModuleCategory[] = ['data', 'logic', 'integration']

/** Group of a node in the node picker; unknown types fall into the last group. */
export function stepCategory(type: StepType): ModuleCategory {
  return byType.get(type)?.category ?? 'integration'
}

/** Steps with then / else branches (condition). */
export function hasBranches(step: Pick<WorkflowStep, 'type'>) {
  return !!byType.get(step.type)?.branches
}

/** Steps that return the response and end the sync flow. */
export function isTerminal(step: Pick<WorkflowStep, 'type'>) {
  return !!byType.get(step.type)?.terminal
}

/** UID of the table the step operates on, given by the DSL tableKey. */
export function stepTable(step: WorkflowStep | null | undefined): string | undefined {
  const key = step ? byType.get(step.type)?.tableKey : undefined
  const uid = key ? step!.config[key] : undefined
  return typeof uid === 'string' && uid ? uid : undefined
}

/** Whether a value is blank: unset, an empty string or an empty array. */
export function isBlank(v: unknown) {
  return v === undefined || v === null || v === '' || (Array.isArray(v) && v.length === 0)
}
