import type {
  AdminCreateUser,
  AdminOverview as ApiOverview,
  AdminProject,
  AdminUser as ApiAdminUser,
  SystemSettings,
} from './api'
import { client } from './client'

export type { AdminProject, SystemSettings }

/** lastSignInAt is null if the user has never signed in. */
export type AdminUser = Omit<ApiAdminUser, 'lastSignInAt'> & { lastSignInAt: string | null }
export type AdminOverview = Omit<ApiOverview, 'recentUsers'> & { recentUsers: AdminUser[] }
export type UserStatusFilter = '' | 'active' | 'disabled' | 'admin'

export interface ListParams {
  page?: number
  pageSize?: number
  keyword?: string
}

export const adminApi = {
  overview: async () => (await client.admin.getAdminOverview()).data as AdminOverview,

  users: async ({ status, ...params }: ListParams & { status?: UserStatusFilter }) =>
    (await client.admin.listAdminUsers({ ...params, status: status || undefined })).data as {
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
}
