import assert from 'node:assert/strict'
import test from 'node:test'

import type { WorkflowParam } from '../src/types/workflow.ts'
import { DEFAULT_PARAM_ERROR_BODY, messageCodes, paramErrorCodes, setParamMessage } from '../src/utils/paramErrors.ts'

const param = (p: Partial<WorkflowParam>): WorkflowParam => ({ id: 'p1', key: 'k', source: 'body', type: 'string', required: false, ...p })

test('param error codes follow the param rules', () => {
  assert.deepEqual(paramErrorCodes(param({})), ['invalid_type'])
  assert.deepEqual(paramErrorCodes(param({ source: 'query' })), [])
  assert.deepEqual(paramErrorCodes(param({ required: true, format: 'phone', maxLength: 20 })), ['required', 'null', 'empty', 'invalid_type', 'too_long', 'format'])
  assert.deepEqual(paramErrorCodes(param({ source: 'header', type: 'integer', required: true, min: 1 })), ['required', 'empty', 'invalid_type', 'too_small'])
  assert.deepEqual(paramErrorCodes(param({ source: 'query', type: 'array', itemType: 'string', options: ['a'], maxItems: 3 })), ['too_many_items', 'not_option'])
  assert.deepEqual(paramErrorCodes(param({ type: 'enum', options: ['a'] })), ['invalid_type', 'not_option'])
  assert.deepEqual(paramErrorCodes(param({ pattern: '^a$' })), ['invalid_type', 'pattern'])
})

test('param messages keep stale codes visible and drop empty ones', () => {
  const p = param({ messages: { too_long: 'x' } })
  assert.deepEqual(messageCodes(p), ['invalid_type', 'too_long'])
  setParamMessage(p, 'invalid_type', '类型错误')
  assert.deepEqual(p.messages, { too_long: 'x', invalid_type: '类型错误' })
  setParamMessage(p, 'too_long', '')
  setParamMessage(p, 'invalid_type', '')
  assert.equal('messages' in p, false)
})

test('the default template mirrors the default response', () => {
  assert.deepEqual(JSON.parse(DEFAULT_PARAM_ERROR_BODY), { error: { code: 'invalid_params', message: '{{error.message}}', details: '{{errors}}' } })
})
