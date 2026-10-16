import type { Component } from 'vue'
import { shallowRef } from 'vue'
import { Blocks, Database, FilePlus2, GitBranch, Globe, KeyRound, Mail, PencilLine, Reply } from '@lucide/vue'

import { i18n, t } from '@/i18n'
import type { CredentialTypeDef, FlowKind, ModuleCategory, ModuleDef, StepType, WorkflowModules, WorkflowStep } from '@/types/workflow'

/** Maps the icon names in the DSL to icon components; unknown names fall back to a generic icon. */
const ICONS: Record<string, Component> = {
  database: Database,
  'file-plus': FilePlus2,
  'pencil-line': PencilLine,
  'git-branch': GitBranch,
  reply: Reply,
  mail: Mail,
  globe: Globe,
  key: KeyRound,
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

export function modulesLoaded() {
  return byType.size > 0
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

/** Step types allowed in a kind of flow, sorted by the DSL order. */
export function stepTypesFor(flow: FlowKind): StepType[] {
  return defs.value.modules.filter((m) => m.flows.includes(flow)).map((m) => m.type)
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
