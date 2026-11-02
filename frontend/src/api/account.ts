import type {
  AccessToken as ApiAccessToken,
  CreateAccessToken,
  DeleteAccount,
  UpdateUserSettings,
  UserSession,
  UserSettings,
} from './api'
import { client } from './client'

export type { CreateAccessToken, DeleteAccount, UpdateUserSettings, UserSession, UserSettings }

export type AccessTokenScope = CreateAccessToken['scopes'][number]

/** expiresAt and lastUsedAt are null if the token never expires or has never been used. */
export type AccessToken = Omit<ApiAccessToken, 'expiresAt' | 'lastUsedAt'> & { expiresAt: string | null; lastUsedAt: string | null }

/** The secret is the plaintext of the new token, it is only returned once. */
export interface CreatedAccessToken {
  token: AccessToken
  secret: string
}

/** Lists, creates and revokes the tokens of one account, shared by the account settings and the bot management. */
export interface AccessTokenApi {
  list: () => Promise<AccessToken[]>
  create: (body: CreateAccessToken) => Promise<CreatedAccessToken>
  revoke: (tokenId: number) => Promise<void>
}

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
  tokens: {
    list: async () => (await client.auth.listAccessTokens()).data as AccessToken[],
    create: async (body: CreateAccessToken) => (await client.auth.createAccessToken(body)).data as CreatedAccessToken,
    revoke: async (tokenId: number) => {
      await client.auth.revokeAccessToken(tokenId)
    },
  } satisfies AccessTokenApi,
}
