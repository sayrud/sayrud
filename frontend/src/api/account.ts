import type { UserSession, UserSettings } from './api'
import { client } from './client'

export type { UserSession, UserSettings }

export const accountApi = {
  settings: async () => (await client.auth.getUserSettings()).data as UserSettings,
  saveSettings: async (settings: UserSettings) =>
    (await client.auth.updateUserSettings(settings)).data as UserSettings,
  sessions: async () => (await client.auth.listSessions()).data as UserSession[],
  revokeSession: async (sessionId: number) => {
    await client.auth.revokeSession(sessionId)
  },
  revokeOthers: async () => {
    await client.auth.revokeOtherSessions()
  },
  deleteAccount: async (password: string) => {
    await client.auth.deleteAccount({ password })
  },
}
