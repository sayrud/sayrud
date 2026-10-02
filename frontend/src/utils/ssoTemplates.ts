import type { AuthProviderConfig } from '@/api/api'

export type ProviderType = 'oauth2' | 'oidc' | 'saml' | 'ldap'

export interface ProviderTemplate {
  key: string
  type: ProviderType
  /** Brand names are not translated. */
  name: string
  icon: string
  slug: string
  config: AuthProviderConfig
}

/** Templates to add a sign-in method, they only prefill the form and are saved as plain oauth2 / oidc / saml / ldap configs. */
export const TEMPLATES: ProviderTemplate[] = [
  {
    key: 'github',
    type: 'oauth2',
    name: 'GitHub',
    icon: 'github',
    slug: 'github',
    config: {
      authURL: 'https://github.com/login/oauth/authorize',
      tokenURL: 'https://github.com/login/oauth/access_token',
      userInfoURL: 'https://api.github.com/user',
      emailsURL: 'https://api.github.com/user/emails',
      scopes: ['read:user', 'user:email'],
      subjectPath: 'id',
      emailPath: 'email',
      namePath: 'name',
    },
  },
  {
    key: 'gitlab',
    type: 'oidc',
    name: 'GitLab',
    icon: 'gitlab',
    slug: 'gitlab',
    config: { issuer: 'https://gitlab.com', scopes: ['openid', 'email', 'profile'] },
  },
  {
    key: 'gogs',
    type: 'oauth2',
    name: 'Gogs',
    icon: 'gogs',
    slug: 'gogs',
    config: {
      authURL: 'https://gogs.example.com/login/oauth/authorize',
      tokenURL: 'https://gogs.example.com/login/oauth/access_token',
      userInfoURL: 'https://gogs.example.com/api/v1/user',
      emailsURL: 'https://gogs.example.com/api/v1/user/emails',
      scopes: ['read:user'],
      subjectPath: 'id',
      emailPath: 'email',
      namePath: 'full_name',
    },
  },
  {
    key: 'gitea',
    type: 'oauth2',
    name: 'Gitea',
    icon: 'gitea',
    slug: 'gitea',
    config: {
      authURL: 'https://gitea.example.com/login/oauth/authorize',
      tokenURL: 'https://gitea.example.com/login/oauth/access_token',
      userInfoURL: 'https://gitea.example.com/api/v1/user',
      emailsURL: 'https://gitea.example.com/api/v1/user/emails',
      scopes: ['read:user'],
      subjectPath: 'id',
      emailPath: 'email',
      namePath: 'full_name',
    },
  },
  {
    key: 'google',
    type: 'oidc',
    name: 'Google',
    icon: 'google',
    slug: 'google',
    config: { issuer: 'https://accounts.google.com', scopes: ['openid', 'email', 'profile'] },
  },
  {
    key: 'microsoft',
    type: 'oidc',
    name: 'Microsoft',
    icon: 'microsoft',
    slug: 'microsoft',
    config: { issuer: 'https://login.microsoftonline.com/{tenant}/v2.0', scopes: ['openid', 'email', 'profile'] },
  },
  {
    key: 'keycloak',
    type: 'oidc',
    name: 'Keycloak',
    icon: 'keycloak',
    slug: 'keycloak',
    config: { issuer: 'https://keycloak.example.com/realms/{realm}', scopes: ['openid', 'email', 'profile'] },
  },
  { key: 'oidc', type: 'oidc', name: 'OpenID Connect', icon: 'oidc', slug: 'oidc', config: { scopes: ['openid', 'email', 'profile'] } },
  {
    key: 'oauth2',
    type: 'oauth2',
    name: 'OAuth 2.0',
    icon: 'oauth2',
    slug: 'oauth',
    config: { subjectPath: 'id', emailPath: 'email', namePath: 'name' },
  },
  { key: 'saml', type: 'saml', name: 'SAML 2.0', icon: 'saml', slug: 'saml', config: { trustEmail: true } },
  {
    key: 'ldap',
    type: 'ldap',
    name: 'LDAP',
    icon: 'ldap',
    slug: 'ldap',
    config: {
      userFilter: '(&(objectClass=person)(uid={username}))',
      subjectAttribute: 'uid',
      emailAttribute: 'mail',
      nameAttribute: 'displayName',
      groupsAttribute: 'memberOf',
      trustEmail: true,
    },
  },
  {
    key: 'ad',
    type: 'ldap',
    name: 'Active Directory',
    icon: 'windows',
    slug: 'ad',
    config: {
      userFilter: '(&(objectClass=user)(sAMAccountName={username}))',
      subjectAttribute: 'objectGUID',
      emailAttribute: 'mail',
      nameAttribute: 'displayName',
      groupsAttribute: 'memberOf',
      trustEmail: true,
    },
  },
]

/** 可选图标，与 ProviderIcon 支持的取值一致。 */
export const ICONS = ['github', 'gitlab', 'gogs', 'gitea', 'google', 'microsoft', 'windows', 'keycloak', 'oidc', 'oauth2', 'saml', 'ldap']

/** Data of the admin form, the empty secrets keep the saved ones. */
export interface ProviderDraft {
  name: string
  slug: string
  icon: string
  type: ProviderType
  enabled: boolean
  config: AuthProviderConfig
  clientSecret: string
  bindPassword: string
  autoCreateUser: boolean
  linkByEmail: boolean
  allowedEmailDomains: string[]
  allowedGroups: string[]
}

export const SLUG_PATTERN = /^[a-z0-9][a-z0-9-]{0,31}$/

/** Returns a slug not in taken, e.g. github, github-2. */
export function uniqueSlug(base: string, taken: string[]): string {
  if (!taken.includes(base)) return base
  for (let i = 2; ; i++) {
    const slug = `${base.slice(0, 32 - String(i).length - 1)}-${i}`
    if (!taken.includes(slug)) return slug
  }
}

export function draftFromTemplate(template: ProviderTemplate, takenSlugs: string[]): ProviderDraft {
  return {
    name: template.name,
    slug: uniqueSlug(template.slug, takenSlugs),
    icon: template.icon,
    type: template.type,
    enabled: true,
    config: structuredClone(template.config),
    clientSecret: '',
    bindPassword: '',
    autoCreateUser: false,
    linkByEmail: true,
    allowedEmailDomains: [],
    allowedGroups: [],
  }
}

/** Returns the URLs to fill in at the identity provider, empty if the external URL is not set. */
export function providerURLs(externalURL: string, slug: string, type: ProviderType) {
  const base = externalURL ? `${externalURL}/_/auth/sso/${slug}` : ''
  return {
    callbackURL: base && (type === 'oauth2' || type === 'oidc') ? `${base}/callback` : '',
    acsURL: base && type === 'saml' ? `${base}/acs` : '',
    metadataURL: base && type === 'saml' ? `${base}/metadata` : '',
  }
}
