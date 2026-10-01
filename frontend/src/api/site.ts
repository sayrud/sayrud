import type { SiteInfo } from './api'
import { client } from './client'

export type { SiteInfo }

export const siteApi = {
  get: async () => (await client.site.getSiteInfo()).data as SiteInfo,
}
