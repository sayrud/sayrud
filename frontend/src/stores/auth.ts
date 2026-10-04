import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { accountApi, type DeleteAccount } from '@/api/account'
import type { RequestParams } from '@/api/api'
import { authApi, type Profile } from '@/api/auth'
import { ApiError } from '@/api/client'
import { ssoApi } from '@/api/sso'
import type { Identity } from '@/collab/identity'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<Profile | null>(null)

  // Keep profile controls locked even when their page is left and reopened during a request.
  const updatingProfile = ref(false)

  let loading: Promise<Profile | null> | null = null
  /** The profile has been requested, so the guest pages do not request it again on every navigation. */
  let checked = false

  /** Collaborator identity of the signed-in user, the same as the one assigned by the server over WebSocket. */
  const identity = computed<Identity>(() => ({
    memberId: user.value ? `usr${user.value.id}` : '',
    name: user.value?.userName ?? '',
    color: user.value?.color ?? '#3370ff',
    avatarUrl: user.value?.avatarUrl ?? '',
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

  async function ldapSignIn(slug: string, username: string, password: string) {
    user.value = await ssoApi.ldapSignIn(slug, username, password)
  }

  async function signOut() {
    try {
      await authApi.signOut()
    } finally {
      user.value = null
    }
  }

  async function saveProfile(request: Promise<Profile>) {
    updatingProfile.value = true

    try {
      const profile = await request
      if (user.value?.id === profile.id) user.value = profile

      return profile
    } finally {
      updatingProfile.value = false
    }
  }

  function updateName(userName: string) {
    return saveProfile(authApi.updateProfile(userName))
  }

  function uploadAvatar(file: File, params: RequestParams = {}) {
    return saveProfile(authApi.uploadAvatar(file, params))
  }

  function removeAvatar() {
    return saveProfile(authApi.removeAvatar())
  }

  async function deleteAccount(confirm: DeleteAccount) {
    await accountApi.deleteAccount(confirm)
    user.value = null
  }

  /** Forgets the user after the session expires, without calling the API. */
  function clear() {
    user.value = null
  }

  return {
    user,
    updatingProfile,
    identity,
    ensureLoaded,
    signIn,
    signUp,
    ldapSignIn,
    signOut,
    updateName,
    uploadAvatar,
    removeAvatar,
    updatePassword: authApi.updatePassword,
    deleteAccount,
    clear,
  }
})
