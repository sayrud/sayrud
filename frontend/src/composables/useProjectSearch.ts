import { adminApi } from '@/api/admin'
import { projectsApi } from '@/api/bitable'
import type { TokenProjectOption } from '@/utils/accessToken'

/** Searches the bases the signed-in user takes part in, they are few enough to load once and filter locally. */
export function useMemberProjectSearch() {
  let projects: Promise<TokenProjectOption[]> | null = null
  return async (keyword: string): Promise<TokenProjectOption[]> => {
    projects ??= projectsApi
      .list()
      .then(({ projects }) => projects)
      .catch((e: unknown) => {
        projects = null
        throw e
      })
    const lower = keyword.toLowerCase()
    return (await projects).filter((p) => p.name.toLowerCase().includes(lower)).slice(0, 50)
  }
}

/** Searches every base of the site, for the system bots managed by admins. */
export function useSiteProjectSearch() {
  return async (keyword: string): Promise<TokenProjectOption[]> =>
    (await adminApi.projects({ keyword: keyword || undefined, pageSize: 20 })).projects
}
