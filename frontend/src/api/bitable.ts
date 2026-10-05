import type { Attachment, FieldType, MemberRole, Project, ProjectMember, RecordData, SelectOption, SLField, SLRecord, SLTable, SLView, UserBrief } from '@/types/bitable'
import type { Changeset } from '@/collab/types'
import type { RequestParams, SLField as ApiField, SLRecord as ApiRecord, SLView as ApiView } from './api'
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

// The generated types describe metadata / config / data as loose records, narrow them to the precise frontend types.
const asFields = (list: ApiField[]) => list as unknown as SLField[]
const asRecords = (list: ApiRecord[]) => list as unknown as SLRecord[]
const asViews = (list: ApiView[]) => list as unknown as SLView[]

export const attachmentsApi = {
  upload: async (projectUID: string, field: SLField, file: File, params: RequestParams = {}): Promise<Attachment> =>
    (await client.projects.uploadAttachment(projectUID, field.tableUID, field.uid, { file }, params)).data,
}

export const projectsApi = {
  list: async () => (await client.projects.listProjects({ pageSize: 1000 })).data as { projects: ProjectListItem[]; total: number },
  get: async (projectUID: string) => (await client.projects.getProject(projectUID)).data as Project,
  create: async (name: string) => (await client.projects.createProject({ name })).data as Project,
  update: async (projectUID: string, patch: string | Partial<Pick<Project, 'name' | 'icon' | 'color'>>) => {
    await client.projects.updateProject(projectUID, typeof patch === 'string' ? { name: patch } : patch)
  },
  delete: async (projectUID: string) => {
    await client.projects.deleteProject(projectUID)
  },
  transferOwner: async (projectUID: string, userID: number) => {
    await client.projects.transferProjectOwner(projectUID, { userID })
  },
}

export const membersApi = {
  list: async (projectUID: string) => (await client.projects.listProjectMembers(projectUID)).data as ProjectMember[],
  lookup: async (projectUID: string, email: string) =>
    (await client.projects.lookupProjectMemberCandidate(projectUID, { email })).data as UserBrief,
  add: async (projectUID: string, email: string, role: MemberRole) =>
    (await client.projects.addProjectMember(projectUID, { email, role })).data as ProjectMember,
  update: async (projectUID: string, userID: number, role: MemberRole) => {
    await client.projects.updateProjectMember(projectUID, userID, { role })
  },
  remove: async (projectUID: string, userID: number) => {
    await client.projects.removeProjectMember(projectUID, userID)
  },
}

export const tablesApi = {
  list: async (projectUID: string) => (await client.projects.listTables(projectUID, { pageSize: 1000 })).data,
  create: async (projectUID: string, name: string) => (await client.projects.createTable(projectUID, { name })).data as SLTable,
  update: async (projectUID: string, tableUID: string, patch: string | Partial<Pick<SLTable, 'name' | 'icon' | 'color'>>) => {
    await client.projects.updateTable(projectUID, tableUID, typeof patch === 'string' ? { name: patch } : patch)
  },
  delete: async (projectUID: string, tableUID: string) => {
    await client.projects.deleteTable(projectUID, tableUID)
  },
  fieldTypes: async (projectUID: string) => (await client.projects.listFieldTypes(projectUID)).data as Record<FieldType, string>,
}

export const syncApi = {
  options: async (projectUID: string, field: SLField, data: RecordData) =>
    (await client.projects.resolveFieldOptions(projectUID, field.tableUID, { fieldUID: field.uid, type: field.type, metadata: field.metadata, data })).data as unknown as SelectOption[],
  snapshot: async (projectUID: string, tableUID: string): Promise<TableSnapshot> => {
    const s = (await client.projects.getTableSnapshot(projectUID, tableUID)).data
    return { rev: s.rev, fields: asFields(s.fields), views: asViews(s.views), records: asRecords(s.records) }
  },
  /** Fetches all the changesets after since, following the pages automatically. */
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
