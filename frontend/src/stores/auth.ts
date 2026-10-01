import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { accountApi } from '@/api/account'
import { authApi, type Profile } from '@/api/auth'
import { ApiError } from '@/api/client'
import type { Identity } from '@/collab/identity'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<Profile | null>(null)
  let loading: Promise<Profile | null> | null = null
  /** The profile has been requested, so the guest pages do not request it again on every navigation. */
  let checked = false

  /** Collaborator identity of the signed-in user, the same as the one assigned by the server over WebSocket. */
  const identity = computed<Identity>(() => ({
    memberId: user.value ? `usr${user.value.id}` : '',
    name: user.value?.userName ?? '',
    color: user.value?.color ?? '#3370ff',
  }))

  /** Loads the profile once, it resolves null if not signed in. */
  function ensureLoaded(): Promise<Profile | null> {
    if (user.value || checked) return Promise.resolve(user.value)
    loading ??= authApi
      .profile()
      .then((p) => (user.value = p))
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 401) return null
        throw e
      })
      .then((p) => {
        checked = true
        return p
      })
      .finally(() => (loading = null))
    return loading
  }

  async function signIn(email: string, password: string) {
    user.value = await authApi.signIn(email, password)
  }

  async function signUp(email: string, userName: string, password: string) {
    user.value = await authApi.signUp(email, userName, password)
  }

  async function signOut() {
    try {
      await authApi.signOut()
    } finally {
      user.value = null
    }
  }

  async function updateName(userName: string) {
    user.value = await authApi.updateProfile(userName)
  }

  async function deleteAccount(password: string) {
    await accountApi.deleteAccount(password)
    user.value = null
  }

  /** Forgets the user after the session expires, without calling the API. */
  function clear() {
    user.value = null
  }

  return {
    user,
    identity,
    ensureLoaded,
    signIn,
    signUp,
    signOut,
    updateName,
    updatePassword: authApi.updatePassword,
    deleteAccount,
    clear,
  }
})
