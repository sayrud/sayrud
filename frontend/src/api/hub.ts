import type { HubOrigin, HubRelease, HubUpdate, HubUpdatesResp } from './api'
import { client } from './client'

export type { HubOrigin, HubRelease, HubUpdate, HubUpdatesResp }

export type HubKind = HubUpdate['kind']

export const hubApi = {
  /** Checks the packages installed from Sayrud Hub; refresh asks the Hub again instead of reusing the last result. */
  updates: async (refresh = false) => (await client.admin.checkAdminHubUpdates(refresh ? { refresh } : {})).data,
}
