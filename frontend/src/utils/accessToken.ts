import type { AccessToken, AccessTokenScope } from '@/api/account'

export const ACCESS_TOKEN_SCOPES: AccessTokenScope[] = ['read', 'write', 'events']

/** Days until the token expires, never if 0, or a date picked by the user. */
export type TokenExpiry = 7 | 30 | 90 | 365 | 0 | 'custom'

export const TOKEN_EXPIRY_PRESETS: Exclude<TokenExpiry, 'custom'>[] = [7, 30, 90, 365, 0]

/** Tokens expiring within this many days are highlighted. */
export const EXPIRING_SOON_DAYS = 7

/** A project that can be picked to restrict a token. */
export interface TokenProjectOption {
  uid: string
  name: string
  icon?: string
  color?: string
}

/** The editable access of a token: its scopes, the projects it is restricted to and its expiration. */
export interface TokenAccessDraft {
  scopes: AccessTokenScope[]
  restricted: boolean
  projects: string[]
  expiry: TokenExpiry
  /** YYYY-MM-DD in local time, for the custom expiration. */
  customDate?: string
  /** The saved expiration, kept as is while its day is not changed. */
  savedExpiresAt?: string
}

export function newTokenAccess(): TokenAccessDraft {
  return { scopes: ['read'], restricted: false, projects: [], expiry: 90 }
}

function localDate(time: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(time.getDate())}`
}

/** Loads the access of a saved token for editing, the deleted projects are dropped. */
export function tokenAccessOf(token: Pick<AccessToken, 'scopes' | 'projects' | 'expiresAt'> & { expiresAt: string | null }): TokenAccessDraft {
  const projects = token.projects.filter((p) => !p.deleted).map((p) => p.uid)
  const draft: TokenAccessDraft = {
    scopes: sortScopes(token.scopes),
    restricted: token.projects.length > 0,
    projects,
    expiry: 0,
  }
  if (token.expiresAt) {
    draft.expiry = 'custom'
    draft.customDate = localDate(new Date(token.expiresAt))
    draft.savedExpiresAt = token.expiresAt
  }
  return draft
}

/** Returns the ISO expiration time sent to the server, undefined for never; a custom date expires at its end of day. */
export function tokenExpiresAt(expiry: TokenExpiry, customDate: string | undefined, now = new Date()): string | undefined {
  if (expiry === 0) return undefined
  if (expiry === 'custom') {
    if (!customDate) return undefined
    const [y, m, d] = customDate.split('-').map(Number)
    return new Date(y!, m! - 1, d!, 23, 59, 59).toISOString()
  }
  return new Date(now.getTime() + expiry * 86_400_000).toISOString()
}

/** Returns the expiration of the draft, keeping the saved time while its day is unchanged. */
export function draftExpiresAt(draft: TokenAccessDraft, now = new Date()): string | undefined {
  if (draft.expiry === 'custom' && draft.savedExpiresAt && draft.customDate === localDate(new Date(draft.savedExpiresAt))) {
    return draft.savedExpiresAt
  }
  return tokenExpiresAt(draft.expiry, draft.customDate, now)
}

/** The request body of the access, the projects are only sent when restricted. */
export function tokenAccessBody(draft: TokenAccessDraft, now = new Date()) {
  return {
    scopes: sortScopes(draft.scopes),
    projects: draft.restricted ? [...draft.projects] : [],
    expiresAt: draftExpiresAt(draft, now),
  }
}

export type TokenState = 'active' | 'expiring' | 'expired'

export function tokenState(token: { expired: boolean; expiresAt: string | null }, now = new Date()): TokenState {
  if (token.expired) return 'expired'
  if (!token.expiresAt) return 'active'
  const left = new Date(token.expiresAt).getTime() - now.getTime()
  if (left <= 0) return 'expired'
  return left <= EXPIRING_SOON_DAYS * 86_400_000 ? 'expiring' : 'active'
}

/** Keeps the scopes in the canonical order of the server. */
export function sortScopes(scopes: readonly AccessTokenScope[]): AccessTokenScope[] {
  return ACCESS_TOKEN_SCOPES.filter((s) => scopes.includes(s))
}
