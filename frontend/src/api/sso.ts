import type {
  AdminAuthProvider,
  AuthProviderConfig,
  CreateAuthProvider,
  ListAdminAuthProvidersResp,
  Profile,
  SiteAuthProvider,
  TestAuthProvider,
  UpdateAuthProvider,
  UserIdentity as ApiUserIdentity,
} from './api'
import { API_BASE, client } from './client'

export type { AdminAuthProvider, AuthProviderConfig, CreateAuthProvider, SiteAuthProvider, UpdateAuthProvider }

export type ProviderType = AdminAuthProvider['type']

/** lastUsedAt is null if the identity has never been used to sign in. */
export type UserIdentity = Omit<ApiUserIdentity, 'lastUsedAt'> & { lastUsedAt: string | null }

/** URL to start a redirect sign-in, it needs a full-page navigation since the server redirects to the identity provider. */
export function ssoStartURL(slug: string, mode: 'login' | 'link', redirect?: string): string {
  const query = new URLSearchParams({ mode })
  if (redirect) query.set('redirect', redirect)
  return `${API_BASE}/auth/sso/${encodeURIComponent(slug)}/start?${query}`
}

export const ssoApi = {
  ldapSignIn: async (slug: string, username: string, password: string) =>
    (await client.auth.ldapSignIn(slug, { username, password })).data as Profile,
  identities: async () => (await client.auth.listIdentities()).data as UserIdentity[],
  unbind: async (identityId: number) => {
    await client.auth.deleteIdentity(identityId)
  },

  providers: async () => (await client.admin.listAdminAuthProviders()).data as ListAdminAuthProvidersResp,
  create: async (body: CreateAuthProvider) => (await client.admin.createAdminAuthProvider(body)).data as AdminAuthProvider,
  update: async (id: number, body: UpdateAuthProvider) =>
    (await client.admin.updateAdminAuthProvider(id, body)).data as AdminAuthProvider,
  remove: async (id: number) => {
    await client.admin.deleteAdminAuthProvider(id)
  },
  reorder: async (ids: number[]) => {
    await client.admin.setAdminAuthProviderPositions({ ids })
  },
  test: async (body: TestAuthProvider) => {
    await client.admin.testAdminAuthProvider(body)
  },
}
