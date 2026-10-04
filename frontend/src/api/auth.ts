import type { Profile, RequestParams } from './api'
import { client } from './client'

export type { Profile }

export const authApi = {
  profile: async () => (await client.auth.getProfile()).data as Profile,
  signIn: async (email: string, password: string) => (await client.auth.signIn({ email, password })).data as Profile,
  signUp: async (email: string, userName: string, password: string) =>
    (await client.auth.signUp({ email, userName, password })).data as Profile,
  signOut: async () => {
    await client.auth.signOut()
  },

  updateProfile: async (userName: string) => (await client.auth.updateProfile({ userName })).data as Profile,

  uploadAvatar: async (file: File, params: RequestParams = {}) =>
    (await client.auth.uploadAvatar({ file }, params)).data as Profile,
  removeAvatar: async () => (await client.auth.removeAvatar()).data as Profile,

  updatePassword: async (oldPassword: string, newPassword: string) => {
    await client.auth.updatePassword({ oldPassword, newPassword })
  },
}
