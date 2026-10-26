import type { FieldType } from '@/types/bitable'
import type { ListParams } from './admin'
import type { PluginLanguage } from '../utils/workflowPlugin'
import type {
  AdminFieldShortcut,
  FieldShortcutManifest as ApiManifest,
  SaveFieldShortcut,
  ShortcutField,
  ShortcutJob as ApiJob,
  ShortcutSection,
  TestFieldShortcutResp,
} from './api'
import { client } from './client'

export type { AdminFieldShortcut, SaveFieldShortcut, ShortcutField, ShortcutSection, TestFieldShortcutResp }

export type ShortcutKind = 'script'

export type FieldShortcutManifest = Omit<ApiManifest, 'kind' | 'resultTypes'> & { kind: ShortcutKind; resultTypes: FieldType[] }

export type ShortcutJobStatus = 'pending' | 'running' | 'failed' | 'done'

export type ShortcutJob = Omit<ApiJob, 'status'> & { status: ShortcutJobStatus }

/** Data of the SHORTCUT_JOBS WebSocket message, the same as dto.ShortcutJobsMessage of the backend. */
export interface ShortcutJobsMessage {
  /** Table of the jobs. */
  tableUID: string
  /** Fetch all the jobs of the table instead of applying the jobs. */
  reload?: boolean
  /** Changed jobs, a done job has been removed. Empty if reload is true. */
  jobs: ShortcutJob[]
}

export type ShortcutRunScope = 'all' | 'empty' | 'records'

export const shortcutsApi = {
  list: async (projectUID: string) => (await client.projects.listFieldShortcuts(projectUID)).data as FieldShortcutManifest[],
  jobs: async (projectUID: string, tableUID: string) => (await client.projects.listShortcutJobs(projectUID, tableUID)).data as ShortcutJob[],
  run: async (projectUID: string, tableUID: string, fieldUID: string, scope: ShortcutRunScope, recordUIDs?: string[]) =>
    (await client.projects.runFieldShortcut(projectUID, tableUID, fieldUID, { scope, recordUIDs })).data.count,
}

export const adminShortcutsApi = {
  list: async (params: ListParams = {}) => (await client.admin.listAdminFieldShortcuts(params)).data,
  get: async (uid: string) => (await client.admin.getAdminFieldShortcut(uid)).data,
  create: async (body: SaveFieldShortcut) => (await client.admin.createAdminFieldShortcut(body)).data,
  update: async (uid: string, body: SaveFieldShortcut) => (await client.admin.updateAdminFieldShortcut(uid, body)).data,
  delete: async (uid: string) => {
    await client.admin.deleteAdminFieldShortcut(uid)
  },
  inspect: async (body: { source?: string; url?: string; language?: PluginLanguage }) => (await client.admin.inspectAdminFieldShortcut(body)).data,
  install: async (source: string, language: PluginLanguage) => (await client.admin.installAdminFieldShortcut({ source, language })).data,
  test: async (body: SaveFieldShortcut, params: Record<string, unknown>, uid?: string) =>
    (await client.admin.testAdminFieldShortcut({ uid, shortcut: body, params })).data,
}
