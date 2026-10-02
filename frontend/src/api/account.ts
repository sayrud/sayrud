import type { DeleteAccount, UpdateUserSettings, UserSession, UserSettings } from './api'
import { client } from './client'

export type { DeleteAccount, UpdateUserSettings, UserSession, UserSettings }

export const accountApi = {
  settings: async () => (await client.auth.getUserSettings()).data as UserSettings,
  saveSettings: async (settings: UpdateUserSettings) =>
    (await client.auth.updateUserSettings(settings)).data as UserSettings,
  sessions: async () => (await client.auth.listSessions()).data as UserSession[],
  revokeSession: async (sessionId: number) => {
    await client.auth.revokeSession(sessionId)
  },
  revokeOthers: async () => {
    await client.auth.revokeOtherSessions()
  },
  /** Users with a password confirm with it, the others with their email. */
  deleteAccount: async (confirm: DeleteAccount) => {
    await client.auth.deleteAccount(confirm)
  },
}
