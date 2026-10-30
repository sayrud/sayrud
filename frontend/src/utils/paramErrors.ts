import type { ParamErrorCode, WorkflowParam } from '@/types/workflow'

/** Variables a param error response can reference. */
export const PARAM_ERROR_VARIABLES = ['error.message', 'error.code', 'error.param', 'error.path', 'errors', 'run.id']

/** Custom response template equivalent to the default param error response. */
export const DEFAULT_PARAM_ERROR_BODY = JSON.stringify({ error: { code: 'invalid_params', message: '{{error.message}}', details: '{{errors}}' } }, null, 2)

const isSet = (n: number | undefined) => typeof n === 'number'

/** Error codes the param can report with its current rules, in validation order. */
export function paramErrorCodes(p: WorkflowParam): ParamErrorCode[] {
  const codes: ParamErrorCode[] = []
  if (p.required) {
    codes.push('required')
    if (p.source === 'body') codes.push('null')
    if (p.source === 'body' ? p.type === 'string' || p.type === 'enum' : p.type !== 'array') codes.push('empty')
  }
  // Query and header values are text, so only non-text types can have the wrong type there.
  const text = p.type === 'string' || p.type === 'enum' || (p.type === 'array' && p.itemType !== 'number')
  if (p.source === 'body' || !text) codes.push('invalid_type')
  if (isSet(p.minLength)) codes.push('too_short')
  if (isSet(p.maxLength)) codes.push('too_long')
  if (isSet(p.min)) codes.push('too_small')
  if (isSet(p.max)) codes.push('too_large')
  if (isSet(p.maxItems)) codes.push('too_many_items')
  if (p.pattern) codes.push('pattern')
  if (p.format) codes.push('format')
  if (p.type === 'enum' || (p.type === 'array' && p.options?.length)) codes.push('not_option')
  return codes
}

/** Codes shown in the message editor: the applicable ones, plus configured ones that no longer apply so they can be removed. */
export function messageCodes(p: WorkflowParam): ParamErrorCode[] {
  const codes = paramErrorCodes(p)
  for (const code of Object.keys(p.messages ?? {}) as ParamErrorCode[]) {
    if (!codes.includes(code)) codes.push(code)
  }
  return codes
}

/** Sets the message of a code; an empty message removes it. */
export function setParamMessage(p: WorkflowParam, code: ParamErrorCode, message: string) {
  const messages = { ...p.messages }
  if (message) messages[code] = message
  else delete messages[code]
  if (Object.keys(messages).length) p.messages = messages
  else delete p.messages
}
