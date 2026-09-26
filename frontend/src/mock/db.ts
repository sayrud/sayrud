import type { Project, SLField, SLRecord, SLTable, SLView } from '@/types/bitable'
import { buildSeed } from './seed'

export interface MockDatabase {
  version: number
  projects: Project[]
  tables: SLTable[]
  fields: SLField[]
  records: SLRecord[]
  views: SLView[]
}

const STORAGE_KEY = 'sayrud:mock-db'
const VERSION = 1

function load(): MockDatabase {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as MockDatabase
      if (parsed.version === VERSION) return parsed
    }
  } catch {
    // 数据损坏时回退到示例数据。
  }
  return { version: VERSION, ...buildSeed() }
}

export const db: MockDatabase = load()

let saveTimer: ReturnType<typeof setTimeout> | undefined

export function persist() {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(db))
    } catch {
      // 超出存储配额时只保留内存数据。
    }
  }, 200)
}

export function resetMockDatabase() {
  localStorage.removeItem(STORAGE_KEY)
  Object.assign(db, { version: VERSION, ...buildSeed() })
  persist()
}

export const now = () => new Date().toISOString()
