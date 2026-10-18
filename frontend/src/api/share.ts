import type { LinkShare, UpdateLinkShare } from './api'
import { client } from './client'
import type { Project, SLTable } from '@/types/bitable'

export type { LinkShare } from './api'

export const sharesApi = {
  get: async (projectUID: string, tableUID: string): Promise<LinkShare> =>
    (await client.projects.getLinkShare(projectUID, tableUID)).data,
  update: async (projectUID: string, tableUID: string, data: UpdateLinkShare): Promise<LinkShare> =>
    (await client.projects.updateLinkShare(projectUID, tableUID, data)).data,
  resolve: async (projectUID: string, tableUID?: string, shareToken?: string) =>
    (await client.shares.resolveLinkShare({ projectUID, tableUID, shareToken })).data,
  open: async (token: string) =>
    (await client.shares.openLinkShare(token)).data as { project: Project; tables: SLTable[]; tableUID: string; includeChildren: boolean },
  unlock: async (token: string, password: string) => {
    await client.shares.unlockLinkShare(token, { password })
  },
}
