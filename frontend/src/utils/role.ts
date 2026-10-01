import type { MemberRole, ProjectRole } from '@/types/bitable'
import { lazyLabels, t } from '../i18n/translate.ts'

const LEVELS: Record<ProjectRole, number> = { viewer: 1, editor: 2, manager: 3, owner: 4 }

/** Reports whether the role has all the permissions of min, the same as db.ProjectRole.AtLeast of the backend. */
export function roleAtLeast(role: ProjectRole | undefined | null, min: ProjectRole): boolean {
  return !!role && LEVELS[role] >= LEVELS[min]
}

export const ROLE_LABELS: Record<ProjectRole, string> = lazyLabels({
  owner: () => t('role.owner'),
  manager: () => t('role.manager'),
  editor: () => t('role.editor'),
  viewer: () => t('role.viewer'),
})

const ROLE_DESCRIPTIONS: Record<MemberRole, string> = lazyLabels({
  manager: () => t('role.managerDescription'),
  editor: () => t('role.editorDescription'),
  viewer: () => t('role.viewerDescription'),
})

export const MEMBER_ROLES: { role: MemberRole; label: string; description: string }[] = (['manager', 'editor', 'viewer'] as const).map(
  (role) => ({
    role,
    get label() {
      return ROLE_LABELS[role]
    },
    get description() {
      return ROLE_DESCRIPTIONS[role]
    },
  }),
)
