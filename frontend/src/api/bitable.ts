import type {
  FieldMetadata,
  FieldType,
  Project,
  QueryRecordsOptions,
  RecordData,
  SLField,
  SLRecord,
  SLTable,
  SLView,
  ViewConfig,
  ViewType,
} from '@/types/bitable'
import { http } from './http'

const p = (projectUID: string) => `/projects/${projectUID}`
const t = (projectUID: string, tableUID: string) => `${p(projectUID)}/tables/${tableUID}`

export interface ProjectListItem extends Project {
  tableCount: number
}

export interface TableListItem extends SLTable {
  count: number
}

export interface CreateFieldInput {
  label: string
  type: FieldType
  metadata: FieldMetadata
}

export interface UpdateFieldInput {
  label?: string
  type?: FieldType
  metadata?: FieldMetadata
}

export const projectsApi = {
  list: () => http.get<{ projects: ProjectListItem[]; total: number }>('/projects'),
  get: (projectUID: string) => http.get<Project>(p(projectUID)),
  create: (name: string) => http.post<Project>('/projects', { name }),
  update: (projectUID: string, name: string) => http.put(p(projectUID), { name }),
  delete: (projectUID: string) => http.delete(p(projectUID)),
}

export const tablesApi = {
  list: (projectUID: string) => http.get<{ tables: TableListItem[]; total: number }>(`${p(projectUID)}/tables`),
  create: (projectUID: string, name: string) => http.post<SLTable>(`${p(projectUID)}/tables`, { name }),
  update: (projectUID: string, tableUID: string, name: string) => http.put(t(projectUID, tableUID), { name }),
  delete: (projectUID: string, tableUID: string) => http.delete(t(projectUID, tableUID)),
  fieldTypes: (projectUID: string) => http.get<Record<FieldType, string>>(`${p(projectUID)}/tables/types`),
}

export const fieldsApi = {
  list: (projectUID: string, tableUID: string) => http.get<SLField[]>(`${t(projectUID, tableUID)}/fields`),
  create: (projectUID: string, tableUID: string, fields: CreateFieldInput[]) =>
    http.post<SLField[]>(`${t(projectUID, tableUID)}/fields`, { fields }),
  update: (projectUID: string, tableUID: string, fieldUID: string, input: UpdateFieldInput) =>
    http.put<SLField>(`${t(projectUID, tableUID)}/fields/${fieldUID}`, input),
  setPosition: (projectUID: string, tableUID: string, fieldUID: string, position: number) =>
    http.put(`${t(projectUID, tableUID)}/fields/${fieldUID}/position`, { position }),
  delete: (projectUID: string, tableUID: string, fieldUID: string) =>
    http.delete(`${t(projectUID, tableUID)}/fields/${fieldUID}`),
}

export const recordsApi = {
  list: (projectUID: string, tableUID: string, limit = 0, offset = 0) =>
    http.get<{ records: SLRecord[]; total: number }>(
      `${t(projectUID, tableUID)}/records?limit=${limit}&offset=${offset}`,
    ),
  query: (projectUID: string, tableUID: string, options: QueryRecordsOptions) =>
    http.post<{ records: SLRecord[]; total: number }>(`${t(projectUID, tableUID)}/records/query`, options),
  create: (projectUID: string, tableUID: string, data: RecordData) =>
    http.post<SLRecord>(`${t(projectUID, tableUID)}/records`, { data }),
  batchCreate: (projectUID: string, tableUID: string, data: RecordData[]) =>
    http.post<SLRecord[]>(`${t(projectUID, tableUID)}/records/batch`, { data }),
  update: (projectUID: string, tableUID: string, recordUID: string, data: RecordData) =>
    http.put(`${t(projectUID, tableUID)}/records/${recordUID}`, { data }),
  delete: (projectUID: string, tableUID: string, recordUID: string) =>
    http.delete(`${t(projectUID, tableUID)}/records/${recordUID}`),
}

export const viewsApi = {
  list: (projectUID: string, tableUID: string) => http.get<SLView[]>(`${t(projectUID, tableUID)}/views`),
  create: (projectUID: string, tableUID: string, input: { name: string; type: ViewType; config?: Partial<ViewConfig> }) =>
    http.post<SLView>(`${t(projectUID, tableUID)}/views`, input),
  update: (
    projectUID: string,
    tableUID: string,
    viewUID: string,
    input: { name?: string; config?: ViewConfig; position?: number },
  ) => http.put<SLView>(`${t(projectUID, tableUID)}/views/${viewUID}`, input),
  delete: (projectUID: string, tableUID: string, viewUID: string) =>
    http.delete(`${t(projectUID, tableUID)}/views/${viewUID}`),
}
