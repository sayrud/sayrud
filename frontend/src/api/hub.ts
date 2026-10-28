import type {
  HubInstalled,
  HubOrigin,
  HubPackage,
  HubPackageDetail,
  HubPermissions,
  HubRelease,
  HubReleaseDetail,
  HubUpdate,
  HubUpdatesResp,
} from './api'
import { client } from './client'

export type { HubInstalled, HubOrigin, HubPackage, HubPackageDetail, HubPermissions, HubRelease, HubReleaseDetail, HubUpdate, HubUpdatesResp }

export type HubKind = HubUpdate['kind']

export type HubSort = 'popular' | 'recent' | 'new'

export type HubCategory = 'data' | 'logic' | 'integration'

export interface HubCatalogQuery {
  kind: HubKind
  q?: string
  /** Only applies to the workflow plugins. */
  category?: HubCategory
  sort?: HubSort
  page?: number
  pageSize?: number
}

export const hubApi = {
  /** Checks the packages installed from Sayrud Hub; refresh asks the Hub again instead of reusing the last result. */
  updates: async (refresh = false) => (await client.admin.checkAdminHubUpdates(refresh ? { refresh } : {})).data,
  /** Searches the catalog of Sayrud Hub, with the installation of each package. */
  packages: async (query: HubCatalogQuery) => (await client.admin.listAdminHubPackages(query)).data,
  package: async (kind: HubKind, id: string) => (await client.admin.getAdminHubPackage(kind, encodeURIComponent(id))).data,
}
