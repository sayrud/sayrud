import type { FieldType, Project, SLField, SLRecord, SLTable, SLView } from '@/types/bitable'
import type { Changeset } from '@/collab/types'
import type { SLField as ApiField, SLRecord as ApiRecord, SLView as ApiView } from './api'
import { client } from './client'

export interface ProjectListItem extends Project {
  tableCount: number
}

export interface TableListItem extends SLTable {
  count: number
}

export interface TableSnapshot {
  rev: number
  fields: SLField[]
  views: SLView[]
  records: SLRecord[]
}

// 生成的类型把 metadata / config / data 描述为宽松的 Record，这里收窄为前端的精确类型。
const asFields = (list: ApiField[]) => list as unknown as SLField[]
const asRecords = (list: ApiRecord[]) => list as unknown as SLRecord[]
const asViews = (list: ApiView[]) => list as unknown as SLView[]

export const projectsApi = {
  list: async () => (await client.projects.listProjects({ pageSize: 1000 })).data,
  get: async (projectUID: string) => (await client.projects.getProject(projectUID)).data as Project,
  create: async (name: string) => (await client.projects.createProject({ name })).data as Project,
  update: async (projectUID: string, name: string) => {
    await client.projects.updateProject(projectUID, { name })
  },
  delete: async (projectUID: string) => {
    await client.projects.deleteProject(projectUID)
  },
}

export const tablesApi = {
  list: async (projectUID: string) => (await client.projects.listTables(projectUID, { pageSize: 1000 })).data,
  create: async (projectUID: string, name: string) => (await client.projects.createTable(projectUID, { name })).data as SLTable,
  update: async (projectUID: string, tableUID: string, name: string) => {
    await client.projects.updateTable(projectUID, tableUID, { name })
  },
  delete: async (projectUID: string, tableUID: string) => {
    await client.projects.deleteTable(projectUID, tableUID)
  },
  fieldTypes: async (projectUID: string) => (await client.projects.listFieldTypes(projectUID)).data as Record<FieldType, string>,
}

export const syncApi = {
  snapshot: async (projectUID: string, tableUID: string): Promise<TableSnapshot> => {
    const s = (await client.projects.getTableSnapshot(projectUID, tableUID)).data
    return { rev: s.rev, fields: asFields(s.fields), views: asViews(s.views), records: asRecords(s.records) }
  },
  /** 拉取 since 之后的全部 changeset，自动翻页。 */
  changesets: async (projectUID: string, tableUID: string, since: number): Promise<Changeset[]> => {
    const all: Changeset[] = []
    for (;;) {
      const resp = (await client.projects.listChangesets(projectUID, tableUID, { since })).data
      all.push(...(resp.changesets as Changeset[]))
      if (!resp.hasMore || !resp.changesets.length) return all
      since = resp.changesets[resp.changesets.length - 1]!.rev
    }
  },
  fields: async (projectUID: string, tableUID: string) => asFields((await client.projects.listFields(projectUID, tableUID)).data),
  views: async (projectUID: string, tableUID: string) => asViews((await client.projects.listViews(projectUID, tableUID)).data),
  fetchRecords: async (projectUID: string, tableUID: string, uids: string[]) => {
    const out: SLRecord[] = []
    for (let i = 0; i < uids.length; i += 1000) {
      out.push(...asRecords((await client.projects.fetchRecords(projectUID, tableUID, { uids: uids.slice(i, i + 1000) })).data))
    }
    return out
  },
}
