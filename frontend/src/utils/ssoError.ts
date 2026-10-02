/** Error codes returned by the server in the sso_error query, the same as the constants in internal/sso. */
export const SSO_ERROR_CODES = [
  'provider_not_found',
  'sso_not_configured',
  'invalid_state',
  'idp_error',
  'access_denied',
  'not_linked',
  'email_required',
  'email_taken',
  'identity_taken',
  'account_disabled',
] as const

/** Returns the message key of the error code, unknown codes are treated as idp_error. */
export function ssoErrorKey(code: unknown): string {
  const known = (SSO_ERROR_CODES as readonly unknown[]).includes(code)
  return `sso.errors.${known ? code : 'idp_error'}`
}
