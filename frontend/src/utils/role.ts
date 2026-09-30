import type { MemberRole, ProjectRole } from '@/types/bitable'

const LEVELS: Record<ProjectRole, number> = { viewer: 1, editor: 2, manager: 3, owner: 4 }

/** Reports whether the role has all the permissions of min, the same as db.ProjectRole.AtLeast of the backend. */
export function roleAtLeast(role: ProjectRole | undefined | null, min: ProjectRole): boolean {
  return !!role && LEVELS[role] >= LEVELS[min]
}

export const ROLE_LABELS: Record<ProjectRole, string> = {
  owner: '所有者',
  manager: '可管理',
  editor: '可编辑',
  viewer: '可查看',
}

export const MEMBER_ROLES: { role: MemberRole; label: string; description: string }[] = [
  { role: 'manager', label: '可管理', description: '可编辑内容，并管理协作者和权限' },
  { role: 'editor', label: '可编辑', description: '可编辑数据表、字段、视图和记录' },
  { role: 'viewer', label: '可查看', description: '只能查看，筛选和排序仅自己可见' },
]
