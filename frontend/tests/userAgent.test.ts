import assert from 'node:assert/strict'
import test from 'node:test'

import { parseUserAgent } from '../src/utils/userAgent.ts'

test('parses the browser, os and device of common user agents', () => {
  assert.deepEqual(
    parseUserAgent(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36',
    ),
    { browser: 'Chrome', os: 'macOS', device: 'desktop' },
  )
  assert.deepEqual(
    parseUserAgent(
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36 Edg/129.0.0.0',
    ),
    { browser: 'Edge', os: 'Windows', device: 'desktop' },
  )
  assert.deepEqual(
    parseUserAgent(
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1',
    ),
    { browser: 'Safari', os: 'iOS', device: 'mobile' },
  )
  assert.deepEqual(
    parseUserAgent(
      'Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
    ),
    { browser: 'Safari', os: 'iOS', device: 'tablet' },
  )
  assert.deepEqual(
    parseUserAgent(
      'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Mobile Safari/537.36',
    ),
    { browser: 'Chrome', os: 'Android', device: 'mobile' },
  )
  assert.deepEqual(parseUserAgent('Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0'), {
    browser: 'Firefox',
    os: 'Linux',
    device: 'desktop',
  })
  assert.deepEqual(parseUserAgent(''), { browser: '未知浏览器', os: '未知系统', device: 'desktop' })
})
