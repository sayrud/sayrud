import type {
  AdminAISettings,
  AdminCreateUser,
  AdminOverview as ApiOverview,
  AdminProject,
  AdminUser as ApiAdminUser,
  SystemSettings,
  UpdateAISettings,
} from './api'
import type { Bot } from './bots'
import { client } from './client'

export type { AdminAISettings, AdminProject, SystemSettings, UpdateAISettings }

/** lastSignInAt is null if the user has never signed in. */
export type AdminUser = Omit<ApiAdminUser, 'lastSignInAt'> & { lastSignInAt: string | null }
export type AdminOverview = Omit<ApiOverview, 'recentUsers'> & { recentUsers: AdminUser[] }
export type UserStatusFilter = '' | 'active' | 'disabled' | 'admin'
export type BotStatusFilter = '' | 'active' | 'disabled'

export interface ListParams {
  page?: number
  pageSize?: number
  keyword?: string
}

export const adminApi = {
  overview: async () => (await client.admin.getAdminOverview()).data as AdminOverview,

  /** kind is empty to list both the people and the bots. */
  users: async ({ status, kind, ...params }: ListParams & { status?: UserStatusFilter; kind?: '' | 'human' | 'bot' }) =>
    (await client.admin.listAdminUsers({ ...params, status: status || undefined, kind: kind || undefined })).data as {
      users: AdminUser[]
      total: number
    },
  createUser: async (body: AdminCreateUser) => (await client.admin.createAdminUser(body)).data as AdminUser,
  updateUser: async (userId: number, userName: string) => {
    await client.admin.updateAdminUser(userId, { userName })
  },
  setAdmin: async (userId: number, isAdmin: boolean) => {
    await client.admin.setAdminUserAdmin(userId, { isAdmin })
  },
  setDisabled: async (userId: number, disabled: boolean) => {
    await client.admin.setAdminUserStatus(userId, { disabled })
  },
  resetPassword: async (userId: number, password: string) => {
    await client.admin.resetAdminUserPassword(userId, { password })
  },
  revokeSessions: async (userId: number) => {
    await client.admin.revokeAdminUserSessions(userId)
  },
  deleteUser: async (userId: number, transferTo?: number) => {
    await client.admin.deleteAdminUser(userId, transferTo ? { transferTo } : undefined)
  },

  bots: async ({ status, ...params }: ListParams & { status?: BotStatusFilter }) =>
    (await client.admin.listAdminBots({ ...params, status: status || undefined })).data as { bots: Bot[]; total: number },
  /** Hands the bot over to another member, who then limits its access and alone can invite it. */
  setBotOwner: async (botId: number, ownerId: number) => (await client.admin.setAdminBotOwner(botId, { ownerId })).data as Bot,

  projects: async (params: ListParams) =>
    (await client.admin.listAdminProjects(params)).data as { projects: AdminProject[]; total: number },
  transferProject: async (projectUid: string, userID: number) => {
    await client.admin.transferAdminProject(projectUid, { userID })
  },
  deleteProject: async (projectUid: string) => {
    await client.admin.deleteAdminProject(projectUid)
  },

  settings: async () => (await client.admin.getSystemSettings()).data as SystemSettings,
  saveSettings: async (s: SystemSettings) => (await client.admin.updateSystemSettings(s)).data as SystemSettings,

  aiSettings: async () => (await client.admin.getAdminAiSettings()).data,
  saveAISettings: async (body: UpdateAISettings) => (await client.admin.updateAdminAiSettings(body)).data,
  testAISettings: async (body: UpdateAISettings) => (await client.admin.testAdminAiSettings(body)).data,
}
