import type { AccessToken, CreatedAccessToken } from './account'
import type { Bot as ApiBot, CreateBot, RequestParams, UpdateBotAccess, UserBrief } from './api'
import { client } from './client'

export type { CreateBot, UpdateBotAccess }

/** owner is null only if the owner has just been deleted, token is null if the bot has none yet. */
export type Bot = Omit<ApiBot, 'owner' | 'token'> & { owner: UserBrief | null; token: AccessToken | null }

/** The secret of the only access token of the new bot, it is only returned once. */
export interface CreatedBot {
  bot: Bot
  secret: string
}

/** Manages bots with the same components in the admin console and the account settings. */
export interface BotApi {
  get: (botId: number) => Promise<Bot>
  create: (body: CreateBot) => Promise<CreatedBot>
  rename: (botId: number, userName: string) => Promise<void>
  setDisabled: (botId: number, disabled: boolean) => Promise<void>
  uploadAvatar: (botId: number, file: File, params?: RequestParams) => Promise<UserBrief>
  removeAvatar: (botId: number) => Promise<UserBrief>
  /** Changes what the token may do, its secret stays the same. */
  updateAccess: (botId: number, body: UpdateBotAccess) => Promise<Bot>
  /** Replaces the secret of the token, the old one stops working at once. */
  resetToken: (botId: number) => Promise<CreatedAccessToken>
}

export const adminBotsApi: BotApi = {
  get: async (botId) => (await client.admin.getAdminBot(botId)).data as Bot,
  create: async (body) => (await client.admin.createAdminBot(body)).data as CreatedBot,
  rename: async (botId, userName) => {
    await client.admin.updateAdminUser(botId, { userName })
  },
  setDisabled: async (botId, disabled) => {
    await client.admin.setAdminUserStatus(botId, { disabled })
  },
  uploadAvatar: async (botId, file, params = {}) => (await client.admin.uploadAdminBotAvatar(botId, { file }, params)).data,
  removeAvatar: async (botId) => (await client.admin.removeAdminBotAvatar(botId)).data,
  updateAccess: async (botId, body) => (await client.admin.updateAdminBotAccess(botId, body)).data as Bot,
  resetToken: async (botId) => (await client.admin.resetAdminBotToken(botId)).data as CreatedAccessToken,
}

/** limit is 0 if the member is not limited, canCreate tells whether one more bot can be created now. */
export interface MyBots {
  bots: Bot[]
  limit: number
  canCreate: boolean
}

export const myBotsApi: BotApi & { list: () => Promise<MyBots>; remove: (botId: number) => Promise<void> } = {
  list: async () => (await client.auth.listMyBots()).data as MyBots,
  get: async (botId) => (await client.auth.getMyBot(botId)).data as Bot,
  create: async (body) => (await client.auth.createMyBot(body)).data as CreatedBot,
  rename: async (botId, userName) => {
    await client.auth.updateMyBot(botId, { userName })
  },
  setDisabled: async (botId, disabled) => {
    await client.auth.setMyBotStatus(botId, { disabled })
  },
  remove: async (botId) => {
    await client.auth.deleteMyBot(botId)
  },
  uploadAvatar: async (botId, file, params = {}) => (await client.auth.uploadMyBotAvatar(botId, { file }, params)).data,
  removeAvatar: async (botId) => (await client.auth.removeMyBotAvatar(botId)).data,
  updateAccess: async (botId, body) => (await client.auth.updateMyBotAccess(botId, body)).data as Bot,
  resetToken: async (botId) => (await client.auth.resetMyBotToken(botId)).data as CreatedAccessToken,
}
